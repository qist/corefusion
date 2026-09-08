<template>
  <el-tabs>
    <el-tab-pane label="Listen" name="listen"><SingboxListen :model="model" @update="emit" /></el-tab-pane>
  </el-tabs>
</template>

<script setup>
import SingboxListen from '../common/SingboxListen.vue'
defineProps({ model: { type: Object, required: true }, serverId: Number, accessHost: { type: String, default: '' } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>