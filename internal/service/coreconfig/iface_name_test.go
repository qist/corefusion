package coreconfig

import (
	"strings"
	"testing"
)

// 线上踩过的坑：默认名 `WireGuard-<6位>` 是 16 字符，开启「使用系统网络集成」后
// 核心直接 FATAL：`open tun: create ifreq: invalid argument`。
func TestNormalizeIfaceNameRejectsTooLong(t *testing.T) {
	got := NormalizeIfaceName("WireGuard-ul7am9", "wg-ep-ul7am9")
	if got != "wg-ep-ul7am9" {
		t.Fatalf("超长名字应回退到 tag，实际 %q", got)
	}
	if len(got) > maxIfaceNameLen {
		t.Fatalf("结果必须 ≤%d 字节，实际 %d", maxIfaceNameLen, len(got))
	}
}

func TestNormalizeIfaceNameKeepsValidName(t *testing.T) {
	for _, name := range []string{"wg0", "wg-ul7am9", "WARP_1", "ts.ep"} {
		if got := NormalizeIfaceName(name, "fallback"); got != name {
			t.Fatalf("合法名字 %q 不应被改写，实际 %q", name, got)
		}
	}
}

func TestNormalizeIfaceNameSanitizesIllegalChars(t *testing.T) {
	cases := map[string]string{
		"中文网卡":  "",      // 全是非法字符 → 回退 tag
		"my wg": "my-wg", // 空格 → -
		"wg/0":  "wg-0",  // 斜杠 → -
		" wg0 ": "wg0",   // 首尾空白
		"-wg0-": "wg0",   // 首尾分隔符
		"a..b":  "a..b",  // 点号在内核里合法
		"wg0\n": "wg0",   // 换行（配置里手填时会出现）
	}
	for in, want := range cases {
		got := NormalizeIfaceName(in, "wg-ep-abc123")
		if want == "" {
			if got != "wg-ep-abc123" {
				t.Fatalf("%q 应回退 tag，实际 %q", in, got)
			}
			continue
		}
		if got != want {
			t.Fatalf("%q 应规范成 %q，实际 %q", in, want, got)
		}
	}
}

func TestNormalizeIfaceNameRejectsReservedAndEmpty(t *testing.T) {
	for _, in := range []string{"", "lo", ".", ".."} {
		got := NormalizeIfaceName(in, "wg-ep-abc123")
		if got != "wg-ep-abc123" {
			t.Fatalf("%q 应回退 tag，实际 %q", in, got)
		}
	}
	// tag 也不可用（超长）时截断
	got := NormalizeIfaceName("WireGuard-ul7am9", "warp-endpoint-name-too-long")
	if got != "warp-endpoint-n" {
		t.Fatalf("tag 应截断到 15 字节，实际 %q", got)
	}
}

// 兜底哈希必须确定：同一次渲染两次调用不能生成不同网卡名。
func TestNormalizeIfaceNameFallbackIsStable(t *testing.T) {
	a := NormalizeIfaceName("中文", "中文标签")
	b := NormalizeIfaceName("中文", "中文标签")
	if a != b {
		t.Fatalf("兜底名不稳定：%q vs %q", a, b)
	}
	if !strings.HasPrefix(a, "wg") || len(a) > maxIfaceNameLen {
		t.Fatalf("兜底名不合法：%q", a)
	}
}
