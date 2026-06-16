<template>
  <div>
    <el-form label-width="140px" label-position="left">
      <el-form-item label="Default Network">
        <el-select v-model="form.network" style="width:100%" clearable @change="emit">
          <el-option label="Raw (TCP)" value="raw" /><el-option label="WebSocket" value="ws" />
          <el-option label="gRPC" value="grpc" /><el-option label="HTTP/2" value="http" />
          <el-option label="mKCP" value="kcp" /><el-option label="QUIC" value="quic" />
          <el-option label="HTTPUpgrade" value="httpupgrade" /><el-option label="XHTTP (SplitHTTP)" value="xhttp" />
          <el-option label="Hysteria" value="hysteria" />
        </el-select>
      </el-form-item>
      <el-form-item label="Security">
        <el-select v-model="form.security" style="width:100%" @change="emit">
          <el-option label="None" value="none" /><el-option label="TLS" value="tls" /><el-option label="Reality" value="reality" />
        </el-select>
      </el-form-item>
    </el-form>

    <!-- TLS -->
    <template v-if="form.security === 'tls'">
      <el-divider content-position="left">TLS Settings</el-divider>
      <el-form label-width="160px" label-position="left">
        <el-form-item label="Server Name"><el-input v-model="form.tls.serverName" @change="emit" /></el-form-item>
        <el-form-item label="ALPN"><el-input v-model="form.tls.alpnStr" placeholder="h2, http/1.1" @change="emit" /></el-form-item>
        <el-form-item label="Allow Insecure"><el-switch v-model="form.tls.allowInsecure" @change="emit" /></el-form-item>
        <el-form-item label="Fingerprint">
          <el-select v-model="form.tls.fingerprint" style="width:100%" clearable @change="emit">
            <el-option label="chrome" value="chrome" /><el-option label="firefox" value="firefox" />
            <el-option label="safari" value="safari" /><el-option label="edge" value="edge" />
            <el-option label="random" value="random" />
          </el-select>
        </el-form-item>
      </el-form>
    </template>

    <!-- REALITY -->
    <template v-if="form.security === 'reality'">
      <el-divider content-position="left">REALITY Settings</el-divider>
      <el-form label-width="160px" label-position="left">
        <el-form-item label="Target"><el-input v-model="form.reality.target" placeholder="www.microsoft.com:443" @change="emit" /></el-form-item>
        <el-form-item label="Server Names"><el-input v-model="form.reality.serverNamesStr" @change="emit" /></el-form-item>
        <el-form-item label="Private Key"><el-input v-model="form.reality.privateKey" @change="emit" /></el-form-item>
        <el-form-item label="Short IDs"><el-input v-model="form.reality.shortIdsStr" placeholder="hex IDs, comma-sep" @change="emit" /></el-form-item>
        <el-form-item label="Max Time Diff (ms)"><el-input-number v-model="form.reality.maxTimeDiff" :min="0" @change="emit" /></el-form-item>
        <el-form-item label="Fingerprint">
          <el-select v-model="form.reality.fingerprint" style="width:100%" @change="emit">
            <el-option label="chrome" value="chrome" /><el-option label="firefox" value="firefox" />
          </el-select>
        </el-form-item>
      </el-form>
    </template>

    <!-- WebSocket -->
    <template v-if="form.network === 'ws'">
      <el-divider content-position="left">WebSocket Settings</el-divider>
      <el-form label-width="160px" label-position="left">
        <el-form-item label="Path"><el-input v-model="form.ws.path" @change="emit" /></el-form-item>
        <el-form-item label="Host"><el-input v-model="form.ws.host" @change="emit" /></el-form-item>
      </el-form>
    </template>

    <!-- gRPC -->
    <template v-if="form.network === 'grpc'">
      <el-divider content-position="left">gRPC Settings</el-divider>
      <el-form label-width="160px" label-position="left">
        <el-form-item label="Service Name"><el-input v-model="form.grpc.serviceName" @change="emit" /></el-form-item>
        <el-form-item label="Multi Mode"><el-switch v-model="form.grpc.multiMode" @change="emit" /></el-form-item>
        <el-form-item label="Idle Timeout (s)"><el-input-number v-model="form.grpc.idle_timeout" :min="0" @change="emit" /></el-form-item>
      </el-form>
    </template>

    <!-- XHTTP (SplitHTTP) -->
    <template v-if="form.network === 'xhttp'">
      <el-divider content-position="left">XHTTP Settings</el-divider>
      <el-form label-width="160px" label-position="left">
        <el-form-item label="Path"><el-input v-model="form.xhttp.path" @change="emit" /></el-form-item>
        <el-form-item label="Host"><el-input v-model="form.xhttp.host" @change="emit" /></el-form-item>
        <el-form-item label="Mode"><el-input v-model="form.xhttp.mode" placeholder="auto" @change="emit" /></el-form-item>
      </el-form>
    </template>

    <!-- Hysteria -->
    <template v-if="form.network === 'hysteria'">
      <el-divider content-position="left">Hysteria Settings</el-divider>
      <el-form label-width="160px" label-position="left">
        <el-form-item label="Version"><el-input-number v-model="form.hysteria.version" :min="0" @change="emit" /></el-form-item>
        <el-form-item label="Auth"><el-input v-model="form.hysteria.auth" @change="emit" /></el-form-item>
        <el-form-item label="Up Bandwidth"><el-input v-model="form.hysteria.up" placeholder="10 mbps" @change="emit" /></el-form-item>
        <el-form-item label="Down Bandwidth"><el-input v-model="form.hysteria.down" placeholder="100 mbps" @change="emit" /></el-form-item>
      </el-form>
    </template>

    <!-- Raw TCP -->
    <template v-if="form.network === 'raw'">
      <el-divider content-position="left">Raw TCP Settings</el-divider>
      <el-form label-width="160px" label-position="left">
        <el-form-item label="Header Type">
          <el-select v-model="form.raw.headerType" style="width:100%" clearable @change="emit">
            <el-option label="none" value="none" /><el-option label="http" value="http" />
          </el-select>
        </el-form-item>
      </el-form>
    </template>

    <!-- mKCP -->
    <template v-if="form.network === 'kcp'">
      <el-divider content-position="left">mKCP Settings</el-divider>
      <el-form label-width="160px" label-position="left">
        <el-form-item label="MTU"><el-input-number v-model="form.kcp.mtu" :min="0" @change="emit" /></el-form-item>
        <el-form-item label="TTI"><el-input-number v-model="form.kcp.tti" :min="0" @change="emit" /></el-form-item>
        <el-form-item label="Uplink Capacity"><el-input-number v-model="form.kcp.uplinkCapacity" :min="0" @change="emit" /></el-form-item>
        <el-form-item label="Downlink Capacity"><el-input-number v-model="form.kcp.downlinkCapacity" :min="0" @change="emit" /></el-form-item>
        <el-form-item label="Congestion"><el-switch v-model="form.kcp.congestion" @change="emit" /></el-form-item>
        <el-form-item label="Seed"><el-input v-model="form.kcp.seed" @change="emit" /></el-form-item>
        <el-form-item label="Header Type">
          <el-select v-model="form.kcp.headerType" style="width:100%" clearable @change="emit">
            <el-option label="none" value="none" /><el-option label="srtp" value="srtp" /><el-option label="utp" value="utp" />
            <el-option label="wechat-video" value="wechat-video" /><el-option label="dtls" value="dtls" /><el-option label="wireguard" value="wireguard" />
          </el-select>
        </el-form-item>
      </el-form>
    </template>

    <!-- Socket Options -->
    <el-divider content-position="left">Socket Options</el-divider>
    <el-form label-width="180px" label-position="left">
      <el-form-item label="Mark"><el-input-number v-model="form.sockopt.mark" :min="0" @change="emit" /></el-form-item>
      <el-form-item label="TCP Fast Open"><el-switch v-model="form.sockopt.tcpFastOpen" @change="emit" /></el-form-item>
      <el-form-item label="TProxy">
        <el-select v-model="form.sockopt.tproxy" style="width:100%" clearable @change="emit">
          <el-option label="off" value="off" /><el-option label="redirect" value="redirect" /><el-option label="tproxy" value="tproxy" />
        </el-select>
      </el-form-item>
      <el-form-item label="Domain Strategy">
        <el-select v-model="form.sockopt.domainStrategy" style="width:100%" clearable @change="emit">
          <el-option label="AsIs" value="AsIs" /><el-option label="UseIP" value="UseIP" /><el-option label="UseIPv4" value="UseIPv4" /><el-option label="UseIPv6" value="UseIPv6" />
          <el-option label="ForceIP" value="ForceIP" /><el-option label="ForceIPv4" value="ForceIPv4" /><el-option label="ForceIPv6" value="ForceIPv6" />
        </el-select>
      </el-form-item>
      <el-form-item label="Dialer Proxy"><el-input v-model="form.sockopt.dialerProxy" placeholder="Outbound tag" @change="emit" /></el-form-item>
      <el-form-item label="Interface"><el-input v-model="form.sockopt.interface" placeholder="eth0" @change="emit" /></el-form-item>
      <el-form-item label="TCP Congestion"><el-input v-model="form.sockopt.tcpcongestion" placeholder="bbr, cubic" @change="emit" /></el-form-item>
      <el-form-item label="TCP Max Seg"><el-input-number v-model="form.sockopt.tcpMaxSeg" :min="0" @change="emit" /></el-form-item>
      <el-form-item label="TCP Window Clamp"><el-input-number v-model="form.sockopt.tcpWindowClamp" :min="0" @change="emit" /></el-form-item>
      <el-form-item label="TCP Keep Alive Idle (s)"><el-input-number v-model="form.sockopt.tcpKeepAliveIdle" :min="0" @change="emit" /></el-form-item>
      <el-form-item label="TCP Keep Alive Interval (s)"><el-input-number v-model="form.sockopt.tcpKeepAliveInterval" :min="0" @change="emit" /></el-form-item>
      <el-form-item label="TCP User Timeout (ms)"><el-input-number v-model="form.sockopt.tcpUserTimeout" :min="0" @change="emit" /></el-form-item>
      <el-form-item label="TCP MPTCP"><el-switch v-model="form.sockopt.tcpMptcp" @change="emit" /></el-form-item>
      <el-form-item label="V6 Only"><el-switch v-model="form.sockopt.v6only" @change="emit" /></el-form-item>
      <el-form-item label="Accept Proxy Protocol"><el-switch v-model="form.sockopt.acceptProxyProtocol" @change="emit" /></el-form-item>
      <el-form-item label="Address Port Strategy">
        <el-select v-model="form.sockopt.addressPortStrategy" style="width:100%" clearable @change="emit">
          <el-option label="none" value="" /><el-option label="SrvPortOnly" value="SrvPortOnly" /><el-option label="SrvAddressOnly" value="SrvAddressOnly" /><el-option label="SrvPortAndAddress" value="SrvPortAndAddress" />
          <el-option label="TxtPortOnly" value="TxtPortOnly" /><el-option label="TxtAddressOnly" value="TxtAddressOnly" /><el-option label="TxtPortAndAddress" value="TxtPortAndAddress" />
        </el-select>
      </el-form-item>
    </el-form>

    <!-- Happy Eyeballs -->
    <el-divider content-position="left">Happy Eyeballs</el-divider>
    <el-form label-width="180px" label-position="left">
      <el-form-item label="Try Delay (ms)"><el-input-number v-model="form.sockopt.happyEyeballs.tryDelayMs" :min="0" @change="emit" /></el-form-item>
      <el-form-item label="Prioritize IPv6"><el-switch v-model="form.sockopt.happyEyeballs.prioritizeIPv6" @change="emit" /></el-form-item>
      <el-form-item label="Interleave"><el-input-number v-model="form.sockopt.happyEyeballs.interleave" :min="1" @change="emit" /></el-form-item>
      <el-form-item label="Max Concurrent Try"><el-input-number v-model="form.sockopt.happyEyeballs.maxConcurrentTry" :min="0" @change="emit" /></el-form-item>
    </el-form>
  </div>
</template>
<script setup>
import { reactive, onMounted } from 'vue'
const props = defineProps({ data: Object })
const emitFn = defineEmits(['update'])
const form = reactive({
  network: 'raw',
  security: 'none',
  tls: { serverName: '', alpnStr: '', allowInsecure: false, fingerprint: '' },
  reality: { target: '', serverNamesStr: '', privateKey: '', shortIdsStr: '', maxTimeDiff: 0, fingerprint: 'chrome' },
  ws: { path: '/', host: '' },
  grpc: { serviceName: '', multiMode: false, idle_timeout: 60 },
  xhttp: { path: '/', host: '', mode: '' },
  hysteria: { version: 0, auth: '', up: '', down: '' },
  raw: { headerType: 'none' },
  kcp: { mtu: 1350, tti: 50, uplinkCapacity: 5, downlinkCapacity: 20, congestion: false, seed: '', headerType: 'none' },
  sockopt: {
    mark: 0, tcpFastOpen: false, tproxy: 'off', domainStrategy: '', dialerProxy: '', interface: '',
    tcpcongestion: '', tcpMaxSeg: 0, tcpWindowClamp: 0, tcpKeepAliveIdle: 0, tcpKeepAliveInterval: 0,
    tcpUserTimeout: 0, tcpMptcp: false, v6only: false, acceptProxyProtocol: false, addressPortStrategy: '',
    happyEyeballs: { tryDelayMs: 0, prioritizeIPv6: false, interleave: 1, maxConcurrentTry: 4 },
  },
})

function sync() {
  if (!props.data) return
  form.network = props.data.network || 'raw'
  form.security = props.data.security || 'none'
  if (props.data.tlsSettings) {
    var t = props.data.tlsSettings
    form.tls = { serverName: t.serverName || '', alpnStr: (t.alpn || []).join(', '), allowInsecure: t.allowInsecure || false, fingerprint: t.fingerprint || '' }
  }
  if (props.data.realitySettings) {
    var r = props.data.realitySettings
    form.reality = { target: r.target || '', serverNamesStr: (r.serverNames || []).join(', '), privateKey: r.privateKey || '', shortIdsStr: (r.shortIds || []).join(', '), maxTimeDiff: r.maxTimeDiff || 0, fingerprint: r.fingerprint || 'chrome' }
  }
  if (props.data.wsSettings) { var w = props.data.wsSettings; form.ws = { path: w.path || '/', host: w.host || '' } }
  if (props.data.grpcSettings) { var g = props.data.grpcSettings; form.grpc = { serviceName: g.serviceName || '', multiMode: g.multiMode || false, idle_timeout: g.idle_timeout || 60 } }
  if (props.data.xhttpSettings) { var x = props.data.xhttpSettings; form.xhttp = { path: x.path || '/', host: x.host || '', mode: x.mode || '' } }
  if (props.data.hysteriaSettings) { var h = props.data.hysteriaSettings; form.hysteria = { version: h.version || 0, auth: h.auth || '', up: h.up || '', down: h.down || '' } }
  if (props.data.rawSettings || props.data.tcpSettings) { var rt = props.data.rawSettings || props.data.tcpSettings; form.raw = { headerType: (rt.header && rt.header.type) || 'none' } }
  if (props.data.kcpSettings) { var k = props.data.kcpSettings; form.kcp = { mtu: k.mtu || 1350, tti: k.tti || 50, uplinkCapacity: k.uplinkCapacity || 5, downlinkCapacity: k.downlinkCapacity || 20, congestion: k.congestion || false, seed: k.seed || '', headerType: (k.header && k.header.type) || 'none' } }
  if (props.data.sockopt) {
    var s = props.data.sockopt
    form.sockopt = {
      mark: s.mark || 0, tcpFastOpen: s.tcpFastOpen || false, tproxy: s.tproxy || 'off', domainStrategy: s.domainStrategy || '',
      dialerProxy: s.dialerProxy || '', interface: s.interface || '', tcpcongestion: s.tcpcongestion || s.tcpCongestion || '',
      tcpMaxSeg: s.tcpMaxSeg || 0, tcpWindowClamp: s.tcpWindowClamp || 0, tcpKeepAliveIdle: s.tcpKeepAliveIdle || 0,
      tcpKeepAliveInterval: s.tcpKeepAliveInterval || 0, tcpUserTimeout: s.tcpUserTimeout || 0, tcpMptcp: s.tcpMptcp || false,
      v6only: s.v6only || s.V6Only || false, acceptProxyProtocol: s.acceptProxyProtocol || false, addressPortStrategy: s.addressPortStrategy || '',
      happyEyeballs: s.happyEyeballs || { tryDelayMs: 0, prioritizeIPv6: false, interleave: 1, maxConcurrentTry: 4 },
    }
  }
}

function build() {
  var out = { network: form.network, security: form.security }
  if (form.security === 'tls') {
    var t = form.tls
    out.tlsSettings = { serverName: t.serverName || undefined, alpn: t.alpnStr ? t.alpnStr.split(',').map(function(s) { return s.trim() }) : undefined, allowInsecure: t.allowInsecure, fingerprint: t.fingerprint || undefined }
  }
  if (form.security === 'reality') {
    var r = form.reality
    out.realitySettings = { target: r.target, serverNames: r.serverNamesStr ? r.serverNamesStr.split(',').map(function(s) { return s.trim() }) : [], privateKey: r.privateKey, shortIds: r.shortIdsStr ? r.shortIdsStr.split(',').map(function(s) { return s.trim() }) : [], maxTimeDiff: r.maxTimeDiff, fingerprint: r.fingerprint }
  }
  if (form.network === 'ws') out.wsSettings = form.ws
  if (form.network === 'grpc') out.grpcSettings = form.grpc
  if (form.network === 'xhttp') out.xhttpSettings = { path: form.xhttp.path, host: form.xhttp.host || undefined, mode: form.xhttp.mode || undefined }
  if (form.network === 'hysteria') out.hysteriaSettings = { version: form.hysteria.version, auth: form.hysteria.auth || undefined, up: form.hysteria.up || undefined, down: form.hysteria.down || undefined }
  if (form.network === 'raw') out.rawSettings = { header: { type: form.raw.headerType || 'none' } }
  if (form.network === 'kcp') out.kcpSettings = { mtu: form.kcp.mtu, tti: form.kcp.tti, uplinkCapacity: form.kcp.uplinkCapacity, downlinkCapacity: form.kcp.downlinkCapacity, congestion: form.kcp.congestion, seed: form.kcp.seed || undefined, header: { type: form.kcp.headerType || 'none' } }
  out.sockopt = {
    mark: form.sockopt.mark, tcpFastOpen: form.sockopt.tcpFastOpen, tproxy: form.sockopt.tproxy,
    domainStrategy: form.sockopt.domainStrategy || undefined, dialerProxy: form.sockopt.dialerProxy || undefined,
    interface: form.sockopt.interface || undefined, tcpcongestion: form.sockopt.tcpcongestion || undefined,
    tcpMaxSeg: form.sockopt.tcpMaxSeg || undefined, tcpWindowClamp: form.sockopt.tcpWindowClamp || undefined,
    tcpKeepAliveIdle: form.sockopt.tcpKeepAliveIdle || undefined, tcpKeepAliveInterval: form.sockopt.tcpKeepAliveInterval || undefined,
    tcpUserTimeout: form.sockopt.tcpUserTimeout || undefined, tcpMptcp: form.sockopt.tcpMptcp,
    v6only: form.sockopt.v6only || undefined, acceptProxyProtocol: form.sockopt.acceptProxyProtocol || undefined,
    addressPortStrategy: form.sockopt.addressPortStrategy || undefined,
    happyEyeballs: form.sockopt.happyEyeballs.tryDelayMs ? form.sockopt.happyEyeballs : undefined,
  }
  return out
}

function emit() { emitFn('update', build()) }
onMounted(sync)
</script>
