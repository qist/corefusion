<template>
  <el-form label-width="120px" label-position="left">
    <el-form-item label="IP Pool (CIDR)">
      <el-input v-model="form.ipPool" placeholder="198.18.0.0/16" />
    </el-form-item>
    <el-form-item label="Pool Size">
      <el-input-number v-model="form.poolSize" :min="0" />
    </el-form-item>
  </el-form>
</template>
<script setup>
import { reactive, watch, onMounted } from 'vue'
const props = defineProps({ data: Object })
const emit = defineEmits(['update'])
const form = reactive({ ipPool: '', poolSize: 0 })
function syncFromData() {
  if (!props.data) return
  form.ipPool = props.data.ipPool || ''
  form.poolSize = props.data.poolSize || 0
}
watch(() => props.data, syncFromData, { deep: true })

onMounted(syncFromData)
</script>
