package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// ChainLinkRepository handles chain_links database operations.
type ChainLinkRepository struct {
	db *gorm.DB
}

func NewChainLinkRepository(db *gorm.DB) *ChainLinkRepository {
	return &ChainLinkRepository{db: db}
}

// ListByEntry returns all chain links for a given entry (server + optional inbound).
// If inboundID is 0, returns all links for the server (including global ones with entry_inbound_id=0).
// If inboundID > 0, returns both global links (entry_inbound_id=0) and inbound-specific links.
func (r *ChainLinkRepository) ListByEntry(ctx context.Context, serverID int64, inboundID int64) ([]entity.ChainLink, error) {
	var links []entity.ChainLink
	q := r.db.WithContext(ctx).Where("entry_server_id = ?", serverID)
	if inboundID > 0 {
		q = q.Where("entry_inbound_id = 0 OR entry_inbound_id = ?", inboundID)
	}
	err := q.Where("enabled = ?", true).
		Order("priority ASC, sort_order ASC, id ASC").Find(&links).Error
	return links, err
}

// ListAllByEntry returns all chain links for a given entry server (including disabled).
func (r *ChainLinkRepository) ListAllByEntry(ctx context.Context, serverID int64) ([]entity.ChainLink, error) {
	var links []entity.ChainLink
	err := r.db.WithContext(ctx).
		Where("entry_server_id = ?", serverID).
		Order("priority ASC, sort_order ASC, id ASC").Find(&links).Error
	return links, err
}

// ListByExit returns all chain links pointing to a given exit server.
func (r *ChainLinkRepository) ListByExit(ctx context.Context, serverID int64) ([]entity.ChainLink, error) {
	var links []entity.ChainLink
	err := r.db.WithContext(ctx).
		Where("exit_server_id = ?", serverID).
		Order("priority ASC, id ASC").Find(&links).Error
	return links, err
}

// ListAll returns all chain links.
func (r *ChainLinkRepository) ListAll(ctx context.Context) ([]entity.ChainLink, error) {
	var links []entity.ChainLink
	err := r.db.WithContext(ctx).
		Order("priority ASC, sort_order ASC, id ASC").Find(&links).Error
	return links, err
}

// GetByID retrieves a chain link by ID.
func (r *ChainLinkRepository) GetByID(ctx context.Context, id int64) (*entity.ChainLink, error) {
	var link entity.ChainLink
	err := r.db.WithContext(ctx).First(&link, id).Error
	if err != nil {
		return nil, err
	}
	return &link, nil
}

// Create creates a new chain link.
func (r *ChainLinkRepository) Create(ctx context.Context, link *entity.ChainLink) error {
	link.CreatedAt = time.Now()
	link.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Create(link).Error
}

// Update updates a chain link.
func (r *ChainLinkRepository) Update(ctx context.Context, link *entity.ChainLink) error {
	link.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(link).Error
}

// Delete deletes a chain link.
func (r *ChainLinkRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.ChainLink{}, id).Error
}

// DeleteByEntry deletes all chain links for an entry server.
func (r *ChainLinkRepository) DeleteByEntry(ctx context.Context, serverID int64) error {
	return r.db.WithContext(ctx).
		Where("entry_server_id = ?", serverID).
		Delete(&entity.ChainLink{}).Error
}

// DeleteByExit deletes all chain links pointing to an exit server.
func (r *ChainLinkRepository) DeleteByExit(ctx context.Context, serverID int64) error {
	return r.db.WithContext(ctx).
		Where("exit_server_id = ?", serverID).
		Delete(&entity.ChainLink{}).Error
}
