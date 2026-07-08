package repository

import (
	"context"
	"time"

	"corefusion/internal/model/entity"
	"gorm.io/gorm"
)

// ClientLogPackageRepository manages client log packaging tasks.
type ClientLogPackageRepository struct {
	db *gorm.DB
}

// NewClientLogPackageRepository creates a new ClientLogPackageRepository.
func NewClientLogPackageRepository(db *gorm.DB) *ClientLogPackageRepository {
	return &ClientLogPackageRepository{db: db}
}

// Create creates a new log package task.
func (r *ClientLogPackageRepository) Create(ctx context.Context, pkg *entity.ClientLogPackage) error {
	return r.db.WithContext(ctx).Create(pkg).Error
}

// GetByID returns a log package by ID.
func (r *ClientLogPackageRepository) GetByID(ctx context.Context, id string) (*entity.ClientLogPackage, error) {
	var pkg entity.ClientLogPackage
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&pkg).Error; err != nil {
		return nil, err
	}
	return &pkg, nil
}

// Update updates a log package.
func (r *ClientLogPackageRepository) Update(ctx context.Context, pkg *entity.ClientLogPackage) error {
	return r.db.WithContext(ctx).Save(pkg).Error
}

// ListExpired returns all expired packages that need cleanup.
func (r *ClientLogPackageRepository) ListExpired(ctx context.Context) ([]entity.ClientLogPackage, error) {
	var pkgs []entity.ClientLogPackage
	err := r.db.WithContext(ctx).
		Where("status IN ? AND expires_at < ?", []string{"ready", "processing"}, time.Now()).
		Find(&pkgs).Error
	return pkgs, err
}

// ListByClientID returns log packages for a specific client, ordered by creation time desc.
func (r *ClientLogPackageRepository) ListByClientID(ctx context.Context, clientID int64, limit, offset int) ([]entity.ClientLogPackage, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	var pkgs []entity.ClientLogPackage
	var total int64
	q := r.db.WithContext(ctx).Model(&entity.ClientLogPackage{}).Where("client_id = ?", clientID)
	q.Count(&total)
	err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&pkgs).Error
	return pkgs, total, err
}

// Delete removes a log package record.
func (r *ClientLogPackageRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&entity.ClientLogPackage{}).Error
}
