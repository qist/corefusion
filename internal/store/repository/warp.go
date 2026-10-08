package repository

import (
	"context"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// WarpRepository handles WARP config database operations.
type WarpRepository struct {
	db *gorm.DB
}

// NewWarpRepository creates a new WarpRepository.
func NewWarpRepository(db *gorm.DB) *WarpRepository {
	return &WarpRepository{db: db}
}

// warpSecret 是 WARP 配置里需要加密落库的敏感列（与 config_json 同一把密钥）。
// 设备 ID / 公钥不是秘密，保持明文便于检索。
func warpSecret(c *entity.WarpConfig) {
	// 整列就是凭据（不是 JSON，无键名可判定）→ 无条件加密
	c.Token = EncryptConfigField(c.Token)
	c.PrivateKey = EncryptConfigField(c.PrivateKey)
	c.LicenseKey = EncryptConfigField(c.LicenseKey)
	c.ConfigJSON = EncryptConfigJSON(c.ConfigJSON)
}

func warpReveal(c *entity.WarpConfig) {
	c.Token = DecryptConfigJSON(c.Token)
	c.PrivateKey = DecryptConfigJSON(c.PrivateKey)
	c.LicenseKey = DecryptConfigJSON(c.LicenseKey)
	c.ConfigJSON = DecryptConfigJSON(c.ConfigJSON)
}

// Create creates a new WARP config.
func (r *WarpRepository) Create(ctx context.Context, cfg *entity.WarpConfig) error {
	warpSecret(cfg)
	return r.db.WithContext(ctx).Create(cfg).Error
}

// GetByID retrieves a WARP config by ID.
func (r *WarpRepository) GetByID(ctx context.Context, id int64) (*entity.WarpConfig, error) {
	var cfg entity.WarpConfig
	err := r.db.WithContext(ctx).First(&cfg, id).Error
	if err != nil {
		return nil, err
	}
	warpReveal(&cfg)
	return &cfg, nil
}

// GetActiveByServer retrieves the latest active WARP config for a specific server.
// If coreType is non-empty, it filters by core_type as well; otherwise it returns
// the latest active config for the server regardless of core type.
func (r *WarpRepository) GetActiveByServer(ctx context.Context, serverID int64, coreType string) (*entity.WarpConfig, error) {
	var cfg entity.WarpConfig
	q := r.db.WithContext(ctx).Where("server_id = ? AND status != ?", serverID, "not_registered")
	if coreType != "" {
		q = q.Where("core_type = ?", coreType)
	}
	err := q.Order("id DESC").First(&cfg).Error
	if err != nil {
		return nil, err
	}
	warpReveal(&cfg)
	return &cfg, nil
}

// ListByServer lists all WARP configs for a specific server.
func (r *WarpRepository) ListByServer(ctx context.Context, serverID int64) ([]*entity.WarpConfig, error) {
	var cfgs []*entity.WarpConfig
	err := r.db.WithContext(ctx).
		Where("server_id = ? AND status != ?", serverID, "not_registered").
		Order("id DESC").
		Find(&cfgs).Error
	if err == nil {
		for _, c := range cfgs {
			warpReveal(c)
		}
	}
	return cfgs, err
}

// Update updates a WARP config.
func (r *WarpRepository) Update(ctx context.Context, cfg *entity.WarpConfig) error {
	warpSecret(cfg)
	return r.db.WithContext(ctx).Save(cfg).Error
}

// Delete deletes a WARP config by ID.
func (r *WarpRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.WarpConfig{}, id).Error
}

// DeleteByServer deletes all WARP configs for a specific server.
func (r *WarpRepository) DeleteByServer(ctx context.Context, serverID int64) error {
	return r.db.WithContext(ctx).
		Where("server_id = ?", serverID).
		Delete(&entity.WarpConfig{}).Error
}
