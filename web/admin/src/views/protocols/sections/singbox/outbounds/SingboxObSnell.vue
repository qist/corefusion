<template>
  <el-form label-width="150px" label-position="left">
    <el-form-item :label="t('singbox.obProtocol.version')">
      <el-input-number v-model="model.version" :min="1" :max="6" style="width:100%" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.psk')">
      <div style="display:flex;gap:8px;flex:1">
        <el-input v-model="model.psk" @change="emit" />
        <el-button size="small" @click="model.psk = generateKey(); emit()">{{ t('singbox.users.generate') }}</el-button>
      </div>
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.obfs')">
      <el-select v-model="model.obfs" style="width:100%" clearable @change="emit">
        <el-option label="none" value="none" />
        <el-option label="http" value="http" />
        <el-option label="tls" value="tls" />
      </el-select>
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.obfsHost')">
      <el-input v-model="model.obfs_host" placeholder="example.com" @change="emit" />
    </el-form-item>
    <el-form-item v-if="model.version >= 6" :label="t('singbox.obProtocol.mode')">
      <el-select v-model="model._snellMode" style="width:100%" clearable @change="emit">
        <el-option label="default" value="default" />
        <el-option label="unshaped" value="unshaped" />
        <el-option label="unsafe-raw" value="unsafe-raw" />
      </el-select>
    </el-form-item>
  </el-form>
</template>

<script setup>
// Snell 出站：专属设置(Version/PSK/Obfs/Mode)；Base/TLS 由容器层提供
import { useI18n } from 'vue-i18n'
import { generateKey } from '../utils/singboxDefaults'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>
