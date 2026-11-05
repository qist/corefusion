<template>
  <el-form label-width="200px" label-position="left">
    <el-form-item :label="t('proto2.initialStreamWindow')">
      <el-input-number v-model="form.initialStreamReceiveWindow" :min="0" />
    </el-form-item>
    <el-form-item :label="t('proto2.maxStreamWindow')">
      <el-input-number v-model="form.maxStreamReceiveWindow" :min="0" />
    </el-form-item>
    <el-form-item :label="t('proto2.initialConnWindow')">
      <el-input-number v-model="form.initialConnectionReceiveWindow" :min="0" />
    </el-form-item>
    <el-form-item :label="t('proto2.maxConnWindow')">
      <el-input-number v-model="form.maxConnectionReceiveWindow" :min="0" />
    </el-form-item>
  </el-form>
</template>
<script setup>
import { reactive, watch, onMounted } from 'vue'
const props = defineProps({ data: Object })
const emit = defineEmits(['update'])
const form = reactive({ initialStreamReceiveWindow: 0, maxStreamReceiveWindow: 0, initialConnectionReceiveWindow: 0, maxConnectionReceiveWindow: 0 })
function syncFromData() {
  if (!props.data) return
  Object.keys(form).forEach(k => { if (props.data[k] !== undefined) form[k] = props.data[k] })
}
watch(() => props.data, syncFromData, { deep: true })
watch(form, () => emit('update', { ...form }), { deep: true })
onMounted(syncFromData)
</script>
