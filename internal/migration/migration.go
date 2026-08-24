package migration

import (
	"fmt"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/security/crypto"
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
		&entity.NodeTrafficUsage{},

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
		&entity.CARoot{},

		// Log Rotation
		&entity.LogRotationPolicy{},
		&entity.LogRotationJob{},

		// Log Cleanup (database table cleanup rules)
		&entity.LogCleanupRule{},

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

		// Plans & Clients
		&entity.Plan{},
		&entity.PlanNode{},
		&entity.Client{},

		// Plan v2: Templates, Variants, Node Groups, Promotions, Orders
		&entity.PlanTemplate{},
		&entity.PlanVariant{},
		&entity.NodeGroup{},
		&entity.NodeGroupMember{},
		&entity.Promotion{},
		&entity.PlanOrder{},

		// Mihomo Config
		&entity.MihomoConfig{},
		&entity.MihomoGeneral{},
		&entity.MihomoDNS{},
		&entity.MihomoHost{},
		&entity.MihomoTunnel{},
		&entity.MihomoSniffer{},
		&entity.MihomoExternalController{},
		&entity.MihomoTLS{},
		&entity.MihomoTun{},
		&entity.MihomoProxyGroup{},
		&entity.MihomoRuleProvider{},
		&entity.MihomoRule{},
		&entity.MihomoSpecialOutbound{},

		// External Nodes
		&entity.ExternalNodeSource{},
		&entity.ExternalNode{},

		// Node Servers (multi-region architecture)
		&entity.NodeServer{},

		// Agent Certificate Revocations (CRL)
		&entity.AgentCertRevocation{},

		// Phase 6: Server Metrics & Speed Test Records
		&entity.ServerMetric{},
		&entity.SpeedTestRecord{},

		// Supplementary: Node Audit Logs, Server Tags, Cert Distribution Queue
		&entity.NodeAuditLog{},
		&entity.ServerTag{},
		&entity.CertDistributionQueueItem{},

		// Client Log Packages (subscription user log access)
		&entity.ClientLogPackage{},

		// Portal Templates
		&entity.PortalTemplate{},

		// Commerce: Charge Sites, Orders, Cards, Balances
		&entity.ChargeSite{},
		&entity.ChargeOrder{},
		&entity.ChargeCard{},
		&entity.ClientBalance{},
		&entity.BalanceTransaction{},
		&entity.CallbackNonce{},
		&entity.ChargeCallbackRetry{},
		&entity.ChargeCallbackEvent{},
		&entity.ChargeSiteSecret{},    // v3.2: 多版本密钥管理
		&entity.ChargeSecurityEvent{}, // v3.2: 充值风控事件

		// Referral / 邀请推广
		&entity.ReferralCode{},
		&entity.ReferralRecord{},
		&entity.ReferralWithdrawal{},

		// Ticket / 工单
		&entity.Ticket{},
		&entity.TicketMessage{},

		// Gift / 福利
		&entity.ClientGift{},

		// Article / 教程文章
		&entity.Article{},

		// Shared Account / 共享账号
		&entity.SharedAccount{},
		&entity.SharedAccountRepairLog{},

		// OAuth Binding / 第三方绑定
		&entity.ClientOAuthBinding{},

		// Self-Unban / 自解封
		&entity.ClientSelfUnbanLog{},

		// IP Whitelist / IP白名单来源
		&entity.IPWhitelistEntry{},

		// Rate Limit Config / 限速配置
		&entity.RateLimitConfig{},

		// WAF Rule Store (nginx/caddy shared rules)
		&entity.WafRule{},
		&entity.WafConfig{},
		&entity.WafDomainConfig{},
	}

	// PostgreSQL has no "longtext" type. CoreFusion entities use type:longtext
	// for large-text columns (MySQL maps it to LONGTEXT, 4GB). To keep a single
	// entity definition across databases, we register a PostgreSQL DOMAIN that
	// aliases longtext -> text (which is unbounded in PG). This must run before
	// AutoMigrate so the type exists when tables are created.
	if db.Dialector.Name() == "postgres" {
		// CREATE DOMAIN ... IF NOT EXISTS is not supported on all PG versions;
		// use a DO block to create the alias idempotently.
		ddl := `DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'longtext') THEN
				CREATE DOMAIN longtext AS text;
			END IF;
		END $$;`
		if err := db.Exec(ddl).Error; err != nil {
			return fmt.Errorf("create longtext domain: %w", err)
		}
	}

	for _, model := range models {
		if err := db.AutoMigrate(model); err != nil {
			return err
		}
		log.Printf("migrated: %T", model)
	}

	log.Println("all migrations completed")

	// Run all post-AutoMigrate data migrations / seeds.
	// Exported so the cross-DB migration tool (db.MigrateTo) can run them on
	// the target database BEFORE copying data, ensuring the target schema
	// matches the source (e.g. charge_cards plaintext→encrypted column drop).
	if err := RunDataMigrations(db); err != nil {
		return err
	}

	return nil
}

// RunDataMigrations runs all post-AutoMigrate data migrations and seeds.
// It is safe to call multiple times (all steps are idempotent).
func RunDataMigrations(db *gorm.DB) error {
	// Migrate existing data to server_id=1 (local node)
	migrateServerID(db)

	// Migrate geo rules from per-node (server_id=1) to global (server_id=0)
	migrateGeoRulesToGlobal(db)

	// Fix unique indexes to be per-server (GORM AutoMigrate cannot alter existing indexes)
	migrateUniqueIndexes(db)

	// v3.3: Migrate charge_cards from plaintext card_code to encrypted storage
	migrateChargeCardEncryption(db)

	// v3.3: Add agent_id column to charge_cards for agent/sales tracking
	migrateChargeCardAgentColumn(db)

	// Seed default outbounds if none exist
	seedDefaultOutbounds(db)

	// Seed default share domain configs if none exist
	seedDefaultShareDomains(db)

	// Migrate legacy plans to plan_templates (one-time data migration)
	migratePlansToTemplates(db)

	// Seed default log cleanup rules
	seedDefaultLogCleanupRules(db)

	return nil
}

// migrateUniqueIndexes drops legacy single-column unique indexes and
// recreates them as composite (server_id, ...) indexes so that different
// node servers can use the same tag / (core_type, section) without conflict.
// GORM AutoMigrate cannot drop/recreate existing indexes, so we do it manually.
func migrateUniqueIndexes(db *gorm.DB) {
	// core_configs: idx_core_section was (core_type, section), needs (server_id, core_type, section)
	recreateIndex(db, "core_configs", "idx_core_section",
		"CREATE UNIQUE INDEX idx_core_section ON core_configs(server_id, core_type, section)")

	// inbound_configs: idx_inbound_configs_tag was (tag), needs (server_id, tag) as idx_inbound_tag_server
	dropIndex(db, "inbound_configs", "idx_inbound_configs_tag")
	createIndex(db, "CREATE UNIQUE INDEX IF NOT EXISTS idx_inbound_tag_server ON inbound_configs(server_id, tag)")

	// outbound_configs: idx_outbound_configs_tag and idx_outbound_tag were (tag), needs (server_id, tag)
	dropIndex(db, "outbound_configs", "idx_outbound_configs_tag")
	dropIndex(db, "outbound_configs", "idx_outbound_tag")
	createIndex(db, "CREATE UNIQUE INDEX IF NOT EXISTS idx_outbound_tag_server ON outbound_configs(server_id, tag)")

	// waf_rules: ensure unique (name, rule_scope, server_id, domain)
	// GORM AutoMigrate creates the table but may not enforce the composite unique index.
	// Drop old index (without domain column) and create new one with domain.
	dropIndex(db, "waf_rules", "idx_waf_rules_scope")
	createIndex(db, "CREATE UNIQUE INDEX IF NOT EXISTS idx_waf_rules_scope ON waf_rules(name, rule_scope, server_id, domain)")

	// waf_configs: ensure unique (server_id, proxy_type) for OnConflict upsert
	createIndex(db, "CREATE UNIQUE INDEX IF NOT EXISTS idx_waf_configs_server_proxy ON waf_configs(server_id, proxy_type)")

	// waf_domain_configs: ensure unique (server_id, domain) for OnConflict upsert
	createIndex(db, "CREATE UNIQUE INDEX IF NOT EXISTS idx_waf_domain_configs_server_domain ON waf_domain_configs(server_id, domain)")
}

// dropIndex drops an index if it exists, logging the result.
// 兼容 MySQL 与 PostgreSQL: MySQL 不支持 DROP INDEX ... IF EXISTS,
// 且语法为 DROP INDEX idx ON table;PostgreSQL 支持 DROP INDEX IF EXISTS idx。
func dropIndex(db *gorm.DB, table, indexName string) {
	var sql string
	if db.Dialector.Name() == "mysql" {
		sql = "DROP INDEX " + indexName + " ON " + table
	} else {
		sql = "DROP INDEX IF EXISTS " + indexName
	}
	if err := db.Exec(sql).Error; err != nil {
		// MySQL: 索引不存在(错误码 1091)属幂等场景,视为成功
		if isIgnorableIndexError(err) {
			return
		}
		log.Printf("WARNING: drop index %s on %s: %v", indexName, table, err)
	}
}

// createIndex creates a new index, logging the result.
// PostgreSQL 支持 CREATE INDEX IF NOT EXISTS;MySQL 不支持 IF NOT EXISTS。
func createIndex(db *gorm.DB, sql string) {
	if db.Dialector.Name() == "mysql" {
		sql = strings.Replace(sql, " IF NOT EXISTS", "", 1)
	}
	if err := db.Exec(sql).Error; err != nil {
		// MySQL: 索引已存在(错误码 1061)属幂等场景,视为成功
		if isIgnorableIndexError(err) {
			return
		}
		log.Printf("WARNING: create index: %s: %v", sql, err)
	}
}

// isIgnorableIndexError reports whether a DDL index error is benign and can be
// ignored during migration. MySQL returns 1091 (index to drop does not exist)
// and 1061 (index already exists) when migrations are re-run; both are expected
// in idempotent migration flows.
func isIgnorableIndexError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	if strings.Contains(msg, "1091") || strings.Contains(msg, "1061") {
		return true
	}
	// Fallback: match by English message fragments.
	if strings.Contains(msg, "Can't DROP") || strings.Contains(msg, "Duplicate key name") {
		return true
	}
	return false
}

// recreateIndex drops the old index and creates a new one with the given SQL.
func recreateIndex(db *gorm.DB, table, indexName, createSQL string) {
	dropIndex(db, table, indexName)
	createIndex(db, createSQL)
}

// migrateChargeCardEncryption v3.3: 迁移 charge_cards 从明文 card_code 到加密存储。
// 旧表有 card_code 明文列，新 schema 用 card_code_hash (SHA-256) + card_code_encrypted (AES-256-GCM)。
// 迁移步骤：
//   1. AutoMigrate 已添加新列 (card_code_hash, card_code_encrypted, created_by_admin_id, created_by_admin_name)
//   2. 对已有行：card_code → hash + encrypt，然后删除旧 card_code 列
func migrateChargeCardEncryption(db *gorm.DB) {
	// 检查是否有旧的 card_code 明文列(兼容 SQLite: 用 Migrator().HasColumn,
	// 而非 information_schema.columns,后者在 SQLite 上不存在)。
	hasOldColumn := db.Migrator().HasColumn("charge_cards", "card_code")
	if !hasOldColumn {
		return // 已迁移过
	}

	log.Println("[migration] charge_cards: migrating from plaintext card_code to encrypted storage")

	// 读取所有有明文 card_code 但没有 card_code_hash 的行
	type oldCard struct {
		ID       int64  `gorm:"column:id"`
		CardCode string `gorm:"column:card_code"`
	}
	var cards []oldCard
	db.Table("charge_cards").Where("card_code IS NOT NULL AND card_code != '' AND (card_code_hash IS NULL OR card_code_hash = '')").Find(&cards)

	for _, card := range cards {
		cardHash := crypto.HashSecret(card.CardCode)
		encrypted, err := crypto.EncryptSecret(card.CardCode)
		if err != nil {
			log.Printf("[migration] WARNING: failed to encrypt card_code for card %d: %v", card.ID, err)
			continue
		}
		db.Exec("UPDATE charge_cards SET card_code_hash = ?, card_code_encrypted = ? WHERE id = ?", cardHash, encrypted, card.ID)
	}

	// 删除旧的 card_code 明文列
	db.Exec("ALTER TABLE charge_cards DROP COLUMN card_code")
	// 删除旧的 card_code 唯一索引（如果还存在）
	db.Exec("DROP INDEX IF EXISTS idx_charge_cards_card_code")

	log.Printf("[migration] charge_cards: migrated %d cards to encrypted storage", len(cards))

	// Create partial unique index on card_code_hash (exclude empty values to avoid conflicts)
	// This replaces the GORM-managed uniqueIndex tag which fails when multiple rows have empty hash
	recreateIndex(db, "charge_cards", "idx_charge_cards_card_code_hash",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_charge_cards_card_code_hash ON charge_cards(card_code_hash) WHERE card_code_hash != ''")
}

// migrateChargeCardAgentColumn v3.3: 添加 agent_id 和 agent_name 列到 charge_cards 表
// 用于追踪充值卡的销售代理员/推广员
func migrateChargeCardAgentColumn(db *gorm.DB) {
	// 检查是否已有 agent_id 列(兼容 SQLite: 用 Migrator().HasColumn)
	hasAgentIDColumn := db.Migrator().HasColumn("charge_cards", "agent_id")
	if hasAgentIDColumn {
		return // 已存在，无需迁移
	}

	log.Println("[migration] charge_cards: adding agent_id and agent_name columns")

	// 添加 agent_id 列
	if err := db.Exec("ALTER TABLE charge_cards ADD COLUMN agent_id INTEGER").Error; err != nil {
		log.Printf("[migration] WARNING: failed to add agent_id column: %v", err)
	}
	// 添加 agent_name 列
	if err := db.Exec("ALTER TABLE charge_cards ADD COLUMN agent_name VARCHAR(64)").Error; err != nil {
		log.Printf("[migration] WARNING: failed to add agent_name column: %v", err)
	}
	// 创建 agent_id 索引
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_charge_cards_agent_id ON charge_cards(agent_id)").Error; err != nil {
		log.Printf("[migration] WARNING: failed to create agent_id index: %v", err)
	}

	log.Println("[migration] charge_cards: agent columns added successfully")
}

// migrateServerID sets server_id=1 for all existing rows in tables that
// newly gained the server_id column. This ensures existing data is
// associated with the local node after the schema migration.
func migrateServerID(db *gorm.DB) {
	tables := []string{
		"core_configs",
		"outbound_configs",
		"inbound_configs",
		"proxy_sites",
		"caddy_sites",
		"nginx_sites",
		"warp_configs",
	}
	for _, table := range tables {
		result := db.Table(table).Where("server_id = 0 OR server_id IS NULL").Update("server_id", 1)
		if result.Error != nil {
			log.Printf("WARNING: migrate server_id for %s: %v", table, result.Error)
		} else if result.RowsAffected > 0 {
			log.Printf("migrated %d rows in %s to server_id=1", result.RowsAffected, table)
		}
	}
}

// migrateGeoRulesToGlobal migrates geo rule sources and files from per-node
// (server_id=1) to global (server_id=0). Geo rules are now managed globally
// and synced to all nodes, so all existing data is moved to server_id=0.
func migrateGeoRulesToGlobal(db *gorm.DB) {
	tables := []string{"rule_sources", "geo_rule_files"}
	for _, table := range tables {
		result := db.Table(table).Where("server_id = 1").Update("server_id", 0)
		if result.Error != nil {
			log.Printf("WARNING: migrate geo rules %s to global: %v", table, result.Error)
		} else if result.RowsAffected > 0 {
			log.Printf("migrated %d rows in %s to global (server_id=0)", result.RowsAffected, table)
		}
	}
}

func seedDefaultOutbounds(db *gorm.DB) {
	var count int64
	db.Model(&entity.OutboundConfig{}).Count(&count)
	if count > 0 {
		return
	}
	defaults := []entity.OutboundConfig{
		{ServerID: 1, CoreType: "xray", Protocol: "freedom", Tag: "direct", ConfigJSON: `{"protocol":"freedom","tag":"direct"}`, Enabled: true, IsDefault: true},
		{ServerID: 1, CoreType: "xray", Protocol: "blackhole", Tag: "blocked", ConfigJSON: `{"protocol":"blackhole","tag":"blocked"}`, Enabled: true, IsDefault: true},
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

// migratePlansToTemplates migrates legacy plans table data to plan_templates + node_groups + node_group_members.
// This is a one-time data migration that runs after schema migration.
// It is idempotent: if plan_templates already exist, it skips migration.
func migratePlansToTemplates(db *gorm.DB) {
	// Check if plans table exists and has data
	var planCount int64
	if !db.Migrator().HasTable("plans") {
		return
	}
	db.Table("plans").Count(&planCount)
	if planCount == 0 {
		return
	}

	// Skip if plan_templates already exist (migration already done)
	var templateCount int64
	db.Table("plan_templates").Count(&templateCount)
	if templateCount > 0 {
		return
	}

	// Check if plan_nodes table exists
	hasPlanNodesTable := db.Migrator().HasTable("plan_nodes")

	type legacyPlan struct {
		ID                       int64              `gorm:"column:id"`
		Name                     string             `gorm:"column:name"`
		Description              string             `gorm:"column:description"`
		QuotaBytes               int64              `gorm:"column:quota_bytes"`
		Duration                 int                `gorm:"column:duration"`
		Enabled                  bool               `gorm:"column:enabled"`
		SortOrder                int                `gorm:"column:sort_order"`
		BillingUsedTemplate      string             `gorm:"column:billing_used_template"`
		BillingRemainingTemplate string             `gorm:"column:billing_remaining_template"`
		BillingResetTemplate     string             `gorm:"column:billing_reset_template"`
		BillingExpiryTemplate    string             `gorm:"column:billing_expiry_template"`
		BillingInboundConfigIDs  entity.Int64List   `gorm:"column:billing_inbound_config_ids"`
		BillingExternalNodeIDs   entity.Int64List   `gorm:"column:billing_external_node_ids"`
	}

	var plans []legacyPlan
	if err := db.Table("plans").Find(&plans).Error; err != nil {
		log.Printf("WARNING: failed to read legacy plans: %v", err)
		return
	}

	now := time.Now()
	migratedCount := 0

	for _, plan := range plans {
		// Create a node group for this plan
		nodeGroup := &entity.NodeGroup{
			Name:        plan.Name,
			Description: "Migrated from legacy plan",
			Tier:        entity.NodeTierStandard,
			Enabled:     true,
			SortOrder:   plan.SortOrder,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := db.Create(nodeGroup).Error; err != nil {
			log.Printf("WARNING: create node group for plan %d: %v", plan.ID, err)
			continue
		}

		// Migrate plan_nodes → node_group_members
		memberIDMap := make(map[int64]int64) // plan_node.id → node_group_member.id

		if hasPlanNodesTable {
			type legacyPlanNode struct {
				ID              int64 `gorm:"column:id"`
				PlanID          int64 `gorm:"column:plan_id"`
				InboundConfigID int64 `gorm:"column:inbound_config_id"`
				ExternalNodeID  int64 `gorm:"column:external_node_id"`
				Enabled         bool  `gorm:"column:enabled"`
			}

			var planNodes []legacyPlanNode
			db.Table("plan_nodes").Where("plan_id = ?", plan.ID).Find(&planNodes)

			for _, pn := range planNodes {
				member := &entity.NodeGroupMember{
					GroupID:         nodeGroup.ID,
					InboundConfigID: pn.InboundConfigID,
					ExternalNodeID:  pn.ExternalNodeID,
					Enabled:         pn.Enabled,
					CreatedAt:       now,
				}
				if err := db.Create(member).Error; err != nil {
					log.Printf("WARNING: create node group member for plan_node %d: %v", pn.ID, err)
					continue
				}
				memberIDMap[pn.ID] = member.ID
			}
		}

		// Create plan template
		tmpl := &entity.PlanTemplate{
			Name:                       plan.Name,
			Description:                plan.Description,
			Category:                   entity.PlanCategoryStandard,
			NodeGroupIDs:               entity.Int64List{nodeGroup.ID},
			DefaultQuotaBytes:          plan.QuotaBytes,
			DefaultQuotaResetCycle:     "none",
			SuspendOnQuotaExhausted:    1,
			BillingUsedTemplate:        plan.BillingUsedTemplate,
			BillingRemainingTemplate:   plan.BillingRemainingTemplate,
			BillingResetTemplate:       plan.BillingResetTemplate,
			BillingExpiryTemplate:      plan.BillingExpiryTemplate,
			SortOrder:                  plan.SortOrder,
			Enabled:                    plan.Enabled,
			CreatedAt:                  now,
			UpdatedAt:                  now,
		}
		if err := db.Create(tmpl).Error; err != nil {
			log.Printf("WARNING: create plan template for plan %d: %v", plan.ID, err)
			continue
		}

		// Update clients.plan_id to point to the new template
		db.Table("clients").Where("plan_id = ?", plan.ID).Update("plan_id", tmpl.ID)

		// Update subscriptions.billing_node_ids for affected clients
		if len(memberIDMap) > 0 {
			var clientIDs []int64
			db.Table("clients").Where("plan_id = ?", tmpl.ID).Pluck("id", &clientIDs)
			for _, clientID := range clientIDs {
				var sub entity.Subscription
				if err := db.Where("client_id = ?", clientID).First(&sub).Error; err != nil {
					continue
				}
				if len(sub.BillingNodeIDs) > 0 {
					newIDs := make(entity.Int64List, 0, len(sub.BillingNodeIDs))
					for _, oldID := range sub.BillingNodeIDs {
						if newID, ok := memberIDMap[oldID]; ok {
							newIDs = append(newIDs, newID)
						}
					}
					sub.BillingNodeIDs = newIDs
					db.Save(&sub)
				}
			}
		}

		migratedCount++
	}

	if migratedCount > 0 {
		log.Printf("migrated %d legacy plans to plan_templates with node groups", migratedCount)
	}
}

// seedDefaultLogCleanupRules seeds the default log cleanup rules if none exist.
// These rules replace the previously hardcoded retention periods in log_cleanup.go.
func seedDefaultLogCleanupRules(db *gorm.DB) {
	var count int64
	db.Model(&entity.LogCleanupRule{}).Count(&count)
	if count > 0 {
		return
	}

	defaults := []entity.LogCleanupRule{
		// ── Traffic tables (30-day retention) ──
		{Category: "traffic", Table: "traffic_usage_ledger", DisplayName: "Traffic Usage Ledger", RetainDays: 30, Enabled: true, CleanupCron: "0 */6 * * *"},
		{Category: "traffic", Table: "traffic_sessions", DisplayName: "Traffic Sessions", RetainDays: 30, Enabled: true, CleanupCron: "0 */6 * * *"},
		{Category: "traffic", Table: "traffic_quota_reset_logs", DisplayName: "Traffic Quota Reset Logs", RetainDays: 30, Enabled: true, CleanupCron: "0 */6 * * *"},

		// ── Audit / Access log tables (90-day retention) ──
		{Category: "audit", Table: "subscription_access_logs", DisplayName: "Subscription Access Logs", RetainDays: 90, Enabled: true, CleanupCron: "0 */6 * * *"},
		{Category: "audit", Table: "subscription_rotations", DisplayName: "Subscription Token Rotations", RetainDays: 90, Enabled: true, CleanupCron: "0 */6 * * *"},
		{Category: "audit", Table: "security_audit_events", DisplayName: "Security Audit Events", RetainDays: 90, Enabled: true, CleanupCron: "0 */6 * * *"},
		{Category: "audit", Table: "security_alerts", DisplayName: "Security Alerts (resolved)", RetainDays: 90, Enabled: true, CleanupCron: "0 */6 * * *"},
		{Category: "audit", Table: "node_audit_logs", DisplayName: "Node Audit Logs", RetainDays: 90, Enabled: true, CleanupCron: "0 */6 * * *"},
		{Category: "audit", Table: "charge_security_events", DisplayName: "Charge Security Events", RetainDays: 90, Enabled: true, CleanupCron: "0 */6 * * *"},

		// ── Runtime tables (90-day retention) ──
		{Category: "runtime", Table: "core_crash_logs", DisplayName: "Core Crash Logs", RetainDays: 90, Enabled: true, CleanupCron: "0 */6 * * *"},
		{Category: "runtime", Table: "runtime_apply_job_events", DisplayName: "Runtime Apply Job Events", RetainDays: 90, Enabled: true, CleanupCron: "0 */6 * * *"},
		{Category: "runtime", Table: "runtime_apply_job_targets", DisplayName: "Runtime Apply Job Targets", RetainDays: 90, Enabled: true, CleanupCron: "0 */6 * * *"},
		{Category: "runtime", Table: "runtime_snapshots", DisplayName: "Runtime Snapshots", RetainDays: 90, Enabled: true, CleanupCron: "0 */6 * * *"},
		{Category: "runtime", Table: "runtime_apply_jobs", DisplayName: "Runtime Apply Jobs (completed)", RetainDays: 90, Enabled: true, CleanupCron: "0 */6 * * *"},
		{Category: "runtime", Table: "update_jobs", DisplayName: "Update Jobs (completed)", RetainDays: 90, Enabled: true, CleanupCron: "0 */6 * * *"},
		{Category: "runtime", Table: "config_snapshots", DisplayName: "Config Snapshots", RetainDays: 90, Enabled: true, CleanupCron: "0 */6 * * *"},
		{Category: "runtime", Table: "certificate_jobs", DisplayName: "Certificate Jobs (completed)", RetainDays: 90, Enabled: true, CleanupCron: "0 */6 * * *"},
		{Category: "runtime", Table: "log_rotation_jobs", DisplayName: "Log Rotation Job History", RetainDays: 90, Enabled: true, CleanupCron: "0 */6 * * *"},
		{Category: "runtime", Table: "backup_cleanup_jobs", DisplayName: "Backup Cleanup Job History", RetainDays: 90, Enabled: true, CleanupCron: "0 */6 * * *"},

		// ── Commerce log tables (180-day retention, financial logs need longer retention) ──
		{Category: "commerce", Table: "charge_callback_events", DisplayName: "Charge Callback Events", RetainDays: 180, Enabled: true, CleanupCron: "0 */6 * * *"},
		{Category: "commerce", Table: "charge_callback_retries", DisplayName: "Charge Callback Retries (resolved)", RetainDays: 180, Enabled: true, CleanupCron: "0 */6 * * *"},

		// ── Client log tables (90-day retention) ──
		{Category: "client", Table: "client_self_unban_logs", DisplayName: "Client Self-Unban Logs", RetainDays: 90, Enabled: true, CleanupCron: "0 */6 * * *"},
		{Category: "client", Table: "shared_account_repair_logs", DisplayName: "Shared Account Repair Logs", RetainDays: 90, Enabled: true, CleanupCron: "0 */6 * * *"},
		{Category: "client", Table: "client_log_packages", DisplayName: "Client Log Packages", RetainDays: 7, Enabled: true, CleanupCron: "0 */6 * * *"},

		// ── Metrics tables (7-day retention, time-series data needs short retention) ──
		{Category: "metrics", Table: "server_metrics", DisplayName: "Server Metrics", RetainDays: 7, Enabled: true, CleanupCron: "0 */6 * * *"},
		{Category: "metrics", Table: "speed_test_records", DisplayName: "Speed Test Records", RetainDays: 30, Enabled: true, CleanupCron: "0 */6 * * *"},
	}

	now := time.Now()
	for i := range defaults {
		defaults[i].CreatedAt = now
		defaults[i].UpdatedAt = now
		if err := db.Create(&defaults[i]).Error; err != nil {
			log.Printf("WARNING: seed log cleanup rule %s failed: %v", defaults[i].Table, err)
		}
	}
	log.Printf("seeded %d default log cleanup rules", len(defaults))
}
