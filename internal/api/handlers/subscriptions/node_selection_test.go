package subscriptions

import (
	"testing"
	"time"

	"corefusion/internal/model/entity"
)

func testMembers(n int) []entity.NodeGroupMember {
	out := make([]entity.NodeGroupMember, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, entity.NodeGroupMember{ID: int64(i), GroupID: 1, Enabled: true})
	}
	return out
}

func idsOf(members []entity.NodeGroupMember) []int64 {
	out := make([]int64, 0, len(members))
	for _, m := range members {
		out = append(out, m.ID)
	}
	return out
}

// TestSelectMembersDisabled 未配置限量（0）或候选不足时保持原样。
func TestSelectMembersDisabled(t *testing.T) {
	sub := &entity.Subscription{ID: 1}
	all := testMembers(5)

	got, sel := selectMembersForSubscription(sub, &entity.PlanTemplate{SubNodeLimit: 0}, all, time.Now())
	if len(got) != 5 || sel != nil {
		t.Fatalf("limit=0 应返回全部且不写快照, got=%d sel=%v", len(got), sel)
	}

	got, sel = selectMembersForSubscription(sub, &entity.PlanTemplate{SubNodeLimit: 10}, all, time.Now())
	if len(got) != 5 || sel != nil {
		t.Fatalf("候选少于限量应返回全部, got=%d sel=%v", len(got), sel)
	}

	got, sel = selectMembersForSubscription(sub, nil, all, time.Now())
	if len(got) != 5 || sel != nil {
		t.Fatalf("tmpl=nil 应返回全部, got=%d sel=%v", len(got), sel)
	}
}

// TestSelectMembersStable 同一订阅 + 同一批候选 → 结果恒定（不是每请求随机）。
func TestSelectMembersStable(t *testing.T) {
	sub := &entity.Subscription{ID: 42}
	tmpl := &entity.PlanTemplate{SubNodeLimit: 10}
	all := testMembers(20)
	now := time.Unix(1_800_000_000, 0)

	first, sel1 := selectMembersForSubscription(sub, tmpl, all, now)
	if len(first) != 10 || len(sel1) != 10 {
		t.Fatalf("应选出 10 个: members=%d sel=%d", len(first), len(sel1))
	}
	for i := 1; i < 5; i++ {
		_, againSel := selectMembersForSubscription(sub, tmpl, all, now.Add(time.Duration(i)*time.Minute))
		if !sameInt64List(againSel, sel1) {
			t.Fatalf("同一 epoch 内结果应在两次调用间保持稳定: %v vs %v", againSel, sel1)
		}
	}

	// 不同订阅应拿到不同子集（20 选 10，碰撞概率极低）
	other, otherSel := selectMembersForSubscription(&entity.Subscription{ID: 43}, tmpl, all, now)
	if sameInt64List(otherSel, sel1) {
		t.Fatalf("不同订阅不应得到完全相同的集合: %v", otherSel)
	}
	if len(other) != 10 {
		t.Fatalf("另一订阅也应选出 10 个，got=%d", len(other))
	}
}

// TestSelectMembersCachedAndInvalidated 缓存命中；节点被移除后重算。
func TestSelectMembersCachedAndInvalidated(t *testing.T) {
	sub := &entity.Subscription{ID: 7}
	tmpl := &entity.PlanTemplate{SubNodeLimit: 5}
	all := testMembers(20)
	now := time.Unix(1_800_000_000, 0)

	_, sel := selectMembersForSubscription(sub, tmpl, all, now)
	sub.NodeSelectionIDs = entity.Int64List(sel)
	sub.NodeSelectionAt = &now

	// 缓存命中：即使传入的顺序不同，也按快照顺序返回
	reordered := append([]entity.NodeGroupMember(nil), all...)
	for i, j := 0, len(reordered)-1; i < j; i, j = i+1, j-1 {
		reordered[i], reordered[j] = reordered[j], reordered[i]
	}
	got, gotSel := selectMembersForSubscription(sub, tmpl, reordered, now.Add(time.Minute))
	if !sameInt64List(gotSel, sel) || !sameInt64List(idsOf(got), sel) {
		t.Fatalf("缓存命中时应保持原快照顺序: got=%v want=%v", gotSel, sel)
	}

	// 把选中的一个节点移除 → 缓存失效，重新选满
	removed := make([]entity.NodeGroupMember, 0, len(all))
	for _, m := range all {
		if m.ID != sel[0] {
			removed = append(removed, m)
		}
	}
	_, newSel := selectMembersForSubscription(sub, tmpl, removed, now.Add(2*time.Minute))
	if len(newSel) != 5 {
		t.Fatalf("节点移除后应重新选满 5 个，got=%d", len(newSel))
	}
	for _, id := range newSel {
		if id == sel[0] {
			t.Fatalf("已移除的节点不应再被选中: %v", newSel)
		}
	}
}

// TestSelectMembersRotation 到轮换周期后应换一批（epoch 变化）。
func TestSelectMembersRotation(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	sub := &entity.Subscription{ID: 9}
	tmpl := &entity.PlanTemplate{SubNodeLimit: 10, SubRotateHours: 1}
	all := testMembers(50)

	_, sel := selectMembersForSubscription(sub, tmpl, all, now)
	sub.NodeSelectionIDs = entity.Int64List(sel)
	sub.NodeSelectionAt = &now

	// 未到期：沿用快照
	_, same := selectMembersForSubscription(sub, tmpl, all, now.Add(30*time.Minute))
	if !sameInt64List(same, sel) {
		t.Fatalf("轮换未到期应沿用快照")
	}

	// 到期：应重新选择（50 选 10，撞同一集合概率可忽略）
	_, rotated := selectMembersForSubscription(sub, tmpl, all, now.Add(2*time.Hour))
	if sameInt64List(rotated, sel) {
		t.Fatalf("到轮换周期后应换一批节点")
	}
}

// TestSelectMembersEdgeCases 空列表 / 只剩一个节点不 panic。
func TestSelectMembersEdgeCases(t *testing.T) {
	tmpl := &entity.PlanTemplate{SubNodeLimit: 3}
	sub := &entity.Subscription{ID: 1}

	got, sel := selectMembersForSubscription(sub, tmpl, nil, time.Now())
	if len(got) != 0 || sel != nil {
		t.Fatalf("空列表应直接返回: got=%d sel=%v", len(got), sel)
	}

	got, sel = selectMembersForSubscription(sub, tmpl, testMembers(3), time.Now())
	if len(got) != 3 || sel != nil {
		t.Fatalf("恰好等于限量应返回全部: got=%d sel=%v", len(got), sel)
	}
}
