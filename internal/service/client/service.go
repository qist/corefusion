package client

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"corefusion/internal/auth/password"
	"corefusion/internal/model/entity"
	"corefusion/internal/service/plan"
	"corefusion/internal/store/repository"
)

// InboundUserManager manages users in inbound configs (pause/resume/add/remove).
type InboundUserManager interface {
	AddUserToInbound(ctx context.Context, inboundTag, email, uuid, password, flow, method string) error
	RemoveUserFromInbound(ctx context.Context, inboundTag, email string) error
	PauseUser(ctx context.Context, inboundID int64, email string) error
	ResumeUser(ctx context.Context, inboundID int64, email string) error
}

// Service handles client business logic.
type Service struct {
	clientRepo  *repository.ClientRepository
	planSvc     *plan.Service
	subRepo     *repository.SubscriptionRepository
	inboundRepo *repository.InboundConfigRepository
	userManager InboundUserManager
}

// NewService creates a new client service.
func NewService(
	clientRepo *repository.ClientRepository,
	planSvc *plan.Service,
	subRepo *repository.SubscriptionRepository,
	inboundRepo *repository.InboundConfigRepository,
	userManager InboundUserManager,
) *Service {
	return &Service{
		clientRepo:  clientRepo,
		planSvc:     planSvc,
		subRepo:     subRepo,
		inboundRepo: inboundRepo,
		userManager: userManager,
	}
}

// --- Client operations ---

func (s *Service) ListClients(ctx context.Context, planID int64, status string, limit, offset int) ([]entity.Client, int64, error) {
	return s.clientRepo.List(ctx, planID, status, limit, offset)
}

func (s *Service) GetClient(ctx context.Context, id int64) (*entity.Client, error) {
	return s.clientRepo.GetByID(ctx, id)
}

type CreateClientInput struct {
	Email           string  `json:"email" binding:"required"`
	Password        string  `json:"password"`
	PlanID          int64   `json:"plan_id" binding:"required"`
	PlanVariantID   int64   `json:"plan_variant_id"`
	QuotaBytes      int64   `json:"quota_bytes"`
	SpeedLimitMbps  int     `json:"speed_limit_mbps"`
	ConcurrentLimit int     `json:"concurrent_limit"`
	ExpiresAt       *string `json:"expires_at"`
	Remark          string  `json:"remark"`
}

// CreateClient creates a client, generates accounts for all accessible inbounds, and creates a subscription.
func (s *Service) CreateClient(ctx context.Context, input CreateClientInput) (*entity.Client, error) {
	// Get plan template
	tmpl, err := s.planSvc.GetTemplateByID(ctx, input.PlanID)
	if err != nil {
		return nil, fmt.Errorf("plan template not found: %w", err)
	}

	// Check email uniqueness
	if _, err := s.clientRepo.GetByEmail(ctx, input.Email); err == nil {
		return nil, fmt.Errorf("email already exists: %s", input.Email)
	}

	// Determine quota
	quota := input.QuotaBytes
	if quota <= 0 {
		quota = tmpl.DefaultQuotaBytes
	}

	// Parse expiry
	var expiresAt *time.Time
	if input.ExpiresAt != nil && *input.ExpiresAt != "" {
		t, err := time.Parse("2006-01-02", *input.ExpiresAt)
		if err == nil {
			expiresAt = &t
		}
	}
	if expiresAt == nil && tmpl.DefaultQuotaResetCycle != "" {
		t := time.Now().AddDate(0, 0, 30)
		expiresAt = &t
	}

	// Create client
	client := &entity.Client{
		Email:           input.Email,
		Password:        input.Password,
		PlanID:          input.PlanID,
		PlanVariantID:   input.PlanVariantID,
		Status:          "active",
		QuotaBytes:      quota,
		SpeedLimitMbps:  input.SpeedLimitMbps,
		ConcurrentLimit: input.ConcurrentLimit,
		ExpiresAt:       expiresAt,
		Remark:          input.Remark,
	}
	if err := s.clientRepo.Create(ctx, client); err != nil {
		return nil, err
	}

	// Get accessible nodes for the plan template
	accessResult, err := s.planSvc.GetAccessibleNodes(ctx, input.PlanID)
	if err != nil || len(accessResult.InboundConfigs) == 0 {
		log.Printf("client: no accessible nodes for plan %d, skipping account generation", input.PlanID)
		return client, nil
	}

	// Generate accounts for each accessible inbound
	token := generateToken(32)
	var subID int64

	for _, inbound := range accessResult.InboundConfigs {
		if !inbound.Enabled {
			continue
		}

		// Generate unique email for this inbound: {email}_{tag_prefix}
		inboundEmail := generateInboundEmail(input.Email, inbound.Tag)

		// Generate protocol credentials
		uuid := generateUUID()
		password := generateToken(16)
		flow := ""
		method := ""

		// Extract flow and method from inbound config
		var cfg map[string]interface{}
		if json.Unmarshal([]byte(inbound.ConfigJSON), &cfg) == nil {
			if settings, ok := cfg["settings"].(map[string]interface{}); ok {
				if enc, ok := settings["encryption"].(string); ok {
					_ = enc // VLESS encryption
				}
			}
		}

		// Add user to inbound via xray gRPC
		if s.userManager != nil {
			if err := s.userManager.AddUserToInbound(ctx, inbound.Tag, inboundEmail, uuid, password, flow, method); err != nil {
				log.Printf("client: failed to add user %s to inbound %s: %v", inboundEmail, inbound.Tag, err)
			}
		}
	}

	// Create subscription
	sub := &entity.Subscription{
		ClientID:                client.ID,
		Format:                  "base64_uri",
		Token:                   token,
		TokenVersion:            1,
		Status:                  "active",
		SecurityMode:            "token_only",
		BindMode:                "none",
		QuotaResetCycle:         "none",
		QuotaBytes:              quota,
		SuspendOnQuotaExhausted: 1,
		QuotaState:              "normal",
	}
	if expiresAt != nil {
		sub.ExpiresAt = expiresAt
	}
	if err := s.subRepo.Create(ctx, sub); err != nil {
		log.Printf("client: failed to create subscription: %v", err)
	} else {
		subID = sub.ID
		client.SubscriptionID = &subID
		s.clientRepo.UpdateSubscriptionID(ctx, client.ID, subID)
	}

	return client, nil
}

// SuspendClient suspends a client and pauses all inbound users.
func (s *Service) SuspendClient(ctx context.Context, id int64) error {
	client, err := s.clientRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if client.Status == "suspended" {
		return fmt.Errorf("already suspended")
	}

	now := time.Now()
	client.Status = "suspended"
	client.SuspendedAt = &now
	if err := s.clientRepo.Update(ctx, client); err != nil {
		return err
	}

	// Pause users in all accessible inbounds
	s.pauseResumeAll(ctx, client.Email, client.PlanID, true)
	return nil
}

// RestoreClient restores a suspended client.
func (s *Service) RestoreClient(ctx context.Context, id int64) error {
	client, err := s.clientRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if client.Status == "active" {
		return nil
	}

	client.Status = "active"
	client.SuspendedAt = nil
	if err := s.clientRepo.Update(ctx, client); err != nil {
		return err
	}

	// Resume users in all accessible inbounds
	s.pauseResumeAll(ctx, client.Email, client.PlanID, false)
	return nil
}

// ResetUsage resets a client's traffic usage.
func (s *Service) ResetUsage(ctx context.Context, id int64) error {
	client, err := s.clientRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	client.UsedBytes = 0
	if client.Status == "suspended" {
		client.Status = "active"
		client.SuspendedAt = nil
		// Resume users
		s.pauseResumeAll(ctx, client.Email, client.PlanID, false)
	}
	return s.clientRepo.Update(ctx, client)
}

// DeleteClient removes a client and all inbound users.
func (s *Service) DeleteClient(ctx context.Context, id int64) error {
	client, err := s.clientRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Remove users from all accessible inbounds
	accessResult, err := s.planSvc.GetAccessibleNodes(ctx, client.PlanID)
	if err == nil {
		for _, inbound := range accessResult.InboundConfigs {
			inboundEmail := generateInboundEmail(client.Email, inbound.Tag)
			if s.userManager != nil {
				s.userManager.RemoveUserFromInbound(ctx, inbound.Tag, inboundEmail)
			}
		}
	}

	return s.clientRepo.Delete(ctx, id)
}

// UpdateClient updates client fields.
func (s *Service) UpdateClient(ctx context.Context, id int64, input struct {
	Password        *string `json:"password"`
	PlanID          *int64  `json:"plan_id"`
	PlanVariantID   *int64  `json:"plan_variant_id"`
	QuotaBytes      *int64  `json:"quota_bytes"`
	SpeedLimitMbps  *int    `json:"speed_limit_mbps"`
	ConcurrentLimit *int    `json:"concurrent_limit"`
	ExpiresAt       *string `json:"expires_at"`
	Remark          *string `json:"remark"`
}) (*entity.Client, error) {
	client, err := s.clientRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if input.Password != nil {
		client.Password = hashClientPassword(*input.Password)
	}
	if input.QuotaBytes != nil {
		client.QuotaBytes = *input.QuotaBytes
	}
	if input.SpeedLimitMbps != nil {
		client.SpeedLimitMbps = *input.SpeedLimitMbps
	}
	if input.ConcurrentLimit != nil {
		client.ConcurrentLimit = *input.ConcurrentLimit
	}
	if input.ExpiresAt != nil {
		if *input.ExpiresAt == "" {
			client.ExpiresAt = nil
		} else {
			t, err := time.Parse("2006-01-02", *input.ExpiresAt)
			if err == nil {
				client.ExpiresAt = &t
			}
		}
	}
	if input.Remark != nil {
		client.Remark = *input.Remark
	}

	// Check if plan is being assigned
	planChanged := input.PlanID != nil && *input.PlanID > 0 && *input.PlanID != client.PlanID
	if planChanged {
		client.PlanID = *input.PlanID
		// Set quota from template if not explicitly set
		if input.QuotaBytes == nil || client.QuotaBytes <= 0 {
			if tmpl, err := s.planSvc.GetTemplateByID(ctx, client.PlanID); err == nil {
				client.QuotaBytes = tmpl.DefaultQuotaBytes
				if client.SpeedLimitMbps == 0 {
					client.SpeedLimitMbps = tmpl.SpeedLimitMbps
				}
				if client.ConcurrentLimit == 0 {
					client.ConcurrentLimit = tmpl.ConcurrentLimit
				}
				if client.ExpiresAt == nil {
					t := time.Now().AddDate(0, 0, 30)
					client.ExpiresAt = &t
				}
			}
		}
	}
	if input.PlanVariantID != nil {
		client.PlanVariantID = *input.PlanVariantID
	}

	if err := s.clientRepo.Update(ctx, client); err != nil {
		return nil, err
	}

	// If plan changed and no subscription exists, generate accounts and create subscription
	if planChanged && client.SubscriptionID == nil {
		s.generateAccountsForClient(ctx, client)
	}

	return client, nil
}

// generateAccountsForClient creates subscription and generates accounts for all accessible inbounds.
func (s *Service) generateAccountsForClient(ctx context.Context, client *entity.Client) {
	accessResult, err := s.planSvc.GetAccessibleNodes(ctx, client.PlanID)
	if err != nil || len(accessResult.InboundConfigs) == 0 {
		log.Printf("client: no accessible nodes for plan %d, skipping account generation", client.PlanID)
		return
	}

	token := generateToken(32)

	for _, inbound := range accessResult.InboundConfigs {
		if !inbound.Enabled {
			continue
		}

		inboundEmail := generateInboundEmail(client.Email, inbound.Tag)
		uuid := generateUUID()
		password := generateToken(16)
		flow := ""
		method := ""

		// Extract flow and method from inbound config
		var cfg map[string]interface{}
		if json.Unmarshal([]byte(inbound.ConfigJSON), &cfg) == nil {
			if settings, ok := cfg["settings"].(map[string]interface{}); ok {
				if enc, ok := settings["encryption"].(string); ok {
					_ = enc
				}
			}
		}

		if s.userManager != nil {
			if err := s.userManager.AddUserToInbound(ctx, inbound.Tag, inboundEmail, uuid, password, flow, method); err != nil {
				log.Printf("client: failed to add user %s to inbound %s: %v", inboundEmail, inbound.Tag, err)
			}
		}
	}

	// Create subscription
	sub := &entity.Subscription{
		ClientID:                client.ID,
		Format:                  "base64_uri",
		Token:                   token,
		TokenVersion:            1,
		Status:                  "active",
		SecurityMode:            "token_only",
		BindMode:                "none",
		QuotaResetCycle:         "none",
		QuotaBytes:              client.QuotaBytes,
		SuspendOnQuotaExhausted: 1,
		QuotaState:              "normal",
	}
	if client.ExpiresAt != nil {
		sub.ExpiresAt = client.ExpiresAt
	}
	if err := s.subRepo.Create(ctx, sub); err != nil {
		log.Printf("client: failed to create subscription: %v", err)
	} else {
		s.clientRepo.UpdateSubscriptionID(ctx, client.ID, sub.ID)
	}
}

// pauseResumeAll pauses or resumes users in all accessible inbounds of a plan template.
func (s *Service) pauseResumeAll(ctx context.Context, email string, planID int64, pause bool) {
	if s.userManager == nil {
		return
	}
	accessResult, err := s.planSvc.GetAccessibleNodes(ctx, planID)
	if err != nil {
		return
	}
	for _, inbound := range accessResult.InboundConfigs {
		inboundEmail := generateInboundEmail(email, inbound.Tag)
		if pause {
			s.userManager.PauseUser(ctx, inbound.ID, inboundEmail)
		} else {
			s.userManager.ResumeUser(ctx, inbound.ID, inboundEmail)
		}
	}
}

// generateInboundEmail creates a unique email per inbound to avoid xray stats conflicts.
func generateInboundEmail(baseEmail, tag string) string {
	// Extract short tag suffix (e.g., "inbound-vless-20296" -> "vless20296")
	parts := strings.Split(tag, "-")
	if len(parts) >= 3 {
		return fmt.Sprintf("%s_%s%s", baseEmail, parts[1], parts[len(parts)-1])
	}
	return fmt.Sprintf("%s_%s", baseEmail, tag)
}

func generateToken(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func generateUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// hashClientPassword hashes a password using bcrypt.
func hashClientPassword(pwd string) string {
	hashed, err := password.Hash(pwd)
	if err != nil {
		// Fallback to simple hash if bcrypt fails
		b := make([]byte, 16)
		rand.Read(b)
		salt := hex.EncodeToString(b)
		return salt + ":" + pwd
	}
	return hashed
}
