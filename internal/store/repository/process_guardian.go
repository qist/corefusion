package repository

import (
	"context"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// ProcessGuardianRepository handles process guardian database operations.
type ProcessGuardianRepository struct {
	db *gorm.DB
}

// NewProcessGuardianRepository creates a new ProcessGuardianRepository.
func NewProcessGuardianRepository(db *gorm.DB) *ProcessGuardianRepository {
	return &ProcessGuardianRepository{db: db}
}

func (r *ProcessGuardianRepository) Create(ctx context.Context, g *entity.ProcessGuardian) error {
	return r.db.WithContext(ctx).Create(g).Error
}

func (r *ProcessGuardianRepository) GetByServiceID(ctx context.Context, serviceID int64) (*entity.ProcessGuardian, error) {
	var g entity.ProcessGuardian
	err := r.db.WithContext(ctx).Where("service_id = ?", serviceID).First(&g).Error
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func (r *ProcessGuardianRepository) List(ctx context.Context) ([]entity.ProcessGuardian, error) {
	var list []entity.ProcessGuardian
	err := r.db.WithContext(ctx).Order("id ASC").Find(&list).Error
	return list, err
}

func (r *ProcessGuardianRepository) ListAutoStart(ctx context.Context) ([]entity.ProcessGuardian, error) {
	var list []entity.ProcessGuardian
	err := r.db.WithContext(ctx).Where("auto_start = ? AND manually_stopped = ?", true, false).Find(&list).Error
	return list, err
}

func (r *ProcessGuardianRepository) Update(ctx context.Context, g *entity.ProcessGuardian) error {
	return r.db.WithContext(ctx).Save(g).Error
}

func (r *ProcessGuardianRepository) Delete(ctx context.Context, serviceID int64) error {
	return r.db.WithContext(ctx).Where("service_id = ?", serviceID).Delete(&entity.ProcessGuardian{}).Error
}

// CrashLogRepository handles crash log database operations.
type CrashLogRepository struct {
	db *gorm.DB
}

// NewCrashLogRepository creates a new CrashLogRepository.
func NewCrashLogRepository(db *gorm.DB) *CrashLogRepository {
	return &CrashLogRepository{db: db}
}

func (r *CrashLogRepository) Create(ctx context.Context, log *entity.CoreCrashLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *CrashLogRepository) ListByServiceID(ctx context.Context, serviceID int64, limit int) ([]entity.CoreCrashLog, error) {
	var list []entity.CoreCrashLog
	err := r.db.WithContext(ctx).Where("service_id = ?", serviceID).Order("crash_at DESC").Limit(limit).Find(&list).Error
	return list, err
}

func (r *CrashLogRepository) DeleteByServiceID(ctx context.Context, serviceID int64) error {
	return r.db.WithContext(ctx).Where("service_id = ?", serviceID).Delete(&entity.CoreCrashLog{}).Error
}
