package netutil

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// 这些正是攻击者会填的地址：一旦服务端去抓，就等价于"把服务端当代理读内网/云元数据"
// （安全审计 v5 S1）。
func TestSafeFetchRejectsInternalTargets(t *testing.T) {
	ctx := context.Background()
	blocked := []string{
		"http://127.0.0.1:8080/",
		"http://localhost:8080/", // 解析到 127.0.0.1 / ::1
		"http://169.254.169.254/latest/meta-data/iam/security-credentials/", // 云元数据
		"http://10.0.0.1/",
		"http://192.168.1.1/",
		"http://172.16.0.1/",
		"http://[::1]:8080/",
		"http://[fd00::1]/",
		"http://0.0.0.0/",
	}
	for _, u := range blocked {
		_, err := SafeFetchGET(ctx, u, 2*time.Second, 1024)
		if err == nil {
			t.Errorf("SafeFetchGET(%q) 必须被拒（SSRF 防护失效）", u)
		}
	}
}

func TestSafeFetchRejectsNonHTTPScheme(t *testing.T) {
	ctx := context.Background()
	for _, u := range []string{
		"file:///etc/passwd",
		"gopher://127.0.0.1:6379/_INFO",
		"ftp://example.com/x",
		"http://",
	} {
		if _, err := SafeFetchGET(ctx, u, 2*time.Second, 1024); err == nil {
			t.Errorf("SafeFetchGET(%q) 必须被拒（协议校验失效）", u)
		}
	}
}

// 用本地服务器验证机制本身（allowPrivate 仅供测试；它同时也证明"内网拒绝"不是靠请求发不出去）。
func TestSafeFetchMechanicsWithAllowPrivate(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		switch r.URL.Path {
		case "/ok":
			_, _ = fmt.Fprint(w, "hello")
		case "/big":
			_, _ = w.Write(bytes.Repeat([]byte("x"), 5000))
		case "/redirect":
			http.Redirect(w, r, "/ok", http.StatusFound)
		case "/500":
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	client := NewSafeHTTPClient(5*time.Second, true)
	ctx := context.Background()

	// ① 正常抓取
	data, err := safeFetchGETWithClient(ctx, client, srv.URL+"/ok", 1024)
	if err != nil || string(data) != "hello" {
		t.Fatalf("正常抓取失败：data=%q err=%v", data, err)
	}

	// ② 响应体超限必须报错（且不能把超大内容读进内存）
	if _, err := safeFetchGETWithClient(ctx, client, srv.URL+"/big", 1024); err == nil ||
		!strings.Contains(err.Error(), "exceeds") {
		t.Errorf("超过 maxBytes 必须报错，得到 %v", err)
	}

	// ③ 不跟随重定向：请求数必须仍为 2（/ok + /big），说明 /redirect **没有**被跟随到 /ok
	before := atomic.LoadInt32(&hits)
	if _, err := safeFetchGETWithClient(ctx, client, srv.URL+"/redirect", 1024); err == nil ||
		!strings.Contains(err.Error(), "redirect") {
		t.Errorf("重定向必须拒绝，得到 %v", err)
	}
	if after := atomic.LoadInt32(&hits); after != before+1 {
		t.Errorf("重定向被跟随了（服务端多收到 %d 次请求）—— 302 到内网就能绕过全部校验", after-before-1)
	}

	// ④ 非 200 视为失败
	if _, err := safeFetchGETWithClient(ctx, client, srv.URL+"/500", 1024); err == nil ||
		!strings.Contains(err.Error(), "status") {
		t.Errorf("非 200 必须报错，得到 %v", err)
	}
}
