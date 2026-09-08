<template>
  <el-tabs>
    <el-tab-pane label="Listen" name="listen"><SingboxListen :model="model" @update="emit" /></el-tab-pane>
    <el-tab-pane label="Snell" name="proto">
      <el-form label-width="140px" label-position="left">
        <el-form-item label="Version">
          <el-select v-model="model.version" style="width:100%" @change="emit">
            <el-option label="4" :value="4" />
            <el-option label="5" :value="5" />
            <el-option label="6" :value="6" />
          </el-select>
        </el-form-item>
        <el-form-item label="PSK">
          <div style="display:flex;gap:8px;flex:1">
            <el-input v-model="model.psk" @change="emit" />
            <el-button size="small" @click="model.psk = generateKey(); emit()">Generate</el-button>
          </div>
        </el-form-item>
        <el-form-item v-if="model.version === 5" label="Obfs Mode">
          <el-select v-model="model.obfs" style="width:100%" clearable @change="emit">
            <el-option label="" value="" />
            <el-option label="none" value="none" />
            <el-option label="http" value="http" />
            <el-option label="tls" value="tls" />
          </el-select>
        </el-form-item>
      </el-form>
    </el-tab-pane>
    <el-tab-pane label="Users" name="users"><SingboxUsers :model="model" :access-host="accessHost" @update="emit" /></el-tab-pane>
  </el-tabs>
</template>

<script setup>
import SingboxListen from '../common/SingboxListen.vue'
import SingboxUsers from '../common/SingboxUsers.vue'
import { generateKey } from '../utils/singboxDefaults'
defineProps({ model: { type: Object, required: true }, serverId: Number, accessHost: { type: String, default: '' } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>