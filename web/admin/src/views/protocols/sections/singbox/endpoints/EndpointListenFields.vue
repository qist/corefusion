<template>
  <el-form label-width="150px" label-position="left" size="small">
    <el-form-item :label="t('singbox.listen.bindInterface')">
      <el-input v-model="model.bind_interface" />
    </el-form-item>
    <el-form-item :label="t('singbox.listen.routingMark')">
      <el-input-number v-model="model.routing_mark" :min="0" />
    </el-form-item>
    <el-form-item :label="t('singbox.listen.reuseAddr')">
      <el-switch v-model="model.reuse_addr" />
      <div class="hint">{{ t('singbox.listen.reuseAddrHint') }}</div>
    </el-form-item>
    <el-form-item :label="t('singbox.listen.netns')">
      <el-input v-model="model.netns" />
    </el-form-item>
    <el-form-item :label="t('singbox.outboundBase.tcpFastOpen')">
      <el-switch v-model="model.tcp_fast_open" />
    </el-form-item>
    <el-form-item :label="t('singbox.outboundBase.tcpMultiPath')">
      <el-switch v-model="model.tcp_multi_path" />
    </el-form-item>
    <el-form-item :label="t('singbox.listen.disableTcpKeepAlive')">
      <el-switch v-model="model.disable_tcp_keep_alive" />
    </el-form-item>
    <el-form-item :label="t('singbox.listen.tcpKeepAlive')">
      <el-input v-model="model.tcp_keep_alive" placeholder="5m" />
    </el-form-item>
    <el-form-item :label="t('singbox.listen.tcpKeepAliveInterval')">
      <el-input v-model="model.tcp_keep_alive_interval" placeholder="45s" />
    </el-form-item>
    <el-form-item :label="t('singbox.outboundBase.udpFragment')">
      <el-switch v-model="model.udp_fragment" />
    </el-form-item>
    <el-form-item :label="t('singbox.outboundBase.detour')">
      <el-input v-model="model.detour" :placeholder="t('singbox.outboundBase.detourHint')" />
    </el-form-item>
  </el-form>
</template>

<script setup>
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
</script>

<style scoped>
.hint { color: #909399; font-size: 12px; margin-top: 4px; }
</style>
