package subscriptions

import (
	"encoding/json"
	"testing"
)

// buildExtra 是 singbox_json 订阅的节点来源：hy2 的混淆 / BBR / 动态端口必须
// 落到 Extra 上，否则订阅里的 hy2 就是"能连但参数不全"的裸节点（分享链接却正常）。
func TestBuildExtraHysteria2(t *testing.T) {
	raw := `{
	  "tag": "inbound-hysteria-51071",
	  "protocol": "hysteria",
	  "settings": {"clients": [{"auth": "AUTH", "email": "u@x.com"}], "version": 2},
	  "streamSettings": {
	    "network": "hysteria",
	    "security": "tls",
	    "tlsSettings": {"serverName": "www.example.com", "alpn": ["h3"]},
	    "hysteriaSettings": {"version": 2, "obfs": "salamander", "obfsPassword": "OBFS", "hopPorts": "10000-19999"},
	    "finalmask": {"quicParams": {"congestion": "bbr", "bbrProfile": "conservative", "maxIdleTimeout": 30}}
	  }
	}`
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &config); err != nil {
		t.Fatal(err)
	}
	extra := buildExtra(config, getMap(config, "streamSettings"), "hysteria")

	for key, want := range map[string]interface{}{
		"obfs":             "salamander",
		"obfs-password":    "OBFS",
		"ports":            "10000-19999",
		"bbr-profile":      "conservative",
		"max-idle-timeout": 30,
	} {
		if extra[key] != want {
			t.Errorf("Extra[%s] = %v, want %v", key, extra[key], want)
		}
	}
	if _, has := extra["finalmask"]; !has {
		t.Error("Extra 缺 finalmask（xray 订阅需要它）")
	}
	// bbr 拥塞下不应把 brutalUp/Down 转成 up/down —— 会让客户端反过来请求 brutal，
	// 与服务端设定的 bbr_profile 冲突。
	if v, has := extra["up"]; has {
		t.Errorf("bbr 拥塞不应下发 up: %v", v)
	}
}
