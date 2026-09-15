<template>
  <el-form label-width="150px" label-position="left">
    <el-form-item :label="t('singbox.obProtocol.upMbps')">
      <el-input v-model="model.up" :placeholder="t('singbox.obProtocol.speedPlaceholder')" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.downMbps')">
      <el-input v-model="model.down" :placeholder="t('singbox.obProtocol.speedPlaceholder')" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.obfs')">
      <el-input v-model="model.obfs" :placeholder="t('singbox.obProtocol.obfsHint')" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.auth')">
      <el-input v-model="model.auth_str" type="textarea" :rows="2" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.recvWindowConn')">
      <el-input-number v-model="model.recv_window_conn" :min="0" style="width:100%" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.recvWindow')">
      <el-input-number v-model="model.recv_window" :min="0" style="width:100%" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.disableMtuDiscovery')">
      <el-switch v-model="model.disable_mtu_discovery" @change="emit" />
    </el-form-item>
  </el-form>
</template>

<script setup>
// Hysteria 出站：专属设置(Up/Down/Obfs/Auth/RecvWindow/DisableMtu)；Base/TLS 由容器层提供
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>
