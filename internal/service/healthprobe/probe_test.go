package healthprobe

import (
	"testing"
	"time"
)

// 连续失败 3 次才判不健康，成功一次立即恢复；避免一次网络抖动就把优选 IP 下线。
func TestNextStateThreshold(t *testing.T) {
	healthy, fail := nextState(false, 0)
	if !healthy || fail != 1 {
		t.Fatalf("第 1 次失败应仍健康：healthy=%v fail=%d", healthy, fail)
	}
	healthy, fail = nextState(false, 1)
	if !healthy || fail != 2 {
		t.Fatalf("第 2 次失败应仍健康：healthy=%v fail=%d", healthy, fail)
	}
	healthy, fail = nextState(false, 2)
	if healthy || fail != 3 {
		t.Fatalf("第 3 次连续失败应判不健康：healthy=%v fail=%d", healthy, fail)
	}
	healthy, fail = nextState(true, 3)
	if !healthy || fail != 0 {
		t.Fatalf("成功应恢复健康并清零：healthy=%v fail=%d", healthy, fail)
	}
}

func TestFirstPort(t *testing.T) {
	if got := firstPort("", 443); got != 443 {
		t.Fatalf("空端口列表应回落 443，got %d", got)
	}
	if got := firstPort("[8443,443]", 0); got != 8443 {
		t.Fatalf("应取第一个端口，got %d", got)
	}
	if got := firstPort("[]", 0); got != 443 {
		t.Fatalf("空数组应回落 443，got %d", got)
	}
}

// 延迟必须向上取整且至少 1ms：截断成 0 会让「延迟阈值」失效，界面上也像没探测到。
func TestElapsedMsNeverZero(t *testing.T) {
	if got := elapsedMs(time.Now()); got < 1 {
		t.Fatalf("elapsedMs = %d, want >= 1", got)
	}
	if got := elapsedMs(time.Now().Add(-1500 * time.Microsecond)); got != 2 {
		t.Fatalf("1.5ms 应向上取整为 2ms，got %d", got)
	}
}

// 自动模式按 sing-box → xray → mihomo 顺序取第一个已安装核心；都没装返回空。
func TestFirstAvailableDialer(t *testing.T) {
	if got := firstAvailableDialer([]string{"xray", "mihomo"}); got != "xray" {
		t.Fatalf("应取列表中第一个可用核心，got %q", got)
	}
	if got := firstAvailableDialer([]string{"", "mihomo"}); got != "mihomo" {
		t.Fatalf("应跳过空项，got %q", got)
	}
	if got := firstAvailableDialer(nil); got != "" {
		t.Fatalf("没有可用核心应返回空，got %q", got)
	}
}

func TestDialerDisplayName(t *testing.T) {
	cases := map[string]string{"singbox": "sing-box", "xray": "xray", "mihomo": "mihomo", "clash": "clash"}
	for in, want := range cases {
		if got := dialerDisplayName(in); got != want {
			t.Errorf("dialerDisplayName(%q) = %q, want %q", in, got, want)
		}
	}
}
