<template>
  <el-form label-width="180px" label-position="left">
    <el-form-item label="Max Idle Time (ms)">
      <el-input-number v-model="form.max_idle_time" :min="0" />
    </el-form-item>
    <el-form-item label="Auth Timeout (ms)">
      <el-input-number v-model="form.auth_timeout" :min="0" />
    </el-form-item>
    <el-form-item label="ALPN">
      <el-input v-model="alpnStr" placeholder="h3" @change="emitUpdate" />
    </el-form-item>
    <el-form-item label="Zero RTT Handshake">
      <el-switch v-model="form.zero_rtt_handshake" />
    </el-form-item>
    <el-form-item label="Heartbeat (ms)">
      <el-input-number v-model="form.heartbeat" :min="0" />
    </el-form-item>
  </el-form>
</template>
<script setup>
import { reactive, ref, watch, onMounted } from 'vue'
const props = defineProps({ data: Object })
const emit = defineEmits(['update'])
const form = reactive({ max_idle_time: 15000, auth_timeout: 1000, zero_rtt_handshake: false, heartbeat: 0 })
const alpnStr = ref('h3')

function syncFromData() {
  if (!props.data) return
  Object.keys(form).forEach(k => { if (props.data[k] !== undefined) form[k] = props.data[k] })
  alpnStr.value = (props.data.alpn || ['h3']).join(', ')
}

function emitUpdate() {
  emit('update', { ...form, alpn: alpnStr.value ? alpnStr.value.split(',').map(s => s.trim()) : [] })
}

watch(() => props.data, syncFromData, { deep: true })
watch(form, () => emitUpdate(), { deep: true })
onMounted(syncFromData)
</script>
