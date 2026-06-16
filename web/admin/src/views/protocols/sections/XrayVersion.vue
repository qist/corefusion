<template>
  <el-form label-width="120px" label-position="left">
    <el-form-item label="Min Version">
      <el-input v-model="form.min" placeholder="e.g. 1.8.0" @change="emit" />
    </el-form-item>
    <el-form-item label="Max Version">
      <el-input v-model="form.max" placeholder="e.g. 25.0.0" @change="emit" />
    </el-form-item>
    <el-alert title="Version constraints control which Xray-core versions are allowed to load this config. Leave empty for no限制." type="info" :closable="false" show-icon />
  </el-form>
</template>
<script setup>
import { reactive, onMounted } from 'vue'
const props = defineProps({ data: Object })
const emitFn = defineEmits(['update'])
const form = reactive({ min: '', max: '' })
function sync() { if (!props.data) return; form.min = props.data.min || ''; form.max = props.data.max || '' }
function emit() { emitFn('update', { ...form }) }
onMounted(sync)
</script>
