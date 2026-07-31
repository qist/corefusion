<template>
  <div v-loading="loading" style="max-width:920px">
    <!-- 月度流量趋势 -->
    <el-card shadow="never" class="traffic-card">
      <template #header>
        <div class="card-head">
          <span class="card-title">{{ t('dashboard.trafficMonthly') }}</span>
          <el-radio-group v-model="months" size="small" @change="loadTraffic">
            <el-radio-button :value="3">3M</el-radio-button>
            <el-radio-button :value="6">6M</el-radio-button>
            <el-radio-button :value="12">12M</el-radio-button>
          </el-radio-group>
        </div>
      </template>

      <div v-if="trafficErr" class="empty error">{{ trafficErr }}</div>
      <div v-else-if="traffic.length === 0" class="empty">{{ t('common.noData') }}</div>
      <div v-else class="bar-chart">
        <div class="bar-row" v-for="m in traffic" :key="m.period_start">
          <div class="bar-label">{{ fmtMonth(m.period_start) }}</div>
          <div class="bar-track">
            <div class="bar-fill" :class="{ over: m.suspended }" :style="{ width: barWidth(m.total_bytes) }">
              <span class="bar-tip">{{ formatBytes(m.total_bytes) }}</span>
            </div>
          </div>
          <div class="bar-sub">↑{{ formatBytes(m.rx_bytes) }} ↓{{ formatBytes(m.tx_bytes) }}</div>
        </div>
        <div class="bar-legend">
          <span class="legend-item"><i class="dot total"></i>{{ t('dashboard.totalTraffic') }} {{ formatBytes(totalBytes) }}</span>
          <span v-if="quotaBytes > 0" class="legend-item">/ {{ formatBytes(quotaBytes) }}</span>
        </div>
      </div>
    </el-card>

    <!-- 实时速率 -->
    <el-card shadow="never" class="traffic-card">
      <template #header>
        <div class="card-head">
          <span class="card-title">{{ t('dashboard.trafficRealtime') }}</span>
          <el-radio-group v-model="duration" size="small" @change="loadMetrics">
            <el-radio-button value="1h">1h</el-radio-button>
            <el-radio-button value="6h">6h</el-radio-button>
            <el-radio-button value="24h">24h</el-radio-button>
          </el-radio-group>
        </div>
      </template>

      <div v-if="metricsErr" class="empty error">{{ metricsErr }}</div>
      <div v-else-if="metrics.length === 0" class="empty">{{ t('common.noData') }}</div>
      <div v-else>
        <div class="line-legend">
          <span class="legend-item"><i class="dot up"></i>{{ t('dashboard.uploadSpeed') }}</span>
          <span class="legend-item"><i class="dot down"></i>{{ t('dashboard.downloadSpeed') }}</span>
          <span class="legend-item muted">{{ t('dashboard.peak') }}: ↑{{ formatBytes(peakUp) }}/s ↓{{ formatBytes(peakDown) }}/s</span>
        </div>
        <svg class="line-chart" :viewBox="`0 0 ${chartW} ${chartH}`" preserveAspectRatio="none">
          <line x1="0" y1="0" :x2="chartW" y2="0" class="grid" />
          <line x1="0" :y1="chartH" :x2="chartW" :y2="chartH" class="grid" />
          <polyline :points="downPoints" class="line down" fill="none" />
          <polyline :points="upPoints" class="line up" fill="none" />
        </svg>
        <div class="line-axis">
          <span>{{ fmtTime(metrics[0].created_at) }}</span>
          <span>{{ fmtTime(metrics[metrics.length - 1].created_at) }}</span>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { getTrafficHistory, getMetricsHistory } from '../../api/nodeServer'

const props = defineProps({ serverId: { type: Number, required: true } })
const { t } = useI18n()

const months = ref(6)
const duration = ref('24h')
const loading = ref(false)
const traffic = ref([])
const trafficErr = ref('')
const metrics = ref([])
const metricsErr = ref('')

const chartW = 600
const chartH = 140
const padY = 10

const totalBytes = computed(() => traffic.value.reduce((s, m) => s + (m.total_bytes || 0), 0))
const quotaBytes = computed(() => traffic.value.reduce((m, x) => Math.max(m, x.quota_bytes || 0), 0))
const peakUp = computed(() => metrics.value.reduce((m, p) => Math.max(m, p.traffic_uplink || 0), 0))
const peakDown = computed(() => metrics.value.reduce((m, p) => Math.max(m, p.traffic_downlink || 0), 0))

const maxTotal = computed(() => {
  let m = 0
  traffic.value.forEach(x => { m = Math.max(m, x.total_bytes || 0) })
  return m || 1
})
function barWidth(v) {
  return Math.max(4, Math.round((v / maxTotal.value) * 100)) + '%'
}

const maxRate = computed(() => Math.max(peakUp.value, peakDown.value) || 1)
function buildPoints(field) {
  const arr = metrics.value
  const n = arr.length
  return arr.map((p, i) => {
    const x = n === 1 ? chartW / 2 : (i / (n - 1)) * chartW
    const v = p[field] || 0
    const y = chartH - padY - (v / maxRate.value) * (chartH - padY * 2)
    return `${x.toFixed(1)},${y.toFixed(1)}`
  }).join(' ')
}
const upPoints = computed(() => buildPoints('traffic_uplink'))
const downPoints = computed(() => buildPoints('traffic_downlink'))

function formatBytes(bytes) {
  if (!bytes || bytes <= 0) return '0B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  const i = Math.min(units.length - 1, Math.floor(Math.log(bytes) / Math.log(1024)))
  return (bytes / Math.pow(1024, i)).toFixed(i === 0 ? 0 : 1) + units[i]
}
function fmtMonth(s) { return s ? ('' + s).slice(0, 7) : '' }
function fmtTime(s) {
  if (!s) return ''
  return new Date(s).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

async function loadTraffic() {
  trafficErr.value = ''
  try {
    const data = await getTrafficHistory(props.serverId, months.value)
    traffic.value = Array.isArray(data) ? data : []
  } catch (e) {
    trafficErr.value = e?.response?.data?.message || t('common.loadFailed')
    traffic.value = []
  }
}
async function loadMetrics() {
  metricsErr.value = ''
  try {
    const data = await getMetricsHistory(props.serverId, { duration: duration.value, interval: '5m' })
    metrics.value = Array.isArray(data) ? data : []
  } catch (e) {
    metricsErr.value = e?.response?.data?.message || t('common.loadFailed')
    metrics.value = []
  }
}

onMounted(() => { loadTraffic(); loadMetrics() })
</script>

<style scoped>
.traffic-card { margin-bottom: 16px; border: none; }
.card-head { display: flex; justify-content: space-between; align-items: center; }
.card-title { font-weight: 600; font-size: 15px; }
.empty { text-align: center; color: #909399; padding: 24px 0; font-size: 13px; }
.empty.error { color: #f56c6c; }
.bar-chart { display: flex; flex-direction: column; gap: 10px; }
.bar-row { display: flex; align-items: center; gap: 12px; font-size: 13px; }
.bar-label { width: 48px; color: #909399; flex: none; }
.bar-track { flex: 1; height: 18px; background: #f0f2f5; border-radius: 4px; overflow: hidden; }
.bar-fill { height: 100%; background: linear-gradient(90deg, #409eff, #66b1ff); border-radius: 4px; min-width: 4px; transition: width .3s; display: flex; align-items: center; justify-content: flex-end; padding-right: 6px; }
.bar-fill.over { background: linear-gradient(90deg, #f56c6c, #f78989); }
.bar-tip { font-size: 11px; color: #fff; white-space: nowrap; }
.bar-sub { width: 150px; flex: none; color: #909399; font-size: 12px; text-align: right; }
.bar-legend { margin-top: 4px; font-size: 12px; color: #909399; display: flex; gap: 10px; flex-wrap: wrap; }
.legend-item { display: flex; align-items: center; gap: 4px; }
.legend-item.muted { margin-left: auto; }
.dot { width: 8px; height: 8px; border-radius: 50%; display: inline-block; }
.dot.total { background: #409eff; }
.dot.up { background: #67c23a; }
.dot.down { background: #409eff; }
.line-legend { display: flex; gap: 14px; font-size: 13px; color: #606266; margin-bottom: 8px; flex-wrap: wrap; }
.line-chart { width: 100%; height: 140px; display: block; }
.line { stroke-width: 1.6; vector-effect: non-scaling-stroke; }
.line.up { stroke: #67c23a; }
.line.down { stroke: #409eff; }
.grid { stroke: #ebeef5; stroke-width: 1; }
.line-axis { display: flex; justify-content: space-between; font-size: 11px; color: #909399; margin-top: 4px; }
</style>
