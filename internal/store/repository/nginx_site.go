package repository

import (
	"context"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// NginxSiteRepository handles Nginx site database operations.
type NginxSiteRepository struct {
	db *gorm.DB
}

// NewNginxSiteRepository creates a new NginxSiteRepository.
func NewNginxSiteRepository(db *gorm.DB) *NginxSiteRepository {
	return &NginxSiteRepository{db: db}
}

func (r *NginxSiteRepository) Create(ctx context.Context, site *entity.NginxSite) error {
	return r.db.WithContext(ctx).Create(site).Error
}

func (r *NginxSiteRepository) GetByID(ctx context.Context, id int64) (*entity.NginxSite, error) {
	var site entity.NginxSite
	err := r.db.WithContext(ctx).First(&site, id).Error
	if err != nil {
		return nil, err
	}
	return &site, nil
}

func (r *NginxSiteRepository) List(ctx context.Context) ([]entity.NginxSite, error) {
	var sites []entity.NginxSite
	err := r.db.WithContext(ctx).Order("priority ASC, id ASC").Find(&sites).Error
	return sites, err
}

func (r *NginxSiteRepository) ListEnabled(ctx context.Context) ([]entity.NginxSite, error) {
	var sites []entity.NginxSite
	err := r.db.WithContext(ctx).Where("enabled = ?", true).Order("priority ASC, id ASC").Find(&sites).Error
	return sites, err
}

func (r *NginxSiteRepository) Update(ctx context.Context, site *entity.NginxSite) error {
	return r.db.WithContext(ctx).Save(site).Error
}

func (r *NginxSiteRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.NginxSite{}, id).Error
}
