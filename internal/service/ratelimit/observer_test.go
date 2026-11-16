package ratelimitsvc

import (
	"testing"
	"time"

	agentpb "corefusion/internal/runtime/agent/proto"
)

// 采样 key 必须跨轮稳定：key 与条目存的账号不一致时，cleanupSamples 每轮都会
// 把样本删掉 → first_seen 归零 → 前端算出「498484 小时」的连接时长。
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

// 同一订阅的多个入站账号（email 带不同 _shortTag）各有独立计数器：
// 采样键必须区分账号，否则两个账号的累计值互相覆盖 → 速率跳变、first_seen 反复重建。
func TestRateForIsolatesAccountsOfSameSubscription(t *testing.T) {
	o := &LocalObserver{samples: map[string]*localOnlineSample{}}

	if _, _, firstA, _ := o.rateFor(coreXray, "a@example.com_vless1", 1000, 100, 1000, true); firstA != 1000 {
		t.Fatalf("账号 A 首次采样 first_seen 应为 1000，实际 %d", firstA)
	}
	// 账号 B 首次出现：不能继承 A 的基线
	dlB, ulB, firstB, _ := o.rateFor(coreXray, "a@example.com_anytls2", 900000, 5000, 1000, true)
	if dlB != 0 || ulB != 0 {
		t.Fatalf("账号 B 首次采样速率应为 0，实际 dl=%d ul=%d（继承了账号 A 的基线）", dlB, ulB)
	}
	if firstB != 1000 {
		t.Fatalf("账号 B 首次采样 first_seen 应为 1000，实际 %d", firstB)
	}
	// 账号 A 正常增长：按自身基线算速率，不能被 B 的 900000 影响
	dlA, ulA, firstA2, _ := o.rateFor(coreXray, "a@example.com_vless1", 1300, 130, 1015, true)
	if dlA != 20 || ulA != 2 {
		t.Fatalf("账号 A 速率应为 dl=20 ul=2，实际 dl=%d ul=%d", dlA, ulA)
	}
	if firstA2 != 1000 {
		t.Fatalf("账号 A 的 first_seen 不应被账号 B 覆盖：%d", firstA2)
	}
	if len(o.samples) != 2 {
		t.Fatalf("两个账号应各有一份样本，实际 %d", len(o.samples))
	}
}

// cleanupSamples 的存活键必须与 rateFor 一致（账号全名），否则条目会被当成下线删掉。
func TestCleanupSamplesUsesAccountEmailKey(t *testing.T) {
	now := time.Now().Unix()
	o := &LocalObserver{samples: map[string]*localOnlineSample{
		coreXray + "|a@example.com_vless1": {lastActive: now},
	}}
	o.cleanupSamples(coreXray, []*agentpb.OnlineUserEntry{{
		Email:        "a@example.com",
		AccountEmail: "a@example.com_vless1",
	}})
	if _, ok := o.samples[coreXray+"|a@example.com_vless1"]; !ok {
		t.Fatal("在线条目对应的账号样本不应被清理")
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

// 速率必须用 trafficsync 的增量换算，而不是对核心计数器做差分：
// trafficsync 每 30s 用 reset_=true 清零同一份计数器，差分只会得到 0/残值。
func TestDeltaRateFromTrafficsyncDeltas(t *testing.T) {
	now := int64(1_000_000)
	deltas := map[string]localUserDelta{
		"a@example.com_vless1": {down: 450_000, up: 150_000, at: now, window: 30},
	}
	dl, ul, ok := deltaRate(deltas, "a@example.com_vless1", now)
	if !ok {
		t.Fatal("有新鲜增量时必须返回速率")
	}
	if dl != 15_000 || ul != 5_000 {
		t.Fatalf("速率应为 15000/5000 B/s，实际 %d/%d", dl, ul)
	}
	// 同一订阅的另一个账号没有增量 → 不能串到 A 的速率
	if _, _, ok := deltaRate(deltas, "a@example.com_anytls2", now); ok {
		t.Fatal("未上报增量的账号不应拿到别的账号的速率")
	}
	// 增量过期（trafficsync 停了）→ 回落到计数器差分
	if _, _, ok := deltaRate(deltas, "a@example.com_vless1", now+2*30+11); ok {
		t.Fatal("过期增量不应继续展示旧速率")
	}
}

// SetLocalUserDeltas 每轮整体替换：上一轮有流量、本轮没有的账号必须归零。
func TestSetLocalUserDeltasReplacesSnapshot(t *testing.T) {
	s := &Service{}
	s.SetLocalUserDeltas([]LocalUserDelta{{Email: "a@x.com", Up: 300, Down: 600, Window: 30}})
	d := s.LocalUserDeltas()["a@x.com"]
	if d.up != 300 || d.down != 600 || d.window != 30 {
		t.Fatalf("增量未落库: %+v", d)
	}
	s.SetLocalUserDeltas([]LocalUserDelta{{Email: "b@x.com", Up: 1, Down: 2, Window: 30}})
	if _, ok := s.LocalUserDeltas()["a@x.com"]; ok {
		t.Fatal("本轮没有增量的账号应被移除，否则界面会一直显示上一轮的速率")
	}
	dl, ul, ok := deltaRate(s.LocalUserDeltas(), "b@x.com", d.at)
	if !ok || dl != 0 || ul != 0 {
		t.Fatalf("小增量按周期换算后应为 0 B/s（不四舍五入），实际 %d/%d ok=%v", dl, ul, ok)
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
