<template>
  <el-form label-width="140px" label-position="left">
    <el-form-item :label="t('singbox.protocol.wireguard.privateKey')">
      <div style="display:flex;gap:8px;flex:1">
        <el-input v-model="model._wg_private_key" @change="emit" />
        <el-button size="small" @click="model._wg_private_key = generateKey(); emit()">{{ t('singbox.users.generate') }}</el-button>
      </div>
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.wireguard.address')">
      <el-input v-model="model._wg_address" :placeholder="t('singbox.protocol.wireguard.addressPlaceholder')" @change="emit" />
    </el-form-item>
  </el-form>
</template>

<script setup>
// WireGuard 协议专属设置（通用 Listen 由容器层提供）
import { useI18n } from 'vue-i18n'
import { generateKey } from '../utils/singboxDefaults'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>