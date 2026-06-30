package repository

import (
	"context"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// PlanTemplateRepository handles plan_templates database operations.
type PlanTemplateRepository struct {
	db *gorm.DB
}

func NewPlanTemplateRepository(db *gorm.DB) *PlanTemplateRepository {
	return &PlanTemplateRepository{db: db}
}

func (r *PlanTemplateRepository) List(ctx context.Context) ([]entity.PlanTemplate, error) {
	var items []entity.PlanTemplate
	err := r.db.WithContext(ctx).Order("sort_order ASC, id ASC").Find(&items).Error
	return items, err
}

func (r *PlanTemplateRepository) ListEnabled(ctx context.Context) ([]entity.PlanTemplate, error) {
	var items []entity.PlanTemplate
	err := r.db.WithContext(ctx).Where("enabled = ?", true).Order("sort_order ASC, id ASC").Find(&items).Error
	return items, err
}

func (r *PlanTemplateRepository) GetByID(ctx context.Context, id int64) (*entity.PlanTemplate, error) {
	var item entity.PlanTemplate
	err := r.db.WithContext(ctx).First(&item, id).Error
	return &item, err
}

func (r *PlanTemplateRepository) Create(ctx context.Context, item *entity.PlanTemplate) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *PlanTemplateRepository) Update(ctx context.Context, item *entity.PlanTemplate) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *PlanTemplateRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.PlanTemplate{}, id).Error
}

func (r *PlanTemplateRepository) CountClients(ctx context.Context, templateID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&entity.Client{}).
		Where("plan_id = ?", templateID).Count(&count).Error
	return count, err
}

// PlanVariantRepository handles plan_variants database operations.
type PlanVariantRepository struct {
	db *gorm.DB
}

func NewPlanVariantRepository(db *gorm.DB) *PlanVariantRepository {
	return &PlanVariantRepository{db: db}
}

func (r *PlanVariantRepository) ListByTemplateID(ctx context.Context, templateID int64) ([]entity.PlanVariant, error) {
	var items []entity.PlanVariant
	err := r.db.WithContext(ctx).
		Where("template_id = ?", templateID).
		Order("sort_order ASC, id ASC").
		Find(&items).Error
	return items, err
}

func (r *PlanVariantRepository) GetByID(ctx context.Context, id int64) (*entity.PlanVariant, error) {
	var item entity.PlanVariant
	err := r.db.WithContext(ctx).First(&item, id).Error
	return &item, err
}

func (r *PlanVariantRepository) Create(ctx context.Context, item *entity.PlanVariant) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *PlanVariantRepository) Update(ctx context.Context, item *entity.PlanVariant) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *PlanVariantRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.PlanVariant{}, id).Error
}

func (r *PlanVariantRepository) GetDefaultByTemplateID(ctx context.Context, templateID int64) (*entity.PlanVariant, error) {
	var item entity.PlanVariant
	err := r.db.WithContext(ctx).
		Where("template_id = ? AND is_default = ? AND enabled = ?", templateID, true, true).
		First(&item).Error
	return &item, err
}
