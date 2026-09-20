package sharelink

import (
	"strings"
	"testing"
)

// 认证类链接（socks/http）：用户名、密码要**分别**转义，中间的分隔 ':' 不能被编码成 %3A
// —— 不少客户端按第一个 ':' 切分 userinfo，整体转义后会认证失败。
func TestGenerateAuthLinkKeepsUserinfoSeparator(t *testing.T) {
	link := Generate(Node{
		Tag:      "t",
		Protocol: "socks",
		Host:     "1.2.3.4",
		Port:     1080,
		Settings: map[string]interface{}{
			"clients": []interface{}{map[string]interface{}{
				"username": "user@x.com",
				"password": "p#ss word",
			}},
		},
	})
	if link == "" {
		t.Fatal("socks 链接不应为空")
	}
	if !strings.HasPrefix(link, "socks://user%40x.com:p%23ss%20word@1.2.3.4:1080?") {
		t.Fatalf("userinfo 编码不对: %s", link)
	}
	if strings.Contains(strings.Split(link, "@")[0], "%3A") {
		t.Fatalf("userinfo 的分隔 ':' 不能被编码: %s", link)
	}
}

// xray 的 socks/http 客户端凭据字段是 user/pass（不是 username/password），
// 别名取不到的话生成的链接就没有凭据。
func TestGenerateAuthLinkSupportsUserPassAlias(t *testing.T) {
	link := Generate(Node{
		Tag:      "t",
		Protocol: "http",
		Host:     "1.2.3.4",
		Port:     8080,
		Settings: map[string]interface{}{
			"clients": []interface{}{map[string]interface{}{"user": "u@x.com", "pass": "p#ss"}},
		},
		StreamSettings: map[string]interface{}{"security": "none"},
	})
	if !strings.HasPrefix(link, "http://u%40x.com:p%23ss@1.2.3.4:8080?") {
		t.Fatalf("http 链接未取到 user/pass: %s", link)
	}
}

// anytls 有官方 URI 规范：anytls://[auth@]host[:port]/?sni=&insecure=
// （Shadowrocket 可直接导入，所以不能用"无标准 scheme"当理由跳过）。
func TestGenerateAnyTLSLink(t *testing.T) {
	link := Generate(Node{
		Tag:      "any",
		Protocol: "anytls",
		Host:     "1.2.3.4",
		Port:     8443,
		Settings: map[string]interface{}{
			"clients": []interface{}{map[string]interface{}{"password": "P@ss word"}},
		},
		StreamSettings: map[string]interface{}{
			"security": "tls",
			"tlsSettings": map[string]interface{}{
				"serverName":    "real.example.com",
				"allowInsecure": true,
				"fingerprint":   "chrome",
				"alpn":          []interface{}{"h2", "http/1.1"},
			},
		},
	})
	for _, want := range []string{
		"anytls://P%40ss%20word@1.2.3.4:8443/?",
		"sni=real.example.com",
		"insecure=1",
		"fp=chrome",
	} {
		if !strings.Contains(link, want) {
			t.Errorf("anytls 链接缺 %q: %s", want, link)
		}
	}
}

// tuic 的拥塞控制 / UDP 中继模式在 sing-box 里是顶层字段，归一进 streamSettings.tuicSettings
// 后必须出现在链接里（此前只读 settings，永远取不到）。
func TestGenerateTuicLinkCarriesCongestionAndRelayMode(t *testing.T) {
	link := Generate(Node{
		Tag:      "t",
		Protocol: "tuic",
		Host:     "1.2.3.4",
		Port:     443,
		Settings: map[string]interface{}{
			"clients": []interface{}{map[string]interface{}{"uuid": "11111111-1111-1111-1111-111111111111", "password": "p"}},
		},
		StreamSettings: map[string]interface{}{
			"security":    "tls",
			"tlsSettings": map[string]interface{}{"serverName": "sni.example.com"},
			"tuicSettings": map[string]interface{}{
				"congestion_control": "bbr",
				"udp_relay_mode":     "native",
			},
		},
	})
	for _, want := range []string{"congestion_control=bbr", "udp_relay_mode=native", "sni=sni.example.com"} {
		if !strings.Contains(link, want) {
			t.Errorf("tuic 链接缺 %s: %s", want, link)
		}
	}
}

// REALITY 的 ALPN 也要下发（客户端按它做 TLS 握手），此前后端漏发、前端有。
func TestGenerateVlessLinkRealityIncludesAlpn(t *testing.T) {
	link := Generate(Node{
		Tag:      "r",
		Protocol: "vless",
		Host:     "1.2.3.4",
		Port:     443,
		Settings: map[string]interface{}{
			"clients": []interface{}{map[string]interface{}{"id": "11111111-1111-1111-1111-111111111111"}},
		},
		StreamSettings: map[string]interface{}{
			"security": "reality",
			"network":  "tcp",
			"tlsSettings": map[string]interface{}{
				"serverName": "www.apple.com",
				"alpn":       []interface{}{"h2", "http/1.1"},
			},
			"realitySettings": map[string]interface{}{
				"_password":   "PBK",
				"shortIds":    []interface{}{"1234"},
				"serverNames": []interface{}{"www.apple.com"},
			},
		},
	})
	for _, want := range []string{"security=reality", "pbk=PBK", "alpn=h2%2Chttp%2F1.1"} {
		if !strings.Contains(link, want) {
			t.Errorf("reality 链接缺 %s: %s", want, link)
		}
	}
}
