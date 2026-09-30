package auth

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"corefusion/internal/auth/jwt"
	"corefusion/internal/auth/password"
	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrUserDisabled       = errors.New("user account is disabled")
)

// Service handles authentication business logic.
type Service struct {
	userRepo *repository.UserRepository
	jwt      *jwt.JWT
	usedJTI  refreshJTIRegistry
}

// refreshJTIRegistry 记录"已用于换发"的 refresh token jti（安全审计 L-4）。
//
// refresh token 在 7 天 TTL 内本可与新一轮并行使用且不可检测 —— 这里把它做成
// **一次性**：已消费的 jti 再次出现即视为泄漏信号，直接吊销该用户全部 token。
// 故意放内存（进程重启即清空）：只损失重启窗口内的检测能力，不引入存储依赖。
type refreshJTIRegistry struct {
	mu   sync.Mutex
	used map[string]time.Time // jti → token 过期时间
}

// seenAndRecord 返回该 jti 是否已被消费过（无论结果都会记录/续期过期时间）。
func (r *refreshJTIRegistry) seenAndRecord(jti string, expiresAt time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.used == nil {
		r.used = make(map[string]time.Time)
	}
	if len(r.used) > 4096 {
		now := time.Now()
		for k, exp := range r.used {
			if now.After(exp) {
				delete(r.used, k)
			}
		}
	}
	_, seen := r.used[jti]
	r.used[jti] = expiresAt
	return seen
}

// NewService creates a new auth service.
func NewService(userRepo *repository.UserRepository, jwtSecret string) *Service {
	return &Service{
		userRepo: userRepo,
		jwt:      jwt.New(jwtSecret, 24*time.Hour, 7*24*time.Hour),
	}
}

// Login authenticates a user and returns tokens.
func (s *Service) Login(ctx context.Context, username, passwordStr string) (accessToken, refreshToken string, user *entity.User, err error) {
	user, err = s.GetActiveUserByUsername(ctx, username)
	if err != nil {
		return "", "", nil, err
	}

	if !password.Check(passwordStr, user.PasswordHash) {
		return "", "", nil, ErrInvalidCredentials
	}

	accessToken, refreshToken, err = s.IssueTokens(ctx, user)
	if err != nil {
		return "", "", nil, err
	}

	return accessToken, refreshToken, user, nil
}

// GetActiveUserByUsername 按用户名返回处于启用状态的用户，供各登录入口共用。
// 不存在 / 已禁用统一返回 ErrInvalidCredentials（不暴露账号存在性）。
func (s *Service) GetActiveUserByUsername(ctx context.Context, username string) (*entity.User, error) {
	user, err := s.userRepo.GetByUsername(ctx, username)
	if err != nil || user == nil {
		return nil, ErrInvalidCredentials
	}
	if user.Status != 1 {
		return nil, ErrUserDisabled
	}
	return user, nil
}

// IssueTokens 为已通过身份验证的用户签发 access/refresh token 对。
// 供 Passkey 等非口令登录途径复用 —— 保证所有登录入口走同一套 token 生成逻辑
// （含 token_version，登出/踢下线时版本 +1 即可吊销全部入口签发的 token）。
func (s *Service) IssueTokens(ctx context.Context, user *entity.User) (accessToken, refreshToken string, err error) {
	if user == nil || user.Status != 1 {
		return "", "", ErrUserDisabled
	}

	accessToken, err = s.jwt.GenerateAccessTokenWithVersion(user.ID, user.Username, user.Role, user.TokenVersion)
	if err != nil {
		return "", "", err
	}

	refreshToken, err = s.jwt.GenerateRefreshTokenWithVersion(user.ID, user.Username, user.Role, user.TokenVersion)
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

// RefreshToken validates a refresh token and returns new tokens.
func (s *Service) RefreshToken(ctx context.Context, refreshTokenStr string) (accessToken, refreshToken string, err error) {
	claims, err := s.jwt.ValidateTyped(refreshTokenStr, jwt.TokenTypeRefresh)
	if err != nil {
		return "", "", err
	}

	user, err := s.userRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		return "", "", ErrInvalidCredentials
	}

	if user.Status != 1 {
		return "", "", ErrUserDisabled
	}

	// 版本必须一致 ✗：否则"登出后拿旧 refresh token 继续续签"这条通道仍然通着。
	if user.TokenVersion != claims.TokenVersion {
		return "", "", ErrTokenRevoked
	}

	accessToken, err = s.jwt.GenerateAccessTokenWithVersion(user.ID, user.Username, user.Role, user.TokenVersion)
	if err != nil {
		return "", "", err
	}

	refreshToken, err = s.jwt.GenerateRefreshTokenWithVersion(user.ID, user.Username, user.Role, user.TokenVersion)
	if err != nil {
		return "", "", err
	}

	// 安全审计 L-4：refresh 轮换一次性检测。旧 token（无 jti 的历史 token 除外）
	// 已被消费过还再次提交 → 判定为泄漏，立即吊销该用户全部 token 并拒绝换发。
	if claims.ID != "" {
		exp := time.Now().Add(s.refreshTTL())
		if claims.ExpiresAt != nil {
			exp = claims.ExpiresAt.Time
		}
		if s.usedJTI.seenAndRecord(claims.ID, exp) {
			log.Printf("[security] refresh token reuse detected (user %d, jti %s); revoking all tokens", claims.UserID, claims.ID)
			_ = s.userRepo.BumpTokenVersion(ctx, claims.UserID)
			return "", "", ErrTokenRevoked
		}
	}

	return accessToken, refreshToken, nil
}

// refreshTTL 返回 refresh token 的 TTL（新签发 token 的有效期，用作兜底过期时间）。
func (s *Service) refreshTTL() time.Duration {
	return 7 * 24 * time.Hour
}

// ErrTokenRevoked 表示该 token 已被吊销（登出/踢下线后旧 token 的表现）。
var ErrTokenRevoked = errors.New("token revoked")

// Logout 吊销该用户**所有**已签发的 token（把令牌版本 +1）。
//
// 语义说明：JWT 无状态，"只登出当前这一台设备"需要按 token 记录 jti（额外存储）；
// 这里选择"登出即吊销该账号全部会话" —— 这是能真正解决"token 被窃取后无法作废"的最小方案，
// 也正好满足"把人踢下线"的需求。前端丢弃本地 token 的行为不变。
func (s *Service) Logout(ctx context.Context, userID int64) error {
	if userID <= 0 {
		return ErrInvalidCredentials
	}
	return s.userRepo.BumpTokenVersion(ctx, userID)
}

// ValidateToken validates an access token and returns the claims.
func (s *Service) ValidateToken(tokenStr string) (*jwt.Claims, error) {
	return s.jwt.Validate(tokenStr)
}

// GetByID retrieves a user by ID.
func (s *Service) GetByID(ctx context.Context, userID int64) (*entity.User, error) {
	return s.userRepo.GetByID(ctx, userID)
}

// SetTOTPSecret 加密保存用户的 TOTP 密钥（2FA setup 步骤；尚未启用）。
func (s *Service) SetTOTPSecret(ctx context.Context, userID int64, encryptedSecret string) error {
	return s.userRepo.SetTOTP(ctx, userID, false, encryptedSecret)
}

// ChangePassword changes a user's password.
func (s *Service) ChangePassword(ctx context.Context, userID int64, oldPassword, newPassword string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	if !password.Check(oldPassword, user.PasswordHash) {
		return ErrInvalidCredentials
	}

	hash, err := password.Hash(newPassword)
	if err != nil {
		return err
	}

	user.PasswordHash = hash
	if err := s.userRepo.Update(ctx, user); err != nil {
		return err
	}
	// 安全审计 M-3：改密成功后吊销该用户此前签发的所有 token（含 refresh），
	// 否则旧 token 在 TTL 内继续有效 —— 与 CLI 管理路径"改口令同时 BumpTokenVersion"的
	// 意图一致（admin.go）。
	return s.userRepo.BumpTokenVersion(ctx, userID)
}

// CreateInitialAdmin creates the first admin user if no users exist.
func (s *Service) CreateInitialAdmin(ctx context.Context, username, passwordStr string) error {
	_, total, err := s.userRepo.List(ctx, 1, 0)
	if err != nil {
		return err
	}
	if total > 0 {
		return errors.New("admin user already exists")
	}

	hash, err := password.Hash(passwordStr)
	if err != nil {
		return err
	}

	user := &entity.User{
		Username:     username,
		PasswordHash: hash,
		Role:         "super_admin",
		Status:       1,
	}

	return s.userRepo.Create(ctx, user)
}
