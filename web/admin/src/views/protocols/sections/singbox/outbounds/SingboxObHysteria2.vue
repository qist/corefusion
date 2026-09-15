<template>
  <el-form label-width="150px" label-position="left">
    <el-form-item :label="t('singbox.obProtocol.password')">
      <div style="display:flex;gap:8px;flex:1">
        <el-input v-model="model.password" @change="emit" />
        <el-button size="small" @click="model.password = generatePassword(); emit()">{{ t('singbox.users.generate') }}</el-button>
      </div>
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.upMbps')">
      <el-input v-model="model['up_mbps']" :placeholder="t('singbox.obProtocol.speedPlaceholder')" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.downMbps')">
      <el-input v-model="model['down_mbps']" :placeholder="t('singbox.obProtocol.speedPlaceholder')" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.obfs')">
      <el-input v-model="model._hy2ObfsPassword" :placeholder="t('singbox.obProtocol.obfsPasswordHint')" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.disableMtuDiscovery')">
      <el-switch v-model="model.disable_mtu_discovery" @change="emit" />
    </el-form-item>
  </el-form>
</template>

<script setup>
// Hysteria2 出站：专属设置(Password/UpMbps/DownMbps/Obfs/DisableMtu)；Base/TLS 由容器层提供
import { useI18n } from 'vue-i18n'
import { generatePassword } from '../utils/singboxDefaults'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>
