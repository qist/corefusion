<template>
  <div>
    <div class="preview-actions">
      <el-button size="small" @click="copyJson">Copy JSON</el-button>
      <el-button size="small" @click="downloadJson">Download</el-button>
    </div>
    <pre class="json-display">{{ formattedJson }}</pre>
  </div>
</template>
<script setup>
import { computed } from 'vue'
import { ElMessage } from 'element-plus'
const props = defineProps({ data: Object, coreType: String })
const formattedJson = computed(() => {
  try { return JSON.stringify(props.data, null, 2) } catch { return '{}' }
})
function copyJson() {
  navigator.clipboard.writeText(formattedJson.value)
  ElMessage.success('Copied')
}
function downloadJson() {
  const blob = new Blob([formattedJson.value], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url; a.download = `${props.coreType}-config.json`; a.click()
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
