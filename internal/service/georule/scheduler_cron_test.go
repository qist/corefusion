package georule

import (
	"testing"
	"time"
)

// 任意「每天一次」的标准 cron 都必须能建任务（内置规则源用 "0 4 * * *"）。
func TestParseCronIntervalDaily(t *testing.T) {
	for _, cron := range []string{"0 4 * * *", "30 2 * * *", "0 3 * * *", "0 0 * * *"} {
		if got := parseCronInterval(cron); got != 24*time.Hour {
			t.Errorf("parseCronInterval(%q) = %v, want 24h", cron, got)
		}
	}
}

func TestParseCronIntervalPresets(t *testing.T) {
	cases := map[string]time.Duration{
		"@every 30m":   30 * time.Minute,
		"0 * * * *":    time.Hour,
		"0 */6 * * *":  6 * time.Hour,
		"0 */12 * * *": 12 * time.Hour,
		"0 0 * * 0":    7 * 24 * time.Hour,
	}
	for cron, want := range cases {
		if got := parseCronInterval(cron); got != want {
			t.Errorf("parseCronInterval(%q) = %v, want %v", cron, got, want)
		}
	}
}

func TestParseCronIntervalInvalid(t *testing.T) {
	for _, cron := range []string{"", "not a cron", "*/5 * * * *", "0 4 1 * *"} {
		if got := parseCronInterval(cron); got != 0 {
			t.Errorf("parseCronInterval(%q) = %v, want 0", cron, got)
		}
	}
}
