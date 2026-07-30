package access

import (
	"context"
	"time"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// gormBanRepository implements BanRepository using GORM over the service_ip_bans table.
type gormBanRepository struct {
	db *gorm.DB
}

// NewBanRepository creates a GORM-backed BanRepository.
func NewBanRepository(db *gorm.DB) BanRepository {
	return &gormBanRepository{db: db}
}

// Create persists a ban record.
func (r *gormBanRepository) Create(ctx context.Context, ban *BanRecord) error {
	if ban == nil {
		return nil
	}
	row := &entity.ServiceIPBan{
		ServiceID:  ban.ServiceID,
		EndpointID: endpointIDOrNil(ban.EndpointID),
		IPCIDR:     ban.IPCIDR,
		Reason:     ban.Reason,
		Source:     ban.Source,
		ExpiresAt:  &ban.ExpiresAt,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	return r.db.WithContext(ctx).Create(row).Error
}

// FindActive returns the most specific active ban for the given service/endpoint/IP.
func (r *gormBanRepository) FindActive(ctx context.Context, serviceID, endpointID int64, ip string) *BanRecord {
	var row entity.ServiceIPBan
	err := r.db.WithContext(ctx).
		Where("service_id = ?", serviceID).
		Where("expires_at IS NULL OR expires_at > ?", time.Now()).
		Where("ip_cidr = ? OR ip_cidr = ?", ip+"/32", ip).
		Where("endpoint_id = ? OR endpoint_id IS NULL", endpointID).
		First(&row).Error
	if err != nil {
		return nil
	}
	var expires time.Time
	if row.ExpiresAt != nil {
		expires = *row.ExpiresAt
	}
	return &BanRecord{
		ID:         row.ID,
		ServiceID:  row.ServiceID,
		EndpointID: derefInt64(row.EndpointID),
		IPCIDR:     row.IPCIDR,
		Reason:     row.Reason,
		Source:     row.Source,
		ExpiresAt:  expires,
	}
}

// DeleteExpired removes all bans whose expiry has passed.
func (r *gormBanRepository) DeleteExpired(ctx context.Context) (int64, error) {
	res := r.db.WithContext(ctx).
		Where("expires_at IS NOT NULL AND expires_at <= ?", time.Now()).
		Delete(&entity.ServiceIPBan{})
	return res.RowsAffected, res.Error
}

// PolicyRepository loads access policies for connection checks.
type PolicyRepository interface {
	ListActive(ctx context.Context, serviceID, endpointID int64) ([]entity.ServiceAccessPolicy, error)
}

type gormPolicyRepository struct {
	db *gorm.DB
}

// NewPolicyRepository creates a GORM-backed PolicyRepository.
func NewPolicyRepository(db *gorm.DB) PolicyRepository {
	return &gormPolicyRepository{db: db}
}

// ListActive returns enabled policies for the service, matching the given endpoint or
// service-wide (endpoint_id IS NULL).
func (r *gormPolicyRepository) ListActive(ctx context.Context, serviceID, endpointID int64) ([]entity.ServiceAccessPolicy, error) {
	var policies []entity.ServiceAccessPolicy
	err := r.db.WithContext(ctx).
		Where("service_id = ? AND status = 1", serviceID).
		Where("endpoint_id = ? OR endpoint_id IS NULL", endpointID).
		Find(&policies).Error
	return policies, err
}

// endpointIDOrNil converts an int64 endpoint id to a *int64, mapping 0 to NULL so that
// a zero value is stored as a service-wide (global) policy rather than endpoint 0.
func endpointIDOrNil(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return &v
}

// derefInt64 returns the value of a *int64 or 0 if nil.
func derefInt64(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}
