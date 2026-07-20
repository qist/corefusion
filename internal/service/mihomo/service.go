package mihomo

import (
	"context"
	"fmt"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// Service handles mihomo configuration business logic.
type Service struct {
	configRepo    *repository.MihomoConfigRepository
	generalRepo   *repository.MihomoGeneralRepository
	dnsRepo       *repository.MihomoDNSRepository
	hostRepo      *repository.MihomoHostRepository
	tunnelRepo    *repository.MihomoTunnelRepository
	snifferRepo   *repository.MihomoSnifferRepository
	extCtrlRepo   *repository.MihomoExternalControllerRepository
	tlsRepo       *repository.MihomoTLSRepository
	tunRepo       *repository.MihomoTunRepository
	proxyGroupRepo *repository.MihomoProxyGroupRepository
	ruleProviderRepo *repository.MihomoRuleProviderRepository
	ruleRepo      *repository.MihomoRuleRepository
	outboundRepo  *repository.MihomoSpecialOutboundRepository
}

// NewService creates a new mihomo service.
func NewService(
	configRepo *repository.MihomoConfigRepository,
	generalRepo *repository.MihomoGeneralRepository,
	dnsRepo *repository.MihomoDNSRepository,
	hostRepo *repository.MihomoHostRepository,
	tunnelRepo *repository.MihomoTunnelRepository,
	snifferRepo *repository.MihomoSnifferRepository,
	extCtrlRepo *repository.MihomoExternalControllerRepository,
	tlsRepo *repository.MihomoTLSRepository,
	tunRepo *repository.MihomoTunRepository,
	proxyGroupRepo *repository.MihomoProxyGroupRepository,
	ruleProviderRepo *repository.MihomoRuleProviderRepository,
	ruleRepo *repository.MihomoRuleRepository,
	outboundRepo *repository.MihomoSpecialOutboundRepository,
) *Service {
	return &Service{
		configRepo:       configRepo,
		generalRepo:      generalRepo,
		dnsRepo:          dnsRepo,
		hostRepo:         hostRepo,
		tunnelRepo:       tunnelRepo,
		snifferRepo:      snifferRepo,
		extCtrlRepo:      extCtrlRepo,
		tlsRepo:          tlsRepo,
		tunRepo:          tunRepo,
		proxyGroupRepo:   proxyGroupRepo,
		ruleProviderRepo: ruleProviderRepo,
		ruleRepo:         ruleRepo,
		outboundRepo:     outboundRepo,
	}
}

// List returns all mihomo configs.
func (s *Service) List(ctx context.Context) ([]entity.MihomoConfig, error) {
	return s.configRepo.List(ctx)
}

// GetByID returns a mihomo config with all related data.
func (s *Service) GetByID(ctx context.Context, id int64) (*entity.MihomoConfig, map[string]interface{}, error) {
	config, err := s.configRepo.GetByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}

	data := map[string]interface{}{}

	// General
	if general, err := s.generalRepo.GetByConfigID(ctx, id); err == nil {
		data["general"] = general
	}

	// DNS
	if dns, err := s.dnsRepo.GetByConfigID(ctx, id); err == nil {
		data["dns"] = dns
	}

	// Hosts
	if hosts, err := s.hostRepo.ListByConfigID(ctx, id); err == nil {
		data["hosts"] = hosts
	}

	// Tunnels
	if tunnels, err := s.tunnelRepo.ListByConfigID(ctx, id); err == nil {
		data["tunnels"] = tunnels
	}

	// Sniffer
	if sniffer, err := s.snifferRepo.GetByConfigID(ctx, id); err == nil {
		data["sniffer"] = sniffer
	}

	// External Controller
	if extCtrl, err := s.extCtrlRepo.GetByConfigID(ctx, id); err == nil {
		data["external_controller"] = extCtrl
	}

	// TLS
	if tls, err := s.tlsRepo.GetByConfigID(ctx, id); err == nil {
		data["tls"] = tls
	}

	// TUN
	if tun, err := s.tunRepo.GetByConfigID(ctx, id); err == nil {
		data["tun"] = tun
	}

	// Proxy Groups
	if groups, err := s.proxyGroupRepo.ListByConfigID(ctx, id); err == nil {
		data["proxy_groups"] = groups
	}

	// Rule Providers
	if providers, err := s.ruleProviderRepo.ListByConfigID(ctx, id); err == nil {
		data["rule_providers"] = providers
	}

	// Rules
	if rules, err := s.ruleRepo.ListByConfigID(ctx, id); err == nil {
		data["rules"] = rules
	}

	// Special Outbounds
	if outbounds, err := s.outboundRepo.ListByConfigID(ctx, id); err == nil {
		data["special_outbounds"] = outbounds
	}

	return config, data, nil
}

// GetConfigByID returns just the mihomo config (without related data).
func (s *Service) GetConfigByID(ctx context.Context, id int64) (*entity.MihomoConfig, error) {
	return s.configRepo.GetByID(ctx, id)
}

// UpdateConfig updates a mihomo config.
func (s *Service) UpdateConfig(ctx context.Context, config *entity.MihomoConfig) error {
	return s.configRepo.Update(ctx, config)
}

// CreateInput is the input for creating a mihomo config.
type CreateInput struct {
	Name        string           `json:"name" binding:"required"`
	ClientType  string           `json:"client_type"`
	Format      string           `json:"format"`
	Description string           `json:"description"`
	Platform    string           `json:"platform"`
	Enabled     *bool            `json:"enabled"`
	SortOrder   int              `json:"sort_order"`
	ConfigJSON  string           `json:"config_json"`
	PlanIDs     entity.Int64List `json:"plan_ids"`
}

// Create creates a new mihomo config with default settings.
func (s *Service) Create(ctx context.Context, input CreateInput) (*entity.MihomoConfig, error) {
	format := input.Format
	if format == "" {
		format = "mihomo_yaml"
	}
	clientType := input.ClientType
	if clientType == "" {
		clientType = "clash"
	}
	config := &entity.MihomoConfig{
		Name:        input.Name,
		ClientType:  clientType,
		Format:      format,
		Description: input.Description,
		Platform:    input.Platform,
		Enabled:     true,
		SortOrder:   input.SortOrder,
		ConfigJSON:  input.ConfigJSON,
		PlanIDs:     input.PlanIDs,
	}
	if input.Enabled != nil {
		config.Enabled = *input.Enabled
	}
	if err := s.configRepo.Create(ctx, config); err != nil {
		return nil, err
	}

	// Create default general settings
	s.generalRepo.Create(ctx, &entity.MihomoGeneral{
		ConfigID: config.ID,
		Mode:     "rule",
		LogLevel: "info",
		IPv6:     true,
	})

	// Create default DNS settings
	s.dnsRepo.Create(ctx, &entity.MihomoDNS{
		ConfigID:     config.ID,
		Enable:       true,
		Listen:       "0.0.0.0:53",
		EnhancedMode: "redir-host",
	})

	return config, nil
}

// Delete deletes a mihomo config and all related data.
func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.configRepo.Delete(ctx, id)
}

// --- Proxy Group operations ---

func (s *Service) ListProxyGroups(ctx context.Context, configID int64) ([]entity.MihomoProxyGroup, error) {
	return s.proxyGroupRepo.ListByConfigID(ctx, configID)
}

type CreateProxyGroupInput struct {
	Name           string  `json:"name" binding:"required"`
	Type           string  `json:"type"`
	Proxies        string  `json:"proxies"`
	Hidden         bool    `json:"hidden"`
	URL            string  `json:"url"`
	Interval       int     `json:"interval"`
	Lazy           bool    `json:"lazy"`
	Timeout        int     `json:"timeout"`
	MaxFailedTimes int     `json:"max_failed_times"`
	EmptyFallback  string  `json:"empty_fallback"`
	DisableUDP     bool    `json:"disable_udp"`
	Filter         string  `json:"filter"`
	ExcludeFilter  string  `json:"exclude_filter"`
	ExcludeType    string  `json:"exclude_type"`
	IncludeAll     bool    `json:"include_all"`
	InterfaceName  string  `json:"interface_name"`
	RoutingMark    int     `json:"routing_mark"`
}

func (s *Service) CreateProxyGroup(ctx context.Context, configID int64, input CreateProxyGroupInput) (*entity.MihomoProxyGroup, error) {
	group := &entity.MihomoProxyGroup{
		ConfigID:       configID,
		Name:           input.Name,
		Type:           input.Type,
		Proxies:        input.Proxies,
		Hidden:         input.Hidden,
		URL:            input.URL,
		Interval:       input.Interval,
		Lazy:           input.Lazy,
		Timeout:        input.Timeout,
		MaxFailedTimes: input.MaxFailedTimes,
		EmptyFallback:  input.EmptyFallback,
		DisableUDP:     input.DisableUDP,
		Filter:         input.Filter,
		ExcludeFilter:  input.ExcludeFilter,
		ExcludeType:    input.ExcludeType,
		IncludeAll:     input.IncludeAll,
		InterfaceName:  input.InterfaceName,
		RoutingMark:    input.RoutingMark,
	}
	if group.Type == "" {
		group.Type = "select"
	}
	if group.Interval == 0 {
		group.Interval = 300
	}
	if group.Timeout == 0 {
		group.Timeout = 5000
	}
	if group.MaxFailedTimes == 0 {
		group.MaxFailedTimes = 5
	}
	if err := s.proxyGroupRepo.Create(ctx, group); err != nil {
		return nil, err
	}
	return group, nil
}

func (s *Service) DeleteProxyGroup(ctx context.Context, id int64) error {
	return s.proxyGroupRepo.Delete(ctx, id)
}

// --- Rule operations ---

func (s *Service) ListRules(ctx context.Context, configID int64) ([]entity.MihomoRule, error) {
	return s.ruleRepo.ListByConfigID(ctx, configID)
}

type CreateRuleInput struct {
	RuleString string `json:"rule_string" binding:"required"`
	Priority   int    `json:"priority"`
	Enabled    *bool  `json:"enabled"`
}

func (s *Service) CreateRule(ctx context.Context, configID int64, input CreateRuleInput) (*entity.MihomoRule, error) {
	rule := &entity.MihomoRule{
		ConfigID:   configID,
		RuleString: input.RuleString,
		Priority:   input.Priority,
		Enabled:    true,
	}
	if input.Enabled != nil {
		rule.Enabled = *input.Enabled
	}
	if err := s.ruleRepo.Create(ctx, rule); err != nil {
		return nil, err
	}
	return rule, nil
}

func (s *Service) DeleteRule(ctx context.Context, id int64) error {
	return s.ruleRepo.Delete(ctx, id)
}

// --- Rule Provider operations ---

func (s *Service) ListRuleProviders(ctx context.Context, configID int64) ([]entity.MihomoRuleProvider, error) {
	return s.ruleProviderRepo.ListByConfigID(ctx, configID)
}

type CreateRuleProviderInput struct {
	Name         string `json:"name" binding:"required"`
	Type         string `json:"type"`
	Behavior     string `json:"behavior"`
	Format       string `json:"format"`
	URL          string `json:"url"`
	Path         string `json:"path"`
	Proxy        string `json:"proxy"`
	PathInBundle string `json:"path_in_bundle"`
	Interval     int    `json:"interval"`
}

func (s *Service) CreateRuleProvider(ctx context.Context, configID int64, input CreateRuleProviderInput) (*entity.MihomoRuleProvider, error) {
	rp := &entity.MihomoRuleProvider{
		ConfigID:     configID,
		Name:         input.Name,
		Type:         input.Type,
		Behavior:     input.Behavior,
		Format:       input.Format,
		URL:          input.URL,
		Path:         input.Path,
		Proxy:        input.Proxy,
		PathInBundle: input.PathInBundle,
		Interval:     input.Interval,
	}
	if rp.Type == "" {
		rp.Type = "http"
	}
	if rp.Behavior == "" {
		rp.Behavior = "domain"
	}
	if rp.Format == "" {
		rp.Format = "yaml"
	}
	if rp.Proxy == "" {
		rp.Proxy = "DIRECT"
	}
	if rp.Interval == 0 {
		rp.Interval = 86400
	}
	if err := s.ruleProviderRepo.Create(ctx, rp); err != nil {
		return nil, err
	}
	return rp, nil
}

func (s *Service) DeleteRuleProvider(ctx context.Context, id int64) error {
	return s.ruleProviderRepo.Delete(ctx, id)
}

// --- Generate YAML ---

// GenerateYAML generates mihomo YAML config from database.
func (s *Service) GenerateYAML(ctx context.Context, configID int64) (string, error) {
	config, data, err := s.GetByID(ctx, configID)
	if err != nil {
		return "", err
	}
	if !config.Enabled {
		return "", fmt.Errorf("config is disabled")
	}

	yaml := ""

	// General
	if g, ok := data["general"].(*entity.MihomoGeneral); ok {
		yaml += fmt.Sprintf("mixed-port: %d\n", g.MixedPort)
		if g.Port > 0 {
			yaml += fmt.Sprintf("port: %d\n", g.Port)
		}
		if g.SocksPort > 0 {
			yaml += fmt.Sprintf("socks-port: %d\n", g.SocksPort)
		}
		yaml += fmt.Sprintf("allow-lan: %v\n", g.AllowLan)
		yaml += fmt.Sprintf("bind-address: '%s'\n", g.BindAddress)
		yaml += fmt.Sprintf("mode: %s\n", g.Mode)
		yaml += fmt.Sprintf("log-level: %s\n", g.LogLevel)
		yaml += fmt.Sprintf("ipv6: %v\n", g.IPv6)
		yaml += fmt.Sprintf("find-process-mode: %s\n", g.FindProcessMode)
	}

	// DNS
	if d, ok := data["dns"].(*entity.MihomoDNS); ok && d.Enable {
		yaml += "dns:\n"
		yaml += fmt.Sprintf("  enable: true\n")
		yaml += fmt.Sprintf("  listen: %s\n", d.Listen)
		yaml += fmt.Sprintf("  enhanced-mode: %s\n", d.EnhancedMode)
		// Add nameservers, etc.
	}

	// Rules
	if rules, ok := data["rules"].([]entity.MihomoRule); ok && len(rules) > 0 {
		yaml += "rules:\n"
		for _, r := range rules {
			if r.Enabled {
				yaml += fmt.Sprintf("  - %s\n", r.RuleString)
			}
		}
	}

	return yaml, nil
}
