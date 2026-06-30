package nodeserver

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/model/dto"
	nodeserverSvc "corefusion/internal/service/nodeserver"
)

// Handler handles node server HTTP requests.
type Handler struct {
	svc *nodeserverSvc.Service
}

// NewHandler creates a new node server handler.
func NewHandler(svc *nodeserverSvc.Service) *Handler {
	return &Handler{svc: svc}
}

// List returns all node servers with optional filtering.
func (h *Handler) List(c *gin.Context) {
	var q dto.NodeServerListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.BadRequest(c, "INVALID_QUERY", err.Error())
		return
	}

	servers, total, err := h.svc.List(c.Request.Context(), &q)
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}

	items := make([]interface{}, len(servers))
	for i, s := range servers {
		items[i] = h.svc.ToResponse(&s)
	}

	response.OK(c, gin.H{
		"items": items,
		"total": total,
	})
}

// GetByID returns a single node server by ID.
func (h *Handler) GetByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}

	server, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", "server not found")
		return
	}

	response.OK(c, h.svc.ToResponse(server))
}

// Create creates a new node server.
func (h *Handler) Create(c *gin.Context) {
	var req dto.NodeServerCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_BODY", err.Error())
		return
	}

	server, err := h.svc.Create(c.Request.Context(), &req)
	if err != nil {
		response.InternalError(c, "CREATE_FAILED", err.Error())
		return
	}

	response.Created(c, h.svc.ToResponse(server))
}

// Update updates a node server.
func (h *Handler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}

	var req dto.NodeServerUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_BODY", err.Error())
		return
	}

	server, err := h.svc.Update(c.Request.Context(), id, &req)
	if err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}

	response.OK(c, h.svc.ToResponse(server))
}

// Delete removes a node server.
func (h *Handler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"deleted": true})
}

// GenerateBootstrapToken generates a bootstrap token for a server.
// The token is returned in plaintext — it is only shown once.
func (h *Handler) GenerateBootstrapToken(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}

	result, err := h.svc.GenerateBootstrapToken(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "TOKEN_GEN_FAILED", err.Error())
		return
	}

	response.OK(c, result)
}

// GetCACert returns the panel's CA root certificate PEM.
// This is used by the Agent to verify the panel's TLS certificate.
func (h *Handler) GetCACert(c *gin.Context) {
	pem := h.svc.GetCACertPEM()
	if pem == nil {
		response.NotFound(c, "CA_NOT_INITIALIZED", "CA not initialized")
		return
	}

	c.Data(http.StatusOK, "application/x-pem-file", pem)
}

// RevokeAgentCertificate revokes a server's Agent certificate.
// The Agent will be disconnected and its cert added to the CRL.
func (h *Handler) RevokeAgentCertificate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		// Allow empty body with default reason
		req.Reason = "manual_revocation"
	}

	// adminID from JWT context (0 if not available)
	adminID, _ := c.Get("user_id")
	var aid int64
	if v, ok := adminID.(int64); ok {
		aid = v
	}

	if err := h.svc.RevokeAgentCertificate(c.Request.Context(), id, aid, req.Reason); err != nil {
		response.InternalError(c, "REVOKE_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"revoked": true, "server_id": id, "reason": req.Reason})
}

// DistributeCertificate distributes a TLS certificate to a specific server.
func (h *Handler) DistributeCertificate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}

	var req struct {
		CertificateID int64 `json:"certificate_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_BODY", err.Error())
		return
	}

	if err := h.svc.DistributeCertificate(c.Request.Context(), id, req.CertificateID); err != nil {
		response.InternalError(c, "DISTRIBUTE_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"distributed": true, "server_id": id, "certificate_id": req.CertificateID})
}

// DistributeCertificateToAll distributes a TLS certificate to all associated servers.
func (h *Handler) DistributeCertificateToAll(c *gin.Context) {
	var req struct {
		CertificateID int64 `json:"certificate_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_BODY", err.Error())
		return
	}

	if err := h.svc.DistributeCertificateToAll(c.Request.Context(), req.CertificateID); err != nil {
		response.InternalError(c, "DISTRIBUTE_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"distributed": true, "certificate_id": req.CertificateID})
}

// ──────────────────── Phase 3 Endpoints ────────────────────

// GetFailoverStatus returns the failover status for a primary_backup server.
func (h *Handler) GetFailoverStatus(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}

	status, err := h.svc.GetFailoverStatus(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "FAILOVER_STATUS_FAILED", err.Error())
		return
	}

	response.OK(c, status)
}

// TriggerFailoverCheck triggers a manual failover check.
func (h *Handler) TriggerFailoverCheck(c *gin.Context) {
	if err := h.svc.TriggerFailoverCheck(c.Request.Context()); err != nil {
		response.InternalError(c, "FAILOVER_CHECK_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"triggered": true})
}

// SyncMirror triggers an immediate mirror sync for a specific mirror node.
func (h *Handler) SyncMirror(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}

	if err := h.svc.SyncMirrorNow(c.Request.Context(), id); err != nil {
		response.InternalError(c, "MIRROR_SYNC_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"synced": true, "mirror_id": id})
}

// GetMirrorStatus returns the sync status of a mirror node.
func (h *Handler) GetMirrorStatus(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}

	status, err := h.svc.GetMirrorStatus(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "MIRROR_STATUS_FAILED", err.Error())
		return
	}

	response.OK(c, status)
}

// GenerateChainConfig generates chain proxy outbound config for a chain_entry node.
func (h *Handler) GenerateChainConfig(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}

	outbounds, rules, err := h.svc.GenerateChainConfig(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "CHAIN_CONFIG_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{
		"outbounds":     outbounds,
		"routing_rules": rules,
	})
}

// GenerateChainExitConfig generates relay inbound config for a chain_exit node.
func (h *Handler) GenerateChainExitConfig(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}

	inbound, err := h.svc.GenerateChainExitConfig(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "CHAIN_EXIT_CONFIG_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"inbound": inbound})
}

// GetChainHealthStatus returns health status of all chain_exit nodes for a chain_entry.
func (h *Handler) GetChainHealthStatus(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}

	statuses, err := h.svc.GetChainHealthStatus(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "CHAIN_HEALTH_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"exits": statuses})
}

// ListBillableServers returns server IDs that participate in billing.
func (h *Handler) ListBillableServers(c *gin.Context) {
	ids, err := h.svc.ListBillableServerIDs(c.Request.Context())
	if err != nil {
		response.InternalError(c, "BILLING_FILTER_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"server_ids": ids, "count": len(ids)})
}

// ValidateChainConfig validates a chain_config JSON.
func (h *Handler) ValidateChainConfig(c *gin.Context) {
	var req struct {
		NodeType      string `json:"node_type" binding:"required"`
		ChainConfig   string `json:"chain_config"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_BODY", err.Error())
		return
	}

	if err := h.svc.ValidateChainConfig(req.NodeType, req.ChainConfig); err != nil {
		response.BadRequest(c, "VALIDATION_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"valid": true})
}

// ──────────────────── Phase 4 Endpoints ────────────────────

// SyncUser triggers user credential sync to all servers.
// POST /node-servers/users/sync  {client_id, action}
func (h *Handler) SyncUser(c *gin.Context) {
	var req struct {
		ClientID int64  `json:"client_id" binding:"required"`
		Action   string `json:"action" binding:"required"` // add, update, remove
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_BODY", err.Error())
		return
	}

	if err := h.svc.SyncUserToServers(c.Request.Context(), req.ClientID, req.Action); err != nil {
		response.InternalError(c, "SYNC_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"synced": true, "client_id": req.ClientID, "action": req.Action})
}

// SyncUserRemoval removes a user from all servers.
// DELETE /node-servers/users/:client_id
func (h *Handler) SyncUserRemoval(c *gin.Context) {
	clientID, err := strconv.ParseInt(c.Param("client_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid client ID")
		return
	}

	if err := h.svc.SyncUserRemoval(c.Request.Context(), clientID); err != nil {
		response.InternalError(c, "SYNC_REMOVAL_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"removed": true, "client_id": clientID})
}

// GetRetryQueueStatus returns the current retry queue size.
func (h *Handler) GetRetryQueueStatus(c *gin.Context) {
	size := h.svc.GetRetryQueueSize()
	response.OK(c, gin.H{"pending_count": size})
}

// ProcessRetryQueue manually triggers retry queue processing.
func (h *Handler) ProcessRetryQueue(c *gin.Context) {
	h.svc.ProcessRetryQueue(c.Request.Context())
	response.OK(c, gin.H{"processed": true})
}

// GetTrafficStats returns multi-server traffic collector statistics.
func (h *Handler) GetTrafficStats(c *gin.Context) {
	stats := h.svc.GetTrafficStats()
	if stats == nil {
		response.OK(c, gin.H{"enabled": false})
		return
	}
	response.OK(c, gin.H{"enabled": true, "stats": stats})
}

// ──────────────────── Phase 5 Endpoints ────────────────────

// GetDashboardStatus returns the aggregated server health summary.
func (h *Handler) GetDashboardStatus(c *gin.Context) {
	summary, err := h.svc.GetDashboardStatus(c.Request.Context())
	if err != nil {
		response.InternalError(c, "DASHBOARD_FAILED", err.Error())
		return
	}
	response.OK(c, summary)
}

// RecommendRegion recommends a region for a client IP.
func (h *Handler) RecommendRegion(c *gin.Context) {
	var req struct {
		ClientIP       string   `json:"client_ip"`
		AllowedRegions []string `json:"allowed_regions" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_BODY", err.Error())
		return
	}

	region := h.svc.RecommendRegion(req.ClientIP, req.AllowedRegions)
	response.OK(c, gin.H{"recommended_region": region})
}

// GetAgentInstallScript generates an install script for a server's Agent.
func (h *Handler) GetAgentInstallScript(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}

	// Panel host/port from query params or defaults
	panelHost := c.Query("host")
	if panelHost == "" {
		panelHost = c.Request.Host
		// Strip port if present
		if idx := len(panelHost) - 1; idx > 0 {
			for i := idx; i >= 0; i-- {
				if panelHost[i] == ':' {
					panelHost = panelHost[:i]
					break
				}
			}
		}
	}

	panelPort := 8080
	if p := c.Query("port"); p != "" {
		if port, err := strconv.Atoi(p); err == nil {
			panelPort = port
		}
	}

	script, err := h.svc.GenerateAgentInstallScript(c.Request.Context(), id, panelHost, panelPort)
	if err != nil {
		response.InternalError(c, "SCRIPT_GEN_FAILED", err.Error())
		return
	}

	c.Data(http.StatusOK, "text/x-shellscript", []byte(script))
}

// ──────────────────── Phase 6 Endpoints ────────────────────

// GetMetricsHistory returns historical metrics for a server.
func (h *Handler) GetMetricsHistory(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}

	duration := 1 * time.Hour
	if d := c.Query("duration"); d != "" {
		if parsed, err := time.ParseDuration(d); err == nil {
			duration = parsed
		}
	}
	interval := 5 * time.Minute
	if i := c.Query("interval"); i != "" {
		if parsed, err := time.ParseDuration(i); err == nil {
			interval = parsed
		}
	}

	result, err := h.svc.GetMetricsHistory(id, duration, interval)
	if err != nil {
		response.InternalError(c, "METRICS_HISTORY_FAILED", err.Error())
		return
	}
	response.OK(c, result)
}

// GetAggregatedMetrics returns aggregated metrics across all servers.
func (h *Handler) GetAggregatedMetrics(c *gin.Context) {
	result, err := h.svc.GetAggregatedMetrics(c.Request.Context())
	if err != nil {
		response.InternalError(c, "AGGREGATED_METRICS_FAILED", err.Error())
		return
	}
	response.OK(c, result)
}

// TestServerSpeed tests latency to a specific server.
func (h *Handler) TestServerSpeed(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	result, err := h.svc.TestServerSpeed(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "SPEED_TEST_FAILED", err.Error())
		return
	}
	response.OK(c, result)
}

// TestAllServersSpeed tests latency to all servers.
func (h *Handler) TestAllServersSpeed(c *gin.Context) {
	result, err := h.svc.TestAllServersSpeed(c.Request.Context())
	if err != nil {
		response.InternalError(c, "SPEED_TEST_ALL_FAILED", err.Error())
		return
	}
	response.OK(c, result)
}

// GetSpeedTestResults returns cached speed test results.
func (h *Handler) GetSpeedTestResults(c *gin.Context) {
	response.OK(c, h.svc.GetSpeedTestResults())
}

// SelectNodesForSubscription selects best nodes for a subscription request.
func (h *Handler) SelectNodesForSubscription(c *gin.Context) {
	var req struct {
		UserRegion  string  `json:"user_region"`
		Count       int     `json:"count"`
		ExcludeIDs  []int64 `json:"exclude_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_BODY", err.Error())
		return
	}
	if req.Count <= 0 {
		req.Count = 3
	}
	result, err := h.svc.SelectNodesForSubscription(c.Request.Context(), req.UserRegion, req.Count, req.ExcludeIDs)
	if err != nil {
		response.InternalError(c, "NODE_SELECTION_FAILED", err.Error())
		return
	}
	response.OK(c, result)
}

// UpdateAgent triggers an Agent update on a specific server.
func (h *Handler) UpdateAgent(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	var req struct {
		Version     string `json:"version"`
		DownloadURL string `json:"download_url"`
		Checksum    string `json:"checksum"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_BODY", err.Error())
		return
	}
	if err := h.svc.UpdateAgent(c.Request.Context(), id, req.Version, req.DownloadURL, req.Checksum); err != nil {
		response.InternalError(c, "AGENT_UPDATE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"updating": true, "server_id": id})
}

// UpdateAllAgents triggers Agent updates on all eligible remote servers.
func (h *Handler) UpdateAllAgents(c *gin.Context) {
	if err := h.svc.UpdateAllAgents(c.Request.Context()); err != nil {
		response.InternalError(c, "AGENT_UPDATE_ALL_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"triggered": true})
}

// GetPendingAgentUpdates returns all pending Agent update tasks.
func (h *Handler) GetPendingAgentUpdates(c *gin.Context) {
	response.OK(c, h.svc.GetPendingAgentUpdates())
}

// ──────────────────── Supplementary: Server Management ────────────────────

// GetServerStatus returns real-time status of a server.
func (h *Handler) GetServerStatus(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	result, err := h.svc.GetServerStatus(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "STATUS_FAILED", err.Error())
		return
	}
	response.OK(c, result)
}

// ControlCore sends a control command to a server's core.
func (h *Handler) ControlCore(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	var req struct {
		CoreType string `json:"core_type" binding:"required"`
		Action   string `json:"action" binding:"required"` // start, stop, restart
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_BODY", err.Error())
		return
	}
	if err := h.svc.ControlCore(c.Request.Context(), id, req.CoreType, req.Action); err != nil {
		response.InternalError(c, "CONTROL_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"controlled": true, "server_id": id, "action": req.Action})
}

// SyncServerConfig triggers a manual config sync to a server.
func (h *Handler) SyncServerConfig(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	var req struct {
		CoreType string `json:"core_type" binding:"required"`
		Config   string `json:"config"`
		Version  int64  `json:"version"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_BODY", err.Error())
		return
	}
	if err := h.svc.SyncServerConfig(c.Request.Context(), id, req.CoreType, []byte(req.Config), req.Version); err != nil {
		response.InternalError(c, "SYNC_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"synced": true, "server_id": id})
}

// GetBootstrapCommand returns the bootstrap installation command.
func (h *Handler) GetBootstrapCommand(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	panelHost := c.Query("host")
	if panelHost == "" {
		panelHost = c.Request.Host
		if idx := len(panelHost) - 1; idx > 0 {
			for i := idx; i >= 0; i-- {
				if panelHost[i] == ':' {
					panelHost = panelHost[:i]
					break
				}
			}
		}
	}
	panelPort := 8080
	if p := c.Query("port"); p != "" {
		if port, err := strconv.Atoi(p); err == nil {
			panelPort = port
		}
	}
	script, err := h.svc.GetBootstrapCommand(c.Request.Context(), id, panelHost, panelPort)
	if err != nil {
		response.InternalError(c, "BOOTSTRAP_CMD_FAILED", err.Error())
		return
	}
	c.Data(http.StatusOK, "text/x-shellscript", []byte(script))
}

// UpdateNodeType changes the node type of a server.
func (h *Handler) UpdateNodeType(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	var req struct {
		NodeType string `json:"node_type" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_BODY", err.Error())
		return
	}
	if err := h.svc.UpdateNodeType(c.Request.Context(), id, req.NodeType); err != nil {
		response.InternalError(c, "NODE_TYPE_UPDATE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"updated": true, "server_id": id, "node_type": req.NodeType})
}

// TriggerServerFailover triggers failover for a specific server.
func (h *Handler) TriggerServerFailover(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	if err := h.svc.TriggerServerFailover(c.Request.Context(), id); err != nil {
		response.InternalError(c, "FAILOVER_TRIGGER_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"triggered": true, "server_id": id})
}

// ListServerCertificates returns certificates associated with a server.
func (h *Handler) ListServerCertificates(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	result, err := h.svc.ListServerCertificates(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "LIST_CERTS_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"certificates": result})
}

// AssociateCertificate associates a TLS certificate with a server.
func (h *Handler) AssociateCertificate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	cid, err := strconv.ParseInt(c.Param("cid"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_CERT_ID", "invalid certificate ID")
		return
	}
	if err := h.svc.AssociateCertificate(c.Request.Context(), id, cid); err != nil {
		response.InternalError(c, "ASSOCIATE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"associated": true, "server_id": id, "certificate_id": cid})
}

// DissociateCertificate removes the TLS certificate association.
func (h *Handler) DissociateCertificate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	cid, err := strconv.ParseInt(c.Param("cid"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_CERT_ID", "invalid certificate ID")
		return
	}
	_ = cid // we just clear the association regardless of cid
	if err := h.svc.DissociateCertificate(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DISSOCIATE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"dissociated": true, "server_id": id})
}

// ──────────────────── Supplementary: CA Center ────────────────────

// GetCAStatus returns the CA root certificate status.
func (h *Handler) GetCAStatus(c *gin.Context) {
	response.OK(c, h.svc.GetCAStatus())
}

// ListAgentCerts returns all Agent mTLS certificates.
func (h *Handler) ListAgentCerts(c *gin.Context) {
	result, err := h.svc.ListAgentCerts(c.Request.Context())
	if err != nil {
		response.InternalError(c, "LIST_AGENT_CERTS_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"certs": result})
}

// RenewAgentCertificate renews an Agent's mTLS certificate.
func (h *Handler) RenewAgentCertificate(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("server_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	if err := h.svc.RenewAgentCertificate(c.Request.Context(), serverID); err != nil {
		response.InternalError(c, "RENEW_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"renewed": true, "server_id": serverID})
}

// ListRevocations returns all certificate revocations.
func (h *Handler) ListRevocations(c *gin.Context) {
	result, err := h.svc.ListRevocations(c.Request.Context())
	if err != nil {
		response.InternalError(c, "LIST_REVOCATIONS_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"revocations": result})
}

// ExportCRL exports the CRL.
func (h *Handler) ExportCRL(c *gin.Context) {
	data, err := h.svc.ExportCRL(c.Request.Context())
	if err != nil {
		response.InternalError(c, "EXPORT_CRL_FAILED", err.Error())
		return
	}
	c.Data(http.StatusOK, "application/x-pem-file", data)
}

// GetDistributionQueue returns the cert distribution queue.
func (h *Handler) GetDistributionQueue(c *gin.Context) {
	status := c.Query("status")
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
	items, total, err := h.svc.GetDistributionQueue(c.Request.Context(), status, limit, offset)
	if err != nil {
		response.InternalError(c, "QUEUE_LIST_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"items": items, "total": total})
}

// RetryDistribution retries a failed distribution item.
func (h *Handler) RetryDistribution(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid queue item ID")
		return
	}
	if err := h.svc.RetryDistribution(c.Request.Context(), id); err != nil {
		response.InternalError(c, "RETRY_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"retried": true, "id": id})
}

// DeleteDistributionQueueItem deletes a distribution queue item.
func (h *Handler) DeleteDistributionQueueItem(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid queue item ID")
		return
	}
	if err := h.svc.DeleteDistributionQueueItem(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_QUEUE_ITEM_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"deleted": true, "id": id})
}

// ──────────────────── Supplementary: Region Overview ────────────────────

// GetRegionsOverview returns aggregated stats per region.
func (h *Handler) GetRegionsOverview(c *gin.Context) {
	result, err := h.svc.GetRegionsOverview(c.Request.Context())
	if err != nil {
		response.InternalError(c, "REGIONS_OVERVIEW_FAILED", err.Error())
		return
	}
	response.OK(c, result)
}

// ListServersByRegion returns all servers in a specific region.
func (h *Handler) ListServersByRegion(c *gin.Context) {
	region := c.Param("region")
	servers, err := h.svc.ListServersByRegion(c.Request.Context(), region)
	if err != nil {
		response.InternalError(c, "LIST_BY_REGION_FAILED", err.Error())
		return
	}
	items := make([]interface{}, len(servers))
	for i, s := range servers {
		items[i] = h.svc.ToResponse(&s)
	}
	response.OK(c, gin.H{"items": items, "total": len(items)})
}

// ──────────────────── Supplementary: Node Audit Logs ────────────────────

// ListNodeAuditLogs returns audit logs with optional filtering.
func (h *Handler) ListNodeAuditLogs(c *gin.Context) {
	var q struct {
		ServerID int64  `form:"server_id"`
		Action   string `form:"action"`
		Limit    int    `form:"limit"`
		Offset   int    `form:"offset"`
	}
	if err := c.ShouldBindQuery(&q); err != nil {
		response.BadRequest(c, "INVALID_QUERY", err.Error())
		return
	}
	if q.Limit <= 0 || q.Limit > 200 {
		q.Limit = 50
	}
	logs, total, err := h.svc.ListAuditLogs(c.Request.Context(), q.ServerID, q.Action, q.Limit, q.Offset)
	if err != nil {
		response.InternalError(c, "AUDIT_LOGS_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"items": logs, "total": total, "limit": q.Limit, "offset": q.Offset})
}

// ExportNodeAuditLogs exports audit logs as CSV.
func (h *Handler) ExportNodeAuditLogs(c *gin.Context) {
	from := time.Now().Add(-24 * time.Hour)
	to := time.Now()
	if f := c.Query("from"); f != "" {
		if parsed, err := time.Parse(time.RFC3339, f); err == nil {
			from = parsed
		}
	}
	if t := c.Query("to"); t != "" {
		if parsed, err := time.Parse(time.RFC3339, t); err == nil {
			to = parsed
		}
	}
	logs, err := h.svc.ListAuditLogsByTimeRange(c.Request.Context(), from, to)
	if err != nil {
		response.InternalError(c, "EXPORT_AUDIT_FAILED", err.Error())
		return
	}
	c.Header("Content-Disposition", "attachment; filename=node-audit-logs.csv")
	c.Header("Content-Type", "text/csv")
	c.String(http.StatusOK, "id,server_id,admin_id,action,detail,source_ip,success,error_message,created_at\n")
	for _, l := range logs {
		c.String(http.StatusOK, "%d,%d,%d,%s,%s,%s,%t,%s,%s\n",
			l.ID, l.ServerID, l.AdminID, l.Action, l.Detail, l.SourceIP, l.Success, l.ErrorMessage, l.CreatedAt.Format(time.RFC3339))
	}
}

// ──────────────────── Supplementary: Node Tags ────────────────────

// ListServerTags returns all tags for a server.
func (h *Handler) ListServerTags(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	result, err := h.svc.ListServerTags(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "LIST_TAGS_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"tags": result})
}

// AddServerTag adds or updates a tag on a server.
func (h *Handler) AddServerTag(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	var req struct {
		TagKey   string `json:"tag_key" binding:"required"`
		TagValue string `json:"tag_value"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_BODY", err.Error())
		return
	}
	if err := h.svc.AddServerTag(c.Request.Context(), id, req.TagKey, req.TagValue); err != nil {
		response.InternalError(c, "ADD_TAG_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"added": true, "server_id": id, "tag_key": req.TagKey})
}

// RemoveServerTag removes a tag from a server.
func (h *Handler) RemoveServerTag(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	tagKey := c.Param("tagKey")
	if tagKey == "" {
		response.BadRequest(c, "INVALID_TAG_KEY", "tag key is required")
		return
	}
	if err := h.svc.RemoveServerTag(c.Request.Context(), id, tagKey); err != nil {
		response.InternalError(c, "REMOVE_TAG_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"removed": true, "server_id": id, "tag_key": tagKey})
}

// ──────────────────── Supplementary: Agent Degradation & Reconnect ────────────────────

// GetDegradationStatus returns the Agent degradation status.
func (h *Handler) GetDegradationStatus(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	result, err := h.svc.GetDegradationStatus(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "DEGRADATION_STATUS_FAILED", err.Error())
		return
	}
	response.OK(c, result)
}

// ForceReconnect forces an Agent to reconnect.
func (h *Handler) ForceReconnect(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	if err := h.svc.ForceReconnect(c.Request.Context(), id); err != nil {
		response.InternalError(c, "FORCE_RECONNECT_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"disconnected": true, "server_id": id})
}
