package logmanage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// WAF 引擎自己按天写 logs/<date>_waf.log；发现日志应枚举磁盘上真实存在的
// 日期文件（新→旧），而不是固定返回"今天"的路径（否则跨天后日志查看看不到历史）。
func TestDiscoverWAFLogsEnumeratesExistingDayFiles(t *testing.T) {
	dir := t.TempDir()
	logsDir := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"2026-11-06_waf.log",
		"2026-10-13_waf.log",
		"2026-11-06_waf.log.old",
		"nginx.log",
		"not-a-date_waf.log",
	} {
		if err := os.WriteFile(filepath.Join(logsDir, name), []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	svc := &Service{dataDir: dir}
	got, err := svc.discoverWAFLogs(context.Background())
	if err != nil {
		t.Fatalf("discoverWAFLogs: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 WAF day files, got %d: %+v", len(got), got)
	}
	if got[0].Domain != "2026-11-06" || got[1].Domain != "2026-10-13" {
		t.Fatalf("expected newest-first dates, got %q, %q", got[0].Domain, got[1].Domain)
	}
	for _, d := range got {
		if d.SourceType != "waf" || d.LogFormat != "json" {
			t.Fatalf("unexpected discovery entry: %+v", d)
		}
		if !filepath.IsAbs(d.LogPath) || filepath.Dir(d.LogPath) != logsDir {
			t.Fatalf("expected absolute path under logs dir, got %q", d.LogPath)
		}
	}
}

type fakeAvailability map[string]bool

func (f fakeAvailability) Available(sourceType string) bool { return f[sourceType] }

// 未安装/未运行的核心不应出现在发现日志里。
func TestDiscoverCoreLogsSkipsUnavailableCores(t *testing.T) {
	svc := &Service{dataDir: t.TempDir()}
	svc.SetProcessAvailability(fakeAvailability{"xray": false, "singbox": true})

	got, err := svc.discoverCoreLogs(context.Background())
	if err != nil {
		t.Fatalf("discoverCoreLogs: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected only the running core, got %+v", got)
	}
	if got[0].SourceType != "singbox" {
		t.Fatalf("expected singbox log, got %+v", got[0])
	}
}

func TestDiscoverWAFLogsNoFiles(t *testing.T) {
	svc := &Service{dataDir: t.TempDir()}
	got, err := svc.discoverWAFLogs(context.Background())
	if err != nil {
		t.Fatalf("discoverWAFLogs: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no WAF entries when no files exist, got %+v", got)
	}
}

// xray 的**进程日志**（logs/xray.log）必须能被发现并查看：
// 启动横幅、配置解析失败、端口占用这类"起不来"的原因只写在这里，
// xray 自己的 error 日志要先跑起来才会写 ✗（真机：这个文件一直没人看得到 ✓）。
func TestDiscoverCoreLogsIncludesXrayStartupLog(t *testing.T) {
	dir := t.TempDir()
	runtimeDir := filepath.Join(dir, "runtime")
	if err := os.MkdirAll(runtimeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `{"log":{"access":"logs/xray-access.log","error":"logs/xray-error.log"}}`
	if err := os.WriteFile(filepath.Join(runtimeDir, "xray.jsonc"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := &Service{dataDir: dir}
	svc.SetProcessAvailability(fakeAvailability{"xray": true})
	got, err := svc.discoverCoreLogs(context.Background())
	if err != nil {
		t.Fatalf("discoverCoreLogs: %v", err)
	}

	var startup *DiscoveredLog
	for i := range got {
		if got[i].Domain == "startup" {
			startup = &got[i]
		}
	}
	if startup == nil {
		t.Fatalf("缺少 xray 进程日志（startup），实际 %+v", got)
	}
	if filepath.Base(startup.LogPath) != "xray.log" {
		t.Fatalf("startup 应指向 logs/xray.log，实际 %q", startup.LogPath)
	}
	if !filepath.IsAbs(startup.LogPath) {
		t.Fatalf("日志路径应为绝对路径（前端直接读文件），实际 %q", startup.LogPath)
	}
}

// 运行时配置里 error 就是 logs/xray.log 时（默认值/旧配置）不能重复出现两条。
func TestDiscoverCoreLogsNoDuplicateWhenErrorIsProcessLog(t *testing.T) {
	dir := t.TempDir()
	svc := &Service{dataDir: dir}
	svc.SetProcessAvailability(fakeAvailability{"xray": true})
	got, err := svc.discoverCoreLogs(context.Background())
	if err != nil {
		t.Fatalf("discoverCoreLogs: %v", err)
	}
	seen := map[string]int{}
	for _, d := range got {
		seen[d.LogPath]++
	}
	for p, n := range seen {
		if n > 1 {
			t.Fatalf("同一个日志文件被发现多次（%q × %d）：%+v", p, n, got)
		}
	}
}
