package repository

import (
	"context"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// ACMEAccountConfigRepository handles ACME account config database operations.
type ACMEAccountConfigRepository struct {
	db *gorm.DB
}

// NewACMEAccountConfigRepository creates a new ACMEAccountConfigRepository.
func NewACMEAccountConfigRepository(db *gorm.DB) *ACMEAccountConfigRepository {
	return &ACMEAccountConfigRepository{db: db}
}

func (r *ACMEAccountConfigRepository) Create(ctx context.Context, cfg *entity.ACMEAccountConfig) error {
	return r.db.WithContext(ctx).Create(cfg).Error
}

func (r *ACMEAccountConfigRepository) GetByID(ctx context.Context, id int64) (*entity.ACMEAccountConfig, error) {
	var cfg entity.ACMEAccountConfig
	err := r.db.WithContext(ctx).First(&cfg, id).Error
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (r *ACMEAccountConfigRepository) GetDefault(ctx context.Context) (*entity.ACMEAccountConfig, error) {
	var cfg entity.ACMEAccountConfig
	err := r.db.WithContext(ctx).Where("is_default = ? AND status = ?", true, "active").First(&cfg).Error
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (r *ACMEAccountConfigRepository) List(ctx context.Context) ([]entity.ACMEAccountConfig, error) {
	var list []entity.ACMEAccountConfig
	err := r.db.WithContext(ctx).Order("id ASC").Find(&list).Error
	return list, err
}

func (r *ACMEAccountConfigRepository) Update(ctx context.Context, cfg *entity.ACMEAccountConfig) error {
	return r.db.WithContext(ctx).Save(cfg).Error
}

func (r *ACMEAccountConfigRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.ACMEAccountConfig{}, id).Error
}
