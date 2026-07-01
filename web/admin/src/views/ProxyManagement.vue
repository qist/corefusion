<template>
  <div style="padding:20px">
    <!-- For local nodes, embed the full Proxy.vue (Caddy/Nginx management) -->
    <Proxy v-if="isLocal" :server-id="serverId" />

    <!-- For remote nodes, show proxy status and control -->
    <div v-else>
      <h3 style="margin-bottom:16px">{{ t('proxy.title') }}</h3>

      <!-- Proxy Status -->
      <el-card shadow="hover" style="margin-bottom:16px" body-style="padding:12px 20px">
        <div style="display:flex;align-items:center;gap:24px;font-size:13px;flex-wrap:wrap">
          <div>
            <span style="color:#909399">Caddy: </span>
            <el-tag size="small" :type="caddyStatus==='running'?'success':'info'">{{ caddyStatus || 'unknown' }}</el-tag>
          </div>
          <div>
            <span style="color:#909399">Nginx: </span>
            <el-tag size="small" :type="nginxStatus==='running'?'success':'info'">{{ nginxStatus || 'unknown' }}</el-tag>
          </div>
          <div style="margin-left:auto;display:flex;gap:8px">
            <el-button size="small" @click="loadStatus"><el-icon><Refresh /></el-icon> {{ t('common.refresh') }}</el-button>
          </div>
        </div>
      </el-card>

      <!-- Proxy Control -->
      <el-card shadow="never">
        <h4 style="margin-bottom:16px">{{ t('proxy.control') }}</h4>
        <el-table :data="proxyList" size="small">
          <el-table-column prop="name" :label="t('common.name')" width="120" />
          <el-table-column :label="t('nodeServer.status')" width="100">
            <template #default="{row}">
              <el-tag size="small" :type="row.status==='running'?'success':'info'">{{ row.status }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="t('common.actions')" width="280">
            <template #default="{row}">
              <el-button v-if="row.status!=='running'" type="success" size="small" :loading="controlLoading===row.type" @click="doControl(row.type,'start')">{{ t('protoCenter.status.start') }}</el-button>
              <el-button v-if="row.status==='running'" type="danger" size="small" :loading="controlLoading===row.type" @click="doControl(row.type,'stop')">{{ t('protoCenter.status.stop') }}</el-button>
              <el-button size="small" :loading="controlLoading===row.type" @click="doControl(row.type,'restart')">{{ t('protoCenter.status.restart') }}</el-button>
              <el-button size="small" :loading="controlLoading===row.type" @click="doControl(row.type,'reload')">{{ t('proxy.reload') }}</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-card>

      <!-- Sync Config -->
      <el-card shadow="never" style="margin-top:16px">
        <h4 style="margin-bottom:16px">{{ t('proxy.syncConfig') }}</h4>
        <div style="display:flex;gap:12px;align-items:center">
          <el-select v-model="syncProxyType" placeholder="Proxy Type" size="small" style="width:120px">
            <el-option label="Caddy" value="caddy" />
            <el-option label="Nginx" value="nginx" />
          </el-select>
          <el-button type="primary" size="small" :loading="syncing" @click="doSync">{{ t('proxy.syncNow') }}</el-button>
        </div>
      </el-card>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import client from '../api/client'
import Proxy from './Proxy.vue'

const { t } = useI18n()

const props = defineProps({
  serverId: { type: Number, default: 0 },
  isLocal: { type: Boolean, default: false }
})

const caddyStatus = ref('')
const nginxStatus = ref('')
const controlLoading = ref('')
const syncing = ref(false)
const syncProxyType = ref('caddy')

const proxyList = ref([
  { type: 'caddy', name: 'Caddy', status: 'unknown' },
  { type: 'nginx', name: 'Nginx', status: 'unknown' },
])

async function loadStatus() {
  try {
    const data = await client.get(`/node-servers/${props.serverId}/proxy/status`)
    caddyStatus.value = data?.caddy?.status || 'unknown'
    nginxStatus.value = data?.nginx?.status || 'unknown'
    proxyList.value[0].status = caddyStatus.value
    proxyList.value[1].status = nginxStatus.value
  } catch {
    // For remote nodes, status may not be available
  }
}

async function doControl(proxyType, action) {
  controlLoading.value = proxyType
  try {
    await client.post(`/node-servers/${props.serverId}/proxy/control`, { proxy_type: proxyType, action })
    ElMessage.success(t('common.success'))
    loadStatus()
  } catch (e) {
    ElMessage.error(e?.message || t('common.failed'))
  } finally {
    controlLoading.value = ''
  }
}

async function doSync() {
  syncing.value = true
  try {
    await client.post(`/node-servers/${props.serverId}/proxy/sync`, { proxy_type: syncProxyType.value })
    ElMessage.success(t('common.success'))
  } catch (e) {
    ElMessage.error(e?.message || t('common.failed'))
  } finally {
    syncing.value = false
  }
}

onMounted(() => {
  if (!props.isLocal) {
    loadStatus()
  }
})
</script>
