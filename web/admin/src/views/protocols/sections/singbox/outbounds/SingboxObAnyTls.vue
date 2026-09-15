<template>
  <el-form label-width="150px" label-position="left">
    <el-form-item :label="t('singbox.obProtocol.password')">
      <div style="display:flex;gap:8px;flex:1">
        <el-input v-model="model.password" @change="emit" />
        <el-button size="small" @click="model.password = generatePassword(); emit()">{{ t('singbox.users.generate') }}</el-button>
      </div>
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.padding')">
      <el-switch v-model="model._anytlsPadding" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.heuristic')">
      <el-input-number v-model="model.fingerprint_hashrule" :min="0" style="width:100%" @change="emit" />
    </el-form-item>
  </el-form>
</template>

<script setup>
// AnyTLS 出站：专属设置(Padding/Heuristic)；Base/TLS 由容器层提供
import { useI18n } from 'vue-i18n'
import { generatePassword } from '../utils/singboxDefaults'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>
