<template>
  <el-form label-width="160px" label-position="left">
    <!-- Token -->
    <el-form-item :label="t('singbox.protocol.cloudflared.token')">
      <el-input v-model="model.token" @change="emit" />
      <div class="hint">{{ t('singbox.protocol.cloudflared.tokenHint') }}</div>
    </el-form-item>

    <!-- High Availability -->
    <el-divider content-position="left">{{ t('singbox.protocol.cloudflared.haSection') }}</el-divider>
    <el-form-item :label="t('singbox.protocol.cloudflared.haConnections')">
      <el-input-number v-model="model.ha_connections" :min="0" :max="16" style="width:100%" @change="emit" />
      <div class="hint">{{ t('singbox.protocol.cloudflared.haConnectionsHint') }}</div>
    </el-form-item>

    <!-- Protocol -->
    <el-divider content-position="left">{{ t('singbox.protocol.cloudflared.protocolSection') }}</el-divider>
    <el-form-item :label="t('singbox.protocol.cloudflared.protocol')">
      <el-select v-model="model.protocol" style="width:100%" clearable @change="emit">
        <el-option label="auto" value="auto" />
        <el-option label="quic" value="quic" />
        <el-option label="http2" value="http2" />
        <el-option label="h2mux" value="h2mux" />
      </el-select>
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.cloudflared.postQuantum')">
      <el-switch v-model="model.post_quantum" @change="emit" />
      <span style="font-size:12px;color:#909399;margin-left:8px">{{ t('singbox.protocol.cloudflared.postQuantumHint') }}</span>
    </el-form-item>

    <!-- Edge IP / Datagram / Region -->
    <el-divider content-position="left">{{ t('singbox.protocol.cloudflared.edgeSection') }}</el-divider>
    <el-form-item :label="t('singbox.protocol.cloudflared.edgeIpVersion')">
      <el-select v-model="model._edgeIpVersion" style="width:100%" clearable @change="onEdgeIpVersionChange">
        <el-option :label="t('singbox.protocol.cloudflared.edgeIpDefault')" value="0" />
        <el-option label="IPv4" value="4" />
        <el-option label="IPv6" value="6" />
      </el-select>
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.cloudflared.datagramVersion')">
      <el-select v-model="model.datagram_version" style="width:100%" clearable @change="emit">
        <el-option label="v2" value="v2" />
        <el-option label="v3" value="v3" />
      </el-select>
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.cloudflared.region')">
      <el-input v-model="model.region" @change="emit" />
    </el-form-item>

    <!-- Grace Period -->
    <el-form-item :label="t('singbox.protocol.cloudflared.gracePeriod')">
      <el-input v-model="model.grace_period" placeholder="10s" @change="emit" />
      <div class="hint">{{ t('singbox.protocol.cloudflared.gracePeriodHint') }}</div>
    </el-form-item>

    <!-- Control Dialer -->
    <el-divider content-position="left">{{ t('singbox.protocol.cloudflared.controlDialer') }}</el-divider>
    <el-form-item :label="t('singbox.protocol.cloudflared.detour')">
      <el-input v-model="model._cfdControlDetour" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.cloudflared.bindInterface')">
      <el-input v-model="model._cfdControlBindInterface" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.cloudflared.routingMark')">
      <el-input-number v-model="model._cfdControlRoutingMark" :min="0" style="width:100%" @change="emit" />
    </el-form-item>

    <!-- Tunnel Dialer -->
    <el-divider content-position="left">{{ t('singbox.protocol.cloudflared.tunnelDialer') }}</el-divider>
    <el-form-item :label="t('singbox.protocol.cloudflared.detour')">
      <el-input v-model="model._cfdTunnelDetour" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.cloudflared.bindInterface')">
      <el-input v-model="model._cfdTunnelBindInterface" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.cloudflared.routingMark')">
      <el-input-number v-model="model._cfdTunnelRoutingMark" :min="0" style="width:100%" @change="emit" />
    </el-form-item>
  </el-form>
</template>

<script setup>
// Cloudflared (Cloudflare Tunnel) 入站协议专属设置
// sing-box 源码: /opt/sing-box/option/cloudflared.go
//   CloudflaredInboundOptions {
//     Token, HighAvailabilityConnections, Protocol, PostQuantum,
//     EdgeIPVersion, DatagramVersion, GracePeriod, Region,
//     ControlDialer, TunnelDialer
//   }
// 注意: cloudflared 入站不需要 listen_port（连接由 Cloudflare 边缘发起）
// 通用 Listen 由容器层提供
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
const props = defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }

function onEdgeIpVersionChange() {
  // "0" means default (unset) - convert to number for the enum field
  props.model.edge_ip_version = parseInt(props.model._edgeIpVersion) || 0
  emit()
}
</script>

<style scoped>
.hint { color: #909399; font-size: 12px; margin-top: 4px; }
</style>
