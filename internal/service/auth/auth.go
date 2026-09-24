package auth

import (
	"context"
	"errors"
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
	user, err = s.userRepo.GetByUsername(ctx, username)
	if err != nil {
		return "", "", nil, ErrInvalidCredentials
	}

	if user.Status != 1 {
		return "", "", nil, ErrUserDisabled
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

	return accessToken, refreshToken, nil
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
	return s.userRepo.Update(ctx, user)
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
