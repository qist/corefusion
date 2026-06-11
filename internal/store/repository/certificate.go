package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// CertificateRepository handles certificate database operations.
type CertificateRepository struct {
	db *gorm.DB
}

// NewCertificateRepository creates a new CertificateRepository.
func NewCertificateRepository(db *gorm.DB) *CertificateRepository {
	return &CertificateRepository{db: db}
}

// Create creates a new certificate.
func (r *CertificateRepository) Create(ctx context.Context, cert *entity.Certificate) error {
	return r.db.WithContext(ctx).Create(cert).Error
}

// GetByID retrieves a certificate by ID.
func (r *CertificateRepository) GetByID(ctx context.Context, id int64) (*entity.Certificate, error) {
	var cert entity.Certificate
	err := r.db.WithContext(ctx).First(&cert, id).Error
	if err != nil {
		return nil, err
	}
	return &cert, nil
}

// GetByDomain retrieves a certificate by domain.
func (r *CertificateRepository) GetByDomain(ctx context.Context, domain string) (*entity.Certificate, error) {
	var cert entity.Certificate
	err := r.db.WithContext(ctx).Where("domain = ?", domain).First(&cert).Error
	if err != nil {
		return nil, err
	}
	return &cert, nil
}

// List retrieves all certificates with pagination.
func (r *CertificateRepository) List(ctx context.Context, limit, offset int) ([]entity.Certificate, int64, error) {
	var certs []entity.Certificate
	var total int64

	r.db.WithContext(ctx).Model(&entity.Certificate{}).Count(&total)
	err := r.db.WithContext(ctx).Order("id DESC").Limit(limit).Offset(offset).Find(&certs).Error
	return certs, total, err
}

// Update updates a certificate.
func (r *CertificateRepository) Update(ctx context.Context, cert *entity.Certificate) error {
	return r.db.WithContext(ctx).Save(cert).Error
}

// Delete deletes a certificate by ID.
func (r *CertificateRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.Certificate{}, id).Error
}

// FindExpiringSoon retrieves certificates expiring within the given duration.
func (r *CertificateRepository) FindExpiringSoon(ctx context.Context, within time.Duration) ([]entity.Certificate, error) {
	var certs []entity.Certificate
	err := r.db.WithContext(ctx).Where("expires_at < ? AND status = ?", time.Now().Add(within), "active").Find(&certs).Error
	return certs, err
}
