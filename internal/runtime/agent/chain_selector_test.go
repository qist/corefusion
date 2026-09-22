package agent

import (
	"testing"

	"corefusion/internal/model/entity"
)

// autoTagFor 模拟生成器里那个"自动 tag"回退（chain-<入口入站ID>-<出口ID> / -ext<外部ID>）。
func autoTagFor(link entity.ChainLink) string {
	if link.ExitExternalID > 0 {
		return "chain-" + itoa(link.EntryInboundID) + "-ext" + itoa(link.ExitExternalID)
	}
	return "chain-" + itoa(link.EntryInboundID) + "-" + itoa(link.ExitServerID)
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	var b []byte
	for v > 0 {
		b = append([]byte{byte('0' + v%10)}, b...)
		v /= 10
	}
	return string(b)
}

// TestChainGroupSelectorsUsesCustomExitTag 回归那个"流量静默直连"的 bug：
// balancer.selector / observatory.subjectSelector 必须是**实际出站 tag** —— 出口配了自定义
// `exit_tag` 时，用自动前缀 `chain-<入站ID>-` 会圈不到它 → balancer 候选为空 →
// 运行期落到 fallbackTag(direct) → 流量绕过出口直连 ✗（xray 不报错，极隐蔽）。
func TestChainGroupSelectorsUsesCustomExitTag(t *testing.T) {
	links := []entity.ChainLink{
		{EntryInboundID: 80, ExitServerID: 2, ExitTag: "lb-exit-A"}, // 自定义
		{EntryInboundID: 80, ExitServerID: 3},                       // 自动
		{EntryInboundID: 80, ExitExternalID: 7, ExitTag: "ext-hk"},  // 外部 + 自定义
	}
	got := chainGroupSelectors(links, autoTagFor)
	want := []string{"lb-exit-A", "chain-80-3", "ext-hk"}

	if len(got) != len(want) {
		t.Fatalf("数量不对：got=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("第 %d 个 selector 应为实际出站 tag %q，得到 %q（got=%v）", i, want[i], got[i], got)
		}
	}
}

// TestChainGroupSelectorsNeverFallsBackToPrefix 当**每个**出口都配了自定义 tag 时，
// 返回列表里不允许出现任何自动前缀 —— 那正是会让 balancer 空转的情形。
func TestChainGroupSelectorsNeverFallsBackToPrefix(t *testing.T) {
	links := []entity.ChainLink{
		{EntryInboundID: 80, ExitServerID: 2, ExitTag: "a"},
		{EntryInboundID: 80, ExitServerID: 3, ExitTag: "b"},
	}
	got := chainGroupSelectors(links, autoTagFor)
	for _, tag := range got {
		if tag != "a" && tag != "b" {
			t.Errorf("自定义 tag 场景下不应出现别的值（尤其不能是 chain-80- 前缀）：%v", got)
		}
	}
	// 反向确认：此时若用前缀圈定，一个都匹配不到
	prefixHit := 0
	for _, tag := range got {
		if len(tag) >= 8 && tag[:8] == "chain-80" {
			prefixHit++
		}
	}
	if prefixHit != 0 {
		t.Errorf("居然匹配到了自动前缀，说明这个回归测试失效了：%v", got)
	}
}
