package clientauth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"corefusion/internal/auth/jwt"
	"corefusion/internal/auth/password"
	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// SettingStore provides access to system settings.
type SettingStore interface {
	GetByKey(ctx context.Context, key string) (string, error)
}

// Service handles client authentication and self-service operations.
type Service struct {
	clientRepo   *repository.ClientRepository
	tmplRepo     *repository.PlanTemplateRepository
	subRepo      *repository.SubscriptionRepository
	balanceRepo  *repository.ClientBalanceRepository
	referralSvc  *ReferralService // P1: 邀请码注册绑定
	settingSvc   SettingStore
	jwt          *jwt.JWT
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
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
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
	Balance         int64      `json:"balance"`            // 余额（分）
	QuotaResetCycle string     `json:"quota_reset_cycle"`   // 流量重置周期
	QuotaResetDay   *int       `json:"quota_reset_day"`     // 流量重置日
	NextResetAt     *time.Time `json:"next_reset_at"`       // 下次重置时间
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

	return s.generateTokens(client)
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

	// Get subscription token and status
	if client.SubscriptionID != nil {
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

// ValidateAccessToken validates a JWT token and returns the client ID.
func (s *Service) ValidateAccessToken(tokenStr string) (int64, error) {
	claims, err := s.jwt.Validate(tokenStr)
	if err != nil {
		return 0, err
	}
	// Client IDs are stored as UserID in JWT
	return claims.UserID, nil
}

// RefreshToken validates a refresh token and issues a new token pair.
func (s *Service) RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	claims, err := s.jwt.Validate(refreshToken)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token")
	}

	client, err := s.clientRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("client not found")
	}

	return s.generateTokens(client)
}

func (s *Service) generateTokens(client *entity.Client) (*TokenResponse, error) {
	accessToken, err := s.jwt.GenerateAccessToken(client.ID, client.Email, "client")
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.jwt.GenerateRefreshToken(client.ID, client.Email, "client")
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

