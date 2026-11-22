package healthprobe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"corefusion/internal/runtime/agent"
	"corefusion/internal/service/coreconfig"
)

// 链路探测的临时核心是**两跳**的：
//
//	socks-in（本机）→ 探针入站出站（指向 CDN 域名/IP）→ 出口节点（dialerProxy）
//
// 出口节点只用 `dialerProxy` / `detour` 承接"目标出站到 CDN"这一段的 TCP 连接；
// 真正的协议握手、TLS/REALITY、HTTP 请求都由目标入站完成。这样才有意义：
// 只有入站真的收到连接并放行流量，才说明「CDN 域名/IP → 入站」这条链路可用。
const (
	chainProbeExitTag    = "probe-exit"
	chainProbeTargetFmt  = "probe-target-%d"
	chainProbeInboundFmt = "probe-in-%d"
)

// chainProbeDialerProtocols 是各拨号核心能直接构造出站的目标入站协议。
//
// ⚠ 目标入站不是"随便什么协议都能测"：临时核心必须自己会这个协议的**客户端**。
// xray 没有 hysteria2/tuic/anytls 出站；sing-box 的转换层目前只覆盖
// vless/vmess/trojan/shadowsocks/hysteria2。选错核心时直接报错，避免生成
// 一个核心不认的出站后"整轮全部失败"却不给原因。
var chainProbeDialerProtocols = map[string]map[string]bool{
	"xray": {
		"vless":       true,
		"vmess":       true,
		"trojan":      true,
		"shadowsocks": true,
		"ss":          true,
	},
	"singbox": {
		"vless":       true,
		"vmess":       true,
		"trojan":      true,
		"shadowsocks": true,
		"ss":          true,
		"hysteria2":   true,
	},
}

// buildChainProbeConfig 生成一轮链路探测的临时核心配置。
//
// targets 是完整目标列表，valid 是其中可用（Proxy 非空）的下标，ports 与 valid
// 一一对应：每个目标一个本地 socks 入口，路由到它自己的目标出站；所有目标出站
// 都通过同一个出口出站拨号（`sockopt.dialerProxy` / `detour`）。
func buildChainProbeConfig(
	dialer string,
	exitProxy map[string]interface{},
	targets []ChainTarget,
	valid []int,
	ports []int,
) (map[string]interface{}, error) {
	if len(exitProxy) == 0 {
		return nil, fmt.Errorf("出口节点配置为空，无法建立链式拨号")
	}
	if len(valid) == 0 {
		return nil, fmt.Errorf("没有任何目标具备可用的入站出站参数")
	}
	if len(valid) != len(ports) {
		return nil, fmt.Errorf("内部错误：目标数与本地端口数不一致")
	}
	// 整轮共用同一条探针入站，协议必然一致；逐个校验便于报出具体协议。
	// mihomo 直接吃自己的 proxy map（不做 xray 转换），协议能力由 mihomo 自己保证，
	// 因此不需要这里的白名单；xray/sing-box 必须校验，否则会生成核心不认的出站。
	if dialer != "mihomo" {
		supported := chainProbeDialerProtocols[dialer]
		if supported == nil {
			return nil, fmt.Errorf("拨号核心 %q 暂不支持链路探测（可选 xray / sing-box / mihomo）", dialerDisplayName(dialer))
		}
		for _, idx := range valid {
			p := strings.ToLower(strings.TrimSpace(targets[idx].Protocol))
			if !supported[p] {
				return nil, fmt.Errorf(
					"拨号核心 %s 不支持探针入站协议 %q：请在「探测设置 → 拨号核心」里改用兼容的核心",
					dialerDisplayName(dialer), targets[idx].Protocol)
			}
		}
	}

	switch dialer {
	case "xray":
		return buildChainProbeXrayConfig(exitProxy, targets, valid, ports)
	case "singbox":
		return buildChainProbeSingboxConfig(exitProxy, targets, valid, ports)
	case "mihomo":
		return buildChainProbeMihomoConfig(exitProxy, targets, valid, ports)
	default:
		return nil, fmt.Errorf("拨号核心 %q 暂不支持链路探测（可选 xray / sing-box / mihomo）", dialerDisplayName(dialer))
	}
}

// buildChainProbeXrayConfig 生成 xray 形态的链式探测配置。
func buildChainProbeXrayConfig(exitProxy map[string]interface{}, targets []ChainTarget, valid []int, ports []int) (map[string]interface{}, error) {
	outbounds, err := buildChainProbeXrayOutbounds(exitProxy, targets, valid)
	if err != nil {
		return nil, err
	}

	inbounds := make([]interface{}, 0, len(valid))
	rules := make([]interface{}, 0, len(valid))
	for j := range valid {
		inTag := fmt.Sprintf(chainProbeInboundFmt, j)
		outTag := fmt.Sprintf(chainProbeTargetFmt, j)
		inbounds = append(inbounds, map[string]interface{}{
			"tag":      inTag,
			"listen":   "127.0.0.1",
			"port":     ports[j],
			"protocol": "socks",
			"settings": map[string]interface{}{"auth": "noauth", "udp": false},
		})
		rules = append(rules, map[string]interface{}{
			"type":        "field",
			"inboundTag":  []interface{}{inTag},
			"outboundTag": outTag,
		})
	}

	return map[string]interface{}{
		"log":       map[string]interface{}{"loglevel": "error"},
		"inbounds":  inbounds,
		"outbounds": outbounds,
		"routing": map[string]interface{}{
			"domainStrategy": "AsIs",
			"rules":          rules,
		},
	}, nil
}

// buildChainProbeXrayOutbounds 生成「目标出站（dialerProxy=出口）+ 出口出站」。
// 目标出站排在前面：即使路由规则没命中，也不会把探测流量直接落到出口上。
func buildChainProbeXrayOutbounds(exitProxy map[string]interface{}, targets []ChainTarget, valid []int) ([]interface{}, error) {
	exitOB, err := agent.BuildXrayOutboundFromMihomoProxy(normalizeProxyMap(exitProxy))
	if err != nil {
		return nil, fmt.Errorf("出口节点无法转换为客户端出站: %w", err)
	}
	exitOB["tag"] = chainProbeExitTag

	outbounds := make([]interface{}, 0, len(valid)+1)
	for j, idx := range valid {
		target := targets[idx]
		ob, err := agent.BuildXrayOutboundFromMihomoProxy(normalizeProxyMap(target.Proxy))
		if err != nil {
			return nil, fmt.Errorf("目标 %s:%d 无法转换为客户端出站: %w", target.Host, target.Port, err)
		}
		ob["tag"] = fmt.Sprintf(chainProbeTargetFmt, j)
		attachXrayDialerProxy(ob, chainProbeExitTag)
		outbounds = append(outbounds, ob)
	}
	outbounds = append(outbounds, exitOB)
	return outbounds, nil
}

// attachXrayDialerProxy 让目标出站的底层 TCP 连接先经过出口出站。
func attachXrayDialerProxy(outbound map[string]interface{}, exitTag string) {
	if outbound == nil || exitTag == "" {
		return
	}
	ss, _ := outbound["streamSettings"].(map[string]interface{})
	if ss == nil {
		ss = map[string]interface{}{}
		outbound["streamSettings"] = ss
	}
	sockopt, _ := ss["sockopt"].(map[string]interface{})
	if sockopt == nil {
		sockopt = map[string]interface{}{}
		ss["sockopt"] = sockopt
	}
	sockopt["dialerProxy"] = exitTag
}

// buildChainProbeSingboxConfig 生成 sing-box 形态的链式探测配置。
func buildChainProbeSingboxConfig(exitProxy map[string]interface{}, targets []ChainTarget, valid []int, ports []int) (map[string]interface{}, error) {
	xrayOutbounds, err := buildChainProbeXrayOutbounds(exitProxy, targets, valid)
	if err != nil {
		return nil, err
	}
	// 转换层读的是 xray 出站（settings.vnext 等）。BuildXrayOutboundFromMihomoProxy
	// 在内存里用的是 []map[string]interface{}，而转换层按 JSON 形状断言 []interface{}；
	// 这里先做一次 JSON 往返把嵌套切片归一化，避免服务器地址被静默丢掉。
	normalized, err := normalizeJSONArray(xrayOutbounds)
	if err != nil {
		return nil, fmt.Errorf("归一化出站配置失败: %w", err)
	}
	outs := coreconfig.ConvertXrayOutboundsToSingbox(normalized)
	if len(outs) != len(valid)+1 {
		return nil, fmt.Errorf("转换 sing-box 出站失败（期望 %d 条，实际 %d 条）", len(valid)+1, len(outs))
	}
	for _, item := range outs {
		m, _ := item.(map[string]interface{})
		if m == nil {
			continue
		}
		if tag, _ := m["tag"].(string); tag != "" && tag != chainProbeExitTag {
			// sing-box 的链式写法：目标出站的连接经 detour 指定的出口拨号。
			m["detour"] = chainProbeExitTag
		}
	}

	inbounds := make([]interface{}, 0, len(valid))
	rules := make([]interface{}, 0, len(valid))
	for j := range valid {
		inTag := fmt.Sprintf(chainProbeInboundFmt, j)
		outTag := fmt.Sprintf(chainProbeTargetFmt, j)
		inbounds = append(inbounds, map[string]interface{}{
			"type":        "socks",
			"tag":         inTag,
			"listen":      "127.0.0.1",
			"listen_port": ports[j],
		})
		rules = append(rules, map[string]interface{}{
			"inbound":  []interface{}{inTag},
			"outbound": outTag,
		})
	}

	return map[string]interface{}{
		"log":       map[string]interface{}{"level": "error"},
		"inbounds":  inbounds,
		"outbounds": outs,
		"route": map[string]interface{}{
			"rules": rules,
			"final": chainProbeExitTag,
		},
	}, nil
}

// buildChainProbeMihomoConfig 生成 mihomo 形态的链式探测配置。
//
// mihomo 用的是「每个 socks listener 指定 proxy」的原生能力，链式关系写在该
// proxy 的 `dialer-proxy` 上 —— 不需要像 xray 那样额外做路由规则/出站转换。
func buildChainProbeMihomoConfig(exitProxy map[string]interface{}, targets []ChainTarget, valid []int, ports []int) (map[string]interface{}, error) {
	exit := normalizeProxyMap(exitProxy)
	if exit == nil {
		return nil, fmt.Errorf("出口节点配置为空，无法建立链式拨号")
	}
	exit["name"] = chainProbeExitTag

	proxies := make([]interface{}, 0, len(valid)+1)
	listeners := make([]interface{}, 0, len(valid))
	for j, idx := range valid {
		pm := normalizeProxyMap(targets[idx].Proxy)
		if pm == nil {
			return nil, fmt.Errorf("目标 %s:%d 的出站参数为空", targets[idx].Host, targets[idx].Port)
		}
		tag := fmt.Sprintf(chainProbeTargetFmt, j)
		pm["name"] = tag
		// mihomo 的链式写法：目标节点的连接经 dialer-proxy 指定的出口拨号。
		pm["dialer-proxy"] = chainProbeExitTag
		proxies = append(proxies, pm)
		listeners = append(listeners, map[string]interface{}{
			"name":   fmt.Sprintf(chainProbeInboundFmt, j),
			"type":   "socks",
			"port":   ports[j],
			"listen": "127.0.0.1",
			"proxy":  tag,
		})
	}
	proxies = append(proxies, exit)

	return map[string]interface{}{
		"log-level": "error",
		"mode":      "rule",
		"proxies":   proxies,
		"listeners": listeners,
	}, nil
}

// normalizeProxyMap 把内存里构造的 mihomo proxy map 归一化成 JSON 形状。
// 链接转换器直接产出 `alpn: []string`、`host: []string` 之类的具体切片，
// 而出站转换层按 JSON 反序列化的 `[]interface{}` 断言；不归一会静默丢掉
// ALPN/Host（gRPC over TLS 需要 ALPN=h2，丢了就会握手失败）。
func normalizeProxyMap(pm map[string]interface{}) map[string]interface{} {
	if pm == nil {
		return nil
	}
	raw, err := json.Marshal(pm)
	if err != nil {
		return pm
	}
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return pm
	}
	return out
}

// normalizeJSONArray 把带具体切片类型的结构转成纯 JSON 形状（[]interface{}）。
func normalizeJSONArray(v interface{}) ([]interface{}, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out []interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// startChainDialer 起一个临时核心，为每个目标提供一个本地 SOCKS 拨号入口。
// 返回的 dials 与 targets 下标一一对齐；无法渲染的目标会得到一个始终报错的
// 拨号函数（探测结果里能看到具体原因，不会拖垮其他目标）。
func (p *Prober) startChainDialer(
	ctx context.Context,
	dialer string,
	exitProxy map[string]interface{},
	targets []ChainTarget,
) ([]dialFunc, func(), string, error) {
	noop := func() {}
	if len(exitProxy) == 0 {
		return nil, noop, "", fmt.Errorf("出口节点配置为空，无法建立链式拨号")
	}

	valid := make([]int, 0, len(targets))
	for i := range targets {
		if targets[i].Proxy != nil {
			valid = append(valid, i)
		}
	}
	if len(valid) == 0 {
		return nil, noop, "", fmt.Errorf("没有任何目标具备可用的入站出站参数")
	}

	ports, err := uniqueFreePorts(len(valid))
	if err != nil {
		return nil, noop, "", err
	}
	conf, err := buildChainProbeConfig(dialer, exitProxy, targets, valid, ports)
	if err != nil {
		return nil, noop, "", err
	}

	binPath, err := p.resolveBinary(dialer)
	if err != nil {
		return nil, noop, "", err
	}
	dir := filepath.Join(p.dataDir, "probe", fmt.Sprintf("run-%d", time.Now().UnixNano()))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, noop, "", err
	}
	cleanupDir := func() { _ = os.RemoveAll(dir) }
	cfgPath := filepath.Join(dir, "config.json")
	if err := writeJSON(cfgPath, conf); err != nil {
		cleanupDir()
		return nil, noop, "", err
	}

	args := []string{"run", "-c", cfgPath}
	if dialer == "mihomo" {
		// mihomo 的配置是 YAML/JSON，启动参数是 -f/-d，不是 sing-box/xray 的 run -c。
		args = []string{"-f", cfgPath, "-d", dir}
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

	addrs := make([]string, len(valid))
	for j := range valid {
		addrs[j] = net.JoinHostPort("127.0.0.1", strconv.Itoa(ports[j]))
	}
	if !waitPorts(addrs, 8*time.Second) {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		msg := strings.TrimSpace(coreOut.String())
		cleanupDir()
		if msg == "" {
			msg = "无输出"
		}
		return nil, noop, "", fmt.Errorf("%s 未能在 8 秒内打开本地 SOCKS 端口：%s", dialerDisplayName(dialer), truncateErr(msg))
	}

	dials := make([]dialFunc, len(targets))
	for j, idx := range valid {
		socksAddr := addrs[j]
		dials[idx] = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return socks5Dial(socksAddr, addr, probeTimeout)
		}
	}
	for i := range targets {
		if dials[i] == nil {
			dials[i] = func(ctx context.Context, network, addr string) (net.Conn, error) {
				return nil, fmt.Errorf("入站参数无法渲染为客户端出站")
			}
		}
	}

	cleanup := func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		cleanupDir()
	}
	return dials, cleanup, dialer, nil
}

// uniqueFreePorts 一次性占用多个互不重复的空闲端口。
// freePort 会先关掉监听再返回，连续调用可能拿到同一个端口，必须去重。
func uniqueFreePorts(n int) ([]int, error) {
	seen := make(map[int]struct{}, n)
	out := make([]int, 0, n)
	for len(out) < n {
		port, err := freePort()
		if err != nil {
			return nil, err
		}
		if _, ok := seen[port]; ok {
			continue
		}
		seen[port] = struct{}{}
		out = append(out, port)
	}
	return out, nil
}

// waitPorts 等待全部本地端口可用；任何一个没起来都算失败（配置/核心报错）。
func waitPorts(addrs []string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for _, addr := range addrs {
		left := time.Until(deadline)
		if left <= 0 || !waitPort(addr, left) {
			return false
		}
	}
	return true
}
