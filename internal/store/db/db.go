package db

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"corefusion/internal/migration"
	"corefusion/internal/model/entity"

	"github.com/qist/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// compile-time check
var _ = strconv.IntSize

// DB is the global database instance.
var DB *gorm.DB

// currentCfg holds the config used at Init time so that other packages
// can query the database type and DSN at runtime.
var currentCfg Config

// Database type constants.
const (
	DBTypeSQLite   = "sqlite"
	DBTypeMySQL    = "mysql"
	DBTypePostgres = "postgres"
)

// dialectorFactory creates a gorm.Dialector from a DSN string.
type dialectorFactory func(dsn string) gorm.Dialector

// dialectorRegistry holds registered dialector factories.
// SQLite is built-in. MySQL/PostgreSQL are registered by their respective files.
var dialectorRegistry = map[string]dialectorFactory{}

// RegisterDialector registers a dialector factory for the given database type.
// Called from db_mysql.go and db_postgres.go init() functions.
func RegisterDialector(dbType string, factory dialectorFactory) {
	dialectorRegistry[dbType] = factory
}

// Config holds database configuration.
//
// Configuration via DSN (recommended):
//
//	CF_DB_DSN=mysql://user:password@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local
//	CF_DB_DSN=postgres://user:password@127.0.0.1:5432/dbname?sslmode=disable
//	CF_DB_DSN=sqlite:///path/to/app.db
//	CF_DB_DSN=/path/to/app.db  (bare path defaults to SQLite)
//
// When CF_DB_DSN has a scheme prefix (mysql://, postgres://, sqlite://),
// Type is auto-detected. Otherwise, CF_DB_TYPE can be set explicitly.
type Config struct {
	Type     string // sqlite | mysql | postgres (default: auto-detect from DSN, fallback: sqlite)
	LogLevel logger.LogLevel

	// DSN is the database connection string.
	//   - SQLite:     file path (e.g. "data/sqlite/app.db")
	//   - MySQL:      "mysql://user:password@tcp(host:port)/dbname?params"
	//                 or native Go MySQL DSN: "user:password@tcp(host:port)/dbname?params"
	//   - PostgreSQL: "postgres://user:password@host:port/dbname?sslmode=disable"
	DSN string

	// Connection pool (MySQL / PostgreSQL only)
	MaxOpenConns    int           // Maximum open connections (default: 100)
	MaxIdleConns    int           // Maximum idle connections (default: 10)
	ConnMaxLifetime time.Duration // Maximum connection lifetime (default: 1h)
}

// DefaultConfig returns the default database config (SQLite).
func DefaultConfig() Config {
	return Config{
		Type:     DBTypeSQLite,
		DSN:      "data/sqlite/app.db",
		LogLevel: logger.Warn,
	}
}

// ConfigFromEnv builds a Config from environment variables or .env map.
//
// Reads CF_DB_DSN (primary) and CF_DB_TYPE (fallback for type hint).
// If neither is set, defaults to SQLite with the given defaultSQLitePath.
//
// DSN auto-detection rules:
//   - "mysql://..."        → MySQL
//   - "postgres://..."     → PostgreSQL (also accepts "postgresql://")
//   - "sqlite://..."       → SQLite (strips the scheme prefix)
//   - bare path (no "://") → SQLite (treated as file path)
func ConfigFromEnv(env map[string]string, defaultSQLitePath string) Config {
	cfg := DefaultConfig()
	cfg.DSN = defaultSQLitePath

	dsn := env["CF_DB_DSN"]
	dbType := env["CF_DB_TYPE"]

	if dsn != "" {
		cfg.DSN = dsn
		// Auto-detect type from DSN scheme
		switch {
		case strings.HasPrefix(dsn, "mysql://"):
			cfg.Type = DBTypeMySQL
			cfg.DSN = strings.TrimPrefix(dsn, "mysql://")
		case strings.HasPrefix(dsn, "postgres://"), strings.HasPrefix(dsn, "postgresql://"):
			cfg.Type = DBTypePostgres
			// Keep the full postgres:// DSN — Go's pgx driver uses this format natively
		case strings.HasPrefix(dsn, "sqlite://"):
			cfg.Type = DBTypeSQLite
			cfg.DSN = strings.TrimPrefix(dsn, "sqlite://")
		default:
			// Bare path → SQLite
			cfg.Type = DBTypeSQLite
		}
	}

	// CF_DB_TYPE overrides auto-detected type (if explicitly set)
	if dbType != "" {
		cfg.Type = dbType
	}

	// Connection pool for MySQL / PostgreSQL
	if cfg.Type != DBTypeSQLite {
		cfg.MaxOpenConns = 100
		cfg.MaxIdleConns = 10
		cfg.ConnMaxLifetime = 1 * time.Hour

		if v := env["CF_DB_MAX_OPEN_CONNS"]; v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				cfg.MaxOpenConns = n
			}
		}
		if v := env["CF_DB_MAX_IDLE_CONNS"]; v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				cfg.MaxIdleConns = n
			}
		}
	}

	return cfg
}

// Init initializes the database connection based on Config.Type and Config.DSN.
func Init(cfg Config) error {
	var dialector gorm.Dialector

	switch cfg.Type {
	case DBTypeSQLite, "":
		// Register a connection hook so that EVERY new connection opened
		// by the pool automatically receives the correct PRAGMAs. Without
		// this, only the first connection gets WAL/busy_timeout set via
		// Exec below; subsequent connections opened by the pool may run
		// with default settings (DELETE journal mode, no busy_timeout),
		// leading to occasional SQLITE_BUSY errors under concurrent load.
		sqlite.RegisterConnectionHook(func(conn sqlite.ExecQuerierContext, dsn string) error {
			ctx := context.Background()
			// WAL mode: allows concurrent readers during writes.
			conn.ExecContext(ctx, "PRAGMA journal_mode=WAL", nil)
			// Busy timeout: wait up to 5s on lock contention before returning SQLITE_BUSY.
			conn.ExecContext(ctx, "PRAGMA busy_timeout=5000", nil)
			// Synchronous=NORMAL: safe with WAL, much faster than FULL.
			conn.ExecContext(ctx, "PRAGMA synchronous=NORMAL", nil)
			// Foreign keys: enforce referential integrity.
			conn.ExecContext(ctx, "PRAGMA foreign_keys=ON", nil)
			// Cache size: 64MB negative = kibibytes, improves read performance
			// for large databases (default is ~2MB).
			conn.ExecContext(ctx, "PRAGMA cache_size=-65536", nil)
			// Temp store: use memory for temp tables/indexes (avoids disk I/O).
			conn.ExecContext(ctx, "PRAGMA temp_store=MEMORY", nil)
			return nil
		})
		dialector = initSQLite(cfg.DSN)
	default:
		// Look up registered dialector (mysql, postgres, etc.)
		factory, ok := dialectorRegistry[cfg.Type]
		if !ok {
			available := []string{DBTypeSQLite}
			for k := range dialectorRegistry {
				available = append(available, k)
			}
			return fmt.Errorf("unsupported database type: %s (available: %v)", cfg.Type, available)
		}
		dialector = factory(cfg.DSN)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(cfg.LogLevel),
	})
	if err != nil {
		return fmt.Errorf("open %s database: %w", cfg.Type, err)
	}

	// Post-connect configuration
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get underlying db: %w", err)
	}

	switch cfg.Type {
	case DBTypeSQLite, "":
		// The connection hook already applies PRAGMAs to every new
		// connection. Run them once here as well for the initial
		// connection so that any error surfaces immediately at startup
		// (e.g. if WAL mode is not supported on a read-only filesystem).
		if _, err := sqlDB.Exec("PRAGMA journal_mode=WAL"); err != nil {
			return fmt.Errorf("enable WAL mode: %w", err)
		}
		if _, err := sqlDB.Exec("PRAGMA busy_timeout=5000"); err != nil {
			return fmt.Errorf("set busy timeout: %w", err)
		}
		if _, err := sqlDB.Exec("PRAGMA synchronous=NORMAL"); err != nil {
			return fmt.Errorf("set synchronous: %w", err)
		}
		if _, err := sqlDB.Exec("PRAGMA foreign_keys=ON"); err != nil {
			log.Printf("WARNING: enable foreign keys: %v", err)
		}
		// Limit maximum SQL length to prevent runaway queries from
		// consuming excessive memory (default is 1,000,000 bytes).
		// This is a safety net, not a tight limit.
		log.Printf("database initialized: %s (sqlite, WAL mode, connection hook active)", cfg.DSN)

	default:
		// MySQL / PostgreSQL: configure connection pool
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
		sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
		log.Printf("database initialized: %s (pool: %d/%d)", cfg.Type, cfg.MaxIdleConns, cfg.MaxOpenConns)
	}

	currentCfg = cfg
	DB = db
	return nil
}

// initSQLite creates a SQLite Dialector from a file path.
func initSQLite(path string) gorm.Dialector {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		log.Printf("WARNING: create db directory: %v", err)
	}
	return sqlite.Open(path)
}

// Close closes the database connection.
func Close() error {
	if DB == nil {
		return nil
	}
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// TestConnection attempts to open a connection with the given Config and verify it works.
// It opens a temporary connection, runs a simple query, then closes it.
// Does NOT affect the global DB instance.
func TestConnection(cfg Config) error {
	var dialector gorm.Dialector

	switch cfg.Type {
	case DBTypeSQLite, "":
		dialector = initSQLite(cfg.DSN)
	default:
		factory, ok := dialectorRegistry[cfg.Type]
		if !ok {
			return fmt.Errorf("unsupported database type: %s", cfg.Type)
		}
		dialector = factory(cfg.DSN)
	}

	testDB, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return fmt.Errorf("connect %s: %w", cfg.Type, err)
	}
	defer func() {
		if sqlDB, err := testDB.DB(); err == nil {
			sqlDB.Close()
		}
	}()

	// Verify connection is alive
	sqlDB, err := testDB.DB()
	if err != nil {
		return fmt.Errorf("get underlying db: %w", err)
	}
	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("ping %s: %w", cfg.Type, err)
	}

	return nil
}

// StartType returns the database type that was used at Init time.
var startType string
var startDSN string

// RecordStartup records the database config used at startup for change detection.
func RecordStartup(cfg Config) {
	startType = cfg.Type
	startDSN = cfg.DSN
}

// StartupConfig returns the database type and DSN used at startup.
func StartupConfig() (dbType, dsn string) {
	return startType, startDSN
}

// MigrateTo copies all data from the currently running database to the target database.
// The source is the global DB instance (currently connected).
// The target is opened temporarily from the given Config.
// Strategy:
//  1. Open target database connection
//  2. Run AutoMigrate on target to create all tables
//  3. Read each table from current DB → clear target table → batch insert into target
//
// This supports migration between any combination of SQLite/MySQL/PostgreSQL.
// Returns (tablesCopied, rowsCopied, error).
func MigrateTo(dstCfg Config) (tablesCopied int, rowsCopied int64, err error) {
	if DB == nil {
		return 0, 0, fmt.Errorf("source database not initialized")
	}

	// Open target database
	dstDB, cleanup, err := openDB(dstCfg)
	if err != nil {
		return 0, 0, fmt.Errorf("open target database: %w", err)
	}
	defer cleanup()

	// Step 1: Run AutoMigrate on target to create all tables
	log.Println("migrate: running AutoMigrate on target database...")
	if err := runAutoMigrate(dstDB); err != nil {
		return 0, 0, fmt.Errorf("auto-migrate target: %w", err)
	}
	log.Println("migrate: target database schema created")

	// ⚠ **写入预检**（"迁移必须能写进去"）：AutoMigrate 成功 ≠ 后续还能写 ✗ ——
	// 目标库可能是只读账号/只读挂载/磁盘满/处于 recovery，而旧实现遇到写失败只打 warning
	// 就继续 ✗ → 迁移"成功"返回却**丢数据** ✗（静默数据丢失）。这里先真写一次探针，
	// 写不进去就直接失败 ✓。
	if err := verifyTargetWritable(dstDB); err != nil {
		return 0, 0, fmt.Errorf("target database is not writable: %w", err)
	}
	log.Println("migrate: target database write check passed")

	// Step 2: Get list of tables from current (source) database
	srcType, _ := StartupConfig()
	tableNames, err := listTables(DB, srcType)
	if err != nil {
		return 0, 0, fmt.Errorf("list source tables: %w", err)
	}

	// Step 3: Copy data table by table
	for _, table := range tableNames {
		if strings.HasPrefix(table, "sqlite_") || strings.HasPrefix(table, "_cf_") {
			continue
		}

		// Read all rows from current (source) table
		var rows []map[string]interface{}
		if err := DB.Table(table).Find(&rows).Error; err != nil {
			// 读不出来**不能**当"这张表没数据"跳过 ✗ —— 那等于静默漏迁一整张表 ✗
			return 0, 0, fmt.Errorf("read source table %s: %w", table, err)
		}
		if len(rows) == 0 {
			log.Printf("migrate: skip table %s (empty)", table)
			continue
		}

		// Only keep columns that actually exist in the target table.
		// This makes migration robust against schema drift (e.g. source has a
		// column that was later removed from the entity/model).
		var dstCols []string
		if err := dstDB.Raw(
			"SELECT column_name FROM information_schema.columns WHERE table_name = ?", table,
		).Scan(&dstCols).Error; err != nil {
			log.Printf("migrate: warning: cannot read target columns for %s: %v", table, err)
		}
		colSet := make(map[string]struct{}, len(dstCols))
		for _, c := range dstCols {
			colSet[c] = struct{}{}
		}
		// Detect boolean columns in the target so we can coerce source values
		// (MySQL stores bool as tinyint, read back as int64) into real bool for
		// databases like PostgreSQL that require a native boolean type.
		boolCols := make(map[string]struct{})
		var boolColRows []string
		if err := dstDB.Raw(
			"SELECT column_name FROM information_schema.columns WHERE table_name = ? AND data_type IN ('boolean','bool')", table,
		).Scan(&boolColRows).Error; err == nil {
			for _, c := range boolColRows {
				boolCols[c] = struct{}{}
			}
		}
		if len(colSet) > 0 {
			for i := range rows {
				for k := range rows[i] {
					if _, ok := colSet[k]; !ok {
						delete(rows[i], k)
						continue
					}
					// Coerce int64 (0/1) from MySQL tinyint into native bool for
					// the target (PostgreSQL driver rejects int64 for bool columns).
					if _, isBool := boolCols[k]; isBool {
						switch v := rows[i][k].(type) {
						case bool:
							// already bool, keep
						case int64:
							rows[i][k] = v != 0
						case int:
							rows[i][k] = v != 0
						case int32:
							rows[i][k] = v != 0
						case int16:
							rows[i][k] = v != 0
						case int8:
							rows[i][k] = v != 0
						case uint64:
							rows[i][k] = v != 0
						case uint:
							rows[i][k] = v != 0
						case uint32:
							rows[i][k] = v != 0
						case uint16:
							rows[i][k] = v != 0
						case uint8:
							rows[i][k] = v != 0
						case float64:
							rows[i][k] = v != 0
						case float32:
							rows[i][k] = v != 0
						case string:
							// MySQL tinyint may be returned as "0"/"1" []byte or string
							rows[i][k] = v == "1" || v == "true" || v == "t"
						case []byte:
							rows[i][k] = string(v) == "1" || string(v) == "true" || string(v) == "t"
						default:
							// Fallback: treat zero/nil as false
							rows[i][k] = false
						}
					}
				}
				// gorm expects a gorm.DeletedAt key for soft-delete columns;
				// when absent it is fine to drop.
				delete(rows[i], "DeletedAt")
			}
		}

		// Clear target table data：清不掉就**必须失败** ✗（否则新旧数据混在一起 ✗）
		if err := dstDB.Exec(fmt.Sprintf("DELETE FROM %s", table)).Error; err != nil {
			return 0, 0, fmt.Errorf("clear target table %s: %w", table, err)
		}

		// Batch insert into target（批量失败退化为逐行；**任何一行写不进去即失败** ✗）
		batchSize := 500
		for i := 0; i < len(rows); i += batchSize {
			end := i + batchSize
			if end > len(rows) {
				end = len(rows)
			}
			if err := dstDB.Table(table).Create(rows[i:end]).Error; err != nil {
				log.Printf("migrate: batch insert into %s failed (%v), retrying row by row...", table, err)
				for idx, row := range rows[i:end] {
					if rerr := dstDB.Table(table).Create(row).Error; rerr != nil {
						return 0, 0, fmt.Errorf("insert into %s failed at row %d: %w", table, i+idx, rerr)
					}
				}
			}
		}

		// 拷后核对：目标行数必须等于源行数 ✓ —— 这是"确实写进去了"的最终判据。
		var targetCount int64
		if err := dstDB.Table(table).Count(&targetCount).Error; err != nil {
			return 0, 0, fmt.Errorf("count target table %s: %w", table, err)
		}
		if targetCount != int64(len(rows)) {
			return 0, 0, fmt.Errorf("table %s row mismatch after copy: source=%d target=%d",
				table, len(rows), targetCount)
		}

		tablesCopied++
		rowsCopied += int64(len(rows))
		log.Printf("migrate: copied table %s (%d rows, verified)", table, len(rows))
	}

	return tablesCopied, rowsCopied, nil
}

// migrateProbeTable 是写入预检用的探针表名。
// 用 `_cf_` 前缀是刻意的：拷贝循环本来就会跳过 `_cf_` 前缀的表 ✓，所以探针表不会被当成业务表拷走 ✗。
const migrateProbeTable = "_cf_migration_probe"

// verifyTargetWritable 真实写一次探针，确认目标库**确实可写**。
//
// 覆盖 AutoMigrate 之后的写入路径：建表 → 插行 → 读回校验 → 删表。
// 任一步失败即返回错误（调用方据此直接中止迁移 ✗）。列类型用 INTEGER/TEXT，三种方言都接受 ✓。
func verifyTargetWritable(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("target database handle is nil")
	}
	drop := func() { _ = db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", migrateProbeTable)).Error }
	drop()

	if err := db.Exec(fmt.Sprintf("CREATE TABLE %s (id INTEGER, note TEXT)", migrateProbeTable)).Error; err != nil {
		return fmt.Errorf("create probe table: %w", err)
	}
	defer drop()

	if err := db.Exec(fmt.Sprintf("INSERT INTO %s (id, note) VALUES (?, ?)", migrateProbeTable), 1, "probe").Error; err != nil {
		return fmt.Errorf("insert probe row: %w", err)
	}
	var got int64
	if err := db.Raw(fmt.Sprintf("SELECT COUNT(*) FROM %s", migrateProbeTable)).Scan(&got).Error; err != nil {
		return fmt.Errorf("read back probe table: %w", err)
	}
	if got != 1 {
		return fmt.Errorf("probe row not persisted (count=%d)", got)
	}
	return nil
}

// runAutoMigrate runs the same AutoMigrate logic as the main application on a given DB.
// This ensures the target database has all required tables before data is copied.
// AutoMigrateOnDB runs AutoMigrate on the given database connection.
func AutoMigrateOnDB(gormDB *gorm.DB) error {
	return runAutoMigrate(gormDB)
}

func runAutoMigrate(db *gorm.DB) error {
	// PostgreSQL has no "longtext" type. Register a DOMAIN alias longtext->text
	// so the cross-database entity definitions (type:longtext) work on PG too.
	if db.Dialector.Name() == "postgres" {
		ddl := `DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'longtext') THEN
				CREATE DOMAIN longtext AS text;
			END IF;
		END $$;`
		if err := db.Exec(ddl).Error; err != nil {
			return fmt.Errorf("create longtext domain: %w", err)
		}
	}
	// Import the entity package to get all models
	// We use the same model list as migration.AutoMigrate
	models := allEntityModels()
	for _, model := range models {
		if err := db.AutoMigrate(model); err != nil {
			return fmt.Errorf("auto-migrate %T: %w", model, err)
		}
	}
	// Run post-AutoMigrate data migrations/seeds (e.g. charge_cards
	// plaintext→encrypted column drop) BEFORE copying data, so the target
	// schema matches the source. Safe to run on any dialect.
	if err := migration.RunDataMigrations(db); err != nil {
		return fmt.Errorf("post auto-migrate data migrations: %w", err)
	}
	return nil
}

// allEntityModels returns all entity models for AutoMigrate.
// This is the same list used in migration.AutoMigrate.
func allEntityModels() []interface{} {
	return []interface{}{
		&entity.User{},
		&entity.Node{},
		&entity.CoreConfig{},
		&entity.InboundConfig{},
		&entity.OutboundConfig{},
		&entity.InboundService{},
		&entity.ServiceEndpoint{},
		&entity.NodeTrafficUsage{},
		&entity.ServiceAccessPolicy{},
		&entity.ServiceIPBan{},
		&entity.ServiceGeoRule{},
		&entity.Subscription{},
		&entity.SubscriptionAccessLog{},
		&entity.SubscriptionRotation{},
		&entity.Certificate{},
		&entity.ACMEAccount{},
		&entity.CertificateJob{},
		&entity.TrafficQuota{},
		&entity.TrafficUsageLedger{},
		&entity.TrafficUsageSummary{},
		&entity.TrafficSession{},
		&entity.TrafficQuotaResetLog{},
		&entity.SecurityAuditEvent{},
		&entity.SecurityAlert{},
		&entity.AlertChannel{},
		&entity.RuntimeSnapshot{},
		&entity.RuntimeApplyJob{},
		&entity.RuntimeApplyJobTarget{},
		&entity.RuntimeApplyJobEvent{},
		&entity.ReverseProxyInstance{},
		&entity.BackupRetentionPolicy{},
		&entity.BackupCleanupJob{},
		&entity.BinaryRelease{},
		&entity.UpdateJob{},
		&entity.RuleSource{},
		&entity.GeoRuleFile{},
		&entity.WarpConfig{},
		&entity.ProcessGuardian{},
		&entity.CoreCrashLog{},
		&entity.ProxySite{},
		&entity.CaddySite{},
		&entity.NginxSite{},
		&entity.DNSChannel{},
		&entity.EABConfig{},
		&entity.ACMEAccountConfig{},
		&entity.CARoot{},
		&entity.LogRotationPolicy{},
		&entity.LogRotationJob{},
		&entity.SystemSetting{},
		&entity.NotificationTemplate{},
		&entity.BackupRecord{},
		&entity.BackupSchedule{},
		&entity.BackupStorage{},
		&entity.ShareDomainConfig{},
		&entity.CFIPSource{},
		&entity.CFIPEntry{},
		&entity.SubscriptionFormat{},
		&entity.SubscriptionRule{},
		&entity.SubscriptionEndpoint{},
		&entity.Plan{},
		&entity.PlanNode{},
		&entity.Client{},
		&entity.PlanTemplate{},
		&entity.PlanVariant{},
		&entity.NodeGroup{},
		&entity.NodeGroupMember{},
		&entity.Promotion{},
		&entity.PlanOrder{},
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
		&entity.ExternalNodeSource{},
		&entity.ExternalNode{},
		&entity.ChainLink{},
		&entity.ExternalNodeServer{},
		&entity.NodeServer{},
		&entity.NodeCertificate{}, // 节点 ↔ 证书 多对多 ✓（一个节点可绑多张 ✓）
		&entity.AgentCertRevocation{},
		&entity.ServerMetric{},
		&entity.SpeedTestRecord{},
		&entity.NodeAuditLog{},
		&entity.ServerTag{},
		&entity.CertDistributionQueueItem{},
		&entity.ClientLogPackage{},
		&entity.PortalTemplate{},
		&entity.ChargeSite{},
		&entity.ChargeOrder{},
		&entity.ChargeCard{},
		&entity.ClientBalance{},
		&entity.BalanceTransaction{},
		&entity.CallbackNonce{},
		&entity.ChargeCallbackRetry{},
		&entity.ChargeCallbackEvent{},
		&entity.ChargeSiteSecret{},
		&entity.ChargeSecurityEvent{},
		&entity.ReferralCode{},
		&entity.ReferralRecord{},
		&entity.ReferralWithdrawal{},
		&entity.Ticket{},
		&entity.TicketMessage{},
		&entity.ClientGift{},
		&entity.Article{},
		&entity.SharedAccount{},
		&entity.SharedAccountRepairLog{},
		&entity.ClientOAuthBinding{},
		&entity.ClientSelfUnbanLog{},
		&entity.IPWhitelistEntry{},
	}
}

// MigrateFromSQLite opens a SQLite database file and migrates all its data
// into the target database specified by dstCfg. This is used for cross-database
// restore: when a backup created on SQLite needs to be restored into a MySQL or
// PostgreSQL instance. Unlike MigrateTo, this does not use the global DB.
func MigrateFromSQLite(sqlitePath string, dstCfg Config) (tablesCopied int, rowsCopied int64, err error) {
	// Open source SQLite database
	srcCfg := Config{
		Type: DBTypeSQLite,
		DSN:  sqlitePath,
	}
	srcDB, srcCleanup, err := openDB(srcCfg)
	if err != nil {
		return 0, 0, fmt.Errorf("open source sqlite: %w", err)
	}
	defer srcCleanup()

	// Open target database
	dstDB, dstCleanup, err := openDB(dstCfg)
	if err != nil {
		return 0, 0, fmt.Errorf("open target database: %w", err)
	}
	defer dstCleanup()

	// Step 1: Run AutoMigrate on target to create all tables
	log.Println("migrate: running AutoMigrate on target database...")
	if err := runAutoMigrate(dstDB); err != nil {
		return 0, 0, fmt.Errorf("auto-migrate target: %w", err)
	}
	log.Println("migrate: target database schema created")

	// ⚠ **写入预检**（"迁移必须能写进去"）：AutoMigrate 成功 ≠ 后续还能写 ✗ ——
	// 目标库可能是只读账号/只读挂载/磁盘满/处于 recovery，而旧实现遇到写失败只打 warning
	// 就继续 ✗ → 迁移"成功"返回却**丢数据** ✗（静默数据丢失）。这里先真写一次探针，
	// 写不进去就直接失败 ✓。
	if err := verifyTargetWritable(dstDB); err != nil {
		return 0, 0, fmt.Errorf("target database is not writable: %w", err)
	}
	log.Println("migrate: target database write check passed")

	// Step 2: Get list of tables from source SQLite database
	tableNames, err := listTables(srcDB, DBTypeSQLite)
	if err != nil {
		return 0, 0, fmt.Errorf("list source tables: %w", err)
	}

	// Step 3: Copy data table by table
	for _, table := range tableNames {
		if strings.HasPrefix(table, "sqlite_") || strings.HasPrefix(table, "_cf_") {
			continue
		}

		var rows []map[string]interface{}
		if err := srcDB.Table(table).Find(&rows).Error; err != nil {
			// 同 MigrateTo：读不出来就失败，不能当"空表"跳过 ✗
			return 0, 0, fmt.Errorf("read source table %s: %w", table, err)
		}
		if len(rows) == 0 {
			log.Printf("migrate: skip table %s (empty)", table)
			continue
		}

		var dstCols []string
		if err := dstDB.Raw(
			"SELECT column_name FROM information_schema.columns WHERE table_name = ?", table,
		).Scan(&dstCols).Error; err != nil {
			log.Printf("migrate: warning: cannot read target columns for %s: %v", table, err)
		}
		colSet := make(map[string]struct{}, len(dstCols))
		for _, c := range dstCols {
			colSet[c] = struct{}{}
		}
		boolCols := make(map[string]struct{})
		var boolColRows []string
		if err := dstDB.Raw(
			"SELECT column_name FROM information_schema.columns WHERE table_name = ? AND data_type IN ('boolean','bool')", table,
		).Scan(&boolColRows).Error; err == nil {
			for _, c := range boolColRows {
				boolCols[c] = struct{}{}
			}
		}
		if len(colSet) > 0 {
			for i := range rows {
				filtered := make(map[string]interface{}, len(rows[i]))
				for k, v := range rows[i] {
					if _, ok := colSet[k]; ok {
						if _, isBool := boolCols[k]; isBool {
							switch tv := v.(type) {
							case int64:
								filtered[k] = tv != 0
							case int:
								filtered[k] = tv != 0
							case bool:
								filtered[k] = tv
							default:
								filtered[k] = v
							}
						} else {
							filtered[k] = v
						}
					}
				}
				rows[i] = filtered
			}
		}

		// Clear target table：清不掉就**必须失败** ✗（旧实现是 continue → 静默漏一整张表 ✗）
		if err := dstDB.Exec("TRUNCATE TABLE " + table).Error; err != nil {
			// TRUNCATE may fail (e.g. foreign key constraints), try DELETE
			if err := dstDB.Exec("DELETE FROM " + table).Error; err != nil {
				return 0, 0, fmt.Errorf("clear target table %s: %w", table, err)
			}
		}

		// Batch insert：**任何一批写不进去即失败** ✗（旧实现只记 log 继续 → 数据缺口 ✗）
		batchSize := 500
		for i := 0; i < len(rows); i += batchSize {
			end := i + batchSize
			if end > len(rows) {
				end = len(rows)
			}
			if err := dstDB.Table(table).Create(rows[i:end]).Error; err != nil {
				return 0, 0, fmt.Errorf("insert into %s failed at offset %d: %w", table, i, err)
			}
		}

		// 拷后核对：目标行数必须等于源行数 ✓
		var targetCount int64
		if err := dstDB.Table(table).Count(&targetCount).Error; err != nil {
			return 0, 0, fmt.Errorf("count target table %s: %w", table, err)
		}
		if targetCount != int64(len(rows)) {
			return 0, 0, fmt.Errorf("table %s row mismatch after copy: source=%d target=%d",
				table, len(rows), targetCount)
		}

		tablesCopied++
		rowsCopied += int64(len(rows))
		log.Printf("migrate: copied %d rows from %s (verified)", len(rows), table)
	}

	return tablesCopied, rowsCopied, nil
}

// OpenDBPublic opens a temporary database connection from a Config.
// Returns the gorm.DB and a cleanup function.
func OpenDBPublic(cfg Config) (*gorm.DB, func(), error) {
	return openDB(cfg)
}

// openDB opens a temporary database connection from a Config.
// Returns the gorm.DB and a cleanup function.
func openDB(cfg Config) (*gorm.DB, func(), error) {
	var dialector gorm.Dialector

	switch cfg.Type {
	case DBTypeSQLite, "":
		dialector = initSQLite(cfg.DSN)
	default:
		factory, ok := dialectorRegistry[cfg.Type]
		if !ok {
			return nil, nil, fmt.Errorf("unsupported database type: %s", cfg.Type)
		}
		dialector = factory(cfg.DSN)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("open %s: %w", cfg.Type, err)
	}

	cleanup := func() {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	}

	return db, cleanup, nil
}

// listTables returns table names for the given database connection.
func listTables(db *gorm.DB, dbType string) ([]string, error) {
	var tableNames []string

	switch dbType {
	case DBTypeSQLite, "":
		if err := db.Raw("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name").Scan(&tableNames).Error; err != nil {
			return nil, err
		}
	case DBTypeMySQL:
		if err := db.Raw("SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE' ORDER BY table_name").Scan(&tableNames).Error; err != nil {
			return nil, err
		}
	case DBTypePostgres:
		if err := db.Raw("SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename").Scan(&tableNames).Error; err != nil {
			return nil, err
		}
	default:
		// Fallback: try GORM's Migrator
		if err := db.Raw("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name").Scan(&tableNames).Error; err != nil {
			return nil, fmt.Errorf("unsupported database type for table listing: %s", dbType)
		}
	}

	return tableNames, nil
}

// ParseDSN parses a DSN string (possibly with scheme prefix) into a Config.
// Used by the settings service to test/migrate without going through .env.
func ParseDSN(rawDSN string) Config {
	cfg := DefaultConfig()

	switch {
	case strings.HasPrefix(rawDSN, "mysql://"):
		cfg.Type = DBTypeMySQL
		cfg.DSN = strings.TrimPrefix(rawDSN, "mysql://")
	case strings.HasPrefix(rawDSN, "postgres://"), strings.HasPrefix(rawDSN, "postgresql://"):
		cfg.Type = DBTypePostgres
		cfg.DSN = rawDSN // pgx driver uses full URI
	case strings.HasPrefix(rawDSN, "sqlite://"):
		cfg.Type = DBTypeSQLite
		cfg.DSN = strings.TrimPrefix(rawDSN, "sqlite://")
	default:
		cfg.Type = DBTypeSQLite
		cfg.DSN = rawDSN
	}

	return cfg
}
