<template>
  <el-form label-width="140px" label-position="left">
    <el-form-item :label="t('singbox.protocol.tuic.congestionControl')">
      <el-select v-model="model.congestion_control" style="width:100%" @change="emit">
        <el-option label="bbr" value="bbr" />
        <el-option label="cubic" value="cubic" />
        <el-option label="new_reno" value="new_reno" />
      </el-select>
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.tuic.authTimeout')">
      <el-input v-model="model.auth_timeout" placeholder="10s" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.tuic.maxIdleTime')">
      <el-input v-model="model.max_idle_time" placeholder="60s" @change="emit" />
    </el-form-item>
  </el-form>
</template>

<script setup>
// TUIC 协议专属设置（通用 Listen/Users/TLS 由容器层提供）
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>