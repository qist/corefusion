package subscriptions

import (
	"net/url"
	"strings"
	"testing"

	"corefusion/internal/model/entity"
	"corefusion/internal/subscription/formatter"
	"corefusion/internal/subscription/mihomoconv"
)

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

func TestExternalNodeWorkerLinkKeepsSNIAndFingerprint(t *testing.T) {
	pm := map[string]interface{}{
		"name": "maimai.boown.com", "type": "vless", "server": "bestcf.030101.xyz", "port": float64(443),
		"uuid": "d094c691-d4ec-4dd1-b935-83a764484716", "tls": true,
		"sni": "maimai.boown.com", "servername": "maimai.boown.com", "client-fingerprint": "random",
		"network": "ws",
		"ws-opts": map[string]interface{}{
			"path": "/?ed=2560", "headers": map[string]interface{}{"Host": "maimai.boown.com"},
		},
	}
	if got := pm["sni"]; got != "maimai.boown.com" {
		t.Fatalf("SNI 被覆盖: %v", got)
	}
	if got := pm["client-fingerprint"]; got != "random" {
		t.Fatalf("指纹被覆盖: %v", got)
	}

	link := formatter.MihomoToURI(pm)
	u, err := url.Parse(link)
	if err != nil {
		t.Fatalf("parse link: %v", err)
	}
	q := u.Query()
	if u.Hostname() != "bestcf.030101.xyz" {
		t.Errorf("server = %q, want 优选域名", u.Hostname())
	}
	if got := q.Get("sni"); got != "maimai.boown.com" {
		t.Errorf("链接 sni = %q, want maimai.boown.com", got)
	}
	if got := q.Get("host"); got != "maimai.boown.com" {
		t.Errorf("链接 host = %q, want maimai.boown.com", got)
	}
	if got := q.Get("fp"); got != "random" {
		t.Errorf("链接 fp = %q, want random", got)
	}
	if got := q.Get("path"); got != "/?ed=2560" {
		t.Errorf("链接 path = %q", got)
	}
	if got := q.Get("ed"); got != "" {
		t.Errorf("path 已带 ?ed=2560 时不应再输出独立 ed 参数，实际 %q", got)
	}
}

// cf_ip 池里配置了 SNI（如 time.is）时，导入节点的 SNI 同样不能被覆盖。
func TestExternalNodeTrojanWorkerKeepsImportedSNI(t *testing.T) {
	pm := map[string]interface{}{
		"name": "sweet-bonus", "type": "trojan", "server": "time.is", "port": float64(443),
		"password": "5wjtwj7qlkzlpzkx81f1jpyi15tmerun", "tls": true,
		"sni": "sweet-bonus-aeb7.boown.com", "servername": "sweet-bonus-aeb7.boown.com",
		"client-fingerprint": "random", "network": "ws",
		"ws-opts": map[string]interface{}{
			"path": "/?ed=2560", "headers": map[string]interface{}{"Host": "sweet-bonus-aeb7.boown.com"},
		},
	}
	if got := pm["sni"]; got != "sweet-bonus-aeb7.boown.com" {
		t.Fatalf("SNI 被覆盖: %v", got)
	}
	link := formatter.MihomoToURI(pm)
	if !strings.Contains(link, "sni=sweet-bonus-aeb7.boown.com") {
		t.Fatalf("链接缺少原 SNI: %s", link)
	}
}

// vless 导入时只写 servername（没有 sni）：生成链接必须用 servername 作为 SNI，
// 不能被 CF 池的域名覆盖。生产数据实测形状。
func TestExternalNodeVLESSOnlyServernameKept(t *testing.T) {
	pm := map[string]interface{}{
		"type": "vless", "server": "cf.877774.xyz", "port": float64(443),
		"uuid": "d094c691-d4ec-4dd1-b935-83a764484716", "tls": true,
		"servername": "maimai.boown.com", "client-fingerprint": "random", "network": "ws",
		"ws-opts": map[string]interface{}{
			"path": "/?ed=2560", "max-early-data": 2560,
			"early-data-header-name": "Sec-WebSocket-Protocol",
			"headers":                map[string]interface{}{"Host": "maimai.boown.com"},
		},
	}
	link := formatter.MihomoToURI(pm)
	t.Logf("link = %s", link)
	u, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if got := q.Get("sni"); got != "maimai.boown.com" {
		t.Fatalf("sni = %q, want maimai.boown.com", got)
	}
	if got := q.Get("path"); got != "/?ed=2560" {
		t.Errorf("path = %q, want /?ed=2560", got)
	}
	if got := q.Get("ed"); got != "" {
		t.Errorf("不应额外输出 ed 参数（path 已带 ed），实际 %q", got)
	}
	if got := q.Get("fp"); got != "random" {
		t.Errorf("fp = %q, want random", got)
	}
}

// 端到端：原始 worker 链接 → 导入（ConvertsV2Ray）→ 访问域名只换 server/port →
// 生成链接。除 server 外所有字段必须与原始完全一致（这是用户要的「保留原状」）。
func TestExternalNodeImportRoundTripKeepsOriginal(t *testing.T) {
	cases := []struct {
		name string
		uri  string
		sni  string
		host string
	}{
		{
			name: "trojan",
			uri: "trojan://5wjtwj7qlkzlpzkx81f1jpyi15tmerun@sweet-bonus-aeb7.boown.com:443" +
				"?security=tls&type=ws&host=sweet-bonus-aeb7.boown.com&sni=sweet-bonus-aeb7.boown.com" +
				"&fp=random&path=%2F%3Fed%3D2560#sweet-bonus-aeb7.boown.com",
			sni:  "sweet-bonus-aeb7.boown.com",
			host: "sweet-bonus-aeb7.boown.com",
		},
		{
			name: "vless",
			uri: "vless://d094c691-d4ec-4dd1-b935-83a764484716@maimai.boown.com:443" +
				"?encryption=none&security=tls&type=ws&host=maimai.boown.com&sni=maimai.boown.com" +
				"&fp=random&path=%2F%3Fed%3D2560#maimai.boown.com",
			sni:  "maimai.boown.com",
			host: "maimai.boown.com",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			proxies, err := mihomoconv.ConvertsV2Ray([]byte(c.uri))
			if err != nil || len(proxies) == 0 {
				t.Fatalf("导入失败: %v", err)
			}
			pm := proxies[0]

			n := &entity.ExternalNode{Port: 443}
			n.SetAccessDomain(map[string]interface{}{
				"enabled": true, "type": "cf_domain", "cfDomain": "bestcf.030101.xyz",
			})
			applyExternalNodeAccessDomain(pm, n, nil)

			link := formatter.MihomoToURI(pm)
			t.Logf("link = %s", link)
			u, err := url.Parse(link)
			if err != nil {
				t.Fatal(err)
			}
			q := u.Query()
			if u.Hostname() != "bestcf.030101.xyz" {
				t.Errorf("server 应替换为访问域名，实际 %q", u.Hostname())
			}
			if got := q.Get("sni"); got != c.sni {
				t.Errorf("sni = %q, want %q", got, c.sni)
			}
			if got := q.Get("host"); got != c.host {
				t.Errorf("host = %q, want %q", got, c.host)
			}
			if got := q.Get("fp"); got != "random" {
				t.Errorf("fp = %q, want random", got)
			}
			if got := q.Get("path"); got != "/?ed=2560" {
				t.Errorf("path = %q, want /?ed=2560", got)
			}
			if got := q.Get("ed"); got != "" {
				t.Errorf("不应额外输出 ed 参数，实际 %q", got)
			}
		})
	}
}
