package subscriptions

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"corefusion/internal/model/entity"
	"corefusion/internal/service/healthprobe"
	"corefusion/internal/subscription/formatter"
	"corefusion/internal/subscription/sharelink"
)

// 链路探测的目标/出口解析：复用订阅渲染的同一套逻辑（协议凭证、传输层、
// 前置 TLS、接入域名），保证"探测用的参数"与"客户端实际拿到的参数"一致。
//
// ⚠ 副作用说明：探测必须用入站里**真实存在**的账号完成协议握手，所以会
// 从每个入站的首个账号发起一次很小的 HTTP 请求（trace 页，几 KB）。
// 该账号会因此产生少量流量、并在探测时刻短暂显示为在线 —— 这是验证
// 真实链路的必要代价，管理端会在界面上提示。

// chainProbeEntry 是一个待测的 CDN 条目。
//
// ⚠ 只包含「CDN 池」里的条目（cf_domain / cf_ip）。直连域名（direct）与
// 各入站自己的 custom 域名一律不纳入：它们指向各自的回源节点，域名和入口
// 都不同，用一条探针入站去测必然失败，测出来的"坏"是假结论。
type chainProbeEntry struct {
	Kind     string // cf_domain / cf_ip
	EntryID  int64  // share_domain_configs.id 或 cf_ip_entries.id
	Host     string
	Port     int
	PrevFail int
	// Healthy 是上次探测结论：名额分配时优先测健康条目，坏条目保留恢复名额。
	Healthy bool
}

// chainProbeInbound 是「探针入站」：一轮只用它完成协议握手。
//
// 两个来源：
//   - 自建节点入站：cfg/view 非空，按订阅渲染同款逻辑生成客户端出站；
//   - 外部导入节点：ext/proxy 非空，proxy 是导入的完整客户端出站参数，
//     每个目标只替换 server/port（与订阅渲染外部节点的语义一致）。
type chainProbeInbound struct {
	cfg         *entity.InboundConfig
	view        map[string]interface{}
	ad          map[string]interface{}
	adType      string
	nodeName    string
	displayName string
	// 外部导入节点作为探针入站时使用。
	ext   *entity.ExternalNode
	proxy map[string]interface{}
}

func (p chainProbeInbound) protocol() string {
	if p.cfg != nil {
		return p.cfg.Protocol
	}
	if p.ext != nil {
		return p.ext.Protocol
	}
	return ""
}

func (p chainProbeInbound) value() string {
	if p.ext != nil {
		return fmt.Sprintf("ext:%d", p.ext.ID)
	}
	if p.cfg != nil {
		return fmt.Sprintf("inb:%d", p.cfg.ID)
	}
	return ""
}

func (p chainProbeInbound) label() string {
	if p.cfg == nil {
		return fmt.Sprintf("%s (%s)", p.displayName, p.protocol())
	}
	node := p.nodeName
	if node == "" {
		node = fmt.Sprintf("节点 %d", p.cfg.ServerID)
	}
	return fmt.Sprintf("%s / %s:%d", node, p.cfg.Protocol, p.cfg.Port)
}

// ChainProbeInbounds 返回可作为「探针入站」的候选：启用的、接入域名类型为
// cf_domain / cf_ip 的入站，以及同样配了 CDN 接入的外部导入节点。
// 关闭 CDN（enabled=false）或直连（direct / custom）的入口不参与探测。
func (h *PublicHandler) ChainProbeInbounds(ctx context.Context) ([]healthprobe.ChainProbeInbound, error) {
	probes := h.chainProbeInbounds(ctx)
	out := make([]healthprobe.ChainProbeInbound, 0, len(probes))
	for _, p := range probes {
		out = append(out, healthprobe.ChainProbeInbound{
			Value:        p.value(),
			Label:        p.label(),
			Protocol:     p.protocol(),
			Host:         p.nodeName,
			AccessType:   p.adType,
			AccessDomain: chainProbeAccessDomainValue(p.ad, p.adType),
		})
	}
	return out, nil
}

// ChainProbeTargets 返回「全部启用的 CDN 域名/IP」条目，并用 probeSpec 指定的
// 入口（自建入站 "inb:<id>" / 外部导入节点 "ext:<id>"）渲染成客户端出站参数。
//
// 口径：一轮固定用这一个入站完成握手，把 CDN 池里的条目逐个测一遍（不限于
// Cloudflare：池里 cf_domain 类型可以放任何 CDN 的域名；cf_ip 是优选 IP）。
// 结果回写条目健康状态后，订阅就不会再分发坏条目。
//
// probeSpec 为空（还没选探针入站）时不产生目标 —— 不做随机兜底，避免
// 页面显示与定时探测用的不是同一条入站。
func (h *PublicHandler) ChainProbeTargets(ctx context.Context, probeSpec string) ([]healthprobe.ChainTarget, error) {
	probeSpec = strings.TrimSpace(probeSpec)
	if probeSpec == "" {
		return nil, nil
	}
	probes := h.chainProbeInbounds(ctx)
	if len(probes) == 0 {
		return nil, nil
	}
	probe, err := pickChainProbeInbound(probes, probeSpec)
	if err != nil {
		return nil, err
	}

	entries, tlsEntries := h.chainProbeEntries(ctx)
	// 探针入站里手填、但不在池子里的 CF 域名/IP：也要测（EntryID=0，只出结果不回写）。
	if extra := chainProbeExplicitTarget(probe.ad, probe.adType, entries); extra != nil {
		entries = append(entries, *extra)
	}
	if len(entries) == 0 {
		return nil, nil
	}

	out := make([]healthprobe.ChainTarget, 0, len(entries))
	for _, e := range entries {
		var (
			pm  map[string]interface{}
			err error
		)
		if probe.ext != nil {
			// 外部导入节点：proxy 已是完整客户端出站，只替换 CDN 条目的地址/端口。
			pm = chainProbeExternalTargetProxy(probe, e)
		} else {
			view := chainProbeTargetView(probe.view, e, tlsEntries)
			pm, err = chainProbeProxyMap(view, probe.cfg, e.Host, e.Port)
		}
		if err != nil || pm == nil {
			continue
		}
		var probeInboundID, serverID int64
		if probe.cfg != nil {
			probeInboundID = probe.cfg.ID
			serverID = probe.cfg.ServerID
		}
		out = append(out, healthprobe.ChainTarget{
			Kind:           e.Kind,
			EntryID:        e.EntryID,
			PrevFail:       e.PrevFail,
			Healthy:        e.Healthy,
			ProbeInboundID: probeInboundID,
			InboundID:      probeInboundID,
			ServerID:       serverID,
			NodeName:       probe.nodeName,
			Name:           probe.displayName,
			Protocol:       probe.protocol(),
			Host:           e.Host,
			Port:           e.Port,
			Proxy:          pm,
		})
	}
	// 稳定排序：游标轮转才能可预期地覆盖全部条目（不受 DB/map 顺序影响）。
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		if out[i].Host != out[j].Host {
			return out[i].Host < out[j].Host
		}
		return out[i].Port < out[j].Port
	})
	return out, nil
}

// chainProbeTargetView 用探针入站生成某个 CDN 目标的客户端视图。
//
// ⚠ 必须保留入站自己的 `_accessDomain.tls`（SNI/ALPN/指纹）—— 它才是该入站
// 真实的接入域名。之前把整段 tls 删掉，SNI 就变成优选域名/IP：
//   - 优选域名：CF 边缘按 SNI 路由到那个域名所属的站点，回源不会到本入站；
//   - 优选 IP：没有可用 SNI，TLS 握手直接被重置（实测 curl 35）。
//
// 两种情况下入站都收不到连接，探测却可能因为"出口能上网"而显示成功。
//
// 只有池条目**显式**配置了 SNI/ALPN/指纹时才覆盖：那是针对该 CDN 条目的特例。
func chainProbeTargetView(probeView map[string]interface{}, e chainProbeEntry, tlsEntries []shareDomainEntry) map[string]interface{} {
	view := deepCopyMap(probeView)
	if ad := getMap(view, "_accessDomain"); ad != nil {
		if tlsCfg := getMap(ad, "tls"); tlsCfg != nil {
			if entry := frontTLSEntry(e.Host, e.Kind, tlsEntries); entry != nil {
				if v := strings.TrimSpace(entry.SNI); v != "" {
					tlsCfg["sni"] = v
				}
				if v := strings.TrimSpace(entry.ALPN); v != "" {
					tlsCfg["alpn"] = v
				}
				if v := strings.TrimSpace(entry.Fingerprint); v != "" {
					tlsCfg["fingerprint"] = v
				}
			}
		}
	}
	applyFrontTLS(view, e.Host, e.Kind, tlsEntries)
	ensureTargetSNI(view)
	return view
}

// chainProbeExternalTargetProxy 用外部导入节点生成某个 CDN 目标的客户端出站。
//
// 与订阅渲染外部节点的语义一致：proxy_json 是用户导入的完整配置
// （SNI / Host / 指纹 / path / early-data 都在里面），只替换 server/port，
// 不改写任何 TLS 字段 —— 改写 SNI 会让 CDN 回源直接失败。
func chainProbeExternalTargetProxy(probe *chainProbeInbound, e chainProbeEntry) map[string]interface{} {
	if probe == nil || len(probe.proxy) == 0 {
		return nil
	}
	pm := cloneProxyMap(probe.proxy)
	if pm == nil {
		return nil
	}
	pm["server"] = e.Host
	pm["port"] = float64(e.Port)
	if probe.displayName != "" {
		pm["name"] = probe.displayName
	}
	return pm
}

// ensureTargetSNI 给「TLS 但 serverName 为空」的目标补上入站自己的接入 SNI。
//
// cf_ip 条目的 host 是 IP，入站又常是 nginx 前置的明文（security=none）：
// applyFrontTLS 会把 security 改成 tls，但 isIPHost 让 serverName 不会写成 IP。
// 不补的话客户端会拿 IP 当 SNI，TLS 握手必失败 —— 会把可用 IP 误判成坏。
func ensureTargetSNI(view map[string]interface{}) {
	ss := getMap(view, "streamSettings")
	if ss == nil {
		return
	}
	if strings.ToLower(strings.TrimSpace(getString(ss, "security", ""))) != "tls" {
		return
	}
	tlsCfg := getMap(ss, "tlsSettings")
	if tlsCfg == nil {
		tlsCfg = map[string]interface{}{}
		ss["tlsSettings"] = tlsCfg
	}
	if strings.TrimSpace(getString(tlsCfg, "serverName", "")) != "" {
		return
	}
	adTLS := getMap(getMap(view, "_accessDomain"), "tls")
	if adTLS == nil {
		return
	}
	if enabled, _ := adTLS["enabled"].(bool); !enabled {
		return
	}
	if sni := strings.TrimSpace(getString(adTLS, "sni", "")); sni != "" {
		tlsCfg["serverName"] = sni
	}
}

// pickChainProbeInbound 按 "inb:<id>"（自建入站）或 "ext:<id>"（外部导入节点）
// 选中探针入站。
func pickChainProbeInbound(probes []chainProbeInbound, spec string) (*chainProbeInbound, error) {
	var (
		id  int64
		err error
	)
	switch {
	case strings.HasPrefix(spec, "inb:"):
		id, err = strconv.ParseInt(strings.TrimPrefix(spec, "inb:"), 10, 64)
	case strings.HasPrefix(spec, "ext:"):
		id, err = strconv.ParseInt(strings.TrimPrefix(spec, "ext:"), 10, 64)
	default:
		return nil, fmt.Errorf("invalid probe inbound spec: %s", spec)
	}
	if err != nil || id <= 0 {
		return nil, fmt.Errorf("invalid probe inbound spec: %s", spec)
	}
	for i := range probes {
		if probes[i].cfg != nil && probes[i].cfg.ID == id && strings.HasPrefix(spec, "inb:") {
			return &probes[i], nil
		}
		if probes[i].ext != nil && probes[i].ext.ID == id && strings.HasPrefix(spec, "ext:") {
			return &probes[i], nil
		}
	}
	return nil, fmt.Errorf("probe inbound %s not found or has no CDN domain enabled", spec)
}

// chainProbeInbounds 收集所有「启用 + 接入域名开启且走 CDN」的入口：
// 自建节点入站，以及配了同样 CDN 接入的外部导入节点。
// 只认 cf_domain / cf_ip：direct（直连域名）和 custom（各节点自己的域名）
// 的入口和域名各不相同，不能拿一条入口的凭据代表它们，必须排除。
func (h *PublicHandler) chainProbeInbounds(ctx context.Context) []chainProbeInbound {
	if h.inboundRepo == nil {
		return nil
	}
	var out []chainProbeInbound
	for _, coreType := range []string{"xray", "singbox"} {
		cfgs, err := h.inboundRepo.ListAll(ctx, coreType)
		if err != nil {
			continue
		}
		for i := range cfgs {
			cfg := &cfgs[i]
			if !cfg.Enabled || cfg.Port == 0 || !shareableProtocols[cfg.Protocol] {
				continue
			}
			view, err := chainProbeView(cfg)
			if err != nil || view == nil {
				continue
			}
			ad := getMap(view, "_accessDomain")
			if ad == nil {
				continue
			}
			if enabled, _ := ad["enabled"].(bool); !enabled {
				continue
			}
			adType, _ := ad["type"].(string)
			// 只收 CDN 入站。direct 是直连域名，custom 是各节点自己的域名，
			// 它们不能共用一条探针入站的凭据，出现即排除。
			if adType != "cf_domain" && adType != "cf_ip" {
				continue
			}
			nodeName := ""
			if h.nodeServerRepo != nil {
				if node, err := h.nodeServerRepo.GetByID(ctx, cfg.ServerID); err == nil && node != nil {
					nodeName = node.Name
				}
			}
			out = append(out, chainProbeInbound{
				cfg: cfg, view: view, ad: ad, adType: adType,
				nodeName: nodeName, displayName: nodeDisplayName(cfg.Remark, cfg.Tag),
			})
		}
	}

	// 外部导入节点：访问域名启用且类型为 cf_* 时同样可以作为探针入站。
	// 与订阅渲染外部节点的语义一致：proxy_json 已是完整客户端出站，
	// 每个目标只替换 server/port，不改写 SNI/Host 等 TLS 字段。
	if h.extNodeRepo != nil {
		nodes, err := h.extNodeRepo.ListAll(ctx)
		if err == nil {
			for i := range nodes {
				node := &nodes[i]
				if !node.Enabled || strings.TrimSpace(node.ProxyJSON) == "" {
					continue
				}
				adType, ok := node.CDNAccessDomain()
				if !ok {
					continue
				}
				var pm map[string]interface{}
				if json.Unmarshal([]byte(node.ProxyJSON), &pm) != nil || len(pm) == 0 {
					continue
				}
				name := strings.TrimSpace(node.Name)
				if name == "" {
					name = node.Host
				}
				out = append(out, chainProbeInbound{
					ad: node.LoadAccessDomain(), adType: adType,
					displayName: name, ext: node, proxy: pm,
				})
			}
		}
	}
	return out
}

// chainProbeAccessDomainValue 取入站当前明确选择的域名/IP（空 = 从池里随机取）。
func chainProbeAccessDomainValue(ad map[string]interface{}, adType string) string {
	switch adType {
	case "cf_domain":
		return strings.TrimSpace(getString(ad, "cfDomain", ""))
	case "cf_ip":
		return strings.TrimSpace(getString(ad, "cfIP", ""))
	}
	return ""
}

// chainProbeEntries 收集全部启用的 CDN 池条目：
//   - share_domain_configs 里除 direct（直连）/ 旧 cf_ip 之外的类型 —— 实际上就是
//     cf_domain，CF 之外的 CDN 域名也放在这一类里，会一并纳入；
//   - cf_ip_entries 的优选 IP。
//
// ⚠ 直连域名（direct）与各入站的 custom 域名不纳入：它们各自指向不同的回源
// 节点，域名和入口都不同，用同一条探针入站去测必然失败，会把好条目误判成坏。
//
// 第二个返回值用于前置 TLS 判断（TLS 是否在 CDN 终结）。
func (h *PublicHandler) chainProbeEntries(ctx context.Context) ([]chainProbeEntry, []shareDomainEntry) {
	var out []chainProbeEntry
	var tlsEntries []shareDomainEntry
	seen := make(map[string]struct{})

	if h.shareDomainRepo != nil {
		items, err := h.shareDomainRepo.List(ctx, "")
		if err == nil {
			for _, d := range items {
				// direct 是直连域名，不经 CDN；cf_ip 统一走 cf_ip_entries，避免重复。
				if !d.Enabled || strings.TrimSpace(d.Value) == "" || d.Type == "direct" || d.Type == "cf_ip" {
					continue
				}
				ports := chainProbePorts(d.Ports)
				port := chainProbePickPort(ports, d.Port)
				key := d.Type + "|" + d.Value + "|" + strconv.Itoa(port)
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
				out = append(out, chainProbeEntry{Kind: d.Type, EntryID: d.ID, Host: d.Value, Port: port,
					PrevFail: d.FailCount, Healthy: d.Healthy})
				tlsEntries = append(tlsEntries, shareDomainEntry{
					Type: d.Type, Host: d.Value, Port: port, Ports: ports,
					TLSMode: d.TLSMode, SNI: d.SNI, ALPN: d.ALPN, Fingerprint: d.Fingerprint,
				})
			}
		}
	}

	if h.cfIPEntryRepo != nil {
		items, err := h.cfIPEntryRepo.List(ctx, 0)
		if err == nil {
			for _, ip := range items {
				if !ip.Enabled || strings.TrimSpace(ip.IP) == "" {
					continue
				}
				ports := chainProbePorts(ip.Ports)
				port := chainProbePickPort(ports, 443)
				key := "cf_ip|" + ip.IP + "|" + strconv.Itoa(port)
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
				out = append(out, chainProbeEntry{Kind: "cf_ip", EntryID: ip.ID, Host: ip.IP, Port: port,
					PrevFail: ip.FailCount, Healthy: ip.Healthy})
				// CF IP 没有独立的前置 TLS 条目：frontTLSEntry 对 cf_* 默认按 auto 处理。
				tlsEntries = append(tlsEntries, shareDomainEntry{Type: "cf_ip", Host: ip.IP, Port: port, Ports: ports, TLSMode: "auto"})
			}
		}
	}

	return out, tlsEntries
}

// chainProbeExplicitTarget 处理「探针入站里手填、但不在池子里」的 CF 域名/IP。
// 只有 cf_domain / cf_ip 会走到这里：直连（direct）和自定义（custom）域名
// 不属于 CDN 池，不参与链路探测。
func chainProbeExplicitTarget(ad map[string]interface{}, adType string, entries []chainProbeEntry) *chainProbeEntry {
	if ad == nil {
		return nil
	}
	pick := func(key string) string {
		v, _ := ad[key].(string)
		return strings.TrimSpace(v)
	}
	var host string
	switch adType {
	case "cf_domain":
		host = pick("cfDomain")
	case "cf_ip":
		host = pick("cfIP")
	default:
		return nil
	}
	if host == "" {
		return nil
	}
	for _, e := range entries {
		if e.Host == host {
			return nil
		}
	}
	port := 443
	switch v := ad["customPort"].(type) {
	case float64:
		if v > 0 {
			port = int(v)
		}
	case int:
		if v > 0 {
			port = v
		}
	}
	return &chainProbeEntry{Kind: adType, Host: host, Port: port}
}

func chainProbePorts(raw string) []int {
	var ports []int
	if strings.TrimSpace(raw) != "" {
		_ = json.Unmarshal([]byte(raw), &ports)
	}
	return ports
}

// chainProbePickPort 固定取第一个端口：每次随机换端口会让结果无法比较。
func chainProbePickPort(ports []int, fallback int) int {
	if len(ports) > 0 && ports[0] > 0 {
		return ports[0]
	}
	if fallback > 0 {
		return fallback
	}
	return 443
}

// ChainProbeExitProxy 解析出口节点出站参数。
//
//	"ext:<id>" → 导入的外部节点（external_nodes）
//	"inb:<id>" → 自建节点上的入站（inbound_configs）
func (h *PublicHandler) ChainProbeExitProxy(ctx context.Context, spec string) (map[string]interface{}, string, error) {
	spec = strings.TrimSpace(spec)
	switch {
	case strings.HasPrefix(spec, "ext:"):
		id, err := strconv.ParseInt(strings.TrimPrefix(spec, "ext:"), 10, 64)
		if err != nil || id <= 0 {
			return nil, "", fmt.Errorf("invalid egress node spec: %s", spec)
		}
		if h.extNodeRepo == nil {
			return nil, "", fmt.Errorf("external node repository unavailable")
		}
		node, err := h.extNodeRepo.GetByID(ctx, id)
		if err != nil || node == nil {
			return nil, "", fmt.Errorf("external node %d not found", id)
		}
		var pm map[string]interface{}
		if err := json.Unmarshal([]byte(node.ProxyJSON), &pm); err != nil || len(pm) == 0 {
			return nil, "", fmt.Errorf("external node %d has invalid configuration", id)
		}
		return pm, "外部节点 " + node.Name, nil

	case strings.HasPrefix(spec, "inb:"):
		id, err := strconv.ParseInt(strings.TrimPrefix(spec, "inb:"), 10, 64)
		if err != nil || id <= 0 {
			return nil, "", fmt.Errorf("invalid egress node spec: %s", spec)
		}
		if h.inboundRepo == nil {
			return nil, "", fmt.Errorf("inbound repository unavailable")
		}
		cfg, err := h.inboundRepo.GetByID(ctx, id)
		if err != nil || cfg == nil {
			return nil, "", fmt.Errorf("inbound %d not found", id)
		}
		view, err := chainProbeView(cfg)
		if err != nil || view == nil {
			return nil, "", fmt.Errorf("inbound %d configuration cannot be parsed", id)
		}
		shareDomains := h.loadShareDomains(ctx)

		// 出口优先用节点自己的对外地址：链路探测的意义就是"从这条线路出去"，
		// 走 CDN 反而绕回了 CDN。只有节点地址为空时才退回接入域名。
		host := ""
		nodeLabel := ""
		if h.nodeServerRepo != nil {
			if node, err := h.nodeServerRepo.GetByID(ctx, cfg.ServerID); err == nil && node != nil {
				nodeLabel = node.Name
				if node.IsLocal {
					// 面板本机的节点名/主机名不是可路由地址（如 "local-panel"）：
					// 临时核心与入站同机，直接走回环。
					host = "127.0.0.1"
				} else {
					host = preferNodeAddress(*node)
				}
			}
		}
		port := h.sharePort(ctx, *cfg, view)
		if port <= 0 {
			port = cfg.Port
		}
		directToNode := host != ""
		if !directToNode {
			if h2, p2, ok := resolveAccessDomain(view, port, shareDomains); ok && h2 != "" {
				host, port = h2, p2
				applyFrontTLS(view, host, accessDomainType(view), shareDomains)
			}
		}
		if host == "" {
			return nil, "", fmt.Errorf("inbound %d: its node has no usable address (public IP, hostname and private IP are all empty)", id)
		}
		pm, err := chainProbeProxyMap(view, cfg, host, port)
		if err != nil || pm == nil {
			return nil, "", fmt.Errorf("inbound %d: failed to build client outbound: %v", id, err)
		}
		// 直连节点地址时，入站证书通常签给域名（或面板自签），TLS 校验必然失败；
		// 这里只是拿它当跳板，跳过校验不影响安全性（探测流量不敏感）。
		if directToNode {
			if ss := getMap(view, "streamSettings"); getString(ss, "security", "none") == "tls" {
				pm["skip-cert-verify"] = true
			}
		}
		name := nodeLabel
		if name == "" {
			name = fmt.Sprintf("节点 %d", cfg.ServerID)
		}
		return pm, name + "/" + nodeDisplayName(cfg.Remark, cfg.Tag), nil
	}
	return nil, "", fmt.Errorf("unsupported egress spec: %s", spec)
}

// ChainProbeEntryTemplate 把入口渲染成「客户端配置模板」。
//
// 与 ChainProbeExitProxy 的关键差别：CDN 入站的 TLS 往往在 nginx 前置终止，
// 入站自身是明文（尤其 unix socket 架构），直接渲染出的出站没有 SNI。
// 这里按订阅同款逻辑把接入域名（cf_domain / cf_ip）的 TLS/SNI 应用进去，
// 之后由调用方只替换 server/port —— 这才是客户端真实组合。
func (h *PublicHandler) ChainProbeEntryTemplate(ctx context.Context, spec string) (map[string]interface{}, string, error) {
	spec = strings.TrimSpace(spec)
	if strings.HasPrefix(spec, "ext:") {
		return h.ChainProbeExitProxy(ctx, spec)
	}
	if !strings.HasPrefix(spec, "inb:") {
		return nil, "", fmt.Errorf("unsupported entry spec: %s", spec)
	}
	id, err := strconv.ParseInt(strings.TrimPrefix(spec, "inb:"), 10, 64)
	if err != nil || id <= 0 {
		return nil, "", fmt.Errorf("invalid entry spec: %s", spec)
	}
	if h.inboundRepo == nil {
		return nil, "", fmt.Errorf("inbound repository unavailable")
	}
	cfg, err := h.inboundRepo.GetByID(ctx, id)
	if err != nil || cfg == nil {
		return nil, "", fmt.Errorf("inbound %d not found", id)
	}
	view, err := chainProbeView(cfg)
	if err != nil || view == nil {
		return nil, "", fmt.Errorf("inbound %d configuration cannot be parsed", id)
	}

	nodeName := fmt.Sprintf("节点 %d", cfg.ServerID)
	nodeAddr := ""
	if h.nodeServerRepo != nil {
		if node, err := h.nodeServerRepo.GetByID(ctx, cfg.ServerID); err == nil && node != nil {
			if node.Name != "" {
				nodeName = node.Name
			}
			if node.IsLocal {
				nodeAddr = "127.0.0.1"
			} else {
				nodeAddr = preferNodeAddress(*node)
			}
		}
	}

	shareDomains := h.loadShareDomains(ctx)
	host, port := nodeAddr, cfg.Port
	ad := getMap(view, "_accessDomain")
	if adType := accessDomainType(view); adType == "cf_domain" || adType == "cf_ip" {
		if h2, p2, ok := resolveAccessDomain(view, port, shareDomains); ok && h2 != "" {
			host, port = h2, p2
		}
	}
	// 入站自己的接入 TLS 才是真实 SNI（nginx 前置时入站 security=none，
	// 池条目的 SNI 只是兜底）。主机名仅作占位，server 随后会被替换成优选条目。
	if tls := getMap(ad, "tls"); tls != nil {
		if enabled, _ := tls["enabled"].(bool); enabled {
			sni := getString(tls, "sni", "")
			if host == "" {
				host = sni
			}
			applyFrontTLSSettings(view, host, sni, getString(tls, "alpn", ""), getString(tls, "fingerprint", ""))
		}
	}
	if host == "" {
		return nil, "", fmt.Errorf("inbound %d: no usable address", id)
	}
	pm, err := chainProbeProxyMap(view, cfg, host, port)
	if err != nil || pm == nil {
		return nil, "", fmt.Errorf("inbound %d: failed to build client outbound: %v", id, err)
	}
	return pm, nodeName + "/" + nodeDisplayName(cfg.Remark, cfg.Tag), nil
}

// chainProbeView 把入站配置解析成 xray 形态（sing-box 入站归一），并确保
// _accessDomain 结构存在（历史数据兼容）。
func chainProbeView(cfg *entity.InboundConfig) (map[string]interface{}, error) {
	if cfg == nil || strings.TrimSpace(cfg.ConfigJSON) == "" {
		return nil, fmt.Errorf("empty config")
	}
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(cfg.ConfigJSON), &config); err != nil {
		return nil, err
	}
	if isSingboxInboundConfig(config) {
		user := firstSingboxUser(config)
		if user == nil {
			return nil, fmt.Errorf("sing-box inbound has no users")
		}
		config = singboxViewForUser(config, user, getString(user, "name", ""))
	}
	ensureAccessDomain(config)
	return config, nil
}

// firstSingboxUser 取 sing-box 入站的第一个用户（探测账号）。
func firstSingboxUser(config map[string]interface{}) map[string]interface{} {
	arr, ok := config["users"].([]interface{})
	if !ok {
		return nil
	}
	for _, item := range arr {
		if m, ok := item.(map[string]interface{}); ok {
			return m
		}
	}
	return nil
}

// firstAccount 取 socks/http 入站的第一个账号（settings.accounts）。
func firstAccount(settings map[string]interface{}) map[string]interface{} {
	arr, ok := settings["accounts"].([]interface{})
	if !ok {
		return nil
	}
	for _, item := range arr {
		if m, ok := item.(map[string]interface{}); ok {
			return m
		}
	}
	return nil
}

// chainProbeProxyMap 把（已归一化的）入站配置渲染成 mihomo 出站 map。
func chainProbeProxyMap(config map[string]interface{}, cfg *entity.InboundConfig, host string, port int) (map[string]interface{}, error) {
	if config == nil || cfg == nil || host == "" || port <= 0 {
		return nil, fmt.Errorf("incomplete target")
	}
	settings := getMap(config, "settings")

	// socks / http 入站的账号在 settings.accounts（无 clients），sharelink 走不通：
	// 直接构造出站 map（这两种协议本身没有传输层加密，字段很少）。
	if cfg.Protocol == "socks" || cfg.Protocol == "http" {
		pm := map[string]interface{}{
			"name":   nodeDisplayName(cfg.Remark, cfg.Tag),
			"type":   cfg.Protocol,
			"server": host,
			"port":   float64(port),
			"udp":    false,
		}
		if acc := firstAccount(settings); acc != nil {
			if v, ok := acc["user"].(string); ok && v != "" {
				pm["username"] = v
			}
			if v, ok := acc["pass"].(string); ok && v != "" {
				pm["password"] = v
			} else if v, ok := acc["password"].(string); ok && v != "" {
				pm["password"] = v
			}
		}
		return pm, nil
	}

	client := firstClient(settings)
	if client == nil {
		// shadowsocks 单用户入站的凭据在 settings 顶层（method + password），
		// 没有 clients 数组；合成一条给 sharelink 用，否则该入站永远不能当出口。
		if strings.HasPrefix(cfg.Protocol, "shadowsocks") || cfg.Protocol == "ss" {
			if pw := getString(settings, "password", ""); pw != "" {
				settings = deepCopyMap(settings)
				client = map[string]interface{}{"password": pw}
				settings["clients"] = []interface{}{client}
			}
		}
	}
	if client == nil {
		return nil, fmt.Errorf("inbound has no usable account")
	}

	var password, flow string
	password, _ = client["password"].(string)
	flow, _ = client["flow"].(string)
	// vision 只在裸 TCP/raw 下合法，否则客户端会握手失败。
	flow = formatter.FlowForTransport(flow, configTransportOf(config))
	if cfg.Protocol == "hysteria" {
		if auth, ok := client["auth"].(string); ok && auth != "" {
			password = auth
		}
	}
	if cfg.Protocol == "snell" {
		if psk := getString(config, "psk", ""); psk != "" {
			password = psk
		}
	}
	credExtra := map[string]interface{}{}
	applyClientCredentials(cfg.Protocol, client, config, &password, credExtra)
	_ = ss2022ClientPassword(config, cfg.Protocol, password) // 保持与订阅渲染同口径（长度校正已在生成链接时处理）

	shareNode := sharelink.Node{
		Tag:            nodeDisplayName(cfg.Remark, cfg.Tag),
		Protocol:       cfg.Protocol,
		Host:           host,
		Port:           port,
		Settings:       linkSettingsForUser(config, settings, cfg.Protocol, password),
		StreamSettings: getMap(config, "streamSettings"),
		FinalMask:      finalMaskWithPortHop(config),
	}
	link := sharelink.Generate(shareNode)
	if link == "" {
		return nil, fmt.Errorf("protocol %s cannot generate a share link", cfg.Protocol)
	}
	pms := convertLinksToMihomoProxyMaps([]string{link})
	if len(pms) == 0 || pms[0] == nil {
		return nil, fmt.Errorf("protocol %s cannot be converted to a client outbound", cfg.Protocol)
	}
	pm := pms[0]
	// 链路上必须指向我们解析出来的 CDN/节点地址：链接转换在个别协议
	// （如 trojan + reality）上会丢端口，出站就变成 port=0（xray 直接报
	// "Invalid Trojan port"）。这里以调用方给的 host/port 为准。
	pm["server"] = host
	pm["port"] = float64(port)
	return pm, nil
}
