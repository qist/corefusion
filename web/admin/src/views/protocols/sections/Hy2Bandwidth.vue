<template>
  <el-form label-width="120px" label-position="left">
    <el-form-item label="Upload">
      <el-input v-model="form.up" placeholder="100 mbps" />
    </el-form-item>
    <el-form-item label="Download">
      <el-input v-model="form.down" placeholder="200 mbps" />
    </el-form-item>
  </el-form>
</template>
<script setup>
import { reactive, watch, onMounted } from 'vue'
const props = defineProps({ data: Object })
const emit = defineEmits(['update'])
const form = reactive({ up: '', down: '' })
function syncFromData() {
  if (!props.data) return
  form.up = props.data.up || ''
  form.down = props.data.down || ''
}
watch(() => props.data, syncFromData, { deep: true })
watch(form, () => emit('update', { ...form }), { deep: true })
onMounted(syncFromData)
</script>
