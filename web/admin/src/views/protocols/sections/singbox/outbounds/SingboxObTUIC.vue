<template>
  <el-tabs>
    <el-tab-pane label="Basic" name="basic"><SingboxOutboundBase :model="model" @update="emit" /></el-tab-pane>
    <el-tab-pane label="TUIC" name="proto">
      <el-form label-width="150px" label-position="left">
        <el-form-item label="UUID">
          <div style="display:flex;gap:8px;flex:1">
            <el-input v-model="model.uuid" @change="emit" />
            <el-button size="small" @click="model.uuid = generateUuid(); emit()">Generate</el-button>
          </div>
        </el-form-item>
        <el-form-item label="Password">
          <div style="display:flex;gap:8px;flex:1">
            <el-input v-model="model.password" @change="emit" />
            <el-button size="small" @click="model.password = generatePassword(); emit()">Generate</el-button>
          </div>
        </el-form-item>
        <el-form-item label="Congestion Control">
          <el-select v-model="model.congestion_control" style="width:100%" @change="emit">
            <el-option label="bbr" value="bbr" />
            <el-option label="cubic" value="cubic" />
            <el-option label="new_reno" value="new_reno" />
          </el-select>
        </el-form-item>
      </el-form>
    </el-tab-pane>
    <el-tab-pane label="TLS" name="tls"><SingboxOutboundTls :model="model" @update="emit" /></el-tab-pane>
  </el-tabs>
</template>

<script setup>
import SingboxOutboundBase from '../common/SingboxOutboundBase.vue'
import SingboxOutboundTls from '../common/SingboxOutboundTls.vue'
import { generateUuid, generatePassword } from '../utils/singboxDefaults'
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>