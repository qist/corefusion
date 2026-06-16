<template>
  <div>
    <el-divider content-position="left">Access Log</el-divider>
    <el-form label-width="140px" label-position="left">
      <el-form-item label="Output Mode">
        <el-radio-group v-model="accessMode" @change="onAccessModeChange">
          <el-radio value="file">File</el-radio>
          <el-radio value="stdout">Stdout</el-radio>
          <el-radio value="none">Disable</el-radio>
        </el-radio-group>
      </el-form-item>
      <el-form-item v-if="accessMode === 'file'" label="File Path">
        <el-input v-model="form.access" placeholder="/var/log/xray/access.log" @change="emit" />
      </el-form-item>
    </el-form>

    <el-divider content-position="left">Error Log</el-divider>
    <el-form label-width="140px" label-position="left">
      <el-form-item label="Output Mode">
        <el-radio-group v-model="errorMode" @change="onErrorModeChange">
          <el-radio value="file">File</el-radio>
          <el-radio value="stdout">Stdout</el-radio>
          <el-radio value="none">Disable</el-radio>
        </el-radio-group>
      </el-form-item>
      <el-form-item v-if="errorMode === 'file'" label="File Path">
        <el-input v-model="form.error" placeholder="/var/log/xray/error.log" @change="emit" />
      </el-form-item>
      <el-form-item label="Log Level">
        <el-select v-model="form.loglevel" style="width:100%" @change="emit">
          <el-option label="debug" value="debug" />
          <el-option label="info" value="info" />
          <el-option label="warning" value="warning" />
          <el-option label="error" value="error" />
          <el-option label="none" value="none" />
        </el-select>
      </el-form-item>
    </el-form>

    <el-divider content-position="left">Options</el-divider>
    <el-form label-width="140px" label-position="left">
      <el-form-item label="DNS Log">
        <el-switch v-model="form.dnsLog" @change="emit" />
        <span class="hint">Log DNS queries in error log</span>
      </el-form-item>
      <el-form-item label="Timestamp">
        <el-switch v-model="form.timestamp" @change="emit" />
        <span class="hint">Prefix log lines with timestamps</span>
      </el-form-item>
      <el-form-item label="Mask Address">
        <el-select v-model="form.maskAddress" style="width:100%" clearable @change="emit">
          <el-option label="Disabled" value="" />
          <el-option label="quarter (1.2.*.*)" value="quarter" />
          <el-option label="half (1.*.*.*)" value="half" />
          <el-option label="full ([Masked IPv4])" value="full" />
        </el-select>
      </el-form-item>
    </el-form>
  </div>
</template>
<script setup>
import { reactive, ref, onMounted } from 'vue'
const props = defineProps({ data: Object })
const emitFn = defineEmits(['update'])
const form = reactive({ access: '', error: '', loglevel: 'info', dnsLog: false, timestamp: true, maskAddress: '' })
const accessMode = ref('stdout')
const errorMode = ref('stdout')

function sync() {
  if (!props.data) return
  form.loglevel = props.data.loglevel || 'warning'
  form.dnsLog = props.data.dnsLog || false
  form.timestamp = props.data.timestamp !== false
  form.maskAddress = props.data.maskAddress || ''
  // Access log mode
  if (props.data.access === 'none' || props.data.access === '') {
    accessMode.value = props.data.access === 'none' ? 'none' : 'stdout'
    form.access = ''
  } else {
    accessMode.value = 'file'
    form.access = props.data.access
  }
  // Error log mode
  if (props.data.error === 'none' || props.data.error === '') {
    errorMode.value = props.data.error === 'none' ? 'none' : 'stdout'
    form.error = ''
  } else {
    errorMode.value = 'file'
    form.error = props.data.error
  }
}

function onAccessModeChange(val) {
  if (val === 'file') form.access = 'logs/xray-access.log'
  else if (val === 'stdout') form.access = ''
  else form.access = 'none'
  emit()
}

function onErrorModeChange(val) {
  if (val === 'file') form.error = 'logs/xray-error.log'
  else if (val === 'stdout') form.error = ''
  else form.error = 'none'
  emit()
}

function emit() {
  emitFn('update', {
    access: form.access,
    error: form.error,
    loglevel: form.loglevel,
    dnsLog: form.dnsLog,
    timestamp: form.timestamp,
    maskAddress: form.maskAddress || undefined,
  })
}

onMounted(sync)
</script>
<style scoped>.hint { margin-left: 12px; font-size: 12px; color: var(--el-text-color-secondary); }</style>
