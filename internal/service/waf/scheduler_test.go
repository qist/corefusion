package waf

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeSettingStore struct {
	vals map[string]string
}

func (f *fakeSettingStore) GetByKey(_ context.Context, key string) (string, error) {
	if f.vals == nil {
		return "", errors.New("not found")
	}
	v, ok := f.vals[key]
	if !ok {
		return "", errors.New("not found")
	}
	return v, nil
}

func (f *fakeSettingStore) Set(_ context.Context, key, value string) error {
	if f.vals == nil {
		f.vals = map[string]string{}
	}
	f.vals[key] = value
	return nil
}

func TestParseSchedule(t *testing.T) {
	cases := []struct {
		cron string
		want scheduleSpec
		ok   bool
	}{
		{"@every 24h", scheduleSpec{every: 24 * time.Hour}, true},
		{"@every 30m", scheduleSpec{every: 30 * time.Minute}, true},
		{"0 * * * *", scheduleSpec{hourlyStep: 1}, true},
		{"0 */6 * * *", scheduleSpec{hourlyStep: 6}, true},
		{"0 3 * * *", scheduleSpec{daily: true, hour: 3}, true},
		{"0 0 * * *", scheduleSpec{daily: true, hour: 0}, true},
		{"0 0 * * 0", scheduleSpec{every: 7 * 24 * time.Hour}, true},
		{"@every abc", scheduleSpec{}, false},
		{"5 3 * * *", scheduleSpec{}, false}, // 不支持非整点分钟
		{"0 3 1 * *", scheduleSpec{}, false}, // 不支持按日期
		{"", scheduleSpec{}, false},
	}
	for _, c := range cases {
		got, ok := parseSchedule(c.cron)
		if ok != c.ok {
			t.Errorf("%q: ok=%v want %v", c.cron, ok, c.ok)
			continue
		}
		if ok && got != c.want {
			t.Errorf("%q: got %+v want %+v", c.cron, got, c.want)
		}
	}
}

func mustZone(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	return loc
}

// 每日 03:00 必须按**本地墙上时间**对齐，而不是「启动后 24 小时」。
func TestNextRunDailyWallClock(t *testing.T) {
	loc := mustZone(t, "Asia/Shanghai")
	spec := scheduleSpec{daily: true, hour: 3}

	// 当天 01:00，上次同步 2 小时前（未错过窗口）→ 今天 03:00
	now := time.Date(2026, 11, 20, 1, 0, 0, 0, loc)
	last := now.Add(-2 * time.Hour)
	if got := nextRun(spec, now, last, 15*time.Second); !got.Equal(time.Date(2026, 11, 20, 3, 0, 0, 0, loc)) {
		t.Fatalf("got %v, want 2026-11-20 03:00", got.In(loc))
	}

	// 当天 10:00 → 明天 03:00
	now = time.Date(2026, 11, 20, 10, 0, 0, 0, loc)
	last = now.Add(-2 * time.Hour)
	if got := nextRun(spec, now, last, 15*time.Second); !got.Equal(time.Date(2026, 11, 21, 3, 0, 0, 0, loc)) {
		t.Fatalf("got %v, want 2026-11-21 03:00", got.In(loc))
	}
}

// 面板频繁重启导致上次同步已错过窗口：启动后应尽快补一次，而不是再等 24 小时。
func TestNextRunCatchUpWhenWindowMissed(t *testing.T) {
	loc := mustZone(t, "Asia/Shanghai")
	now := time.Date(2026, 11, 20, 10, 0, 0, 0, loc)
	delay := 15 * time.Second

	daily := scheduleSpec{daily: true, hour: 3}
	if got := nextRun(daily, now, now.Add(-30*time.Hour), delay); !got.Equal(now.Add(delay)) {
		t.Fatalf("daily overdue: got %v, want %v", got, now.Add(delay))
	}

	every := scheduleSpec{every: 24 * time.Hour}
	if got := nextRun(every, now, now.Add(-25*time.Hour), delay); !got.Equal(now.Add(delay)) {
		t.Fatalf("@every overdue: got %v, want %v", got, now.Add(delay))
	}

	// 从未同步过（新装/刚从旧版本升级）也应补一次
	if got := nextRun(every, now, time.Time{}, delay); !got.Equal(now.Add(delay)) {
		t.Fatalf("never synced: got %v, want %v", got, now.Add(delay))
	}
}

// @every 的基准是「上次同步」而不是「本次启动」：重启不会重置计时。
func TestNextRunEveryUsesLastRun(t *testing.T) {
	loc := mustZone(t, "Asia/Shanghai")
	now := time.Date(2026, 11, 20, 10, 0, 0, 0, loc)
	last := now.Add(-20 * time.Hour)
	spec := scheduleSpec{every: 24 * time.Hour}
	got := nextRun(spec, now, last, 15*time.Second)
	if want := last.Add(24 * time.Hour); !got.Equal(want) {
		t.Fatalf("got %v, want %v（4 小时后，而不是 24 小时后）", got, want)
	}
}

func TestNextRunHourlyStep(t *testing.T) {
	loc := mustZone(t, "Asia/Shanghai")
	now := time.Date(2026, 11, 20, 7, 30, 0, 0, loc)
	last := now.Add(-30 * time.Minute)
	spec := scheduleSpec{hourlyStep: 6}
	got := nextRun(spec, now, last, 15*time.Second)
	if want := time.Date(2026, 11, 20, 12, 0, 0, 0, loc); !got.Equal(want) {
		t.Fatalf("got %v, want %v", got.In(loc), want)
	}
}

// 上次同步时间要落库：面板重启后 loadLastRun 能恢复，计时不从头开始。
func TestSchedulerPersistsLastRun(t *testing.T) {
	store := &fakeSettingStore{}
	svc := &Service{settingSvc: store}
	s := NewScheduler(svc)

	loc := mustZone(t, "Asia/Shanghai")
	at := time.Date(2026, 11, 20, 10, 0, 0, 0, loc)
	s.recordRun(at, true, "ok")

	if raw := store.vals[wafLastSyncSettingKey]; raw != at.Format(time.RFC3339) {
		t.Fatalf("persisted = %q, want %q", raw, at.Format(time.RFC3339))
	}

	s2 := NewScheduler(&Service{settingSvc: store})
	s2.loadLastRun()
	if !s2.lastRunAt.Equal(at) {
		t.Fatalf("loaded = %v, want %v", s2.lastRunAt, at)
	}
}

// 调度时区必须跟随面板设置 panel.timezone（初始化向导里选的），
// 而不是默默用服务器时区 —— 否则「每天 03:00」会在错误的时刻执行。
func TestSchedulerLocationFollowsPanelTimezone(t *testing.T) {
	store := &fakeSettingStore{vals: map[string]string{"panel.timezone": "Asia/Tokyo"}}
	s := NewScheduler(&Service{settingSvc: store})
	if loc := s.resolveLocation(); loc == nil || loc.String() != "Asia/Tokyo" {
		t.Fatalf("got %v, want Asia/Tokyo", loc)
	}

	// 非法时区名回落，不能让调度器停摆
	store.vals["panel.timezone"] = "Mars/Olympus"
	s2 := NewScheduler(&Service{settingSvc: store})
	if loc := s2.resolveLocation(); loc == nil {
		t.Fatal("invalid timezone should fall back to server local time")
	}
}
