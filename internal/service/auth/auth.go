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

	accessToken, err = s.jwt.GenerateAccessToken(user.ID, user.Username, user.Role)
	if err != nil {
		return "", "", nil, err
	}

	refreshToken, err = s.jwt.GenerateRefreshToken(user.ID, user.Username, user.Role)
	if err != nil {
		return "", "", nil, err
	}

	return accessToken, refreshToken, user, nil
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

	accessToken, err = s.jwt.GenerateAccessToken(user.ID, user.Username, user.Role)
	if err != nil {
		return "", "", err
	}

	refreshToken, err = s.jwt.GenerateRefreshToken(user.ID, user.Username, user.Role)
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
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
