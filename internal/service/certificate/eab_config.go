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
	// 用 GCP 授权自动申请 Google EAB 时才有 ✓
	GoogleProjectID string
	GoogleSAJSON    string
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
		list[i].HasGoogleSA = strings.TrimSpace(list[i].GoogleSAJSON) != ""
	}
	return list, nil
}

// GetByCA 按 CA 找一份**可直接用**的 EAB 配置 ✓ —— 这就是"自动关联" ✓：
// 签发时只要选了 Google / ZeroSSL ✓，不用再手选 EAB ✗，面板自动挑一份 ✓。
//
// 规则：优先挑"填了 HMAC-Key"的 ✓；同一 CA 有多份时取**最早创建的**（可预测 ✓），
// 用户若要指定具体某一份 → 签发时显式选 eab_config_id ✓（显式优先 ✓）。
func (s *EABConfigService) GetByCA(ctx context.Context, caProvider string) (*entity.EABConfig, error) {
	ca := normalizeCAProvider(caProvider)
	list, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range list {
		if normalizeCAProvider(list[i].CAProvider) != ca {
			continue
		}
		if strings.TrimSpace(list[i].KeyID) == "" || strings.TrimSpace(list[i].MACKey) == "" {
			continue // 缺 KID 或 HMAC-Key 的不能直接用 ✗
		}
		return &list[i], nil
	}
	return nil, nil // 没配 → 调用方给出清晰提示 ✓
}

func (s *EABConfigService) Create(ctx context.Context, in EABConfigInput) (*entity.EABConfig, error) {
	if strings.TrimSpace(in.Name) == "" {
		return nil, errors.New("name is required")
	}
	cfg := &entity.EABConfig{
		Name:            strings.TrimSpace(in.Name),
		CAProvider:      normalizeCAProvider(in.CAProvider),
		KeyID:           strings.TrimSpace(in.KeyID),
		MACKey:          strings.TrimSpace(in.MACKey),
		GoogleProjectID: strings.TrimSpace(in.GoogleProjectID),
		GoogleSAJSON:    strings.TrimSpace(in.GoogleSAJSON),
		Status:          "active",
	}
	if err := s.repo.Create(ctx, cfg); err != nil {
		return nil, err
	}
	cfg.HasMACKey = cfg.MACKey != ""
	cfg.HasGoogleSA = strings.TrimSpace(cfg.GoogleSAJSON) != ""
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
	if strings.TrimSpace(in.GoogleProjectID) != "" {
		cfg.GoogleProjectID = strings.TrimSpace(in.GoogleProjectID)
	}
	// 服务账号 JSON 同理：留空 = 保持原值 ✓（前端也拿不到它 ✗）
	if strings.TrimSpace(in.GoogleSAJSON) != "" {
		cfg.GoogleSAJSON = strings.TrimSpace(in.GoogleSAJSON)
	}
	if err := s.repo.Update(ctx, cfg); err != nil {
		return nil, err
	}
	cfg.HasMACKey = cfg.MACKey != ""
	cfg.HasGoogleSA = strings.TrimSpace(cfg.GoogleSAJSON) != ""
	return cfg, nil
}

func (s *EABConfigService) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}
