<template>
  <div>
    <div style="display:flex;justify-content:space-between;margin-bottom:16px">
      <h3 style="margin:0">{{ t('chainProxy.title') }}</h3>
      <div style="display:flex;gap:8px">
        <el-button type="primary" size="small" @click="showLinkDlg">{{ t('chainProxy.addLink') }}</el-button>
        <el-button size="small" @click="showExitDlg">{{ t('chainProxy.addExit') }}</el-button>
      </div>
    </div>

    <!-- Topology Overview -->
    <el-card style="margin-bottom:16px">
      <template #header>
        <div style="display:flex;justify-content:space-between;align-items:center">
          <span>{{ t('chainProxy.topology') }}</span>
          <el-button text size="small" @click="fetchTopology" :icon="Refresh">{{ t('common.reset') }}</el-button>
        </div>
      </template>
      <el-table :data="topoNodes" stripe size="small" v-loading="topoLoading">
        <el-table-column prop="id" label="ID" width="60" />
        <el-table-column prop="name" :label="t('common.name')" width="180" />
        <el-table-column prop="type" :label="t('common.type')" width="90">
          <template #default="{row}">
            <el-tag size="small" :type="row.type==='external' ? 'warning' : 'primary'">{{ row.type }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="role" :label="t('chainProxy.role')" width="130">
          <template #default="{row}">
            <el-tag size="small" :type="roleTagType(row.role)">{{ roleLabel(row.role) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('chainProxy.relayOnly')" width="100">
          <template #default="{row}">
            <el-tag v-if="row.relay_only" size="small" type="info">{{ t('common.yes') }}</el-tag>
            <span v-else>—</span>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Chain Links -->
    <el-card style="margin-bottom:16px">
      <template #header>{{ t('chainProxy.linksTitle') }}</template>
      <el-table :data="links" stripe size="small" v-loading="linksLoading">
        <el-table-column prop="id" label="ID" width="50" />
        <el-table-column :label="t('chainProxy.entryNode')" width="160">
          <template #default="{row}">{{ serverName(row.entry_server_id) }}</template>
        </el-table-column>
        <el-table-column :label="t('chainProxy.exitNode')" width="160">
          <template #default="{row}">{{ exitName(row) }}</template>
        </el-table-column>
        <el-table-column prop="exit_tag" :label="t('chainProxy.exitTag')" width="120" />
        <el-table-column prop="priority" :label="t('chainProxy.priority')" width="80" />
        <el-table-column prop="weight" :label="t('chainProxy.weight')" width="80" />
        <el-table-column prop="sort_order" :label="t('chainProxy.sortOrder')" width="80" />
        <el-table-column :label="t('common.status')" width="70">
          <template #default="{row}">
            <el-switch v-model="row.enabled" size="small" @change="toggleLink(row)" />
          </template>
        </el-table-column>
        <el-table-column :label="t('common.actions')" width="120">
          <template #default="{row}">
            <el-button size="small" text @click="editLink(row)">{{ t('common.edit') }}</el-button>
            <el-button size="small" type="danger" text @click="removeLink(row)">{{ t('common.delete') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Exit Configs -->
    <el-card>
      <template #header>{{ t('chainProxy.exitConfigsTitle') }}</template>
      <el-table :data="exitConfigs" stripe size="small" v-loading="exitsLoading">
        <el-table-column prop="server_id" label="ID" width="60" />
        <el-table-column :label="t('common.name')" width="160">
          <template #default="{row}">{{ serverName(row.server_id) }}</template>
        </el-table-column>
        <el-table-column prop="relay_port" :label="t('chainProxy.relayPort')" width="90" />
        <el-table-column prop="relay_protocol" :label="t('chainProxy.relayProtocol')" width="100" />
        <el-table-column prop="relay_transport" :label="t('chainProxy.relayTransport')" width="100" />
        <el-table-column prop="relay_security" :label="t('chainProxy.relaySecurity')" width="100" />
        <el-table-column :label="t('chainProxy.relayOnly')" width="90">
          <template #default="{row}">
            <el-tag v-if="row.relay_only" size="small" type="info">{{ t('common.yes') }}</el-tag>
            <span v-else>—</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('common.status')" width="70">
          <template #default="{row}">
            <el-switch v-model="row.enabled" size="small" @change="toggleExit(row)" />
          </template>
        </el-table-column>
        <el-table-column :label="t('common.actions')" width="120">
          <template #default="{row}">
            <el-button size="small" text @click="editExit(row)">{{ t('common.edit') }}</el-button>
            <el-button size="small" type="danger" text @click="removeExit(row)">{{ t('common.delete') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Link Dialog -->
    <el-dialog v-model="linkDlg" :title="linkEditing ? t('chainProxy.editLink') : t('chainProxy.addLink')" width="560">
      <el-form label-width="110px" size="small">
        <el-form-item :label="t('chainProxy.entryNode')">
          <el-select v-model="linkForm.entry_server_id" filterable style="width:100%">
            <el-option v-for="s in servers" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('chainProxy.exitNodeType')">
          <el-radio-group v-model="linkForm.exit_type" @change="onExitTypeChange">
            <el-radio label="server">{{ t('chainProxy.managedServer') }}</el-radio>
            <el-radio label="external">{{ t('chainProxy.externalNode') }}</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="linkForm.exit_type==='server'" :label="t('chainProxy.exitNode')">
          <el-select v-model="linkForm.exit_server_id" filterable style="width:100%">
            <el-option v-for="s in servers" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
        </el-form-item>
        <el-form-item v-else :label="t('chainProxy.exitNode')">
          <el-select v-model="linkForm.exit_external_id" filterable style="width:100%">
            <el-option v-for="s in externalServers" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('chainProxy.exitTag')">
          <el-input v-model="linkForm.exit_tag" placeholder="chain-exit-1" />
        </el-form-item>
        <el-form-item :label="t('chainProxy.priority')">
          <el-input-number v-model="linkForm.priority" :min="1" :max="99" />
        </el-form-item>
        <el-form-item :label="t('chainProxy.weight')">
          <el-input-number v-model="linkForm.weight" :min="1" :max="100" />
        </el-form-item>
        <el-form-item :label="t('chainProxy.sortOrder')">
          <el-input-number v-model="linkForm.sort_order" :min="0" />
        </el-form-item>
        <el-form-item :label="t('common.status')">
          <el-switch v-model="linkForm.enabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="linkDlg=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="linkSaving" @click="saveLink">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- Exit Config Dialog -->
    <el-dialog v-model="exitDlg" :title="exitEditing ? t('chainProxy.editExit') : t('chainProxy.addExit')" width="560">
      <el-form label-width="110px" size="small">
        <el-form-item :label="t('chainProxy.exitNode')">
          <el-select v-model="exitForm.server_id" filterable :disabled="exitEditing" style="width:100%">
            <el-option v-for="s in servers" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('chainProxy.relayPort')">
          <el-input-number v-model="exitForm.relay_port" :min="1" :max="65535" />
        </el-form-item>
        <el-form-item :label="t('chainProxy.relayProtocol')">
          <el-select v-model="exitForm.relay_protocol" style="width:160px">
            <el-option label="vless" value="vless" />
            <el-option label="vmess" value="vmess" />
            <el-option label="trojan" value="trojan" />
            <el-option label="shadowsocks" value="shadowsocks" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('chainProxy.relayTransport')">
          <el-select v-model="exitForm.relay_transport" style="width:160px">
            <el-option label="tcp" value="tcp" />
            <el-option label="ws" value="ws" />
            <el-option label="grpc" value="grpc" />
            <el-option label="httpupgrade" value="httpupgrade" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('chainProxy.relaySecurity')">
          <el-select v-model="exitForm.relay_security" style="width:160px">
            <el-option label="none" value="none" />
            <el-option label="tls" value="tls" />
            <el-option label="reality" value="reality" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('chainProxy.relayOnly')">
          <el-switch v-model="exitForm.relay_only" />
          <span style="margin-left:8px;font-size:12px;color:#909399">{{ t('chainProxy.relayOnlyHint') }}</span>
        </el-form-item>
        <el-form-item :label="t('common.status')">
          <el-switch v-model="exitForm.enabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="exitDlg=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="exitSaving" @click="saveExit">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import * as chainApi from '../api/chainlink'
import { listNodeServers } from '../api/nodeServer'

const { t } = useI18n()

// ── Data ──
const topoNodes = ref([])
const topoLoading = ref(false)
const links = ref([])
const linksLoading = ref(false)
const exitConfigs = ref([])
const exitsLoading = ref(false)
const servers = ref([])
const externalServers = ref([])

// ── Link Dialog ──
const linkDlg = ref(false)
const linkEditing = ref(false)
const linkSaving = ref(false)
const linkForm = ref({
  id: 0,
  entry_server_id: 0,
  exit_type: 'server',
  exit_server_id: 0,
  exit_external_id: 0,
  exit_tag: '',
  priority: 1,
  weight: 100,
  sort_order: 0,
  enabled: true,
})

// ── Exit Config Dialog ──
const exitDlg = ref(false)
const exitEditing = ref(false)
const exitSaving = ref(false)
const exitForm = ref({
  server_id: 0,
  relay_port: 0,
  relay_protocol: 'vless',
  relay_transport: 'tcp',
  relay_security: 'none',
  relay_only: false,
  enabled: true,
})

// ── Helpers ──
function serverName(id) {
  const s = servers.value.find(x => x.id === id)
  return s ? s.name : `#${id}`
}

function exitName(row) {
  if (row.exit_server_id) return serverName(row.exit_server_id)
  const es = externalServers.value.find(x => x.id === row.exit_external_id)
  return es ? es.name : `ext:${row.exit_external_id}`
}

function roleLabel(role) {
  const map = {
    'entry': t('chainProxy.roleEntry'),
    'exit': t('chainProxy.roleExit'),
    'entry+exit': t('chainProxy.roleEntryExit'),
    'exit+entry': t('chainProxy.roleExitEntry'),
    'external_entry': t('chainProxy.roleExternalEntry'),
    'standalone': t('chainProxy.roleStandalone'),
  }
  return map[role] || role
}

function roleTagType(role) {
  const map = {
    'entry': 'primary',
    'exit': 'success',
    'entry+exit': 'warning',
    'exit+entry': 'warning',
    'external_entry': 'info',
    'standalone': '',
  }
  return map[role] || ''
}

function onExitTypeChange(val) {
  if (val === 'server') {
    linkForm.value.exit_external_id = 0
  } else {
    linkForm.value.exit_server_id = 0
  }
}

// ── Fetch ──
async function fetchAll() {
  await Promise.all([fetchTopology(), fetchLinks(), fetchExitConfigs(), fetchServers(), fetchExternalServers()])
}

async function fetchTopology() {
  topoLoading.value = true
  try {
    const data = await chainApi.getTopology()
    topoNodes.value = data.nodes || []
  } finally { topoLoading.value = false }
}

async function fetchLinks() {
  linksLoading.value = true
  try {
    links.value = await chainApi.listLinks()
  } finally { linksLoading.value = false }
}

async function fetchExitConfigs() {
  exitsLoading.value = true
  try {
    exitConfigs.value = await chainApi.listExitConfigs()
  } finally { exitsLoading.value = false }
}

async function fetchServers() {
  servers.value = await listNodeServers()
}

async function fetchExternalServers() {
  externalServers.value = await chainApi.listExternalServers()
}

// ── Link CRUD ──
function showLinkDlg() {
  linkEditing.value = false
  linkForm.value = {
    id: 0, entry_server_id: 0, exit_type: 'server',
    exit_server_id: 0, exit_external_id: 0, exit_tag: '',
    priority: 1, weight: 100, sort_order: 0, enabled: true,
  }
  linkDlg.value = true
}

function editLink(row) {
  linkEditing.value = true
  linkForm.value = {
    id: row.id,
    entry_server_id: row.entry_server_id,
    exit_type: row.exit_external_id ? 'external' : 'server',
    exit_server_id: row.exit_server_id || 0,
    exit_external_id: row.exit_external_id || 0,
    exit_tag: row.exit_tag || '',
    priority: row.priority,
    weight: row.weight,
    sort_order: row.sort_order,
    enabled: row.enabled,
  }
  linkDlg.value = true
}

async function saveLink() {
  if (!linkForm.value.entry_server_id) {
    ElMessage.warning(t('chainProxy.selectEntry'))
    return
  }
  if (linkForm.value.exit_type === 'server' && !linkForm.value.exit_server_id) {
    ElMessage.warning(t('chainProxy.selectExit'))
    return
  }
  if (linkForm.value.exit_type === 'external' && !linkForm.value.exit_external_id) {
    ElMessage.warning(t('chainProxy.selectExit'))
    return
  }

  linkSaving.value = true
  try {
    const payload = {
      entry_server_id: linkForm.value.entry_server_id,
      exit_server_id: linkForm.value.exit_type === 'server' ? linkForm.value.exit_server_id : 0,
      exit_external_id: linkForm.value.exit_type === 'external' ? linkForm.value.exit_external_id : 0,
      exit_tag: linkForm.value.exit_tag,
      priority: linkForm.value.priority,
      weight: linkForm.value.weight,
      sort_order: linkForm.value.sort_order,
      enabled: linkForm.value.enabled,
    }
    if (linkEditing.value) {
      await chainApi.updateLink(linkForm.value.id, payload)
    } else {
      await chainApi.createLink(payload)
    }
    ElMessage.success(t('common.success'))
    linkDlg.value = false
    await fetchLinks()
    await fetchTopology()
  } catch { /* error handled by interceptor */ } finally { linkSaving.value = false }
}

async function toggleLink(row) {
  try {
    await chainApi.updateLink(row.id, { enabled: row.enabled })
    ElMessage.success(t('common.success'))
  } catch {
    row.enabled = !row.enabled
  }
}

async function removeLink(row) {
  try {
    await ElMessageBox.confirm(t('chainProxy.confirmDeleteLink'), t('common.confirm'), { type: 'warning' })
    await chainApi.deleteLink(row.id)
    ElMessage.success(t('common.success'))
    await fetchLinks()
    await fetchTopology()
  } catch { /* cancelled */ }
}

// ── Exit Config CRUD ──
function showExitDlg() {
  exitEditing.value = false
  exitForm.value = {
    server_id: 0, relay_port: 0,
    relay_protocol: 'vless', relay_transport: 'tcp',
    relay_security: 'none', relay_only: false, enabled: true,
  }
  exitDlg.value = true
}

function editExit(row) {
  exitEditing.value = true
  exitForm.value = { ...row }
  exitDlg.value = true
}

async function saveExit() {
  if (!exitForm.value.server_id) {
    ElMessage.warning(t('chainProxy.selectExit'))
    return
  }
  exitSaving.value = true
  try {
    if (exitEditing.value) {
      const { relay_port, relay_protocol, relay_transport, relay_security, relay_only, enabled } = exitForm.value
      await chainApi.updateExitConfig(exitForm.value.server_id, {
        relay_port, relay_protocol, relay_transport, relay_security, relay_only, enabled,
      })
    } else {
      await chainApi.createExitConfig(exitForm.value)
    }
    ElMessage.success(t('common.success'))
    exitDlg.value = false
    await fetchExitConfigs()
    await fetchTopology()
  } catch { /* handled */ } finally { exitSaving.value = false }
}

async function toggleExit(row) {
  try {
    await chainApi.updateExitConfig(row.server_id, { enabled: row.enabled })
    ElMessage.success(t('common.success'))
  } catch {
    row.enabled = !row.enabled
  }
}

async function removeExit(row) {
  try {
    await ElMessageBox.confirm(t('chainProxy.confirmDeleteExit'), t('common.confirm'), { type: 'warning' })
    await chainApi.deleteExitConfig(row.server_id)
    ElMessage.success(t('common.success'))
    await fetchExitConfigs()
    await fetchTopology()
  } catch { /* cancelled */ }
}

onMounted(fetchAll)
</script>
