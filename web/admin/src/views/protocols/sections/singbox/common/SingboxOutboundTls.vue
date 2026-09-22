<template>
  <el-form label-width="140px" label-position="left">
    <!-- Security：none / Standard TLS / Reality（对齐 xray 下拉模型，互斥） -->
    <el-form-item :label="t('singbox.outboundTls.security')">
      <el-select v-model="securityMode" style="width:100%" @change="onSecurityChange">
        <el-option :label="t('singbox.outboundTls.none')" value="none" />
        <el-option :label="t('singbox.outboundTls.standardTls')" value="tls" />
        <el-option v-if="realitySupported" :label="t('singbox.outboundTls.reality')" value="reality" />
      </el-select>
      <div v-if="!realitySupported" class="hint warn-hint">{{ t('singbox.outboundTls.realityProtocolOnly') }}</div>
      <div v-else class="hint">{{ t('singbox.outboundTls.mutualExclusive') }}</div>
    </el-form-item>

    <!-- Standard TLS（客户端） -->
    <template v-if="securityMode === 'tls'">
      <el-form-item :label="t('singbox.outboundTls.serverName')">
        <el-input v-model="model._tlsServerName" :placeholder="t('singbox.outboundTls.serverNamePlaceholder')" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.outboundTls.alpn')">
        <el-select v-model="model._tlsAlpn" style="width:100%" multiple clearable @change="emit">
          <el-option label="h2" value="h2" />
          <el-option label="http/1.1" value="http/1.1" />
          <el-option label="h3" value="h3" />
        </el-select>
      </el-form-item>
      <el-form-item :label="t('singbox.outboundTls.insecure')">
        <el-switch v-model="model._tlsInsecure" @change="emit" />
      </el-form-item>
      <el-form-item label="TLS Spoof（fork 特性）">
        <el-input v-model="model._tlsSpoof" placeholder="要伪装成的域名或 IP" @change="emit" />
        <div class="hint">
          本 fork 的 TLS spoof：把连接伪装到指定目标（如 tls.spoof = "www.bing.com"）；留空不启用。
          填了 spoof 才能选下面的 method（核心源码有 `spoof_method requires spoof` 校验）。
        </div>
      </el-form-item>
      <el-form-item label="Spoof Method">
        <el-select v-model="model._tlsSpoofMethod" style="width:100%" clearable @change="emit">
          <el-option label="(默认)" value="" />
          <el-option label="wrong-sequence" value="wrong-sequence" />
          <el-option label="wrong-checksum" value="wrong-checksum" />
          <el-option label="wrong-ack" value="wrong-ack" />
          <el-option label="wrong-md5" value="wrong-md5" />
          <el-option label="wrong-timestamp" value="wrong-timestamp" />
        </el-select>
      </el-form-item>
      <el-form-item label="Record Fragment">
        <el-switch v-model="model._tlsRecordFragment" @change="emit" />
        <div class="hint">TLS 记录分片（抗特征），配合下面的回退延迟</div>
      </el-form-item>
      <el-form-item label="Fragment Fallback Delay">
        <el-input v-model="model._tlsFragmentFallbackDelay" placeholder="500ms" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.outboundTls.clientCertPath')">
        <el-input v-model="model._tlsClientCertPath" placeholder="/path/to/client.crt" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.outboundTls.clientKeyPath')">
        <el-input v-model="model._tlsClientKeyPath" placeholder="/path/to/client.key" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.outboundTls.echConfig')">
        <el-input v-model="model._tlsEchConfig" type="textarea" :rows="2" placeholder="Base64 ECH config" @change="emit" />
      </el-form-item>
    </template>

    <!-- Reality（客户端：public_key + short_id(字符串) + uTLS fingerprint） -->
    <template v-else-if="securityMode === 'reality'">
      <el-form-item :label="t('singbox.outboundTls.serverName')">
        <el-input v-model="model._tlsServerName" :placeholder="t('singbox.outboundTls.serverNamePlaceholder')" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.outboundTls.publicKey')">
        <el-input v-model="model._realityPublicKey" placeholder="base64 X25519 public key" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.outboundTls.shortId')">
        <el-input v-model="model._realityShortId" :placeholder="t('singbox.outboundTls.shortIdPlaceholder')" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.outboundTls.utlsFingerprint')">
        <el-select v-model="model._utlsFingerprint" style="width:100%" clearable @change="emit">
          <el-option label="" value="" />
          <el-option v-for="fp in utlsFingerprints" :key="fp" :label="fp" :value="fp" />
        </el-select>
      </el-form-item>
    </template>
  </el-form>
</template>

<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { REALITY_PROTOCOLS } from '../utils/singboxDefaults'

const { t } = useI18n()
const props = defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }

const utlsFingerprints = ['chrome', 'firefox', 'edge', 'safari', '360', 'qq', 'ios', 'android', 'random', 'randomized']

const realitySupported = computed(() => REALITY_PROTOCOLS.includes(props.model.type))

const securityMode = computed(() => {
  if (props.model._realityEnabled && realitySupported.value) return 'reality'
  if (props.model._tlsEnabled) return 'tls'
  return 'none'
})

function onSecurityChange(val) {
  if (val === 'reality') {
    props.model._tlsEnabled = true
    props.model._realityEnabled = true
  } else if (val === 'tls') {
    props.model._tlsEnabled = true
    props.model._realityEnabled = false
    props.model._realityPublicKey = ''
    props.model._realityShortId = ''
  } else {
    props.model._tlsEnabled = false
    props.model._realityEnabled = false
    props.model._realityPublicKey = ''
    props.model._realityShortId = ''
  }
  emit()
}
</script>

<style scoped>
.hint { color: #909399; font-size: 12px; margin-top: 4px; }
.warn-hint { color: #e6a23c; }
</style>