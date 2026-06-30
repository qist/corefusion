package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// CertRevocationRepository handles agent_cert_revocations database operations.
type CertRevocationRepository struct {
	db *gorm.DB
}

// NewCertRevocationRepository creates a new CertRevocationRepository.
func NewCertRevocationRepository(db *gorm.DB) *CertRevocationRepository {
	return &CertRevocationRepository{db: db}
}

// Create creates a new revocation record.
func (r *CertRevocationRepository) Create(ctx context.Context, revocation *entity.AgentCertRevocation) error {
	return r.db.WithContext(ctx).Create(revocation).Error
}

// ListActive returns all active revocations (effective_at <= now).
func (r *CertRevocationRepository) ListActive(ctx context.Context) ([]entity.AgentCertRevocation, error) {
	var revocations []entity.AgentCertRevocation
	err := r.db.WithContext(ctx).
		Where("effective_at <= ?", time.Now()).
		Order("revoked_at DESC").
		Find(&revocations).Error
	return revocations, err
}

// IsRevoked checks if a certificate serial is revoked.
func (r *CertRevocationRepository) IsRevoked(ctx context.Context, serial string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&entity.AgentCertRevocation{}).
		Where("cert_serial = ? AND effective_at <= ?", serial, time.Now()).
		Count(&count).Error
	return count > 0, err
}

// ListByServer returns all revocations for a specific server.
func (r *CertRevocationRepository) ListByServer(ctx context.Context, serverID int64) ([]entity.AgentCertRevocation, error) {
	var revocations []entity.AgentCertRevocation
	err := r.db.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("revoked_at DESC").
		Find(&revocations).Error
	return revocations, err
}

// Delete removes a revocation record (used when un-revoking, rare).
func (r *CertRevocationRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.AgentCertRevocation{}, id).Error
}
