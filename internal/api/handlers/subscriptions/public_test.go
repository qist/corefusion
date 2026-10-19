package subscriptions

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"corefusion/internal/model/entity"
	"corefusion/internal/subscription/formatter"

	"github.com/gin-gonic/gin"
)

func TestBuildBillingInfoEntriesRequiresExplicitSelection(t *testing.T) {
	sub := &entity.Subscription{}
	templates := defaultBillingTemplateConfig()
	items := []renderedSubscriptionNode{
		{
			InboundConfigID: 1,
			Link:            "vless://uuid@example.com:443#node-a",
			Node:            &formatter.Node{Name: "node-a", Protocol: "vless"},
		},
	}

	got := buildBillingInfoEntries(sub, templates, items)
	if len(got) != 0 {
		t.Fatalf("expected no billing entries without explicit selection, got %d", len(got))
	}

	if header := buildSubscriptionInfoHeader(sub, templates); header != "" {
		t.Fatalf("expected empty info header without explicit selection, got %q", header)
	}
}

func TestBuildBillingInfoEntriesUsesExplicitNodeOrder(t *testing.T) {
	sub := &entity.Subscription{
		QuotaBytes:     20 * 1024 * 1024 * 1024,
		UsedBytesCache: 4 * 1024 * 1024 * 1024,
	}
	items := []renderedSubscriptionNode{
		{
			MemberID:        11,
			InboundConfigID: 11,
			Link:            "vless://uuid-a@example.com:443#node-a",
			Node:            &formatter.Node{Name: "node-a", Protocol: "vless"},
		},
		{
			MemberID:        22,
			InboundConfigID: 22,
			Link:            "vless://uuid-b@example.com:443#node-b",
			Node:            &formatter.Node{Name: "node-b", Protocol: "vless"},
		},
	}

	templates := defaultBillingTemplateConfig()
	templates.BillingNodeIDs = []int64{22, 11}
	got := buildBillingInfoEntries(sub, templates, items)
	labels := buildBillingInfoLabels(sub, templates)
	if len(got) != len(labels) {
		t.Fatalf("expected %d billing entries, got %d", len(labels), len(got))
	}

	for i, wantNode := range got {
		if wantNode.Node == nil {
			t.Fatalf("entry %d should include formatter node", i)
		}
		if wantNode.Node.Name != labels[i] {
			t.Fatalf("entry %d should be renamed to %q, got %q", i, labels[i], wantNode.Node.Name)
		}
		if wantNode.Link == "" {
			t.Fatalf("entry %d should keep a share link clone", i)
		}
	}
}

func TestRenderBillingTemplateSupportsCustomVariables(t *testing.T) {
	sub := &entity.Subscription{
		QuotaBytes:     10 * 1024 * 1024 * 1024,
		UsedBytesCache: 3 * 1024 * 1024 * 1024,
		QuotaState:     "normal",
	}
	got := renderBillingTemplate("Traffic {remaining_human} / {quota_gb} / {quota_state}", sub)
	if !strings.Contains(got, "Traffic 7.00 GB / 10.00 / normal") {
		t.Fatalf("unexpected rendered template %q", got)
	}
}

func TestLoadBillingTemplateConfigFallsBackToPlanOverride(t *testing.T) {
	handler := &PublicHandler{}
	plan := &entity.PlanTemplate{
		BillingRemainingTemplate: "Plan Remaining {remaining_human}",
		BillingResetTemplate:     "Plan Reset {reset_human}",
	}
	got := handler.loadBillingTemplateConfig(context.Background(), plan)
	if got.RemainingTemplate != "Plan Remaining {remaining_human}" {
		t.Fatalf("expected plan remaining template override, got %q", got.RemainingTemplate)
	}
	if got.ResetTemplate != "Plan Reset {reset_human}" {
		t.Fatalf("expected plan reset template override, got %q", got.ResetTemplate)
	}
	if got.ExpiryTemplate != "Expires: {expires_human}" {
		t.Fatalf("expected default expiry template fallback, got %q", got.ExpiryTemplate)
	}
}

func TestRenameShareLinkSupportsRawURLBase64VMess(t *testing.T) {
	payload, err := json.Marshal(map[string]interface{}{
		"v":    "2",
		"ps":   "old-name",
		"add":  "example.com",
		"port": "443",
		"id":   "11111111-1111-1111-1111-111111111111",
	})
	if err != nil {
		t.Fatalf("marshal vmess payload: %v", err)
	}

	link := "vmess://" + base64.RawURLEncoding.EncodeToString(payload)
	renamed := renameShareLink(link, "remaining-info")
	if renamed == link {
		t.Fatalf("expected vmess link to be renamed")
	}

	decoded, err := decodeVmessPayload(renamed[len("vmess://"):])
	if err != nil {
		t.Fatalf("decode renamed vmess payload: %v", err)
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(decoded, &obj); err != nil {
		t.Fatalf("unmarshal renamed vmess payload: %v", err)
	}
	if obj["ps"] != "remaining-info" {
		t.Fatalf("expected ps to be updated, got %v", obj["ps"])
	}
}

func TestBuildExtraSupportsRawHTTPDisguise(t *testing.T) {
	ss := map[string]interface{}{
		"network": "raw",
		"rawSettings": map[string]interface{}{
			"header": map[string]interface{}{
				"type": "http",
				"request": map[string]interface{}{
					"path": []interface{}{"/"},
					"headers": map[string]interface{}{
						"Host":       []interface{}{"www.baidu.com"},
						"User-Agent": []interface{}{"Mozilla/5.0"},
					},
				},
			},
		},
	}

	extra := buildExtra(map[string]interface{}{}, ss, "vless")
	httpOpts, ok := extra["http-opts"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected http-opts to be present, got %#v", extra)
	}
	paths, ok := httpOpts["path"].([]string)
	if !ok || len(paths) != 1 || paths[0] != "/" {
		t.Fatalf("expected path /, got %#v", httpOpts["path"])
	}
	headers, ok := httpOpts["headers"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected headers map, got %#v", httpOpts["headers"])
	}
	hosts, ok := headers["Host"].([]string)
	if !ok || len(hosts) != 1 || hosts[0] != "www.baidu.com" {
		t.Fatalf("expected Host header, got %#v", headers["Host"])
	}
}

func TestConvertLinksToMihomoProxyMapsUsesConverterForRawHTTP(t *testing.T) {
	links := []string{
		"vless://9dc54a2a-a82c-4dcb-8a0b-c693ab6e8731@example.com:443?encryption=none&headerType=http&host=www.baidu.com&path=%2F&security=none&type=raw#inbound-vless-20296",
	}

	proxyMaps := convertLinksToMihomoProxyMaps(links)
	if len(proxyMaps) != 1 {
		t.Fatalf("expected 1 proxy map, got %d", len(proxyMaps))
	}

	proxy := proxyMaps[0]
	if proxy["name"] != "inbound-vless-20296" {
		t.Fatalf("expected proxy name to come from URI fragment, got %#v", proxy["name"])
	}
	if proxy["network"] != "http" {
		t.Fatalf("expected raw/http to convert into mihomo http network, got %#v", proxy["network"])
	}

	httpOpts, ok := proxy["http-opts"].(map[string]any)
	if !ok {
		t.Fatalf("expected http-opts from converter, got %#v", proxy["http-opts"])
	}
	yaml := string(generatePlainMihomoYAML(proxyMaps))
	if !strings.Contains(yaml, "network: http") {
		t.Fatalf("expected generated YAML to use converted network")
	}
	if !strings.Contains(yaml, "http-opts:") {
		t.Fatalf("expected generated YAML to keep converted http-opts")
	}
	if httpOpts["headers"] == nil {
		t.Fatalf("expected converted http headers, got %#v", httpOpts)
	}
}

func TestConvertLinksToMihomoProxyMapsNormalizesVMessWSPathAndRemovesXUDP(t *testing.T) {
	payload, err := json.Marshal(map[string]interface{}{
		"v":    "2",
		"ps":   "office",
		"add":  "grafana.qkagame.com",
		"port": 443,
		"id":   "cefad44f-3d22-489b-aec7-e25961363376",
		"aid":  0,
		"net":  "ws",
		"path": "/apps?ed=2560",
		"host": "grafana.qkagame.com",
		"tls":  "tls",
		"scy":  "auto",
		"alpn": "",
	})
	if err != nil {
		t.Fatalf("marshal vmess payload: %v", err)
	}

	links := []string{"vmess://" + base64.StdEncoding.EncodeToString(payload)}
	proxyMaps := convertLinksToMihomoProxyMaps(links)
	if len(proxyMaps) != 1 {
		t.Fatalf("expected 1 proxy map, got %d", len(proxyMaps))
	}

	proxy := proxyMaps[0]
	if _, exists := proxy["xudp"]; exists {
		t.Fatalf("expected vmess proxy not to include xudp, got %#v", proxy["xudp"])
	}
	if _, exists := proxy["alpn"]; exists {
		t.Fatalf("expected empty alpn to be removed, got %#v", proxy["alpn"])
	}

	wsOpts, ok := proxy["ws-opts"].(map[string]any)
	if !ok {
		t.Fatalf("expected ws-opts from converter, got %#v", proxy["ws-opts"])
	}
	if wsOpts["path"] != "/apps?ed=2560" {
		t.Fatalf("expected vmess ws path to preserve early-data query, got %#v", wsOpts["path"])
	}

	yaml := string(generatePlainMihomoYAML(proxyMaps))
	if strings.Contains(yaml, "xudp: true") {
		t.Fatalf("expected generated YAML not to contain xudp for vmess, got %s", yaml)
	}
	if !strings.Contains(yaml, "path: \"/apps?ed=2560\"") && !strings.Contains(yaml, "path: /apps?ed=2560") {
		t.Fatalf("expected generated YAML to keep full ws path, got %s", yaml)
	}
}

func TestBuildRenderedMihomoProxyMapsPreservesExternalVMessFields(t *testing.T) {
	externalProxy := map[string]interface{}{
		"alterId":            0,
		"cipher":             "auto",
		"client-fingerprint": "chrome",
		"name":               "office",
		"network":            "ws",
		"port":               443,
		"server":             "grafana.qkagame.com",
		"servername":         "grafana.qkagame.com",
		"skip-cert-verify":   false,
		"tls":                true,
		"type":               "vmess",
		"udp":                true,
		"uuid":               "cefad44f-3d22-489b-aec7-e25961363376",
		"ws-opts": map[string]interface{}{
			"early-data-header-name": "Sec-WebSocket-Protocol",
			"headers": map[string]interface{}{
				"Host": "grafana.qkagame.com",
			},
			"max-early-data": 4096,
			"path":           "/apps?ed=2560",
		},
	}

	items := []renderedSubscriptionNode{
		{
			Link:     formatter.MihomoToURI(externalProxy),
			ProxyMap: externalProxy,
		},
	}

	proxyMaps := buildRenderedMihomoProxyMaps(items)
	if len(proxyMaps) != 1 {
		t.Fatalf("expected 1 proxy map, got %d", len(proxyMaps))
	}

	proxy := proxyMaps[0]
	if _, exists := proxy["xudp"]; exists {
		t.Fatalf("expected external vmess proxy not to include xudp, got %#v", proxy["xudp"])
	}

	wsOpts, ok := proxy["ws-opts"].(map[string]any)
	if !ok {
		t.Fatalf("expected ws-opts on external proxy, got %#v", proxy["ws-opts"])
	}
	if wsOpts["max-early-data"] != float64(4096) && wsOpts["max-early-data"] != 4096 {
		t.Fatalf("expected external vmess max-early-data to stay 4096, got %#v", wsOpts["max-early-data"])
	}
	if wsOpts["path"] != "/apps?ed=2560" {
		t.Fatalf("expected external vmess path to stay /apps?ed=2560, got %#v", wsOpts["path"])
	}

	yaml := string(generatePlainMihomoYAML(proxyMaps))
	if !strings.Contains(yaml, "max-early-data: 4096") {
		t.Fatalf("expected generated YAML to keep max-early-data 4096, got %s", yaml)
	}
	if !strings.Contains(yaml, "path: \"/apps?ed=2560\"") && !strings.Contains(yaml, "path: /apps?ed=2560") {
		t.Fatalf("expected generated YAML to keep full ws path, got %s", yaml)
	}
}

func TestBuildRenderedMihomoProxyMapsPreservesExternalXHTTPFields(t *testing.T) {
	externalProxy := map[string]interface{}{
		"client-fingerprint": "chrome",
		"encryption":         "",
		"name":               "any_SNI_H2_No_SNI",
		"network":            "xhttp",
		"packet-encoding":    "xudp",
		"port":               443,
		"reality-opts": map[string]interface{}{
			"public-key":             "6TCFf3d10en3Ior1gpMEusL58WMpo7faWD6Titx9B1M",
			"short-id":               "03e822599b26",
			"support-x25519mlkem768": true,
		},
		"server":           "rem5ogoe5m.swsixtwo.top",
		"servername":       "ravexo.com",
		"skip-cert-verify": false,
		"tls":              true,
		"type":             "vless",
		"udp":              true,
		"uuid":             "8bd281a3-5fff-4c74-8a1a-83e3e20efa2e",
		"xhttp-opts": map[string]interface{}{
			"host":            "poki.com",
			"mode":            "stream-one",
			"no-grpc-header":  true,
			"path":            "/",
			"x-padding-bytes": "100-1000",
		},
	}

	items := []renderedSubscriptionNode{
		{
			Link:     formatter.MihomoToURI(externalProxy),
			ProxyMap: externalProxy,
		},
	}

	proxyMaps := buildRenderedMihomoProxyMaps(items)
	if len(proxyMaps) != 1 {
		t.Fatalf("expected 1 proxy map, got %d", len(proxyMaps))
	}

	proxy := proxyMaps[0]
	if proxy["packet-encoding"] != "xudp" {
		t.Fatalf("expected packet-encoding xudp, got %#v", proxy["packet-encoding"])
	}
	if proxy["network"] != "xhttp" {
		t.Fatalf("expected network xhttp, got %#v", proxy["network"])
	}

	realityOpts, ok := proxy["reality-opts"].(map[string]any)
	if !ok {
		t.Fatalf("expected reality-opts, got %#v", proxy["reality-opts"])
	}
	if realityOpts["support-x25519mlkem768"] != true {
		t.Fatalf("expected support-x25519mlkem768 true, got %#v", realityOpts["support-x25519mlkem768"])
	}

	xhttpOpts, ok := proxy["xhttp-opts"].(map[string]any)
	if !ok {
		t.Fatalf("expected xhttp-opts, got %#v", proxy["xhttp-opts"])
	}
	if xhttpOpts["no-grpc-header"] != true {
		t.Fatalf("expected no-grpc-header true, got %#v", xhttpOpts["no-grpc-header"])
	}
	if xhttpOpts["x-padding-bytes"] != "100-1000" {
		t.Fatalf("expected x-padding-bytes 100-1000, got %#v", xhttpOpts["x-padding-bytes"])
	}

	yaml := string(generatePlainMihomoYAML(proxyMaps))
	for _, want := range []string{
		"name: any_SNI_H2_No_SNI",
		"network: xhttp",
		"packet-encoding: xudp",
		"support-x25519mlkem768: true",
		"no-grpc-header: true",
		"x-padding-bytes: 100-1000",
		"host: poki.com",
		"mode: stream-one",
	} {
		if !strings.Contains(yaml, want) {
			t.Fatalf("expected generated YAML to contain %q, got %s", want, yaml)
		}
	}
}

func TestParseSubscriptionLinksToFormatterNodesPreservesKCPAndHysteria2(t *testing.T) {
	links := []string{
		"vless://0c333ab9-1c3a-4c01-a96f-a59743e64d93@www.example.com:54461?encryption=none&headerType=wireguard&mtu=1350&security=none&seed=SwOzlkxeZpKaCxWysYucYQ%3D%3D&tti=50&type=kcp#inbound-vless-54461",
		"hysteria2://i%2Bcvb54Txqci333NMLE%2BxWoyGD13hLkzrRZxXaV8qYo%3D@www.example.com:51071?alpn=h3&fm=%7B%22quicParams%22%3A%7B%22bbrProfile%22%3A%22conservative%22%2C%22congestion%22%3A%22bbr%22%2C%22udpHop%22%3A%7B%22interval%22%3A%225-10%22%2C%22ports%22%3A%2240000-50000%22%7D%7D%2C%22udp%22%3A%5B%7B%22settings%22%3A%7B%22password%22%3A%222k7wdHOubCfS%22%7D%2C%22type%22%3A%22salamander%22%7D%5D%7D&fp=chrome&mport=40000-50000&obfs=salamander&obfs-password=2k7wdHOubCfS&security=tls&sni=www.example.com#inbound-hysteria-51071",
	}

	nodes := parseSubscriptionLinksToFormatterNodes(links)
	if len(nodes) != 2 {
		t.Fatalf("expected 2 parsed nodes, got %d", len(nodes))
	}

	kcpNode := nodes[0]
	if kcpNode.Transport != "kcp" {
		t.Fatalf("expected kcp transport, got %q", kcpNode.Transport)
	}
	kcpSettings, ok := kcpNode.Extra["kcp-settings"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected kcp-settings, got %#v", kcpNode.Extra["kcp-settings"])
	}
	if kcpSettings["headerType"] != "wireguard" {
		t.Fatalf("expected kcp headerType wireguard, got %#v", kcpSettings["headerType"])
	}
	if kcpSettings["mtu"] != 1350 {
		t.Fatalf("expected kcp mtu 1350, got %#v", kcpSettings["mtu"])
	}
	if kcpSettings["tti"] != 50 {
		t.Fatalf("expected kcp tti 50, got %#v", kcpSettings["tti"])
	}

	hyNode := nodes[1]
	if hyNode.Protocol != "hysteria2" {
		t.Fatalf("expected hysteria2 protocol, got %q", hyNode.Protocol)
	}
	if hyNode.Password != "i+cvb54Txqci333NMLE+xWoyGD13hLkzrRZxXaV8qYo=" {
		t.Fatalf("expected decoded hysteria2 password, got %q", hyNode.Password)
	}
	if hyNode.Extra["ports"] != "40000-50000" {
		t.Fatalf("expected hysteria2 ports 40000-50000, got %#v", hyNode.Extra["ports"])
	}
	if hyNode.Extra["hop-interval"] != "5-10" {
		t.Fatalf("expected hysteria2 hop-interval 5-10, got %#v", hyNode.Extra["hop-interval"])
	}
	if hyNode.Extra["obfs-password"] != "2k7wdHOubCfS" {
		t.Fatalf("expected hysteria2 obfs password, got %#v", hyNode.Extra["obfs-password"])
	}
}

func TestXrayFormatterFromRawLinksPreservesKCPAndHysteria2Fields(t *testing.T) {
	links := []string{
		"vless://0c333ab9-1c3a-4c01-a96f-a59743e64d93@www.example.com:54461?encryption=none&headerType=wireguard&mtu=1350&security=none&seed=SwOzlkxeZpKaCxWysYucYQ%3D%3D&tti=50&type=kcp#inbound-vless-54461",
		"hysteria2://i%2Bcvb54Txqci333NMLE%2BxWoyGD13hLkzrRZxXaV8qYo%3D@www.example.com:51071?alpn=h3&fm=%7B%22quicParams%22%3A%7B%22bbrProfile%22%3A%22conservative%22%2C%22congestion%22%3A%22bbr%22%2C%22udpHop%22%3A%7B%22interval%22%3A%225-10%22%2C%22ports%22%3A%2240000-50000%22%7D%7D%2C%22udp%22%3A%5B%7B%22settings%22%3A%7B%22password%22%3A%222k7wdHOubCfS%22%7D%2C%22type%22%3A%22salamander%22%7D%5D%7D&fp=chrome&mport=40000-50000&obfs=salamander&obfs-password=2k7wdHOubCfS&security=tls&sni=www.example.com#inbound-hysteria-51071",
	}

	nodes := parseSubscriptionLinksToFormatterNodes(links)
	data, err := formatter.GetFormatter(formatter.FormatXrayJSON).Format(nodes)
	if err != nil {
		t.Fatalf("format xray json: %v", err)
	}
	output := string(data)

	for _, want := range []string{
		"\"address\": \"www.example.com\"",
		// VLESS 的每个 user 必须显式带 encryption（Xray 25+ 强制，缺了整份配置加载失败）
		"\"encryption\": \"none\"",
		"\"network\": \"kcp\"",
		"\"kcpSettings\": {",
		// mkcp 的 header/seed 已从 kcpSettings 迁到 finalmask.udp 的 mkcp-legacy 条目
		// （写回 kcpSettings.header/seed 会被 Xray 26 拒绝：feature has been removed）
		"\"type\": \"mkcp-legacy\"",
		"\"header\": \"wireguard\"",
		"\"value\": \"SwOzlkxeZpKaCxWysYucYQ==\"",
		"\"protocol\": \"hysteria\"",
		"\"network\": \"hysteria\"",
		"\"auth\": \"i+cvb54Txqci333NMLE+xWoyGD13hLkzrRZxXaV8qYo=\"",
		"\"obfs\": \"salamander\"",
		"\"obfsPassword\": \"2k7wdHOubCfS\"",
		"\"hopPorts\": \"40000-50000\"",
		"\"hopInterval\": \"5-10\"",
		"\"finalmask\": {",
		"\"quicParams\": {",
		"\"congestion\": \"bbr\"",
		"\"bbrProfile\": \"conservative\"",
		"\"udpHop\": {",
		"\"interval\": \"5-10\"",
		"\"ports\": \"40000-50000\"",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected xray json to contain %q, got %s", want, output)
		}
	}
}

func TestConvertLinksToMihomoProxyMapsHandlesExternalNodeURI(t *testing.T) {
	proxy := map[string]interface{}{
		"name":     "external-trojan",
		"type":     "trojan",
		"server":   "example.com",
		"port":     443,
		"password": "secret",
		"network":  "ws",
		"sni":      "cdn.example.com",
		"ws-opts": map[string]interface{}{
			"path": "/ws",
			"headers": map[string]interface{}{
				"Host": "cdn.example.com",
			},
		},
	}

	link := formatter.MihomoToURI(proxy)
	if link == "" {
		t.Fatalf("expected external mihomo proxy to be convertible into URI")
	}

	proxyMaps := convertLinksToMihomoProxyMaps([]string{link})
	if len(proxyMaps) != 1 {
		t.Fatalf("expected 1 converted external proxy, got %d", len(proxyMaps))
	}

	yaml := string(generatePlainMihomoYAML(proxyMaps))
	if !strings.Contains(yaml, "name: external-trojan") {
		t.Fatalf("expected external proxy name in YAML, got %s", yaml)
	}
	if !strings.Contains(yaml, "network: ws") {
		t.Fatalf("expected external proxy ws network in YAML, got %s", yaml)
	}
	if !strings.Contains(yaml, "path: /ws") {
		t.Fatalf("expected external proxy ws path in YAML, got %s", yaml)
	}
}

// 订阅节点以 **http proxy-provider** 注入（官方模板写法：type:http + url + interval +
// health-check），分组通过 use 引用；mihomo 按 interval 定时拉 provider 专用端点
// （?format=mihomo_proxies_yaml，只输出 proxies 列表）→ 订阅能定时更新。
// buildProviderURL：订阅端点管理里配了启用独立域名 → 优先用
// https://<domain><path>/<token>（与 clientext buildSubLinks 口径一致）；
// 未配置（或 repo 未注入）→ 回退本请求 scheme://Host/sub/<token>。
func TestBuildProviderURLUsesSubscriptionEndpointDomain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &PublicHandler{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/sub/test-token", nil)

	// 未注入 repo → 回退本请求 host（X-Forwarded-Proto 判定 https）
	c.Request.Header.Set("X-Forwarded-Proto", "https")
	c.Request.Header.Set("X-Forwarded-Host", "panel.example.com")
	got := h.buildProviderURL(c, "test-token")
	want0 := "https://panel.example.com/sub/test-token?format=mihomo_proxies_yaml"
	want := want0
	if got != want {
		t.Fatalf("fallback url = %q, want %q", got, want)
	}

	// 注入 repo 且有启用独立域名（path=/ → 拼成根路径）
	h2 := &PublicHandler{}
	h2.SetSubEndpointRepo(fakeSubEndpointSource{endpoints: []entity.SubscriptionEndpoint{
		{ID: 1, Domain: "sub.example.com", Path: "/", Enabled: true},
	}})
	got = h2.buildProviderURL(c, "test-token")
	want = "https://sub.example.com/test-token?format=mihomo_proxies_yaml"
	if got != want {
		t.Fatalf("endpoint url = %q, want %q", got, want)
	}

	// 启用端点但 Domain 为空 → 跳过，回退本请求
	h3 := &PublicHandler{}
	h3.SetSubEndpointRepo(fakeSubEndpointSource{endpoints: []entity.SubscriptionEndpoint{
		{ID: 2, Domain: "", Path: "/", Enabled: true},
	}})
	got = h3.buildProviderURL(c, "test-token")
	if got != want0 {
		t.Fatalf("empty-domain url = %q, want fallback %q", got, want0)
	}
}

// fakeSubEndpointSource 实现 PublicHandler.SubEndpointSource 最小接口。
type fakeSubEndpointSource struct {
	endpoints []entity.SubscriptionEndpoint
}

func (f fakeSubEndpointSource) GetEnabled(_ context.Context) ([]entity.SubscriptionEndpoint, error) {
	return f.endpoints, nil
}

func TestGenerateMihomoYAMLInjectsHTTPProxyProvider(t *testing.T) {
	config := &entity.MihomoConfig{
		ConfigJSON: `{"proxy-groups":[
			{"name":"PROXY","type":"select","proxies":["DIRECT"],"_hasExternalNodes":true},
			{"name":"AUTO","type":"url-test","_hasExternalNodes":true,"url":"https://www.gstatic.com/generate_204","interval":300}
		]}`,
	}
	proxyMaps := []map[string]interface{}{
		{"name": "node-a", "type": "ss", "server": "1.2.3.4", "port": 443},
	}
	providerURL := "http://panel.example.com/sub/test-token?format=mihomo_proxies_yaml"
	out := string((&PublicHandler{}).generateMihomoYAML(config, nil, proxyMaps, providerURL))

	if !strings.Contains(out, "proxy-providers:") {
		t.Fatalf("YAML should contain proxy-providers section: %s", out)
	}
	if !strings.Contains(out, "    type: http") {
		t.Fatalf("dynamic provider should be http type: %s", out)
	}
	if !strings.Contains(out, `    url: "`+providerURL+`"`) {
		t.Fatalf("provider should point at dedicated subscription endpoint: %s", out)
	}
	if !strings.Contains(out, "    interval: 3600") {
		t.Fatalf("provider should carry update interval: %s", out)
	}
	if !strings.Contains(out, "health-check:") || !strings.Contains(out, "      enable: true") {
		t.Fatalf("provider should carry health-check: %s", out)
	}
	if !strings.Contains(out, "- subscription-nodes") {
		t.Fatalf("groups with _hasExternalNodes should reference the dynamic provider via use: %s", out)
	}
	if !strings.Contains(out, "- DIRECT") {
		t.Fatalf("static proxies of the group should be preserved: %s", out)
	}
	// 节点由 provider 定时拉取下发：主配置不再静态铺顶层 proxies（重名整份拒载）
	if strings.Contains(out, "name: node-a") {
		t.Fatalf("main config should not statically list nodes when provider is active: %s", out)
	}

	// 无 providerURL（单测/极端场景）时不生成 provider，节点走顶层 proxies
	out2 := string((&PublicHandler{}).generateMihomoYAML(config, nil, proxyMaps, ""))
	if strings.Contains(out2, "proxy-providers:") {
		t.Fatalf("no providerURL should mean no provider section: %s", out2)
	}
	if !strings.Contains(out2, "name: node-a") {
		t.Fatalf("no providerURL should fall back to top-level proxies: %s", out2)
	}
}

// 撞名规避：config_json 里用户自定义了同名的 proxy-providers 时，
// 动态 provider 应改用 subscription-nodes-2，且 use 引用要一致。
func TestGenerateMihomoYAMLProviderNameCollision(t *testing.T) {
	config := &entity.MihomoConfig{
		ConfigJSON: `{"proxy-providers":{"subscription-nodes":{"type":"http","url":"https://example.com/sub.yaml","interval":3600}},
			"proxy-groups":[{"name":"PROXY","type":"select","_hasExternalNodes":true}]}`,
	}
	proxyMaps := []map[string]interface{}{
		{"name": "node-a", "type": "ss", "server": "1.2.3.4", "port": 443},
	}
	out := string((&PublicHandler{}).generateMihomoYAML(config, nil, proxyMaps, "http://panel.example.com/sub/t?format=mihomo_proxies_yaml"))

	if !strings.Contains(out, "  subscription-nodes:") {
		t.Fatalf("user-defined provider should be emitted as-is: %s", out)
	}
	if !strings.Contains(out, "  subscription-nodes-2:") {
		t.Fatalf("dynamic provider should be renamed on collision: %s", out)
	}
	if !strings.Contains(out, "- subscription-nodes-2") {
		t.Fatalf("use reference should point at the renamed dynamic provider: %s", out)
	}
}

// mergeFormatSections：config_json 里已编辑好的配置段应原样合并进订阅输出，
// 且只合并白名单内的段（防止 Mihomo 残留/未知顶层键让 sing-box 整份拒载）。
func TestMergeFormatSectionsInjectsAllowedSectionsOnly(t *testing.T) {
	cfg := &entity.MihomoConfig{ConfigJSON: `{
		"log": {"level": "warn"},
		"dns": {"servers": [{"address": "1.1.1.1"}]},
		"route": {"final": "direct"},
		"mixed-port": 10801,
		"proxy-groups": [{"name": "PROXY"}]
	}`}
	base := []byte(`{"outbounds":[{"type":"direct","tag":"direct"}]}`)
	out := string(mergeFormatSections(base, cfg, "log", "dns", "route", "experimental"))

	for _, want := range []string{`"level": "warn"`, `"address": "1.1.1.1"`, `"final": "direct"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("merged output should contain %s: %s", want, out)
		}
	}
	// 白名单外的 Mihomo 残留字段绝不能透出（sing-box 严格解码会整份拒载）
	for _, forbidden := range []string{"mixed-port", "proxy-groups"} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("output must not contain non-allowlisted key %q: %s", forbidden, out)
		}
	}
}

// _disabled: true 的段视为停用；空对象跳过；无可用段时原样返回。
func TestMergeFormatSectionsSkipsDisabledAndEmpty(t *testing.T) {
	cfg := &entity.MihomoConfig{ConfigJSON: `{
		"log": {"level": "warn", "_disabled": true},
		"dns": {},
		"experimental": {"cache_file": {"enabled": true}}
	}`}
	base := []byte(`{"outbounds":[]}`)
	out := string(mergeFormatSections(base, cfg, "log", "dns", "experimental"))
	if strings.Contains(out, "warn") {
		t.Fatalf("_disabled section must be skipped: %s", out)
	}
	if !strings.Contains(out, "cache_file") {
		t.Fatalf("enabled section should be merged: %s", out)
	}

	// 启用段里的内部标记 `_disabled` 必须被剥掉（sing-box 严格解码会整份拒载）
	marked := &entity.MihomoConfig{ConfigJSON: `{"dns": {"servers": [{"type":"udp","server":"8.8.8.8"}], "_disabled": false}}`}
	outMarked := string(mergeFormatSections(base, marked, "dns"))
	if strings.Contains(outMarked, "_disabled") {
		t.Fatalf("internal _disabled marker must be stripped from output: %s", outMarked)
	}
	if !strings.Contains(outMarked, "8.8.8.8") {
		t.Fatalf("enabled section content should survive stripping: %s", outMarked)
	}

	// 无任何可用段 → 返回原数据（不做无谓的重新序列化）
	empty := &entity.MihomoConfig{ConfigJSON: `{}`}
	orig := []byte(`{"outbounds":[]}`)
	if got := mergeFormatSections(orig, empty, "log", "dns"); string(got) != string(orig) {
		t.Fatalf("no matching section should return input unchanged, got %s", got)
	}
}

// experimental.v2ray_api 是节点侧给面板抓计数的通道，需要 with_v2ray_api 构建；
// 官方客户端带上会直接启动失败 —— 订阅输出里必须剔除，clash_api 要保留。
func TestMergeFormatSectionsDropsV2RayAPIFromClientOutput(t *testing.T) {
	cfg := &entity.MihomoConfig{ConfigJSON: `{"experimental":{
		"clash_api":{"external_controller":"127.0.0.1:9090"},
		"v2ray_api":{"listen":"127.0.0.1:10086","stats":{"enabled":true}}
	}}`}
	base := []byte(`{"outbounds":[]}`)
	out := string(mergeFormatSections(base, cfg, "experimental"))

	if strings.Contains(out, "v2ray_api") || strings.Contains(out, "10086") {
		t.Fatalf("v2ray_api must not reach client subscription: %s", out)
	}
	if !strings.Contains(out, "clash_api") || !strings.Contains(out, "9090") {
		t.Fatalf("clash_api should survive: %s", out)
	}

	// 只剩 v2ray_api 的 experimental → 整段无意义，不写空段
	only := &entity.MihomoConfig{ConfigJSON: `{"experimental":{"v2ray_api":{"listen":"127.0.0.1:10086"}}}`}
	orig := []byte(`{"outbounds":[]}`)
	if got := mergeFormatSections(orig, only, "experimental"); string(got) != string(orig) {
		t.Fatalf("v2ray_api-only experimental should be dropped entirely, got %s", got)
	}
}

// injectRuleSets：rule_set 必须落在 route 段内（sing-box 1.15 顶层写会 FATAL），
// 且兼容历史顶层旧写法读取、按 tag 去重。
func TestInjectRuleSetsPlacesUnderRoute(t *testing.T) {
	cfg := &entity.MihomoConfig{ConfigJSON: `{"route":{"rule_set":[
		{"type":"remote","tag":"geosite-cn","format":"binary","url":"https://example.com/a.srs"}
	]}}`}
	base := []byte(`{"outbounds":[{"type":"direct","tag":"direct"}]}`)
	out := string(injectRuleSets(base, cfg))

	if strings.Contains(out, `"rule_set"`) == false {
		t.Fatalf("route.rule_set should be injected: %s", out)
	}
	// 顶层绝不能出现 rule_set：用 JSON 结构断言而非子串（子串会命中 route 内的那处）
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("output should be valid JSON: %v", err)
	}
	if _, topLevel := parsed["rule_set"]; topLevel {
		t.Fatalf("rule_set must not be written at top level: %s", out)
	}
	route, _ := parsed["route"].(map[string]interface{})
	if route == nil {
		t.Fatalf("route section should exist: %s", out)
	}
	if _, ok := route["rule_set"].([]interface{}); !ok {
		t.Fatalf("route.rule_set should be an array: %s", out)
	}

	// 历史顶层旧写法仍应被读取并搬到 route 内
	legacy := &entity.MihomoConfig{ConfigJSON: `{"rule_set":[{"type":"local","tag":"legacy","path":"rules/x.json"}]}`}
	outLegacy := string(injectRuleSets(base, legacy))
	var parsedLegacy map[string]interface{}
	if err := json.Unmarshal([]byte(outLegacy), &parsedLegacy); err != nil {
		t.Fatalf("legacy output should be valid JSON: %v", err)
	}
	routeLegacy, _ := parsedLegacy["route"].(map[string]interface{})
	if routeLegacy == nil {
		t.Fatalf("legacy rule_set should be migrated into route: %s", outLegacy)
	}
	if arr, _ := routeLegacy["rule_set"].([]interface{}); len(arr) != 1 {
		t.Fatalf("legacy rule_set should land in route.rule_set: %s", outLegacy)
	}
}
