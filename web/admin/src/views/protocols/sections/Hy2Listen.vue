<template>
  <el-form label-width="120px" label-position="left">
    <el-form-item label="Listen Address">
      <el-input v-model="form.listen" placeholder=":443" />
    </el-form-item>
  </el-form>
</template>
<script setup>
import { reactive, watch, onMounted } from 'vue'
const props = defineProps({ data: Object })
const emit = defineEmits(['update'])
const form = reactive({ listen: ':443' })
function syncFromData() {
  if (typeof props.data === 'string') form.listen = props.data
  else if (props.data?.listen) form.listen = props.data.listen
}
watch(() => props.data, syncFromData, { deep: true })
watch(form, () => emit('update', form.listen), { deep: true })
onMounted(syncFromData)
</script>
