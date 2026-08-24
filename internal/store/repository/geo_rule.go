package repository

import (
	"context"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// RuleSourceRepository handles rule source database operations.
type RuleSourceRepository struct {
	db *gorm.DB
}

// NewRuleSourceRepository creates a new RuleSourceRepository.
func NewRuleSourceRepository(db *gorm.DB) *RuleSourceRepository {
	return &RuleSourceRepository{db: db}
}

func (r *RuleSourceRepository) Create(ctx context.Context, src *entity.RuleSource) error {
	return r.db.WithContext(ctx).Create(src).Error
}

func (r *RuleSourceRepository) GetByID(ctx context.Context, id int64) (*entity.RuleSource, error) {
	var src entity.RuleSource
	err := r.db.WithContext(ctx).First(&src, id).Error
	if err != nil {
		return nil, err
	}
	return &src, nil
}

func (r *RuleSourceRepository) List(ctx context.Context) ([]entity.RuleSource, error) {
	var sources []entity.RuleSource
	err := r.db.WithContext(ctx).Order("id ASC").Find(&sources).Error
	return sources, err
}

// ListByServer lists rule sources for a specific node server.
// Since geo-rules are now global (server_id=0), this returns global sources.
func (r *RuleSourceRepository) ListByServer(ctx context.Context, serverID int64) ([]entity.RuleSource, error) {
	var sources []entity.RuleSource
	err := r.db.WithContext(ctx).Where("server_id = 0").Order("id ASC").Find(&sources).Error
	return sources, err
}

func (r *RuleSourceRepository) Update(ctx context.Context, src *entity.RuleSource) error {
	return r.db.WithContext(ctx).Save(src).Error
}

func (r *RuleSourceRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.RuleSource{}, id).Error
}

// GeoRuleRepository handles geo rule file database operations.
type GeoRuleRepository struct {
	db *gorm.DB
}

// NewGeoRuleRepository creates a new GeoRuleRepository.
func NewGeoRuleRepository(db *gorm.DB) *GeoRuleRepository {
	return &GeoRuleRepository{db: db}
}

func (r *GeoRuleRepository) Create(ctx context.Context, rule *entity.GeoRuleFile) error {
	return r.db.WithContext(ctx).Create(rule).Error
}

func (r *GeoRuleRepository) GetByID(ctx context.Context, id int64) (*entity.GeoRuleFile, error) {
	var rule entity.GeoRuleFile
	err := r.db.WithContext(ctx).First(&rule, id).Error
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

func (r *GeoRuleRepository) GetBySourceAndName(ctx context.Context, sourceID int64, name string) (*entity.GeoRuleFile, error) {
	var rule entity.GeoRuleFile
	err := r.db.WithContext(ctx).Where("source_id = ? AND name = ?", sourceID, name).First(&rule).Error
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

func (r *GeoRuleRepository) List(ctx context.Context) ([]entity.GeoRuleFile, error) {
	var rules []entity.GeoRuleFile
	err := r.db.WithContext(ctx).Order("source_id ASC, id ASC").Find(&rules).Error
	return rules, err
}

// ListByServer lists geo rule files for a specific node server.
// Since geo-rules are now global (server_id=0), this returns global files.
func (r *GeoRuleRepository) ListByServer(ctx context.Context, serverID int64) ([]entity.GeoRuleFile, error) {
	var rules []entity.GeoRuleFile
	err := r.db.WithContext(ctx).Where("server_id = 0").Order("source_id ASC, id ASC").Find(&rules).Error
	return rules, err
}

func (r *GeoRuleRepository) ListBySource(ctx context.Context, sourceID int64) ([]entity.GeoRuleFile, error) {
	var rules []entity.GeoRuleFile
	err := r.db.WithContext(ctx).Where("source_id = ?", sourceID).Order("id ASC").Find(&rules).Error
	return rules, err
}

func (r *GeoRuleRepository) ListAutoSync(ctx context.Context) ([]entity.GeoRuleFile, error) {
	var rules []entity.GeoRuleFile
	err := r.db.WithContext(ctx).Where("auto_sync = ?", true).Find(&rules).Error
	return rules, err
}

func (r *GeoRuleRepository) Update(ctx context.Context, rule *entity.GeoRuleFile) error {
	return r.db.WithContext(ctx).Save(rule).Error
}

func (r *GeoRuleRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.GeoRuleFile{}, id).Error
}

func (r *GeoRuleRepository) DeleteBySource(ctx context.Context, sourceID int64) error {
	return r.db.WithContext(ctx).Where("source_id = ?", sourceID).Delete(&entity.GeoRuleFile{}).Error
}
