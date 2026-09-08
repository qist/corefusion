<template>
  <el-form label-width="140px" label-position="left">
    <el-form-item label="Disabled">
      <el-switch v-model="form.disabled" @change="emit" />
      <span class="hint">Disable log output entirely</span>
    </el-form-item>
    <el-form-item label="Level">
      <el-select v-model="form.level" style="width:100%" @change="emit">
        <el-option label="trace" value="trace" />
        <el-option label="debug" value="debug" />
        <el-option label="info" value="info" />
        <el-option label="warn" value="warn" />
        <el-option label="error" value="error" />
        <el-option label="fatal" value="fatal" />
        <el-option label="panic" value="panic" />
      </el-select>
    </el-form-item>
    <el-form-item label="Output">
      <el-select v-model="outputMode" style="width:100%" @change="onOutputModeChange">
        <el-option label="stderr (default)" value="stderr" />
        <el-option label="file" value="file" />
        <el-option label="none (discard)" value="none" />
      </el-select>
    </el-form-item>
    <el-form-item v-if="outputMode === 'file'" label="File Path">
      <el-input v-model="form.output" placeholder="logs/singbox.log" @change="emit" />
    </el-form-item>
    <el-form-item label="Timestamp">
      <el-switch v-model="form.timestamp" @change="emit" />
    </el-form-item>
  </el-form>
</template>
<script setup>
import { reactive, ref, onMounted } from 'vue'
const props = defineProps({ data: Object })
const emitFn = defineEmits(['update'])
const form = reactive({ disabled: false, level: 'info', output: 'stderr', timestamp: true })
const outputMode = ref('stderr')

function sync() {
  if (!props.data) return
  form.disabled = props.data.disabled || false
  form.level = props.data.level || 'info'
  form.timestamp = props.data.timestamp !== false
  const out = props.data.output || ''
  if (out === '' || out === 'none') {
    outputMode.value = out === 'none' ? 'none' : 'stderr'
    form.output = ''
  } else {
    outputMode.value = out === 'stderr' ? 'stderr' : 'file'
    form.output = out === 'stderr' ? '' : out
  }
}

function onOutputModeChange(val) {
  if (val === 'file') form.output = 'logs/singbox.log'
  else if (val === 'stderr') form.output = ''
  else form.output = 'none'
  emit()
}

function emit() {
  const out = { level: form.level, timestamp: form.timestamp }
  if (form.disabled) out.disabled = true
  if (outputMode.value === 'file' && form.output) out.output = form.output
  else if (outputMode.value === 'none') out.output = 'none'
  else out.output = 'stderr'
  emitFn('update', out)
}

onMounted(sync)
</script>
<style scoped>.hint { margin-left: 12px; font-size: 12px; color: var(--el-text-color-secondary); }</style>
