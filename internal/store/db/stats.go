package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/qist/sqlite"
)

// ---------------------------------------------------------------------------
// Unified types
// ---------------------------------------------------------------------------

// DBStatsResponse is the unified response returned by GetDBStats regardless
// of the underlying database engine. Only the field matching the current
// engine is populated; the rest remain nil.
type DBStatsResponse struct {
	Engine   string       `json:"engine"` // "sqlite" | "mysql" | "postgres"
	SQLite   *SQLiteStats `json:"sqlite,omitempty"`
	MySQL    *MySQLStats  `json:"mysql,omitempty"`
	Postgres *PGStats     `json:"postgres,omitempty"`
	Info     *DBInfo      `json:"info"` // basic info common to all engines
}

// DBInfo holds basic configuration/metadata about the database, common
// across all engines. Engine-specific details are in the respective
// *Stats structs.
type DBInfo struct {
	Engine           string `json:"engine"`            // "sqlite" | "mysql" | "postgres"`
	Version          string `json:"version"`           // engine version string
	DatabaseSize     int64  `json:"database_size"`     // database size in bytes (best-effort)
	ConnectionString string `json:"connection_string"` // sanitized DSN (password masked)
}

// ---------------------------------------------------------------------------
// SQLite stats (via sqlite3_db_status)
// ---------------------------------------------------------------------------

// SQLiteStats holds runtime statistics for a SQLite database connection.
// All values are captured atomically from a single connection via the
// sqlite3_db_status C API.
type SQLiteStats struct {
	// Cache statistics (page cache)
	CacheUsedBytes  int     `json:"cache_used_bytes"`  // Memory used by the page cache
	CacheHitCount   int     `json:"cache_hit_count"`   // Page cache hits (cumulative)
	CacheMissCount  int     `json:"cache_miss_count"`  // Page cache misses (cumulative)
	CacheWriteCount int     `json:"cache_write_count"` // Page cache writes (cumulative)
	CacheSpillCount int     `json:"cache_spill_count"` // Cache spills to disk (cumulative)
	CacheHitRatio   float64 `json:"cache_hit_ratio"`   // hit / (hit + miss), 0-1

	// Memory statistics
	SchemaUsedBytes int `json:"schema_used_bytes"` // Memory used by the schema (tables, indexes, etc.)
	StmtUsedBytes   int `json:"stmt_used_bytes"`   // Memory used by prepared statements

	// Lookaside allocator
	LookasideUsedBytes int `json:"lookaside_used_bytes"` // Lookaside memory currently in use
	LookasideHitCount  int `json:"lookaside_hit_count"`  // Lookaside allocations served from the fast pool
	LookasideMissCount int `json:"lookaside_miss_count"` // Lookaside allocations that missed (fell through to main allocator)

	// Transaction state
	DeferredFKs int `json:"deferred_fks"` // 1 if unresolved deferred FK constraints exist

	// Temp buffer
	TempbufSpillCount int `json:"tempbuf_spill_count"` // Temp buffer spills to disk (cumulative)

	// PRAGMA-derived info
	JournalMode string `json:"journal_mode"` // WAL / DELETE / TRUNCATE
	Synchronous string `json:"synchronous"`  // OFF / NORMAL / FULL
	ForeignKeys string `json:"foreign_keys"` // ON / OFF
	CacheSize   int    `json:"cache_size"`   // Number of pages (negative = kibibytes)
	PageSize    int    `json:"page_size"`    // Page size in bytes
	PageCount   int    `json:"page_count"`   // Total pages in the database file
	TempStore   string `json:"temp_store"`   // DEFAULT / FILE / MEMORY
	BusyTimeout int    `json:"busy_timeout"` // Busy timeout in milliseconds
}

// GetSQLiteStats returns runtime statistics for the SQLite database.
// Only valid when the database type is SQLite; returns an error otherwise.
func GetSQLiteStats() (*SQLiteStats, error) {
	if DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	dbType, _ := StartupConfig()
	if dbType != DBTypeSQLite && dbType != "" {
		return nil, fmt.Errorf("database stats only available for SQLite (current: %s)", dbType)
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql.DB: %w", err)
	}

	ctx := context.Background()
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("get connection: %w", err)
	}
	defer conn.Close()

	stats := &SQLiteStats{}

	err = conn.Raw(func(dc any) error {
		status, ok := dc.(sqlite.DBStatus)
		if !ok {
			return fmt.Errorf("driver does not support DBStatus")
		}

		var err error

		stats.CacheUsedBytes, _, err = status.Status(sqlite.DBStatusCacheUsed, false)
		if err != nil {
			return fmt.Errorf("cache_used: %w", err)
		}
		stats.CacheHitCount, _, err = status.Status(sqlite.DBStatusCacheHit, false)
		if err != nil {
			return fmt.Errorf("cache_hit: %w", err)
		}
		stats.CacheMissCount, _, err = status.Status(sqlite.DBStatusCacheMiss, false)
		if err != nil {
			return fmt.Errorf("cache_miss: %w", err)
		}
		stats.CacheWriteCount, _, err = status.Status(sqlite.DBStatusCacheWrite, false)
		if err != nil {
			return fmt.Errorf("cache_write: %w", err)
		}
		stats.CacheSpillCount, _, err = status.Status(sqlite.DBStatusCacheSpill, false)
		if err != nil {
			return fmt.Errorf("cache_spill: %w", err)
		}
		stats.SchemaUsedBytes, _, err = status.Status(sqlite.DBStatusSchemaUsed, false)
		if err != nil {
			return fmt.Errorf("schema_used: %w", err)
		}
		stats.StmtUsedBytes, _, err = status.Status(sqlite.DBStatusStmtUsed, false)
		if err != nil {
			return fmt.Errorf("stmt_used: %w", err)
		}
		stats.LookasideUsedBytes, _, err = status.Status(sqlite.DBStatusLookasideUsed, false)
		if err != nil {
			return fmt.Errorf("lookaside_used: %w", err)
		}
		_, stats.LookasideHitCount, err = status.Status(sqlite.DBStatusLookasideHit, false)
		if err != nil {
			return fmt.Errorf("lookaside_hit: %w", err)
		}
		_, stats.LookasideMissCount, err = status.Status(sqlite.DBStatusLookasideMissSize, false)
		if err != nil {
			return fmt.Errorf("lookaside_miss: %w", err)
		}
		stats.DeferredFKs, _, err = status.Status(sqlite.DBStatusDeferredFKs, false)
		if err != nil {
			return fmt.Errorf("deferred_fks: %w", err)
		}
		stats.TempbufSpillCount, _, err = status.Status(sqlite.DBStatusTempbufSpill, false)
		if err != nil {
			return fmt.Errorf("tempbuf_spill: %w", err)
		}

		total := stats.CacheHitCount + stats.CacheMissCount
		if total > 0 {
			stats.CacheHitRatio = float64(stats.CacheHitCount) / float64(total)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// PRAGMA values (via regular queries, not the status API)
	queryStr := func(pragma string) string {
		var val string
		_ = sqlDB.QueryRow("PRAGMA " + pragma).Scan(&val)
		return val
	}
	queryInt := func(pragma string) int {
		var val int
		_ = sqlDB.QueryRow("PRAGMA " + pragma).Scan(&val)
		return val
	}

	// Map numeric PRAGMA values to human-readable strings
	mapSync := func(v string) string {
		switch v {
		case "0":
			return "OFF"
		case "1":
			return "NORMAL"
		case "2":
			return "FULL"
		case "3":
			return "EXTRA"
		default:
			return v
		}
	}
	mapOnOff := func(v string) string {
		switch v {
		case "0":
			return "OFF"
		case "1":
			return "ON"
		default:
			return v
		}
	}
	mapTempStore := func(v string) string {
		switch v {
		case "0":
			return "DEFAULT"
		case "1":
			return "FILE"
		case "2":
			return "MEMORY"
		default:
			return v
		}
	}

	stats.JournalMode = queryStr("journal_mode")
	stats.Synchronous = mapSync(queryStr("synchronous"))
	stats.ForeignKeys = mapOnOff(queryStr("foreign_keys"))
	stats.CacheSize = queryInt("cache_size")
	stats.PageSize = queryInt("page_size")
	stats.PageCount = queryInt("page_count")
	stats.TempStore = mapTempStore(queryStr("temp_store"))
	stats.BusyTimeout = queryInt("busy_timeout")

	return stats, nil
}

// ---------------------------------------------------------------------------
// MySQL stats (via SHOW GLOBAL STATUS + performance_schema)
// ---------------------------------------------------------------------------

// MySQLStats holds runtime statistics for a MySQL database connection,
// gathered from SHOW GLOBAL STATUS and the performance_schema.
type MySQLStats struct {
	// Connection pool (from sql.DB.Stats())
	MaxOpenConnections int `json:"max_open_connections"` // Maximum configured open connections
	OpenConnections    int `json:"open_connections"`     // Currently open connections
	InUse              int `json:"in_use"`               // Connections currently in use
	Idle               int `json:"idle"`                 // Idle connections

	// MySQL global status variables
	ThreadsConnected   int   `json:"threads_connected"`    // Threads currently connected
	ThreadsRunning     int   `json:"threads_running"`      // Threads currently running
	ThreadsCreated     int64 `json:"threads_created"`      // Threads created since startup
	ThreadsCached      int   `json:"threads_cached"`       // Threads in the thread cache
	AbortedConnects    int64 `json:"aborted_connects"`     // Failed connection attempts
	Connections        int64 `json:"connections"`          // Total connection attempts (cumulative)
	MaxUsedConnections int   `json:"max_used_connections"` // Maximum simultaneously used connections

	// Query statistics
	Queries     int64 `json:"queries"`      // Total queries (cumulative)
	Questions   int64 `json:"questions"`    // Total questions asked by clients (cumulative)
	SlowQueries int64 `json:"slow_queries"` // Queries longer than long_query_time
	ComSelect   int64 `json:"com_select"`   // SELECT statements
	ComInsert   int64 `json:"com_insert"`   // INSERT statements
	ComUpdate   int64 `json:"com_update"`   // UPDATE statements
	ComDelete   int64 `json:"com_delete"`   // DELETE statements
	ComCommit   int64 `json:"com_commit"`   // COMMIT statements
	ComRollback int64 `json:"com_rollback"` // ROLLBACK statements

	// Buffer pool (InnoDB)
	BufferPoolSize     int64   `json:"buffer_pool_size"`          // Total buffer pool size in bytes
	BufferPoolUsed     int64   `json:"buffer_pool_used"`          // Used buffer pool in bytes
	BufferPoolFree     int64   `json:"buffer_pool_free"`          // Free buffer pool in bytes
	BufferPoolHitRate  float64 `json:"buffer_pool_hit_rate"`      // Hit ratio (0-1)
	BufferPoolReadReqs int64   `json:"buffer_pool_read_requests"` // Read requests (cumulative)
	BufferPoolReads    int64   `json:"buffer_pool_reads"`         // Physical reads (cumulative)

	// Temporary objects
	CreatedTmpTables     int64 `json:"created_tmp_tables"`      // In-memory temp tables created
	CreatedTmpDiskTables int64 `json:"created_tmp_disk_tables"` // On-disk temp tables created

	// Key cache (MyISAM, reported for completeness)
	KeyBlocksUsed    int64 `json:"key_blocks_used"`
	KeyBlocksUnused  int64 `json:"key_blocks_unused"`
	KeyReadRequests  int64 `json:"key_read_requests"`
	KeyReads         int64 `json:"key_reads"`
	KeyWrites        int64 `json:"key_writes"`
	KeyWriteRequests int64 `json:"key_write_requests"`

	// Bytes in / out
	BytesReceived int64 `json:"bytes_received"` // Bytes received from clients (cumulative)
	BytesSent     int64 `json:"bytes_sent"`     // Bytes sent to clients (cumulative)

	// Uptime
	Uptime int64 `json:"uptime"` // Seconds since server started
}

// GetMySQLStats returns runtime statistics for a MySQL database.
func GetMySQLStats() (*MySQLStats, error) {
	if DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql.DB: %w", err)
	}

	stats := &MySQLStats{}

	// Connection pool stats (from Go's database/sql)
	sqlStats := sqlDB.Stats()
	stats.MaxOpenConnections = sqlStats.MaxOpenConnections
	stats.OpenConnections = sqlStats.OpenConnections
	stats.InUse = sqlStats.InUse
	stats.Idle = sqlStats.Idle

	// Query all SHOW GLOBAL STATUS variables in one round-trip
	rows, err := sqlDB.Query(`
		SELECT variable_name, variable_value
		FROM performance_schema.global_status
		WHERE variable_name IN (
			'Threads_connected', 'Threads_running', 'Threads_created', 'Threads_cached',
			'Aborted_connects', 'Connections', 'Max_used_connections',
			'Queries', 'Questions', 'Slow_queries',
			'Com_select', 'Com_insert', 'Com_update', 'Com_delete',
			'Com_commit', 'Com_rollback',
			'Innodb_buffer_pool_pages_total', 'Innodb_buffer_pool_pages_data',
			'Innodb_buffer_pool_pages_free', 'Innodb_buffer_pool_pages_flushed',
			'Innodb_buffer_pool_read_requests', 'Innodb_buffer_pool_reads',
			'Created_tmp_tables', 'Created_tmp_disk_tables',
			'Key_blocks_used', 'Key_blocks_unused',
			'Key_read_requests', 'Key_reads', 'Key_writes', 'Key_write_requests',
			'Bytes_received', 'Bytes_sent', 'Uptime'
		)
	`)
	if err != nil {
		// Fallback: try SHOW GLOBAL STATUS (older MySQL / MariaDB)
		return getMySQLStatsLegacy(sqlDB, stats)
	}
	defer rows.Close()

	// Collect into a map for easy lookup
	statusMap := make(map[string]string, 32)
	for rows.Next() {
		var name, val string
		if err := rows.Scan(&name, &val); err != nil {
			continue
		}
		statusMap[name] = val
	}

	// Also query the InnoDB buffer pool page size
	var pageSize int64
	_ = sqlDB.QueryRow("SELECT variable_value FROM performance_schema.global_status WHERE variable_name = 'Innodb_page_size'").Scan(&pageSize)
	if pageSize == 0 {
		pageSize = 16384 // default InnoDB page size
	}

	populateMySQLStats(stats, statusMap, pageSize)
	return stats, nil
}

// getMySQLStatsLegacy is a fallback for MySQL/MariaDB versions that don't
// have performance_schema.global_status. It uses SHOW GLOBAL STATUS.
func getMySQLStatsLegacy(sqlDB *sql.DB, stats *MySQLStats) (*MySQLStats, error) {
	rows, err := sqlDB.Query("SHOW GLOBAL STATUS")
	if err != nil {
		return nil, fmt.Errorf("query global status: %w", err)
	}
	defer rows.Close()

	statusMap := make(map[string]string, 64)
	for rows.Next() {
		var name, val string
		if err := rows.Scan(&name, &val); err != nil {
			continue
		}
		statusMap[name] = val
	}

	var pageSize int64
	if v, ok := statusMap["Innodb_page_size"]; ok {
		fmt.Sscanf(v, "%d", &pageSize)
	}
	if pageSize == 0 {
		pageSize = 16384
	}

	populateMySQLStats(stats, statusMap, pageSize)
	return stats, nil
}

// populateMySQLStats fills a MySQLStats struct from a status variable map.
func populateMySQLStats(stats *MySQLStats, m map[string]string, pageSize int64) {
	parseInt64 := func(key string) int64 {
		if v, ok := m[key]; ok {
			var n int64
			fmt.Sscanf(v, "%d", &n)
			return n
		}
		return 0
	}
	parseInt := func(key string) int {
		return int(parseInt64(key))
	}

	stats.ThreadsConnected = parseInt("Threads_connected")
	stats.ThreadsRunning = parseInt("Threads_running")
	stats.ThreadsCreated = parseInt64("Threads_created")
	stats.ThreadsCached = parseInt("Threads_cached")
	stats.AbortedConnects = parseInt64("Aborted_connects")
	stats.Connections = parseInt64("Connections")
	stats.MaxUsedConnections = parseInt("Max_used_connections")

	stats.Queries = parseInt64("Queries")
	stats.Questions = parseInt64("Questions")
	stats.SlowQueries = parseInt64("Slow_queries")
	stats.ComSelect = parseInt64("Com_select")
	stats.ComInsert = parseInt64("Com_insert")
	stats.ComUpdate = parseInt64("Com_update")
	stats.ComDelete = parseInt64("Com_delete")
	stats.ComCommit = parseInt64("Com_commit")
	stats.ComRollback = parseInt64("Com_rollback")

	// InnoDB buffer pool
	pagesTotal := parseInt64("Innodb_buffer_pool_pages_total")
	pagesData := parseInt64("Innodb_buffer_pool_pages_data")
	pagesFree := parseInt64("Innodb_buffer_pool_pages_free")
	readReqs := parseInt64("Innodb_buffer_pool_read_requests")
	reads := parseInt64("Innodb_buffer_pool_reads")

	stats.BufferPoolSize = pagesTotal * pageSize
	stats.BufferPoolUsed = pagesData * pageSize
	stats.BufferPoolFree = pagesFree * pageSize
	stats.BufferPoolReadReqs = readReqs
	stats.BufferPoolReads = reads
	if readReqs > 0 {
		stats.BufferPoolHitRate = float64(readReqs-reads) / float64(readReqs)
	}

	stats.CreatedTmpTables = parseInt64("Created_tmp_tables")
	stats.CreatedTmpDiskTables = parseInt64("Created_tmp_disk_tables")

	stats.KeyBlocksUsed = parseInt64("Key_blocks_used")
	stats.KeyBlocksUnused = parseInt64("Key_blocks_unused")
	stats.KeyReadRequests = parseInt64("Key_read_requests")
	stats.KeyReads = parseInt64("Key_reads")
	stats.KeyWrites = parseInt64("Key_writes")
	stats.KeyWriteRequests = parseInt64("Key_write_requests")

	stats.BytesReceived = parseInt64("Bytes_received")
	stats.BytesSent = parseInt64("Bytes_sent")
	stats.Uptime = parseInt64("Uptime")
}

// ---------------------------------------------------------------------------
// PostgreSQL stats (via pg_stat_database + pg_stat_activity)
// ---------------------------------------------------------------------------

// PGStats holds runtime statistics for a PostgreSQL database connection,
// gathered from pg_stat_database, pg_stat_activity, and pg_database_size.
type PGStats struct {
	// Connection pool (from Go's database/sql)
	MaxOpenConnections int `json:"max_open_connections"`
	OpenConnections    int `json:"open_connections"`
	InUse              int `json:"in_use"`
	Idle               int `json:"idle"`

	// Server-level connection stats
	TotalConnections  int   `json:"total_connections"`  // Total active connections to the server
	ActiveConnections int   `json:"active_connections"` // Connections in active state
	IdleConnections   int   `json:"idle_connections"`   // Connections in idle state
	IdleInTxn         int   `json:"idle_in_txn"`        // Connections idle in transaction
	MaxConnections    int64 `json:"max_connections"`    // Server max_connections setting

	// Database-level stats (from pg_stat_database)
	NumCommit      int64   `json:"num_commit"`      // Committed transactions
	NumRollback    int64   `json:"num_rollback"`    // Rolled back transactions
	BlksRead       int64   `json:"blks_read"`       // Disk blocks read
	BlksHit        int64   `json:"blks_hit"`        // Buffer hits
	BlksHitRatio   float64 `json:"blks_hit_ratio"`  // Hit ratio (0-1)
	TuplesReturned int64   `json:"tuples_returned"` // Tuples returned by queries
	TuplesFetched  int64   `json:"tuples_fetched"`  // Tuples fetched by queries
	TuplesInserted int64   `json:"tuples_inserted"` // Tuples inserted
	TuplesUpdated  int64   `json:"tuples_updated"`  // Tuples updated
	TuplesDeleted  int64   `json:"tuples_deleted"`  // Tuples deleted
	TuplesConflict int64   `json:"tuples_conflict"` // Tuples conflicted
	TempFiles      int64   `json:"temp_files"`      // Temporary files created
	TempBytes      int64   `json:"temp_bytes"`      // Bytes written to temp files
	Deadlocks      int64   `json:"deadlocks"`       // Detected deadlocks
	StatsReset     string  `json:"stats_reset"`     // Last stats reset timestamp

	// Cache (shared_buffers)
	SharedBuffersSize  int64 `json:"shared_buffers_size"`  // shared_buffers in bytes
	EffectiveCacheSize int64 `json:"effective_cache_size"` // effective_cache_size in bytes

	// Database size
	DatabaseSize int64 `json:"database_size"` // Database size in bytes

	// Server version
	ServerVersion string `json:"server_version"` // PostgreSQL version string
}

// GetPostgresStats returns runtime statistics for a PostgreSQL database.
func GetPostgresStats() (*PGStats, error) {
	if DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql.DB: %w", err)
	}

	stats := &PGStats{}

	// Connection pool stats
	sqlStats := sqlDB.Stats()
	stats.MaxOpenConnections = sqlStats.MaxOpenConnections
	stats.OpenConnections = sqlStats.OpenConnections
	stats.InUse = sqlStats.InUse
	stats.Idle = sqlStats.Idle

	// Server version
	_ = sqlDB.QueryRow("SHOW server_version").Scan(&stats.ServerVersion)

	// Max connections setting
	_ = sqlDB.QueryRow("SHOW max_connections").Scan(&stats.MaxConnections)

	// Shared buffers and effective_cache_size (human-readable like "128MB")
	var sbStr, ecsStr string
	_ = sqlDB.QueryRow("SHOW shared_buffers").Scan(&sbStr)
	_ = sqlDB.QueryRow("SHOW effective_cache_size").Scan(&ecsStr)
	stats.SharedBuffersSize = parsePGSize(sbStr)
	stats.EffectiveCacheSize = parsePGSize(ecsStr)

	// Connection stats from pg_stat_activity
	rows, err := sqlDB.Query(`
		SELECT state, COUNT(*)
		FROM pg_stat_activity
		WHERE datname IS NOT NULL
		GROUP BY state
	`)
	if err == nil {
		for rows.Next() {
			var state string
			var count int64
			if err := rows.Scan(&state, &count); err != nil {
				continue
			}
			stats.TotalConnections += int(count)
			switch state {
			case "active":
				stats.ActiveConnections = int(count)
			case "idle":
				stats.IdleConnections = int(count)
			case "idle in transaction":
				stats.IdleInTxn = int(count)
			}
		}
		rows.Close()
	}

	// Database-level stats from pg_stat_database
	var (
		numCommit, numRollback                                       int64
		blksRead, blksHit                                            int64
		tuplesReturned, tuplesFetched                                int64
		tuplesInserted, tuplesUpdated, tuplesDeleted, tuplesConflict int64
		tempFiles, tempBytes                                         int64
		deadlocks                                                    int64
		statsReset                                                   sql.NullString
	)
	err = sqlDB.QueryRow(`
		SELECT
			COALESCE(num_commit, 0),
			COALESCE(num_rollback, 0),
			COALESCE(blks_read, 0),
			COALESCE(blks_hit, 0),
			COALESCE(tuples_returned, 0),
			COALESCE(tuples_fetched, 0),
			COALESCE(tuples_inserted, 0),
			COALESCE(tuples_updated, 0),
			COALESCE(tuples_deleted, 0),
			COALESCE(tuples_conflict, 0),
			COALESCE(temp_files, 0),
			COALESCE(temp_bytes, 0),
			COALESCE(deadlocks, 0),
			stats_reset
		FROM pg_stat_database
		WHERE datname = current_database()
	`).Scan(
		&numCommit, &numRollback, &blksRead, &blksHit,
		&tuplesReturned, &tuplesFetched, &tuplesInserted, &tuplesUpdated,
		&tuplesDeleted, &tuplesConflict, &tempFiles, &tempBytes,
		&deadlocks, &statsReset,
	)
	if err == nil {
		stats.NumCommit = numCommit
		stats.NumRollback = numRollback
		stats.BlksRead = blksRead
		stats.BlksHit = blksHit
		if blksHit+blksRead > 0 {
			stats.BlksHitRatio = float64(blksHit) / float64(blksHit+blksRead)
		}
		stats.TuplesReturned = tuplesReturned
		stats.TuplesFetched = tuplesFetched
		stats.TuplesInserted = tuplesInserted
		stats.TuplesUpdated = tuplesUpdated
		stats.TuplesDeleted = tuplesDeleted
		stats.TuplesConflict = tuplesConflict
		stats.TempFiles = tempFiles
		stats.TempBytes = tempBytes
		stats.Deadlocks = deadlocks
		if statsReset.Valid {
			stats.StatsReset = statsReset.String
		}
	}

	// Database size
	_ = sqlDB.QueryRow("SELECT pg_database_size(current_database())").Scan(&stats.DatabaseSize)

	return stats, nil
}

// parsePGSize converts a PostgreSQL human-readable size string like "128MB"
// or "4GB" into bytes.
func parsePGSize(s string) int64 {
	if s == "" {
		return 0
	}
	var num float64
	var unit string
	fmt.Sscanf(s, "%f%s", &num, &unit)
	multiplier := int64(1)
	switch unit {
	case "B", "":
		multiplier = 1
	case "kB", "KB":
		multiplier = 1024
	case "MB":
		multiplier = 1024 * 1024
	case "GB":
		multiplier = 1024 * 1024 * 1024
	case "TB":
		multiplier = 1024 * 1024 * 1024 * 1024
	}
	return int64(num) * multiplier
}

// ---------------------------------------------------------------------------
// Unified entry point
// ---------------------------------------------------------------------------

// GetDBStats returns runtime statistics for whatever database engine is
// currently active. It dispatches to GetSQLiteStats, GetMySQLStats, or
// GetPostgresStats based on the configured database type.
func GetDBStats() (*DBStatsResponse, error) {
	if DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	dbType, dsn := StartupConfig()
	if dbType == "" {
		dbType = DBTypeSQLite
	}

	resp := &DBStatsResponse{
		Engine: dbType,
		Info:   &DBInfo{Engine: dbType, ConnectionString: maskDSN(dsn, dbType)},
	}

	var err error
	switch dbType {
	case DBTypeSQLite:
		resp.SQLite, err = GetSQLiteStats()
		if resp.SQLite != nil {
			resp.Info.Version = resp.SQLite.JournalMode // will be replaced below
		}
		// Get SQLite version
		var version string
		sqlDB, _ := DB.DB()
		if sqlDB != nil {
			_ = sqlDB.QueryRow("SELECT sqlite_version()").Scan(&version)
		}
		resp.Info.Version = version
		// Database size = page_count * page_size
		if resp.SQLite != nil {
			resp.Info.DatabaseSize = int64(resp.SQLite.PageCount) * int64(resp.SQLite.PageSize)
		}

	case DBTypeMySQL:
		resp.MySQL, err = GetMySQLStats()
		// Get MySQL version
		var version string
		sqlDB, _ := DB.DB()
		if sqlDB != nil {
			_ = sqlDB.QueryRow("SELECT VERSION()").Scan(&version)
		}
		resp.Info.Version = version
		// Try to get database size from information_schema
		if sqlDB != nil {
			var dbSize int64
			_ = sqlDB.QueryRow(`
				SELECT SUM(data_length + index_length)
				FROM information_schema.tables
				WHERE table_schema = DATABASE()
			`).Scan(&dbSize)
			resp.Info.DatabaseSize = dbSize
		}

	case DBTypePostgres:
		resp.Postgres, err = GetPostgresStats()
		if resp.Postgres != nil {
			resp.Info.Version = resp.Postgres.ServerVersion
			resp.Info.DatabaseSize = resp.Postgres.DatabaseSize
		}

	default:
		return nil, fmt.Errorf("unsupported database type for stats: %s", dbType)
	}

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// maskDSN returns a DSN with the password replaced by "****" for safe
// display in the stats endpoint.
func maskDSN(dsn, dbType string) string {
	switch dbType {
	case DBTypeSQLite:
		// SQLite DSN is just a file path; no masking needed
		return dsn
	case DBTypeMySQL, DBTypePostgres:
		// Mask password in "user:password@host/db" or "scheme://user:password@host/db"
		if i := indexOf(dsn, "://"); i >= 0 {
			scheme := dsn[:i+3]
			rest := dsn[i+3:]
			if at := indexOf(rest, "@"); at >= 0 {
				userPass := rest[:at]
				if colon := indexOf(userPass, ":"); colon >= 0 {
					return scheme + userPass[:colon] + ":****@" + rest[at+1:]
				}
			}
		}
		// Also handle bare "user:password@host/db" (MySQL native DSN)
		if at := indexOf(dsn, "@"); at >= 0 {
			userPass := dsn[:at]
			if colon := indexOf(userPass, ":"); colon >= 0 {
				return userPass[:colon] + ":****@" + dsn[at+1:]
			}
		}
		return dsn
	default:
		return dsn
	}
}

// indexOf returns the index of the first occurrence of substr in s, or -1.
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// ---------------------------------------------------------------------------
// SQLite file size helper (used by backup service)
// ---------------------------------------------------------------------------

// SQLiteFileSize returns the size of the SQLite database file on disk.
// Returns 0 if the file does not exist or the database is not SQLite.
func SQLiteFileSize(dsn string) int64 {
	if dsn == "" {
		return 0
	}
	// Strip query parameters if present
	path := dsn
	if q := indexOf(path, "?"); q >= 0 {
		path = path[:q]
	}
	// Make absolute if relative
	if !filepath.IsAbs(path) {
		path = filepath.Join(getWorkDir(), path)
	}
	if info, err := os.Stat(path); err == nil {
		return info.Size()
	}
	return 0
}

// getWorkDir returns the current working directory.
func getWorkDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}
