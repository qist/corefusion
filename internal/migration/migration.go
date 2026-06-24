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

		// Share Domain Configs
		&entity.ShareDomainConfig{},

		// CF IP Sources & Entries
		&entity.CFIPSource{},
		&entity.CFIPEntry{},

		// Subscription Formats & Rules
		&entity.SubscriptionFormat{},
		&entity.SubscriptionRule{},

		// Subscription Endpoints
		&entity.SubscriptionEndpoint{},
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

	// Seed default share domain configs if none exist
	seedDefaultShareDomains(db)

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

func seedDefaultShareDomains(db *gorm.DB) {
	var count int64
	db.Model(&entity.ShareDomainConfig{}).Count(&count)
	log.Printf("share domain configs count: %d", count)
	if count > 0 {
		return
	}

	// CF HTTP ports: 80, 8080, 8880, 2052, 2082, 2086, 2095
	// CF HTTPS ports: 443, 2053, 2083, 2087, 2096, 8443
	cfHttpPorts := "[80,8080,8880,2052,2082,2086,2095]"
	cfHttpsPorts := "[443,2053,2083,2087,2096,8443]"

	defaults := []entity.ShareDomainConfig{
		// CF Priority Domains
		{Type: "cf_domain", Value: "mfa.gov.ua", Port: 443, Ports: cfHttpPorts, Enabled: true, SortOrder: 1},
		{Type: "cf_domain", Value: "www.shopify.com", Port: 443, Ports: cfHttpPorts, Enabled: true, SortOrder: 2},
		{Type: "cf_domain", Value: "www.visa.cn", Port: 443, Ports: cfHttpPorts, Enabled: true, SortOrder: 3},
		{Type: "cf_domain", Value: "time.is", Port: 443, Ports: cfHttpPorts, Enabled: true, SortOrder: 4},
		{Type: "cf_domain", Value: "icook.hk", Port: 443, Ports: cfHttpPorts, Enabled: true, SortOrder: 5},
		{Type: "cf_domain", Value: "icook.tw", Port: 443, Ports: cfHttpPorts, Enabled: true, SortOrder: 6},

		// CF Priority IPs (domain form)
		{Type: "cf_ip", Value: "cloudflare.182682.xyz", Port: 443, Ports: cfHttpsPorts, Enabled: true, SortOrder: 1},
		{Type: "cf_ip", Value: "tencentapp.cn", Port: 443, Ports: cfHttpsPorts, Enabled: true, SortOrder: 2},
		{Type: "cf_ip", Value: "cf.877774.xyz", Port: 443, Ports: cfHttpsPorts, Enabled: true, SortOrder: 3},
		{Type: "cf_ip", Value: "cfip.xxxxxxxx.tk", Port: 443, Ports: cfHttpsPorts, Enabled: true, SortOrder: 4},
		{Type: "cf_ip", Value: "bestcf.030101.xyz", Port: 443, Ports: cfHttpsPorts, Enabled: true, SortOrder: 5},
		{Type: "cf_ip", Value: "777.ai7777777.xyz", Port: 443, Ports: cfHttpsPorts, Enabled: true, SortOrder: 6},
		{Type: "cf_ip", Value: "cdn.2020111.xyz", Port: 443, Ports: cfHttpsPorts, Enabled: true, SortOrder: 7},
		{Type: "cf_ip", Value: "cfip.cfcdn.vip", Port: 443, Ports: cfHttpsPorts, Enabled: true, SortOrder: 8},
		{Type: "cf_ip", Value: "cf.0sm.com", Port: 443, Ports: cfHttpsPorts, Enabled: true, SortOrder: 9},
		{Type: "cf_ip", Value: "cfip.1323123.xyz", Port: 443, Ports: cfHttpsPorts, Enabled: true, SortOrder: 10},
		{Type: "cf_ip", Value: "cloudflare-ip.mofashi.ltd", Port: 443, Ports: cfHttpsPorts, Enabled: true, SortOrder: 11},
		{Type: "cf_ip", Value: "cf.877771.xyz", Port: 443, Ports: cfHttpsPorts, Enabled: true, SortOrder: 12},
		{Type: "cf_ip", Value: "cf.900501.xyz", Port: 443, Ports: cfHttpsPorts, Enabled: true, SortOrder: 13},
		{Type: "cf_ip", Value: "cdns.doon.eu.org", Port: 443, Ports: cfHttpsPorts, Enabled: true, SortOrder: 14},
		{Type: "cf_ip", Value: "fn.130519.xyz", Port: 443, Ports: cfHttpsPorts, Enabled: true, SortOrder: 15},
		{Type: "cf_ip", Value: "nrt.xxxxxxxx.nyc.mn", Port: 443, Ports: cfHttpsPorts, Enabled: true, SortOrder: 16},
		{Type: "cf_ip", Value: "nrtcfdns.zone.id", Port: 443, Ports: cfHttpsPorts, Enabled: true, SortOrder: 17},
		{Type: "cf_ip", Value: "saas.sin.fan", Port: 443, Ports: cfHttpsPorts, Enabled: true, SortOrder: 18},
		{Type: "cf_ip", Value: "xn--b6gac.eu.org", Port: 443, Ports: cfHttpsPorts, Enabled: true, SortOrder: 19},
		{Type: "cf_ip", Value: "freeyx.cloudflare88.eu.org", Port: 443, Ports: cfHttpsPorts, Enabled: true, SortOrder: 20},
	}

	for _, d := range defaults {
		if err := db.Create(&d).Error; err != nil {
			log.Printf("WARNING: seed share domain %s failed: %v", d.Value, err)
		} else {
			log.Printf("seeded share domain: %s [%s]", d.Value, d.Type)
		}
	}
}
