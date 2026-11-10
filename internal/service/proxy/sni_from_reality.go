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
