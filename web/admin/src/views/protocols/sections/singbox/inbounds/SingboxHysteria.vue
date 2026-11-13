<template>
  <el-form label-width="140px" label-position="left">
    <el-form-item :label="t('singbox.protocol.hysteria.up')">
      <el-input v-model="model.up" placeholder="100 Mbps" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.hysteria.down')">
      <el-input v-model="model.down" placeholder="100 Mbps" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.hysteria.upMbps')">
      <el-input-number v-model="model.up_mbps" :min="0" style="width:100%" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.hysteria.downMbps')">
      <el-input-number v-model="model.down_mbps" :min="0" style="width:100%" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.hysteria.obfs')">
      <el-select v-model="model.obfs" style="width:100%" clearable @change="emit">
        <el-option label="" value="" />
        <el-option label="salamander" value="salamander" />
        <el-option label="none" value="none" />
      </el-select>
    </el-form-item>

    <el-divider content-position="left">{{ t('singbox.protocol.hysteria.quicSection') }}</el-divider>
    <el-form-item :label="t('singbox.protocol.hysteria.initialPacketSize')">
      <el-input-number v-model="model._hy2InitialPacketSize" :min="0" style="width:100%" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.hysteria.disablePathMtu')">
      <el-switch v-model="model._hy2DisablePathMtu" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.hysteria.idleTimeout')">
      <el-input v-model="model._hy2IdleTimeout" placeholder="30s" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.hysteria.keepAlivePeriod')">
      <el-input v-model="model._hy2KeepAlivePeriod" placeholder="10s" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.hysteria.streamRecvWindow')">
      <el-input v-model="model._hy2StreamRecvWindow" placeholder="8MB" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.hysteria.connRecvWindow')">
      <el-input v-model="model._hy2ConnRecvWindow" placeholder="20MB" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.hysteria.maxConcurrentStreams')">
      <el-input-number v-model="model._hy2MaxConcurrentStreams" :min="0" style="width:100%" @change="emit" />
    </el-form-item>
  </el-form>
</template>

<script setup>
// Hysteria 协议专属设置（option/hysteria.go：Up/Down/UpMbps/DownMbps/Obfs/QUICOptions）
// 用户凭据为 auth_str（用户列表里仍以 password 展示，build 时映射）。
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>
