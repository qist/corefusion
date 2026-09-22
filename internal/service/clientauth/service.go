package clientauth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"corefusion/internal/auth/jwt"
	"corefusion/internal/auth/password"
	"corefusion/internal/model/entity"
	"corefusion/internal/service/nodeserver"
	"corefusion/internal/store/repository"
)

// SettingStore provides access to system settings.
type SettingStore interface {
	GetByKey(ctx context.Context, key string) (string, error)
}

// Service handles client authentication and self-service operations.
type Service struct {
	clientRepo      *repository.ClientRepository
	tmplRepo        *repository.PlanTemplateRepository
	subRepo         *repository.SubscriptionRepository
	balanceRepo     *repository.ClientBalanceRepository
	referralSvc     *ReferralService // P1: 邀请码注册绑定
	settingSvc      SettingStore
	jwt             *jwt.JWT
	trafficQuotaSvc *nodeserver.NodeTrafficQuotaService
}

// NewService creates a new client auth service.
func NewService(
	clientRepo *repository.ClientRepository,
	tmplRepo *repository.PlanTemplateRepository,
	subRepo *repository.SubscriptionRepository,
	settingSvc SettingStore,
	jwtSecret string,
) *Service {
	return &Service{
		clientRepo:  clientRepo,
		tmplRepo:    tmplRepo,
		subRepo:     subRepo,
		balanceRepo: nil, // will be set via SetBalanceRepo if needed
		settingSvc:  settingSvc,
		jwt:         jwt.New(jwtSecret, 24*time.Hour, 7*24*time.Hour),
	}
}

// SetBalanceRepo sets the balance repository (called from router initialization).
func (s *Service) SetBalanceRepo(repo *repository.ClientBalanceRepository) {
	s.balanceRepo = repo
}

// SetReferralService sets the referral service (called from router initialization).
// P1: 邀请码注册绑定 — 注册时自动建立邀请关系
func (s *Service) SetReferralService(svc *ReferralService) {
	s.referralSvc = svc
}

// SetTrafficQuotaSvc injects the node traffic quota service, used to aggregate a
// client's traffic history across the billing nodes of their subscription.
func (s *Service) SetTrafficQuotaSvc(svc *nodeserver.NodeTrafficQuotaService) {
	s.trafficQuotaSvc = svc
}

// TrafficHistoryPoint is one monthly data point of a client's aggregated traffic
// across the billing nodes of their subscription.
type TrafficHistoryPoint struct {
	PeriodStart time.Time `json:"period_start"`
	RxBytes     int64     `json:"rx_bytes"`
	TxBytes     int64     `json:"tx_bytes"`
	TotalBytes  int64     `json:"total_bytes"`
}

// GetTrafficHistory returns the authenticated client's traffic usage history (trend),
// aggregated across the billing nodes (Subscription.BillingNodeIDs) of the client's
// subscription. Returns an empty slice when the client has no subscription, no billing
// nodes, or the quota service is not wired. Safe for the portal "future extension"
// endpoint GET /api/v1/client-auth/traffic-history.
func (s *Service) GetTrafficHistory(ctx context.Context, clientID int64, months int) ([]TrafficHistoryPoint, error) {
	if months <= 0 || months > 24 {
		months = 6
	}
	if s.trafficQuotaSvc == nil {
		return []TrafficHistoryPoint{}, nil
	}
	client, err := s.clientRepo.GetByID(ctx, clientID)
	if err != nil {
		return nil, err
	}
	if client.SubscriptionID == nil || *client.SubscriptionID == 0 {
		return []TrafficHistoryPoint{}, nil
	}
	sub, err := s.subRepo.GetByID(ctx, *client.SubscriptionID)
	if err != nil {
		return nil, err
	}
	nodeIDs := []int64(sub.BillingNodeIDs)
	if len(nodeIDs) == 0 {
		return []TrafficHistoryPoint{}, nil
	}
	usage, err := s.trafficQuotaSvc.GetTrafficHistoryByServers(ctx, nodeIDs, months)
	if err != nil {
		return nil, err
	}
	agg := make(map[time.Time]*TrafficHistoryPoint, len(usage))
	for i := range usage {
		u := usage[i]
		total := u.RxBytes + u.TxBytes
		p, ok := agg[u.PeriodStart]
		if !ok {
			p = &TrafficHistoryPoint{PeriodStart: u.PeriodStart}
			agg[u.PeriodStart] = p
		}
		p.RxBytes += u.RxBytes
		p.TxBytes += u.TxBytes
		p.TotalBytes += total
	}
	points := make([]TrafficHistoryPoint, 0, len(agg))
	for _, p := range agg {
		points = append(points, *p)
	}
	sort.Slice(points, func(i, j int) bool {
		return points[i].PeriodStart.After(points[j].PeriodStart)
	})
	return points, nil
}

// RegisterInput is the input for client registration.
type RegisterInput struct {
	Email        string `json:"email" binding:"required,email"`
	Password     string `json:"password" binding:"required,min=6"`
	ReferralCode string `json:"referral_code"` // P1: 邀请码（可选）
}

// LoginInput is the input for client login.
type LoginInput struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// TokenResponse is the response for login/register.
type TokenResponse struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	Client       *ClientInfo `json:"client"`
}

// ClientInfo is the public client info.
type ClientInfo struct {
	ID              int64      `json:"id"`
	Email           string     `json:"email"`
	PlanID          int64      `json:"plan_id"`
	PlanName        string     `json:"plan_name"`
	Status          string     `json:"status"`
	QuotaBytes      int64      `json:"quota_bytes"`
	UsedBytes       int64      `json:"used_bytes"`
	UploadBytes     int64      `json:"upload_bytes"`
	DownloadBytes   int64      `json:"download_bytes"`
	ExpiresAt       *time.Time `json:"expires_at"`
	IPWhitelist     string     `json:"ip_whitelist"`
	DeviceLimit     int        `json:"device_limit"`
	SubToken        string     `json:"sub_token"`
	SubStatus       string     `json:"sub_status"`
	SubUpdatedAt    *time.Time `json:"sub_updated_at"`
	LastNodeUsageAt *time.Time `json:"last_node_usage_at"`
	CreatedAt       time.Time  `json:"created_at"`

	// P0-5: Extended fields
	Balance         int64      `json:"balance"`           // 余额（分）
	QuotaResetCycle string     `json:"quota_reset_cycle"` // 流量重置周期
	QuotaResetDay   *int       `json:"quota_reset_day"`   // 流量重置日
	NextResetAt     *time.Time `json:"next_reset_at"`     // 下次重置时间
}

// Register creates a new client account.
func (s *Service) Register(ctx context.Context, input RegisterInput) (*TokenResponse, error) {
	// Check if registration is enabled
	enabled, _ := s.settingSvc.GetByKey(ctx, "client_registration_enabled")
	if enabled != "true" {
		return nil, fmt.Errorf("registration is closed")
	}

	// Check email uniqueness
	if _, err := s.clientRepo.GetByEmail(ctx, input.Email); err == nil {
		return nil, fmt.Errorf("email already registered")
	}

	// Hash password
	hashedPwd, err := password.Hash(input.Password)
	if err != nil {
		return nil, err
	}

	// Create client (no plan assigned yet)
	client := &entity.Client{
		Email:    input.Email,
		Password: hashedPwd,
		Status:   "active",
	}
	if err := s.clientRepo.Create(ctx, client); err != nil {
		return nil, err
	}

	// P1: 邀请码注册绑定 — 自动建立邀请关系
	if input.ReferralCode != "" && s.referralSvc != nil {
		if err := s.referralSvc.ProcessReferral(ctx, input.ReferralCode, client.ID); err != nil {
			// 邀请码错误不影响注册流程，仅记录日志
			fmt.Printf("[register] failed to process referral code: %v\n", err)
		}
	}

	return s.generateTokens(client)
}

// Login authenticates a client by email and password.
func (s *Service) Login(ctx context.Context, input LoginInput) (*TokenResponse, error) {
	client, err := s.clientRepo.GetByEmail(ctx, input.Email)
	if err != nil {
		return nil, fmt.Errorf("invalid email or password")
	}

	if !password.Check(input.Password, client.Password) {
		return nil, fmt.Errorf("invalid email or password")
	}

	// ⚠ 账号被管理员停用后不允许再登录 ✗：旧实现只比密码，被停用的客户端照样能拿到 token
	// 并访问 /api/v1/client-auth/*（自助解封、工单、资料…）→ 封禁策略形同虚设（安全审计 v5 M2）。
	if err := ensureClientActive(client); err != nil {
		return nil, err
	}

	return s.generateTokens(client)
}

// ensureClientActive 拒绝非 active 状态的客户端（停用或其它未知状态一律拒绝，失败关闭）。
func ensureClientActive(client *entity.Client) error {
	if client == nil {
		return fmt.Errorf("client not found")
	}
	if client.Status != "active" {
		return fmt.Errorf("账号当前状态为 %q，已无法登录；请联系客服", client.Status)
	}
	return nil
}

// GetMe returns the client's account info.
func (s *Service) GetMe(ctx context.Context, clientID int64) (*ClientInfo, error) {
	client, err := s.clientRepo.GetByID(ctx, clientID)
	if err != nil {
		return nil, err
	}

	info := &ClientInfo{
		ID:              client.ID,
		Email:           client.Email,
		PlanID:          client.PlanID,
		Status:          client.Status,
		QuotaBytes:      client.QuotaBytes,
		UsedBytes:       client.UsedBytes,
		ExpiresAt:       client.ExpiresAt,
		IPWhitelist:     client.IPWhitelist,
		DeviceLimit:     client.DeviceLimit,
		SubUpdatedAt:    client.SubUpdatedAt,
		LastNodeUsageAt: client.LastNodeUsageAt,
		CreatedAt:       client.CreatedAt,
	}

	// Override with real-time subscription data if available.
	// The subscriptions table is kept up-to-date by the traffic sync service,
	// while the clients.used_bytes field may be stale.

	// Get plan template name
	if client.PlanID > 0 {
		if tmpl, err := s.tmplRepo.GetByID(ctx, client.PlanID); err == nil {
			info.PlanName = tmpl.Name
		}
	}

	// Get subscription token and status (only for active clients)
	if client.SubscriptionID != nil && client.Status == "active" {
		if sub, err := s.subRepo.GetByID(ctx, *client.SubscriptionID); err == nil {
			info.SubToken = sub.Token
			info.SubStatus = sub.Status
			// Use real-time traffic data from subscription record
			info.UsedBytes = sub.UsedBytesCache
			info.UploadBytes = sub.UploadBytesCache
			info.DownloadBytes = sub.DownloadBytesCache
			if sub.QuotaBytes > 0 {
				info.QuotaBytes = sub.QuotaBytes
			}
			if sub.ExpiresAt != nil {
				info.ExpiresAt = sub.ExpiresAt
			}
			// Use subscription's last access time if client's last_node_usage_at is empty
			if info.LastNodeUsageAt == nil && sub.LastAccessAt != nil {
				info.LastNodeUsageAt = sub.LastAccessAt
			}
			// Final fallback: use subscription's updated_at (updated by traffic sync)
			if info.LastNodeUsageAt == nil && !sub.UpdatedAt.IsZero() && sub.UsedBytesCache > 0 {
				info.LastNodeUsageAt = &sub.UpdatedAt
			}

			// P0-5: Extended subscription info
			info.QuotaResetCycle = sub.QuotaResetCycle
			info.QuotaResetDay = sub.QuotaResetDay
			info.NextResetAt = sub.NextResetAt
		}
	}

	// P0-5: Get balance
	if s.balanceRepo != nil {
		if bal, err := s.balanceRepo.GetByClientID(ctx, clientID); err == nil {
			info.Balance = bal.Balance
		}
	}

	return info, nil
}

// RotateToken rotates the client's subscription token.
func (s *Service) RotateToken(ctx context.Context, clientID int64) (string, error) {
	client, err := s.clientRepo.GetByID(ctx, clientID)
	if err != nil {
		return "", err
	}

	if client.SubscriptionID == nil {
		return "", fmt.Errorf("no subscription found")
	}

	sub, err := s.subRepo.GetByID(ctx, *client.SubscriptionID)
	if err != nil {
		return "", err
	}

	// Generate new token
	newToken := generateToken(32)
	sub.Token = newToken
	sub.TokenVersion++

	if err := s.subRepo.Update(ctx, sub); err != nil {
		return "", err
	}

	now := time.Now()
	client.SubUpdatedAt = &now
	s.clientRepo.Update(ctx, client)

	return newToken, nil
}

// UpdateSettings updates client's IP whitelist and device limit.
func (s *Service) UpdateSettings(ctx context.Context, clientID int64, ipWhitelist string, deviceLimit int) error {
	client, err := s.clientRepo.GetByID(ctx, clientID)
	if err != nil {
		return err
	}

	client.IPWhitelist = ipWhitelist
	client.DeviceLimit = deviceLimit
	return s.clientRepo.Update(ctx, client)
}

// ChangePassword changes the client's password.
func (s *Service) ChangePassword(ctx context.Context, clientID int64, oldPwd, newPwd string) error {
	client, err := s.clientRepo.GetByID(ctx, clientID)
	if err != nil {
		return err
	}

	if !password.Check(oldPwd, client.Password) {
		return fmt.Errorf("old password is incorrect")
	}

	hashedPwd, err := password.Hash(newPwd)
	if err != nil {
		return err
	}

	client.Password = hashedPwd
	return s.clientRepo.Update(ctx, client)
}

// errNotClientToken 表示 token 合法但主体不是客户端（例如管理端用户）。
var errNotClientToken = errors.New("token subject is not a client")

// errClientTokenRevoked 表示 token 已被登出/踢下线吊销（令牌版本不匹配）。
var errClientTokenRevoked = errors.New("client token has been revoked; please sign in again")

// ValidateAccessToken validates a JWT token and returns the client ID.
func (s *Service) ValidateAccessToken(tokenStr string) (int64, error) {
	claims, err := s.jwt.ValidateTyped(tokenStr, jwt.TokenTypeAccess)
	if err != nil {
		return 0, err
	}
	// Client IDs are stored as UserID in JWT
	return claims.UserID, nil
}

// ValidateClientAccessToken 与 ValidateAccessToken 相同，但**额外要求主体确实是客户端**。
//
// ⚠ 客户端与管理端共用同一把 JWT 签名密钥 ✗，因此管理端的 access token 也能通过
// ValidateAccessToken ✓（只是它的 user_id 会被当成 client_id ✗）→ 结果是管理端 token 可以：
//   - 调用**所有**客户端受保护接口（审计 v5 S2 修的是**反方向**：客户端 token 进管理端 ✓）；
//   - 从 `POST /api/v1/client-auth/ticket` 签出"客户端下载票据"（本轮 M10 真机实测：管理端
//     token 拿到 200 ✗，本应 401）。
//
// 客户端 token 的 role 由 generateTokens 固定写成 "client" → 这里硬性要求它 ✓，
// 于是两类 token 被彻底分开（管理端 token 不再能冒充客户端）。
func (s *Service) ValidateClientAccessToken(tokenStr string) (int64, error) {
	claims, err := s.jwt.ValidateTyped(tokenStr, jwt.TokenTypeAccess)
	if err != nil {
		return 0, err
	}
	if !strings.EqualFold(claims.Role, "client") {
		return 0, errNotClientToken
	}
	// 令牌版本校验：登出/踢下线后（版本 +1）旧 token 立即失效 ✗→✓
	if s.clientRepo != nil {
		tv, verr := s.clientRepo.GetTokenVersion(context.Background(), claims.UserID)
		if verr != nil {
			return 0, verr
		}
		if tv != claims.TokenVersion {
			return 0, errClientTokenRevoked
		}
	}
	return claims.UserID, nil
}

// Logout 让该客户端此前签发的**所有** token（含 refresh）立即失效（版本 +1）。
//
// 与"只清前端本地存储"的区别：后者对服务端无效 ✗ —— 被窃取的 access/refresh 在有效期内仍可用。
func (s *Service) Logout(ctx context.Context, clientID int64) error {
	if clientID <= 0 {
		return fmt.Errorf("invalid client id")
	}
	return s.clientRepo.BumpTokenVersion(ctx, clientID)
}

// RefreshToken validates a refresh token and issues a new token pair.
func (s *Service) RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	claims, err := s.jwt.ValidateTyped(refreshToken, jwt.TokenTypeRefresh)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token")
	}

	client, err := s.clientRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("client not found")
	}

	// 与登录同样校验状态 ✗：否则"登录被拦、但刷新通道还能一直续签"（旧 token 有效期内可无限续命）
	if err := ensureClientActive(client); err != nil {
		return nil, err
	}

	// ⚠ 还要校验令牌版本 ✗：否则"登出后拿旧 refresh 继续续签"仍然通（管理端 M8 已经踩过这个洞）
	if client.TokenVersion != claims.TokenVersion {
		return nil, errClientTokenRevoked
	}

	return s.generateTokens(client)
}

func (s *Service) generateTokens(client *entity.Client) (*TokenResponse, error) {
	// 带上 token_version：登出/踢下线后版本 +1 → 旧 token 立即失效（审计 v5 M8 的机制，
	// 本轮把它补到客户端侧；以前客户端登出只清前端本地存储，服务端仍认旧 token ✗）。
	accessToken, err := s.jwt.GenerateAccessTokenWithVersion(client.ID, client.Email, "client", client.TokenVersion)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.jwt.GenerateRefreshTokenWithVersion(client.ID, client.Email, "client", client.TokenVersion)
	if err != nil {
		return nil, err
	}

	return &TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		Client:       &ClientInfo{ID: client.ID, Email: client.Email},
	}, nil
}

func generateToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// Fallback should never happen with crypto/rand
		panic("failed to generate random token: " + err.Error())
	}
	return hex.EncodeToString(b)
}
