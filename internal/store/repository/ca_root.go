package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// CARootRepository handles CA root certificate storage operations.
type CARootRepository struct {
	db *gorm.DB
}

// NewCARootRepository creates a new CARootRepository.
func NewCARootRepository(db *gorm.DB) *CARootRepository {
	return &CARootRepository{db: db}
}

// GetActive returns the active CA root (the one with the largest ID / latest creation).
func (r *CARootRepository) GetActive(ctx context.Context) (*entity.CARoot, error) {
	var ca entity.CARoot
	err := r.db.WithContext(ctx).Order("id DESC").First(&ca).Error
	if err != nil {
		return nil, err
	}
	return &ca, nil
}

// ListValid returns all CA roots that have not yet expired. During a CA
// rotation these include both the outgoing (old) CA and the incoming (new) CA,
// so callers can build a multi-CA trust pool / bundle.
func (r *CARootRepository) ListValid(ctx context.Context) ([]entity.CARoot, error) {
	var list []entity.CARoot
	err := r.db.WithContext(ctx).
		Where("expires_at > ?", time.Now()).
		Order("id ASC").
		Find(&list).Error
	return list, err
}

// HasValidWithin reports whether a CA already exists whose expiry is after
// the given threshold time. Used to avoid generating duplicate CA roots during
// the overlap window (we only want one "incoming" CA at a time).
func (r *CARootRepository) HasValidWithin(ctx context.Context, threshold time.Time) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&entity.CARoot{}).
		Where("expires_at > ?", threshold).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// Create inserts a new CA root record.
func (r *CARootRepository) Create(ctx context.Context, ca *entity.CARoot) error {
	return r.db.WithContext(ctx).Create(ca).Error
}
