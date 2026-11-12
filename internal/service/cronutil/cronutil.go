// Package cronutil 提供面板里统一使用的 cron 解析。
//
// 面板只支持一个**受限子集**（不引入 cron 库，避免与服务器本地时区/描述符差异）：
//   - "@every 1h" / "@every 30m"
//   - 5 段 cron 的分、时字段：`*`、`*/N`、单个数字
//   - 日/月必须为 `*`（或 `?`），周可为 `*` 或单个 0-7（0/7 = 周日）
//
// 日志轮转（logmanage）与数据库日志清理（traffic.LogCleanupService）共用同一套语义，
// 避免两处对 "0 */6 * * *" 的解析/时区理解不一致。
package cronutil

import (
	"strconv"
	"strings"
	"time"
)

// NextRotation 返回 after 之后（严格晚于）的下一个计划执行时间。
// 返回 false 表示表达式不受支持/非法。
func NextRotation(cron string, after time.Time) (time.Time, bool) {
	cron = strings.TrimSpace(cron)
	if strings.HasPrefix(cron, "@every ") {
		d, err := time.ParseDuration(strings.TrimSpace(cron[len("@every "):]))
		if err != nil || d <= 0 {
			return time.Time{}, false
		}
		return after.Add(d), true
	}

	fields := strings.Fields(cron)
	if len(fields) != 5 {
		return time.Time{}, false
	}
	// 日/月不支持指定（UI 未提供），必须通配。
	if fields[2] != "*" && fields[2] != "?" {
		return time.Time{}, false
	}
	if fields[3] != "*" && fields[3] != "?" {
		return time.Time{}, false
	}
	weekdays, weekdaySet, ok := parseWeekday(fields[4])
	if !ok {
		return time.Time{}, false
	}
	minutes, ok := parseField(fields[0], 0, 59)
	if !ok {
		return time.Time{}, false
	}
	hours, ok := parseField(fields[1], 0, 23)
	if !ok {
		return time.Time{}, false
	}

	// 从下一分钟开始逐分钟找（最多 400 天；正常表达式几秒内就命中）。
	t := after.Truncate(time.Minute).Add(time.Minute)
	limit := t.Add(400 * 24 * time.Hour)
	for t.Before(limit) {
		if hours[t.Hour()] && minutes[t.Minute()] {
			if !weekdaySet || weekdays[int(t.Weekday())] {
				return t, true
			}
		}
		t = t.Add(time.Minute)
	}
	return time.Time{}, false
}

// parseField 解析分/时字段：`*`、`*/N`、单个数字。
func parseField(field string, min, max int) (map[int]bool, bool) {
	out := make(map[int]bool)
	switch {
	case field == "*" || field == "?":
		for i := min; i <= max; i++ {
			out[i] = true
		}
		return out, true
	case strings.HasPrefix(field, "*/"):
		n, err := strconv.Atoi(field[2:])
		if err != nil || n <= 0 {
			return nil, false
		}
		for i := min; i <= max; i++ {
			if i%n == 0 {
				out[i] = true
			}
		}
		return out, len(out) > 0
	default:
		n, err := strconv.Atoi(field)
		if err != nil || n < min || n > max {
			return nil, false
		}
		out[n] = true
		return out, true
	}
}

// parseWeekday 解析周字段：`*` / `?` / 单个 0-7（0 和 7 都表示周日）。
// 第二个返回值表示是否限制了具体星期。
func parseWeekday(field string) (map[int]bool, bool, bool) {
	out := make(map[int]bool)
	if field == "*" || field == "?" {
		return out, false, true
	}
	n, err := strconv.Atoi(field)
	if err != nil || n < 0 || n > 7 {
		return nil, false, false
	}
	if n == 7 {
		n = 0
	}
	out[n] = true
	return out, true, true
}
