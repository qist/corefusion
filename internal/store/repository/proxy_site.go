package repository

import (
	"context"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// ProxySiteRepository handles proxy site database operations.
type ProxySiteRepository struct {
	db *gorm.DB
}

// NewProxySiteRepository creates a new ProxySiteRepository.
func NewProxySiteRepository(db *gorm.DB) *ProxySiteRepository {
	return &ProxySiteRepository{db: db}
}

func (r *ProxySiteRepository) Create(ctx context.Context, site *entity.ProxySite) error {
	return r.db.WithContext(ctx).Create(site).Error
}

func (r *ProxySiteRepository) GetByID(ctx context.Context, id int64) (*entity.ProxySite, error) {
	var site entity.ProxySite
	err := r.db.WithContext(ctx).First(&site, id).Error
	if err != nil {
		return nil, err
	}
	return &site, nil
}

func (r *ProxySiteRepository) List(ctx context.Context) ([]entity.ProxySite, error) {
	var sites []entity.ProxySite
	err := r.db.WithContext(ctx).Order("priority ASC, id ASC").Find(&sites).Error
	return sites, err
}

func (r *ProxySiteRepository) ListByType(ctx context.Context, proxyType string) ([]entity.ProxySite, error) {
	var sites []entity.ProxySite
	err := r.db.WithContext(ctx).Where("proxy_type = ?", proxyType).Order("priority ASC, id ASC").Find(&sites).Error
	return sites, err
}

func (r *ProxySiteRepository) ListEnabled(ctx context.Context) ([]entity.ProxySite, error) {
	var sites []entity.ProxySite
	err := r.db.WithContext(ctx).Where("enabled = ?", true).Order("priority ASC, id ASC").Find(&sites).Error
	return sites, err
}

func (r *ProxySiteRepository) ListEnabledByType(ctx context.Context, proxyType string) ([]entity.ProxySite, error) {
	var sites []entity.ProxySite
	err := r.db.WithContext(ctx).Where("enabled = ? AND proxy_type = ?", true, proxyType).Order("priority ASC, id ASC").Find(&sites).Error
	return sites, err
}

func (r *ProxySiteRepository) Update(ctx context.Context, site *entity.ProxySite) error {
	return r.db.WithContext(ctx).Save(site).Error
}

func (r *ProxySiteRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.ProxySite{}, id).Error
}
