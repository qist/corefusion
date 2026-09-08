<template>
  <el-tabs>
    <el-tab-pane label="Listen" name="listen"><SingboxListen :model="model" @update="emit" /></el-tab-pane>
    <el-tab-pane label="Shadowsocks" name="proto">
      <el-form label-width="140px" label-position="left">
        <el-form-item label="Method">
          <el-select v-model="model.method" style="width:100%" filterable @change="emit">
            <el-option v-for="m in ssMethods" :key="m" :label="m" :value="m" />
          </el-select>
        </el-form-item>
        <el-form-item label="Network">
          <el-select v-model="model.network" style="width:100%" clearable @change="emit">
            <el-option label="" value="" />
            <el-option label="tcp" value="tcp" />
            <el-option label="udp" value="udp" />
            <el-option label="tcp,udp" value="tcp,udp" />
          </el-select>
        </el-form-item>
      </el-form>
    </el-tab-pane>
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
const ssMethods = ['2022-blake3-aes-128-gcm', '2022-blake3-aes-256-gcm', '2022-blake3-chacha20-poly1305', 'aes-256-gcm', 'aes-128-gcm', 'chacha20-ietf-poly1305', 'none']
</script>