package repository

import (
	"context"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// LogRotationPolicyRepository handles log rotation policy database operations.
type LogRotationPolicyRepository struct {
	db *gorm.DB
}

// NewLogRotationPolicyRepository creates a new LogRotationPolicyRepository.
func NewLogRotationPolicyRepository(db *gorm.DB) *LogRotationPolicyRepository {
	return &LogRotationPolicyRepository{db: db}
}

func (r *LogRotationPolicyRepository) Create(ctx context.Context, policy *entity.LogRotationPolicy) error {
	return r.db.WithContext(ctx).Create(policy).Error
}

func (r *LogRotationPolicyRepository) GetByID(ctx context.Context, id int64) (*entity.LogRotationPolicy, error) {
	var policy entity.LogRotationPolicy
	err := r.db.WithContext(ctx).First(&policy, id).Error
	if err != nil {
		return nil, err
	}
	return &policy, nil
}

func (r *LogRotationPolicyRepository) List(ctx context.Context) ([]entity.LogRotationPolicy, error) {
	var policies []entity.LogRotationPolicy
	err := r.db.WithContext(ctx).Order("id ASC").Find(&policies).Error
	return policies, err
}

func (r *LogRotationPolicyRepository) ListEnabled(ctx context.Context) ([]entity.LogRotationPolicy, error) {
	var policies []entity.LogRotationPolicy
	err := r.db.WithContext(ctx).Where("enabled = ?", true).Order("id ASC").Find(&policies).Error
	return policies, err
}

func (r *LogRotationPolicyRepository) GetBySourceType(ctx context.Context, sourceType string) ([]entity.LogRotationPolicy, error) {
	var policies []entity.LogRotationPolicy
	err := r.db.WithContext(ctx).Where("source_type = ?", sourceType).Order("id ASC").Find(&policies).Error
	return policies, err
}

func (r *LogRotationPolicyRepository) GetByLogPath(ctx context.Context, logPath string) (*entity.LogRotationPolicy, error) {
	var policy entity.LogRotationPolicy
	err := r.db.WithContext(ctx).Where("log_path = ?", logPath).First(&policy).Error
	if err != nil {
		return nil, err
	}
	return &policy, nil
}

func (r *LogRotationPolicyRepository) Update(ctx context.Context, policy *entity.LogRotationPolicy) error {
	return r.db.WithContext(ctx).Save(policy).Error
}

func (r *LogRotationPolicyRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.LogRotationPolicy{}, id).Error
}

// LogRotationJobRepository handles log rotation job database operations.
type LogRotationJobRepository struct {
	db *gorm.DB
}

// NewLogRotationJobRepository creates a new LogRotationJobRepository.
func NewLogRotationJobRepository(db *gorm.DB) *LogRotationJobRepository {
	return &LogRotationJobRepository{db: db}
}

func (r *LogRotationJobRepository) Create(ctx context.Context, job *entity.LogRotationJob) error {
	return r.db.WithContext(ctx).Create(job).Error
}

func (r *LogRotationJobRepository) ListByPolicyID(ctx context.Context, policyID int64, limit, offset int) ([]entity.LogRotationJob, int64, error) {
	var jobs []entity.LogRotationJob
	var total int64

	q := r.db.WithContext(ctx).Where("policy_id = ?", policyID)
	q.Model(&entity.LogRotationJob{}).Count(&total)
	err := q.Order("id DESC").Limit(limit).Offset(offset).Find(&jobs).Error
	return jobs, total, err
}

func (r *LogRotationJobRepository) GetLatestByPolicy(ctx context.Context, policyID int64) (*entity.LogRotationJob, error) {
	var job entity.LogRotationJob
	err := r.db.WithContext(ctx).Where("policy_id = ?", policyID).Order("id DESC").First(&job).Error
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *LogRotationJobRepository) List(ctx context.Context, limit, offset int) ([]entity.LogRotationJob, int64, error) {
	var jobs []entity.LogRotationJob
	var total int64

	r.db.WithContext(ctx).Model(&entity.LogRotationJob{}).Count(&total)
	err := r.db.WithContext(ctx).Order("id DESC").Limit(limit).Offset(offset).Find(&jobs).Error
	return jobs, total, err
}
