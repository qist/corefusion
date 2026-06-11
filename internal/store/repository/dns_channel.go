package repository

import (
	"context"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// DNSChannelRepository handles DNS channel database operations.
type DNSChannelRepository struct {
	db *gorm.DB
}

// NewDNSChannelRepository creates a new DNSChannelRepository.
func NewDNSChannelRepository(db *gorm.DB) *DNSChannelRepository {
	return &DNSChannelRepository{db: db}
}

func (r *DNSChannelRepository) Create(ctx context.Context, ch *entity.DNSChannel) error {
	return r.db.WithContext(ctx).Create(ch).Error
}

func (r *DNSChannelRepository) GetByID(ctx context.Context, id int64) (*entity.DNSChannel, error) {
	var ch entity.DNSChannel
	err := r.db.WithContext(ctx).First(&ch, id).Error
	if err != nil {
		return nil, err
	}
	return &ch, nil
}

func (r *DNSChannelRepository) List(ctx context.Context) ([]entity.DNSChannel, error) {
	var list []entity.DNSChannel
	err := r.db.WithContext(ctx).Order("id ASC").Find(&list).Error
	return list, err
}

func (r *DNSChannelRepository) Update(ctx context.Context, ch *entity.DNSChannel) error {
	return r.db.WithContext(ctx).Save(ch).Error
}

func (r *DNSChannelRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.DNSChannel{}, id).Error
}
