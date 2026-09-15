<template>
  <el-form label-width="150px" label-position="left">
    <!-- ShadowTLS 出站：仅 version / password / outbound TLS 的 server_name 等 -->
    <el-form-item :label="t('singbox.obProtocol.version')">
      <el-select v-model="model.version" style="width:100%" @change="emit">
        <el-option :label="t('singbox.obProtocol.v1')" :value="1" />
        <el-option :label="t('singbox.obProtocol.v2')" :value="2" />
        <el-option :label="t('singbox.obProtocol.v3')" :value="3" />
      </el-select>
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.password')">
      <div style="display:flex;gap:8px;flex:1">
        <el-input v-model="model.password" @change="emit" />
        <el-button size="small" @click="model.password = generatePassword(); emit()">{{ t('singbox.users.generate') }}</el-button>
      </div>
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.strictMode')">
      <el-switch v-model="model._shadowtlsStrictMode" @change="emit" />
    </el-form-item>
  </el-form>
</template>

<script setup>
// ShadowTLS 出站：专属设置(Version/Password/StrictMode)
import { useI18n } from 'vue-i18n'
import { generatePassword } from '../utils/singboxDefaults'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>
