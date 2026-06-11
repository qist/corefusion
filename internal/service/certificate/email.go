package certificate

import (
	"context"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// ACMEEmailService handles ACME email/account business logic.
type ACMEEmailService struct {
	repo *repository.ACMEAccountConfigRepository
}

// NewACMEEmailService creates a new ACME email service.
func NewACMEEmailService(repo *repository.ACMEAccountConfigRepository) *ACMEEmailService {
	return &ACMEEmailService{repo: repo}
}

func (s *ACMEEmailService) Create(ctx context.Context, email string, isDefault bool) (*entity.ACMEAccountConfig, error) {
	if isDefault {
		s.unsetDefaults(ctx)
	}
	cfg := &entity.ACMEAccountConfig{
		Email:     email,
		IsDefault: isDefault,
		Status:    "active",
	}
	if err := s.repo.Create(ctx, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (s *ACMEEmailService) GetByID(ctx context.Context, id int64) (*entity.ACMEAccountConfig, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *ACMEEmailService) GetDefault(ctx context.Context) (*entity.ACMEAccountConfig, error) {
	return s.repo.GetDefault(ctx)
}

func (s *ACMEEmailService) List(ctx context.Context) ([]entity.ACMEAccountConfig, error) {
	return s.repo.List(ctx)
}

func (s *ACMEEmailService) Update(ctx context.Context, id int64, email string, isDefault bool) (*entity.ACMEAccountConfig, error) {
	cfg, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if isDefault && !cfg.IsDefault {
		s.unsetDefaults(ctx)
	}
	cfg.Email = email
	cfg.IsDefault = isDefault
	if err := s.repo.Update(ctx, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (s *ACMEEmailService) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

func (s *ACMEEmailService) unsetDefaults(ctx context.Context) {
	configs, _ := s.repo.List(ctx)
	for _, cfg := range configs {
		if cfg.IsDefault {
			cfg.IsDefault = false
			s.repo.Update(ctx, &cfg)
		}
	}
}
