package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"corefusion/internal/model/entity"
	coreconfigSvc "corefusion/internal/service/coreconfig"
	"corefusion/internal/store/db"
	"corefusion/internal/store/repository"

	agentpb "corefusion/internal/runtime/agent/proto"

	"gorm.io/gorm/clause"
)

// handleSyncData processes a SyncDataRequest from the panel.
// It writes the received data to the local SQLite database, then optionally
// generates the runtime config and starts the core process.
//
// The flow is:
//  1. Write core_configs, inbound_configs, outbound_configs, warp_configs,
//     share_domain_configs to local DB (upsert by ID)
//  2. If apply_and_start is true, generate runtime config from local DB
//  3. If apply_and_start is true, start the core process
func (a *AgentApp) handleSyncData(ctx context.Context, req *agentpb.SyncDataRequest) (*agentpb.SyncDataResponse, error) {
	resp := &agentpb.SyncDataResponse{}

	// Use standard DB (no debug logging)
	database := db.DB

	// 1. Upsert core_configs
	for _, entry := range req.CoreConfigs {
		record := entity.CoreConfig{
			ServerID:   entry.ServerId,
			CoreType:   entry.CoreType,
			Section:    entry.Section,
			ConfigJSON: entry.ConfigJson,
		}
		// Do NOT set record.ID — let ON CONFLICT(server_id, core_type, section) handle the upsert.
		if entry.UpdatedAt > 0 {
			record.UpdatedAt = time.Unix(entry.UpdatedAt, 0)
		}
		result := database.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "server_id"}, {Name: "core_type"}, {Name: "section"}},
			DoUpdates: clause.AssignmentColumns([]string{"config_json", "updated_at"}),
		}).Select("server_id", "core_type", "section", "config_json", "updated_at").Create(&record)
		if result.Error == nil {
			resp.CoreConfigsApplied++
		} else {
			log.Printf("[Agent] upsert core_config id=%d: %v", entry.Id, result.Error)
		}
	}

	// 2. Upsert inbound_configs
	for _, entry := range req.InboundConfigs {
		record := entity.InboundConfig{
			Remark:     entry.Remark,
			CoreType:   entry.CoreType,
			Protocol:   entry.Protocol,
			Tag:        entry.Tag,
			Listen:     entry.Listen,
			Port:       int(entry.Port),
			ConfigJSON: entry.ConfigJson,
			Enabled:    entry.Enabled,
			ServerID:   entry.ServerId,
			Region:     entry.Region,
		}
		// Do NOT set record.ID — let ON CONFLICT(tag, server_id) handle the upsert.
		// Setting record.ID causes GORM to take a different code path that may
		// skip zero-value fields like Enabled=false during UPDATE.
		if entry.CreatedAt > 0 {
			record.CreatedAt = time.Unix(entry.CreatedAt, 0)
		}
		if entry.UpdatedAt > 0 {
			record.UpdatedAt = time.Unix(entry.UpdatedAt, 0)
		}
		// Use Select to ensure zero-value fields (like Enabled=false) are included
		// in the INSERT statement, so ON CONFLICT DO UPDATE SET can update them correctly.
		// ON CONFLICT uses (tag, server_id) which matches the unique index,
		// not (id) which may not match when the record was synced from another DB.
		result := database.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "tag"}, {Name: "server_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"remark", "core_type", "protocol", "listen", "port", "config_json", "enabled", "region", "updated_at"}),
		}).Select("remark", "core_type", "protocol", "tag", "listen", "port", "config_json", "enabled", "server_id", "region", "created_at", "updated_at").Create(&record)
		if result.Error == nil {
			resp.InboundConfigsApplied++
		} else {
			log.Printf("[Agent] upsert inbound_config id=%d: %v", entry.Id, result.Error)
		}
	}

	// 3. Upsert outbound_configs
	for _, entry := range req.OutboundConfigs {
		record := entity.OutboundConfig{
			ServerID:   entry.ServerId,
			Remark:     entry.Remark,
			CoreType:   entry.CoreType,
			Protocol:   entry.Protocol,
			Tag:        entry.Tag,
			ConfigJSON: entry.ConfigJson,
			Enabled:    entry.Enabled,
			IsDefault:  entry.IsDefault,
			SortOrder:  int(entry.SortOrder),
		}
		// Do NOT set record.ID — let ON CONFLICT(server_id, tag) handle the upsert.
		if entry.CreatedAt > 0 {
			record.CreatedAt = time.Unix(entry.CreatedAt, 0)
		}
		if entry.UpdatedAt > 0 {
			record.UpdatedAt = time.Unix(entry.UpdatedAt, 0)
		}
		result := database.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "server_id"}, {Name: "tag"}},
			DoUpdates: clause.AssignmentColumns([]string{"remark", "core_type", "protocol", "config_json", "enabled", "is_default", "sort_order", "updated_at"}),
		}).Select("server_id", "remark", "core_type", "protocol", "tag", "config_json", "enabled", "is_default", "sort_order", "created_at", "updated_at").Create(&record)
		if result.Error == nil {
			resp.OutboundConfigsApplied++
		} else {
			log.Printf("[Agent] upsert outbound_config id=%d: %v", entry.Id, result.Error)
		}
	}

	// 4. Upsert warp_configs
	for _, entry := range req.WarpConfigs {
		record := entity.WarpConfig{
			ServerID:    entry.ServerId,
			CoreType:    entry.CoreType,
			Remark:      entry.Remark,
			DeviceID:    entry.DeviceId,
			Token:       entry.Token,
			PrivateKey:  entry.PrivateKey,
			PublicKey:   entry.PublicKey,
			Reserved:    entry.Reserved,
			AddressIPv4: entry.AddressIpv4,
			AddressIPv6: entry.AddressIpv6,
			Endpoint:    entry.Endpoint,
			ClientID:    entry.ClientId,
			AccountID:   entry.AccountId,
			AccountType: entry.AccountType,
			LicenseKey:  entry.LicenseKey,
			Status:      entry.Status,
			ConfigJSON:  entry.ConfigJson,
		}
		// Do NOT set record.ID — use ON CONFLICT for upsert
		if entry.RegistrationAt > 0 {
			record.RegistrationAt = time.Unix(entry.RegistrationAt, 0)
		}
		if entry.CreatedAt > 0 {
			record.CreatedAt = time.Unix(entry.CreatedAt, 0)
		}
		if entry.UpdatedAt > 0 {
			record.UpdatedAt = time.Unix(entry.UpdatedAt, 0)
		}
		result := database.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{"server_id", "core_type", "remark", "device_id", "token", "private_key", "public_key", "reserved", "address_ipv4", "address_ipv6", "endpoint", "client_id", "account_id", "account_type", "license_key", "registration_at", "status", "config_json", "updated_at"}),
		}).Select("server_id", "core_type", "remark", "device_id", "token", "private_key", "public_key", "reserved", "address_ipv4", "address_ipv6", "endpoint", "client_id", "account_id", "account_type", "license_key", "registration_at", "status", "config_json", "created_at", "updated_at").Create(&record)
		if result.Error == nil {
			resp.WarpConfigsApplied++
		} else {
			log.Printf("[Agent] upsert warp_config id=%d: %v", entry.Id, result.Error)
		}
	}

	// 5. Upsert share_domain_configs
	for _, entry := range req.ShareDomainConfigs {
		record := entity.ShareDomainConfig{
			Type:      entry.Type,
			Value:     entry.Value,
			Port:      int(entry.Port),
			Ports:     entry.Ports,
			Enabled:   entry.Enabled,
			SortOrder: int(entry.SortOrder),
			ServerID:  entry.ServerId,
			Region:    entry.Region,
		}
		// Do NOT set record.ID — use ON CONFLICT for upsert
		if entry.CreatedAt > 0 {
			record.CreatedAt = time.Unix(entry.CreatedAt, 0)
		}
		if entry.UpdatedAt > 0 {
			record.UpdatedAt = time.Unix(entry.UpdatedAt, 0)
		}
		result := database.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{"type", "value", "port", "ports", "enabled", "sort_order", "server_id", "region", "updated_at"}),
		}).Select("type", "value", "port", "ports", "enabled", "sort_order", "server_id", "region", "created_at", "updated_at").Create(&record)
		if result.Error == nil {
			resp.ShareDomainConfigsApplied++
		} else {
			log.Printf("[Agent] upsert share_domain_config id=%d: %v", entry.Id, result.Error)
		}
	}

	// 5.5 Land TLS certificates to local disk
	// Certificates are sent as PEM content in req.Certificates.
	// We write them to dataDir + relativePath so that xray's certificateFile
	// references (relative to data/runtime/) resolve correctly.
	for _, certEntry := range req.Certificates {
		if certEntry.CertContent == "" || certEntry.KeyContent == "" {
			continue
		}
		// Write certificate file
		certPath := certEntry.CertPath
		if !filepath.IsAbs(certPath) {
			certPath = filepath.Join(a.dataDir, certPath)
		}
		os.MkdirAll(filepath.Dir(certPath), 0750)
		if err := os.WriteFile(certPath, []byte(certEntry.CertContent), 0640); err != nil {
			log.Printf("[Agent] SyncData: write cert %s: %v", certEntry.Domain, err)
			continue
		}
		// Write key file
		keyPath := certEntry.KeyPath
		if !filepath.IsAbs(keyPath) {
			keyPath = filepath.Join(a.dataDir, keyPath)
		}
		os.MkdirAll(filepath.Dir(keyPath), 0750)
		if err := os.WriteFile(keyPath, []byte(certEntry.KeyContent), 0600); err != nil {
			log.Printf("[Agent] SyncData: write key %s: %v", certEntry.Domain, err)
			continue
		}
		// Write fullchain if provided
		if certEntry.FullchainContent != "" && certEntry.FullchainPath != "" {
			fullchainPath := certEntry.FullchainPath
			if !filepath.IsAbs(fullchainPath) {
				fullchainPath = filepath.Join(a.dataDir, fullchainPath)
			}
			os.MkdirAll(filepath.Dir(fullchainPath), 0750)
			os.WriteFile(fullchainPath, []byte(certEntry.FullchainContent), 0640)
		}
		log.Printf("[Agent] SyncData: certificate landed: %s → %s", certEntry.Domain, certPath)
	}

	// 5.6 Save Nginx settings and log output config to local DB
	// These are needed by the Agent to generate correct proxy configs on startup.
	if req.NginxSettings != "" && a.settingRepo != nil {
		key := perNodeSettingsKey("proxy.nginx_settings", req.ServerId)
		if err := a.settingRepo.Set(ctx, key, req.NginxSettings); err != nil {
			log.Printf("[Agent] SyncData: save nginx settings failed: %v", err)
		} else {
			log.Printf("[Agent] SyncData: saved nginx settings to %s (%d bytes)", key, len(req.NginxSettings))
		}
	}
	if req.LogOutputConfig != "" && a.settingRepo != nil {
		key := perNodeSettingsKey("proxy.log_output_config", req.ServerId)
		if err := a.settingRepo.Set(ctx, key, req.LogOutputConfig); err != nil {
			log.Printf("[Agent] SyncData: save log output config failed: %v", err)
		} else {
			log.Printf("[Agent] SyncData: saved log output config to %s (%d bytes)", key, len(req.LogOutputConfig))
		}
	}
	if req.CoreLogOutputConfig != "" && a.settingRepo != nil {
		key := perNodeSettingsKey("core.log_output_config", req.ServerId)
		if err := a.settingRepo.Set(ctx, key, req.CoreLogOutputConfig); err != nil {
			log.Printf("[Agent] SyncData: save core log output config failed: %v", err)
		} else {
			log.Printf("[Agent] SyncData: saved core log output config to %s (%d bytes)", key, len(req.CoreLogOutputConfig))
		}
	}
	// Save manually_stopped config for proxy auto-start
	if a.settingRepo != nil {
		key := perNodeSettingsKey("proxy.nginx.manually_stopped", req.ServerId)
		value := "false"
		if req.NginxManuallyStopped {
			value = "true"
		}
		if err := a.settingRepo.Set(ctx, key, value); err != nil {
			log.Printf("[Agent] SyncData: save nginx manually_stopped failed: %v", err)
		} else {
			log.Printf("[Agent] SyncData: saved nginx manually_stopped=%s to %s", value, key)
		}
		key = perNodeSettingsKey("proxy.caddy.manually_stopped", req.ServerId)
		value = "false"
		if req.CaddyManuallyStopped {
			value = "true"
		}
		if err := a.settingRepo.Set(ctx, key, value); err != nil {
			log.Printf("[Agent] SyncData: save caddy manually_stopped failed: %v", err)
		} else {
			log.Printf("[Agent] SyncData: saved caddy manually_stopped=%s to %s", value, key)
		}

		// Save proxy guardian settings (auto_start, auto_restart, max_restarts)
		// These are stored using the keys that proxy.Manager.GetProxyGuardianSettings()
		// reads: proxy.{type}.auto_start / .auto_restart / .max_restarts
		// (without per-node suffix, since the Agent manages only one node)
		saveProxyGuardianSetting(ctx, a.settingRepo, "nginx", "auto_start", fmt.Sprintf("%v", req.NginxAutoStart))
		saveProxyGuardianSetting(ctx, a.settingRepo, "nginx", "auto_restart", fmt.Sprintf("%v", req.NginxAutoRestart))
		saveProxyGuardianSetting(ctx, a.settingRepo, "nginx", "max_restarts", fmt.Sprintf("%d", req.NginxMaxRestarts))
		saveProxyGuardianSetting(ctx, a.settingRepo, "caddy", "auto_start", fmt.Sprintf("%v", req.CaddyAutoStart))
		saveProxyGuardianSetting(ctx, a.settingRepo, "caddy", "auto_restart", fmt.Sprintf("%v", req.CaddyAutoRestart))
		saveProxyGuardianSetting(ctx, a.settingRepo, "caddy", "max_restarts", fmt.Sprintf("%d", req.CaddyMaxRestarts))
	}

	resp.Success = true

	// 6. If apply_and_start, generate config from local DB and start core
	// After upserting all rows above, the local DB is the single source of truth.
	// generateConfigFromLocalDB uses PrepareFullConfig (same as panel) to produce
	// the final clean config — disabled inbounds/outbounds are filtered out,
	// internal fields are stripped, etc.
	if req.ApplyAndStart {
		coreType := "xray"
		if req.ConfigCoreType != "" {
			coreType = req.ConfigCoreType
		}

		configData, err := generateConfigFromLocalDB(ctx, req.ServerId, coreType)
		if err != nil {
			resp.Error = fmt.Sprintf("generate config: %v", err)
			log.Printf("[Agent] SyncData: config generation failed: %v", err)
			return resp, nil
		}
		if len(configData) > 0 {
			if err := a.procMgr.WriteRuntimeConfig(coreType, configData); err != nil {
				resp.Error = fmt.Sprintf("write config: %v", err)
				log.Printf("[Agent] SyncData: write config failed: %v", err)
				return resp, nil
			}
			resp.ConfigGenerated = true
			log.Printf("[Agent] SyncData: generated config from local DB (%d bytes)", len(configData))
		}

		// Start the core process (configGenerator will also call generateConfigFromLocalDB,
		// which produces the same result — no conflict)
		// Resolve the correct serviceID from DB — must match what ControlCore uses,
		// otherwise the healthcheck will monitor a different serviceID and report "crashed".
		serviceID := int64(0)
		if a.svcRepo != nil {
			services, _, err := a.svcRepo.List(ctx, 1, coreType, 1, 0)
			if err == nil && len(services) > 0 {
				serviceID = services[0].ID
			}
		}
		if err := a.procMgr.Start(ctx, serviceID, coreType); err != nil {
			resp.Error = fmt.Sprintf("start core: %v", err)
			log.Printf("[Agent] SyncData: core start failed: %v", err)
			return resp, nil
		}
		resp.CoreStarted = true
		log.Printf("[Agent] SyncData: core started successfully")
	}

	// 7. Refresh LogWriters based on the updated configs.
	// core_configs may have changed (e.g. protocol center switched log mode),
	// so we need to re-read the derived mode and update writer state.
	// This makes LogWriter status reflect changes immediately without requiring
	// a manual save in the log management UI.
	if a.logMgr != nil {
		a.logMgr.StartAutoWritersForServer(req.ServerId)
		log.Printf("[Agent] SyncData: log writers refreshed for server %d", req.ServerId)
	}

	return resp, nil
}

// generateConfigFromLocalDB generates a complete runtime config from the local
// SQLite database, using the same PrepareFullConfig logic as the panel.
// This is the fallback path when the panel is unreachable (config_content is empty).
func generateConfigFromLocalDB(ctx context.Context, serverID int64, coreType string) ([]byte, error) {
	// Read all core config sections and reconstruct full config (same as coreconfig.Service.GetFull)
	var coreConfigs []entity.CoreConfig
	if err := db.DB.WithContext(ctx).Where("server_id = ? AND core_type = ?", serverID, coreType).Find(&coreConfigs).Error; err != nil {
		return nil, fmt.Errorf("read core_configs: %w", err)
	}
	if len(coreConfigs) == 0 {
		return nil, nil // no data yet
	}

	fullConfig := make(map[string]interface{})
	for _, cc := range coreConfigs {
		var val interface{}
		if err := json.Unmarshal([]byte(cc.ConfigJSON), &val); err != nil {
			log.Printf("[Agent] skip core_config section %s: %v", cc.Section, err)
			continue
		}
		fullConfig[cc.Section] = val
	}

	// Use PrepareFullConfig — same pipeline the panel uses.
	// This handles: cleaning empty fields, stripping _disabled sections,
	// filtering disabled routing rules, removing outbounds from core_configs,
	// ensuring stats section, applying API inbound/routing, merging enabled
	// inbounds/outbounds from inbound_configs/outbound_configs tables.
	data, err := coreconfigSvc.PrepareFullConfig(coreconfigSvc.PrepareFullConfigOptions{
		Ctx:              ctx,
		ServerID:         serverID,
		CoreType:         coreType,
		FullConfig:       fullConfig,
		InboundProvider:  &localInboundProvider{},
		OutboundProvider: &localOutboundProvider{},
	})
	if err != nil {
		return nil, fmt.Errorf("PrepareFullConfig: %w", err)
	}
	return data, nil
}

// localInboundProvider implements InboundConfigProvider using the Agent's local DB.
// It only returns enabled inbounds — matching the panel's behavior.
type localInboundProvider struct{}

func (p *localInboundProvider) GetEnabledInbounds(ctx context.Context, serverID int64, coreType string) ([]json.RawMessage, error) {
	var configs []entity.InboundConfig
	if err := db.DB.WithContext(ctx).Where("server_id = ? AND core_type = ? AND enabled = ?", serverID, coreType, true).Find(&configs).Error; err != nil {
		return nil, err
	}
	result := make([]json.RawMessage, 0, len(configs))
	for _, c := range configs {
		// Parse, clean internal fields, and re-marshal — same as panel's inboundconfig.Service
		var inbound map[string]interface{}
		if err := json.Unmarshal([]byte(c.ConfigJSON), &inbound); err != nil {
			log.Printf("[Agent] skip inbound_config tag=%s: %v", c.Tag, err)
			continue
		}
		coreconfigSvc.CleanInboundFields(inbound, c.Tag, c.Listen, c.Protocol, c.Port)
		data, err := json.Marshal(inbound)
		if err != nil {
			log.Printf("[Agent] marshal inbound_config tag=%s: %v", c.Tag, err)
			continue
		}
		result = append(result, data)
	}
	return result, nil
}

// localOutboundProvider implements OutboundConfigProvider using the Agent's local DB.
// It only returns enabled outbounds — matching the panel's behavior.
type localOutboundProvider struct{}

func (p *localOutboundProvider) GetEnabledOutbounds(ctx context.Context, serverID int64, coreType string) ([]map[string]interface{}, error) {
	var configs []entity.OutboundConfig
	if err := db.DB.WithContext(ctx).Where("server_id = ? AND core_type = ? AND enabled = ?", serverID, coreType, true).Order("sort_order ASC, id ASC").Find(&configs).Error; err != nil {
		return nil, err
	}
	result := make([]map[string]interface{}, 0, len(configs))
	for _, c := range configs {
		m, err := parseOutboundConfigJSON(c.ConfigJSON, c.Tag, c.Protocol)
		if err != nil {
			log.Printf("[Agent] skip outbound_config tag=%s: %v", c.Tag, err)
			continue
		}
		result = append(result, m)
	}
	return result, nil
}

// parseOutboundConfigJSON parses config_json into an outbound map.
// Handles: normal JSON objects, double-escaped JSON strings, and simple
// protocol names like "direct"/"blackhole".
func parseOutboundConfigJSON(configJSON, dbTag, dbProtocol string) (map[string]interface{}, error) {
	raw := strings.TrimSpace(configJSON)
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty config_json")
	}

	// Try 1: Direct parse as JSON object
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err == nil {
		// Ensure tag and protocol from DB are authoritative
		m["tag"] = dbTag
		m["protocol"] = dbProtocol
		return m, nil
	}

	// Try 2: Double-escaped JSON string — unwrap one layer and re-parse
	// e.g. "\"{\\\"protocol\\\":\\\"freedom\\\",\\\"tag\\\":\\\"direct\\\"}\""
	// → unquote → {"protocol":"freedom","tag":"direct"}
	var inner string
	if err := json.Unmarshal([]byte(raw), &inner); err == nil {
		var m2 map[string]interface{}
		if err := json.Unmarshal([]byte(inner), &m2); err == nil {
			m2["tag"] = dbTag
			m2["protocol"] = dbProtocol
			return m2, nil
		}
		// inner is a plain string like "direct" — construct from DB fields
		return map[string]interface{}{"tag": dbTag, "protocol": dbProtocol}, nil
	}

	// Try 3: Simple string like "direct" — construct from DB fields
	return map[string]interface{}{"tag": dbTag, "protocol": dbProtocol}, nil
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

// saveProxyGuardianSetting saves a single proxy guardian setting to the Agent's
// local DB using the key format that proxy.Manager.GetProxyGuardianSettings() reads.
// The key is: proxy.{proxyType}.{settingName} (e.g., proxy.nginx.auto_restart).
func saveProxyGuardianSetting(ctx context.Context, repo *repository.SystemSettingRepository, proxyType, settingName, value string) {
	key := fmt.Sprintf("proxy.%s.%s", proxyType, settingName)
	if err := repo.Set(ctx, key, value); err != nil {
		log.Printf("[Agent] SyncData: save %s failed: %v", key, err)
	} else {
		log.Printf("[Agent] SyncData: saved %s=%s", key, value)
	}
}
