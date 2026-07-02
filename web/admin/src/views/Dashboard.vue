<template>
  <div>
    <h2 style="margin-bottom:20px">{{ t('dashboard.title') }}</h2>

    <!-- Row 1: Resource Usage -->
    <el-row :gutter="16" style="margin-bottom:16px">
      <el-col :span="6" v-for="item in resourceCards" :key="item.label">
        <el-card shadow="hover" style="text-align:center">
          <div style="position:relative;width:110px;height:110px;margin:0 auto 10px">
            <svg viewBox="0 0 36 36" style="width:100%;height:100%;transform:rotate(-90deg)">
              <circle cx="18" cy="18" r="15.9" fill="none" stroke="#f0f0f0" stroke-width="3" />
              <circle cx="18" cy="18" r="15.9" fill="none" :stroke="item.color" stroke-width="3"
                :stroke-dasharray="`${item.usage} ${100 - item.usage}`" stroke-linecap="round" />
            </svg>
            <div style="position:absolute;top:50%;left:50%;transform:translate(-50%,-50%)">
              <div style="font-size:18px;font-weight:700;color:#303133">{{ item.usage }}%</div>
            </div>
          </div>
          <div style="font-weight:600;color:#303133;margin-bottom:4px">{{ item.label }}</div>
          <div style="font-size:12px;color:#909399">{{ item.used }} / {{ item.total }}</div>
        </el-card>
      </el-col>
    </el-row>

    <!-- Row 2: Quick Stats -->
    <el-row :gutter="16" style="margin-bottom:16px">
      <el-col :span="6" v-for="item in cards" :key="item.label">
        <el-card shadow="hover">
          <template #header>
            <div style="display:flex;align-items:center;gap:8px">
              <span style="font-size:18px">{{ item.icon }}</span>
              <span>{{ t(item.label) }}</span>
            </div>
          </template>
          <div style="font-size:32px;font-weight:700;color:#303133;text-align:center;padding:8px 0">{{ item.value }}</div>
        </el-card>
      </el-col>
    </el-row>

    <!-- Row 3: Info Cards -->
    <el-row :gutter="16" style="margin-bottom:16px">
      <el-col :span="8">
        <el-card shadow="hover" style="min-height:180px">
          <template #header>{{ t('dashboard.systemInfo') }}</template>
          <div style="display:flex;flex-direction:column;gap:8px;font-size:13px">
            <div style="display:flex;justify-content:space-between">
              <span style="color:#909399">{{ t('dashboard.os') }}</span>
              <span style="font-weight:600">{{ stats.os || '-' }} / {{ stats.arch || '-' }}</span>
            </div>
            <div style="display:flex;justify-content:space-between">
              <span style="color:#909399">{{ t('dashboard.hostname') }}</span>
              <span style="font-weight:600">{{ stats.hostname || '-' }}</span>
            </div>
            <div style="display:flex;justify-content:space-between">
              <span style="color:#909399">{{ t('dashboard.systemUptime') }}</span>
              <span style="font-weight:600">{{ formatUptime(stats.uptime) }}</span>
            </div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="8">
        <el-card shadow="hover" style="min-height:180px">
          <template #header>{{ t('dashboard.panelStatus') }}</template>
          <div style="display:flex;flex-direction:column;gap:8px;font-size:13px">
            <div style="display:flex;justify-content:space-between">
              <span style="color:#909399">{{ t('dashboard.version') }}</span>
              <span style="font-weight:600">{{ stats.version || 'v0.1.0' }}</span>
            </div>
            <div style="display:flex;justify-content:space-between">
              <span style="color:#909399">{{ t('dashboard.panelUptime') }}</span>
              <span style="font-weight:600">{{ formatUptime(stats.panel_uptime) }}</span>
            </div>
            <div style="display:flex;justify-content:space-between">
              <span style="color:#909399">Goroutines</span>
              <span style="font-weight:600">{{ stats.goroutines || 0 }}</span>
            </div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="8">
        <el-card shadow="hover" style="min-height:180px">
          <template #header>{{ t('dashboard.traffic') }}</template>
          <div style="display:flex;flex-direction:column;gap:8px;font-size:13px">
            <div style="display:flex;justify-content:space-between;align-items:center">
              <span style="color:#909399">{{ t('dashboard.upload') }}</span>
              <span style="font-weight:600;color:#409eff">{{ formatBytes(stats.net_bytes_sent) }}</span>
            </div>
            <div style="display:flex;justify-content:space-between;align-items:center">
              <span style="color:#909399">{{ t('dashboard.download') }}</span>
              <span style="font-weight:600;color:#67c23a">{{ formatBytes(stats.net_bytes_recv) }}</span>
            </div>
            <div style="display:flex;justify-content:space-between;align-items:center;border-top:1px solid #f0f0f0;padding-top:6px">
              <span style="color:#909399">{{ t('dashboard.totalTraffic') }}</span>
              <span style="font-weight:700;color:#303133">{{ formatBytes((stats.net_bytes_sent || 0) + (stats.net_bytes_recv || 0)) }}</span>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- Row 4: Load & Connections -->
    <el-row :gutter="16" style="margin-bottom:16px">
      <el-col :span="12">
        <el-card shadow="hover">
          <template #header>{{ t('dashboard.systemLoad') }}</template>
          <div style="display:flex;justify-content:space-around;text-align:center;padding:12px 0">
            <div>
              <div style="font-size:26px;font-weight:700;color:#409eff">{{ stats.load_avg_1?.toFixed(2) || '0.00' }}</div>
              <div style="font-size:12px;color:#909399;margin-top:4px">1 {{ t('dashboard.minute') }}</div>
            </div>
            <div>
              <div style="font-size:26px;font-weight:700;color:#e6a23c">{{ stats.load_avg_5?.toFixed(2) || '0.00' }}</div>
              <div style="font-size:12px;color:#909399;margin-top:4px">5 {{ t('dashboard.minutes') }}</div>
            </div>
            <div>
              <div style="font-size:26px;font-weight:700;color:#f56c6c">{{ stats.load_avg_15?.toFixed(2) || '0.00' }}</div>
              <div style="font-size:12px;color:#909399;margin-top:4px">15 {{ t('dashboard.minutes') }}</div>
            </div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="12">
        <el-card shadow="hover">
          <template #header>{{ t('dashboard.connections') }}</template>
          <div style="display:flex;justify-content:space-around;text-align:center;padding:12px 0">
            <div>
              <div style="font-size:26px;font-weight:700;color:#409eff">{{ stats.tcp_connections || 0 }}</div>
              <div style="font-size:12px;color:#909399;margin-top:4px">TCP</div>
            </div>
            <div>
              <div style="font-size:26px;font-weight:700;color:#67c23a">{{ stats.udp_connections || 0 }}</div>
              <div style="font-size:12px;color:#909399;margin-top:4px">UDP</div>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- Row 5: Bandwidth -->
    <el-card shadow="hover" style="margin-bottom:16px">
      <template #header>{{ t('dashboard.bandwidth') }}</template>
      <div style="display:flex;justify-content:space-around;text-align:center;padding:8px 0">
        <div>
          <div style="font-size:20px;font-weight:700;color:#409eff">{{ formatSpeed(uploadSpeed) }}</div>
          <div style="font-size:12px;color:#909399;margin-top:4px">{{ t('dashboard.uploadSpeed') }}</div>
        </div>
        <div>
          <div style="font-size:20px;font-weight:700;color:#67c23a">{{ formatSpeed(downloadSpeed) }}</div>
          <div style="font-size:12px;color:#909399;margin-top:4px">{{ t('dashboard.downloadSpeed') }}</div>
        </div>
        <div>
          <div style="font-size:18px;font-weight:600;color:#303133">{{ formatBytes(stats.net_bytes_sent) }}</div>
          <div style="font-size:12px;color:#909399;margin-top:4px">{{ t('dashboard.totalUpload') }}</div>
        </div>
        <div>
          <div style="font-size:18px;font-weight:600;color:#303133">{{ formatBytes(stats.net_bytes_recv) }}</div>
          <div style="font-size:12px;color:#909399;margin-top:4px">{{ t('dashboard.totalDownload') }}</div>
        </div>
      </div>
    </el-card>

    <!-- Row 6: Core Traffic -->
    <el-card shadow="hover">
      <template #header>{{ t('dashboard.coreTraffic') }}</template>
      <el-table :data="coreTraffic" stripe>
        <el-table-column prop="name" :label="t('common.name')" width="140" />
        <el-table-column prop="core_type" :label="t('service.coreType')" width="90">
          <template #default="{row}"><el-tag size="small">{{ row.core_type }}</el-tag></template>
        </el-table-column>
        <el-table-column :label="t('dashboard.state')" width="80">
          <template #default="{row}">
            <el-tag :type="row.state==='running'?'success':'info'" size="small">{{ row.state }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('dashboard.upload')" width="110">
          <template #default="{row}"><span style="color:#409eff">{{ formatBytes(row.upload || 0) }}</span></template>
        </el-table-column>
        <el-table-column :label="t('dashboard.download')" width="110">
          <template #default="{row}"><span style="color:#67c23a">{{ formatBytes(row.download || 0) }}</span></template>
        </el-table-column>
        <el-table-column :label="t('dashboard.liveUpload')" width="110">
          <template #default="{row}"><span style="color:#409eff">{{ formatSpeed(row.live_upload || 0) }}</span></template>
        </el-table-column>
        <el-table-column :label="t('dashboard.liveDownload')" width="110">
          <template #default="{row}"><span style="color:#67c23a">{{ formatSpeed(row.live_download || 0) }}</span></template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import client from '../api/client'

const { t } = useI18n()

const stats = ref({
  cpu_usage: 0, cpu_cores: 0, mem_total: 0, mem_used: 0, mem_free: 0, mem_usage: 0,
  swap_total: 0, swap_used: 0, swap_free: 0, swap_usage: 0,
  disk_total: 0, disk_used: 0, disk_free: 0, disk_usage: 0,
  hostname: '', os: '', arch: '', kernel: '', uptime: 0,
  load_avg_1: 0, load_avg_5: 0, load_avg_15: 0,
  tcp_connections: 0, udp_connections: 0,
  net_bytes_sent: 0, net_bytes_recv: 0, current_time: '', version: ''
})

const cards = ref([
  { label: 'dashboard.protoCenterCount', value: 0, icon: '🖥' },
  { label: 'dashboard.coreCount', value: 4, icon: '⚡' },
  { label: 'dashboard.subscriptionCount', value: 0, icon: '📋' },
  { label: 'dashboard.certCount', value: 0, icon: '🔒' },
])

const uploadSpeed = ref(0)
const downloadSpeed = ref(0)
const lastSent = ref(0)
const lastRecv = ref(0)
const coreTraffic = ref([])
let statsTimer = null

const resourceCards = computed(() => [
  { label: 'CPU', usage: Math.round(stats.value.cpu_usage || 0), used: `${stats.value.cpu_cores} cores`, total: `${stats.value.cpu_cores} cores`, color: stats.value.cpu_usage > 80 ? '#f56c6c' : '#409eff' },
  { label: t('dashboard.memory'), usage: Math.round(stats.value.mem_usage || 0), used: formatBytes(stats.value.mem_used), total: formatBytes(stats.value.mem_total), color: stats.value.mem_usage > 80 ? '#f56c6c' : '#67c23a' },
  { label: 'Swap', usage: Math.round(stats.value.swap_usage || 0), used: formatBytes(stats.value.swap_used), total: formatBytes(stats.value.swap_total), color: stats.value.swap_usage > 80 ? '#f56c6c' : '#e6a23c' },
  { label: t('dashboard.disk'), usage: Math.round(stats.value.disk_usage || 0), used: formatBytes(stats.value.disk_used), total: formatBytes(stats.value.disk_total), color: stats.value.disk_usage > 80 ? '#f56c6c' : '#909399' }
])

function formatBytes(bytes) {
  if (!bytes || bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0, b = bytes
  while (b >= 1024 && i < units.length - 1) { b /= 1024; i++ }
  return b.toFixed(i > 0 ? 1 : 0) + ' ' + units[i]
}

function formatSpeed(bps) {
  if (!bps || bps === 0) return '0 B/s'
  return formatBytes(bps) + '/s'
}

function formatUptime(sec) {
  if (!sec) return '-'
  const d = Math.floor(sec / 86400), h = Math.floor((sec % 86400) / 3600), m = Math.floor((sec % 3600) / 60)
  if (d > 0) return `${d}d ${h}h ${m}m`
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
}

async function fetchStats() {
  try {
    const data = await client.get('/system/stats')
    if (data) {
      if (lastSent.value > 0) {
        uploadSpeed.value = Math.max(0, data.net_bytes_sent - lastSent.value)
        downloadSpeed.value = Math.max(0, data.net_bytes_recv - lastRecv.value)
      }
      lastSent.value = data.net_bytes_sent
      lastRecv.value = data.net_bytes_recv
      stats.value = data
    }
  } catch (e) {}
}

async function fetchCards() {
  try {
    const [n, sub, c] = await Promise.all([
      client.get('/nodes', { params: { limit: 1 } }),
      client.get('/subscriptions', { params: { limit: 1 } }),
      client.get('/certificates', { params: { limit: 1 } }),
    ])
    cards.value[0].value = n?.total || 0
    cards.value[2].value = sub?.total || 0
    cards.value[3].value = c?.total || 0
  } catch (e) {}
}

async function fetchCoreTraffic() {
  try {
    const data = await client.get('/node-servers/1/services/process-status')
    coreTraffic.value = (data || []).map(s => ({ ...s, upload: 0, download: 0, live_upload: 0, live_download: 0 }))
  } catch (e) {}
}

onMounted(() => {
  fetchStats(); fetchCards(); fetchCoreTraffic()
  statsTimer = setInterval(fetchStats, 5000)
})
onUnmounted(() => { if (statsTimer) clearInterval(statsTimer) })
</script>
