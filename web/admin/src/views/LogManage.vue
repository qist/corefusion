<template>
  <div>
    <!-- Status Overview -->
    <el-row :gutter="16" style="margin-bottom:20px">
      <el-col :span="6">
        <el-card shadow="hover">
          <div class="stat-card">
            <div class="stat-value">{{ status.total_policies }}</div>
            <div class="stat-label">{{ t('logManage.totalPolicies') }}</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover">
          <div class="stat-card">
            <div class="stat-value text-success">{{ status.enabled_count }}</div>
            <div class="stat-label">{{ t('logManage.enabledPolicies') }}</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover">
          <div class="stat-card">
            <div class="stat-value text-primary">{{ status.scheduled_count }}</div>
            <div class="stat-label">{{ t('logManage.scheduledJobs') }}</div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover">
          <div class="stat-card">
            <div class="stat-value text-warning">{{ status.total_jobs }}</div>
            <div class="stat-label">{{ t('logManage.totalRuns') }}</div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- Header -->
    <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:16px">
      <h2 style="margin:0">{{ t('logManage.title') }}</h2>
      <div style="display:flex;gap:10px">
        <el-button @click="handleDiscover" :loading="discovering">
          <el-icon><Search /></el-icon>{{ t('logManage.discover') }}
        </el-button>
        <el-button type="success" @click="handleSync" :loading="syncing">
          <el-icon><Refresh /></el-icon>{{ t('logManage.syncPolicies') }}
        </el-button>
        <el-button type="primary" @click="showCreate">
          <el-icon><Plus /></el-icon>{{ t('logManage.create') }}
        </el-button>
      </div>
    </div>

    <!-- Discovered Logs (collapsible) -->
    <el-collapse-transition>
      <el-card v-if="discoveredLogs.length > 0" style="margin-bottom:16px">
        <template #header>
          <div style="display:flex;justify-content:space-between;align-items:center">
            <span>{{ t('logManage.discoveredLogs') }} ({{ discoveredLogs.length }})</span>
            <el-button text size="small" @click="discoveredLogs = []">{{ t('common.close') }}</el-button>
          </div>
        </template>
        <el-table :data="discoveredLogs" size="small" stripe>
          <el-table-column prop="source_type" :label="t('logManage.sourceType')" width="100">
            <template #default="{row}">
              <el-tag :type="row.source_type === 'nginx' ? 'success' : 'primary'" size="small">
                {{ row.source_type }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="domain" :label="t('logManage.domain')" width="180" show-overflow-tooltip>
            <template #default="{row}">
              {{ row.domain || '-' }}
            </template>
          </el-table-column>
          <el-table-column prop="log_path" :label="t('logManage.logPath')" />
          <el-table-column prop="log_format" :label="t('logManage.logFormat')" width="80">
            <template #default="{row}">
              {{ row.log_format || '-' }}
            </template>
          </el-table-column>
          <el-table-column :label="t('common.actions')" width="160" align="center">
            <template #default="{row}">
              <el-button
                v-if="!row.has_policy"
                type="primary"
                size="small"
                :loading="addingPath === row.log_path"
                @click="handleAddFromDiscover(row)"
              >
                <el-icon><Plus /></el-icon>{{ t('logManage.addToPolicy') }}
              </el-button>
              <el-tag v-else type="success" size="small">
                <el-icon><CircleCheck /></el-icon>{{ t('logManage.alreadyInPolicy') }}
              </el-tag>
            </template>
          </el-table-column>
        </el-table>
      </el-card>
    </el-collapse-transition>

    <!-- Policies Table -->
    <el-card>
      <el-table :data="policies" v-loading="loading" stripe>
        <el-table-column prop="id" :label="t('common.id')" width="60" />
        <el-table-column prop="name" :label="t('common.name')" min-width="140" />
        <el-table-column prop="source_type" :label="t('logManage.sourceType')" width="100">
          <template #default="{row}">
            <el-tag :type="row.source_type === 'nginx' ? 'success' : row.source_type === 'caddy' ? 'primary' : 'info'" size="small">
              {{ row.source_type }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="log_path" :label="t('logManage.logPath')" min-width="200" show-overflow-tooltip />
        <el-table-column prop="enabled" :label="t('common.status')" width="80">
          <template #default="{row}">
            <el-tag :type="row.enabled ? 'success' : 'danger'" size="small">
              {{ row.enabled ? t('common.enabled') : t('common.disabled') }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="rotate_cron" :label="t('logManage.rotateCron')" width="140" />
        <el-table-column prop="retain_days" :label="t('logManage.retainDays')" width="100" />
        <el-table-column prop="compress" :label="t('logManage.compress')" width="80">
          <template #default="{row}">
            <el-tag :type="row.compress ? 'warning' : 'info'" size="small">
              {{ row.compress ? t('common.yes') : t('common.no') }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('common.actions')" width="380" fixed="right">
          <template #default="{row}">
            <el-button size="small" type="success" @click="showLogViewer(row)">
              <el-icon><View /></el-icon>{{ t('logManage.viewLog') }}
            </el-button>
            <el-button size="small" type="primary" @click="handleRotate(row)" :loading="rotatingId === row.id">
              <el-icon><RefreshRight /></el-icon>{{ t('logManage.rotate') }}
            </el-button>
            <el-button size="small" @click="showJobs(row)">{{ t('logManage.history') }}</el-button>
            <el-button size="small" @click="showEdit(row)">{{ t('common.edit') }}</el-button>
            <el-button size="small" type="danger" @click="handleDelete(row)">{{ t('common.delete') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Create/Edit Dialog -->
    <el-dialog v-model="dialogVisible" :title="editing ? t('logManage.edit') : t('logManage.create')" width="550">
      <el-form :model="form" label-width="120px">
        <el-form-item :label="t('common.name')" required>
          <el-input v-model="form.name" :placeholder="t('logManage.namePlaceholder')" />
        </el-form-item>
        <el-form-item :label="t('logManage.sourceType')" required>
          <el-select v-model="form.source_type" style="width:100%">
            <el-option label="Nginx" value="nginx" />
            <el-option label="Caddy" value="caddy" />
            <el-option label="Custom" value="custom" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('logManage.logPath')" required>
          <el-select
            v-model="form.log_path"
            filterable
            allow-create
            default-first-option
            :placeholder="t('logManage.logPathPlaceholder')"
            style="width:100%"
            @focus="loadDiscoveredOptions"
            @change="onLogPathSelect"
          >
            <el-option-group :label="'Nginx'">
              <el-option
                v-for="item in discoveredOptions.filter(d => d.source_type === 'nginx')"
                :key="item.log_path"
                :label="item.log_path"
                :value="item.log_path"
              >
                <span style="float:left">{{ item.domain || 'nginx' }}</span>
                <span style="float:right;color:#8492a6;font-size:12px">{{ item.log_path }}</span>
              </el-option>
            </el-option-group>
            <el-option-group :label="'Caddy'">
              <el-option
                v-for="item in discoveredOptions.filter(d => d.source_type === 'caddy')"
                :key="item.log_path"
                :label="item.log_path"
                :value="item.log_path"
              >
                <span style="float:left">{{ item.domain || 'caddy' }}</span>
                <span style="float:right;color:#8492a6;font-size:12px">{{ item.log_path }}</span>
              </el-option>
            </el-option-group>
            <el-option-group v-if="discoveredOptions.filter(d => d.source_type === 'custom').length" :label="'Custom'">
              <el-option
                v-for="item in discoveredOptions.filter(d => d.source_type === 'custom')"
                :key="item.log_path"
                :label="item.log_path"
                :value="item.log_path"
              />
            </el-option-group>
          </el-select>
        </el-form-item>
        <el-form-item :label="t('common.status')">
          <el-switch v-model="form.enabled" :active-text="t('common.enabled')" :inactive-text="t('common.disabled')" />
        </el-form-item>
        <el-form-item :label="t('logManage.rotateCron')">
          <el-select v-model="form.rotate_cron" style="width:100%">
            <el-option :label="t('logManage.cronDaily')" value="0 0 * * *" />
            <el-option :label="t('logManage.cronDaily3am')" value="0 3 * * *" />
            <el-option :label="t('logManage.cronWeekly')" value="0 0 * * 0" />
            <el-option :label="t('logManage.cronEvery12h')" value="0 */12 * * *" />
            <el-option :label="t('logManage.cronEvery6h')" value="0 */6 * * *" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('logManage.retainDays')">
          <el-input-number v-model="form.retain_days" :min="1" :max="365" />
        </el-form-item>
        <el-form-item :label="t('logManage.compress')">
          <el-switch v-model="form.compress" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="saving" @click="handleSave">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- Job History Dialog -->
    <el-dialog v-model="jobsVisible" :title="t('logManage.jobHistory')" width="700">
      <el-table :data="jobs" v-loading="jobsLoading" stripe size="small">
        <el-table-column prop="id" :label="t('common.id')" width="60" />
        <el-table-column prop="status" :label="t('common.status')" width="80">
          <template #default="{row}">
            <el-tag :type="row.status === 'success' ? 'success' : 'danger'" size="small">
              {{ row.status }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="rotated_files" :label="t('logManage.rotatedFiles')" width="100" />
        <el-table-column prop="cleaned_files" :label="t('logManage.cleanedFiles')" width="100" />
        <el-table-column prop="freed_bytes" :label="t('logManage.freedSpace')" width="100">
          <template #default="{row}">
            {{ formatBytes(row.freed_bytes) }}
          </template>
        </el-table-column>
        <el-table-column prop="started_at" :label="t('logManage.startedAt')" min-width="160">
          <template #default="{row}">
            {{ formatTime(row.started_at) }}
          </template>
        </el-table-column>
        <el-table-column prop="error_message" :label="t('logManage.error')" min-width="150" show-overflow-tooltip />
      </el-table>
    </el-dialog>

    <!-- Log Viewer Dialog -->
    <el-dialog v-model="logViewerVisible" :title="t('logManage.viewLog') + ' - ' + logViewerPath" width="80%" top="5vh" :close-on-click-modal="false" @close="closeLogViewer">
      <div class="log-toolbar">
        <el-tag :type="logStreamConnected ? 'success' : 'info'" size="small">
          {{ logStreamConnected ? t('logManage.liveStreaming') : t('logManage.disconnected') }}
        </el-tag>
        <el-tag size="small" type="warning">{{ t('logManage.lineCount', { count: logLines.length }) }}</el-tag>
        <div style="flex:1"></div>
        <el-button size="small" :type="logAutoScroll ? 'primary' : 'default'" @click="logAutoScroll = !logAutoScroll">
          {{ logAutoScroll ? t('logManage.autoScrollOn') : t('logManage.autoScrollOff') }}
        </el-button>
        <el-button size="small" @click="clearLogLines">{{ t('logManage.clearLog') }}</el-button>
      </div>
      <div ref="logContainerRef" class="log-container" @scroll="onLogScroll">
        <div v-for="(line, i) in logLines" :key="i" class="log-line">{{ line }}</div>
        <div v-if="logLines.length === 0 && !logLoading" class="log-empty">{{ t('logManage.noLogContent') }}</div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, onUnmounted, nextTick } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { Plus, Refresh, Search, RefreshRight, View, CircleCheck } from '@element-plus/icons-vue'
import client from '../api/client'

const { t } = useI18n()

const policies = ref([])
const loading = ref(false)
const status = ref({ total_policies: 0, enabled_count: 0, scheduled_count: 0, total_jobs: 0 })

const dialogVisible = ref(false)
const editing = ref(null)
const saving = ref(false)
const form = ref({
  name: '',
  source_type: 'nginx',
  log_path: '',
  enabled: true,
  rotate_cron: '0 0 * * *',
  retain_days: 30,
  compress: false,
})

const discoveredLogs = ref([])
const discovering = ref(false)
const syncing = ref(false)
const rotatingId = ref(null)
const discoveredOptions = ref([])
const addingPath = ref(null)

const jobsVisible = ref(false)
const jobsLoading = ref(false)
const jobs = ref([])

// Log Viewer
const logViewerVisible = ref(false)
const logViewerPath = ref('')
const logLines = ref([])
const logLoading = ref(false)
const logAutoScroll = ref(true)
const logStreamConnected = ref(false)
const logContainerRef = ref(null)
let eventSource = null

async function fetchStatus() {
  try {
    const r = await client.get('/log-manage/status')
    status.value = r || status.value
  } catch {}
}

async function fetchData() {
  loading.value = true
  try {
    const r = await client.get('/log-manage/policies')
    policies.value = Array.isArray(r) ? r : []
  } finally {
    loading.value = false
  }
}

function showCreate() {
  editing.value = null
  form.value = {
    name: '',
    source_type: 'nginx',
    log_path: '',
    enabled: true,
    rotate_cron: '0 0 * * *',
    retain_days: 30,
    compress: false,
  }
  dialogVisible.value = true
  loadDiscoveredOptions()
}

function showEdit(row) {
  editing.value = row
  form.value = {
    name: row.name,
    source_type: row.source_type,
    log_path: row.log_path,
    enabled: row.enabled,
    rotate_cron: row.rotate_cron,
    retain_days: row.retain_days,
    compress: row.compress,
  }
  dialogVisible.value = true
  loadDiscoveredOptions()
}

async function loadDiscoveredOptions() {
  try {
    const r = await client.post('/log-manage/discover')
    discoveredOptions.value = Array.isArray(r) ? r : []
  } catch {
    discoveredOptions.value = []
  }
}

function onLogPathSelect(val) {
  // Auto-fill source_type and name when selecting a discovered log
  const found = discoveredOptions.value.find(d => d.log_path === val)
  if (found) {
    form.value.source_type = found.source_type
    if (!form.value.name || form.value.name === '') {
      form.value.name = found.domain || found.log_path.split('/').pop()
    }
  }
}

async function handleSave() {
  if (!form.value.name || !form.value.log_path) {
    ElMessage.warning(t('logManage.fillRequired'))
    return
  }
  saving.value = true
  try {
    if (editing.value) {
      await client.put(`/log-manage/policies/${editing.value.id}`, form.value)
    } else {
      await client.post('/log-manage/policies', form.value)
    }
    ElMessage.success(t('common.success'))
    dialogVisible.value = false
    fetchData()
    fetchStatus()
  } finally {
    saving.value = false
  }
}

async function handleDelete(row) {
  await ElMessageBox.confirm(
    t('logManage.deleteConfirm', { name: row.name }),
    t('common.confirm')
  )
  await client.delete(`/log-manage/policies/${row.id}`)
  ElMessage.success(t('common.success'))
  fetchData()
  fetchStatus()
}

async function handleRotate(row) {
  rotatingId.value = row.id
  try {
    await client.post(`/log-manage/policies/${row.id}/rotate`)
    ElMessage.success(t('logManage.rotateSuccess'))
    fetchStatus()
  } finally {
    rotatingId.value = null
  }
}

async function handleDiscover() {
  discovering.value = true
  try {
    const r = await client.post('/log-manage/discover')
    const logs = Array.isArray(r) ? r : []
    // Cross-check with existing policies
    const policyPaths = new Set(policies.value.map(p => p.log_path))
    for (const log of logs) {
      if (policyPaths.has(log.log_path)) {
        log.has_policy = true
      }
    }
    discoveredLogs.value = logs
    ElMessage.success(t('logManage.discoverDone', { count: discoveredLogs.value.length }))
  } finally {
    discovering.value = false
  }
}

async function handleSync() {
  syncing.value = true
  try {
    const r = await client.post('/log-manage/sync')
    ElMessage.success(t('logManage.syncDone', { count: r?.created || 0 }))
    fetchData()
    fetchStatus()
  } finally {
    syncing.value = false
  }
}

async function handleAddFromDiscover(row) {
  addingPath.value = row.log_path
  try {
    const name = row.domain || row.log_path.split('/').pop().replace('.log', '')
    await client.post('/log-manage/policies', {
      name: name,
      source_type: row.source_type,
      log_path: row.log_path,
      enabled: true,
      rotate_cron: '0 0 * * *',
      retain_days: 30,
      compress: false,
    })
    ElMessage.success(t('logManage.policyCreated', { name }))
    // Mark as having policy in the discovered list
    row.has_policy = true
    fetchData()
    fetchStatus()
  } catch (e) {
    // If error is "already exists", still mark it
    if (e?.response?.data?.message?.includes('UNIQUE') || e?.message?.includes('exists')) {
      row.has_policy = true
    }
  } finally {
    addingPath.value = null
  }
}

async function showJobs(row) {
  jobsVisible.value = true
  jobsLoading.value = true
  try {
    const r = await client.get(`/log-manage/policies/${row.id}/jobs`)
    jobs.value = r?.items || []
  } finally {
    jobsLoading.value = false
  }
}

function formatBytes(bytes) {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let i = 0
  let val = bytes
  while (val >= 1024 && i < units.length - 1) { val /= 1024; i++ }
  return `${val.toFixed(1)} ${units[i]}`
}

function formatTime(ts) {
  if (!ts) return ''
  return new Date(ts).toLocaleString()
}

async function showLogViewer(row) {
  logViewerPath.value = row.log_path
  logLines.value = []
  logAutoScroll.value = true
  logViewerVisible.value = true
  logLoading.value = true

  // Load initial lines
  try {
    const token = localStorage.getItem('access_token')
    const r = await client.get(`/log-manage/policies/${row.id}/log`, { params: { lines: 500 } })
    logLines.value = r?.lines || []
    await nextTick()
    scrollToBottom()
  } catch (e) {
    logLines.value = [`[Error loading log: ${e.message || e}]`]
  } finally {
    logLoading.value = false
  }

  // Start SSE stream
  startLogStream(row.id)
}

function startLogStream(policyId) {
  closeLogStream()
  const token = localStorage.getItem('access_token')
  const base = client.defaults.baseURL || '/api/v1'
  const url = `${base}/log-manage/policies/${policyId}/log-stream?lines=0&token=${encodeURIComponent(token)}`

  eventSource = new EventSource(url)

  eventSource.addEventListener('log', (e) => {
    logLines.value.push(e.data)
    // Keep max 5000 lines in memory
    if (logLines.value.length > 5000) {
      logLines.value = logLines.value.slice(-3000)
    }
    if (logAutoScroll.value) {
      nextTick(scrollToBottom)
    }
  })

  eventSource.onopen = () => {
    logStreamConnected.value = true
  }

  eventSource.onerror = () => {
    logStreamConnected.value = false
    // Auto-reconnect after 3s
    setTimeout(() => {
      if (logViewerVisible.value) {
        startLogStream(policyId)
      }
    }, 3000)
  }
}

function closeLogStream() {
  if (eventSource) {
    eventSource.close()
    eventSource = null
    logStreamConnected.value = false
  }
}

function closeLogViewer() {
  closeLogStream()
  logLines.value = []
  logViewerVisible.value = false
}

function clearLogLines() {
  logLines.value = []
}

function scrollToBottom() {
  if (logContainerRef.value) {
    logContainerRef.value.scrollTop = logContainerRef.value.scrollHeight
  }
}

function onLogScroll() {
  if (!logContainerRef.value) return
  const { scrollTop, scrollHeight, clientHeight } = logContainerRef.value
  // If user scrolled up more than 50px from bottom, disable auto-scroll
  logAutoScroll.value = scrollHeight - scrollTop - clientHeight < 50
}

onMounted(() => {
  fetchData()
  fetchStatus()
})

onUnmounted(() => {
  closeLogStream()
})
</script>

<style scoped>
.stat-card {
  text-align: center;
  padding: 8px 0;
}
.stat-value {
  font-size: 28px;
  font-weight: 700;
  line-height: 1.2;
}
.stat-label {
  font-size: 13px;
  color: #909399;
  margin-top: 4px;
}
.text-success { color: #67c23a; }
.text-primary { color: #409eff; }
.text-warning { color: #e6a23c; }

.log-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
  padding: 6px 0;
}
.log-container {
  background: #1e1e1e;
  color: #d4d4d4;
  font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
  font-size: 13px;
  line-height: 1.5;
  height: 60vh;
  overflow-y: auto;
  padding: 12px;
  border-radius: 6px;
  white-space: pre-wrap;
  word-break: break-all;
}
.log-line {
  padding: 0 4px;
  min-height: 20px;
}
.log-line:hover {
  background: rgba(255,255,255,0.05);
}
.log-empty {
  color: #666;
  text-align: center;
  padding: 40px 0;
}
</style>
