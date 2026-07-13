package subscription

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"corefusion/internal/model/entity"
	"corefusion/internal/service/plan"
	"corefusion/internal/store/repository"
)

// Service handles subscription business logic.
type Service struct {
	repo       *repository.SubscriptionRepository
	accessLog  *repository.AccessLogRepository
	rotation   *repository.RotationRepository
	clientRepo *repository.ClientRepository
	planSvc    *plan.Service
}

// NewService creates a new subscription service.
func NewService(
	repo *repository.SubscriptionRepository,
	accessLog *repository.AccessLogRepository,
	rotation *repository.RotationRepository,
	clientRepo *repository.ClientRepository,
	planSvc *plan.Service,
) *Service {
	return &Service{
		repo:       repo,
		accessLog:  accessLog,
		rotation:   rotation,
		clientRepo: clientRepo,
		planSvc:    planSvc,
	}
}

// CreateSubscriptionInput is the input for creating a subscription.
type CreateSubscriptionInput struct {
	ClientID        int64      `json:"client_id" binding:"required"`
	Format          string     `json:"format" binding:"required"`
	SecurityMode    string     `json:"security_mode"`
	BindMode        string     `json:"bind_mode"`
	QuotaBytes      int64      `json:"quota_bytes"`
	QuotaResetCycle string     `json:"quota_reset_cycle"`
	BillingNodeIDs  []int64    `json:"billing_node_ids"`
	MaxActiveDays   *int       `json:"max_active_days"`
	ExpiresAt       *time.Time `json:"expires_at"`
}

// Create creates a new subscription with a random token.
func (s *Service) Create(ctx context.Context, input CreateSubscriptionInput) (*entity.Subscription, error) {
	token, err := generateToken(32)
	if err != nil {
		return nil, err
	}
	billingNodeIDs, err := s.normalizeBillingNodeIDs(ctx, input.ClientID, input.BillingNodeIDs)
	if err != nil {
		return nil, err
	}

	securityMode := input.SecurityMode
	if securityMode == "" {
		securityMode = "token_only"
	}
	bindMode := input.BindMode
	if bindMode == "" {
		bindMode = "none"
	}
	quotaResetCycle := input.QuotaResetCycle
	if quotaResetCycle == "" {
		quotaResetCycle = "none"
	}

	sub := &entity.Subscription{
		ClientID:        input.ClientID,
		Format:          input.Format,
		Token:           token,
		TokenVersion:    1,
		Status:          "active",
		SecurityMode:    securityMode,
		BindMode:        bindMode,
		QuotaBytes:      input.QuotaBytes,
		QuotaResetCycle: quotaResetCycle,
		BillingNodeIDs:  billingNodeIDs,
		MaxActiveDays:   input.MaxActiveDays,
		ExpiresAt:       input.ExpiresAt,
		QuotaState:      "normal",
	}

	if err := s.repo.Create(ctx, sub); err != nil {
		return nil, err
	}

	return sub, nil
}

// GetByID retrieves a subscription by ID.
func (s *Service) GetByID(ctx context.Context, id int64) (*entity.Subscription, error) {
	return s.repo.GetByID(ctx, id)
}

// List retrieves all subscriptions with pagination.
func (s *Service) List(ctx context.Context, status string, limit, offset int) ([]entity.Subscription, int64, error) {
	return s.repo.List(ctx, status, limit, offset)
}

// UpdateSubscriptionInput is the input for updating a subscription.
type UpdateSubscriptionInput struct {
	SecurityMode    *string    `json:"security_mode"`
	BindMode        *string    `json:"bind_mode"`
	QuotaBytes      *int64     `json:"quota_bytes"`
	QuotaResetCycle *string    `json:"quota_reset_cycle"`
	BillingNodeIDs  *[]int64   `json:"billing_node_ids"`
	MaxActiveDays   *int       `json:"max_active_days"`
	ExpiresAt       *time.Time `json:"expires_at"`
	Status          *string    `json:"status"`
	MihomoConfigID  *int64     `json:"mihomo_config_id"`
}

// Update updates a subscription.
func (s *Service) Update(ctx context.Context, id int64, input UpdateSubscriptionInput) (*entity.Subscription, error) {
	sub, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if input.SecurityMode != nil {
		sub.SecurityMode = *input.SecurityMode
	}
	if input.BindMode != nil {
		sub.BindMode = *input.BindMode
	}
	if input.QuotaBytes != nil {
		sub.QuotaBytes = *input.QuotaBytes
	}
	if input.QuotaResetCycle != nil {
		sub.QuotaResetCycle = *input.QuotaResetCycle
	}
	if input.BillingNodeIDs != nil {
		billingNodeIDs, err := s.normalizeBillingNodeIDs(ctx, sub.ClientID, *input.BillingNodeIDs)
		if err != nil {
			return nil, err
		}
		sub.BillingNodeIDs = billingNodeIDs
	}
	if input.MaxActiveDays != nil {
		sub.MaxActiveDays = input.MaxActiveDays
	}
	if input.ExpiresAt != nil {
		sub.ExpiresAt = input.ExpiresAt
	}
	if input.Status != nil {
		sub.Status = *input.Status
	}
	if input.MihomoConfigID != nil {
		sub.MihomoConfigID = input.MihomoConfigID
	}

	if err := s.repo.Update(ctx, sub); err != nil {
		return nil, err
	}

	return sub, nil
}

// Delete deletes a subscription by ID.
func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

// Rotate generates a new token for a subscription.
func (s *Service) Rotate(ctx context.Context, id int64, rotatedBy int64, reason string) (*entity.Subscription, error) {
	sub, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	newToken, err := generateToken(32)
	if err != nil {
		return nil, err
	}

	// Record rotation history
	rot := &entity.SubscriptionRotation{
		SubscriptionID: id,
		OldToken:       sub.Token,
		NewToken:       newToken,
		RotatedBy:      rotatedBy,
		Reason:         reason,
	}
	if err := s.rotation.Create(ctx, rot); err != nil {
		return nil, err
	}

	sub.Token = newToken
	sub.TokenVersion++
	if err := s.repo.Update(ctx, sub); err != nil {
		return nil, err
	}

	return sub, nil
}

// Revoke revokes a subscription.
func (s *Service) Revoke(ctx context.Context, id int64) (*entity.Subscription, error) {
	sub, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if sub.Status == "revoked" {
		return nil, errors.New("subscription already revoked")
	}

	now := time.Now()
	sub.Status = "revoked"
	sub.RevokedAt = &now
	if err := s.repo.Update(ctx, sub); err != nil {
		return nil, err
	}

	return sub, nil
}

// Restore restores a suspended or expired subscription.
func (s *Service) Restore(ctx context.Context, id int64) (*entity.Subscription, error) {
	sub, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if sub.Status == "active" {
		return sub, nil
	}

	sub.Status = "active"
	sub.SuspendedAt = nil
	sub.RevokedAt = nil
	sub.QuotaState = "restored"
	if err := s.repo.Update(ctx, sub); err != nil {
		return nil, err
	}

	return sub, nil
}

// Enable enables a disabled subscription.
func (s *Service) Enable(ctx context.Context, id int64) (*entity.Subscription, error) {
	sub, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	sub.Status = "active"
	if err := s.repo.Update(ctx, sub); err != nil {
		return nil, err
	}

	return sub, nil
}

// ResetUsage resets the usage counter for a subscription.
func (s *Service) ResetUsage(ctx context.Context, id int64) (*entity.Subscription, error) {
	sub, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	sub.UsedBytesCache = 0
	sub.UploadBytesCache = 0
	sub.DownloadBytesCache = 0
	sub.QuotaState = "normal"
	now := time.Now()
	sub.LastResetAt = &now
	if err := s.repo.Update(ctx, sub); err != nil {
		return nil, err
	}

	return sub, nil
}

// RecordAccess records a subscription access event.
func (s *Service) RecordAccess(ctx context.Context, id int64, ip, userAgent, realIP, cdnIP, cdnProvider, accessType string) error {
	// Update subscription access stats
	if err := s.repo.UpdateAccessStats(ctx, id, ip, userAgent); err != nil {
		return err
	}

	// Create access log entry
	log := &entity.SubscriptionAccessLog{
		SubscriptionID: id,
		IP:             ip,
		RealIP:         realIP,
		CDNIP:          cdnIP,
		CDNProvider:    cdnProvider,
		AccessType:     accessType,
		UserAgent:      userAgent,
		StatusCode:     200,
		HitResult:      "served",
	}
	return s.accessLog.Create(ctx, log)
}

// ListAccessLogs retrieves access logs for a subscription.
func (s *Service) ListAccessLogs(ctx context.Context, subID int64, limit, offset int) ([]entity.SubscriptionAccessLog, int64, error) {
	return s.accessLog.ListBySubscriptionID(ctx, subID, limit, offset)
}

// ListRotations retrieves rotation history for a subscription.
func (s *Service) ListRotations(ctx context.Context, subID int64, limit, offset int) ([]entity.SubscriptionRotation, int64, error) {
	return s.rotation.ListBySubscriptionID(ctx, subID, limit, offset)
}

// GetByToken retrieves a subscription by token (for subscription URL access).
func (s *Service) GetByToken(ctx context.Context, token string) (*entity.Subscription, error) {
	return s.repo.GetByToken(ctx, token)
}

// generateToken generates a random hex token.
func generateToken(byteLen int) (string, error) {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *Service) normalizeBillingNodeIDs(ctx context.Context, clientID int64, requested []int64) (entity.Int64List, error) {
	deduped := uniquePositiveInt64s(requested)
	if len(deduped) == 0 {
		return entity.Int64List{}, nil
	}
	if clientID <= 0 || s.clientRepo == nil || s.planSvc == nil {
		return entity.Int64List(deduped), nil
	}

	members, err := s.planSvc.ResolveClientNodeMembers(ctx, clientID)
	if err != nil {
		return nil, err
	}
	return filterAllowedBillingNodeIDs(deduped, members), nil
}

func uniquePositiveInt64s(ids []int64) []int64 {
	if len(ids) == 0 {
		return nil
	}
	result := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func filterAllowedBillingNodeIDs(requested []int64, members []entity.NodeGroupMember) entity.Int64List {
	if len(requested) == 0 || len(members) == 0 {
		return entity.Int64List{}
	}
	allowed := make(map[int64]struct{}, len(members))
	for _, m := range members {
		if m.Enabled && m.ID > 0 {
			allowed[m.ID] = struct{}{}
		}
	}

	filtered := make([]int64, 0, len(requested))
	for _, id := range requested {
		if _, ok := allowed[id]; ok {
			filtered = append(filtered, id)
		}
	}
	return entity.Int64List(filtered)
}
