<template>
  <div>
    <!-- Header -->
    <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:16px">
      <div style="display:flex;align-items:center;gap:12px">
        <el-button text @click="goBack"><el-icon><ArrowLeft /></el-icon> {{ t('common.back') }}</el-button>
        <span style="font-size:18px;font-weight:600">{{ server?.name || '...' }}</span>
        <el-tag v-if="server" size="small" :type="server.is_local ? 'warning' : 'success'">
          {{ server.is_local ? t('nodeServer.localNode') : t('nodeServer.remoteNode') }}
        </el-tag>
        <el-tag v-if="server" size="small" :type="statusTag(server.agent_status)">
          <span :class="['status-dot', server.agent_status]"></span>
          {{ statusLabel(server.agent_status) }}
        </el-tag>
      </div>
      <el-button size="small" @click="loadServer"><el-icon><Refresh /></el-icon> {{ t('common.refresh') }}</el-button>
    </div>

    <!-- Server Info Bar -->
    <el-card v-if="server" shadow="hover" style="margin-bottom:16px" body-style="padding:12px 20px">
      <div style="display:flex;gap:32px;font-size:13px;flex-wrap:wrap">
        <div><span style="color:#909399">{{ t('nodeServer.hostname') }}: </span>{{ server.hostname }}</div>
        <div><span style="color:#909399">{{ t('nodeServer.region') }}: </span>{{ server.region || '-' }}</div>
        <div><span style="color:#909399">{{ t('nodeServer.nodeType') }}: </span>{{ nodeTypeLabel(server.node_type) }}</div>
        <div v-if="server.agent_version"><span style="color:#909399">{{ t('nodeServer.agentVersion') }}: </span>{{ server.agent_version }}</div>
        <div v-if="server.last_heartbeat_at"><span style="color:#909399">{{ t('nodeServer.lastHeartbeat') }}: </span>{{ formatTime(server.last_heartbeat_at) }}</div>
      </div>
    </el-card>

    <!-- Tabs -->
    <el-tabs v-model="activeTab" type="border-card">
      <!-- Overview Tab -->
      <el-tab-pane :label="t('nodeServer.overview')" name="overview">
        <el-descriptions v-if="server" :column="3" border>
          <el-descriptions-item :label="t('common.name')">{{ server.name }}</el-descriptions-item>
          <el-descriptions-item :label="t('nodeServer.hostname')">{{ server.hostname }}</el-descriptions-item>
          <el-descriptions-item :label="t('nodeServer.region')">{{ server.region || '-' }}</el-descriptions-item>
          <el-descriptions-item :label="t('nodeServer.nodeType')">{{ nodeTypeLabel(server.node_type) }}</el-descriptions-item>
          <el-descriptions-item :label="t('nodeServer.agentStatus')">
            <el-tag size="small" :type="statusTag(server.agent_status)">{{ statusLabel(server.agent_status) }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item :label="t('nodeServer.agentVersion')">{{ server.agent_version || '-' }}</el-descriptions-item>
          <el-descriptions-item :label="t('nodeServer.lastHeartbeat')">{{ server.last_heartbeat_at ? formatTime(server.last_heartbeat_at) : '-' }}</el-descriptions-item>
          <el-descriptions-item :label="t('nodeServer.certStatus')">
            <span v-if="server.agent_cert_expires_at">{{ formatTime(server.agent_cert_expires_at) }}</span>
            <span v-else>-</span>
          </el-descriptions-item>
          <el-descriptions-item :label="t('nodeServer.isLocal')">
            <el-tag size="small" :type="server.is_local ? 'warning' : 'info'">{{ server.is_local ? t('common.yes') : t('common.no') }}</el-tag>
          </el-descriptions-item>
        </el-descriptions>
      </el-tab-pane>

      <!-- Protocol Config Tab -->
      <el-tab-pane :label="t('menu.protoCenter')" name="protocols">
        <ProtocolCenter :server-id="serverId" />
      </el-tab-pane>

      <!-- Proxy Management Tab -->
      <el-tab-pane :label="t('menu.proxy')" name="proxy">
        <ProxyManagement :server-id="serverId" :is-local="server?.is_local || false" />
      </el-tab-pane>

      <!-- Geo Rules Tab -->
      <el-tab-pane :label="t('service.tabs.rules')" name="rules">
        <RuleTab :server-id="serverId" />
      </el-tab-pane>

      <!-- Binary Management Tab -->
      <el-tab-pane :label="t('nodeServer.binaryManagement')" name="binaries">
        <div style="padding:20px">
          <!-- Core Binaries -->
          <h3 style="margin-bottom:16px">{{ t('nodeServer.coreBinaries') }}</h3>
          <el-table :data="binaries" v-loading="binariesLoading" stripe size="small" style="margin-bottom:24px">
            <el-table-column prop="core_type" :label="t('nodeServer.coreType')" width="120" />
            <el-table-column :label="t('nodeServer.installed')" width="100">
              <template #default="{row}">
                <el-tag size="small" :type="row.installed ? 'success' : 'info'">{{ row.installed ? t('common.yes') : t('common.no') }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="version" :label="t('nodeServer.agentVersion')" width="150" />
            <el-table-column prop="binary_path" :label="t('nodeServer.binaryPath')" />
            <el-table-column :label="t('common.actions')" width="160">
              <template #default="{row}">
                <el-button type="primary" size="small" :loading="installLoading === row.core_type" @click="doInstallBinary(row.core_type)">
                  {{ row.installed ? t('common.update') : t('common.install') }}
                </el-button>
              </template>
            </el-table-column>
          </el-table>

          <!-- Proxy Binaries -->
          <h3 style="margin-bottom:16px">{{ t('nodeServer.proxyBinaries') }}</h3>
          <el-table :data="proxyStatuses" v-loading="proxyStatusLoading" stripe size="small">
            <el-table-column prop="name" :label="t('nodeServer.proxyType')" width="120" />
            <el-table-column :label="t('nodeServer.installed')" width="100">
              <template #default="{row}">
                <el-tag size="small" :type="row.installed ? 'success' : 'info'">{{ row.installed ? t('common.yes') : t('common.no') }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="version" :label="t('nodeServer.agentVersion')" width="150" />
            <el-table-column prop="status" :label="t('nodeServer.status')" width="100">
              <template #default="{row}">
                <el-tag size="small" :type="row.status==='running'?'success':'info'">{{ row.status }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="path" :label="t('nodeServer.binaryPath')" />
            <el-table-column :label="t('common.actions')" width="160">
              <template #default="{row}">
                <el-button type="primary" size="small" :loading="proxyInstallLoading === row.type" @click="doInstallProxy(row.type)">
                  {{ row.installed ? t('common.update') : t('common.install') }}
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </div>
      </el-tab-pane>

      <!-- Core Control Tab -->
      <el-tab-pane :label="t('nodeServer.controlCore')" name="control">
        <div style="padding:20px">
          <h3 style="margin-bottom:16px">{{ t('nodeServer.controlCore') }}</h3>
          <el-table :data="coreStatuses" v-loading="statusLoading" stripe size="small">
            <el-table-column prop="core_type" :label="t('nodeServer.coreType')" width="120" />
            <el-table-column :label="t('nodeServer.status')" width="100">
              <template #default="{row}">
                <el-tag size="small" :type="row.state==='running'?'success':row.state==='crashed'?'danger':'info'">{{ row.state }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="pid" label="PID" width="80" />
            <el-table-column prop="version" :label="t('nodeServer.agentVersion')" width="100" />
            <el-table-column :label="t('common.actions')" width="240">
              <template #default="{row}">
                <el-button v-if="row.state!=='running'" type="success" size="small" :loading="controlLoading" @click="doControl(row.core_type,'start')">{{ t('protoCenter.status.start') }}</el-button>
                <el-button v-if="row.state==='running'" type="danger" size="small" :loading="controlLoading" @click="doControl(row.core_type,'stop')">{{ t('protoCenter.status.stop') }}</el-button>
                <el-button size="small" :loading="controlLoading" @click="doControl(row.core_type,'restart')">{{ t('protoCenter.status.restart') }}</el-button>
              </template>
            </el-table-column>
          </el-table>
        </div>
      </el-tab-pane>

      <!-- Certificates Tab -->
      <el-tab-pane :label="t('menu.certificates')" name="certs">
        <div style="padding:20px">
          <el-table :data="certificates" v-loading="certLoading" stripe size="small">
            <el-table-column prop="id" label="ID" width="60" />
            <el-table-column prop="domain" :label="t('nodeServer.hostname')" />
            <el-table-column prop="issuer" label="Issuer" />
            <el-table-column :label="t('nodeServer.certStatus')" width="180">
              <template #default="{row}">
                <span>{{ row.expires_at ? formatTime(row.expires_at) : '-' }}</span>
              </template>
            </el-table-column>
          </el-table>
        </div>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { ArrowLeft, Refresh } from '@element-plus/icons-vue'
import client from '../api/client'
import { listBinaries, installBinary, installProxy, getRemoteProxyStatus } from '../api/nodeServer'
import ProtocolCenter from './protocols/ProtocolCenter.vue'
import ProxyManagement from './ProxyManagement.vue'
import RuleTab from './services/RuleTab.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

const serverId = parseInt(route.params.id) || 0
const activeTab = ref('overview')
const server = ref(null)
const coreStatuses = ref([])
const certificates = ref([])
const statusLoading = ref(false)
const certLoading = ref(false)
const controlLoading = ref(false)

// Binary management
const binaries = ref([])
const binariesLoading = ref(false)
const installLoading = ref('')
const proxyStatuses = ref([])
const proxyStatusLoading = ref(false)
const proxyInstallLoading = ref('')

const nodeTypeOptions = {
  standalone: t('nodeServer.nodeTypeStandalone'),
  primary: t('nodeServer.nodeTypePrimary'),
  mirror: t('nodeServer.nodeTypeMirror'),
  chain_entry: t('nodeServer.nodeTypeChainEntry'),
  chain_exit: t('nodeServer.nodeTypeChainExit'),
}

const statusLabels = {
  online: t('nodeServer.statusOnline'),
  offline: t('nodeServer.statusOffline'),
  degraded: t('nodeServer.statusDegraded'),
  pending: t('nodeServer.statusPending'),
}

function nodeTypeLabel(nt) { return nodeTypeOptions[nt] || nt || '-' }
function statusLabel(s) { return statusLabels[s] || s || '-' }
function statusTag(s) {
  if (s === 'online') return 'success'
  if (s === 'degraded') return 'warning'
  if (s === 'pending') return 'info'
  return 'danger'
}
function formatTime(ts) {
  if (!ts || ts === '0001-01-01T00:00:00Z') return '-'
  return new Date(ts).toLocaleString()
}

function goBack() { router.push('/node-servers') }

async function loadServer() {
  try {
    server.value = await client.get(`/node-servers/${serverId}`)
  } catch (e) {
    ElMessage.error(t('common.loadFailed'))
  }
}

async function loadStatus() {
  statusLoading.value = true
  try {
    const data = await client.get(`/node-servers/${serverId}/status`)
    coreStatuses.value = data?.cores || []
  } catch {
    coreStatuses.value = []
  } finally {
    statusLoading.value = false
  }
}

async function loadCerts() {
  certLoading.value = true
  try {
    const data = await client.get(`/node-servers/${serverId}/certificates`)
    certificates.value = data?.certificates || []
  } catch {
    certificates.value = []
  } finally {
    certLoading.value = false
  }
}

async function doControl(coreType, action) {
  controlLoading.value = true
  try {
    await client.post(`/node-servers/${serverId}/control`, { core_type: coreType, action })
    ElMessage.success(t('common.success'))
    loadStatus()
  } catch (e) {
    ElMessage.error(e?.message || t('common.failed'))
  } finally {
    controlLoading.value = false
  }
}

// Binary management
async function loadBinaries() {
  binariesLoading.value = true
  try {
    const data = await listBinaries(serverId)
    binaries.value = data?.binaries || []
  } catch {
    binaries.value = []
  } finally {
    binariesLoading.value = false
  }
}

async function loadProxyStatuses() {
  proxyStatusLoading.value = true
  try {
    const data = await getRemoteProxyStatus(serverId)
    proxyStatuses.value = data?.proxies || []
  } catch {
    proxyStatuses.value = []
  } finally {
    proxyStatusLoading.value = false
  }
}

async function doInstallBinary(coreType) {
  installLoading.value = coreType
  try {
    await installBinary(serverId, { core_type: coreType })
    ElMessage.success(t('common.success'))
    loadBinaries()
  } catch (e) {
    ElMessage.error(e?.message || t('common.failed'))
  } finally {
    installLoading.value = ''
  }
}

async function doInstallProxy(proxyType) {
  proxyInstallLoading.value = proxyType
  try {
    await installProxy(serverId, proxyType)
    ElMessage.success(t('common.success'))
    loadProxyStatuses()
  } catch (e) {
    ElMessage.error(e?.message || t('common.failed'))
  } finally {
    proxyInstallLoading.value = ''
  }
}

onMounted(() => {
  loadServer()
  loadStatus()
  loadCerts()
  loadBinaries()
  loadProxyStatuses()
})
</script>

<style scoped>
.status-dot {
  display: inline-block;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  margin-right: 4px;
}
.status-dot.online { background: #67c23a; }
.status-dot.offline { background: #f56c6c; }
.status-dot.degraded { background: #e6a23c; }
.status-dot.pending { background: #909399; }
</style>
