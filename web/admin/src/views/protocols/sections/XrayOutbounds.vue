<template>
  <div>
    <div class="sub-header">
      <span>Outbound List</span>
      <el-button size="small" type="primary" @click="addOutbound">Add Outbound</el-button>
    </div>
    <div v-for="(ob, i) in form" :key="i" class="outbound-card">
      <el-card shadow="never">
        <div class="card-header">
          <span>Outbound #{{ i + 1 }}</span>
          <el-button size="small" type="danger" text @click="removeOutbound(i)">Remove</el-button>
        </div>
        <el-form label-width="120px" label-position="left" size="small">
          <el-form-item label="Tag">
            <el-input v-model="ob.tag" placeholder="outbound-tag" />
          </el-form-item>
          <el-form-item label="Protocol">
            <el-select v-model="ob.protocol" style="width:100%">
              <el-option label="Freedom (direct)" value="freedom" /><el-option label="Blackhole (block)" value="blackhole" />
              <el-option label="VMess" value="vmess" /><el-option label="VLESS" value="vless" />
              <el-option label="Trojan" value="trojan" /><el-option label="Shadowsocks" value="shadowsocks" />
              <el-option label="SOCKS" value="socks" /><el-option label="HTTP" value="http" />
              <el-option label="WireGuard" value="wireguard" />
            </el-select>
          </el-form-item>

          <template v-if="['vmess','vless','trojan','shadowsocks'].includes(ob.protocol)">
            <el-form-item label="Server">
              <el-input v-model="ob._server" placeholder="example.com" />
            </el-form-item>
            <el-form-item label="Port">
              <el-input-number v-model="ob._port" :min="1" :max="65535" />
            </el-form-item>
          </template>
          <template v-if="ob.protocol === 'vmess' || ob.protocol === 'vless'">
            <el-form-item label="UUID">
              <el-input v-model="ob._uuid" />
            </el-form-item>
            <el-form-item label="Flow" v-if="ob.protocol === 'vless'">
              <el-select v-model="ob._flow" style="width:100%" clearable>
                <el-option label="none" value="" /><el-option label="xtls-rprx-vision" value="xtls-rprx-vision" />
              </el-select>
            </el-form-item>
            <el-form-item label="Security" v-if="ob.protocol === 'vmess'">
              <el-select v-model="ob._security" style="width:100%">
                <el-option label="auto" value="auto" /><el-option label="aes-128-gcm" value="aes-128-gcm" />
                <el-option label="chacha20-poly1305" value="chacha20-poly1305" />
              </el-select>
            </el-form-item>
          </template>
          <template v-if="ob.protocol === 'trojan' || ob.protocol === 'shadowsocks'">
            <el-form-item label="Password">
              <el-input v-model="ob._password" />
            </el-form-item>
          </template>

          <!-- Stream Settings -->
          <template v-if="['vmess','vless','trojan','shadowsocks'].includes(ob.protocol)">
            <el-divider content-position="left">Stream Settings</el-divider>
            <el-form-item label="Network">
              <el-select v-model="ob._network" style="width:100%" clearable>
                <el-option label="TCP" value="tcp" /><el-option label="WebSocket" value="ws" />
                <el-option label="gRPC" value="grpc" /><el-option label="HTTP/2" value="http" />
              </el-select>
            </el-form-item>
            <el-form-item label="Security">
              <el-select v-model="ob._securityMode" style="width:100%">
                <el-option label="None" value="none" /><el-option label="TLS" value="tls" /><el-option label="Reality" value="reality" />
              </el-select>
            </el-form-item>
            <template v-if="ob._securityMode === 'tls'">
              <el-form-item label="SNI">
                <el-input v-model="ob._sni" placeholder="example.com" />
              </el-form-item>
              <el-form-item label="Fingerprint">
                <el-select v-model="ob._fingerprint" style="width:100%" clearable>
                  <el-option label="chrome" value="chrome" /><el-option label="firefox" value="firefox" />
                  <el-option label="safari" value="safari" /><el-option label="random" value="random" />
                </el-select>
              </el-form-item>
            </template>
            <template v-if="ob._securityMode === 'reality'">
              <el-form-item label="Server Name">
                <el-input v-model="ob._realitySni" placeholder="example.com" />
              </el-form-item>
              <el-form-item label="Public Key">
                <el-input v-model="ob._realityPublicKey" />
              </el-form-item>
              <el-form-item label="Short ID">
                <el-input v-model="ob._realityShortId" />
              </el-form-item>
              <el-form-item label="Fingerprint">
                <el-select v-model="ob._realityFingerprint" style="width:100%">
                  <el-option label="chrome" value="chrome" /><el-option label="firefox" value="firefox" />
                  <el-option label="safari" value="safari" /><el-option label="random" value="random" />
                </el-select>
              </el-form-item>
            </template>
          </template>

          <!-- Freedom settings -->
          <template v-if="ob.protocol === 'freedom'">
            <el-form-item label="Domain Strategy">
              <el-select v-model="ob._domainStrategy" style="width:100%">
                <el-option label="AsIs" value="AsIs" /><el-option label="UseIP" value="UseIP" />
                <el-option label="UseIPv4" value="UseIPv4" /><el-option label="UseIPv6" value="UseIPv6" />
              </el-select>
            </el-form-item>
          </template>

          <!-- WireGuard settings -->
          <template v-if="ob.protocol === 'wireguard'">
            <el-form-item label="Secret Key">
              <el-input v-model="ob._wgSecretKey" show-password />
            </el-form-item>
            <el-form-item label="Addresses">
              <el-input v-model="ob._wgAddressesStr" placeholder="10.0.0.2/32, fd00::2/128" />
            </el-form-item>
            <el-form-item label="MTU">
              <el-input-number v-model="ob._wgMtu" :min="0" />
            </el-form-item>
            <el-form-item label="Workers">
              <el-input-number v-model="ob._wgWorkers" :min="0" />
            </el-form-item>
            <el-form-item label="Reserved">
              <el-input v-model="ob._wgReservedStr" placeholder="0, 0, 0" />
            </el-form-item>
            <el-form-item label="No Kernel TUN">
              <el-switch v-model="ob._wgNoKernelTun" />
            </el-form-item>
            <el-divider content-position="left">Peers</el-divider>
            <div v-for="(p, pi) in ob._wgPeers" :key="pi" style="margin-bottom:12px;padding:8px;background:var(--el-fill-color-light);border-radius:4px">
              <el-form-item label="Public Key" size="small"><el-input v-model="p.publicKey" /></el-form-item>
              <el-form-item label="Endpoint" size="small"><el-input v-model="p.endpoint" placeholder="1.2.3.4:51820" /></el-form-item>
              <el-form-item label="Pre-Shared Key" size="small"><el-input v-model="p.preSharedKey" show-password /></el-form-item>
              <el-form-item label="Keep Alive (s)" size="small"><el-input-number v-model="p.keepAlive" :min="0" /></el-form-item>
              <el-form-item label="Allowed IPs" size="small"><el-input v-model="p.allowedIPsStr" placeholder="0.0.0.0/0, ::/0" /></el-form-item>
              <el-button size="small" type="danger" text @click="ob._wgPeers.splice(pi,1)">Remove Peer</el-button>
            </div>
            <el-button size="small" @click="ob._wgPeers.push({publicKey:'',endpoint:'',preSharedKey:'',keepAlive:0,allowedIPsStr:'0.0.0.0/0, ::/0'})">Add Peer</el-button>
          </template>
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

function parseOutbound(ob) {
  const ss = ob.streamSettings || {}
  const vnext = ob.settings && ob.settings.vnext && ob.settings.vnext[0] ? ob.settings.vnext[0] : {}
  const srv = ob.settings && ob.settings.servers && ob.settings.servers[0] ? ob.settings.servers[0] : {}
  const user0 = vnext.users && vnext.users[0] ? vnext.users[0] : {}
  const tls = ss.tlsSettings || {}
  const reality = ss.realitySettings || {}
  var peers = (ob.settings && ob.settings.peers) || []
  return {
    tag: ob.tag || '', protocol: ob.protocol || 'freedom',
    _server: vnext.address || srv.address || '',
    _port: vnext.port || srv.port || 443,
    _uuid: user0.id || '',
    _flow: user0.flow || '',
    _security: user0.security || 'auto',
    _password: srv.password || '',
    _network: ss.network || 'tcp',
    _securityMode: ss.security || 'none',
    _sni: tls.serverName || '',
    _fingerprint: tls.fingerprint || '',
    _realitySni: reality.serverName || '',
    _realityPublicKey: reality.publicKey || '',
    _realityShortId: reality.shortId || '',
    _realityFingerprint: reality.fingerprint || 'chrome',
    _domainStrategy: (ob.settings && ob.settings.domainStrategy) || 'AsIs',
    _wgSecretKey: (ob.settings && ob.settings.secretKey) || '',
    _wgAddressesStr: ((ob.settings && ob.settings.address) || []).join(', '),
    _wgMtu: (ob.settings && ob.settings.mtu) || 1420,
    _wgWorkers: (ob.settings && ob.settings.workers) || 0,
    _wgReservedStr: ((ob.settings && ob.settings.reserved) || []).join(', '),
    _wgNoKernelTun: (ob.settings && ob.settings.noKernelTun) || false,
    _wgPeers: peers.map(function(p) { return { publicKey: p.publicKey || '', endpoint: p.endpoint || '', preSharedKey: p.preSharedKey || '', keepAlive: p.keepAlive || 0, allowedIPsStr: (p.allowedIPs || ['0.0.0.0/0', '::/0']).join(', ') } }),
  }
}

function buildOutbound(ob) {
  const result = { tag: ob.tag, protocol: ob.protocol, settings: {}, streamSettings: {} }
  if (['vmess', 'vless'].includes(ob.protocol)) {
    const user = ob.protocol === 'vmess'
      ? { id: ob._uuid, alterId: 0, security: ob._security }
      : { id: ob._uuid, encryption: 'none', flow: ob._flow }
    result.settings.vnext = [{ address: ob._server, port: ob._port, users: [user] }]
    result.streamSettings = { network: ob._network, security: ob._securityMode }
    if (ob._securityMode === 'tls') result.streamSettings.tlsSettings = { serverName: ob._sni, fingerprint: ob._fingerprint }
    if (ob._securityMode === 'reality') result.streamSettings.realitySettings = { serverName: ob._realitySni, publicKey: ob._realityPublicKey, shortId: ob._realityShortId, fingerprint: ob._realityFingerprint }
  } else if (ob.protocol === 'trojan') {
    result.settings.servers = [{ address: ob._server, port: ob._port, password: ob._password }]
    result.streamSettings = { network: ob._network, security: ob._securityMode }
    if (ob._securityMode === 'tls') result.streamSettings.tlsSettings = { serverName: ob._sni }
    if (ob._securityMode === 'reality') result.streamSettings.realitySettings = { serverName: ob._realitySni, publicKey: ob._realityPublicKey, shortId: ob._realityShortId, fingerprint: ob._realityFingerprint }
  } else if (ob.protocol === 'wireguard') {
    result.settings.secretKey = ob._wgSecretKey
    result.settings.address = ob._wgAddressesStr ? ob._wgAddressesStr.split(',').map(function(s) { return s.trim() }) : []
    result.settings.mtu = ob._wgMtu || 1420
    if (ob._wgWorkers) result.settings.workers = ob._wgWorkers
    if (ob._wgReservedStr) result.settings.reserved = ob._wgReservedStr.split(',').map(function(s) { return parseInt(s.trim()) })
    result.settings.noKernelTun = ob._wgNoKernelTun || false
    result.settings.peers = (ob._wgPeers || []).map(function(p) {
      return { publicKey: p.publicKey, endpoint: p.endpoint, preSharedKey: p.preSharedKey || undefined, keepAlive: p.keepAlive || 0, allowedIPs: p.allowedIPsStr ? p.allowedIPsStr.split(',').map(function(s) { return s.trim() }) : ['0.0.0.0/0', '::/0'] }
    })
    delete result.streamSettings
  } else if (ob.protocol === 'freedom') {
    result.settings = { domainStrategy: ob._domainStrategy }
    delete result.streamSettings
  } else if (ob.protocol === 'blackhole') {
    result.settings = { response: { type: 'none' } }
    delete result.streamSettings
  }
  return result
}

function syncFromData() {
  form.splice(0, form.length)
  if (!Array.isArray(props.data)) return
  props.data.forEach(ob => form.push(parseOutbound(ob)))
}

function addOutbound() { form.push(parseOutbound({ protocol: 'freedom', tag: 'direct' })) }
function removeOutbound(i) { form.splice(i, 1) }

watch(() => props.data, syncFromData, { deep: true })

onMounted(syncFromData)
</script>
<style scoped>
.sub-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; font-weight: 600; }
.outbound-card { margin-bottom: 16px; }
.card-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; font-weight: 600; }
</style>
