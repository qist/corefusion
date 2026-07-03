package xraystats

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	statsService "github.com/xtls/xray-core/app/stats/command"

	"corefusion/internal/api/response"
	agentpb "corefusion/internal/runtime/agent/proto"
)

// SettingStore provides access to system settings.
type SettingStore interface {
	GetByKey(ctx context.Context, key string) (string, error)
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

// CoreConfigService provides access to core configuration.
type CoreConfigService interface {
	GetFull(ctx context.Context, serverID int64, coreType string) (map[string]interface{}, error)
}

// RemoteXrayAPI provides xray API proxying to remote Agent nodes.
type RemoteXrayAPI interface {
	XrayAPIOnServer(ctx context.Context, serverID int64, req *agentpb.XrayAPIRequest) (*agentpb.XrayAPIResponse, error)
}

// Handler handles xray stats HTTP requests.
type Handler struct {
	settingSvc    SettingStore
	coreConfigSvc CoreConfigService
	nodeOperator  RemoteXrayAPI
}

// NewHandler creates a new xray stats handler.
func NewHandler(settingSvc SettingStore) *Handler {
	return &Handler{settingSvc: settingSvc}
}

// Close is kept for API compatibility.
func (h *Handler) Close() {}

// SetCoreConfigService sets the core config service.
func (h *Handler) SetCoreConfigService(svc CoreConfigService) {
	h.coreConfigSvc = svc
}

// SetNodeOperator sets the node operator for remote xray API proxying.
func (h *Handler) SetNodeOperator(op RemoteXrayAPI) {
	h.nodeOperator = op
}

// localServerID is the server ID of the local node.
const localServerID = int64(1)

// parseServerID extracts the :id parameter from the URL.
func parseServerID(c *gin.Context) int64 {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return localServerID
	}
	return id
}

// isRemote returns true if the server ID refers to a remote node.
func isRemote(serverID int64) bool {
	return serverID != localServerID
}

// connect creates a fresh gRPC connection for each request.
// This avoids stale transports after xray restarts or reloads.
func (h *Handler) connect(ctx context.Context, serverID int64) (*grpc.ClientConn, error) {
	addr := h.getStatsAddr(ctx, serverID)
	if addr == "" {
		return nil, fmt.Errorf("xray API not available for remote server %d", serverID)
	}
	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to xray: %w", err)
	}
	return conn, nil
}

// getStatsAddr returns the xray stats API address for a given server.
// For local nodes: Priority 1. DB setting, 2. Config file api.listen, 3. Default 127.0.0.1:10085
// For remote nodes: not supported.
func (h *Handler) getStatsAddr(ctx context.Context, serverID int64) string {
	if serverID != localServerID {
		return "" // Remote nodes: xray API not directly accessible from panel
	}
	// 1. Check DB setting
	if h.settingSvc != nil {
		val, err := h.settingSvc.GetByKey(ctx, "xray.stats_addr")
		if err == nil && val != "" {
			return val
		}
	}

	// 2. Check config file api.listen
	if h.coreConfigSvc != nil {
		fullConfig, err := h.coreConfigSvc.GetFull(ctx, serverID, "xray")
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

	// 3. Default
	return "127.0.0.1:10085"
}

// remoteXrayAPI calls the remote Agent's XrayAPI RPC.
// Returns (responseJSON, handled, errorOccurred).
//   - (resp, true, false):  remote call succeeded, resp contains JSON
//   - (nil, true, true):    remote call failed, error already written to response
//   - (nil, false, false):  server is local, caller should use direct connection
func (h *Handler) remoteXrayAPI(c *gin.Context, method string, payload interface{}) (json.RawMessage, bool, bool) {
	serverID := parseServerID(c)
	if !isRemote(serverID) {
		return nil, false, false // local — caller continues with direct gRPC
	}
	if h.nodeOperator == nil {
		response.InternalError(c, "NOT_CONFIGURED", "node operator not configured for remote servers")
		return nil, true, true
	}

	payloadBytes, _ := json.Marshal(payload)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	resp, err := h.nodeOperator.XrayAPIOnServer(ctx, serverID, &agentpb.XrayAPIRequest{
		Service: "stats",
		Method:  method,
		Payload: payloadBytes,
	})
	if err != nil {
		response.InternalError(c, "REMOTE_API_FAILED", err.Error())
		return nil, true, true
	}
	if !resp.Success {
		response.InternalError(c, "REMOTE_API_ERROR", resp.Error)
		return nil, true, true
	}
	return json.RawMessage(resp.Response), true, false
}

// GetStats handles GET /api/v1/xray/stats.
func (h *Handler) GetStats(c *gin.Context) {
	// Remote: proxy via Agent
	resp, handled, _ := h.remoteXrayAPI(c, "getStats", map[string]interface{}{})
	if handled {
		if resp != nil {
			var result StatsResponse
			if err := json.Unmarshal(resp, &result); err == nil {
				response.OK(c, &result)
			} else {
				var raw interface{}
				json.Unmarshal(resp, &raw)
				response.OK(c, raw)
			}
		}
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	stats, err := h.queryStats(ctx, parseServerID(c))
	if err != nil {
		response.InternalError(c, "STATS_QUERY_FAILED", err.Error())
		return
	}

	response.OK(c, stats)
}

// GetUserStats handles GET /api/v1/xray/stats/users.
func (h *Handler) GetUserStats(c *gin.Context) {
	// Remote: proxy via Agent
	if resp, handled, _ := h.remoteXrayAPI(c, "getStats", map[string]interface{}{}); handled {
		if resp != nil {
			var result StatsResponse
			if err := json.Unmarshal(resp, &result); err == nil {
				response.OK(c, result.Users)
			} else {
				var raw interface{}
				json.Unmarshal(resp, &raw)
				response.OK(c, raw)
			}
		}
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	stats, err := h.queryStats(ctx, parseServerID(c))
	if err != nil {
		response.InternalError(c, "STATS_QUERY_FAILED", err.Error())
		return
	}

	response.OK(c, stats.Users)
}

// ResetUserStats handles POST /api/v1/xray/stats/reset/:email.
func (h *Handler) ResetUserStats(c *gin.Context) {
	email := c.Param("email")
	if email == "" {
		response.BadRequest(c, "INVALID_PARAMS", "email is required")
		return
	}

	// Remote: proxy via Agent
	if resp, handled, _ := h.remoteXrayAPI(c, "resetUserStats", map[string]interface{}{"email": email}); handled {
		if resp != nil {
			var result interface{}
			json.Unmarshal(resp, &result)
			response.OK(c, result)
		}
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	err := h.resetUserStats(ctx, email, parseServerID(c))
	if err != nil {
		response.InternalError(c, "STATS_RESET_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"message": "stats reset successfully", "email": email})
}

// queryStats queries xray stats via gRPC.
func (h *Handler) queryStats(ctx context.Context, serverID int64) (*StatsResponse, error) {
	// Open a fresh connection for this request.
	conn, err := h.connect(ctx, serverID)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// Create stats service client
	client := statsService.NewStatsServiceClient(conn)

	// Query stats
	resp, err := client.QueryStats(ctx, &statsService.QueryStatsRequest{Reset_: false})
	if err != nil {
		return nil, fmt.Errorf("failed to query stats: %w", err)
	}

	// Parse stats
	users := make(map[string]*UserTraffic)
	inbounds := make(map[string]*InboundTraffic)

	for _, stat := range resp.GetStat() {
		name := stat.GetName()
		value := stat.GetValue()

		// Parse user stats: user>>>email>>>traffic>>>uplink/downlink
		if strings.Contains(name, "user>>>") && strings.Contains(name, ">>>traffic>>>") {
			parts := strings.Split(name, ">>>")
			if len(parts) == 4 {
				email := parts[1]
				direction := parts[3]

				if users[email] == nil {
					users[email] = &UserTraffic{Email: email}
				}

				switch direction {
				case "uplink":
					users[email].Uplink = value
				case "downlink":
					users[email].Downlink = value
				}
			}
		}

		// Parse inbound stats: inbound>>>tag>>>traffic>>>uplink/downlink
		if strings.Contains(name, "inbound>>>") && strings.Contains(name, ">>>traffic>>>") {
			parts := strings.Split(name, ">>>")
			if len(parts) == 4 {
				tag := parts[1]
				direction := parts[3]

				if inbounds[tag] == nil {
					inbounds[tag] = &InboundTraffic{Tag: tag}
				}

				switch direction {
				case "uplink":
					inbounds[tag].Uplink = value
				case "downlink":
					inbounds[tag].Downlink = value
				}
			}
		}
	}

	// Convert maps to slices
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

	return &StatsResponse{
		Users:    userList,
		Inbounds: inboundList,
	}, nil
}

// resetUserStats resets user stats.
func (h *Handler) resetUserStats(ctx context.Context, email string, serverID int64) error {
	// Open a fresh connection for this request.
	conn, err := h.connect(ctx, serverID)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Create stats service client
	client := statsService.NewStatsServiceClient(conn)

	// Query stats to find user's stat names
	resp, err := client.QueryStats(ctx, &statsService.QueryStatsRequest{Reset_: false})
	if err != nil {
		return fmt.Errorf("failed to query stats: %w", err)
	}

	// Find and reset user's stats
	for _, stat := range resp.GetStat() {
		name := stat.GetName()
		if strings.Contains(name, "user>>>"+email+">>>traffic>>>") {
			// Reset this stat
			_, err := client.QueryStats(ctx, &statsService.QueryStatsRequest{
				Pattern: name,
				Reset_:  true,
			})
			if err != nil {
				return fmt.Errorf("failed to reset stat %s: %w", name, err)
			}
		}
	}

	return nil
}
