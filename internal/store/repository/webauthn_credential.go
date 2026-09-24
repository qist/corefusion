package repository

import (
	"context"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// WebAuthnCredentialRepository handles WebAuthn credential database operations.
type WebAuthnCredentialRepository struct {
	db *gorm.DB
}

// NewWebAuthnCredentialRepository creates a new WebAuthnCredentialRepository.
func NewWebAuthnCredentialRepository(db *gorm.DB) *WebAuthnCredentialRepository {
	return &WebAuthnCredentialRepository{db: db}
}

// Create stores a new credential.
func (r *WebAuthnCredentialRepository) Create(ctx context.Context, cred *entity.WebAuthnCredential) error {
	return r.db.WithContext(ctx).Create(cred).Error
}

// GetByID returns a credential by its internal ID, scoped to the owning user.
func (r *WebAuthnCredentialRepository) GetByID(ctx context.Context, id, userID int64) (*entity.WebAuthnCredential, error) {
	var cred entity.WebAuthnCredential
	err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).First(&cred).Error
	if err != nil {
		return nil, err
	}
	return &cred, nil
}

// GetByCredentialID returns a credential by its base64url(rawID), scoped by RP ID.
// 这是登录（assertion）时按凭证 ID 定位记录的主路径。
func (r *WebAuthnCredentialRepository) GetByCredentialID(ctx context.Context, rpid, credentialID string) (*entity.WebAuthnCredential, error) {
	var cred entity.WebAuthnCredential
	// ⚠ 列名是 GORM 从字段 RPID 生成的 rp_id（不是 rpid）。
	err := r.db.WithContext(ctx).Where("rp_id = ? AND credential_id = ?", rpid, credentialID).First(&cred).Error
	if err != nil {
		return nil, err
	}
	return &cred, nil
}

// ListByUser returns all credentials owned by a user.
func (r *WebAuthnCredentialRepository) ListByUser(ctx context.Context, userID int64) ([]entity.WebAuthnCredential, error) {
	var creds []entity.WebAuthnCredential
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("id DESC").Find(&creds).Error
	return creds, err
}

// Update persists changes to a credential (SignCount / JSON blob / last_used_at …).
func (r *WebAuthnCredentialRepository) Update(ctx context.Context, cred *entity.WebAuthnCredential) error {
	return r.db.WithContext(ctx).Save(cred).Error
}

// Delete removes a credential, scoped to the owning user (id = credential 内部 ID)。
func (r *WebAuthnCredentialRepository) Delete(ctx context.Context, id, userID int64) error {
	return r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&entity.WebAuthnCredential{}).Error
}