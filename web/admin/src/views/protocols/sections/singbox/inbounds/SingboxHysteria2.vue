<template>
  <el-form label-width="140px" label-position="left">
    <el-form-item :label="t('singbox.protocol.hysteria2.upMbps')">
      <el-input v-model="model.up_mbps" placeholder="100" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.hysteria2.downMbps')">
      <el-input v-model="model.down_mbps" placeholder="100" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.hysteria2.ignoreClientBw')">
      <el-switch v-model="model.ignore_client_bandwidth" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.hysteria2.masquerade')">
      <el-input v-model="model.masquerade" placeholder="https://example.com or JSON" @change="emit" />
      <div class="hint">{{ t('singbox.protocol.hysteria2.masqueradeHint') }}</div>
    </el-form-item>
  </el-form>
</template>

<script setup>
// Hysteria2 协议专属设置（通用 Listen/Users/TLS 由容器层提供）
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>

<style scoped>
.hint { color: #909399; font-size: 12px; margin-top: 4px; }
</style>