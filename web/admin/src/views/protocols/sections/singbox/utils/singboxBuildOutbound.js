// sing-box outbound 通用 build/parse（表单模型 <-> sing-box 配置）
// 出站用 _ 前缀字段表达表单，build 时映射为标准 sing-box 字段。
// ⚠ 字段名以 /opt/sing-box/option/*.go 的 json tag 为准：sing-box 对未知字段会 FATAL。
import { generateKey, generatePassword } from './singboxDefaults'
import { supportsReality, OUTBOUND_NETWORK_PROTOCOLS, OUTBOUND_SERVER_PROTOCOLS } from './singboxDefaults'
// 出站「标签」生成器与 xray 共用（见 sections/utils/nodeNaming.js），不另起一套。
import { genOutboundTag } from '../../utils/nodeNaming'

// 逗号/换行分隔字符串 → 数组
function splitList(v) {
  if (Array.isArray(v)) return v
  return String(v || '').split(/[\n,]/).map(s => s.trim()).filter(Boolean)
}

// URI 头/请求头：[[{key,value}]] → {k:v}
function headersFromList(list) {
  const hdrs = {}
  if (Array.isArray(list)) {
    for (const h of list) { if (h.key) hdrs[h.key] = h.value || '' }
  }
  return hdrs
}

// 端口段：sing-box LegacyListable 要求 "a:b" 形式；表单允许用户输入 "a-b"
function splitPorts(v) {
  return splitList(v).map(s => s.replace('-', ':'))
}

// udp_over_tcp: 布尔或 {enabled, version}
function buildUdpOverTCP(ob) {
  if (!ob._udpOverTCP) return undefined
  const v = parseInt(ob._udpOverTCPVersion) || 0
  return v ? { enabled: true, version: v } : true
}

export function buildOutbound(ob) {
  // sing-box 没有 shadowsocks2022 出站类型，只是「2022 系加密方式」的界面别名
  const type = ob.type === 'shadowsocks2022' ? 'shadowsocks' : ob.type
  const out = { type, tag: ob.tag }
  if (OUTBOUND_SERVER_PROTOCOLS.includes(type)) {
    // Hysteria2 Realm 模式由 realm 分配端口，不能再写 server/server_port/server_ports
    const realmMode = ob.type === 'hysteria2' && ob._hy2RealmEnabled
    if (ob.server && !realmMode) out.server = ob.server
    if (ob.server_port && !realmMode) out.server_port = ob.server_port
  }
  // 凭据字段按协议白名单写出（unknown field 会被 sing-box strict decode 拒绝）
  if (['vmess', 'vless', 'tuic'].includes(type) && ob.uuid) out.uuid = ob.uuid
  if (['trojan', 'shadowsocks', 'shadowsocks2022', 'hysteria2', 'tuic', 'anytls', 'shadowtls', 'ssh', 'socks', 'http', 'naive'].includes(type) && ob.password) {
    out.password = ob.password
  }
  if (['http', 'socks', 'naive'].includes(type) && ob.username) out.username = ob.username
  if (type === 'vless' && ob.flow) out.flow = ob.flow

  // Dial Fields（AbstractDialerOptions）：block / bridge / selector / urltest 没有拨号字段
  if (!['block', 'bridge', 'selector', 'urltest'].includes(type)) {
    if (ob.detour && type !== 'direct') out.detour = ob.detour
    if (ob.bind_interface) out.bind_interface = ob.bind_interface
    if (ob.inet4_bind_address) out.inet4_bind_address = ob.inet4_bind_address
    if (ob.inet6_bind_address) out.inet6_bind_address = ob.inet6_bind_address
    if (ob.bind_address_no_port) out.bind_address_no_port = true
    if (ob.protect_path) out.protect_path = ob.protect_path
    if (ob.routing_mark) out.routing_mark = ob.routing_mark
    if (ob.reuse_addr) out.reuse_addr = true
    if (ob.netns) out.netns = ob.netns
    if (ob.connect_timeout) out.connect_timeout = ob.connect_timeout
    if (ob.tcp_fast_open) out.tcp_fast_open = true
    if (ob.tcp_multi_path) out.tcp_multi_path = true
    if (ob.disable_tcp_keep_alive) out.disable_tcp_keep_alive = true
    if (ob.tcp_keep_alive) out.tcp_keep_alive = ob.tcp_keep_alive
    if (ob.tcp_keep_alive_interval) out.tcp_keep_alive_interval = ob.tcp_keep_alive_interval
    if (ob.udp_fragment) out.udp_fragment = true
    if (ob.domain_resolver) out.domain_resolver = ob.domain_resolver
    if (ob.network_strategy) out.network_strategy = ob.network_strategy
    const netTypes = splitList(ob._networkType)
    if (netTypes.length) out.network_type = netTypes
    const fallbackNetTypes = splitList(ob._fallbackNetworkType)
    if (fallbackNetTypes.length) out.fallback_network_type = fallbackNetTypes
    if (ob.fallback_delay) out.fallback_delay = ob.fallback_delay
  }
  // NetworkList：仅写入源码确实有 network 字段的协议，且必须是数组（"tcp,udp" 单字符串会被判非法）
  if (OUTBOUND_NETWORK_PROTOCOLS.includes(ob.type) && ob.network) {
    const nets = splitList(ob.network)
    if (nets.length) out.network = nets
  }

  // VMess
  if (ob.type === 'vmess') {
    if (ob.security && ob.security !== 'auto') out.security = ob.security
    if (ob.alterId) out.alter_id = ob.alterId
    if (ob.global_padding) out.global_padding = true
    if (ob.authenticated_length) out.authenticated_length = true
    if (ob.packet_encoding) out.packet_encoding = ob.packet_encoding
  }
  // VLESS（注意：没有 encryption 字段）
  if (ob.type === 'vless' && ob.packet_encoding) out.packet_encoding = ob.packet_encoding

  // Shadowsocks
  if (ob.type === 'shadowsocks' || ob.type === 'shadowsocks2022') {
    if (ob.method) out.method = ob.method
    if (ob.plugin) out.plugin = ob.plugin
    if (ob.plugin_opts) out.plugin_opts = ob.plugin_opts
    const uot = buildUdpOverTCP(ob)
    if (uot !== undefined) out.udp_over_tcp = uot
  }

  // AnyTLS 出站（option/anytls.go: password/idle_session_*/min_idle_session/client_metadata）
  if (ob.type === 'anytls') {
    if (ob.password) out.password = ob.password
    if (ob.idle_session_check_interval) out.idle_session_check_interval = ob.idle_session_check_interval
    if (ob.idle_session_timeout) out.idle_session_timeout = ob.idle_session_timeout
    if (ob.min_idle_session) out.min_idle_session = ob.min_idle_session
    if (ob.client_metadata) out.client_metadata = ob.client_metadata
  }

  // Snell（version 只接受 4 / 6）
  if (ob.type === 'snell') {
    if (ob.version) out.version = ob.version
    if (ob.psk) out.psk = ob.psk
    if (ob.userkey) out.userkey = ob.userkey
    if (ob.reuse) out.reuse = true
    if (ob.version === 4) {
      if (ob.obfs) out.obfs_mode = ob.obfs
      if (ob.obfs_host) out.obfs_host = ob.obfs_host
    } else if (ob.version === 6 && ob._snellMode) {
      out.mode = ob._snellMode
    }
  }

  // TUIC 出站（没有 auth_timeout / max_idle_time / disable_sni）
  if (ob.type === 'tuic') {
    if (ob.congestion_control) out.congestion_control = ob.congestion_control
    // udp_over_stream 与 udp_relay_mode 互斥（源码 protocol/tuic/outbound.go 会直接报错）
    if (ob.udp_over_stream) out.udp_over_stream = true
    else if (ob.udp_relay_mode) out.udp_relay_mode = ob.udp_relay_mode
    if (ob.zero_rtt_handshake) out.zero_rtt_handshake = true
    if (ob.heartbeat) out.heartbeat = ob.heartbeat
    applyQuic(out, ob, 'tuic')
  }

  // Hysteria2 出站
  if (ob.type === 'hysteria2') {
    if (ob.up_mbps) out.up_mbps = parseInt(ob.up_mbps) || 0
    if (ob.down_mbps) out.down_mbps = parseInt(ob.down_mbps) || 0
    const ports = splitPorts(ob.server_ports)
    if (ports.length && !ob._hy2RealmEnabled) out.server_ports = ports
    if (ob.hop_interval) out.hop_interval = ob.hop_interval
    if (ob.hop_interval_max) out.hop_interval_max = ob.hop_interval_max
    // obfs: {type, password}，gecko 支持 min/max_packet_size
    if (ob._hy2ObfsType) {
      out.obfs = { type: ob._hy2ObfsType }
      if (ob._hy2ObfsPassword) out.obfs.password = ob._hy2ObfsPassword
      if (ob._hy2ObfsType === 'gecko') {
        if (ob._hy2ObfsMinPacketSize) out.obfs.min_packet_size = ob._hy2ObfsMinPacketSize
        if (ob._hy2ObfsMaxPacketSize) out.obfs.max_packet_size = ob._hy2ObfsMaxPacketSize
      }
    }
    if (ob._hy2BbrProfile) out.bbr_profile = ob._hy2BbrProfile
    if (ob._hy2BrutalDebug) out.brutal_debug = true
    if (ob._hy2DisableChromeParrot) out.disable_chrome_parrot = true
    if (ob._hy2RealmEnabled) {
      const realm = {
        server_url: ob._hy2RealmServerUrl || '',
        realm_id: ob._hy2RealmId || '',
        stun_servers: splitList(ob._hy2RealmStunServers),
      }
      if (ob._hy2RealmToken) realm.token = ob._hy2RealmToken
      if (ob._hy2RealmIpVersion) realm.ip_version = ob._hy2RealmIpVersion
      if (ob._hy2RealmPortMappingEnabled) {
        realm.port_mapping = { enabled: true }
        if (ob._hy2RealmPortMappingTimeout) realm.port_mapping.timeout = ob._hy2RealmPortMappingTimeout
        if (ob._hy2RealmPortMappingLifetime) realm.port_mapping.lifetime = ob._hy2RealmPortMappingLifetime
      }
      if (ob._hy2RealmHttpClientTag) realm.http_client = ob._hy2RealmHttpClientTag
      out.realm = realm
    }
    applyQuic(out, ob, 'hy2')
  }

  // Hysteria v1 出站
  if (ob.type === 'hysteria') {
    if (ob.up) out.up = ob.up
    if (ob.down) out.down = ob.down
    if (ob.up_mbps) out.up_mbps = parseInt(ob.up_mbps) || 0
    if (ob.down_mbps) out.down_mbps = parseInt(ob.down_mbps) || 0
    if (ob.obfs) out.obfs = ob.obfs
    if (ob.auth_str) out.auth_str = ob.auth_str
    const ports = splitPorts(ob.server_ports)
    if (ports.length) out.server_ports = ports
    if (ob.hop_interval) out.hop_interval = ob.hop_interval
    applyQuic(out, ob, 'hy2')
  }

  // Naive（字段名是 quic_congestion_control）
  if (ob.type === 'naive') {
    if (ob.quic_congestion_control) out.quic_congestion_control = ob.quic_congestion_control
    if (ob.insecure_concurrency) out.insecure_concurrency = ob.insecure_concurrency
    const extra = headersFromList(ob._naiveExtraHeaders)
    if (Object.keys(extra).length) out.extra_headers = extra
    if (ob.stream_receive_window) out.stream_receive_window = ob.stream_receive_window
    const uot = buildUdpOverTCP(ob)
    if (uot !== undefined) out.udp_over_tcp = uot
    if (ob.quic) out.quic = true
    if (ob.quic_session_receive_window) out.quic_session_receive_window = ob.quic_session_receive_window
  }

  // HTTP 出站
  if (ob.type === 'http') {
    const httpV = parseInt(ob.version) || 0
    // path / headers 仅 HTTP/1 支持（源码会报 "path and Host header are only supported by HTTP/1"）
    if (httpV <= 1) {
      if (ob.path) out.path = ob.path
      const hdrs = headersFromList(ob._httpHeaders)
      if (Object.keys(hdrs).length) out.headers = hdrs
    }
    if (httpV) out.version = httpV
    if (ob.disable_version_fallback) out.disable_version_fallback = true
    // version 2/3 时 HTTP2/QUIC 字段才有效，但多写无害；仅在有值时写入
    if (ob._httpIdleTimeout) out.idle_timeout = ob._httpIdleTimeout
    if (ob._httpKeepAlivePeriod) out.keep_alive_period = ob._httpKeepAlivePeriod
    if (ob._httpStreamRecvWindow) out.stream_receive_window = ob._httpStreamRecvWindow
    if (ob._httpConnRecvWindow) out.connection_receive_window = ob._httpConnRecvWindow
    if (ob._httpMaxConcurrentStreams) out.max_concurrent_streams = ob._httpMaxConcurrentStreams
    if (parseInt(ob.version) === 3) {
      if (ob._httpInitialPacketSize) out.initial_packet_size = ob._httpInitialPacketSize
      if (ob._httpDisablePathMtu) out.disable_path_mtu_discovery = true
    }
  }

  // SOCKS 出站
  if (ob.type === 'socks') {
    if (ob.version) out.version = String(ob.version)
    const uot = buildUdpOverTCP(ob)
    if (uot !== undefined) out.udp_over_tcp = uot
  }

  // SSH 出站
    if (ob.type === 'ssh') {
    if (ob.user) out.user = ob.user
    if (ob.password) out.password = ob.password
    // private_key 是 PEM 块（多行），必须整体作为单条，不能按行拆
    if (String(ob.private_key || '').trim()) out.private_key = [ob.private_key.trim()]
    if (ob.private_key_path) out.private_key_path = ob.private_key_path
    if (ob.private_key_passphrase) out.private_key_passphrase = ob.private_key_passphrase
    const hostKeys = splitList(ob.host_key)
    if (hostKeys.length) out.host_key = hostKeys
    const hka = splitList(ob.host_key_algorithms)
    if (hka.length) out.host_key_algorithms = hka
    if (ob.client_version) out.client_version = ob.client_version
    const ciphers = splitList(ob.cipher)
    if (ciphers.length) out.cipher = ciphers
    const macs = splitList(ob.mac)
    if (macs.length) out.mac = macs
    const kex = splitList(ob.kex_algorithm)
    if (kex.length) out.kex_algorithm = kex
  }

  // Tor 出站（torrc 是 map[string]string）
  if (ob.type === 'tor') {
    if (ob.executable_path) out.executable_path = ob.executable_path
    const args = splitList(ob._torExtraArgs)
    if (args.length) out.extra_args = args
    if (ob.data_directory) out.data_directory = ob.data_directory
    const torrc = {}
    for (const line of String(ob.torrc || '').split('\n')) {
      const idx = line.indexOf('=')
      if (idx > 0) torrc[line.slice(0, idx).trim()] = line.slice(idx + 1).trim()
    }
    if (Object.keys(torrc).length) out.torrc = torrc
  }

  // Bridge 出站
  if (ob.type === 'bridge') {
    if (ob.interface) out.interface = ob.interface
    if (ob.bridge_name) out.bridge_name = ob.bridge_name
    if (ob.iproute2_table_index) out.iproute2_table_index = ob.iproute2_table_index
    if (ob.iproute2_rule_index) out.iproute2_rule_index = ob.iproute2_rule_index
  }

  // selector / urltest
  if (ob.type === 'selector' || ob.type === 'urltest') {
    const outs = splitList(ob._outboundsStr)
    if (outs.length) out.outbounds = outs
    if (ob.type === 'selector' && ob.default) out.default = ob.default
    if (ob.url) out.url = ob.url
    if (ob.interval) out.interval = ob.interval
    if (ob.tolerance) out.tolerance = ob.tolerance
    if (ob.idle_timeout) out.idle_timeout = ob.idle_timeout
    if (ob.interrupt_exist_connections) out.interrupt_exist_connections = true
  }

  // TLS
  if (ob._tlsEnabled) {
    const tls = { enabled: true }
    const realityOk = supportsReality(ob.type, ob._transportType)
    if (ob._tlsServerName) tls.server_name = ob._tlsServerName
    if (ob._tlsAlpn?.length) tls.alpn = ob._tlsAlpn
    if (ob._tlsDisableSni) tls.disable_sni = true
    if (ob._tlsEngine) tls.engine = ob._tlsEngine
    if (ob._tlsMinVersion) tls.min_version = ob._tlsMinVersion
    if (ob._tlsMaxVersion) tls.max_version = ob._tlsMaxVersion
    if (ob._tlsCipherSuites) tls.cipher_suites = splitList(ob._tlsCipherSuites)
    if (Array.isArray(ob._tlsCurvePreferences) && ob._tlsCurvePreferences.length) {
      tls.curve_preferences = ob._tlsCurvePreferences
    }
    if (ob._tlsHandshakeTimeout) tls.handshake_timeout = ob._tlsHandshakeTimeout
    if (ob._tlsKernelTx) tls.kernel_tx = true
    if (ob._tlsKernelRx) tls.kernel_rx = true
    if (ob._realityEnabled && realityOk) {
      // 客户端 Reality：public_key + short_id + utls
      if (ob._utlsFingerprint) tls.utls = { enabled: true, fingerprint: ob._utlsFingerprint }
      tls.reality = { enabled: true }
      if (ob._realityPublicKey) tls.reality.public_key = ob._realityPublicKey
      if (ob._realityShortId) tls.reality.short_id = ob._realityShortId
    } else {
      if (ob._tlsInsecure) tls.insecure = true
      if (ob._tlsFingerprint) tls.utls = { enabled: true, fingerprint: ob._tlsFingerprint }
      if (ob._tlsCertificate) tls.certificate = [ob._tlsCertificate.trim()]
      if (ob._tlsCertificatePath) tls.certificate_path = ob._tlsCertificatePath
      // 证书指纹（base64 SHA-256）：OutboundTLSOptions
      if (ob._tlsCertificateSha256) tls.certificate_sha256 = splitList(ob._tlsCertificateSha256)
      if (ob._tlsCertificatePublicKeySha256) tls.certificate_public_key_sha256 = splitList(ob._tlsCertificatePublicKeySha256)
      if (ob._tlsClientCertPath) tls.client_certificate_path = ob._tlsClientCertPath
      if (ob._tlsClientKeyPath) tls.client_key_path = ob._tlsClientKeyPath
      if (ob._tlsClientCertificate) tls.client_certificate = [ob._tlsClientCertificate.trim()]
      if (ob._tlsClientKey) tls.client_key = [ob._tlsClientKey.trim()]
      if (ob._tlsFragment) tls.fragment = true
      // ECH 是对象（tls.ech），不是 ech_config
      if (ob._tlsEchEnabled) {
        tls.ech = { enabled: true }
        if (ob._tlsEchConfig) tls.ech.config = splitList(ob._tlsEchConfig)
        if (ob._tlsEchConfigPath) tls.ech.config_path = ob._tlsEchConfigPath
        if (ob._tlsEchQueryServerName) tls.ech.query_server_name = ob._tlsEchQueryServerName
      }
    }
    // 本 fork 的 TLS spoof / 记录分片（客户端侧特性）
    if (ob._tlsSpoof) {
      tls.spoof = ob._tlsSpoof
      // 源码约束：spoof_method requires spoof，所以只在有 spoof 时才写
      if (ob._tlsSpoofMethod) tls.spoof_method = ob._tlsSpoofMethod
    }
    if (ob._tlsRecordFragment) tls.record_fragment = true
    if (ob._tlsFragmentFallbackDelay) tls.fragment_fallback_delay = ob._tlsFragmentFallbackDelay
    out.tls = tls
  }

  // V2Ray Transport：按类型只输出该类型支持的字段（交叉字段会 unknown field）
  if (ob._transportType) {
    const tp = { type: ob._transportType }
    const hdrs = headersFromList(ob._transportHeaders)
    switch (ob._transportType) {
      case 'http':
        if (ob._transportHost) tp.host = splitList(ob._transportHost)
        if (ob._transportPath) tp.path = ob._transportPath
        if (ob._transportMethod) tp.method = ob._transportMethod
        if (Object.keys(hdrs).length) tp.headers = hdrs
        if (ob._transportIdleTimeout) tp.idle_timeout = ob._transportIdleTimeout
        if (ob._transportPingTimeout) tp.ping_timeout = ob._transportPingTimeout
        break
      case 'ws':
        if (ob._transportPath) tp.path = ob._transportPath
        if (Object.keys(hdrs).length) tp.headers = hdrs
        if (ob._transportMaxEarlyData) tp.max_early_data = ob._transportMaxEarlyData
        if (ob._transportEarlyDataHeader) tp.early_data_header_name = ob._transportEarlyDataHeader
        break
      case 'grpc':
        if (ob._transportServiceName) tp.service_name = ob._transportServiceName
        if (ob._transportIdleTimeout) tp.idle_timeout = ob._transportIdleTimeout
        if (ob._transportPingTimeout) tp.ping_timeout = ob._transportPingTimeout
        if (ob._transportPermitWithoutStream) tp.permit_without_stream = true
        break
      case 'httpupgrade':
        if (ob._transportHost) tp.host = ob._transportHost
        if (ob._transportPath) tp.path = ob._transportPath
        if (Object.keys(hdrs).length) tp.headers = hdrs
        break
      case 'quic':
        // 无额外字段
        break
    }
    out.transport = tp
  }

  // Multiplex
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

// QUICOptions = HTTP2Options + initial_packet_size/disable_path_mtu_discovery
// prefix: 'hy2'（Hysteria/Hysteria2 共用 _hy2* 表单字段）| 'tuic'
function applyQuic(out, ob, prefix) {
  const k = (name) => prefix === 'tuic' ? ob['_tuic' + name] : ob['_hy2' + name]
  if (k('InitialPacketSize')) out.initial_packet_size = k('InitialPacketSize')
  if (k('DisablePathMtu')) out.disable_path_mtu_discovery = true
  if (k('IdleTimeout')) out.idle_timeout = k('IdleTimeout')
  if (k('KeepAlivePeriod')) out.keep_alive_period = k('KeepAlivePeriod')
  if (k('StreamRecvWindow')) out.stream_receive_window = k('StreamRecvWindow')
  if (k('ConnRecvWindow')) out.connection_receive_window = k('ConnRecvWindow')
  if (k('MaxConcurrentStreams')) out.max_concurrent_streams = k('MaxConcurrentStreams')
}

export function parseOutbound(ob) {
  const o = { ...ob }
  o.username = ob.username || ''
  // Dial Fields
  o.detour = ob.detour || ''
  o.bind_interface = ob.bind_interface || ''
  o.inet4_bind_address = ob.inet4_bind_address || ''
  o.inet6_bind_address = ob.inet6_bind_address || ''
  o.bind_address_no_port = ob.bind_address_no_port || false
  o.protect_path = ob.protect_path || ''
  o.routing_mark = ob.routing_mark || 0
  o.reuse_addr = ob.reuse_addr || false
  o.netns = ob.netns || ''
  o.connect_timeout = ob.connect_timeout || ''
  o.tcp_fast_open = ob.tcp_fast_open || false
  o.tcp_multi_path = ob.tcp_multi_path || false
  o.disable_tcp_keep_alive = ob.disable_tcp_keep_alive || false
  o.tcp_keep_alive = ob.tcp_keep_alive || ''
  o.tcp_keep_alive_interval = ob.tcp_keep_alive_interval || ''
  o.udp_fragment = ob.udp_fragment || false
  o.domain_resolver = typeof ob.domain_resolver === 'string' ? ob.domain_resolver : (ob.domain_resolver?.server || '')
  o.network_strategy = ob.network_strategy || ''
  o._networkType = Array.isArray(ob.network_type) ? ob.network_type.join(',') : (ob.network_type || '')
  o._fallbackNetworkType = Array.isArray(ob.fallback_network_type) ? ob.fallback_network_type.join(',') : (ob.fallback_network_type || '')
  o.fallback_delay = ob.fallback_delay || ''
  // NetworkList：数组 → 逗号字符串（仅 UI 展示）
  o.network = Array.isArray(ob.network) ? ob.network.join(',') : (ob.network || '')

  // Snell
  if (ob.type === 'snell') {
    o.obfs = ob.obfs_mode || ''
    o._snellMode = ob.mode || ''
    o.psk = ob.psk || ''
    o.version = ob.version || 4
    o.obfs_host = ob.obfs_host || ''
    o.userkey = ob.userkey || ''
    o.reuse = ob.reuse || false
  }
  // SS 2022 方法回显为界面别名
  if (ob.type === 'shadowsocks' && String(ob.method || '').startsWith('2022-')) {
    o.type = 'shadowsocks2022'
  }
  if (ob.type === 'shadowsocks' || o.type === 'shadowsocks2022') {
    o.plugin = ob.plugin || ''
    o.plugin_opts = ob.plugin_opts || ''
    o._udpOverTCP = !!ob.udp_over_tcp
    o._udpOverTCPVersion = typeof ob.udp_over_tcp === 'object' ? (ob.udp_over_tcp.version || 0) : 0
  }
  // AnyTLS
  if (ob.type === 'anytls') {
    o.idle_session_check_interval = ob.idle_session_check_interval || ''
    o.idle_session_timeout = ob.idle_session_timeout || ''
    o.min_idle_session = ob.min_idle_session || 0
    o.client_metadata = ob.client_metadata || ''
  }
  // TUIC
  o.udp_over_stream = ob.udp_over_stream || false
  // VMess
  o.security = ob.security || 'auto'
  o.alterId = ob.alter_id || 0
  o.global_padding = ob.global_padding || false
  o.authenticated_length = ob.authenticated_length || false
  o.packet_encoding = ob.packet_encoding || ''
  o.encryption = ob.encryption || 'none'

  // Hysteria2 obfs / realm / quic
  if (ob.type === 'hysteria2' && ob.obfs && typeof ob.obfs === 'object') {
    o._hy2ObfsType = ob.obfs.type || ''
    o._hy2ObfsPassword = ob.obfs.password || ''
    o._hy2ObfsMinPacketSize = ob.obfs.min_packet_size || 0
    o._hy2ObfsMaxPacketSize = ob.obfs.max_packet_size || 0
  }
  o._hy2BbrProfile = ob.bbr_profile || ''
  o._hy2BrutalDebug = ob.brutal_debug || false
  o._hy2DisableChromeParrot = ob.disable_chrome_parrot || false
  o.server_ports = Array.isArray(ob.server_ports) ? ob.server_ports.join(',') : (ob.server_ports || '')
  o.hop_interval = ob.hop_interval || ''
  o.hop_interval_max = ob.hop_interval_max || ''
  if (ob.realm && typeof ob.realm === 'object') {
    o._hy2RealmEnabled = true
    o._hy2RealmServerUrl = ob.realm.server_url || ''
    o._hy2RealmToken = ob.realm.token || ''
    o._hy2RealmId = ob.realm.realm_id || ''
    o._hy2RealmStunServers = Array.isArray(ob.realm.stun_servers) ? ob.realm.stun_servers.join(',') : ''
    o._hy2RealmIpVersion = ob.realm.ip_version || 0
    if (ob.realm.port_mapping) {
      o._hy2RealmPortMappingEnabled = ob.realm.port_mapping.enabled || false
      o._hy2RealmPortMappingTimeout = ob.realm.port_mapping.timeout || ''
      o._hy2RealmPortMappingLifetime = ob.realm.port_mapping.lifetime || ''
    }
    if (typeof ob.realm.http_client === 'string') o._hy2RealmHttpClientTag = ob.realm.http_client
  }
  for (const item of [
    ['hy2', '_hy2'], ['tuic', '_tuic'],
  ]) {
    const [, p] = item
    o[p + 'InitialPacketSize'] = ob.initial_packet_size || 0
    o[p + 'DisablePathMtu'] = ob.disable_path_mtu_discovery || false
    o[p + 'IdleTimeout'] = ob.idle_timeout || ''
    o[p + 'KeepAlivePeriod'] = ob.keep_alive_period || ''
    o[p + 'StreamRecvWindow'] = ob.stream_receive_window || ''
    o[p + 'ConnRecvWindow'] = ob.connection_receive_window || ''
    o[p + 'MaxConcurrentStreams'] = ob.max_concurrent_streams || 0
  }
  // Hysteria v1
  o.auth_str = ob.auth_str || ''
  // Naive
  if (ob.type === 'naive') {
    o.quic_congestion_control = ob.quic_congestion_control || ''
    o.insecure_concurrency = ob.insecure_concurrency || 0
    o._naiveExtraHeaders = ob.extra_headers && typeof ob.extra_headers === 'object'
      ? Object.entries(ob.extra_headers).map(([k, v]) => ({ key: k, value: v ? String(v) : '' }))
      : []
    o.stream_receive_window = ob.stream_receive_window || ''
    o._udpOverTCP = !!ob.udp_over_tcp
    o._udpOverTCPVersion = typeof ob.udp_over_tcp === 'object' ? (ob.udp_over_tcp.version || 0) : 0
    o.quic = ob.quic || false
    o.quic_session_receive_window = ob.quic_session_receive_window || ''
  }
  // HTTP / SOCKS
  if (ob.type === 'http') {
    o.path = ob.path || ''
    o.version = ob.version || 0
    o.disable_version_fallback = ob.disable_version_fallback || false
    o._httpHeaders = ob.headers && typeof ob.headers === 'object'
      ? Object.entries(ob.headers).map(([k, v]) => ({ key: k, value: v ? String(v) : '' }))
      : []
    o._httpIdleTimeout = ob.idle_timeout || ''
    o._httpKeepAlivePeriod = ob.keep_alive_period || ''
    o._httpStreamRecvWindow = ob.stream_receive_window || ''
    o._httpConnRecvWindow = ob.connection_receive_window || ''
    o._httpMaxConcurrentStreams = ob.max_concurrent_streams || 0
    o._httpInitialPacketSize = ob.initial_packet_size || 0
    o._httpDisablePathMtu = ob.disable_path_mtu_discovery || false
  }
  if (ob.type === 'socks') {
    o.version = ob.version || '5'
    o._udpOverTCP = !!ob.udp_over_tcp
    o._udpOverTCPVersion = typeof ob.udp_over_tcp === 'object' ? (ob.udp_over_tcp.version || 0) : 0
  }
  // SSH / Tor / Bridge
  o.user = ob.user || ''
  o.private_key = Array.isArray(ob.private_key) ? ob.private_key.join('\n') : (ob.private_key || '')
  o.private_key_path = ob.private_key_path || ''
  o.private_key_passphrase = ob.private_key_passphrase || ''
  o.host_key = Array.isArray(ob.host_key) ? ob.host_key.join(', ') : (ob.host_key || '')
  o.host_key_algorithms = Array.isArray(ob.host_key_algorithms) ? ob.host_key_algorithms.join(',') : (ob.host_key_algorithms || '')
  o.client_version = ob.client_version || ''
  o.cipher = Array.isArray(ob.cipher) ? ob.cipher.join(',') : (ob.cipher || '')
  o.mac = Array.isArray(ob.mac) ? ob.mac.join(',') : (ob.mac || '')
  o.kex_algorithm = Array.isArray(ob.kex_algorithm) ? ob.kex_algorithm.join(',') : (ob.kex_algorithm || '')
  o.executable_path = ob.executable_path || 'tor'
  o.data_directory = ob.data_directory || ''
  o._torExtraArgs = Array.isArray(ob.extra_args) ? ob.extra_args.join(', ') : (ob.extra_args || '')
  o.torrc = ob.torrc && typeof ob.torrc === 'object'
    ? Object.entries(ob.torrc).map(([k, v]) => `${k}=${v}`).join('\n')
    : ''
  o.interface = ob.interface || ''
  o.bridge_name = ob.bridge_name || ''
  o.iproute2_table_index = ob.iproute2_table_index || 0
  o.iproute2_rule_index = ob.iproute2_rule_index || 0
  // selector / urltest
  o._outboundsStr = (ob.outbounds || []).join(', ')
  o.idle_timeout = ob.idle_timeout || ''

  // TLS
  o._tlsEnabled = ob.tls?.enabled || false
  o._tlsServerName = ob.tls?.server_name || ''
  o._tlsAlpn = ob.tls?.alpn || []
  o._tlsInsecure = ob.tls?.insecure || false
  o._tlsDisableSni = ob.tls?.disable_sni || false
  o._tlsEngine = ob.tls?.engine || ''
  o._tlsMinVersion = ob.tls?.min_version || ''
  o._tlsMaxVersion = ob.tls?.max_version || ''
  o._tlsCipherSuites = (ob.tls?.cipher_suites || []).join('\n')
  o._tlsCurvePreferences = ob.tls?.curve_preferences || []
  o._tlsHandshakeTimeout = ob.tls?.handshake_timeout || ''
  o._tlsKernelTx = ob.tls?.kernel_tx || false
  o._tlsKernelRx = ob.tls?.kernel_rx || false
  o._tlsCertificate = (ob.tls?.certificate || []).join('\n')
  o._tlsCertificatePath = ob.tls?.certificate_path || ''
  o._tlsCertificateSha256 = (ob.tls?.certificate_sha256 || []).join('\n')
  o._tlsCertificatePublicKeySha256 = (ob.tls?.certificate_public_key_sha256 || []).join('\n')
  o._tlsClientCertificate = (ob.tls?.client_certificate || []).join('\n')
  o._tlsClientKey = (ob.tls?.client_key || []).join('\n')
  o._tlsClientCertPath = ob.tls?.client_certificate_path || ''
  o._tlsClientKeyPath = ob.tls?.client_key_path || ''
  o._tlsFragment = ob.tls?.fragment || false
  o._tlsSpoof = ob.tls?.spoof || ''
  o._tlsSpoofMethod = ob.tls?.spoof_method || ''
  o._tlsRecordFragment = !!ob.tls?.record_fragment
  o._tlsFragmentFallbackDelay = ob.tls?.fragment_fallback_delay || ''
  o._tlsFingerprint = ob.tls?.utls?.fingerprint || ''
  o._tlsEchEnabled = ob.tls?.ech?.enabled || false
  o._tlsEchConfig = (ob.tls?.ech?.config || []).join('\n')
  o._tlsEchConfigPath = ob.tls?.ech?.config_path || ''
  o._tlsEchQueryServerName = ob.tls?.ech?.query_server_name || ''
  // Transport
  o._transportType = ob.transport?.type || ''
  o._transportPath = ob.transport?.path || ''
  o._transportServiceName = ob.transport?.service_name || ''
  const rawHost = ob.transport?.host
  o._transportHost = Array.isArray(rawHost) ? rawHost.join(',') : (rawHost || '')
  o._transportMaxEarlyData = ob.transport?.max_early_data || 0
  o._transportEarlyDataHeader = ob.transport?.early_data_header_name || ''
  o._transportIdleTimeout = ob.transport?.idle_timeout || ''
  o._transportPingTimeout = ob.transport?.ping_timeout || ''
  o._transportPermitWithoutStream = ob.transport?.permit_without_stream || false
  o._transportMethod = ob.transport?.method || ''
  o._transportHeaders = ob.transport?.headers && typeof ob.transport.headers === 'object'
    ? Object.entries(ob.transport.headers).map(([k, v]) => ({ key: k, value: v ? String(v) : '' }))
    : []
  // Multiplex
  o._multiplexEnabled = ob.multiplex?.enabled || false
  o._multiplexProtocol = ob.multiplex?.protocol || 'smux'
  o._multiplexMaxConn = ob.multiplex?.max_connections || 0
  o._multiplexMinStreams = ob.multiplex?.min_streams || 0
  o._multiplexMaxStreams = ob.multiplex?.max_streams || 0
  o._multiplexPadding = ob.multiplex?.padding || false
  o._multiplexBrutalUp = ob.multiplex?.brutal?.up_mbps || 0
  o._multiplexBrutalDown = ob.multiplex?.brutal?.down_mbps || 0
  // Reality（客户端）
  o._realityEnabled = ob.tls?.reality?.enabled || false
  o._realityPublicKey = ob.tls?.reality?.public_key || ''
  o._realityShortId = ob.tls?.reality?.short_id || ''
  o._utlsFingerprint = ob.tls?.utls?.fingerprint || ''
  return o
}

export function outboundDefaultsForType(type) {
  const base = {
    type, tag: genOutboundTag(type), server: '', server_port: 443,
    uuid: '', password: '', username: '', flow: '', alterId: 0, packet_encoding: '',
    method: '', up: '', down: '', 'up_mbps': '', 'down_mbps': '', congestion_control: 'bbr',
    _outboundsStr: '', default: '', url: '', interval: '', tolerance: 0, idle_timeout: '',
    interrupt_exist_connections: false,
    _tlsEnabled: false, _tlsServerName: '', _tlsAlpn: [], _tlsInsecure: false, _tlsDisableSni: false,
    _tlsEngine: '', _tlsMinVersion: '', _tlsMaxVersion: '', _tlsCipherSuites: '', _tlsCurvePreferences: [],
    _tlsHandshakeTimeout: '', _tlsKernelTx: false, _tlsKernelRx: false,
    _tlsCertificate: '', _tlsCertificatePath: '', _tlsCertificateSha256: '', _tlsCertificatePublicKeySha256: '',
    _tlsClientCertificate: '', _tlsClientKey: '',
    _tlsFragment: false,
    _tlsSpoof: '', _tlsSpoofMethod: '', _tlsRecordFragment: false, _tlsFragmentFallbackDelay: '',
    _tlsFingerprint: '', _tlsEchEnabled: false, _tlsEchConfig: '', _tlsEchConfigPath: '', _tlsEchQueryServerName: '',
    _tlsClientCertPath: '', _tlsClientKeyPath: '',
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
    detour: '', inet4_bind_address: '', inet6_bind_address: '', bind_address_no_port: false,
    protect_path: '', reuse_addr: false, netns: '', connect_timeout: '',
    disable_tcp_keep_alive: false, tcp_keep_alive: '', tcp_keep_alive_interval: '',
    network_strategy: '', _networkType: '', _fallbackNetworkType: '', fallback_delay: '',
    // Snell
    psk: '', version: 0, obfs: '', obfs_host: '', _snellMode: '', userkey: '', reuse: false,
    // Tor
    executable_path: 'tor', data_directory: '', _torExtraArgs: '', torrc: '',
    // SSH
    user: 'root', private_key: '', private_key_path: '', private_key_passphrase: '',
    host_key: '', host_key_algorithms: '', client_version: '', cipher: '', mac: '', kex_algorithm: '',
    // ShadowTLS
    _shadowtlsStrictMode: false, _shadowtlsWildcardSni: 'off',
    // Dial common
    network: '', tcp_fast_open: false, tcp_multi_path: false, udp_fragment: false,
    // VMess
    security: 'auto', global_padding: false, authenticated_length: false,
    // VLESS
    encryption: 'none',
    // TUIC extra
    zero_rtt_handshake: false, heartbeat: '', udp_relay_mode: '', udp_over_stream: false,
    // Hysteria / Hysteria2
    _hy2ObfsType: '', _hy2ObfsPassword: '', _hy2ObfsMinPacketSize: 0, _hy2ObfsMaxPacketSize: 0,
    _hy2BbrProfile: '', _hy2BrutalDebug: false, _hy2DisableChromeParrot: false,
    _hy2RealmEnabled: false, _hy2RealmServerUrl: '', _hy2RealmToken: '', _hy2RealmId: '',
    _hy2RealmStunServers: '', _hy2RealmIpVersion: 0,
    _hy2RealmPortMappingEnabled: false, _hy2RealmPortMappingTimeout: '', _hy2RealmPortMappingLifetime: '',
    _hy2RealmHttpClientTag: '',
    server_ports: '', hop_interval: '', hop_interval_max: '',
    _hy2InitialPacketSize: 0, _hy2DisablePathMtu: false,
    _hy2IdleTimeout: '', _hy2KeepAlivePeriod: '', _hy2StreamRecvWindow: '', _hy2ConnRecvWindow: '', _hy2MaxConcurrentStreams: 0,
    _tuicInitialPacketSize: 0, _tuicDisablePathMtu: false,
    _tuicIdleTimeout: '', _tuicKeepAlivePeriod: '', _tuicStreamRecvWindow: '', _tuicConnRecvWindow: '', _tuicMaxConcurrentStreams: 0,
    // AnyTLS
    idle_session_check_interval: '', idle_session_timeout: '', min_idle_session: 0, client_metadata: '',
    // Naive
    quic_congestion_control: 'bbr', insecure_concurrency: 0, stream_receive_window: '',
    quic: false, quic_session_receive_window: '', _naiveExtraHeaders: [],
    // HTTP / SOCKS
    path: '', disable_version_fallback: false, _httpHeaders: [],
    _httpIdleTimeout: '', _httpKeepAlivePeriod: '', _httpStreamRecvWindow: '', _httpConnRecvWindow: '',
    _httpMaxConcurrentStreams: 0, _httpInitialPacketSize: 0, _httpDisablePathMtu: false,
    _udpOverTCP: false, _udpOverTCPVersion: 0,
    // Shadowsocks
    plugin: '', plugin_opts: '',
    // Bridge
    interface: '', bridge_name: '', iproute2_table_index: 0, iproute2_rule_index: 0,
    auth_str: '',
  }
  if (['direct', 'block', 'selector', 'urltest', 'tor', 'bridge'].includes(type)) {
    delete base.server; delete base.server_port
  }
  if (type === 'shadowsocks2022') {
    base.method = '2022-blake3-aes-256-gcm'
  }
  if (type === 'snell') {
    // 出站只接受 version 4 / 6（入站是 5/6）
    base.version = 4
    base.psk = generateKey()
  }
  if (type === 'tuic') {
    base.password = generatePassword()
  }
  if (type === 'socks') {
    base.version = '5'
  }
  return base
}
