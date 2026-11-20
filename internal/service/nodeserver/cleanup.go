package nodeserver

import (
	"context"
	"fmt"
	"log"
	"strings"
)

// nodeScopedTables 是**随节点生命周期存在**的表：节点删除后不应继续保留，
// 否则同一个 server_id 被复用时会「继承」上一个节点的配置。
//
// 为什么必须清理：core_configs / inbound_configs 等表按 server_id 存，
// 新建节点时 SeedDefaults 看到「段已存在」就跳过 → 新节点拿到的是旧节点的
// 空路由、旧入站、旧出站（实测 server_id=20 复用了 2026-09 的残行）。
//
// 这里只列**纯节点自身**的数据；node_audit_logs 属于审计留痕，故意保留。
var nodeScopedTables = []string{
	"core_configs",
	"inbound_configs",
	"outbound_configs",
	"endpoint_configs",
	"inbound_services",
	"warp_configs",
	"proxy_sites",
	"caddy_sites",
	"nginx_sites",
	"rate_limit_configs",
	"rate_limit_ip_entries",
	"waf_rules",
	"waf_configs",
	"waf_domain_configs",
	"log_rotation_policies",
	"share_domain_configs",
	"server_metrics",
	"node_traffic_usage",
	"inbound_user_traffic",
	"agent_cert_revocations",
	"cert_distribution_queue",
	"server_tags",
	"client_log_packages",
}

// purgeNodeScopedData 删除某节点的所有节点级数据，返回删除的行数。
//
// 调用场景有两个：
//  1. 删除节点：先清子表再删 node_servers，避免留下孤儿数据；
//  2. 新建节点：新 ID 在插入前不可能有活节点，此时查到的同 ID 行一定是
//     已删节点的残留 → 先清掉，`SeedDefaults` 才会写入真正的默认配置。
//
// 表不存在（旧库尚未迁移）只记日志、不中断：这是尽力而为的清理。
func (s *Service) purgeNodeScopedData(ctx context.Context, serverID int64) int {
	if s.db == nil || serverID <= 0 {
		return 0
	}
	db := s.db.WithContext(ctx)

	// 先把「套餐引用本节点入站」的绑定删掉，否则 inbound_configs 删完会留下
	// 指向不存在入站的 plan_nodes 记录（套餐里出现幽灵节点）。
	if err := db.Exec(
		"DELETE FROM plan_nodes WHERE inbound_config_id IN (SELECT id FROM inbound_configs WHERE server_id = ?)",
		serverID,
	).Error; err != nil && !isMissingTableErr(err) {
		log.Printf("[nodeserver] purge plan_nodes for server %d: %v", serverID, err)
	}

	total := 0
	for _, table := range nodeScopedTables {
		res := db.Exec(fmt.Sprintf("DELETE FROM %s WHERE server_id = ?", table), serverID)
		if res.Error != nil {
			if !isMissingTableErr(res.Error) {
				log.Printf("[nodeserver] purge %s for server %d: %v", table, serverID, res.Error)
			}
			continue
		}
		total += int(res.RowsAffected)
	}
	return total
}

// isMissingTableErr 判断是否「表不存在」（旧库/精简部署）。
func isMissingTableErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such table") ||
		strings.Contains(msg, "doesn't exist") ||
		strings.Contains(msg, "does not exist")
}
