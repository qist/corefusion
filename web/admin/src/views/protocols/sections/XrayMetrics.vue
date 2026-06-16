<template>
  <el-form label-width="120px" label-position="left">
    <el-form-item label="Tag">
      <el-input v-model="form.tag" placeholder="metrics_out" @change="emit" />
    </el-form-item>
    <el-form-item label="Listen">
      <el-input v-model="form.listen" placeholder="127.0.0.1:11111" @change="emit" />
    </el-form-item>
  </el-form>
</template>
<script setup>
import { reactive, onMounted } from 'vue'
const props = defineProps({ data: Object })
const emitFn = defineEmits(['update'])
const form = reactive({ tag: 'metrics_out', listen: '127.0.0.1:11111' })
function sync() { if (!props.data) return; form.tag = props.data.tag || ''; form.listen = props.data.listen || '' }
function emit() { emitFn('update', { ...form }) }
onMounted(sync)
</script>
