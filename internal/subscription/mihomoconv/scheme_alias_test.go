package mihomoconv

import "testing"

// 面板早期生成的分享链接是 shadowsocks://，转换器只认 ss:// 时整批 ss 节点
// 会被静默丢出 mihomo 订阅（用户侧表现：订阅里少了节点、弹窗里"无法生成片段"）。
func TestConvertsShadowsocksSchemeAliases(t *testing.T) {
	links := []string{
		"ss://YWVzLTI1Ni1nY206cHc=@1.2.3.4:443#t1",
		"shadowsocks://YWVzLTI1Ni1nY206cHc=@1.2.3.4:443#t2",
		"shadowsocks2022://YWVzLTI1Ni1nY206cHc=@1.2.3.4:443#t3",
	}
	for _, link := range links {
		proxies, err := ConvertsV2Ray([]byte(link))
		if err != nil || len(proxies) != 1 {
			t.Fatalf("%s: 转换失败 n=%d err=%v", link, len(proxies), err)
		}
		p := proxies[0]
		if p["type"] != "ss" || p["server"] != "1.2.3.4" || p["password"] != "pw" {
			t.Errorf("%s: 结果不对: %v", link, p)
		}
		if p["cipher"] != "aes-256-gcm" {
			t.Errorf("%s: cipher = %v, want aes-256-gcm", link, p["cipher"])
		}
	}
}
