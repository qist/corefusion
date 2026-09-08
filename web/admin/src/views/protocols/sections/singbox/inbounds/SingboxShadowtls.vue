<template>
  <el-tabs>
    <el-tab-pane label="Listen" name="listen"><SingboxListen :model="model" @update="emit" /></el-tab-pane>
    <el-tab-pane label="ShadowTLS" name="proto">
      <el-form label-width="140px" label-position="left">
        <el-form-item label="Version">
          <el-select v-model="model.version" style="width:100%" @change="emit">
            <el-option label="1" :value="1" />
            <el-option label="2" :value="2" />
            <el-option label="3" :value="3" />
          </el-select>
        </el-form-item>
        <el-form-item label="Password">
          <div style="display:flex;gap:8px;flex:1">
            <el-input v-model="model.password" @change="emit" />
            <el-button size="small" @click="model.password = generatePassword(); emit()">Generate</el-button>
          </div>
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
import { generatePassword } from '../utils/singboxDefaults'
defineProps({ model: { type: Object, required: true }, serverId: Number, accessHost: { type: String, default: '' } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>