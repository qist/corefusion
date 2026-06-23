package repository

import (
	"context"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// CFIPSourceRepository handles CF IP source database operations.
type CFIPSourceRepository struct {
	db *gorm.DB
}

// NewCFIPSourceRepository creates a new CF IP source repository.
func NewCFIPSourceRepository(db *gorm.DB) *CFIPSourceRepository {
	return &CFIPSourceRepository{db: db}
}

// List returns all CF IP sources.
func (r *CFIPSourceRepository) List(ctx context.Context) ([]entity.CFIPSource, error) {
	var sources []entity.CFIPSource
	err := r.db.WithContext(ctx).Order("id ASC").Find(&sources).Error
	return sources, err
}

// GetByID returns a CF IP source by ID.
func (r *CFIPSourceRepository) GetByID(ctx context.Context, id int64) (*entity.CFIPSource, error) {
	var source entity.CFIPSource
	err := r.db.WithContext(ctx).First(&source, id).Error
	return &source, err
}

// Create creates a new CF IP source.
func (r *CFIPSourceRepository) Create(ctx context.Context, source *entity.CFIPSource) error {
	return r.db.WithContext(ctx).Create(source).Error
}

// Update updates an existing CF IP source.
func (r *CFIPSourceRepository) Update(ctx context.Context, source *entity.CFIPSource) error {
	return r.db.WithContext(ctx).Save(source).Error
}

// Delete deletes a CF IP source by ID.
func (r *CFIPSourceRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.CFIPSource{}, id).Error
}

// GetEnabled returns all enabled CF IP sources.
func (r *CFIPSourceRepository) GetEnabled(ctx context.Context) ([]entity.CFIPSource, error) {
	var sources []entity.CFIPSource
	err := r.db.WithContext(ctx).Where("enabled = ?", true).Find(&sources).Error
	return sources, err
}

// CFIPEntryRepository handles CF IP entry database operations.
type CFIPEntryRepository struct {
	db *gorm.DB
}

// NewCFIPEntryRepository creates a new CF IP entry repository.
func NewCFIPEntryRepository(db *gorm.DB) *CFIPEntryRepository {
	return &CFIPEntryRepository{db: db}
}

// List returns all CF IP entries, optionally filtered by source.
func (r *CFIPEntryRepository) List(ctx context.Context, sourceID int64) ([]entity.CFIPEntry, error) {
	var entries []entity.CFIPEntry
	query := r.db.WithContext(ctx).Order("id ASC")
	if sourceID > 0 {
		query = query.Where("source_id = ?", sourceID)
	}
	err := query.Find(&entries).Error
	return entries, err
}

// Create creates a new CF IP entry.
func (r *CFIPEntryRepository) Create(ctx context.Context, entry *entity.CFIPEntry) error {
	return r.db.WithContext(ctx).Create(entry).Error
}

// Delete deletes a CF IP entry by ID.
func (r *CFIPEntryRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.CFIPEntry{}, id).Error
}

// DeleteBySource deletes all CF IP entries for a source.
func (r *CFIPEntryRepository) DeleteBySource(ctx context.Context, sourceID int64) error {
	return r.db.WithContext(ctx).Where("source_id = ?", sourceID).Delete(&entity.CFIPEntry{}).Error
}

// ClearAll deletes all CF IP entries.
func (r *CFIPEntryRepository) ClearAll(ctx context.Context) error {
	return r.db.WithContext(ctx).Where("1 = 1").Delete(&entity.CFIPEntry{}).Error
}
