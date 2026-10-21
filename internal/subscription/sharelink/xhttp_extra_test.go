package sharelink

import (
	"net/url"
	"strings"
	"testing"
)

// xhttpAdvancedFields 是 xhttp 的"填充 / 会话 / 上行"定制字段集合。
//
// 背景：此前曾把这一组字段判为"会被静默丢掉"（A 类），
// 依据是一个**并不存在的 `internal/configgen/xray` 强类型生成器** ✗。逐项核对后纠正：
// 服务端配置走 `prepare.go` 的 map 透传（不丢），客户端分享链接走下面这个白名单
// （`buildXhttpExtra`），而这 11 个字段**全都在白名单里**。
//
// 这个测试就是那条结论的"防线"：白名单是**手工维护**的，将来重构时若漏掉某个字段，
// 用户配了却拿不到，会立刻在这里失败。
//
// 注意白名单会做一次**重命名**：`uplinkHttpMethod` → `uplinkHTTPMethod`（对齐 xray 的 JSON tag），
// 断言要按输出名来（见 xhttpOutputKeyAlias）。
// 白名单会做一次重命名（与 xray 的 JSON tag 对齐），断言按输出名来。
var xhttpOutputKeyAlias = map[string]string{
	"uplinkHttpMethod": "uplinkHTTPMethod",
}

func xhttpOutputKey(src string) string {
	if alias, ok := xhttpOutputKeyAlias[src]; ok {
		return alias
	}
	return src
}

var xhttpAdvancedFields = map[string]interface{}{
	"xPaddingBytes":       "100-200",
	"xPaddingObfsMode":    true,
	"xPaddingKey":         "pk",
	"xPaddingHeader":      "ph",
	"xPaddingMethod":      "pm",
	"xPaddingPlacement":   "pp",
	"seqKey":              "sk",
	"seqPlacement":        "sp",
	"sessionIDKey":        "sidk",
	"sessionIDPlacement":  "sidp",
	"uplinkDataKey":       "udk",
	"uplinkDataPlacement": "udp",
	"uplinkChunkSize":     float64(4096),
	"uplinkHttpMethod":    "POST",
}

// TestBuildXhttpExtraKeepsAdvancedFields 直接锁住白名单：每个字段都要出现在 extra 里。
func TestBuildXhttpExtraKeepsAdvancedFields(t *testing.T) {
	xhttp := map[string]interface{}{"path": "/xh", "mode": "auto"}
	for k, v := range xhttpAdvancedFields {
		xhttp[k] = v
	}
	extra := buildXhttpExtra(xhttp)
	if extra == "" {
		t.Fatalf("buildXhttpExtra 返回空（字段全被丢了）")
	}
	for k := range xhttpAdvancedFields {
		if want := xhttpOutputKey(k); !strings.Contains(extra, want) {
			t.Errorf("extra 里缺少字段 %q\n  extra=%s", want, extra)
		}
	}
}

// TestXhttpExtraReachesShareLink 端到端：字段要真的进入用户拿到的分享链接（extra 查询参数里）。
func TestXhttpExtraReachesShareLink(t *testing.T) {
	ss := map[string]interface{}{
		"network":       "xhttp",
		"security":      "none",
		"xhttpSettings": buildXhttpMap(),
	}
	link := Generate(Node{
		Tag: "x", Protocol: "vless", Host: "1.2.3.4", Port: 443,
		StreamSettings: ss,
		Settings: map[string]interface{}{
			"clients": []interface{}{map[string]interface{}{"id": "11111111-1111-1111-1111-111111111111"}},
		},
	})
	if link == "" {
		t.Fatal("生成空链接")
	}
	u, err := url.Parse(link)
	if err != nil {
		t.Fatalf("链接不是合法 URL: %v (%s)", err, link)
	}
	extra := u.Query().Get("extra")
	if extra == "" {
		t.Fatalf("链接里没有 extra 参数（xhttp 定制字段没下发）: %s", link)
	}
	for k := range xhttpAdvancedFields {
		if want := xhttpOutputKey(k); !strings.Contains(extra, want) {
			t.Errorf("extra 里缺少 %q\n  extra=%s", want, extra)
		}
	}
}

func buildXhttpMap() map[string]interface{} {
	m := map[string]interface{}{"path": "/xh", "host": "example.com", "mode": "auto"}
	for k, v := range xhttpAdvancedFields {
		m[k] = v
	}
	return m
}
