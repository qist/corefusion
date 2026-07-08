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

        <!-- Agent Update Section -->
        <el-card v-if="server && !server.is_local" shadow="never" style="margin-top:16px">
          <template #header>
            <div style="display:flex;align-items:center;gap:8px">
              <el-icon><Upload /></el-icon>
              <span style="font-weight:600">Agent Update</span>
            </div>
          </template>

          <!-- Current vs Panel Version -->
          <div style="display:flex;gap:24px;align-items:center;margin-bottom:16px;flex-wrap:wrap">
            <div>
              <span style="color:#909399;font-size:13px">Current Agent Version: </span>
              <el-tag size="small" type="info">{{ server.agent_version || 'unknown' }}</el-tag>
            </div>
            <div v-if="panelAgentVersion">
              <span style="color:#909399;font-size:13px">Panel Local Version: </span>
              <el-tag size="small" :type="server.agent_version === panelAgentVersion.version ? 'success' : 'warning'">
                {{ panelAgentVersion.version }}
              </el-tag>
              <el-button
                size="small" type="primary" style="margin-left:8px"
                :disabled="server.agent_version === panelAgentVersion.version || updating"
                :loading="updating"
                @click="pushUpdateFromPanel"
              >
                Update from Panel
              </el-button>
            </div>
            <div v-else-if="panelVersionLoading">
              <el-text type="info" size="small">Loading panel version...</el-text>
            </div>
            <div v-else>
              <el-text type="info" size="small">No Agent binary on panel</el-text>
            </div>
          </div>

          <el-divider content-position="center">Or from GitHub Releases</el-divider>

          <!-- GitHub Releases -->
          <div v-loading="releasesLoading">
            <div v-if="githubReleases.length === 0 && !releasesLoading" style="text-align:center;padding:20px;color:#909399">
              No releases available
            </div>
            <el-radio-group v-model="selectedRelease" style="display:flex;flex-wrap:wrap;gap:8px;margin-bottom:12px">
              <el-radio-button
                v-for="rel in githubReleases" :key="rel.version"
                :value="rel.version"
              >
                {{ rel.version }}
                <el-tag v-if="rel.prerelease" size="small" type="warning" style="margin-left:4px">pre</el-tag>
              </el-radio-button>
            </el-radio-group>

            <div v-if="selectedRelease" style="margin-bottom:12px">
              <div style="font-size:13px;color:#909399;margin-bottom:4px">Available assets:</div>
              <el-select v-model="selectedAsset" placeholder="Select asset" size="small" style="width:100%">
                <el-option
                  v-for="asset in selectedReleaseAssets" :key="asset.name"
                  :label="asset.name" :value="asset.download_url"
                />
              </el-select>
            </div>

            <el-button
              size="small" type="primary"
              :disabled="!selectedAsset || updating"
              :loading="updating"
              @click="pushUpdateFromGitHub"
            >
              Update from GitHub
            </el-button>
          </div>

          <!-- Update Status -->
          <div v-if="updateTask" style="margin-top:16px;padding:12px;background:#f5f7fa;border-radius:4px">
            <div style="display:flex;align-items:center;gap:8px">
              <span style="font-size:13px;font-weight:600">Update Status:</span>
              <el-tag size="small" :type="updateStatusTag(updateTask.status)">{{ updateTask.status }}</el-tag>
              <span style="font-size:13px;color:#909399">{{ updateTask.from_version }} → {{ updateTask.to_version }}</span>
            </div>
            <div v-if="updateTask.error" style="margin-top:4px;font-size:12px;color:#f56c6c">{{ updateTask.error }}</div>
          </div>
        </el-card>
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

      <!-- Core Process Tab (merged: binary management + process control + guardian) -->
      <el-tab-pane :label="t('service.tabs.process')" name="process">
        <CoreProcessTab :server-id="serverId" />
      </el-tab-pane>

      <!-- WARP Tab (per node) -->
      <el-tab-pane :label="t('service.tabs.warp')" name="warp">
        <WarpTab :server-id="serverId" />
      </el-tab-pane>

      <!-- Direct Domains Tab (per node) -->
      <el-tab-pane :label="t('nodeServer.directDomains')" name="shareDomain">
        <ShareDomainTab :server-id="serverId" />
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
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { ArrowLeft, Refresh, Upload } from '@element-plus/icons-vue'
import client from '../api/client'
import { getAgentVersion, getAgentReleases, updateAgent, getPendingAgentUpdates } from '../api/nodeServer'
import ProtocolCenter from './protocols/ProtocolCenter.vue'
import ProxyManagement from './ProxyManagement.vue'
import RuleTab from './services/RuleTab.vue'
import CoreProcessTab from './node-server/CoreProcessTab.vue'
import WarpTab from './services/WarpTab.vue'
import ShareDomainTab from './services/ShareDomainTab.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

const serverId = parseInt(route.params.id) || 0
const activeTab = ref('overview')
const server = ref(null)
const certificates = ref([])
const certLoading = ref(false)

// Agent update state
const panelAgentVersion = ref(null)
const panelVersionLoading = ref(false)
const githubReleases = ref([])
const releasesLoading = ref(false)
const selectedRelease = ref('')
const selectedAsset = ref('')
const updating = ref(false)
const updateTask = ref(null)
let pollTimer = null

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

const selectedReleaseAssets = computed(() => {
  if (!selectedRelease.value) return []
  const rel = githubReleases.value.find(r => r.version === selectedRelease.value)
  return rel?.assets || []
})

function nodeTypeLabel(nt) { return nodeTypeOptions[nt] || nt || '-' }
function statusLabel(s) { return statusLabels[s] || s || '-' }
function statusTag(s) {
  if (s === 'online') return 'success'
  if (s === 'degraded') return 'warning'
  if (s === 'pending') return 'info'
  return 'danger'
}
function updateStatusTag(s) {
  if (s === 'done') return 'success'
  if (s === 'failed') return 'danger'
  if (s === 'pending') return 'info'
  return 'warning'
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

async function loadPanelAgentVersion() {
  panelVersionLoading.value = true
  try {
    panelAgentVersion.value = await getAgentVersion()
  } catch {
    panelAgentVersion.value = null
  } finally {
    panelVersionLoading.value = false
  }
}

async function loadGithubReleases() {
  releasesLoading.value = true
  try {
    const data = await getAgentReleases(20)
    githubReleases.value = data?.releases || []
  } catch {
    githubReleases.value = []
  } finally {
    releasesLoading.value = false
  }
}

async function pushUpdateFromPanel() {
  if (!panelAgentVersion.value) return
  updating.value = true
  try {
    await updateAgent(serverId, {
      version: panelAgentVersion.value.version,
      download_url: '',
      checksum: '',
    })
    ElMessage.success('Update pushed')
    startPolling()
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || 'Update failed')
  } finally {
    updating.value = false
  }
}

async function pushUpdateFromGitHub() {
  if (!selectedAsset.value || !selectedRelease.value) return
  updating.value = true
  try {
    await updateAgent(serverId, {
      version: selectedRelease.value,
      download_url: selectedAsset.value,
      checksum: '',
    })
    ElMessage.success('Update pushed')
    startPolling()
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || 'Update failed')
  } finally {
    updating.value = false
  }
}

function startPolling() {
  stopPolling()
  pollTimer = setInterval(async () => {
    try {
      const data = await getPendingAgentUpdates()
      const tasks = data?.updates || []
      const task = tasks.find(t => t.server_id === serverId)
      if (task) {
        updateTask.value = task
        if (task.status === 'done' || task.status === 'failed') {
          stopPolling()
          loadServer()
        }
      } else {
        // No pending task — check if server version changed
        await loadServer()
        if (server.value?.agent_version) {
          stopPolling()
        }
      }
    } catch {
      // ignore polling errors
    }
  }, 3000)
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

onMounted(() => {
  loadServer()
  loadCerts()
  loadPanelAgentVersion()
  loadGithubReleases()
})

onUnmounted(() => {
  stopPolling()
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
