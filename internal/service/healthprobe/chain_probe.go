package healthprobe

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// 链路探测（chain probe）：验证「国内/回国出口节点 → CDN 加速域名/IP → 入站」
// 这条完整链路是否真的能代理流量。
//
// 与 CheckAll 的区别：
//   - CheckAll 只测「本机/入口节点到 CF 优选 IP/域名」的 TCP/TLS 可达性；
//   - 链路探测会把入站渲染成真实的客户端出站（含协议凭证、传输层、TLS/REALITY），
//     从指定出口拨号访问一个外部 URL，要求 HTTP 200 且上下行都有字节。
//
// 为什么必须这样：CDN 节点在国内经常被封锁/限速，仅测 TCP 443 通不代表
// 能用（TLS 握手被 RST、回源域名不通、CDN 端口回源失败都会表现为"能连不能上网"）。
const (
	SettingChainEnabled     = "cfip.chain_probe.enabled"
	SettingChainExit        = "cfip.chain_probe.exit"
	SettingChainInbound     = "cfip.chain_probe.inbound"
	SettingChainConcurrency = "cfip.chain_probe.concurrency"
	SettingChainMaxTargets  = "cfip.chain_probe.max_targets"
	SettingChainProbeURL    = "cfip.chain_probe.probe_url"
	SettingChainResults     = "cfip.chain_probe.results"
	SettingChainCursor      = "cfip.chain_probe.cursor"

	DefaultChainConcurrency = 2
	DefaultChainMaxTargets  = 20
	DefaultChainProbeURL    = "https://www.cloudflare.com/cdn-cgi/trace"
	chainProbeTimeout       = 15 * time.Second
	chainProbeMaxBody       = 64 << 10
)

// ChainConfig 是链路探测配置（每次探测前从设置读取，改完下一轮生效）。
type ChainConfig struct {
	// Enabled 表示是否随定时探测一起执行链路探测。
	Enabled bool `json:"enabled"`
	// Exit 出口标识：
	//   "ext:<id>"  = 导入的外部节点（external_nodes）
	//   "inb:<id>"  = 自建节点上的入站（inbound_configs）
	//   ""          = 本机直连（不推荐：国内主机直连 CDN 不代表客户端线路）
	Exit string `json:"exit"`
	// Inbound 是「探针入站」标识（"inb:<id>"）：一轮探测只用它完成协议握手，
	// 把全部启用的 CDN 域名/IP 逐个测一遍。必须显式选择，不再随机挑：
	// 随机会导致同一批条目每轮用不同入站，结果无法横向比较。
	Inbound string `json:"inbound"`
	// Concurrency 是同时探测的目标数（1-8）。临时核心只起一个，
	// 并发太高只会让加密/握手抢占 CPU，默认 2。
	Concurrency int `json:"concurrency"`
	// MaxTargets 是单轮最多探测的目标数（1-500）。目标多时按游标轮转，
	// 避免一次把整个 CDN 集群全测一遍（CPU/带宽抖动）。
	MaxTargets int `json:"max_targets"`
	// ProbeURL 是穿透代理访问的 URL，默认 CF trace 页（返回纯文本且无缓存）。
	ProbeURL string `json:"probe_url"`
}

// ChainTarget 是一个待探测的 CDN 条目（域名或 IP）。
//
// 口径：**一轮固定用「一个」选定入站当探针**完成协议握手，把全部启用的 CDN
// 域名/IP（不限于 Cloudflare，任何 CDN 的域名都可以放进分享域名池）逐个测一遍；
// 结果按条目写回健康状态，订阅渲染就不会再分发坏条目。
type ChainTarget struct {
	// Kind 决定结果写回哪张表：cf_domain → share_domain_configs；cf_ip → cf_ip_entries；
	// custom（入站自定义 CDN 域名）没有独立表，只出结果不回写。
	Kind string `json:"kind"`
	// EntryID 是条目主键（0 = 不在池里、只出结果不回写）。
	EntryID int64 `json:"entry_id"`
	// PrevFail 是该条目当前连续失败次数（回写时按阈值累计，避免一次抖动下线）。
	PrevFail int `json:"-"`
	// ProbeInboundID 是本轮用来完成握手的入站（只用于展示/排障）。
	ProbeInboundID int64 `json:"probe_inbound_id"`
	// InboundID 与 ProbeInboundID 同值，保留给旧前端字段。
	InboundID int64  `json:"inbound_id"`
	ServerID  int64  `json:"server_id"`
	NodeName  string `json:"node_name"`
	Name      string `json:"name"` // 探针入站显示名
	Protocol  string `json:"protocol"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	// Proxy 是 mihomo 出站 map（JSON 形状），由订阅渲染同一套逻辑生成。
	// 不参与序列化：只给探针内部起临时核心用。
	Proxy map[string]interface{} `json:"-"`
}

// ChainResult 是单个 CDN 条目的探测结果。
type ChainResult struct {
	Kind           string `json:"kind"`
	EntryID        int64  `json:"entry_id"`
	ProbeInboundID int64  `json:"probe_inbound_id"`
	InboundID      int64  `json:"inbound_id"`
	ServerID       int64  `json:"server_id"`
	NodeName       string `json:"node_name"`
	Name           string `json:"name"`
	Protocol       string `json:"protocol"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Exit           string `json:"exit"`
	Healthy        bool   `json:"healthy"`
	HTTPStatus     int    `json:"http_status"`
	LatencyMs      int    `json:"latency_ms"`
	UpBytes        int64  `json:"up_bytes"`
	DownBytes      int64  `json:"down_bytes"`
	Error          string `json:"error"`
	CheckedAt      int64  `json:"checked_at"`
}

// ChainSummary 是一轮链路探测的汇总。
type ChainSummary struct {
	Checked    int           `json:"checked"`
	Healthy    int           `json:"healthy"`
	Unhealthy  int           `json:"unhealthy"`
	Skipped    int           `json:"skipped"`
	Total      int           `json:"total"`
	Entry      string        `json:"entry"`
	Probe      string        `json:"probe"`
	DurationMs int           `json:"duration_ms"`
	Results    []ChainResult `json:"results"`
}

// ChainProbeInbound 是「探针入站」候选（配了 CDN 接入域名的启用入站）。
type ChainProbeInbound struct {
	Value        string `json:"value"`         // "inb:<id>"
	Label        string `json:"label"`         // 节点 / 协议:端口
	Protocol     string `json:"protocol"`      //
	Host         string `json:"host"`          // 节点名
	AccessType   string `json:"access_type"`   // cf_domain / cf_ip / custom
	AccessDomain string `json:"access_domain"` // 该入站当前使用的域名/IP（空=从池里取）
}

// ChainTargetProvider 由订阅 handler 实现：负责把入站渲染成客户端出站参数。
// 放在接口里是为了让探测服务不反向依赖 HTTP handler 包。
type ChainTargetProvider interface {
	// ChainProbeTargets 返回全部启用的 CDN 域名/IP 目标，并用 probeSpec 指定的
	// 入站（"inb:<id>"）渲染成客户端出站参数。
	ChainProbeTargets(ctx context.Context, probeSpec string) ([]ChainTarget, error)
	// ChainProbeInbounds 返回可作为探针入站的候选。
	ChainProbeInbounds(ctx context.Context) ([]ChainProbeInbound, error)
	// ChainProbeExitProxy 按 "ext:<id>" / "inb:<id>" 解析出口节点的出站参数。
	ChainProbeExitProxy(ctx context.Context, spec string) (map[string]interface{}, string, error)
}

// SetChainTargetProvider 注入链路探测的目标/出口解析器。
func (p *Prober) SetChainTargetProvider(provider ChainTargetProvider) {
	p.chainProv = provider
}

// ChainTargets 转发给注入的解析器，供管理端预览待探测目标。
// 预览与实际探测使用同一个探针入站，避免页面显示与实际执行不一致。
func (p *Prober) ChainTargets(ctx context.Context) ([]ChainTarget, error) {
	if p.chainProv == nil {
		return nil, nil
	}
	return p.chainProv.ChainProbeTargets(ctx, p.ReadChainConfig(ctx).Inbound)
}

// ChainProbeInboundOptions 返回可作为探针入站的候选（管理端下拉用）。
func (p *Prober) ChainProbeInboundOptions(ctx context.Context) ([]ChainProbeInbound, error) {
	if p.chainProv == nil {
		return nil, nil
	}
	return p.chainProv.ChainProbeInbounds(ctx)
}

// ReadChainConfig 读取链路探测配置（缺失项回落默认值）。
func (p *Prober) ReadChainConfig(ctx context.Context) ChainConfig {
	cfg := ChainConfig{
		Enabled:     false,
		Concurrency: DefaultChainConcurrency,
		MaxTargets:  DefaultChainMaxTargets,
		ProbeURL:    DefaultChainProbeURL,
	}
	if p.setting == nil {
		return cfg
	}
	if v, err := p.setting.GetByKey(ctx, SettingChainEnabled); err == nil {
		cfg.Enabled = parseBoolSetting(v, false)
	}
	if v, err := p.setting.GetByKey(ctx, SettingChainExit); err == nil {
		cfg.Exit = strings.TrimSpace(v)
	}
	if v, err := p.setting.GetByKey(ctx, SettingChainInbound); err == nil {
		cfg.Inbound = strings.TrimSpace(v)
	}
	if v, err := p.setting.GetByKey(ctx, SettingChainConcurrency); err == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			cfg.Concurrency = clampInt(n, 1, 8)
		}
	}
	if v, err := p.setting.GetByKey(ctx, SettingChainMaxTargets); err == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			cfg.MaxTargets = clampInt(n, 1, 500)
		}
	}
	if v, err := p.setting.GetByKey(ctx, SettingChainProbeURL); err == nil {
		if u := strings.TrimSpace(v); u != "" {
			cfg.ProbeURL = u
		}
	}
	return cfg
}

// SaveChainConfig 保存链路探测配置（nil 表示不改该项）。
func (p *Prober) SaveChainConfig(ctx context.Context, cfg ChainConfig) error {
	if p.setting == nil {
		return errors.New("settings repository not available")
	}
	set := func(key, val string) error { return p.setting.Set(ctx, key, val) }
	if err := set(SettingChainEnabled, strconv.FormatBool(cfg.Enabled)); err != nil {
		return err
	}
	if err := set(SettingChainExit, strings.TrimSpace(cfg.Exit)); err != nil {
		return err
	}
	if err := set(SettingChainInbound, strings.TrimSpace(cfg.Inbound)); err != nil {
		return err
	}
	if err := set(SettingChainConcurrency, strconv.Itoa(clampInt(cfg.Concurrency, 1, 8))); err != nil {
		return err
	}
	if err := set(SettingChainMaxTargets, strconv.Itoa(clampInt(cfg.MaxTargets, 1, 500))); err != nil {
		return err
	}
	url := strings.TrimSpace(cfg.ProbeURL)
	if url == "" {
		url = DefaultChainProbeURL
	}
	if err := set(SettingChainProbeURL, url); err != nil {
		return err
	}
	p.Wake()
	return nil
}

// ChainResults 返回上次链路探测的结果（按检查时间倒序）。
func (p *Prober) ChainResults(ctx context.Context) ([]ChainResult, int64) {
	if p.setting == nil {
		return nil, 0
	}
	raw, err := p.setting.GetByKey(ctx, SettingChainResults)
	if err != nil || strings.TrimSpace(raw) == "" {
		return nil, 0
	}
	var payload struct {
		UpdatedAt int64         `json:"updated_at"`
		Results   []ChainResult `json:"results"`
	}
	if json.Unmarshal([]byte(raw), &payload) != nil {
		return nil, 0
	}
	return payload.Results, payload.UpdatedAt
}

// CheckChain 执行一轮链路探测。force=true 时忽略 Enabled 开关（手动检测）。
func (p *Prober) CheckChain(ctx context.Context, force bool) (ChainSummary, error) {
	cfg := p.ReadChainConfig(ctx)
	sum := ChainSummary{}
	if !cfg.Enabled && !force {
		return sum, nil
	}
	if p.chainProv == nil {
		return sum, errors.New("链路探测目标解析器未初始化")
	}
	if cfg.Exit == "" {
		return sum, errors.New("未选择出口节点：链路探测必须从一个真实出口发起，本机直连的结论没有参考意义")
	}
	if cfg.Inbound == "" {
		return sum, errors.New("未选择探针入站：链路探测固定用这一个入站完成协议握手，请先选择")
	}

	// chainMu 只保护链路探测自身：优选 IP 探测用 p.mu，两者分开加锁，
	// 避免定时任务里 CheckAll 持锁后再进 CheckChain 造成死锁。
	p.chainMu.Lock()
	defer p.chainMu.Unlock()

	targets, err := p.chainProv.ChainProbeTargets(ctx, cfg.Inbound)
	if err != nil {
		return sum, err
	}
	sum.Total = len(targets)
	if len(targets) == 0 {
		return sum, nil
	}

	// 轮转：目标多时每次只测一段，下一次从游标继续。
	cursor := p.readIntSetting(ctx, SettingChainCursor, 0)
	if cursor < 0 || cursor >= len(targets) {
		cursor = 0
	}
	limit := cfg.MaxTargets
	if limit <= 0 || limit > len(targets) {
		limit = len(targets)
	}
	picked := make([]ChainTarget, 0, limit)
	for i := 0; i < limit; i++ {
		picked = append(picked, targets[(cursor+i)%len(targets)])
	}
	sum.Skipped = len(targets) - len(picked)
	// 本轮探针入站（所有目标共用同一条入站做握手）。
	sum.Probe = fmt.Sprintf("%s (%s)", targets[0].Name, targets[0].Protocol)

	proxy, label, err := p.chainProv.ChainProbeExitProxy(ctx, cfg.Exit)
	if err != nil {
		return sum, err
	}
	// 拨号核心跟随「探测设置」：用户在探测设置里选了哪个就用哪个，
	// 为空时按自动顺序（xray → sing-box → mihomo）选，与优选探测完全同一口径。
	dialer, err := p.pickDialer(p.ReadConfig(ctx).Dialer)
	if err != nil {
		return sum, err
	}
	dial, cleanup, usedDialer, err := p.startDialerWithProxy(ctx, dialer, proxy)
	if err != nil {
		return sum, err
	}
	defer cleanup()
	sum.Entry = fmt.Sprintf("%s/%s", label, usedDialer)

	started := time.Now()
	results := make([]ChainResult, len(picked))
	sem := make(chan struct{}, clampInt(cfg.Concurrency, 1, 8))
	var wg sync.WaitGroup
	for i := range picked {
		t := picked[i]
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, target ChainTarget) {
			defer wg.Done()
			defer func() { <-sem }()
			res := ChainResult{
				Kind:           target.Kind,
				EntryID:        target.EntryID,
				ProbeInboundID: target.ProbeInboundID,
				InboundID:      target.InboundID,
				ServerID:       target.ServerID,
				NodeName:       target.NodeName,
				Name:           target.Name,
				Protocol:       target.Protocol,
				Host:           target.Host,
				Port:           target.Port,
				Exit:           sum.Entry,
				CheckedAt:      time.Now().Unix(),
			}
			if target.Proxy == nil {
				res.Error = "入站参数无法渲染为客户端出站"
				results[idx] = res
				return
			}
			status, up, down, latency, perr := probeChainHTTP(ctx, dial, cfg.ProbeURL)
			res.HTTPStatus = status
			res.LatencyMs = latency
			res.UpBytes = up
			res.DownBytes = down
			if perr != nil {
				res.Error = truncateErr(perr.Error())
			} else if status < 200 || status >= 400 {
				res.Error = fmt.Sprintf("HTTP %d", status)
			} else if up <= 0 || down <= 0 {
				res.Error = "链路无实际流量"
			} else {
				res.Healthy = true
			}
			results[idx] = res
		}(i, t)
	}
	wg.Wait()
	sum.DurationMs = int(time.Since(started) / time.Millisecond)

	// 结果写回条目健康状态：订阅渲染读的就是 Healthy，
	// 不通的条目因此不会再被分发给客户端。
	p.writeBackChainHealth(ctx, picked, results)

	// 游标推进（本轮测过的下一个位置）。
	next := (cursor + limit) % len(targets)
	_ = p.setting.Set(ctx, SettingChainCursor, strconv.Itoa(next))

	for _, r := range results {
		sum.Checked++
		if r.Healthy {
			sum.Healthy++
		} else {
			sum.Unhealthy++
		}
	}
	sum.Results = results
	p.saveChainResults(ctx, results, sum.Probe)
	return sum, nil
}

// ChainLastProbe 返回最近一轮实际使用的探针入站（页面展示用）。
// 目标列表按当前选择的探针入站生成，这里记录的是最近一次真正跑过的那一条。
func (p *Prober) ChainLastProbe(ctx context.Context) string {
	if p.setting == nil {
		return ""
	}
	raw, err := p.setting.GetByKey(ctx, SettingChainResults)
	if err != nil || strings.TrimSpace(raw) == "" {
		return ""
	}
	var payload struct {
		Probe string `json:"probe"`
	}
	if json.Unmarshal([]byte(raw), &payload) != nil {
		return ""
	}
	return payload.Probe
}

// saveChainResults 把本轮结果与历史结果合并（按入站 ID 覆盖），避免
// 轮转探测时没测到的目标从页面上消失。
func (p *Prober) saveChainResults(ctx context.Context, fresh []ChainResult, probe string) {
	if p.setting == nil || len(fresh) == 0 {
		return
	}
	old, _ := p.ChainResults(ctx)
	merged := make(map[string]ChainResult, len(old)+len(fresh))
	// 合并键必须按「条目」而不是入站：同一轮所有目标共用一条探针入站，
	// 用 InboundID 当键会把所有结果合并成一行。
	key := func(r ChainResult) string {
		return fmt.Sprintf("%s|%d|%s|%d", r.Kind, r.EntryID, r.Host, r.Port)
	}
	for _, r := range old {
		merged[key(r)] = r
	}
	for _, r := range fresh {
		merged[key(r)] = r
	}
	out := make([]ChainResult, 0, len(merged))
	for _, r := range merged {
		out = append(out, r)
	}
	payload, err := json.Marshal(map[string]interface{}{
		"updated_at": time.Now().Unix(),
		"probe":      probe,
		"results":    out,
	})
	if err != nil {
		return
	}
	_ = p.setting.Set(ctx, SettingChainResults, string(payload))
}

// writeBackChainHealth 把链路探测结果写回条目健康状态。
// 写回口径与原有探活一致：失败连续达到 failThreshold 次才置不健康，
// 成功立即恢复 —— 避免一次抖动把整池条目从订阅里摘掉。
func (p *Prober) writeBackChainHealth(ctx context.Context, targets []ChainTarget, results []ChainResult) {
	if len(targets) == 0 || len(results) != len(targets) {
		return
	}
	now := time.Now()
	for i, r := range results {
		t := targets[i]
		if t.EntryID <= 0 {
			continue // 不在池里的临时条目：只出结果，没有可写的行
		}
		healthy, fail := nextState(r.Healthy, t.PrevFail)
		// 只写健康状态与失败计数，不覆盖 latency_ms：链路延迟是端到端 HTTP
		// 往返（含协议握手 + TLS + 请求），与 TCP 探测的延迟量级不同；
		// 订阅的「延迟阈值」按 TCP 探测值筛选，混写会让筛选失真。
		if t.Kind == "cf_ip" {
			if p.cfRepo != nil {
				if err := p.cfRepo.UpdateHealthKeepLatency(ctx, t.EntryID, healthy, fail, now); err != nil {
					log.Printf("[healthprobe] chain update cf_ip %s failed: %v", t.Host, err)
				}
			}
			continue
		}
		// 其余类型都来自分享域名池（cf_domain 以及其它 CDN 域名）：写回同一张表。
		if p.shareRepo != nil {
			if err := p.shareRepo.UpdateHealthKeepLatency(ctx, t.EntryID, healthy, fail, now); err != nil {
				log.Printf("[healthprobe] chain update %s %s failed: %v", t.Kind, t.Host, err)
			}
		}
	}
}

func (p *Prober) readIntSetting(ctx context.Context, key string, def int) int {
	if p.setting == nil {
		return def
	}
	v, err := p.setting.GetByKey(ctx, key)
	if err != nil {
		return def
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return def
	}
	return n
}

// countingConn 统计一次连接上的上下行字节（含 TLS 握手），用于判定
// 「是否真的有流量」，而不是只看 HTTP 状态码。
type countingConn struct {
	net.Conn
	up   *int64
	down *int64
}

func (c *countingConn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	if n > 0 {
		atomic.AddInt64(c.down, int64(n))
	}
	return n, err
}

func (c *countingConn) Write(b []byte) (int, error) {
	// 必须**先计数再写 syscall**：TCP 是流式的，字节一进内核缓冲区对端就可能
	// 立刻回包；而汇总侧是在拿到响应后读计数。若先 write 后累加，快机器上
	// （CI runner 实测必现）会出现「请求已发出、响应已收到，但 up 仍为 0」，
	// 从而把健康的节点误判成不通。失败/短写再把多算的部分扣回去。
	atomic.AddInt64(c.up, int64(len(b)))
	n, err := c.Conn.Write(b)
	if n < len(b) {
		atomic.AddInt64(c.up, -int64(len(b)-n))
	}
	return n, err
}

// probeChainHTTP 通过代理出站访问 probeURL，返回状态码、上下行字节与延迟。
func probeChainHTTP(ctx context.Context, dial dialFunc, probeURL string) (status int, up, down int64, latencyMs int, err error) {
	var u, d int64
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			raw, derr := dialConn(ctx, dial, network, addr)
			if derr != nil {
				return nil, derr
			}
			return &countingConn{Conn: raw, up: &u, down: &d}, nil
		},
		DisableKeepAlives: true,
		// 探测只看链路是否可用，不做 h2 协商（h2 会让计时/字节统计混入更多因素）。
		ForceAttemptHTTP2: false,
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12},
	}
	defer transport.CloseIdleConnections()

	reqCtx, cancel := context.WithTimeout(ctx, chainProbeTimeout)
	defer cancel()
	start := time.Now()
	req, rerr := http.NewRequestWithContext(reqCtx, http.MethodGet, probeURL, nil)
	if rerr != nil {
		return 0, 0, 0, 0, rerr
	}
	req.Header.Set("User-Agent", "CoreFusion-ChainProbe/1.0")
	req.Header.Set("Cache-Control", "no-store")

	client := &http.Client{Transport: transport, Timeout: chainProbeTimeout}
	resp, derr := client.Do(req)
	latencyMs = elapsedMs(start)
	if derr != nil {
		return 0, atomic.LoadInt64(&u), atomic.LoadInt64(&d), latencyMs, derr
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, chainProbeMaxBody))
	return resp.StatusCode, atomic.LoadInt64(&u), atomic.LoadInt64(&d), latencyMs, nil
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func truncateErr(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 900 {
		return s[:900] + "…"
	}
	return s
}
