<template>
  <el-form label-width="150px" label-position="left">
    <el-form-item :label="t('singbox.obProtocol.method')">
      <el-select v-model="model.method" style="width:100%" filterable @change="emit">
        <el-option v-for="m in ssMethods" :key="m" :label="m" :value="m" />
      </el-select>
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.password')">
      <div style="display:flex;gap:8px;flex:1">
        <el-input v-model="model.password" @change="emit" />
        <el-button size="small" @click="model.password = generatePassword(); emit()">{{ t('singbox.users.generate') }}</el-button>
      </div>
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.packetEncoding')">
      <el-select v-model="model.packet_encoding" style="width:100%" clearable @change="emit">
        <el-option label="" value="" />
        <el-option label="xudp" value="xudp" />
        <el-option label="packet" value="packet" />
      </el-select>
    </el-form-item>
  </el-form>
</template>

<script setup>
// Shadowsocks 出站：专属设置(Method/Password/PacketEncoding)；Base/Multiplex 由容器层提供
import { useI18n } from 'vue-i18n'
import { generatePassword } from '../utils/singboxDefaults'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
const ssMethods = ['2022-blake3-aes-128-gcm', '2022-blake3-aes-256-gcm', '2022-blake3-chacha20-poly1305', 'aes-256-gcm', 'aes-128-gcm', 'chacha20-ietf-poly1305', 'none']
</script>