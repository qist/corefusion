package repository

import (
	"context"

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
	var configs []entity.MihomoConfig
	err := r.db.WithContext(ctx).Order("sort_order ASC, id ASC").Find(&configs).Error
	return configs, err
}

func (r *MihomoConfigRepository) GetByID(ctx context.Context, id int64) (*entity.MihomoConfig, error) {
	var config entity.MihomoConfig
	err := r.db.WithContext(ctx).First(&config, id).Error
	return &config, err
}

func (r *MihomoConfigRepository) Create(ctx context.Context, config *entity.MihomoConfig) error {
	return r.db.WithContext(ctx).Create(config).Error
}

func (r *MihomoConfigRepository) Update(ctx context.Context, config *entity.MihomoConfig) error {
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
	return &general, err
}

func (r *MihomoGeneralRepository) Create(ctx context.Context, general *entity.MihomoGeneral) error {
	return r.db.WithContext(ctx).Create(general).Error
}

func (r *MihomoGeneralRepository) Update(ctx context.Context, general *entity.MihomoGeneral) error {
	return r.db.WithContext(ctx).Save(general).Error
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

func (r *MihomoDNSRepository) Update(ctx context.Context, dns *entity.MihomoDNS) error {
	return r.db.WithContext(ctx).Save(dns).Error
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

func (r *MihomoHostRepository) Create(ctx context.Context, host *entity.MihomoHost) error {
	return r.db.WithContext(ctx).Create(host).Error
}

func (r *MihomoHostRepository) Update(ctx context.Context, host *entity.MihomoHost) error {
	return r.db.WithContext(ctx).Save(host).Error
}

func (r *MihomoHostRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.MihomoHost{}, id).Error
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

func (r *MihomoTunnelRepository) Create(ctx context.Context, tunnel *entity.MihomoTunnel) error {
	return r.db.WithContext(ctx).Create(tunnel).Error
}

func (r *MihomoTunnelRepository) Update(ctx context.Context, tunnel *entity.MihomoTunnel) error {
	return r.db.WithContext(ctx).Save(tunnel).Error
}

func (r *MihomoTunnelRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.MihomoTunnel{}, id).Error
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

func (r *MihomoSnifferRepository) Create(ctx context.Context, sniffer *entity.MihomoSniffer) error {
	return r.db.WithContext(ctx).Create(sniffer).Error
}

func (r *MihomoSnifferRepository) Update(ctx context.Context, sniffer *entity.MihomoSniffer) error {
	return r.db.WithContext(ctx).Save(sniffer).Error
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

func (r *MihomoExternalControllerRepository) Create(ctx context.Context, ctrl *entity.MihomoExternalController) error {
	return r.db.WithContext(ctx).Create(ctrl).Error
}

func (r *MihomoExternalControllerRepository) Update(ctx context.Context, ctrl *entity.MihomoExternalController) error {
	return r.db.WithContext(ctx).Save(ctrl).Error
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
	return &tls, err
}

func (r *MihomoTLSRepository) Create(ctx context.Context, tls *entity.MihomoTLS) error {
	return r.db.WithContext(ctx).Create(tls).Error
}

func (r *MihomoTLSRepository) Update(ctx context.Context, tls *entity.MihomoTLS) error {
	return r.db.WithContext(ctx).Save(tls).Error
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

func (r *MihomoTunRepository) Create(ctx context.Context, tun *entity.MihomoTun) error {
	return r.db.WithContext(ctx).Create(tun).Error
}

func (r *MihomoTunRepository) Update(ctx context.Context, tun *entity.MihomoTun) error {
	return r.db.WithContext(ctx).Save(tun).Error
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

func (r *MihomoProxyGroupRepository) Create(ctx context.Context, group *entity.MihomoProxyGroup) error {
	return r.db.WithContext(ctx).Create(group).Error
}

func (r *MihomoProxyGroupRepository) Update(ctx context.Context, group *entity.MihomoProxyGroup) error {
	return r.db.WithContext(ctx).Save(group).Error
}

func (r *MihomoProxyGroupRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.MihomoProxyGroup{}, id).Error
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

func (r *MihomoRuleProviderRepository) Create(ctx context.Context, provider *entity.MihomoRuleProvider) error {
	return r.db.WithContext(ctx).Create(provider).Error
}

func (r *MihomoRuleProviderRepository) Update(ctx context.Context, provider *entity.MihomoRuleProvider) error {
	return r.db.WithContext(ctx).Save(provider).Error
}

func (r *MihomoRuleProviderRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.MihomoRuleProvider{}, id).Error
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

func (r *MihomoRuleRepository) Create(ctx context.Context, rule *entity.MihomoRule) error {
	return r.db.WithContext(ctx).Create(rule).Error
}

func (r *MihomoRuleRepository) Update(ctx context.Context, rule *entity.MihomoRule) error {
	return r.db.WithContext(ctx).Save(rule).Error
}

func (r *MihomoRuleRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.MihomoRule{}, id).Error
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

func (r *MihomoSpecialOutboundRepository) Create(ctx context.Context, outbound *entity.MihomoSpecialOutbound) error {
	return r.db.WithContext(ctx).Create(outbound).Error
}

func (r *MihomoSpecialOutboundRepository) Update(ctx context.Context, outbound *entity.MihomoSpecialOutbound) error {
	return r.db.WithContext(ctx).Save(outbound).Error
}

func (r *MihomoSpecialOutboundRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.MihomoSpecialOutbound{}, id).Error
}
