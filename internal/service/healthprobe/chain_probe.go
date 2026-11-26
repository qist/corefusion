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
//     再把它挂在出口节点后面（xray dialerProxy / sing-box detour），从出口拨号：
//     出口 → CDN 域名/IP → 入站 → 外部 URL，要求 HTTP 200 且上下行都有字节。
//
// 为什么必须这样：CDN 节点在国内经常被封锁/限速，仅测 TCP 443 通不代表
// 能用（TLS 握手被 RST、回源域名不通、CDN 端口回源失败都会表现为"能连不能上网"）。
// 更关键的是：只有真正连到探针入站，才算验证了「CDN → 入站」这一段 ——
// 只从出口直连外部 URL 的话，出口有日志、入站没日志，结论完全是假的。
const (
	SettingChainEnabled     = "cfip.chain_probe.enabled"
	SettingChainExit        = "cfip.chain_probe.exit"
	SettingChainInterface   = "cfip.chain_probe.interface"
	SettingChainAgent       = "cfip.chain_probe.agent_id"
	SettingChainInherit     = "cfip.chain_probe.inherit_probe"
	SettingChainInterval    = "cfip.chain_probe.interval_minutes"
	SettingChainDialer      = "cfip.chain_probe.dialer"
	SettingChainInbound     = "cfip.chain_probe.inbound"
	SettingChainConcurrency = "cfip.chain_probe.concurrency"
	SettingChainMaxTargets  = "cfip.chain_probe.max_targets"
	SettingChainProbeURL    = "cfip.chain_probe.probe_url"
	SettingChainResults     = "cfip.chain_probe.results"
	SettingChainCursor      = "cfip.chain_probe.cursor"

	// 域名 / IP 分开控制：各有独立开关、单轮名额、结果与游标。
	// 两者共用出口 / 探针入站 / 并发 / 探测 URL，但不共用名额和结果，
	// 这样 IP 再多也不会挤掉域名，页面数据也不会相互覆盖。
	SettingChainIPEnabled        = "cfip.chain_probe.ip_enabled"
	SettingChainDomainEnabled    = "cfip.chain_probe.domain_enabled"
	SettingChainIPMaxTargets     = "cfip.chain_probe.ip_max_targets"
	SettingChainDomainMaxTargets = "cfip.chain_probe.domain_max_targets"
	SettingChainIPResults        = "cfip.chain_probe.ip_results"
	SettingChainDomainResults    = "cfip.chain_probe.domain_results"

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
	// Interface 是发起探测时绑定的本机网卡（如 eth0、ppp0）。
	// 留空 = 使用系统默认路由；xray / sing-box / mihomo 都支持。
	Interface string `json:"interface"`
	// AgentID 指定在哪台节点上执行探测（0 = 面板本机）。
	// 选定后临时核心跑在该节点上，网卡也取该节点的。
	AgentID int64 `json:"agent_id"`
	// InheritProbe=true（默认）时沿用上方「探测设置」的间隔与拨号核心；
	// false 时使用本卡片自己的 IntervalMinutes / Dialer。
	InheritProbe    bool   `json:"inherit_probe"`
	IntervalMinutes int    `json:"interval_minutes"`
	Dialer          string `json:"dialer"`
	// Inbound 是「探针入站」标识（自建入站 "inb:<id>" / 外部导入节点 "ext:<id>"）：
	// 一轮探测只用它完成协议握手，把全部启用的 CDN 域名/IP 逐个测一遍。
	// 必须显式选择，不再随机挑：随机会导致同一批条目每轮用不同入口，
	// 结果无法横向比较。
	Inbound string `json:"inbound"`
	// Concurrency 是同时探测的目标数（1-8）。临时核心只起一个，
	// 并发太高只会让加密/握手抢占 CPU，默认 2。
	Concurrency int `json:"concurrency"`
	// MaxTargets 是旧版的单轮名额（1-500）。新版本拆成 IPMaxTargets /
	// DomainMaxTargets，这里只作为老库升级时的默认值保留。
	MaxTargets int `json:"max_targets"`
	// DomainEnabled / IPEnabled 分别控制两类目标是否参与自动巡检。
	// 手动检测不受开关限制（可以单独点某一类的「检测」）。
	DomainEnabled bool `json:"domain_enabled"`
	IPEnabled     bool `json:"ip_enabled"`
	// DomainMaxTargets / IPMaxTargets 是两类各自的单轮名额（1-500）。
	// 分开后 IP 再多也不会挤掉域名，两类结果也不会互相覆盖。
	DomainMaxTargets int `json:"domain_max_targets"`
	IPMaxTargets     int `json:"ip_max_targets"`
	// ProbeURL 是穿透代理访问的 URL，默认 CF trace 页（返回纯文本且无缓存）。
	ProbeURL string `json:"probe_url"`
	// KindsConfigured 只用于保存请求：true 表示前端已显式提供分类型开关/名额。
	// 旧前端/直接调用不传时，按总开关与旧总名额回退，避免把两类名额误写成 1。
	KindsConfigured bool `json:"kinds_configured"`
}

// ChainRemoteItem 是 Agent 端返回的单目标探测结果。
type ChainRemoteItem struct {
	Success    bool
	Error      string
	HTTPStatus int
	UpBytes    int64
	DownBytes  int64
	LatencyMs  int
}

// ChainRemoteRawTarget 是「只做 TCP/TLS」的探测目标（优选 IP/域名）。
type ChainRemoteRawTarget struct {
	Host string
	Port int
	SNI  string
	// Mode: tcp（只建连）/ tls（在 TCON 上完成 TLS 握手）。
	Mode string
}

// ChainRemoteRequest 描述一轮在 Agent 上执行的探测。
type ChainRemoteRequest struct {
	ServerID    int64
	Dialer      string
	ConfigJSON  string
	ProbeURL    string
	TimeoutMs   int
	Concurrency int
	// RawTargets 非空时不做 HTTP，只按 Mode 做 TCP/TLS 探测。
	RawTargets []ChainRemoteRawTarget
}

// ChainRemoteRunner 在指定节点执行一轮探测（configJSON 内端口为 0，由 Agent 分配）。
type ChainRemoteRunner func(ctx context.Context, req ChainRemoteRequest) ([]ChainRemoteItem, error)

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
	// Healthy 是该条目上次优选/链路探测的结论：名额分配时优先测健康条目，
	// 但每类仍保留少量名额给不健康条目做恢复探测。
	Healthy bool `json:"healthy"`
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

// ChainProbeInbound 是「探针入站」候选（配了 CDN 接入域名的启用入口：
// 自建节点入站或外部导入节点）。
type ChainProbeInbound struct {
	Value        string `json:"value"`         // "inb:<id>" / "ext:<id>"
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
	// 入口（自建入站 "inb:<id>" / 外部导入节点 "ext:<id>"）渲染成客户端出站参数。
	ChainProbeTargets(ctx context.Context, probeSpec string) ([]ChainTarget, error)
	// ChainProbeInbounds 返回可作为探针入站的候选。
	ChainProbeInbounds(ctx context.Context) ([]ChainProbeInbound, error)
	// ChainProbeExitProxy 按 "ext:<id>" / "inb:<id>" 解析出口节点的出站参数。
	ChainProbeExitProxy(ctx context.Context, spec string) (map[string]interface{}, string, error)
	// ChainProbeEntryTemplate 把入口渲染成「客户端配置模板」：
	// 对 CDN 入站会应用接入域名的 TLS/SNI（即使入站本身是 nginx 前置的明文），
	// 便于优选探测只替换 server/port 去真实拨号。非 CDN 入口退回普通出站参数。
	ChainProbeEntryTemplate(ctx context.Context, spec string) (map[string]interface{}, string, error)
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

// ResolveEntryTemplate 把探测入口渲染成客户端配置模板（优选探测用）。
func (p *Prober) ResolveEntryTemplate(ctx context.Context, spec string) (map[string]interface{}, string, error) {
	if p.chainProv == nil {
		return nil, "", fmt.Errorf("入口 %s 无法解析：探测目标解析器未初始化", spec)
	}
	return p.chainProv.ChainProbeEntryTemplate(ctx, spec)
}

// ReadChainConfig 读取链路探测配置（缺失项回落默认值）。
func (p *Prober) ReadChainConfig(ctx context.Context) ChainConfig {
	cfg := ChainConfig{
		Enabled:      false,
		InheritProbe: true,
		Concurrency:  DefaultChainConcurrency,
		MaxTargets:   DefaultChainMaxTargets,
		ProbeURL:     DefaultChainProbeURL,
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
	if v, err := p.setting.GetByKey(ctx, SettingChainInterface); err == nil {
		cfg.Interface = strings.TrimSpace(v)
	}
	if v, err := p.setting.GetByKey(ctx, SettingChainInherit); err == nil {
		cfg.InheritProbe = parseBoolSetting(v, true)
	}
	if v, err := p.setting.GetByKey(ctx, SettingChainInterval); err == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 {
			cfg.IntervalMinutes = clampInt(n, 1, 1440)
		}
	}
	if v, err := p.setting.GetByKey(ctx, SettingChainDialer); err == nil {
		cfg.Dialer = strings.TrimSpace(v)
	}
	if v, err := p.setting.GetByKey(ctx, SettingChainAgent); err == nil {
		if n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil && n > 0 {
			cfg.AgentID = n
		}
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
	// 域名 / IP 分开控制。老库没有这两个键时沿用原总开关与总名额，
	// 升级后行为不变；一旦保存过新配置就以各自的值为准。
	cfg.DomainEnabled = cfg.Enabled
	cfg.IPEnabled = cfg.Enabled
	if v, err := p.setting.GetByKey(ctx, SettingChainDomainEnabled); err == nil {
		cfg.DomainEnabled = parseBoolSetting(v, cfg.Enabled)
	}
	if v, err := p.setting.GetByKey(ctx, SettingChainIPEnabled); err == nil {
		cfg.IPEnabled = parseBoolSetting(v, cfg.Enabled)
	}
	cfg.DomainMaxTargets = cfg.MaxTargets
	cfg.IPMaxTargets = cfg.MaxTargets
	if v, err := p.setting.GetByKey(ctx, SettingChainDomainMaxTargets); err == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			cfg.DomainMaxTargets = clampInt(n, 1, 500)
		}
	}
	if v, err := p.setting.GetByKey(ctx, SettingChainIPMaxTargets); err == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			cfg.IPMaxTargets = clampInt(n, 1, 500)
		}
	}
	return cfg
}

// enabledChainKinds 返回自动巡检要跑的类型（域名 / IP 各自独立开关）。
func (c ChainConfig) enabledChainKinds() []string {
	out := make([]string, 0, 2)
	if c.DomainEnabled {
		out = append(out, "domain")
	}
	if c.IPEnabled {
		out = append(out, "ip")
	}
	return out
}

// maxTargetsForKind 返回某一类的单轮名额；新字段缺失时回落到旧总名额。
func (c ChainConfig) maxTargetsForKind(kind string) int {
	switch kind {
	case "domain":
		if c.DomainMaxTargets > 0 {
			return c.DomainMaxTargets
		}
	case "ip":
		if c.IPMaxTargets > 0 {
			return c.IPMaxTargets
		}
	}
	if c.MaxTargets > 0 {
		return c.MaxTargets
	}
	return DefaultChainMaxTargets
}

// normalizeChainKind 归一化前端传入的类型（domain / ip）。
func normalizeChainKind(kind string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "domain", "cf_domain":
		return "domain", true
	case "ip", "cf_ip":
		return "ip", true
	}
	return "", false
}

// chainTargetKind 把探测类型映射为目标 / 结果里的 Kind。
func chainTargetKind(kind string) string {
	if kind == "ip" {
		return "cf_ip"
	}
	return "cf_domain"
}

// chainResultsSettingKey 返回某一类结果的存储键。
func chainResultsSettingKey(kind string) string {
	if kind == "ip" {
		return SettingChainIPResults
	}
	return SettingChainDomainResults
}

// filterChainTargetsByKind 只保留某一类目标。
func filterChainTargetsByKind(targets []ChainTarget, kind string) []ChainTarget {
	want := chainTargetKind(kind)
	out := make([]ChainTarget, 0, len(targets))
	for _, t := range targets {
		if t.Kind == want {
			out = append(out, t)
		}
	}
	return out
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
	if err := set(SettingChainInterface, strings.TrimSpace(cfg.Interface)); err != nil {
		return err
	}
	if cfg.AgentID < 0 {
		cfg.AgentID = 0
	}
	if err := set(SettingChainAgent, strconv.FormatInt(cfg.AgentID, 10)); err != nil {
		return err
	}
	if err := set(SettingChainInherit, strconv.FormatBool(cfg.InheritProbe)); err != nil {
		return err
	}
	if err := set(SettingChainInterval, strconv.Itoa(clampInt(cfg.IntervalMinutes, 1, 1440))); err != nil {
		return err
	}
	if err := set(SettingChainDialer, strings.TrimSpace(cfg.Dialer)); err != nil {
		return err
	}
	if err := set(SettingChainInbound, strings.TrimSpace(cfg.Inbound)); err != nil {
		return err
	}
	if err := set(SettingChainConcurrency, strconv.Itoa(clampInt(cfg.Concurrency, 1, 8))); err != nil {
		return err
	}
	// 旧前端 / 直接调用不带分类型字段：跟随总开关与旧总名额，
	// 不能因为 bool 零值把两类都关掉、把名额写成 1。
	if !cfg.KindsConfigured {
		cfg.DomainEnabled = cfg.Enabled
		cfg.IPEnabled = cfg.Enabled
		if cfg.DomainMaxTargets <= 0 {
			cfg.DomainMaxTargets = cfg.MaxTargets
		}
		if cfg.IPMaxTargets <= 0 {
			cfg.IPMaxTargets = cfg.MaxTargets
		}
	}
	if cfg.DomainMaxTargets <= 0 {
		cfg.DomainMaxTargets = DefaultChainMaxTargets
	}
	if cfg.IPMaxTargets <= 0 {
		cfg.IPMaxTargets = DefaultChainMaxTargets
	}
	// 域名 / IP 各自的名额与开关（分开存储，互不影响）。
	if err := set(SettingChainDomainEnabled, strconv.FormatBool(cfg.DomainEnabled)); err != nil {
		return err
	}
	if err := set(SettingChainIPEnabled, strconv.FormatBool(cfg.IPEnabled)); err != nil {
		return err
	}
	if err := set(SettingChainDomainMaxTargets, strconv.Itoa(clampInt(cfg.DomainMaxTargets, 1, 500))); err != nil {
		return err
	}
	if err := set(SettingChainIPMaxTargets, strconv.Itoa(clampInt(cfg.IPMaxTargets, 1, 500))); err != nil {
		return err
	}
	// 旧总名额仍写一份（取两类较大值），回滚到旧版本时不会突然变成默认值。
	legacyMax := cfg.MaxTargets
	if legacyMax <= 0 {
		legacyMax = cfg.IPMaxTargets
		if cfg.DomainMaxTargets > legacyMax {
			legacyMax = cfg.DomainMaxTargets
		}
	}
	if err := set(SettingChainMaxTargets, strconv.Itoa(clampInt(legacyMax, 1, 500))); err != nil {
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

// ChainResultsForKind 返回某一类（ip / domain）上次的探测结果。
func (p *Prober) ChainResultsForKind(ctx context.Context, kind string) ([]ChainResult, int64) {
	kind, ok := normalizeChainKind(kind)
	if !ok || p.setting == nil {
		return nil, 0
	}
	if raw, err := p.setting.GetByKey(ctx, chainResultsSettingKey(kind)); err == nil && strings.TrimSpace(raw) != "" {
		if results, updatedAt, ok := parseChainResultsPayload(raw); ok {
			return results, updatedAt
		}
	}
	// 兼容旧版本：以前只有一份 results，按结果里的 Kind 拆出本类。
	raw, err := p.setting.GetByKey(ctx, SettingChainResults)
	if err != nil || strings.TrimSpace(raw) == "" {
		return nil, 0
	}
	results, updatedAt, ok := parseChainResultsPayload(raw)
	if !ok {
		return nil, 0
	}
	want := chainTargetKind(kind)
	out := make([]ChainResult, 0, len(results))
	for _, r := range results {
		if r.Kind == want {
			out = append(out, r)
		}
	}
	return out, updatedAt
}

func parseChainResultsPayload(raw string) ([]ChainResult, int64, bool) {
	var payload struct {
		UpdatedAt int64         `json:"updated_at"`
		Results   []ChainResult `json:"results"`
	}
	if json.Unmarshal([]byte(raw), &payload) != nil {
		return nil, 0, false
	}
	return payload.Results, payload.UpdatedAt, true
}

// CheckChain 执行一轮链路探测（定时任务入口）：只跑已启用的类型。
// 域名与 IP 各自用自己的名额、游标和结果，互不影响。
func (p *Prober) CheckChain(ctx context.Context, force bool) (ChainSummary, error) {
	cfg := p.ReadChainConfig(ctx)
	sum := ChainSummary{}
	if !cfg.Enabled && !force {
		return sum, nil
	}
	kinds := cfg.enabledChainKinds()
	if len(kinds) == 0 {
		return sum, nil
	}

	// chainMu 只保护链路探测自身：优选 IP 探测用 p.mu，两者分开加锁，
	// 避免定时任务里 CheckAll 持锁后再进 CheckChain 造成死锁。
	p.chainMu.Lock()
	defer p.chainMu.Unlock()

	if err := p.validateChainRuntime(cfg); err != nil {
		return sum, err
	}
	var firstErr error
	for _, kind := range kinds {
		one, err := p.checkChainKind(ctx, cfg, kind)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		sum = mergeChainSummary(sum, one)
	}
	return sum, firstErr
}

// CheckChainKind 手动检测单一类型（kind = ip / domain）。
// force=true 表示忽略总开关（页面上的「检测」按钮即手动检测）。
func (p *Prober) CheckChainKind(ctx context.Context, kind string, force bool) (ChainSummary, error) {
	norm, ok := normalizeChainKind(kind)
	if !ok {
		return ChainSummary{}, fmt.Errorf("未知的链路探测类型 %q（可选 ip / domain）", kind)
	}
	cfg := p.ReadChainConfig(ctx)
	if !cfg.Enabled && !force {
		return ChainSummary{}, nil
	}
	p.chainMu.Lock()
	defer p.chainMu.Unlock()
	if err := p.validateChainRuntime(cfg); err != nil {
		return ChainSummary{}, err
	}
	return p.checkChainKind(ctx, cfg, norm)
}

func (p *Prober) validateChainRuntime(cfg ChainConfig) error {
	if p.chainProv == nil {
		return errors.New("链路探测目标解析器未初始化")
	}
	if cfg.Exit == "" {
		return errors.New("未选择出口节点：链路探测必须从一个真实出口发起，本机直连的结论没有参考意义")
	}
	if !ValidEgressInterface(cfg.Interface) {
		return fmt.Errorf("网卡名不合法：%q（只允许字母、数字、_ - . :，长度 1-32）", cfg.Interface)
	}
	if cfg.Inbound == "" {
		return errors.New("未选择探针入站：链路探测固定用这一个入站完成协议握手，请先选择")
	}
	return nil
}

// SetChainRemoteRunner 注入「在指定 Agent 上执行链路探测」的实现。
func (p *Prober) SetChainRemoteRunner(fn ChainRemoteRunner) {
	p.chainRemote = fn
}

// EffectiveInterval 返回链路探测实际使用的间隔（分钟）。
// 继承「探测设置」或未单独配置时用 probeInterval。
func (c ChainConfig) EffectiveInterval(probeInterval int) int {
	if c.InheritProbe || c.IntervalMinutes <= 0 {
		if probeInterval <= 0 {
			return DefaultIntervalMinutes
		}
		return probeInterval
	}
	return c.IntervalMinutes
}

// checkChainKind 只探测一类目标（调用方必须已持有 chainMu）。
func (p *Prober) checkChainKind(ctx context.Context, cfg ChainConfig, kind string) (ChainSummary, error) {
	sum := ChainSummary{}
	targets, err := p.chainProv.ChainProbeTargets(ctx, cfg.Inbound)
	if err != nil {
		return sum, err
	}
	targets = filterChainTargetsByKind(targets, kind)
	sum.Total = len(targets)
	if len(targets) == 0 {
		return sum, nil
	}

	// 一类一个名额上限 + 每类独立游标：IP 再多也不会占用域名的名额，
	// 两类结果分别落库，页面互不覆盖。
	picked, cursors := selectChainTargets(targets, p.readChainCursors(ctx), cfg.maxTargetsForKind(kind))
	sum.Skipped = len(targets) - len(picked)
	// 本轮探针入站（所有目标共用同一条入站做握手）。
	sum.Probe = fmt.Sprintf("%s (%s)", picked[0].Name, picked[0].Protocol)

	proxy, label, err := p.chainProv.ChainProbeExitProxy(ctx, cfg.Exit)
	if err != nil {
		return sum, err
	}
	// 拨号核心跟随「探测设置」：用户在探测设置里选了哪个就用哪个，
	// 为空时按自动顺序（xray → sing-box → mihomo）选，与优选探测完全同一口径。
	probeCfg := p.ReadConfig(ctx)
	dialerChoice := probeCfg.Dialer
	if !cfg.InheritProbe {
		dialerChoice = cfg.Dialer
	}
	dialer, err := p.pickDialer(dialerChoice)
	if err != nil {
		return sum, err
	}
	// 临时核心里每个目标一条链：出口 → 目标 CDN 域名/IP → 探针入站 → probe_url。
	// 不能只从出口直连 probe_url —— 那样探针入站根本不会收到连接。
	started := time.Now()
	results := make([]ChainResult, len(picked))
	if cfg.AgentID > 0 && p.chainRemote != nil {
		// 在指定节点上执行：配置仍由面板生成，端口写 0 交给 Agent 分配。
		valid := make([]int, 0, len(picked))
		for i := range picked {
			valid = append(valid, i)
		}
		conf, cerr := buildChainProbeConfig(dialer, proxy, picked, valid, make([]int, len(picked)), cfg.Interface)
		if cerr != nil {
			return sum, cerr
		}
		raw, merr := json.Marshal(conf)
		if merr != nil {
			return sum, merr
		}
		items, rerr := p.chainRemote(ctx, ChainRemoteRequest{
			ServerID:    cfg.AgentID,
			Dialer:      dialer,
			ConfigJSON:  string(raw),
			ProbeURL:    cfg.ProbeURL,
			TimeoutMs:   int(chainProbeTimeout / time.Millisecond),
			Concurrency: clampInt(cfg.Concurrency, 1, 8),
		})
		if rerr != nil {
			return sum, rerr
		}
		if len(items) != len(picked) {
			return sum, fmt.Errorf("Agent 返回结果数与目标数不一致（%d != %d）", len(items), len(picked))
		}
		sum.Entry = fmt.Sprintf("%s/%s@agent:%d", label, dialer, cfg.AgentID)
		now := time.Now().Unix()
		for i := range picked {
			target := picked[i]
			it := items[i]
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
				CheckedAt:      now,
				HTTPStatus:     it.HTTPStatus,
				LatencyMs:      it.LatencyMs,
				UpBytes:        it.UpBytes,
				DownBytes:      it.DownBytes,
			}
			if target.Proxy == nil {
				res.Error = "入站参数无法渲染为客户端出站"
			} else if !it.Success {
				res.Error = truncateErr(it.Error)
			} else {
				res.Healthy = true
			}
			results[i] = res
		}
	} else {
		dials, cleanup, usedDialer, err := p.startChainDialer(ctx, dialer, proxy, picked, cfg.Interface)
		if err != nil {
			return sum, err
		}
		defer cleanup()
		sum.Entry = fmt.Sprintf("%s/%s", label, usedDialer)

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
				status, up, down, latency, perr := probeChainHTTP(ctx, dials[idx], cfg.ProbeURL)
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
	}
	sum.DurationMs = int(time.Since(started) / time.Millisecond)

	// 结果写回条目健康状态：订阅渲染读的就是 Healthy，
	// 不通的条目因此不会再被分发给客户端。
	p.writeBackChainHealth(ctx, picked, results)

	// 游标推进：每类各自前进本轮取走的数量，下一轮从各类的下一个条目继续。
	p.saveChainCursors(ctx, cursors)

	for _, r := range results {
		sum.Checked++
		if r.Healthy {
			sum.Healthy++
		} else {
			sum.Unhealthy++
		}
	}
	sum.Results = results
	// 结果按类型分开存：域名和 IP 的数据不会相互覆盖。
	p.saveChainResults(ctx, kind, results, sum.Probe)
	return sum, nil
}

func mergeChainSummary(dst, src ChainSummary) ChainSummary {
	dst.Checked += src.Checked
	dst.Healthy += src.Healthy
	dst.Unhealthy += src.Unhealthy
	dst.Skipped += src.Skipped
	dst.Total += src.Total
	dst.DurationMs += src.DurationMs
	if src.Entry != "" {
		dst.Entry = src.Entry
	}
	if src.Probe != "" {
		dst.Probe = src.Probe
	}
	dst.Results = append(dst.Results, src.Results...)
	return dst
}

// ChainLastProbeForKind 返回某一类最近一轮实际使用的探针入站（页面展示用）。
func (p *Prober) ChainLastProbeForKind(ctx context.Context, kind string) string {
	kind, ok := normalizeChainKind(kind)
	if !ok || p.setting == nil {
		return ""
	}
	raw, err := p.setting.GetByKey(ctx, chainResultsSettingKey(kind))
	if err != nil || strings.TrimSpace(raw) == "" {
		// 兼容旧版合并结果：探针入站在两类里是同一个。
		raw, err = p.setting.GetByKey(ctx, SettingChainResults)
	}
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

// chainCursorPhaseKey 是「名额少于类型数」时用来轮换类型的游标键。
const chainCursorPhaseKey = "*phase"

// groupChainTargetsByKind 按 Kind 分组，并保留首次出现的顺序。
func groupChainTargetsByKind(targets []ChainTarget) ([]string, map[string][]ChainTarget) {
	order := make([]string, 0, 3)
	groups := make(map[string][]ChainTarget, 3)
	for _, t := range targets {
		if _, ok := groups[t.Kind]; !ok {
			order = append(order, t.Kind)
		}
		groups[t.Kind] = append(groups[t.Kind], t)
	}
	return order, groups
}

// chainTargetQuotas 按各类型的条目数比例分配本轮名额，并保证每类至少 1 个。
// 例如 140 个 IP + 12 个域名、名额 30 → 域名 2 个、IP 28 个。
func chainTargetQuotas(order []string, groups map[string][]ChainTarget, limit int) map[string]int {
	total := 0
	for _, k := range order {
		total += len(groups[k])
	}
	quotas := make(map[string]int, len(order))
	assigned := 0
	for _, k := range order {
		n := len(groups[k])
		q := 0
		if total > 0 {
			q = limit * n / total
		}
		if q < 1 {
			q = 1
		}
		if q > n {
			q = n
		}
		quotas[k] = q
		assigned += q
	}
	// 取整后总配额可能偏离 limit：多了从最大的类减，少了补给剩余最多的类。
	for assigned > limit {
		best, bestQ := "", 0
		for _, k := range order {
			if quotas[k] > 1 && quotas[k] > bestQ {
				best, bestQ = k, quotas[k]
			}
		}
		if best == "" {
			break
		}
		quotas[best]--
		assigned--
	}
	for assigned < limit {
		best, bestRoom := "", 0
		for _, k := range order {
			if room := len(groups[k]) - quotas[k]; room > bestRoom {
				best, bestRoom = k, room
			}
		}
		if best == "" {
			break
		}
		quotas[best]++
		assigned++
	}
	return quotas
}

// takeChainGroup 从一组条目里按游标连续取 n 个（循环覆盖），并推进该组游标。
// 健康/不健康两组各有独立游标，互不干扰。
func takeChainGroup(items []ChainTarget, cursorKey string, n int, cursors map[string]int) []ChainTarget {
	if n <= 0 || len(items) == 0 {
		return nil
	}
	if n > len(items) {
		n = len(items)
	}
	idx := cursors[cursorKey] % len(items)
	out := make([]ChainTarget, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, items[(idx+i)%len(items)])
	}
	cursors[cursorKey] = (idx + n) % len(items)
	return out
}

// selectChainTargets 按类型配额从各自的游标取目标，返回本轮目标与推进后的游标。
//
// 每个类型独立游标是核心：全局限位会让 IP 把名额吃光；按类型配额后，只要
// max_targets ≥ 类型数，每轮都会同时覆盖 IP 和域名，且同一轮内不重复。
//
// 健康优先：每类配额里约 80% 给上次探测健康的条目（这些才是真正会被分发的），
// 至少留 1 个名额给不健康条目做恢复探测；某一组不够时由另一组补满。
func selectChainTargets(targets []ChainTarget, cursors map[string]int, limit int) ([]ChainTarget, map[string]int) {
	if len(targets) == 0 {
		return nil, cursors
	}
	if limit <= 0 || limit > len(targets) {
		limit = len(targets)
	}
	if cursors == nil {
		cursors = map[string]int{}
	}
	order, groups := groupChainTargetsByKind(targets)

	// 每类内部再按健康状态拆组，各自独立游标。
	okGroups := make(map[string][]ChainTarget, len(order))
	badGroups := make(map[string][]ChainTarget, len(order))
	for _, k := range order {
		for _, t := range groups[k] {
			if t.Healthy {
				okGroups[k] = append(okGroups[k], t)
			} else {
				badGroups[k] = append(badGroups[k], t)
			}
		}
	}

	// 名额少于类型数（例如 max_targets=1）：按相位在类型间轮转，
	// 否则小名额永远只落在大类（IP）上，域名仍然测不到。
	if limit < len(order) {
		phase := cursors[chainCursorPhaseKey] % len(order)
		picked := make([]ChainTarget, 0, limit)
		for i := 0; i < limit; i++ {
			k := order[(phase+i)%len(order)]
			got := takeChainGroup(okGroups[k], k+"|ok", 1, cursors)
			if len(got) == 0 {
				got = takeChainGroup(badGroups[k], k+"|bad", 1, cursors)
			}
			picked = append(picked, got...)
		}
		cursors[chainCursorPhaseKey] = (phase + limit) % len(order)
		return picked, cursors
	}

	quotas := chainTargetQuotas(order, groups, limit)
	taken := make(map[string][]ChainTarget, len(order))
	for _, k := range order {
		q := quotas[k]
		ok := okGroups[k]
		bad := badGroups[k]
		okQuota := 0
		if len(ok) > 0 {
			okQuota = q * 8 / 10
			if okQuota < 1 {
				okQuota = 1
			}
			// 不健康条目存在时，至少保留 1 个恢复名额。
			if len(bad) > 0 && okQuota >= q && q > 1 {
				okQuota = q - 1
			}
			if okQuota > len(ok) {
				okQuota = len(ok)
			}
		}
		got := takeChainGroup(ok, k+"|ok", okQuota, cursors)
		if rem := q - len(got); rem > 0 {
			got = append(got, takeChainGroup(bad, k+"|bad", rem, cursors)...)
		}
		// 不健康条目不够（或全健康）：用健康条目补满名额。
		if rem := q - len(got); rem > 0 {
			got = append(got, takeChainGroup(ok, k+"|ok", rem, cursors)...)
		}
		taken[k] = got
	}

	// 输出顺序按类型交替，便于页面阅读/排障（不影响探测结果）。
	picked := make([]ChainTarget, 0, limit)
	for {
		moved := false
		for _, k := range order {
			if len(taken[k]) == 0 {
				continue
			}
			picked = append(picked, taken[k][0])
			taken[k] = taken[k][1:]
			moved = true
		}
		if !moved {
			break
		}
	}
	return picked, cursors
}

// readChainCursors 读取每类游标（JSON）。旧版本存的是单个整数，
// 语义已不相容，直接忽略、从 0 开始（最多多测一轮，不会丢目标）。
func (p *Prober) readChainCursors(ctx context.Context) map[string]int {
	cursors := map[string]int{}
	if p.setting == nil {
		return cursors
	}
	v, err := p.setting.GetByKey(ctx, SettingChainCursor)
	if err != nil {
		return cursors
	}
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(v, "{") {
		return cursors
	}
	_ = json.Unmarshal([]byte(v), &cursors)
	return cursors
}

// saveChainCursors 持久化每类游标，下一轮从各类的下一个条目继续。
func (p *Prober) saveChainCursors(ctx context.Context, cursors map[string]int) {
	if p.setting == nil {
		return
	}
	b, err := json.Marshal(cursors)
	if err != nil {
		return
	}
	_ = p.setting.Set(ctx, SettingChainCursor, string(b))
}

// saveChainResults 用本轮结果**整体覆盖**上一轮结果。
//
// 不能跟历史结果合并：条目被删除/换池、探针入站或出口被改掉之后，旧结果会
// 永远留在页面上（用户看到的是几个月前的 104.x IP 和早已不存在的条目），
// 也无法判断"这张表到底是不是这次测试的结果"。轮转探测（单轮名额小于
// 条目总数）时，这里记录的就是"本轮测过的这一段"。
//
// 结果按类型（ip / domain）分开存：两类各自有独立的单轮名额和游标，
// 写回时不能共用一份，否则后跑的一类会把另一类的结果覆盖掉。
func (p *Prober) saveChainResults(ctx context.Context, kind string, fresh []ChainResult, probe string) {
	if p.setting == nil {
		return
	}
	if fresh == nil {
		fresh = []ChainResult{}
	}
	kind, ok := normalizeChainKind(kind)
	if !ok {
		return
	}
	payload, err := json.Marshal(map[string]interface{}{
		"updated_at": time.Now().Unix(),
		"probe":      probe,
		"results":    fresh,
	})
	if err != nil {
		return
	}
	_ = p.setting.Set(ctx, chainResultsSettingKey(kind), string(payload))
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
