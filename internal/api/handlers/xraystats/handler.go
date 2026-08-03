package xraystats

import (
	"context"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/coredata"
	agentpb "corefusion/internal/runtime/agent/proto"
	"corefusion/internal/service/audit"
)

// SettingStore provides access to system settings.
type SettingStore interface {
	GetByKey(ctx context.Context, key string) (string, error)
}

// CoreConfigService provides access to core configuration.
type CoreConfigService interface {
	GetFull(ctx context.Context, serverID int64, coreType string) (map[string]interface{}, error)
}

// RemoteXrayAPI provides xray API proxying to remote Agent nodes.
type RemoteXrayAPI interface {
	XrayAPIOnServer(ctx context.Context, serverID int64, req *agentpb.XrayAPIRequest) (*agentpb.XrayAPIResponse, error)
}

// Handler handles xray stats HTTP requests.
// 所有 xray gRPC 访问均通过 coredata.CoreDataProvider 完成，
// 本地直连与远程经 Agent 的路由对 Handler 透明。
type Handler struct {
	settingSvc    SettingStore
	coreConfigSvc CoreConfigService
	nodeOperator  RemoteXrayAPI
	provider      coredata.CoreDataProvider
}

// localServerID is the server ID of the local node.
const localServerID = coredata.LocalServerID

// NewHandler creates a new xray stats handler. Dependencies are injected
// via SetCoreConfigService / SetNodeOperator; the provider is built lazily.
func NewHandler(settingSvc SettingStore) *Handler {
	h := &Handler{settingSvc: settingSvc}
	h.rebuildProvider()
	return h
}

// Close is kept for API compatibility.
func (h *Handler) Close() {}

// SetCoreConfigService sets the core config service (used to resolve api.listen).
func (h *Handler) SetCoreConfigService(svc CoreConfigService) {
	h.coreConfigSvc = svc
	h.rebuildProvider()
}

// SetNodeOperator sets the node operator for remote xray API proxying.
func (h *Handler) SetNodeOperator(op RemoteXrayAPI) {
	h.nodeOperator = op
	h.rebuildProvider()
}

// rebuildProvider 用当前持有的依赖重建 provider。
func (h *Handler) rebuildProvider() {
	if h.settingSvc == nil && h.nodeOperator == nil {
		h.provider = nil
		return
	}
	deps := coredata.StatsAddrDeps{
		SettingGetter: func(ctx context.Context, key string) (string, error) {
			return h.settingSvc.GetByKey(ctx, key)
		},
	}
	if h.coreConfigSvc != nil {
		deps.APIListenGetter = func(ctx context.Context) (string, error) {
			full, err := h.coreConfigSvc.GetFull(ctx, localServerID, "xray")
			if err != nil {
				return "", err
			}
			apiRaw, ok := full["api"]
			if !ok {
				return "", fmt.Errorf("no api section")
			}
			apiMap, ok := apiRaw.(map[string]interface{})
			if !ok {
				return "", fmt.Errorf("api not map")
			}
			listen, ok := apiMap["listen"].(string)
			if !ok || listen == "" {
				return "", fmt.Errorf("no listen")
			}
			return listen, nil
		}
	}
	addr := coredata.ResolveStatsAddr(context.Background(), deps)
	h.provider = coredata.NewProvider(localServerID, coredata.Options{
		IsLocal: func(serverID int64) bool { return serverID == localServerID },
		AddrResolver: func(ctx context.Context, serverID int64) (string, error) {
			return addr, nil
		},
		Remote: h.nodeOperator,
	})
}

// parseServerID extracts the :id parameter from the URL.
func parseServerID(c *gin.Context) int64 {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return localServerID
	}
	return id
}

// GetStats handles GET /api/v1/xray/stats.
func (h *Handler) GetStats(c *gin.Context) {
	serverID := parseServerID(c)
	stats, err := h.provider.GetUserTraffic(c.Request.Context(), serverID)
	if err != nil {
		response.InternalError(c, "STATS_QUERY_FAILED", err.Error())
		return
	}
	inbounds, err := h.provider.GetInboundTraffic(c.Request.Context(), serverID)
	if err != nil {
		response.InternalError(c, "STATS_QUERY_FAILED", err.Error())
		return
	}
	response.OK(c, coredata.StatsResponse{Users: stats, Inbounds: inbounds})
}

// GetUserStats handles GET /api/v1/xray/stats/users.
func (h *Handler) GetUserStats(c *gin.Context) {
	stats, err := h.provider.GetUserTraffic(c.Request.Context(), parseServerID(c))
	if err != nil {
		response.InternalError(c, "STATS_QUERY_FAILED", err.Error())
		return
	}
	response.OK(c, stats)
}

// ResetUserStats handles POST /api/v1/xray/stats/reset/:email.
func (h *Handler) ResetUserStats(c *gin.Context) {
	sid := parseServerID(c)
	email := c.Param("email")
	if email == "" {
		response.BadRequest(c, "INVALID_PARAMS", "email is required")
		return
	}
	if err := h.provider.ResetUserTraffic(c.Request.Context(), sid, email); err != nil {
		audit.RecordNodeFromCtx(c, sid, "xray_reset_stats", email, false, err.Error())
		response.InternalError(c, "STATS_RESET_FAILED", err.Error())
		return
	}
	audit.RecordNodeFromCtx(c, sid, "xray_reset_stats", email, true, "")
	response.OK(c, gin.H{"message": "stats reset successfully", "email": email})
}
