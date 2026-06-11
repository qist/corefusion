package repository

import (
	"context"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// NotificationTemplateRepository handles notification template database operations.
type NotificationTemplateRepository struct {
	db *gorm.DB
}

// NewNotificationTemplateRepository creates a new NotificationTemplateRepository.
func NewNotificationTemplateRepository(db *gorm.DB) *NotificationTemplateRepository {
	return &NotificationTemplateRepository{db: db}
}

func (r *NotificationTemplateRepository) Create(ctx context.Context, t *entity.NotificationTemplate) error {
	return r.db.WithContext(ctx).Create(t).Error
}

func (r *NotificationTemplateRepository) GetByID(ctx context.Context, id int64) (*entity.NotificationTemplate, error) {
	var t entity.NotificationTemplate
	err := r.db.WithContext(ctx).First(&t, id).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *NotificationTemplateRepository) List(ctx context.Context) ([]entity.NotificationTemplate, error) {
	var list []entity.NotificationTemplate
	err := r.db.WithContext(ctx).Order("id ASC").Find(&list).Error
	return list, err
}

func (r *NotificationTemplateRepository) ListEnabled(ctx context.Context) ([]entity.NotificationTemplate, error) {
	var list []entity.NotificationTemplate
	err := r.db.WithContext(ctx).Where("enabled = ?", true).Order("id ASC").Find(&list).Error
	return list, err
}

func (r *NotificationTemplateRepository) Update(ctx context.Context, t *entity.NotificationTemplate) error {
	return r.db.WithContext(ctx).Save(t).Error
}

func (r *NotificationTemplateRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.NotificationTemplate{}, id).Error
}
