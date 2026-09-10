<template>
  <el-form label-width="140px" label-position="left">
    <el-form-item :label="t('singbox.protocol.snell.version')">
      <el-select v-model="model.version" style="width:100%" @change="emit">
        <el-option label="5" :value="5" />
        <el-option label="6" :value="6" />
      </el-select>
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.snell.psk')">
      <div style="display:flex;gap:8px;flex:1">
        <el-input v-model="model.psk" @change="emit" />
        <el-button size="small" @click="model.psk = generateKey(); emit()">{{ t('singbox.users.generate') }}</el-button>
      </div>
    </el-form-item>

    <!-- v5: obfs_mode -->
    <template v-if="model.version === 5">
      <el-divider content-position="left">v5 {{ t('singbox.protocol.snell.obfsSection') }}</el-divider>
      <el-form-item :label="t('singbox.protocol.snell.obfsMode')">
        <el-select v-model="model.obfs" style="width:100%" clearable @change="emit">
          <el-option label="" value="" />
          <el-option label="None" value="none" />
          <el-option label="HTTP" value="http" />
          <el-option label="TLS" value="tls" />
        </el-select>
      </el-form-item>
    </template>

    <!-- v6: mode -->
    <template v-if="model.version === 6">
      <el-divider content-position="left">v6 {{ t('singbox.protocol.snell.modeSection') }}</el-divider>
      <el-form-item :label="t('singbox.protocol.snell.mode')">
        <el-select v-model="model._snellMode" style="width:100%" @change="emit">
          <el-option label="Default" value="default" />
          <el-option label="Unshaped" value="unshaped" />
          <el-option label="Unsafe Raw" value="unsafe-raw" />
        </el-select>
      </el-form-item>
    </template>
  </el-form>
</template>

<script setup>
// Snell 协议专属设置
// sing-box 源码: SnellInboundOptions (discriminated union by version)
//   AbstractSnellInboundOptions { ListenOptions, PSK, Users[]SnellUser{name,userkey} }
//   v5 → SnellObfsServerOptions { obfs_mode: none|http|tls }
//   v6 → SnellV6Options { mode: default|unshaped|unsafe-raw }
// 注意：入站只支持 v5 和 v6（v4 仅出站）
// 通用 Listen/Users 由容器层提供
import { useI18n } from 'vue-i18n'
import { generateKey } from '../utils/singboxDefaults'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>
