package sharelink

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	realityutil "corefusion/internal/reality"
)

// xray 25 起 TCP 传输的新名字是 "raw"，但链接里必须写标准值 **"tcp"**：
// 并非所有客户端都认识 raw（mihomo 的 network 也不认），而两者在服务端等价。
func TestGenerateLinkNormalizesRawTransportToTCP(t *testing.T) {
	ss := map[string]interface{}{"network": "raw", "security": "none"}

	vless := Generate(Node{
		Tag: "v", Protocol: "vless", Host: "1.2.3.4", Port: 443,
		StreamSettings: ss,
		Settings:       map[string]interface{}{"clients": []interface{}{map[string]interface{}{"id": "11111111-1111-1111-1111-111111111111"}}},
	})
	if !strings.Contains(vless, "type=tcp") || strings.Contains(vless, "type=raw") {
		t.Errorf("vless 链接应发 type=tcp: %s", vless)
	}

	trojan := Generate(Node{
		Tag: "t", Protocol: "trojan", Host: "1.2.3.4", Port: 443,
		StreamSettings: map[string]interface{}{"network": "raw", "security": "tls"},
		Settings:       map[string]interface{}{"clients": []interface{}{map[string]interface{}{"password": "pw"}}},
	})
	if !strings.Contains(trojan, "type=tcp") || strings.Contains(trojan, "type=raw") {
		t.Errorf("trojan 链接应发 type=tcp: %s", trojan)
	}

	// vmess 的传输在 base64(JSON).net 里
	vmess := Generate(Node{
		Tag: "m", Protocol: "vmess", Host: "1.2.3.4", Port: 443,
		StreamSettings: ss,
		Settings:       map[string]interface{}{"clients": []interface{}{map[string]interface{}{"id": "11111111-1111-1111-1111-111111111111"}}},
	})
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(vmess, "vmess://"))
	if err != nil {
		t.Fatalf("vmess 链接不是合法 base64: %v", err)
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("vmess 链接 JSON 非法: %v", err)
	}
	if obj["net"] != "tcp" {
		t.Errorf("vmess net = %v, want tcp", obj["net"])
	}

	// mkcp 仍然写作 kcp
	kcp := Generate(Node{
		Tag: "k", Protocol: "vless", Host: "1.2.3.4", Port: 443,
		StreamSettings: map[string]interface{}{"network": "mkcp", "security": "none"},
		Settings:       map[string]interface{}{"clients": []interface{}{map[string]interface{}{"id": "11111111-1111-1111-1111-111111111111"}}},
	})
	if !strings.Contains(kcp, "type=kcp") {
		t.Errorf("mkcp 应写作 kcp: %s", kcp)
	}
}

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

// REALITY 在面板里有三种存法，取参必须都能认（认不出就是"整段参数丢失"，
// 客户端配置退化成普通 TLS → 连不上）。sni/sid 取**第一个**，保证链接/三种片段一致。
func TestRealityClientParamsShapes(t *testing.T) {
	shapes := map[string]map[string]interface{}{
		"扁平(老配置)": {
			"publicKey":   "PBK",
			"shortId":     "SID",
			"fingerprint": "chrome",
			"serverNames": []interface{}{"sni.example.com"},
		},
		"settings[](面板编辑器)": {
			"privateKey":  "PRIV",
			"shortIds":    []interface{}{"S1", "S2"},
			"serverNames": []interface{}{"sni.example.com", "other.example.com"},
			"settings": []interface{}{map[string]interface{}{
				"fingerprint": "chrome", "password": "PBK",
			}},
		},
		"下划线(sing-box 反推)": {
			"_password":   "PBK",
			"shortIds":    []interface{}{"S1"},
			"serverNames": []interface{}{"sni.example.com"},
		},
	}
	for label, reality := range shapes {
		ss := map[string]interface{}{"security": "reality", "realitySettings": reality}
		got := RealityClientParams(ss)
		if got.PublicKey != "PBK" {
			t.Errorf("%s: PublicKey = %q, want PBK", label, got.PublicKey)
		}
		if got.ShortID != "SID" && got.ShortID != "S1" {
			t.Errorf("%s: ShortID = %q, want SID/S1", label, got.ShortID)
		}
		if got.ServerName != "sni.example.com" {
			t.Errorf("%s: ServerName = %q", label, got.ServerName)
		}
	}
	// 指纹：settings[0].fingerprint 与顶层都要认
	if got := RealityClientParams(map[string]interface{}{"realitySettings": map[string]interface{}{
		"settings": []interface{}{map[string]interface{}{"fingerprint": "safari"}},
	}}); got.Fingerprint != "safari" {
		t.Errorf("settings[0].fingerprint 未识别: %+v", got)
	}
	if got := RealityClientParams(map[string]interface{}{"realitySettings": map[string]interface{}{
		"fingerprint": "firefox",
	}}); got.Fingerprint != "firefox" {
		t.Errorf("顶层 fingerprint 未识别: %+v", got)
	}
	// 没有 realitySettings → 全空（不能 panic）
	if got := RealityClientParams(map[string]interface{}{"security": "tls"}); got != (RealityParams{}) {
		t.Errorf("非 REALITY 应返回空值: %+v", got)
	}
}

// 只存了服务端私钥（面板里很常见：编辑器/老数据没保存 pbk）时必须**推导出公钥**：
// 客户端没有 pbk 就完全连不上（链接里缺 pbk、xray 片段 publicKey 缺失会被 xray 拒载）。
func TestRealityClientParamsDerivesPublicKey(t *testing.T) {
	// xray 的 privateKey 推导（用 reality 包同一实现的已知测试向量校验一致性）
	const priv = "CAP0iqPwrizd4edy_hLIKtlpA7V9WE2gAU0LxpfeCUY"
	pbk := realityutil.PublicKeyFromPrivate(priv)
	if pbk == "" {
		t.Fatal("私钥推导失败（测试向量不对？）")
	}
	got := RealityClientParams(map[string]interface{}{"realitySettings": map[string]interface{}{
		"privateKey":  priv,
		"serverNames": []interface{}{"www.oracle.com"},
		"shortIds":    []interface{}{"30b3ee1d"},
		"settings":    []interface{}{map[string]interface{}{"fingerprint": "chrome"}},
	}})
	if got.PublicKey != pbk {
		t.Errorf("PublicKey = %q, want %q（应由 privateKey 推导）", got.PublicKey, pbk)
	}
	// 链接里必须带上 pbk
	link := Generate(Node{
		Tag: "n", Protocol: "vless", Host: "www.test.ss", Port: 443,
		Settings: map[string]interface{}{"clients": []interface{}{map[string]interface{}{"id": "eba0408a-f302-4f10-8e10-c85c0340fbd4"}}},
		StreamSettings: map[string]interface{}{
			"security": "reality",
			"realitySettings": map[string]interface{}{
				"privateKey":  priv,
				"serverNames": []interface{}{"www.oracle.com"},
				"shortIds":    []interface{}{"30b3ee1d"},
			},
		},
	})
	if !strings.Contains(link, "pbk="+pbk) {
		t.Errorf("链接缺推导出的 pbk: %s", link)
	}
}
