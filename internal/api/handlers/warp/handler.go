package warp

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/domain"
	"corefusion/internal/service/warp"
)

// ConfigSyncHook triggers config sync to a specific server after data changes.
// Implemented by configSyncHookAdapter in router.go.
type ConfigSyncHook interface {
	SyncServerConfig(ctx context.Context, serverID int64, coreType string)
}

// HotApplySyncHook 是 ConfigSyncHook 的可选扩展：声明改动已通过核心 API 热下发。
type HotApplySyncHook interface {
	SyncServerConfigHotApplied(ctx context.Context, serverID int64, coreType string)
}

// XrayOutboundApplier 把出站热下发到运行中的 xray（本地直连 gRPC / 远端经 Agent）。
// Implemented by xrayConfigApplier in router.go（内部复用 coredata.ControlProvider）。
type XrayOutboundApplier interface {
	AddOutbound(ctx context.Context, serverID int64, configJSON []byte) error
}

// Handler handles WARP HTTP requests.
// All endpoints are scoped to a specific node server via the :id path parameter.
type Handler struct {
	svc      *warp.Service
	syncHook ConfigSyncHook
	applier  XrayOutboundApplier
}

// NewHandler creates a new WARP handler.
func NewHandler(svc *warp.Service) *Handler {
	return &Handler{svc: svc}
}

// SetSyncHook 注入配置同步钩子：一键创建 MASQUE 出站后需要把新出站推到运行中的核心。
func (h *Handler) SetSyncHook(hook ConfigSyncHook) {
	h.syncHook = hook
}

// SetOutboundApplier 注入「新出站进程内热下发」能力（MASQUE 出站创建后立即生效）。
func (h *Handler) SetOutboundApplier(applier XrayOutboundApplier) {
	h.applier = applier
}

// triggerConfigSync 触发该节点的配置同步（xray 支持热下发时只写盘不重启）。
func (h *Handler) triggerConfigSync(serverID int64, coreType string) {
	if h.syncHook == nil {
		return
	}
	if domain.CoreType(coreType).SupportsHotApply() {
		if hot, ok := h.syncHook.(HotApplySyncHook); ok {
			go hot.SyncServerConfigHotApplied(context.Background(), serverID, coreType)
			return
		}
	}
	go h.syncHook.SyncServerConfig(context.Background(), serverID, coreType)
}

// List handles GET /api/v1/node-servers/:id/warp
// Returns all WARP configs for the specified node server.
func (h *Handler) List(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	items, err := h.svc.ListByServer(serverID)
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"items": items})
}

// Status handles GET /api/v1/node-servers/:id/warp/status?core_type=xray
// Returns the active WARP registration status for the specified node server.
// If core_type is provided, returns the config matching that core type;
// otherwise returns the latest active config for the server.
func (h *Handler) Status(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	coreType := c.Query("core_type")
	status, err := h.svc.GetStatus(serverID, coreType)
	if err != nil {
		response.InternalError(c, "STATUS_FAILED", err.Error())
		return
	}
	response.OK(c, status)
}

// Register handles POST /api/v1/node-servers/:id/warp/register
// Registers a new WARP config for the specified node server.
// Body: { "core_type": "xray", "remark": "" }
func (h *Handler) Register(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	var req struct {
		CoreType string `json:"core_type"`
		Remark   string `json:"remark"`
	}
	// Allow empty body — core_type defaults to ""
	c.ShouldBindJSON(&req)
	status, err := h.svc.Register(serverID, req.CoreType)
	if err != nil {
		response.InternalError(c, "REGISTER_FAILED", err.Error())
		return
	}
	response.OK(c, status)
}

// GenerateConfig handles POST /api/v1/node-servers/:id/warp/generate-config
// Generates a WireGuard config file from the stored WARP credentials.
// Body: { "core_type": "xray" }
func (h *Handler) GenerateConfig(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	var req struct {
		CoreType string `json:"core_type"`
	}
	c.ShouldBindJSON(&req)
	config, err := h.svc.GenerateConfig(serverID, req.CoreType)
	if err != nil {
		response.InternalError(c, "GENERATE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"config": config})
}

// GetConfig handles GET /api/v1/node-servers/:id/warp/config?core_type=xray
// Returns the stored WireGuard config for the specified node server.
func (h *Handler) GetConfig(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	coreType := c.Query("core_type")
	config, _ := h.svc.GetConfig(serverID, coreType)
	response.OK(c, gin.H{"config": config})
}

// Delete handles DELETE /api/v1/node-servers/:id/warp/:warpId?remote=true
// Deletes a specific WARP config by its ID. remote=true 时同时注销 Cloudflare 设备。
func (h *Handler) Delete(c *gin.Context) {
	warpID, err := strconv.ParseInt(c.Param("warpId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid warp config ID")
		return
	}
	remote := false
	switch strings.ToLower(strings.TrimSpace(c.Query("remote"))) {
	case "1", "true", "yes":
		remote = true
	}
	res, err := h.svc.Delete(warpID, remote)
	if err != nil {
		response.InternalError(c, "RESET_FAILED", err.Error())
		return
	}
	if res == nil {
		res = &warp.DeleteResult{}
	}
	response.OK(c, gin.H{
		"message":          "deleted",
		"remote_attempted": res.RemoteAttempted,
		"remote_deleted":   res.RemoteDeleted,
		"remote_error":     res.RemoteError,
	})
}

// RegisterMasque handles POST /api/v1/node-servers/:id/warp/masque/register
// 走 WARP 的两步注册（先建 WireGuard 设备、再登记 P-256 公钥切 masque）。
// 与旧的 /warp/register 并存，旧记录不受影响。
// Body: { "core_type": "xray", "remark": "" }
func (h *Handler) RegisterMasque(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	var req struct {
		CoreType string `json:"core_type"`
		Remark   string `json:"remark"`
	}
	c.ShouldBindJSON(&req)
	status, err := h.svc.RegisterMasque(serverID, req.CoreType, req.Remark)
	if err != nil {
		response.InternalError(c, "REGISTER_FAILED", err.Error())
		return
	}
	response.OK(c, status)
}

// CreateMasqueOutbound handles POST /api/v1/node-servers/:id/warp/:warpId/masque-outbound
// 用已注册的 MASQUE 设备直接创建一条 xray 出站，返回 tag 供路由引用。
// Body: { "remark": "", "tag": "", "alpn": ["h3"], "server_name": "", "port": 0 }
func (h *Handler) CreateMasqueOutbound(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	warpID, err := strconv.ParseInt(c.Param("warpId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid warp ID")
		return
	}
	var in warp.MasqueOutboundInput
	if err := c.ShouldBindJSON(&in); err != nil && err.Error() != "EOF" {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	result, err := h.svc.CreateMasqueOutbound(serverID, warpID, in)
	if err != nil {
		response.InternalError(c, "CREATE_OUTBOUND_FAILED", err.Error())
		return
	}
	// 新出站必须下发到运行中的核心，否则路由选不到它（xray 热下发，不重启）。
	// 服务端自己下发，不再依赖前端保存后再调 /xray/control/outbound。
	if h.applier != nil {
		if raw, merr := json.Marshal(result.Config); merr == nil {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
			if aerr := h.applier.AddOutbound(ctx, serverID, raw); aerr != nil {
				log.Printf("warp: hot add masque outbound %q (server=%d) failed, core NOT restarted: %v", result.Tag, serverID, aerr)
			}
			cancel()
		}
	}
	h.triggerConfigSync(serverID, "xray")
	response.OK(c, result)
}
