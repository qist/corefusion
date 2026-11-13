package proxy

import (
	"encoding/json"
	"fmt"
	"strings"

	"corefusion/internal/model/entity"
)

// derivedSNIRule 是一条从入站派生出来的 SNI 转发规则（**不落库**，只在生成
// nginx/caddy 配置时合并进站点列表）。
type derivedSNIRule struct {
	Domain        string
	Target        string
	ProxyProtocol bool
	// FromSingbox 标记该规则来自 sing-box 入站（核心不支持 PROXY 协议）。
	// nginx 的 443 分流若全局开了 proxy_protocol，就只能跳过这类规则：
	// 导到一个必然握手失败的后端，比落到 fallback 更糟。
	FromSingbox bool
}

// SNIRulesFromInbound 从「被前置 SNI 代理承载的 REALITY 入站」派生 SNI 规则。
//
// 为什么必须派生：REALITY 的 TLS 握手在 xray 里完成，nginx/caddy 无法解密，
// 只能在 443 按 ClientHello 的 SNI 做 **TCP 分流**。所以入站
// `realitySettings.serverNames` 里的**每一个**域名都必须在 SNI map 里有对应条目，
// 否则客户端随机挑到未映射的域名就会落到 `default`（通常是 fallback 入站），
// 表现为「同一个节点时通时不通」——线上 `aiml.apple.com` 缺映射就是这么炸的。
//
// 只对**需要前置代理**的入站派生：
//   - listen 是 unix socket（`/…`）或抽象 socket（`@…`）：客户端根本连不到；
//   - 或 `sockopt.acceptProxyProtocol=true`：必须由 nginx/caddy 补 PROXY 头。
//
// 直连监听 `0.0.0.0:port`（且不要求 PROXY 头）的入站客户端能直接连，
// 不该被 nginx 抢走。
func SNIRulesFromInbound(cfg entity.InboundConfig) []derivedSNIRule {
	if !cfg.Enabled {
		return nil
	}
	var doc map[string]interface{}
	if err := json.Unmarshal([]byte(cfg.ConfigJSON), &doc); err != nil {
		return nil
	}
	ss, _ := doc["streamSettings"].(map[string]interface{})
	if ss == nil {
		// sing-box 入站没有 streamSettings，REALITY 在顶层 type + tls.reality。
		if _, isSingbox := doc["type"]; isSingbox {
			return singboxRealitySNIRules(cfg, doc)
		}
		return nil
	}
	if sec, _ := ss["security"].(string); !strings.EqualFold(sec, "reality") {
		return nil
	}
	reality, _ := ss["realitySettings"].(map[string]interface{})
	if reality == nil {
		return nil
	}
	names := jsonStringList(reality["serverNames"])
	if len(names) == 0 {
		return nil
	}
	target, needsFront := inboundSNITarget(cfg, ss)
	if !needsFront || target == "" {
		return nil
	}
	proxyProtocol := inboundNeedsProxyProtocol(ss)

	seen := make(map[string]bool, len(names))
	out := make([]derivedSNIRule, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		// "" 是「允许无 SNI」的服务端标记，不是可映射的域名。
		if name == "" || strings.EqualFold(name, "fallback") {
			continue
		}
		key := strings.ToLower(name)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, derivedSNIRule{Domain: name, Target: target, ProxyProtocol: proxyProtocol})
	}
	return out
}

// singboxRealitySNIRules 从 sing-box REALITY 入站派生 SNI 规则。
//
// sing-box 的 REALITY 同样在核心内完成 TLS 握手，前置代理只能按 SNI 做 TCP
// 分流；它的服务名是单个 `tls.server_name`（不是 xray 的 serverNames 列表）。
//
// 与 xray 的差别：xray 只在「必须走前置」（unix socket / acceptProxyProtocol）
// 时派生，而 sing-box 完全没有 proxy_protocol 能力，只要它在直连端口监听，
// 443 上的同名 SNI 就该转到这个端口，客户端才能「统一用 443」访问。
// 生成的规则一律 ProxyProtocol=false；节点 nginx 若全局开了 proxy_protocol，
// 调用方会跳过这些规则（见 NginxManager.withDerivedSNISites）。
func singboxRealitySNIRules(cfg entity.InboundConfig, doc map[string]interface{}) []derivedSNIRule {
	tlsCfg, _ := doc["tls"].(map[string]interface{})
	if tlsCfg == nil {
		return nil
	}
	if enabled, _ := tlsCfg["enabled"].(bool); !enabled {
		return nil
	}
	reality, _ := tlsCfg["reality"].(map[string]interface{})
	if reality == nil {
		return nil
	}
	if enabled, _ := reality["enabled"].(bool); !enabled {
		return nil
	}
	// 协议不支持 REALITY 时（前端会拦住）配置里可能只是残留字段，跳过。
	if proto, _ := doc["type"].(string); !singboxRealityProtocol(proto) {
		return nil
	}
	name, _ := tlsCfg["server_name"].(string)
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	target := singboxListenTarget(cfg)
	if target == "" {
		return nil
	}
	return []derivedSNIRule{{
		Domain:        name,
		Target:        target,
		ProxyProtocol: false,
		FromSingbox:   true,
	}}
}

// singboxRealityProtocol 返回该 sing-box 入站协议是否支持 REALITY 服务端。
// 与前端 supportsReality 一致：仅 vless / vmess / trojan（传输另有限制，
// 这里不参与 SNI 派生判断）。
func singboxRealityProtocol(proto string) bool {
	switch strings.ToLower(strings.TrimSpace(proto)) {
	case "vless", "vmess", "trojan":
		return true
	}
	return false
}

// singboxListenTarget 返回 sing-box 入站对前置代理暴露的后端地址：
// unix / 抽象 socket 用 `unix:` 写法（nginx 可直接用），TCP 监听用 127.0.0.1:port。
func singboxListenTarget(cfg entity.InboundConfig) string {
	listen := strings.TrimSpace(cfg.Listen)
	if strings.HasPrefix(listen, "/") {
		path := strings.SplitN(listen, ",", 2)[0]
		if path == "" {
			return ""
		}
		return "unix:" + path
	}
	if strings.HasPrefix(listen, "@") {
		return "unix:" + listen
	}
	if cfg.Port <= 0 {
		return ""
	}
	host := listen
	switch host {
	case "", "0.0.0.0", "::", "[::]":
		host = "127.0.0.1"
	}
	return fmt.Sprintf("%s:%d", host, cfg.Port)
}

// inboundSNITarget 返回该入站对 SNI 前置代理暴露的后端地址，以及是否真的需要前置代理。
func inboundSNITarget(cfg entity.InboundConfig, ss map[string]interface{}) (string, bool) {
	listen := strings.TrimSpace(cfg.Listen)
	// unix socket：DB 里存 "/dev/shm/x.sock,0666"，nginx 写 "unix:/dev/shm/x.sock"。
	if strings.HasPrefix(listen, "/") {
		path := strings.SplitN(listen, ",", 2)[0]
		if path == "" {
			return "", false
		}
		return "unix:" + path, true
	}
	// 抽象 socket：nginx 需要 "unix:@name"。
	if strings.HasPrefix(listen, "@") {
		return "unix:" + listen, true
	}
	if !inboundNeedsProxyProtocol(ss) {
		return "", false
	}
	if cfg.Port <= 0 {
		return "", false
	}
	host := listen
	switch host {
	case "", "0.0.0.0", "::", "[::]":
		host = "127.0.0.1"
	}
	return fmt.Sprintf("%s:%d", host, cfg.Port), true
}

// inboundNeedsProxyProtocol 报告该入站是否要求前端补 PROXY 协议头。
func inboundNeedsProxyProtocol(ss map[string]interface{}) bool {
	if ss == nil {
		return false
	}
	sockopt, _ := ss["sockopt"].(map[string]interface{})
	if sockopt == nil {
		return false
	}
	v, _ := sockopt["acceptProxyProtocol"].(bool)
	return v
}

// jsonStringList 读取 ["a","b"] / []interface{} 两种形态的字符串列表。
func jsonStringList(v interface{}) []string {
	switch list := v.(type) {
	case []string:
		return list
	case []interface{}:
		out := make([]string, 0, len(list))
		for _, raw := range list {
			if s, ok := raw.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// mergeDerivedNginxSNISites 把派生规则追加成 SNI 站点；**已存在的同域名显式站点优先**
// （用户可能在「Nginx 代理」里手工指定了别的 sni_target / local_port，不能覆盖）。
func mergeDerivedNginxSNISites(serverID int64, existing []entity.NginxSite, rules []derivedSNIRule) []entity.NginxSite {
	if len(rules) == 0 {
		return existing
	}
	seen := make(map[string]bool, len(existing)+len(rules))
	for _, s := range existing {
		seen[strings.ToLower(strings.TrimSpace(s.Domain))] = true
	}
	out := existing
	for _, r := range rules {
		key := strings.ToLower(r.Domain)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, entity.NginxSite{
			ServerID:      serverID,
			Domain:        r.Domain,
			Template:      "sni",
			SNITarget:     r.Target,
			ProxyProtocol: r.ProxyProtocol,
			L4Protocol:    "tcp",
			Priority:      100,
			Enabled:       true,
		})
	}
	return out
}

// mergeDerivedCaddySNISites 同 nginx 版本（Caddy 的 SNI 规则字段一致）。
func mergeDerivedCaddySNISites(serverID int64, existing []entity.CaddySite, rules []derivedSNIRule) []entity.CaddySite {
	if len(rules) == 0 {
		return existing
	}
	seen := make(map[string]bool, len(existing)+len(rules))
	for _, s := range existing {
		seen[strings.ToLower(strings.TrimSpace(s.Domain))] = true
	}
	out := existing
	for _, r := range rules {
		key := strings.ToLower(r.Domain)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, entity.CaddySite{
			ServerID:      serverID,
			Domain:        r.Domain,
			Template:      "sni",
			SNITarget:     r.Target,
			ProxyProtocol: r.ProxyProtocol,
			L4Protocol:    "tcp",
			Priority:      100,
			Enabled:       true,
		})
	}
	return out
}

// caddyProxyTarget 把内部统一的 "unix:/path" 写法转成 Caddy layer4 的写法。
//
// nginx 的 unix socket 是 `unix:/dev/shm/x.sock`（冒号），Caddy 是
// `unix//dev/shm/x.sock`（`unix/` 前缀 + 绝对路径，所以是双斜杠）。
// 直接把 nginx 写法喂给 Caddy 会被当成主机名解析，运行时连接失败
// （配置能加载、但流量永远到不了后端）。
func caddyProxyTarget(target string) string {
	if strings.HasPrefix(target, "unix:") {
		return "unix/" + strings.TrimPrefix(target, "unix:")
	}
	return target
}
