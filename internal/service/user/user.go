package user

import (
	"context"

	"corefusion/internal/auth/password"
	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// Service handles user business logic.
type Service struct {
	repo *repository.UserRepository
}

// NewService creates a new user service.
func NewService(repo *repository.UserRepository) *Service {
	return &Service{repo: repo}
}

// CreateUserInput is the input for creating a user.
type CreateUserInput struct {
	Username string `json:"username" binding:"required"`
	Email    string `json:"email"`
	Password string `json:"password" binding:"required"`
	Role     string `json:"role" binding:"required"`
}

// UpdateUserInput is the input for updating a user.
type UpdateUserInput struct {
	Username *string `json:"username"`
	Email    *string `json:"email"`
	Password *string `json:"password"`
	Role     *string `json:"role"`
	Status   *int    `json:"status"`
}

// Create creates a new user.
func (s *Service) Create(ctx context.Context, input CreateUserInput) (*entity.User, error) {
	hash, err := password.Hash(input.Password)
	if err != nil {
		return nil, err
	}

	user := &entity.User{
		Username:     input.Username,
		Email:        input.Email,
		PasswordHash: hash,
		Role:         input.Role,
		Status:       1,
	}

	if err := s.repo.Create(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

// GetByID retrieves a user by ID.
func (s *Service) GetByID(ctx context.Context, id int64) (*entity.User, error) {
	return s.repo.GetByID(ctx, id)
}

// List retrieves all users with pagination.
func (s *Service) List(ctx context.Context, limit, offset int) ([]entity.User, int64, error) {
	return s.repo.List(ctx, limit, offset)
}

// Update updates a user.
func (s *Service) Update(ctx context.Context, id int64, input UpdateUserInput) (*entity.User, error) {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if input.Username != nil {
		user.Username = *input.Username
	}
	if input.Email != nil {
		user.Email = *input.Email
	}
	if input.Password != nil {
		hash, err := password.Hash(*input.Password)
		if err != nil {
			return nil, err
		}
		user.PasswordHash = hash
	}
	if input.Role != nil {
		user.Role = *input.Role
	}
	if input.Status != nil {
		user.Status = *input.Status
	}

	if err := s.repo.Update(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

// Delete deletes a user by ID.
func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}
