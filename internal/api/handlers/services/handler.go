package services

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/model/dto"
	"corefusion/internal/model/entity"
	svcService "corefusion/internal/service/service"
)

// Handler handles inbound service HTTP requests.
type Handler struct {
	svcSvc *svcService.Service
}

// NewHandler creates a new service handler.
func NewHandler(svcSvc *svcService.Service) *Handler {
	return &Handler{svcSvc: svcSvc}
}

// List handles GET /api/v1/node-servers/:id/services.
func (h *Handler) List(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		serverID = 1 // fallback
	}
	var q struct {
		dto.ListQuery
		CoreType string `form:"core_type"`
	}
	if err := c.ShouldBindQuery(&q); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if q.Limit <= 0 || q.Limit > 100 {
		q.Limit = 20
	}

	services, total, err := h.svcSvc.List(c.Request.Context(), serverID, q.CoreType, q.Limit, q.Offset)
	if err != nil {
		response.InternalError(c, "LIST_SERVICES_FAILED", err.Error())
		return
	}

	items := make([]gin.H, len(services))
	for i, s := range services {
		items[i] = svcService.ServiceToJSON(&s)
	}

	response.Paginated(c, items, total, q.Limit, q.Offset)
}

// Create handles POST /api/v1/node-servers/:id/services.
func (h *Handler) Create(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		serverID = 1
	}
	var input svcService.CreateServiceInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	input.ServerID = serverID

	svc, err := h.svcSvc.Create(c.Request.Context(), input)
	if err != nil {
		response.InternalError(c, "CREATE_SERVICE_FAILED", err.Error())
		return
	}

	response.Created(c, svcService.ServiceToJSON(svc))
}

// ensureServiceBelongsToServer 校验 :serviceId 属于路径里的 :id 节点。
// 旧实现只按 serviceId 查库，跨节点可读/改/删（安全审计 v6 M-3）。
func (h *Handler) ensureServiceBelongsToServer(c *gin.Context, svc *entity.InboundService) bool {
	sid, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || sid <= 0 || svc.ServerID == sid {
		return true
	}
	response.NotFound(c, "SERVICE_NOT_FOUND", "service not found")
	return false
}

// GetByID handles GET /api/v1/node-servers/:id/services/:serviceId.
func (h *Handler) GetByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("serviceId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid service id")
		return
	}

	svc, endpoints, err := h.svcSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "SERVICE_NOT_FOUND", "service not found")
		return
	}
	if !h.ensureServiceBelongsToServer(c, svc) {
		return
	}

	result := svcService.ServiceToJSON(svc)
	result["endpoints"] = endpoints

	response.OK(c, result)
}

// Update handles PUT /api/v1/node-servers/:id/services/:serviceId.
func (h *Handler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("serviceId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid service id")
		return
	}

	var input svcService.UpdateServiceInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	// 先校验归属，避免「用 A 节点 URL 改 B 节点服务」。
	existing, _, err := h.svcSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "SERVICE_NOT_FOUND", "service not found")
		return
	}
	if !h.ensureServiceBelongsToServer(c, existing) {
		return
	}

	svc, err := h.svcSvc.Update(c.Request.Context(), id, input)
	if err != nil {
		response.InternalError(c, "UPDATE_SERVICE_FAILED", err.Error())
		return
	}

	response.OK(c, svcService.ServiceToJSON(svc))
}

// Delete handles DELETE /api/v1/node-servers/:id/services/:serviceId.
func (h *Handler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("serviceId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid service id")
		return
	}

	existing, _, err := h.svcSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "SERVICE_NOT_FOUND", "service not found")
		return
	}
	if !h.ensureServiceBelongsToServer(c, existing) {
		return
	}

	if err := h.svcSvc.Delete(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_SERVICE_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"message": "service deleted"})
}

// ListEndpoints handles GET /api/v1/node-servers/:id/services/:serviceId/endpoints.
func (h *Handler) ListEndpoints(c *gin.Context) {
	serviceID, err := strconv.ParseInt(c.Param("serviceId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid service id")
		return
	}

	svc, _, err := h.svcSvc.GetByID(c.Request.Context(), serviceID)
	if err != nil {
		response.NotFound(c, "SERVICE_NOT_FOUND", "service not found")
		return
	}
	if !h.ensureServiceBelongsToServer(c, svc) {
		return
	}

	endpoints, err := h.svcSvc.ListEndpoints(c.Request.Context(), serviceID)
	if err != nil {
		response.InternalError(c, "LIST_ENDPOINTS_FAILED", err.Error())
		return
	}

	response.OK(c, endpoints)
}

// CreateEndpoint handles POST /api/v1/node-servers/:id/services/:serviceId/endpoints.
func (h *Handler) CreateEndpoint(c *gin.Context) {
	serviceID, err := strconv.ParseInt(c.Param("serviceId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid service id")
		return
	}

	var input svcService.CreateEndpointInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	svc, _, err := h.svcSvc.GetByID(c.Request.Context(), serviceID)
	if err != nil {
		response.NotFound(c, "SERVICE_NOT_FOUND", "service not found")
		return
	}
	if !h.ensureServiceBelongsToServer(c, svc) {
		return
	}

	ep, err := h.svcSvc.CreateEndpoint(c.Request.Context(), serviceID, input)
	if err != nil {
		response.InternalError(c, "CREATE_ENDPOINT_FAILED", err.Error())
		return
	}

	response.Created(c, ep)
}

// UpdateEndpoint handles PUT /api/v1/services/:id/endpoints/:endpointId.
func (h *Handler) UpdateEndpoint(c *gin.Context) {
	endpointID, err := strconv.ParseInt(c.Param("endpointId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid endpoint id")
		return
	}

	var input svcService.UpdateEndpointInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	ep, err := h.svcSvc.UpdateEndpoint(c.Request.Context(), endpointID, input)
	if err != nil {
		response.InternalError(c, "UPDATE_ENDPOINT_FAILED", err.Error())
		return
	}

	response.OK(c, ep)
}

// DeleteEndpoint handles DELETE /api/v1/services/:id/endpoints/:endpointId.
func (h *Handler) DeleteEndpoint(c *gin.Context) {
	endpointID, err := strconv.ParseInt(c.Param("endpointId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid endpoint id")
		return
	}

	if err := h.svcSvc.DeleteEndpoint(c.Request.Context(), endpointID); err != nil {
		response.InternalError(c, "DELETE_ENDPOINT_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"message": "endpoint deleted"})
}
