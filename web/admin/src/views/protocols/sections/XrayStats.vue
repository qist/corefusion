<template>
  <el-form label-width="120px" label-position="left">
    <el-form-item label="Enable Stats">
      <el-switch v-model="enabled" @change="emitUpdate" />
      <span class="hint">Enable the traffic statistics engine</span>
    </el-form-item>
  </el-form>
</template>
<script setup>
import { ref, watch, onMounted } from 'vue'
const props = defineProps({ data: Object })
const emit = defineEmits(['update'])
const enabled = ref(false)
function syncFromData() { enabled.value = props.data && typeof props.data === 'object' }
function emitUpdate() { emit('update', enabled.value ? {} : null) }
watch(() => props.data, syncFromData)
onMounted(syncFromData)
</script>
<style scoped>.hint { margin-left: 12px; font-size: 12px; color: var(--el-text-color-secondary); }</style>
