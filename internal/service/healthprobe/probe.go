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
	"context"
	"crypto/tls"
	"encoding/json"
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
	SettingInterval   = "cfip.probe.interval_minutes"
	SettingMaxLatency = "cfip.probe.max_latency_ms"
	SettingEntryNode  = "cfip.probe.entry_node_id"
	SettingDialer     = "cfip.probe.dialer"

	DefaultIntervalMinutes = 5
	DefaultMaxLatencyMs    = 0  // 0 = 不限
	DefaultDialer          = "" // 空 = 自动选择已安装的核心（sing-box → xray → mihomo）
)

// Config 是一次探测的运行时配置（每次探测前从设置里读，改完立即生效）。
type Config struct {
	IntervalMinutes int    `json:"interval_minutes"`
	MaxLatencyMs    int    `json:"max_latency_ms"`
	EntryNodeID     int64  `json:"entry_node_id"`
	Dialer          string `json:"dialer"`
}

// Result 是一次全量探测的汇总（供接口返回/日志）。
type Result struct {
	Checked   int    `json:"checked"`
	Healthy   int    `json:"healthy"`
	Unhealthy int    `json:"unhealthy"`
	Skipped   int    `json:"skipped"`
	Entry     string `json:"entry"`
	Interval  int    `json:"interval_minutes"`
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
	}
}

// ReadConfig 读取当前探测配置（缺失项回落默认值）。
func (p *Prober) ReadConfig(ctx context.Context) Config {
	cfg := Config{
		IntervalMinutes: DefaultIntervalMinutes,
		MaxLatencyMs:    DefaultMaxLatencyMs,
		Dialer:          DefaultDialer,
	}
	if p.setting == nil {
		return cfg
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
	if v, err := p.setting.GetByKey(ctx, SettingEntryNode); err == nil {
		if n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil && n > 0 {
			cfg.EntryNodeID = n
		}
	}
	if v, err := p.setting.GetByKey(ctx, SettingDialer); err == nil {
		cfg.Dialer = strings.TrimSpace(v)
	}
	return cfg
}

// dialerCandidates 是自动模式下的优先顺序。
var dialerCandidates = []string{"singbox", "xray", "mihomo"}

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

// SaveConfig 保存探测配置（nil 表示不改该项）。
func (p *Prober) SaveConfig(ctx context.Context, intervalMinutes, maxLatencyMs *int, entryNodeID *int64, dialer *string) error {
	if p.setting == nil {
		return fmt.Errorf("settings repository not available")
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
	if entryNodeID != nil {
		if err := p.setting.Set(ctx, SettingEntryNode, strconv.FormatInt(*entryNodeID, 10)); err != nil {
			return err
		}
	}
	if dialer != nil {
		if err := p.setting.Set(ctx, SettingDialer, strings.TrimSpace(*dialer)); err != nil {
			return err
		}
	}
	return nil
}

// StartScheduler 周期性探测：每轮先读配置，间隔改完下一轮就生效。
func (p *Prober) StartScheduler(ctx context.Context) {
	go func() {
		for {
			cfg := p.ReadConfig(ctx)
			res, err := p.CheckAll(ctx)
			if err != nil {
				log.Printf("[healthprobe] check failed: %v", err)
			} else {
				log.Printf("[healthprobe] checked=%d healthy=%d unhealthy=%d skipped=%d entry=%s interval=%dm",
					res.Checked, res.Healthy, res.Unhealthy, res.Skipped, res.Entry, res.Interval)
			}
			interval := time.Duration(cfg.IntervalMinutes) * time.Minute
			if interval < time.Minute {
				interval = time.Minute
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(interval):
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

	// 入口节点：起临时核心做拨号。
	// ⚠ 明确选了入口节点就必须用它：核心未安装/节点无效时直接报错提示，
	//   不能静默退回本机 —— 否则用户以为在按节点线路探测，实际还是服务器直连，结论失真 ✗。
	var dial dialFunc
	cleanup := func() {}
	if cfg.EntryNodeID > 0 && p.nodeRepo != nil {
		d, fn, usedDialer, err := p.startDialer(ctx, cfg)
		if err != nil {
			return res, err
		}
		dial, cleanup = d, fn
		res.Entry = fmt.Sprintf("node:%d/%s", cfg.EntryNodeID, usedDialer)
	}
	defer cleanup()

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

// startDialer 起一个临时的 sing-box / mihomo / xray，用外部导入节点做 SOCKS 拨号。
// 返回 dial、清理函数与实际使用的核心名。
//
// 口径：**选了哪个核心就必须用哪个**，未安装直接报错提示安装，不静默退回本机；
// 未选择（dialer 为空）时按 sing-box → xray → mihomo 顺序取第一个已安装的。
func (p *Prober) startDialer(ctx context.Context, cfg Config) (dialFunc, func(), string, error) {
	noop := func() {}
	dialer := strings.ToLower(strings.TrimSpace(cfg.Dialer))
	if dialer == "clash" {
		dialer = "mihomo"
	}
	if dialer == "v2ray" {
		dialer = "xray"
	}
	if dialer == "" {
		dialer = firstAvailableDialer(p.installedDialers())
		if dialer == "" {
			return nil, noop, "", fmt.Errorf("未安装任何可用拨号核心（sing-box / xray / mihomo），请先在「二进制管理」中安装后再探测")
		}
	}
	switch dialer {
	case "singbox", "mihomo", "xray":
	default:
		return nil, noop, "", fmt.Errorf("不支持的拨号核心 %q（可选 sing-box / mihomo / xray）", cfg.Dialer)
	}

	node, err := p.nodeRepo.GetByID(ctx, cfg.EntryNodeID)
	if err != nil || node == nil {
		return nil, noop, "", fmt.Errorf("外部节点 %d 不存在或不可用：%v", cfg.EntryNodeID, err)
	}
	var proxy map[string]interface{}
	if err := json.Unmarshal([]byte(node.ProxyJSON), &proxy); err != nil || len(proxy) == 0 {
		return nil, noop, "", fmt.Errorf("外部节点 %d 的 proxy_json 无效", cfg.EntryNodeID)
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
		if err := writeJSON(cfgPath, conf); err != nil {
			cleanupDir()
			return nil, noop, "", err
		}
		args = []string{"run", "-c", cfgPath}
	}

	cmd := exec.Command(binPath, args...)
	cmd.Dir = dir
	if err := cmd.Start(); err != nil {
		cleanupDir()
		return nil, noop, "", fmt.Errorf("启动 %s 失败：%w", dialerDisplayName(dialer), err)
	}
	proxyAddr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	if !waitPort(proxyAddr, 6*time.Second) {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		cleanupDir()
		return nil, noop, "", fmt.Errorf("%s 未能在 6 秒内打开本地 SOCKS 端口（节点配置可能无效）", dialerDisplayName(dialer))
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
