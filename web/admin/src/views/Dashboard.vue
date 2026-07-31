<template>
  <div class="dashboard">
    <div class="dash-header">
      <div>
        <h2 class="dash-title">{{ t('dashboard.title') }}</h2>
        <span class="dash-sub">{{ t('dashboard.subtitle') }}</span>
      </div>
      <div class="dash-actions">
        <span class="dash-updated" v-if="lastUpdated">{{ t('dashboard.lastUpdated') }}: {{ lastUpdated }}</span>
        <el-button size="small" :loading="loading" @click="loadAll">{{ t('common.refresh') }}</el-button>
      </div>
    </div>

    <div v-loading="loading">
      <!-- Fleet summary cards -->
      <el-row :gutter="16" class="summary-row">
        <el-col :span="4">
          <el-card shadow="hover" class="summary-card">
            <div class="sum-label">{{ t('dashboard.fleetNodes') }}</div>
            <div class="sum-value">{{ (fleet && fleet.online) || 0 }}<span class="sum-unit"> / {{ (fleet && fleet.total) || 0 }}</span></div>
            <div class="sum-foot">
              <span class="foot-on">{{ t('dashboard.fleetOnline') }} {{ (fleet && fleet.online) || 0 }}</span>
              <span class="foot-off" v-if="(fleet && fleet.offline)">{{ t('dashboard.fleetOffline') }} {{ fleet.offline }}</span>
              <span class="foot-deg" v-if="(fleet && fleet.degraded)">{{ t('dashboard.fleetDegraded') }} {{ fleet.degraded }}</span>
            </div>
          </el-card>
        </el-col>
        <el-col :span="4">
          <el-card shadow="hover" class="summary-card">
            <div class="sum-label">{{ t('dashboard.fleetServices') }}</div>
            <div class="sum-value">{{ (overview && overview.service_count) || 0 }}</div>
            <div class="sum-foot"><span class="foot-muted">{{ t('dashboard.fleetServices') }}</span></div>
          </el-card>
        </el-col>
        <el-col :span="4">
          <el-card shadow="hover" class="summary-card">
            <div class="sum-label">{{ t('dashboard.fleetSubscriptions') }}</div>
            <div class="sum-value">{{ (overview && overview.active_subscription_count) || 0 }}</div>
            <div class="sum-foot">
              <span class="foot-warn" v-if="(overview && overview.quota_warning_count)">{{ t('dashboard.fleetQuotaWarn') }} {{ overview.quota_warning_count }}</span>
              <span class="foot-muted" v-else>{{ t('dashboard.fleetQuotaWarn') }} 0</span>
            </div>
          </el-card>
        </el-col>
        <el-col :span="4">
          <el-card shadow="hover" class="summary-card">
            <div class="sum-label">{{ t('dashboard.fleetCerts') }}</div>
            <div class="sum-value">{{ (overview && overview.active_cert_count) || 0 }}</div>
            <div class="sum-foot">
              <span class="foot-warn" v-if="(overview && overview.expiring_cert_count)">{{ t('dashboard.alertExpiringCert') }} {{ overview.expiring_cert_count }}</span>
              <span class="foot-muted" v-else>{{ t('dashboard.alertExpiringCert') }} 0</span>
            </div>
          </el-card>
        </el-col>
        <el-col :span="4">
          <el-card shadow="hover" class="summary-card">
            <div class="sum-label">{{ t('dashboard.fleetRegions') }}</div>
            <div class="sum-value">{{ regionOptions.length }}</div>
            <div class="sum-foot"><span class="foot-muted">{{ regionOptions.length }} {{ t('dashboard.filterRegion') }}</span></div>
          </el-card>
        </el-col>
        <el-col :span="4">
          <el-card shadow="hover" class="summary-card panel-card">
            <div class="sum-label">{{ t('dashboard.panelServer') }}</div>
            <template v-if="panelStats">
              <div class="sum-value">{{ panelStats.cpu_usage != null ? Math.round(panelStats.cpu_usage) : '—' }}<span class="sum-unit">%</span></div>
              <div class="sum-foot">
                <span class="foot-muted" v-if="panelStats.mem_usage != null">MEM {{ Math.round(panelStats.mem_usage) }}%</span>
              </div>
            </template>
            <div class="sum-foot" v-else><span class="foot-muted">—</span></div>
          </el-card>
        </el-col>
      </el-row>

      <!-- Node health grid -->
      <el-card class="block-card">
        <template #header>
          <div class="card-head">
            <span class="card-title">{{ t('dashboard.nodeGrid') }}</span>
            <div class="filters">
              <el-select v-model="statusFilter" :placeholder="t('dashboard.filterStatus')" clearable size="small" class="f-select">
                <el-option :label="t('dashboard.filterAll')" value="" />
                <el-option :label="t('dashboard.statusOnline')" value="online" />
                <el-option :label="t('dashboard.statusOffline')" value="offline" />
                <el-option :label="t('dashboard.statusDegraded')" value="degraded" />
              </el-select>
              <el-select v-model="typeFilter" :placeholder="t('dashboard.filterType')" clearable size="small" class="f-select">
                <el-option :label="t('dashboard.filterAll')" value="" />
                <el-option :label="t('dashboard.typeStandalone')" value="standalone" />
                <el-option :label="t('dashboard.typeMirror')" value="mirror" />
                <el-option :label="t('dashboard.typePrimaryBackup')" value="primary_backup" />
                <el-option :label="t('dashboard.typeChainEntry')" value="chain_entry" />
                <el-option :label="t('dashboard.typeChainExit')" value="chain_exit" />
              </el-select>
              <el-select v-model="regionFilter" :placeholder="t('dashboard.filterRegion')" clearable size="small" class="f-select" v-if="regionOptions.length">
                <el-option :label="t('dashboard.filterAll')" value="" />
                <el-option v-for="r in regionOptions" :key="r" :label="r" :value="r" />
              </el-select>
              <el-input v-model="keyword" :placeholder="t('common.search')" clearable size="small" class="f-input" />
            </div>
          </div>
        </template>

        <div v-if="!filteredNodes.length" class="empty-hint">{{ t('common.noData') }}</div>
        <el-row :gutter="16" v-else>
          <el-col :xs="24" :sm="12" :md="8" :lg="6" v-for="node in filteredNodes" :key="node.id" class="node-col">
            <div class="node-card" :class="{ 'is-offline': !node.online }" @click="goDetail(node)">
              <div class="node-head">
                <span class="status-dot" :class="'dot-' + (node.status || 'offline')"></span>
                <span class="node-name" :title="node.name">{{ node.name }}</span>
                <el-tag size="mini" class="node-type">{{ typeLabel(node.node_type) }}</el-tag>
              </div>
              <div class="node-sub">{{ node.region || '—' }}<template v-if="node.hostname"> · {{ node.hostname }}</template></div>

              <template v-if="node.online && node.metric">
                <div class="metric-line">
                  <span class="ml-label">{{ t('dashboard.cpu') }}</span>
                  <el-progress :percentage="clampPct(node.metric.cpu_usage)" :color="pctColor(node.metric.cpu_usage)" :show-text="false" class="ml-bar" />
                  <span class="ml-val">{{ Math.round(node.metric.cpu_usage) }}%</span>
                </div>
                <div class="metric-line">
                  <span class="ml-label">{{ t('dashboard.mem') }}</span>
                  <el-progress :percentage="clampPct(node.metric.mem_usage)" :color="pctColor(node.metric.mem_usage)" :show-text="false" class="ml-bar" />
                  <span class="ml-val">{{ Math.round(node.metric.mem_usage) }}%</span>
                </div>
                <div class="metric-line">
                  <span class="ml-label">{{ t('dashboard.disk') }}</span>
                  <el-progress :percentage="clampPct(node.metric.disk_usage)" :color="pctColor(node.metric.disk_usage)" :show-text="false" class="ml-bar" />
                  <span class="ml-val">{{ Math.round(node.metric.disk_usage) }}%</span>
                </div>
                <div class="node-foot">
                  <span class="net-up">↑ {{ formatBytes(node.metric.net_tx_bytes) }}/s</span>
                  <span class="net-down">↓ {{ formatBytes(node.metric.net_rx_bytes) }}/s</span>
                  <span class="uptime" v-if="node.metric.uptime_seconds">{{ formatUptime(node.metric.uptime_seconds) }}</span>
                </div>
              </template>
              <template v-else>
                <div class="offline-box">
                  <div class="offline-text">{{ t('dashboard.statusOffline') }}</div>
                  <div class="offline-heart" v-if="node.last_heartbeat_age_seconds != null">{{ t('dashboard.lastHeartbeat') }} {{ node.last_heartbeat_age_seconds }}s {{ t('dashboard.heartbeatAgo') }}</div>
                  <div class="offline-heart" v-else>—</div>
                </div>
              </template>
            </div>
          </el-col>
        </el-row>
      </el-card>

      <!-- Cluster resource -->
      <el-row :gutter="16" class="block-row">
        <el-col :span="14">
          <el-card class="block-card">
            <template #header><span class="card-title">{{ t('dashboard.clusterResource') }}</span></template>
            <el-row :gutter="16">
              <el-col :span="8">
                <div class="cluster-item">
                  <div class="ci-label">{{ t('dashboard.clusterAvgCpu') }}</div>
                  <el-progress type="dashboard" :width="92" :percentage="Math.round(avgCpu)" :color="pctColor(avgCpu)" />
                </div>
              </el-col>
              <el-col :span="8">
                <div class="cluster-item">
                  <div class="ci-label">{{ t('dashboard.clusterAvgMem') }}</div>
                  <el-progress type="dashboard" :width="92" :percentage="Math.round(avgMem)" :color="pctColor(avgMem)" />
                </div>
              </el-col>
              <el-col :span="8">
                <div class="cluster-item">
                  <div class="ci-label">{{ t('dashboard.clusterAvgDisk') }}</div>
                  <el-progress type="dashboard" :width="92" :percentage="Math.round(avgDisk)" :color="pctColor(avgDisk)" />
                </div>
              </el-col>
            </el-row>
            <div class="cluster-foot">
              <span>{{ t('dashboard.netTraffic') }}: {{ formatBytes(totalNet) }}/s</span>
              <span>{{ t('dashboard.userTraffic') }}: {{ formatBytes(totalTraffic) }}/s</span>
              <span class="muted">{{ t('dashboard.basedOn') }} {{ onlineWithMetric.length }} {{ t('dashboard.fleetOnline') }}</span>
            </div>
          </el-card>
        </el-col>
        <el-col :span="10">
          <el-card class="block-card">
            <template #header><span class="card-title">{{ t('dashboard.topLoadNodes') }}</span></template>
            <div v-if="!topLoad.length" class="empty-hint">{{ t('common.noData') }}</div>
            <div v-for="n in topLoad" :key="n.id" class="top-row" @click="goDetail(n)">
              <span class="top-name">{{ n.name }}</span>
              <el-progress :percentage="clampPct(n.metric.cpu_usage)" :color="pctColor(n.metric.cpu_usage)" :show-text="false" class="top-bar" />
              <span class="top-val">{{ Math.round(n.metric.cpu_usage) }}%</span>
            </div>
          </el-card>
        </el-col>
      </el-row>

      <!-- Alerts -->
      <el-card class="block-card">
        <template #header><span class="card-title">{{ t('dashboard.alerts') }}</span></template>
        <el-alert v-if="!hasAlerts" :title="t('dashboard.noAlerts')" type="success" :closable="false" show-icon />
        <div v-else class="alert-grid">
          <div class="alert-group" v-if="offlineNodes.length">
            <div class="ag-title ag-off">{{ t('dashboard.alertOffline') }} ({{ offlineNodes.length }})</div>
            <div class="ag-list">
              <span class="ag-chip" v-for="n in offlineNodes" :key="n.id" @click="goDetail(n)">{{ n.name }}</span>
            </div>
          </div>
          <div class="alert-group" v-if="degradedNodes.length">
            <div class="ag-title ag-deg">{{ t('dashboard.alertDegraded') }} ({{ degradedNodes.length }})</div>
            <div class="ag-list">
              <span class="ag-chip" v-for="n in degradedNodes" :key="n.id" @click="goDetail(n)">{{ n.name }}</span>
            </div>
          </div>
          <div class="alert-group" v-if="highResource.length">
            <div class="ag-title ag-warn">{{ t('dashboard.alertHighResource') }} ({{ highResource.length }})</div>
            <div class="ag-list">
              <span class="ag-chip" v-for="n in highResource" :key="n.id" @click="goDetail(n)">{{ n.name }} {{ Math.round(n.metric.cpu_usage) }}%</span>
            </div>
          </div>
          <div class="alert-group" v-if="expiringCertList.length">
            <div class="ag-title ag-warn">{{ t('dashboard.alertExpiringCert') }} ({{ expiringCertList.length }})</div>
            <div class="ag-list">
              <span class="ag-chip" v-for="c in expiringCertList" :key="c.id" @click="goCerts()">
                {{ c.label }} · {{ certDays(c.expires_at) }}{{ t('dashboard.daysLeft') }}
              </span>
            </div>
          </div>
          <div class="alert-group" v-if="quotaWarn">
            <div class="ag-title ag-warn">{{ t('dashboard.alertQuotaWarn') }} ({{ quotaWarn }})</div>
          </div>
        </div>
      </el-card>
    </div>
  </div>
</template>

<script>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import client from '../api/client'
import {
  getDashboardStatus,
  getAggregatedMetrics,
  getMetricsOverview,
} from '../api/nodeServer'

const TYPE_MAP = {
  standalone: 'typeStandalone',
  mirror: 'typeMirror',
  primary_backup: 'typePrimaryBackup',
  chain_entry: 'typeChainEntry',
  chain_exit: 'typeChainExit',
}
const STATUS_MAP = {
  online: 'statusOnline',
  offline: 'statusOffline',
  degraded: 'statusDegraded',
}

export default {
  name: 'Dashboard',
  setup() {
    const { t } = useI18n()
    const router = useRouter()

    const fleet = ref(null)
    const aggregated = ref(null)
    const overview = ref(null)
    const panelStats = ref(null)
    const loading = ref(false)
    const lastUpdated = ref('')

    const statusFilter = ref('')
    const typeFilter = ref('')
    const regionFilter = ref('')
    const keyword = ref('')

    let timer = null

    function avg(arr) {
      if (!arr.length) return 0
      return arr.reduce((s, v) => s + v, 0) / arr.length
    }
    function clampPct(v) { return Math.max(0, Math.min(100, Math.round(v || 0))) }
    function pctColor(v) {
      if (v >= 80) return '#f56c6c'
      if (v >= 60) return '#e6a23c'
      return '#67c23a'
    }
    function typeLabel(v) { return t('dashboard.' + (TYPE_MAP[v] || 'typeStandalone')) }
    function statusLabel(v) { return t('dashboard.' + (STATUS_MAP[v] || 'statusOffline')) }
    function formatUptime(sec) {
      if (!sec) return '—'
      const d = Math.floor(sec / 86400)
      const h = Math.floor((sec % 86400) / 3600)
      const m = Math.floor((sec % 3600) / 60)
      if (d > 0) return d + 'd ' + h + 'h'
      if (h > 0) return h + 'h ' + m + 'm'
      return m + 'm'
    }

    const nodes = computed(() => {
      const servers = (fleet.value && fleet.value.servers) || []
      const agg = aggregated.value || {}
      return servers.map((s) => ({ ...s, metric: agg[s.id] || null }))
    })

    const filteredNodes = computed(() => {
      const kw = (keyword.value || '').toLowerCase()
      return nodes.value.filter((n) => {
        if (statusFilter.value && n.status !== statusFilter.value) return false
        if (typeFilter.value && n.node_type !== typeFilter.value) return false
        if (regionFilter.value && n.region !== regionFilter.value) return false
        if (kw && !((n.name || '').toLowerCase().includes(kw))) return false
        return true
      })
    })

    const regionOptions = computed(() => {
      const br = (fleet.value && fleet.value.by_region) || {}
      return Object.keys(br)
    })

    const onlineWithMetric = computed(() =>
      nodes.value.filter((n) => n.online && n.metric)
    )
    const avgCpu = computed(() => avg(onlineWithMetric.value.map((n) => n.metric.cpu_usage)))
    const avgMem = computed(() => avg(onlineWithMetric.value.map((n) => n.metric.mem_usage)))
    const avgDisk = computed(() => avg(onlineWithMetric.value.map((n) => n.metric.disk_usage)))
    const totalNet = computed(() =>
      onlineWithMetric.value.reduce((s, n) => s + (n.metric.net_tx_bytes || 0) + (n.metric.net_rx_bytes || 0), 0)
    )
    const totalTraffic = computed(() =>
      onlineWithMetric.value.reduce((s, n) => s + (n.metric.traffic_uplink || 0) + (n.metric.traffic_downlink || 0), 0)
    )
    const topLoad = computed(() =>
      [...onlineWithMetric.value].sort((a, b) => b.metric.cpu_usage - a.metric.cpu_usage).slice(0, 5)
    )

    const offlineNodes = computed(() => nodes.value.filter((n) => !n.online))
    const degradedNodes = computed(() => nodes.value.filter((n) => n.status === 'degraded'))
    const highResource = computed(() =>
      onlineWithMetric.value.filter(
        (n) => n.metric.cpu_usage > 80 || n.metric.mem_usage > 80 || n.metric.disk_usage > 80
      )
    )
    const expiringCertList = computed(() => (overview.value && overview.value.expiring_certs) || [])
    const expiringCerts = computed(() => expiringCertList.value.length)
    const quotaWarn = computed(() => (overview.value && overview.value.quota_warning_count) || 0)
    const hasAlerts = computed(
      () => offlineNodes.value.length || degradedNodes.value.length || highResource.value.length || expiringCerts.value || quotaWarn.value
    )

    function goDetail(node) {
      router.push(`/node-servers/${node.id}/detail`)
    }
    function goCerts() {
      router.push('/certificates')
    }
    function certDays(expiresAt) {
      if (!expiresAt) return '—'
      const exp = new Date(expiresAt)
      if (isNaN(exp.getTime())) return '—'
      const days = Math.ceil((exp.getTime() - Date.now()) / 86400000)
      return days > 0 ? days : 0
    }

    function formatBytes(bytes) {
      if (!bytes || bytes === 0) return '0 B'
      const units = ['B', 'KB', 'MB', 'GB', 'TB']
      let i = 0
      let b = bytes
      while (b >= 1024 && i < units.length - 1) {
        b /= 1024
        i++
      }
      return b.toFixed(i > 0 ? 1 : 0) + ' ' + units[i]
    }

    async function loadAll() {
      loading.value = true
      try {
        const [d, a, o, p] = await Promise.all([
          getDashboardStatus(),
          getAggregatedMetrics(),
          getMetricsOverview(),
          client.get('/system/stats').catch(() => null),
        ])
        fleet.value = d
        aggregated.value = a
        overview.value = o
        panelStats.value = p
        lastUpdated.value = new Date().toLocaleTimeString()
      } catch (e) {
        // surface error via console; keep last good data
        console.error('[Dashboard] load failed', e)
      } finally {
        loading.value = false
      }
    }

    onMounted(() => {
      loadAll()
      timer = setInterval(loadAll, 8000)
    })
    onUnmounted(() => {
      if (timer) clearInterval(timer)
    })

    return {
      t,
      fleet, aggregated, overview, panelStats, loading, lastUpdated,
      statusFilter, typeFilter, regionFilter, keyword,
      filteredNodes, regionOptions, onlineWithMetric,
      avgCpu, avgMem, avgDisk, totalNet, totalTraffic, topLoad,
      offlineNodes, degradedNodes, highResource, expiringCerts, expiringCertList, quotaWarn, hasAlerts,
      typeLabel, statusLabel, clampPct, pctColor, formatBytes, formatUptime, goDetail, goCerts, certDays,
    }
  },
}
</script>

<style scoped>
.dashboard { padding: 4px 8px 24px; }
.dash-header { display: flex; justify-content: space-between; align-items: flex-end; margin: 4px 0 18px; }
.dash-title { margin: 0; font-size: 20px; font-weight: 600; }
.dash-sub { color: var(--el-text-color-secondary); font-size: 13px; }
.dash-actions { display: flex; align-items: center; gap: 12px; }
.dash-updated { color: var(--el-text-color-secondary); font-size: 12px; }

/* Fleet summary cards */
.summary-row { margin-bottom: 16px; }
.summary-card { border: none; box-shadow: 0 1px 3px rgba(0,0,0,.06); height: 116px; }
.summary-card :deep(.el-card__body) {
  height: 100%;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  padding: 16px 18px;
}
.sum-label { color: var(--el-text-color-secondary); font-size: 13px; font-weight: 500; }
.sum-value { font-size: 30px; font-weight: 700; line-height: 1; letter-spacing: -.5px; }
.sum-unit { font-size: 14px; font-weight: 500; color: var(--el-text-color-secondary); margin-left: 3px; }
.sum-foot { display: flex; gap: 8px; font-size: 12px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.foot-on { color: #67c23a; }
.foot-off { color: #f56c6c; }
.foot-deg { color: #e6a23c; }
.foot-warn { color: #e6a23c; }
.foot-muted { color: var(--el-text-color-secondary); }
.panel-card { border: none; background: linear-gradient(135deg, #1f2a44, #2d3b5e); box-shadow: 0 2px 8px rgba(31,42,68,.25); }
.panel-card .sum-label,
.panel-card .sum-value,
.panel-card .sum-unit { color: #fff; }
.panel-card .foot-muted { color: rgba(255,255,255,.7); }

/* Shared block cards */
.block-card { margin-bottom: 16px; border: none; box-shadow: 0 1px 3px rgba(0,0,0,.06); }
/* 同一行内的卡片等高，避免右侧卡片明显偏矮 */
.block-row { margin-bottom: 16px; }
.block-row > .el-col > .block-card { height: 100%; margin-bottom: 0; }
.card-head { display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 8px; }
.card-title { font-weight: 600; font-size: 15px; }
.filters { display: flex; gap: 8px; flex-wrap: wrap; }
.f-select { width: 120px; }
.f-input { width: 160px; }

/* Node health grid */
.node-col { margin-bottom: 16px; }
.node-card {
  position: relative;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 10px;
  padding: 14px 14px 14px 17px;
  cursor: pointer;
  transition: all .2s;
  background: var(--el-bg-color);
  display: flex;
  flex-direction: column;
  min-height: 172px;
  overflow: hidden;
}
.node-card::before { content: ''; position: absolute; left: 0; top: 0; bottom: 0; width: 3px; background: var(--el-color-primary); }
.node-card.is-offline { background: #fafafa; }
.node-card.is-offline::before { background: #c0c4cc; }
.node-card:hover { box-shadow: 0 6px 16px rgba(0,0,0,.10); transform: translateY(-2px); border-color: var(--el-color-primary-light-5); }
.node-head { display: flex; align-items: center; gap: 8px; }
.node-name { font-weight: 600; font-size: 14px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex: 1; }
.node-type { flex: none; }
.node-sub { color: var(--el-text-color-secondary); font-size: 12px; margin: 4px 0 12px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.status-dot { width: 9px; height: 9px; border-radius: 50%; flex: none; }
.dot-online { background: #67c23a; }
.dot-offline { background: #c0c4cc; }
.dot-degraded { background: #e6a23c; }

.metric-line { display: flex; align-items: center; gap: 8px; margin: 5px 0; font-size: 12px; }
.ml-label { width: 30px; color: var(--el-text-color-secondary); flex: none; }
.ml-bar { flex: 1; }
.ml-val { width: 38px; text-align: right; font-variant-numeric: tabular-nums; flex: none; }
.node-foot { display: flex; gap: 10px; flex-wrap: wrap; margin-top: auto; padding-top: 10px; font-size: 12px; color: var(--el-text-color-secondary); border-top: 1px solid var(--el-border-color-lighter); }
.net-up { color: #e6a23c; }
.net-down { color: #409eff; }
.uptime { margin-left: auto; }

.offline-box { margin-top: auto; padding: 14px 0; text-align: center; }
.offline-text { color: #909399; font-weight: 600; }
.offline-heart { color: var(--el-text-color-secondary); font-size: 12px; margin-top: 4px; }

/* Cluster resource */
.cluster-item { text-align: center; padding: 8px 0; }
.ci-label { font-size: 13px; color: var(--el-text-color-secondary); margin-bottom: 8px; }
.cluster-foot { display: flex; gap: 18px; flex-wrap: wrap; margin-top: 14px; font-size: 13px; }
.cluster-foot .muted { color: var(--el-text-color-secondary); }

/* Top load list */
.top-row { display: flex; align-items: center; gap: 10px; padding: 7px 0; cursor: pointer; border-bottom: 1px solid var(--el-border-color-lighter); }
.top-row:last-child { border-bottom: none; }
.top-name { width: 120px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 13px; }
.top-bar { flex: 1; }
.top-val { width: 42px; text-align: right; font-size: 13px; font-variant-numeric: tabular-nums; }

/* Alerts */
.alert-grid { display: flex; flex-direction: column; gap: 14px; }
.ag-title { font-weight: 600; font-size: 13px; margin-bottom: 8px; }
.ag-off { color: #f56c6c; }
.ag-deg { color: #e6a23c; }
.ag-warn { color: #e6a23c; }
.ag-list { display: flex; gap: 8px; flex-wrap: wrap; }
.ag-chip { background: var(--el-fill-color-light); border-radius: 6px; padding: 3px 10px; font-size: 12px; cursor: pointer; transition: all .15s; }
.ag-chip:hover { background: var(--el-color-primary-light-9); color: var(--el-color-primary); }

.empty-hint { color: var(--el-text-color-secondary); text-align: center; padding: 24px 0; font-size: 13px; }
</style>
