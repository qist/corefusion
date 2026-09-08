<template>
  <el-form label-width="140px" label-position="left">
    <!-- Security：none / Standard TLS / Reality，互斥（对齐 xray 下拉模型） -->
    <el-form-item :label="t('singbox.tls.security')">
      <el-select v-model="securityMode" style="width:100%" @change="onSecurityChange">
        <el-option :label="t('singbox.tls.none')" value="none" />
        <el-option :label="t('singbox.tls.standardTls')" value="tls" />
        <el-option v-if="realitySupported" :label="t('singbox.tls.reality')" value="reality" />
      </el-select>
      <div v-if="!realitySupported" class="hint warn-hint">{{ t('singbox.tls.realityProtocolOnly') }}</div>
      <div v-else-if="securityMode === 'reality' && !realityTransportOk" class="hint warn-hint">
        {{ t('singbox.tls.realityTransportOnly', { transport: model._transportType || 'TCP' }) }}
      </div>
      <div v-else class="hint">{{ t('singbox.tls.mutualExclusive') }}</div>
    </el-form-item>

    <!-- Standard TLS（证书模式） -->
    <template v-if="securityMode === 'tls'">
      <el-form-item :label="t('singbox.tls.serverName')">
        <el-input v-model="model._tlsServerName" :placeholder="t('singbox.tls.serverNamePlaceholder')" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.tls.alpn')">
        <el-select v-model="model._tlsAlpn" style="width:100%" multiple clearable @change="emit">
          <el-option label="h2" value="h2" />
          <el-option label="http/1.1" value="http/1.1" />
          <el-option label="h3" value="h3" />
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
      <el-form-item :label="t('singbox.tls.certPath')">
        <el-input v-model="model._tlsCertPath" :placeholder="t('singbox.tls.certPathPlaceholder')" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.tls.keyPath')">
        <el-input v-model="model._tlsKeyPath" :placeholder="t('singbox.tls.keyPathPlaceholder')" @change="emit" />
      </el-form-item>
    </template>

    <!-- Reality（服务端：仅 handshake + private_key + short_id[] + max_time_difference） -->
    <template v-else-if="securityMode === 'reality'">
      <el-form-item :label="t('singbox.tls.serverName')">
        <el-input v-model="model._tlsServerName" :placeholder="t('singbox.tls.serverNamePlaceholder')" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.tls.handshakeServer')">
        <div style="display:flex;gap:8px;flex:1">
          <el-input v-model="model._realityHandshakeServer" placeholder="example.com" @change="emit" />
          <el-button size="small" @click="pickRealTarget">{{ t('singbox.tls.pick') }}</el-button>
        </div>
      </el-form-item>
      <el-form-item :label="t('singbox.tls.handshakePort')">
        <el-input-number v-model="model._realityHandshakePort" :min="1" :max="65535" style="width:100%" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.tls.privateKey')">
        <div style="display:flex;gap:8px;flex:1">
          <el-input v-model="model._realityPrivateKey" @change="emit" />
          <el-button size="small" @click="genRealityKey">{{ t('singbox.tls.generate') }}</el-button>
        </div>
      </el-form-item>
      <el-form-item :label="t('singbox.tls.publicKeyClient')">
        <div style="display:flex;gap:8px;flex:1">
          <el-input v-model="model._realityPublicKey" readonly />
          <el-button size="small" @click="copyPublicKey">{{ t('singbox.tls.copy') }}</el-button>
        </div>
        <div class="hint">{{ t('singbox.tls.publicKeyClientHint') }}</div>
      </el-form-item>
      <el-form-item :label="t('singbox.tls.shortIds')">
        <el-input v-model="model._realityShortIds" type="textarea" :rows="2" :placeholder="t('singbox.tls.shortIdsPlaceholder')" @change="emit" />
        <el-button size="small" style="margin-top:4px" @click="genRandomShortId">{{ t('singbox.tls.generateRandomShortIds') }}</el-button>
      </el-form-item>
      <el-form-item :label="t('singbox.tls.maxTimeDiff')">
        <el-input v-model="model._realityMaxTimeDiff" :placeholder="t('singbox.tls.maxTimeDiffPlaceholder')" @change="emit" />
      </el-form-item>
    </template>
  </el-form>
</template>

<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import client from '../../../../../api/client'
import { REALITY_PROTOCOLS, REALITY_TRANSPORTS } from '../utils/singboxDefaults'

const { t } = useI18n()
const props = defineProps({
  model: { type: Object, required: true },
  serverId: { type: Number, required: true },
})
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }

// 协议是否支持 Reality（仅 vless / vmess / trojan）
const realitySupported = computed(() => REALITY_PROTOCOLS.includes(props.model.type))
// 当前传输是否被 Reality 支持
const realityTransportOk = computed(() => REALITY_TRANSPORTS.includes(props.model._transportType || ''))

// Security 下拉模型：none / tls / reality（三者互斥）
const securityMode = computed(() => {
  if (props.model._realityEnabled && realitySupported.value) return 'reality'
  if (props.model._tlsEnabled) return 'tls'
  return 'none'
})

function onSecurityChange(val) {
  if (val === 'reality') {
    props.model._tlsEnabled = true
    props.model._realityEnabled = true
    // 启用 Reality 后清空标准证书字段，避免同时存在
    props.model._tlsCertPath = ''
    props.model._tlsKeyPath = ''
  } else if (val === 'tls') {
    props.model._tlsEnabled = true
    props.model._realityEnabled = false
    resetReality()
  } else {
    props.model._tlsEnabled = false
    props.model._realityEnabled = false
    resetReality()
  }
  emit()
}

function resetReality() {
  props.model._realityHandshakeServer = ''
  props.model._realityHandshakePort = 443
  props.model._realityPrivateKey = ''
  props.model._realityPublicKey = ''
  props.model._realityShortIds = ''
  props.model._realityMaxTimeDiff = ''
}

const REALITY_TARGETS = [
  { target: 'www.amazon.com:443', sni: 'www.amazon.com' },
  { target: 'www.apple.com:443', sni: 'www.apple.com' },
  { target: 'www.microsoft.com:443', sni: 'www.microsoft.com' },
  { target: 'addons.mozilla.org:443', sni: 'addons.mozilla.org' },
  { target: 'www.cloudflare.com:443', sni: 'www.cloudflare.com' },
  { target: 'www.yahoo.com:443', sni: 'www.yahoo.com' },
]

function pickRealTarget() {
  const tgt = REALITY_TARGETS[Math.floor(Math.random() * REALITY_TARGETS.length)]
  props.model._realityHandshakeServer = tgt.sni
  props.model._realityHandshakePort = 443
  props.model._tlsServerName = tgt.sni
  emit()
}

// 用 sing-box generate reality-keypair 生成（服务端只需 private_key，public_key 给客户端/分享用）
async function genRealityKey() {
  if (securityMode.value !== 'reality') onSecurityChange('reality')
  try {
    const data = await client.post(`/node-servers/${props.serverId}/binaries/reality-key`)
    if (data && data.privateKey) {
      props.model._realityPrivateKey = data.privateKey
      props.model._realityPublicKey = data.publicKey || ''
      genRandomShortId()
      emit()
      ElMessage.success(t('singbox.tls.keypairGenerated'))
    }
  } catch (e) {
    ElMessage.error(t('singbox.tls.generateFailed', { msg: e.message || e }))
  }
}

function copyPublicKey() {
  const pk = props.model._realityPublicKey || ''
  if (!pk) { ElMessage.warning(t('singbox.tls.generateKeyFirst')); return }
  const ta = document.createElement('textarea')
  ta.value = pk
  document.body.appendChild(ta)
  ta.select()
  try { document.execCommand('copy'); ElMessage.success(t('singbox.tls.copied')) } catch { ElMessage.error(t('singbox.users.copyError')) }
  document.body.removeChild(ta)
}

function genRandomShortId() {
  const ids = ['']
  for (let i = 0; i < 12; i++) {
    const len = Math.random() > 0.5 ? 8 : 16
    ids.push(Array.from(crypto.getRandomValues(new Uint8Array(len / 2))).map(b => b.toString(16).padStart(2, '0')).join(''))
  }
  props.model._realityShortIds = ids.join('\n')
  emit()
}
</script>

<style scoped>
.hint { color: #909399; font-size: 12px; margin-top: 4px; }
.warn-hint { color: #e6a23c; }
</style>