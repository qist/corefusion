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

      <!-- Core Process Tab (merged: binary management + process control + guardian) -->
      <el-tab-pane :label="t('service.tabs.process')" name="process">
        <CoreProcessTab :server-id="serverId" />
      </el-tab-pane>

      <!-- WARP Tab (per node) -->
      <el-tab-pane :label="t('service.tabs.warp')" name="warp">
        <WarpTab :server-id="serverId" />
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
import ProtocolCenter from './protocols/ProtocolCenter.vue'
import ProxyManagement from './ProxyManagement.vue'
import RuleTab from './services/RuleTab.vue'
import CoreProcessTab from './node-server/CoreProcessTab.vue'
import WarpTab from './services/WarpTab.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

const serverId = parseInt(route.params.id) || 0
const activeTab = ref('overview')
const server = ref(null)
const certificates = ref([])
const certLoading = ref(false)

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

onMounted(() => {
  loadServer()
  loadCerts()
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
