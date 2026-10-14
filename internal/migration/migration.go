package migration

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"corefusion/internal/auth/password"
	"corefusion/internal/model/entity"
	"corefusion/internal/security/crypto"
	"corefusion/internal/store/repository"
)

// AutoMigrate runs auto-migration for all entity models.
func AutoMigrate(db *gorm.DB) error {
	models := []interface{}{
		// User & Auth
		&entity.User{},
		&entity.WebAuthnCredential{},
		&entity.RecoveryCode{},

		// Nodes
		&entity.Node{},

		// Core Configs
		&entity.CoreConfig{},
		&entity.InboundConfig{},
		&entity.OutboundConfig{},
		&entity.EndpointConfig{},

		// Services
		&entity.InboundService{},
		&entity.ServiceEndpoint{},
		&entity.NodeTrafficUsage{},

		// Access Control
		&entity.ServiceAccessPolicy{},
		&entity.ServiceIPBan{},

		// Subscriptions
		&entity.Subscription{},
		&entity.SubscriptionAccessLog{},
		&entity.SubscriptionRotation{},

		// Certificates
		&entity.Certificate{},

		// Traffic & Quota
		&entity.InboundUserTraffic{},
		&entity.TrafficQuota{},
		&entity.TrafficUsageLedger{},
		&entity.TrafficUsageSummary{},
		&entity.TrafficSession{},
		&entity.TrafficQuotaResetLog{},

		// Security & Audit
		&entity.SecurityAuditEvent{},

		// Runtime
		&entity.RuntimeSnapshot{},
		&entity.RuntimeApplyJob{},
		&entity.RuntimeApplyJobTarget{},
		&entity.RuntimeApplyJobEvent{},

		// Backups

		// Binaries & Updates

		// Routing

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
		&entity.EABConfig{},
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
		&entity.NotificationSendLog{},

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

		// Chain Proxy (new model: decoupled from node_type)
		&entity.ChainLink{},
		&entity.ExternalNodeServer{},

		// Node Servers (multi-region architecture)
		&entity.NodeServer{},
		&entity.NodeCertificate{}, // 节点 ↔ 证书 多对多 ✓（一个节点可绑多张 ✓）

		// Agent Certificate Revocations (CRL)
		&entity.AgentCertRevocation{},

		// Phase 6: Server Metrics & Speed Test Records
		&entity.ServerMetric{},

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
		&entity.RateLimitIPEntry{},

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

	// 把寄存在 inbound_configs.config_json 里的用户流量计数迁到 inbound_user_traffic，
	// 并顺带清掉 _quota/_expiry 遗留字段（幂等）
	migrateInboundUserTraffic(db)

	// 存量配置密文化：把 6 张配置表里的明文 config_json / proxy_json 与 WARP 敏感列
	// 加密为 "enc:" 形式（幂等，只处理不带前缀的存量明文）
	migrateConfigJSONEncryption(db)

	// Migrate geo rules from per-node (server_id=1) to global (server_id=0)
	migrateGeoRulesToGlobal(db)

	// Fix unique indexes to be per-server (GORM AutoMigrate cannot alter existing indexes)
	migrateUniqueIndexes(db)

	// v3.3: Migrate charge_cards from plaintext card_code to encrypted storage
	migrateChargeCardEncryption(db)

	// v3.5：DNS 渠道凭据（Cloudflare token / 谷歌 GCP service account JSON 等）
	// 从明文 base64 迁移到 AES-256-GCM 加密存储（与充值卡同一把密钥 ✓）
	migrateDNSChannelEncryption(db)

	// v3.5：EAB 凭据（EAB-HMAC-Key / GCP 服务账号 JSON）从明文迁移到加密存储
	migrateEABConfigEncryption(db)

	// v3.5：ACME 账号绑定的 EAB-HMAC-Key 同样从明文迁移到加密存储
	migrateACMEAccountConfigEncryption(db)

	// v3.3: Add agent_id column to charge_cards for agent/sales tracking
	migrateChargeCardAgentColumn(db)

	// v3.4：把历史上以「可逆形式」落库的客户端密码重新 bcrypt（详见函数注释）
	migrateClientPasswordHashes(db)

	// Seed default outbounds if none exist
	seedDefaultOutbounds(db)

	// Seed default share domain configs if none exist
	seedDefaultShareDomains(db)

	// Migrate legacy plans to plan_templates (one-time data migration)
	migratePlansToTemplates(db)

	// Seed default log cleanup rules
	seedDefaultLogCleanupRules(db)

	// Migrate legacy chain_entry/chain_exit to new chain_links model

	// v2.0：链式出口改为"使用出口节点上已有的入站"，旧模式（chain_exit_configs）已彻底移除 →
	// 该表不再被任何代码使用，直接删除（幂等；删前打印行数留底）。
	dropLegacyChainExitConfigs(db)

	// 2026-10 数据库审计：清掉全仓零引用的死表（0 行才删，详见 unusedLegacyTables）
	dropUnusedLegacyTables(db)

	// hysteria2/tuic 独立核心下线：清残留配置与服务条目，并给存量 sing-box
	// 的空壳 ntp/dns 换上有意义的默认值（详见函数注释）
	migrateDropStandaloneHy2Tuic(db)
	migrateSeedSingboxDefaults(db)

	return nil
}

// migrateClientPasswordHashes 把历史上以可恢复形式落库的客户端密码重新哈希成 bcrypt。
//
// 背景：客户端密码曾有两种非 bcrypt 形式落库：
//
//  1. 32 位 hex 盐 + ":" + 明文密码（早期 hashClientPassword 在 bcrypt 失败时的 fallback）；
//  2. 纯明文（更早的路径）。
//
// 后果有两条：
//
//  1. 密码可逆 —— 任何能读到 clients 表的人都直接拿到客户的门户密码；
//  2. 门户登录只走 bcrypt（clientauth/service.go 的 password.Check），这些账号永远登不上。
//
// 做法：把可恢复的原文取出来重新 bcrypt 落库。幂等 —— bcrypt 行以 $2 开头，
// 用 password NOT LIKE '$2%' 在 SQL 里排除，正常库上每次启动只是一次空查询。
// 无法处理的行（原文为空、或原文自身超过 72 字节）只记日志、保持原样，绝不写坏数据。
func migrateClientPasswordHashes(db *gorm.DB) {
	if !db.Migrator().HasTable("clients") {
		return
	}

	type legacyClient struct {
		ID       int64  `gorm:"column:id"`
		Password string `gorm:"column:password"`
	}
	var rows []legacyClient
	if err := db.Table("clients").
		Select("id, password").
		Where("password IS NOT NULL AND password <> '' AND password NOT LIKE ?", "$2%").
		Find(&rows).Error; err != nil {
		log.Printf("[migration] WARNING: scanning legacy client passwords failed: %v", err)
		return
	}
	if len(rows) == 0 {
		return
	}

	log.Printf("[migration] clients: found %d non-bcrypt password(s), re-hashing", len(rows))
	fixed, skipped := 0, 0
	for _, row := range rows {
		plain := legacyPasswordPlaintext(row.Password)
		if plain == "" {
			skipped++
			continue
		}
		hashed, err := password.Hash(plain)
		if err != nil {
			log.Printf("[migration] WARNING: client %d password cannot be re-hashed (%v); left as-is", row.ID, err)
			skipped++
			continue
		}
		if err := db.Exec("UPDATE clients SET password = ? WHERE id = ?", hashed, row.ID).Error; err != nil {
			log.Printf("[migration] WARNING: updating client %d password failed: %v", row.ID, err)
			skipped++
			continue
		}
		fixed++
	}
	log.Printf("[migration] clients: re-hashed %d legacy password(s), skipped %d", fixed, skipped)
}

// legacyPasswordPlaintext 从历史存储格式里取出原文：
//   - 32 位 hex 盐 + ":" + 明文（早期 fallback）→ 取冒号后的部分；
//   - 其他非 bcrypt 值（纯明文）→ 原样返回。
//
// 只识别已知的盐格式，其余按纯明文处理：即使判断错了，结果也不会比现状更差 ——
// 这些值本来就已经不可能通过 bcrypt 校验（等于账号本来也登不上）。
func legacyPasswordPlaintext(stored string) string {
	if i := strings.Index(stored, ":"); i == 32 && isHexString(stored[:i]) {
		return stored[i+1:]
	}
	return stored
}

func isHexString(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// dropLegacyChainExitConfigs 删除链式旧模式遗留的 `chain_exit_configs` 表。
//
// 背景：v2.0 起链式出口一律使用"出口节点上已有的入站"（chain_links.exit_inbound_id），
// 旧的 chain_exit_configs（自动创建 relay 中转入口）连同实体/仓库/接口一起删除了
// （见 17bb1493、ed20abd1）。表留着只会让人误会它还在生效 → 这里直接 drop。
//
// 幂等：先判 HasTable ✓，表不存在时什么都不做 ✓。
func dropLegacyChainExitConfigs(db *gorm.DB) {
	if !db.Migrator().HasTable("chain_exit_configs") {
		return
	}
	var rows int64
	if err := db.Table("chain_exit_configs").Count(&rows).Error; err == nil {
		log.Printf("[migration] dropping legacy chain_exit_configs table (%d rows, mode removed in chain v2.0)", rows)
	} else {
		log.Printf("[migration] dropping legacy chain_exit_configs table (mode removed in chain v2.0)")
	}
	if err := db.Migrator().DropTable("chain_exit_configs"); err != nil {
		log.Printf("[migration] drop chain_exit_configs failed: %v", err)
	}
}

// unusedLegacyTables 是 2026-10 数据库审计确认的**死表**：
// 有实体/AutoMigrate 历史登记但全仓零业务引用（无仓储、无服务、无 handler 读写），
// 表内 0 行。实体与 AutoMigrate 注册已一并删除，这里把表本身 drop 掉，
// 免得库里留一堆让人误以为在生效的空表。
//
// 排查口径：结构体名 + 裸表名全仓 grep（排除 model/migration/db 注册），
// 并复核 log_cleanup 清单与数据迁移的特殊引用（update_jobs 等三条清理规则已随删）。
// 注：recovery_codes 虽 0 行，但 2FA 恢复码服务在用（router twoFASvc）→ 不在此列。
var unusedLegacyTables = []string{
	"charge_callback_logs",      // 无实体，历史回调日志
	"routing_regions",           // 2026-09-23 已摘 AutoMigrate，注释也标注"保留但未接线"
	"acme_accounts",             // 旧 ACME 账号表，已被 acme_account_configs 取代
	"reverse_proxy_instances",   // 反代实例（本地 caddy/nginx 特性未落地）
	"speed_test_records",        // 测速结果只在内存/日志，从不落库
	"binary_releases",           // 更新功能走面板内存任务，从不落库
	"update_jobs",               // 同上
	"backup_retention_policies", // 备份保留策略未接线（backup_records/schedules 在用，不受影响）
	"backup_cleanup_jobs",       // 同上
	"certificate_jobs",          // 证书任务历史未接线（certificates 在用）
	"service_geo_rules",         // 独立服务特性中唯一零引用的附属表
	"server_share_profiles",     // 实体零引用（domain/subscription 的同名 DTO 是另一类型）
}

// dropUnusedLegacyTables 删除上述死表。幂等：HasTable 判在先。
func dropUnusedLegacyTables(db *gorm.DB) {
	for _, t := range unusedLegacyTables {
		if !db.Migrator().HasTable(t) {
			continue
		}
		var rows int64
		if err := db.Table(t).Count(&rows).Error; err == nil && rows > 0 {
			// 有数据的表不 drop —— 审计口径是「0 行死表」，真有数据要人工确认
			log.Printf("[migration] skip dropping %s: %d rows present, needs manual review", t, rows)
			continue
		}
		if err := db.Migrator().DropTable(t); err != nil {
			log.Printf("[migration] drop %s failed: %v", t, err)
			continue
		}
		log.Printf("[migration] dropped unused legacy table: %s", t)
	}
}

// migrateDropStandaloneHy2Tuic 下线 hysteria2/tuic 独立核心（2026-10 决策）：
//   - core_configs 里这两个核心的分段配置（listen/auth/users…）
//   - 所有节点（含远程）为它们 seed 的 service 行与关联 guardian
//   - 日志轮转策略（hysteria2.log / tuic.log）与日志设置里的 hy2/tuic 键
//
// 保留：出站/外部节点的 hysteria2/tuic 支持（domain.CoreType 域，与本清理无关）、
// sing-box 内的 hy2/tuic 入站、磁盘上已下载的二进制文件（不删，留作手工使用）。
// 幂等：先判行数，没数据不动。
func migrateDropStandaloneHy2Tuic(db *gorm.DB) {
	dead := []string{"hysteria2", "tuic"}

	// 1. 分段配置行
	for _, ct := range dead {
		res := db.Where("core_type = ?", ct).Delete(&entity.CoreConfig{})
		if res.Error != nil {
			log.Printf("[migration] drop core_configs/%s failed: %v", ct, res.Error)
		} else if res.RowsAffected > 0 {
			log.Printf("[migration] dropped %d core_configs rows of standalone core %s", res.RowsAffected, ct)
		}
	}

	// 2. service + 关联 guardian（**所有节点**：1 为本地，>1 为远程）。
	//    之前只清 server_id=1，导致远程节点上一直残留 hy2/tuic 服务行并随 SyncData 下发。
	var svcIDs []int64
	if err := db.Table("inbound_services").
		Where("core_type IN ?", dead).
		Pluck("id", &svcIDs).Error; err != nil {
		log.Printf("[migration] list hy2/tuic services failed: %v", err)
	} else if len(svcIDs) > 0 {
		if err := db.Where("service_id IN ?", svcIDs).Delete(&entity.ProcessGuardian{}).Error; err != nil {
			log.Printf("[migration] drop guardians for hy2/tuic failed: %v", err)
		}
		if res := db.Where("id IN ?", svcIDs).Delete(&entity.InboundService{}); res.Error != nil {
			log.Printf("[migration] drop hy2/tuic services failed: %v", res.Error)
		} else {
			log.Printf("[migration] dropped %d service rows (+guardians) for standalone hy2/tuic", res.RowsAffected)
		}
	}

	// 3. 日志轮转策略（hysteria2.log / tuic.log）
	if res := db.Where("source_type IN ?", dead).Delete(&entity.LogRotationPolicy{}); res.Error != nil {
		log.Printf("[migration] drop log_rotation_policies for hy2/tuic failed: %v", res.Error)
	} else if res.RowsAffected > 0 {
		log.Printf("[migration] dropped %d log_rotation_policies of hy2/tuic", res.RowsAffected)
	}

	// 4. 日志设置：删掉 proxy.log_writers.{hysteria2,tuic}；从 core.log_output_config[.N]
	//    的 JSON 里剔除 hy2/tuic 键（代码现在只写 xray/singbox，留着会在日志配置里显示幽灵核心）。
	for _, ct := range dead {
		db.Exec("DELETE FROM system_settings WHERE key = ?", "proxy.log_writers."+ct)
	}
	stripStandaloneCoreFromLogOutputSettings(db, dead)
}

// stripStandaloneCoreFromLogOutputSettings 从 core.log_output_config[.N] 的 JSON 值里
// 删掉已下线的独立核心键。解析失败/无该键则原样保留。
func stripStandaloneCoreFromLogOutputSettings(db *gorm.DB, dead []string) {
	var rows []entity.SystemSetting
	if err := db.Where("key = ? OR key LIKE ?", "core.log_output_config", "core.log_output_config.%").
		Find(&rows).Error; err != nil {
		log.Printf("[migration] load core.log_output_config failed: %v", err)
		return
	}
	for _, row := range rows {
		var m map[string]json.RawMessage
		if err := json.Unmarshal([]byte(row.Value), &m); err != nil {
			continue
		}
		changed := false
		for _, ct := range dead {
			if _, ok := m[ct]; ok {
				delete(m, ct)
				changed = true
			}
		}
		if !changed {
			continue
		}
		out, err := json.Marshal(m)
		if err != nil {
			continue
		}
		if err := db.Model(&entity.SystemSetting{}).Where("key = ?", row.Key).Update("value", string(out)).Error; err != nil {
			log.Printf("[migration] strip hy2/tuic from %s failed: %v", row.Key, err)
		} else {
			log.Printf("[migration] stripped hy2/tuic keys from setting %s", row.Key)
		}
	}
}

// migrateSeedSingboxDefaults 把存量 sing-box 的**空壳** ntp / dns 换成
// coreconfig.DefaultSectionJSON 里的新默认（打开协议中心才有内容，此前是
// {} / 空 servers）。只在「确实没人配置过」时替换：
//   - ntp：解析后是空对象
//   - dns：servers 为空数组且没有 fakeip/hosts 类 server
//
// 用户配置过的原样保留。幂等：替换后即不满足条件。
func migrateSeedSingboxDefaults(db *gorm.DB) {
	// 默认值与 coreconfig.DefaultSectionJSON["singbox"] 保持一致（那边改这里也要改）
	const (
		ntpDefault = `{"enabled":true,"server":"time.cloudflare.com","server_port":123,"interval":"30m"}`
		dnsDefault = `{"servers":[{"type":"local","tag":"dns-local"},{"type":"udp","server":"1.1.1.1","server_port":53,"tag":"dns-remote"}],"rules":[],"final":"dns-remote"}`
	)
	type row struct {
		ID      int64
		Section string
		Val     string
	}
	var rows []row
	if err := db.Table("core_configs").
		Select("id, section, config_json AS val").
		Where("core_type = ? AND section IN ?", "singbox", []string{"ntp", "dns"}).
		Scan(&rows).Error; err != nil {
		log.Printf("[migration] scan singbox ntp/dns failed: %v", err)
		return
	}
	updated := 0
	for _, r := range rows {
		var v map[string]interface{}
		if err := json.Unmarshal([]byte(r.Val), &v); err != nil {
			continue
		}
		var def string
		switch r.Section {
		case "ntp":
			if len(v) != 0 {
				continue // 不是空壳 → 用户配置过，保留
			}
			def = ntpDefault
		case "dns":
			servers, _ := v["servers"].([]interface{})
			if len(servers) != 0 {
				continue // 配过 DNS server，保留
			}
			def = dnsDefault
		}
		if err := db.Table("core_configs").Where("id = ?", r.ID).Update("config_json", def).Error; err != nil {
			log.Printf("[migration] seed singbox %s (id=%d) failed: %v", r.Section, r.ID, err)
			continue
		}
		updated++
	}
	if updated > 0 {
		log.Printf("[migration] seeded defaults for %d empty singbox ntp/dns sections", updated)
	}
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
//  1. AutoMigrate 已添加新列 (card_code_hash, card_code_encrypted, created_by_admin_id, created_by_admin_name)
//  2. 对已有行：card_code → hash + encrypt，然后删除旧 card_code 列
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

// migrateDNSChannelEncryption v3.5：把 DNS 渠道凭据从**裸 base64(JSON)** 迁移到
// AES-256-GCM 加密存储 ✓（与充值卡 / CA 私钥共用同一把密钥 ✓，见 internal/security/crypto ✓）。
//
// 为什么只做"再加密一层" ✗：历史行里存的就是 `base64({"CF_Token":"..."})` ✓ ——
// 明文可逆 ✗（`base64 -d` 就能拿到 token ✗）。加密后的读路径是
// `DecryptSecret → base64 decode → JSON` ✓（见 service/certificate/channel.go ✓），
// 所以这里把**整个旧串**当作待解密的明文原样包一层即可 ✓，语义不变 ✓。
//
// 幂等 ✓：只处理**不带 `enc:` 前缀**的行 ✓（历史 base64 不含冒号前缀 ✓），
// 迁完再跑命中 0 行 ✓。已部署实例重启一次即完成 ✓，不迁移也能读 ✓（读路径兼容旧格式 ✓）。
func migrateDNSChannelEncryption(db *gorm.DB) {
	if !db.Migrator().HasTable("dns_channels") {
		return
	}

	type dnsChannelRow struct {
		ID      int64  `gorm:"column:id"`
		EnvVars string `gorm:"column:env_vars"`
	}
	var rows []dnsChannelRow
	if err := db.Table("dns_channels").
		Where("env_vars IS NOT NULL AND env_vars != '' AND env_vars NOT LIKE 'enc:%'").
		Find(&rows).Error; err != nil {
		log.Printf("[migration] dns_channels: 查询待加密凭据失败：%v", err)
		return
	}
	if len(rows) == 0 {
		return // 已迁移过（或本来就没有渠道）✓
	}

	log.Printf("[migration] dns_channels: 待加密凭据 %d 条", len(rows))
	migrated := 0
	for _, r := range rows {
		enc, err := encryptPlainSecret(r.EnvVars)
		if err != nil {
			log.Printf("[migration] WARNING: dns_channels %d 凭据加密失败：%v", r.ID, err)
			continue
		}
		if err := db.Exec("UPDATE dns_channels SET env_vars = ? WHERE id = ?", enc, r.ID).Error; err != nil {
			log.Printf("[migration] WARNING: dns_channels %d 凭据回写失败：%v", r.ID, err)
			continue
		}
		migrated++
	}
	log.Printf("[migration] dns_channels: 已加密 %d/%d 条凭据", migrated, len(rows))
}

// migrateEABConfigEncryption v3.5：把 EAB 凭据（`mac_key` = EAB-HMAC-Key ✓、
// `google_sa_json` = GCP 服务账号密钥 JSON ✓）从明文迁移到 AES-256-GCM 加密存储 ✓
// （与充值卡 / DNS 渠道凭据 **同一把密钥** ✓，见 internal/security/crypto ✓）。
//
// 为什么这两项必须加密 ✗：它们都能直接换到 CA 侧的身份 ✗ ——
//   - `mac_key`：拿去就能注册 Google / ZeroSSL 的 ACME 账号 ✓；
//   - `google_sa_json`：内含 `private_key` ✓ → 可以以该服务账号身份调 GCP 接口 ✓
//     （申请 EAB、乃至这个项目下它能碰到的任何 API ✗）。
//
// 而它们原先在库里是**明文** ✗（`sqlite3` 一条 select 就能看到私钥 ✗）。
//
// 幂等 ✓：只处理不以 `enc:` 开头的非空值 ✓ —— mac_key 是 base64url（无冒号 ✓）、
// 服务账号 JSON 以 `{` 开头 ✓，都不会被误判成已加密 ✓。
func migrateEABConfigEncryption(db *gorm.DB) {
	if !db.Migrator().HasTable("eab_configs") {
		return
	}

	type eabRow struct {
		ID           int64  `gorm:"column:id"`
		MACKey       string `gorm:"column:mac_key"`
		GoogleSAJSON string `gorm:"column:google_sa_json"`
	}
	var rows []eabRow
	if err := db.Table("eab_configs").
		Where("(mac_key IS NOT NULL AND mac_key != '' AND mac_key NOT LIKE 'enc:%')" +
			" OR (google_sa_json IS NOT NULL AND google_sa_json != '' AND google_sa_json NOT LIKE 'enc:%')").
		Find(&rows).Error; err != nil {
		log.Printf("[migration] eab_configs: 查询待加密凭据失败：%v", err)
		return
	}
	if len(rows) == 0 {
		return // 已迁移过 ✓
	}

	log.Printf("[migration] eab_configs: 待加密凭据 %d 条", len(rows))
	migrated := 0
	for _, r := range rows {
		mac, err := encryptPlainSecret(r.MACKey)
		if err != nil {
			log.Printf("[migration] WARNING: eab_configs %d mac_key 加密失败：%v", r.ID, err)
			continue
		}
		sa, err := encryptPlainSecret(r.GoogleSAJSON)
		if err != nil {
			log.Printf("[migration] WARNING: eab_configs %d google_sa_json 加密失败：%v", r.ID, err)
			continue
		}
		// 两个字段一起回写 ✓：避免"mac_key 已加密、sa_json 还是明文"的半迁移状态 ✗（再跑一次也能补上 ✓）。
		if err := db.Exec("UPDATE eab_configs SET mac_key = ?, google_sa_json = ? WHERE id = ?", mac, sa, r.ID).Error; err != nil {
			log.Printf("[migration] WARNING: eab_configs %d 凭据回写失败：%v", r.ID, err)
			continue
		}
		migrated++
	}
	log.Printf("[migration] eab_configs: 已加密 %d/%d 条凭据", migrated, len(rows))
}

// migrateACMEAccountConfigEncryption v3.5：把 ACME 账号绑定用的 EAB-HMAC-Key
// （`acme_account_configs.eabmac_key` ✓）从明文迁移到 AES-256-GCM 加密存储 ✓。
//
// 为什么单独写 ✗（和 `eab_configs` 的迁移不合并 ✓）：
// `eab_configs` 是"EAB 凭据**主存储**" ✓（面板拿它去 CA 申请 EAB ✓），
// 这里是"某个账号**绑定**的那对 KID/MAC" ✓（签发时按所选账号自动带上 ✓）——
// 两张表各有独立的写入路径 ✓，混在一个循环里只会让幂等判断更难写对 ✗。
//
// 幂等 ✓：只挑不以 `enc:` 开头的非空值 ✓（EAB MAC 是 base64url ✓ 不含冒号 ✓ 不会被误判 ✓）。
func migrateACMEAccountConfigEncryption(db *gorm.DB) {
	if !db.Migrator().HasTable("acme_account_configs") {
		return
	}

	type row struct {
		ID        int64  `gorm:"column:id"`
		EABMACKey string `gorm:"column:eabmac_key"`
	}
	var rows []row
	if err := db.Table("acme_account_configs").
		Where("eabmac_key IS NOT NULL AND eabmac_key != '' AND eabmac_key NOT LIKE 'enc:%'").
		Find(&rows).Error; err != nil {
		log.Printf("[migration] acme_account_configs: 查询待加密 EAB MAC 失败：%v", err)
		return
	}
	if len(rows) == 0 {
		return // 没有明文或已迁移过 ✓
	}

	log.Printf("[migration] acme_account_configs: 待加密 EAB MAC %d 条", len(rows))
	migrated := 0
	for _, r := range rows {
		enc, err := encryptPlainSecret(r.EABMACKey)
		if err != nil {
			log.Printf("[migration] WARNING: acme_account_configs %d EAB MAC 加密失败：%v", r.ID, err)
			continue
		}
		if err := db.Exec("UPDATE acme_account_configs SET eabmac_key = ? WHERE id = ?", enc, r.ID).Error; err != nil {
			log.Printf("[migration] WARNING: acme_account_configs %d 回写失败：%v", r.ID, err)
			continue
		}
		migrated++
	}
	log.Printf("[migration] acme_account_configs: 已加密 %d/%d 条 EAB MAC", migrated, len(rows))
}

// encryptPlainSecret 加密一个"确定是明文"的值 ✓：空值保持空 ✓、已加密的原样返回 ✓（幂等 ✓）。
func encryptPlainSecret(s string) (string, error) {
	if strings.TrimSpace(s) == "" || crypto.IsEncrypted(s) {
		return s, nil
	}
	return crypto.EncryptSecret(s)
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

// migrateInboundUserTraffic 把寄存在 inbound_configs.config_json 里的**面板元数据**
// 一次性迁出（幂等，可重复执行）：
//
//   - _usedUp/_usedDown → inbound_user_traffic 表（按 入站×email 累加）
//   - _quota/_expiry     → 直接删除（前端遗留字段，后端从不读取）
//
// _paused / _accessDomain / _portHop 属于配置语义，保持不动。
// 每条配置的「写流量表 + 改 config_json」在同一个事务里完成，避免半途失败导致
// 计数被重复累加。
func migrateInboundUserTraffic(db *gorm.DB) {
	var configs []entity.InboundConfig
	err := db.Where(
		"config_json LIKE ? OR config_json LIKE ? OR config_json LIKE ? OR config_json LIKE ?",
		"%\"_usedUp\"%", "%\"_usedDown\"%", "%\"_quota\"%", "%\"_expiry\"%",
	).Find(&configs).Error
	if err != nil {
		log.Printf("[migration] inbound_user_traffic: scan failed: %v", err)
		return
	}
	if len(configs) == 0 {
		return
	}

	panelKeys := []string{"_usedUp", "_usedDown", "_quota", "_expiry"}
	moved, cleaned := 0, 0

	for _, cfg := range configs {
		var inbound map[string]interface{}
		if err := json.Unmarshal([]byte(cfg.ConfigJSON), &inbound); err != nil {
			continue
		}

		type counter struct {
			email string
			up    int64
			down  int64
		}
		var counters []counter
		changed := false

		for _, container := range panelMetaContainers(inbound) {
			for _, key := range []string{"users", "clients", "accounts"} {
				items, ok := container[key].([]interface{})
				if !ok {
					continue
				}
				for _, item := range items {
					m, ok := item.(map[string]interface{})
					if !ok {
						continue
					}
					up := panelMetaInt64(m["_usedUp"])
					down := panelMetaInt64(m["_usedDown"])
					email := panelMetaEmail(m)
					for _, k := range panelKeys {
						if _, exists := m[k]; exists {
							delete(m, k)
							changed = true
						}
					}
					if email != "" && (up != 0 || down != 0) {
						counters = append(counters, counter{email: email, up: up, down: down})
					}
				}
			}
		}
		if !changed {
			continue
		}

		data, err := json.Marshal(inbound)
		if err != nil {
			continue
		}
		newJSON := string(data)

		txErr := db.Transaction(func(tx *gorm.DB) error {
			for _, c := range counters {
				row := entity.InboundUserTraffic{
					ServerID:  cfg.ServerID,
					InboundID: cfg.ID,
					Email:     c.email,
					UsedUp:    c.up,
					UsedDown:  c.down,
					UpdatedAt: time.Now(),
				}
				if err := tx.Clauses(clause.OnConflict{
					Columns: []clause.Column{{Name: "server_id"}, {Name: "inbound_id"}, {Name: "email"}},
					DoUpdates: clause.Assignments(map[string]interface{}{
						"used_up":    gorm.Expr("used_up + ?", c.up),
						"used_down":  gorm.Expr("used_down + ?", c.down),
						"updated_at": time.Now(),
					}),
				}).Create(&row).Error; err != nil {
					return err
				}
			}
			return tx.Model(&entity.InboundConfig{}).
				Where("id = ?", cfg.ID).
				Update("config_json", newJSON).Error
		})
		if txErr != nil {
			log.Printf("[migration] inbound_user_traffic: config %d (%s) failed: %v", cfg.ID, cfg.Tag, txErr)
			continue
		}
		moved += len(counters)
		cleaned++
	}

	log.Printf("[migration] inbound_user_traffic: moved %d user counter(s) out of config_json, cleaned %d config(s)", moved, cleaned)
}

// panelMetaContainers 返回可存放用户条目的容器（xray: 顶层 + settings；
// sing-box: 顶层）。
func panelMetaContainers(inbound map[string]interface{}) []map[string]interface{} {
	containers := []map[string]interface{}{inbound}
	if settings, ok := inbound["settings"].(map[string]interface{}); ok {
		containers = append(containers, settings)
	}
	return containers
}

// panelMetaEmail 返回用户条目的身份字段（与归户口径一致）。
func panelMetaEmail(m map[string]interface{}) string {
	for _, key := range []string{"email", "user", "name", "username"} {
		if v, _ := m[key].(string); v != "" {
			return v
		}
	}
	return ""
}

// panelMetaInt64 兼容 JSON 数字（float64）与整数两种形态。
func panelMetaInt64(v interface{}) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	case int:
		return int64(n)
	}
	return 0
}

// migrateConfigJSONEncryption 把配置表里的存量明文密文化（与充值卡同一把密钥、
// 同一 "enc:" 格式），幂等：只挑不带前缀的行。
//
// 覆盖范围（第 5 步加密方案的 6 张表 + WARP 敏感列）：
//   - inbound_configs / outbound_configs / core_configs / endpoint_configs /
//     mihomo_configs 的 config_json
//   - external_nodes 的 proxy_json（含 uuid / password / obfs-password）
//   - warp_configs 的 token / private_key / license_key / config_json
//
// 不覆盖 inbound_services.config_json（另一套「服务」特性，未纳入本次范围）。
func migrateConfigJSONEncryption(db *gorm.DB) {
	type row struct {
		ID  int64
		Val string
	}

	jsonTables := []struct {
		table string
		col   string
	}{
		{"inbound_configs", "config_json"},
		{"outbound_configs", "config_json"},
		{"core_configs", "config_json"},
		{"endpoint_configs", "config_json"},
		{"mihomo_configs", "config_json"},
		{"external_nodes", "proxy_json"},
	}

	for _, t := range jsonTables {
		var rows []row
		if err := db.Table(t.table).
			Select("id, " + t.col + " AS val").
			Where(t.col + " IS NOT NULL AND " + t.col + " != ''").
			Scan(&rows).Error; err != nil {
			log.Printf("[migration] config encryption scan %s failed: %v", t.table, err)
			continue
		}
		encrypted, decrypted := 0, 0
		for _, r := range rows {
			alreadyEnc := strings.HasPrefix(r.Val, "enc:")
			plain := r.Val
			if alreadyEnc {
				p, err := crypto.DecryptSecret(r.Val)
				if err != nil {
					// 密钥不符（跨机恢复）→ 保持原样，不要覆盖成明文
					log.Printf("[migration] config encryption %s#%d decrypt failed: %v", t.table, r.ID, err)
					continue
				}
				plain = p
			}
			sensitive := repository.IsSensitiveConfigJSON(plain)
			switch {
			case sensitive && !alreadyEnc:
				enc, err := crypto.EncryptSecret(plain)
				if err != nil {
					log.Printf("[migration] config encryption %s#%d failed: %v", t.table, r.ID, err)
					continue
				}
				if err := db.Table(t.table).Where("id = ?", r.ID).Update(t.col, enc).Error; err != nil {
					log.Printf("[migration] config encryption write %s#%d failed: %v", t.table, r.ID, err)
					continue
				}
				encrypted++
			case !sensitive && alreadyEnc:
				// 早前版本「整包加密」遗留的无凭据密文 → 解回明文，保持 DB 可读
				if err := db.Table(t.table).Where("id = ?", r.ID).Update(t.col, plain).Error; err != nil {
					log.Printf("[migration] config decrypt-back write %s#%d failed: %v", t.table, r.ID, err)
					continue
				}
				decrypted++
			}
		}
		if encrypted > 0 || decrypted > 0 {
			log.Printf("[migration] config crypto: %s.%s encrypted %d row(s), plaintext-restored %d row(s)",
				t.table, t.col, encrypted, decrypted)
		}
	}

	// mihomo 的 TLS 私钥与代理认证口令同样是「整列凭据」，一律加密
	for _, t := range []struct{ table, col string }{
		{"mihomo_tls", "private_key"},
		{"mihomo_general", "authentication"},
	} {
		var rows []row
		if err := db.Table(t.table).
			Select("id, "+t.col+" AS val").
			Where(t.col+" IS NOT NULL AND "+t.col+" != '' AND "+t.col+" NOT LIKE ?", "enc:%").
			Scan(&rows).Error; err != nil {
			log.Printf("[migration] mihomo encryption scan %s failed: %v", t.table, err)
			continue
		}
		for _, r := range rows {
			enc, err := crypto.EncryptSecret(r.Val)
			if err != nil {
				continue
			}
			if err := db.Table(t.table).Where("id = ?", r.ID).Update(t.col, enc).Error; err != nil {
				log.Printf("[migration] mihomo encryption write %s#%d failed: %v", t.table, r.ID, err)
			}
		}
	}

	// WARP 是独立列存储（整列就是凭据，没有键名可判定），四列一律加密
	for _, col := range []string{"token", "private_key", "license_key", "config_json"} {
		var rows []row
		if err := db.Table("warp_configs").
			Select("id, "+col+" AS val").
			Where(col+" IS NOT NULL AND "+col+" != '' AND "+col+" NOT LIKE ?", "enc:%").
			Scan(&rows).Error; err != nil {
			log.Printf("[migration] warp encryption scan %s failed: %v", col, err)
			continue
		}
		for _, r := range rows {
			enc, err := crypto.EncryptSecret(r.Val)
			if err != nil {
				log.Printf("[migration] warp encryption %s#%d failed: %v", col, r.ID, err)
				continue
			}
			if err := db.Table("warp_configs").Where("id = ?", r.ID).Update(col, enc).Error; err != nil {
				log.Printf("[migration] warp encryption write %s#%d failed: %v", col, r.ID, err)
			}
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
		ID                       int64            `gorm:"column:id"`
		Name                     string           `gorm:"column:name"`
		Description              string           `gorm:"column:description"`
		QuotaBytes               int64            `gorm:"column:quota_bytes"`
		Duration                 int              `gorm:"column:duration"`
		Enabled                  bool             `gorm:"column:enabled"`
		SortOrder                int              `gorm:"column:sort_order"`
		BillingUsedTemplate      string           `gorm:"column:billing_used_template"`
		BillingRemainingTemplate string           `gorm:"column:billing_remaining_template"`
		BillingResetTemplate     string           `gorm:"column:billing_reset_template"`
		BillingExpiryTemplate    string           `gorm:"column:billing_expiry_template"`
		BillingInboundConfigIDs  entity.Int64List `gorm:"column:billing_inbound_config_ids"`
		BillingExternalNodeIDs   entity.Int64List `gorm:"column:billing_external_node_ids"`
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
			Name:                     plan.Name,
			Description:              plan.Description,
			Category:                 entity.PlanCategoryStandard,
			NodeGroupIDs:             entity.Int64List{nodeGroup.ID},
			DefaultQuotaBytes:        plan.QuotaBytes,
			DefaultQuotaResetCycle:   "none",
			SuspendOnQuotaExhausted:  1,
			BillingUsedTemplate:      plan.BillingUsedTemplate,
			BillingRemainingTemplate: plan.BillingRemainingTemplate,
			BillingResetTemplate:     plan.BillingResetTemplate,
			BillingExpiryTemplate:    plan.BillingExpiryTemplate,
			SortOrder:                plan.SortOrder,
			Enabled:                  plan.Enabled,
			CreatedAt:                now,
			UpdatedAt:                now,
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
		{Category: "audit", Table: "node_audit_logs", DisplayName: "Node Audit Logs", RetainDays: 90, Enabled: true, CleanupCron: "0 */6 * * *"},
		{Category: "audit", Table: "charge_security_events", DisplayName: "Charge Security Events", RetainDays: 90, Enabled: true, CleanupCron: "0 */6 * * *"},

		// ── Runtime tables (90-day retention) ──
		{Category: "runtime", Table: "core_crash_logs", DisplayName: "Core Crash Logs", RetainDays: 90, Enabled: true, CleanupCron: "0 */6 * * *"},
		{Category: "runtime", Table: "runtime_apply_job_events", DisplayName: "Runtime Apply Job Events", RetainDays: 90, Enabled: true, CleanupCron: "0 */6 * * *"},
		{Category: "runtime", Table: "runtime_apply_job_targets", DisplayName: "Runtime Apply Job Targets", RetainDays: 90, Enabled: true, CleanupCron: "0 */6 * * *"},
		{Category: "runtime", Table: "runtime_snapshots", DisplayName: "Runtime Snapshots", RetainDays: 90, Enabled: true, CleanupCron: "0 */6 * * *"},
		{Category: "runtime", Table: "runtime_apply_jobs", DisplayName: "Runtime Apply Jobs (completed)", RetainDays: 90, Enabled: true, CleanupCron: "0 */6 * * *"},
		{Category: "runtime", Table: "config_snapshots", DisplayName: "Config Snapshots", RetainDays: 90, Enabled: true, CleanupCron: "0 */6 * * *"},
		{Category: "runtime", Table: "log_rotation_jobs", DisplayName: "Log Rotation Job History", RetainDays: 90, Enabled: true, CleanupCron: "0 */6 * * *"},

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
