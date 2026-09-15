<template>
  <el-form label-width="150px" label-position="left">
    <el-form-item :label="t('singbox.obProtocol.uuid')">
      <div style="display:flex;gap:8px;flex:1">
        <el-input v-model="model.uuid" @change="emit" />
        <el-button size="small" @click="model.uuid = generateUuid(); emit()">{{ t('singbox.users.generate') }}</el-button>
      </div>
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.encryption')">
      <el-select v-model="model.encryption" style="width:100%" @change="emit">
        <el-option label="none" value="none" />
      </el-select>
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.flow')">
      <el-select v-model="model.flow" style="width:100%" clearable @change="emit">
        <el-option label="" value="" />
        <el-option label="xtls-rprx-vision" value="xtls-rprx-vision" />
        <el-option label="xtls-rprx-vision-udp443" value="xtls-rprx-vision-udp443" />
      </el-select>
    </el-form-item>
  </el-form>
</template>

<script setup>
// VLESS 出站：专属设置(UUID/Encryption/Flow)；Base/TLS/Transport/Multiplex 由容器层提供
import { useI18n } from 'vue-i18n'
import { generateUuid } from '../utils/singboxDefaults'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>
