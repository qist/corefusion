<template>
  <el-form label-width="120px" label-position="left">
    <el-form-item label="Listen Address">
      <el-input v-model="form.server" placeholder="[::]:443" />
    </el-form-item>
  </el-form>
</template>
<script setup>
import { reactive, watch, onMounted } from 'vue'
const props = defineProps({ data: String })
const emit = defineEmits(['update'])
const form = reactive({ server: '[::]:443' })
function syncFromData() {
  if (typeof props.data === 'string') form.server = props.data
}
watch(() => props.data, syncFromData)
watch(form, () => emit('update', form.server), { deep: true })
onMounted(syncFromData)
</script>
