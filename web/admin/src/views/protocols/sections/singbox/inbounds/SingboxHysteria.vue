<template>
  <el-tabs>
    <el-tab-pane label="Listen" name="listen"><SingboxListen :model="model" @update="emit" /></el-tab-pane>
    <el-tab-pane label="Hysteria" name="proto">
      <el-form label-width="140px" label-position="left">
        <el-form-item label="Up">
          <el-input v-model="model.up" placeholder="100 Mbps" @change="emit" />
        </el-form-item>
        <el-form-item label="Down">
          <el-input v-model="model.down" placeholder="100 Mbps" @change="emit" />
        </el-form-item>
        <el-form-item label="Obfs">
          <el-select v-model="model.obfs" style="width:100%" clearable @change="emit">
            <el-option label="" value="" />
            <el-option label="salamander" value="salamander" />
            <el-option label="none" value="none" />
          </el-select>
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