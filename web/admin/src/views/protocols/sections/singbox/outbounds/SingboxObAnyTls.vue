<template>
  <el-form label-width="170px" label-position="left">
    <el-form-item :label="t('singbox.obProtocol.password')">
      <div style="display:flex;gap:8px;flex:1">
        <el-input v-model="model.password" @change="emit" />
        <el-button size="small" @click="model.password = generatePassword(); emit()">{{ t('singbox.users.generate') }}</el-button>
      </div>
    </el-form-item>
    <el-divider content-position="left">{{ t('singbox.obProtocol.anytlsSession') }}</el-divider>
    <el-form-item :label="t('singbox.obProtocol.idleSessionCheckInterval')">
      <el-input v-model="model.idle_session_check_interval" placeholder="30s" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.idleSessionTimeout')">
      <el-input v-model="model.idle_session_timeout" placeholder="30s" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.minIdleSession')">
      <el-input-number v-model="model.min_idle_session" :min="0" style="width:100%" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.clientMetadata')">
      <el-input v-model="model.client_metadata" :placeholder="t('singbox.obProtocol.clientMetadataHint')" @change="emit" />
    </el-form-item>
  </el-form>
</template>

<script setup>
// AnyTLS 出站（option/anytls.go: password / idle_session_check_interval /
// idle_session_timeout / min_idle_session / client_metadata）
// Base / TLS 由容器层提供；AnyTLS 出站没有 padding 字段（padding_scheme 仅入站有）。
import { useI18n } from 'vue-i18n'
import { generatePassword } from '../utils/singboxDefaults'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>
