package subscriptions

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"corefusion/internal/model/entity"
	"corefusion/internal/subscription/formatter"
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
		"hysteria2://i%2Bcvb54Txqci333NMLE%2BxWoyGD13hLkzrRZxXaV8qYo%3D@www.example.com:51071?alpn=h3&fm=%7B%22quicParams%22%3A%7B%22udpHop%22%3A%7B%22interval%22%3A%225-10%22%2C%22ports%22%3A%2240000-50000%22%7D%7D%2C%22udp%22%3A%5B%7B%22settings%22%3A%7B%22password%22%3A%222k7wdHOubCfS%22%7D%2C%22type%22%3A%22salamander%22%7D%5D%7D&fp=chrome&mport=40000-50000&obfs=salamander&obfs-password=2k7wdHOubCfS&security=tls&sni=www.example.com#inbound-hysteria-51071",
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
		"hysteria2://i%2Bcvb54Txqci333NMLE%2BxWoyGD13hLkzrRZxXaV8qYo%3D@www.example.com:51071?alpn=h3&fm=%7B%22quicParams%22%3A%7B%22udpHop%22%3A%7B%22interval%22%3A%225-10%22%2C%22ports%22%3A%2240000-50000%22%7D%7D%2C%22udp%22%3A%5B%7B%22settings%22%3A%7B%22password%22%3A%222k7wdHOubCfS%22%7D%2C%22type%22%3A%22salamander%22%7D%5D%7D&fp=chrome&mport=40000-50000&obfs=salamander&obfs-password=2k7wdHOubCfS&security=tls&sni=www.example.com#inbound-hysteria-51071",
	}

	nodes := parseSubscriptionLinksToFormatterNodes(links)
	data, err := formatter.GetFormatter(formatter.FormatXrayJSON).Format(nodes)
	if err != nil {
		t.Fatalf("format xray json: %v", err)
	}
	output := string(data)

	for _, want := range []string{
		"\"address\": \"www.example.com\"",
		"\"network\": \"kcp\"",
		"\"kcpSettings\": {",
		"\"seed\": \"SwOzlkxeZpKaCxWysYucYQ==\"",
		"\"type\": \"wireguard\"",
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
