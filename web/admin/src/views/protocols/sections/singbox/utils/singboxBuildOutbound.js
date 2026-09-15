// sing-box outbound 通用 build/parse（表单模型 <-> sing-box 配置）
// 出站用 _ 前缀字段表达表单，build 时映射为标准 sing-box 字段。
import { inboundDefaultsForType } from './singboxDefaults'
import { supportsReality, REALITY_PROTOCOLS } from './singboxDefaults'

export function buildOutbound(ob) {
  const out = { type: ob.type, tag: ob.tag }
  if (ob.server) out.server = ob.server
  if (ob.server_port) out.server_port = ob.server_port
  if (ob.uuid) out.uuid = ob.uuid
  if (ob.password) out.password = ob.password
  if (ob.username) out.username = ob.username
  if (ob.flow) out.flow = ob.flow
  if (ob.method) out.method = ob.method
  if (ob.alterId) out.alterId = ob.alterId
  if (ob.packet_encoding) out.packet_encoding = ob.packet_encoding
  if (ob.congestion_control) out.congestion_control = ob.congestion_control
  if (ob['up_mbps']) out.up_mbps = ob['up_mbps']
  if (ob['down_mbps']) out.down_mbps = ob['down_mbps']
  if (ob['up']) out.up = ob['up']
  if (ob['down']) out.down = ob['down']
  if (ob._outboundsStr) out.outbounds = ob._outboundsStr.split(',').map(s => s.trim()).filter(Boolean)
  if (ob.default) out.default = ob.default
  if (ob.url) out.url = ob.url
  if (ob.interval) out.interval = ob.interval
  if (ob.tolerance) out.tolerance = ob.tolerance
  if (ob.interrupt_exist_connections) out.interrupt_exist_connections = ob.interrupt_exist_connections
  if (ob.bind_interface) out.bind_interface = ob.bind_interface
  if (ob.routing_mark) out.routing_mark = ob.routing_mark
  if (ob.domain_resolver) out.domain_resolver = ob.domain_resolver
  if (ob._tlsEnabled) {
    const tls = { enabled: true }
    const realityOk = supportsReality(ob.type, ob._transportType)
    if (ob._realityEnabled && realityOk) {
      // 客户端 Reality：public_key + short_id + utls
      if (ob._tlsServerName) tls.server_name = ob._tlsServerName
      if (ob._tlsAlpn?.length) tls.alpn = ob._tlsAlpn
      if (ob._utlsFingerprint) tls.utls = { enabled: true, fingerprint: ob._utlsFingerprint }
      if (ob._realityPublicKey) tls.reality = { enabled: true, public_key: ob._realityPublicKey }
      if (ob._realityShortId) tls.reality.short_id = ob._realityShortId
      if (!tls.reality) tls.reality = { enabled: true }
    } else {
      if (ob._tlsServerName) tls.server_name = ob._tlsServerName
      if (ob._tlsAlpn?.length) tls.alpn = ob._tlsAlpn
      if (ob._tlsInsecure) tls.insecure = ob._tlsInsecure
      if (ob._tlsFingerprint) tls.utls = { enabled: true, fingerprint: ob._tlsFingerprint }
      if (ob._tlsEchConfig) tls.ech_config = ob._tlsEchConfig
      if (ob._tlsClientCertPath) tls.client_certificate_path = ob._tlsClientCertPath
      if (ob._tlsClientKeyPath) tls.client_key_path = ob._tlsClientKeyPath
    }
    out.tls = tls
  }
  if (ob._transportType) {
    const tp = { type: ob._transportType }
    if (ob._transportPath) tp.path = ob._transportPath
    if (ob._transportHost) tp.host = ob._transportHost
    if (ob._transportServiceName) tp.service_name = ob._transportServiceName
    if (ob._transportMaxEarlyData) tp.max_early_data = ob._transportMaxEarlyData
    if (ob._transportEarlyDataHeader) tp.early_data_header_name = ob._transportEarlyDataHeader
    if (ob._transportIdleTimeout) tp.idle_timeout = ob._transportIdleTimeout
    if (ob._transportPingTimeout) tp.ping_timeout = ob._transportPingTimeout
    if (ob._transportPermitWithoutStream) tp.permit_without_stream = true
    if (ob._transportMethod) tp.method = ob._transportMethod
    // headers: [{key,value}] → {key:value}
    if (Array.isArray(ob._transportHeaders) && ob._transportHeaders.length) {
      const hdrs = {}
      for (const h of ob._transportHeaders) { if (h.key) hdrs[h.key] = h.value || '' }
      if (Object.keys(hdrs).length) tp.headers = hdrs
    }
    out.transport = tp
  }
  if (ob._multiplexEnabled) {
    const mp = { enabled: true }
    if (ob._multiplexProtocol) mp.protocol = ob._multiplexProtocol
    if (ob._multiplexMaxConn) mp.max_connections = ob._multiplexMaxConn
    if (ob._multiplexMinStreams) mp.min_streams = ob._multiplexMinStreams
    if (ob._multiplexMaxStreams) mp.max_streams = ob._multiplexMaxStreams
    if (ob._multiplexPadding) mp.padding = ob._multiplexPadding
    if (ob._multiplexBrutalUp > 0 || ob._multiplexBrutalDown > 0) {
      mp.brutal = { enabled: true }
      if (ob._multiplexBrutalUp > 0) mp.brutal.up_mbps = ob._multiplexBrutalUp
      if (ob._multiplexBrutalDown > 0) mp.brutal.down_mbps = ob._multiplexBrutalDown
    }
    out.multiplex = mp
  }
  return out
}

export function parseOutbound(ob) {
  const o = { ...ob }
  o.username = ob.username || ''
  o._tlsEnabled = ob.tls?.enabled || false
  o._tlsServerName = ob.tls?.server_name || ''
  o._tlsAlpn = ob.tls?.alpn || []
  o._tlsInsecure = ob.tls?.insecure || false
  o._tlsFingerprint = ob.tls?.utls?.fingerprint || ''
  o._tlsEchConfig = ob.tls?.ech_config || ''
  o._tlsClientCertPath = ob.tls?.client_certificate_path || ''
  o._tlsClientKeyPath = ob.tls?.client_key_path || ''
  o._transportType = ob.transport?.type || ''
  o._transportPath = ob.transport?.path || ''
  o._transportHost = ob.transport?.host || ''
  o._transportServiceName = ob.transport?.service_name || ''
  o._transportMaxEarlyData = ob.transport?.max_early_data || 0
  o._transportEarlyDataHeader = ob.transport?.early_data_header_name || ''
  o._transportIdleTimeout = ob.transport?.idle_timeout || ''
  o._transportPingTimeout = ob.transport?.ping_timeout || ''
  o._transportPermitWithoutStream = ob.transport?.permit_without_stream || false
  o._transportMethod = ob.transport?.method || ''
  // headers: {key:value} → [{key,value}]
  if (ob.transport?.headers && typeof ob.transport.headers === 'object') {
    o._transportHeaders = Object.entries(ob.transport.headers).map(([k, v]) => ({ key: k, value: v ? String(v) : '' }))
  } else {
    o._transportHeaders = []
  }
  o._multiplexEnabled = ob.multiplex?.enabled || false
  o._multiplexProtocol = ob.multiplex?.protocol || 'smux'
  o._multiplexMaxConn = ob.multiplex?.max_connections || 0
  o._multiplexMinStreams = ob.multiplex?.min_streams || 0
  o._multiplexMaxStreams = ob.multiplex?.max_streams || 0
  o._multiplexPadding = ob.multiplex?.padding || false
  o._multiplexBrutalUp = ob.multiplex?.brutal?.up_mbps || 0
  o._multiplexBrutalDown = ob.multiplex?.brutal?.down_mbps || 0
  o._outboundsStr = (ob.outbounds || []).join(', ')
  // Reality（客户端）
  o._realityEnabled = ob.tls?.reality?.enabled || false
  o._realityPublicKey = ob.tls?.reality?.public_key || ''
  o._realityShortId = ob.tls?.reality?.short_id || ''
  o._utlsFingerprint = ob.tls?.utls?.fingerprint || ''
  return o
}

export function outboundDefaultsForType(type) {
  const base = {
    type, tag: '', server: '', server_port: 443,
    uuid: '', password: '', username: '', flow: '', alterId: 0, packet_encoding: '',
    method: '', up: '', down: '', 'up_mbps': '', 'down_mbps': '', congestion_control: 'bbr',
    _outboundsStr: '', default: '', url: '', interval: '', tolerance: 0, interrupt_exist_connections: false,
    _tlsEnabled: false, _tlsServerName: '', _tlsAlpn: [], _tlsInsecure: false,
    _tlsFingerprint: '', _tlsEchConfig: '', _tlsClientCertPath: '', _tlsClientKeyPath: '',
    _transportType: '', _transportPath: '/', _transportHost: '', _transportServiceName: '',
    _transportMaxEarlyData: 0, _transportEarlyDataHeader: '', _transportIdleTimeout: '', _transportPingTimeout: '',
    _transportMethod: '', _transportHeaders: [],
    _transportPermitWithoutStream: false,
    _multiplexEnabled: false, _multiplexProtocol: 'smux', _multiplexMaxConn: 0,
    _multiplexMinStreams: 0, _multiplexMaxStreams: 0, _multiplexPadding: false,
    _multiplexBrutalUp: 0, _multiplexBrutalDown: 0,
    // Reality（客户端）
    _realityEnabled: false, _realityPublicKey: '', _realityShortId: '', _utlsFingerprint: 'chrome',
    bind_interface: '', routing_mark: 0, domain_resolver: '',
  }
  if (type === 'direct' || type === 'block' || type === 'dns') {
    delete base.server; delete base.server_port
  }
  return base
}