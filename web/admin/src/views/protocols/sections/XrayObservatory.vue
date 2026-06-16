<template>
  <el-form label-width="140px" label-position="left">
    <el-form-item label="Subject Selector">
      <el-input v-model="selectorStr" placeholder="Comma-separated outbound tags" @change="emitUpdate" />
    </el-form-item>
    <el-form-item label="Probe URL">
      <el-input v-model="form.probeURL" placeholder="http://www.google.com/generate_204" />
    </el-form-item>
    <el-form-item label="Probe Interval">
      <el-input v-model="form.probeInterval" placeholder="120s" />
    </el-form-item>
    <el-form-item label="Concurrency">
      <el-switch v-model="form.enableConcurrency" />
    </el-form-item>
  </el-form>
</template>
<script setup>
import { reactive, ref, watch, onMounted } from 'vue'
const props = defineProps({ data: Object })
const emit = defineEmits(['update'])
const form = reactive({ probeURL: '', probeInterval: '120s', enableConcurrency: false, subjectSelector: [] })
const selectorStr = ref('')
function syncFromData() {
  if (!props.data) return
  form.probeURL = props.data.probeURL || ''
  form.probeInterval = props.data.probeInterval || '120s'
  form.enableConcurrency = props.data.enableConcurrency || false
  form.subjectSelector = props.data.subjectSelector || []
  selectorStr.value = form.subjectSelector.join(', ')
}
function emitUpdate() {
  form.subjectSelector = selectorStr.value.split(',').map(s => s.trim()).filter(Boolean)
  emit('update', { ...form })
}
watch(() => props.data, syncFromData, { deep: true })

onMounted(syncFromData)
</script>
