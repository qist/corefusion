<template>
  <el-form label-width="160px" label-position="left">
    <el-form-item :label="t('singbox.obProtocol.up')">
      <el-input v-model="model.up" :placeholder="t('singbox.obProtocol.speedPlaceholder')" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.down')">
      <el-input v-model="model.down" :placeholder="t('singbox.obProtocol.speedPlaceholder')" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.upMbps')">
      <el-input-number v-model="model.up_mbps" :min="0" style="width:100%" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.downMbps')">
      <el-input-number v-model="model.down_mbps" :min="0" style="width:100%" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.obfs')">
      <el-input v-model="model.obfs" :placeholder="t('singbox.obProtocol.obfsHint')" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.auth')">
      <el-input v-model="model.auth_str" type="textarea" :rows="2" @change="emit" />
    </el-form-item>

    <el-divider content-position="left">{{ t('singbox.obProtocol.portHopping') }}</el-divider>
    <el-form-item :label="t('singbox.obProtocol.serverPorts')">
      <el-input v-model="model.server_ports" :placeholder="t('singbox.obProtocol.serverPortsHint')" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.hopInterval')">
      <el-input v-model="model.hop_interval" placeholder="30s" @change="emit" />
    </el-form-item>

    <el-divider content-position="left">{{ t('singbox.obProtocol.quicSection') }}</el-divider>
    <el-form-item :label="t('singbox.obProtocol.initialPacketSize')">
      <el-input-number v-model="model._hy2InitialPacketSize" :min="0" style="width:100%" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.disablePathMtu')">
      <el-switch v-model="model._hy2DisablePathMtu" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.idleTimeout')">
      <el-input v-model="model._hy2IdleTimeout" placeholder="30s" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.keepAlivePeriod')">
      <el-input v-model="model._hy2KeepAlivePeriod" placeholder="10s" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.streamRecvWindow')">
      <el-input v-model="model._hy2StreamRecvWindow" placeholder="8MB" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.connRecvWindow')">
      <el-input v-model="model._hy2ConnRecvWindow" placeholder="20MB" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.maxConcurrentStreams')">
      <el-input-number v-model="model._hy2MaxConcurrentStreams" :min="0" style="width:100%" @change="emit" />
    </el-form-item>
  </el-form>
</template>

<script setup>
// Hysteria v1 出站：option/hysteria.go HysteriaOutboundOptions
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>
