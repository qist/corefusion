package formatter

import "testing"

// TestXrayFormatterMasque MASQUE 只有 xray 出站能表达：
// settings 是 address/port/remoteDNS，认证与 HTTP 参数在 streamSettings.masqueSettings。
func TestXrayFormatterMasque(t *testing.T) {
	ob := renderFirstXrayOutbound(t, Node{
		Name:       "m",
		Protocol:   "masque",
		Host:       "1.2.3.4",
		Port:       443,
		Transport:  "masque",
		TLS:        true,
		ServerName: "example.com",
		Password:   "secret",
		Extra: map[string]interface{}{
			"username":    "u@example.com",
			"masque-path": "/.well-known/masque/ip/{target}/{ipproto}/",
			"alpn":        []string{"h3"},
		},
	})

	if ob["protocol"] != "masque" {
		t.Fatalf("protocol = %v", ob["protocol"])
	}
	settings, _ := ob["settings"].(map[string]interface{})
	// 经过 JSON 往返，数字统一变成 float64。
	if settings["address"] != "1.2.3.4" || settings["port"] != float64(443) {
		t.Errorf("settings = %#v", settings)
	}
	ss, _ := ob["streamSettings"].(map[string]interface{})
	if ss["network"] != "masque" {
		t.Errorf("network = %v, want masque", ss["network"])
	}
	if ss["security"] != "tls" {
		t.Errorf("security = %v, want tls（masque 必须 TLS）", ss["security"])
	}
	mq, _ := ss["masqueSettings"].(map[string]interface{})
	if mq["path"] != "/.well-known/masque/ip/{target}/{ipproto}/" {
		t.Errorf("masqueSettings.path = %v", mq["path"])
	}
	if mq["user"] != "u@example.com" || mq["pass"] != "secret" {
		t.Errorf("masqueSettings creds = %#v", mq)
	}
	tls, _ := ss["tlsSettings"].(map[string]interface{})
	if tls["serverName"] != "example.com" {
		t.Errorf("serverName = %v", tls["serverName"])
	}
	alpn, _ := tls["alpn"].([]interface{})
	if len(alpn) != 1 || alpn[0] != "h3" {
		t.Errorf("alpn = %#v", tls["alpn"])
	}
}

// TestXrayFormatterMasqueForcesTLSEvenIfNodeSaysPlain 即使节点标记非 TLS，
// masque 也必须补上 TLS（否则 xray 直接拒载整份配置）。
func TestXrayFormatterMasqueForcesTLS(t *testing.T) {
	ob := renderFirstXrayOutbound(t, Node{
		Name: "m", Protocol: "masque", Host: "1.2.3.4", Port: 443,
		Transport: "masque",
	})
	ss, _ := ob["streamSettings"].(map[string]interface{})
	if ss["security"] != "tls" {
		t.Errorf("security = %v, want tls", ss["security"])
	}
	if ss["network"] != "masque" {
		t.Errorf("network = %v, want masque", ss["network"])
	}
}
