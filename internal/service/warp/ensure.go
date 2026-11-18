package warp

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"corefusion/internal/model/entity"
)

// EnsureResult 是「确保 WARP/MASQUE 出站存在」的结果。
type EnsureResult struct {
	Tag     string `json:"tag"`
	Mode    string `json:"mode"`
	Created bool   `json:"created"`
}

// 固定的出站/端点 tag：路由规则按 tag 引用，重复点击导入时复用同一条。
// 注意：两个核心的存储位置不同（xray → outbound_configs，sing-box → endpoint_configs），
// tag 相同也不会互相覆盖；但记录本身必须按 core_type 隔离。
const (
	WarpOutboundTag   = "warp-wg"
	MasqueOutboundTag = "warp-masque" // 仅 xray（sing-box 未来支持时另起 tag）
)

// EnsureOutbound 确保目标节点上存在指定模式的 WARP 出站，并返回可被路由引用的 tag。
//   - xray: wireguard 出站（内核 TUN，noKernelTun=false）
//   - xray + masque: protocol=masque 出站（用户态）
//   - singbox: wireguard endpoint（system=true，即开启内核 TUN）
//
// 已存在同名出站时直接复用，不再重复注册 Cloudflare 设备。
func (s *Service) EnsureOutbound(ctx context.Context, serverID int64, coreType, mode string) (*EnsureResult, error) {
	if serverID <= 0 {
		return nil, fmt.Errorf("invalid server id")
	}
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "warp"
	}

	switch coreType {
	case "xray":
		if mode == "masque" {
			return s.ensureXrayMasque(ctx, serverID)
		}
		return s.ensureXrayWarp(ctx, serverID)
	case "singbox":
		if mode == "masque" {
			// 扩展点：sing-box 的 masque-client 目前不能用于 WARP（边缘不下发
			// ADDRESS_ASSIGN）。支持后在这里实现 ensureSingboxMasque，写入
			// endpoint_configs（masque-client endpoint），不能复用 xray 的出站表。
			return nil, fmt.Errorf("sing-box 暂不支持 MASQUE 出站（请使用 WARP WireGuard）")
		}
		return s.ensureSingboxWarp(ctx, serverID)
	default:
		return nil, fmt.Errorf("unsupported core type: %s", coreType)
	}
}

func (s *Service) ensureXrayWarp(ctx context.Context, serverID int64) (*EnsureResult, error) {
	if s.outboundRepo == nil {
		return nil, fmt.Errorf("outbound repository not configured")
	}
	if cfg, err := s.outboundRepo.GetByTag(ctx, serverID, WarpOutboundTag); err == nil && cfg != nil {
		return &EnsureResult{Tag: cfg.Tag, Mode: "warp"}, nil
	}

	cfg, err := s.activeWarpConfig(ctx, serverID, "xray")
	if err != nil {
		return nil, err
	}

	raw, err := buildXrayWarpOutbound(cfg, WarpOutboundTag)
	if err != nil {
		return nil, err
	}
	var configMap map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &configMap); err != nil {
		return nil, err
	}
	maxOrder, _ := s.outboundRepo.GetMaxSortOrder(ctx, serverID, "xray")
	ob := &entity.OutboundConfig{
		ServerID:   serverID,
		CoreType:   "xray",
		Remark:     "WARP",
		Protocol:   "wireguard",
		Tag:        WarpOutboundTag,
		ConfigJSON: raw,
		Enabled:    true,
		SortOrder:  maxOrder + 1,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := s.outboundRepo.Create(ctx, ob); err != nil {
		return nil, fmt.Errorf("create warp outbound: %w", err)
	}
	return &EnsureResult{Tag: WarpOutboundTag, Mode: "warp", Created: true}, nil
}

func (s *Service) ensureXrayMasque(ctx context.Context, serverID int64) (*EnsureResult, error) {
	if s.outboundRepo == nil {
		return nil, fmt.Errorf("outbound repository not configured")
	}
	if cfg, err := s.outboundRepo.GetByTag(ctx, serverID, MasqueOutboundTag); err == nil && cfg != nil {
		return &EnsureResult{Tag: cfg.Tag, Mode: "masque"}, nil
	}

	cfg, err := s.activeMasqueConfig(ctx, serverID, "xray")
	if err != nil {
		return nil, err
	}
	res, err := s.CreateMasqueOutbound(serverID, cfg.ID, MasqueOutboundInput{
		Tag:    MasqueOutboundTag,
		Remark: "WARP MASQUE",
	})
	if err != nil {
		return nil, err
	}
	return &EnsureResult{Tag: res.Tag, Mode: "masque", Created: true}, nil
}

func (s *Service) ensureSingboxWarp(ctx context.Context, serverID int64) (*EnsureResult, error) {
	if s.endpointRepo == nil {
		return nil, fmt.Errorf("endpoint repository not configured")
	}
	existing, err := s.endpointRepo.List(ctx, serverID, "singbox")
	if err == nil {
		for _, ep := range existing {
			if ep.Tag == WarpOutboundTag {
				return &EnsureResult{Tag: ep.Tag, Mode: "warp"}, nil
			}
		}
	}

	cfg, err := s.activeWarpConfig(ctx, serverID, "singbox")
	if err != nil {
		return nil, err
	}
	raw, err := buildSingboxWarpEndpoint(cfg, WarpOutboundTag)
	if err != nil {
		return nil, err
	}
	ep := &entity.EndpointConfig{
		Remark:       "WARP",
		CoreType:     "singbox",
		EndpointType: "wireguard",
		Tag:          WarpOutboundTag,
		ConfigJSON:   raw,
		Enabled:      true,
		ServerID:     serverID,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := s.endpointRepo.Create(ctx, ep); err != nil {
		return nil, fmt.Errorf("create warp endpoint: %w", err)
	}
	return &EnsureResult{Tag: WarpOutboundTag, Mode: "warp", Created: true}, nil
}

// activeWarpConfig 返回可用于 WireGuard 出站的注册记录；没有则现场注册一台。
//
// 必须按核心类型精确匹配：xray 与 sing-box 的 WARP 设备不能共用同一条记录
// （两者会用同一对密钥/同一 reserved 建隧道，互抢会话）。core_type 为空的历史
// 记录只用于手动展示，不再被自动附加复用；需要时按类型新注册一条。
func (s *Service) activeWarpConfig(ctx context.Context, serverID int64, coreType string) (*entity.WarpConfig, error) {
	cfgs, _ := s.repo.ListByServer(ctx, serverID)
	for _, cfg := range cfgs {
		if strings.EqualFold(cfg.TunnelType, masqueTunnel) {
			continue
		}
		if !strings.EqualFold(cfg.CoreType, coreType) {
			continue
		}
		if cfg.Status != "" && cfg.Status != "not_registered" && cfg.PrivateKey != "" {
			return cfg, nil
		}
	}
	status, err := s.Register(serverID, coreType)
	if err != nil {
		return nil, fmt.Errorf("register warp: %w", err)
	}
	if status == nil || status.ID == 0 {
		return nil, fmt.Errorf("register warp: empty result")
	}
	cfg, err := s.repo.GetByID(ctx, status.ID)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func (s *Service) activeMasqueConfig(ctx context.Context, serverID int64, coreType string) (*entity.WarpConfig, error) {
	cfgs, _ := s.repo.ListByServer(ctx, serverID)
	for _, cfg := range cfgs {
		if !strings.EqualFold(cfg.TunnelType, masqueTunnel) {
			continue
		}
		if !strings.EqualFold(cfg.CoreType, coreType) {
			continue
		}
		if cfg.PrivateKey != "" && cfg.PublicKey != "" {
			return cfg, nil
		}
	}
	if _, err := s.RegisterMasque(serverID, coreType, "route-import"); err != nil {
		return nil, fmt.Errorf("register masque: %w", err)
	}
	cfgs, _ = s.repo.ListByServer(ctx, serverID)
	for _, cfg := range cfgs {
		if strings.EqualFold(cfg.TunnelType, masqueTunnel) && strings.EqualFold(cfg.CoreType, coreType) {
			return cfg, nil
		}
	}
	return nil, fmt.Errorf("register masque: no usable config")
}

// buildXrayWarpOutbound 生成 xray wireguard 出站配置。
// noKernelTun=false 表示启用内核 TUN（与出站表单的默认值一致）。
func buildXrayWarpOutbound(cfg *entity.WarpConfig, tag string) (string, error) {
	if cfg == nil || cfg.PrivateKey == "" || cfg.PublicKey == "" {
		return "", fmt.Errorf("WARP 记录缺少 WireGuard 凭据")
	}
	addresses := make([]string, 0, 2)
	if cfg.AddressIPv4 != "" {
		addresses = append(addresses, ensureCIDR(cfg.AddressIPv4, 32))
	}
	if cfg.AddressIPv6 != "" {
		addresses = append(addresses, ensureCIDR(cfg.AddressIPv6, 128))
	}
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = "engage.cloudflareclient.com:2408"
	}
	if _, _, err := net.SplitHostPort(endpoint); err != nil {
		endpoint = net.JoinHostPort(strings.Trim(endpoint, "[]"), "2408")
	}

	ob := map[string]interface{}{
		"tag":      tag,
		"protocol": "wireguard",
		"settings": map[string]interface{}{
			"secretKey":      cfg.PrivateKey,
			"address":        addresses,
			"mtu":            1280,
			"reserved":       parseReserved(cfg.Reserved),
			"noKernelTun":    false,
			"domainStrategy": "ForceIP",
			"remoteDNS":      []string{"1.1.1.1", "1.0.0.1", "2606:4700:4700::1111", "2606:4700:4700::1001"},
			"peers": []map[string]interface{}{{
				"publicKey":  cfg.PublicKey,
				"endpoint":   endpoint,
				"allowedIPs": []string{"0.0.0.0/0", "::/0"},
			}},
		},
	}
	data, err := json.Marshal(ob)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// buildSingboxWarpEndpoint 生成 sing-box wireguard endpoint（system=true → 内核 TUN）。
func buildSingboxWarpEndpoint(cfg *entity.WarpConfig, tag string) (string, error) {
	if cfg == nil || cfg.PrivateKey == "" || cfg.PublicKey == "" {
		return "", fmt.Errorf("WARP 记录缺少 WireGuard 凭据")
	}
	addresses := make([]string, 0, 2)
	if cfg.AddressIPv4 != "" {
		addresses = append(addresses, ensureCIDR(cfg.AddressIPv4, 32))
	}
	if cfg.AddressIPv6 != "" {
		addresses = append(addresses, ensureCIDR(cfg.AddressIPv6, 128))
	}
	host, port := splitHostPortDefault(cfg.Endpoint, "engage.cloudflareclient.com", 2408)

	ep := map[string]interface{}{
		"type":        "wireguard",
		"tag":         tag,
		"system":      true,
		"name":        ifaceNameForTag(tag),
		"mtu":         1280,
		"address":     addresses,
		"private_key": cfg.PrivateKey,
		"peers": []map[string]interface{}{{
			"address":                       host,
			"port":                          port,
			"public_key":                    cfg.PublicKey,
			"allowed_ips":                   []string{"0.0.0.0/0", "::/0"},
			"persistent_keepalive_interval": 30,
			"reserved":                      parseReserved(cfg.Reserved),
		}},
	}
	data, err := json.Marshal(ep)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// parseReserved 把 "[148,154,190]" / "148,154,190" 解析成 []int。
func parseReserved(s string) []int {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	out := make([]int, 0, 3)
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			return nil
		}
		out = append(out, n)
	}
	return out
}

func splitHostPortDefault(endpoint, defaultHost string, defaultPort int) (string, int) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return defaultHost, defaultPort
	}
	host, portStr, err := net.SplitHostPort(endpoint)
	if err == nil {
		port, perr := strconv.Atoi(portStr)
		if perr == nil && port > 0 {
			return host, port
		}
		return host, defaultPort
	}
	return strings.Trim(endpoint, "[]"), defaultPort
}

// ifaceNameForTag 生成 ≤15 字节的合法网卡名（sing-box system 端点用它创建内核接口）。
func ifaceNameForTag(tag string) string {
	clean := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return -1
	}, strings.ToLower(tag))
	clean = strings.Trim(clean, "-_")
	if len(clean) > 14 {
		clean = clean[:14]
	}
	if clean == "" {
		clean = "warp0"
	}
	return clean
}
