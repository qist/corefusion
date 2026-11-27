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

// ShortLinkHook 是订阅生命周期与短链的联动面。
// 由 shortlink.Service 实现；放在接口里避免 service 之间硬依赖。
type ShortLinkHook interface {
	AutoCreateForSubscription(ctx context.Context, sub *entity.Subscription) error
	DeleteBySubscription(ctx context.Context, subID int64) error
	SyncTokenVersion(ctx context.Context, subID int64, version int) error
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
	shortLinks   ShortLinkHook
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

// SetShortLinkHook injects short-link lifecycle hooks (auto create / cleanup / rotation).
func (s *Service) SetShortLinkHook(hook ShortLinkHook) {
	s.shortLinks = hook
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
			s.blockClient(ctx, sub.ClientID, "auto_quota")
		}
	}

	// 2. Natural expiry.
	expired, err := s.repo.ListExpired(ctx, now, 5000)
	if err != nil {
		return err
	}
	for i := range expired {
		sub := &expired[i]
		// 永久规格：库里残留的 expires_at（规格改成永久之前的数据）不能真的把账号停掉，
		// 只清掉到期时间。下次对账/启动时会同步修好客户记录。
		if s.isPermanentClient(ctx, sub.ClientID) {
			sub.ExpiresAt = nil
			if err := s.repo.Update(ctx, sub); err != nil {
				log.Printf("[quota-enforce] clear stale expiry of subscription %d failed: %v", sub.ID, err)
			}
			continue
		}
		sub.Status = "expired"
		sub.QuotaState = "expired_days"
		// 配额策略 reset_on_expire：到期即把未用完的额度清零（作废），
		// 续费时按新套餐配额重新发放；accumulate / reset_on_renew 保留原额度。
		if s.resolveQuotaPolicy(ctx, sub.ClientID) == entity.QuotaPolicyResetOnExpire {
			sub.QuotaBytes = 0
			sub.UsedBytesCache = 0
			sub.UploadBytesCache = 0
			sub.DownloadBytesCache = 0
			s.zeroClientQuota(ctx, sub.ClientID)
		}
		if err := s.repo.Update(ctx, sub); err != nil {
			log.Printf("[quota-enforce] expire subscription %d failed: %v", sub.ID, err)
			continue
		}
		if sub.ClientID != 0 {
			s.blockClient(ctx, sub.ClientID, "auto_expired")
		}
	}

	return nil
}

// isPermanentClient 报告该客户的套餐规格是否为「永久有效」。
// 用于让过期任务放过永久用户（老数据可能残留 expires_at）。
func (s *Service) isPermanentClient(ctx context.Context, clientID int64) bool {
	if clientID <= 0 || s.clientRepo == nil || s.planSvc == nil {
		return false
	}
	client, err := s.clientRepo.GetByID(ctx, clientID)
	if err != nil || client == nil || client.PlanVariantID <= 0 {
		return false
	}
	variant, err := s.planSvc.GetVariantByID(ctx, client.PlanVariantID)
	if err != nil || variant == nil {
		return false
	}
	return variant.IsPermanent()
}

// resolveQuotaPolicy 解析客户当前套餐的配额策略（规格 > 套餐 > 累加）。
// 查不到任何配置时按 accumulate 处理，保证存量账号行为不变。
func (s *Service) resolveQuotaPolicy(ctx context.Context, clientID int64) string {
	if clientID <= 0 || s.clientRepo == nil || s.planSvc == nil {
		return entity.QuotaPolicyAccumulate
	}
	client, err := s.clientRepo.GetByID(ctx, clientID)
	if err != nil || client == nil || client.PlanVariantID <= 0 {
		return entity.QuotaPolicyAccumulate
	}
	variant, err := s.planSvc.GetVariantByID(ctx, client.PlanVariantID)
	if err != nil || variant == nil {
		return entity.QuotaPolicyAccumulate
	}
	var tmpl *entity.PlanTemplate
	if variant.TemplateID > 0 {
		if t, terr := s.planSvc.GetTemplateByID(ctx, variant.TemplateID); terr == nil {
			tmpl = t
		}
	}
	return entity.ResolveQuotaPolicy(variant, tmpl)
}

// zeroClientQuota 把客户的配额与已用量归零（到期清零策略）。
func (s *Service) zeroClientQuota(ctx context.Context, clientID int64) {
	if clientID <= 0 || s.clientRepo == nil {
		return
	}
	client, err := s.clientRepo.GetByID(ctx, clientID)
	if err != nil || client == nil {
		return
	}
	if client.QuotaBytes == 0 && client.UsedBytes == 0 {
		return
	}
	client.QuotaBytes = 0
	client.UsedBytes = 0
	if err := s.clientRepo.Update(ctx, client); err != nil {
		log.Printf("[quota-enforce] zero quota for client %d failed: %v", clientID, err)
	}
}

// EnforceQuotaResets 执行到期的周期性配额重置（quota_reset_cycle=monthly 且 next_reset_at 已到）。
// 复用 ResetUsage：同时清零订阅/客户缓存与节点侧计数，然后推进下一次重置时间。
func (s *Service) EnforceQuotaResets(ctx context.Context) error {
	now := time.Now()
	due, err := s.repo.ListDueQuotaResets(ctx, now, 1000)
	if err != nil {
		return err
	}
	for i := range due {
		sub := &due[i]
		if _, err := s.ResetUsage(ctx, sub.ID); err != nil {
			log.Printf("[quota-reset] reset subscription %d failed: %v", sub.ID, err)
			continue
		}
		next := entity.NextQuotaResetAt(sub.QuotaResetCycle, sub.QuotaResetDay, now)
		if next == nil {
			continue
		}
		// 重新读取：ResetUsage 已经把用量清零，用巡检时拿到的旧副本 Save 会把它写回去。
		fresh, gerr := s.repo.GetByID(ctx, sub.ID)
		if gerr != nil {
			log.Printf("[quota-reset] reload subscription %d failed: %v", sub.ID, gerr)
			continue
		}
		fresh.NextResetAt = next
		if err := s.repo.Update(ctx, fresh); err != nil {
			log.Printf("[quota-reset] advance next_reset_at for subscription %d failed: %v", sub.ID, err)
		}
	}
	return nil
}

// StartQuotaResetScheduler 启动周期性的配额重置巡检（每月/自定义周期）。
// 返回的 channel 关闭即停止；调用方负责在进程退出时关闭。
func (s *Service) StartQuotaResetScheduler(interval time.Duration) chan struct{} {
	stopCh := make(chan struct{})
	if interval <= 0 {
		interval = time.Hour
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				if err := s.EnforceQuotaResets(ctx); err != nil {
					log.Printf("[quota-reset] enforce failed: %v", err)
				}
				cancel()
			}
		}
	}()
	return stopCh
}

// blockClient suspends the client (if still active) and removes its credentials from all
// proxy cores, effectively cutting off traffic for the subscription.
// reason 会写入 clients.suspend_reason，决定该账号能否自助解封（auto_quota 可解）。
func (s *Service) blockClient(ctx context.Context, clientID int64, reason string) {
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
	client.SuspendReason = reason
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
	// ⚠ 同理：「激活后有效天数」已决议不做（有效期唯一来源是套餐的 expires_at）→ 写入即拒绝 ✓
	if err := subdomain.ValidateActivationWindow(input.MaxActiveDays); err != nil {
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

	// 短链自动生成失败不能影响订阅创建（只是少一条便捷链接）。
	if s.shortLinks != nil {
		_ = s.shortLinks.AutoCreateForSubscription(ctx, sub)
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
	// ⚠「激活后有效天数」已决议不做 ✗：订阅有效期唯一来源是套餐的 `expires_at`，
	// 这个字段没有任何执行点 —— 写入即拒绝，避免"设了天数却什么都不发生"的虚假预期
	// （做法与上面的 security_mode 一致）。库列保留仅为兼容，不再参与任何判定。
	if err := subdomain.ValidateActivationWindow(input.MaxActiveDays); err != nil {
		return nil, err
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

// Delete deletes a subscription by ID（连同短链一起清理，避免 code 长期占用）。
func (s *Service) Delete(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	if s.shortLinks != nil {
		_ = s.shortLinks.DeleteBySubscription(ctx, id)
	}
	return nil
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
	// 短链默认靠“快照版本不匹配”自动失效；只有 FollowTokenRotation=true 的
	// 短链需要同步到新版本，才能继续可用。
	if s.shortLinks != nil {
		_ = s.shortLinks.SyncTokenVersion(ctx, sub.ID, sub.TokenVersion)
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

// ListAccessLogs retrieves access logs for a subscription.
func (s *Service) ListAccessLogs(ctx context.Context, subID int64, limit, offset int) ([]entity.SubscriptionAccessLog, int64, error) {
	return s.accessLog.ListBySubscriptionID(ctx, subID, limit, offset)
}

// ListRotations retrieves rotation history for a subscription.
func (s *Service) ListRotations(ctx context.Context, subID int64, limit, offset int) ([]entity.SubscriptionRotation, int64, error) {
	return s.rotation.ListBySubscriptionID(ctx, subID, limit, offset)
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
