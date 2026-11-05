package subscriptions

import (
	"testing"

	"corefusion/internal/subscription/formatter"
)

// TestMasqueOnlyXrayFormat MASQUE 没有 URI 方案、mihomo 的 masque 只认 mTLS、
// sing-box 没有 masque 出站 —— 只有 xray_json 能表达，其它格式必须剔除。
func TestMasqueOnlyXrayFormat(t *testing.T) {
	if !snippetSupportMatrix("xray")["masque"] {
		t.Error("xray 格式应支持 masque")
	}
	for _, format := range []string{"uri", "mihomo", "singbox"} {
		if snippetSupportMatrix(format)["masque"] {
			t.Errorf("%s 格式不应支持 masque", format)
		}
	}
	for _, format := range []string{"base64_uri", "raw_uri", "mihomo_yaml", "singbox_json"} {
		if formatSupportedProtocols(format)["masque"] {
			t.Errorf("%s 格式不应支持 masque", format)
		}
	}
	if !formatSupportedProtocols("xray_json")["masque"] {
		t.Error("xray_json 应支持 masque")
	}
	if !shareableProtocols["masque"] {
		t.Error("masque 必须能被订阅渲染（否则永远不进订阅）")
	}
}

// TestMasqueNodeNotDroppedByXray xray 的剔除规则只针对明文 vless/trojan，
// masque 必须保留。
func TestMasqueNodeNotDroppedByXray(t *testing.T) {
	node := &formatter.Node{
		Name: "m", Protocol: "masque", Host: "1.2.3.4", Port: 443,
		Transport: "masque", TLS: true,
	}
	if reason := xrayNodeDropReason(node); reason != "" {
		t.Errorf("masque 不应被 xray 剔除，reason=%q", reason)
	}
}
