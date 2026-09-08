<template>
  <el-form label-width="140px" label-position="left">
    <el-form-item :label="t('singbox.protocol.snell.version')">
      <el-select v-model="model.version" style="width:100%" @change="emit">
        <el-option label="4" :value="4" />
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
    <el-form-item v-if="model.version === 5" :label="t('singbox.protocol.snell.obfsMode')">
      <el-select v-model="model.obfs" style="width:100%" clearable @change="emit">
        <el-option label="" value="" />
        <el-option label="none" value="none" />
        <el-option label="http" value="http" />
        <el-option label="tls" value="tls" />
      </el-select>
    </el-form-item>
  </el-form>
</template>

<script setup>
// Snell 协议专属设置（通用 Listen/Users 由容器层提供）
import { useI18n } from 'vue-i18n'
import { generateKey } from '../utils/singboxDefaults'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>