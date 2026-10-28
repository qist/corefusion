package subscriptions

import (
	"crypto/sha256"
	"encoding/binary"
	"sort"
	"time"

	"corefusion/internal/model/entity"
)

// selectMembersForSubscription 按套餐「订阅限量 N 个节点」规则为订阅挑选节点。
//
// 设计要点（详见 /opt/corefusion-private/design/SUBSCRIPTION_NODE_SELECTION.md）：
//   - **稳定**：同一订阅 + 同一批候选 + 同一 epoch，结果恒定；不是每请求随机。
//   - **可复现**：用 sha256("<subID>:<memberID>:<epoch>") 排序，进程重启/多副本一致。
//   - **可控轮换**：tmpl.SubRotateHours>0 时按小时分桶换 epoch；0 表示不主动轮换。
//   - 缓存命中（node_selection_ids 仍全部有效且未到轮换期）直接复用，保证订阅刷新不变。
//
// 返回值：按选择顺序排列的 members；以及本次选中的 member ID 快照（未启用限量时为 nil）。
func selectMembersForSubscription(
	sub *entity.Subscription,
	tmpl *entity.PlanTemplate,
	members []entity.NodeGroupMember,
	now time.Time,
) ([]entity.NodeGroupMember, []int64) {
	if sub == nil || tmpl == nil || tmpl.SubNodeLimit <= 0 || len(members) <= tmpl.SubNodeLimit {
		return members, nil
	}

	limit := tmpl.SubNodeLimit
	byID := make(map[int64]entity.NodeGroupMember, len(members))
	ids := make([]int64, 0, len(members))
	for _, m := range members {
		byID[m.ID] = m
		ids = append(ids, m.ID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	// ① 缓存命中：数量对、全部仍在候选里、且未到轮换时间
	if len(sub.NodeSelectionIDs) == limit && !rotationDue(sub, tmpl, now) {
		cached := make([]entity.NodeGroupMember, 0, limit)
		valid := true
		for _, id := range sub.NodeSelectionIDs {
			m, ok := byID[id]
			if !ok {
				valid = false
				break
			}
			cached = append(cached, m)
		}
		if valid {
			return cached, append([]int64(nil), sub.NodeSelectionIDs...)
		}
	}

	// ② 重新选择：hash(subID, memberID, epoch) 升序，tie-break 用 ID
	epoch := int64(0)
	if tmpl.SubRotateHours > 0 {
		epoch = now.Unix() / int64(tmpl.SubRotateHours*3600)
	}
	type scored struct {
		id    int64
		score uint64
	}
	scoredIDs := make([]scored, 0, len(ids))
	for _, id := range ids {
		scoredIDs = append(scoredIDs, scored{id: id, score: memberSelectScore(sub.ID, id, epoch)})
	}
	sort.Slice(scoredIDs, func(i, j int) bool {
		if scoredIDs[i].score != scoredIDs[j].score {
			return scoredIDs[i].score < scoredIDs[j].score
		}
		return scoredIDs[i].id < scoredIDs[j].id
	})

	chosen := make([]entity.NodeGroupMember, 0, limit)
	chosenIDs := make([]int64, 0, limit)
	for _, s := range scoredIDs[:limit] {
		chosen = append(chosen, byID[s.id])
		chosenIDs = append(chosenIDs, s.id)
	}
	return chosen, chosenIDs
}

// rotationDue 判断缓存的选择是否已到轮换时间（SubRotateHours<=0 表示永不主动轮换）。
func rotationDue(sub *entity.Subscription, tmpl *entity.PlanTemplate, now time.Time) bool {
	if tmpl.SubRotateHours <= 0 {
		return false
	}
	if sub.NodeSelectionAt == nil {
		return true
	}
	return now.Sub(*sub.NodeSelectionAt) >= time.Duration(tmpl.SubRotateHours)*time.Hour
}

// memberSelectScore 计算 (订阅, 节点, epoch) 的稳定分值。
func memberSelectScore(subID, memberID, epoch int64) uint64 {
	var buf [24]byte
	binary.BigEndian.PutUint64(buf[0:8], uint64(subID))
	binary.BigEndian.PutUint64(buf[8:16], uint64(memberID))
	binary.BigEndian.PutUint64(buf[16:24], uint64(epoch))
	sum := sha256.Sum256(buf[:])
	return binary.BigEndian.Uint64(sum[:8])
}

// sameInt64List 判断两份选择快照是否一致（用于避免无意义的写库）。
func sameInt64List(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
