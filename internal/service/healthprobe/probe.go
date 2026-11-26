// Package healthprobe 探测「CF 优选域名 / CF 优选 IP」的可达性与延迟。
//
// 设计口径：
//   - Enabled 是**管理员开关**，Healthy 是**探测结论** —— 两者分开，
//     探测失败只标记不健康，不改用户开关，避免误操作不可逆；
//   - 连续失败 3 次才判定不健康（一次抖动不下线），成功即恢复；
//   - 订阅渲染只挑健康、且延迟不超过阈值的条目；如果全部不达标则回退全量，
//     防止一次网络故障把订阅里的地址全清空。
//
// 探测入口（可选）：
//   - 默认本机直连；
//   - 也可以选一个「外部导入节点」，临时起 sing-box / mihomo 做 SOCKS 拨号，
//     探测完立即结束进程并清理临时配置（不与线上核心共用配置）。
//
// 探活方式：
//   - 优选 IP 只探 TCP（Cloudflare 对裸 IP 做 TLS 常被重置，TCP 可达即可用）；
//   - 优选域名做 TLS 握手（域名过 CF 代理，握手失败说明不可用）。
package healthprobe

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"corefusion/internal/model/entity"
	"corefusion/internal/runtime/agent"
	"corefusion/internal/service/binaries"
	"corefusion/internal/service/coreconfig"
	"corefusion/internal/store/repository"
)

const (
	probeTimeout  = 3 * time.Second
	failThreshold = 3
	concurrency   = 8

	// 设置键（管理端可改）
	SettingEnabled    = "cfip.probe.enabled"
	SettingInterval   = "cfip.probe.interval_minutes"
	SettingMaxLatency = "cfip.probe.max_latency_ms"
	// SettingEntry 是统一的入口标识："ext:<id>" 外部导入节点 / "inb:<id>" 自建入站。
	// 空 = 本机直连。旧键 SettingEntryNode 仅用于兼容老库/老前端。
	SettingEntry     = "cfip.probe.entry"
	SettingEntryNode = "cfip.probe.entry_node_id"
	SettingDialer    = "cfip.probe.dialer"
	// SettingAgent 指定执行探测的节点（0 = 面板本机）；SettingInterface 是出口网卡。
	SettingAgent     = "cfip.probe.agent_id"
	SettingInterface = "cfip.probe.interface"

	// 默认关闭：不是所有部署都用 Cloudflare 优选（CF IP 池 / 优选域名），
	// 无人使用还每 5 分钟跑一轮探测纯属浪费（尤其选了入口节点时还要起临时
	// 核心进程）。管理员在「分流资源 → CF IP 池 / 直连 & CF 优选域名」里
	// 打开探测后才开始定时执行；手动「检测连通性」不受影响。
	DefaultEnabled         = false
	DefaultIntervalMinutes = 5
	DefaultMaxLatencyMs    = 0  // 0 = 不限
	DefaultDialer          = "" // 空 = 自动选择已安装的核心（xray → sing-box → mihomo）
)

// Config 是一次探测的运行时配置（每次探测前从设置里读，改完立即生效）。
type Config struct {
	// Enabled 控制定时探测是否运行。默认关闭；关闭时只保留手动「检测连通性」。
	Enabled         bool `json:"enabled"`
	IntervalMinutes int  `json:"interval_minutes"`
	MaxLatencyMs    int  `json:"max_latency_ms"`
	// Entry 统一入口标识："ext:<id>" 外部导入节点 / "inb:<id>" 自建入站；空 = 本机直连。
	// 选了走 CDN 的入口（带 SNI 的 TLS 配置）时，探测会把它当客户端配置模板，
	// 只替换 server/port 去真实拨号；否则仍按「从该节点出去」的出口语义使用。
	Entry string `json:"entry"`
	// EntryNodeID 是旧字段（只支持外部节点），保留用于兼容老前端与老库。
	EntryNodeID int64  `json:"entry_node_id"`
	Dialer      string `json:"dialer"`
	// AgentID 指定在哪台节点上执行探测（0 = 面板本机）。
	// 选定后临时核心与拨号都在该节点上跑，网卡也取该节点的。
	AgentID int64 `json:"agent_id"`
	// Interface 是执行探测时绑定的本机网卡（仅对执行探测的那台机器有意义）。
	// 留空 = 系统默认路由。
	Interface string `json:"interface"`
}

// Result 是一次全量探测的汇总（供接口返回/日志）。
type Result struct {
	Checked   int `json:"checked"`
	Healthy   int `json:"healthy"`
	Unhealthy int `json:"unhealthy"`
	Skipped   int `json:"skipped"`
	// LatencyOnly 是「只刷新延迟、不判定健康」的条目数：链路探测启用时，
	// CF 条目的健康状态由链路探测独占判定，TCP/TLS 探测只负责测延迟。
	LatencyOnly int    `json:"latency_only"`
	Entry       string `json:"entry"`
	Interval    int    `json:"interval_minutes"`
}

// Prober 负责优选域名/IP 的探活。
type Prober struct {
	cfRepo    *repository.CFIPEntryRepository
	shareRepo *repository.ShareDomainConfigRepository
	setting   *repository.SystemSettingRepository
	nodeRepo  *repository.ExternalNodeRepository
	resolver  *binaries.Resolver
	dataDir   string

	// mu 防止手动检测与定时检测同时跑（会互相覆盖 fail_count）。
	mu sync.Mutex
	// chainMu 保护链路探测自身（与优选 IP 探测分开加锁，避免 CheckAll
	// 持锁期间调用链路探测造成嵌套死锁）。
	chainMu sync.Mutex
	// chainProv 由订阅 handler 注入，把入站渲染成客户端出站参数。
	chainProv ChainTargetProvider
	// chainRemote 由 router 注入，在选定的 Agent 上执行链路探测。
	chainRemote ChainRemoteRunner
	// agentDialers 由 router 注入，查询执行节点上已安装的拨号核心。
	// 探测核心装在哪台机器上必须问那台机器，不能用面板的清单顶替。
	agentDialers AgentDialerLister
	// wake 唤醒调度器重新读配置（开关/间隔改动后立即生效）。
	wake chan struct{}
}

// New 创建探测器。
func New(
	cfRepo *repository.CFIPEntryRepository,
	shareRepo *repository.ShareDomainConfigRepository,
	setting *repository.SystemSettingRepository,
	nodeRepo *repository.ExternalNodeRepository,
	resolver *binaries.Resolver,
	dataDir string,
) *Prober {
	return &Prober{
		cfRepo:    cfRepo,
		shareRepo: shareRepo,
		setting:   setting,
		nodeRepo:  nodeRepo,
		resolver:  resolver,
		dataDir:   dataDir,
		wake:      make(chan struct{}, 1),
	}
}

// ReadConfig 读取当前探测配置（缺失项回落默认值）。
func (p *Prober) ReadConfig(ctx context.Context) Config {
	cfg := Config{
		Enabled:         DefaultEnabled,
		IntervalMinutes: DefaultIntervalMinutes,
		MaxLatencyMs:    DefaultMaxLatencyMs,
		Dialer:          DefaultDialer,
	}
	if p.setting == nil {
		return cfg
	}
	if v, err := p.setting.GetByKey(ctx, SettingEnabled); err == nil {
		cfg.Enabled = parseBoolSetting(v, DefaultEnabled)
	}
	if v, err := p.setting.GetByKey(ctx, SettingInterval); err == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 {
			cfg.IntervalMinutes = n
		}
	}
	if v, err := p.setting.GetByKey(ctx, SettingMaxLatency); err == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n >= 0 {
			cfg.MaxLatencyMs = n
		}
	}
	if v, err := p.setting.GetByKey(ctx, SettingEntry); err == nil {
		cfg.Entry = strings.TrimSpace(v)
	}
	if v, err := p.setting.GetByKey(ctx, SettingEntryNode); err == nil {
		if n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil && n > 0 {
			cfg.EntryNodeID = n
		}
	}
	// 老库只写了 entry_node_id：推导成统一的 ext: 形式。
	if cfg.Entry == "" && cfg.EntryNodeID > 0 {
		cfg.Entry = fmt.Sprintf("ext:%d", cfg.EntryNodeID)
	}
	if v, err := p.setting.GetByKey(ctx, SettingDialer); err == nil {
		cfg.Dialer = strings.TrimSpace(v)
	}
	if v, err := p.setting.GetByKey(ctx, SettingAgent); err == nil {
		if n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil && n > 0 {
			cfg.AgentID = n
		}
	}
	if v, err := p.setting.GetByKey(ctx, SettingInterface); err == nil {
		cfg.Interface = strings.TrimSpace(v)
	}
	return cfg
}

// AgentDialerInfo 是执行节点上报的已安装拨号核心。
type AgentDialerInfo struct {
	Dialers  []string          `json:"dialers"`
	Versions map[string]string `json:"dialer_versions"`
}

// AgentDialerLister 查询某个节点上已安装的拨号核心（serverID=0 表示面板本机）。
type AgentDialerLister func(ctx context.Context, serverID int64) (AgentDialerInfo, error)

// SetAgentDialerLister 注入「查询执行节点拨号核心」的实现。
func (p *Prober) SetAgentDialerLister(fn AgentDialerLister) {
	p.agentDialers = fn
}

// dialerCandidates 是自动模式下的优先顺序。
//
// xray 优先：链路探测会把入站渲染成客户端出站（vmess/vless + ws/grpc/reality
// 等组合），sing-box 在部分组合上会直接 reject，表现为「全部条目失败」的假结论。
// 优选探测与链路探测共用这个顺序，保证「探测设置里不选核心」时两者口径一致。
var dialerCandidates = []string{"xray", "singbox", "mihomo"}

// installedDialers 返回当前节点已安装的拨号核心。
func (p *Prober) installedDialers() []string {
	if p.resolver == nil {
		return nil
	}
	out := make([]string, 0, len(dialerCandidates))
	for _, name := range dialerCandidates {
		if _, _, err := p.resolver.Resolve(name); err == nil {
			out = append(out, name)
		}
	}
	return out
}

// firstAvailableDialer 取第一个已安装核心（纯函数便于测试）。
func firstAvailableDialer(installed []string) string {
	for _, name := range installed {
		if strings.TrimSpace(name) != "" {
			return name
		}
	}
	return ""
}

// dialerDisplayName 是给用户看的核心名。
func dialerDisplayName(name string) string {
	switch name {
	case "singbox":
		return "sing-box"
	case "xray":
		return "xray"
	case "mihomo":
		return "mihomo"
	}
	return name
}

// parseBoolSetting 解析设置里的布尔值；无法识别时用默认值（不误开）。
func parseBoolSetting(v string, def bool) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return def
}

// SaveConfig 保存探测配置（nil 表示不改该项）。
// ConfigUpdate 是「探测设置」的一次局部更新（nil 字段保持不变）。
type ConfigUpdate struct {
	Enabled         *bool
	IntervalMinutes *int
	MaxLatencyMs    *int
	Entry           *string
	EntryNodeID     *int64
	Dialer          *string
	AgentID         *int64
	Interface       *string
}

func (p *Prober) SaveConfig(ctx context.Context, upd ConfigUpdate) error {
	if p.setting == nil {
		return fmt.Errorf("settings repository not available")
	}
	enabled := upd.Enabled
	intervalMinutes := upd.IntervalMinutes
	maxLatencyMs := upd.MaxLatencyMs
	entry := upd.Entry
	entryNodeID := upd.EntryNodeID
	dialer := upd.Dialer
	if enabled != nil {
		val := "false"
		if *enabled {
			val = "true"
		}
		if err := p.setting.Set(ctx, SettingEnabled, val); err != nil {
			return err
		}
	}
	if intervalMinutes != nil {
		if err := p.setting.Set(ctx, SettingInterval, strconv.Itoa(*intervalMinutes)); err != nil {
			return err
		}
	}
	if maxLatencyMs != nil {
		if err := p.setting.Set(ctx, SettingMaxLatency, strconv.Itoa(*maxLatencyMs)); err != nil {
			return err
		}
	}
	if entry != nil {
		spec := strings.TrimSpace(*entry)
		if err := p.setting.Set(ctx, SettingEntry, spec); err != nil {
			return err
		}
		// 同步旧字段：ext:<id> 时写回 entry_node_id，保证回滚到旧版本仍可用。
		legacy := int64(0)
		if strings.HasPrefix(spec, "ext:") {
			if n, err := strconv.ParseInt(strings.TrimPrefix(spec, "ext:"), 10, 64); err == nil && n > 0 {
				legacy = n
			}
		}
		if err := p.setting.Set(ctx, SettingEntryNode, strconv.FormatInt(legacy, 10)); err != nil {
			return err
		}
	} else if entryNodeID != nil {
		// 旧前端只传 entry_node_id。
		if err := p.setting.Set(ctx, SettingEntryNode, strconv.FormatInt(*entryNodeID, 10)); err != nil {
			return err
		}
		spec := ""
		if *entryNodeID > 0 {
			spec = fmt.Sprintf("ext:%d", *entryNodeID)
		}
		if err := p.setting.Set(ctx, SettingEntry, spec); err != nil {
			return err
		}
	}
	if dialer != nil {
		if err := p.setting.Set(ctx, SettingDialer, strings.TrimSpace(*dialer)); err != nil {
			return err
		}
	}
	if upd.AgentID != nil {
		agentID := *upd.AgentID
		if agentID < 0 {
			agentID = 0
		}
		if err := p.setting.Set(ctx, SettingAgent, strconv.FormatInt(agentID, 10)); err != nil {
			return err
		}
	}
	if upd.Interface != nil {
		iface := strings.TrimSpace(*upd.Interface)
		if !ValidEgressInterface(iface) {
			return fmt.Errorf("非法网卡名：%q", iface)
		}
		if err := p.setting.Set(ctx, SettingInterface, iface); err != nil {
			return err
		}
	}
	// 开关/间隔可能是从「关闭」切到「开启」：立刻唤醒调度器，不用等旧间隔。
	p.Wake()
	return nil
}

// Wake 唤醒调度器按新配置重新判断（非阻塞）。
func (p *Prober) Wake() {
	if p.wake == nil {
		return
	}
	select {
	case p.wake <- struct{}{}:
	default:
	}
}

// StartScheduler 周期性探测：每轮先读配置，间隔改完下一轮就生效。
//
// 时间间隔与拨号核心都来自「探测设置」（Config）：
//   - 间隔 = cfg.IntervalMinutes；
//   - 核心 = cfg.Dialer（空 = 自动，xray → sing-box → mihomo）。
//
// 两个开关各自独立：优选探测看 cfg.Enabled，链路探测看 ChainConfig.Enabled。
// 只开链路探测时也会按探测设置的间隔跑（不会因为优选探测关着就不跑）；
// 两者都关时才完全不探测，只等配置变化或进程退出。
func (p *Prober) StartScheduler(ctx context.Context) {
	go func() {
		var lastProbe, lastChain time.Time
		for {
			cfg := p.ReadConfig(ctx)
			chainCfg := p.ReadChainConfig(ctx)
			if !cfg.Enabled && !chainCfg.Enabled {
				select {
				case <-ctx.Done():
					return
				case <-p.wake:
				}
				continue
			}

			probeEvery := time.Duration(cfg.IntervalMinutes) * time.Minute
			if probeEvery < time.Minute {
				probeEvery = time.Minute
			}
			// 链路探测默认继承「探测设置」；独立设置时用自己的间隔。
			chainEvery := time.Duration(chainCfg.EffectiveInterval(cfg.IntervalMinutes)) * time.Minute
			if chainEvery < time.Minute {
				chainEvery = time.Minute
			}

			now := time.Now()
			// 优选探测：即使只开链路探测也要跑，用来刷新延迟数据。
			if cfg.Enabled || chainCfg.Enabled {
				if lastProbe.IsZero() || now.Sub(lastProbe) >= probeEvery {
					res, err := p.CheckAll(ctx)
					if err != nil {
						log.Printf("[healthprobe] check failed: %v", err)
					} else {
						log.Printf("[healthprobe] checked=%d healthy=%d unhealthy=%d skipped=%d latency_only=%d entry=%s interval=%dm",
							res.Checked, res.Healthy, res.Unhealthy, res.Skipped, res.LatencyOnly, res.Entry, res.Interval)
					}
					lastProbe = time.Now()
				}
			}
			// 链路探测串行在优选 IP 之后：两者都要起临时核心，同时跑会互相抢 CPU。
			if chainCfg.Enabled && (lastChain.IsZero() || time.Since(lastChain) >= chainEvery) {
				if chainRes, cerr := p.CheckChain(ctx, false); cerr != nil {
					log.Printf("[healthprobe] chain check failed: %v", cerr)
				} else if chainRes.Checked > 0 {
					log.Printf("[healthprobe] chain checked=%d healthy=%d unhealthy=%d skipped=%d entry=%s duration=%dms",
						chainRes.Checked, chainRes.Healthy, chainRes.Unhealthy, chainRes.Skipped, chainRes.Entry, chainRes.DurationMs)
				}
				lastChain = time.Now()
			}

			wait := 24 * time.Hour
			if cfg.Enabled || chainCfg.Enabled {
				if d := time.Until(lastProbe.Add(probeEvery)); d < wait {
					wait = d
				}
			}
			if chainCfg.Enabled {
				if d := time.Until(lastChain.Add(chainEvery)); d < wait {
					wait = d
				}
			}
			if wait < 5*time.Second {
				wait = 5 * time.Second
			}
			select {
			case <-ctx.Done():
				return
			case <-p.wake:
			case <-time.After(wait):
			}
		}
	}()
}

// dialFunc 统一拨号入口：nil 表示本机直连。
type dialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// CheckAll 探测全部优选 IP 与优选域名。
func (p *Prober) CheckAll(ctx context.Context) (Result, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	cfg := p.ReadConfig(ctx)
	res := Result{Entry: "local", Interval: cfg.IntervalMinutes}

	// 链路探测开启时，CF 条目（域名/IP）的健康状态由链路探测独占判定：
	// 这里的 TCP/TLS 探测只说明"端口能连"，会把链路探测判死的条目复活，
	// 导致订阅又把坏节点分发出去。因此这些条目交给 CheckChain 处理。
	chainAuthority := p.ReadChainConfig(ctx).Enabled

	// 入口必须显式选择（ext:<id> / inb:<id>），不再支持本机直连：
	// 裸 TCP 探测只能证明「端口有人监听」，会把别人的源站、已失效的优选 IP
	// 全判成健康；只有带接入域名（SNI）的真实拨号才有意义。
	if cfg.Entry == "" {
		if chainAuthority {
			// 健康由链路探测独占；没入口时不产生任何结论（连延迟也不写，
			// 避免给订阅的延迟阈值喂假数据）。
			return res, nil
		}
		return res, errors.New("未选择探测入口：已禁用本机直连探测（只测 TCP 会把坏 IP 判成可用），请在「探测入口节点」里选择一个配了 CDN 接入的节点/入站")
	}

	// 入口：统一按 "ext:<id>" / "inb:<id>" 解析成客户端出站。
	//   - 走 CDN 的入口（TLS + SNI）：当**客户端配置模板**，逐条替换 server/port 真拨号；
	//   - 其它入口：保持「从该节点出去」的出口语义（临时核心做 SOCKS 跳板）。
	// 明确选了入口就必须用它：核心未安装/入口无效时直接报错，不能静默退回本机。
	var dial dialFunc
	cleanup := func() {}
	var tpl map[string]interface{}
	if cfg.Entry != "" {
		pm, label, err := p.ResolveEntryTemplate(ctx, cfg.Entry)
		if err != nil {
			return res, err
		}
		if isCDNTemplate(pm) {
			tpl = pm
			res.Entry = "tpl:" + label
			if cfg.AgentID > 0 {
				res.Entry = fmt.Sprintf("%s@agent:%d", res.Entry, cfg.AgentID)
			}
		} else if cfg.AgentID > 0 {
			// 出口语义也支持在别的节点上执行：临时核心跑在 Agent 上，
			// 面板只负责生成配置（端口写 0），TCP/TLS 探测由 Agent 完成。
			if p.chainRemote == nil {
				return res, errors.New("Agent 执行器未初始化，无法在指定节点上执行")
			}
			dialer, derr := p.pickDialerOn(ctx, cfg.Dialer, cfg.AgentID)
			if derr != nil {
				return res, derr
			}
			res.Entry = fmt.Sprintf("%s/%s@agent:%d", label, dialer, cfg.AgentID)
			return p.checkAllExitRemote(ctx, cfg, res, pm, chainAuthority)
		} else {
			d, fn, usedDialer, err := p.startEntryDialer(ctx, cfg, pm)
			if err != nil {
				return res, err
			}
			dial, cleanup = d, fn
			res.Entry = label + "/" + usedDialer
		}
	}
	defer cleanup()

	// 模板模式：真实拨号 + 双向流量判定，不再走 probeTCP/probeTLS。
	if tpl != nil {
		return p.checkAllWithTemplate(ctx, cfg, res, tpl, chainAuthority)
	}

	var wg sync.WaitGroup
	var resMu sync.Mutex
	sem := make(chan struct{}, concurrency)

	record := func(healthy bool) {
		resMu.Lock()
		res.Checked++
		if healthy {
			res.Healthy++
		} else {
			res.Unhealthy++
		}
		resMu.Unlock()
	}
	skip := func() {
		resMu.Lock()
		res.Skipped++
		resMu.Unlock()
	}
	latencyOnly := func() {
		resMu.Lock()
		res.LatencyOnly++
		resMu.Unlock()
	}

	var firstErr error
	setErr := func(err error) {
		resMu.Lock()
		if firstErr == nil {
			firstErr = err
		}
		resMu.Unlock()
	}

	// 1) 优选 IP（cf_ip_entries）
	if p.cfRepo != nil {
		entries, err := p.cfRepo.List(ctx, 0)
		if err != nil {
			setErr(err)
		} else {
			for i := range entries {
				e := entries[i]
				if !e.Enabled || strings.TrimSpace(e.IP) == "" {
					skip()
					continue
				}
				port := firstPort(e.Ports, 443)
				wg.Add(1)
				sem <- struct{}{}
				go func(e entity.CFIPEntry, port int) {
					defer wg.Done()
					defer func() { <-sem }()
					ok, latency := probeTCP(ctx, e.IP, port, dial)
					if chainAuthority {
						// 链路探测独占健康判定：这里只刷新延迟（订阅的延迟阈值
						// 按 TCP 握手延迟筛），不写 healthy，避免把判死的条目复活。
						if err := p.cfRepo.UpdateLatency(ctx, e.ID, latency, time.Now()); err != nil {
							log.Printf("[healthprobe] update cf ip latency %s failed: %v", e.IP, err)
						}
						latencyOnly()
						return
					}
					healthy, fail := nextState(ok, e.FailCount)
					if err := p.cfRepo.UpdateHealth(ctx, e.ID, healthy, latency, fail, time.Now()); err != nil {
						log.Printf("[healthprobe] update cf ip %s failed: %v", e.IP, err)
					}
					record(healthy)
				}(e, port)
			}
		}
	}

	// 2) 优选域名（share_domain_configs 里 type=cf_domain）
	if p.shareRepo != nil {
		domains, err := p.shareRepo.List(ctx, "cf_domain")
		if err != nil {
			setErr(err)
		} else {
			for i := range domains {
				d := domains[i]
				if !d.Enabled || strings.TrimSpace(d.Value) == "" {
					skip()
					continue
				}
				port := firstPort(d.Ports, d.Port)
				wg.Add(1)
				sem <- struct{}{}
				go func(d entity.ShareDomainConfig, port int) {
					defer wg.Done()
					defer func() { <-sem }()
					ok, latency := probeTLS(ctx, d.Value, port, dial)
					if chainAuthority {
						// 同上：只刷新延迟，健康状态留给链路探测。
						if err := p.shareRepo.UpdateLatency(ctx, d.ID, latency, time.Now()); err != nil {
							log.Printf("[healthprobe] update share domain latency %s failed: %v", d.Value, err)
						}
						latencyOnly()
						return
					}
					healthy, fail := nextState(ok, d.FailCount)
					if err := p.shareRepo.UpdateHealth(ctx, d.ID, healthy, latency, fail, time.Now()); err != nil {
						log.Printf("[healthprobe] update share domain %s failed: %v", d.Value, err)
					}
					record(healthy)
				}(d, port)
			}
		}
	}

	wg.Wait()
	return res, firstErr
}

// nextState 根据本次结果推导 healthy / 连续失败次数。
// 连续失败达到阈值才判不健康；任一次成功立即恢复。
func nextState(ok bool, prevFail int) (healthy bool, failCount int) {
	if ok {
		return true, 0
	}
	failCount = prevFail + 1
	return failCount < failThreshold, failCount
}

// firstPort 从 JSON 端口数组里取第一个端口；取不到时回落 fallback。
func firstPort(portsJSON string, fallback int) int {
	if strings.TrimSpace(portsJSON) != "" {
		var ports []int
		if json.Unmarshal([]byte(portsJSON), &ports) == nil && len(ports) > 0 && ports[0] > 0 {
			return ports[0]
		}
	}
	if fallback > 0 {
		return fallback
	}
	return 443
}

// probeTCP 测 TCP 可达性，返回毫秒延迟。
func probeTCP(ctx context.Context, host string, port int, dial dialFunc) (bool, int) {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	start := time.Now()
	conn, err := dialConn(ctx, dial, "tcp", addr)
	if err != nil {
		return false, 0
	}
	_ = conn.Close()
	return true, elapsedMs(start)
}

// probeTLS 对优选域名做 TLS 握手（跳过证书校验：CF 边缘证书常与本机信任链无关）。
func probeTLS(ctx context.Context, host string, port int, dial dialFunc) (bool, int) {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	start := time.Now()
	raw, err := dialConn(ctx, dial, "tcp", addr)
	if err != nil {
		return false, 0
	}
	defer raw.Close()
	tlsConn := tls.Client(raw, &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS12,
	})
	hsCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	if err := tlsConn.HandshakeContext(hsCtx); err != nil {
		return false, 0
	}
	return true, elapsedMs(start)
}

// elapsedMs 向上取整到毫秒：本机/同区域探测常在 1ms 内完成，
// 直接截断会写成 0ms（看起来像"没测到"，也没法和阈值比较）。
func elapsedMs(start time.Time) int {
	d := time.Since(start)
	ms := int(d / time.Millisecond)
	if d%time.Millisecond != 0 {
		ms++
	}
	if ms < 1 {
		ms = 1
	}
	return ms
}

// dialConn 走指定拨号器（nil = 本机直连）。
func dialConn(ctx context.Context, dial dialFunc, network, addr string) (net.Conn, error) {
	if dial != nil {
		return dial(ctx, network, addr)
	}
	d := &net.Dialer{Timeout: probeTimeout}
	return d.DialContext(ctx, network, addr)
}

// pickDialer 归一拨号核心名；空值按 xray → sing-box → mihomo 取第一个已安装的。
func (p *Prober) pickDialer(configured string) (string, error) {
	dialer := strings.ToLower(strings.TrimSpace(configured))
	if dialer == "clash" {
		dialer = "mihomo"
	}
	if dialer == "v2ray" {
		dialer = "xray"
	}
	if dialer == "" {
		dialer = firstAvailableDialer(p.installedDialers())
		if dialer == "" {
			return "", fmt.Errorf("未安装任何可用拨号核心（sing-box / xray / mihomo），请先在「二进制管理」中安装后再探测")
		}
	}
	switch dialer {
	case "singbox", "mihomo", "xray":
		return dialer, nil
	default:
		return "", fmt.Errorf("不支持的拨号核心 %q（可选 sing-box / mihomo / xray）", configured)
	}
}

// AgentDialers 返回执行节点上真正已安装的拨号核心。
// agentID=0 时返回面板本机的清单（与 EffectiveDialer 口径一致）。
func (p *Prober) AgentDialers(ctx context.Context, agentID int64) (AgentDialerInfo, error) {
	if agentID <= 0 {
		installed := p.installedDialers()
		info := AgentDialerInfo{Dialers: installed, Versions: map[string]string{}}
		for _, name := range installed {
			if p.resolver != nil {
				if v := p.resolver.Version(name); v != "" {
					info.Versions[name] = v
				}
			}
		}
		return info, nil
	}
	if p.agentDialers == nil {
		return AgentDialerInfo{}, fmt.Errorf("执行节点能力查询未初始化")
	}
	return p.agentDialers(ctx, agentID)
}

// pickDialerOn 在「指定执行节点」上挑选拨号核心。
//
// 选中 Agent 时核心必须装在该节点上：面板的已安装清单只代表面板本机，
// 用它挑出来的核心到节点上会直接 `binary not installed`。
func (p *Prober) pickDialerOn(ctx context.Context, configured string, agentID int64) (string, error) {
	if agentID <= 0 {
		return p.pickDialer(configured)
	}
	info, err := p.AgentDialers(ctx, agentID)
	if err != nil {
		return "", fmt.Errorf("无法查询执行节点（agent:%d）的拨号核心：%w", agentID, err)
	}
	installed := make(map[string]bool, len(info.Dialers))
	for _, name := range info.Dialers {
		installed[name] = true
	}
	want := normalizeDialerName(configured)
	if want != "" {
		if !installed[want] {
			return "", fmt.Errorf("执行节点（agent:%d）未安装 %s 拨号核心：请在「节点 → 核心/二进制」里安装，或改用该节点已安装的核心（%s）",
				agentID, dialerDisplayName(want), joinDialerNames(info.Dialers))
		}
		return want, nil
	}
	for _, name := range dialerCandidates {
		if installed[name] {
			return name, nil
		}
	}
	return "", fmt.Errorf("执行节点（agent:%d）没有可用的拨号核心（xray / sing-box / mihomo 均未安装）", agentID)
}

// EffectiveDialerOn 返回指定执行节点上实际会用的拨号核心（空配置按该节点
// 已安装情况自动挑选）。查询失败时返回空串，由页面提示。
func (p *Prober) EffectiveDialerOn(ctx context.Context, configured string, agentID int64) string {
	if d, err := p.pickDialerOn(ctx, configured, agentID); err == nil {
		return d
	}
	return ""
}

// normalizeDialerName 归一核心名（v2ray→xray，clash→mihomo）。
func normalizeDialerName(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "clash":
		return "mihomo"
	case "v2ray":
		return "xray"
	}
	return strings.ToLower(strings.TrimSpace(name))
}

func joinDialerNames(names []string) string {
	if len(names) == 0 {
		return "无"
	}
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = dialerDisplayName(n)
	}
	return strings.Join(out, " / ")
}

// EffectiveDialer 返回实际会用的拨号核心（配置为空时按自动顺序解析）。
// 管理端用它展示「探测设置」当前生效的拨号工具，避免页面只显示空值——
// 链路探测的拨号核心就是从探测设置读的，两处必须一致。
func (p *Prober) EffectiveDialer(configured string) string {
	if d, err := p.pickDialer(configured); err == nil {
		return d
	}
	return ""
}

// startDialerWithProxy 用给定的 mihomo 出站 map 起一个临时核心做 SOCKS 拨号。
// 供「优选探测」（外部节点）与「链路探测」（外部节点或自建节点入站）共用。
func (p *Prober) startDialerWithProxy(ctx context.Context, dialer string, proxy map[string]interface{}, iface string) (dialFunc, func(), string, error) {
	noop := func() {}
	if len(proxy) == 0 {
		return nil, noop, "", fmt.Errorf("出口节点配置为空，无法建立拨号")
	}
	switch dialer {
	case "singbox", "mihomo", "xray":
	default:
		return nil, noop, "", fmt.Errorf("不支持的拨号核心 %q（可选 sing-box / mihomo / xray）", dialer)
	}
	port, err := freePort()
	if err != nil {
		return nil, noop, "", err
	}
	dir := filepath.Join(p.dataDir, "probe", fmt.Sprintf("run-%d", time.Now().UnixNano()))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, noop, "", err
	}
	cleanupDir := func() { _ = os.RemoveAll(dir) }

	var binPath, cfgPath string
	var args []string
	switch dialer {
	case "mihomo":
		binPath, err = p.resolveBinary("mihomo")
		if err != nil {
			cleanupDir()
			return nil, noop, "", err
		}
		cfgPath = filepath.Join(dir, "config.yaml")
		// JSON 是合法 YAML，mihomo 直接吃。
		conf := map[string]interface{}{
			"mixed-port": port,
			"allow-lan":  false,
			"mode":       "global",
			"log-level":  "silent",
			"proxies":    []interface{}{proxy},
		}
		applyInterfaceToAllOutbounds(conf, dialer, iface)
		if err := writeJSON(cfgPath, conf); err != nil {
			cleanupDir()
			return nil, noop, "", err
		}
		args = []string{"-f", cfgPath, "-d", dir}
	case "xray":
		binPath, err = p.resolveBinary("xray")
		if err != nil {
			cleanupDir()
			return nil, noop, "", err
		}
		xob, convErr := agent.BuildXrayOutboundFromMihomoProxy(proxy)
		if convErr != nil {
			cleanupDir()
			return nil, noop, "", fmt.Errorf("convert proxy for xray: %w", convErr)
		}
		cfgPath = filepath.Join(dir, "config.json")
		conf := map[string]interface{}{
			"log": map[string]interface{}{"loglevel": "error"},
			"inbounds": []interface{}{map[string]interface{}{
				"tag":      "probe-in",
				"listen":   "127.0.0.1",
				"port":     port,
				"protocol": "socks",
				"settings": map[string]interface{}{"auth": "noauth", "udp": false},
			}},
			"outbounds": []interface{}{xob},
		}
		applyInterfaceToAllOutbounds(conf, dialer, iface)
		if err := writeJSON(cfgPath, conf); err != nil {
			cleanupDir()
			return nil, noop, "", err
		}
		args = []string{"run", "-c", cfgPath}
	default: // singbox
		binPath, err = p.resolveBinary("singbox")
		if err != nil {
			cleanupDir()
			return nil, noop, "", err
		}
		// mihomo proxy map → xray 风格出站 → sing-box 出站（与链式代理同一套转换）。
		xob, convErr := agent.BuildXrayOutboundFromMihomoProxy(proxy)
		if convErr != nil {
			cleanupDir()
			return nil, noop, "", fmt.Errorf("convert proxy for sing-box: %w", convErr)
		}
		outs := coreconfig.ConvertXrayOutboundsToSingbox([]interface{}{xob})
		if len(outs) == 0 {
			cleanupDir()
			return nil, noop, "", fmt.Errorf("convert proxy to sing-box outbound failed")
		}
		cfgPath = filepath.Join(dir, "config.json")
		conf := map[string]interface{}{
			"log": map[string]interface{}{"level": "error"},
			"inbounds": []interface{}{map[string]interface{}{
				"type": "socks", "tag": "probe-in", "listen": "127.0.0.1", "listen_port": port,
			}},
			"outbounds": outs,
		}
		applyInterfaceToAllOutbounds(conf, dialer, iface)
		if err := writeJSON(cfgPath, conf); err != nil {
			cleanupDir()
			return nil, noop, "", err
		}
		args = []string{"run", "-c", cfgPath}
	}

	cmd := exec.Command(binPath, args...)
	cmd.Dir = dir
	var coreOut bytes.Buffer
	cmd.Stdout = &coreOut
	cmd.Stderr = &coreOut
	if err := cmd.Start(); err != nil {
		cleanupDir()
		return nil, noop, "", fmt.Errorf("启动 %s 失败：%w", dialerDisplayName(dialer), err)
	}
	proxyAddr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	if !waitPort(proxyAddr, 6*time.Second) {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		msg := strings.TrimSpace(coreOut.String())
		cleanupDir()
		if msg == "" {
			msg = "无输出"
		}
		return nil, noop, "", fmt.Errorf("%s 未能在 6 秒内打开本地 SOCKS 端口：%s", dialerDisplayName(dialer), truncateErr(msg))
	}
	cleanup := func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		cleanupDir()
	}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return socks5Dial(proxyAddr, addr, probeTimeout)
	}, cleanup, dialer, nil
}

// resolveBinary 找核心二进制（面板 data/binaries/active 下）。
func (p *Prober) resolveBinary(coreType string) (string, error) {
	if p.resolver == nil {
		return "", fmt.Errorf("binary resolver not configured")
	}
	path, _, err := p.resolver.Resolve(coreType)
	if err != nil {
		return "", fmt.Errorf("%s 拨号核心未安装，请先在「二进制管理」中安装：%w", dialerDisplayName(coreType), err)
	}
	return path, nil
}

func writeJSON(path string, v interface{}) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func waitPort(addr string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// startEntryDialer 把一个入口出站参数起成临时 SOCKS 拨号器（出口语义）。
func (p *Prober) startEntryDialer(ctx context.Context, cfg Config, pm map[string]interface{}) (dialFunc, func(), string, error) {
	dialer, err := p.pickDialer(cfg.Dialer)
	if err != nil {
		return nil, func() {}, "", err
	}
	return p.startDialerWithProxy(ctx, dialer, pm, cfg.Interface)
}

// isCDNTemplate 判断入口配置能否当「客户端配置模板」用：
// 有 TLS 且带 SNI/servername。这类配置（如 VLESSGRPC、trojan-worker）本身就是
// 客户端连 CDN 的真实形态，替换 server 后可直接验证优选 IP/域名。
func isCDNTemplate(pm map[string]interface{}) bool {
	if len(pm) == 0 {
		return false
	}
	tls, _ := pm["tls"].(bool)
	if !tls {
		return false
	}
	return proxyString(pm, "sni", "servername", "server-name") != ""
}

func proxyString(pm map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := pm[k].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// templateProxy 复制入口配置，只替换 server/port。
// SNI/Host/ALPN/ws/grpc 参数与凭据必须保持原样 —— 这就是客户端真实组合。
// skip-cert-verify 强制关闭：入口配置当跳板时会被置 true，沿用会掩盖证书不匹配，
// 把「别人的源站」误判成可用。
func templateProxy(tpl map[string]interface{}, host string, port int) map[string]interface{} {
	raw, err := json.Marshal(tpl)
	if err != nil {
		return nil
	}
	var cp map[string]interface{}
	if json.Unmarshal(raw, &cp) != nil || cp == nil {
		return nil
	}
	cp["server"] = host
	cp["port"] = port
	cp["skip-cert-verify"] = false
	delete(cp, "dialer-proxy")
	return cp
}

const chainTemplateBatchSize = 40

// checkAllWithTemplate 用入口配置当模板，逐条替换 server 后真实拨号。
// 判定口径与链路探测一致：HTTP 2xx/3xx 且上下行都有字节才算健康。
func (p *Prober) checkAllWithTemplate(ctx context.Context, cfg Config, res Result, tpl map[string]interface{}, chainAuthority bool) (Result, error) {
	dialer, err := p.pickDialerOn(ctx, cfg.Dialer, cfg.AgentID)
	if err != nil {
		return res, err
	}
	probeURL := strings.TrimSpace(p.ReadChainConfig(ctx).ProbeURL)
	if probeURL == "" {
		probeURL = DefaultChainProbeURL
	}

	type tplItem struct {
		kind string
		id   int64
		prev int
		host string
		port int
	}
	items := make([]tplItem, 0, 256)

	if p.cfRepo != nil {
		entries, err := p.cfRepo.List(ctx, 0)
		if err != nil {
			return res, err
		}
		for _, e := range entries {
			if !e.Enabled || strings.TrimSpace(e.IP) == "" {
				res.Skipped++
				continue
			}
			items = append(items, tplItem{kind: "cf_ip", id: e.ID, prev: e.FailCount,
				host: strings.TrimSpace(e.IP), port: firstPort(e.Ports, 443)})
		}
	}
	if p.shareRepo != nil {
		domains, err := p.shareRepo.List(ctx, "cf_domain")
		if err != nil {
			return res, err
		}
		for _, d := range domains {
			if !d.Enabled || strings.TrimSpace(d.Value) == "" {
				res.Skipped++
				continue
			}
			items = append(items, tplItem{kind: "cf_domain", id: d.ID, prev: d.FailCount,
				host: strings.TrimSpace(d.Value), port: firstPort(d.Ports, d.Port)})
		}
	}

	for start := 0; start < len(items); start += chainTemplateBatchSize {
		end := start + chainTemplateBatchSize
		if end > len(items) {
			end = len(items)
		}
		chunk := items[start:end]
		targets := make([]ChainTarget, len(chunk))
		tplProto := strings.ToLower(proxyString(tpl, "type"))
		for i, it := range chunk {
			targets[i] = ChainTarget{
				Kind: it.kind, EntryID: it.id, PrevFail: it.prev,
				Host: it.host, Port: it.port, Protocol: tplProto,
				Proxy: templateProxy(tpl, it.host, it.port),
			}
		}
		var outs []ChainResult
		if cfg.AgentID > 0 {
			// 在指定节点上执行：配置仍由面板生成，端口写 0 交给 Agent 分配。
			if p.chainRemote == nil {
				return res, errors.New("Agent 执行器未初始化，无法在指定节点上执行")
			}
			conf, cerr := buildChainProbeConfig(dialer, nil, targets, allIndices(len(targets)), make([]int, len(targets)), cfg.Interface)
			if cerr != nil {
				return res, cerr
			}
			rawConf, merr := json.Marshal(conf)
			if merr != nil {
				return res, merr
			}
			items, rerr := p.chainRemote(ctx, ChainRemoteRequest{
				ServerID:    cfg.AgentID,
				Dialer:      dialer,
				ConfigJSON:  string(rawConf),
				ProbeURL:    probeURL,
				TimeoutMs:   int(chainProbeTimeout / time.Millisecond),
				Concurrency: 8,
			})
			if rerr != nil {
				return res, rerr
			}
			if len(items) != len(targets) {
				return res, fmt.Errorf("Agent 返回结果数与目标数不一致（%d != %d）", len(items), len(targets))
			}
			outs = make([]ChainResult, len(targets))
			now := time.Now().Unix()
			for i, t := range targets {
				it := items[i]
				outs[i] = ChainResult{
					Kind: t.Kind, EntryID: t.EntryID, Host: t.Host, Port: t.Port,
					Exit: res.Entry, CheckedAt: now,
					HTTPStatus: it.HTTPStatus, UpBytes: it.UpBytes, DownBytes: it.DownBytes,
					LatencyMs: it.LatencyMs, Healthy: it.Success, Error: it.Error,
				}
			}
		} else {
			dials, cleanup, _, derr := p.startChainDialer(ctx, dialer, nil, targets, cfg.Interface)
			if derr != nil {
				return res, derr
			}
			outs = p.runTemplateProbeLocal(ctx, dials, targets, probeURL, res.Entry)
			cleanup()
		}

		now := time.Now()
		for i, t := range targets {
			r := outs[i]
			if chainAuthority {
				// 链路探测开启时健康由链路探测独占，这里只刷新延迟。
				if t.Kind == "cf_ip" && p.cfRepo != nil {
					_ = p.cfRepo.UpdateLatency(ctx, t.EntryID, r.LatencyMs, now)
				} else if t.Kind == "cf_domain" && p.shareRepo != nil {
					_ = p.shareRepo.UpdateLatency(ctx, t.EntryID, r.LatencyMs, now)
				}
				res.LatencyOnly++
				continue
			}
			healthy, fail := nextState(r.Healthy, t.PrevFail)
			if t.Kind == "cf_ip" && p.cfRepo != nil {
				if err := p.cfRepo.UpdateHealth(ctx, t.EntryID, healthy, r.LatencyMs, fail, now); err != nil {
					log.Printf("[healthprobe] update cf ip %s failed: %v", t.Host, err)
				}
			} else if t.Kind == "cf_domain" && p.shareRepo != nil {
				if err := p.shareRepo.UpdateHealth(ctx, t.EntryID, healthy, r.LatencyMs, fail, now); err != nil {
					log.Printf("[healthprobe] update share domain %s failed: %v", t.Host, err)
				}
			}
			res.Checked++
			if healthy {
				res.Healthy++
			} else {
				res.Unhealthy++
			}
		}
	}
	return res, nil
}

// allIndices 返回 [0,n) 的下标列表。
func allIndices(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

// probeTarget 是一条优选探测目标（CF 优选 IP 或优选域名）。
type probeTarget struct {
	kind string
	id   int64
	prev int
	host string
	port int
}

// collectProbeTargets 读取当前启用的优选 IP / 域名。返回跳过数量。
func (p *Prober) collectProbeTargets(ctx context.Context) ([]probeTarget, int, error) {
	items := make([]probeTarget, 0, 256)
	skipped := 0
	if p.cfRepo != nil {
		entries, err := p.cfRepo.List(ctx, 0)
		if err != nil {
			return nil, skipped, err
		}
		for _, e := range entries {
			if !e.Enabled || strings.TrimSpace(e.IP) == "" {
				skipped++
				continue
			}
			items = append(items, probeTarget{kind: "cf_ip", id: e.ID, prev: e.FailCount,
				host: strings.TrimSpace(e.IP), port: firstPort(e.Ports, 443)})
		}
	}
	if p.shareRepo != nil {
		domains, err := p.shareRepo.List(ctx, "cf_domain")
		if err != nil {
			return nil, skipped, err
		}
		for _, d := range domains {
			if !d.Enabled || strings.TrimSpace(d.Value) == "" {
				skipped++
				continue
			}
			items = append(items, probeTarget{kind: "cf_domain", id: d.ID, prev: d.FailCount,
				host: strings.TrimSpace(d.Value), port: firstPort(d.Ports, d.Port)})
		}
	}
	return items, skipped, nil
}

// updateProbeTarget 把一条探测结论写回库；链路探测开启时只刷新延迟。
func (p *Prober) updateProbeTarget(ctx context.Context, res *Result, t probeTarget, healthy bool, latency int, chainAuthority bool, now time.Time) {
	if chainAuthority {
		if t.kind == "cf_ip" && p.cfRepo != nil {
			_ = p.cfRepo.UpdateLatency(ctx, t.id, latency, now)
		} else if t.kind == "cf_domain" && p.shareRepo != nil {
			_ = p.shareRepo.UpdateLatency(ctx, t.id, latency, now)
		}
		res.LatencyOnly++
		return
	}
	next, fail := nextState(healthy, t.prev)
	if t.kind == "cf_ip" && p.cfRepo != nil {
		if err := p.cfRepo.UpdateHealth(ctx, t.id, next, latency, fail, now); err != nil {
			log.Printf("[healthprobe] update cf ip %s failed: %v", t.host, err)
		}
	} else if t.kind == "cf_domain" && p.shareRepo != nil {
		if err := p.shareRepo.UpdateHealth(ctx, t.id, next, latency, fail, now); err != nil {
			log.Printf("[healthprobe] update share domain %s failed: %v", t.host, err)
		}
	}
	res.Checked++
	if next {
		res.Healthy++
	} else {
		res.Unhealthy++
	}
}

// checkAllExitRemote 在指定 Agent 上做「出口语义」探测：临时核心只有一条出口，
// 优选 IP 做 TCP 建连、优选域名做 TLS 握手。
func (p *Prober) checkAllExitRemote(ctx context.Context, cfg Config, res Result, pm map[string]interface{}, chainAuthority bool) (Result, error) {
	dialer, err := p.pickDialerOn(ctx, cfg.Dialer, cfg.AgentID)
	if err != nil {
		return res, err
	}
	items, skipped, err := p.collectProbeTargets(ctx)
	if err != nil {
		return res, err
	}
	res.Skipped += skipped
	if len(items) == 0 {
		return res, nil
	}
	conf, err := buildDialerOnlyConfig(dialer, pm, cfg.Interface)
	if err != nil {
		return res, err
	}
	rawConf, err := json.Marshal(conf)
	if err != nil {
		return res, err
	}
	targets := make([]ChainRemoteRawTarget, len(items))
	for i, it := range items {
		targets[i] = ChainRemoteRawTarget{Host: it.host, Port: it.port, Mode: "tcp"}
		if it.kind == "cf_domain" {
			targets[i].Mode = "tls"
			targets[i].SNI = it.host
		}
	}
	results, err := p.chainRemote(ctx, ChainRemoteRequest{
		ServerID:    cfg.AgentID,
		Dialer:      dialer,
		ConfigJSON:  string(rawConf),
		TimeoutMs:   int(probeTimeout / time.Millisecond),
		Concurrency: concurrency,
		RawTargets:  targets,
	})
	if err != nil {
		return res, err
	}
	if len(results) != len(items) {
		return res, fmt.Errorf("Agent 返回结果数与目标数不一致（%d != %d）", len(results), len(items))
	}
	now := time.Now()
	for i, it := range items {
		p.updateProbeTarget(ctx, &res, it, results[i].Success, results[i].LatencyMs, chainAuthority, now)
	}
	return res, nil
}

// runTemplateProbeLocal 用本机临时核心逐个目标接口做 HTTP 探测。
func (p *Prober) runTemplateProbeLocal(ctx context.Context, dials []dialFunc, targets []ChainTarget, probeURL, entry string) []ChainResult {
	outs := make([]ChainResult, len(targets))
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for i := range targets {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, t ChainTarget) {
			defer wg.Done()
			defer func() { <-sem }()
			cr := ChainResult{Kind: t.Kind, EntryID: t.EntryID, Host: t.Host, Port: t.Port,
				Exit: entry, CheckedAt: time.Now().Unix()}
			if t.Proxy == nil {
				cr.Error = "入口配置无法复制为客户端出站"
				outs[idx] = cr
				return
			}
			status, up, down, latency, perr := probeChainHTTP(ctx, dials[idx], probeURL)
			cr.HTTPStatus, cr.UpBytes, cr.DownBytes, cr.LatencyMs = status, up, down, latency
			switch {
			case perr != nil:
				cr.Error = truncateErr(perr.Error())
			case status < 200 || status >= 400:
				cr.Error = fmt.Sprintf("HTTP %d", status)
			case up <= 0 || down <= 0:
				cr.Error = "链路无实际流量"
			default:
				cr.Healthy = true
			}
			outs[idx] = cr
		}(i, targets[i])
	}
	wg.Wait()
	return outs
}

// socks5Dial 走 SOCKS5（无认证）连到目标，返回已建立的连接。
func socks5Dial(proxyAddr, target string, timeout time.Duration) (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", proxyAddr, timeout)
	if err != nil {
		return nil, err
	}
	_ = conn.SetDeadline(time.Now().Add(timeout))
	if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		conn.Close()
		return nil, err
	}
	buf := make([]byte, 2)
	if _, err := readFull(conn, buf); err != nil || buf[0] != 0x05 || buf[1] != 0x00 {
		conn.Close()
		return nil, fmt.Errorf("socks5 handshake failed")
	}
	host, portStr, err := net.SplitHostPort(target)
	if err != nil {
		conn.Close()
		return nil, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		conn.Close()
		return nil, err
	}
	req := []byte{0x05, 0x01, 0x00}
	if ip := net.ParseIP(host); ip != nil && ip.To4() != nil {
		req = append(req, 0x01)
		req = append(req, ip.To4()...)
	} else if ip != nil {
		req = append(req, 0x04)
		req = append(req, ip.To16()...)
	} else {
		if len(host) > 255 {
			conn.Close()
			return nil, fmt.Errorf("host too long")
		}
		req = append(req, 0x03, byte(len(host)))
		req = append(req, []byte(host)...)
	}
	req = append(req, byte(port>>8), byte(port))
	if _, err := conn.Write(req); err != nil {
		conn.Close()
		return nil, err
	}
	// 应答：VER REP RSV ATYP [ADDR] PORT
	head := make([]byte, 4)
	if _, err := readFull(conn, head); err != nil || head[1] != 0x00 {
		conn.Close()
		return nil, fmt.Errorf("socks5 connect rejected")
	}
	var skip int
	switch head[3] {
	case 0x01:
		skip = 4 + 2
	case 0x04:
		skip = 16 + 2
	case 0x03:
		l := make([]byte, 1)
		if _, err := readFull(conn, l); err != nil {
			conn.Close()
			return nil, err
		}
		skip = int(l[0]) + 2
	default:
		conn.Close()
		return nil, fmt.Errorf("socks5 bad atyp")
	}
	if _, err := readFull(conn, make([]byte, skip)); err != nil {
		conn.Close()
		return nil, err
	}
	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}

func readFull(conn net.Conn, buf []byte) (int, error) {
	n := 0
	for n < len(buf) {
		m, err := conn.Read(buf[n:])
		n += m
		if err != nil {
			return n, err
		}
	}
	return n, nil
}
