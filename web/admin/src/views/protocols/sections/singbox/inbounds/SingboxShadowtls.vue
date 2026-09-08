<template>
  <el-form label-width="140px" label-position="left">
    <el-form-item :label="t('singbox.protocol.shadowtls.version')">
      <el-select v-model="model.version" style="width:100%" @change="emit">
        <el-option label="1" :value="1" />
        <el-option label="2" :value="2" />
        <el-option label="3" :value="3" />
      </el-select>
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.shadowtls.password')">
      <div style="display:flex;gap:8px;flex:1">
        <el-input v-model="model.password" @change="emit" />
        <el-button size="small" @click="model.password = generatePassword(); emit()">{{ t('singbox.users.generate') }}</el-button>
      </div>
    </el-form-item>
  </el-form>
</template>

<script setup>
// ShadowTLS 协议专属设置（通用 Listen/Users/TLS 由容器层提供）
import { useI18n } from 'vue-i18n'
import { generatePassword } from '../utils/singboxDefaults'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>