<template>
  <el-form label-width="180px" label-position="left">
    <el-divider content-position="left">{{ t('singbox.protocol.tproxy.networkSection') }}</el-divider>
    <el-form-item :label="t('singbox.protocol.tproxy.network')">
      <el-select v-model="model._tproxyNetwork" style="width:100%" @change="emit">
        <el-option label="tcp" value="tcp" />
        <el-option label="udp" value="udp" />
        <el-option label="tcp,udp" value="tcp,udp" />
      </el-select>
    </el-form-item>

    <el-divider content-position="left">{{ t('singbox.protocol.tproxy.udpSection') }}</el-divider>
    <el-form-item :label="t('singbox.protocol.tproxy.udpMapping')">
      <el-select v-model="model._tproxyUdpMapping" style="width:100%" clearable @change="emit">
        <el-option label="" value="" />
        <el-option label="endpoint_independent" value="endpoint_independent" />
        <el-option label="address_dependent" value="address_dependent" />
        <el-option label="address_and_port_dependent" value="address_and_port_dependent" />
      </el-select>
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.tproxy.udpFiltering')">
      <el-select v-model="model._tproxyUdpFiltering" style="width:100%" clearable @change="emit">
        <el-option label="" value="" />
        <el-option label="endpoint_independent" value="endpoint_independent" />
        <el-option label="address_dependent" value="address_dependent" />
        <el-option label="address_and_port_dependent" value="address_and_port_dependent" />
      </el-select>
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.tproxy.udpNatMax')">
      <el-input-number v-model="model._tproxyUdpNatMax" :min="0" style="width:100%" @change="emit" />
    </el-form-item>
  </el-form>
</template>

<script setup>
// TProxy 协议专属设置
// sing-box 源码: TProxyInboundOptions { ListenOptions, Network, UDPMapping, UDPFiltering, UDPNATMax }
// 通用 Listen(tag/listen/listen_port/sniff/domain_strategy) 由容器层提供
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>
