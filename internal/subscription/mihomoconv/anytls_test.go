package mihomoconv

import "testing"

// anytls:// 有官方 URI 规范（anytls/anytls-go 的 docs/uri_scheme.md），
// Shadowrocket 能直接导入；转到 mihomo 时必须映射成 type: anytls 的代理，
// 否则 mihomo 订阅里这个节点会丢。
func TestConvertsAnyTLSURI(t *testing.T) {
	uri := "anytls://P%40ssw0rd@example.com:8443/?alpn=h2%2Chttp%2F1.1&fp=chrome&insecure=1&sni=real.example.com#my%20node"
	proxies, err := ConvertsV2Ray([]byte(uri))
	if err != nil {
		t.Fatal(err)
	}
	if len(proxies) != 1 {
		t.Fatalf("应转出 1 个代理, got %d", len(proxies))
	}
	p := proxies[0]
	checks := map[string]interface{}{
		"type":               "anytls",
		"server":             "example.com",
		"port":               "8443",
		"password":           "P@ssw0rd",
		"sni":                "real.example.com",
		"skip-cert-verify":   true,
		"client-fingerprint": "chrome",
		"name":               "my node",
	}
	for k, want := range checks {
		if p[k] != want {
			t.Errorf("%s = %v, want %v", k, p[k], want)
		}
	}
	alpn, _ := p["alpn"].([]string)
	if len(alpn) != 2 || alpn[0] != "h2" {
		t.Errorf("alpn 转换不对: %v", p["alpn"])
	}
}

// socks/http：面板生成的是明文 user:pass（用户名/密码分别转义），转换后必须是
// **解码后**的原值（此前用 User.String() 会把 user%40x.com 原样留在配置里 → 认证失败）。
func TestConvertsSocksURIWithEscapedCredentials(t *testing.T) {
	uri := "socks://user%40x.com:p%23ss%20word@example.com:1080?security=none#socks"
	proxies, err := ConvertsV2Ray([]byte(uri))
	if err != nil {
		t.Fatal(err)
	}
	if len(proxies) != 1 {
		t.Fatalf("应转出 1 个代理, got %d", len(proxies))
	}
	p := proxies[0]
	if p["type"] != "socks5" {
		t.Errorf("type = %v, want socks5", p["type"])
	}
	if p["username"] != "user@x.com" || p["password"] != "p#ss word" {
		t.Errorf("凭据未解码: username=%v password=%v", p["username"], p["password"])
	}
}

// hysteria2 的密码在面板里常是 base64（含 + / =），链接里会被转义成 %2B/%2F/%3D。
// 用 url.User.String() 读到的是**未解码**的串，直接写进 mihomo 配置 → 认证必然失败。
func TestConvertsHysteria2URIWithBase64Password(t *testing.T) {
	uri := "hysteria2://i%2Bcvb54Txqci333NMLE%2BxWoyGD13hLkzrRZxXaV8qYo%3D@example.com:51071" +
		"?obfs=salamander&obfs-password=2k7wdHOubCfS&security=tls&sni=www.example.com#hy2"
	proxies, err := ConvertsV2Ray([]byte(uri))
	if err != nil {
		t.Fatal(err)
	}
	if len(proxies) != 1 {
		t.Fatalf("应转出 1 个代理, got %d", len(proxies))
	}
	p := proxies[0]
	if p["password"] != "i+cvb54Txqci333NMLE+xWoyGD13hLkzrRZxXaV8qYo=" {
		t.Fatalf("hysteria2 密码未解码: %v", p["password"])
	}
	if p["obfs"] != "salamander" || p["obfs-password"] != "2k7wdHOubCfS" {
		t.Errorf("obfs 未转换: %v / %v", p["obfs"], p["obfs-password"])
	}
}
