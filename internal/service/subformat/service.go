package subformat

import (
	"context"
	"log"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// Service handles subscription format business logic.
type Service struct {
	formatRepo *repository.SubscriptionFormatRepository
	ruleRepo   *repository.SubscriptionRuleRepository
}

// NewService creates a new subscription format service.
func NewService(formatRepo *repository.SubscriptionFormatRepository, ruleRepo *repository.SubscriptionRuleRepository) *Service {
	return &Service{
		formatRepo: formatRepo,
		ruleRepo:   ruleRepo,
	}
}

// List returns all subscription formats.
func (s *Service) List(ctx context.Context) ([]entity.SubscriptionFormat, error) {
	return s.formatRepo.List(ctx)
}

// GetByID returns a subscription format by ID with its rules.
func (s *Service) GetByID(ctx context.Context, id int64) (*entity.SubscriptionFormat, []entity.SubscriptionRule, error) {
	format, err := s.formatRepo.GetByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	rules, err := s.ruleRepo.ListByFormatID(ctx, id)
	if err != nil {
		return format, nil, err
	}
	return format, rules, nil
}

// CreateInput is the input for creating a subscription format.
type CreateInput struct {
	Name               string           `json:"name" binding:"required"`
	ClientType         string           `json:"client_type" binding:"required"`
	Format             string           `json:"format" binding:"required"`
	ContentType        string           `json:"content_type"`
	Extension          string           `json:"extension"`
	Description        string           `json:"description"`
	IsDefault          bool             `json:"is_default"`
	Enabled            *bool            `json:"enabled"`
	SortOrder          int              `json:"sort_order"`
	PlanIDs            entity.Int64List `json:"plan_ids"`
	ListenersJSON      string           `json:"listeners_json"`
	ProxyGroupsJSON    string           `json:"proxy_groups_json"`
	RuleProvidersJSON  string           `json:"rule_providers_json"`
	ProxyProvidersJSON string           `json:"proxy_providers_json"`
	DNSJSON            string           `json:"dns_json"`
	GeneralJSON        string           `json:"general_json"`
}

// Create creates a new subscription format.
func (s *Service) Create(ctx context.Context, input CreateInput) (*entity.SubscriptionFormat, error) {
	format := &entity.SubscriptionFormat{
		Name:               input.Name,
		ClientType:         input.ClientType,
		Format:             input.Format,
		ContentType:        input.ContentType,
		Extension:          input.Extension,
		Description:        input.Description,
		IsDefault:          input.IsDefault,
		Enabled:            true,
		SortOrder:          input.SortOrder,
		PlanIDs:            input.PlanIDs,
		ListenersJSON:      input.ListenersJSON,
		ProxyGroupsJSON:    input.ProxyGroupsJSON,
		RuleProvidersJSON:  input.RuleProvidersJSON,
		ProxyProvidersJSON: input.ProxyProvidersJSON,
		DNSJSON:            input.DNSJSON,
		GeneralJSON:        input.GeneralJSON,
	}
	if input.Enabled != nil {
		format.Enabled = *input.Enabled
	}
	if err := s.formatRepo.Create(ctx, format); err != nil {
		return nil, err
	}
	return format, nil
}

// UpdateInput is the input for updating a subscription format.
type UpdateInput struct {
	Name               *string           `json:"name"`
	ClientType         *string           `json:"client_type"`
	Format             *string           `json:"format"`
	ContentType        *string           `json:"content_type"`
	Extension          *string           `json:"extension"`
	Description        *string           `json:"description"`
	IsDefault          *bool             `json:"is_default"`
	Enabled            *bool             `json:"enabled"`
	SortOrder          *int              `json:"sort_order"`
	PlanIDs            *entity.Int64List `json:"plan_ids"`
	ListenersJSON      *string           `json:"listeners_json"`
	ProxyGroupsJSON    *string           `json:"proxy_groups_json"`
	RuleProvidersJSON  *string           `json:"rule_providers_json"`
	ProxyProvidersJSON *string           `json:"proxy_providers_json"`
	DNSJSON            *string           `json:"dns_json"`
	GeneralJSON        *string           `json:"general_json"`
}

// Update updates a subscription format.
func (s *Service) Update(ctx context.Context, id int64, input UpdateInput) (*entity.SubscriptionFormat, error) {
	format, err := s.formatRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if input.Name != nil {
		format.Name = *input.Name
	}
	if input.ClientType != nil {
		format.ClientType = *input.ClientType
	}
	if input.Format != nil {
		format.Format = *input.Format
	}
	if input.ContentType != nil {
		format.ContentType = *input.ContentType
	}
	if input.Extension != nil {
		format.Extension = *input.Extension
	}
	if input.Description != nil {
		format.Description = *input.Description
	}
	if input.IsDefault != nil {
		format.IsDefault = *input.IsDefault
	}
	if input.Enabled != nil {
		format.Enabled = *input.Enabled
	}
	if input.SortOrder != nil {
		format.SortOrder = *input.SortOrder
	}
	if input.ListenersJSON != nil {
		format.ListenersJSON = *input.ListenersJSON
	}
	if input.ProxyGroupsJSON != nil {
		format.ProxyGroupsJSON = *input.ProxyGroupsJSON
	}
	if input.RuleProvidersJSON != nil {
		format.RuleProvidersJSON = *input.RuleProvidersJSON
	}
	if input.ProxyProvidersJSON != nil {
		format.ProxyProvidersJSON = *input.ProxyProvidersJSON
	}
	if input.DNSJSON != nil {
		format.DNSJSON = *input.DNSJSON
	}
	if input.GeneralJSON != nil {
		format.GeneralJSON = *input.GeneralJSON
	}
	if input.PlanIDs != nil {
		format.PlanIDs = *input.PlanIDs
	}
	if err := s.formatRepo.Update(ctx, format); err != nil {
		return nil, err
	}
	return format, nil
}

// Delete deletes a subscription format and its rules.
func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.formatRepo.Delete(ctx, id)
}

// --- Rule operations ---

// ListRules returns all rules for a format.
func (s *Service) ListRules(ctx context.Context, formatID int64) ([]entity.SubscriptionRule, error) {
	return s.ruleRepo.ListByFormatID(ctx, formatID)
}

// CreateRuleInput is the input for creating a rule.
type CreateRuleInput struct {
	RuleType     string `json:"rule_type" binding:"required"`
	MatchValue   string `json:"match_value"`
	OutboundTag  string `json:"outbound_tag" binding:"required"`
	RuleProvider string `json:"rule_provider"`
	RuleAction   string `json:"rule_action"`
	Priority     int    `json:"priority"`
	Enabled      *bool  `json:"enabled"`
}

// CreateRule creates a new rule for a format.
func (s *Service) CreateRule(ctx context.Context, formatID int64, input CreateRuleInput) (*entity.SubscriptionRule, error) {
	rule := &entity.SubscriptionRule{
		FormatID:     formatID,
		RuleType:     input.RuleType,
		MatchValue:   input.MatchValue,
		OutboundTag:  input.OutboundTag,
		RuleProvider: input.RuleProvider,
		RuleAction:   input.RuleAction,
		Priority:     input.Priority,
		Enabled:      true,
	}
	if input.Enabled != nil {
		rule.Enabled = *input.Enabled
	}
	if err := s.ruleRepo.Create(ctx, rule); err != nil {
		return nil, err
	}
	return rule, nil
}

// UpdateRuleInput is the input for updating a rule.
type UpdateRuleInput struct {
	RuleType     *string `json:"rule_type"`
	MatchValue   *string `json:"match_value"`
	OutboundTag  *string `json:"outbound_tag"`
	RuleProvider *string `json:"rule_provider"`
	RuleAction   *string `json:"rule_action"`
	Priority     *int    `json:"priority"`
	Enabled      *bool   `json:"enabled"`
}

// UpdateRule updates a rule.
func (s *Service) UpdateRule(ctx context.Context, ruleID int64, input UpdateRuleInput) (*entity.SubscriptionRule, error) {
	rule, err := s.ruleRepo.GetByID(ctx, ruleID)
	if err != nil {
		return nil, err
	}
	if input.RuleType != nil {
		rule.RuleType = *input.RuleType
	}
	if input.MatchValue != nil {
		rule.MatchValue = *input.MatchValue
	}
	if input.OutboundTag != nil {
		rule.OutboundTag = *input.OutboundTag
	}
	if input.RuleProvider != nil {
		rule.RuleProvider = *input.RuleProvider
	}
	if input.RuleAction != nil {
		rule.RuleAction = *input.RuleAction
	}
	if input.Priority != nil {
		rule.Priority = *input.Priority
	}
	if input.Enabled != nil {
		rule.Enabled = *input.Enabled
	}
	if err := s.ruleRepo.Update(ctx, rule); err != nil {
		return nil, err
	}
	return rule, nil
}

// DeleteRule deletes a rule.
func (s *Service) DeleteRule(ctx context.Context, ruleID int64) error {
	return s.ruleRepo.Delete(ctx, ruleID)
}

// SortRulesInput is the input for sorting rules.
type SortRulesInput struct {
	Items []struct {
		ID       int64 `json:"id"`
		Priority int   `json:"priority"`
	} `json:"items" binding:"required"`
}

// SortRules updates priority for multiple rules.
func (s *Service) SortRules(ctx context.Context, input SortRulesInput) error {
	updates := make([]struct {
		ID       int64
		Priority int
	}, len(input.Items))
	for i, item := range input.Items {
		updates[i] = struct {
			ID       int64
			Priority int
		}{ID: item.ID, Priority: item.Priority}
	}
	return s.ruleRepo.BatchUpdatePriority(ctx, updates)
}

// SeedDefaults creates the default subscription formats if none exist.
func (s *Service) SeedDefaults(ctx context.Context) {
	count, err := s.formatRepo.Count(ctx)
	if err != nil || count > 0 {
		return
	}

	defaults := []struct {
		format entity.SubscriptionFormat
		rules  []entity.SubscriptionRule
	}{
		{
			format: entity.SubscriptionFormat{
				Name:        "Clash for Windows",
				ClientType:  "clash",
				Format:      "mihomo_yaml",
				ContentType: "text/yaml; charset=utf-8",
				Extension:   ".yaml",
				Description: "Clash / Clash.Meta / Mihomo 客户端订阅格式",
				IsDefault:   true,
				Enabled:     true,
				SortOrder:   10,
			},
			rules: []entity.SubscriptionRule{
				{RuleType: "geosite", MatchValue: "private", OutboundTag: "DIRECT", Priority: 10},
				{RuleType: "geoip", MatchValue: "private", OutboundTag: "DIRECT", Priority: 20},
				{RuleType: "geosite", MatchValue: "cn", OutboundTag: "DIRECT", Priority: 30},
				{RuleType: "geoip", MatchValue: "cn", OutboundTag: "DIRECT", Priority: 40},
				{RuleType: "match", MatchValue: "", OutboundTag: "PROXY", Priority: 100},
			},
		},
		{
			format: entity.SubscriptionFormat{
				Name:        "Shadowrocket",
				ClientType:  "shadowrocket",
				Format:      "base64_uri",
				ContentType: "text/plain; charset=utf-8",
				Extension:   ".txt",
				Description: "Shadowrocket / 小火箭 订阅格式 (Base64 URI)",
				IsDefault:   false,
				Enabled:     true,
				SortOrder:   20,
			},
			rules: []entity.SubscriptionRule{},
		},
		{
			format: entity.SubscriptionFormat{
				Name:        "Sing-Box",
				ClientType:  "singbox",
				Format:      "singbox_json",
				ContentType: "application/json; charset=utf-8",
				Extension:   ".json",
				Description: "Sing-Box 客户端订阅格式",
				IsDefault:   false,
				Enabled:     true,
				SortOrder:   30,
			},
			rules: []entity.SubscriptionRule{
				{RuleType: "domain_suffix", MatchValue: "cn", OutboundTag: "direct", Priority: 10},
				{RuleType: "ip_cidr", MatchValue: "geoip:cn", OutboundTag: "direct", Priority: 20},
				{RuleType: "ip_cidr", MatchValue: "geoip:private", OutboundTag: "direct", Priority: 30},
				{RuleType: "default", MatchValue: "", OutboundTag: "proxy", Priority: 100},
			},
		},
		{
			format: entity.SubscriptionFormat{
				Name:        "V2RayN",
				ClientType:  "v2ray",
				Format:      "base64_uri",
				ContentType: "text/plain; charset=utf-8",
				Extension:   ".txt",
				Description: "V2RayN / V2Ray 客户端订阅格式 (Base64 URI)",
				IsDefault:   false,
				Enabled:     true,
				SortOrder:   40,
			},
			rules: []entity.SubscriptionRule{},
		},
		{
			format: entity.SubscriptionFormat{
				Name:        "V2Ray JSON",
				ClientType:  "v2ray",
				Format:      "xray_json",
				ContentType: "application/json; charset=utf-8",
				Extension:   ".json",
				Description: "V2Ray / Xray 客户端 JSON 订阅格式",
				IsDefault:   false,
				Enabled:     true,
				SortOrder:   50,
			},
			rules: []entity.SubscriptionRule{},
		},
	}

	for _, d := range defaults {
		if err := s.formatRepo.Create(ctx, &d.format); err != nil {
			log.Printf("seed: failed to create format %s: %v", d.format.Name, err)
			continue
		}
		log.Printf("seed: added subscription format %s", d.format.Name)

		for i := range d.rules {
			d.rules[i].FormatID = d.format.ID
			d.rules[i].Enabled = true
			if err := s.ruleRepo.Create(ctx, &d.rules[i]); err != nil {
				log.Printf("seed: failed to create rule for %s: %v", d.format.Name, err)
			}
		}
	}
}
