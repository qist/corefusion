<template>
  <div>
    <!-- Caddy Management -->
<CaddyProxy v-if="activeProxy==='caddy'" :server-id="serverId" :is-local="isLocal" @back="switchProxy" />

<!-- Nginx Management -->
<NginxProxy v-else-if="activeProxy==='nginx'" :server-id="serverId" :is-local="isLocal" @back="switchProxy" />

    <!-- Selection Mode -->
    <div v-else>
      <h2 style="margin-bottom:20px">{{ t('proxy.title') }}</h2>

      <!-- Show which proxy is currently running -->
      <el-alert v-if="runningProxy" type="success" :closable="false" show-icon style="margin-bottom:20px;max-width:600px;margin-left:auto;margin-right:auto">
        {{ runningProxyName }} {{ t('proxy.isRunning') }}
      </el-alert>

      <div style="display:flex;gap:24px;justify-content:center;margin-top:60px">
        <el-card v-for="p in proxyOptions" :key="p.type" shadow="hover" class="proxy-select-card" @click="selectProxy(p.type)">
          <div style="text-align:center;padding:20px 40px">
            <div style="font-size:32px;margin-bottom:12px">{{ p.icon }}</div>
            <div style="font-size:18px;font-weight:600;margin-bottom:8px">
              {{ p.name }}
              <el-tag v-if="runningProxy === p.type" type="success" size="small" style="margin-left:4px">running</el-tag>
            </div>
            <div style="font-size:13px;color:#909399">{{ p.desc }}</div>
          </div>
        </el-card>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import CaddyProxy from './CaddyProxy.vue'
import NginxProxy from './NginxProxy.vue'
import client from '../api/client'

const { t } = useI18n()

const props = defineProps({
  serverId: { type: Number, required: true },
  isLocal: { type: Boolean, default: false }
})

const activeProxy = ref('')
const runningProxy = ref('')

// When serverId changes (user switches to a different node),
// reset the proxy selection so the correct proxy panel is shown
// based on the new node's actual running state.
watch(() => props.serverId, () => {
  activeProxy.value = ''
  fetchStatus()
})

const proxyOptions = [
  { type: 'nginx', name: 'Nginx', icon: '🌐', desc: t('proxy.nginxDesc') },
  { type: 'caddy', name: 'Caddy', icon: '🔒', desc: t('proxy.caddyDesc') }
]

const runningProxyName = computed(() => {
  const p = proxyOptions.find(o => o.type === runningProxy.value)
  return p ? p.name : ''
})

function selectProxy(type) {
  activeProxy.value = type
}

function switchProxy() {
  activeProxy.value = ''
  fetchStatus()
}

async function fetchStatus() {
  try {
    const status = await client.get(`/node-servers/${props.serverId}/proxy/status`)
    if (status.nginx && status.nginx.status === 'running') {
      runningProxy.value = 'nginx'
    } else if (status.caddy && status.caddy.status === 'running') {
      runningProxy.value = 'caddy'
    } else {
      runningProxy.value = ''
    }
    // Auto-select the running proxy if no manual selection has been made.
    // This ensures each node shows the correct proxy panel based on its
    // actual running state, rather than a stale localStorage value.
    if (!activeProxy.value && runningProxy.value) {
      activeProxy.value = runningProxy.value
    }
  } catch (e) {}
}

onMounted(() => {
  fetchStatus()
})
</script>

<style scoped>
.proxy-select-card { cursor:pointer; transition:all .2s; width:240px; }
.proxy-select-card:hover { border-color:#409eff; transform:translateY(-4px); box-shadow:0 8px 24px rgba(64,158,255,.15); }
</style>
