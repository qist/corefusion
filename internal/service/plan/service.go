package plan

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// Service handles plan template, variant, node group, promotion, and order business logic.
type Service struct {
	db             *gorm.DB
	tmplRepo       *repository.PlanTemplateRepository
	variantRepo    *repository.PlanVariantRepository
	groupRepo      *repository.NodeGroupRepository
	memberRepo     *repository.NodeGroupMemberRepository
	promoRepo      *repository.PromotionRepository
	orderRepo      *repository.PlanOrderRepository
	clientRepo     *repository.ClientRepository
	inboundRepo    *repository.InboundConfigRepository
	extNodeRepo    *repository.ExternalNodeRepository
}

// NewService creates a new plan service.
func NewService(
	db *gorm.DB,
	tmplRepo *repository.PlanTemplateRepository,
	variantRepo *repository.PlanVariantRepository,
	groupRepo *repository.NodeGroupRepository,
	memberRepo *repository.NodeGroupMemberRepository,
	promoRepo *repository.PromotionRepository,
	orderRepo *repository.PlanOrderRepository,
	clientRepo *repository.ClientRepository,
	inboundRepo *repository.InboundConfigRepository,
	extNodeRepo *repository.ExternalNodeRepository,
) *Service {
	return &Service{
		db:          db,
		tmplRepo:    tmplRepo,
		variantRepo: variantRepo,
		groupRepo:   groupRepo,
		memberRepo:  memberRepo,
		promoRepo:   promoRepo,
		orderRepo:   orderRepo,
		clientRepo:  clientRepo,
		inboundRepo: inboundRepo,
		extNodeRepo: extNodeRepo,
	}
}

// ──────────────────── Plan Template ────────────────────

// ListTemplates returns all plan templates.
func (s *Service) ListTemplates(ctx context.Context) ([]entity.PlanTemplate, error) {
	return s.tmplRepo.List(ctx)
}

// GetTemplate returns a template by ID with its variants.
func (s *Service) GetTemplate(ctx context.Context, id int64) (*entity.PlanTemplate, []entity.PlanVariant, error) {
	tmpl, err := s.tmplRepo.GetByID(ctx, id)
	if err != nil {
		return nil, nil, fmt.Errorf("template not found: %w", err)
	}
	variants, err := s.variantRepo.ListByTemplateID(ctx, id)
	if err != nil {
		return tmpl, nil, err
	}
	return tmpl, variants, nil
}

// CreateTemplateInput is the input for creating a plan template.
type CreateTemplateInput struct {
	Name                      string   `json:"name" binding:"required"`
	Description               string   `json:"description"`
	Category                  string   `json:"category"`
	NodeGroupIDs              []int64  `json:"node_group_ids"`
	DeviceLimit               int      `json:"device_limit"`
	SpeedLimitMbps            int      `json:"speed_limit_mbps"`
	ConcurrentLimit           int      `json:"concurrent_limit"`
	DefaultQuotaBytes         int64    `json:"default_quota_bytes"`
	DefaultQuotaResetCycle    string   `json:"default_quota_reset_cycle"`
	DefaultQuotaResetDay      *int     `json:"default_quota_reset_day"`
	SuspendOnQuotaExhausted   *int     `json:"suspend_on_quota_exhausted"`
	IsTrial                   bool     `json:"is_trial"`
	TrialDurationHours        int      `json:"trial_duration_hours"`
	TrialAutoConvertTemplateID *int64  `json:"trial_auto_convert_template_id"`
	BillingUsedTemplate       string   `json:"billing_used_template"`
	BillingRemainingTemplate  string   `json:"billing_remaining_template"`
	BillingResetTemplate      string   `json:"billing_reset_template"`
	BillingExpiryTemplate     string   `json:"billing_expiry_template"`
	BillingNodeIDs            []int64 `json:"billing_node_ids"`
	SortOrder                 int      `json:"sort_order"`
	Enabled                   *bool    `json:"enabled"`
	Metadata                  string   `json:"metadata"`
}

// CreateTemplate creates a new plan template.
func (s *Service) CreateTemplate(ctx context.Context, input CreateTemplateInput) (*entity.PlanTemplate, error) {
	category := input.Category
	if category == "" {
		category = entity.PlanCategoryStandard
	}
	if input.IsTrial {
		category = entity.PlanCategoryTrial
	}

	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}

	suspendVal := 1
	if input.SuspendOnQuotaExhausted != nil {
		suspendVal = *input.SuspendOnQuotaExhausted
	}

	now := time.Now()
	tmpl := &entity.PlanTemplate{
		Name:                       input.Name,
		Description:                input.Description,
		Category:                   category,
		NodeGroupIDs:               entity.Int64List(input.NodeGroupIDs),
		DeviceLimit:                input.DeviceLimit,
		SpeedLimitMbps:             input.SpeedLimitMbps,
		ConcurrentLimit:            input.ConcurrentLimit,
		DefaultQuotaBytes:          input.DefaultQuotaBytes,
		DefaultQuotaResetCycle:     input.DefaultQuotaResetCycle,
		DefaultQuotaResetDay:       input.DefaultQuotaResetDay,
		SuspendOnQuotaExhausted:    suspendVal,
		IsTrial:                    input.IsTrial,
		TrialDurationHours:         input.TrialDurationHours,
		TrialAutoConvertTemplateID: input.TrialAutoConvertTemplateID,
		BillingUsedTemplate:        input.BillingUsedTemplate,
		BillingRemainingTemplate:   input.BillingRemainingTemplate,
		BillingResetTemplate:       input.BillingResetTemplate,
		BillingExpiryTemplate:      input.BillingExpiryTemplate,
		BillingNodeIDs:             entity.Int64List(input.BillingNodeIDs),
		SortOrder:                  input.SortOrder,
		Enabled:                    enabled,
		Metadata:                   input.Metadata,
		CreatedAt:                  now,
		UpdatedAt:                  now,
	}
	if err := s.tmplRepo.Create(ctx, tmpl); err != nil {
		return nil, fmt.Errorf("create template: %w", err)
	}
	return tmpl, nil
}

// UpdateTemplateInput is the input for updating a plan template.
type UpdateTemplateInput struct {
	Name                      *string  `json:"name"`
	Description               *string  `json:"description"`
	Category                  *string  `json:"category"`
	NodeGroupIDs              *[]int64 `json:"node_group_ids"`
	DeviceLimit               *int     `json:"device_limit"`
	SpeedLimitMbps            *int     `json:"speed_limit_mbps"`
	ConcurrentLimit           *int     `json:"concurrent_limit"`
	DefaultQuotaBytes         *int64   `json:"default_quota_bytes"`
	DefaultQuotaResetCycle    *string  `json:"default_quota_reset_cycle"`
	DefaultQuotaResetDay      *int     `json:"default_quota_reset_day"`
	SuspendOnQuotaExhausted   *int     `json:"suspend_on_quota_exhausted"`
	IsTrial                   *bool    `json:"is_trial"`
	TrialDurationHours        *int     `json:"trial_duration_hours"`
	TrialAutoConvertTemplateID *int64  `json:"trial_auto_convert_template_id"`
	BillingUsedTemplate       *string  `json:"billing_used_template"`
	BillingRemainingTemplate  *string  `json:"billing_remaining_template"`
	BillingResetTemplate      *string  `json:"billing_reset_template"`
	BillingExpiryTemplate     *string  `json:"billing_expiry_template"`
	BillingNodeIDs            *[]int64 `json:"billing_node_ids"`
	SortOrder                 *int     `json:"sort_order"`
	Enabled                   *bool    `json:"enabled"`
	Metadata                  *string  `json:"metadata"`
}

// UpdateTemplate updates a plan template.
func (s *Service) UpdateTemplate(ctx context.Context, id int64, input UpdateTemplateInput) (*entity.PlanTemplate, error) {
	tmpl, err := s.tmplRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("template not found: %w", err)
	}
	if input.Name != nil {
		tmpl.Name = *input.Name
	}
	if input.Description != nil {
		tmpl.Description = *input.Description
	}
	if input.Category != nil {
		tmpl.Category = *input.Category
	}
	if input.NodeGroupIDs != nil {
		tmpl.NodeGroupIDs = entity.Int64List(*input.NodeGroupIDs)
	}
	if input.DeviceLimit != nil {
		tmpl.DeviceLimit = *input.DeviceLimit
	}
	if input.SpeedLimitMbps != nil {
		tmpl.SpeedLimitMbps = *input.SpeedLimitMbps
	}
	if input.ConcurrentLimit != nil {
		tmpl.ConcurrentLimit = *input.ConcurrentLimit
	}
	if input.DefaultQuotaBytes != nil {
		tmpl.DefaultQuotaBytes = *input.DefaultQuotaBytes
	}
	if input.DefaultQuotaResetCycle != nil {
		tmpl.DefaultQuotaResetCycle = *input.DefaultQuotaResetCycle
	}
	if input.DefaultQuotaResetDay != nil {
		tmpl.DefaultQuotaResetDay = input.DefaultQuotaResetDay
	}
	if input.SuspendOnQuotaExhausted != nil {
		tmpl.SuspendOnQuotaExhausted = *input.SuspendOnQuotaExhausted
	}
	if input.IsTrial != nil {
		tmpl.IsTrial = *input.IsTrial
	}
	if input.TrialDurationHours != nil {
		tmpl.TrialDurationHours = *input.TrialDurationHours
	}
	if input.TrialAutoConvertTemplateID != nil {
		tmpl.TrialAutoConvertTemplateID = input.TrialAutoConvertTemplateID
	}
	if input.BillingUsedTemplate != nil {
		tmpl.BillingUsedTemplate = *input.BillingUsedTemplate
	}
	if input.BillingRemainingTemplate != nil {
		tmpl.BillingRemainingTemplate = *input.BillingRemainingTemplate
	}
	if input.BillingResetTemplate != nil {
		tmpl.BillingResetTemplate = *input.BillingResetTemplate
	}
	if input.BillingExpiryTemplate != nil {
		tmpl.BillingExpiryTemplate = *input.BillingExpiryTemplate
	}
	if input.BillingNodeIDs != nil {
		tmpl.BillingNodeIDs = entity.Int64List(*input.BillingNodeIDs)
	}
	if input.SortOrder != nil {
		tmpl.SortOrder = *input.SortOrder
	}
	if input.Enabled != nil {
		tmpl.Enabled = *input.Enabled
	}
	if input.Metadata != nil {
		tmpl.Metadata = *input.Metadata
	}
	tmpl.UpdatedAt = time.Now()
	if err := s.tmplRepo.Update(ctx, tmpl); err != nil {
		return nil, fmt.Errorf("update template: %w", err)
	}
	return tmpl, nil
}

// DeleteTemplate deletes a plan template if no clients are associated.
func (s *Service) DeleteTemplate(ctx context.Context, id int64) error {
	count, err := s.tmplRepo.CountClients(ctx, id)
	if err != nil {
		return fmt.Errorf("count clients: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("cannot delete template: %d clients are still associated", count)
	}
	return s.tmplRepo.Delete(ctx, id)
}

// SetTemplateNodeGroups sets the node group IDs for a template.
func (s *Service) SetTemplateNodeGroups(ctx context.Context, templateID int64, groupIDs []int64) error {
	tmpl, err := s.tmplRepo.GetByID(ctx, templateID)
	if err != nil {
		return fmt.Errorf("template not found: %w", err)
	}
	tmpl.NodeGroupIDs = entity.Int64List(groupIDs)
	tmpl.UpdatedAt = time.Now()
	return s.tmplRepo.Update(ctx, tmpl)
}

// ──────────────────── Plan Variant ────────────────────

// ListVariants returns all variants for a template.
func (s *Service) ListVariants(ctx context.Context, templateID int64) ([]entity.PlanVariant, error) {
	return s.variantRepo.ListByTemplateID(ctx, templateID)
}

// CreateVariantInput is the input for creating a plan variant.
type CreateVariantInput struct {
	Name            string `json:"name" binding:"required"`
	BillingCycle    string `json:"billing_cycle"`
	DurationDays    int    `json:"duration_days"`
	Price           int64  `json:"price"`
	OriginalPrice   int64  `json:"original_price"`
	Currency        string `json:"currency"`
	QuotaBytes      int64  `json:"quota_bytes"`
	QuotaResetCycle string `json:"quota_reset_cycle"`
	QuotaResetDay   *int   `json:"quota_reset_day"`
	IsRecommended   bool   `json:"is_recommended"`
	IsDefault       bool   `json:"is_default"`
	Enabled         *bool  `json:"enabled"`
	SortOrder       int    `json:"sort_order"`
}

// CreateVariant creates a new plan variant.
func (s *Service) CreateVariant(ctx context.Context, templateID int64, input CreateVariantInput) (*entity.PlanVariant, error) {
	billingCycle := input.BillingCycle
	if billingCycle == "" {
		billingCycle = entity.BillingCycleMonthly
	}
	duration := input.DurationDays
	if duration <= 0 {
		duration = 30
	}
	currency := input.Currency
	if currency == "" {
		currency = "CNY"
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}

	now := time.Now()
	variant := &entity.PlanVariant{
		TemplateID:      templateID,
		Name:            input.Name,
		BillingCycle:    billingCycle,
		DurationDays:    duration,
		Price:           input.Price,
		OriginalPrice:   input.OriginalPrice,
		Currency:        currency,
		QuotaBytes:      input.QuotaBytes,
		QuotaResetCycle: input.QuotaResetCycle,
		QuotaResetDay:   input.QuotaResetDay,
		IsRecommended:   input.IsRecommended,
		IsDefault:       input.IsDefault,
		Enabled:         enabled,
		SortOrder:       input.SortOrder,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.variantRepo.Create(ctx, variant); err != nil {
		return nil, fmt.Errorf("create variant: %w", err)
	}
	return variant, nil
}

// UpdateVariantInput is the input for updating a plan variant.
type UpdateVariantInput struct {
	Name            *string `json:"name"`
	BillingCycle    *string `json:"billing_cycle"`
	DurationDays    *int    `json:"duration_days"`
	Price           *int64  `json:"price"`
	OriginalPrice   *int64  `json:"original_price"`
	Currency        *string `json:"currency"`
	QuotaBytes      *int64  `json:"quota_bytes"`
	QuotaResetCycle *string `json:"quota_reset_cycle"`
	QuotaResetDay   *int    `json:"quota_reset_day"`
	IsRecommended   *bool   `json:"is_recommended"`
	IsDefault       *bool   `json:"is_default"`
	Enabled         *bool   `json:"enabled"`
	SortOrder       *int    `json:"sort_order"`
}

// UpdateVariant updates a plan variant.
func (s *Service) UpdateVariant(ctx context.Context, id int64, input UpdateVariantInput) (*entity.PlanVariant, error) {
	variant, err := s.variantRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("variant not found: %w", err)
	}
	if input.Name != nil {
		variant.Name = *input.Name
	}
	if input.BillingCycle != nil {
		variant.BillingCycle = *input.BillingCycle
	}
	if input.DurationDays != nil {
		variant.DurationDays = *input.DurationDays
	}
	if input.Price != nil {
		variant.Price = *input.Price
	}
	if input.OriginalPrice != nil {
		variant.OriginalPrice = *input.OriginalPrice
	}
	if input.Currency != nil {
		variant.Currency = *input.Currency
	}
	if input.QuotaBytes != nil {
		variant.QuotaBytes = *input.QuotaBytes
	}
	if input.QuotaResetCycle != nil {
		variant.QuotaResetCycle = *input.QuotaResetCycle
	}
	if input.QuotaResetDay != nil {
		variant.QuotaResetDay = input.QuotaResetDay
	}
	if input.IsRecommended != nil {
		variant.IsRecommended = *input.IsRecommended
	}
	if input.IsDefault != nil {
		variant.IsDefault = *input.IsDefault
	}
	if input.Enabled != nil {
		variant.Enabled = *input.Enabled
	}
	if input.SortOrder != nil {
		variant.SortOrder = *input.SortOrder
	}
	variant.UpdatedAt = time.Now()
	if err := s.variantRepo.Update(ctx, variant); err != nil {
		return nil, fmt.Errorf("update variant: %w", err)
	}
	return variant, nil
}

// DeleteVariant deletes a plan variant.
func (s *Service) DeleteVariant(ctx context.Context, id int64) error {
	return s.variantRepo.Delete(ctx, id)
}

// ──────────────────── Node Group ────────────────────

// ListNodeGroups returns all node groups.
func (s *Service) ListNodeGroups(ctx context.Context) ([]entity.NodeGroup, error) {
	return s.groupRepo.List(ctx)
}

// CreateNodeGroupInput is the input for creating a node group.
type CreateNodeGroupInput struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Region      string `json:"region"`
	Tier        string `json:"tier"`
	SortOrder   int    `json:"sort_order"`
	Enabled     *bool  `json:"enabled"`
}

// CreateNodeGroup creates a new node group.
func (s *Service) CreateNodeGroup(ctx context.Context, input CreateNodeGroupInput) (*entity.NodeGroup, error) {
	tier := input.Tier
	if tier == "" {
		tier = entity.NodeTierStandard
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	now := time.Now()
	group := &entity.NodeGroup{
		Name:        input.Name,
		Description: input.Description,
		Region:      input.Region,
		Tier:        tier,
		SortOrder:   input.SortOrder,
		Enabled:     enabled,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.groupRepo.Create(ctx, group); err != nil {
		return nil, fmt.Errorf("create node group: %w", err)
	}
	return group, nil
}

// UpdateNodeGroupInput is the input for updating a node group.
type UpdateNodeGroupInput struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Region      *string `json:"region"`
	Tier        *string `json:"tier"`
	SortOrder   *int    `json:"sort_order"`
	Enabled     *bool   `json:"enabled"`
}

// UpdateNodeGroup updates a node group.
func (s *Service) UpdateNodeGroup(ctx context.Context, id int64, input UpdateNodeGroupInput) (*entity.NodeGroup, error) {
	group, err := s.groupRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("node group not found: %w", err)
	}
	if input.Name != nil {
		group.Name = *input.Name
	}
	if input.Description != nil {
		group.Description = *input.Description
	}
	if input.Region != nil {
		group.Region = *input.Region
	}
	if input.Tier != nil {
		group.Tier = *input.Tier
	}
	if input.SortOrder != nil {
		group.SortOrder = *input.SortOrder
	}
	if input.Enabled != nil {
		group.Enabled = *input.Enabled
	}
	group.UpdatedAt = time.Now()
	if err := s.groupRepo.Update(ctx, group); err != nil {
		return nil, fmt.Errorf("update node group: %w", err)
	}
	return group, nil
}

// DeleteNodeGroup deletes a node group and its members.
func (s *Service) DeleteNodeGroup(ctx context.Context, id int64) error {
	return s.groupRepo.Delete(ctx, id)
}

// ListGroupMembers returns all members of a node group.
func (s *Service) ListGroupMembers(ctx context.Context, groupID int64) ([]entity.NodeGroupMember, error) {
	return s.memberRepo.ListByGroupID(ctx, groupID)
}

// AddGroupMemberInput is the input for adding a member to a node group.
type AddGroupMemberInput struct {
	InboundConfigID int64 `json:"inbound_config_id"`
	ExternalNodeID  int64 `json:"external_node_id"`
}

// AddGroupMember adds a member to a node group.
func (s *Service) AddGroupMember(ctx context.Context, groupID int64, input AddGroupMemberInput) (*entity.NodeGroupMember, error) {
	if input.InboundConfigID == 0 && input.ExternalNodeID == 0 {
		return nil, fmt.Errorf("inbound_config_id or external_node_id required")
	}
	member := &entity.NodeGroupMember{
		GroupID:         groupID,
		InboundConfigID: input.InboundConfigID,
		ExternalNodeID:  input.ExternalNodeID,
		Enabled:         true,
		CreatedAt:       time.Now(),
	}
	if err := s.memberRepo.Create(ctx, member); err != nil {
		return nil, fmt.Errorf("add group member: %w", err)
	}
	return member, nil
}

// RemoveGroupMember removes a member from a node group.
func (s *Service) RemoveGroupMember(ctx context.Context, memberID int64) error {
	return s.memberRepo.Delete(ctx, memberID)
}

// ──────────────────── Promotion ────────────────────

// ListPromotions returns all promotions.
func (s *Service) ListPromotions(ctx context.Context, limit, offset int) ([]entity.Promotion, int64, error) {
	return s.promoRepo.List(ctx, limit, offset)
}

// CreatePromotionInput is the input for creating a promotion.
type CreatePromotionInput struct {
	Code              string   `json:"code" binding:"required"`
	Name              string   `json:"name" binding:"required"`
	Description       string   `json:"description"`
	DiscountType      string   `json:"discount_type"`
	DiscountValue     int64    `json:"discount_value"`
	MaxDiscountAmount int64    `json:"max_discount_amount"`
	ApplicableScope   string   `json:"applicable_scope"`
	ApplicableIDs     []int64  `json:"applicable_ids"`
	MaxUses           int      `json:"max_uses"`
	MaxUsesPerUser    int      `json:"max_uses_per_user"`
	StartsAt          *time.Time `json:"starts_at"`
	EndsAt            *time.Time `json:"ends_at"`
	Enabled           *bool    `json:"enabled"`
}

// CreatePromotion creates a new promotion.
func (s *Service) CreatePromotion(ctx context.Context, input CreatePromotionInput) (*entity.Promotion, error) {
	discountType := input.DiscountType
	if discountType == "" {
		discountType = entity.DiscountTypePercentage
	}
	scope := input.ApplicableScope
	if scope == "" {
		scope = entity.PromoScopeAll
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	now := time.Now()
	promo := &entity.Promotion{
		Code:              input.Code,
		Name:              input.Name,
		Description:       input.Description,
		DiscountType:      discountType,
		DiscountValue:     input.DiscountValue,
		MaxDiscountAmount: input.MaxDiscountAmount,
		ApplicableScope:   scope,
		ApplicableIDs:     entity.Int64List(input.ApplicableIDs),
		MaxUses:           input.MaxUses,
		MaxUsesPerUser:    input.MaxUsesPerUser,
		StartsAt:          input.StartsAt,
		EndsAt:            input.EndsAt,
		Enabled:           enabled,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := s.promoRepo.Create(ctx, promo); err != nil {
		return nil, fmt.Errorf("create promotion: %w", err)
	}
	return promo, nil
}

// UpdatePromotionInput is the input for updating a promotion.
type UpdatePromotionInput struct {
	Name              *string    `json:"name"`
	Description       *string    `json:"description"`
	DiscountType      *string    `json:"discount_type"`
	DiscountValue     *int64     `json:"discount_value"`
	MaxDiscountAmount *int64     `json:"max_discount_amount"`
	ApplicableScope   *string    `json:"applicable_scope"`
	ApplicableIDs     *[]int64   `json:"applicable_ids"`
	MaxUses           *int       `json:"max_uses"`
	MaxUsesPerUser    *int       `json:"max_uses_per_user"`
	StartsAt          *time.Time `json:"starts_at"`
	EndsAt            *time.Time `json:"ends_at"`
	Enabled           *bool      `json:"enabled"`
}

// UpdatePromotion updates a promotion.
func (s *Service) UpdatePromotion(ctx context.Context, id int64, input UpdatePromotionInput) (*entity.Promotion, error) {
	promo, err := s.promoRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("promotion not found: %w", err)
	}
	if input.Name != nil {
		promo.Name = *input.Name
	}
	if input.Description != nil {
		promo.Description = *input.Description
	}
	if input.DiscountType != nil {
		promo.DiscountType = *input.DiscountType
	}
	if input.DiscountValue != nil {
		promo.DiscountValue = *input.DiscountValue
	}
	if input.MaxDiscountAmount != nil {
		promo.MaxDiscountAmount = *input.MaxDiscountAmount
	}
	if input.ApplicableScope != nil {
		promo.ApplicableScope = *input.ApplicableScope
	}
	if input.ApplicableIDs != nil {
		promo.ApplicableIDs = entity.Int64List(*input.ApplicableIDs)
	}
	if input.MaxUses != nil {
		promo.MaxUses = *input.MaxUses
	}
	if input.MaxUsesPerUser != nil {
		promo.MaxUsesPerUser = *input.MaxUsesPerUser
	}
	if input.StartsAt != nil {
		promo.StartsAt = input.StartsAt
	}
	if input.EndsAt != nil {
		promo.EndsAt = input.EndsAt
	}
	if input.Enabled != nil {
		promo.Enabled = *input.Enabled
	}
	promo.UpdatedAt = time.Now()
	if err := s.promoRepo.Update(ctx, promo); err != nil {
		return nil, fmt.Errorf("update promotion: %w", err)
	}
	return promo, nil
}

// DeletePromotion deletes a promotion.
func (s *Service) DeletePromotion(ctx context.Context, id int64) error {
	return s.promoRepo.Delete(ctx, id)
}

// ValidatePromotionResult is the result of validating a promotion code.
type ValidatePromotionResult struct {
	Valid         bool   `json:"valid"`
	DiscountType  string `json:"discount_type"`
	DiscountValue int64  `json:"discount_value"`
	OriginalAmount int64 `json:"original_amount"`
	DiscountAmount int64 `json:"discount_amount"`
	PaidAmount    int64  `json:"paid_amount"`
	RemainingUses int    `json:"remaining_uses"`
	ErrorMessage  string `json:"error_message,omitempty"`
}

// ValidatePromotion validates a promotion code against a variant and returns the discount.
func (s *Service) ValidatePromotion(ctx context.Context, code string, variantID int64) (*ValidatePromotionResult, error) {
	promo, err := s.promoRepo.GetByCode(ctx, code)
	if err != nil {
		return &ValidatePromotionResult{Valid: false, ErrorMessage: "invalid promotion code"}, nil
	}
	if !promo.Enabled {
		return &ValidatePromotionResult{Valid: false, ErrorMessage: "promotion is disabled"}, nil
	}
	now := time.Now()
	if promo.StartsAt != nil && now.Before(*promo.StartsAt) {
		return &ValidatePromotionResult{Valid: false, ErrorMessage: "promotion not started yet"}, nil
	}
	if promo.EndsAt != nil && now.After(*promo.EndsAt) {
		return &ValidatePromotionResult{Valid: false, ErrorMessage: "promotion expired"}, nil
	}
	if promo.MaxUses > 0 && promo.UsedCount >= promo.MaxUses {
		return &ValidatePromotionResult{Valid: false, ErrorMessage: "promotion usage limit reached"}, nil
	}

	variant, err := s.variantRepo.GetByID(ctx, variantID)
	if err != nil {
		return nil, fmt.Errorf("variant not found: %w", err)
	}

	// Check applicability
	if promo.ApplicableScope == entity.PromoScopeTemplate {
		found := false
		for _, id := range promo.ApplicableIDs {
			if id == variant.TemplateID {
				found = true
				break
			}
		}
		if !found {
			return &ValidatePromotionResult{Valid: false, ErrorMessage: "promotion not applicable to this plan template"}, nil
		}
	} else if promo.ApplicableScope == entity.PromoScopeSpecificVariant {
		found := false
		for _, id := range promo.ApplicableIDs {
			if id == variantID {
				found = true
				break
			}
		}
		if !found {
			return &ValidatePromotionResult{Valid: false, ErrorMessage: "promotion not applicable to this variant"}, nil
		}
	}

	originalAmount := variant.Price
	discount := calculateDiscount(promo, originalAmount)

	remainingUses := 0
	if promo.MaxUses > 0 {
		remainingUses = promo.MaxUses - promo.UsedCount
	}

	return &ValidatePromotionResult{
		Valid:          true,
		DiscountType:   promo.DiscountType,
		DiscountValue:  promo.DiscountValue,
		OriginalAmount: originalAmount,
		DiscountAmount: discount,
		PaidAmount:     originalAmount - discount,
		RemainingUses:  remainingUses,
	}, nil
}

// calculateDiscount computes the discount amount for a given promotion and original price.
func calculateDiscount(promo *entity.Promotion, originalAmount int64) int64 {
	switch promo.DiscountType {
	case entity.DiscountTypePercentage:
		discount := originalAmount * promo.DiscountValue / 100
		if promo.MaxDiscountAmount > 0 && discount > promo.MaxDiscountAmount {
			discount = promo.MaxDiscountAmount
		}
		return discount
	case entity.DiscountTypeFixedAmount:
		if promo.DiscountValue > originalAmount {
			return originalAmount
		}
		return promo.DiscountValue
	}
	return 0
}

// ──────────────────── Plan Order & Client Operations ────────────────────

// ListClientOrders returns order history for a client.
func (s *Service) ListClientOrders(ctx context.Context, clientID int64, limit, offset int) ([]entity.PlanOrder, int64, error) {
	return s.orderRepo.ListByClientID(ctx, clientID, limit, offset)
}

// RenewInput is the input for renewing a client's subscription.
type RenewInput struct {
	VariantID     int64  `json:"variant_id" binding:"required"`
	PromotionCode string `json:"promotion_code"`
}

// RenewResult is the result of a renewal operation.
type RenewResult struct {
	OrderID        int64      `json:"order_id"`
	OrderType      string     `json:"order_type"`
	VariantID      int64      `json:"variant_id"`
	VariantName    string     `json:"variant_name"`
	OriginalAmount int64      `json:"original_amount"`
	DiscountAmount int64      `json:"discount_amount"`
	PaidAmount     int64      `json:"paid_amount"`
	PromotionCode  string     `json:"promotion_code"`
	EffectiveFrom  time.Time  `json:"effective_from"`
	EffectiveTo    *time.Time `json:"effective_to"`
	ClientExpiresAt *time.Time `json:"client_expires_at"`
}

// Renew renews a client's subscription with the given variant.
func (s *Service) Renew(ctx context.Context, clientID int64, input RenewInput) (*RenewResult, error) {
	client, err := s.clientRepo.GetByID(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("client not found: %w", err)
	}

	variant, err := s.variantRepo.GetByID(ctx, input.VariantID)
	if err != nil {
		return nil, fmt.Errorf("variant not found: %w", err)
	}

	// Calculate new expiry
	now := time.Now()
	var newExpiry time.Time
	if client.ExpiresAt != nil && client.ExpiresAt.After(now) {
		newExpiry = client.ExpiresAt.AddDate(0, 0, variant.DurationDays)
	} else {
		newExpiry = now.AddDate(0, 0, variant.DurationDays)
	}

	// Apply promotion if provided
	var discountAmount int64
	var promoID *int64
	var promoCode string
	if input.PromotionCode != "" {
		result, err := s.ValidatePromotion(ctx, input.PromotionCode, input.VariantID)
		if err != nil {
			return nil, fmt.Errorf("validate promotion: %w", err)
		}
		if result.Valid {
			discountAmount = result.DiscountAmount
			promo, _ := s.promoRepo.GetByCode(ctx, input.PromotionCode)
			if promo != nil {
				promoID = &promo.ID
				promoCode = promo.Code
			}
		}
	}

	originalAmount := variant.Price
	paidAmount := originalAmount - discountAmount

	// Create order
	order := &entity.PlanOrder{
		ClientID:       clientID,
		OrderType:      entity.OrderTypeRenew,
		FromVariantID:  nil,
		ToVariantID:    input.VariantID,
		OriginalAmount: originalAmount,
		DiscountAmount: discountAmount,
		PaidAmount:     paidAmount,
		PromotionID:    promoID,
		PromotionCode:  promoCode,
		EffectiveFrom:  now,
		EffectiveTo:    &newExpiry,
		Status:         entity.OrderStatusCompleted,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.orderRepo.Create(ctx, order); err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}

	// Update client
	client.PlanVariantID = input.VariantID
	client.PlanID = variant.TemplateID
	client.ExpiresAt = &newExpiry
	client.Status = "active"
	// Update quota if variant specifies
	if variant.QuotaBytes > 0 {
		client.QuotaBytes = variant.QuotaBytes
	}
	client.UpdatedAt = now
	if err := s.clientRepo.Update(ctx, client); err != nil {
		return nil, fmt.Errorf("update client: %w", err)
	}

	// Increment promo usage
	if promoID != nil {
		_ = s.promoRepo.IncrementUsedCount(ctx, *promoID)
	}

	return &RenewResult{
		OrderID:        order.ID,
		OrderType:      order.OrderType,
		VariantID:      input.VariantID,
		VariantName:    variant.Name,
		OriginalAmount: originalAmount,
		DiscountAmount: discountAmount,
		PaidAmount:     paidAmount,
		PromotionCode:  promoCode,
		EffectiveFrom:  now,
		EffectiveTo:    &newExpiry,
		ClientExpiresAt: &newExpiry,
	}, nil
}

// UpgradeInput is the input for upgrading a client's plan.
type UpgradeInput struct {
	VariantID     int64  `json:"variant_id" binding:"required"`
	PromotionCode string `json:"promotion_code"`
}

// Upgrade upgrades a client's plan to a higher variant immediately.
func (s *Service) Upgrade(ctx context.Context, clientID int64, input UpgradeInput) (*RenewResult, error) {
	client, err := s.clientRepo.GetByID(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("client not found: %w", err)
	}

	oldVariantID := client.PlanVariantID
	variant, err := s.variantRepo.GetByID(ctx, input.VariantID)
	if err != nil {
		return nil, fmt.Errorf("variant not found: %w", err)
	}

	now := time.Now()
	var newExpiry time.Time
	if client.ExpiresAt != nil && client.ExpiresAt.After(now) {
		newExpiry = *client.ExpiresAt
	} else {
		newExpiry = now.AddDate(0, 0, variant.DurationDays)
	}

	// Apply promotion
	var discountAmount int64
	var promoID *int64
	var promoCode string
	if input.PromotionCode != "" {
		result, _ := s.ValidatePromotion(ctx, input.PromotionCode, input.VariantID)
		if result != nil && result.Valid {
			discountAmount = result.DiscountAmount
			promo, _ := s.promoRepo.GetByCode(ctx, input.PromotionCode)
			if promo != nil {
				promoID = &promo.ID
				promoCode = promo.Code
			}
		}
	}

	originalAmount := variant.Price
	paidAmount := originalAmount - discountAmount

	var fromVariantID *int64
	if oldVariantID > 0 {
		fromVariantID = &oldVariantID
	}

	order := &entity.PlanOrder{
		ClientID:       clientID,
		OrderType:      entity.OrderTypeUpgrade,
		FromVariantID:  fromVariantID,
		ToVariantID:    input.VariantID,
		OriginalAmount: originalAmount,
		DiscountAmount: discountAmount,
		PaidAmount:     paidAmount,
		PromotionID:    promoID,
		PromotionCode:  promoCode,
		EffectiveFrom:  now,
		EffectiveTo:    &newExpiry,
		Status:         entity.OrderStatusCompleted,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.orderRepo.Create(ctx, order); err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}

	// Update client
	client.PlanVariantID = input.VariantID
	client.PlanID = variant.TemplateID
	client.ExpiresAt = &newExpiry
	if variant.QuotaBytes > 0 {
		client.QuotaBytes = variant.QuotaBytes
	}
	client.UpdatedAt = now

	// Inherit features from template
	tmpl, _ := s.tmplRepo.GetByID(ctx, variant.TemplateID)
	if tmpl != nil {
		if client.SpeedLimitMbps == 0 {
			client.SpeedLimitMbps = tmpl.SpeedLimitMbps
		}
		if client.ConcurrentLimit == 0 {
			client.ConcurrentLimit = tmpl.ConcurrentLimit
		}
		client.DeviceLimit = tmpl.DeviceLimit
	}
	if err := s.clientRepo.Update(ctx, client); err != nil {
		return nil, fmt.Errorf("update client: %w", err)
	}

	if promoID != nil {
		_ = s.promoRepo.IncrementUsedCount(ctx, *promoID)
	}

	return &RenewResult{
		OrderID:        order.ID,
		OrderType:      order.OrderType,
		VariantID:      input.VariantID,
		VariantName:    variant.Name,
		OriginalAmount: originalAmount,
		DiscountAmount: discountAmount,
		PaidAmount:     paidAmount,
		PromotionCode:  promoCode,
		EffectiveFrom:  now,
		EffectiveTo:    &newExpiry,
		ClientExpiresAt: &newExpiry,
	}, nil
}

// DowngradeInput is the input for downgrading a client's plan (takes effect at expiry).
type DowngradeInput struct {
	VariantID int64 `json:"variant_id" binding:"required"`
}

// Downgrade schedules a downgrade for when the current subscription expires.
func (s *Service) Downgrade(ctx context.Context, clientID int64, input DowngradeInput) (*RenewResult, error) {
	client, err := s.clientRepo.GetByID(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("client not found: %w", err)
	}

	oldVariantID := client.PlanVariantID
	variant, err := s.variantRepo.GetByID(ctx, input.VariantID)
	if err != nil {
		return nil, fmt.Errorf("variant not found: %w", err)
	}

	now := time.Now()
	// Downgrade takes effect at current expiry
	var effectiveFrom time.Time
	if client.ExpiresAt != nil && client.ExpiresAt.After(now) {
		effectiveFrom = *client.ExpiresAt
	} else {
		effectiveFrom = now
	}
	newExpiry := effectiveFrom.AddDate(0, 0, variant.DurationDays)

	var fromVariantID *int64
	if oldVariantID > 0 {
		fromVariantID = &oldVariantID
	}

	order := &entity.PlanOrder{
		ClientID:       clientID,
		OrderType:      entity.OrderTypeDowngrade,
		FromVariantID:  fromVariantID,
		ToVariantID:    input.VariantID,
		OriginalAmount: variant.Price,
		DiscountAmount: 0,
		PaidAmount:     variant.Price,
		EffectiveFrom:  effectiveFrom,
		EffectiveTo:    &newExpiry,
		Status:         entity.OrderStatusPending, // pending until expiry
		CreatedAt:      now,
		UpdatedAt:      now,
		Remark:         "scheduled downgrade, takes effect at current expiry",
	}
	if err := s.orderRepo.Create(ctx, order); err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}

	return &RenewResult{
		OrderID:        order.ID,
		OrderType:      order.OrderType,
		VariantID:      input.VariantID,
		VariantName:    variant.Name,
		OriginalAmount: variant.Price,
		DiscountAmount: 0,
		PaidAmount:     variant.Price,
		EffectiveFrom:  effectiveFrom,
		EffectiveTo:    &newExpiry,
	}, nil
}

// ──────────────────── Node Permission Resolution ────────────────────

// AccessibleNodesResult contains the inbound configs and external nodes accessible by a template.
type AccessibleNodesResult struct {
	InboundConfigs []entity.InboundConfig `json:"inbound_configs"`
	ExternalNodes  []entity.ExternalNode  `json:"external_nodes"`
}

// GetTemplateByID returns a plan template by ID (without variants).
// This is a lightweight getter used by other services that only need template info.
func (s *Service) GetTemplateByID(ctx context.Context, id int64) (*entity.PlanTemplate, error) {
	return s.tmplRepo.GetByID(ctx, id)
}

// ResolveClientNodes resolves all nodes accessible by a client via their plan template's node groups.
func (s *Service) ResolveClientNodes(ctx context.Context, clientID int64) (*AccessibleNodesResult, error) {
	client, err := s.clientRepo.GetByID(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("client not found: %w", err)
	}
	if client.PlanID == 0 {
		return &AccessibleNodesResult{
			InboundConfigs: []entity.InboundConfig{},
			ExternalNodes:  []entity.ExternalNode{},
		}, nil
	}
	return s.GetAccessibleNodes(ctx, client.PlanID)
}

// ResolveClientNodeMembers returns the raw NodeGroupMember list for a client's plan template.
// Used for billing node ID filtering (replaces planNodeRepo.ListByPlanID).
func (s *Service) ResolveClientNodeMembers(ctx context.Context, clientID int64) ([]entity.NodeGroupMember, error) {
	client, err := s.clientRepo.GetByID(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("client not found: %w", err)
	}
	if client.PlanID == 0 {
		return nil, nil
	}
	tmpl, err := s.tmplRepo.GetByID(ctx, client.PlanID)
	if err != nil {
		return nil, err
	}
	if len(tmpl.NodeGroupIDs) == 0 {
		return nil, nil
	}
	return s.memberRepo.ListByGroupIDs(ctx, []int64(tmpl.NodeGroupIDs))
}

// GetTemplateNodeMembers returns the raw NodeGroupMember list for a plan template.
// This is used by handlers that need member IDs for rendering and billing node selection.
func (s *Service) GetTemplateNodeMembers(ctx context.Context, templateID int64) ([]entity.NodeGroupMember, error) {
	tmpl, err := s.tmplRepo.GetByID(ctx, templateID)
	if err != nil {
		return nil, err
	}
	if len(tmpl.NodeGroupIDs) == 0 {
		return nil, nil
	}
	return s.memberRepo.ListByGroupIDs(ctx, []int64(tmpl.NodeGroupIDs))
}

// GetAccessibleNodes returns all nodes accessible by a plan template via its node groups.
func (s *Service) GetAccessibleNodes(ctx context.Context, templateID int64) (*AccessibleNodesResult, error) {
	tmpl, err := s.tmplRepo.GetByID(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("template not found: %w", err)
	}

	members, err := s.memberRepo.ListByGroupIDs(ctx, []int64(tmpl.NodeGroupIDs))
	if err != nil {
		return nil, fmt.Errorf("list group members: %w", err)
	}

	result := &AccessibleNodesResult{
		InboundConfigs: []entity.InboundConfig{},
		ExternalNodes:  []entity.ExternalNode{},
	}
	seenInbound := make(map[int64]bool)
	seenExt := make(map[int64]bool)

	for _, m := range members {
		if !m.Enabled {
			continue
		}
		if m.InboundConfigID > 0 && !seenInbound[m.InboundConfigID] {
			ib, err := s.inboundRepo.GetByID(ctx, m.InboundConfigID)
			if err == nil && ib != nil && ib.Enabled {
				result.InboundConfigs = append(result.InboundConfigs, *ib)
				seenInbound[m.InboundConfigID] = true
			}
		}
		if m.ExternalNodeID > 0 && !seenExt[m.ExternalNodeID] {
			if s.extNodeRepo != nil {
				en, err := s.extNodeRepo.GetByID(ctx, m.ExternalNodeID)
				if err == nil && en != nil && en.Enabled {
					result.ExternalNodes = append(result.ExternalNodes, *en)
					seenExt[m.ExternalNodeID] = true
				}
			}
		}
	}

	return result, nil
}

// ──────────────────── Billing Node Options ────────────────────

// InboundWithServer is an InboundConfig enriched with its NodeServer name.
type InboundWithServer struct {
	entity.InboundConfig
	ServerName string `json:"server_name" gorm:"column:server_name"`
}

// ListAllInboundsWithServer returns all inbound configs across all servers,
// enriched with the associated NodeServer name.
func (s *Service) ListAllInboundsWithServer(ctx context.Context) ([]InboundWithServer, error) {
	var results []InboundWithServer
	err := s.db.WithContext(ctx).
		Table("inbound_configs AS ic").
		Select("ic.*, ns.name AS server_name").
		Joins("LEFT JOIN node_servers AS ns ON ns.id = ic.server_id").
		Order("ns.sort_order, ns.id, ic.id").
		Find(&results).Error
	if err != nil {
		return nil, fmt.Errorf("list inbounds with server: %w", err)
	}
	return results, nil
}

// BillingNodeOption represents a node group member enriched with display info
// for billing node selection in the UI.
type BillingNodeOption struct {
	MemberID     int64  `json:"member_id"`
	Type         string `json:"type"`          // "inbound" or "external"
	ServerID     int64  `json:"server_id"`
	ServerName   string `json:"server_name"`   // host node name
	NodeName     string `json:"node_name"`     // inbound remark/tag or external node name
	Protocol     string `json:"protocol"`
	Port         int    `json:"port"`
	DisplayLabel string `json:"display_label"` // "主机节点 - 入口/节点名称 (protocol:port)"
}

// GetBillingNodeOptions returns enriched node group members for a plan template,
// suitable for billing node selection in the UI. Each option shows the host
// node name, inbound/node name, and protocol:port.
func (s *Service) GetBillingNodeOptions(ctx context.Context, templateID int64) ([]BillingNodeOption, error) {
	members, err := s.GetTemplateNodeMembers(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("get template node members: %w", err)
	}
	if len(members) == 0 {
		return []BillingNodeOption{}, nil
	}

	// Collect IDs for batch queries
	var inboundIDs, extNodeIDs []int64
	for _, m := range members {
		if m.InboundConfigID > 0 {
			inboundIDs = append(inboundIDs, m.InboundConfigID)
		}
		if m.ExternalNodeID > 0 {
			extNodeIDs = append(extNodeIDs, m.ExternalNodeID)
		}
	}

	// Batch fetch inbounds with server names
	inboundMap := make(map[int64]InboundWithServer)
	if len(inboundIDs) > 0 {
		var inbounds []InboundWithServer
		s.db.WithContext(ctx).
			Table("inbound_configs AS ic").
			Select("ic.*, ns.name AS server_name").
			Joins("LEFT JOIN node_servers AS ns ON ns.id = ic.server_id").
			Where("ic.id IN ?", inboundIDs).
			Find(&inbounds)
		for _, ib := range inbounds {
			inboundMap[ib.ID] = ib
		}
	}

	// Batch fetch external nodes
	extNodeMap := make(map[int64]entity.ExternalNode)
	if s.extNodeRepo != nil && len(extNodeIDs) > 0 {
		allExt, _ := s.extNodeRepo.ListAll(ctx)
		for _, en := range allExt {
			extNodeMap[en.ID] = en
		}
	}

	options := make([]BillingNodeOption, 0, len(members))
	for _, m := range members {
		if !m.Enabled {
			continue
		}
		opt := BillingNodeOption{MemberID: m.ID}

		if m.InboundConfigID > 0 {
			ib, ok := inboundMap[m.InboundConfigID]
			if !ok || !ib.Enabled {
				continue
			}
			opt.Type = "inbound"
			opt.ServerID = ib.ServerID
			opt.ServerName = ib.ServerName
			if opt.ServerName == "" {
				opt.ServerName = "本地节点"
			}
			opt.NodeName = ib.Remark
			if opt.NodeName == "" {
				opt.NodeName = ib.Tag
			}
			opt.Protocol = ib.Protocol
			opt.Port = ib.Port
			opt.DisplayLabel = fmt.Sprintf("%s - %s (%s:%d)", opt.ServerName, opt.NodeName, opt.Protocol, opt.Port)
		} else if m.ExternalNodeID > 0 {
			en, ok := extNodeMap[m.ExternalNodeID]
			if !ok || !en.Enabled {
				continue
			}
			opt.Type = "external"
			opt.ServerName = "外部节点"
			opt.NodeName = en.Name
			opt.Protocol = en.Protocol
			opt.Port = en.Port
			opt.DisplayLabel = fmt.Sprintf("%s - %s (%s:%d)", opt.ServerName, opt.NodeName, opt.Protocol, opt.Port)
		} else {
			continue
		}

		options = append(options, opt)
	}

	return options, nil
}
