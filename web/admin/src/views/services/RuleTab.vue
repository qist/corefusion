<template>
  <div>
    <!-- Sources -->
    <div style="display:flex;justify-content:space-between;margin-bottom:16px">
      <h3>{{ t('service.rules.sources') }}</h3>
      <div>
        <el-button @click="showAddSource">{{ t('service.rules.addSource') }}</el-button>
        <el-button type="primary" @click="handleSyncAll" :loading="syncAllLoading">{{ t('service.rules.syncAll') }}</el-button>
        <el-button type="success" @click="handlePushAll" :loading="pushAllLoading">{{ t('service.rules.pushAll') }}</el-button>
      </div>
    </div>
    <el-card style="margin-bottom:20px">
      <el-table v-if="!isMobile" :data="sources" v-loading="sourcesLoading" stripe>
        <el-table-column prop="name" :label="t('common.name')" width="200" />
        <el-table-column prop="repo_url" label="Repo" />
        <el-table-column prop="sync_cron" :label="t('service.rules.syncCron')" width="140" />
        <el-table-column :label="t('service.rules.scheduleEnabled')" width="100">
          <template #default="{row}">
            <el-switch v-model="row.enabled" @change="handleToggleSchedule(row)" />
          </template>
        </el-table-column>
        <el-table-column prop="last_sync_tag" label="Version" width="140" />
        <el-table-column :label="t('common.status')" width="100">
          <template #default="{row}">
            <el-tag :type="row.status==='syncing'?'warning':'success'" size="small">{{ row.status }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('common.actions')" width="300">
          <template #default="{row}">
            <div style="display:flex;gap:2px;align-items:center;white-space:nowrap">
              <el-button size="small" :disabled="row.repo_url.startsWith('http')" @click="showFetchAssets(row)">{{ t('service.rules.browseFiles') }}</el-button>
              <el-button size="small" type="success" @click="handleSyncSource(row)" :loading="row._syncing">{{ t('service.rules.sync') }}</el-button>
              <el-button size="small" @click="showEditSource(row)">{{ t('common.edit') }}</el-button>
              <el-button size="small" type="danger" @click="handleDeleteSource(row)">{{ t('common.delete') }}</el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>
      <MobileCards v-else :rows="sources" title-field="name" :fields="ruleSourceCardFields">
        <template #actions="{ row }">
          <el-switch v-model="row.enabled" @change="handleToggleSchedule(row)" />
          <el-button size="small" :disabled="row.repo_url.startsWith('http')" @click="showFetchAssets(row)">{{ t('service.rules.browseFiles') }}</el-button>
          <el-button size="small" type="success" @click="handleSyncSource(row)" :loading="row._syncing">{{ t('service.rules.sync') }}</el-button>
          <el-button size="small" @click="showEditSource(row)">{{ t('common.edit') }}</el-button>
          <el-button size="small" type="danger" @click="handleDeleteSource(row)">{{ t('common.delete') }}</el-button>
        </template>
      </MobileCards>
    </el-card>

    <!-- Selected Files -->
    <div style="display:flex;justify-content:space-between;margin-bottom:16px">
      <h3>{{ t('service.rules.selectedFiles') }}</h3>
      <el-button type="primary" plain @click="showAddFile">{{ t('service.rules.addFile') }}</el-button>
    </div>
    <el-card>
      <el-table v-if="!isMobile" :data="files" v-loading="filesLoading" stripe>
        <el-table-column prop="name" label="File" min-width="250" />
        <el-table-column :label="t('service.rules.fileSize')" width="120">
          <template #default="{row}">{{ row.file_size ? formatSize(row.file_size) : '-' }}</template>
        </el-table-column>
        <el-table-column prop="version" label="Version" width="140" />
        <el-table-column :label="t('service.rules.lastUpdated')" width="180">
          <template #default="{row}">{{ row.last_updated && row.last_updated !== '0001-01-01T00:00:00Z' ? new Date(row.last_updated).toLocaleString() : '-' }}</template>
        </el-table-column>
        <el-table-column :label="t('common.status')" width="100">
          <template #default="{row}">
            <el-tag :type="row.status==='available'?'success':row.status==='downloading'?'warning':'danger'" size="small">
              {{ row.status }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('common.actions')" width="200">
          <template #default="{row}">
            <div style="display:flex;gap:2px;align-items:center;white-space:nowrap">
              <el-button size="small" type="primary" @click="handleDownload(row)" :loading="row._loading">{{ t('service.rules.download') }}</el-button>
              <el-button size="small" type="danger" @click="handleDeleteFile(row)">{{ t('common.delete') }}</el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>
      <MobileCards v-else :rows="files" title-field="name" :fields="ruleFileCardFields">
        <template #actions="{ row }">
          <el-button size="small" type="primary" @click="handleDownload(row)" :loading="row._loading">{{ t('service.rules.download') }}</el-button>
          <el-button size="small" type="danger" @click="handleDeleteFile(row)">{{ t('common.delete') }}</el-button>
        </template>
      </MobileCards>
    </el-card>

    <!-- Add/Edit Source Dialog -->
    <el-dialog v-model="sourceDialogVisible" :title="editingSource ? t('common.edit') : t('service.rules.addSource')" width="600">
      <el-form :model="sourceForm" label-width="100px">
        <el-form-item :label="t('common.name')"><el-input v-model="sourceForm.name" /></el-form-item>
        <el-form-item label="Repo URL">
          <el-input v-model="sourceForm.repo_url" :placeholder="t('service.rules.repoUrlPlaceholder')" @blur="fetchAssetsForSource" />
          <div style="font-size:11px;color:#999;margin-top:4px">{{ t('service.rules.repoUrlHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('service.rules.syncCron')">
          <el-select v-model="sourceForm.sync_cron" filterable allow-create style="width:100%">
            <el-option :label="t('common.disabled')" value="" />
            <el-option :label="t('service.rules.cronEvery6h')" value="0 */6 * * *" />
            <el-option :label="t('service.rules.cronEvery12h')" value="0 */12 * * *" />
            <el-option :label="t('service.rules.cronDaily3am')" value="0 3 * * *" />
            <el-option :label="t('service.rules.cronDailyMidnight')" value="0 0 * * *" />
            <el-option :label="t('service.rules.cronWeekly')" value="0 0 * * 0" />
            <el-option label="@every 1h" value="@every 1h" />
            <el-option label="@every 6h" value="@every 6h" />
            <el-option label="@every 12h" value="@every 12h" />
            <el-option label="@every 24h" value="@every 24h" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="!editingSource && !isDirectUrl" :label="t('service.rules.selectFiles')">
          <div v-if="sourceAssetsLoading" style="color:#999">{{ t('common.loading') }}</div>
          <div v-else-if="sourceAssets.length === 0" style="color:#999;font-size:12px">{{ t('service.rules.enterRepoHint') }}</div>
          <el-checkbox-group v-model="sourceSelectedFiles" v-else>
            <div style="display:flex;flex-wrap:wrap;gap:4px">
              <el-checkbox v-for="a in sourceAssets" :key="a.name" :label="a.name" border size="small">
                {{ a.name }} <span style="color:#999;font-size:11px">({{ formatSize(a.size) }})</span>
              </el-checkbox>
            </div>
          </el-checkbox-group>
        </el-form-item>
        <el-form-item v-if="!editingSource && isDirectUrl" :label="t('service.rules.directUrlInfo')">
          <span style="font-size:12px;color:#67c23a">{{ t('service.rules.directUrlAutoFile') }}</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="sourceDialogVisible=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="sourceSaving" @click="handleSaveSource">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- Browse Assets Dialog -->
    <el-dialog v-model="assetsDialogVisible" :title="t('service.rules.selectFiles')" width="600">
      <div style="margin-bottom:12px">
        <el-tag v-if="assetsTag">{{ t('service.rules.latestVersion') }}: {{ assetsTag }}</el-tag>
      </div>
      <el-table v-if="!isMobile" ref="assetsTableRef" :data="assets" v-loading="assetsLoading" stripe @selection-change="handleAssetSelect" max-height="400">
        <el-table-column type="selection" width="50" />
        <el-table-column prop="name" label="File" />
        <el-table-column :label="t('service.rules.fileSize')" width="100">
          <template #default="{row}">{{ formatSize(row.size) }}</template>
        </el-table-column>
      </el-table>
      <!-- 手机端：selection 列以行内勾选框替代，仍写入 selectedAssets -->
      <MobileCards v-else :rows="assets" title-field="name" :fields="ruleAssetCardFields">
        <template #actions="{ row }">
          <el-checkbox :model-value="selectedAssets.includes(row.name)" @change="(v) => toggleAsset(row.name, v)" />
        </template>
      </MobileCards>
      <template #footer>
        <el-button @click="assetsDialogVisible=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="selectSaving" @click="handleConfirmSelect">{{ t('service.rules.addSelected') }}</el-button>
      </template>
    </el-dialog>

    <!-- Add File Dialog（手动添加任意 URL 规则文件，如 sing-box .srs 分支 raw 文件） -->
    <el-dialog v-model="addFileDialogVisible" :title="t('service.rules.addFile')" width="600">
      <el-form :model="addFileForm" label-width="110px">
        <el-form-item :label="t('service.rules.source')">
          <el-select v-model="addFileForm.source_id" style="width:100%">
            <el-option v-for="s in sources" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('service.rules.fileName')">
          <el-input v-model="addFileForm.name" :placeholder="t('service.rules.fileNamePlaceholder')" />
        </el-form-item>
        <el-form-item label="URL">
          <el-input v-model="addFileForm.url" type="textarea" :rows="2" :placeholder="t('service.rules.fileUrlPlaceholder')" />
        </el-form-item>
        <div style="font-size:12px;color:#999;margin-top:-6px">{{ t('service.rules.addFileHint') }}</div>
      </el-form>
      <template #footer>
        <el-button @click="addFileDialogVisible=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="addFileSaving" @click="handleSaveFile">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import client from '../../api/client'
import { useIsMobile } from '../../composables/useIsMobile'
import MobileCards from '../../components/MobileCards.vue'

const { t } = useI18n()
const { isMobile } = useIsMobile()

// Sources
const sources = ref([])
const sourcesLoading = ref(false)
const syncAllLoading = ref(false)
const pushAllLoading = ref(false)
const sourceDialogVisible = ref(false)
const sourceSaving = ref(false)
const editingSource = ref(null)
const sourceForm = ref({ name: '', repo_url: '', sync_cron: '0 3 * * *' })
const sourceAssets = ref([])
const sourceAssetsLoading = ref(false)
const sourceSelectedFiles = ref([])
let lastFetchedRepo = ''

const isDirectUrl = computed(() => {
  const url = (sourceForm.value.repo_url || '').trim()
  return url.startsWith('http://') || url.startsWith('https://')
})

// Files
const files = ref([])
const filesLoading = ref(false)

// Assets
const assetsDialogVisible = ref(false)
const assetsLoading = ref(false)
const assets = ref([])
const assetsTag = ref('')
const selectedSourceId = ref(null)
const selectedAssets = ref([])
const assetsTableRef = ref(null)
const selectSaving = ref(false)

function formatSize(bytes) {
  if (!bytes) return '-'
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1048576) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / 1048576).toFixed(1) + ' MB'
}

// 手机端卡片字段（与桌面表格列对应）
const ruleSourceCardFields = computed(() => [
  { label: 'Repo', text: (row) => row.repo_url || '-' },
  { label: t('service.rules.syncCron'), text: (row) => row.sync_cron || '-' },
  { label: 'Version', text: (row) => row.last_sync_tag || '-' },
  { label: t('common.status'), text: (row) => row.status || '-' },
])
const ruleFileCardFields = computed(() => [
  { label: t('service.rules.fileSize'), text: (row) => row.file_size ? formatSize(row.file_size) : '-' },
  { label: 'Version', text: (row) => row.version || '-' },
  { label: t('service.rules.lastUpdated'), text: (row) => row.last_updated && row.last_updated !== '0001-01-01T00:00:00Z' ? new Date(row.last_updated).toLocaleString() : '-' },
  { label: t('common.status'), text: (row) => row.status || '-' },
])
const ruleAssetCardFields = computed(() => [
  { label: t('service.rules.fileSize'), text: (row) => formatSize(row.size) },
])
// 手机端卡片勾选（替代 el-table selection 列）
function toggleAsset(name, checked) {
  if (checked) {
    if (!selectedAssets.value.includes(name)) selectedAssets.value.push(name)
  } else {
    selectedAssets.value = selectedAssets.value.filter(n => n !== name)
  }
}

// --- Sources ---
async function fetchSources() {
  sourcesLoading.value = true
  try {
    const data = await client.get('/geo-rules/sources')
    sources.value = (data || []).map(s => ({ ...s, _syncing: false }))
    // Auto-seed defaults if this node has no rule sources yet
    if (sources.value.length === 0) {
      try {
        await client.post('/geo-rules/seed')
        // Re-fetch after seed
        const reData = await client.get('/geo-rules/sources')
        sources.value = (reData || []).map(s => ({ ...s, _syncing: false }))
        // Re-fetch files since seed creates default file records (geoip.dat, geosite.dat)
        fetchFiles()
      } catch (e) {
        // seed failed silently, user can add manually
      }
    }
  } finally { sourcesLoading.value = false }
}

function showAddSource() {
  editingSource.value = null
  sourceForm.value = { name: '', repo_url: '', sync_cron: '' }
  sourceAssets.value = []
  sourceSelectedFiles.value = []
  lastFetchedRepo = ''
  sourceDialogVisible.value = true
}

function showEditSource(row) {
  editingSource.value = row
  sourceForm.value = { name: row.name, repo_url: row.repo_url, sync_cron: row.sync_cron || '' }
  sourceAssets.value = []
  sourceSelectedFiles.value = []
  sourceDialogVisible.value = true
}

async function fetchAssetsForSource() {
  const repo = sourceForm.value.repo_url.trim()
  if (!repo || repo === lastFetchedRepo || editingSource.value) return
  if (repo.startsWith('http://') || repo.startsWith('https://')) return
  lastFetchedRepo = repo
  sourceAssetsLoading.value = true
  sourceAssets.value = []
  sourceSelectedFiles.value = []
  try {
    // Fetch releases via backend API (with GitHub acceleration)
    const data = await client.get('/geo-rules/github-assets', { params: { repo } })
    sourceAssets.value = (data.assets || []).map(a => ({ name: a.name, size: a.size, download_url: a.download_url }))
    sourceSelectedFiles.value = []
  } catch (e) { ElMessage.error(t('common.failed')) } finally { sourceAssetsLoading.value = false }
}

async function handleSaveSource() {
  sourceSaving.value = true
  try {
    if (editingSource.value) {
      await client.put(`/geo-rules/sources/${editingSource.value.id}`, sourceForm.value)
    } else {
      const src = await client.post('/geo-rules/sources', sourceForm.value)
      // If files were selected, save them
      if (sourceSelectedFiles.value.length > 0 && src && src.id) {
        await client.post(`/geo-rules/sources/${src.id}/select`, {
          files: sourceSelectedFiles.value
        })
      }
    }
    ElMessage.success(t('common.success'))
    sourceDialogVisible.value = false
    fetchSources()
    fetchFiles()
  } finally { sourceSaving.value = false }
}

async function handleDeleteSource(row) {
  await ElMessageBox.confirm(t('service.rules.deleteSourceConfirm', { name: row.name }), t('common.confirm'))
  await client.delete(`/geo-rules/sources/${row.id}`)
  ElMessage.success(t('common.success'))
  fetchSources()
  fetchFiles()
}

async function pollSyncStatus(jobKey, callback) {
  const poll = async () => {
    try {
      const job = await client.get(`/geo-rules/sync-status/${jobKey}`)
      if (job.status === 'running') {
        setTimeout(poll, 1500)
      } else {
        callback(job)
      }
    } catch (e) {
      callback({ status: 'error', error: 'poll failed' })
    }
  }
  poll()
}

async function handleSyncSource(row) {
  row._syncing = true
  try {
    const { job_key } = await client.post(`/geo-rules/sources/${row.id}/sync`)
    pollSyncStatus(job_key, (job) => {
      row._syncing = false
      if (job.status === 'done' && job.results && job.results.length > 0) {
        const r = job.results[0]
        ElMessage.success(t('service.rules.syncDone', { downloaded: r.downloaded, total: r.total, tag: r.tag }))
      } else {
        ElMessage.error(job.error || t('common.failed'))
      }
      fetchSources()
      fetchFiles()
    })
  } catch (e) { row._syncing = false }
}

async function handleToggleSchedule(row) {
  try {
    await client.put(`/geo-rules/sources/${row.id}/schedule`, {
      enabled: row.enabled,
      sync_cron: row.sync_cron || ''
    })
    ElMessage.success(t('common.success'))
  } catch (e) {
    row.enabled = !row.enabled
  }
}

async function handleSyncAll() {
  syncAllLoading.value = true
  try {
    const { job_key } = await client.post('/geo-rules/sync-all')
    pollSyncStatus(job_key, (job) => {
      syncAllLoading.value = false
      if (job.status === 'done' && job.results) {
        const total = job.results.reduce((sum, r) => sum + (r.downloaded || 0), 0)
        ElMessage.success(t('service.rules.syncAllDone', { count: total }))
      } else {
        ElMessage.error(job.error || t('common.failed'))
      }
      fetchSources()
      fetchFiles()
    })
  } catch (e) { syncAllLoading.value = false }
}

// --- Browse & Select Assets ---
async function showFetchAssets(row) {
  selectedSourceId.value = row.id
  assetsDialogVisible.value = true
  assetsLoading.value = true
  selectedAssets.value = []
  try {
    const data = await client.get(`/geo-rules/sources/${row.id}/assets`)
    assets.value = data.assets || []
    assetsTag.value = data.tag || ''
    // Pre-select files already added
    const existingNames = new Set((files.value || []).filter(f => f.source_id === row.id).map(f => f.name))
    selectedAssets.value = [...existingNames]
    // Toggle table row selection after DOM update
    setTimeout(() => {
      if (assetsTableRef.value) {
        assets.value.forEach(a => {
          if (existingNames.has(a.name)) {
            assetsTableRef.value.toggleRowSelection(a, true)
          }
        })
      }
    }, 0)
  } catch (e) {
    assets.value = []
  } finally { assetsLoading.value = false }
}

function handleAssetSelect(selection) {
  selectedAssets.value = selection.map(a => a.name)
}

async function handleConfirmSelect() {
  if (selectedAssets.value.length === 0) return
  const fileNames = selectedAssets.value
  selectSaving.value = true
  try {
    await client.post(`/geo-rules/sources/${selectedSourceId.value}/select`, {
      files: fileNames
    })
    ElMessage.success(t('common.success'))
    assetsDialogVisible.value = false
    fetchFiles()
  } finally { selectSaving.value = false }
}

// --- Add File（手动添加任意 URL 规则文件，如 sing-box .srs） ---
const addFileDialogVisible = ref(false)
const addFileSaving = ref(false)
const addFileForm = ref({ source_id: null, name: '', url: '' })

function showAddFile() {
  addFileForm.value = { source_id: sources.value[0]?.id ?? null, name: '', url: '' }
  addFileDialogVisible.value = true
}

async function handleSaveFile() {
  if (!addFileForm.value.source_id) return
  addFileSaving.value = true
  try {
    await client.post(`/geo-rules/sources/${addFileForm.value.source_id}/files`, {
      name: addFileForm.value.name.trim(),
      url: addFileForm.value.url.trim()
    })
    ElMessage.success(t('service.rules.addFileStarted'))
    addFileDialogVisible.value = false
    fetchFiles()
  } catch (e) { ElMessage.error(e?.response?.data?.message || t('common.failed')) } finally { addFileSaving.value = false }
}

// --- Files ---
async function fetchFiles() {
  filesLoading.value = true
  try {
    const data = await client.get('/geo-rules/files')
    files.value = (data || []).map(f => ({ ...f, _loading: false }))
  } finally { filesLoading.value = false }
}

async function handleDownload(row) {
  row._loading = true
  try {
    await client.post(`/geo-rules/files/${row.id}/download`)
    ElMessage.success(t('common.success'))
    fetchFiles()
  } catch (e) { ElMessage.error(t('common.failed')) } finally { row._loading = false }
}

async function handleDeleteFile(row) {
  await ElMessageBox.confirm(t('service.rules.deleteConfirm', { name: row.name }), t('common.confirm'))
  await client.delete(`/geo-rules/files/${row.id}`)
  ElMessage.success(t('common.success'))
  fetchFiles()
}

async function handlePushAll() {
  pushAllLoading.value = true
  try {
    await client.post('/geo-rules/push-all')
    ElMessage.success(t('common.success'))
  } catch (e) { ElMessage.error(t('common.failed')) } finally { pushAllLoading.value = false }
}


onMounted(() => {
  fetchSources().then(() => fetchFiles())
})
</script>
