package metrics

import (
	"context"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"corefusion/internal/api/response"
	"corefusion/internal/model/entity"
)

// CoreStateProvider 提供节点上各核心的实时运行状态（core_type → state）。
// 由 service/node.Service 实现：本机读进程表，远端读 Agent 心跳缓存。
// 用于替代 inbound_services.process_status —— 那一列只有本机核心进程管理器会写。
type CoreStateProvider interface {
	CoreStates(ctx context.Context, serverID int64) (map[string]string, error)
}

// Handler handles metrics HTTP requests.
type Handler struct {
	db         *gorm.DB
	coreStates CoreStateProvider
}

// NewHandler creates a new metrics handler.
func NewHandler(db *gorm.DB) *Handler {
	return &Handler{db: db}
}

// SetCoreStateProvider 注入核心实时状态提供者（本机进程表 + 远端 Agent 上报缓存）。
func (h *Handler) SetCoreStateProvider(p CoreStateProvider) {
	h.coreStates = p
}

// Overview handles GET /api/v1/metrics/overview.
func (h *Handler) Overview(c *gin.Context) {
	var nodeCount, serviceCount, serviceRunningCount, serviceStoppedCount, serviceUnknownCount int64
	var subCount, subSuspendedCount, subExpiredCount, clientCount, certCount int64

	// 节点数 = 节点服务器（与 Dashboard 节点卡、node_servers 列表同一口径）。
	// entity.Node 是「分流节点」，和用户看到的节点不是一回事，会恒为 0。
	h.db.Model(&entity.NodeServer{}).Count(&nodeCount)
	// 只统计挂在真实存在的节点服务器上的核心服务，排除指向已删除服务器的孤儿记录。
	// 每台节点会为 xray / sing-box 各建一行，所以 4 台节点 = 8 行是正常的。
	svcQuery := func() *gorm.DB {
		return h.db.Table("inbound_services").
			Joins("INNER JOIN node_servers ON node_servers.id = inbound_services.server_id")
	}
	svcQuery().Count(&serviceCount)
	// 状态统计优先用**实时状态**（与「节点详情 → 核心进程」同源：本机进程表 +
	// 远端 Agent 心跳缓存），不再依赖 inbound_services.process_status ——
	// 那一列只有本机核心进程管理器会写，远端节点恒为空，按库统计会把整台
	// 远端节点误报成「未上报」。
	type svcRow struct {
		ServerID int64
		CoreType string
	}
	var svcRows []svcRow
	svcQuery().Select("inbound_services.server_id, inbound_services.core_type").Scan(&svcRows)
	serviceCount = int64(len(svcRows))
	if h.coreStates != nil {
		byServer := make(map[int64][]string, len(svcRows))
		for _, r := range svcRows {
			byServer[r.ServerID] = append(byServer[r.ServerID], r.CoreType)
		}
		ctx := c.Request.Context()
		for serverID, cores := range byServer {
			states, err := h.coreStates.CoreStates(ctx, serverID)
			for _, core := range cores {
				// 取不到节点状态（Agent 未连接 / 尚未上报）→ 计为「未上报」；
				// 已上报但没有该核心记录 → 与核心进程页口径一致，按「已停止」。
				if err != nil {
					serviceUnknownCount++
					continue
				}
				if states[core] == "running" {
					serviceRunningCount++
				} else {
					serviceStoppedCount++
				}
			}
		}
	} else {
		svcQuery().Where("inbound_services.process_status = ?", "running").Count(&serviceRunningCount)
		svcQuery().Where("inbound_services.process_status = ?", "stopped").Count(&serviceStoppedCount)
		svcQuery().Where("inbound_services.process_status IS NULL OR inbound_services.process_status = ''").Count(&serviceUnknownCount)
	}

	// 订阅按状态拆分：「活跃订阅」只算 active，暂停/过期的单独给出，
	// 避免用户误以为暂停的订阅凭空消失。
	h.db.Model(&entity.Subscription{}).Where("status = ?", "active").Count(&subCount)
	h.db.Model(&entity.Subscription{}).Where("status = ?", "suspended").Count(&subSuspendedCount)
	h.db.Model(&entity.Subscription{}).Where("status = ?", "expired").Count(&subExpiredCount)
	// 订阅人数 = 客户端账号数（一个客户一个 client，对应一个订阅）。
	h.db.Model(&entity.Client{}).Where("status = ?", "active").Count(&clientCount)
	h.db.Model(&entity.Certificate{}).Where("status = ?", "active").Count(&certCount)

	// Count expiring certs (within 30 days)
	var expiringCertCount int64
	h.db.Model(&entity.Certificate{}).Where("status = ? AND expires_at < datetime('now', '+30 days')", "active").Count(&expiringCertCount)

	// Count active subscriptions with quota issues
	var quotaWarningCount int64
	h.db.Model(&entity.Subscription{}).Where("quota_state IN ?", []string{"near_limit", "over_limit_soft", "over_limit_hard"}).Count(&quotaWarningCount)

	// 即将过期证书明细（30 天内，最多 10 条），供前端「异常告警」列出
	var expiringCerts []entity.Certificate
	h.db.Model(&entity.Certificate{}).
		Where("status = ? AND expires_at IS NOT NULL AND expires_at < datetime('now', '+30 days')", "active").
		Order("expires_at ASC").
		Limit(10).
		Find(&expiringCerts)
	expiringCertList := make([]gin.H, 0, len(expiringCerts))
	for _, c := range expiringCerts {
		label := c.Domain
		if label == "" {
			label = c.Name
		}
		expiringCertList = append(expiringCertList, gin.H{
			"id":         c.ID,
			"name":       c.Name,
			"domain":     c.Domain,
			"label":      label,
			"expires_at": c.ExpiresAt,
		})
	}

	response.OK(c, gin.H{
		"node_count":                   nodeCount,
		"service_count":                serviceCount,
		"service_running_count":        serviceRunningCount,
		"service_stopped_count":        serviceStoppedCount,
		"service_unknown_count":        serviceUnknownCount,
		"active_subscription_count":    subCount,
		"suspended_subscription_count": subSuspendedCount,
		"expired_subscription_count":   subExpiredCount,
		"active_client_count":          clientCount,
		"active_cert_count":            certCount,
		"expiring_cert_count":          expiringCertCount,
		"expiring_certs":               expiringCertList,
		"quota_warning_count":          quotaWarningCount,
	})
}

// NodeMetrics handles GET /api/v1/metrics/nodes.
func (h *Handler) NodeMetrics(c *gin.Context) {
	var nodes []entity.Node
	h.db.Find(&nodes)

	var active, inactive int
	for _, n := range nodes {
		if n.Status == 1 {
			active++
		} else {
			inactive++
		}
	}

	response.OK(c, gin.H{
		"total":    len(nodes),
		"active":   active,
		"inactive": inactive,
	})
}

// CertMetrics handles GET /api/v1/metrics/certificates.
func (h *Handler) CertMetrics(c *gin.Context) {
	var total, active, expiring int64
	h.db.Model(&entity.Certificate{}).Count(&total)
	h.db.Model(&entity.Certificate{}).Where("status = ?", "active").Count(&active)
	h.db.Model(&entity.Certificate{}).Where("status = ? AND expires_at < datetime('now', '+30 days')", "active").Count(&expiring)

	response.OK(c, gin.H{
		"total":    total,
		"active":   active,
		"expiring": expiring,
	})
}
