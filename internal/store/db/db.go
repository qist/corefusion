package db

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"corefusion/internal/model/entity"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// compile-time check
var _ = strconv.IntSize

// DB is the global database instance.
var DB *gorm.DB

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
		// SQLite: enable WAL mode for better concurrency
		if _, err := sqlDB.Exec("PRAGMA journal_mode=WAL"); err != nil {
			return fmt.Errorf("enable WAL mode: %w", err)
		}
		if _, err := sqlDB.Exec("PRAGMA busy_timeout=5000"); err != nil {
			return fmt.Errorf("set busy timeout: %w", err)
		}
		if _, err := sqlDB.Exec("PRAGMA synchronous=NORMAL"); err != nil {
			return fmt.Errorf("set synchronous: %w", err)
		}
		log.Printf("database initialized: %s (sqlite, WAL mode)", cfg.DSN)

	default:
		// MySQL / PostgreSQL: configure connection pool
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
		sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
		log.Printf("database initialized: %s (pool: %d/%d)", cfg.Type, cfg.MaxIdleConns, cfg.MaxOpenConns)
	}

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
			log.Printf("migrate: skip table %s (read error): %v", table, err)
			continue
		}
		if len(rows) == 0 {
			log.Printf("migrate: skip table %s (empty)", table)
			continue
		}

		// Clear target table data
		if err := dstDB.Exec(fmt.Sprintf("DELETE FROM %s", table)).Error; err != nil {
			log.Printf("migrate: warning: could not clear target table %s: %v", table, err)
		}

		// Batch insert into target
		batchSize := 500
		for i := 0; i < len(rows); i += batchSize {
			end := i + batchSize
			if end > len(rows) {
				end = len(rows)
			}
			if err := dstDB.Table(table).Create(rows[i:end]).Error; err != nil {
				log.Printf("migrate: warning: batch insert into %s failed: %v", table, err)
				// Try row-by-row as fallback
				for _, row := range rows[i:end] {
					if err := dstDB.Table(table).Create(row).Error; err != nil {
						log.Printf("migrate: warning: row insert into %s failed: %v", table, err)
					}
				}
			}
		}

		tablesCopied++
		rowsCopied += int64(len(rows))
		log.Printf("migrate: copied table %s (%d rows)", table, len(rows))
	}

	return tablesCopied, rowsCopied, nil
}

// runAutoMigrate runs the same AutoMigrate logic as the main application on a given DB.
// This ensures the target database has all required tables before data is copied.
func runAutoMigrate(db *gorm.DB) error {
	// Import the entity package to get all models
	// We use the same model list as migration.AutoMigrate
	models := allEntityModels()
	for _, model := range models {
		if err := db.AutoMigrate(model); err != nil {
			return fmt.Errorf("auto-migrate %T: %w", model, err)
		}
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
		&entity.ServiceAccessPolicy{},
		&entity.ServiceIPBan{},
		&entity.ServiceGeoRule{},
		&entity.Subscription{},
		&entity.SubscriptionAccessLog{},
		&entity.SubscriptionRotation{},
		&entity.ServerShareProfile{},
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
		&entity.RoutingRegion{},
		&entity.RuleSource{},
		&entity.GeoRuleFile{},
		&entity.WarpConfig{},
		&entity.ProcessGuardian{},
		&entity.CoreCrashLog{},
		&entity.ProxySite{},
		&entity.CaddySite{},
		&entity.NginxSite{},
		&entity.DNSChannel{},
		&entity.ACMEAccountConfig{},
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
		&entity.NodeServer{},
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
