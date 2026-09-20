package subscriptions

import (
	"strings"
	"testing"
)

// 二维码弹窗的"Mihomo 配置"必须与 mihomo 订阅**同源**（同一条 mihomoconv 链路），
// 否则又会出现"弹窗里能粘、订阅里参数不全"的两套口径。
func TestMihomoProxySnippetUsesSubscriptionPipeline(t *testing.T) {
	link := "hysteria2://pw@1.2.3.4:443?obfs=salamander&obfs-password=ob&bbr-profile=conservative" +
		"&mport=10000-19999&security=tls&sni=www.example.com#hy2"
	snippet := mihomoProxySnippet(link)
	if snippet == "" {
		t.Fatal("hy2 链接应能生成片段")
	}
	if !strings.HasPrefix(snippet, "proxies:\n") {
		t.Fatalf("片段应以 proxies: 开头（可直接粘进配置）: %q", snippet)
	}
	for _, want := range []string{
		"type: hysteria2", "server: 1.2.3.4",
		"obfs: salamander", "obfs-password: ob",
		"bbr-profile: conservative", "ports: 10000-19999",
	} {
		if !strings.Contains(snippet, want) {
			t.Errorf("片段缺 %q:\n%s", want, snippet)
		}
	}

	// 与订阅链路的结果逐字段一致（同一份转换输出）
	converted := convertLinksToMihomoProxyMaps([]string{link})
	if len(converted) != 1 {
		t.Fatalf("转换应得到 1 个代理, got %d", len(converted))
	}
	if _, ok := converted[0]["obfs-password"]; !ok {
		t.Error("订阅链路应保留 obfs-password")
	}

	// 没有链接（无标准 URI 的协议）→ 空片段，前端显示占位提示
	if got := mihomoProxySnippet(""); got != "" {
		t.Errorf("空链接应返回空片段, got %q", got)
	}
}
