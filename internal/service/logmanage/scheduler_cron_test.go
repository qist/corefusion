package logmanage

import (
	"testing"
	"time"
)

// 生产回归：面板一天重启多次，旧实现用 time.NewTicker(24h) 从进程启动计时，
// 永远走不到下一次 tick → 11-10 之后日志轮转再没执行过。
// 现在必须按墙上时钟算下一次，重启不会重置到“24 小时后”。
func TestNextRotationDailyIsWallClock(t *testing.T) {
	loc := time.Local
	// 11-11 17:41 启动，daily 0 0 * * * → 下一次必须是 11-12 00:00
	after := time.Date(2026, 11, 11, 17, 41, 0, 0, loc)
	next, ok := nextRotation("0 0 * * *", after)
	if !ok {
		t.Fatal("daily cron should be supported")
	}
	want := time.Date(2026, 11, 12, 0, 0, 0, 0, loc)
	if !next.Equal(want) {
		t.Fatalf("nextRotation(0 0 * * *) = %s, want %s", next, want)
	}

	// 00:00 整点启动：下一次是明天 00:00（严格晚于 after，避免重复执行）
	after = time.Date(2026, 11, 12, 0, 0, 0, 0, loc)
	next, _ = nextRotation("0 0 * * *", after)
	if want := time.Date(2026, 11, 13, 0, 0, 0, 0, loc); !next.Equal(want) {
		t.Fatalf("整点启动应排到次日，got %s want %s", next, want)
	}
}

func TestNextRotationIntervals(t *testing.T) {
	loc := time.Local
	base := time.Date(2026, 11, 11, 10, 20, 0, 0, loc)

	cases := []struct {
		cron string
		want time.Time
	}{
		{"0 */6 * * *", time.Date(2026, 11, 11, 12, 0, 0, 0, loc)},
		{"0 */12 * * *", time.Date(2026, 11, 11, 12, 0, 0, 0, loc)},
		{"0 * * * *", time.Date(2026, 11, 11, 11, 0, 0, 0, loc)},
		{"0 3 * * *", time.Date(2026, 11, 12, 3, 0, 0, 0, loc)},
		{"@every 1h", base.Add(time.Hour)},
		{"@every 30m", base.Add(30 * time.Minute)},
	}
	for _, c := range cases {
		got, ok := nextRotation(c.cron, base)
		if !ok {
			t.Fatalf("%q should be supported", c.cron)
		}
		if !got.Equal(c.want) {
			t.Errorf("%q => %s, want %s", c.cron, got, c.want)
		}
	}
}

// 周计划：0 0 * * 0 = 每周日 00:00。
func TestNextRotationWeekly(t *testing.T) {
	loc := time.Local
	// 2026-11-11 是周三
	after := time.Date(2026, 11, 11, 17, 41, 0, 0, loc)
	next, ok := nextRotation("0 0 * * 0", after)
	if !ok {
		t.Fatal("weekly cron should be supported")
	}
	if next.Weekday() != time.Sunday || next.Hour() != 0 || next.Minute() != 0 {
		t.Fatalf("weekly next = %s (%s), want Sunday 00:00", next, next.Weekday())
	}
	if !next.After(after) || next.Sub(after) > 7*24*time.Hour {
		t.Fatalf("weekly next 超出预期范围：%s", next)
	}
}

func TestNextRotationRejectsUnsupported(t *testing.T) {
	base := time.Date(2026, 11, 11, 10, 0, 0, 0, time.Local)
	for _, cron := range []string{"", "bad", "0 0 1 * *", "0 0 * *", "@every", "@every 0s", "61 * * * *", "0 24 * * *"} {
		if _, ok := nextRotation(cron, base); ok {
			t.Errorf("invalid cron %q should be rejected", cron)
		}
	}
}
