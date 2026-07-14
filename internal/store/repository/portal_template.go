package repository

import (
	"context"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// PortalTemplateRepository handles portal template database operations.
type PortalTemplateRepository struct {
	db *gorm.DB
}

// NewPortalTemplateRepository creates a new PortalTemplateRepository.
func NewPortalTemplateRepository(db *gorm.DB) *PortalTemplateRepository {
	return &PortalTemplateRepository{db: db}
}

// List returns all portal templates.
func (r *PortalTemplateRepository) List(ctx context.Context) ([]entity.PortalTemplate, error) {
	var templates []entity.PortalTemplate
	err := r.db.WithContext(ctx).Order("is_built_in DESC, updated_at DESC").Find(&templates).Error
	return templates, err
}

// GetByID returns a portal template by ID.
func (r *PortalTemplateRepository) GetByID(ctx context.Context, id int64) (*entity.PortalTemplate, error) {
	var tmpl entity.PortalTemplate
	err := r.db.WithContext(ctx).First(&tmpl, id).Error
	return &tmpl, err
}

// GetActive returns the currently active portal template.
func (r *PortalTemplateRepository) GetActive(ctx context.Context) (*entity.PortalTemplate, error) {
	var tmpl entity.PortalTemplate
	err := r.db.WithContext(ctx).Where("is_active = ?", true).First(&tmpl).Error
	return &tmpl, err
}

// Create creates a new portal template.
func (r *PortalTemplateRepository) Create(ctx context.Context, tmpl *entity.PortalTemplate) error {
	return r.db.WithContext(ctx).Create(tmpl).Error
}

// Update updates an existing portal template.
func (r *PortalTemplateRepository) Update(ctx context.Context, tmpl *entity.PortalTemplate) error {
	return r.db.WithContext(ctx).Save(tmpl).Error
}

// Delete deletes a portal template by ID.
func (r *PortalTemplateRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.PortalTemplate{}, id).Error
}

// DeactivateAll deactivates all portal templates.
func (r *PortalTemplateRepository) DeactivateAll(ctx context.Context) error {
	return r.db.WithContext(ctx).Model(&entity.PortalTemplate{}).
		Where("is_active = ?", true).
		Update("is_active", false).Error
}

// Activate sets the given template as the only active one.
func (r *PortalTemplateRepository) Activate(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&entity.PortalTemplate{}).
			Where("is_active = ?", true).
			Update("is_active", false).Error; err != nil {
			return err
		}
		return tx.Model(&entity.PortalTemplate{}).
			Where("id = ?", id).
			Update("is_active", true).Error
	})
}
