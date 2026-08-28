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

    <!-- Create Chain Proxy Wizard Dialog (see design doc §12.2) -->
    <el-dialog v-model="linkDlg" :title="t('chainProxy.createWizard')" width="680">
      <el-steps :active="wizardStep" finish-status="success" align-center>
        <el-step :title="t('chainProxy.stepEntry')" />
        <el-step :title="t('chainProxy.stepExits')" />
        <el-step :title="t('chainProxy.stepConfirm')" />
      </el-steps>
      <div style="margin-top:18px">
        <!-- Step 1: choose entry node + entry inbound -->
        <div v-show="wizardStep===0">
          <el-form label-width="110px" size="small">
            <el-form-item :label="t('chainProxy.entryNode')">
              <el-select v-model="wiz.entryServerId" filterable style="width:100%" @change="onEntryServerChange">
                <el-option v-for="s in servers" :key="s.id" :label="s.name" :value="s.id" />
              </el-select>
            </el-form-item>
            <el-form-item :label="t('chainProxy.entryInbound')">
              <el-select v-model="wiz.entryInboundId" filterable style="width:100%">
                <el-option v-for="ib in inboundConfigs" :key="ib.id" :label="`${ib.tag || ('in-'+ib.id)} · ${ib.protocol||''}`" :value="ib.id" />
              </el-select>
              <span style="font-size:12px;color:#909399">{{ t('chainProxy.entryInboundHint') }}</span>
            </el-form-item>
          </el-form>
        </div>
        <!-- Step 2: choose exits (cluster + external, both multi-select, like Plan/NodeGroup) -->
        <div v-show="wizardStep===1">
          <el-alert :title="t('chainProxy.multiExitHint')" type="info" :closable="false" style="margin-bottom:12px" />
          <el-row :gutter="16">
            <el-col :span="12">
              <div class="exit-group-title">{{ t('chainProxy.clusterNodes') }}</div>
              <el-select v-model="wiz.clusterExits" multiple filterable style="width:100%" :placeholder="t('chainProxy.selectExits')">
                <el-option v-for="s in servers" :key="s.id" :label="s.name" :value="s.id" />
              </el-select>
            </el-col>
            <el-col :span="12">
              <div class="exit-group-title">{{ t('chainProxy.externalNodes') }}</div>
              <el-select v-model="wiz.externalExits" multiple filterable style="width:100%" :placeholder="t('chainProxy.selectExits')">
                <el-option v-for="s in externalServers" :key="s.id" :label="s.name" :value="s.id" />
              </el-select>
            </el-col>
          </el-row>
          <div style="margin-top:10px;font-size:12px;color:#909399">
            {{ t('chainProxy.selectedCount', { n: totalSelectedExits }) }}
            <span v-if="totalSelectedExits>1" style="color:#e6a23c"> · {{ t('chainProxy.willLoadBalance') }}</span>
            <span v-else-if="totalSelectedExits===1" style="color:#67c23a"> · {{ t('chainProxy.willDirect') }}</span>
          </div>
        </div>
        <!-- Step 3: confirm -->
        <div v-show="wizardStep===2">
          <el-descriptions :column="1" border size="small">
            <el-descriptions-item :label="t('chainProxy.entryNode')">{{ entryServerName }}</el-descriptions-item>
            <el-descriptions-item :label="t('chainProxy.entryInbound')">{{ entryInboundLabel }}</el-descriptions-item>
            <el-descriptions-item :label="t('chainProxy.exitType')">
              {{ totalSelectedExits>1 ? t('chainProxy.loadBalance') : t('chainProxy.direct') }}
            </el-descriptions-item>
            <el-descriptions-item :label="t('chainProxy.exitCount')">{{ totalSelectedExits }}</el-descriptions-item>
          </el-descriptions>
        </div>
      </div>
      <template #footer>
        <el-button @click="linkDlg=false">{{ t('common.cancel') }}</el-button>
        <el-button v-if="wizardStep>0" @click="wizardStep--">{{ t('common.prev') }}</el-button>
        <el-button v-if="wizardStep<2" type="primary" @click="nextWizardStep" :disabled="!canNextStep">{{ t('common.next') }}</el-button>
        <el-button v-else type="primary" :loading="linkSaving" @click="createChainProxy">{{ t('common.create') }}</el-button>
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
import { listNodeServers, getInboundConfigs } from '../api/nodeServer'

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

// ── Create Chain Proxy Wizard (§5.5 / §12.2) ──
const wizardStep = ref(0)
const inboundConfigs = ref([])
const wiz = ref({
  entryServerId: 0,
  entryInboundId: 0,
  clusterExits: [],   // 选中的集群节点 server_id 列表
  externalExits: [],  // 选中的外部节点 external_id 列表
})
const totalSelectedExits = computed(() => wiz.value.clusterExits.length + wiz.value.externalExits.length)
const canNextStep = computed(() => {
  if (wizardStep.value === 0) return wiz.value.entryServerId && wiz.value.entryInboundId
  if (wizardStep.value === 1) return totalSelectedExits.value > 0
  return true
})
const entryServerName = computed(() => serverName(wiz.value.entryServerId))
const entryInboundLabel = computed(() => {
  const ib = inboundConfigs.value.find(x => x.id === wiz.value.entryInboundId)
  return ib ? `${ib.tag || ('in-' + ib.id)} · ${ib.protocol || ''}` : `#${wiz.value.entryInboundId}`
})
function resetWizard() {
  wizardStep.value = 0
  inboundConfigs.value = []
  wiz.value = { entryServerId: 0, entryInboundId: 0, clusterExits: [], externalExits: [] }
}

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
  // 打开「创建链式代理」向导（§5.5），取代原单条创建表单
  linkEditing.value = false
  resetWizard()
  linkDlg.value = true
}

async function onEntryServerChange(serverId) {
  wiz.value.entryInboundId = 0
  inboundConfigs.value = []
  if (!serverId) return
  try {
    const data = await getInboundConfigs(serverId)
    inboundConfigs.value = Array.isArray(data) ? data : (data.inbound_configs || [])
  } catch { /* ignore */ }
}

function nextWizardStep() {
  if (!canNextStep.value) return
  if (wizardStep.value < 2) wizardStep.value++
}

async function createChainProxy() {
  if (!canNextStep.value) return
  const exits = [
    ...wiz.value.clusterExits.map(id => ({ is_external: false, id })),
    ...wiz.value.externalExits.map(id => ({ is_external: true, id })),
  ]
  if (exits.length === 0) {
    ElMessage.warning(t('chainProxy.selectExit'))
    return
  }
  linkSaving.value = true
  try {
    const payload = {
      entry_server_id: wiz.value.entryServerId,
      entry_inbound_id: wiz.value.entryInboundId,
      exits,
    }
    await chainApi.createChainProxy(payload)
    ElMessage.success(t('common.success'))
    linkDlg.value = false
    await fetchLinks()
    await fetchTopology()
  } catch { /* error handled by interceptor */ } finally { linkSaving.value = false }
}

async function editLink(row) {
  // 编辑复用创建向导：预填入口与单个出口，保存时统一走清后建（§5.4）
  linkEditing.value = true
  resetWizard()
  wiz.value.entryServerId = row.entry_server_id
  wiz.value.entryInboundId = row.entry_inbound_id || row.entryInboundId || 0
  if (row.exit_external_id) {
    wiz.value.externalExits = [row.exit_external_id]
  } else if (row.exit_server_id) {
    wiz.value.clusterExits = [row.exit_server_id]
  }
  // 加载该入口节点的入站列表，便于回显入站标签
  if (wiz.value.entryServerId) {
    try {
      const data = await getInboundConfigs(wiz.value.entryServerId)
      inboundConfigs.value = Array.isArray(data) ? data : (data.inbound_configs || [])
    } catch { /* ignore */ }
  }
  linkDlg.value = true
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
