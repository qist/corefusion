<template>
  <div style="padding: 16px">
    <div style="display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap;margin-bottom:12px">
      <div style="display:flex;align-items:center;gap:10px;flex-wrap:wrap">
        <span style="font-size:15px;font-weight:600">{{ t('nodeCheckup.title') }}</span>
        <el-tag size="small" type="info">{{ t('nodeCheckup.lastCheck') }}: {{ lastCheckText }}</el-tag>
        <el-tag v-if="report" size="small">{{ envText }}</el-tag>
      </div>
      <div style="display:flex;gap:8px">
        <el-button size="small" :loading="loading" @click="load(true)">{{ t('nodeCheckup.refresh') }}</el-button>
        <el-button size="small" type="primary" plain @click="openGuide">{{ t('nodeCheckup.guide') }}</el-button>
        <el-button size="small" @click="openSettings">{{ t('nodeCheckup.settings') }}</el-button>
      </div>
    </div>

    <el-alert v-if="error" :title="error" type="error" :closable="false" show-icon style="margin-bottom:12px" />

    <el-alert
      type="info" :closable="false" show-icon
      style="margin-bottom:12px"
      :title="t('nodeCheckup.persistHint')" />

    <div v-if="report" style="display:flex;gap:8px;flex-wrap:wrap;margin-bottom:14px">
      <el-tag v-for="s in summaryTags" :key="s.key" :type="s.type" size="small">{{ s.label }}: {{ s.count }}</el-tag>
    </div>

    <el-empty v-if="!loading && !report && !error" :description="t('nodeCheckup.empty')" />

    <div v-for="group in groups" :key="group.category" style="margin-bottom:14px">
      <div style="font-size:13px;font-weight:600;color:#606266;margin:6px 0">{{ t('nodeCheckup.category.' + group.category) }}</div>
      <div v-for="it in group.items" :key="it.id" style="border:1px solid #ebeef5;border-radius:6px;padding:10px 12px;margin-bottom:8px">
        <div style="display:flex;align-items:center;gap:8px;flex-wrap:wrap">
          <el-tag size="small" :type="severityType(it.severity)">{{ t('nodeCheckup.severity.' + it.severity) }}</el-tag>
          <span style="font-weight:600">{{ it.id }}</span>
          <el-tag v-if="it.scope && it.scope !== 'host'" size="small" type="info">{{ scopeLabel(it.scope) }}</el-tag>
        </div>
        <div v-if="paramText(it)" style="font-size:12px;color:#606266;margin-top:6px;word-break:break-all">{{ paramText(it) }}</div>
        <el-collapse v-if="(it.evidence && it.evidence.length) || (it.commands && it.commands.length)" style="margin-top:6px">
          <el-collapse-item :title="t('nodeCheckup.evidence')" :name="it.id">
            <div v-for="(e, i) in it.evidence" :key="'e'+i" style="font-size:12px;color:#909399;font-family:monospace;white-space:pre-wrap">{{ e }}</div>
            <div v-for="(c, i) in it.commands" :key="'c'+i" style="margin-top:8px">
              <pre style="background:#f5f7fa;padding:8px;border-radius:4px;font-size:12px;white-space:pre-wrap;margin:0">{{ c.text }}</pre>
              <el-button size="small" link type="primary" @click="copy(c.text)">{{ t('nodeCheckup.copy') }}</el-button>
            </div>
          </el-collapse-item>
        </el-collapse>
      </div>
    </div>

    <el-dialog v-model="guideVisible" :title="t('nodeCheckup.guide')" width="860px" top="5vh">
      <div style="display:flex;gap:8px;margin-bottom:10px">
        <el-button size="small" type="primary" @click="copy(guideText)">{{ t('nodeCheckup.copyAll') }}</el-button>
        <el-button size="small" @click="downloadGuide">{{ t('nodeCheckup.download') }}</el-button>
      </div>
      <pre style="max-height:62vh;overflow:auto;background:#f5f7fa;padding:12px;border-radius:6px;font-size:12px;white-space:pre-wrap;margin:0">{{ guideText }}</pre>
    </el-dialog>

    <el-dialog v-model="settingsVisible" :title="t('nodeCheckup.settings')" width="520px">
      <el-form label-width="180px" size="small">
        <el-form-item :label="t('nodeCheckup.enabled')">
          <el-switch v-model="settings.enabled" />
        </el-form-item>
        <el-form-item :label="t('nodeCheckup.intervalHours')">
          <el-input-number v-model="settings.interval_hours" :min="1" :max="168" />
        </el-form-item>
        <el-divider content-position="left">{{ t('nodeCheckup.thresholds') }}</el-divider>
        <el-form-item :label="t('nodeCheckup.diskWarnPct')">
          <el-input-number v-model="settings.thresholds.disk_warn_pct" :min="1" :max="99" />
        </el-form-item>
        <el-form-item :label="t('nodeCheckup.diskHighPct')">
          <el-input-number v-model="settings.thresholds.disk_high_pct" :min="2" :max="100" />
        </el-form-item>
        <el-form-item :label="t('nodeCheckup.memAvailableWarnPct')">
          <el-input-number v-model="settings.thresholds.mem_available_warn_pct" :min="1" :max="99" />
        </el-form-item>
        <el-form-item :label="t('nodeCheckup.conntrackMin')">
          <el-input-number v-model="settings.thresholds.conntrack_min" :min="1024" :step="1024" />
        </el-form-item>
        <el-form-item :label="t('nodeCheckup.somaxconnMin')">
          <el-input-number v-model="settings.thresholds.somaxconn_min" :min="128" :step="128" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button size="small" @click="settingsVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button size="small" type="primary" :loading="saving" @click="saveSettings">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import client from '../../api/client'

const props = defineProps({ serverId: { type: Number, required: true } })
const { t } = useI18n()
const loading = ref(false)
const error = ref('')
const report = ref(null)

const summaryTags = computed(() => {
  const s = report.value?.summary || {}
  return [
    { key: 'high', count: s.high || 0, type: 'danger', label: t('nodeCheckup.severity.high') },
    { key: 'warn', count: s.warn || 0, type: 'warning', label: t('nodeCheckup.severity.warn') },
    { key: 'info', count: s.info || 0, type: 'info', label: t('nodeCheckup.severity.info') },
    { key: 'unknown', count: s.unknown || 0, type: 'info', label: t('nodeCheckup.severity.unknown') },
    { key: 'ok', count: s.ok || 0, type: 'success', label: t('nodeCheckup.severity.ok') },
  ]
})

const lastCheckText = computed(() => {
  if (!report.value?.generated_at) return t('nodeCheckup.never')
  return new Date(report.value.generated_at * 1000).toLocaleString()
})

const envText = computed(() => {
  const env = report.value?.env || {}
  const parts = []
  parts.push(env.kind === 'container' ? t('nodeCheckup.container') : t('nodeCheckup.host'))
  if (env.runtime) parts.push(env.runtime)
  if (env.kind === 'container' && !env.systemd) parts.push(t('nodeCheckup.noSystemd'))
  return parts.join(' · ')
})

const groups = computed(() => {
  const map = new Map()
  for (const it of report.value?.items || []) {
    const cat = it.category || 'system'
    if (!map.has(cat)) map.set(cat, [])
    map.get(cat).push(it)
  }
  return [...map.entries()].map(([category, items]) => ({ category, items }))
})

function severityType(sev) {
  if (sev === 'high') return 'danger'
  if (sev === 'warn') return 'warning'
  if (sev === 'info' || sev === 'unknown') return 'info'
  return 'success'
}

function scopeLabel(scope) {
  const key = 'nodeCheckup.scope.' + scope
  return t(key)
}

function paramText(it) {
  const p = it.params || {}
  const parts = []
  for (const [k, v] of Object.entries(p)) {
    if (v == null) continue
    if (Array.isArray(v)) parts.push(`${k}=${v.join(',')}`)
    else if (typeof v === 'object') continue
    else parts.push(`${k}=${v}`)
  }
  return parts.join('  ')
}

function copy(text) {
  navigator.clipboard.writeText(text)
  ElMessage.success(t('nodeCheckup.copied'))
}

async function load(refresh = false) {
  loading.value = true
  error.value = ''
  try {
    const url = `/node-servers/${props.serverId}/checkup${refresh ? '?refresh=1' : ''}`
    const data = await client.get(url)
    report.value = data
  } catch (e) {
    error.value = e?.response?.data?.message || e?.message || t('nodeCheckup.loadFailed')
  } finally {
    loading.value = false
  }
}

const guideVisible = ref(false)
const guideText = ref('')

async function openGuide() {
  guideVisible.value = true
  guideText.value = t('common.loading')
  try {
    const data = await client.get(`/node-servers/${props.serverId}/checkup/guide`)
    guideText.value = data?.markdown || ''
  } catch (e) {
    guideText.value = e?.response?.data?.message || e?.message || t('nodeCheckup.guideFailed')
  }
}

function downloadGuide() {
  const blob = new Blob([guideText.value], { type: 'text/markdown;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `corefusion-node-${props.serverId}-tuning.md`
  a.click()
  URL.revokeObjectURL(url)
}

const settingsVisible = ref(false)
const saving = ref(false)
const settings = ref({
  enabled: false,
  interval_hours: 24,
  thresholds: {
    disk_warn_pct: 85, disk_high_pct: 95, mem_available_warn_pct: 10,
    conntrack_min: 262144, somaxconn_min: 32768,
  },
})

async function openSettings() {
  try {
    const data = await client.get('/system/node-checkup')
    settings.value = {
      ...data,
      thresholds: { ...settings.value.thresholds, ...(data.thresholds || {}) },
    }
    settingsVisible.value = true
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || t('nodeCheckup.loadFailed'))
  }
}

async function saveSettings() {
  saving.value = true
  try {
    await client.put('/system/node-checkup', settings.value)
    ElMessage.success(t('common.success'))
    settingsVisible.value = false
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || t('nodeCheckup.saveFailed'))
  } finally {
    saving.value = false
  }
}

onMounted(() => load(false))
</script>
