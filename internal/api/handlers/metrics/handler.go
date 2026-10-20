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
	var nodeCount, serviceCount, subCount, certCount int64

	h.db.Model(&entity.Node{}).Count(&nodeCount)
	// 只统计挂在真实存在的节点服务器上的入站服务，排除指向已删除服务器的孤儿记录
	h.db.Table("inbound_services").
		Joins("INNER JOIN node_servers ON node_servers.id = inbound_services.server_id").
		Count(&serviceCount)
	h.db.Model(&entity.Subscription{}).Where("status = ?", "active").Count(&subCount)
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
		"node_count":                nodeCount,
		"service_count":             serviceCount,
		"active_subscription_count": subCount,
		"active_cert_count":         certCount,
		"expiring_cert_count":       expiringCertCount,
		"expiring_certs":            expiringCertList,
		"quota_warning_count":       quotaWarningCount,
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
