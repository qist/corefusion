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
	h.db.Model(&entity.InboundService{}).Count(&serviceCount)
	h.db.Model(&entity.Subscription{}).Where("status = ?", "active").Count(&subCount)
	h.db.Model(&entity.Certificate{}).Where("status = ?", "active").Count(&certCount)

	// Count expiring certs (within 30 days)
	var expiringCertCount int64
	h.db.Model(&entity.Certificate{}).Where("status = ? AND expires_at < datetime('now', '+30 days')", "active").Count(&expiringCertCount)

	// Count active subscriptions with quota issues
	var quotaWarningCount int64
	h.db.Model(&entity.Subscription{}).Where("quota_state IN ?", []string{"near_limit", "over_limit_soft", "over_limit_hard"}).Count(&quotaWarningCount)

	response.OK(c, gin.H{
		"node_count":          nodeCount,
		"service_count":       serviceCount,
		"active_subscription_count": subCount,
		"active_cert_count":   certCount,
		"expiring_cert_count": expiringCertCount,
		"quota_warning_count": quotaWarningCount,
	})
}

// ServiceMetrics handles GET /api/v1/metrics/services.
func (h *Handler) ServiceMetrics(c *gin.Context) {
	var services []entity.InboundService
	h.db.Find(&services)

	items := make([]gin.H, len(services))
	for i, s := range services {
		items[i] = gin.H{
			"service_id":     s.ID,
			"name":           s.Name,
			"core_type":      s.CoreType,
			"process_status": s.ProcessStatus,
		}
	}

	response.OK(c, items)
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


