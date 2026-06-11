package repository

import (
	"context"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// BackupRepository handles backup database operations.
type BackupRepository struct {
	db *gorm.DB
}

// NewBackupRepository creates a new BackupRepository.
func NewBackupRepository(db *gorm.DB) *BackupRepository {
	return &BackupRepository{db: db}
}

// --- Backup Records ---

func (r *BackupRepository) CreateRecord(ctx context.Context, rec *entity.BackupRecord) error {
	return r.db.WithContext(ctx).Create(rec).Error
}

func (r *BackupRepository) GetRecordByID(ctx context.Context, id int64) (*entity.BackupRecord, error) {
	var rec entity.BackupRecord
	err := r.db.WithContext(ctx).First(&rec, id).Error
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

func (r *BackupRepository) ListRecords(ctx context.Context) ([]entity.BackupRecord, error) {
	var list []entity.BackupRecord
	err := r.db.WithContext(ctx).Order("created_at DESC").Find(&list).Error
	return list, err
}

func (r *BackupRepository) DeleteRecord(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.BackupRecord{}, id).Error
}

// --- Backup Schedules ---

func (r *BackupRepository) CreateSchedule(ctx context.Context, s *entity.BackupSchedule) error {
	return r.db.WithContext(ctx).Create(s).Error
}

func (r *BackupRepository) ListSchedules(ctx context.Context) ([]entity.BackupSchedule, error) {
	var list []entity.BackupSchedule
	err := r.db.WithContext(ctx).Order("id ASC").Find(&list).Error
	return list, err
}

func (r *BackupRepository) UpdateSchedule(ctx context.Context, s *entity.BackupSchedule) error {
	return r.db.WithContext(ctx).Save(s).Error
}

func (r *BackupRepository) DeleteSchedule(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.BackupSchedule{}, id).Error
}

// --- Backup Storages ---

func (r *BackupRepository) CreateStorage(ctx context.Context, s *entity.BackupStorage) error {
	return r.db.WithContext(ctx).Create(s).Error
}

func (r *BackupRepository) ListStorages(ctx context.Context) ([]entity.BackupStorage, error) {
	var list []entity.BackupStorage
	err := r.db.WithContext(ctx).Order("id ASC").Find(&list).Error
	return list, err
}

func (r *BackupRepository) UpdateStorage(ctx context.Context, s *entity.BackupStorage) error {
	return r.db.WithContext(ctx).Save(s).Error
}

func (r *BackupRepository) DeleteStorage(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.BackupStorage{}, id).Error
}
