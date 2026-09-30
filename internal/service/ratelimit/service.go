package ratelimitsvc

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"gorm.io/gorm"

	"corefusion/internal/model/dto"
	"corefusion/internal/model/entity"
	runtimeagent "corefusion/internal/runtime/agent"
	agentpb "corefusion/internal/runtime/agent/proto"
	ratelimit "corefusion/internal/runtime/agent/ratelimit"
	"corefusion/internal/store/repository"
)

// Service handles rate limit configuration business logic.
type Service struct {
	repo   *repository.RateLimitConfigRepository
	nodeOp NodeOperator
}

// ErrNodeUnsupported 表示节点的内核/依赖不支持限流（启用被拒绝时返回，Handler 映射为 400）。
type ErrNodeUnsupported struct{ Reason string }

func (e *ErrNodeUnsupported) Error() string {
	return "node does not support rate limiting: " + e.Reason
}

// InitLocalBackend 初始化**本地（面板进程内）**限速后端 —— 等价于 Agent 端的
// initRateLimitWith：iface 自动探测 → 创建管理器 → Init → 按持久化 DB 重建 IP 关联 → 清理循环。
// 面板启用限速（配置级 apply）与面板启动时的自动初始化都走这里。
// ⚠ main.go 与 router 的 fallback LocalAgent 构造路径**都必须**调用本函数挂载管理器，
// 否则该实例的 AssociateIP/ApplyLimits 会报 "rate limit manager not configured"。
func InitLocalBackend(ctx context.Context, database *gorm.DB, localAgent *runtimeagent.LocalAgent, maxDown, maxUp int32) error {
	var localServer entity.NodeServer
	if err := database.WithContext(ctx).Where("is_local = ?", true).First(&localServer).Error; err != nil {
		return fmt.Errorf("local server row not found: %w", err)
	}
	var config entity.RateLimitConfig
	if err := database.WithContext(ctx).Where("server_id = ?", localServer.ID).First(&config).Error; err != nil {
		return fmt.Errorf("rate limit config not found: %w", err)
	}
	if !config.Enabled {
		return fmt.Errorf("rate limiting disabled for local server %d", localServer.ID)
	}

	iface := config.Iface
	if iface == "" {
		out, err := exec.Command("sh", "-c", "ip -o -4 route show default | awk '{print $5}' | head -n1").CombinedOutput()
		iface = strings.TrimSpace(string(out))
		if err != nil || iface == "" {
			return fmt.Errorf("rate limit: no network interface found")
		}
		if iface != config.Iface {
			config.Iface = iface
			_ = database.WithContext(ctx).Save(&config).Error
		}
	}

	mgr := ratelimit.NewManager(iface)
	localAgent.WithRateLimitManager(mgr)
	localServerID := localServer.ID
	mgr.WithPersistFunc(
		func(ip string, subID int64, maxDownMbps, maxUpMbps, connLimit, deviceLimit int32) {
			entry := entity.RateLimitIPEntry{
				ServerID:       localServerID,
				IP:             ip,
				SubscriptionID: subID,
				MaxDownMbps:    maxDownMbps,
				MaxUpMbps:      maxUpMbps,
				ConnLimit:      connLimit,
				DeviceLimit:    deviceLimit,
				LastSeen:       time.Now(),
				CreatedAt:      time.Now(),
			}
			if err := database.Where("ip = ? AND server_id = ?", ip, localServerID).
				Assign(entry).FirstOrCreate(&entry).Error; err != nil {
				log.Printf("[RateLimitService] persist local IP entry failed: %v", err)
			}
		},
		func(ip string) {
			database.Where("ip = ? AND server_id = ?", ip, localServerID).
				Delete(&entity.RateLimitIPEntry{})
		},
	)

	if err := mgr.Init(ctx); err != nil {
		return fmt.Errorf("init nft/tc backend on iface=%s: %w", iface, err)
	}

	// 重建持久化的 IP 关联（含并发/设备数限制）
	var entries []entity.RateLimitIPEntry
	if err := database.WithContext(ctx).Where("server_id = ?", localServerID).Find(&entries).Error; err == nil {
		for _, entry := range entries {
			rate := &ratelimit.RateConfig{
				MaxDownMbps: entry.MaxDownMbps,
				MaxUpMbps:   entry.MaxUpMbps,
				ConnLimit:   entry.ConnLimit,
				DeviceLimit: entry.DeviceLimit,
			}
			if err := mgr.RestoreFromDB(ctx, entry.IP, entry.SubscriptionID, rate); err != nil {
				log.Printf("[RateLimitService] restore local rate limit for ip=%s failed: %v", entry.IP, err)
			}
		}
	}

	if config.IdleTTLSeconds > 0 && config.CleanupInterval > 0 {
		mgr.WithCleanupTTL(time.Duration(config.IdleTTLSeconds) * time.Second)
		mgr.StartCleanupLoop(time.Duration(config.CleanupInterval) * time.Second)
	}

	log.Printf("[RateLimitService] local rate limit initialized: iface=%s, default=↓%dMbps/↑%dMbps",
		iface, config.DefaultMaxDownMbps, config.DefaultMaxUpMbps)
	return nil
}

// NodeOperator is a minimal interface for sending commands to nodes.
type NodeOperator interface {
	AssociateIP(ctx context.Context, serverID int64, req *agentpb.AssociateIPRequest) error
	ApplyLimits(ctx context.Context, serverID int64, req *agentpb.ApplyLimitsRequest) error
	IsOnline(serverID int64) bool
	// GetRateLimitStatus 查询节点**实时**限速状态与能力探测（内核/依赖是否支持）。
	GetRateLimitStatus(ctx context.Context, serverID int64, subID int64) (*dto.AgentRateLimitStatus, error)
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
// Availability 字段来自 Agent **实时探测**（在线时；离线/查询失败回退 DB 值）。
func (s *Service) GetConfig(ctx context.Context, serverID int64) (*dto.RateLimitConfigVO, error) {
	config, err := s.repo.GetByServerID(ctx, serverID)
	if err != nil {
		// No config found — return default
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
		}
	}
	vo := configToVO(config)

	// 实时探测：DB 里的 availability 是静态值（从不刷新），不能作为"支持/不支持"依据。
	// ⚠ 不用 IsOnline 做前置门：本地节点（面板 in-process Agent）没有 gRPC 连接记录，
	// IsOnline 恒 false → 本地面板永远显示"不支持"。改为直接查询，失败再回退 DB
	// （离线远程节点 SendCommand 会立刻报错，不会阻塞）。
	if s.nodeOp != nil {
		if live, err := s.nodeOp.GetRateLimitStatus(ctx, serverID, 0); err == nil && live != nil {
			avail := live.Availability()
			vo.NftAvailable = avail.NftAvailable
			vo.TcAvailable = avail.TcAvailable
			vo.IfbAvailable = avail.IfbAvailable
			vo.ConnTrackAvailable = avail.ConnTrackAvailable
			vo.KernelSupported = avail.KernelSupported
			vo.UnsupportedReason = avail.UnsupportedReason
			vo.ActiveEntries = len(live.Entries)
		}
	}
	return vo, nil
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

	// 启用前 fail-close：节点**实时探测**不支持限流（缺 nft/tc 或内核太旧）→ 拒绝保存，
	// 避免"开关打开了但节点上什么都不会发生"的假配置。
	// （不设 IsOnline 前置门：本地节点同样要过检查；离线节点查询立即失败 → 跳过检查放行）
	if config.Enabled && s.nodeOp != nil {
		if live, err := s.nodeOp.GetRateLimitStatus(ctx, serverID, 0); err == nil && live != nil {
			if !live.Availability().OverallSupported() {
				reason := live.UnsupportedReason
				if reason == "" {
					reason = "missing required tools (nft/tc)"
				}
				return nil, &ErrNodeUnsupported{Reason: reason}
			}
		}
	}

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

	// 实时查询在线 Agent：能力探测 + 活跃条目（DB 里的 availability 是静态值，从不刷新 ✗）
	// 同 GetConfig：不设 IsOnline 前置门，本地节点（in-process）也要能查到。
	if s.nodeOp != nil {
		if live, err := s.nodeOp.GetRateLimitStatus(ctx, serverID, 0); err == nil && live != nil {
			resp.Availability = live.Availability()
			resp.ActiveEntries = live.Entries
		} else if err != nil {
			log.Printf("[RateLimitService] live status query for server %d failed, fallback to DB: %v", serverID, err)
		}
	}

	return resp, nil
}

// SyncConfigToAgent pushes the current rate limit config to the server's Agent.
// This is called when the admin updates the rate limit configuration for a node.
// ⚠ 不做 IsOnline 前置门：本地节点（面板 in-process Agent）没有 gRPC 连接记录，
// IsOnline 恒 false → 本地面板启用限速后配置永远下不去。本地走 in-process 委托
// （ApplyLimits → initRateLimitOnDemand），离线远程节点 SendCommand 立刻报错。
func (s *Service) SyncConfigToAgent(ctx context.Context, serverID int64) error {
	if s.nodeOp == nil {
		return fmt.Errorf("node operator not configured")
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
// ⚠ 不做 IsOnline 前置门：本地节点要能收到关联（离线远程由底层立刻报错跳过）。
func (s *Service) PushIPAssociation(ctx context.Context, serverID int64, ip string, subID int64, maxDownMbps, maxUpMbps, connLimit, deviceLimit int32) error {
	if s.nodeOp == nil {
		return fmt.Errorf("node operator not configured")
	}

	// 仅对启用限流的节点推送（此前注释声称有检查但实现遗漏 → 未启用节点
	// 的 Agent 反复收到 AssociateIP 报 "not initialized" 日志噪音）。
	if cfg, err := s.repo.GetByServerID(ctx, serverID); err != nil || cfg == nil || !cfg.Enabled {
		return nil
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
		req := &agentpb.ApplyLimitsRequest{
			Action:         "update",
			SubscriptionId: subID,
			Value:          maxDownMbps,
			MaxDownMbps:    maxDownMbps,
			MaxUpMbps:      maxUpMbps,
			ConnLimit:      connLimit,
			DeviceLimit:    deviceLimit,
		}
		// 本地节点（in-process）也要能收到更新；离线远程 ApplyLimits 立刻报错 → 记日志跳过
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
		req := &agentpb.ApplyLimitsRequest{
			Action:         "remove_subscription",
			SubscriptionId: subID,
		}
		// 本地节点也要清理（离线远程立刻报错 → 记日志跳过）
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
