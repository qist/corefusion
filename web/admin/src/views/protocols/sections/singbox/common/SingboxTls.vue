<template>
  <el-form label-width="140px" label-position="left">
    <el-form-item label="TLS Enabled">
      <el-switch v-model="model._tlsEnabled" @change="emit" />
    </el-form-item>
    <template v-if="model._tlsEnabled">
      <el-form-item label="Server Name">
        <el-input v-model="model._tlsServerName" placeholder="example.com" @change="emit" />
      </el-form-item>
      <el-form-item label="ALPN">
        <el-select v-model="model._tlsAlpn" style="width:100%" multiple clearable @change="emit">
          <el-option label="h2" value="h2" />
          <el-option label="http/1.1" value="http/1.1" />
          <el-option label="h3" value="h3" />
        </el-select>
      </el-form-item>
      <el-form-item label="Min Version">
        <el-select v-model="model._tlsMinVersion" style="width:100%" clearable @change="emit">
          <el-option label="" value="" />
          <el-option label="1.0" value="1.0" />
          <el-option label="1.1" value="1.1" />
          <el-option label="1.2" value="1.2" />
          <el-option label="1.3" value="1.3" />
        </el-select>
      </el-form-item>
      <el-form-item label="Certificate Path">
        <el-input v-model="model._tlsCertPath" placeholder="/path/to/cert.pem" @change="emit" />
      </el-form-item>
      <el-form-item label="Key Path">
        <el-input v-model="model._tlsKeyPath" placeholder="/path/to/key.pem" @change="emit" />
      </el-form-item>

      <!-- uTLS (client-side fingerprint, used on outbound; exposed for inbound trojan etc.) -->
      <el-form-item label="uTLS Fingerprint">
        <el-select v-model="model._utlsFingerprint" style="width:100%" clearable @change="emit">
          <el-option label="" value="" />
          <el-option v-for="fp in utlsFingerprints" :key="fp" :label="fp" :value="fp" />
        </el-select>
      </el-form-item>

      <!-- Reality -->
      <el-form-item label="Reality">
        <el-switch v-model="model._realityEnabled" @change="emit" />
      </el-form-item>
      <template v-if="model._realityEnabled">
        <el-form-item label="Handshake Server">
          <div style="display:flex;gap:8px;flex:1">
            <el-input v-model="model._realityHandshakeServer" placeholder="example.com" @change="emit" />
            <el-button size="small" @click="pickRealTarget">Pick</el-button>
          </div>
        </el-form-item>
        <el-form-item label="Handshake Port">
          <el-input-number v-model="model._realityHandshakePort" :min="1" :max="65535" style="width:100%" @change="emit" />
        </el-form-item>
        <el-form-item label="Private Key">
          <div style="display:flex;gap:8px;flex:1">
            <el-input v-model="model._realityPrivateKey" @change="emit" />
            <el-button size="small" @click="genRealityKey">Generate</el-button>
          </div>
        </el-form-item>
        <el-form-item label="Public Key">
          <el-input v-model="model._realityPublicKey" readonly />
          <div class="hint">Auto-filled by Generate (base64 X25519 public key)</div>
        </el-form-item>
        <el-form-item label="Short IDs">
          <el-input v-model="model._realityShortIds" type="textarea" :rows="2" placeholder="One per line" @change="emit" />
          <el-button size="small" style="margin-top:4px" @click="genRandomShortId">Generate Random Short IDs</el-button>
        </el-form-item>
        <el-form-item label="Max Time Diff">
          <el-input v-model="model._realityMaxTimeDiff" placeholder="7m" @change="emit" />
        </el-form-item>
      </template>
    </template>
  </el-form>
</template>

<script setup>
import { ElMessage } from 'element-plus'
import client from '../../../../../api/client'

const props = defineProps({
  model: { type: Object, required: true },
  serverId: { type: Number, required: true },
})
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }

const utlsFingerprints = ['chrome', 'firefox', 'edge', 'safari', '360', 'qq', 'ios', 'android', 'random', 'randomized']

const REALITY_TARGETS = [
  { target: 'www.amazon.com:443', sni: 'www.amazon.com' },
  { target: 'www.apple.com:443', sni: 'www.apple.com' },
  { target: 'www.microsoft.com:443', sni: 'www.microsoft.com' },
  { target: 'addons.mozilla.org:443', sni: 'addons.mozilla.org' },
  { target: 'www.cloudflare.com:443', sni: 'www.cloudflare.com' },
  { target: 'www.yahoo.com:443', sni: 'www.yahoo.com' },
]

function pickRealTarget() {
  const t = REALITY_TARGETS[Math.floor(Math.random() * REALITY_TARGETS.length)]
  model._realityHandshakeServer = t.sni
  model._realityHandshakePort = 443
  model._tlsServerName = t.sni
  emit()
}

// 用 sing-box generate reality-keypair 生成（不依赖 xray）
async function genRealityKey() {
  if (!model._realityEnabled) model._realityEnabled = true
  try {
    const data = await client.post(`/node-servers/${props.serverId}/binaries/reality-key`)
    if (data && data.privateKey) {
      model._realityPrivateKey = data.privateKey
      model._realityPublicKey = data.publicKey || ''
      genRandomShortId()
      emit()
      ElMessage.success('Reality keypair generated')
    }
  } catch (e) {
    ElMessage.error('Generate failed: ' + (e.message || e))
  }
}

function genRandomShortId() {
  const ids = ['']
  for (let i = 0; i < 12; i++) {
    const len = Math.random() > 0.5 ? 8 : 16
    ids.push(Array.from(crypto.getRandomValues(new Uint8Array(len / 2))).map(b => b.toString(16).padStart(2, '0')).join(''))
  }
  model._realityShortIds = ids.join('\n')
  emit()
}
</script>

<style scoped>
.hint { color: #909399; font-size: 12px; margin-top: 4px; }
</style>