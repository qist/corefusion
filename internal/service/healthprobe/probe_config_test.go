package healthprobe

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

func newProbeTestService(t *testing.T) *Prober {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "probe.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&entity.SystemSetting{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return New(nil, nil, repository.NewSystemSettingRepository(db), nil, nil, t.TempDir())
}

// 探测默认关闭：没配置过（或键不存在）时不能自动开始定时探测。
func TestReadConfigProbeDisabledByDefault(t *testing.T) {
	p := newProbeTestService(t)
	cfg := p.ReadConfig(context.Background())
	if cfg.Enabled {
		t.Fatalf("默认应为关闭，实际 enabled=true")
	}
	if cfg.IntervalMinutes != DefaultIntervalMinutes {
		t.Fatalf("默认间隔 = %d, want %d", cfg.IntervalMinutes, DefaultIntervalMinutes)
	}
}

// 保存开关后立即读回新值（调度器会被 Wake 唤醒，不必等上一轮间隔）。
func TestSaveConfigTogglesProbe(t *testing.T) {
	p := newProbeTestService(t)
	ctx := context.Background()

	on := true
	if err := p.SaveConfig(ctx, &on, nil, nil, nil, nil); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if cfg := p.ReadConfig(ctx); !cfg.Enabled {
		t.Fatalf("开启后应 enabled=true")
	}

	off := false
	if err := p.SaveConfig(ctx, &off, nil, nil, nil, nil); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if cfg := p.ReadConfig(ctx); cfg.Enabled {
		t.Fatalf("关闭后应 enabled=false")
	}
}

// 间隔/阈值等其它字段的保存不受开关影响。
func TestSaveConfigKeepsOtherFields(t *testing.T) {
	p := newProbeTestService(t)
	ctx := context.Background()

	on := true
	interval := 30
	maxLatency := 800
	entryNode := int64(7)
	dialer := "xray"
	if err := p.SaveConfig(ctx, &on, &interval, &maxLatency, &entryNode, &dialer); err != nil {
		t.Fatalf("save: %v", err)
	}
	cfg := p.ReadConfig(ctx)
	if !cfg.Enabled || cfg.IntervalMinutes != 30 || cfg.MaxLatencyMs != 800 || cfg.EntryNodeID != 7 || cfg.Dialer != "xray" {
		t.Fatalf("配置读回不一致: %+v", cfg)
	}
}

func TestParseBoolSetting(t *testing.T) {
	cases := []struct {
		in   string
		def  bool
		want bool
	}{
		{"true", false, true},
		{"1", false, true},
		{"on", false, true},
		{"false", true, false},
		{"0", true, false},
		{"", true, true},
		{"", false, false},
		{"garbage", true, true},
	}
	for _, c := range cases {
		if got := parseBoolSetting(c.in, c.def); got != c.want {
			t.Errorf("parseBoolSetting(%q, %v) = %v, want %v", c.in, c.def, got, c.want)
		}
	}
}
