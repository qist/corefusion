<template>
  <el-form label-width="150px" label-position="left">
    <el-form-item :label="t('singbox.obProtocol.uuid')">
      <div style="display:flex;gap:8px;flex:1">
        <el-input v-model="model.uuid" @change="emit" />
        <el-button size="small" @click="model.uuid = generateUuid(); emit()">{{ t('singbox.users.generate') }}</el-button>
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
        <el-option label="new_reno" value="new_reno" />
      </el-select>
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.udpRelayMode')">
      <el-select v-model="model.udp_relay_mode" style="width:100%" clearable @change="emit">
        <el-option label="" value="" />
        <el-option label="native" value="native" />
        <el-option label="quic" value="quic" />
      </el-select>
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.zeroRtt')">
      <el-switch v-model="model.zero_rtt_handshake" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.authTimeout')">
      <el-input v-model="model.auth_timeout" placeholder="3s" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.maxIdleTime')">
      <el-input v-model="model.max_idle_time" placeholder="15s" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.heartbeat')">
      <el-input v-model="model.heartbeat" placeholder="10s" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.disableSni')">
      <el-switch v-model="model.disable_sni" @change="emit" />
    </el-form-item>
  </el-form>
</template>

<script setup>
// TUIC 出站：专属设置(UUID/Password/CongestionControl/UDP mode/0-RTT/Timeouts)；Base/TLS 由容器层提供
import { useI18n } from 'vue-i18n'
import { generateUuid, generatePassword } from '../utils/singboxDefaults'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>
