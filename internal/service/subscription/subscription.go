package subscription

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"sync"
	"time"

	subdomain "corefusion/internal/domain/subscription"
	"corefusion/internal/model/entity"
	"corefusion/internal/service/plan"
	"corefusion/internal/store/repository"
)

// TrafficResetHook resets inbound config traffic and triggers remote sync.
// Implemented by inboundconfig.Service + sync hook.
type TrafficResetHook interface {
	ResetUserTrafficByEmail(ctx context.Context, email string) ([]int64, error)
	SyncServerConfig(ctx context.Context, serverID int64, coreType string)
}

// Service handles subscription business logic.
type Service struct {
	repo         *repository.SubscriptionRepository
	accessLog    *repository.AccessLogRepository
	rotation     *repository.RotationRepository
	clientRepo   *repository.ClientRepository
	planSvc      *plan.Service
	trafficHook  TrafficResetHook
	userSyncer   UserSyncer
	enforcerOnce sync.Once
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

// SetTrafficResetHook sets the hook for resetting inbound traffic and syncing.
func (s *Service) SetTrafficResetHook(hook TrafficResetHook) {
	s.trafficHook = hook
}

// UserSyncer removes or re-adds a user's credentials on all proxy cores. Used to enforce
// quota suspension and subscription expiry by cutting off the user at the core.
type UserSyncer interface {
	SyncUserToServers(ctx context.Context, clientID int64, action string) error
}

// SetUserSyncer injects the user sync client used to enforce quota/expiry on nodes.
func (s *Service) SetUserSyncer(syncer UserSyncer) {
	s.userSyncer = syncer
}

// EnforceQuotaAndExpiry scans active subscriptions and enforces quota exhaustion and
// natural expiry. It marks the subscription, suspends/blocks the underlying client and
// removes the user from the proxy cores when required. Already-handled rows are skipped
// by the repository queries, so repeated calls are safe.
func (s *Service) EnforceQuotaAndExpiry(ctx context.Context) error {
	now := time.Now()

	// 1. Quota exhaustion.
	exhausted, err := s.repo.ListQuotaExhausted(ctx)
	if err != nil {
		return err
	}
	for i := range exhausted {
		sub := &exhausted[i]
		if sub.SuspendOnQuotaExhausted == 1 {
			sub.QuotaState = "suspended"
			sub.Status = "suspended"
		} else {
			sub.QuotaState = "over_limit_hard"
		}
		suspendedAt := now
		sub.SuspendedAt = &suspendedAt
		if err := s.repo.Update(ctx, sub); err != nil {
			log.Printf("[quota-enforce] update subscription %d failed: %v", sub.ID, err)
			continue
		}
		if sub.SuspendOnQuotaExhausted == 1 && sub.ClientID != 0 {
			s.blockClient(ctx, sub.ClientID)
		}
	}

	// 2. Natural expiry.
	expired, err := s.repo.ListExpired(ctx, now, 5000)
	if err != nil {
		return err
	}
	for i := range expired {
		sub := &expired[i]
		sub.Status = "expired"
		sub.QuotaState = "expired_days"
		if err := s.repo.Update(ctx, sub); err != nil {
			log.Printf("[quota-enforce] expire subscription %d failed: %v", sub.ID, err)
			continue
		}
		if sub.ClientID != 0 {
			s.blockClient(ctx, sub.ClientID)
		}
	}

	return nil
}

// blockClient suspends the client (if still active) and removes its credentials from all
// proxy cores, effectively cutting off traffic for the subscription.
func (s *Service) blockClient(ctx context.Context, clientID int64) {
	client, err := s.clientRepo.GetByID(ctx, clientID)
	if err != nil {
		log.Printf("[quota-enforce] get client %d failed: %v", clientID, err)
		return
	}
	if client.Status == "suspended" || client.Status == "expired" {
		return // already blocked
	}
	now := time.Now()
	client.Status = "suspended"
	client.SuspendedAt = &now
	if err := s.clientRepo.Update(ctx, client); err != nil {
		log.Printf("[quota-enforce] suspend client %d failed: %v", clientID, err)
		return
	}
	if s.userSyncer == nil {
		return
	}
	cid := clientID
	go func() {
		if err := s.userSyncer.SyncUserToServers(context.Background(), cid, "remove"); err != nil {
			log.Printf("[quota-enforce] remove client %d from servers failed: %v", cid, err)
		}
	}()
}

// StartEnforcer starts the periodic quota/expiry scan. Safe to call multiple times; only
// the first call launches the loop.
func (s *Service) StartEnforcer(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	s.enforcerOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			// Run once immediately so enforcement applies without waiting a full interval.
			if err := s.EnforceQuotaAndExpiry(ctx); err != nil {
				log.Printf("[quota-enforce] scan error: %v", err)
			}
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if err := s.EnforceQuotaAndExpiry(ctx); err != nil {
						log.Printf("[quota-enforce] scan error: %v", err)
					}
				}
			}
		}()
	})
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
	// ⚠ 未实现的模式**拒绝落库** ✗：存进去不会带来任何保护，却会让人误以为受保护
	// （安全审计 v5 待决项：`SecurityMode`/`BindMode` 从未被校验过）。
	if err := subdomain.ValidateSecurityFields(securityMode, bindMode); err != nil {
		return nil, err
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
	// 同样拒绝未实现的模式 ✗（否则 PUT 能把订阅改成"看起来更安全"的状态）
	if err := subdomain.ValidateSecurityFields(sub.SecurityMode, sub.BindMode); err != nil {
		return nil, err
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

	// Re-add the user to all proxy cores if it was removed due to quota suspension.
	if sub.ClientID != 0 && s.userSyncer != nil {
		if client, err := s.clientRepo.GetByID(ctx, sub.ClientID); err == nil && client != nil {
			if client.Status == "suspended" {
				client.Status = "active"
				client.SuspendedAt = nil
				_ = s.clientRepo.Update(ctx, client)
				cid := client.ID
				go func() {
					if err := s.userSyncer.SyncUserToServers(context.Background(), cid, "add"); err != nil {
						log.Printf("[quota-enforce] re-add client %d to servers failed: %v", cid, err)
					}
				}()
			}
		}
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
// This also resets _usedUp/_usedDown in all inbound configs for the
// associated client's email, and syncs to remote Agent nodes.
func (s *Service) ResetUsage(ctx context.Context, id int64) (*entity.Subscription, error) {
	sub, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 1. Reset subscription cache columns
	sub.UsedBytesCache = 0
	sub.UploadBytesCache = 0
	sub.DownloadBytesCache = 0
	sub.QuotaState = "normal"
	now := time.Now()
	sub.LastResetAt = &now
	if err := s.repo.Update(ctx, sub); err != nil {
		return nil, err
	}

	// 2. Reset client used_bytes
	if s.clientRepo != nil {
		client, err := s.clientRepo.GetByID(ctx, sub.ClientID)
		if err == nil && client != nil {
			client.UsedBytes = 0
			if client.Status == "suspended" {
				client.Status = "active"
				client.SuspendedAt = nil
			}
			_ = s.clientRepo.Update(ctx, client)

			// 3. Reset _usedUp/_usedDown in inbound configs + sync to remote
			if s.trafficHook != nil {
				serverIDs, err := s.trafficHook.ResetUserTrafficByEmail(ctx, client.Email)
				if err != nil {
					log.Printf("resetUsage: failed to reset inbound traffic for %s: %v", client.Email, err)
				} else {
					// 4. Trigger sync to each affected remote server
					for _, sid := range serverIDs {
						if sid > 1 { // skip local server
							s.trafficHook.SyncServerConfig(ctx, sid, "xray")
						}
					}
				}
			}
		}
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
