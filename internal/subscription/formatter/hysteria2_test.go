package formatter

import (
	"encoding/json"
	"strings"
	"testing"
)

// hy2 的订阅参数必须"最大化"下发（混淆 / 带宽 / BBR / 动态端口），
// 且不能写出客户端非法的字段。sing-box 出站字段名已用 sing-box check 实证。
func TestSingBoxFormatterHysteria2Params(t *testing.T) {
	node := Node{
		Name: "hy2", Protocol: "hysteria2", Host: "1.2.3.4", Port: 47180,
		Transport: "hysteria", // 来自 streamSettings.network，但 sing-box 出站没有这个字段
		TLS:       true, ServerName: "www.example.com", Password: "PW",
		Extra: map[string]interface{}{
			"obfs":          "salamander",
			"obfs-password": "OBFS",
			"up":            "200 mbps",
			"down":          "300 mbps",
			"bbr-profile":   "conservative",
			"ports":         "10000-19999",
			"hop-interval":  "30",
		},
	}
	data, err := (&SingBoxFormatter{}).Format([]Node{node})
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Outbounds []map[string]interface{} `json:"outbounds"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	ob := cfg.Outbounds[0]
	if _, has := ob["transport"]; has {
		t.Fatal("hysteria2 出站不能带 transport（sing-box 会 FATAL: unknown field）")
	}
	obfs, _ := ob["obfs"].(map[string]interface{})
	if obfs["type"] != "salamander" || obfs["password"] != "OBFS" {
		t.Fatalf("obfs 缺失或字段名不对: %v", ob["obfs"])
	}
	if ob["up_mbps"] != float64(200) || ob["down_mbps"] != float64(300) {
		t.Fatalf("带宽缺失: up=%v down=%v", ob["up_mbps"], ob["down_mbps"])
	}
	if ob["bbr_profile"] != "conservative" {
		t.Fatalf("bbr_profile 缺失: %v", ob["bbr_profile"])
	}
	ports, _ := ob["server_ports"].([]interface{})
	if len(ports) != 1 || ports[0] != "10000:19999" {
		t.Fatalf("server_ports 缺失或格式不对（应为起:止）: %v", ob["server_ports"])
	}
	if ob["hop_interval"] != "30s" {
		t.Fatalf("hop_interval 缺失或格式不对: %v", ob["hop_interval"])
	}
}

// hop-interval 在 xray / mihomo 是"秒数或秒数区间"，sing-box 只接受单个 duration
// —— 区间必须取上界，否则客户端加载配置会 FATAL（unknown unit "-"）。
func TestSingboxHopInterval(t *testing.T) {
	cases := map[string]string{
		"30":    "30s",
		"5-10":  "10s",
		"5~10":  "10s",
		"30s":   "30s",
		"1m30s": "1m30s",
		"":      "",
	}
	for in, want := range cases {
		if got := singboxHopInterval(in); got != want {
			t.Errorf("singboxHopInterval(%q) = %q, want %q", in, got, want)
		}
	}
}

// bbr-profile 在 sing-box 侧是以链接参数下发的（不在 fm 里），
// xray 口径要把它合进 finalmask.quicParams.bbrProfile，否则互转会丢拥塞控制。
func TestURIToXrayNodeHysteria2BBR(t *testing.T) {
	uri := "hysteria2://PW@1.2.3.4:47180?obfs=salamander&obfs-password=OBFS&bbr-profile=conservative&mport=10000-19999&security=tls&sni=www.example.com#n"
	node := ParseURIToNode(uri)
	if node == nil {
		t.Fatal("解析失败")
	}
	if node.Extra["bbr-profile"] != "conservative" {
		t.Fatalf("bbr-profile 未解析: %v", node.Extra["bbr-profile"])
	}
	if node.Extra["ports"] != "10000-19999" {
		t.Fatalf("mport 未解析: %v", node.Extra["ports"])
	}
	data, err := (&XrayFormatter{}).Format([]Node{*node})
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, `"bbrProfile": "conservative"`) {
		t.Fatalf("xray 配置缺 bbrProfile:\n%s", s)
	}
	if !strings.Contains(s, `"obfs": "salamander"`) {
		t.Fatalf("xray 配置缺 obfs:\n%s", s)
	}
	if !strings.Contains(s, `"hopPorts": "10000-19999"`) {
		t.Fatalf("xray 配置缺 hopPorts:\n%s", s)
	}
}
