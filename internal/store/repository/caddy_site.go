package repository

import (
	"context"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// CaddySiteRepository handles Caddy site database operations.
type CaddySiteRepository struct {
	db *gorm.DB
}

// NewCaddySiteRepository creates a new CaddySiteRepository.
func NewCaddySiteRepository(db *gorm.DB) *CaddySiteRepository {
	return &CaddySiteRepository{db: db}
}

func (r *CaddySiteRepository) Create(ctx context.Context, site *entity.CaddySite) error {
	return r.db.WithContext(ctx).Create(site).Error
}

func (r *CaddySiteRepository) GetByID(ctx context.Context, id int64) (*entity.CaddySite, error) {
	var site entity.CaddySite
	err := r.db.WithContext(ctx).First(&site, id).Error
	if err != nil {
		return nil, err
	}
	return &site, nil
}

func (r *CaddySiteRepository) List(ctx context.Context, serverID int64) ([]entity.CaddySite, error) {
	var sites []entity.CaddySite
	err := r.db.WithContext(ctx).Where("server_id = ?", serverID).Order("priority ASC, id ASC").Find(&sites).Error
	return sites, err
}

func (r *CaddySiteRepository) ListEnabled(ctx context.Context, serverID int64) ([]entity.CaddySite, error) {
	var sites []entity.CaddySite
	err := r.db.WithContext(ctx).Where("server_id = ? AND enabled = ?", serverID, true).Order("priority ASC, id ASC").Find(&sites).Error
	return sites, err
}

func (r *CaddySiteRepository) Update(ctx context.Context, site *entity.CaddySite) error {
	return r.db.WithContext(ctx).Save(site).Error
}

func (r *CaddySiteRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.CaddySite{}, id).Error
}
