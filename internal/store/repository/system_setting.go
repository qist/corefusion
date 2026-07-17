package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"corefusion/internal/model/entity"
)

// SystemSettingRepository handles system setting database operations.
type SystemSettingRepository struct {
	db *gorm.DB
}

// NewSystemSettingRepository creates a new SystemSettingRepository.
func NewSystemSettingRepository(db *gorm.DB) *SystemSettingRepository {
	return &SystemSettingRepository{db: db}
}

// GetByKey retrieves a setting by key.
func (r *SystemSettingRepository) GetByKey(ctx context.Context, key string) (string, error) {
	var s entity.SystemSetting
	err := r.db.WithContext(ctx).Where("`key` = ?", key).First(&s).Error
	if err != nil {
		return "", err
	}
	return s.Value, nil
}

// Set upserts a setting value.
func (r *SystemSettingRepository) Set(ctx context.Context, key, value string) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&entity.SystemSetting{Key: key, Value: value}).Error
}

// GetByPrefix retrieves all settings with a key prefix.
func (r *SystemSettingRepository) GetByPrefix(ctx context.Context, prefix string) (map[string]string, error) {
	var settings []entity.SystemSetting
	err := r.db.WithContext(ctx).Where("`key` LIKE ?", prefix+"%").Find(&settings).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for _, s := range settings {
		result[s.Key] = s.Value
	}
	return result, nil
}

// Delete deletes a setting by key.
func (r *SystemSettingRepository) Delete(ctx context.Context, key string) error {
	return r.db.WithContext(ctx).Where("`key` = ?", key).Delete(&entity.SystemSetting{}).Error
}
