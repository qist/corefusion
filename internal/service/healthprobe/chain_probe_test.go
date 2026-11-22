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

func chainTestPool(ipN, domainN int) []ChainTarget {
	// 与 ChainProbeTargets 一致：按 Kind 排序后域名在前、IP 在后。
	targets := make([]ChainTarget, 0, ipN+domainN)
	for i := 0; i < domainN; i++ {
		targets = append(targets, ChainTarget{Kind: "cf_domain", Host: fmt.Sprintf("d%d.example.com", i)})
	}
	for i := 0; i < ipN; i++ {
		targets = append(targets, ChainTarget{Kind: "cf_ip", Host: fmt.Sprintf("104.20.0.%d", i)})
	}
	return targets
}

// 线上真实分布：140 个 IP + 12 个域名、max_targets=30。
// 旧实现按全局游标取连续窗口，游标落到 IP 段后整轮 30 个全是 IP，域名永远轮不到。
func TestSelectChainTargetsCoversBothKinds(t *testing.T) {
	targets := chainTestPool(140, 12)
	picked, _ := selectChainTargets(targets, nil, 30)
	if len(picked) != 30 {
		t.Fatalf("本轮数量 = %d, want 30", len(picked))
	}
	ips, domains := 0, 0
	seen := make(map[string]bool, len(picked))
	for _, tg := range picked {
		if seen[tg.Host] {
			t.Fatalf("同一轮出现重复目标：%s", tg.Host)
		}
		seen[tg.Host] = true
		switch tg.Kind {
		case "cf_ip":
			ips++
		case "cf_domain":
			domains++
		}
	}
	if ips == 0 || domains == 0 {
		t.Fatalf("30 个名额必须同时覆盖 IP 和域名：ips=%d domains=%d", ips, domains)
	}
	if domains != 2 {
		t.Fatalf("按比例域名应占 2 个名额，得到 %d", domains)
	}
}

// 连续多轮时域名游标独立推进，12 个域名应在 6 轮内全部被测到。
func TestSelectChainTargetsRotatesDomainsIndependently(t *testing.T) {
	targets := chainTestPool(140, 12)
	cursors := map[string]int{}
	seen := make(map[string]bool)
	for round := 0; round < 6; round++ {
		picked, next := selectChainTargets(targets, cursors, 30)
		cursors = next
		for _, tg := range picked {
			if tg.Kind == "cf_domain" {
				seen[tg.Host] = true
			}
		}
	}
	if len(seen) != 12 {
		t.Fatalf("6 轮应覆盖全部 12 个域名，实际 %d", len(seen))
	}
}

// max_targets 小于类型数时，跨轮次在类型间交替，不能永远只测 IP。
func TestSelectChainTargetsSmallLimitAlternatesKinds(t *testing.T) {
	targets := chainTestPool(140, 12)
	cursors := map[string]int{}
	kinds := make([]string, 0, 4)
	for i := 0; i < 4; i++ {
		var picked []ChainTarget
		picked, cursors = selectChainTargets(targets, cursors, 1)
		kinds = append(kinds, picked[0].Kind)
	}
	for i := 1; i < len(kinds); i++ {
		if kinds[i] == kinds[i-1] {
			t.Fatalf("连续两轮测了同一类型：%v", kinds)
		}
	}
	if kinds[0] != "cf_domain" || kinds[1] != "cf_ip" {
		t.Fatalf("小名额应按相位交替类型，得到 %v", kinds)
	}
}

// 只有一种 Kind 且名额够时，应原样返回全部目标。
func TestSelectChainTargetsSingleKindUnchanged(t *testing.T) {
	targets := []ChainTarget{{Kind: "cf_ip", Host: "a"}, {Kind: "cf_ip", Host: "b"}}
	picked, _ := selectChainTargets(targets, nil, 10)
	if len(picked) != 2 || picked[0].Host != "a" || picked[1].Host != "b" {
		t.Fatalf("单类不应改变顺序：%#v", picked)
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
