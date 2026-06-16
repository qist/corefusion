<template>
  <el-form label-width="120px" label-position="left">
    <el-form-item label="Listen Address">
      <el-input v-model="form.listen" placeholder="127.0.0.1:25601" />
    </el-form-item>
    <el-form-item label="Secret">
      <el-input v-model="form.secret" show-password />
    </el-form-item>
  </el-form>
</template>
<script setup>
import { reactive, watch, onMounted } from 'vue'
const props = defineProps({ data: Object })
const emit = defineEmits(['update'])
const form = reactive({ listen: '', secret: '' })
function syncFromData() {
  if (!props.data) return
  form.listen = props.data.listen || ''
  form.secret = props.data.secret || ''
}
watch(() => props.data, syncFromData, { deep: true })
watch(form, () => emit('update', { ...form }), { deep: true })
onMounted(syncFromData)
</script>
