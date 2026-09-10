<template>
  <el-form label-width="160px" label-position="left">
    <el-form-item :label="t('singbox.protocol.naive.network')">
      <el-select v-model="model.network" style="width:100%" clearable @change="emit">
        <el-option label="" value="" />
        <el-option label="TCP" value="tcp" />
        <el-option label="UDP" value="udp" />
        <el-option label="TCP + UDP" value="tcp,udp" />
      </el-select>
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.naive.quicCongestionControl')">
      <el-select v-model="model.quic_congestion_control" style="width:100%" clearable @change="emit">
        <el-option label="" value="" />
        <el-option label="BBR" value="bbr" />
        <el-option label="Cubic" value="cubic" />
        <el-option label="Reno" value="reno" />
      </el-select>
    </el-form-item>
  </el-form>
</template>

<script setup>
// Naive 协议专属设置（通用 Listen/Users/TLS 由容器层提供）
// sing-box 源码: NaiveInboundOptions { ListenOptions, Users, Network, QUICCongestionControl, InboundTLSOptionsContainer }
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>
