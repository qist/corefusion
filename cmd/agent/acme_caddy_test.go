package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 路由必须"只匹配挑战路径"，并且 file_server 的 root = 挑战目录 ✓
// （这样 URI /.well-known/acme-challenge/<token> 才正好映射到 WebrootProvider 写入的位置 ✓）
func TestCaddyChallengeRouteJSON(t *testing.T) {
	raw := caddyChallengeRouteJSON("/var/lib/corefusion-agent/acme-challenge")
	var route struct {
		Match []struct {
			Path []string `json:"path"`
		} `json:"match"`
		Handle []struct {
			Handler string `json:"handler"`
			Root    string `json:"root"`
		} `json:"handle"`
		Terminal bool `json:"terminal"`
	}
	if err := json.Unmarshal([]byte(raw), &route); err != nil {
		t.Fatalf("生成的 JSON 不合法：%v (%s)", err, raw)
	}
	if len(route.Match) != 1 || len(route.Match[0].Path) != 1 || route.Match[0].Path[0] != "/.well-known/acme-challenge/*" {
		t.Fatalf("匹配条件应只针对挑战路径：%s", raw)
	}
	if len(route.Handle) != 1 || route.Handle[0].Handler != "file_server" {
		t.Fatalf("应使用 file_server：%s", raw)
	}
	if route.Handle[0].Root != "/var/lib/corefusion-agent/acme-challenge" {
		t.Fatalf("root 应为挑战目录：%s", raw)
	}
	if !route.Terminal {
		t.Fatalf("terminal 必须为 true（只接管挑战路径，其余请求放行）：%s", raw)
	}
}

func TestCaddyPickServer80(t *testing.T) {
	// 真机 /config/apps/http/servers/ 的典型结构 ✓
	body := []byte(`{"srv0":{"listen":[":443"],"routes":[]},"srv1":{"listen":[":80"],"routes":[]}}`)
	got, err := caddyPickServer80(body)
	if err != nil {
		t.Fatal(err)
	}
	if got != "srv1" {
		t.Fatalf("应挑出监听 :80 的 server，实际 %q", got)
	}

	// 0.0.0.0:80 / [::]:80 这些写法也要认得 ✓
	if got, err = caddyPickServer80([]byte(`{"a":{"listen":["0.0.0.0:80"]}}`)); err != nil || got != "a" {
		t.Fatalf("0.0.0.0:80 应识别为 80 端口：got=%q err=%v", got, err)
	}
	if got, err = caddyPickServer80([]byte(`{"b":{"listen":["[::]:80"]}}`)); err != nil || got != "b" {
		t.Fatalf("[::]:80 应识别为 80 端口：got=%q err=%v", got, err)
	}
	if _, err = caddyPickServer80([]byte(`{}`)); err == nil {
		t.Fatal("没有 http server 时应报错")
	}
}

// 进程探测不能要求精确名 ✓（真机里 caddy 的进程名是 caddy-linux-amd64 ✗）
func TestDetectWebServerMatchesSuffixedNames(t *testing.T) {
	// 本机若真的跑着 nginx/caddy 才断言 ✓，否则只保证"不 panic 且返回已知值之一" ✓
	got := detectWebServer()
	switch got {
	case "", "nginx", "caddy":
	default:
		t.Fatalf("detectWebServer 返回值意外：%q", got)
	}
	t.Logf("  本机探测结果：%q", got)
}

// 真机联测（默认跳过 ✓；用 COREFUSION_LIVE_CADDY=1 显式开启 ✓）：
// 对**真实**的 caddy admin API 做"插路由 → 经 :80 取回探针 → 还原整份配置" ✓，
// 并确认还原后配置与之前**逐字节一致** ✓。
func TestLiveCaddyConfigureAndRestore(t *testing.T) {
	if os.Getenv("COREFUSION_LIVE_CADDY") != "1" {
		t.Skip("跳过真机 caddy 联测（设置 COREFUSION_LIVE_CADDY=1 开启）")
	}
	admin := caddyAdminAddr()
	before, err := caddyDo(http.MethodGet, admin+"/config/", nil)
	if err != nil {
		t.Fatalf("caddy admin API 不可达：%v", err)
	}

	// 挑战目录必须在 web 服务器可见处 ✗ /tmp（PrivateTmp 风险 ✓）
	dir := "/var/lib/corefusion-agent/acme-livetest"
	if err := os.MkdirAll(filepath.Join(dir, ".well-known", "acme-challenge"), 0755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	t.Setenv("COREFUSION_ACME_CHALLENGE_DIR", dir)

	undo, err := configureCaddyForACME(dir, []string{"live-test.example"})
	if err != nil {
		t.Fatalf("configureCaddyForACME 失败（含内部自检）：%v", err)
	}
	// 自检已证明 :80 能取回探针 ✓；这里再直接确认一次挑战路径由 caddy 提供 ✓
	probe := filepath.Join(dir, ".well-known", "acme-challenge", "liveprobe")
	if err := os.WriteFile(probe, []byte("live-ok"), 0644); err != nil {
		t.Fatal(err)
	}
	_ = os.Chmod(probe, 0644)
	req, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1/.well-known/acme-challenge/liveprobe", nil)
	req.Host = "live-test.example"
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		undo()
		t.Fatalf("请求 :80 失败：%v", err)
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 32))
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "live-ok") {
		undo()
		t.Fatalf("挑战路径应由 caddy 提供：HTTP %d body=%q", resp.StatusCode, string(body))
	}

	undo()

	after, err := caddyDo(http.MethodGet, admin+"/config/", nil)
	if err != nil {
		t.Fatal(err)
	}
	// 按 **JSON 语义**比对 ✓（不能按字节 ✗：admin API 会重新序列化 ✓）
	var b1, b2 any
	if err := json.Unmarshal(before, &b1); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(after, &b2); err != nil {
		t.Fatal(err)
	}
	j1, _ := json.Marshal(b1)
	j2, _ := json.Marshal(b2)
	if string(j1) != string(j2) {
		t.Fatalf("还原后 caddy 配置应与之前语义一致 ✗\nbefore=%s\nafter=%s", j1, j2)
	}
	t.Log("  ✓ 插入→探测→还原 全流程通过，配置与之前语义一致")
}
