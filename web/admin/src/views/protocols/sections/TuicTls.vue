<template>
  <el-form label-width="120px" label-position="left">
    <el-form-item label="Certificate">
      <el-input v-model="cert" placeholder="/path/to/cert.pem" @change="emitUpdate" />
    </el-form-item>
    <el-form-item label="Private Key">
      <el-input v-model="key" placeholder="/path/to/key.pem" @change="emitUpdate" />
    </el-form-item>
  </el-form>
</template>
<script setup>
import { ref, watch, onMounted } from 'vue'
const props = defineProps({ data: Object })
const emit = defineEmits(['update'])
const cert = ref('')
const key = ref('')

function syncFromData() {
  if (!props.data) return
  cert.value = props.data.certificate || ''
  key.value = props.data.private_key || ''
}

function emitUpdate() {
  emit('update', { certificate: cert.value, private_key: key.value })
}

watch(() => props.data, syncFromData, { deep: true })
onMounted(syncFromData)
</script>
