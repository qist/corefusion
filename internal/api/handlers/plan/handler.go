package plan

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	planSvc "corefusion/internal/service/plan"
)

// Handler handles plan template, variant, node group, promotion, and order HTTP requests.
type Handler struct {
	svc *planSvc.Service
}

// NewHandler creates a new plan handler.
func NewHandler(svc *planSvc.Service) *Handler {
	return &Handler{svc: svc}
}

// ──────────────────── Plan Template ────────────────────

// ListTemplates handles GET /api/v1/plan-templates.
func (h *Handler) ListTemplates(c *gin.Context) {
	templates, err := h.svc.ListTemplates(c.Request.Context())
	if err != nil {
		response.InternalError(c, "LIST_TEMPLATES_FAILED", err.Error())
		return
	}
	response.OK(c, templates)
}

// GetTemplate handles GET /api/v1/plan-templates/:id.
func (h *Handler) GetTemplate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid template id")
		return
	}
	tmpl, variants, err := h.svc.GetTemplate(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "TEMPLATE_NOT_FOUND", err.Error())
		return
	}
	response.OK(c, gin.H{"template": tmpl, "variants": variants})
}

// CreateTemplate handles POST /api/v1/plan-templates.
func (h *Handler) CreateTemplate(c *gin.Context) {
	var input planSvc.CreateTemplateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	tmpl, err := h.svc.CreateTemplate(c.Request.Context(), input)
	if err != nil {
		response.InternalError(c, "CREATE_TEMPLATE_FAILED", err.Error())
		return
	}
	response.Created(c, tmpl)
}

// UpdateTemplate handles PUT /api/v1/plan-templates/:id.
func (h *Handler) UpdateTemplate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid template id")
		return
	}
	var input planSvc.UpdateTemplateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	tmpl, err := h.svc.UpdateTemplate(c.Request.Context(), id, input)
	if err != nil {
		response.InternalError(c, "UPDATE_TEMPLATE_FAILED", err.Error())
		return
	}
	response.OK(c, tmpl)
}

// DeleteTemplate handles DELETE /api/v1/plan-templates/:id.
func (h *Handler) DeleteTemplate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid template id")
		return
	}
	if err := h.svc.DeleteTemplate(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_TEMPLATE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

// SetTemplateNodeGroups handles PUT /api/v1/plan-templates/:id/node-groups.
func (h *Handler) SetTemplateNodeGroups(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid template id")
		return
	}
	var req struct {
		NodeGroupIDs []int64 `json:"node_group_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if err := h.svc.SetTemplateNodeGroups(c.Request.Context(), id, req.NodeGroupIDs); err != nil {
		response.InternalError(c, "SET_NODE_GROUPS_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"updated": true})
}

// GetAccessibleNodes handles GET /api/v1/plan-templates/:id/accessible-nodes.
func (h *Handler) GetAccessibleNodes(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid template id")
		return
	}
	result, err := h.svc.GetAccessibleNodes(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "ACCESSIBLE_NODES_FAILED", err.Error())
		return
	}
	response.OK(c, result)
}

// GetBillingNodeOptions handles GET /api/v1/plan-templates/:id/billing-node-options.
// Returns enriched node group members with host server name, inbound/node name,
// and protocol:port for billing node selection in the UI.
func (h *Handler) GetBillingNodeOptions(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid template id")
		return
	}
	options, err := h.svc.GetBillingNodeOptions(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "BILLING_NODE_OPTIONS_FAILED", err.Error())
		return
	}
	response.OK(c, options)
}

// SyncAccountsForTemplate handles POST /api/v1/plan-templates/:id/sync-accounts.
// Manually triggers account synchronization for all clients bound to the given
// plan template across all accessible inbound configs. Used to repair existing
// data where clients are missing accounts on nodes.
func (h *Handler) SyncAccountsForTemplate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid template id")
		return
	}
	count, err := h.svc.SyncAccountsForTemplate(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "SYNC_ACCOUNTS_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"synced": true, "inbounds_processed": count})
}

// SyncAllAccounts handles POST /api/v1/plan-templates/sync-all-accounts.
// Manually triggers account synchronization for ALL enabled plan templates.
// Bulk repair operation for existing data.
func (h *Handler) SyncAllAccounts(c *gin.Context) {
	results, err := h.svc.SyncAllAccounts(c.Request.Context())
	if err != nil {
		response.InternalError(c, "SYNC_ALL_ACCOUNTS_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"synced": true, "templates": results})
}

// ListAllInboundsWithServer handles GET /api/v1/plan-templates/inbounds-with-server.
// Returns all inbound configs across all node servers, enriched with the
// server name for display in the member management UI.
func (h *Handler) ListAllInboundsWithServer(c *gin.Context) {
	result, err := h.svc.ListAllInboundsWithServer(c.Request.Context())
	if err != nil {
		response.InternalError(c, "LIST_INBOUNDS_WITH_SERVER_FAILED", err.Error())
		return
	}
	response.OK(c, result)
}

// ──────────────────── Plan Variant ────────────────────

// ListVariants handles GET /api/v1/plan-templates/:id/variants.
func (h *Handler) ListVariants(c *gin.Context) {
	tmplID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid template id")
		return
	}
	variants, err := h.svc.ListVariants(c.Request.Context(), tmplID)
	if err != nil {
		response.InternalError(c, "LIST_VARIANTS_FAILED", err.Error())
		return
	}
	response.OK(c, variants)
}

// CreateVariant handles POST /api/v1/plan-templates/:id/variants.
func (h *Handler) CreateVariant(c *gin.Context) {
	tmplID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid template id")
		return
	}
	var input planSvc.CreateVariantInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	variant, err := h.svc.CreateVariant(c.Request.Context(), tmplID, input)
	if err != nil {
		response.InternalError(c, "CREATE_VARIANT_FAILED", err.Error())
		return
	}
	response.Created(c, variant)
}

// UpdateVariant handles PUT /api/v1/plan-variants/:id.
func (h *Handler) UpdateVariant(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid variant id")
		return
	}
	var input planSvc.UpdateVariantInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	variant, err := h.svc.UpdateVariant(c.Request.Context(), id, input)
	if err != nil {
		response.InternalError(c, "UPDATE_VARIANT_FAILED", err.Error())
		return
	}
	response.OK(c, variant)
}

// DeleteVariant handles DELETE /api/v1/plan-variants/:id.
func (h *Handler) DeleteVariant(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid variant id")
		return
	}
	if err := h.svc.DeleteVariant(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_VARIANT_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

// ──────────────────── Node Group ────────────────────

// ListNodeGroups handles GET /api/v1/node-groups.
func (h *Handler) ListNodeGroups(c *gin.Context) {
	groups, err := h.svc.ListNodeGroups(c.Request.Context())
	if err != nil {
		response.InternalError(c, "LIST_NODE_GROUPS_FAILED", err.Error())
		return
	}
	response.OK(c, groups)
}

// CreateNodeGroup handles POST /api/v1/node-groups.
func (h *Handler) CreateNodeGroup(c *gin.Context) {
	var input planSvc.CreateNodeGroupInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	group, err := h.svc.CreateNodeGroup(c.Request.Context(), input)
	if err != nil {
		response.InternalError(c, "CREATE_NODE_GROUP_FAILED", err.Error())
		return
	}
	response.Created(c, group)
}

// UpdateNodeGroup handles PUT /api/v1/node-groups/:id.
func (h *Handler) UpdateNodeGroup(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid node group id")
		return
	}
	var input planSvc.UpdateNodeGroupInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	group, err := h.svc.UpdateNodeGroup(c.Request.Context(), id, input)
	if err != nil {
		response.InternalError(c, "UPDATE_NODE_GROUP_FAILED", err.Error())
		return
	}
	response.OK(c, group)
}

// DeleteNodeGroup handles DELETE /api/v1/node-groups/:id.
func (h *Handler) DeleteNodeGroup(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid node group id")
		return
	}
	if err := h.svc.DeleteNodeGroup(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_NODE_GROUP_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

// ListGroupMembers handles GET /api/v1/node-groups/:id/members.
func (h *Handler) ListGroupMembers(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid node group id")
		return
	}
	members, err := h.svc.ListGroupMembers(c.Request.Context(), groupID)
	if err != nil {
		response.InternalError(c, "LIST_MEMBERS_FAILED", err.Error())
		return
	}
	response.OK(c, members)
}

// AddGroupMember handles POST /api/v1/node-groups/:id/members.
func (h *Handler) AddGroupMember(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid node group id")
		return
	}
	var input planSvc.AddGroupMemberInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	member, err := h.svc.AddGroupMember(c.Request.Context(), groupID, input)
	if err != nil {
		response.InternalError(c, "ADD_MEMBER_FAILED", err.Error())
		return
	}
	response.Created(c, member)
}

// RemoveGroupMember handles DELETE /api/v1/node-groups/:id/members/:mid.
func (h *Handler) RemoveGroupMember(c *gin.Context) {
	memberID, err := strconv.ParseInt(c.Param("mid"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid member id")
		return
	}
	if err := h.svc.RemoveGroupMember(c.Request.Context(), memberID); err != nil {
		response.InternalError(c, "REMOVE_MEMBER_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

// ──────────────────── Promotion ────────────────────

// ListPromotions handles GET /api/v1/promotions.
func (h *Handler) ListPromotions(c *gin.Context) {
	limit := 50
	offset := 0
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil {
			limit = v
		}
	}
	if o := c.Query("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil {
			offset = v
		}
	}
	promos, total, err := h.svc.ListPromotions(c.Request.Context(), limit, offset)
	if err != nil {
		response.InternalError(c, "LIST_PROMOTIONS_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"items": promos, "total": total})
}

// CreatePromotion handles POST /api/v1/promotions.
func (h *Handler) CreatePromotion(c *gin.Context) {
	var input planSvc.CreatePromotionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	promo, err := h.svc.CreatePromotion(c.Request.Context(), input)
	if err != nil {
		response.InternalError(c, "CREATE_PROMOTION_FAILED", err.Error())
		return
	}
	response.Created(c, promo)
}

// UpdatePromotion handles PUT /api/v1/promotions/:id.
func (h *Handler) UpdatePromotion(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid promotion id")
		return
	}
	var input planSvc.UpdatePromotionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	promo, err := h.svc.UpdatePromotion(c.Request.Context(), id, input)
	if err != nil {
		response.InternalError(c, "UPDATE_PROMOTION_FAILED", err.Error())
		return
	}
	response.OK(c, promo)
}

// DeletePromotion handles DELETE /api/v1/promotions/:id.
func (h *Handler) DeletePromotion(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid promotion id")
		return
	}
	if err := h.svc.DeletePromotion(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_PROMOTION_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

// ValidatePromotion handles POST /api/v1/promotions/validate.
func (h *Handler) ValidatePromotion(c *gin.Context) {
	var req struct {
		Code      string `json:"code" binding:"required"`
		VariantID int64  `json:"variant_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	result, err := h.svc.ValidatePromotion(c.Request.Context(), req.Code, req.VariantID)
	if err != nil {
		response.InternalError(c, "VALIDATE_PROMOTION_FAILED", err.Error())
		return
	}
	response.OK(c, result)
}

// ──────────────────── Client Plan Operations ────────────────────

// ListClientOrders handles GET /api/v1/clients/:id/orders.
func (h *Handler) ListClientOrders(c *gin.Context) {
	clientID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid client id")
		return
	}
	limit := 50
	offset := 0
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil {
			limit = v
		}
	}
	if o := c.Query("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil {
			offset = v
		}
	}
	orders, total, err := h.svc.ListClientOrders(c.Request.Context(), clientID, limit, offset)
	if err != nil {
		response.InternalError(c, "LIST_ORDERS_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"items": orders, "total": total})
}

// Renew handles POST /api/v1/clients/:id/renew.
func (h *Handler) Renew(c *gin.Context) {
	clientID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid client id")
		return
	}
	var input planSvc.RenewInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	result, err := h.svc.Renew(c.Request.Context(), clientID, input)
	if err != nil {
		response.InternalError(c, "RENEW_FAILED", err.Error())
		return
	}
	response.OK(c, result)
}

// Upgrade handles POST /api/v1/clients/:id/upgrade.
func (h *Handler) Upgrade(c *gin.Context) {
	clientID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid client id")
		return
	}
	var input planSvc.UpgradeInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	result, err := h.svc.Upgrade(c.Request.Context(), clientID, input)
	if err != nil {
		response.InternalError(c, "UPGRADE_FAILED", err.Error())
		return
	}
	response.OK(c, result)
}

// Downgrade handles POST /api/v1/clients/:id/downgrade.
func (h *Handler) Downgrade(c *gin.Context) {
	clientID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid client id")
		return
	}
	var input planSvc.DowngradeInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	result, err := h.svc.Downgrade(c.Request.Context(), clientID, input)
	if err != nil {
		response.InternalError(c, "DOWNGRADE_FAILED", err.Error())
		return
	}
	response.OK(c, result)
}
