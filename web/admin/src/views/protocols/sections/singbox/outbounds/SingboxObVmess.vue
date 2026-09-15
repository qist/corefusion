<template>
  <el-form label-width="150px" label-position="left">
    <el-form-item :label="t('singbox.obProtocol.uuid')">
      <div style="display:flex;gap:8px;flex:1">
        <el-input v-model="model.uuid" @change="emit" />
        <el-button size="small" @click="model.uuid = generateUuid(); emit()">{{ t('singbox.users.generate') }}</el-button>
      </div>
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.alterId')">
      <el-input-number v-model="model.alterId" :min="0" :max="65535" style="width:100%" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.security')">
      <el-select v-model="model.security" style="width:100%" @change="emit">
        <el-option label="auto" value="auto" />
        <el-option label="aes-128-gcm" value="aes-128-gcm" />
        <el-option label="chacha20-poly1305" value="chacha20-poly1305" />
        <el-option label="none" value="none" />
      </el-select>
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
// VMess 出站：专属设置(UUID/AlterID/Security/PacketEncoding)；Base/TLS/Transport/Multiplex 由容器层提供
import { useI18n } from 'vue-i18n'
import { generateUuid } from '../utils/singboxDefaults'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>
