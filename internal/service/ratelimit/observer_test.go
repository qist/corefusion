package ratelimitsvc

import (
	"testing"
	"time"
)

// xray 的采样 key 必须是 base email 且跨轮稳定：key 与条目存的 email 不一致时，
// cleanupSamples 每轮都会把样本删掉 → first_seen 归零 → 前端算出
// 「498484 小时」的连接时长。
func TestRateForKeepsFirstSeenAcrossPolls(t *testing.T) {
	o := &LocalObserver{samples: map[string]*localOnlineSample{}}

	_, _, first1, active1 := o.rateFor(coreXray, "a@example.com", 100, 50, 1000, true)
	if !active1 {
		t.Fatal("xray 条目只要出现就算在线")
	}
	dlRate, _, first2, _ := o.rateFor(coreXray, "a@example.com", 200, 60, 1015, true)
	if first1 != first2 {
		t.Fatalf("first_seen 应保持 %d，实际 %d（样本被当成下线重建了）", first1, first2)
	}
	if dlRate != 6 { // (200-100)/15
		t.Fatalf("下行速率 = %d, want 6", dlRate)
	}
}

// sing-box 没有连接 IP，也没有在线语义：只有流量增长才算活跃，
// 且超过 singboxIdleTTL 没有任何增量就视为离线。
func TestRateForSingboxIdleTTL(t *testing.T) {
	o := &LocalObserver{samples: map[string]*localOnlineSample{}}
	now := int64(10_000)

	if _, _, _, active := o.rateFor(coreSingbox, "s@example.com", 0, 0, now, false); active {
		t.Fatal("首次采样且流量为 0 时不应判定在线")
	}
	if _, _, _, active := o.rateFor(coreSingbox, "s@example.com", 1024, 0, now+15, false); !active {
		t.Fatal("流量增长后应判定在线")
	}
	// 流量不变但在 TTL 内：仍然在线
	if _, _, _, active := o.rateFor(coreSingbox, "s@example.com", 1024, 0, now+15+60, false); !active {
		t.Fatal("TTL 内应保持在线")
	}
	// 超过 TTL 仍无增量：离线
	if _, _, _, active := o.rateFor(coreSingbox, "s@example.com", 1024, 0, now+15+int64(singboxIdleTTL.Seconds())+1, false); active {
		t.Fatal("超过 idle TTL 应视为离线")
	}
}

// 同一邮箱同时连两个核心时，采样必须按核心隔离（否则 first_seen 与速率互相污染）。
func TestRateForIsolatesCores(t *testing.T) {
	o := &LocalObserver{samples: map[string]*localOnlineSample{}}
	_, _, xrayFirst, _ := o.rateFor(coreXray, "a@example.com", 10, 10, 500, true)
	_, _, sbFirst, _ := o.rateFor(coreSingbox, "a@example.com", 10, 10, 900, false)
	if xrayFirst == sbFirst {
		t.Fatalf("两个核心的 first_seen 不应共用同一个样本（都为 %d）", xrayFirst)
	}
	if len(o.samples) != 2 {
		t.Fatalf("应有两份独立样本，实际 %d", len(o.samples))
	}
}

// sing-box 的样本在用户暂时没流量时要保留（否则重新活跃时 first_seen 被重置）。
func TestCleanupSamplesKeepsIdleSingboxSamples(t *testing.T) {
	now := time.Now().Unix()
	o := &LocalObserver{samples: map[string]*localOnlineSample{
		coreSingbox + "|idle@example.com": {lastActive: now - 60},
		coreSingbox + "|dead@example.com": {lastActive: now - int64(singboxSampleTTL.Seconds()) - 10},
	}}
	o.cleanupSamples(coreSingbox, nil)
	if _, ok := o.samples[coreSingbox+"|idle@example.com"]; !ok {
		t.Fatal("仅因本轮无条目就删样本会让 first_seen 反复重置")
	}
	if _, ok := o.samples[coreSingbox+"|dead@example.com"]; ok {
		t.Fatal("超过 sample TTL 的样本应被清理")
	}
}
