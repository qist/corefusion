<template>
  <div style="padding:20px">
    <!-- Core Process Table -->
    <h3 style="margin-bottom:16px">{{ t('service.tabs.process') }}</h3>
    <el-table :data="processes" v-loading="loading" stripe size="small" style="margin-bottom:24px">
      <el-table-column prop="core_type" :label="t('nodeServer.coreType')" width="100">
        <template #default="{row}"><el-tag size="small">{{ row.core_type }}</el-tag></template>
      </el-table-column>
      <el-table-column :label="t('service.process.state')" width="80">
        <template #default="{row}">
          <el-tag :type="stateType(row.state)" size="small">{{ t('service.process.' + row.state, row.state) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="t('service.process.cpu')" width="60">
        <template #default="{row}">{{ row.cpu ? row.cpu.toFixed(1) + '%' : '-' }}</template>
      </el-table-column>
      <el-table-column :label="t('service.process.mem')" width="70">
        <template #default="{row}">{{ row.mem ? formatSize(row.mem) : '-' }}</template>
      </el-table-column>
      <el-table-column :label="t('service.process.version')" width="120">
        <template #default="{row}">
          <span v-if="!row.binary_installed" style="color:#f56c6c;cursor:pointer;font-size:12px" @click="showVersions(row)">
            {{ t('service.process.binaryNotInstalled') }}
          </span>
          <el-link v-else type="primary" @click="showVersions(row)" style="font-size:12px">{{ row.version || '...' }}</el-link>
        </template>
      </el-table-column>
      <el-table-column :label="t('service.process.pid')" width="60">
        <template #default="{row}">{{ row.pid || '-' }}</template>
      </el-table-column>
      <el-table-column :label="t('service.process.uptime')" width="80">
        <template #default="{row}">{{ row.state === 'running' ? formatUptime(row.uptime) : '-' }}</template>
      </el-table-column>
      <el-table-column :label="t('service.process.guardian')" width="200">
        <template #default="{row}">
          <div style="display:flex;gap:12px;align-items:center;font-size:12px">
            <el-tooltip :content="t('service.process.autoStartHint')">
              <span>
                <el-switch v-model="row.auto_start" size="small" @change="updateGuardian(row)" />
                <span style="margin-left:2px;color:#999">{{ t('service.process.autoStart') }}</span>
              </span>
            </el-tooltip>
            <el-tooltip :content="t('service.process.autoRestartHint')">
              <span>
                <el-switch v-model="row.auto_restart" size="small" @change="updateGuardian(row)" />
                <span style="margin-left:2px;color:#999">{{ t('service.process.autoRestart') }}</span>
              </span>
            </el-tooltip>
          </div>
        </template>
      </el-table-column>
      <el-table-column :label="t('common.actions')" width="180">
        <template #default="{row}">
          <el-button v-if="row.state === 'stopped' || row.state === 'crashed'" size="small" type="success" @click="handleStart(row)" :loading="row._loading">{{ t('service.start') }}</el-button>
          <el-button v-else-if="row.state === 'running'" size="small" type="danger" @click="handleStop(row)" :loading="row._loading">{{ t('service.stop') }}</el-button>
          <el-button v-else size="small" type="info" disabled>{{ t('common.loading') }}</el-button>
          <el-button size="small" @click="handleRestart(row)" :loading="row._loading" :disabled="row.state === 'unknown'">{{ t('service.restart') }}</el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- Proxy Binaries -->
    <h3 style="margin-bottom:16px">{{ t('nodeServer.proxyBinaries') }}</h3>
    <el-table :data="proxyRows" v-loading="proxyStatusLoading" stripe size="small" style="margin-bottom:24px">
      <el-table-column prop="name" :label="t('nodeServer.proxyType')" width="100">
        <template #default="{row}"><el-tag size="small">{{ row.name }}</el-tag></template>
      </el-table-column>
      <el-table-column :label="t('service.process.state')" width="80">
        <template #default="{row}">
          <el-tag :type="proxyStateType(row.state)" size="small">{{ t('service.process.' + row.state, row.state) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="t('service.process.cpu')" width="60">
        <template #default="{row}">{{ row.cpu ? row.cpu.toFixed(1) + '%' : '-' }}</template>
      </el-table-column>
      <el-table-column :label="t('service.process.mem')" width="70">
        <template #default="{row}">{{ row.mem ? formatSize(row.mem) : '-' }}</template>
      </el-table-column>
      <el-table-column :label="t('service.process.version')" width="140">
        <template #default="{row}">
          <span v-if="!row.installed" style="color:#f56c6c;cursor:pointer;font-size:12px" @click="showProxyVersions(row)">
            {{ t('service.process.binaryNotInstalled') }}
          </span>
          <el-link v-else type="primary" @click="showProxyVersions(row)" style="font-size:12px">{{ row.version || t('common.install') }}</el-link>
        </template>
      </el-table-column>
      <el-table-column :label="t('service.process.pid')" width="60">
        <template #default="{row}">{{ row.pid || '-' }}</template>
      </el-table-column>
      <el-table-column :label="t('service.process.uptime')" width="80">
        <template #default="{row}">{{ row.state === 'running' ? formatUptime(row.uptime) : '-' }}</template>
      </el-table-column>
      <el-table-column :label="t('service.process.guardian')" width="200">
        <template #default="{row}">
          <div style="display:flex;gap:12px;align-items:center;font-size:12px">
            <el-tooltip :content="t('service.process.autoStartHint')">
              <span>
                <el-switch v-model="row.auto_start" size="small" @change="updateProxyGuardian(row)" />
                <span style="margin-left:2px;color:#999">{{ t('service.process.autoStart') }}</span>
              </span>
            </el-tooltip>
            <el-tooltip :content="t('service.process.autoRestartHint')">
              <span>
                <el-switch v-model="row.auto_restart" size="small" @change="updateProxyGuardian(row)" />
                <span style="margin-left:2px;color:#999">{{ t('service.process.autoRestart') }}</span>
              </span>
            </el-tooltip>
          </div>
        </template>
      </el-table-column>
      <el-table-column :label="t('common.actions')" width="180">
        <template #default="{row}">
          <el-button v-if="row.state === 'stopped' || row.state === 'crashed'" size="small" type="success" @click="handleProxyStart(row)" :loading="row._loading" :disabled="!row.installed">{{ t('service.start') }}</el-button>
          <el-button v-else-if="row.state === 'running'" size="small" type="danger" @click="handleProxyStop(row)" :loading="row._loading">{{ t('service.stop') }}</el-button>
          <el-button v-else size="small" type="info" disabled>{{ t('common.loading') }}</el-button>
          <el-button size="small" @click="handleProxyRestart(row)" :loading="row._loading" :disabled="row.state === 'unknown' || !row.installed">{{ t('service.restart') }}</el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- Version Select Dialog -->
    <el-dialog v-model="versionVisible" :title="t('service.process.selectVersion')" width="620">
      <div v-loading="releasesLoading" style="min-height:100px">
        <div v-if="releases.length === 0 && !releasesLoading" style="text-align:center;color:#999;padding:20px">{{ t('common.noData') }}</div>
        <div style="display:flex;flex-wrap:wrap;gap:8px">
          <div v-for="r in releases" :key="r.version"
               style="width:calc(16.66% - 8px);min-width:88px;padding:8px;border:1px solid #dcdfe6;border-radius:6px;cursor:pointer;text-align:center;transition:all .2s"
               :style="{borderColor: r.prerelease ? '#67c23a' : '#409eff', background: r.prerelease ? '#f0f9eb' : '#ecf5ff'}"
               @click="handleDownloadVersion(r)">
            <div style="font-size:11px;margin-bottom:2px" :style="{color: r.prerelease ? '#67c23a' : '#409eff'}">
              {{ r.prerelease ? 'Pre' : 'Rel' }}
            </div>
            <div style="font-size:12px;font-weight:500;color:#303133">{{ r.version }}</div>
          </div>
        </div>
      </div>
    </el-dialog>

    <!-- Proxy Version Select Dialog -->
    <el-dialog v-model="proxyVersionVisible" :title="t('service.process.selectVersion')" width="620">
      <div v-loading="proxyReleasesLoading" style="min-height:100px">
        <div v-if="proxyReleases.length === 0 && !proxyReleasesLoading" style="text-align:center;color:#999;padding:20px">{{ t('common.noData') }}</div>
        <div style="display:flex;flex-wrap:wrap;gap:8px">
          <div v-for="r in proxyReleases" :key="r.tag_name"
               style="width:calc(16.66% - 8px);min-width:88px;padding:8px;border:1px solid #dcdfe6;border-radius:6px;cursor:pointer;text-align:center;transition:all .2s"
               :style="{borderColor: r.prerelease ? '#67c23a' : '#409eff', background: r.prerelease ? '#f0f9eb' : '#ecf5ff'}"
               @click="handleProxyDownloadVersion(r)">
            <div style="font-size:11px;margin-bottom:2px" :style="{color: r.prerelease ? '#67c23a' : '#409eff'}">
              {{ r.prerelease ? 'Pre' : 'Rel' }}
            </div>
            <div style="font-size:12px;font-weight:500;color:#303133">{{ r.tag_name }}</div>
          </div>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import client from '../../api/client'
import { getProxyStatus, startProxy, stopProxy, restartProxy, updateProxyGuardian as updateProxyGuardianApi, getProxyReleases, downloadProxyVersion } from '../../api/nodeServer'

const { t } = useI18n()
const props = defineProps({ serverId: { type: Number, required: true } })

const processes = ref([])
const loading = ref(false)
let timer = null

// Version dialog
const versionVisible = ref(false)
const releasesLoading = ref(false)
const releases = ref([])
const selectedService = ref(null)

// Proxy binaries
const proxyRows = ref([])
const proxyStatusLoading = ref(false)

// Proxy version dialog
const proxyVersionVisible = ref(false)
const proxyReleasesLoading = ref(false)
const proxyReleases = ref([])
const selectedProxyRow = ref(null)

function stateType(state) {
  if (state === 'running') return 'success'
  if (state === 'crashed') return 'danger'
  return 'info'
}

function proxyStateType(state) {
  if (state === 'running') return 'success'
  if (state === 'crashed') return 'danger'
  return 'info'
}

function formatUptime(seconds) {
  if (!seconds) return '-'
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = seconds % 60
  if (h > 0) return `${h}h ${m}m`
  if (m > 0) return `${m}m ${s}s`
  return `${s}s`
}

function formatSize(bytes) {
  if (!bytes) return '-'
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1048576) return (bytes / 1024).toFixed(0) + ' KB'
  return (bytes / 1048576).toFixed(1) + ' MB'
}

async function fetchData() {
  loading.value = true
  try {
    const data = await client.get(`/node-servers/${props.serverId}/services/process-status`)
    processes.value = (data || []).map(s => ({ ...s, _loading: false }))
  } catch (e) {
    processes.value = []
  } finally {
    loading.value = false
  }
}

async function handleStart(row) {
  row._loading = true
  try {
    await client.post(`/node-servers/${props.serverId}/services/${row.service_id}/start`)
    ElMessage.success(t('common.success'))
    fetchData()
  } catch (e) {} finally { row._loading = false }
}

async function handleStop(row) {
  await ElMessageBox.confirm(t('service.process.stopConfirm', { name: row.name || row.core_type }), t('common.confirm'))
  row._loading = true
  try {
    await client.post(`/node-servers/${props.serverId}/services/${row.service_id}/stop`)
    ElMessage.success(t('common.success'))
    fetchData()
  } catch (e) {} finally { row._loading = false }
}

async function handleRestart(row) {
  row._loading = true
  try {
    await client.post(`/node-servers/${props.serverId}/services/${row.service_id}/restart`)
    ElMessage.success(t('common.success'))
    fetchData()
  } catch (e) {} finally { row._loading = false }
}

async function updateGuardian(row) {
  try {
    await client.put(`/node-servers/${props.serverId}/services/${row.service_id}/guardian`, {
      auto_start: row.auto_start,
      auto_restart: row.auto_restart
    })
  } catch (e) {
    row.auto_start = !row.auto_start
    row.auto_restart = !row.auto_restart
  }
}

// Version management
async function showVersions(row) {
  selectedService.value = row
  versionVisible.value = true
  releasesLoading.value = true
  try {
    const data = await client.get(`/node-servers/${props.serverId}/binaries/releases`, { params: { core_type: row.core_type } })
    const items = Array.isArray(data) ? data : (data.items || [])
    releases.value = items.map(r => ({
      ...r,
      prerelease: r.prerelease !== undefined ? r.prerelease : isOddVersion(r.version)
    }))
  } catch (e) {
    releases.value = []
  } finally {
    releasesLoading.value = false
  }
}

function isOddVersion(v) {
  const parts = (v || '').replace(/^v/, '').split('.')
  const last = parseInt(parts[parts.length - 1])
  return !isNaN(last) && last % 2 !== 0
}

async function handleDownloadVersion(release) {
  await ElMessageBox.confirm(t('service.process.downloadConfirm', { version: release.version }), t('common.confirm'))
  versionVisible.value = false
  try {
    await client.post(`/node-servers/${props.serverId}/binaries/install`, { core_type: selectedService.value.core_type, version: release.version })
    ElMessage.success(t('common.success'))
    fetchData()
  } catch (e) {}
}

// Proxy binaries
async function loadProxyStatuses() {
  proxyStatusLoading.value = true
  try {
    const data = await getProxyStatus(props.serverId)
    const rows = []
    for (const pt of ['caddy', 'nginx']) {
      const info = data?.[pt] || {}
      const guardian = data?.[`${pt}_guardian`] || {}
      const startedAt = guardian.started_at
      let uptime = 0
      if (startedAt && info.status === 'running') {
        const start = new Date(startedAt).getTime()
        if (!isNaN(start)) {
          uptime = Math.floor((Date.now() - start) / 1000)
        }
      }
      rows.push({
        type: info.type || pt,
        name: info.name || pt.charAt(0).toUpperCase() + pt.slice(1),
        installed: info.installed || false,
        version: info.version || '',
        path: info.path || '',
        managed: info.managed || false,
        state: info.status || 'unknown',
        pid: guardian.pid || 0,
        cpu: guardian.cpu || 0,
        mem: guardian.mem || 0,
        uptime,
        auto_start: guardian.auto_start !== undefined ? guardian.auto_start : true,
        auto_restart: guardian.auto_restart !== undefined ? guardian.auto_restart : false,
        max_restarts: guardian.max_restarts || 5,
        current_restarts: guardian.current_restarts || 0,
        _loading: false,
      })
    }
    proxyRows.value = rows
  } catch (e) {
    proxyRows.value = []
  } finally {
    proxyStatusLoading.value = false
  }
}

async function handleProxyStart(row) {
  row._loading = true
  try {
    await startProxy(props.serverId, row.type)
    ElMessage.success(t('common.success'))
    loadProxyStatuses()
  } catch (e) {} finally { row._loading = false }
}

async function handleProxyStop(row) {
  await ElMessageBox.confirm(t('service.process.stopConfirm', { name: row.name }), t('common.confirm'))
  row._loading = true
  try {
    await stopProxy(props.serverId, row.type)
    ElMessage.success(t('common.success'))
    loadProxyStatuses()
  } catch (e) {} finally { row._loading = false }
}

async function handleProxyRestart(row) {
  row._loading = true
  try {
    await restartProxy(props.serverId, row.type)
    ElMessage.success(t('common.success'))
    loadProxyStatuses()
  } catch (e) {} finally { row._loading = false }
}

async function updateProxyGuardian(row) {
  try {
    await updateProxyGuardianApi(props.serverId, row.type, {
      auto_start: row.auto_start,
      auto_restart: row.auto_restart,
      max_restarts: row.max_restarts
    })
  } catch (e) {
    row.auto_start = !row.auto_start
    row.auto_restart = !row.auto_restart
  }
}

// Proxy version management
async function showProxyVersions(row) {
  selectedProxyRow.value = row
  proxyVersionVisible.value = true
  proxyReleasesLoading.value = true
  try {
    const data = await getProxyReleases(props.serverId, row.type)
    const items = Array.isArray(data) ? data : (data.items || [])
    proxyReleases.value = items.map(r => ({
      ...r,
      prerelease: r.prerelease !== undefined ? r.prerelease : false
    }))
  } catch (e) {
    proxyReleases.value = []
  } finally {
    proxyReleasesLoading.value = false
  }
}

async function handleProxyDownloadVersion(release) {
  await ElMessageBox.confirm(t('service.process.downloadConfirm', { version: release.tag_name }), t('common.confirm'))
  proxyVersionVisible.value = false
  try {
    const downloadURL = release.assets && release.assets.length > 0 ? release.assets[0].browser_download_url : ''
    await downloadProxyVersion(props.serverId, {
      type: selectedProxyRow.value.type,
      download_url: downloadURL,
      tag_name: release.tag_name
    })
    ElMessage.success(t('common.success'))
    loadProxyStatuses()
  } catch (e) {}
}

function startTimer() {
  stopTimer()
  timer = setInterval(() => {
    fetchData()
    loadProxyStatuses()
  }, 5000)
}

function stopTimer() {
  if (timer) { clearInterval(timer); timer = null }
}

watch(() => props.serverId, () => {
  fetchData()
  loadProxyStatuses()
})

onMounted(() => {
  fetchData()
  loadProxyStatuses()
  startTimer()
})

onUnmounted(() => {
  stopTimer()
})
</script>
