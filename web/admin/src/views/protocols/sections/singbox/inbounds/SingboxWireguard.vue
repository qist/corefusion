<template>
  <el-tabs>
    <el-tab-pane label="Listen" name="listen"><SingboxListen :model="model" @update="emit" /></el-tab-pane>
    <el-tab-pane label="WireGuard" name="proto">
      <el-form label-width="140px" label-position="left">
        <el-form-item label="Private Key">
          <div style="display:flex;gap:8px;flex:1">
            <el-input v-model="model._wg_private_key" @change="emit" />
            <el-button size="small" @click="model._wg_private_key = generateKey(); emit()">Generate</el-button>
          </div>
        </el-form-item>
        <el-form-item label="Address">
          <el-input v-model="model._wg_address" placeholder="10.0.0.1/24, fd00::1/64" @change="emit" />
        </el-form-item>
      </el-form>
    </el-tab-pane>
  </el-tabs>
</template>

<script setup>
import SingboxListen from '../common/SingboxListen.vue'
import { generateKey } from '../utils/singboxDefaults'
defineProps({ model: { type: Object, required: true }, serverId: Number, accessHost: { type: String, default: '' } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>