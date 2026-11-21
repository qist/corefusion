package logmanage

import "testing"

// 「实时日志」列表与「发现日志」同口径：未安装/未运行的来源不返回。
func TestGetWriterStatusSkipsUnavailableSources(t *testing.T) {
	svc := &Service{dataDir: t.TempDir()}
	svc.SetProcessAvailability(fakeAvailability{
		"nginx": true, "caddy": false, "waf": true, "xray": false, "singbox": true,
	})

	got := svc.GetWriterStatus()
	want := []string{"nginx", "waf", "singbox"}
	if len(got) != len(want) {
		t.Fatalf("sources = %+v, want %v", got, want)
	}
	for i, w := range want {
		if got[i].ProxyType != w {
			t.Errorf("sources[%d] = %q, want %q", i, got[i].ProxyType, w)
		}
	}
}

// 未注入可用性判定时保持旧行为：返回全部来源，避免装配遗漏时列表变空。
func TestGetWriterStatusWithoutAvailability(t *testing.T) {
	svc := &Service{dataDir: t.TempDir()}
	if got := svc.GetWriterStatus(); len(got) != len(allLogTypes) {
		t.Fatalf("expected all %d sources, got %d", len(allLogTypes), len(got))
	}
}
