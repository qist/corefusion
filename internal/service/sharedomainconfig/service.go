package sharedomainconfig

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// DNSUpdater 把域名的 A 记录指向指定 IP（由 dnsrecord.Service 实现）。
// 抽成接口，避免本包依赖具体服务商 SDK。
type DNSUpdater interface {
	UpsertA(ctx context.Context, channelID int64, fqdn, target string, proxied bool) error
}

// Service handles share domain config business logic.
type Service struct {
	repo       *repository.ShareDomainConfigRepository
	dnsUpdater DNSUpdater
}

// NewService creates a new share domain config service.
func NewService(repo *repository.ShareDomainConfigRepository) *Service {
	return &Service{repo: repo}
}

// WithDNSUpdater 注入 DNS 记录更新器（主备入口漂移用）。
func (s *Service) WithDNSUpdater(u DNSUpdater) *Service {
	s.dnsUpdater = u
	return s
}

// DriftForPrimary 把绑定到 primaryServerID 的 direct 入口域名 A 记录指向 targetIP。
// 只处理显式配置了 DNSChannelID 的记录；单条失败不阻断其它记录（返回更新成功数）。
func (s *Service) DriftForPrimary(ctx context.Context, primaryServerID int64, targetIP string) (int, error) {
	if s == nil || s.dnsUpdater == nil {
		return 0, nil
	}
	targetIP = strings.TrimSpace(targetIP)
	if net.ParseIP(targetIP) == nil {
		return 0, fmt.Errorf("入口漂移目标 IP 非法：%q", targetIP)
	}
	all, err := s.repo.List(ctx, "direct")
	if err != nil {
		return 0, fmt.Errorf("读取入口域名失败：%w", err)
	}
	updated := 0
	var firstErr error
	for _, cfg := range all {
		if cfg.DNSChannelID <= 0 || !cfg.MatchServer(primaryServerID) {
			continue
		}
		fqdn := strings.TrimSpace(cfg.DNSRecordName)
		if fqdn == "" {
			fqdn = strings.TrimSpace(cfg.Value)
		}
		if fqdn == "" || net.ParseIP(fqdn) != nil {
			// 值是 IP 的条目不是域名，无法做 DNS 漂移。
			continue
		}
		if err := s.dnsUpdater.UpsertA(ctx, cfg.DNSChannelID, fqdn, targetIP, cfg.DNSProxied); err != nil {
			log.Printf("[EntryDrift] 入口 %s → %s 更新失败：%v", fqdn, targetIP, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		updated++
		log.Printf("[EntryDrift] 入口 %s → %s 已更新（server=%d channel=%d）", fqdn, targetIP, primaryServerID, cfg.DNSChannelID)
	}
	return updated, firstErr
}

// List returns all share domain configs, optionally filtered by type.
func (s *Service) List(ctx context.Context, configType string) ([]entity.ShareDomainConfig, error) {
	return s.repo.List(ctx, configType)
}

// ListByServer returns direct share domain configs for a specific server.
// CF domains are global (server_id=0) and not included here.
// ListByServer returns direct share domain configs effective for a specific server.
// This includes both global (server_id=0) and server-specific (server_id=serverID) direct domains.
// CF domains are global and not included here.
func (s *Service) ListByServer(ctx context.Context, serverID int64) ([]entity.ShareDomainConfig, error) {
	// Get effective configs: global + server-specific
	all, err := s.repo.ListEffectiveForServer(ctx, serverID, "")
	if err != nil {
		return nil, err
	}
	var result []entity.ShareDomainConfig
	for _, d := range all {
		if d.Type == "direct" {
			result = append(result, d)
		}
	}
	return result, nil
}

// Create creates a new share domain config for a specific server.
func (s *Service) Create(ctx context.Context, serverID int64, configType, value string, port int, ports string) (*entity.ShareDomainConfig, error) {
	if port <= 0 {
		port = 443
	}
	if ports == "" {
		ports = "[443]"
	}
	config := &entity.ShareDomainConfig{
		ServerID: serverID,
		Type:     configType,
		Value:    value,
		Port:     port,
		Ports:    ports,
		Enabled:  true,
	}
	config.CreatedAt = time.Now()
	config.UpdatedAt = time.Now()
	if err := s.repo.Create(ctx, config); err != nil {
		return nil, err
	}
	return config, nil
}

// Update updates an existing share domain config.
func (s *Service) Update(ctx context.Context, id int64, configType, value string, port int, ports string, enabled bool) (*entity.ShareDomainConfig, error) {
	config, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if configType != "" {
		config.Type = configType
	}
	if value != "" {
		config.Value = value
	}
	if port > 0 {
		config.Port = port
	}
	if ports != "" {
		config.Ports = ports
	}
	config.Enabled = enabled
	config.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, config); err != nil {
		return nil, err
	}
	return config, nil
}

// Save saves a share domain config directly (used for server_id binding updates).
func (s *Service) Save(ctx context.Context, config *entity.ShareDomainConfig) error {
	return s.repo.Update(ctx, config)
}

// Delete deletes a share domain config by ID.
func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

// ClearAll deletes all share domain configs, optionally filtered by type.
func (s *Service) ClearAll(ctx context.Context, configType string) error {
	return s.repo.ClearAll(ctx, configType)
}

// ClearByServer deletes direct share domain configs for a specific server.
func (s *Service) ClearByServer(ctx context.Context, serverID int64, configType string) error {
	return s.repo.ClearByServer(ctx, serverID, configType)
}

// SeedDefaults seeds default CF domain configs globally (server_id=0).
func (s *Service) SeedDefaults(ctx context.Context) error {
	// Clear existing cf_domain globally first
	if err := s.repo.ClearAll(ctx, "cf_domain"); err != nil {
		return err
	}
	// 清掉历史遗留的 cf_ip 行：这张表只应存 direct / cf_domain，
	// IP 来自 cf_ip_entries。旧版本 seed 把域名写成 cf_ip，订阅逻辑会跳过
	// （public.go 的 loadShareDomains），导致「重置后域名看起来不全」。
	if err := s.repo.ClearAll(ctx, "cf_ip"); err != nil {
		return err
	}

	defaultPorts := "[443]"
	defaults := []entity.ShareDomainConfig{
		{Type: "cf_domain", Value: "mfa.gov.ua", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 1},
		{Type: "cf_domain", Value: "www.shopify.com", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 2},
		{Type: "cf_domain", Value: "www.visa.cn", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 3},
		{Type: "cf_domain", Value: "time.is", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 4},
		{Type: "cf_domain", Value: "icook.hk", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 5},
		{Type: "cf_domain", Value: "icook.tw", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 6},
		{Type: "cf_domain", Value: "cloudflare.182682.xyz", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 7},
		{Type: "cf_domain", Value: "tencentapp.cn", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 8},
		{Type: "cf_domain", Value: "cf.877774.xyz", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 9},
		{Type: "cf_domain", Value: "cfip.xxxxxxxx.tk", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 10},
		{Type: "cf_domain", Value: "bestcf.030101.xyz", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 11},
		{Type: "cf_domain", Value: "777.ai7777777.xyz", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 12},
		{Type: "cf_domain", Value: "cdn.2020111.xyz", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 13},
		{Type: "cf_domain", Value: "cfip.cfcdn.vip", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 14},
		{Type: "cf_domain", Value: "cf.0sm.com", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 15},
		{Type: "cf_domain", Value: "cfip.1323123.xyz", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 16},
		{Type: "cf_domain", Value: "cloudflare-ip.mofashi.ltd", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 17},
		{Type: "cf_domain", Value: "cf.877771.xyz", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 18},
		{Type: "cf_domain", Value: "cf.900501.xyz", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 19},
		{Type: "cf_domain", Value: "cdns.doon.eu.org", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 20},
		{Type: "cf_domain", Value: "fn.130519.xyz", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 21},
		{Type: "cf_domain", Value: "nrt.xxxxxxxx.nyc.mn", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 22},
		{Type: "cf_domain", Value: "nrtcfdns.zone.id", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 23},
		{Type: "cf_domain", Value: "saas.sin.fan", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 24},
		{Type: "cf_domain", Value: "xn--b6gac.eu.org", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 25},
		{Type: "cf_domain", Value: "freeyx.cloudflare88.eu.org", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 26},
	}
	for _, d := range defaults {
		d.CreatedAt = time.Now()
		d.UpdatedAt = time.Now()
		if err := s.repo.Create(ctx, &d); err != nil {
			return err
		}
	}
	return nil
}
