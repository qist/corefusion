package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"corefusion/internal/model/entity"
)

// WafRuleRepository handles waf_rules database operations.
type WafRuleRepository struct {
	db *gorm.DB
}

func NewWafRuleRepository(db *gorm.DB) *WafRuleRepository {
	return &WafRuleRepository{db: db}
}

// List returns all WAF rules (all scopes).
func (r *WafRuleRepository) List(ctx context.Context) ([]entity.WafRule, error) {
	var rules []entity.WafRule
	err := r.db.WithContext(ctx).Order("rule_scope ASC, name ASC").Find(&rules).Error
	return rules, err
}

// ListGlobal returns all global-scope WAF rules.
func (r *WafRuleRepository) ListGlobal(ctx context.Context) ([]entity.WafRule, error) {
	var rules []entity.WafRule
	err := r.db.WithContext(ctx).Where("rule_scope = ?", "global").Order("name ASC").Find(&rules).Error
	return rules, err
}

// ListByServer returns all WAF rules for a specific server (global + node-scoped).
// Does NOT include domain-scoped rules.
func (r *WafRuleRepository) ListByServer(ctx context.Context, serverID int64) ([]entity.WafRule, error) {
	var rules []entity.WafRule
	err := r.db.WithContext(ctx).
		Where("rule_scope = ? OR (rule_scope = ? AND server_id = ?)", "global", "node", serverID).
		Order("rule_scope ASC, name ASC").
		Find(&rules).Error
	return rules, err
}

// ListDomainRules returns all domain-scoped rules for a specific server+domain.
func (r *WafRuleRepository) ListDomainRules(ctx context.Context, serverID int64, domain string) ([]entity.WafRule, error) {
	var rules []entity.WafRule
	err := r.db.WithContext(ctx).
		Where("rule_scope = ? AND server_id = ? AND domain = ?", "domain", serverID, domain).
		Order("name ASC").
		Find(&rules).Error
	return rules, err
}

// ListAllDomainRules returns all domain-scoped rules for a specific server (all domains).
func (r *WafRuleRepository) ListAllDomainRules(ctx context.Context, serverID int64) ([]entity.WafRule, error) {
	var rules []entity.WafRule
	err := r.db.WithContext(ctx).
		Where("rule_scope = ? AND server_id = ?", "domain", serverID).
		Order("domain ASC, name ASC").
		Find(&rules).Error
	return rules, err
}

// ListNodeRules returns all node-scoped rules for a specific server.
func (r *WafRuleRepository) ListNodeRules(ctx context.Context, serverID int64) ([]entity.WafRule, error) {
	var rules []entity.WafRule
	err := r.db.WithContext(ctx).
		Where("rule_scope = ? AND server_id = ?", "node", serverID).
		Order("name ASC").
		Find(&rules).Error
	return rules, err
}

// GetGlobalByName returns a global-scope WAF rule by name.
func (r *WafRuleRepository) GetGlobalByName(ctx context.Context, name string) (*entity.WafRule, error) {
	var rule entity.WafRule
	err := r.db.WithContext(ctx).Where("name = ? AND rule_scope = ?", name, "global").First(&rule).Error
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

// GetNodeByName returns a node-scoped WAF rule by name and server_id.
func (r *WafRuleRepository) GetNodeByName(ctx context.Context, name string, serverID int64) (*entity.WafRule, error) {
	var rule entity.WafRule
	err := r.db.WithContext(ctx).Where("name = ? AND rule_scope = ? AND server_id = ?", name, "node", serverID).First(&rule).Error
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

// GetDomainByName returns a domain-scoped WAF rule by name, server_id and domain.
func (r *WafRuleRepository) GetDomainByName(ctx context.Context, name string, serverID int64, domain string) (*entity.WafRule, error) {
	var rule entity.WafRule
	err := r.db.WithContext(ctx).
		Where("name = ? AND rule_scope = ? AND server_id = ? AND domain = ?", name, "domain", serverID, domain).
		First(&rule).Error
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

// Upsert creates or updates a WAF rule.
// Unique key: (name, rule_scope, server_id, domain)
func (r *WafRuleRepository) Upsert(ctx context.Context, rule *entity.WafRule) error {
	if rule.RuleScope == "" {
		rule.RuleScope = "global"
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}, {Name: "rule_scope"}, {Name: "server_id"}, {Name: "domain"}},
		DoUpdates: clause.AssignmentColumns([]string{"content", "hash", "merge_mode", "enabled", "updated_at"}),
	}).Create(rule).Error
}

// Delete removes a WAF rule by name and scope.
// For global rules: deletes where name=? AND rule_scope='global'
// For node rules: deletes where name=? AND rule_scope='node' AND server_id=?
// For domain rules: deletes where name=? AND rule_scope='domain' AND server_id=? AND domain=?
func (r *WafRuleRepository) Delete(ctx context.Context, name string, serverID int64, domain string) error {
	if serverID == 0 && domain == "" {
		return r.db.WithContext(ctx).Where("name = ? AND rule_scope = ?", name, "global").Delete(&entity.WafRule{}).Error
	}
	if domain == "" {
		return r.db.WithContext(ctx).Where("name = ? AND rule_scope = ? AND server_id = ?", name, "node", serverID).Delete(&entity.WafRule{}).Error
	}
	return r.db.WithContext(ctx).
		Where("name = ? AND rule_scope = ? AND server_id = ? AND domain = ?", name, "domain", serverID, domain).
		Delete(&entity.WafRule{}).Error
}

// MigrateNodeReplaceToAppend flips legacy node-scoped rules whose merge_mode is
// "replace" (or empty) to "append". Node rules are meant to supplement the global
// rule of the same name; "replace" silently drops the global content.
func (r *WafRuleRepository) MigrateNodeReplaceToAppend(ctx context.Context) (int64, error) {
	res := r.db.WithContext(ctx).
		Model(&entity.WafRule{}).
		Where("rule_scope = ? AND (merge_mode = ? OR merge_mode = ? OR merge_mode IS NULL)", "node", "replace", "").
		Update("merge_mode", "append")
	return res.RowsAffected, res.Error
}

// Count returns the total number of global WAF rules.
func (r *WafRuleRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&entity.WafRule{}).Where("rule_scope = ?", "global").Count(&count).Error
	return count, err
}

// CountGlobal returns the total number of global-scope WAF rules.
func (r *WafRuleRepository) CountGlobal(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&entity.WafRule{}).Where("rule_scope = ?", "global").Count(&count).Error
	return count, err
}

// WafConfigRepository handles waf_configs database operations.
type WafConfigRepository struct {
	db *gorm.DB
}

func NewWafConfigRepository(db *gorm.DB) *WafConfigRepository {
	return &WafConfigRepository{db: db}
}

// Get returns the WAF config for a server and proxy type.
// server_id=0 means global default.
func (r *WafConfigRepository) Get(ctx context.Context, serverID int64, proxyType string) (*entity.WafConfig, error) {
	var cfg entity.WafConfig
	err := r.db.WithContext(ctx).Where("server_id = ? AND proxy_type = ?", serverID, proxyType).First(&cfg).Error
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Upsert creates or updates a WAF config.
func (r *WafConfigRepository) Upsert(ctx context.Context, cfg *entity.WafConfig) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "server_id"}, {Name: "proxy_type"}},
		DoUpdates: clause.AssignmentColumns([]string{"content", "hash", "updated_at"}),
	}).Create(cfg).Error
}

// WafDomainConfigRepository handles waf_domain_configs database operations.
type WafDomainConfigRepository struct {
	db *gorm.DB
}

func NewWafDomainConfigRepository(db *gorm.DB) *WafDomainConfigRepository {
	return &WafDomainConfigRepository{db: db}
}

// ListByServer returns all domain WAF configs for a server.
func (r *WafDomainConfigRepository) ListByServer(ctx context.Context, serverID int64) ([]entity.WafDomainConfig, error) {
	var configs []entity.WafDomainConfig
	err := r.db.WithContext(ctx).Where("server_id = ?", serverID).Order("domain ASC").Find(&configs).Error
	return configs, err
}

// GetByDomain returns the WAF config for a specific domain on a server.
func (r *WafDomainConfigRepository) GetByDomain(ctx context.Context, serverID int64, domain string) (*entity.WafDomainConfig, error) {
	var cfg entity.WafDomainConfig
	err := r.db.WithContext(ctx).Where("server_id = ? AND domain = ?", serverID, domain).First(&cfg).Error
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Upsert creates or updates a domain WAF config.
func (r *WafDomainConfigRepository) Upsert(ctx context.Context, cfg *entity.WafDomainConfig) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "server_id"}, {Name: "domain"}},
		DoUpdates: clause.AssignmentColumns([]string{"config", "updated_at"}),
	}).Create(cfg).Error
}

// Delete removes a domain WAF config.
func (r *WafDomainConfigRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.WafDomainConfig{}, id).Error
}

// DeleteByServerAndDomain removes a domain WAF config by server_id and domain.
func (r *WafDomainConfigRepository) DeleteByServerAndDomain(ctx context.Context, serverID int64, domain string) error {
	return r.db.WithContext(ctx).Where("server_id = ? AND domain = ?", serverID, domain).Delete(&entity.WafDomainConfig{}).Error
}

// DeleteByServer removes all domain WAF configs for a server.
func (r *WafDomainConfigRepository) DeleteByServer(ctx context.Context, serverID int64) error {
	return r.db.WithContext(ctx).Where("server_id = ?", serverID).Delete(&entity.WafDomainConfig{}).Error
}
