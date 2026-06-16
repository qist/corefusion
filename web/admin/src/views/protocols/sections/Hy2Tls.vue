<template>
  <el-form label-width="120px" label-position="left">
    <el-form-item label="Cert Path">
      <el-input v-model="form.cert" placeholder="/path/to/cert.pem" />
    </el-form-item>
    <el-form-item label="Key Path">
      <el-input v-model="form.key" placeholder="/path/to/key.pem" />
    </el-form-item>
    <el-form-item label="ALPN">
      <el-input v-model="alpnStr" placeholder="h3" @change="emitUpdate" />
    </el-form-item>
  </el-form>
</template>
<script setup>
import { reactive, ref, watch, onMounted } from 'vue'
const props = defineProps({ data: Object })
const emit = defineEmits(['update'])
const form = reactive({ cert: '', key: '' })
const alpnStr = ref('')
function syncFromData() {
  if (!props.data) return
  form.cert = props.data.cert || ''
  form.key = props.data.key || ''
  alpnStr.value = (props.data.alpn || []).join(', ')
}
function emitUpdate() {
  emit('update', { ...form, alpn: alpnStr.value ? alpnStr.value.split(',').map(s => s.trim()) : [] })
}
watch(() => props.data, syncFromData, { deep: true })
watch(form, () => emitUpdate(), { deep: true })
onMounted(syncFromData)
</script>
