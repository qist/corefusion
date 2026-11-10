package xraycontrol

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"

	proxymanCommand "github.com/xtls/xray-core/app/proxyman/command"
	routerCommand "github.com/xtls/xray-core/app/router/command"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/infra/conf"

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
// Implemented by NodeServerManager.XrayAPIOnServer.
type RemoteXrayAPI interface {
	XrayAPIOnServer(ctx context.Context, serverID int64, req *agentpb.XrayAPIRequest) (*agentpb.XrayAPIResponse, error)
}

// ProtocolResolver 按「节点 + tag」查入站协议。
// 入站 tag 常常只有端口（inbound-59026），调用方不传 protocol 时必须靠数据库解析，
// 否则无法构造正确的 xray 账号类型（会把 vless 当成 trojan）。
type ProtocolResolver func(ctx context.Context, serverID int64, tag string) string

// Handler handles xray control HTTP requests.
type Handler struct {
	settingSvc       SettingStore
	coreConfigSvc    CoreConfigService
	nodeOperator     RemoteXrayAPI
	control          *coredata.ControlProvider
	protocolResolver ProtocolResolver
}

// NewHandler creates a new xray control handler.
func NewHandler(settingSvc SettingStore) *Handler {
	h := &Handler{settingSvc: settingSvc}
	h.rebuildControlProvider()
	return h
}

// SetProtocolResolver 注入入站协议解析器（按 serverID + tag 查库）。
func (h *Handler) SetProtocolResolver(r ProtocolResolver) {
	h.protocolResolver = r
}

// resolveProtocol 在调用方未显式传协议时，按节点 + tag 从数据库补全。
func (h *Handler) resolveProtocol(ctx context.Context, serverID int64, tag, explicit string) string {
	if explicit != "" {
		return explicit
	}
	if h.protocolResolver == nil {
		return ""
	}
	return h.protocolResolver(ctx, serverID, tag)
}

// SetCoreConfigService sets the core config service.
func (h *Handler) SetCoreConfigService(svc CoreConfigService) {
	h.coreConfigSvc = svc
	h.rebuildControlProvider()
}

// SetNodeOperator sets the node operator for remote xray API proxying.
func (h *Handler) SetNodeOperator(op RemoteXrayAPI) {
	h.nodeOperator = op
	h.rebuildControlProvider()
}

// rebuildControlProvider 重建写操作公共层，将建连/远程转发下沉到 coredata.ControlProvider。
func (h *Handler) rebuildControlProvider() {
	h.control = coredata.NewControlProvider(coredata.ControlOptions{
		IsLocal: func(serverID int64) bool { return !isRemote(serverID) },
		AddrResolver: func(ctx context.Context, serverID int64) (string, error) {
			addr := h.getStatsAddr(ctx, serverID)
			if addr == "" {
				return "", fmt.Errorf("xray API not available for server %d", serverID)
			}
			return addr, nil
		},
		Remote: h.nodeOperator,
	})
}

// Control 返回共享控制层（本地直连 xray gRPC / 远端经 Agent 转发）。
//
// 供进程内调用方复用（例如 InboundManager 热加/热删用户），避免再走
// 「面板自身 HTTP API 回环」——面板开 TLS 时那条路会被 400 拒绝。
func (h *Handler) Control() *coredata.ControlProvider {
	return h.control
}

// localServerID is the server ID of the local node.
const localServerID = coredata.LocalServerID

// parseServerID extracts the :id parameter from the URL.
// Returns localServerID (1) if the parameter is missing or invalid.
func parseServerID(c *gin.Context) int64 {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return localServerID
	}
	return id
}

// isRemote returns true if the server ID refers to a remote node.
func isRemote(serverID int64) bool {
	return !coredata.IsLocalNode(serverID)
}

// remoteXrayAPI calls the remote Agent's XrayAPI RPC.
// Returns (responseJSON, handled, errorOccurred).
//   - (resp, true, false):  remote call succeeded, resp contains JSON
//   - (nil, true, true):    remote call failed, error already written to response
//   - (nil, false, false):  server is local, caller should use direct connection
func (h *Handler) remoteXrayAPI(c *gin.Context, service, method string, payload any) (json.RawMessage, bool, bool) {
	serverID := parseServerID(c)
	if !isRemote(serverID) {
		return nil, false, false // local — caller continues with direct gRPC
	}
	if h.control == nil {
		response.InternalError(c, "NOT_CONFIGURED", "control provider not configured for remote servers")
		return nil, true, true
	}

	resp, err := h.control.RemoteCall(c.Request.Context(), serverID, service, method, payload)
	if err != nil {
		response.InternalError(c, "REMOTE_API_FAILED", err.Error())
		return nil, true, true
	}
	return resp, true, false
}

// getStatsAddr returns the xray stats API address for a given server.
// For local nodes: Priority 1. DB setting, 2. Config file api.listen, 3. Default 127.0.0.1:10085
// For remote nodes: not supported (returns empty string; caller should use remoteXrayAPI).
func (h *Handler) getStatsAddr(ctx context.Context, serverID int64) string {
	if serverID != localServerID {
		return "" // Remote nodes: xray API not directly accessible from panel
	}
	deps := coredata.StatsAddrDeps{
		SettingGetter: func(c context.Context, key string) (string, error) {
			return h.settingSvc.GetByKey(c, key)
		},
	}
	if h.coreConfigSvc != nil {
		deps.APIListenGetter = func(c context.Context) (string, error) {
			fullConfig, err := h.coreConfigSvc.GetFull(c, serverID, "xray")
			if err != nil {
				return "", err
			}
			apiRaw, ok := fullConfig["api"]
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
	return coredata.ResolveStatsAddr(ctx, deps)
}

// connect creates a fresh gRPC connection to xray for a given server.
// Xray may restart independently from the panel process, so reusing a cached
// connection can leave control APIs talking to a stale transport. 建连逻辑
// 委托 coredata.ControlProvider.LocalConn（每次新建，不缓存）。
func (h *Handler) connect(ctx context.Context, serverID int64) (*grpc.ClientConn, error) {
	if h.control == nil {
		return nil, fmt.Errorf("control provider not configured")
	}
	return h.control.LocalConn(ctx, serverID)
}

// ==================== Inbound Management ====================

// AddInbound handles POST /api/v1/xray/control/inbound.
func (h *Handler) AddInbound(c *gin.Context) {
	sid := parseServerID(c)
	var req struct {
		Inbound json.RawMessage `json:"inbound" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_PARAMS", err.Error())
		return
	}

	// Remote: proxy via Agent
	if resp, handled, errOcc := h.remoteXrayAPI(c, "proxyman", "addInbound", map[string]interface{}{"config": req.Inbound}); handled {
		if errOcc {
			audit.RecordNodeFromCtx(c, sid, "xray_add_inbound", "remote", false, "remote API failed")
		} else {
			audit.RecordNodeFromCtx(c, sid, "xray_add_inbound", "remote", true, "")
		}
		if resp != nil {
			var result interface{}
			json.Unmarshal(resp, &result)
			response.OK(c, result)
		}
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	conn, err := h.connect(ctx, sid)
	if err != nil {
		audit.RecordNodeFromCtx(c, sid, "xray_add_inbound", "", false, err.Error())
		response.InternalError(c, "CONNECT_FAILED", err.Error())
		return
	}
	defer conn.Close()

	client := proxymanCommand.NewHandlerServiceClient(conn)

	inboundConf := new(conf.InboundDetourConfig)
	if err := json.Unmarshal(req.Inbound, inboundConf); err != nil {
		response.BadRequest(c, "INVALID_CONFIG", "failed to parse inbound config: "+err.Error())
		return
	}

	config, err := inboundConf.Build()
	if err != nil {
		response.BadRequest(c, "BUILD_FAILED", "failed to build inbound config: "+err.Error())
		return
	}

	_, err = client.AddInbound(ctx, &proxymanCommand.AddInboundRequest{Inbound: config})
	if err != nil {
		audit.RecordNodeFromCtx(c, sid, "xray_add_inbound", "", false, err.Error())
		response.InternalError(c, "ADD_INBOUND_FAILED", err.Error())
		return
	}

	audit.RecordNodeFromCtx(c, sid, "xray_add_inbound", "", true, "")
	response.OK(c, gin.H{"message": "inbound added successfully"})
}

// RemoveInbound handles DELETE /api/v1/xray/control/inbound/:tag.
func (h *Handler) RemoveInbound(c *gin.Context) {
	sid := parseServerID(c)
	tag := c.Param("tag")
	if tag == "" {
		response.BadRequest(c, "INVALID_PARAMS", "tag is required")
		return
	}

	// Remote: proxy via Agent
	if resp, handled, errOcc := h.remoteXrayAPI(c, "proxyman", "removeInbound", map[string]interface{}{"tag": tag}); handled {
		if errOcc {
			audit.RecordNodeFromCtx(c, sid, "xray_remove_inbound", tag, false, "remote API failed")
		} else {
			audit.RecordNodeFromCtx(c, sid, "xray_remove_inbound", tag, true, "")
		}
		if resp != nil {
			var result interface{}
			json.Unmarshal(resp, &result)
			response.OK(c, result)
		}
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	conn, err := h.connect(ctx, sid)
	if err != nil {
		audit.RecordNodeFromCtx(c, sid, "xray_remove_inbound", tag, false, err.Error())
		response.InternalError(c, "CONNECT_FAILED", err.Error())
		return
	}
	defer conn.Close()

	client := proxymanCommand.NewHandlerServiceClient(conn)

	_, err = client.RemoveInbound(ctx, &proxymanCommand.RemoveInboundRequest{Tag: tag})
	if err != nil {
		audit.RecordNodeFromCtx(c, sid, "xray_remove_inbound", tag, false, err.Error())
		response.InternalError(c, "REMOVE_INBOUND_FAILED", err.Error())
		return
	}

	audit.RecordNodeFromCtx(c, sid, "xray_remove_inbound", tag, true, "")
	response.OK(c, gin.H{"message": "inbound removed successfully", "tag": tag})
}

// ==================== Outbound Management ====================

// AddOutbound handles POST /api/v1/xray/control/outbound.
//
// 本地/远端统一走 ControlProvider（进程内直连 gRPC 或经 Agent 转发），
// 与 InboundManager 热加用户共用同一套下发实现。
func (h *Handler) AddOutbound(c *gin.Context) {
	sid := parseServerID(c)
	var req struct {
		Outbound json.RawMessage `json:"outbound" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_PARAMS", err.Error())
		return
	}

	if h.control == nil {
		response.InternalError(c, "NOT_CONFIGURED", "control provider not configured")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if err := h.control.AddOutbound(ctx, sid, req.Outbound); err != nil {
		audit.RecordNodeFromCtx(c, sid, "xray_add_outbound", "", false, err.Error())
		response.InternalError(c, "ADD_OUTBOUND_FAILED", err.Error())
		return
	}

	audit.RecordNodeFromCtx(c, sid, "xray_add_outbound", "", true, "")
	response.OK(c, gin.H{"message": "outbound added successfully"})
}

// RemoveOutbound handles DELETE /api/v1/xray/control/outbound/:tag.
func (h *Handler) RemoveOutbound(c *gin.Context) {
	sid := parseServerID(c)
	tag := c.Param("tag")
	if tag == "" {
		response.BadRequest(c, "INVALID_PARAMS", "tag is required")
		return
	}

	if h.control == nil {
		response.InternalError(c, "NOT_CONFIGURED", "control provider not configured")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if err := h.control.RemoveOutbound(ctx, sid, tag); err != nil {
		audit.RecordNodeFromCtx(c, sid, "xray_remove_outbound", tag, false, err.Error())
		response.InternalError(c, "REMOVE_OUTBOUND_FAILED", err.Error())
		return
	}

	audit.RecordNodeFromCtx(c, sid, "xray_remove_outbound", tag, true, "")
	response.OK(c, gin.H{"message": "outbound removed successfully", "tag": tag})
}

// ==================== User Management ====================

// AddUser handles POST /api/v1/xray/control/inbound/:tag/user.
// Supported protocols: VLESS, VMess, Trojan, Shadowsocks, Hysteria.
//
// protocol 可选：不传时按 tag 推断。新入站的 tag 往往只有端口（inbound-59026），
// 推断不出协议，内部调用方（InboundManager）必须显式传 protocol。
func (h *Handler) AddUser(c *gin.Context) {
	sid := parseServerID(c)
	tag := c.Param("tag")
	if tag == "" {
		response.BadRequest(c, "INVALID_PARAMS", "tag is required")
		return
	}

	var req struct {
		Email    string `json:"email" binding:"required"`
		ID       string `json:"id"`
		Password string `json:"password"`
		Flow     string `json:"flow"`
		Method   string `json:"method"`   // Shadowsocks cipher method
		Protocol string `json:"protocol"` // 入站协议（tag 不含协议名时必须传）
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_PARAMS", err.Error())
		return
	}

	if h.control == nil {
		response.InternalError(c, "NOT_CONFIGURED", "control provider not configured")
		return
	}

	detail := fmt.Sprintf("tag=%s email=%s", tag, req.Email)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	protocol := h.resolveProtocol(ctx, sid, tag, req.Protocol)
	if err := h.control.AddUser(ctx, sid, tag, protocol, req.Email, req.ID, req.Password, req.Flow, req.Method); err != nil {
		audit.RecordNodeFromCtx(c, sid, "xray_add_user", detail, false, err.Error())
		response.InternalError(c, "ADD_USER_FAILED", err.Error())
		return
	}

	audit.RecordNodeFromCtx(c, sid, "xray_add_user", detail, true, "")
	response.OK(c, gin.H{"message": "user added successfully", "email": req.Email, "tag": tag})
}

// RemoveUser handles DELETE /api/v1/xray/control/inbound/:tag/user/:email.
func (h *Handler) RemoveUser(c *gin.Context) {
	sid := parseServerID(c)
	tag := c.Param("tag")
	email := c.Param("email")
	if tag == "" || email == "" {
		response.BadRequest(c, "INVALID_PARAMS", "tag and email are required")
		return
	}

	if h.control == nil {
		response.InternalError(c, "NOT_CONFIGURED", "control provider not configured")
		return
	}

	detail := fmt.Sprintf("tag=%s email=%s", tag, email)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if err := h.control.RemoveUser(ctx, sid, tag, email); err != nil {
		audit.RecordNodeFromCtx(c, sid, "xray_remove_user", detail, false, err.Error())
		response.InternalError(c, "REMOVE_USER_FAILED", err.Error())
		return
	}

	audit.RecordNodeFromCtx(c, sid, "xray_remove_user", detail, true, "")
	response.OK(c, gin.H{"message": "user removed from xray", "email": email, "tag": tag})
}

// ==================== Routing Management ====================

// ApplyRouting handles POST /api/v1/xray/control/routing.
//
// 全量替换运行中 xray 的路由规则（本地直连 / 远端经 Agent），
// 与「服务端改路由后自动下发」共用 ControlProvider.ApplyRouting。
func (h *Handler) ApplyRouting(c *gin.Context) {
	sid := parseServerID(c)
	var req struct {
		Routing json.RawMessage `json:"routing" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_PARAMS", err.Error())
		return
	}

	if h.control == nil {
		response.InternalError(c, "NOT_CONFIGURED", "control provider not configured")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	if err := h.control.ApplyRouting(ctx, sid, req.Routing); err != nil {
		audit.RecordNodeFromCtx(c, sid, "xray_apply_routing", "", false, err.Error())
		response.InternalError(c, "APPLY_ROUTING_FAILED", err.Error())
		return
	}

	audit.RecordNodeFromCtx(c, sid, "xray_apply_routing", "", true, "")
	response.OK(c, gin.H{"message": "routing applied successfully"})
}

// AddRoutingRule handles POST /api/v1/xray/control/routing/rule.
func (h *Handler) AddRoutingRule(c *gin.Context) {
	sid := parseServerID(c)
	var req struct {
		Rule   json.RawMessage `json:"rule" binding:"required"`
		Append bool            `json:"append"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_PARAMS", err.Error())
		return
	}

	// Remote: proxy via Agent
	if resp, handled, errOcc := h.remoteXrayAPI(c, "router", "addRoutingRule", map[string]interface{}{"rule": req.Rule, "append": req.Append}); handled {
		if errOcc {
			audit.RecordNodeFromCtx(c, sid, "xray_add_routing_rule", "", false, "remote API failed")
		} else {
			audit.RecordNodeFromCtx(c, sid, "xray_add_routing_rule", "", true, "")
		}
		if resp != nil {
			var result interface{}
			json.Unmarshal(resp, &result)
			response.OK(c, result)
		}
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	conn, err := h.connect(ctx, sid)
	if err != nil {
		audit.RecordNodeFromCtx(c, sid, "xray_add_routing_rule", "", false, err.Error())
		response.InternalError(c, "CONNECT_FAILED", err.Error())
		return
	}
	defer conn.Close()

	client := routerCommand.NewRoutingServiceClient(conn)

	routerConf := &conf.RouterConfig{
		RuleList: []json.RawMessage{req.Rule},
	}

	config, err := routerConf.Build()
	if err != nil {
		response.BadRequest(c, "BUILD_FAILED", "failed to build rule config: "+err.Error())
		return
	}

	_, err = client.AddRule(ctx, &routerCommand.AddRuleRequest{
		ShouldAppend: req.Append,
		Config:       serial.ToTypedMessage(config),
	})
	if err != nil {
		audit.RecordNodeFromCtx(c, sid, "xray_add_routing_rule", "", false, err.Error())
		response.InternalError(c, "ADD_RULE_FAILED", err.Error())
		return
	}

	audit.RecordNodeFromCtx(c, sid, "xray_add_routing_rule", "", true, "")
	response.OK(c, gin.H{"message": "routing rule added successfully"})
}

// RemoveRoutingRule handles DELETE /api/v1/xray/control/routing/rule/:tag.
func (h *Handler) RemoveRoutingRule(c *gin.Context) {
	sid := parseServerID(c)
	tag := c.Param("tag")
	if tag == "" {
		response.BadRequest(c, "INVALID_PARAMS", "tag is required")
		return
	}

	// Remote: proxy via Agent
	if resp, handled, errOcc := h.remoteXrayAPI(c, "router", "removeRoutingRule", map[string]interface{}{"tag": tag}); handled {
		if errOcc {
			audit.RecordNodeFromCtx(c, sid, "xray_remove_routing_rule", tag, false, "remote API failed")
		} else {
			audit.RecordNodeFromCtx(c, sid, "xray_remove_routing_rule", tag, true, "")
		}
		if resp != nil {
			var result interface{}
			json.Unmarshal(resp, &result)
			response.OK(c, result)
		}
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	conn, err := h.connect(ctx, sid)
	if err != nil {
		audit.RecordNodeFromCtx(c, sid, "xray_remove_routing_rule", tag, false, err.Error())
		response.InternalError(c, "CONNECT_FAILED", err.Error())
		return
	}
	defer conn.Close()

	client := routerCommand.NewRoutingServiceClient(conn)

	_, err = client.RemoveRule(ctx, &routerCommand.RemoveRuleRequest{RuleTag: tag})
	if err != nil {
		audit.RecordNodeFromCtx(c, sid, "xray_remove_routing_rule", tag, false, err.Error())
		response.InternalError(c, "REMOVE_RULE_FAILED", err.Error())
		return
	}

	audit.RecordNodeFromCtx(c, sid, "xray_remove_routing_rule", tag, true, "")
	response.OK(c, gin.H{"message": "routing rule removed successfully", "tag": tag})
}

// ListRoutingRules handles GET /api/v1/xray/control/routing/rules.
func (h *Handler) ListRoutingRules(c *gin.Context) {
	// Remote: proxy via Agent
	if resp, handled, _ := h.remoteXrayAPI(c, "router", "listRoutingRules", map[string]interface{}{}); handled {
		if resp != nil {
			var result interface{}
			json.Unmarshal(resp, &result)
			response.OK(c, result)
		}
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	conn, err := h.connect(ctx, parseServerID(c))
	if err != nil {
		response.InternalError(c, "CONNECT_FAILED", err.Error())
		return
	}
	defer conn.Close()

	client := routerCommand.NewRoutingServiceClient(conn)

	resp, err := client.ListRule(ctx, &routerCommand.ListRuleRequest{})
	if err != nil {
		response.InternalError(c, "LIST_RULES_FAILED", err.Error())
		return
	}

	rules := make([]map[string]interface{}, 0)
	for _, rule := range resp.GetRules() {
		rules = append(rules, map[string]interface{}{
			"tag":     rule.GetTag(),
			"ruleTag": rule.GetRuleTag(),
		})
	}

	response.OK(c, rules)
}

// ==================== Balancer Management ====================

// GetBalancerInfo handles GET /api/v1/xray/control/balancer/:tag.
func (h *Handler) GetBalancerInfo(c *gin.Context) {
	tag := c.Param("tag")
	if tag == "" {
		response.BadRequest(c, "INVALID_PARAMS", "tag is required")
		return
	}

	// Remote: proxy via Agent
	if resp, handled, _ := h.remoteXrayAPI(c, "router", "getBalancerInfo", map[string]interface{}{"tag": tag}); handled {
		if resp != nil {
			var result interface{}
			json.Unmarshal(resp, &result)
			response.OK(c, result)
		}
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	conn, err := h.connect(ctx, parseServerID(c))
	if err != nil {
		response.InternalError(c, "CONNECT_FAILED", err.Error())
		return
	}
	defer conn.Close()

	client := routerCommand.NewRoutingServiceClient(conn)

	resp, err := client.GetBalancerInfo(ctx, &routerCommand.GetBalancerInfoRequest{Tag: tag})
	if err != nil {
		response.InternalError(c, "GET_BALANCER_FAILED", err.Error())
		return
	}

	info := map[string]interface{}{"tag": tag}
	if balancer := resp.GetBalancer(); balancer != nil {
		if balancer.Override != nil {
			info["override"] = balancer.Override.Target
		}
		if balancer.PrincipleTarget != nil {
			info["selected"] = balancer.PrincipleTarget.Tag
		}
	}

	response.OK(c, info)
}

// SetBalancerTarget handles POST /api/v1/xray/control/balancer/:tag.
func (h *Handler) SetBalancerTarget(c *gin.Context) {
	sid := parseServerID(c)
	tag := c.Param("tag")
	if tag == "" {
		response.BadRequest(c, "INVALID_PARAMS", "tag is required")
		return
	}

	var req struct {
		Target string `json:"target"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_PARAMS", err.Error())
		return
	}

	// Remote: proxy via Agent
	if resp, handled, errOcc := h.remoteXrayAPI(c, "router", "setBalancerTarget", map[string]interface{}{"tag": tag, "target": req.Target}); handled {
		detail := fmt.Sprintf("tag=%s target=%s", tag, req.Target)
		if errOcc {
			audit.RecordNodeFromCtx(c, sid, "xray_set_balancer", detail, false, "remote API failed")
		} else {
			audit.RecordNodeFromCtx(c, sid, "xray_set_balancer", detail, true, "")
		}
		if resp != nil {
			var result interface{}
			json.Unmarshal(resp, &result)
			response.OK(c, result)
		}
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	conn, err := h.connect(ctx, sid)
	if err != nil {
		audit.RecordNodeFromCtx(c, sid, "xray_set_balancer", fmt.Sprintf("tag=%s target=%s", tag, req.Target), false, err.Error())
		response.InternalError(c, "CONNECT_FAILED", err.Error())
		return
	}
	defer conn.Close()

	client := routerCommand.NewRoutingServiceClient(conn)

	_, err = client.OverrideBalancerTarget(ctx, &routerCommand.OverrideBalancerTargetRequest{
		BalancerTag: tag,
		Target:      req.Target,
	})
	if err != nil {
		audit.RecordNodeFromCtx(c, sid, "xray_set_balancer", fmt.Sprintf("tag=%s target=%s", tag, req.Target), false, err.Error())
		response.InternalError(c, "SET_BALANCER_FAILED", err.Error())
		return
	}

	audit.RecordNodeFromCtx(c, sid, "xray_set_balancer", fmt.Sprintf("tag=%s target=%s", tag, req.Target), true, "")
	response.OK(c, gin.H{"message": "balancer target set successfully", "tag": tag, "target": req.Target})
}

// ==================== Helpers ====================
