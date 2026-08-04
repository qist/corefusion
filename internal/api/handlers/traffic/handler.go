package traffic

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/store/repository"
)

// Handler surfaces traffic quota *monitoring* data sourced from the real,
// engine-backed tables (node_servers + node_traffic_usage) instead of the
// design-only observation tables. Quota enforcement lives in the subscription /
// plan engine; this surface only observes effective quotas and usage.
type Handler struct {
	nodeServerRepo *repository.NodeServerRepository
	repo           *repository.TrafficQuotaRepository
}

// NewHandler creates a new traffic Handler.
func NewHandler(
	nodeServerRepo *repository.NodeServerRepository,
	repo *repository.TrafficQuotaRepository,
) *Handler {
	return &Handler{
		nodeServerRepo: nodeServerRepo,
		repo:           repo,
	}
}

// ListQuotas handles GET /api/v1/traffic/quotas.
// Reads the effective per-node traffic quota from node_servers (the table the
// quota engine actually writes to). Nodes without a quota are reported as
// unlimited.
func (h *Handler) ListQuotas(c *gin.Context) {
	ctx := c.Request.Context()
	servers, err := h.nodeServerRepo.ListAll(ctx)
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	items := make([]gin.H, 0, len(servers))
	for i := range servers {
		s := &servers[i]
		hasQuota := s.EgressInterface != "" && s.TrafficQuotaBytes > 0
		items = append(items, gin.H{
			"server_id":     s.ID,
			"name":          s.Name,
			"region":        s.Region,
			"egress_iface":  s.EgressInterface,
			"has_quota":     hasQuota,
			"quota_bytes":   s.TrafficQuotaBytes,
			"reset_day":     s.TrafficQuotaResetDay,
			"used_bytes":    s.TrafficUsedBytes,
			"period_start":  s.TrafficPeriodStart,
			"suspended":     s.TrafficSuspended,
		})
	}
	response.OK(c, items)
}

// GetQuotaUsage handles GET /api/v1/traffic/quotas/:id/usage.
// Returns the current billing period's traffic usage for a node.
func (h *Handler) GetQuotaUsage(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", err.Error())
		return
	}
	usage, err := h.repo.GetCurrentTrafficUsage(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "USAGE_FAILED", err.Error())
		return
	}
	if usage == nil {
		// No usage record for the current period yet.
		response.OK(c, gin.H{
			"server_id":   id,
			"total_bytes": int64(0),
			"quota_bytes": int64(0),
			"remaining":   int64(0),
			"exceeded":    false,
			"empty":       true,
		})
		return
	}
	used := usage.TotalBytes
	quota := usage.QuotaBytes
	response.OK(c, gin.H{
		"server_id":   usage.ServerID,
		"period_start": usage.PeriodStart,
		"period_end":   usage.PeriodEnd,
		"rx_bytes":     usage.RxBytes,
		"tx_bytes":     usage.TxBytes,
		"total_bytes":  used,
		"quota_bytes":  quota,
		"remaining":    quota - used,
		"exceeded":     quota > 0 && used > quota,
		"suspended":    usage.Suspended,
		"empty":        false,
	})
}

// GetUsageHistory handles GET /api/v1/traffic/usage-history.
// Aggregates historical per-node monthly traffic from node_traffic_usage (the
// table the quota engine writes to) across all nodes.
func (h *Handler) GetUsageHistory(c *gin.Context) {
	ctx := c.Request.Context()
	months := 6
	if m, err := strconv.Atoi(c.Query("months")); err == nil && m > 0 {
		months = m
	}
	servers, err := h.nodeServerRepo.ListAll(ctx)
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	ids := make([]int64, 0, len(servers))
	for i := range servers {
		ids = append(ids, servers[i].ID)
	}
	usage, err := h.repo.ListTrafficUsage(ctx, ids, months)
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	response.OK(c, usage)
}

// ListResetLogs handles GET /api/v1/traffic/reset-logs.
func (h *Handler) ListResetLogs(c *gin.Context) {
	p := response.ParsePagination(c)
	subjectType := c.Query("subject_type")
	items, total, err := h.repo.ListResetLogs(c.Request.Context(), subjectType, p.Limit, p.Offset)
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	response.Paginated(c, items, total, p.Limit, p.Offset)
}
