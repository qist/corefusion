package repository

import (
	"context"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// EABConfigRepository handles EAB config database operations.
type EABConfigRepository struct {
	db *gorm.DB
}

// NewEABConfigRepository creates a new EABConfigRepository.
func NewEABConfigRepository(db *gorm.DB) *EABConfigRepository {
	return &EABConfigRepository{db: db}
}

func (r *EABConfigRepository) Create(ctx context.Context, cfg *entity.EABConfig) error {
	return r.db.WithContext(ctx).Create(cfg).Error
}

func (r *EABConfigRepository) GetByID(ctx context.Context, id int64) (*entity.EABConfig, error) {
	var cfg entity.EABConfig
	if err := r.db.WithContext(ctx).First(&cfg, id).Error; err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (r *EABConfigRepository) List(ctx context.Context) ([]entity.EABConfig, error) {
	var list []entity.EABConfig
	err := r.db.WithContext(ctx).Order("id ASC").Find(&list).Error
	return list, err
}

func (r *EABConfigRepository) Update(ctx context.Context, cfg *entity.EABConfig) error {
	return r.db.WithContext(ctx).Save(cfg).Error
}

func (r *EABConfigRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.EABConfig{}, id).Error
}
