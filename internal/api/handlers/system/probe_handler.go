package system

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"corefusion/internal/api/response"
	"corefusion/internal/model/entity"
	"corefusion/internal/service/binaries"
	"corefusion/internal/service/healthprobe"
	"corefusion/internal/store/repository"

	"github.com/gin-gonic/gin"
)

// ProbeHandler 暴露「优选域名 / 优选 IP」的探活入口与探活配置。
type ProbeHandler struct {
	prober      *healthprobe.Prober
	nodeRepo    *repository.ExternalNodeRepository
	nodeSrvRepo *repository.NodeServerRepository
	inboundRepo *repository.InboundConfigRepository
	resolver    *binaries.Resolver
	// installer 用于面板本机缺失拨号核心时，直接从页面上选版本安装（不用 SSH 登录手动放二进制）。
	installer *binaries.Installer
}

// NewProbeHandler 创建探活处理器。
func NewProbeHandler(
	prober *healthprobe.Prober,
	nodeRepo *repository.ExternalNodeRepository,
	nodeSrvRepo *repository.NodeServerRepository,
	inboundRepo *repository.InboundConfigRepository,
	resolver *binaries.Resolver,
	installer *binaries.Installer,
) *ProbeHandler {
	return &ProbeHandler{
		prober:      prober,
		nodeRepo:    nodeRepo,
		nodeSrvRepo: nodeSrvRepo,
		inboundRepo: inboundRepo,
		resolver:    resolver,
		installer:   installer,
	}
}

// Check 立即探测全部优选域名/IP，返回本次探测汇总。
func (h *ProbeHandler) Check(c *gin.Context) {
	res, err := h.prober.CheckAll(c.Request.Context())
	if err != nil {
		response.InternalError(c, "PROBE_FAILED", err.Error())
		return
	}
	response.OK(c, res)
}

// cdnProbeInboundValues 返回「可作为 CDN 探针入站」的 inb:<id> 集合：
// 启用 + 接入域名开启且类型为 cf_domain / cf_ip（与链路探针入站同一口径）。
func (h *ProbeHandler) cdnProbeInboundValues(ctx context.Context) map[string]bool {
	out := map[string]bool{}
	opts, err := h.prober.ChainProbeInboundOptions(ctx)
	if err != nil {
		return out
	}
	for _, o := range opts {
		out[o.Value] = true
	}
	return out
}

// externalNodeIsCDN 判断外部节点是否配置了 CDN 接入（优选 IP/域名）。
// 只看 access_domain 里的 cfDomain/cfIP：custom/direct 不是 CDN 入口。
func externalNodeIsCDN(n *entity.ExternalNode) bool {
	ad := n.LoadAccessDomain()
	if len(ad) == 0 {
		return false
	}
	return adFieldString(ad, "cfDomain") != "" || adFieldString(ad, "cfIP") != ""
}

func adFieldString(ad map[string]interface{}, key string) string {
	if v, ok := ad[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// GetConfig 返回探活配置 + 可选入口节点 + 当前已安装的拨号核心。
func (h *ProbeHandler) GetConfig(c *gin.Context) {
	ctx := c.Request.Context()
	cfg := h.prober.ReadConfig(ctx)

	type nodeItem struct {
		// Value 是统一入口标识："ext:<id>" / "inb:<id>"（前端直接用）。
		Value    string `json:"value"`
		ID       int64  `json:"id"`
		Name     string `json:"name"`
		Protocol string `json:"protocol"`
		Host     string `json:"host"`
		// Kind: external（外部导入节点）/ inbound（节点侧入站）。
		Kind string `json:"kind"`
	}
	nodes := make([]nodeItem, 0, 32)
	// 优选探测的入口只列「配置了 CDN 接入（优选 IP/域名）」的项：
	//   - 外部节点：access_domain 里选了 cfDomain/cfIP；
	//   - 节点侧入站：_accessDomain.type 为 cf_domain/cf_ip（链路探针入站同一集合）。
	// 直连域名/自定义域名的入口当模板会把 server 换成 CDN 条目，语义不对。
	cdnInbounds := h.cdnProbeInboundValues(ctx)
	if h.nodeRepo != nil {
		if list, err := h.nodeRepo.ListAll(ctx); err == nil {
			for i := range list {
				n := &list[i]
				if !n.Enabled || n.ProxyJSON == "" {
					continue
				}
				if !externalNodeIsCDN(n) {
					continue
				}
				nodes = append(nodes, nodeItem{Value: fmt.Sprintf("ext:%d", n.ID), ID: n.ID,
					Name: n.Name, Protocol: n.Protocol, Host: n.Host, Kind: "external"})
			}
		}
	}
	// 节点侧入口（自建入站）：如 VLESSGRPC。
	// ⚠ 这里不能用链路出口那套 unix/协议过滤：模板模式会把 server 换成 CDN 条目，
	// 入站 listen 是否 unix、是否在 nginx 后面都不影响客户端出站渲染。
	if h.inboundRepo != nil {
		servers := map[int64]*entity.NodeServer{}
		if h.nodeSrvRepo != nil {
			if list, err := h.nodeSrvRepo.ListAll(ctx); err == nil {
				for i := range list {
					servers[list[i].ID] = &list[i]
				}
			}
		}
		for _, core := range []string{"xray", "singbox"} {
			cfgs, err := h.inboundRepo.ListAll(ctx, core)
			if err != nil {
				continue
			}
			for i := range cfgs {
				cfg := &cfgs[i]
				if !cfg.Enabled {
					continue
				}
				value := fmt.Sprintf("inb:%d", cfg.ID)
				if !cdnInbounds[value] {
					continue
				}
				if cfg.Protocol == "socks" || cfg.Protocol == "http" || cfg.Protocol == "mixed" {
					continue
				}
				nodeName := fmt.Sprintf("节点 %d", cfg.ServerID)
				if n := servers[cfg.ServerID]; n != nil && n.Name != "" {
					nodeName = n.Name
				}
				name := strings.TrimSpace(cfg.Remark)
				if name == "" {
					name = cfg.Tag
				}
				if name == "" {
					name = fmt.Sprintf("%s:%d", cfg.Protocol, cfg.Port)
				}
				nodes = append(nodes, nodeItem{Value: value, ID: cfg.ID,
					Name: name, Protocol: cfg.Protocol, Host: nodeName, Kind: "inbound"})
			}
		}
	}

	// 拨号核心：只列出真正装了的（mihomo 需要用户自行安装到 binaries/active/mihomo）。
	// 顺序与自动选择一致（xray → sing-box → mihomo），避免下拉顺序和实际生效值对不上。
	dialers := make([]string, 0, 3)
	versions := make(map[string]string, 3)
	if h.resolver != nil {
		for _, ct := range []string{"xray", "singbox", "mihomo"} {
			if _, _, err := h.resolver.Resolve(ct); err == nil {
				dialers = append(dialers, ct)
				if v := h.resolver.Version(ct); v != "" {
					versions[ct] = v
				}
			}
		}
	}

	// 配置字段平铺，附带入可选入口节点与已安装核心；前端不再单独拼。
	// dialer_versions 用来在页面上显示当前版本，并提供「重装 / 换版本」入口。
	response.OK(c, struct {
		healthprobe.Config
		Nodes          []nodeItem        `json:"nodes"`
		Dialers        []string          `json:"dialers"`
		DialerVersions map[string]string `json:"dialer_versions"`
	}{Config: cfg, Nodes: nodes, Dialers: dialers, DialerVersions: versions})
}

// SaveConfig 更新探活配置（启用开关 / 间隔 / 延迟上限 / 入口节点 / 拨号核心）。
func (h *ProbeHandler) SaveConfig(c *gin.Context) {
	var req struct {
		Enabled         *bool `json:"enabled"`
		IntervalMinutes *int  `json:"interval_minutes"`
		MaxLatencyMs    *int  `json:"max_latency_ms"`
		// Entry 统一入口标识："ext:<id>" / "inb:<id>" / ""（本机）。
		Entry string `json:"entry"`
		// EntryNodeID 兼容旧前端：只传该字段时按外部节点处理。
		EntryNodeID *int64  `json:"entry_node_id"`
		Dialer      *string `json:"dialer"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if req.IntervalMinutes != nil && (*req.IntervalMinutes < 1 || *req.IntervalMinutes > 1440) {
		response.BadRequest(c, "INVALID_REQUEST", "interval_minutes must be between 1 and 1440")
		return
	}
	if req.MaxLatencyMs != nil && *req.MaxLatencyMs < 0 {
		response.BadRequest(c, "INVALID_REQUEST", "max_latency_ms must not be negative")
		return
	}
	// 入口校验：ext:<id> 查外部节点，inb:<id> 查自建入站；空 = 本机。
	entry := strings.TrimSpace(req.Entry)
	if entry != "" {
		switch {
		case strings.HasPrefix(entry, "ext:"):
			if h.nodeRepo == nil {
				response.BadRequest(c, "INVALID_REQUEST", "external node repository unavailable")
				return
			}
			id, err := strconv.ParseInt(strings.TrimPrefix(entry, "ext:"), 10, 64)
			if err != nil || id <= 0 {
				response.BadRequest(c, "INVALID_REQUEST", "invalid entry: "+entry)
				return
			}
			if _, err := h.nodeRepo.GetByID(c.Request.Context(), id); err != nil {
				response.BadRequest(c, "INVALID_REQUEST", "entry node not found")
				return
			}
		case strings.HasPrefix(entry, "inb:"):
			if h.inboundRepo == nil {
				response.BadRequest(c, "INVALID_REQUEST", "inbound repository unavailable")
				return
			}
			id, err := strconv.ParseInt(strings.TrimPrefix(entry, "inb:"), 10, 64)
			if err != nil || id <= 0 {
				response.BadRequest(c, "INVALID_REQUEST", "invalid entry: "+entry)
				return
			}
			if _, err := h.inboundRepo.GetByID(c.Request.Context(), id); err != nil {
				response.BadRequest(c, "INVALID_REQUEST", "entry inbound not found")
				return
			}
		default:
			response.BadRequest(c, "INVALID_REQUEST", "invalid entry: "+entry)
			return
		}
	}
	// 开启探测必须选入口：本机直连只剩 TCP 判活，会把坏条目判成可用。
	enabledNow := h.prober.ReadConfig(c.Request.Context()).Enabled
	if req.Enabled != nil {
		enabledNow = *req.Enabled
	}
	if enabledNow && entry == "" {
		response.BadRequest(c, "INVALID_REQUEST", "probe enabled requires an entry node")
		return
	}
	if err := h.prober.SaveConfig(c.Request.Context(), req.Enabled, req.IntervalMinutes, req.MaxLatencyMs, &entry, req.EntryNodeID, req.Dialer); err != nil {
		response.InternalError(c, "SAVE_FAILED", err.Error())
		return
	}
	response.OK(c, h.prober.ReadConfig(c.Request.Context()))
}

// probeDialerCores 是探活/链路探测可用的拨号核心。
var probeDialerCores = map[string]bool{
	"xray":    true,
	"singbox": true,
	"mihomo":  true,
}

// ListDialerReleases 返回面板本机可安装的拨号核心版本。
// 前端在「拨号工具」里选中未安装的核心时弹出该列表，选版本后直接安装。
func (h *ProbeHandler) ListDialerReleases(c *gin.Context) {
	coreType := strings.ToLower(strings.TrimSpace(c.Query("core_type")))
	if !probeDialerCores[coreType] {
		response.BadRequest(c, "INVALID_REQUEST", "unsupported core_type")
		return
	}
	if h.installer == nil {
		response.InternalError(c, "INSTALLER_UNAVAILABLE", "binary installer not configured")
		return
	}
	releases, err := h.installer.ListReleases(c.Request.Context(), coreType)
	if err != nil {
		response.InternalError(c, "FETCH_FAILED", err.Error())
		return
	}
	if releases == nil {
		releases = []binaries.ReleaseInfo{}
	}
	response.OK(c, releases)
}

// InstallDialer 在面板本机安装拨号核心（xray / sing-box / mihomo）。
// 安装完成后 Resolve 立即能找到二进制，页面刷新即可选中。
func (h *ProbeHandler) InstallDialer(c *gin.Context) {
	var req struct {
		CoreType string `json:"core_type" binding:"required"`
		Version  string `json:"version" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	coreType := strings.ToLower(strings.TrimSpace(req.CoreType))
	if !probeDialerCores[coreType] {
		response.BadRequest(c, "INVALID_REQUEST", "unsupported core_type")
		return
	}
	if h.installer == nil {
		response.InternalError(c, "INSTALLER_UNAVAILABLE", "binary installer not configured")
		return
	}
	info, err := h.installer.Install(c.Request.Context(), coreType, strings.TrimSpace(req.Version), "", "")
	if err != nil {
		response.InternalError(c, "INSTALL_FAILED", err.Error())
		return
	}
	response.OK(c, info)
}

// chainExitOption 是链路探测可选的出口（外部节点 / 自建节点入站）。
type chainExitOption struct {
	Value    string `json:"value"`
	Label    string `json:"label"`
	Kind     string `json:"kind"` // external / inbound
	Protocol string `json:"protocol"`
	Host     string `json:"host"`
	Region   string `json:"region"`
}

// chainExitOptions 列出可作为「链路出口」的候选：
//   - 导入的外部节点（常是机场/回国线路）；
//   - 自建节点上的入站（国内 VPS 的入口配置，参数可直接复用）。
func (h *ProbeHandler) chainExitOptions(ctx context.Context) []chainExitOption {
	out := make([]chainExitOption, 0, 32)
	// 链路出口是「从这条线路出去」的跳板，不能选已经配了 CDN 接入的节点/入站：
	// 那些是探测目标（优选 IP/域名）的入口，当出口会绕回 CDN。
	cdnInbounds := h.cdnProbeInboundValues(ctx)
	if h.nodeRepo != nil {
		if list, err := h.nodeRepo.ListAll(ctx); err == nil {
			for i := range list {
				n := &list[i]
				if !n.Enabled || n.ProxyJSON == "" {
					continue
				}
				if externalNodeIsCDN(n) {
					continue
				}
				out = append(out, chainExitOption{
					Value:    fmt.Sprintf("ext:%d", n.ID),
					Label:    fmt.Sprintf("%s (%s)", n.Name, n.Protocol),
					Kind:     "external",
					Protocol: n.Protocol,
					Host:     n.Host,
				})
			}
		}
	}
	if h.inboundRepo != nil {
		servers := map[int64]*entity.NodeServer{}
		if h.nodeSrvRepo != nil {
			if list, err := h.nodeSrvRepo.ListAll(ctx); err == nil {
				for i := range list {
					n := list[i]
					servers[n.ID] = &n
				}
			}
		}
		for _, core := range []string{"xray", "singbox"} {
			cfgs, err := h.inboundRepo.ListAll(ctx, core)
			if err != nil {
				continue
			}
			for i := range cfgs {
				cfg := &cfgs[i]
				if !cfg.Enabled {
					continue
				}
				if cdnInbounds[fmt.Sprintf("inb:%d", cfg.ID)] {
					continue
				}
				// socks/http 入站没有客户端出站可用的凭据结构，选它必然探测失败，
				// 直接不列出来（这类入站也不是"国内线路出口"的典型用法）。
				if cfg.Protocol == "socks" || cfg.Protocol == "http" || cfg.Protocol == "mixed" {
					continue
				}
				// unix / 抽象 socket 入站只能经 SNI 前置代理访问，客户端直连不到，
				// 当出口必然失败（生产 nginx 前置架构下的入站几乎都是这种）。
				listen := strings.TrimSpace(cfg.Listen)
				if strings.HasPrefix(listen, "/") || strings.HasPrefix(listen, "@") {
					continue
				}
				nodeName := fmt.Sprintf("节点 %d", cfg.ServerID)
				region := ""
				if node := servers[cfg.ServerID]; node != nil {
					if node.Name != "" {
						nodeName = node.Name
					}
					region = node.Region
				}
				label := fmt.Sprintf("%s / %s:%d (%s)", nodeName, cfg.Protocol, cfg.Port, core)
				out = append(out, chainExitOption{
					Value:    fmt.Sprintf("inb:%d", cfg.ID),
					Label:    label,
					Kind:     "inbound",
					Protocol: cfg.Protocol,
					Host:     nodeName,
					Region:   region,
				})
			}
		}
	}
	return out
}

// GetChainConfig 返回链路探测配置、上次结果、待探测目标与出口候选。
func (h *ProbeHandler) GetChainConfig(c *gin.Context) {
	ctx := c.Request.Context()
	cfg := h.prober.ReadChainConfig(ctx)
	// 域名 / IP 分开返回结果：页面各自展示，数据不会相互串。
	ipResults, ipUpdatedAt := h.prober.ChainResultsForKind(ctx, "ip")
	domainResults, domainUpdatedAt := h.prober.ChainResultsForKind(ctx, "domain")
	targets, err := h.prober.ChainTargets(ctx)
	if err != nil {
		// 目标解析失败不应让整个配置页打不开：返回空列表，错误在探测时暴露。
		targets = nil
	}
	ipTargetCount, domainTargetCount := 0, 0
	for _, t := range targets {
		switch t.Kind {
		case "cf_ip":
			ipTargetCount++
		case "cf_domain":
			domainTargetCount++
		}
	}
	inbounds, err := h.prober.ChainProbeInboundOptions(ctx)
	if err != nil {
		inbounds = nil
	}
	// 链路探测不单独配间隔/拨号核心：两者都跟随「探测设置」。
	// 这里把当前生效值一起返回，页面直接把依赖关系显示出来。
	probeCfg := h.prober.ReadConfig(ctx)
	response.OK(c, gin.H{
		"config":                    cfg,
		"ip_results":                ipResults,
		"ip_results_updated_at":     ipUpdatedAt,
		"ip_probe":                  h.prober.ChainLastProbeForKind(ctx, "ip"),
		"domain_results":            domainResults,
		"domain_results_updated_at": domainUpdatedAt,
		"domain_probe":              h.prober.ChainLastProbeForKind(ctx, "domain"),
		"targets":                   targets,
		"target_count":              len(targets),
		"ip_target_count":           ipTargetCount,
		"domain_target_count":       domainTargetCount,
		"exits":                     h.chainExitOptions(ctx),
		"inbounds":                  inbounds,
		"interval_minutes":          probeCfg.IntervalMinutes,
		"dialer":                    probeCfg.Dialer,
		"effective_dialer":          h.prober.EffectiveDialer(probeCfg.Dialer),
	})
}

// SaveChainConfig 保存链路探测配置。
func (h *ProbeHandler) SaveChainConfig(c *gin.Context) {
	var req healthprobe.ChainConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	// 探针入站：允许留空（= 未选择），但一旦填了就必须是当前可用的 CDN 入站。
	if req.Inbound != "" {
		options, err := h.prober.ChainProbeInboundOptions(c.Request.Context())
		if err != nil {
			response.InternalError(c, "INVALID_REQUEST", err.Error())
			return
		}
		ok := false
		for _, o := range options {
			if o.Value == req.Inbound {
				ok = true
				break
			}
		}
		if !ok {
			response.BadRequest(c, "INVALID_REQUEST", "probe inbound not found or no CDN domain is enabled")
			return
		}
	}
	if err := h.prober.SaveChainConfig(c.Request.Context(), req); err != nil {
		response.InternalError(c, "SAVE_FAILED", err.Error())
		return
	}
	response.OK(c, h.prober.ReadChainConfig(c.Request.Context()))
}

// CheckChain 立即执行一轮链路探测（忽略定时开关）。
// kind=ip / domain 时只测该类；不传则两类都测（兼容旧前端）。
func (h *ProbeHandler) CheckChain(c *gin.Context) {
	kind := strings.ToLower(strings.TrimSpace(c.Query("kind")))
	if kind == "" {
		var req struct {
			Kind string `json:"kind"`
		}
		// 允许空 body：旧前端 POST {} 时不报错。
		if err := c.ShouldBindJSON(&req); err == nil {
			kind = strings.ToLower(strings.TrimSpace(req.Kind))
		}
	}
	var (
		res healthprobe.ChainSummary
		err error
	)
	if kind == "" {
		res, err = h.prober.CheckChain(c.Request.Context(), true)
	} else {
		res, err = h.prober.CheckChainKind(c.Request.Context(), kind, true)
	}
	if err != nil {
		response.InternalError(c, "CHAIN_PROBE_FAILED", err.Error())
		return
	}
	response.OK(c, res)
}
