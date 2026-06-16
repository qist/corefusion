<template>
  <div>
    <div class="sub-header">
      <span>Inbound List</span>
      <el-button size="small" type="primary" @click="addInbound">Add Inbound</el-button>
    </div>
    <div v-for="(ib, i) in form" :key="i" class="inbound-card">
      <el-card shadow="never">
        <div class="card-header">
          <span>Inbound #{{ i + 1 }}</span>
          <el-button size="small" type="danger" text @click="removeInbound(i)">Remove</el-button>
        </div>
        <el-form label-width="120px" label-position="left" size="small">
          <el-form-item label="Tag">
            <el-input v-model="ib.tag" placeholder="inbound-tag" />
          </el-form-item>
          <el-form-item label="Listen">
            <el-input v-model="ib.listen" placeholder="0.0.0.0" />
          </el-form-item>
          <el-form-item label="Port">
            <el-input-number v-model="ib.port" :min="1" :max="65535" />
          </el-form-item>
          <el-form-item label="Protocol">
            <el-select v-model="ib.protocol" style="width:100%">
              <el-option label="VMess" value="vmess" /><el-option label="VLESS" value="vless" />
              <el-option label="Trojan" value="trojan" /><el-option label="Shadowsocks" value="shadowsocks" />
              <el-option label="HTTP" value="http" /><el-option label="SOCKS" value="socks" />
              <el-option label="Dokodemo-door" value="dokodemo-door" />
              <el-option label="WireGuard" value="wireguard" />
            </el-select>
          </el-form-item>

          <!-- Protocol-specific settings -->
          <template v-if="ib.protocol === 'vless'">
            <el-divider content-position="left">VLESS Clients</el-divider>
            <div v-for="(c, ci) in ib._clients" :key="ci" class="sub-item">
              <el-input v-model="c.id" placeholder="UUID" style="width:40%" />
              <el-input v-model="c.email" placeholder="Email" style="width:30%" />
              <el-select v-model="c.flow" placeholder="Flow" style="width:20%" clearable>
                <el-option label="none" value="" /><el-option label="xtls-rprx-vision" value="xtls-rprx-vision" />
              </el-select>
              <el-button size="small" type="danger" text @click="ib._clients.splice(ci, 1)">X</el-button>
            </div>
            <el-button size="small" @click="ib._clients.push({ id: '', email: '', flow: '' })">Add Client</el-button>
          </template>

          <template v-if="ib.protocol === 'vmess'">
            <el-divider content-position="left">VMess Clients</el-divider>
            <div v-for="(c, ci) in ib._clients" :key="ci" class="sub-item">
              <el-input v-model="c.id" placeholder="UUID" style="width:50%" />
              <el-input-number v-model="c.alterId" :min="0" placeholder="alterId" />
              <el-input v-model="c.email" placeholder="Email" style="width:25%" />
              <el-button size="small" type="danger" text @click="ib._clients.splice(ci, 1)">X</el-button>
            </div>
            <el-button size="small" @click="ib._clients.push({ id: '', alterId: 0, email: '' })">Add Client</el-button>
          </template>

          <template v-if="ib.protocol === 'trojan'">
            <el-divider content-position="left">Trojan Clients</el-divider>
            <div v-for="(c, ci) in ib._clients" :key="ci" class="sub-item">
              <el-input v-model="c.password" placeholder="Password" style="width:50%" />
              <el-input v-model="c.email" placeholder="Email" style="width:35%" />
              <el-button size="small" type="danger" text @click="ib._clients.splice(ci, 1)">X</el-button>
            </div>
            <el-button size="small" @click="ib._clients.push({ password: '', email: '' })">Add Client</el-button>
          </template>

          <template v-if="ib.protocol === 'shadowsocks'">
            <el-form-item label="Method">
              <el-select v-model="ib._method" style="width:100%">
                <el-option label="aes-256-gcm" value="aes-256-gcm" />
                <el-option label="aes-128-gcm" value="aes-128-gcm" />
                <el-option label="chacha20-poly1305" value="chacha20-poly1305" />
                <el-option label="2022-blake3-aes-128-gcm" value="2022-blake3-aes-128-gcm" />
                <el-option label="2022-blake3-aes-256-gcm" value="2022-blake3-aes-256-gcm" />
              </el-select>
            </el-form-item>
            <el-form-item label="Password">
              <el-input v-model="ib._password" />
            </el-form-item>
          </template>

          <!-- WireGuard settings -->
          <template v-if="ib.protocol === 'wireguard'">
            <el-divider content-position="left">WireGuard Settings</el-divider>
            <el-form-item label="Secret Key">
              <el-input v-model="ib._wgSecretKey" show-password />
            </el-form-item>
            <el-form-item label="Addresses">
              <el-input v-model="ib._wgAddressesStr" placeholder="10.0.0.1/32, fd00::1/128" />
            </el-form-item>
            <el-form-item label="MTU">
              <el-input-number v-model="ib._wgMtu" :min="0" />
            </el-form-item>
            <el-form-item label="Workers">
              <el-input-number v-model="ib._wgWorkers" :min="0" />
            </el-form-item>
            <el-divider content-position="left">Peers</el-divider>
            <div v-for="(p, pi) in ib._wgPeers" :key="pi" style="margin-bottom:12px;padding:8px;background:var(--el-fill-color-light);border-radius:4px">
              <el-form-item label="Public Key" size="small"><el-input v-model="p.publicKey" /></el-form-item>
              <el-form-item label="Endpoint" size="small"><el-input v-model="p.endpoint" placeholder="1.2.3.4:51820" /></el-form-item>
              <el-form-item label="Pre-Shared Key" size="small"><el-input v-model="p.preSharedKey" show-password /></el-form-item>
              <el-form-item label="Keep Alive (s)" size="small"><el-input-number v-model="p.keepAlive" :min="0" /></el-form-item>
              <el-form-item label="Allowed IPs" size="small"><el-input v-model="p.allowedIPsStr" placeholder="0.0.0.0/0, ::/0" /></el-form-item>
              <el-button size="small" type="danger" text @click="ib._wgPeers.splice(pi,1)">Remove Peer</el-button>
            </div>
            <el-button size="small" @click="ib._wgPeers.push({publicKey:'',endpoint:'',preSharedKey:'',keepAlive:0,allowedIPsStr:'0.0.0.0/0, ::/0'})">Add Peer</el-button>
          </template>

          <!-- Stream Settings -->
          <el-divider content-position="left">Stream Settings</el-divider>
          <el-form-item label="Network">
            <el-select v-model="ib._network" style="width:100%" clearable>
              <el-option label="TCP" value="tcp" /><el-option label="WebSocket" value="ws" />
              <el-option label="gRPC" value="grpc" /><el-option label="HTTP/2" value="http" />
              <el-option label="QUIC" value="quic" />
            </el-select>
          </el-form-item>
          <el-form-item label="Security">
            <el-select v-model="ib._security" style="width:100%">
              <el-option label="None" value="none" /><el-option label="TLS" value="tls" /><el-option label="Reality" value="reality" />
            </el-select>
          </el-form-item>
          <template v-if="ib._security === 'tls'">
            <el-form-item label="Server Name">
              <el-input v-model="ib._serverName" placeholder="example.com" />
            </el-form-item>
            <el-form-item label="ALPN">
              <el-input v-model="ib._alpn" placeholder="h2, http/1.1" />
            </el-form-item>
          </template>
          <template v-if="ib._security === 'reality'">
            <el-form-item label="Target">
              <el-input v-model="ib._realityTarget" placeholder="www.microsoft.com:443" />
            </el-form-item>
            <el-form-item label="Server Names">
              <el-input v-model="ib._realityServerNames" placeholder="www.microsoft.com" />
            </el-form-item>
            <el-form-item label="Private Key">
              <el-input v-model="ib._realityPrivateKey" />
            </el-form-item>
            <el-form-item label="Short ID">
              <el-input v-model="ib._realityShortId" />
            </el-form-item>
          </template>
          <template v-if="ib._network === 'ws'">
            <el-form-item label="WS Path">
              <el-input v-model="ib._wsPath" placeholder="/" />
            </el-form-item>
          </template>
          <template v-if="ib._network === 'grpc'">
            <el-form-item label="Service Name">
              <el-input v-model="ib._grpcServiceName" />
            </el-form-item>
          </template>

          <!-- Sniffing -->
          <el-divider content-position="left">Sniffing</el-divider>
          <el-form-item label="Enabled">
            <el-switch v-model="ib._sniffingEnabled" />
          </el-form-item>
        </el-form>
      </el-card>
    </div>
  </div>
</template>

<script setup>
import { reactive, watch, onMounted } from 'vue'
const props = defineProps({ data: Array, coreType: String })
const emit = defineEmits(['update'])

const form = reactive([])

function parseInbound(ib) {
  const ss = ib.streamSettings || {}
  const settings = ib.settings || {}
  const clients = settings.clients || []
  const tls = ss.tlsSettings || {}
  const reality = ss.realitySettings || {}
  const ws = ss.wsSettings || {}
  const grpc = ss.grpcSettings || {}
  const sniffing = ib.sniffing || {}
  var peers = settings.peers || []
  return {
    tag: ib.tag || '', listen: ib.listen || '0.0.0.0', port: ib.port || 443,
    protocol: ib.protocol || 'vless',
    _clients: clients.map(function(c) { return Object.assign({}, c) }),
    _method: settings.method || 'aes-256-gcm',
    _password: settings.password || '',
    _network: ss.network || 'tcp', _security: ss.security || 'none',
    _serverName: tls.serverName || '',
    _alpn: (tls.alpn || []).join(', '),
    _realityTarget: reality.target || '',
    _realityServerNames: (reality.serverNames || []).join(', '),
    _realityPrivateKey: reality.privateKey || '',
    _realityShortId: reality.shortId || '',
    _wsPath: ws.path || '/',
    _grpcServiceName: grpc.serviceName || '',
    _sniffingEnabled: sniffing.enabled || false,
    _wgSecretKey: settings.secretKey || '',
    _wgAddressesStr: (settings.address || []).join(', '),
    _wgMtu: settings.mtu || 1420,
    _wgWorkers: settings.workers || 0,
    _wgPeers: peers.map(function(p) { return { publicKey: p.publicKey || '', endpoint: p.endpoint || '', preSharedKey: p.preSharedKey || '', keepAlive: p.keepAlive || 0, allowedIPsStr: (p.allowedIPs || ['0.0.0.0/0', '::/0']).join(', ') } }),
  }
}

function buildInbound(ib) {
  const result = {
    tag: ib.tag, listen: ib.listen, port: ib.port, protocol: ib.protocol,
    settings: {}, streamSettings: {}, sniffing: { enabled: ib._sniffingEnabled },
  }
  // Protocol settings
  if (ib.protocol === 'vless') result.settings = { clients: ib._clients, decryption: 'none' }
  else if (ib.protocol === 'vmess') result.settings = { clients: ib._clients, disableInsecureEncryption: true }
  else if (ib.protocol === 'trojan') result.settings = { clients: ib._clients }
  else if (ib.protocol === 'shadowsocks') result.settings = { method: ib._method, password: ib._password }
  else if (ib.protocol === 'wireguard') {
    result.settings.secretKey = ib._wgSecretKey
    result.settings.address = ib._wgAddressesStr ? ib._wgAddressesStr.split(',').map(function(s) { return s.trim() }) : []
    result.settings.mtu = ib._wgMtu || 1420
    if (ib._wgWorkers) result.settings.workers = ib._wgWorkers
    result.settings.peers = (ib._wgPeers || []).map(function(p) {
      return { publicKey: p.publicKey, endpoint: p.endpoint, preSharedKey: p.preSharedKey || undefined, keepAlive: p.keepAlive || 0, allowedIPs: p.allowedIPsStr ? p.allowedIPsStr.split(',').map(function(s) { return s.trim() }) : ['0.0.0.0/0', '::/0'] }
    })
    delete result.streamSettings
    return result
  }
  // Stream settings
  result.streamSettings.network = ib._network
  result.streamSettings.security = ib._security
  if (ib._security === 'tls') {
    result.streamSettings.tlsSettings = {
      serverName: ib._serverName,
      alpn: ib._alpn ? ib._alpn.split(',').map(function(s) { return s.trim() }) : [],
    }
  }
  if (ib._security === 'reality') {
    result.streamSettings.realitySettings = {
      target: ib._realityTarget,
      serverNames: ib._realityServerNames ? ib._realityServerNames.split(',').map(function(s) { return s.trim() }) : [],
      privateKey: ib._realityPrivateKey, shortId: ib._realityShortId,
    }
  }
  if (ib._network === 'ws') result.streamSettings.wsSettings = { path: ib._wsPath }
  if (ib._network === 'grpc') result.streamSettings.grpcSettings = { serviceName: ib._grpcServiceName }
  return result
}

function syncFromData() {
  form.splice(0, form.length)
  if (!Array.isArray(props.data)) return
  props.data.forEach(ib => form.push(parseInbound(ib)))
}

function addInbound() {
  form.push(parseInbound({ port: 443, protocol: 'vless', settings: { clients: [] } }))
}

function removeInbound(i) { form.splice(i, 1) }

watch(() => props.data, syncFromData, { deep: true })

onMounted(syncFromData)
</script>
<style scoped>
.sub-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; font-weight: 600; }
.inbound-card { margin-bottom: 16px; }
.card-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; font-weight: 600; }
.sub-item { display: flex; gap: 8px; margin-bottom: 8px; align-items: center; }
</style>
