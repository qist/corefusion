<template>
  <el-form label-width="150px" label-position="left" size="small">
    <el-form-item :label="t('singbox.outboundBase.detour')">
      <el-input v-model="model.detour" :placeholder="t('singbox.outboundBase.detourHint')" />
    </el-form-item>
    <el-form-item :label="t('singbox.outboundBase.bindInterface')">
      <el-input v-model="model.bind_interface" />
    </el-form-item>
    <el-form-item :label="t('singbox.outboundBase.inet4BindAddress')">
      <el-input v-model="model.inet4_bind_address" placeholder="0.0.0.0" />
    </el-form-item>
    <el-form-item :label="t('singbox.outboundBase.inet6BindAddress')">
      <el-input v-model="model.inet6_bind_address" placeholder="::" />
    </el-form-item>
    <el-form-item :label="t('singbox.outboundBase.bindAddressNoPort')">
      <el-switch v-model="model.bind_address_no_port" />
    </el-form-item>
    <el-form-item :label="t('singbox.outboundBase.protectPath')">
      <el-input v-model="model.protect_path" />
    </el-form-item>
    <el-form-item :label="t('singbox.outboundBase.routingMark')">
      <el-input-number v-model="model.routing_mark" :min="0" />
    </el-form-item>
    <el-form-item :label="t('singbox.outboundBase.reuseAddr')">
      <el-switch v-model="model.reuse_addr" />
    </el-form-item>
    <el-form-item :label="t('singbox.outboundBase.connectTimeout')">
      <el-input v-model="model.connect_timeout" placeholder="5s" />
    </el-form-item>
    <el-form-item :label="t('singbox.outboundBase.netns')">
      <el-input v-model="model.netns" />
    </el-form-item>
    <el-form-item :label="t('singbox.outboundBase.tcpFastOpen')">
      <el-switch v-model="model.tcp_fast_open" />
    </el-form-item>
    <el-form-item :label="t('singbox.outboundBase.tcpMultiPath')">
      <el-switch v-model="model.tcp_multi_path" />
    </el-form-item>
    <el-form-item :label="t('singbox.outboundBase.disableTcpKeepAlive')">
      <el-switch v-model="model.disable_tcp_keep_alive" />
    </el-form-item>
    <el-form-item :label="t('singbox.outboundBase.tcpKeepAlive')">
      <el-input v-model="model.tcp_keep_alive" placeholder="5m" />
    </el-form-item>
    <el-form-item :label="t('singbox.outboundBase.tcpKeepAliveInterval')">
      <el-input v-model="model.tcp_keep_alive_interval" placeholder="45s" />
    </el-form-item>
    <el-form-item :label="t('singbox.outboundBase.udpFragment')">
      <el-switch v-model="model.udp_fragment" />
    </el-form-item>
    <el-form-item :label="t('singbox.outboundBase.domainResolver')">
      <el-input v-model="model.domain_resolver" />
    </el-form-item>
    <el-form-item :label="t('singbox.outboundBase.networkStrategy')">
      <el-input v-model="model.network_strategy" placeholder="fallback" />
    </el-form-item>
    <el-form-item :label="t('singbox.outboundBase.networkType')">
      <el-input v-model="model._networkType" placeholder="wifi, ethernet" />
    </el-form-item>
    <el-form-item :label="t('singbox.outboundBase.fallbackNetworkType')">
      <el-input v-model="model._fallbackNetworkType" placeholder="cellular" />
    </el-form-item>
    <el-form-item :label="t('singbox.outboundBase.fallbackDelay')">
      <el-input v-model="model.fallback_delay" placeholder="300ms" />
    </el-form-item>
  </el-form>
</template>

<script setup>
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
</script>
