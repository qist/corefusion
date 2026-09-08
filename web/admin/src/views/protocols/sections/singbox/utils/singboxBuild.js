// sing-box inbound 通用 build/parse（表单 模型 <-> sing-box 配置）
// 表单里用 _ 前缀字段表达配置，build 时映射为标准 sing-box 字段。

export function buildInbound(ib) {
  const out = { type: ib.type, tag: ib.tag }
  if (ib.listen) out.listen = ib.listen
  if (ib.listen_port) out.listen_port = ib.listen_port
  if (ib.domain_strategy) out.domain_strategy = ib.domain_strategy
  if (ib.network && ib.type === 'shadowsocks') out.network = ib.network

  // TUN
  if (ib.type === 'tun') {
    if (ib.interface_name) out.interface_name = ib.interface_name
    if (ib.stack) out.stack = ib.stack
    out.auto_route = !!ib.auto_route
    out.strict_route = !!ib.strict_route
  }

  // Users
  if (ib.users?.length) {
    out.users = ib.users.filter(u => u.name || u.uuid || u.username || u.password || u.userkey).map(u => {
      const user = { name: u.name }
      if (u.uuid) user.uuid = u.uuid
      if (u.username) user.username = u.username
      if (u.password) user.password = u.password
      if (u.userkey) user.userkey = u.userkey
      if (u.alterId !== undefined && u.alterId !== null) user.alterId = u.alterId
      if (u.flow) user.flow = u.flow
      return user
    })
  }

  // Shadowsocks
  if (ib.type === 'shadowsocks') {
    if (ib.method) out.method = ib.method
    if (ib.users?.[0]?.password && !out.users.some(x => x.password)) { out.password = ib.users[0].password }
  }

  // Hysteria2
  if (ib.type === 'hysteria2') {
    if (ib.up_mbps) out.up_mbps = ib.up_mbps
    if (ib.down_mbps) out.down_mbps = ib.down_mbps
    if (ib.ignore_client_bandwidth) out.ignore_client_bandwidth = ib.ignore_client_bandwidth
    if (ib.masquerade) {
      try { out.masquerade = JSON.parse(ib.masquerade) } catch { out.masquerade = ib.masquerade }
    }
  }
  // Hysteria (old)
  if (ib.type === 'hysteria') {
    if (ib.up) out.up = ib.up
    if (ib.down) out.down = ib.down
    if (ib.obfs) out.obfs = ib.obfs
  }

  // TUIC
  if (ib.type === 'tuic') {
    if (ib.congestion_control) out.congestion_control = ib.congestion_control
    if (ib.auth_timeout) out.auth_timeout = ib.auth_timeout
    if (ib.max_idle_time) out.max_idle_time = ib.max_idle_time
  }

  // ShadowTLS
  if (ib.type === 'shadowtls') {
    if (ib.version) out.version = ib.version
    if (ib.users?.[0]?.password && !out.users.some(x => x.password)) { out.password = ib.users[0].password }
  }
  // Snell
  if (ib.type === 'snell') {
    if (ib.version) out.version = ib.version
    if (ib.psk) out.psk = ib.psk
    if (ib.obfs) out.obfs = ib.obfs
    if (ib.obfs_host) out.obfs_host = ib.obfs_host
  }
  // AnyTLS
  if (ib.type === 'anytls') { /* password in users */ }

  // WireGuard
  if (ib.type === 'wireguard') {
    if (ib._wg_private_key) out.private_key = ib._wg_private_key
    if (ib._wg_address) out.address = ib._wg_address.split(',').map(s => s.trim()).filter(Boolean)
    if (ib._wg_peers?.length) out.peers = ib._wg_peers.map(p => ({ ...p }))
    if (ib.listen_port) out.listen_port = ib.listen_port
  }

  // TLS
  if (ib._tlsEnabled) {
    const tls = { enabled: true }
    if (ib._tlsServerName) tls.server_name = ib._tlsServerName
    if (ib._tlsAlpn?.length) tls.alpn = ib._tlsAlpn
    if (ib._tlsMinVersion) tls.min_version = ib._tlsMinVersion
    if (ib._tlsCertPath) tls.certificate_path = ib._tlsCertPath
    if (ib._tlsKeyPath) tls.key_path = ib._tlsKeyPath
    if (ib._utlsFingerprint) tls.utls = { enabled: true, fingerprint: ib._utlsFingerprint }
    if (ib._realityEnabled) {
      tls.reality = { enabled: true }
      if (ib._realityHandshakeServer) tls.reality.handshake = { server: ib._realityHandshakeServer, server_port: ib._realityHandshakePort }
      if (ib._realityPrivateKey) tls.reality.private_key = ib._realityPrivateKey
      if (ib._realityPublicKey) tls.reality.public_key = ib._realityPublicKey
      const sids = ib._realityShortIds ? ib._realityShortIds.split('\n').map(s => s.trim()).filter(Boolean) : []
      if (sids.length) tls.reality.short_id = sids
      if (ib._realityMaxTimeDiff) tls.reality.max_time_difference = ib._realityMaxTimeDiff
    }
    out.tls = tls
  }

  // Transport
  if (ib._transportType && TRANSPORT_TYPES.has(ib.type)) {
    const tp = { type: ib._transportType }
    if (ib._transportPath) tp.path = ib._transportPath
    if (ib._transportHost) tp.host = ib._transportHost
    if (ib._transportServiceName) tp.service_name = ib._transportServiceName
    if (ib._transportMaxEarlyData) tp.max_early_data = ib._transportMaxEarlyData
    if (ib._transportEarlyDataHeader) tp.early_data_header_name = ib._transportEarlyDataHeader
    if (ib._transportIdleTimeout) tp.idle_timeout = ib._transportIdleTimeout
    if (ib._transportPingTimeout) tp.ping_timeout = ib._transportPingTimeout
    if (ib._transportSeed) tp.seed = ib._transportSeed
    out.transport = tp
  }

  // Multiplex
  if (ib._multiplexEnabled) {
    const mp = { enabled: true }
    if (ib._multiplexProtocol) mp.protocol = ib._multiplexProtocol
    if (ib._multiplexMaxConn) mp.max_connections = ib._multiplexMaxConn
    if (ib._multiplexMinStreams) mp.min_streams = ib._multiplexMinStreams
    if (ib._multiplexMaxStreams) mp.max_streams = ib._multiplexMaxStreams
    if (ib._multiplexPadding) mp.padding = ib._multiplexPadding
    out.multiplex = mp
  }
  return out
}

export function parseInbound(ib) {
  const o = { ...ib }
  if (!o.users) o.users = []
  o._tlsEnabled = ib.tls?.enabled || false
  o._tlsServerName = ib.tls?.server_name || ''
  o._tlsAlpn = ib.tls?.alpn || []
  o._tlsMinVersion = ib.tls?.min_version || ''
  o._tlsCertPath = ib.tls?.certificate_path || ''
  o._tlsKeyPath = ib.tls?.key_path || ''
  o._utlsFingerprint = ib.tls?.utls?.fingerprint || ''
  o._realityEnabled = ib.tls?.reality?.enabled || false
  o._realityHandshakeServer = ib.tls?.reality?.handshake?.server || ''
  o._realityHandshakePort = ib.tls?.reality?.handshake?.server_port || 443
  o._realityPrivateKey = ib.tls?.reality?.private_key || ''
  o._realityPublicKey = ib.tls?.reality?.public_key || ''
  o._realityShortIds = (ib.tls?.reality?.short_id || []).join('\n')
  o._realityMaxTimeDiff = ib.tls?.reality?.max_time_difference || ''
  o._transportType = ib.transport?.type || ''
  o._transportPath = ib.transport?.path || ''
  o._transportHost = ib.transport?.host || ''
  o._transportServiceName = ib.transport?.service_name || ''
  o._transportMaxEarlyData = ib.transport?.max_early_data || 0
  o._transportEarlyDataHeader = ib.transport?.early_data_header_name || ''
  o._transportIdleTimeout = ib.transport?.idle_timeout || ''
  o._transportPingTimeout = ib.transport?.ping_timeout || ''
  o._transportSeed = ib.transport?.seed || ''
  o._multiplexEnabled = ib.multiplex?.enabled || false
  o._multiplexProtocol = ib.multiplex?.protocol || 'smux'
  o._multiplexMaxConn = ib.multiplex?.max_connections || 0
  o._multiplexMinStreams = ib.multiplex?.min_streams || 0
  o._multiplexMaxStreams = ib.multiplex?.max_streams || 0
  o._multiplexPadding = ib.multiplex?.padding || false
  // new protocol fields
  o.version = ib.version || 0
  o.psk = ib.psk || ''
  o.userkey = ib.userkey || ''
  o.obfs = ib.obfs || ''
  o.obfs_host = ib.obfs_host || ''
  o._wg_private_key = ib.private_key || ''
  o._wg_address = Array.isArray(ib.address) ? ib.address.join(',') : (ib.address || '')
  o._wg_peers = ib.peers || []
  if (typeof ib.masquerade === 'string') o.masquerade = ib.masquerade
  else if (ib.masquerade) o.masquerade = JSON.stringify(ib.masquerade)
  else o.masquerade = ''
  return o
}

const TRANSPORT_TYPES = new Set(['vless', 'vmess', 'trojan', 'shadowsocks', 'mixed', 'http', 'socks', 'naive'])