package configsync

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	coreconfigSvc "corefusion/internal/service/coreconfig"

	agentpb "corefusion/internal/runtime/agent/proto"
	"corefusion/internal/store/repository"
)

// RawDataProvider provides direct database access for building SyncDataRequest.
// This is needed because SyncData transfers raw table rows (not processed configs),
// so the Agent can store them locally for offline startup.
type RawDataProvider interface {
	// ListCoreConfigs returns all core_config rows for a server
	ListCoreConfigs(ctx context.Context, serverID int64) ([]CoreConfigRow, error)
	// ListInboundConfigs returns all inbound_config rows for a server
	ListInboundConfigs(ctx context.Context, serverID int64) ([]InboundConfigRow, error)
	// ListOutboundConfigs returns all outbound_config rows for a server
	ListOutboundConfigs(ctx context.Context, serverID int64) ([]OutboundConfigRow, error)
	// ListWarpConfigs returns active warp config rows for a specific server
	ListWarpConfigs(ctx context.Context, serverID int64) ([]WarpConfigRow, error)
	// ListShareDomainConfigs returns effective share domain configs for a server
	ListShareDomainConfigs(ctx context.Context, serverID int64) ([]ShareDomainConfigRow, error)
}

// CoreConfigRow represents a row from core_configs table.
type CoreConfigRow = repository.CoreConfigRow

// InboundConfigRow represents a row from inbound_configs table.
type InboundConfigRow = repository.InboundConfigRow

// OutboundConfigRow represents a row from outbound_configs table.
type OutboundConfigRow = repository.OutboundConfigRow

// WarpConfigRow represents a row from warp_configs table.
type WarpConfigRow = repository.WarpConfigRow

// ShareDomainConfigRow represents a row from share_domain_configs table.
type ShareDomainConfigRow = repository.ShareDomainConfigRow

// RepositoryDataProvider implements RawDataProvider using panel repositories.
type RepositoryDataProvider struct {
	coreConfigRepo     *repository.CoreConfigRepository
	inboundConfigRepo  *repository.InboundConfigRepository
	outboundConfigRepo *repository.OutboundConfigRepository
	warpRepo           *repository.WarpRepository
	shareDomainRepo    *repository.ShareDomainConfigRepository
	certRepo           *repository.CertificateRepository
	serverRepo         *repository.NodeServerRepository
	proxySiteRepo      *repository.ProxySiteRepository
	caddySiteRepo      *repository.CaddySiteRepository
	nginxSiteRepo      *repository.NginxSiteRepository
	systemSettingRepo  *repository.SystemSettingRepository
}

// NewRepositoryDataProvider creates a new RawDataProvider backed by repositories.
func NewRepositoryDataProvider(
	coreConfigRepo *repository.CoreConfigRepository,
	inboundConfigRepo *repository.InboundConfigRepository,
	outboundConfigRepo *repository.OutboundConfigRepository,
	warpRepo *repository.WarpRepository,
	shareDomainRepo *repository.ShareDomainConfigRepository,
) *RepositoryDataProvider {
	return &RepositoryDataProvider{
		coreConfigRepo:     coreConfigRepo,
		inboundConfigRepo:  inboundConfigRepo,
		outboundConfigRepo: outboundConfigRepo,
		warpRepo:           warpRepo,
		shareDomainRepo:    shareDomainRepo,
	}
}

// WithCertificateProvider injects certificate and server repositories for
// collecting TLS certificates during SyncData.
func (p *RepositoryDataProvider) WithCertificateProvider(certRepo *repository.CertificateRepository, serverRepo *repository.NodeServerRepository) *RepositoryDataProvider {
	p.certRepo = certRepo
	p.serverRepo = serverRepo
	return p
}

// WithProxySiteProvider injects proxy/caddy/nginx site repositories for
// collecting TLS certificates referenced by reverse proxy sites (Caddy/Nginx).
// This ensures that certificates used by proxy sites are also synced to remote agents.
func (p *RepositoryDataProvider) WithProxySiteProvider(proxySiteRepo *repository.ProxySiteRepository, caddySiteRepo *repository.CaddySiteRepository, nginxSiteRepo *repository.NginxSiteRepository) *RepositoryDataProvider {
	p.proxySiteRepo = proxySiteRepo
	p.caddySiteRepo = caddySiteRepo
	p.nginxSiteRepo = nginxSiteRepo
	return p
}

// WithSystemSettingProvider injects system setting repository for
// collecting nginx/caddy settings and log output config during SyncData.
func (p *RepositoryDataProvider) WithSystemSettingProvider(systemSettingRepo *repository.SystemSettingRepository) *RepositoryDataProvider {
	p.systemSettingRepo = systemSettingRepo
	return p
}

func (p *RepositoryDataProvider) ListCoreConfigs(ctx context.Context, serverID int64) ([]CoreConfigRow, error) {
	return p.coreConfigRepo.ListRows(ctx, serverID)
}

func (p *RepositoryDataProvider) ListInboundConfigs(ctx context.Context, serverID int64) ([]InboundConfigRow, error) {
	return p.inboundConfigRepo.ListRows(ctx, serverID)
}

func (p *RepositoryDataProvider) ListOutboundConfigs(ctx context.Context, serverID int64) ([]OutboundConfigRow, error) {
	return p.outboundConfigRepo.ListRows(ctx, serverID)
}

func (p *RepositoryDataProvider) ListWarpConfigs(ctx context.Context, serverID int64) ([]WarpConfigRow, error) {
	return p.warpRepo.ListRows(ctx, serverID)
}

func (p *RepositoryDataProvider) ListShareDomainConfigs(ctx context.Context, serverID int64) ([]ShareDomainConfigRow, error) {
	return p.shareDomainRepo.ListRows(ctx, serverID)
}

// DataSyncBuilder implements agent.DataSyncProvider using configsync.Service.
// It builds SyncDataRequest messages from the panel database for a specific server.
//
// Important: it generates the final runtime config using PrepareFullConfig on the
// panel side and sends it as config_content. The Agent can use this directly
// without needing to re-implement the full config preparation pipeline.
// The raw DB rows are also included so the Agent can persist them locally
// for offline startup scenarios.
type DataSyncBuilder struct {
	rawProvider     RawDataProvider
	coreSvc         CoreConfigService
	inboundSvc      InboundConfigService
	outboundSvc     OutboundConfigService
	logModeProvider LogModeProvider
}

// NewDataSyncBuilder creates a new DataSyncBuilder.
func NewDataSyncBuilder(rawProvider RawDataProvider) *DataSyncBuilder {
	return &DataSyncBuilder{rawProvider: rawProvider}
}

// WithConfigProviders sets the config service providers for generating
// the final runtime config via PrepareFullConfig.
func (b *DataSyncBuilder) WithConfigProviders(coreSvc CoreConfigService, inboundSvc InboundConfigService, outboundSvc OutboundConfigService) *DataSyncBuilder {
	b.coreSvc = coreSvc
	b.inboundSvc = inboundSvc
	b.outboundSvc = outboundSvc
	return b
}

// WithLogModeProvider sets the log mode provider for per-server log mode lookup.
// This is used by PrepareFullConfig to generate the correct log section in the
// runtime config for each remote node.
func (b *DataSyncBuilder) WithLogModeProvider(p LogModeProvider) *DataSyncBuilder {
	b.logModeProvider = p
	return b
}

// BuildSyncDataRequest builds a SyncDataRequest from the panel database
// for the given server ID.
//
// It does two things:
//  1. Collects raw DB table rows (core_configs, inbound_configs, etc.)
//     so the Agent can persist them to its local SQLite for offline use.
//  2. Generates the final runtime config using PrepareFullConfig (same logic
//     as configsync.Service.syncCoreForServer) and puts it in config_content.
//     The Agent writes this directly to its runtime directory — no need
//     to re-implement the full preparation pipeline on the Agent side.
func (b *DataSyncBuilder) BuildSyncDataRequest(ctx context.Context, serverID int64) (*agentpb.SyncDataRequest, error) {
	req := &agentpb.SyncDataRequest{
		ServerId:      serverID,
		ConfigVersion: time.Now().Unix(),
		ApplyAndStart: true,
	}

	// === Part 1: Raw DB rows for Agent local persistence ===

	// 1. Core configs (all sections for all core types)
	coreConfigs, err := b.rawProvider.ListCoreConfigs(ctx, serverID)
	if err != nil {
		log.Printf("[DataSyncBuilder] failed to list core configs for server %d: %v", serverID, err)
	} else {
		req.CoreConfigs = make([]*agentpb.CoreConfigEntry, len(coreConfigs))
		for i, c := range coreConfigs {
			req.CoreConfigs[i] = &agentpb.CoreConfigEntry{
				Id:         c.ID,
				ServerId:   c.ServerID,
				CoreType:   c.CoreType,
				Section:    c.Section,
				ConfigJson: c.ConfigJSON,
				UpdatedAt:  c.UpdatedAt.Unix(),
			}
		}
		log.Printf("[DataSyncBuilder] server %d: %d core config entries", serverID, len(coreConfigs))
	}

	// 2. Inbound configs
	inboundConfigs, err := b.rawProvider.ListInboundConfigs(ctx, serverID)
	if err != nil {
		log.Printf("[DataSyncBuilder] failed to list inbound configs for server %d: %v", serverID, err)
	} else {
		req.InboundConfigs = make([]*agentpb.InboundConfigEntry, len(inboundConfigs))
		for i, c := range inboundConfigs {
			req.InboundConfigs[i] = &agentpb.InboundConfigEntry{
				Id:         c.ID,
				Remark:     c.Remark,
				CoreType:   c.CoreType,
				Protocol:   c.Protocol,
				Tag:        c.Tag,
				Listen:     c.Listen,
				Port:       int32(c.Port),
				ConfigJson: c.ConfigJSON,
				Enabled:    c.Enabled,
				ServerId:   c.ServerID,
				Region:     c.Region,
				CreatedAt:  c.CreatedAt.Unix(),
				UpdatedAt:  c.UpdatedAt.Unix(),
			}
		}
		log.Printf("[DataSyncBuilder] server %d: %d inbound config entries", serverID, len(inboundConfigs))
	}

	// 3. Outbound configs
	outboundConfigs, err := b.rawProvider.ListOutboundConfigs(ctx, serverID)
	if err != nil {
		log.Printf("[DataSyncBuilder] failed to list outbound configs for server %d: %v", serverID, err)
	} else {
		req.OutboundConfigs = make([]*agentpb.OutboundConfigEntry, len(outboundConfigs))
		for i, c := range outboundConfigs {
			req.OutboundConfigs[i] = &agentpb.OutboundConfigEntry{
				Id:         c.ID,
				ServerId:   c.ServerID,
				Remark:     c.Remark,
				CoreType:   c.CoreType,
				Protocol:   c.Protocol,
				Tag:        c.Tag,
				ConfigJson: c.ConfigJSON,
				Enabled:    c.Enabled,
				IsDefault:  c.IsDefault,
				CreatedAt:  c.CreatedAt.Unix(),
				UpdatedAt:  c.UpdatedAt.Unix(),
			}
		}
		log.Printf("[DataSyncBuilder] server %d: %d outbound config entries", serverID, len(outboundConfigs))
	}

	// 4. WARP configs (per-server, not global)
	warpConfigs, err := b.rawProvider.ListWarpConfigs(ctx, serverID)
	if err != nil {
		log.Printf("[DataSyncBuilder] failed to list warp configs: %v", err)
	} else {
		req.WarpConfigs = make([]*agentpb.WarpConfigEntry, len(warpConfigs))
		for i, c := range warpConfigs {
			var regAt int64
			if !c.RegistrationAt.IsZero() {
				regAt = c.RegistrationAt.Unix()
			}
			req.WarpConfigs[i] = &agentpb.WarpConfigEntry{
				Id:             c.ID,
				DeviceId:       c.DeviceID,
				Token:          c.Token,
				PrivateKey:     c.PrivateKey,
				PublicKey:      c.PublicKey,
				Reserved:       c.Reserved,
				AddressIpv4:    c.AddressIPv4,
				AddressIpv6:    c.AddressIPv6,
				Endpoint:       c.Endpoint,
				ClientId:       c.ClientID,
				AccountId:      c.AccountID,
				AccountType:    c.AccountType,
				LicenseKey:     c.LicenseKey,
				RegistrationAt: regAt,
				Status:         c.Status,
				ConfigJson:     c.ConfigJSON,
				CreatedAt:      c.CreatedAt.Unix(),
				UpdatedAt:      c.UpdatedAt.Unix(),
				ServerId:       c.ServerID,
				CoreType:       c.CoreType,
				Remark:         c.Remark,
			}
		}
		log.Printf("[DataSyncBuilder] %d warp config entries", len(warpConfigs))
	}

	// 5. Share domain configs (effective for this server)
	shareDomainConfigs, err := b.rawProvider.ListShareDomainConfigs(ctx, serverID)
	if err != nil {
		log.Printf("[DataSyncBuilder] failed to list share domain configs for server %d: %v", serverID, err)
	} else {
		req.ShareDomainConfigs = make([]*agentpb.ShareDomainConfigEntry, len(shareDomainConfigs))
		for i, c := range shareDomainConfigs {
			req.ShareDomainConfigs[i] = &agentpb.ShareDomainConfigEntry{
				Id:        c.ID,
				Type:      c.Type,
				Value:     c.Value,
				Port:      int32(c.Port),
				Ports:     c.Ports,
				Enabled:   c.Enabled,
				SortOrder: int32(c.SortOrder),
				ServerId:  c.ServerID,
				Region:    c.Region,
				CreatedAt: c.CreatedAt.Unix(),
				UpdatedAt: c.UpdatedAt.Unix(),
			}
		}
		log.Printf("[DataSyncBuilder] server %d: %d share domain config entries", serverID, len(shareDomainConfigs))
	}

	// === Part 2: Generate final runtime config via PrepareFullConfig ===
	// This is the same logic as configsync.Service.syncCoreForServer().
	// The Agent can use this config directly — no need to re-assemble from DB rows.

	if b.coreSvc != nil && b.inboundSvc != nil && b.outboundSvc != nil {
		coreType := "xray"
		fullConfig, err := b.coreSvc.GetFull(ctx, serverID, coreType)
		if err != nil {
			log.Printf("[DataSyncBuilder] failed to get full config for server %d: %v", serverID, err)
		} else {
			configData, err := coreconfigSvc.PrepareFullConfig(coreconfigSvc.PrepareFullConfigOptions{
				Ctx:              ctx,
				ServerID:         serverID,
				CoreType:         coreType,
				FullConfig:       fullConfig,
				InboundProvider:  b.inboundSvc,
				OutboundProvider: b.outboundSvc,
				LogMode:          b.resolveLogMode(ctx, serverID, coreType),
			})
			if err != nil {
				log.Printf("[DataSyncBuilder] PrepareFullConfig failed for server %d: %v", serverID, err)
			} else {
				req.ConfigContent = configData
				req.ConfigCoreType = coreType
				log.Printf("[DataSyncBuilder] server %d: generated final config via PrepareFullConfig (%d bytes)", serverID, len(configData))
			}
		}
	} else {
		log.Printf("[DataSyncBuilder] config providers not set, skipping PrepareFullConfig")
	}

	// === Part 3: Collect TLS certificates used by this server ===
	// Scan inbound configs for certificateFile references and match against
	// the certificates table. Also include node_servers.certificate_id if set.
	// All matching certificates are sent as PEM content so the Agent can land
	// them on local disk at the correct paths.
	req.Certificates = b.collectCertificates(ctx, serverID, inboundConfigs)
	if len(req.Certificates) > 0 {
		log.Printf("[DataSyncBuilder] server %d: %d certificate entries", serverID, len(req.Certificates))
	}

	// === Part 4: Collect Nginx/Caddy settings and log output config ===
	// These are needed by the Agent to generate correct proxy configs on startup.
	rdp, ok := b.rawProvider.(*RepositoryDataProvider)
	if ok && rdp.systemSettingRepo != nil {
		// Nginx settings
		nginxKey := perNodeSettingsKey("proxy.nginx_settings", serverID)
		if v, err := rdp.systemSettingRepo.GetByKey(ctx, nginxKey); err == nil && v != "" {
			req.NginxSettings = v
			log.Printf("[DataSyncBuilder] server %d: nginx settings from %s (%d bytes)", serverID, nginxKey, len(v))
		} else if v, err := rdp.systemSettingRepo.GetByKey(ctx, "proxy.nginx_settings"); err == nil && v != "" {
			req.NginxSettings = v
			log.Printf("[DataSyncBuilder] server %d: nginx settings from global (%d bytes)", serverID, len(v))
		}

		// Log output config
		logOutputKey := perNodeSettingsKey("proxy.log_output_config", serverID)
		if v, err := rdp.systemSettingRepo.GetByKey(ctx, logOutputKey); err == nil && v != "" {
			req.LogOutputConfig = v
			log.Printf("[DataSyncBuilder] server %d: log output config from %s (%d bytes)", serverID, logOutputKey, len(v))
		} else if v, err := rdp.systemSettingRepo.GetByKey(ctx, "proxy.log_output_config"); err == nil && v != "" {
			req.LogOutputConfig = v
			log.Printf("[DataSyncBuilder] server %d: log output config from global (%d bytes)", serverID, len(v))
		}

		// Proxy manually_stopped config (for auto-start on agent startup)
		nginxManuallyStoppedKey := perNodeSettingsKey("proxy.nginx.manually_stopped", serverID)
		if v, err := rdp.systemSettingRepo.GetByKey(ctx, nginxManuallyStoppedKey); err == nil && v == "true" {
			req.NginxManuallyStopped = true
		} else if v, err := rdp.systemSettingRepo.GetByKey(ctx, "proxy.nginx.manually_stopped"); err == nil && v == "true" {
			req.NginxManuallyStopped = true
		}
		caddyManuallyStoppedKey := perNodeSettingsKey("proxy.caddy.manually_stopped", serverID)
		if v, err := rdp.systemSettingRepo.GetByKey(ctx, caddyManuallyStoppedKey); err == nil && v == "true" {
			req.CaddyManuallyStopped = true
		} else if v, err := rdp.systemSettingRepo.GetByKey(ctx, "proxy.caddy.manually_stopped"); err == nil && v == "true" {
			req.CaddyManuallyStopped = true
		}
	}

	// Log summary
	total := len(req.CoreConfigs) + len(req.InboundConfigs) + len(req.OutboundConfigs) + len(req.WarpConfigs) + len(req.ShareDomainConfigs) + len(req.Certificates)
	hasConfig := len(req.ConfigContent) > 0
	log.Printf("[DataSyncBuilder] built SyncDataRequest for server %d: %d total entries, config_content=%v, certs=%d", serverID, total, hasConfig, len(req.Certificates))

	return req, nil
}

// collectCertificates scans inbound configs for TLS certificate file references,
// matches them against the certificates table, and returns CertificateEntry
// list with PEM content for the Agent to land on disk.
//
// It also includes the node-level certificate (node_servers.certificate_id)
// if one is associated.
func (b *DataSyncBuilder) collectCertificates(ctx context.Context, serverID int64, inboundConfigs []InboundConfigRow) []*agentpb.CertificateEntry {
	rdp, ok := b.rawProvider.(*RepositoryDataProvider)
	if !ok || rdp.certRepo == nil {
		return nil
	}

	// Collect certificate paths referenced in inbound configJSON
	certPathSet := make(map[string]bool) // key = cert path (normalized)

	for _, ic := range inboundConfigs {
		if !ic.Enabled {
			continue
		}
		var inbound map[string]interface{}
		if err := json.Unmarshal([]byte(ic.ConfigJSON), &inbound); err != nil {
			continue
		}
		// Navigate: streamSettings.tlsSettings.certificates[].certificateFile
		ss, ok := inbound["streamSettings"].(map[string]interface{})
		if !ok {
			continue
		}
		tls, ok := ss["tlsSettings"].(map[string]interface{})
		if !ok {
			continue
		}
		certs, ok := tls["certificates"].([]interface{})
		if !ok {
			continue
		}
		for _, c := range certs {
			cm, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			if certFile, ok := cm["certificateFile"].(string); ok && certFile != "" {
				// Normalize path: strip leading "../../../" or "./" prefixes
				normalized := normalizeCertPath(certFile)
				certPathSet[normalized] = true
			}
		}
	}

	// Also check hysteria2/tuic core configs for cert/key path references
	// (these use a flat tls section with cert/key fields)
	if rdp.serverRepo != nil {
		if server, err := rdp.serverRepo.GetByID(ctx, serverID); err == nil {
			if server.CertificateID > 0 {
				// Node-level certificate: fetch and add its paths
				if cert, err := rdp.certRepo.GetByID(ctx, server.CertificateID); err == nil {
					certPathSet[normalizeCertPath(cert.CertPath)] = true
				}
			}
		}
	}

	// Also scan proxy/caddy/nginx site tables for TLS certificate references.
	// Caddy/Nginx configs reference cert/key paths via site.CertPath/site.KeyPath,
	// and those cert files must be landed on the remote agent alongside xray certs.
	b.collectProxySiteCertificates(ctx, serverID, certPathSet)

	if len(certPathSet) == 0 {
		return nil
	}

	// Load all certificates from DB and match by path
	allCerts, _, err := rdp.certRepo.List(ctx, 1000, 0)
	if err != nil {
		log.Printf("[DataSyncBuilder] failed to list certificates: %v", err)
		return nil
	}

	var entries []*agentpb.CertificateEntry
	for _, cert := range allCerts {
		if cert.CertContent == "" || cert.KeyContent == "" {
			continue
		}
		normalizedCertPath := normalizeCertPath(cert.CertPath)
		if !certPathSet[normalizedCertPath] {
			continue
		}
		fullchainContent := ""
		if cert.FullchainPath != "" {
			// Try to read fullchain from disk if not in DB
			fullchainContent = cert.CertContent // fallback: use cert content as fullchain
		}
		entries = append(entries, &agentpb.CertificateEntry{
			Id:               cert.ID,
			Domain:           cert.Domain,
			CertPath:         cert.CertPath,
			KeyPath:          cert.KeyPath,
			FullchainPath:    cert.FullchainPath,
			CertContent:      cert.CertContent,
			KeyContent:       cert.KeyContent,
			FullchainContent: fullchainContent,
			ExpiresAt:        cert.ExpiresAt.Unix(),
		})
	}

	return entries
}

// collectProxySiteCertificates scans proxy_sites, caddy_sites, and nginx_sites
// tables for TLS certificate path references (CertPath/KeyPath fields) and adds
// them to the certPathSet. This ensures certificates used by Caddy/Nginx reverse
// proxy sites are also synced to remote agents.
func (b *DataSyncBuilder) collectProxySiteCertificates(ctx context.Context, serverID int64, certPathSet map[string]bool) {
	rdp, ok := b.rawProvider.(*RepositoryDataProvider)
	if !ok {
		return
	}

	// Scan legacy proxy_sites table (generic Manager)
	if rdp.proxySiteRepo != nil {
		if sites, err := rdp.proxySiteRepo.List(ctx, serverID); err == nil {
			for _, site := range sites {
				if !site.Enabled {
					continue
				}
				if site.CertPath != "" {
					certPathSet[normalizeCertPath(site.CertPath)] = true
				}
			}
		}
	}

	// Scan caddy_sites table (CaddyManager)
	if rdp.caddySiteRepo != nil {
		if sites, err := rdp.caddySiteRepo.List(ctx, serverID); err == nil {
			for _, site := range sites {
				if !site.Enabled {
					continue
				}
				if site.CertPath != "" {
					certPathSet[normalizeCertPath(site.CertPath)] = true
				}
			}
		}
	}

	// Scan nginx_sites table (NginxManager)
	if rdp.nginxSiteRepo != nil {
		if sites, err := rdp.nginxSiteRepo.List(ctx, serverID); err == nil {
			for _, site := range sites {
				if !site.Enabled {
					continue
				}
				if site.CertPath != "" {
					certPathSet[normalizeCertPath(site.CertPath)] = true
				}
			}
		}
	}
}

// CollectProxyCertificates collects TLS certificates referenced by proxy sites
// (Caddy/Nginx) for a specific server. This is used by SyncProxyConfig to land
// certificate files on remote agents alongside the proxy config.
//
// It scans proxy_sites, caddy_sites, and nginx_sites for CertPath references,
// matches them against the certificates table, and returns CertificateEntry
// list with PEM content.
func (b *DataSyncBuilder) CollectProxyCertificates(ctx context.Context, serverID int64) []*agentpb.CertificateEntry {
	rdp, ok := b.rawProvider.(*RepositoryDataProvider)
	if !ok || rdp.certRepo == nil {
		return nil
	}

	certPathSet := make(map[string]bool)
	b.collectProxySiteCertificates(ctx, serverID, certPathSet)

	if len(certPathSet) == 0 {
		return nil
	}

	// Load all certificates from DB and match by path
	allCerts, _, err := rdp.certRepo.List(ctx, 1000, 0)
	if err != nil {
		log.Printf("[DataSyncBuilder] CollectProxyCertificates: failed to list certificates: %v", err)
		return nil
	}

	var entries []*agentpb.CertificateEntry
	for _, cert := range allCerts {
		if cert.CertContent == "" || cert.KeyContent == "" {
			continue
		}
		normalizedCertPath := normalizeCertPath(cert.CertPath)
		if !certPathSet[normalizedCertPath] {
			continue
		}
		fullchainContent := ""
		if cert.FullchainPath != "" {
			fullchainContent = cert.CertContent
		}
		entries = append(entries, &agentpb.CertificateEntry{
			Id:               cert.ID,
			Domain:           cert.Domain,
			CertPath:         cert.CertPath,
			KeyPath:          cert.KeyPath,
			FullchainPath:    cert.FullchainPath,
			CertContent:      cert.CertContent,
			KeyContent:       cert.KeyContent,
			FullchainContent: fullchainContent,
			ExpiresAt:        cert.ExpiresAt.Unix(),
		})
	}

	log.Printf("[DataSyncBuilder] server %d: collected %d proxy certificates", serverID, len(entries))
	return entries
}

// normalizeCertPath strips common relative path prefixes so that paths
// like "../../../certs/gg/cert.pem" and "certs/gg/cert.pem" match.
func normalizeCertPath(path string) string {
	for strings.HasPrefix(path, "../") {
		path = strings.TrimPrefix(path, "../")
	}
	for strings.HasPrefix(path, "./") {
		path = strings.TrimPrefix(path, "./")
	}
	return path
}

// MarshalSyncDataRequest serializes a SyncDataRequest to JSON for debugging.
func MarshalSyncDataRequest(req *agentpb.SyncDataRequest) ([]byte, error) {
	summary := map[string]interface{}{
		"server_id":            req.ServerId,
		"config_version":       req.ConfigVersion,
		"apply_and_start":     req.ApplyAndStart,
		"core_configs":         len(req.CoreConfigs),
		"inbound_configs":      len(req.InboundConfigs),
		"outbound_configs":    len(req.OutboundConfigs),
		"warp_configs":         len(req.WarpConfigs),
		"share_domain_configs": len(req.ShareDomainConfigs),
		"has_config_content":  len(req.ConfigContent) > 0,
		"config_core_type":    req.ConfigCoreType,
	}
	return json.MarshalIndent(summary, "", "  ")
}

// ValidateSyncDataRequest validates that a SyncDataRequest has the minimum required data.
func ValidateSyncDataRequest(req *agentpb.SyncDataRequest) error {
	if req.ServerId <= 0 {
		return fmt.Errorf("invalid server_id: %d", req.ServerId)
	}
	if len(req.CoreConfigs) == 0 && len(req.ConfigContent) == 0 {
		return fmt.Errorf("no core_configs or config_content provided for server %d", req.ServerId)
	}
	return nil
}

// resolveLogMode returns the log mode for a given server and core type.
// Uses the per-server config if available, falling back to the global config.
func (b *DataSyncBuilder) resolveLogMode(ctx context.Context, serverID int64, coreType string) string {
	if b.logModeProvider != nil {
		return b.logModeProvider.GetLogModeForServer(ctx, serverID, coreType)
	}
	return ""
}

// perNodeSettingsKey returns the settings key for a server ID.
// Local node (serverID=1) uses the legacy global key for backward compatibility.
// Remote nodes use a per-node key.
func perNodeSettingsKey(baseKey string, serverID int64) string {
	if serverID <= 1 {
		return baseKey
	}
	return fmt.Sprintf("%s.%d", baseKey, serverID)
}
