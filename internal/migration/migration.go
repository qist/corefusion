package migration

import (
	"log"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// AutoMigrate runs auto-migration for all entity models.
func AutoMigrate(db *gorm.DB) error {
	models := []interface{}{
		// User & Auth
		&entity.User{},

		// Nodes
		&entity.Node{},

		// Core Configs
		&entity.CoreConfig{},
		&entity.InboundConfig{},
		&entity.OutboundConfig{},

		// Services
		&entity.InboundService{},
		&entity.ServiceEndpoint{},

		// Access Control
		&entity.ServiceAccessPolicy{},
		&entity.ServiceIPBan{},
		&entity.ServiceGeoRule{},

		// Subscriptions
		&entity.Subscription{},
		&entity.SubscriptionAccessLog{},
		&entity.SubscriptionRotation{},
		&entity.ServerShareProfile{},

		// Certificates
		&entity.Certificate{},
		&entity.ACMEAccount{},
		&entity.CertificateJob{},

		// Traffic & Quota
		&entity.TrafficQuota{},
		&entity.TrafficUsageLedger{},
		&entity.TrafficUsageSummary{},
		&entity.TrafficSession{},
		&entity.TrafficQuotaResetLog{},

		// Security & Audit
		&entity.SecurityAuditEvent{},
		&entity.SecurityAlert{},
		&entity.AlertChannel{},

		// Runtime
		&entity.RuntimeSnapshot{},
		&entity.RuntimeApplyJob{},
		&entity.RuntimeApplyJobTarget{},
		&entity.RuntimeApplyJobEvent{},
		&entity.ReverseProxyInstance{},

		// Backups
		&entity.BackupRetentionPolicy{},
		&entity.BackupCleanupJob{},

		// Binaries & Updates
		&entity.BinaryRelease{},
		&entity.UpdateJob{},

		// Routing
		&entity.RoutingRegion{},

		// Geo Rules
		&entity.RuleSource{},
		&entity.GeoRuleFile{},

		// WARP
		&entity.WarpConfig{},

		// Process Guardian
		&entity.ProcessGuardian{},
		&entity.CoreCrashLog{},

		// Proxy Sites (legacy)
		&entity.ProxySite{},
		// Proxy Sites (separated)
		&entity.CaddySite{},
		&entity.NginxSite{},

		// Certificate Management
		&entity.DNSChannel{},
		&entity.ACMEAccountConfig{},

		// Log Rotation
		&entity.LogRotationPolicy{},
		&entity.LogRotationJob{},

		// System Settings
		&entity.SystemSetting{},
		&entity.NotificationTemplate{},

		// Backups
		&entity.BackupRecord{},
		&entity.BackupSchedule{},
		&entity.BackupStorage{},
	}

	for _, model := range models {
		if err := db.AutoMigrate(model); err != nil {
			return err
		}
		log.Printf("migrated: %T", model)
	}

	log.Println("all migrations completed")

	// Seed default outbounds if none exist
	seedDefaultOutbounds(db)

	return nil
}

func seedDefaultOutbounds(db *gorm.DB) {
	var count int64
	db.Model(&entity.OutboundConfig{}).Count(&count)
	if count > 0 {
		return
	}
	defaults := []entity.OutboundConfig{
		{CoreType: "xray", Protocol: "freedom", Tag: "direct", ConfigJSON: `{"protocol":"freedom","tag":"direct"}`, Enabled: true, IsDefault: true},
		{CoreType: "xray", Protocol: "blackhole", Tag: "blocked", ConfigJSON: `{"protocol":"blackhole","tag":"blocked"}`, Enabled: true, IsDefault: true},
	}
	for _, ob := range defaults {
		if err := db.Create(&ob).Error; err != nil {
			log.Printf("WARNING: seed outbound %s failed: %v", ob.Tag, err)
		} else {
			log.Printf("seeded default outbound: %s", ob.Tag)
		}
	}
}
