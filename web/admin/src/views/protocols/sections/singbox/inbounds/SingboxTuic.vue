<template>
  <el-tabs>
    <el-tab-pane label="Listen" name="listen"><SingboxListen :model="model" @update="emit" /></el-tab-pane>
    <el-tab-pane label="TUIC" name="proto">
      <el-form label-width="140px" label-position="left">
        <el-form-item label="Congestion Control">
          <el-select v-model="model.congestion_control" style="width:100%" @change="emit">
            <el-option label="bbr" value="bbr" />
            <el-option label="cubic" value="cubic" />
            <el-option label="new_reno" value="new_reno" />
          </el-select>
        </el-form-item>
        <el-form-item label="Auth Timeout">
          <el-input v-model="model.auth_timeout" placeholder="10s" @change="emit" />
        </el-form-item>
        <el-form-item label="Max Idle Time">
          <el-input v-model="model.max_idle_time" placeholder="60s" @change="emit" />
        </el-form-item>
      </el-form>
    </el-tab-pane>
    <el-tab-pane label="Users" name="users"><SingboxUsers :model="model" :access-host="accessHost" @update="emit" /></el-tab-pane>
    <el-tab-pane label="TLS" name="tls"><SingboxTls :model="model" :server-id="serverId" @update="emit" /></el-tab-pane>
  </el-tabs>
</template>

<script setup>
import SingboxListen from '../common/SingboxListen.vue'
import SingboxUsers from '../common/SingboxUsers.vue'
import SingboxTls from '../common/SingboxTls.vue'
defineProps({ model: { type: Object, required: true }, serverId: Number, accessHost: { type: String, default: '' } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>