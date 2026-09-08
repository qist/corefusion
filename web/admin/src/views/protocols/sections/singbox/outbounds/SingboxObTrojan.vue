<template>
  <el-tabs>
    <el-tab-pane label="Basic" name="basic"><SingboxOutboundBase :model="model" @update="emit" /></el-tab-pane>
    <el-tab-pane label="Auth" name="auth">
      <el-form label-width="150px" label-position="left">
        <el-form-item label="Password">
          <div style="display:flex;gap:8px;flex:1">
            <el-input v-model="model.password" @change="emit" />
            <el-button size="small" @click="model.password = generatePassword(); emit()">Generate</el-button>
          </div>
        </el-form-item>
      </el-form>
    </el-tab-pane>
    <el-tab-pane label="TLS" name="tls"><SingboxOutboundTls :model="model" @update="emit" /></el-tab-pane>
    <el-tab-pane label="Transport" name="transport"><SingboxTransport :model="model" @update="emit" /></el-tab-pane>
    <el-tab-pane label="Multiplex" name="multiplex"><SingboxMultiplex :model="model" @update="emit" /></el-tab-pane>
  </el-tabs>
</template>

<script setup>
import SingboxOutboundBase from '../common/SingboxOutboundBase.vue'
import SingboxOutboundTls from '../common/SingboxOutboundTls.vue'
import SingboxTransport from '../common/SingboxTransport.vue'
import SingboxMultiplex from '../common/SingboxMultiplex.vue'
import { generatePassword } from '../utils/singboxDefaults'
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>