<template>
  <!-- 优选探测设置：CF 优选域名 / CF IP 池 共用同一份后端配置 -->
  <el-card style="margin-bottom:16px">
    <template #header>
      <div style="display:flex;justify-content:space-between;align-items:center">
        <span style="font-weight:600">{{ t('service.cfip.probeTitle') }}</span>
        <div style="display:flex;gap:8px">
          <el-button size="small" type="primary" @click="saveProbeConfig">{{ t('common.save') }}</el-button>
        </div>
      </div>
    </template>
    <div style="display:flex;gap:16px;flex-wrap:wrap;align-items:center">
      <div>
        <div style="font-size:12px;color:#909399">{{ t('service.cfip.probeInterval') }}</div>
        <el-input-number v-model="probeCfg.interval_minutes" :min="1" :max="1440" size="small" style="width:130px" />
      </div>
      <div>
        <div style="font-size:12px;color:#909399">{{ t('service.cfip.probeMaxLatency') }}</div>
        <el-input-number v-model="probeCfg.max_latency_ms" :min="0" :max="60000" :step="50" size="small" style="width:150px" />
      </div>
      <div>
        <div style="font-size:12px;color:#909399">{{ t('service.cfip.probeEntryNode') }}</div>
        <el-select v-model="probeCfg.entry_node_id" size="small" style="width:220px" filterable>
          <el-option :label="t('service.cfip.probeLocal')" :value="0" />
          <el-option v-for="n in extNodes" :key="n.id" :label="`${n.name || n.host} (${n.protocol || ''})`" :value="n.id" />
        </el-select>
      </div>
      <div>
        <div style="font-size:12px;color:#909399">{{ t('service.cfip.probeDialer') }}</div>
        <el-select v-model="probeCfg.dialer" size="small" style="width:190px">
          <el-option :label="t('service.cfip.probeAuto')" value="" />
          <el-option :label="dialerLabel('singbox')" value="singbox" />
          <el-option :label="dialerLabel('xray')" value="xray" />
          <el-option :label="dialerLabel('mihomo')" value="mihomo" />
        </el-select>
      </div>
    </div>
    <div style="font-size:11px;color:#909399;margin-top:8px">{{ t('service.cfip.probeHint') }}</div>
  </el-card>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import client from '../../api/client'

const { t } = useI18n()

const probeCfg = ref({ interval_minutes: 5, max_latency_ms: 0, entry_node_id: 0, dialer: '', available_dialers: [] })
const extNodes = ref([])

async function fetchProbeConfig() {
  try {
    const d = await client.get('/cfip/probe-config') || {}
    probeCfg.value = {
      interval_minutes: d.interval_minutes ?? 5,
      max_latency_ms: d.max_latency_ms ?? 0,
      entry_node_id: d.entry_node_id ?? 0,
      dialer: d.dialer || '',
      available_dialers: d.dialers || [],
    }
  } catch { /* 未配置时用默认值 */ }
}

async function fetchExtNodes() {
  try {
    const d = await client.get('/external-nodes')
    extNodes.value = Array.isArray(d) ? d : (d?.items || [])
  } catch { extNodes.value = [] }
}

// 未安装的核心在下拉里直接标注，选中后检测会提示去安装。
function dialerLabel(name) {
  const installed = probeCfg.value.available_dialers || []
  return installed.includes(name) ? name : `${name} ${t('service.cfip.probeMissing')}`
}

async function saveProbeConfig() {
  try {
    await client.put('/cfip/probe-config', probeCfg.value)
    ElMessage.success(t('common.success'))
    await fetchProbeConfig()
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || t('common.failed'))
  }
}

onMounted(() => {
  fetchProbeConfig()
  fetchExtNodes()
})
</script>
