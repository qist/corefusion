<template>
  <div>
    <!-- Sources -->
    <el-card style="margin-bottom:16px">
      <template #header>
        <div style="display:flex;justify-content:space-between;align-items:center">
          <span style="font-weight:600">{{ t('service.cfip.sources') }}</span>
          <div style="display:flex;gap:8px">
            <el-button size="small" @click="seedDefaults">{{ t('service.cfip.seedDefaults') }}</el-button>
            <el-button size="small" type="success" @click="fetchAll">{{ t('service.cfip.fetchAll') }}</el-button>
            <el-button size="small" type="primary" @click="showAddSource">+ {{ t('service.cfip.addSource') }}</el-button>
          </div>
        </div>
      </template>
      <div v-for="src in sources" :key="src.id" class="source-item">
        <div style="display:flex;gap:8px;align-items:center;flex-wrap:wrap">
          <el-switch v-model="src.enabled" @change="updateSource(src)" />
          <el-input v-model="src.name" style="width:120px" size="small" @change="updateSource(src)" />
          <el-tag size="small" :type="src.type === 'bestproxy' ? 'warning' : 'success'">{{ src.type }}</el-tag>
          <el-input v-model="src.url" style="flex:1;min-width:200px" size="small" @change="updateSource(src)" />
          <el-input-number v-model="src.interval" :min="300" :max="86400" :step="300" size="small" style="width:120px" @change="updateSource(src)" />
          <span style="font-size:12px;color:#909399">{{ t('service.cfip.seconds') }}</span>
          <el-button size="small" type="success" text @click="fetchSource(src)">{{ t('service.cfip.fetch') }}</el-button>
          <el-button size="small" type="danger" text @click="deleteSource(src)">X</el-button>
        </div>
        <!-- Ports -->
        <div style="display:flex;gap:4px;align-items:center;margin-top:6px;padding-left:36px;flex-wrap:wrap">
          <template v-if="src.type !== 'bestproxy'">
            <span style="font-size:12px;color:#909399">HTTP:</span>
            <el-checkbox v-for="p in cfHttpPorts" :key="p" :model-value="getPorts(src).includes(p)" @change="(v) => togglePort(src, p, v)" size="small">
              {{ p }}
            </el-checkbox>
            <span style="font-size:12px;color:#909399;margin-left:8px">HTTPS:</span>
            <el-checkbox v-for="p in cfHttpsPorts" :key="p" :model-value="getPorts(src).includes(p)" @change="(v) => togglePort(src, p, v)" size="small">
              {{ p }}
            </el-checkbox>
          </template>
          <template v-else>
            <span style="font-size:12px;color:#909399">HTTPS:</span>
            <el-checkbox :model-value="true" disabled size="small">443</el-checkbox>
            <span style="font-size:11px;color:#909399">({{ t('service.cfip.onlySupport') }})</span>
          </template>
        </div>
        <div v-if="src.last_run" style="font-size:11px;color:#909399;margin-top:4px;padding-left:36px">
          {{ t('service.cfip.lastRun') }}: {{ formatTime(src.last_run) }}
        </div>
      </div>
      <div v-if="!sources.length" style="text-align:center;color:#909399;padding:16px">{{ t('common.noData') }}</div>
    </el-card>

    <!-- Entries -->
    <el-card>
      <template #header>
        <div style="display:flex;justify-content:space-between;align-items:center">
          <span style="font-weight:600">{{ t('service.cfip.entries') }} ({{ entries.length }})</span>
          <div style="display:flex;gap:8px">
            <el-button size="small" type="danger" @click="clearEntries">{{ t('service.cfip.clearAll') }}</el-button>
            <el-button size="small" type="primary" @click="showAddEntry">+ {{ t('service.cfip.addIP') }}</el-button>
          </div>
        </div>
      </template>
      <div style="display:flex;gap:8px;flex-wrap:wrap">
        <div v-for="entry in entries" :key="entry.id" class="ip-item">
          <el-switch v-model="entry.enabled" size="small" style="margin-right:4px" />
          <span style="font-family:monospace;font-size:13px">{{ entry.ip }}</span>
          <span v-if="entry.country" style="font-size:11px;color:#909399;margin-left:4px">{{ entry.country }}</span>
          <el-button size="small" type="danger" text @click="deleteEntry(entry)" style="margin-left:4px">X</el-button>
        </div>
      </div>
      <div v-if="!entries.length" style="text-align:center;color:#909399;padding:16px">{{ t('common.noData') }}</div>
    </el-card>

    <!-- Add Source Dialog -->
    <el-dialog v-model="sourceDlg" :title="t('service.cfip.addSource')" width="500">
      <el-form label-width="80px">
        <el-form-item :label="t('service.cfip.name')">
          <el-input v-model="newSource.name" />
        </el-form-item>
        <el-form-item label="URL">
          <el-input v-model="newSource.url" placeholder="https://..." />
        </el-form-item>
        <el-form-item :label="t('service.cfip.type')">
          <el-select v-model="newSource.type" style="width:100%">
            <el-option :label="t('service.cfip.bestcf')" value="bestcf" />
            <el-option :label="t('service.cfip.bestproxy')" value="bestproxy" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('service.cfip.interval')">
          <el-input-number v-model="newSource.interval" :min="300" :max="86400" :step="300" />
          <span style="margin-left:8px;color:#999">{{ t('service.cfip.seconds') }}</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="sourceDlg=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="addSource">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- Add Entry Dialog -->
    <el-dialog v-model="entryDlg" :title="t('service.cfip.addIP')" width="500">
      <el-form label-width="60px">
        <el-form-item label="IP">
          <el-input v-model="newEntry.ip" placeholder="1.2.3.4" />
        </el-form-item>
        <el-form-item :label="t('service.cfip.port')">
          <div style="display:flex;gap:4px;flex-wrap:wrap">
            <span style="font-size:12px;color:#909399">HTTP:</span>
            <el-checkbox v-for="p in cfHttpPorts" :key="p" :model-value="newEntry.ports.includes(p)" @change="(v) => toggleNewPort(p, v)" size="small">
              {{ p }}
            </el-checkbox>
            <span style="font-size:12px;color:#909399;margin-left:8px">HTTPS:</span>
            <el-checkbox v-for="p in cfHttpsPorts" :key="p" :model-value="newEntry.ports.includes(p)" @change="(v) => toggleNewPort(p, v)" size="small">
              {{ p }}
            </el-checkbox>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="entryDlg=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="addEntry">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import client from '../../api/client'

const { t } = useI18n()
const sources = ref([])
const entries = ref([])
const sourceDlg = ref(false)
const entryDlg = ref(false)
const newSource = ref({ name: '', url: '', type: 'bestcf', interval: 3600 })
const newEntry = ref({ ip: '', ports: [443] })

const cfHttpPorts = [80, 8080, 8880, 2052, 2082, 2086, 2095]
const cfHttpsPorts = [443, 8443, 2053, 2083, 2087, 2096]

function getPorts(src) {
  try {
    return src.ports ? JSON.parse(src.ports) : []
  } catch { return [] }
}

function togglePort(src, port, checked) {
  let ports = getPorts(src)
  if (checked) {
    if (!ports.includes(port)) ports.push(port)
  } else {
    ports = ports.filter(p => p !== port)
  }
  src.ports = JSON.stringify(ports)
  updateSource(src)
}

function toggleNewPort(port, checked) {
  if (checked) {
    if (!newEntry.value.ports.includes(port)) newEntry.value.ports.push(port)
  } else {
    newEntry.value.ports = newEntry.value.ports.filter(p => p !== port)
  }
}

async function fetchSources() {
  try {
    const data = await client.get('/cfip/sources')
    sources.value = Array.isArray(data) ? data : []
  } catch { sources.value = [] }
}

async function fetchEntries() {
  try {
    const data = await client.get('/cfip/entries')
    entries.value = Array.isArray(data) ? data : []
  } catch { entries.value = [] }
}

function showAddSource() {
  newSource.value = { name: '', url: '', type: 'bestcf', interval: 3600 }
  sourceDlg.value = true
}

async function addSource() {
  try {
    const ports = newSource.value.type === 'bestproxy' ? '[443]' : '[443]'
    await client.post('/cfip/sources', { ...newSource.value, ports })
    sourceDlg.value = false
    await fetchSources()
    ElMessage.success(t('common.success'))
  } catch { ElMessage.error(t('common.failed')) }
}

async function updateSource(src) {
  try {
    await client.put(`/cfip/sources/${src.id}`, src)
  } catch { ElMessage.error(t('common.failed')) }
}

async function deleteSource(src) {
  try {
    await ElMessageBox.confirm(t('common.confirm'), t('common.confirm'))
    await client.delete(`/cfip/sources/${src.id}`)
    await fetchSources()
    await fetchEntries()
    ElMessage.success(t('common.success'))
  } catch {}
}

async function fetchSource(src) {
  try {
    await client.post(`/cfip/sources/${src.id}/fetch`)
    await fetchSources()
    await fetchEntries()
    ElMessage.success(t('service.cfip.fetchDone'))
  } catch { ElMessage.error(t('common.failed')) }
}

async function fetchAll() {
  try {
    await client.post('/cfip/fetch-all')
    await fetchSources()
    await fetchEntries()
    ElMessage.success(t('service.cfip.fetchDone'))
  } catch { ElMessage.error(t('common.failed')) }
}

function showAddEntry() {
  newEntry.value = { ip: '', ports: [443] }
  entryDlg.value = true
}

async function addEntry() {
  try {
    await client.post('/cfip/entries', {
      ip: newEntry.value.ip,
      ports: JSON.stringify(newEntry.value.ports)
    })
    entryDlg.value = false
    await fetchEntries()
    ElMessage.success(t('common.success'))
  } catch { ElMessage.error(t('common.failed')) }
}

async function deleteEntry(entry) {
  try {
    await client.delete(`/cfip/entries/${entry.id}`)
    entries.value = entries.value.filter(e => e.id !== entry.id)
  } catch { ElMessage.error(t('common.failed')) }
}

async function clearEntries() {
  try {
    await ElMessageBox.confirm(t('common.confirm'), t('common.confirm'))
    await client.delete('/cfip/entries')
    entries.value = []
    ElMessage.success(t('common.success'))
  } catch {}
}

async function seedDefaults() {
  try {
    await client.post('/cfip/seed')
    await fetchSources()
    ElMessage.success(t('common.success'))
  } catch { ElMessage.error(t('common.failed')) }
}

function formatTime(ts) {
  if (!ts) return '-'
  return new Date(ts).toLocaleString()
}

onMounted(() => { fetchSources(); fetchEntries() })
</script>

<style scoped>
.source-item {
  padding: 10px 0;
  border-bottom: 1px solid var(--el-border-color-lighter);
}
.source-item:last-child { border-bottom: none; }
.ip-item {
  display: inline-flex;
  align-items: center;
  padding: 4px 8px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 4px;
  margin-bottom: 4px;
}
</style>
