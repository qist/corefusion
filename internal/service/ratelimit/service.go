package ratelimitsvc

import (
	"context"
	"fmt"
	"log"
	"time"

	"corefusion/internal/model/dto"
	"corefusion/internal/model/entity"
	agentpb "corefusion/internal/runtime/agent/proto"
	"corefusion/internal/store/repository"
)

// Service handles rate limit configuration business logic.
type Service struct {
	repo   *repository.RateLimitConfigRepository
	nodeOp NodeOperator
}

// NodeOperator is a minimal interface for sending commands to nodes.
type NodeOperator interface {
	AssociateIP(ctx context.Context, serverID int64, req *agentpb.AssociateIPRequest) error
	ApplyLimits(ctx context.Context, serverID int64, req *agentpb.ApplyLimitsRequest) error
	IsOnline(serverID int64) bool
}

// NewService creates a new rate limit service.
func NewService(repo *repository.RateLimitConfigRepository) *Service {
	return &Service{repo: repo}
}

// SetNodeOperator sets the node operator for sending commands to agents.
func (s *Service) SetNodeOperator(op NodeOperator) {
	s.nodeOp = op
}

// GetConfig retrieves the rate limit config for a server.
// If no config exists, returns a default (disabled) config.
func (s *Service) GetConfig(ctx context.Context, serverID int64) (*dto.RateLimitConfigVO, error) {
	config, err := s.repo.GetByServerID(ctx, serverID)
	if err != nil {
		// No config found — return default
		return &dto.RateLimitConfigVO{
			ServerID:           serverID,
			Enabled:            false,
			Iface:              "",
			IfbDevice:          "ifb0",
			DefaultMaxDownMbps: 0,
			DefaultMaxUpMbps:   0,
			IdleTTLSeconds:     3600,
			CleanupInterval:    300,
			BackendType:        "nft_tc",
		}, nil
	}
	return configToVO(config), nil
}

// UpdateConfig creates or updates the rate limit config for a server.
func (s *Service) UpdateConfig(ctx context.Context, serverID int64, req *dto.RateLimitConfigUpdate) (*dto.RateLimitConfigVO, error) {
	config, err := s.repo.GetByServerID(ctx, serverID)
	if err != nil {
		// Create new config
		config = &entity.RateLimitConfig{
			ServerID:           serverID,
			Enabled:            false,
			Iface:              "",
			IfbDevice:          "ifb0",
			DefaultMaxDownMbps: 0,
			DefaultMaxUpMbps:   0,
			IdleTTLSeconds:     3600,
			CleanupInterval:    300,
			BackendType:        "nft_tc",
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
		}
	}

	// Apply updates
	if req.Enabled != nil {
		config.Enabled = *req.Enabled
	}
	if req.Iface != nil {
		config.Iface = *req.Iface
	}
	if req.IfbDevice != nil {
		config.IfbDevice = *req.IfbDevice
	}
	if req.DefaultMaxDownMbps != nil {
		config.DefaultMaxDownMbps = *req.DefaultMaxDownMbps
	}
	if req.DefaultMaxUpMbps != nil {
		config.DefaultMaxUpMbps = *req.DefaultMaxUpMbps
	}
	if req.IdleTTLSeconds != nil {
		config.IdleTTLSeconds = *req.IdleTTLSeconds
	}
	if req.CleanupInterval != nil {
		config.CleanupInterval = *req.CleanupInterval
	}
	if req.BackendType != nil {
		config.BackendType = *req.BackendType
	}
	config.UpdatedAt = time.Now()

	if err := s.repo.Upsert(ctx, config); err != nil {
		return nil, fmt.Errorf("save rate limit config: %w", err)
	}

	log.Printf("[RateLimitService] updated config for server %d: enabled=%v iface=%s backend=%s",
		serverID, config.Enabled, config.Iface, config.BackendType)

	// Sync the updated config to the Agent if rate limiting is enabled
	if config.Enabled && s.nodeOp != nil {
		go func() {
			if err := s.SyncConfigToAgent(context.Background(), serverID); err != nil {
				log.Printf("[RateLimitService] failed to sync config to agent for server %d: %v", serverID, err)
			}
		}()
	}

	return configToVO(config), nil
}

// GetStatus retrieves the current rate limit status for a server.
func (s *Service) GetStatus(ctx context.Context, serverID int64) (*dto.RateLimitStatusResponse, error) {
	config, err := s.GetConfig(ctx, serverID)
	if err != nil {
		return nil, err
	}

	resp := &dto.RateLimitStatusResponse{
		ServerID: serverID,
		Config:   config,
		Availability: dto.RateLimitAvailability{
			NftAvailable: config.NftAvailable,
			TcAvailable:  config.TcAvailable,
			IfbAvailable: config.IfbAvailable,
		},
	}

	// Active entries would be populated from Agent's MarkRegistry
	// via the RATE_LIMIT_STATUS heartbeat report.
	// For now, return empty until the Agent reporting is implemented.

	return resp, nil
}

// SyncConfigToAgent pushes the current rate limit config to the server's Agent.
// This is called when the admin updates the rate limit configuration for a node.
func (s *Service) SyncConfigToAgent(ctx context.Context, serverID int64) error {
	if s.nodeOp == nil {
		return fmt.Errorf("node operator not configured")
	}
	if !s.nodeOp.IsOnline(serverID) {
		log.Printf("[RateLimitService] server %d is offline, skipping config sync", serverID)
		return nil
	}

	config, err := s.repo.GetByServerID(ctx, serverID)
	if err != nil {
		return fmt.Errorf("get rate limit config: %w", err)
	}

	// Send ApplyLimits with the config so Agent can initialize/update its RateLimitManager
	req := &agentpb.ApplyLimitsRequest{
		Action:         "apply",
		SubscriptionId: 0, // config-level, not per-subscription
		Value:          int32(config.DefaultMaxDownMbps),
		MaxDownMbps:    int32(config.DefaultMaxDownMbps),
		MaxUpMbps:      int32(config.DefaultMaxUpMbps),
	}

	if err := s.nodeOp.ApplyLimits(ctx, serverID, req); err != nil {
		return fmt.Errorf("sync config to server %d: %w", serverID, err)
	}

	log.Printf("[RateLimitService] synced rate limit config to server %d", serverID)
	return nil
}

// PushIPAssociation pushes an IP association to a specific server's Agent.
// This is called when a user accesses their subscription URL.
func (s *Service) PushIPAssociation(ctx context.Context, serverID int64, ip string, subID int64, maxDownMbps, maxUpMbps, connLimit, deviceLimit int32) error {
	if s.nodeOp == nil {
		return fmt.Errorf("node operator not configured")
	}

	if !s.nodeOp.IsOnline(serverID) {
		log.Printf("[RateLimitService] server %d is offline, skipping IP association for %s", serverID, ip)
		return nil // Not an error — server is offline
	}

	req := &agentpb.AssociateIPRequest{
		Ip:             ip,
		SubscriptionId: subID,
		MaxDownMbps:    maxDownMbps,
		MaxUpMbps:      maxUpMbps,
		ConnLimit:      connLimit,
		DeviceLimit:    deviceLimit,
	}

	if err := s.nodeOp.AssociateIP(ctx, serverID, req); err != nil {
		return fmt.Errorf("push IP association to server %d: %w", serverID, err)
	}

	return nil
}

// PushSubscriptionRateUpdate 把某订阅的新限速参数推送到所有启用限流的在线节点。
// 管理端修改客户限速/并发后调用 —— Agent 按 action=update 更新 tc class 并
// 重建该订阅已关联 IP 的 connlimit 规则，**无需等用户重新拉订阅**（立即生效）。
func (s *Service) PushSubscriptionRateUpdate(ctx context.Context, subID int64, maxDownMbps, maxUpMbps, connLimit, deviceLimit int32) error {
	if s.nodeOp == nil || subID <= 0 {
		return nil
	}
	servers, err := s.repo.ListEnabledServers(ctx)
	if err != nil {
		return fmt.Errorf("list rate-limit enabled servers: %w", err)
	}
	pushed := 0
	for _, server := range servers {
		if !s.nodeOp.IsOnline(server.ID) {
			continue
		}
		req := &agentpb.ApplyLimitsRequest{
			Action:         "update",
			SubscriptionId: subID,
			Value:          maxDownMbps,
			MaxDownMbps:    maxDownMbps,
			MaxUpMbps:      maxUpMbps,
			ConnLimit:      connLimit,
			DeviceLimit:    deviceLimit,
		}
		if err := s.nodeOp.ApplyLimits(ctx, server.ID, req); err != nil {
			log.Printf("[RateLimitService] rate update for sub %d on server %d failed: %v", subID, server.ID, err)
			continue
		}
		pushed++
	}
	if pushed > 0 {
		log.Printf("[RateLimitService] rate update pushed for sub %d to %d server(s) (↓%d/↑%dMbps conn=%d device=%d)",
			subID, pushed, maxDownMbps, maxUpMbps, connLimit, deviceLimit)
	}
	return nil
}

// NodeServerLister lists node servers for batch operations.
type NodeServerLister interface {
	ListEnabled(ctx context.Context) ([]entity.NodeServer, error)
}

// PushIPAssociationToAll pushes an IP association to all enabled server Agents.
func (s *Service) PushIPAssociationToAll(ctx context.Context, lister NodeServerLister, ip string, subID int64, maxDownMbps, maxUpMbps, connLimit, deviceLimit int32) {
	if lister == nil || s.nodeOp == nil {
		return
	}
	servers, err := lister.ListEnabled(ctx)
	if err != nil {
		return
	}
	for _, server := range servers {
		if server.ID <= 0 {
			continue
		}
		if err := s.PushIPAssociation(ctx, server.ID, ip, subID, maxDownMbps, maxUpMbps, connLimit, deviceLimit); err != nil {
			log.Printf("[RateLimitService] push IP association to server %d failed: %v", server.ID, err)
		}
	}
}

// RemoveSubscriptionOnAllNodes removes rate limit rules for a subscription from all enabled Agents.
// This implements the quota.RateLimitBlocker interface.
func (s *Service) RemoveSubscriptionOnAllNodes(ctx context.Context, subID int64) {
	if s.nodeOp == nil {
		return
	}

	servers, err := s.repo.ListEnabledServers(ctx)
	if err != nil {
		log.Printf("[RateLimitService] RemoveSubscriptionOnAllNodes: failed to list servers: %v", err)
		return
	}

	for _, server := range servers {
		if !s.nodeOp.IsOnline(server.ID) {
			continue
		}
		req := &agentpb.ApplyLimitsRequest{
			Action:         "remove_subscription",
			SubscriptionId: subID,
		}
		if err := s.nodeOp.ApplyLimits(ctx, server.ID, req); err != nil {
			log.Printf("[RateLimitService] remove subscription %d from server %d failed: %v", subID, server.ID, err)
		}
	}
	log.Printf("[RateLimitService] removed subscription %d from %d online servers", subID, len(servers))
}

// configToVO converts an entity to a VO.
func configToVO(config *entity.RateLimitConfig) *dto.RateLimitConfigVO {
	return &dto.RateLimitConfigVO{
		ID:                 config.ID,
		ServerID:           config.ServerID,
		Enabled:            config.Enabled,
		Iface:              config.Iface,
		IfbDevice:          config.IfbDevice,
		DefaultMaxDownMbps: config.DefaultMaxDownMbps,
		DefaultMaxUpMbps:   config.DefaultMaxUpMbps,
		IdleTTLSeconds:     config.IdleTTLSeconds,
		CleanupInterval:    config.CleanupInterval,
		BackendType:        config.BackendType,
		NftAvailable:       config.NftAvailable,
		TcAvailable:        config.TcAvailable,
		IfbAvailable:       config.IfbAvailable,
		ActiveEntries:      config.ActiveEntries,
		CreatedAt:          config.CreatedAt.Format(time.RFC3339),
		UpdatedAt:          config.UpdatedAt.Format(time.RFC3339),
	}
}
