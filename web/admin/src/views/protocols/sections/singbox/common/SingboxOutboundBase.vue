<template>
  <el-form label-width="150px" label-position="left">
    <el-form-item :label="t('singbox.outboundBase.tag')">
      <el-input v-model="model.tag" :placeholder="t('singbox.outboundBase.tagPlaceholder')" @change="emit" />
    </el-form-item>
    <template v-if="isProxy">
      <el-form-item :label="t('singbox.outboundBase.server')">
        <el-input v-model="model.server" :placeholder="t('singbox.outboundBase.serverPlaceholder')" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.outboundBase.serverPort')">
        <el-input-number v-model="model.server_port" :min="1" :max="65535" style="width:100%" @change="emit" />
      </el-form-item>
    </template>
    <!-- Dial Fields -->
    <template v-if="isProxy || showDial">
      <el-divider content-position="left">{{ t('singbox.outboundBase.dial') }}</el-divider>
      <el-form-item :label="t('singbox.outboundBase.network')">
        <el-select v-model="model.network" style="width:100%" clearable @change="emit">
          <el-option label="tcp" value="tcp" />
          <el-option label="udp" value="udp" />
          <el-option label="udp+tcp" value="tcp,udp" />
        </el-select>
      </el-form-item>
      <el-form-item :label="t('singbox.outboundBase.tcpFastOpen')">
        <el-switch v-model="model.tcp_fast_open" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.outboundBase.tcpMultiPath')">
        <el-switch v-model="model.tcp_multi_path" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.outboundBase.udpFragment')">
        <el-switch v-model="model.udp_fragment" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.outboundBase.bindInterface')">
        <el-input v-model="model.bind_interface" placeholder="eth0" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.outboundBase.routingMark')">
        <el-input-number v-model="model.routing_mark" :min="0" style="width:100%" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.outboundBase.domainResolver')">
        <el-input v-model="model.domain_resolver" :placeholder="t('singbox.outboundBase.domainResolver')" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.outboundBase.detour')">
        <el-input v-model="model.detour" :placeholder="t('singbox.outboundBase.detourHint')" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.outboundBase.inet4BindAddress')">
        <el-input v-model="model.inet4_bind_address" placeholder="0.0.0.0" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.outboundBase.inet6BindAddress')">
        <el-input v-model="model.inet6_bind_address" placeholder="::" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.outboundBase.bindAddressNoPort')">
        <el-switch v-model="model.bind_address_no_port" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.outboundBase.protectPath')">
        <el-input v-model="model.protect_path" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.outboundBase.reuseAddr')">
        <el-switch v-model="model.reuse_addr" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.outboundBase.netns')">
        <el-input v-model="model.netns" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.outboundBase.connectTimeout')">
        <el-input v-model="model.connect_timeout" placeholder="5s" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.outboundBase.disableTcpKeepAlive')">
        <el-switch v-model="model.disable_tcp_keep_alive" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.outboundBase.tcpKeepAlive')">
        <el-input v-model="model.tcp_keep_alive" placeholder="5m" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.outboundBase.tcpKeepAliveInterval')">
        <el-input v-model="model.tcp_keep_alive_interval" placeholder="45s" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.outboundBase.networkStrategy')">
        <el-select v-model="model.network_strategy" style="width:100%" clearable @change="emit">
          <el-option label="" value="" />
          <el-option label="default" value="default" />
          <el-option label="fallback" value="fallback" />
          <el-option label="hybrid" value="hybrid" />
        </el-select>
      </el-form-item>
      <el-form-item :label="t('singbox.outboundBase.networkType')">
        <el-select v-model="model._networkType" style="width:100%" multiple clearable @change="onNetworkTypeChange">
          <el-option label="wifi" value="wifi" />
          <el-option label="cellular" value="cellular" />
          <el-option label="ethernet" value="ethernet" />
          <el-option label="other" value="other" />
        </el-select>
      </el-form-item>
      <el-form-item :label="t('singbox.outboundBase.fallbackNetworkType')">
        <el-select v-model="model._fallbackNetworkType" style="width:100%" multiple clearable @change="onFallbackNetworkTypeChange">
          <el-option label="wifi" value="wifi" />
          <el-option label="cellular" value="cellular" />
          <el-option label="ethernet" value="ethernet" />
          <el-option label="other" value="other" />
        </el-select>
      </el-form-item>
      <el-form-item :label="t('singbox.outboundBase.fallbackDelay')">
        <el-input v-model="model.fallback_delay" placeholder="300ms" @change="emit" />
      </el-form-item>
    </template>
  </el-form>
</template>

<script setup>
// 出站基础字段（tag / server / port / dial）
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
const props = defineProps({
  model: { type: Object, required: true },
  isProxy: { type: Boolean, default: true },
  showDial: { type: Boolean, default: false },
})
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }

// el-select multiple 需要数组；表单内部存逗号字符串以便与 build/parse 对齐
function onNetworkTypeChange(v) {
  props.model._networkType = Array.isArray(v) ? v.join(',') : String(v || '')
  emit()
}
function onFallbackNetworkTypeChange(v) {
  props.model._fallbackNetworkType = Array.isArray(v) ? v.join(',') : String(v || '')
  emit()
}
</script>
