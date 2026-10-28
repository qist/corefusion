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
	excluded map[int64]bool,
	now time.Time,
) ([]entity.NodeGroupMember, []int64) {
	// ① 先剔除容量超限/冷却中的成员（P2）——它们不参与选择，也不计入快照。
	candidates := members
	if len(excluded) > 0 {
		candidates = make([]entity.NodeGroupMember, 0, len(members))
		for _, m := range members {
			if excluded[m.ID] {
				continue
			}
			candidates = append(candidates, m)
		}
	}
	if sub == nil || tmpl == nil || tmpl.SubNodeLimit <= 0 || len(candidates) <= tmpl.SubNodeLimit {
		return candidates, nil
	}

	limit := tmpl.SubNodeLimit
	byID := make(map[int64]entity.NodeGroupMember, len(candidates))
	ids := make([]int64, 0, len(candidates))
	for _, m := range candidates {
		byID[m.ID] = m
		ids = append(ids, m.ID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	// ① 未到轮换期：保留快照里仍然有效的成员，只补齐缺失的槽位。
	//    （节点被禁用/超限时，只替换它那一个位置，其余 N-1 个保持不变。）
	keep := make([]entity.NodeGroupMember, 0, limit)
	keepIDs := make(map[int64]bool, limit)
	if !rotationDue(sub, tmpl, now) {
		for _, id := range sub.NodeSelectionIDs {
			if len(keep) >= limit {
				break
			}
			m, ok := byID[id]
			if !ok {
				continue
			}
			keep = append(keep, m)
			keepIDs[id] = true
		}
		if len(keep) == limit {
			outIDs := make([]int64, 0, limit)
			for _, m := range keep {
				outIDs = append(outIDs, m.ID)
			}
			return keep, outIDs
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

	chosen := append([]entity.NodeGroupMember(nil), keep...)
	chosenIDs := make([]int64, 0, limit)
	for _, m := range keep {
		chosenIDs = append(chosenIDs, m.ID)
	}
	for _, sc := range scoredIDs {
		if len(chosen) >= limit {
			break
		}
		if keepIDs[sc.id] {
			continue
		}
		chosen = append(chosen, byID[sc.id])
		chosenIDs = append(chosenIDs, sc.id)
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

// 订阅容量（P2）：节点超限后冷却多久再允许参与选择。
const nodeCapacityCooldown = 30 * time.Minute

// nodeCapacityInput 是容量判定所需的输入（便于单测，不依赖仓储）。
type nodeCapacityInput struct {
	// memberServer：memberID → 节点服务器 ID（0 = 外部节点/未知，不参与容量判定）
	memberServer map[int64]int64
	servers      map[int64]entity.NodeServer
	metrics      map[int64]entity.ServerMetric
}

// capacityExcludedMembers 计算本轮因容量/冷却被排除的成员。
// 返回 excluded（memberID 集合）与 cooldownUpdates（服务器 ID → 新冷却截止时间，
// 调用方负责落库）。首次超限的节点进入冷却，避免阈值边缘反复抖动。
func capacityExcludedMembers(in nodeCapacityInput, now time.Time) (map[int64]bool, map[int64]time.Time) {
	excluded := make(map[int64]bool)
	cooldownUpdates := make(map[int64]time.Time)

	for memberID, serverID := range in.memberServer {
		if serverID <= 0 {
			continue
		}
		srv, ok := in.servers[serverID]
		if !ok || !srv.Enabled {
			excluded[memberID] = true
			continue
		}
		if srv.SubCooldownUntil != nil && now.Before(*srv.SubCooldownUntil) {
			excluded[memberID] = true
			continue
		}
		over := false
		if srv.SubMaxConnections > 0 {
			if m, ok := in.metrics[serverID]; ok && m.TCPConnections > srv.SubMaxConnections {
				over = true
			}
		}
		if !over && srv.SubMaxBandwidthMbps > 0 {
			if m, ok := in.metrics[serverID]; ok {
				// traffic_* 是 bytes/sec → Mbps
				mbps := float64(m.TrafficUplink+m.TrafficDownlink) * 8 / 1_000_000
				if mbps > float64(srv.SubMaxBandwidthMbps) {
					over = true
				}
			}
		}
		if over {
			excluded[memberID] = true
			cooldownUpdates[serverID] = now.Add(nodeCapacityCooldown)
		}
	}
	return excluded, cooldownUpdates
}
