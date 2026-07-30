package services

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/service/access"
	"corefusion/internal/service/audit"
)

// AccessHandler exposes the service-level access-control management API
// (access policies + IP bans) backed by the access control service.
type AccessHandler struct {
	db       *gorm.DB
	accessSvc *access.Service
}

// NewAccessHandler creates a new AccessHandler.
func NewAccessHandler(db *gorm.DB, accessSvc *access.Service) *AccessHandler {
	return &AccessHandler{db: db, accessSvc: accessSvc}
}

// --- Access policies -------------------------------------------------------

// ListPolicies lists all access policies for a service.
func (h *AccessHandler) ListPolicies(c *gin.Context) {
	serviceID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var policies []entity.ServiceAccessPolicy
	if err := h.db.Where("service_id = ?", serviceID).Order("id DESC").Find(&policies).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, policies)
}

// CreatePolicy creates a new access policy.
func (h *AccessHandler) CreatePolicy(c *gin.Context) {
	serviceID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var p entity.ServiceAccessPolicy
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	p.ID = 0
	p.ServiceID = serviceID
	now := time.Now()
	p.CreatedAt = now
	p.UpdatedAt = now
	if err := h.db.Create(&p).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(201, p)
}

// UpdatePolicy updates an existing access policy.
func (h *AccessHandler) UpdatePolicy(c *gin.Context) {
	serviceID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	policyID, _ := strconv.ParseInt(c.Param("policyId"), 10, 64)
	var p entity.ServiceAccessPolicy
	if err := h.db.Where("id = ? AND service_id = ?", policyID, serviceID).First(&p).Error; err != nil {
		c.JSON(404, gin.H{"error": "policy not found"})
		return
	}
	var in entity.ServiceAccessPolicy
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if err := h.db.Model(&p).Updates(map[string]interface{}{
		"policy_scope":               in.PolicyScope,
		"mode":                       in.Mode,
		"ip_cidr":                    in.IPCIDR,
		"max_new_connections_per_min": in.MaxNewConnectionsPerMin,
		"max_concurrent_connections":  in.MaxConcurrentConnections,
		"ban_duration_seconds":        in.BanDurationSeconds,
		"auto_ban_enabled":            in.AutoBanEnabled,
		"status":                     in.Status,
		"updated_at":                 time.Now(),
	}).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, p)
}

// DeletePolicy deletes an access policy.
func (h *AccessHandler) DeletePolicy(c *gin.Context) {
	serviceID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	policyID, _ := strconv.ParseInt(c.Param("policyId"), 10, 64)
	if err := h.db.Where("id = ? AND service_id = ?", policyID, serviceID).Delete(&entity.ServiceAccessPolicy{}).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"success": true})
}

// --- IP bans ----------------------------------------------------------------

// ListIPBans lists all IP bans for a service.
func (h *AccessHandler) ListIPBans(c *gin.Context) {
	serviceID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var bans []entity.ServiceIPBan
	if err := h.db.Where("service_id = ?", serviceID).Order("id DESC").Find(&bans).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, bans)
}

// CreateIPBan creates a manual IP ban.
func (h *AccessHandler) CreateIPBan(c *gin.Context) {
	serviceID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var b entity.ServiceIPBan
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	b.ID = 0
	b.ServiceID = serviceID
	b.Source = "manual"
	now := time.Now()
	b.CreatedAt = now
	b.UpdatedAt = now
	if err := h.db.Create(&b).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	audit.Record(audit.Event{
		Category: "admin", EventType: "ip_ban_created", Result: "success",
		ServiceID: serviceID, UserID: c.GetInt64("user_id"), RemoteIP: c.ClientIP(),
		Reason: b.Reason, Metadata: map[string]any{"ip_cidr": b.IPCIDR, "ban_id": b.ID},
	})
	c.JSON(201, b)
}

// DeleteIPBan deletes an IP ban.
func (h *AccessHandler) DeleteIPBan(c *gin.Context) {
	serviceID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	banID, _ := strconv.ParseInt(c.Param("banId"), 10, 64)
	if err := h.db.Where("id = ? AND service_id = ?", banID, serviceID).Delete(&entity.ServiceIPBan{}).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	audit.Record(audit.Event{
		Category: "admin", EventType: "ip_ban_deleted", Result: "success",
		ServiceID: serviceID, UserID: c.GetInt64("user_id"), RemoteIP: c.ClientIP(),
		Metadata: map[string]any{"ban_id": banID},
	})
	c.JSON(200, gin.H{"success": true})
}
