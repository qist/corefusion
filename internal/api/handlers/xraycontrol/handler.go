package xraycontrol

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	proxymanCommand "github.com/xtls/xray-core/app/proxyman/command"
	routerCommand "github.com/xtls/xray-core/app/router/command"
	statsCommand "github.com/xtls/xray-core/app/stats/command"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/infra/conf"
	hyAccount "github.com/xtls/xray-core/proxy/hysteria/account"
	"github.com/xtls/xray-core/proxy/shadowsocks"
	ss2022 "github.com/xtls/xray-core/proxy/shadowsocks_2022"
	"github.com/xtls/xray-core/proxy/trojan"
	"github.com/xtls/xray-core/proxy/vless"
	"github.com/xtls/xray-core/proxy/vmess"

	"corefusion/internal/api/response"
)

// SettingStore provides access to system settings.
type SettingStore interface {
	GetByKey(ctx context.Context, key string) (string, error)
}

// CoreConfigService provides access to core configuration.
type CoreConfigService interface {
	GetFull(ctx context.Context, serverID int64, coreType string) (map[string]interface{}, error)
}

// Handler handles xray control HTTP requests.
type Handler struct {
	settingSvc    SettingStore
	coreConfigSvc CoreConfigService
}

// NewHandler creates a new xray control handler.
func NewHandler(settingSvc SettingStore) *Handler {
	return &Handler{settingSvc: settingSvc}
}

// SetCoreConfigService sets the core config service.
func (h *Handler) SetCoreConfigService(svc CoreConfigService) {
	h.coreConfigSvc = svc
}

// Close is kept for API compatibility.
func (h *Handler) Close() {}

func ensureImplicitAPIRoutingRule(routerConf *conf.RouterConfig) {
	if routerConf == nil {
		return
	}
	for _, rawRule := range routerConf.RuleList {
		var rule struct {
			InboundTag  []string `json:"inboundTag"`
			OutboundTag string   `json:"outboundTag"`
		}
		if err := json.Unmarshal(rawRule, &rule); err != nil {
			continue
		}
		if rule.OutboundTag != "api" {
			continue
		}
		for _, inboundTag := range rule.InboundTag {
			if inboundTag == "api" {
				return
			}
		}
	}

	apiRule := json.RawMessage(`{"type":"field","inboundTag":["api"],"outboundTag":"api"}`)
	routerConf.RuleList = append([]json.RawMessage{apiRule}, routerConf.RuleList...)
}

// getStatsAddr returns the xray stats API address.
// Priority: 1. DB setting, 2. Config file api.listen, 3. Default 127.0.0.1:10085
func (h *Handler) getStatsAddr(ctx context.Context) string {
	// 1. Check DB setting
	if h.settingSvc != nil {
		val, err := h.settingSvc.GetByKey(ctx, "xray.stats_addr")
		if err == nil && val != "" {
			return val
		}
	}

	// 2. Check config file api.listen
	if h.coreConfigSvc != nil {
		fullConfig, err := h.coreConfigSvc.GetFull(ctx, 1, "xray")
		if err == nil {
			if apiRaw, ok := fullConfig["api"]; ok {
				if apiMap, ok := apiRaw.(map[string]interface{}); ok {
					if listen, ok := apiMap["listen"].(string); ok && listen != "" {
						return listen
					}
				}
			}
		}
	}

	// 3. Default (when listen is empty, auto-added dokodemo-door listens on 10085)
	return "127.0.0.1:10085"
}

// connect creates a fresh gRPC connection to xray.
// Xray may restart independently from the panel process, so reusing a cached
// connection can leave control APIs talking to a stale transport.
func (h *Handler) connect(ctx context.Context) (*grpc.ClientConn, error) {
	addr := h.getStatsAddr(ctx)
	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to xray: %w", err)
	}
	return conn, nil
}

// ==================== Inbound Management ====================

// AddInbound handles POST /api/v1/xray/control/inbound.
func (h *Handler) AddInbound(c *gin.Context) {
	var req struct {
		Inbound json.RawMessage `json:"inbound" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_PARAMS", err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	conn, err := h.connect(ctx)
	if err != nil {
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
		response.InternalError(c, "ADD_INBOUND_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"message": "inbound added successfully"})
}

// RemoveInbound handles DELETE /api/v1/xray/control/inbound/:tag.
func (h *Handler) RemoveInbound(c *gin.Context) {
	tag := c.Param("tag")
	if tag == "" {
		response.BadRequest(c, "INVALID_PARAMS", "tag is required")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	conn, err := h.connect(ctx)
	if err != nil {
		response.InternalError(c, "CONNECT_FAILED", err.Error())
		return
	}
	defer conn.Close()

	client := proxymanCommand.NewHandlerServiceClient(conn)

	_, err = client.RemoveInbound(ctx, &proxymanCommand.RemoveInboundRequest{Tag: tag})
	if err != nil {
		response.InternalError(c, "REMOVE_INBOUND_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"message": "inbound removed successfully", "tag": tag})
}

// ==================== Outbound Management ====================

// AddOutbound handles POST /api/v1/xray/control/outbound.
func (h *Handler) AddOutbound(c *gin.Context) {
	var req struct {
		Outbound json.RawMessage `json:"outbound" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_PARAMS", err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	conn, err := h.connect(ctx)
	if err != nil {
		response.InternalError(c, "CONNECT_FAILED", err.Error())
		return
	}

	client := proxymanCommand.NewHandlerServiceClient(conn)

	outboundConf := new(conf.OutboundDetourConfig)
	if err := json.Unmarshal(req.Outbound, outboundConf); err != nil {
		response.BadRequest(c, "INVALID_CONFIG", "failed to parse outbound config: "+err.Error())
		return
	}

	config, err := outboundConf.Build()
	if err != nil {
		response.BadRequest(c, "BUILD_FAILED", "failed to build outbound config: "+err.Error())
		return
	}

	_, err = client.AddOutbound(ctx, &proxymanCommand.AddOutboundRequest{Outbound: config})
	if err != nil {
		response.InternalError(c, "ADD_OUTBOUND_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"message": "outbound added successfully"})
}

// RemoveOutbound handles DELETE /api/v1/xray/control/outbound/:tag.
func (h *Handler) RemoveOutbound(c *gin.Context) {
	tag := c.Param("tag")
	if tag == "" {
		response.BadRequest(c, "INVALID_PARAMS", "tag is required")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	conn, err := h.connect(ctx)
	if err != nil {
		response.InternalError(c, "CONNECT_FAILED", err.Error())
		return
	}

	client := proxymanCommand.NewHandlerServiceClient(conn)

	_, err = client.RemoveOutbound(ctx, &proxymanCommand.RemoveOutboundRequest{Tag: tag})
	if err != nil {
		response.InternalError(c, "REMOVE_OUTBOUND_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"message": "outbound removed successfully", "tag": tag})
}

// ==================== User Management ====================

// AddUser handles POST /api/v1/xray/control/inbound/:tag/user.
// Supported protocols: VLESS, VMess, Trojan, Shadowsocks
func (h *Handler) AddUser(c *gin.Context) {
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
		Method   string `json:"method"` // Shadowsocks cipher method
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_PARAMS", err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	conn, err := h.connect(ctx)
	if err != nil {
		response.InternalError(c, "CONNECT_FAILED", err.Error())
		return
	}
	defer conn.Close()

	client := proxymanCommand.NewHandlerServiceClient(conn)

	// Build account based on protocol (detected from tag)
	var account *serial.TypedMessage
	tagLower := strings.ToLower(tag)

	if strings.Contains(tagLower, "vless") {
		// VLESS account
		vlessAccount := &vless.Account{
			Id:   req.ID,
			Flow: req.Flow,
		}
		account = serial.ToTypedMessage(vlessAccount)
	} else if strings.Contains(tagLower, "vmess") {
		// VMess account
		vmessAccount := &vmess.Account{
			Id: req.ID,
		}
		account = serial.ToTypedMessage(vmessAccount)
	} else if strings.Contains(tagLower, "hysteria") {
		// Hysteria account - uses auth field
		hysteriaAccount := &hyAccount.Account{
			Auth: req.Password, // Frontend sends auth as password
		}
		account = serial.ToTypedMessage(hysteriaAccount)
	} else if strings.Contains(tagLower, "shadowsocks") || strings.Contains(tagLower, "ss-") {
		// Shadowsocks account
		if strings.HasPrefix(req.Method, "2022-blake3") {
			// 2022-blake3 methods use shadowsocks_2022.Account with Key
			ss2022Account := &ss2022.Account{
				Key: req.Password,
			}
			account = serial.ToTypedMessage(ss2022Account)
		} else {
			// Legacy methods use shadowsocks.Account with Password and CipherType
			cipherType := shadowsocks.CipherType_AES_256_GCM
			if req.Method != "" {
				switch req.Method {
				case "aes-128-gcm":
					cipherType = shadowsocks.CipherType_AES_128_GCM
				case "chacha20-poly1305":
					cipherType = shadowsocks.CipherType_CHACHA20_POLY1305
				case "xchacha20-ietf-poly1305":
					cipherType = shadowsocks.CipherType_XCHACHA20_POLY1305
				case "none":
					cipherType = shadowsocks.CipherType_NONE
				}
			}
			ssAccount := &shadowsocks.Account{
				Password:   req.Password,
				CipherType: cipherType,
			}
			account = serial.ToTypedMessage(ssAccount)
		}
	} else if req.Password != "" {
		// Trojan account (default for password-based)
		trojanAccount := &trojan.Account{
			Password: req.Password,
		}
		account = serial.ToTypedMessage(trojanAccount)
	}

	user := &protocol.User{
		Email:   req.Email,
		Account: account,
	}

	_, err = client.AlterInbound(ctx, &proxymanCommand.AlterInboundRequest{
		Tag:       tag,
		Operation: serial.ToTypedMessage(&proxymanCommand.AddUserOperation{User: user}),
	})
	if err != nil {
		response.InternalError(c, "ADD_USER_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"message": "user added successfully", "email": req.Email, "tag": tag})
}

// RemoveUser handles DELETE /api/v1/xray/control/inbound/:tag/user/:email.
func (h *Handler) RemoveUser(c *gin.Context) {
	tag := c.Param("tag")
	email := c.Param("email")
	if tag == "" || email == "" {
		response.BadRequest(c, "INVALID_PARAMS", "tag and email are required")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	conn, err := h.connect(ctx)
	if err != nil {
		response.InternalError(c, "CONNECT_FAILED", err.Error())
		return
	}
	defer conn.Close()

	client := proxymanCommand.NewHandlerServiceClient(conn)

	_, err = client.AlterInbound(ctx, &proxymanCommand.AlterInboundRequest{
		Tag:       tag,
		Operation: serial.ToTypedMessage(&proxymanCommand.RemoveUserOperation{Email: email}),
	})
	if err != nil {
		response.InternalError(c, "REMOVE_USER_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"message": "user removed from xray", "email": email, "tag": tag})
}

// ==================== Routing Management ====================

// ApplyRouting handles POST /api/v1/xray/control/routing.
func (h *Handler) ApplyRouting(c *gin.Context) {
	var req struct {
		Routing json.RawMessage `json:"routing" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_PARAMS", err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	conn, err := h.connect(ctx)
	if err != nil {
		response.InternalError(c, "CONNECT_FAILED", err.Error())
		return
	}
	defer conn.Close()

	client := routerCommand.NewRoutingServiceClient(conn)

	// Parse full routing config
	routerConf := new(conf.RouterConfig)
	if err := json.Unmarshal(req.Routing, routerConf); err != nil {
		response.BadRequest(c, "INVALID_CONFIG", "failed to parse routing config: "+err.Error())
		return
	}
	ensureImplicitAPIRoutingRule(routerConf)

	config, err := routerConf.Build()
	if err != nil {
		response.BadRequest(c, "BUILD_FAILED", "failed to build routing config: "+err.Error())
		return
	}

	// Apply routing (replace all rules)
	_, err = client.AddRule(ctx, &routerCommand.AddRuleRequest{
		ShouldAppend: false,
		Config:       serial.ToTypedMessage(config),
	})
	if err != nil {
		response.InternalError(c, "APPLY_ROUTING_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"message": "routing applied successfully"})
}

// AddRoutingRule handles POST /api/v1/xray/control/routing/rule.
func (h *Handler) AddRoutingRule(c *gin.Context) {
	var req struct {
		Rule   json.RawMessage `json:"rule" binding:"required"`
		Append bool            `json:"append"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_PARAMS", err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	conn, err := h.connect(ctx)
	if err != nil {
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
		response.InternalError(c, "ADD_RULE_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"message": "routing rule added successfully"})
}

// RemoveRoutingRule handles DELETE /api/v1/xray/control/routing/rule/:tag.
func (h *Handler) RemoveRoutingRule(c *gin.Context) {
	tag := c.Param("tag")
	if tag == "" {
		response.BadRequest(c, "INVALID_PARAMS", "tag is required")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	conn, err := h.connect(ctx)
	if err != nil {
		response.InternalError(c, "CONNECT_FAILED", err.Error())
		return
	}
	defer conn.Close()

	client := routerCommand.NewRoutingServiceClient(conn)

	_, err = client.RemoveRule(ctx, &routerCommand.RemoveRuleRequest{RuleTag: tag})
	if err != nil {
		response.InternalError(c, "REMOVE_RULE_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"message": "routing rule removed successfully", "tag": tag})
}

// ListRoutingRules handles GET /api/v1/xray/control/routing/rules.
func (h *Handler) ListRoutingRules(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	conn, err := h.connect(ctx)
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

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	conn, err := h.connect(ctx)
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

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	conn, err := h.connect(ctx)
	if err != nil {
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
		response.InternalError(c, "SET_BALANCER_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"message": "balancer target set successfully", "tag": tag, "target": req.Target})
}

// ==================== Stats ====================

// GetStats handles GET /api/v1/xray/stats.
func (h *Handler) GetStats(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	statsAddr := h.getStatsAddr(ctx)

	conn, err := grpc.DialContext(ctx, statsAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		response.InternalError(c, "CONNECT_FAILED", err.Error())
		return
	}
	// Connection is reused, don't close

	client := statsCommand.NewStatsServiceClient(conn)

	resp, err := client.QueryStats(ctx, &statsCommand.QueryStatsRequest{Reset_: false})
	if err != nil {
		response.InternalError(c, "STATS_QUERY_FAILED", err.Error())
		return
	}

	users := make(map[string]*UserTraffic)
	inbounds := make(map[string]*InboundTraffic)

	for _, stat := range resp.GetStat() {
		name := stat.GetName()
		value := stat.GetValue()

		if contains(name, "user>>>") && contains(name, ">>>traffic>>>") {
			parts := splitN(name, ">>>")
			if len(parts) == 4 {
				email := parts[1]
				direction := parts[3]
				if users[email] == nil {
					users[email] = &UserTraffic{Email: email}
				}
				if direction == "uplink" {
					users[email].Uplink = value
				} else {
					users[email].Downlink = value
				}
			}
		}

		if contains(name, "inbound>>>") && contains(name, ">>>traffic>>>") {
			parts := splitN(name, ">>>")
			if len(parts) == 4 {
				tag := parts[1]
				direction := parts[3]
				if inbounds[tag] == nil {
					inbounds[tag] = &InboundTraffic{Tag: tag}
				}
				if direction == "uplink" {
					inbounds[tag].Uplink = value
				} else {
					inbounds[tag].Downlink = value
				}
			}
		}
	}

	userList := make([]UserTraffic, 0, len(users))
	for _, u := range users {
		u.Total = u.Uplink + u.Downlink
		userList = append(userList, *u)
	}

	inboundList := make([]InboundTraffic, 0, len(inbounds))
	for _, ib := range inbounds {
		ib.Total = ib.Uplink + ib.Downlink
		inboundList = append(inboundList, *ib)
	}

	response.OK(c, &StatsResponse{Users: userList, Inbounds: inboundList})
}

// UserTraffic represents traffic stats for a user.
type UserTraffic struct {
	Email    string `json:"email"`
	Uplink   int64  `json:"uplink"`
	Downlink int64  `json:"downlink"`
	Total    int64  `json:"total"`
}

// InboundTraffic represents traffic stats for an inbound.
type InboundTraffic struct {
	Tag      string `json:"tag"`
	Uplink   int64  `json:"uplink"`
	Downlink int64  `json:"downlink"`
	Total    int64  `json:"total"`
}

// StatsResponse represents the stats response.
type StatsResponse struct {
	Users    []UserTraffic    `json:"users"`
	Inbounds []InboundTraffic `json:"inbounds"`
}

// ResetUserStats handles POST /api/v1/xray/stats/reset/:email.
func (h *Handler) ResetUserStats(c *gin.Context) {
	email := c.Param("email")
	if email == "" {
		response.BadRequest(c, "INVALID_PARAMS", "email is required")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	statsAddr := h.getStatsAddr(ctx)

	conn, err := grpc.DialContext(ctx, statsAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		response.InternalError(c, "CONNECT_FAILED", err.Error())
		return
	}
	// Connection is reused, don't close

	client := statsCommand.NewStatsServiceClient(conn)

	resp, err := client.QueryStats(ctx, &statsCommand.QueryStatsRequest{Reset_: false})
	if err != nil {
		response.InternalError(c, "STATS_QUERY_FAILED", err.Error())
		return
	}

	for _, stat := range resp.GetStat() {
		name := stat.GetName()
		if contains(name, "user>>>"+email+">>>traffic>>>") {
			client.QueryStats(ctx, &statsCommand.QueryStatsRequest{
				Pattern: name,
				Reset_:  true,
			})
		}
	}

	response.OK(c, gin.H{"message": "stats reset successfully", "email": email})
}

// ==================== Helpers ====================

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func splitN(s, sep string) []string {
	var result []string
	for {
		idx := -1
		for i := 0; i <= len(s)-len(sep); i++ {
			if s[i:i+len(sep)] == sep {
				idx = i
				break
			}
		}
		if idx < 0 {
			result = append(result, s)
			break
		}
		result = append(result, s[:idx])
		s = s[idx+len(sep):]
	}
	return result
}
