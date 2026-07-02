<template>
  <div>
    <el-card>
      <el-table :data="services" v-loading="loading" stripe>
        <el-table-column prop="name" :label="t('common.name')" width="120" />
        <el-table-column prop="core_type" :label="t('service.coreType')" width="80">
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
            <el-button v-if="row.state !== 'running'" size="small" type="success" @click="handleStart(row)" :loading="row._loading">{{ t('service.start') }}</el-button>
            <el-button v-else size="small" type="danger" @click="handleStop(row)" :loading="row._loading">{{ t('service.stop') }}</el-button>
            <el-button size="small" @click="handleRestart(row)" :loading="row._loading">{{ t('service.restart') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

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
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import client from '../../api/client'

const { t } = useI18n()
const services = ref([])
const loading = ref(false)
let timer = null

// Version
const versionVisible = ref(false)
const releasesLoading = ref(false)
const releases = ref([])
const selectedService = ref(null)

function stateType(state) {
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
    const data = await client.get('/node-servers/1/services/process-status')
    services.value = (data || []).map(s => ({ ...s, _loading: false }))
  } catch (e) {} finally {
    loading.value = false
  }
}

async function handleStart(row) {
  row._loading = true
  try {
    await client.post(`/node-servers/1/services/${row.service_id}/start`)
    ElMessage.success(t('common.success'))
    fetchData()
  } catch (e) {} finally { row._loading = false }
}

async function handleStop(row) {
  await ElMessageBox.confirm(t('service.process.stopConfirm', { name: row.name }), t('common.confirm'))
  row._loading = true
  try {
    await client.post(`/node-servers/1/services/${row.service_id}/stop`)
    ElMessage.success(t('common.success'))
    fetchData()
  } catch (e) {} finally { row._loading = false }
}

async function handleRestart(row) {
  row._loading = true
  try {
    await client.post(`/node-servers/1/services/${row.service_id}/restart`)
    ElMessage.success(t('common.success'))
    fetchData()
  } catch (e) {} finally { row._loading = false }
}

async function updateGuardian(row) {
  try {
    await client.put(`/node-servers/1/services/${row.service_id}/guardian`, {
      auto_start: row.auto_start,
      auto_restart: row.auto_restart
    })
  } catch (e) {
    row.auto_start = !row.auto_start
    row.auto_restart = !row.auto_restart
  }
}

// Version
async function showVersions(row) {
  selectedService.value = row
  versionVisible.value = true
  releasesLoading.value = true
  try {
    const data = await client.get('/node-servers/1/binaries/releases', { params: { core_type: row.core_type } })
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
    await client.post('/node-servers/1/binaries/install', { core_type: selectedService.value.core_type, version: release.version })
    ElMessage.success(t('common.success'))
    fetchData()
  } catch (e) {}
}

onMounted(() => {
  fetchData()
  timer = setInterval(fetchData, 5000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>
