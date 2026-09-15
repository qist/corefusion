<template>
  <el-form label-width="150px" label-position="left">
    <el-form-item :label="t('singbox.obProtocol.version')">
      <el-input-number v-model="model.version" :min="1" :max="3" style="width:100%" @change="emit" />
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
    <el-form-item :label="t('singbox.obProtocol.wildcardSni')">
      <el-select v-model="model._shadowtlsWildcardSni" style="width:100%" clearable @change="emit">
        <el-option :label="t('singbox.obProtocol.off')" value="off" />
        <el-option label="authed" value="authed" />
        <el-option label="all" value="all" />
      </el-select>
    </el-form-item>
  </el-form>
</template>

<script setup>
// ShadowTLS 出站：专属设置(Version/Password/StrictMode/WildcardSni)；Base/TLS 由容器层提供
import { useI18n } from 'vue-i18n'
import { generatePassword } from '../utils/singboxDefaults'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>
