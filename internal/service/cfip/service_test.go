package cfip

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"corefusion/pkg/netutil"
)

// TestFetchIPsRejectsLoopback 锁定 SSRF 防护（安全审计 v6 H-2）：
// CF IP 源的 URL 可被有写权限的账号配置，服务端抓取时**必须**拒绝环回/内网地址，
// 否则可读取云元数据（169.254.169.254）或探测内网服务。
//
// 用 127.0.0.1 上的 httptest server 作为「内网目标」：修复前（裸 http.Client）会真的
// 连上并返回 200；修复后必须在拨号层被拒（errors.Is ErrBlockedAddress）。
func TestFetchIPsRejectsLoopback(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		_, _ = w.Write([]byte("1.2.3.4\n5.6.7.8\n"))
	}))
	defer srv.Close()

	s := &Service{}
	_, err := s.fetchIPs(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("loopback URL must be rejected (SSRF protection)")
	}
	if !errors.Is(err, netutil.ErrBlockedAddress) {
		t.Fatalf("expected ErrBlockedAddress, got %v", err)
	}
	if called {
		t.Fatal("request reached the loopback server; SSRF protection did not work")
	}
}

// TestFetchIPsRejectsNonHTTPScheme：只允许 http/https（file://、gopher:// 等一律拒绝）。
func TestFetchIPsRejectsNonHTTPScheme(t *testing.T) {
	s := &Service{}
	if _, err := s.fetchIPs(context.Background(), "file:///etc/passwd"); err == nil {
		t.Fatal("non-http scheme must be rejected")
	}
}

// TestExtractIPs 保持原有解析行为（修复 SSRF 不应改变取 IP 的逻辑）。
func TestExtractIPs(t *testing.T) {
	got := extractIPs("1.2.3.4 5.6.7.8\n1.2.3.4\nnot-an-ip")
	want := []string{"1.2.3.4", "5.6.7.8"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}
