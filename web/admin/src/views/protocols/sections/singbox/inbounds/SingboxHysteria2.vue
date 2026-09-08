<template>
  <el-tabs>
    <el-tab-pane label="Listen" name="listen"><SingboxListen :model="model" @update="emit" /></el-tab-pane>
    <el-tab-pane label="Hysteria2" name="proto">
      <el-form label-width="140px" label-position="left">
        <el-form-item label="Up Mbps">
          <el-input v-model="model.up_mbps" placeholder="100" @change="emit" />
        </el-form-item>
        <el-form-item label="Down Mbps">
          <el-input v-model="model.down_mbps" placeholder="100" @change="emit" />
        </el-form-item>
        <el-form-item label="Ignore Client BW">
          <el-switch v-model="model.ignore_client_bandwidth" @change="emit" />
        </el-form-item>
        <el-form-item label="Masquerade">
          <el-input v-model="model.masquerade" placeholder="https://example.com or JSON" @change="emit" />
          <div class="hint">String URL or JSON object</div>
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

<style scoped>
.hint { color: #909399; font-size: 12px; margin-top: 4px; }
</style>