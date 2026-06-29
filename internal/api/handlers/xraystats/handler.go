package xraystats

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	statsService "github.com/xtls/xray-core/app/stats/command"

	"corefusion/internal/api/response"
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
	GetFull(ctx context.Context, coreType string) (map[string]interface{}, error)
}

// Handler handles xray stats HTTP requests.
type Handler struct {
	settingSvc    SettingStore
	coreConfigSvc CoreConfigService
}

// NewHandler creates a new xray stats handler.
func NewHandler(settingSvc SettingStore) *Handler {
	return &Handler{settingSvc: settingSvc}
}

// Close is kept for API compatibility.
func (h *Handler) Close() {}

// connect creates a fresh gRPC connection for each request.
// This avoids stale transports after xray restarts or reloads.
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

// SetCoreConfigService sets the core config service.
func (h *Handler) SetCoreConfigService(svc CoreConfigService) {
	h.coreConfigSvc = svc
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
		fullConfig, err := h.coreConfigSvc.GetFull(ctx, "xray")
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

// GetStats handles GET /api/v1/xray/stats.
func (h *Handler) GetStats(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	stats, err := h.queryStats(ctx)
	if err != nil {
		response.InternalError(c, "STATS_QUERY_FAILED", err.Error())
		return
	}

	response.OK(c, stats)
}

// GetUserStats handles GET /api/v1/xray/stats/users.
func (h *Handler) GetUserStats(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	stats, err := h.queryStats(ctx)
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

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	err := h.resetUserStats(ctx, email)
	if err != nil {
		response.InternalError(c, "STATS_RESET_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"message": "stats reset successfully", "email": email})
}

// queryStats queries xray stats via gRPC.
func (h *Handler) queryStats(ctx context.Context) (*StatsResponse, error) {
	// Open a fresh connection for this request.
	conn, err := h.connect(ctx)
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
func (h *Handler) resetUserStats(ctx context.Context, email string) error {
	// Open a fresh connection for this request.
	conn, err := h.connect(ctx)
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
