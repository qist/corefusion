<template>
  <div>
    <div class="preview-actions">
      <el-button size="small" @click="loadFromFile" :loading="loading">{{ t('protoCenter.preview.loadFromFile') }}</el-button>
      <el-button size="small" @click="copyJson">{{ t('protoCenter.preview.copyJson') }}</el-button>
      <el-button size="small" @click="downloadJson">{{ t('protoCenter.preview.download') }}</el-button>
    </div>
    <div v-if="fileSource" style="font-size:12px;color:#67c23a;margin-bottom:8px">✓ {{ t('protoCenter.preview.loadedFromFile') }}: {{ filePath }}</div>
    <pre class="json-display">{{ displayJson }}</pre>
  </div>
</template>
<script setup>
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import client from '../../../api/client'
const { t } = useI18n()
const props = defineProps({ data: Object, coreType: String, serverId: Number })
const fileContent = ref('')
const fileSource = ref(false)
const loading = ref(false)
const filePath = ref('')

const displayJson = computed(() => {
  if (fileSource.value && fileContent.value) return fileContent.value
  try { return JSON.stringify(props.data, null, 2) } catch { return '{}' }
})

async function loadFromFile() {
  loading.value = true
  try {
    const data = await client.get(`/node-servers/${props.serverId}/protocols/${props.coreType || 'xray'}/config-file`)
    if (typeof data === 'string') {
      fileContent.value = data
    } else {
      fileContent.value = JSON.stringify(data, null, 2)
    }
    fileSource.value = true
    filePath.value = `runtime/${props.coreType || 'xray'}.jsonc`
    ElMessage.success(t('protoCenter.preview.loaded'))
  } catch (e) {
    ElMessage.error(t('protoCenter.preview.loadFailed') + ': ' + (e.message || e))
  } finally { loading.value = false }
}

function copyJson() {
  navigator.clipboard.writeText(displayJson.value)
  ElMessage.success(t('protoCenter.preview.copied'))
}
function downloadJson() {
  const blob = new Blob([displayJson.value], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url; a.download = `${props.coreType || 'xray'}-config.json`; a.click()
  URL.revokeObjectURL(url)
}
</script>
<style scoped>
.preview-actions { margin-bottom: 12px; }
.json-display {
  background: var(--el-fill-color-light);
  border-radius: 6px;
  padding: 16px;
  font-size: 12px;
  line-height: 1.5;
  max-height: 600px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
