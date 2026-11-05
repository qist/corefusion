package repository

import (
	"context"
	"encoding/json"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// MihomoConfigRepository handles mihomo config database operations.
type MihomoConfigRepository struct {
	db *gorm.DB
}

func NewMihomoConfigRepository(db *gorm.DB) *MihomoConfigRepository {
	return &MihomoConfigRepository{db: db}
}

func (r *MihomoConfigRepository) List(ctx context.Context) ([]entity.MihomoConfig, error) {
	return r.ListPaged(ctx, 0, 0)
}

// ListPaged returns mihomo configs with pagination (limit<=0 means no limit).
func (r *MihomoConfigRepository) ListPaged(ctx context.Context, limit, offset int) ([]entity.MihomoConfig, error) {
	var configs []entity.MihomoConfig
	q := r.db.WithContext(ctx).Order("sort_order ASC, id ASC")
	if limit > 0 {
		q = q.Limit(limit).Offset(offset)
	}
	err := q.Find(&configs).Error
	if err != nil {
		return nil, err
	}
	for i := range configs {
		configs[i].ConfigJSON = DecryptConfigJSON(configs[i].ConfigJSON)
	}
	// Manually load plan_ids for each config (compatibility with MySQL/SQLite)
	for i := range configs {
		var planIDsJSON *string
		r.db.WithContext(ctx).Raw("SELECT plan_ids FROM mihomo_configs WHERE id = ?", configs[i].ID).Scan(&planIDsJSON)
		if planIDsJSON != nil && *planIDsJSON != "" && *planIDsJSON != "null" {
			json.Unmarshal([]byte(*planIDsJSON), &configs[i].PlanIDs)
		}
	}
	return configs, nil
}

// Count returns the total number of mihomo configs.
func (r *MihomoConfigRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&entity.MihomoConfig{}).Count(&count).Error
	return count, err
}

func (r *MihomoConfigRepository) GetByID(ctx context.Context, id int64) (*entity.MihomoConfig, error) {
	var config entity.MihomoConfig
	err := r.db.WithContext(ctx).First(&config, id).Error
	config.ConfigJSON = DecryptConfigJSON(config.ConfigJSON)
	return &config, err
}

func (r *MihomoConfigRepository) Create(ctx context.Context, config *entity.MihomoConfig) error {
	config.ConfigJSON = EncryptConfigJSON(config.ConfigJSON)
	return r.db.WithContext(ctx).Create(config).Error
}

func (r *MihomoConfigRepository) Update(ctx context.Context, config *entity.MihomoConfig) error {
	config.ConfigJSON = EncryptConfigJSON(config.ConfigJSON)
	return r.db.WithContext(ctx).Save(config).Error
}

func (r *MihomoConfigRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete all related records
		tx.Where("config_id = ?", id).Delete(&entity.MihomoGeneral{})
		tx.Where("config_id = ?", id).Delete(&entity.MihomoDNS{})
		tx.Where("config_id = ?", id).Delete(&entity.MihomoHost{})
		tx.Where("config_id = ?", id).Delete(&entity.MihomoTunnel{})
		tx.Where("config_id = ?", id).Delete(&entity.MihomoSniffer{})
		tx.Where("config_id = ?", id).Delete(&entity.MihomoExternalController{})
		tx.Where("config_id = ?", id).Delete(&entity.MihomoTLS{})
		tx.Where("config_id = ?", id).Delete(&entity.MihomoTun{})
		tx.Where("config_id = ?", id).Delete(&entity.MihomoProxyGroup{})
		tx.Where("config_id = ?", id).Delete(&entity.MihomoRuleProvider{})
		tx.Where("config_id = ?", id).Delete(&entity.MihomoRule{})
		tx.Where("config_id = ?", id).Delete(&entity.MihomoSpecialOutbound{})
		return tx.Delete(&entity.MihomoConfig{}, id).Error
	})
}

// MihomoGeneralRepository handles mihomo general config.
type MihomoGeneralRepository struct {
	db *gorm.DB
}

func NewMihomoGeneralRepository(db *gorm.DB) *MihomoGeneralRepository {
	return &MihomoGeneralRepository{db: db}
}

func (r *MihomoGeneralRepository) GetByConfigID(ctx context.Context, configID int64) (*entity.MihomoGeneral, error) {
	var general entity.MihomoGeneral
	err := r.db.WithContext(ctx).Where("config_id = ?", configID).First(&general).Error
	// authentication 是 mihomo 的代理认证口令（明文 JSON 数组），加密落库
	general.Authentication = DecryptConfigJSON(general.Authentication)
	return &general, err
}

func (r *MihomoGeneralRepository) Create(ctx context.Context, general *entity.MihomoGeneral) error {
	general.Authentication = EncryptConfigField(general.Authentication)
	return r.db.WithContext(ctx).Create(general).Error
}

// MihomoDNSRepository handles mihomo DNS config.
type MihomoDNSRepository struct {
	db *gorm.DB
}

func NewMihomoDNSRepository(db *gorm.DB) *MihomoDNSRepository {
	return &MihomoDNSRepository{db: db}
}

func (r *MihomoDNSRepository) GetByConfigID(ctx context.Context, configID int64) (*entity.MihomoDNS, error) {
	var dns entity.MihomoDNS
	err := r.db.WithContext(ctx).Where("config_id = ?", configID).First(&dns).Error
	return &dns, err
}

func (r *MihomoDNSRepository) Create(ctx context.Context, dns *entity.MihomoDNS) error {
	return r.db.WithContext(ctx).Create(dns).Error
}

// MihomoHostRepository handles mihomo hosts.
type MihomoHostRepository struct {
	db *gorm.DB
}

func NewMihomoHostRepository(db *gorm.DB) *MihomoHostRepository {
	return &MihomoHostRepository{db: db}
}

func (r *MihomoHostRepository) ListByConfigID(ctx context.Context, configID int64) ([]entity.MihomoHost, error) {
	var hosts []entity.MihomoHost
	err := r.db.WithContext(ctx).Where("config_id = ?", configID).Order("id ASC").Find(&hosts).Error
	return hosts, err
}

// MihomoTunnelRepository handles mihomo tunnels.
type MihomoTunnelRepository struct {
	db *gorm.DB
}

func NewMihomoTunnelRepository(db *gorm.DB) *MihomoTunnelRepository {
	return &MihomoTunnelRepository{db: db}
}

func (r *MihomoTunnelRepository) ListByConfigID(ctx context.Context, configID int64) ([]entity.MihomoTunnel, error) {
	var tunnels []entity.MihomoTunnel
	err := r.db.WithContext(ctx).Where("config_id = ?", configID).Order("id ASC").Find(&tunnels).Error
	return tunnels, err
}

// MihomoSnifferRepository handles mihomo sniffer config.
type MihomoSnifferRepository struct {
	db *gorm.DB
}

func NewMihomoSnifferRepository(db *gorm.DB) *MihomoSnifferRepository {
	return &MihomoSnifferRepository{db: db}
}

func (r *MihomoSnifferRepository) GetByConfigID(ctx context.Context, configID int64) (*entity.MihomoSniffer, error) {
	var sniffer entity.MihomoSniffer
	err := r.db.WithContext(ctx).Where("config_id = ?", configID).First(&sniffer).Error
	return &sniffer, err
}

// MihomoExternalControllerRepository handles mihomo external controller config.
type MihomoExternalControllerRepository struct {
	db *gorm.DB
}

func NewMihomoExternalControllerRepository(db *gorm.DB) *MihomoExternalControllerRepository {
	return &MihomoExternalControllerRepository{db: db}
}

func (r *MihomoExternalControllerRepository) GetByConfigID(ctx context.Context, configID int64) (*entity.MihomoExternalController, error) {
	var ctrl entity.MihomoExternalController
	err := r.db.WithContext(ctx).Where("config_id = ?", configID).First(&ctrl).Error
	return &ctrl, err
}

// MihomoTLSRepository handles mihomo TLS config.
type MihomoTLSRepository struct {
	db *gorm.DB
}

func NewMihomoTLSRepository(db *gorm.DB) *MihomoTLSRepository {
	return &MihomoTLSRepository{db: db}
}

func (r *MihomoTLSRepository) GetByConfigID(ctx context.Context, configID int64) (*entity.MihomoTLS, error) {
	var tls entity.MihomoTLS
	err := r.db.WithContext(ctx).Where("config_id = ?", configID).First(&tls).Error
	tls.PrivateKey = DecryptConfigJSON(tls.PrivateKey)
	return &tls, err
}

// MihomoTunRepository handles mihomo TUN config.
type MihomoTunRepository struct {
	db *gorm.DB
}

func NewMihomoTunRepository(db *gorm.DB) *MihomoTunRepository {
	return &MihomoTunRepository{db: db}
}

func (r *MihomoTunRepository) GetByConfigID(ctx context.Context, configID int64) (*entity.MihomoTun, error) {
	var tun entity.MihomoTun
	err := r.db.WithContext(ctx).Where("config_id = ?", configID).First(&tun).Error
	return &tun, err
}

// MihomoProxyGroupRepository handles mihomo proxy groups.
type MihomoProxyGroupRepository struct {
	db *gorm.DB
}

func NewMihomoProxyGroupRepository(db *gorm.DB) *MihomoProxyGroupRepository {
	return &MihomoProxyGroupRepository{db: db}
}

func (r *MihomoProxyGroupRepository) ListByConfigID(ctx context.Context, configID int64) ([]entity.MihomoProxyGroup, error) {
	var groups []entity.MihomoProxyGroup
	err := r.db.WithContext(ctx).Where("config_id = ?", configID).Order("id ASC").Find(&groups).Error
	return groups, err
}

// MihomoRuleProviderRepository handles mihomo rule providers.
type MihomoRuleProviderRepository struct {
	db *gorm.DB
}

func NewMihomoRuleProviderRepository(db *gorm.DB) *MihomoRuleProviderRepository {
	return &MihomoRuleProviderRepository{db: db}
}

func (r *MihomoRuleProviderRepository) ListByConfigID(ctx context.Context, configID int64) ([]entity.MihomoRuleProvider, error) {
	var providers []entity.MihomoRuleProvider
	err := r.db.WithContext(ctx).Where("config_id = ?", configID).Order("id ASC").Find(&providers).Error
	return providers, err
}

// MihomoRuleRepository handles mihomo rules.
type MihomoRuleRepository struct {
	db *gorm.DB
}

func NewMihomoRuleRepository(db *gorm.DB) *MihomoRuleRepository {
	return &MihomoRuleRepository{db: db}
}

func (r *MihomoRuleRepository) ListByConfigID(ctx context.Context, configID int64) ([]entity.MihomoRule, error) {
	var rules []entity.MihomoRule
	err := r.db.WithContext(ctx).Where("config_id = ?", configID).Order("priority ASC, id ASC").Find(&rules).Error
	return rules, err
}

func (r *MihomoRuleRepository) GetByID(ctx context.Context, id int64) (*entity.MihomoRule, error) {
	var rule entity.MihomoRule
	if err := r.db.WithContext(ctx).First(&rule, id).Error; err != nil {
		return nil, err
	}
	return &rule, nil
}

func (r *MihomoRuleRepository) Create(ctx context.Context, rule *entity.MihomoRule) error {
	return r.db.WithContext(ctx).Create(rule).Error
}

func (r *MihomoRuleRepository) Update(ctx context.Context, rule *entity.MihomoRule) error {
	return r.db.WithContext(ctx).Save(rule).Error
}

func (r *MihomoRuleRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.MihomoRule{}, id).Error
}

// Reorder 按传入的 id 顺序重排该配置下的规则优先级（priority 递增，留出插入空间）。
// 只更新属于 configID 的行，避免跨配置改动。
func (r *MihomoRuleRepository) Reorder(ctx context.Context, configID int64, ids []int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, id := range ids {
			res := tx.Model(&entity.MihomoRule{}).
				Where("id = ? AND config_id = ?", id, configID).
				Update("priority", (i+1)*10)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return gorm.ErrRecordNotFound
			}
		}
		return nil
	})
}

// MihomoSpecialOutboundRepository handles mihomo special outbounds.
type MihomoSpecialOutboundRepository struct {
	db *gorm.DB
}

func NewMihomoSpecialOutboundRepository(db *gorm.DB) *MihomoSpecialOutboundRepository {
	return &MihomoSpecialOutboundRepository{db: db}
}

func (r *MihomoSpecialOutboundRepository) ListByConfigID(ctx context.Context, configID int64) ([]entity.MihomoSpecialOutbound, error) {
	var outbounds []entity.MihomoSpecialOutbound
	err := r.db.WithContext(ctx).Where("config_id = ?", configID).Order("id ASC").Find(&outbounds).Error
	return outbounds, err
}
