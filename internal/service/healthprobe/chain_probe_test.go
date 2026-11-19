package healthprobe

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// 判定标准是「HTTP 200 + 上下行都有字节」：必须真的把请求写出去、把响应读回来，
// 只拿到状态码不算（有的失败场景是连接被对端半关闭，状态码都不会有）。
func TestProbeChainHTTPCountsTraffic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello-from-probe"))
	}))
	defer srv.Close()

	status, up, down, latency, err := probeChainHTTP(context.Background(), nil, srv.URL)
	if err != nil {
		t.Fatalf("probe failed: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if up <= 0 || down <= 0 {
		t.Fatalf("应统计到上下行字节：up=%d down=%d", up, down)
	}
	if latency <= 0 {
		t.Fatalf("latency = %d, want > 0", latency)
	}
}

// 目标不可达时必须返回错误（不能因为"连上了但没数据"就判正常）。
func TestProbeChainHTTPFailsOnClosedPort(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // 端口已关闭

	if _, _, _, _, err := probeChainHTTP(context.Background(), nil, url); err == nil {
		t.Fatal("closed port must return error")
	}
}
