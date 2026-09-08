<template>
  <el-tabs>
    <el-tab-pane label="Basic" name="basic"><SingboxOutboundBase :model="model" @update="emit" /></el-tab-pane>
    <el-tab-pane label="Shadowsocks" name="proto">
      <el-form label-width="150px" label-position="left">
        <el-form-item label="Method">
          <el-select v-model="model.method" style="width:100%" filterable @change="emit">
            <el-option v-for="m in ssMethods" :key="m" :label="m" :value="m" />
          </el-select>
        </el-form-item>
        <el-form-item label="Password">
          <div style="display:flex;gap:8px;flex:1">
            <el-input v-model="model.password" @change="emit" />
            <el-button size="small" @click="model.password = generatePassword(); emit()">Generate</el-button>
          </div>
        </el-form-item>
        <el-form-item label="Packet Encoding">
          <el-select v-model="model.packet_encoding" style="width:100%" clearable @change="emit">
            <el-option label="" value="" />
            <el-option label="xudp" value="xudp" />
            <el-option label="packet" value="packet" />
          </el-select>
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
const ssMethods = ['2022-blake3-aes-128-gcm', '2022-blake3-aes-256-gcm', '2022-blake3-chacha20-poly1305', 'aes-256-gcm', 'aes-128-gcm', 'chacha20-ietf-poly1305', 'none']
</script>