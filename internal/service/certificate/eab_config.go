package certificate

import (
	"context"
	"errors"
	"strings"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// EABConfigService 管理**独立**的 EAB 配置 ✓（和 ACME 邮箱 / DNS 通道一样的独立配置项 ✓）。
type EABConfigService struct {
	repo *repository.EABConfigRepository
}

func NewEABConfigService(repo *repository.EABConfigRepository) *EABConfigService {
	return &EABConfigService{repo: repo}
}

// EABConfigInput 是新建/更新 EAB 配置的入参 ✓。
type EABConfigInput struct {
	Name       string
	CAProvider string
	KeyID      string
	MACKey     string
}

func (s *EABConfigService) GetByID(ctx context.Context, id int64) (*entity.EABConfig, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *EABConfigService) List(ctx context.Context) ([]entity.EABConfig, error) {
	list, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range list {
		list[i].HasMACKey = strings.TrimSpace(list[i].MACKey) != ""
	}
	return list, nil
}

func (s *EABConfigService) Create(ctx context.Context, in EABConfigInput) (*entity.EABConfig, error) {
	if strings.TrimSpace(in.Name) == "" {
		return nil, errors.New("name is required")
	}
	cfg := &entity.EABConfig{
		Name:       strings.TrimSpace(in.Name),
		CAProvider: normalizeCAProvider(in.CAProvider),
		KeyID:      strings.TrimSpace(in.KeyID),
		MACKey:     strings.TrimSpace(in.MACKey),
		Status:     "active",
	}
	if err := s.repo.Create(ctx, cfg); err != nil {
		return nil, err
	}
	cfg.HasMACKey = cfg.MACKey != ""
	return cfg, nil
}

func (s *EABConfigService) Update(ctx context.Context, id int64, in EABConfigInput) (*entity.EABConfig, error) {
	cfg, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Name) != "" {
		cfg.Name = strings.TrimSpace(in.Name)
	}
	if strings.TrimSpace(in.CAProvider) != "" {
		cfg.CAProvider = normalizeCAProvider(in.CAProvider)
	}
	cfg.KeyID = strings.TrimSpace(in.KeyID)
	// ⚠ HMAC-Key 留空 = **保持原值** ✓（前端拿不到密钥 ✗，编辑时没重填不该被清掉 ✗）
	if strings.TrimSpace(in.MACKey) != "" {
		cfg.MACKey = strings.TrimSpace(in.MACKey)
	}
	if err := s.repo.Update(ctx, cfg); err != nil {
		return nil, err
	}
	cfg.HasMACKey = cfg.MACKey != ""
	return cfg, nil
}

func (s *EABConfigService) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}
