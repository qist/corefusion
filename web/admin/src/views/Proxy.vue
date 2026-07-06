<template>
  <div>
    <!-- Caddy Management -->
<CaddyProxy v-if="activeProxy==='caddy'" :server-id="serverId" :is-local="isLocal" @back="switchProxy" />

<!-- Nginx Management -->
<NginxProxy v-else-if="activeProxy==='nginx'" :server-id="serverId" :is-local="isLocal" @back="switchProxy" />

    <!-- Selection Mode -->
    <div v-else>
      <h2 style="margin-bottom:20px">{{ t('proxy.title') }}</h2>
      <div style="display:flex;gap:24px;justify-content:center;margin-top:60px">
        <el-card v-for="p in proxyOptions" :key="p.type" shadow="hover" class="proxy-select-card" @click="selectProxy(p.type)">
          <div style="text-align:center;padding:20px 40px">
            <div style="font-size:32px;margin-bottom:12px">{{ p.icon }}</div>
            <div style="font-size:18px;font-weight:600;margin-bottom:8px">{{ p.name }}</div>
            <div style="font-size:13px;color:#909399">{{ p.desc }}</div>
          </div>
        </el-card>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import CaddyProxy from './CaddyProxy.vue'
import NginxProxy from './NginxProxy.vue'

const { t } = useI18n()

const props = defineProps({
  serverId: { type: Number, required: true },
  isLocal: { type: Boolean, default: false }
})

const activeProxy = ref(localStorage.getItem('active_proxy') || '')

const proxyOptions = [
  { type: 'nginx', name: 'Nginx', icon: '🌐', desc: t('proxy.nginxDesc') },
  { type: 'caddy', name: 'Caddy', icon: '🔒', desc: t('proxy.caddyDesc') }
]

function selectProxy(type) {
  activeProxy.value = type
  localStorage.setItem('active_proxy', type)
}

function switchProxy() {
  activeProxy.value = ''
  localStorage.removeItem('active_proxy')
}

onMounted(() => {
  // Restore last selection
  const saved = localStorage.getItem('active_proxy')
  if (saved) activeProxy.value = saved
})
</script>

<style scoped>
.proxy-select-card { cursor:pointer; transition:all .2s; width:240px; }
.proxy-select-card:hover { border-color:#409eff; transform:translateY(-4px); box-shadow:0 8px 24px rgba(64,158,255,.15); }
</style>
