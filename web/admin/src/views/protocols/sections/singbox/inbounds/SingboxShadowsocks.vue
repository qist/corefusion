<template>
  <el-form label-width="140px" label-position="left">
    <el-form-item :label="t('singbox.protocol.shadowsocks.method')">
      <el-select v-model="model.method" style="width:100%" filterable @change="emit">
        <el-option v-for="m in ssMethods" :key="m" :label="m" :value="m" />
      </el-select>
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.shadowsocks.network')">
      <el-select v-model="model.network" style="width:100%" clearable @change="emit">
        <el-option label="" value="" />
        <el-option label="tcp" value="tcp" />
        <el-option label="udp" value="udp" />
        <el-option label="tcp,udp" value="tcp,udp" />
      </el-select>
    </el-form-item>
  </el-form>
</template>

<script setup>
// Shadowsocks 协议专属设置（通用 Listen/Users/Transport/TLS/Multiplex 由容器层提供）
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
const ssMethods = ['2022-blake3-aes-128-gcm', '2022-blake3-aes-256-gcm', '2022-blake3-chacha20-poly1305', 'aes-256-gcm', 'aes-128-gcm', 'chacha20-ietf-poly1305', 'none']
</script>