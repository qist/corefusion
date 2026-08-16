<template>
  <div class="db-stats-page">
    <!-- Header -->
    <div class="stats-header">
      <div>
        <h2 style="margin:0 0 4px">{{ t('dbStats.title') }}</h2>
        <span style="color:var(--el-text-color-secondary);font-size:13px">{{ t('dbStats.subtitle') }}</span>
      </div>
      <div style="display:flex;align-items:center;gap:12px">
        <span v-if="lastUpdated" style="color:var(--el-text-color-secondary);font-size:12px">{{ t('dbStats.lastUpdated') }}: {{ lastUpdated }}</span>
        <el-switch v-model="autoRefresh" :active-text="t('dbStats.autoRefresh')" size="small" />
        <el-button size="small" :loading="loading" @click="loadAll">{{ t('common.refresh') }}</el-button>
      </div>
    </div>

    <div v-loading="loading">
      <!-- Overview Cards -->
      <el-row :gutter="16" class="overview-row" v-if="statsData">
        <el-col :span="6">
          <el-card shadow="hover" class="overview-card">
            <div class="ov-label">{{ t('dbStats.engine') }}</div>
            <div class="ov-value">
              <el-tag :type="engineTagType" size="large">{{ statsData.engine }}</el-tag>
            </div>
            <div class="ov-foot">{{ statsData.info?.version || '—' }}</div>
          </el-card>
        </el-col>
        <el-col :span="6">
          <el-card shadow="hover" class="overview-card">
            <div class="ov-label">{{ t('dbStats.databaseSize') }}</div>
            <div class="ov-value">{{ formatBytes(statsData.info?.database_size) }}</div>
            <div class="ov-foot">{{ t('dbStats.connectionStr') }}: <code style="font-size:11px">{{ statsData.info?.connection_string || '—' }}</code></div>
          </el-card>
        </el-col>
        <el-col :span="6">
          <el-card shadow="hover" class="overview-card">
            <div class="ov-label">{{ t('dbStats.connectivity') }}</div>
            <div class="ov-value">
              <el-tag v-if="pingResult" :type="pingResult.status === 'ok' ? 'success' : 'danger'" size="large">
                {{ pingResult.status === 'ok' ? 'OK' : 'FAIL' }}
              </el-tag>
              <span v-else style="color:var(--el-text-color-secondary)">—</span>
            </div>
            <div class="ov-foot" v-if="pingResult">{{ t('dbStats.latency') }}: {{ pingResult.latency }}</div>
          </el-card>
        </el-col>
        <el-col :span="6">
          <el-card shadow="hover" class="overview-card">
            <div class="ov-label">{{ t('dbStats.poolStatus') }}</div>
            <div class="ov-value" v-if="poolStats">
              {{ poolStats.in_use }}<span class="ov-unit"> / {{ poolStats.max_open_connections }}</span>
            </div>
            <div class="ov-foot" v-if="poolStats">
              {{ t('dbStats.idle') }}: {{ poolStats.idle }} · {{ t('dbStats.waitCount') }}: {{ poolStats.wait_count }}
            </div>
          </el-card>
        </el-col>
      </el-row>

      <!-- Connection Pool Detail -->
      <el-card class="section-card" v-if="poolStats">
        <template #header>
          <div class="card-head">
            <span class="card-title"><i class="ct-bar"></i>{{ t('dbStats.poolDetail') }}</span>
            <span style="font-size:12px;color:var(--el-text-color-secondary)">{{ t('dbStats.poolDesc') }}</span>
          </div>
        </template>
        <el-descriptions :column="3" border>
          <el-descriptions-item :label="t('dbStats.maxOpenConns')">{{ poolStats.max_open_connections }}</el-descriptions-item>
          <el-descriptions-item :label="t('dbStats.openConns')">{{ poolStats.open_connections }}</el-descriptions-item>
          <el-descriptions-item :label="t('dbStats.inUse')">
            <el-tag :type="poolStats.in_use > 0 ? 'warning' : 'info'" size="small">{{ poolStats.in_use }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item :label="t('dbStats.idle')">{{ poolStats.idle }}</el-descriptions-item>
          <el-descriptions-item :label="t('dbStats.waitCount')">{{ poolStats.wait_count }}</el-descriptions-item>
          <el-descriptions-item :label="t('dbStats.waitDuration')">{{ poolStats.wait_duration }}</el-descriptions-item>
          <el-descriptions-item :label="t('dbStats.maxIdleClosed')">{{ poolStats.max_idle_closed }}</el-descriptions-item>
          <el-descriptions-item :label="t('dbStats.maxIdleTimeClosed')">{{ poolStats.max_idle_time_closed }}</el-descriptions-item>
          <el-descriptions-item :label="t('dbStats.maxLifetimeClosed')">{{ poolStats.max_lifetime_closed }}</el-descriptions-item>
        </el-descriptions>
      </el-card>

      <!-- SQLite Stats -->
      <template v-if="statsData?.sqlite">
        <el-card class="section-card">
          <template #header>
            <div class="card-head">
              <span class="card-title"><i class="ct-bar"></i>{{ t('dbStats.sqlite.cacheStats') }}</span>
              <el-button size="small" type="warning" :loading="resetting" @click="handleReset">{{ t('dbStats.resetCounters') }}</el-button>
            </div>
          </template>
          <el-row :gutter="16">
            <el-col :span="6">
              <div class="metric-box">
                <div class="metric-label">{{ t('dbStats.sqlite.cacheHitRatio') }}</div>
                <el-progress type="dashboard" :width="100" :percentage="Math.round(statsData.sqlite.cache_hit_ratio * 100)" :color="ratioColor(statsData.sqlite.cache_hit_ratio)">
                  <template #default>
                    <div class="metric-pct">{{ (statsData.sqlite.cache_hit_ratio * 100).toFixed(1) }}<span class="metric-unit">%</span></div>
                  </template>
                </el-progress>
              </div>
            </el-col>
            <el-col :span="6">
              <div class="stat-item">
                <div class="stat-label">{{ t('dbStats.sqlite.cacheHits') }}</div>
                <div class="stat-value text-success">{{ formatNum(statsData.sqlite.cache_hit_count) }}</div>
              </div>
              <div class="stat-item">
                <div class="stat-label">{{ t('dbStats.sqlite.cacheMisses') }}</div>
                <div class="stat-value text-danger">{{ formatNum(statsData.sqlite.cache_miss_count) }}</div>
              </div>
            </el-col>
            <el-col :span="6">
              <div class="stat-item">
                <div class="stat-label">{{ t('dbStats.sqlite.cacheWrites') }}</div>
                <div class="stat-value">{{ formatNum(statsData.sqlite.cache_write_count) }}</div>
              </div>
              <div class="stat-item">
                <div class="stat-label">{{ t('dbStats.sqlite.cacheSpills') }}</div>
                <div class="stat-value">{{ formatNum(statsData.sqlite.cache_spill_count) }}</div>
              </div>
            </el-col>
            <el-col :span="6">
              <div class="stat-item">
                <div class="stat-label">{{ t('dbStats.sqlite.cacheUsed') }}</div>
                <div class="stat-value">{{ formatBytes(statsData.sqlite.cache_used_bytes) }}</div>
              </div>
              <div class="stat-item">
                <div class="stat-label">{{ t('dbStats.sqlite.tempbufSpills') }}</div>
                <div class="stat-value">{{ formatNum(statsData.sqlite.tempbuf_spill_count) }}</div>
              </div>
            </el-col>
          </el-row>
        </el-card>

        <el-row :gutter="16">
          <el-col :span="12">
            <el-card class="section-card">
              <template #header><span class="card-title"><i class="ct-bar"></i>{{ t('dbStats.sqlite.memoryStats') }}</span></template>
              <el-descriptions :column="1" border>
                <el-descriptions-item :label="t('dbStats.sqlite.schemaUsed')">{{ formatBytes(statsData.sqlite.schema_used_bytes) }}</el-descriptions-item>
                <el-descriptions-item :label="t('dbStats.sqlite.stmtUsed')">{{ formatBytes(statsData.sqlite.stmt_used_bytes) }}</el-descriptions-item>
                <el-descriptions-item :label="t('dbStats.sqlite.lookasideUsed')">{{ formatBytes(statsData.sqlite.lookaside_used_bytes) }}</el-descriptions-item>
                <el-descriptions-item :label="t('dbStats.sqlite.lookasideHits')">{{ formatNum(statsData.sqlite.lookaside_hit_count) }}</el-descriptions-item>
                <el-descriptions-item :label="t('dbStats.sqlite.lookasideMisses')">{{ formatNum(statsData.sqlite.lookaside_miss_count) }}</el-descriptions-item>
              </el-descriptions>
            </el-card>
          </el-col>
          <el-col :span="12">
            <el-card class="section-card">
              <template #header><span class="card-title"><i class="ct-bar"></i>{{ t('dbStats.sqlite.pragmaSettings') }}</span></template>
              <el-descriptions :column="1" border>
                <el-descriptions-item :label="t('dbStats.sqlite.journalMode')">
                  <el-tag size="small" :type="statsData.sqlite.journal_mode === 'wal' ? 'success' : 'info'">{{ statsData.sqlite.journal_mode }}</el-tag>
                </el-descriptions-item>
                <el-descriptions-item :label="t('dbStats.sqlite.synchronous')">{{ statsData.sqlite.synchronous }}</el-descriptions-item>
                <el-descriptions-item :label="t('dbStats.sqlite.foreignKeys')">
                  <el-tag size="small" :type="statsData.sqlite.foreign_keys === 'ON' ? 'success' : 'danger'">{{ statsData.sqlite.foreign_keys }}</el-tag>
                </el-descriptions-item>
                <el-descriptions-item :label="t('dbStats.sqlite.cacheSize')">{{ statsData.sqlite.cache_size }}</el-descriptions-item>
                <el-descriptions-item :label="t('dbStats.sqlite.pageSize')">{{ formatBytes(statsData.sqlite.page_size) }}</el-descriptions-item>
                <el-descriptions-item :label="t('dbStats.sqlite.pageCount')">{{ formatNum(statsData.sqlite.page_count) }}</el-descriptions-item>
                <el-descriptions-item :label="t('dbStats.sqlite.tempStore')">{{ statsData.sqlite.temp_store }}</el-descriptions-item>
                <el-descriptions-item :label="t('dbStats.sqlite.busyTimeout')">{{ statsData.sqlite.busy_timeout }} ms</el-descriptions-item>
                <el-descriptions-item :label="t('dbStats.sqlite.deferredFKs')">
                  <el-tag size="small" :type="statsData.sqlite.deferred_fks > 0 ? 'warning' : 'success'">{{ statsData.sqlite.deferred_fks }}</el-tag>
                </el-descriptions-item>
              </el-descriptions>
            </el-card>
          </el-col>
        </el-row>
      </template>

      <!-- MySQL Stats -->
      <template v-if="statsData?.mysql">
        <el-card class="section-card">
          <template #header>
            <div class="card-head">
              <span class="card-title"><i class="ct-bar"></i>{{ t('dbStats.mysql.connections') }}</span>
              <span style="font-size:12px;color:var(--el-text-color-secondary)">{{ t('dbStats.mysql.uptime') }}: {{ formatUptime(statsData.mysql.uptime) }}</span>
            </div>
          </template>
          <el-row :gutter="16">
            <el-col :span="6">
              <div class="metric-box">
                <div class="metric-label">{{ t('dbStats.mysql.threadsConnected') }}</div>
                <el-progress type="dashboard" :width="100" :percentage="ratioPct(statsData.mysql.threads_connected, statsData.mysql.max_used_connections)" :color="pctColor(ratioPct(statsData.mysql.threads_connected, statsData.mysql.max_used_connections))">
                  <template #default>
                    <div class="metric-pct">{{ statsData.mysql.threads_connected }}</div>
                  </template>
                </el-progress>
              </div>
            </el-col>
            <el-col :span="6">
              <div class="stat-item">
                <div class="stat-label">{{ t('dbStats.mysql.threadsRunning') }}</div>
                <div class="stat-value" :class="statsData.mysql.threads_running > 5 ? 'text-danger' : 'text-success'">{{ statsData.mysql.threads_running }}</div>
              </div>
              <div class="stat-item">
                <div class="stat-label">{{ t('dbStats.mysql.threadsCached') }}</div>
                <div class="stat-value">{{ statsData.mysql.threads_cached }}</div>
              </div>
            </el-col>
            <el-col :span="6">
              <div class="stat-item">
                <div class="stat-label">{{ t('dbStats.mysql.maxUsedConns') }}</div>
                <div class="stat-value">{{ statsData.mysql.max_used_connections }}</div>
              </div>
              <div class="stat-item">
                <div class="stat-label">{{ t('dbStats.mysql.abortedConnects') }}</div>
                <div class="stat-value" :class="statsData.mysql.aborted_connects > 0 ? 'text-danger' : ''">{{ statsData.mysql.aborted_connects }}</div>
              </div>
            </el-col>
            <el-col :span="6">
              <div class="stat-item">
                <div class="stat-label">{{ t('dbStats.mysql.totalConnections') }}</div>
                <div class="stat-value">{{ formatNum(statsData.mysql.connections) }}</div>
              </div>
              <div class="stat-item">
                <div class="stat-label">{{ t('dbStats.mysql.threadsCreated') }}</div>
                <div class="stat-value">{{ formatNum(statsData.mysql.threads_created) }}</div>
              </div>
            </el-col>
          </el-row>
        </el-card>

        <el-card class="section-card">
          <template #header><span class="card-title"><i class="ct-bar"></i>{{ t('dbStats.mysql.bufferPool') }}</span></template>
          <el-row :gutter="16">
            <el-col :span="8">
              <div class="metric-box">
                <div class="metric-label">{{ t('dbStats.mysql.bufferHitRate') }}</div>
                <el-progress type="dashboard" :width="100" :percentage="Math.round(statsData.mysql.buffer_pool_hit_rate * 100)" :color="ratioColor(statsData.mysql.buffer_pool_hit_rate)">
                  <template #default>
                    <div class="metric-pct">{{ (statsData.mysql.buffer_pool_hit_rate * 100).toFixed(1) }}<span class="metric-unit">%</span></div>
                  </template>
                </el-progress>
              </div>
            </el-col>
            <el-col :span="16">
              <el-descriptions :column="2" border>
                <el-descriptions-item :label="t('dbStats.mysql.bufferSize')">{{ formatBytes(statsData.mysql.buffer_pool_size) }}</el-descriptions-item>
                <el-descriptions-item :label="t('dbStats.mysql.bufferUsed')">{{ formatBytes(statsData.mysql.buffer_pool_used) }}</el-descriptions-item>
                <el-descriptions-item :label="t('dbStats.mysql.bufferFree')">{{ formatBytes(statsData.mysql.buffer_pool_free) }}</el-descriptions-item>
                <el-descriptions-item :label="t('dbStats.mysql.bufferReadReqs')">{{ formatNum(statsData.mysql.buffer_pool_read_requests) }}</el-descriptions-item>
                <el-descriptions-item :label="t('dbStats.mysql.bufferReads')">{{ formatNum(statsData.mysql.buffer_pool_reads) }}</el-descriptions-item>
              </el-descriptions>
            </el-col>
          </el-row>
        </el-card>

        <el-card class="section-card">
          <template #header><span class="card-title"><i class="ct-bar"></i>{{ t('dbStats.mysql.queryStats') }}</span></template>
          <el-row :gutter="16">
            <el-col :span="6">
              <div class="stat-item">
                <div class="stat-label">{{ t('dbStats.mysql.queries') }}</div>
                <div class="stat-value">{{ formatNum(statsData.mysql.queries) }}</div>
              </div>
            </el-col>
            <el-col :span="6">
              <div class="stat-item">
                <div class="stat-label">{{ t('dbStats.mysql.questions') }}</div>
                <div class="stat-value">{{ formatNum(statsData.mysql.questions) }}</div>
              </div>
            </el-col>
            <el-col :span="6">
              <div class="stat-item">
                <div class="stat-label">{{ t('dbStats.mysql.slowQueries') }}</div>
                <div class="stat-value" :class="statsData.mysql.slow_queries > 0 ? 'text-warning' : ''">{{ formatNum(statsData.mysql.slow_queries) }}</div>
              </div>
            </el-col>
            <el-col :span="6">
              <div class="stat-item">
                <div class="stat-label">{{ t('dbStats.mysql.uptime') }}</div>
                <div class="stat-value">{{ formatUptime(statsData.mysql.uptime) }}</div>
              </div>
            </el-col>
          </el-row>
          <el-divider />
          <el-row :gutter="16">
            <el-col :span="4">
              <div class="stat-item">
                <div class="stat-label">SELECT</div>
                <div class="stat-value">{{ formatNum(statsData.mysql.com_select) }}</div>
              </div>
            </el-col>
            <el-col :span="4">
              <div class="stat-item">
                <div class="stat-label">INSERT</div>
                <div class="stat-value">{{ formatNum(statsData.mysql.com_insert) }}</div>
              </div>
            </el-col>
            <el-col :span="4">
              <div class="stat-item">
                <div class="stat-label">UPDATE</div>
                <div class="stat-value">{{ formatNum(statsData.mysql.com_update) }}</div>
              </div>
            </el-col>
            <el-col :span="4">
              <div class="stat-item">
                <div class="stat-label">DELETE</div>
                <div class="stat-value">{{ formatNum(statsData.mysql.com_delete) }}</div>
              </div>
            </el-col>
            <el-col :span="4">
              <div class="stat-item">
                <div class="stat-label">COMMIT</div>
                <div class="stat-value">{{ formatNum(statsData.mysql.com_commit) }}</div>
              </div>
            </el-col>
            <el-col :span="4">
              <div class="stat-item">
                <div class="stat-label">ROLLBACK</div>
                <div class="stat-value">{{ formatNum(statsData.mysql.com_rollback) }}</div>
              </div>
            </el-col>
          </el-row>
        </el-card>

        <el-row :gutter="16">
          <el-col :span="12">
            <el-card class="section-card">
              <template #header><span class="card-title"><i class="ct-bar"></i>{{ t('dbStats.mysql.tempAndKey') }}</span></template>
              <el-descriptions :column="1" border>
                <el-descriptions-item :label="t('dbStats.mysql.createdTmpTables')">{{ formatNum(statsData.mysql.created_tmp_tables) }}</el-descriptions-item>
                <el-descriptions-item :label="t('dbStats.mysql.createdTmpDiskTables')">
                  <span :class="statsData.mysql.created_tmp_disk_tables > 0 ? 'text-warning' : ''">{{ formatNum(statsData.mysql.created_tmp_disk_tables) }}</span>
                </el-descriptions-item>
                <el-descriptions-item :label="t('dbStats.mysql.keyBlocksUsed')">{{ formatNum(statsData.mysql.key_blocks_used) }}</el-descriptions-item>
                <el-descriptions-item :label="t('dbStats.mysql.keyBlocksUnused')">{{ formatNum(statsData.mysql.key_blocks_unused) }}</el-descriptions-item>
                <el-descriptions-item :label="t('dbStats.mysql.keyReadRequests')">{{ formatNum(statsData.mysql.key_read_requests) }}</el-descriptions-item>
                <el-descriptions-item :label="t('dbStats.mysql.keyReads')">{{ formatNum(statsData.mysql.key_reads) }}</el-descriptions-item>
              </el-descriptions>
            </el-card>
          </el-col>
          <el-col :span="12">
            <el-card class="section-card">
              <template #header><span class="card-title"><i class="ct-bar"></i>{{ t('dbStats.mysql.network') }}</span></template>
              <el-descriptions :column="1" border>
                <el-descriptions-item :label="t('dbStats.mysql.bytesReceived')">{{ formatBytes(statsData.mysql.bytes_received) }}</el-descriptions-item>
                <el-descriptions-item :label="t('dbStats.mysql.bytesSent')">{{ formatBytes(statsData.mysql.bytes_sent) }}</el-descriptions-item>
                <el-descriptions-item :label="t('dbStats.mysql.keyWrites')">{{ formatNum(statsData.mysql.key_writes) }}</el-descriptions-item>
                <el-descriptions-item :label="t('dbStats.mysql.keyWriteRequests')">{{ formatNum(statsData.mysql.key_write_requests) }}</el-descriptions-item>
              </el-descriptions>
            </el-card>
          </el-col>
        </el-row>
      </template>

      <!-- PostgreSQL Stats -->
      <template v-if="statsData?.postgres">
        <el-card class="section-card">
          <template #header>
            <div class="card-head">
              <span class="card-title"><i class="ct-bar"></i>{{ t('dbStats.pg.connections') }}</span>
              <span style="font-size:12px;color:var(--el-text-color-secondary)">v{{ statsData.postgres.server_version }}</span>
            </div>
          </template>
          <el-row :gutter="16">
            <el-col :span="6">
              <div class="metric-box">
                <div class="metric-label">{{ t('dbStats.pg.totalConnections') }}</div>
                <el-progress type="dashboard" :width="100" :percentage="ratioPct(statsData.postgres.total_connections, statsData.postgres.max_connections)" :color="pctColor(ratioPct(statsData.postgres.total_connections, statsData.postgres.max_connections))">
                  <template #default>
                    <div class="metric-pct">{{ statsData.postgres.total_connections }}<span class="metric-unit">/{{ statsData.postgres.max_connections }}</span></div>
                  </template>
                </el-progress>
              </div>
            </el-col>
            <el-col :span="6">
              <div class="stat-item">
                <div class="stat-label">{{ t('dbStats.pg.activeConnections') }}</div>
                <div class="stat-value text-success">{{ statsData.postgres.active_connections }}</div>
              </div>
              <div class="stat-item">
                <div class="stat-label">{{ t('dbStats.pg.idleConnections') }}</div>
                <div class="stat-value">{{ statsData.postgres.idle_connections }}</div>
              </div>
            </el-col>
            <el-col :span="6">
              <div class="stat-item">
                <div class="stat-label">{{ t('dbStats.pg.idleInTxn') }}</div>
                <div class="stat-value" :class="statsData.postgres.idle_in_txn > 0 ? 'text-warning' : ''">{{ statsData.postgres.idle_in_txn }}</div>
              </div>
              <div class="stat-item">
                <div class="stat-label">{{ t('dbStats.pg.maxConnections') }}">{{ statsData.postgres.max_connections }}</div>
              </div>
            </el-col>
            <el-col :span="6">
              <div class="stat-item">
                <div class="stat-label">{{ t('dbStats.pg.sharedBuffers') }}</div>
                <div class="stat-value">{{ formatBytes(statsData.postgres.shared_buffers_size) }}</div>
              </div>
              <div class="stat-item">
                <div class="stat-label">{{ t('dbStats.pg.effectiveCache') }}</div>
                <div class="stat-value">{{ formatBytes(statsData.postgres.effective_cache_size) }}</div>
              </div>
            </el-col>
          </el-row>
        </el-card>

        <el-card class="section-card">
          <template #header><span class="card-title"><i class="ct-bar"></i>{{ t('dbStats.pg.cacheAndTuples') }}</span></template>
          <el-row :gutter="16">
            <el-col :span="8">
              <div class="metric-box">
                <div class="metric-label">{{ t('dbStats.pg.blksHitRatio') }}</div>
                <el-progress type="dashboard" :width="100" :percentage="Math.round(statsData.postgres.blks_hit_ratio * 100)" :color="ratioColor(statsData.postgres.blks_hit_ratio)">
                  <template #default>
                    <div class="metric-pct">{{ (statsData.postgres.blks_hit_ratio * 100).toFixed(1) }}<span class="metric-unit">%</span></div>
                  </template>
                </el-progress>
              </div>
            </el-col>
            <el-col :span="16">
              <el-descriptions :column="2" border>
                <el-descriptions-item :label="t('dbStats.pg.blksRead')">{{ formatNum(statsData.postgres.blks_read) }}</el-descriptions-item>
                <el-descriptions-item :label="t('dbStats.pg.blksHit')">{{ formatNum(statsData.postgres.blks_hit) }}</el-descriptions-item>
                <el-descriptions-item :label="t('dbStats.pg.tuplesReturned')">{{ formatNum(statsData.postgres.tuples_returned) }}</el-descriptions-item>
                <el-descriptions-item :label="t('dbStats.pg.tuplesFetched')">{{ formatNum(statsData.postgres.tuples_fetched) }}</el-descriptions-item>
                <el-descriptions-item :label="t('dbStats.pg.tuplesInserted')">{{ formatNum(statsData.postgres.tuples_inserted) }}</el-descriptions-item>
                <el-descriptions-item :label="t('dbStats.pg.tuplesUpdated')">{{ formatNum(statsData.postgres.tuples_updated) }}</el-descriptions-item>
                <el-descriptions-item :label="t('dbStats.pg.tuplesDeleted')">{{ formatNum(statsData.postgres.tuples_deleted) }}</el-descriptions-item>
                <el-descriptions-item :label="t('dbStats.pg.tuplesConflict')">{{ formatNum(statsData.postgres.tuples_conflict) }}</el-descriptions-item>
              </el-descriptions>
            </el-col>
          </el-row>
        </el-card>

        <el-row :gutter="16">
          <el-col :span="8">
            <el-card class="section-card">
              <template #header><span class="card-title"><i class="ct-bar"></i>{{ t('dbStats.pg.transactions') }}</span></template>
              <el-descriptions :column="1" border>
                <el-descriptions-item :label="t('dbStats.pg.commits')">
                  <span class="text-success">{{ formatNum(statsData.postgres.num_commit) }}</span>
                </el-descriptions-item>
                <el-descriptions-item :label="t('dbStats.pg.rollbacks')">
                  <span :class="statsData.postgres.num_rollback > 0 ? 'text-danger' : ''">{{ formatNum(statsData.postgres.num_rollback) }}</span>
                </el-descriptions-item>
              </el-descriptions>
            </el-card>
          </el-col>
          <el-col :span="8">
            <el-card class="section-card">
              <template #header><span class="card-title"><i class="ct-bar"></i>{{ t('dbStats.pg.tempAndDeadlocks') }}</span></template>
              <el-descriptions :column="1" border>
                <el-descriptions-item :label="t('dbStats.pg.tempFiles')">{{ formatNum(statsData.postgres.temp_files) }}</el-descriptions-item>
                <el-descriptions-item :label="t('dbStats.pg.tempBytes')">{{ formatBytes(statsData.postgres.temp_bytes) }}</el-descriptions-item>
                <el-descriptions-item :label="t('dbStats.pg.deadlocks')">
                  <span :class="statsData.postgres.deadlocks > 0 ? 'text-danger' : ''">{{ statsData.postgres.deadlocks }}</span>
                </el-descriptions-item>
              </el-descriptions>
            </el-card>
          </el-col>
          <el-col :span="8">
            <el-card class="section-card">
              <template #header><span class="card-title"><i class="ct-bar"></i>{{ t('dbStats.pg.misc') }}</span></template>
              <el-descriptions :column="1" border>
                <el-descriptions-item :label="t('dbStats.pg.databaseSize')">{{ formatBytes(statsData.postgres.database_size) }}</el-descriptions-item>
                <el-descriptions-item :label="t('dbStats.pg.statsReset')">{{ statsData.postgres.stats_reset || '—' }}</el-descriptions-item>
              </el-descriptions>
            </el-card>
          </el-col>
        </el-row>
      </template>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import client from '../api/client'

const { t } = useI18n()

const loading = ref(false)
const resetting = ref(false)
const autoRefresh = ref(true)
const lastUpdated = ref('')
const statsData = ref(null)
const poolStats = ref(null)
const pingResult = ref(null)
let timer = null

const engineTagType = computed(() => {
  if (!statsData.value) return 'info'
  switch (statsData.value.engine) {
    case 'sqlite': return 'info'
    case 'mysql': return 'success'
    case 'postgres': return 'warning'
    default: return 'info'
  }
})

function formatBytes(bytes) {
  if (!bytes || bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let b = bytes
  while (b >= 1024 && i < units.length - 1) { b /= 1024; i++ }
  return b.toFixed(i > 0 ? 1 : 0) + ' ' + units[i]
}

function formatNum(n) {
  if (n === 0) return '0'
  if (!n) return '—'
  return n.toLocaleString()
}

function formatUptime(sec) {
  if (!sec || sec === 0) return '—'
  const d = Math.floor(sec / 86400)
  const h = Math.floor((sec % 86400) / 3600)
  const m = Math.floor((sec % 3600) / 60)
  if (d > 0) return `${d}d ${h}h ${m}m`
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
}

function pctColor(v) {
  if (v >= 80) return '#f56c6c'
  if (v >= 60) return '#e6a23c'
  return '#67c23a'
}

function ratioColor(r) {
  if (r >= 0.95) return '#67c23a'
  if (r >= 0.8) return '#e6a23c'
  return '#f56c6c'
}

function ratioPct(cur, max) {
  if (!max || max === 0) return 0
  return Math.min(100, Math.round((cur / max) * 100))
}

async function loadStats() {
  try {
    const data = await client.get('/system/db-stats')
    statsData.value = data
    lastUpdated.value = new Date().toLocaleTimeString()
  } catch (e) {
    // error handled by interceptor
  }
}

async function loadPoolStats() {
  try {
    const data = await client.get('/system/db-pool-stats')
    poolStats.value = data
  } catch (e) { /* ignore */ }
}

async function loadPing() {
  try {
    const data = await client.get('/system/db-stats/ping', { suppressErrorMessage: true })
    pingResult.value = data
  } catch (e) {
    pingResult.value = { status: 'fail', latency: '—' }
  }
}

async function loadAll() {
  loading.value = true
  await Promise.all([loadStats(), loadPoolStats(), loadPing()])
  loading.value = false
}

async function handleReset() {
  resetting.value = true
  try {
    await client.post('/system/db-stats/reset')
    ElMessage.success(t('common.success'))
    await loadStats()
  } catch (e) { ElMessage.error(t('common.failed')) } finally { resetting.value = false }
}

onMounted(() => {
  loadAll()
  timer = setInterval(() => {
    if (autoRefresh.value) loadAll()
  }, 10000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>

<style scoped>
.db-stats-page { padding: 4px 8px 24px; }

.stats-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  margin: 4px 0 18px;
}

.overview-row { margin-bottom: 16px; }
.overview-card {
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 12px;
  box-shadow: 0 1px 3px rgba(16,24,40,.05);
  height: 120px;
  overflow: hidden;
  transition: transform .2s ease, box-shadow .2s ease;
}
.overview-card:hover { transform: translateY(-2px); box-shadow: 0 8px 20px rgba(16,24,40,.08); }
.overview-card :deep(.el-card__body) {
  height: 100%;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  padding: 14px 16px;
}
.ov-label { color: var(--el-text-color-secondary); font-size: 13px; font-weight: 500; }
.ov-value { font-size: 24px; font-weight: 700; line-height: 1; color: var(--el-text-color-primary); }
.ov-unit { font-size: 13px; font-weight: 500; color: var(--el-text-color-secondary); margin-left: 2px; }
.ov-foot { font-size: 12px; color: var(--el-text-color-secondary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

.section-card {
  margin-bottom: 16px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 12px;
  box-shadow: 0 1px 3px rgba(16,24,40,.05);
}
.section-card :deep(.el-card__header) { padding: 14px 18px; border-bottom: 1px solid var(--el-border-color-lighter); }
.card-head { display: flex; justify-content: space-between; align-items: center; }
.card-title { display: inline-flex; align-items: center; gap: 8px; font-weight: 600; font-size: 15px; }
.ct-bar { display: inline-block; width: 4px; height: 15px; border-radius: 2px; background: linear-gradient(180deg, var(--el-color-primary), var(--el-color-primary-light-3)); }

.metric-box { text-align: center; padding: 8px 0; }
.metric-label { color: var(--el-text-color-secondary); font-size: 13px; font-weight: 500; margin-bottom: 8px; }
.metric-pct { font-size: 22px; font-weight: 700; color: var(--el-text-color-primary); line-height: 1; }
.metric-unit { font-size: 13px; font-weight: 500; color: var(--el-text-color-secondary); margin-left: 1px; }

.stat-item { margin-bottom: 12px; }
.stat-item:last-child { margin-bottom: 0; }
.stat-label { color: var(--el-text-color-secondary); font-size: 12px; margin-bottom: 2px; }
.stat-value { font-size: 20px; font-weight: 700; color: var(--el-text-color-primary); line-height: 1.2; }
.text-success { color: #67c23a; }
.text-danger { color: #f56c6c; }
.text-warning { color: #e6a23c; }
</style>