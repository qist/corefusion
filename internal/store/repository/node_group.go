package repository

import (
	"context"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// NodeGroupRepository handles node_groups database operations.
type NodeGroupRepository struct {
	db *gorm.DB
}

func NewNodeGroupRepository(db *gorm.DB) *NodeGroupRepository {
	return &NodeGroupRepository{db: db}
}

func (r *NodeGroupRepository) List(ctx context.Context) ([]entity.NodeGroup, error) {
	var items []entity.NodeGroup
	err := r.db.WithContext(ctx).Order("sort_order ASC, id ASC").Find(&items).Error
	return items, err
}

func (r *NodeGroupRepository) ListByTier(ctx context.Context, tier string) ([]entity.NodeGroup, error) {
	var items []entity.NodeGroup
	err := r.db.WithContext(ctx).
		Where("tier = ? AND enabled = ?", tier, true).
		Order("sort_order ASC, id ASC").
		Find(&items).Error
	return items, err
}

func (r *NodeGroupRepository) GetByID(ctx context.Context, id int64) (*entity.NodeGroup, error) {
	var item entity.NodeGroup
	err := r.db.WithContext(ctx).First(&item, id).Error
	return &item, err
}

func (r *NodeGroupRepository) Create(ctx context.Context, item *entity.NodeGroup) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *NodeGroupRepository) Update(ctx context.Context, item *entity.NodeGroup) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *NodeGroupRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("group_id = ?", id).Delete(&entity.NodeGroupMember{}).Error; err != nil {
			return err
		}
		return tx.Delete(&entity.NodeGroup{}, id).Error
	})
}

// NodeGroupMemberRepository handles node_group_members database operations.
type NodeGroupMemberRepository struct {
	db *gorm.DB
}

func NewNodeGroupMemberRepository(db *gorm.DB) *NodeGroupMemberRepository {
	return &NodeGroupMemberRepository{db: db}
}

func (r *NodeGroupMemberRepository) ListByGroupID(ctx context.Context, groupID int64) ([]entity.NodeGroupMember, error) {
	var items []entity.NodeGroupMember
	err := r.db.WithContext(ctx).
		Where("group_id = ?", groupID).
		Order("id ASC").
		Find(&items).Error
	return items, err
}

func (r *NodeGroupMemberRepository) ListByGroupIDs(ctx context.Context, groupIDs []int64) ([]entity.NodeGroupMember, error) {
	var items []entity.NodeGroupMember
	if len(groupIDs) == 0 {
		return items, nil
	}
	err := r.db.WithContext(ctx).
		Where("group_id IN ? AND enabled = ?", groupIDs, true).
		Order("group_id ASC, id ASC").
		Find(&items).Error
	return items, err
}

func (r *NodeGroupMemberRepository) Create(ctx context.Context, item *entity.NodeGroupMember) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *NodeGroupMemberRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.NodeGroupMember{}, id).Error
}
