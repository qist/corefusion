package main

import (
	"corefusion/internal/model/entity"

	"gorm.io/gorm"
)

// migrateAgentTables runs migrations for the tables that process.Manager,
// proxy.Manager, and SyncData need (inbound_services, process_guardians,
// core_crash_logs, proxy_sites, system_settings, plus xray kernel tables
// for local config generation from synced data).
func migrateAgentTables(database *gorm.DB) error {
	// Only migrate the tables needed by the Agent, not the full Panel schema.
	// We use AutoMigrate with a subset of entities.
	return database.AutoMigrate(
		// Process management tables
		&entity.InboundService{},
		&entity.ProcessGuardian{},
		&entity.CoreCrashLog{},
		&entity.ProxySite{},
		&entity.SystemSetting{},
		// Xray kernel config tables (populated by SyncData, used for local config generation)
		&entity.CoreConfig{},
		&entity.InboundConfig{},
		&entity.OutboundConfig{},
		&entity.WarpConfig{},
		&entity.ShareDomainConfig{},
	)
}
