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
      <el-form-item :label="t('singbox.outboundTls.disableSni')">
        <el-switch v-model="model._tlsDisableSni" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.outboundTls.engine')">
        <el-select v-model="model._tlsEngine" style="width:100%" clearable @change="emit">
          <el-option label="" value="" />
          <el-option label="go" value="go" />
          <el-option label="apple" value="apple" />
          <el-option label="windows" value="windows" />
        </el-select>
      </el-form-item>
      <el-form-item :label="t('singbox.tls.minVersion')">
        <el-select v-model="model._tlsMinVersion" style="width:100%" clearable @change="emit">
          <el-option label="" value="" />
          <el-option label="1.0" value="1.0" />
          <el-option label="1.1" value="1.1" />
          <el-option label="1.2" value="1.2" />
          <el-option label="1.3" value="1.3" />
        </el-select>
      </el-form-item>
      <el-form-item :label="t('singbox.tls.maxVersion')">
        <el-select v-model="model._tlsMaxVersion" style="width:100%" clearable @change="emit">
          <el-option label="" value="" />
          <el-option label="1.0" value="1.0" />
          <el-option label="1.1" value="1.1" />
          <el-option label="1.2" value="1.2" />
          <el-option label="1.3" value="1.3" />
        </el-select>
      </el-form-item>
      <el-form-item :label="t('singbox.tls.cipherSuites')">
        <el-input v-model="model._tlsCipherSuites" type="textarea" :rows="2" :placeholder="t('singbox.tls.cipherSuitesPlaceholder')" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.tls.curvePreferences')">
        <el-select v-model="model._tlsCurvePreferences" style="width:100%" multiple clearable @change="emit">
          <el-option v-for="c in curves" :key="c" :label="c" :value="c" />
        </el-select>
      </el-form-item>
      <el-form-item :label="t('singbox.outboundTls.certificatePath')">
        <el-input v-model="model._tlsCertificatePath" placeholder="/path/to/ca.pem" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.outboundTls.certificateInline')">
        <el-input v-model="model._tlsCertificate" type="textarea" :rows="3" :placeholder="t('singbox.outboundTls.certificateInlineHint')" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.outboundTls.certificateSha256')">
        <el-input v-model="model._tlsCertificateSha256" type="textarea" :rows="2" :placeholder="t('singbox.outboundTls.sha256Hint')" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.outboundTls.certificatePublicKeySha256')">
        <el-input v-model="model._tlsCertificatePublicKeySha256" type="textarea" :rows="2" :placeholder="t('singbox.outboundTls.sha256Hint')" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.tls.handshakeTimeout')">
        <el-input v-model="model._tlsHandshakeTimeout" placeholder="5s" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.tls.kernelTx')">
        <el-switch v-model="model._tlsKernelTx" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.tls.kernelRx')">
        <el-switch v-model="model._tlsKernelRx" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.outboundTls.fragment')">
        <el-switch v-model="model._tlsFragment" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('proto2.tlsSpoofFork')">
        <el-input v-model="model._tlsSpoof" :placeholder="t('protoHint.tls.spoofPh')" @change="emit" />
        <div class="hint">{{ t('protoHint.tls.spoofHint') }}</div>
      </el-form-item>
      <el-form-item :label="t('proto2.spoofMethod')">
        <el-select v-model="model._tlsSpoofMethod" style="width:100%" clearable @change="emit">
          <el-option :label="t('common.defaultOption')" value="" />
          <el-option label="wrong-sequence" value="wrong-sequence" />
          <el-option label="wrong-checksum" value="wrong-checksum" />
          <el-option label="wrong-ack" value="wrong-ack" />
          <el-option label="wrong-md5" value="wrong-md5" />
          <el-option label="wrong-timestamp" value="wrong-timestamp" />
        </el-select>
      </el-form-item>
      <el-form-item :label="t('singboxRoute.tlsRecordFragment')">
        <el-switch v-model="model._tlsRecordFragment" @change="emit" />
        <div class="hint">{{ t('protoHint.tls.recordFragment') }}</div>
      </el-form-item>
      <el-form-item :label="t('singboxRoute.fragmentFallbackDelay')">
        <el-input v-model="model._tlsFragmentFallbackDelay" placeholder="500ms" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.outboundTls.clientCertPath')">
        <el-input v-model="model._tlsClientCertPath" placeholder="/path/to/client.crt" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.outboundTls.clientKeyPath')">
        <el-input v-model="model._tlsClientKeyPath" placeholder="/path/to/client.key" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.outboundTls.clientCertInline')">
        <el-input v-model="model._tlsClientCertificate" type="textarea" :rows="2" :placeholder="t('singbox.outboundTls.certificateInlineHint')" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.outboundTls.clientKeyInline')">
        <el-input v-model="model._tlsClientKey" type="textarea" :rows="2" :placeholder="t('singbox.outboundTls.certificateInlineHint')" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.outboundTls.utlsFingerprint')">
        <el-select v-model="model._tlsFingerprint" style="width:100%" clearable @change="emit">
          <el-option label="" value="" />
          <el-option v-for="fp in utlsFingerprints" :key="fp" :label="fp" :value="fp" />
        </el-select>
      </el-form-item>
      <el-divider content-position="left">{{ t('singbox.outboundTls.ech') }}</el-divider>
      <el-form-item :label="t('singbox.outboundTls.echEnabled')">
        <el-switch v-model="model._tlsEchEnabled" @change="emit" />
      </el-form-item>
      <template v-if="model._tlsEchEnabled">
        <el-form-item :label="t('singbox.outboundTls.echConfig')">
          <el-input v-model="model._tlsEchConfig" type="textarea" :rows="2" :placeholder="t('proto2.base64EchConfig')" @change="emit" />
        </el-form-item>
        <el-form-item :label="t('singbox.outboundTls.echConfigPath')">
          <el-input v-model="model._tlsEchConfigPath" placeholder="/path/to/ech.pem" @change="emit" />
        </el-form-item>
        <el-form-item :label="t('singbox.outboundTls.echQueryServerName')">
          <el-input v-model="model._tlsEchQueryServerName" placeholder="example.com" @change="emit" />
        </el-form-item>
      </template>
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
const curves = ['P256', 'P384', 'P521', 'X25519', 'X25519MLKEM768']

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