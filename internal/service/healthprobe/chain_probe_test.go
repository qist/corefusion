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

// 域名 / IP 分开控制：各自开关、各自名额；新字段缺失时回落到旧总名额。
func TestChainConfigSeparateKinds(t *testing.T) {
	cfg := ChainConfig{
		Enabled:          true,
		MaxTargets:       30,
		DomainEnabled:    true,
		IPEnabled:        false,
		DomainMaxTargets: 8,
		IPMaxTargets:     12,
	}
	if got := cfg.enabledChainKinds(); len(got) != 1 || got[0] != "domain" {
		t.Fatalf("只开域名时应只跑域名，得到 %v", got)
	}
	if got := cfg.maxTargetsForKind("domain"); got != 8 {
		t.Fatalf("域名名额 = %d, want 8", got)
	}
	if got := cfg.maxTargetsForKind("ip"); got != 12 {
		t.Fatalf("IP 名额 = %d, want 12", got)
	}

	// 旧配置只有 MaxTargets：两类都继承它，升级后行为不变。
	legacy := ChainConfig{Enabled: true, MaxTargets: 30, DomainEnabled: true, IPEnabled: true}
	if got := legacy.maxTargetsForKind("domain"); got != 30 {
		t.Fatalf("旧配置域名名额 = %d, want 30", got)
	}
	if got := legacy.maxTargetsForKind("ip"); got != 30 {
		t.Fatalf("旧配置 IP 名额 = %d, want 30", got)
	}

	// 两个开关都关 → 自动巡检不跑任何类型。
	off := ChainConfig{}
	if got := off.enabledChainKinds(); len(got) != 0 {
		t.Fatalf("开关全关时不应跑任何类型，得到 %v", got)
	}
}

// 分类型开关 / 名额必须能各自保存、各自读回，读回后自动巡检只跑启用的类型。
func TestChainConfigSeparateKindsRoundTrip(t *testing.T) {
	p := newProbeTestService(t)
	ctx := context.Background()
	err := p.SaveChainConfig(ctx, ChainConfig{
		Enabled:          true,
		Exit:             "ext:4",
		Inbound:          "inb:10",
		Concurrency:      2,
		ProbeURL:         "https://www.cloudflare.com/cdn-cgi/trace",
		KindsConfigured:  true,
		DomainEnabled:    true,
		IPEnabled:        false,
		DomainMaxTargets: 7,
		IPMaxTargets:     9,
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	got := p.ReadChainConfig(ctx)
	if !got.DomainEnabled || got.IPEnabled {
		t.Fatalf("开关读回错：domain=%v ip=%v", got.DomainEnabled, got.IPEnabled)
	}
	if got.DomainMaxTargets != 7 || got.IPMaxTargets != 9 {
		t.Fatalf("名额读回错：domain=%d ip=%d", got.DomainMaxTargets, got.IPMaxTargets)
	}
	if kinds := got.enabledChainKinds(); len(kinds) != 1 || kinds[0] != "domain" {
		t.Fatalf("自动巡检应只跑域名，得到 %v", kinds)
	}
}

// 旧前端只传 enabled + max_targets：不能把两类开关写成 false、名额写成 1。
func TestChainConfigLegacySaveFollowsMaster(t *testing.T) {
	p := newProbeTestService(t)
	ctx := context.Background()
	err := p.SaveChainConfig(ctx, ChainConfig{
		Enabled:     true,
		Exit:        "ext:4",
		Inbound:     "inb:10",
		Concurrency: 2,
		MaxTargets:  30,
		ProbeURL:    "https://www.cloudflare.com/cdn-cgi/trace",
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	got := p.ReadChainConfig(ctx)
	if !got.DomainEnabled || !got.IPEnabled {
		t.Fatalf("旧前端保存后两类都应跟随总开关，实际 domain=%v ip=%v", got.DomainEnabled, got.IPEnabled)
	}
	if got.DomainMaxTargets != 30 || got.IPMaxTargets != 30 {
		t.Fatalf("旧前端名额应沿用 max_targets=30，实际 domain=%d ip=%d", got.DomainMaxTargets, got.IPMaxTargets)
	}
}

// 分类型探测只取本类目标，另一类一条都不能混进来。
func TestFilterChainTargetsByKind(t *testing.T) {
	targets := chainTestPool(140, 12)
	domains := filterChainTargetsByKind(targets, "domain")
	if len(domains) != 12 {
		t.Fatalf("域名目标 = %d, want 12", len(domains))
	}
	for _, tg := range domains {
		if tg.Kind != "cf_domain" {
			t.Fatalf("域名结果混入了 %s", tg.Kind)
		}
	}
	ips := filterChainTargetsByKind(targets, "ip")
	if len(ips) != 140 {
		t.Fatalf("IP 目标 = %d, want 140", len(ips))
	}
	for _, tg := range ips {
		if tg.Kind != "cf_ip" {
			t.Fatalf("IP 结果混入了 %s", tg.Kind)
		}
	}

	// 单类结果写入/读取使用不同的键，互不覆盖。
	if chainResultsSettingKey("ip") == chainResultsSettingKey("domain") {
		t.Fatal("IP 与域名必须使用不同的结果存储键")
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

func chainTargetsForHealth(kind string, healthy, unhealthy int) []ChainTarget {
	out := make([]ChainTarget, 0, healthy+unhealthy)
	for i := 0; i < healthy; i++ {
		out = append(out, ChainTarget{Kind: kind, Host: fmt.Sprintf("ok-%s-%d", kind, i), Healthy: true})
	}
	for i := 0; i < unhealthy; i++ {
		out = append(out, ChainTarget{Kind: kind, Host: fmt.Sprintf("bad-%s-%d", kind, i), Healthy: false})
	}
	return out
}

func countHealth(picked []ChainTarget) (ok, bad int) {
	for _, t := range picked {
		if t.Healthy {
			ok++
		} else {
			bad++
		}
	}
	return
}

// 每类名额约 80% 给健康条目，至少 1 个给不健康条目做恢复探测。
func TestSelectChainTargetsPrefersHealthy(t *testing.T) {
	picked, _ := selectChainTargets(chainTargetsForHealth("cf_ip", 10, 10), nil, 10)
	ok, bad := countHealth(picked)
	if ok != 8 || bad != 2 {
		t.Fatalf("healthy/unhealthy = %d/%d, want 8/2", ok, bad)
	}
}

// 不健康条目不足时，用健康条目补满名额，不能少测。
func TestSelectChainTargetsFillsQuotaWhenFewUnhealthy(t *testing.T) {
	picked, _ := selectChainTargets(chainTargetsForHealth("cf_ip", 20, 1), nil, 10)
	ok, bad := countHealth(picked)
	if len(picked) != 10 || ok != 9 || bad != 1 {
		t.Fatalf("len=%d healthy=%d unhealthy=%d, want 10/9/1", len(picked), ok, bad)
	}
}

// 全部不健康时仍然要测（恢复探测），不能因为健康组为空就返回空。
func TestSelectChainTargetsAllUnhealthy(t *testing.T) {
	picked, _ := selectChainTargets(chainTargetsForHealth("cf_ip", 0, 10), nil, 5)
	if len(picked) != 5 {
		t.Fatalf("len=%d, want 5", len(picked))
	}
	if ok, bad := countHealth(picked); ok != 0 || bad != 5 {
		t.Fatalf("healthy/unhealthy = %d/%d, want 0/5", ok, bad)
	}
}

// 游标必须推进：同一轮内不重复，下一轮从新的位置继续。
func TestSelectChainTargetsAdvancesHealthyCursors(t *testing.T) {
	targets := chainTargetsForHealth("cf_ip", 10, 10)
	cursors := map[string]int{}
	first, cursors := selectChainTargets(targets, cursors, 10)
	seen := map[string]bool{}
	for _, item := range first {
		if seen[item.Host] {
			t.Fatalf("同一轮出现重复目标: %s", item.Host)
		}
		seen[item.Host] = true
	}
	second, _ := selectChainTargets(targets, cursors, 10)
	if len(second) == 0 {
		t.Fatal("第二轮为空")
	}
	if first[0].Host == second[0].Host {
		t.Fatalf("第二轮起点没有推进，仍为 %s", first[0].Host)
	}
}

func TestChainConfigEffectiveInterval(t *testing.T) {
	c := ChainConfig{InheritProbe: true, IntervalMinutes: 30}
	if got := c.EffectiveInterval(10); got != 10 {
		t.Fatalf("继承时应使用探测设置间隔 10，实际 %d", got)
	}
	c.InheritProbe = false
	if got := c.EffectiveInterval(10); got != 30 {
		t.Fatalf("独立时应使用自己的 30，实际 %d", got)
	}
	c.IntervalMinutes = 0
	if got := c.EffectiveInterval(10); got != 10 {
		t.Fatalf("未配置时应回退探测设置 10，实际 %d", got)
	}
	c.IntervalMinutes = 0
	if got := c.EffectiveInterval(0); got != DefaultIntervalMinutes {
		t.Fatalf("都为空时应回退默认 %d，实际 %d", DefaultIntervalMinutes, got)
	}
}
