package certificate

import (
	"context"
	"strings"

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

// ACMEAccountInput 是"新建/更新 ACME 账号"的入参 ✓（含 CA 与 EAB ✓）。
type ACMEAccountInput struct {
	Email     string
	IsDefault bool
	// CAProvider 留空按 lets_encrypt ✓
	CAProvider string
	EABKeyID   string
	EABMACKey  string
}

func (s *ACMEEmailService) Create(ctx context.Context, in ACMEAccountInput) (*entity.ACMEAccountConfig, error) {
	if in.IsDefault {
		s.unsetDefaults(ctx)
	}
	cfg := &entity.ACMEAccountConfig{
		Email:      in.Email,
		IsDefault:  in.IsDefault,
		Status:     "active",
		CAProvider: normalizeCAProvider(in.CAProvider),
		EABKeyID:   in.EABKeyID,
		EABMACKey:  in.EABMACKey,
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
	list, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range list {
		list[i].HasEAB = list[i].EABMACKey != "" && list[i].EABKeyID != ""
	}
	return list, nil
}

// normalizeCAProvider 归一 CA 名称 ✓（空 → lets_encrypt ✓，保证写库不为空 ✓）。
//
// ⚠ 必须**统一小写** ✗：EAB 的"按 CA 自动关联"是拿归一化后的值做比较 ✓，
// 若这里只去空格 ✗，那么 `"Google"` 与 `"google"` 就匹配不上 ✗
// （单测 `TestEABConfigGetByCA` 真机抓到过 ✓）。
func normalizeCAProvider(p string) string {
	p = strings.ToLower(strings.TrimSpace(p))
	if p == "" {
		return "lets_encrypt"
	}
	return p
}

func (s *ACMEEmailService) Update(ctx context.Context, id int64, in ACMEAccountInput) (*entity.ACMEAccountConfig, error) {
	cfg, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.IsDefault && !cfg.IsDefault {
		s.unsetDefaults(ctx)
	}
	cfg.Email = in.Email
	cfg.IsDefault = in.IsDefault
	if strings.TrimSpace(in.CAProvider) != "" {
		cfg.CAProvider = normalizeCAProvider(in.CAProvider)
	}
	cfg.EABKeyID = in.EABKeyID
	// ⚠ MAC 留空 = **保持原值** ✓（前端拿不到密钥 ✗，编辑时若不重填不该把它清掉 ✗）
	if strings.TrimSpace(in.EABMACKey) != "" {
		cfg.EABMACKey = in.EABMACKey
	}
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
