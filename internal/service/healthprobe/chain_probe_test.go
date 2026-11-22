package healthprobe

import (
	"context"
	"fmt"
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

// 目标按 Kind 排序后，IP 多时轮转窗口全被 IP 占满，域名永远测不到。
// 交替排列后，任意长度 ≥2 的窗口都应同时覆盖两类。
func TestInterleaveByKindCoversBothKinds(t *testing.T) {
	targets := make([]ChainTarget, 0, 30)
	for i := 0; i < 25; i++ {
		targets = append(targets, ChainTarget{Kind: "cf_ip", Host: fmt.Sprintf("1.1.1.%d", i)})
	}
	for i := 0; i < 5; i++ {
		targets = append(targets, ChainTarget{Kind: "cf_domain", Host: fmt.Sprintf("d%d.example.com", i)})
	}
	out := interleaveByKind(targets)
	if len(out) != len(targets) {
		t.Fatalf("长度变了：%d → %d", len(targets), len(out))
	}
	// 模拟 max_targets=20 的轮转窗口
	picked := out[:20]
	ips, domains := 0, 0
	for _, tg := range picked {
		switch tg.Kind {
		case "cf_ip":
			ips++
		case "cf_domain":
			domains++
		}
	}
	if ips == 0 || domains == 0 {
		t.Fatalf("20 个窗口内应同时包含 IP 和域名：ips=%d domains=%d", ips, domains)
	}
	if domains != 5 {
		t.Fatalf("域名应有 5 个进入窗口，得到 %d", domains)
	}
}

// 只有一种 Kind 时保持原顺序。
func TestInterleaveByKindSingleKindUnchanged(t *testing.T) {
	targets := []ChainTarget{{Kind: "cf_ip", Host: "a"}, {Kind: "cf_ip", Host: "b"}}
	out := interleaveByKind(targets)
	if len(out) != 2 || out[0].Host != "a" || out[1].Host != "b" {
		t.Fatalf("单类不应改变顺序：%#v", out)
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
