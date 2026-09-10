<template>
  <el-form label-width="160px" label-position="left">
    <!-- 基础参数 -->
    <el-form-item :label="t('singbox.protocol.tuic.congestionControl')">
      <el-select v-model="model.congestion_control" style="width:100%" @change="emit">
        <el-option label="BBR" value="bbr" />
        <el-option label="Cubic" value="cubic" />
        <el-option label="New Reno" value="new_reno" />
      </el-select>
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.tuic.authTimeout')">
      <el-input v-model="model.auth_timeout" placeholder="10s" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.tuic.heartbeat')">
      <el-input v-model="model.heartbeat" placeholder="10s" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.tuic.zeroRttHandshake')">
      <el-switch v-model="model.zero_rtt_handshake" @change="emit" />
    </el-form-item>

    <!-- QUIC 参数 -->
    <el-divider content-position="left">{{ t('singbox.protocol.tuic.quicSection') }}</el-divider>
    <el-form-item :label="t('singbox.protocol.tuic.initialPacketSize')">
      <el-input-number v-model="model._tuicInitialPacketSize" :min="0" style="width:100%" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.tuic.disablePathMtu')">
      <el-switch v-model="model._tuicDisablePathMtu" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.tuic.idleTimeout')">
      <el-input v-model="model._tuicIdleTimeout" placeholder="30s" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.tuic.keepAlivePeriod')">
      <el-input v-model="model._tuicKeepAlivePeriod" placeholder="10s" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.tuic.streamRecvWindow')">
      <el-input v-model="model._tuicStreamRecvWindow" placeholder="8MB" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.tuic.connRecvWindow')">
      <el-input v-model="model._tuicConnRecvWindow" placeholder="20MB" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.tuic.maxConcurrentStreams')">
      <el-input-number v-model="model._tuicMaxConcurrentStreams" :min="0" style="width:100%" @change="emit" />
    </el-form-item>
  </el-form>
</template>

<script setup>
// TUIC 协议专属设置
// sing-box 源码: TUICInboundOptions { ListenOptions, Users, CongestionControl, AuthTimeout, ZeroRTTHandshake, Heartbeat, InboundTLSOptionsContainer, QUICOptions }
// QUICOptions = HTTP2Options + {initial_packet_size, disable_path_mtu_discovery}
// HTTP2Options = {idle_timeout, keep_alive_period, stream_receive_window, connection_receive_window, max_concurrent_streams}
// 通用 Listen/Users/TLS 由容器层提供
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>
