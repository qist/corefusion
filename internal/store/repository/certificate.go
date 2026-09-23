package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	certpkg "corefusion/internal/runtime/cert"

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

// GetByDomainAndKeyType 按 (域名, 密钥类型) 取证书。
//
// ⚠ 存在的理由 ✗：`GetByDomain` 只按域名取**第一条** ✓，于是"同域同时签 ECC + RSA"
// 时第二个请求会命中第一条并把它**改写** ✗ → 最后只剩一份 ✓（真机现象 ✓）。
// 记录的身份应当是 (domain, key_type) ✓ —— 这样两种密钥类型各自一条 ✓。
func (r *CertificateRepository) GetByDomainAndKeyType(ctx context.Context, domain, keyType string) (*entity.Certificate, error) {
	var cert entity.Certificate
	kt := certpkg.NormalizeKeyType(keyType)
	err := r.db.WithContext(ctx).Where("domain = ? AND key_type = ?", domain, kt).First(&cert).Error
	if err != nil {
		return nil, err
	}
	return &cert, nil
}

// ListByDomain 列出某域名的**所有** active 证书（每种密钥类型一条 ✓）。
//
// 用途 ✓：代理（Caddy / nginx）生成站点配置时要把同域名的**多张**证书一起挂上 ✓
// （ECC + RSA ✓，由客户端能力自动挑 ✓）—— 以前只取第一条 ✗，
// 于是"同时签发"两张都签出来了、站点却只挂一张 ✗。
func (r *CertificateRepository) ListByDomain(ctx context.Context, domain string) ([]entity.Certificate, error) {
	var certs []entity.Certificate
	err := r.db.WithContext(ctx).
		Where("domain = ? AND status = ?", domain, "active").
		Order("key_type").
		Find(&certs).Error
	if err != nil {
		return nil, err
	}
	return certs, nil
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
