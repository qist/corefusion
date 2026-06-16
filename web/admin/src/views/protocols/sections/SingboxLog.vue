<template>
  <el-form label-width="120px" label-position="left">
    <el-form-item label="Disabled">
      <el-switch v-model="form.disabled" />
    </el-form-item>
    <el-form-item label="Level">
      <el-select v-model="form.level" style="width:100%">
        <el-option label="trace" value="trace" /><el-option label="debug" value="debug" />
        <el-option label="info" value="info" /><el-option label="warn" value="warn" />
        <el-option label="error" value="error" /><el-option label="fatal" value="fatal" />
      </el-select>
    </el-form-item>
    <el-form-item label="Output">
      <el-input v-model="form.output" placeholder="stderr" />
    </el-form-item>
    <el-form-item label="Timestamp">
      <el-switch v-model="form.timestamp" />
    </el-form-item>
  </el-form>
</template>
<script setup>
import { reactive, watch, onMounted } from 'vue'
const props = defineProps({ data: Object })
const emit = defineEmits(['update'])
const form = reactive({ disabled: false, level: 'info', output: 'stderr', timestamp: true })
function syncFromData() {
  if (!props.data) return
  Object.keys(form).forEach(k => { if (props.data[k] !== undefined) form[k] = props.data[k] })
}
watch(() => props.data, syncFromData, { deep: true })
watch(form, () => emit('update', { ...form }), { deep: true })
onMounted(syncFromData)
</script>
