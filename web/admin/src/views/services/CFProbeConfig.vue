<template>
  <!-- 优选探测设置：CF 优选域名 / CF IP 池 共用同一份后端配置 -->
  <div>
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
        <div style="font-size:12px;color:#909399">{{ t('service.cfip.probeEnabled') }}</div>
        <el-switch v-model="probeCfg.enabled" />
      </div>
      <div>
        <div style="font-size:12px;color:#909399">{{ t('service.cfip.probeInterval') }}</div>
        <el-input-number v-model="probeCfg.interval_minutes" :min="1" :max="1440" size="small" style="width:130px" :disabled="!probeCfg.enabled" />
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
          <el-option :label="dialerLabel('xray')" value="xray" />
          <el-option :label="dialerLabel('singbox')" value="singbox" />
          <el-option :label="dialerLabel('mihomo')" value="mihomo" />
        </el-select>
      </div>
    </div>
    <div style="font-size:11px;color:#909399;margin-top:8px">{{ t('service.cfip.probeHint') }}</div>
    <div style="font-size:11px;color:#909399;margin-top:4px">{{ t('service.cfip.probeMaxLatencyHint') }}</div>
    <div v-if="!probeCfg.enabled" style="font-size:11px;color:#e6a23c;margin-top:4px">{{ t('service.cfip.probeEnabledHint') }}</div>
  </el-card>

  <!-- 链路探测：国内出口 → CDN 域名/IP → 入站，验证整条链路真能代理流量 -->
  <el-card style="margin-bottom:16px">
    <template #header>
      <div style="display:flex;justify-content:space-between;align-items:center">
        <span style="font-weight:600">{{ t('service.cfip.chainTitle') }}</span>
        <div style="display:flex;gap:8px">
          <el-button size="small" :loading="chainChecking" @click="checkChain">{{ t('service.cfip.chainCheck') }}</el-button>
          <el-button size="small" type="primary" @click="saveChainConfig">{{ t('common.save') }}</el-button>
        </div>
      </div>
    </template>
    <div style="display:flex;gap:16px;flex-wrap:wrap;align-items:center">
      <div>
        <div style="font-size:12px;color:#909399">{{ t('service.cfip.chainEnabled') }}</div>
        <el-switch v-model="chainCfg.enabled" />
      </div>
      <div>
        <div style="font-size:12px;color:#909399">{{ t('service.cfip.chainExit') }}</div>
        <el-select v-model="chainCfg.exit" size="small" style="width:300px" filterable clearable :placeholder="t('service.cfip.chainExitPlaceholder')">
          <el-option-group :label="t('service.cfip.chainExitExternal')">
            <el-option v-for="o in externalExits" :key="o.value" :label="o.label" :value="o.value" />
          </el-option-group>
          <el-option-group :label="t('service.cfip.chainExitInbound')">
            <el-option v-for="o in inboundExits" :key="o.value" :label="o.label" :value="o.value" />
          </el-option-group>
        </el-select>
      </div>
      <div>
        <div style="font-size:12px;color:#909399">{{ t('service.cfip.chainInbound') }}</div>
        <el-select v-model="chainCfg.inbound" size="small" style="width:300px" filterable clearable :placeholder="t('service.cfip.chainInboundPlaceholder')">
          <el-option v-for="o in chainInbounds" :key="o.value" :label="o.label" :value="o.value">
            <span>{{ o.label }}</span>
            <span style="float:right;color:#909399;font-size:12px">{{ o.access_domain || o.access_type }}</span>
          </el-option>
        </el-select>
      </div>
      <div>
        <div style="font-size:12px;color:#909399">{{ t('service.cfip.chainConcurrency') }}</div>
        <el-input-number v-model="chainCfg.concurrency" :min="1" :max="8" size="small" style="width:120px" />
      </div>
      <div>
        <div style="font-size:12px;color:#909399">{{ t('service.cfip.chainMaxTargets') }}</div>
        <el-input-number v-model="chainCfg.max_targets" :min="1" :max="500" size="small" style="width:130px" />
      </div>
      <div style="min-width:320px;flex:1">
        <div style="font-size:12px;color:#909399">{{ t('service.cfip.chainProbeURL') }}</div>
        <el-input v-model="chainCfg.probe_url" size="small" placeholder="https://www.cloudflare.com/cdn-cgi/trace" />
      </div>
    </div>
    <div style="font-size:11px;color:#909399;margin-top:8px">{{ t('service.cfip.chainHint') }}</div>
    <div style="font-size:11px;color:#909399;margin-top:4px">{{ t('service.cfip.chainInboundHint') }}</div>
    <div style="font-size:11px;color:#909399;margin-top:4px">
      {{ t('service.cfip.chainDependsProbe') }}：{{ t('service.cfip.chainProbeInterval') }} {{ chainDeps.interval_minutes }} min
      · {{ t('service.cfip.chainProbeDialer') }} {{ chainDeps.effective_dialer || '-' }}
      <span v-if="!chainDeps.dialer">（{{ t('service.cfip.chainDialerAuto') }}）</span>
    </div>
    <div style="font-size:11px;color:#e6a23c;margin-top:4px">{{ t('service.cfip.chainAccountHint') }}</div>
    <div style="font-size:11px;color:#909399;margin-top:4px">
      {{ t('service.cfip.chainProbeLabel') }}：{{ chainProbeLabel }}
      · {{ t('service.cfip.chainTargetCount') }}：{{ chainTargetCount }}
      <span v-if="chainUpdatedAt"> · {{ t('onlineUsers.updatedAt') }} {{ formatTime(chainUpdatedAt) }}</span>
    </div>

    <el-table v-if="chainResults.length" :data="chainResults" size="small" border stripe style="margin-top:12px" max-height="420">
      <el-table-column :label="t('service.cfip.chainResultStatus')" width="90">
        <template #default="{ row }">
          <el-tag :type="row.healthy ? 'success' : 'danger'" size="small">
            {{ row.healthy ? t('service.cfip.chainHealthy') : t('service.cfip.chainUnhealthy') }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="t('service.cfip.chainResultCDN')" min-width="220" show-overflow-tooltip>
        <template #default="{ row }">
          <el-tag size="small" :type="row.kind === 'cf_ip' ? 'warning' : 'info'" effect="plain" style="margin-right:6px">
            {{ row.kind === 'cf_ip' ? t('service.cfip.chainKindIP') : t('service.cfip.chainKindDomain') }}
          </el-tag>
          <span style="font-family:monospace">{{ row.host }}:{{ row.port }}</span>
        </template>
      </el-table-column>
      <el-table-column :label="t('service.cfip.chainResultProbe')" min-width="200" show-overflow-tooltip>
        <template #default="{ row }">
          <span>{{ row.name || ('#' + row.probe_inbound_id) }}</span>
          <span style="color:#909399;font-size:12px"> · {{ row.protocol }}</span>
          <span v-if="row.node_name" style="color:#c0c4cc;font-size:12px"> @{{ row.node_name }}</span>
        </template>
      </el-table-column>
      <el-table-column :label="t('service.cfip.chainResultHTTP')" width="80" align="center">
        <template #default="{ row }">{{ row.http_status || '-' }}</template>
      </el-table-column>
      <el-table-column :label="t('service.cfip.chainResultLatency')" width="90" align="right">
        <template #default="{ row }">{{ row.latency_ms ? row.latency_ms + ' ms' : '-' }}</template>
      </el-table-column>
      <el-table-column :label="t('service.cfip.chainResultUp')" width="90" align="right">
        <template #default="{ row }">{{ formatBytes(row.up_bytes) }}</template>
      </el-table-column>
      <el-table-column :label="t('service.cfip.chainResultDown')" width="90" align="right">
        <template #default="{ row }">{{ formatBytes(row.down_bytes) }}</template>
      </el-table-column>
      <el-table-column :label="t('service.cfip.chainResultError')" min-width="160" show-overflow-tooltip>
        <template #default="{ row }"><span style="color:#f56c6c">{{ row.error || '' }}</span></template>
      </el-table-column>
      <el-table-column :label="t('service.cfip.chainResultCheckedAt')" width="160">
        <template #default="{ row }">{{ formatTime(row.checked_at) }}</template>
      </el-table-column>
    </el-table>
    <el-empty v-else :description="t('service.cfip.chainNoResult')" :image-size="60" />
  </el-card>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import client from '../../api/client'

const { t } = useI18n()

const probeCfg = ref({ enabled: false, interval_minutes: 5, max_latency_ms: 0, entry_node_id: 0, dialer: '', available_dialers: [] })
const extNodes = ref([])

const chainCfg = ref({
  enabled: false,
  exit: '',
  inbound: '',
  concurrency: 2,
  max_targets: 20,
  probe_url: 'https://www.cloudflare.com/cdn-cgi/trace',
})
const chainResults = ref([])
const chainUpdatedAt = ref(0)
const chainExits = ref([])
const chainInbounds = ref([])
const chainTargets = ref([])
const chainChecking = ref(false)
// 最近一轮实际使用的探针入站（目标列表按当前选择生成，这里记录真正跑过的那条）。
const chainProbe = ref('')
// 链路探测依赖「探测设置」的间隔与拨号核心，这两项只读展示。
const chainDeps = ref({ interval_minutes: 0, dialer: '', effective_dialer: '' })

const chainTargetCount = computed(() => chainTargets.value.length)
const externalExits = computed(() => chainExits.value.filter((o) => o.kind === 'external'))
const inboundExits = computed(() => chainExits.value.filter((o) => o.kind === 'inbound'))
// 最近一轮实际使用的探针入站（没跑过时退回当前选择）。
const chainProbeLabel = computed(() => {
  if (chainProbe.value) return chainProbe.value
  const picked = chainInbounds.value.find((o) => o.value === chainCfg.value.inbound)
  return picked ? picked.label : '-'
})

async function fetchProbeConfig() {
  try {
    const d = await client.get('/cfip/probe-config') || {}
    probeCfg.value = {
      enabled: !!d.enabled,
      interval_minutes: d.interval_minutes ?? 5,
      max_latency_ms: d.max_latency_ms ?? 0,
      entry_node_id: d.entry_node_id ?? 0,
      dialer: d.dialer || '',
      available_dialers: d.dialers || [],
    }
  } catch { /* 未配置时用默认值（默认关闭探测） */ }
}

async function fetchExtNodes() {
  try {
    const d = await client.get('/external-nodes')
    extNodes.value = Array.isArray(d) ? d : (d?.items || [])
  } catch { extNodes.value = [] }
}

async function fetchChainConfig() {
  try {
    const d = await client.get('/cfip/chain-probe-config') || {}
    if (d.config) {
      chainCfg.value = {
        enabled: !!d.config.enabled,
        exit: d.config.exit || '',
        inbound: d.config.inbound || '',
        concurrency: d.config.concurrency ?? 2,
        max_targets: d.config.max_targets ?? 20,
        probe_url: d.config.probe_url || 'https://www.cloudflare.com/cdn-cgi/trace',
      }
    }
    chainExits.value = d.exits || []
    chainInbounds.value = d.inbounds || []
    chainTargets.value = d.targets || []
    chainResults.value = sortResults(d.results || [])
    chainUpdatedAt.value = d.results_updated_at || 0
    chainProbe.value = d.probe || ''
    chainDeps.value = {
      interval_minutes: d.interval_minutes ?? 0,
      dialer: d.dialer || '',
      effective_dialer: d.effective_dialer || '',
    }
  } catch { /* 未配置时保留默认值 */ }
}

function sortResults(list) {
  return [...list].sort((a, b) => {
    if (a.healthy !== b.healthy) return a.healthy ? 1 : -1
    return (b.checked_at || 0) - (a.checked_at || 0)
  })
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

async function saveChainConfig() {
  try {
    await client.put('/cfip/chain-probe-config', chainCfg.value)
    ElMessage.success(t('common.success'))
    await fetchChainConfig()
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || t('common.failed'))
  }
}

async function checkChain() {
  if (!chainCfg.value.exit) {
    ElMessage.warning(t('service.cfip.chainNeedExit'))
    return
  }
  if (!chainCfg.value.inbound) {
    ElMessage.warning(t('service.cfip.chainNeedInbound'))
    return
  }
  // 手动检测走保存后的配置：先落库再跑，避免"页面选了但后端还是旧值"。
  try {
    await client.put('/cfip/chain-probe-config', chainCfg.value)
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || t('common.failed'))
    return
  }
  chainChecking.value = true
  try {
    const res = await client.post('/cfip/chain-probe/check', {}, { timeout: 180000 })
    const summary = res || {}
    if (summary.unhealthy) {
      ElMessage.warning(`${t('service.cfip.chainHealthy')} ${summary.healthy || 0} / ${t('service.cfip.chainUnhealthy')} ${summary.unhealthy}`)
    } else {
      ElMessage.success(`${t('service.cfip.chainHealthy')} ${summary.healthy || 0}`)
    }
    await fetchChainConfig()
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || t('common.failed'))
  } finally {
    chainChecking.value = false
  }
}

function formatTime(ts) {
  if (!ts) return '-'
  return new Date(ts * 1000).toLocaleString()
}

function formatBytes(n) {
  const v = Number(n) || 0
  if (v <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let i = 0
  let val = v
  while (val >= 1024 && i < units.length - 1) {
    val /= 1024
    i++
  }
  return `${val.toFixed(i ? 1 : 0)} ${units[i]}`
}

onMounted(() => {
  fetchProbeConfig()
  fetchExtNodes()
  fetchChainConfig()
})
</script>
