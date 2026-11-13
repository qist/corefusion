<template>
  <el-form label-width="160px" label-position="left">
    <el-form-item :label="t('singbox.obProtocol.uuid')">
      <div style="display:flex;gap:8px;flex:1">
        <el-input v-model="model.uuid" @change="emit" />
        <el-button size="small" @click="model.uuid = generateUuid(); emit()">{{ t('singbox.users.generate') }}</el-button>
      </div>
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.password')">
      <div style="display:flex;gap:8px;flex:1">
        <el-input v-model="model.password" @change="emit" />
        <el-button size="small" @click="model.password = generatePassword(); emit()">{{ t('singbox.users.generate') }}</el-button>
      </div>
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.congestionControl')">
      <el-select v-model="model.congestion_control" style="width:100%" @change="emit">
        <el-option label="bbr" value="bbr" />
        <el-option label="cubic" value="cubic" />
        <el-option label="new_reno" value="new_reno" />
      </el-select>
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.udpRelayMode')">
      <el-select v-model="model.udp_relay_mode" style="width:100%" clearable :disabled="model.udp_over_stream" @change="emit">
        <el-option label="" value="" />
        <el-option label="native" value="native" />
        <el-option label="quic" value="quic" />
      </el-select>
      <div class="hint">{{ t('singbox.obProtocol.udpOverStreamHint') }}</div>
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.udpOverStream')">
      <el-switch v-model="model.udp_over_stream" @change="onUdpStreamChange" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.zeroRtt')">
      <el-switch v-model="model.zero_rtt_handshake" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.heartbeat')">
      <el-input v-model="model.heartbeat" placeholder="10s" @change="emit" />
    </el-form-item>

    <el-divider content-position="left">{{ t('singbox.obProtocol.quicSection') }}</el-divider>
    <el-form-item :label="t('singbox.obProtocol.initialPacketSize')">
      <el-input-number v-model="model._tuicInitialPacketSize" :min="0" style="width:100%" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.disablePathMtu')">
      <el-switch v-model="model._tuicDisablePathMtu" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.idleTimeout')">
      <el-input v-model="model._tuicIdleTimeout" placeholder="30s" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.keepAlivePeriod')">
      <el-input v-model="model._tuicKeepAlivePeriod" placeholder="10s" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.streamRecvWindow')">
      <el-input v-model="model._tuicStreamRecvWindow" placeholder="8MB" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.connRecvWindow')">
      <el-input v-model="model._tuicConnRecvWindow" placeholder="20MB" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.maxConcurrentStreams')">
      <el-input-number v-model="model._tuicMaxConcurrentStreams" :min="0" style="width:100%" @change="emit" />
    </el-form-item>
  </el-form>
</template>

<script setup>
// TUIC 出站：option/tuic.go TUICOutboundOptions
// 注意：auth_timeout / max_idle_time 是入站字段，出站没有；disable_sni 属于 tls。
import { useI18n } from 'vue-i18n'
import { generateUuid, generatePassword } from '../utils/singboxDefaults'
const { t } = useI18n()
const props = defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
// 二者互斥：开启 UDP over Stream 时清空 udp_relay_mode，避免 sing-box 报
// "udp_over_stream is conflict with udp_relay_mode"
function onUdpStreamChange(v) {
  if (v) props.model.udp_relay_mode = ''
  emit()
}
</script>

<style scoped>
.hint { color: #909399; font-size: 12px; margin-top: 4px; }
</style>
