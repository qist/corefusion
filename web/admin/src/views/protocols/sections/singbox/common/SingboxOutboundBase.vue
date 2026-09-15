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
    <template v-if="isProxy">
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
    </template>
  </el-form>
</template>

<script setup>
// 出站基础字段（tag / server / port / dial）
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
defineProps({
  model: { type: Object, required: true },
  isProxy: { type: Boolean, default: true },
})
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>
