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
        <el-option label="none" value="none" />
        <el-option label="zero" value="zero" />
        <el-option label="aes-128-cfb" value="aes-128-cfb" />
        <el-option label="aes-128-gcm" value="aes-128-gcm" />
        <el-option label="chacha20-poly1305" value="chacha20-poly1305" />
      </el-select>
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.packetEncoding')">
      <el-select v-model="model.packet_encoding" style="width:100%" clearable @change="emit">
        <el-option label="" value="" />
        <el-option label="packetaddr" value="packetaddr" />
        <el-option label="xudp" value="xudp" />
      </el-select>
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.globalPadding')">
      <el-switch v-model="model.global_padding" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.authenticatedLength')">
      <el-switch v-model="model.authenticated_length" @change="emit" />
    </el-form-item>
  </el-form>
</template>

<script setup>
// VMess 出站：UUID / AlterID / Security / PacketEncoding / Padding
// sing-box 源码 option/vmess.go：alter_id、global_padding、authenticated_length、packet_encoding(packetaddr|xudp)
import { useI18n } from 'vue-i18n'
import { generateUuid } from '../utils/singboxDefaults'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>
