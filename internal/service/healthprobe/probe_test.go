package healthprobe

import "testing"

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
