<template>
  <el-form label-width="180px" label-position="left">
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
    <el-form-item :label="t('singbox.obProtocol.quicCongestionControl')">
      <el-select v-model="model.quic_congestion_control" style="width:100%" clearable @change="emit">
        <el-option label="" value="" />
        <el-option label="bbr" value="bbr" />
        <el-option label="bbr2" value="bbr2" />
        <el-option label="cubic" value="cubic" />
        <el-option label="reno" value="reno" />
      </el-select>
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.insecureConcurrency')">
      <el-input-number v-model="model.insecure_concurrency" :min="0" style="width:100%" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.streamReceiveWindow')">
      <el-input v-model="model.stream_receive_window" placeholder="8MB" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.extraHeaders')">
      <div v-for="(h, i) in (model._naiveExtraHeaders || [])" :key="i" style="display:flex;gap:8px;margin-bottom:6px">
        <el-input v-model="h.key" placeholder="Header" style="flex:1" @change="emit" />
        <el-input v-model="h.value" placeholder="Value" style="flex:1" @change="emit" />
        <el-button size="small" type="danger" @click="model._naiveExtraHeaders.splice(i, 1); emit()">X</el-button>
      </div>
      <el-button size="small" @click="addHeader">{{ t('singbox.obProtocol.addHeader') }}</el-button>
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.quic')">
      <el-switch v-model="model.quic" @change="emit" />
    </el-form-item>
    <el-form-item v-if="model.quic" :label="t('singbox.obProtocol.quicSessionRecvWindow')">
      <el-input v-model="model.quic_session_receive_window" placeholder="8MB" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.udpOverTcp')">
      <el-switch v-model="model._udpOverTCP" @change="emit" />
    </el-form-item>
    <el-form-item v-if="model._udpOverTCP" :label="t('singbox.obProtocol.udpOverTcpVersion')">
      <el-select v-model="model._udpOverTCPVersion" style="width:100%" @change="emit">
        <el-option :label="t('singbox.obProtocol.uotDefault')" :value="0" />
        <el-option label="1" :value="1" />
        <el-option label="2" :value="2" />
      </el-select>
    </el-form-item>
  </el-form>
</template>

<script setup>
// NaiveProxy 出站：option/naive.go NaiveOutboundOptions
import { useI18n } from 'vue-i18n'
import { generateUsername, generatePassword } from '../utils/singboxDefaults'
const { t } = useI18n()
const props = defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
function addHeader() {
  if (!Array.isArray(props.model._naiveExtraHeaders)) props.model._naiveExtraHeaders = []
  props.model._naiveExtraHeaders.push({ key: '', value: '' })
  emit()
}
</script>
