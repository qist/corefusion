package subscriptions

import "testing"

func plainWSConfig() map[string]interface{} {
	return map[string]interface{}{
		"streamSettings": map[string]interface{}{
			"network":  "ws",
			"security": "none",
			"wsSettings": map[string]interface{}{
				"path": "/ws",
			},
		},
	}
}

// CF 优选域名：核心是明文（TLS 在 Cloudflare 终结），分享必须自动补 TLS，
// SNI 取域名本身。以前这里输出 security=none，客户端明文连 443 直接失败。
func TestApplyFrontTLSCFDomainAuto(t *testing.T) {
	cfg := plainWSConfig()
	pools := []shareDomainEntry{{Type: "cf_domain", Host: "cdn.example.com", Port: 443, TLSMode: "auto"}}
	applyFrontTLS(cfg, "cdn.example.com", "cf_domain", pools)

	ss := getMap(cfg, "streamSettings")
	if got := getString(ss, "security", ""); got != "tls" {
		t.Fatalf("security = %q, want tls", got)
	}
	tls := getMap(ss, "tlsSettings")
	if got := getString(tls, "serverName", ""); got != "cdn.example.com" {
		t.Fatalf("SNI = %q, want cdn.example.com", got)
	}
}

// cf_ip：SNI 必须来自条目配置（IP 本身不能当 SNI）。
func TestApplyFrontTLSCFIPUsesConfiguredSNI(t *testing.T) {
	cfg := plainWSConfig()
	pools := []shareDomainEntry{{
		Type: "cf_ip", Host: "1.2.3.4", Port: 443, TLSMode: "auto",
		SNI: "worker.example.com", ALPN: "h2, http/1.1", Fingerprint: "chrome",
	}}
	applyFrontTLS(cfg, "1.2.3.4", "cf_ip", pools)

	ss := getMap(cfg, "streamSettings")
	if got := getString(ss, "security", ""); got != "tls" {
		t.Fatalf("security = %q, want tls", got)
	}
	tls := getMap(ss, "tlsSettings")
	if got := getString(tls, "serverName", ""); got != "worker.example.com" {
		t.Fatalf("SNI = %q, want worker.example.com", got)
	}
	if alpn := getStringSlice(tls, "alpn"); len(alpn) != 2 || alpn[0] != "h2" || alpn[1] != "http/1.1" {
		t.Fatalf("alpn = %v", alpn)
	}
	if fp := tlsFingerprint(tls); fp != "chrome" {
		t.Fatalf("fingerprint = %q", fp)
	}
	if getString(ss, "network", "") != "ws" || getMap(ss, "wsSettings") == nil {
		t.Error("传输参数不能被 TLS 覆盖改动")
	}
}

// direct + auto：不动（可能是核心自己就是明文，也可能是直连真实端口）。
// direct + on：强制 TLS（自建 nginx/caddy 挡在明文入站前）。
// off：任何类型都不改。
func TestApplyFrontTLSModes(t *testing.T) {
	direct := []shareDomainEntry{{Type: "direct", Host: "direct.example.com", Port: 443, TLSMode: "auto"}}
	cfg := plainWSConfig()
	applyFrontTLS(cfg, "direct.example.com", "direct", direct)
	if got := getString(getMap(cfg, "streamSettings"), "security", ""); got != "none" {
		t.Errorf("direct+auto 不应改 security，实际 %q", got)
	}

	on := []shareDomainEntry{{Type: "direct", Host: "front.example.com", Port: 443, TLSMode: "on", SNI: "sni.example.com"}}
	cfg = plainWSConfig()
	applyFrontTLS(cfg, "front.example.com", "direct", on)
	ss := getMap(cfg, "streamSettings")
	if got := getString(ss, "security", ""); got != "tls" {
		t.Errorf("direct+on 应强制 tls，实际 %q", got)
	}
	if got := getString(getMap(ss, "tlsSettings"), "serverName", ""); got != "sni.example.com" {
		t.Errorf("SNI = %q", got)
	}

	off := []shareDomainEntry{{Type: "cf_domain", Host: "cdn.example.com", Port: 443, TLSMode: "off"}}
	cfg = plainWSConfig()
	applyFrontTLS(cfg, "cdn.example.com", "cf_domain", off)
	if got := getString(getMap(cfg, "streamSettings"), "security", ""); got != "none" {
		t.Errorf("off 不应改 security，实际 %q", got)
	}
}

// REALITY 不能被前置 TLS 覆盖（那会把 security 从 reality 改成 tls，握手必失败）。
func TestApplyFrontTLSLeavesRealityAlone(t *testing.T) {
	cfg := map[string]interface{}{"streamSettings": map[string]interface{}{
		"network": "tcp", "security": "reality",
		"realitySettings": map[string]interface{}{"serverNames": []interface{}{"a.com"}},
	}}
	pools := []shareDomainEntry{{Type: "cf_domain", Host: "cdn.example.com", Port: 443, TLSMode: "on"}}
	applyFrontTLS(cfg, "cdn.example.com", "cf_domain", pools)
	ss := getMap(cfg, "streamSettings")
	if got := getString(ss, "security", ""); got != "reality" {
		t.Fatalf("security = %q, want reality", got)
	}
	if getMap(ss, "tlsSettings") != nil {
		t.Error("REALITY 节点不应被写 tlsSettings")
	}
}

// 入口节点访问域名显式开启 TLS：优先于分享域名池，direct/custom 也补 TLS
// （CDN 可能是别家，或自家 nginx/caddy 在前面终结）。
func TestApplyFrontTLSUsesAccessDomainConfig(t *testing.T) {
	cfg := plainWSConfig()
	cfg["_accessDomain"] = map[string]interface{}{
		"enabled": true, "type": "direct", "directDomain": "edge.example.com",
		"tls": map[string]interface{}{
			"enabled": true, "sni": "sni.example.com",
			"alpn": "h2,http/1.1", "fingerprint": "firefox",
		},
	}
	applyFrontTLS(cfg, "edge.example.com", "direct", nil)

	ss := getMap(cfg, "streamSettings")
	if got := getString(ss, "security", ""); got != "tls" {
		t.Fatalf("security = %q, want tls", got)
	}
	tls := getMap(ss, "tlsSettings")
	if got := getString(tls, "serverName", ""); got != "sni.example.com" {
		t.Fatalf("SNI = %q, want sni.example.com", got)
	}
	if alpn := getStringSlice(tls, "alpn"); len(alpn) != 2 {
		t.Fatalf("alpn = %v", alpn)
	}
	if fp := tlsFingerprint(tls); fp != "firefox" {
		t.Fatalf("fingerprint = %q", fp)
	}
}

// 访问域名显式关闭 TLS（跟随核心）：即使是 cf_domain 也不能覆盖。
func TestApplyFrontTLSAccessDomainDisabledWins(t *testing.T) {
	cfg := plainWSConfig()
	cfg["_accessDomain"] = map[string]interface{}{
		"enabled": true, "type": "cf_domain", "cfDomain": "cdn.example.com",
		"tls": map[string]interface{}{"enabled": false},
	}
	pools := []shareDomainEntry{{Type: "cf_domain", Host: "cdn.example.com", Port: 443, TLSMode: "auto"}}
	applyFrontTLS(cfg, "cdn.example.com", "cf_domain", pools)
	if got := getString(getMap(cfg, "streamSettings"), "security", ""); got != "none" {
		t.Fatalf("显式关闭 TLS 时不应覆盖核心配置，实际 %q", got)
	}
}

// 外部节点：访问域名 tls 配置同样优先
func TestApplyFrontTLSToProxyUsesAccessDomainConfig(t *testing.T) {
	pm := map[string]interface{}{"server": "edge.example.com", "port": float64(443)}
	ad := map[string]interface{}{
		"enabled": true, "type": "direct", "directDomain": "edge.example.com",
		"tls": map[string]interface{}{
			"enabled": true, "sni": "sni.example.com", "fingerprint": "chrome",
		},
	}
	applyFrontTLSToProxy(pm, ad, "direct", nil)
	if pm["tls"] != true || pm["sni"] != "sni.example.com" || pm["client-fingerprint"] != "chrome" {
		t.Fatalf("访问域名 TLS 配置未生效: %v", pm)
	}
}
func TestApplyFrontTLSToProxy(t *testing.T) {
	pm := map[string]interface{}{"server": "cdn.example.com", "port": float64(443)}
	pools := []shareDomainEntry{{
		Type: "cf_domain", Host: "cdn.example.com", Port: 443, TLSMode: "auto",
		SNI: "sni.example.com", ALPN: "h2", Fingerprint: "chrome",
	}}
	applyFrontTLSToProxy(pm, nil, "cf_domain", pools)
	if pm["tls"] != true {
		t.Fatal("外部节点应补 tls=true")
	}
	if pm["sni"] != "sni.example.com" || pm["servername"] != "sni.example.com" {
		t.Fatalf("SNI 未写入: %v", pm)
	}
	if pm["client-fingerprint"] != "chrome" {
		t.Fatalf("指纹未写入: %v", pm)
	}
}
