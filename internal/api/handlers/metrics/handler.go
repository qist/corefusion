package metrics

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"corefusion/internal/api/response"
	"corefusion/internal/model/entity"
)

// Handler handles metrics HTTP requests.
type Handler struct {
	db *gorm.DB
}

// NewHandler creates a new metrics handler.
func NewHandler(db *gorm.DB) *Handler {
	return &Handler{db: db}
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
	// 三种状态分开统计：running / stopped / 未上报。
	// 远端节点的进程状态由 Agent 上报，尚未上报时 process_status 为空 ——
	// 不能当成「已停止」，否则远端节点看起来像全挂了。
	svcQuery().Where("inbound_services.process_status = ?", "running").Count(&serviceRunningCount)
	svcQuery().Where("inbound_services.process_status = ?", "stopped").Count(&serviceStoppedCount)
	svcQuery().Where("inbound_services.process_status IS NULL OR inbound_services.process_status = ''").Count(&serviceUnknownCount)

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
