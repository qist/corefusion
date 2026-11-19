package subscriptions

import (
	"testing"

	"corefusion/internal/model/entity"
)

// CDN 条目端口固定取第一个：每次随机换端口会让探测结果无法横向比较。
func TestChainProbePickPort(t *testing.T) {
	if got := chainProbePickPort([]int{2053, 443}, 8443); got != 2053 {
		t.Fatalf("多端口应取第一个，实际 %d", got)
	}
	if got := chainProbePickPort(nil, 8443); got != 8443 {
		t.Fatalf("无 ports 应回落条目端口，实际 %d", got)
	}
	if got := chainProbePickPort(nil, 0); got != 443 {
		t.Fatalf("都为空应回落 443，实际 %d", got)
	}
}

// 入站里手填的 CF 域名/IP 若不在池子里，也要纳入本轮探测（EntryID=0，不回写健康）。
func TestChainProbeExplicitTarget(t *testing.T) {
	ad := map[string]interface{}{"type": "cf_domain", "cfDomain": "extra.example.com"}
	got := chainProbeExplicitTarget(ad, "cf_domain", []chainProbeEntry{{Kind: "cf_domain", Host: "pool.example.com"}})
	if got == nil || got.Host != "extra.example.com" || got.EntryID != 0 {
		t.Fatalf("池外域名应作为临时目标，实际 %#v", got)
	}

	// 池子里已有的不再重复添加
	if got := chainProbeExplicitTarget(ad, "cf_domain", []chainProbeEntry{{Kind: "cf_domain", Host: "extra.example.com"}}); got != nil {
		t.Fatalf("已存在的条目不应重复添加：%#v", got)
	}

	// 直连域名不参与链路探测：它只是某台节点的直连入口，不能用别的入站凭据代表
	direct := map[string]interface{}{"type": "direct", "directDomain": "x.example.com"}
	if got := chainProbeExplicitTarget(direct, "direct", nil); got != nil {
		t.Fatalf("direct 类型不应产生目标：%#v", got)
	}

	// 自定义域名（custom）同样排除：不同节点域名/入口都不一样
	custom := map[string]interface{}{"type": "custom", "customDomain": "grafana.example.com", "customPort": float64(2053)}
	if got := chainProbeExplicitTarget(custom, "custom", nil); got != nil {
		t.Fatalf("custom 类型不应产生目标：%#v", got)
	}
}

// 探针入站必须按选择命中；未选择/不存在时要明确失败，不能悄悄退回随机。
func TestPickChainProbeInbound(t *testing.T) {
	mk := func(id int64) chainProbeInbound {
		return chainProbeInbound{cfg: &entity.InboundConfig{ID: id, Protocol: "vless", Port: 443}}
	}
	probes := []chainProbeInbound{mk(3), mk(7)}

	got, err := pickChainProbeInbound(probes, "inb:7")
	if err != nil || got == nil || got.cfg.ID != 7 {
		t.Fatalf("应按 id 命中入站: got=%#v err=%v", got, err)
	}
	if _, err := pickChainProbeInbound(probes, "inb:9"); err == nil {
		t.Fatal("不存在的探针入站应报错")
	}
	if _, err := pickChainProbeInbound(probes, "ext:1"); err == nil {
		t.Fatal("非 inb: 前缀应报错")
	}
}

// 坏 CDN 条目不会分发给订阅：判失败的直接排除，整池失败也不回退全量。
// 具体口径见 TestUsableForShare（public_test.go）。
func TestChainProbeExcludesFailedEntries(t *testing.T) {
	good := shareDomainEntry{Host: "ok.example.com", LatencyMs: 80}
	failed := shareDomainEntry{Host: "bad.example.com", LatencyMs: 80}

	kept := make([]shareDomainEntry, 0, 2)
	for _, e := range []shareDomainEntry{failed, good} {
		healthy := e.Host != failed.Host
		if usableForShare(healthy, e.LatencyMs, 0) {
			kept = append(kept, e)
		}
	}
	if len(kept) != 1 || kept[0].Host != good.Host {
		t.Fatalf("判失败的条目不应下发，实际 %#v", kept)
	}

	// 整池都判失败：一个都不发（不回退全量）
	kept = kept[:0]
	for _, e := range []shareDomainEntry{failed} {
		if usableForShare(false, e.LatencyMs, 0) {
			kept = append(kept, e)
		}
	}
	if len(kept) != 0 {
		t.Fatalf("整池失败时不应回退全量，实际 %#v", kept)
	}
}
