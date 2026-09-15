<template>
  <el-form label-width="150px" label-position="left">
    <el-form-item :label="t('singbox.obProtocol.username')">
      <div style="display:flex;gap:8px;flex:1">
        <el-input v-model="model.username" @change="emit" />
        <el-button size="small" @click="model.username = generateUsername(); emit()">{{ t('singbox.users.generate') }}</el-button>
      </div>
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.password')">
      <div style="display:flex;gap:8px;flex:1">
        <el-input v-model="model.password" @change="emit" />
        <el-button size="small" @click="model.password = generatePassword(); emit()">{{ t('singbox.users.generate') }}</el-button>
      </div>
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.congestionControl')">
      <el-select v-model="model.congestion_control" style="width:100%" @change="emit">
        <el-option label="bbr" value="bbr" />
        <el-option label="cubic" value="cubic" />
      </el-select>
    </el-form-item>
  </el-form>
</template>

<script setup>
// NaiveProxy 出站：专属设置(Username/Password/CongestionControl)；Base/TLS 由容器层提供
import { useI18n } from 'vue-i18n'
import { generateUsername, generatePassword } from '../utils/singboxDefaults'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>
