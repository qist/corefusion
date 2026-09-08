<template>
  <el-tabs>
    <el-tab-pane label="Listen" name="listen"><SingboxListen :model="model" @update="emit" /></el-tab-pane>
    <el-tab-pane label="Users" name="users"><SingboxUsers :model="model" :access-host="accessHost" @update="emit" /></el-tab-pane>
    <el-tab-pane label="Transport" name="transport"><SingboxTransport :model="model" @update="emit" /></el-tab-pane>
    <el-tab-pane label="TLS" name="tls"><SingboxTls :model="model" :server-id="serverId" @update="emit" /></el-tab-pane>
    <el-tab-pane label="Multiplex" name="multiplex"><SingboxMultiplex :model="model" @update="emit" /></el-tab-pane>
  </el-tabs>
</template>

<script setup>
import SingboxListen from '../common/SingboxListen.vue'
import SingboxUsers from '../common/SingboxUsers.vue'
import SingboxTransport from '../common/SingboxTransport.vue'
import SingboxTls from '../common/SingboxTls.vue'
import SingboxMultiplex from '../common/SingboxMultiplex.vue'
defineProps({ model: { type: Object, required: true }, serverId: Number, accessHost: { type: String, default: '' } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>