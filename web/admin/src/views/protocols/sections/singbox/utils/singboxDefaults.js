// sing-box 公用工具：协议常量 + 自动填参 + 密钥/用户生成
// sing-box 源码：/opt/sing-box/option/*.go（权威字段来源）

// 需要 TLS 的 Inbound 协议（InboundTLSOptionsContainer，源码验证）
// vless/vmess/trojan/hysteria2/tuic/naive/anytls/mixed/hysteria 有 InboundTLSOptionsContainer
// socks/shadowsocks/shadowtls/snell/wireguard/tun 无 TLS 字段
export const TLS_ENABLED_PROTOCOLS = ['vless', 'vmess', 'trojan', 'hysteria2', 'tuic', 'naive', 'anytls', 'mixed', 'hysteria']

// 支持 V2Ray Transport 的协议（源码验证：仅 vless/vmess/trojan 有 Transport 字段）
export const TRANSPORT_PROTOCOLS = ['vless', 'vmess', 'trojan']

// 支持 Multiplex 的协议（源码验证：vless/vmess/trojan/shadowsocks 有 Multiplex 字段）
export const MULTIPLEX_PROTOCOLS = ['vless', 'vmess', 'trojan', 'shadowsocks']

// 有用户数组的协议
export const USER_PROTOCOLS = ['vless', 'vmess', 'trojan', 'hysteria2', 'tuic', 'shadowsocks', 'mixed', 'http', 'socks', 'naive', 'anytls', 'shadowtls', 'hysteria', 'snell']

// 用户使用 uuid 的协议（VLESS/VMess/TUIC 用户有 uuid 字段）
export const UUID_PROTOCOLS = ['vless', 'vmess', 'tuic']

// 用户使用 password 的协议
export const PASSWORD_PROTOCOLS = ['trojan', 'hysteria2', 'shadowsocks', 'hysteria', 'anytls']

// 用户密码用 base64url key 的协议（password 字段，但用 generateKey 生成）
export const PSK_PROTOCOLS = ['shadowtls']

// 用户使用 username+password（auth.User: socks/http/mixed/naive）
export const AUTH_USER_PROTOCOLS = ['mixed', 'http', 'socks', 'naive']

// 用户使用 userkey 的协议（snell）
export const USERKEY_PROTOCOLS = ['snell']

// 保留此常量用于兼容性（wireguard 已独立为 endpoint）
export const NON_USER_PROTOCOLS = []

// Reality 支持矩阵（sing-box 官方）
// 协议仅限 vless / vmess / trojan
export const REALITY_PROTOCOLS = ['vless', 'vmess', 'trojan']
// 传输仅限 tcp(无 transport) / http / ws / httpupgrade / grpc
export const REALITY_TRANSPORTS = ['', 'http', 'ws', 'httpupgrade', 'grpc']
export const REALITY_TRANSPORT_LABELS = { '': 'TCP', http: 'HTTP', ws: 'WebSocket', httpupgrade: 'HTTPUpgrade', grpc: 'gRPC' }
// 该协议是否支持 Reality（协议 + 传输组合）
export function supportsReality(type, transportType) {
  return REALITY_PROTOCOLS.includes(type) && REALITY_TRANSPORTS.includes(transportType || '')
}

// ---------- 出站支持矩阵 ----------

// 出站（客户端）协议：支持 TLS
export const OUTBOUND_TLS_PROTOCOLS = ['vmess', 'vless', 'trojan', 'hysteria2', 'tuic', 'http', 'socks']
// 出站（客户端）协议：支持 V2Ray Transport
export const OUTBOUND_TRANSPORT_PROTOCOLS = ['vmess', 'vless', 'trojan']
// 出站（客户端）协议：支持 Multiplex
export const OUTBOUND_MULTIPLEX_PROTOCOLS = ['vmess', 'vless', 'trojan', 'shadowsocks']
// 出站（客户端）协议：使用服务器地址（server/server_port）
export const OUTBOUND_SERVER_PROTOCOLS = ['vmess', 'vless', 'trojan', 'hysteria2', 'shadowsocks', 'tuic', 'http', 'socks']
// 出站（客户端）协议：协议专属设置（需要 Protocol tab）
export const OUTBOUND_PROTOCOL_PROTOCOLS = ['vmess', 'vless', 'trojan', 'shadowsocks', 'hysteria2', 'tuic', 'http', 'socks', 'selector', 'urltest', 'direct', 'block', 'dns']

// 该协议支持的 Inbound type 白名单
export const INBOUND_TYPES = [
  'mixed', 'http', 'socks', 'naive', 'tun', 'redirect', 'tproxy',
  'vless', 'vmess', 'trojan', 'hysteria2', 'shadowsocks', 'tuic',
  'anytls', 'shadowtls', 'hysteria', 'snell', 'cloudflared',
]

// 每种协议默认监听端口（打开就自动填入）
export const DEFAULT_PORTS = {
  vless: 443, vmess: 443, trojan: 443, shadowsocks: 8388, hysteria2: 8443,
  tuic: 8443, mixed: 1080, http: 8080, socks: 1080, naive: 80,
  anytls: 8443, shadowtls: 8443, hysteria: 36712, snell: 6153,
  redirect: 1080,
  tproxy: 1080,
  tun: 0,
}

// ---------- 生成器 ----------

export function generateUuid() {
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function (c) {
    const r = (Math.random() * 16) | 0
    const v = c === 'x' ? r : (r & 0x3) | 0x8
    return v.toString(16)
  })
}

export function generatePassword(len = 16) {
  const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789'
  let r = ''
  for (let i = 0; i < len; i++) r += chars.charAt(Math.floor(Math.random() * chars.length))
  return r
}

export function generateUsername() { return 'user' + Math.random().toString(36).substring(2, 8) }

export function generateEmail() { return Math.random().toString(36).substring(2, 8) + '@singbox.com' }

// base64url 随机密钥（用于 snell userkey 等）
export function generateKey() {
  const bytes = crypto.getRandomValues(new Uint8Array(32))
  return btoa(String.fromCharCode(...bytes)).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

// 随机未使用端口（对齐 xray：10000-60000 范围随机）
export function generatePort() { return Math.floor(Math.random() * 50000 + 10000) }

// 生成随机 ID（小写字母+数字，对齐 xray genId）
export function genRandomId(len = 12) {
  const chars = 'abcdefghijklmnopqrstuvwxyz0123456789'
  let r = ''
  for (let i = 0; i < len; i++) r += chars.charAt(Math.floor(Math.random() * chars.length))
  return r
}

// 生成随机路径（带 early data 参数，对齐 xray genPath）
export function genRandomPath() { return '/' + genRandomId(12) + '?ed=2560' }

// 按协议类型给单个用户生成默认凭证字段
export function userDefaultsForType(type) {
  const u = { name: generateEmail() }
  if (UUID_PROTOCOLS.includes(type)) u.uuid = generateUuid()
  if (PASSWORD_PROTOCOLS.includes(type)) u.password = generatePassword()
  if (PSK_PROTOCOLS.includes(type)) u.password = generateKey()
  if (AUTH_USER_PROTOCOLS.includes(type)) { u.username = generateUsername(); u.password = generatePassword() }
  if (USERKEY_PROTOCOLS.includes(type)) u.userkey = generateKey()
  if (type === 'vless') u.flow = 'xtls-rprx-vision'
  if (type === 'vmess') u.alterId = 0
  return u
}

// 按协议类型生成完整 inbound 默认参数（打开新入站就自动填入，可手动修改）
export function inboundDefaultsForType(type) {
  const port = generatePort()
  const base = {
    type,
    tag: 'inbound-' + type + '-' + port,
    listen: '0.0.0.0',
    listen_port: port,
    domain_strategy: '',
    sniff: true,
    sniff_override_destination: false,
    interface_name: '',
    stack: 'system',
    auto_route: true,
    strict_route: false,
    network: '',
    users: USER_PROTOCOLS.includes(type) ? [userDefaultsForType(type)] : [],
    method: '2022-blake3-aes-256-gcm',
    password: '',
    // hysteria2
    up: '', down: '', up_mbps: '', down_mbps: '', ignore_client_bandwidth: false, masquerade: '',
    // hysteria2 obfs (sing-box: {type, password} 结构体)
    _hy2ObfsType: '', _hy2ObfsPassword: '',
    // hysteria2 bbr_profile (standard/conservative/aggressive)
    _hy2BbrProfile: '',
    // hysteria2 QUIC options (QUICOptions = HTTP2Options + QUIC fields)
    _hy2InitialPacketSize: 0, _hy2DisablePathMtu: false,
    _hy2IdleTimeout: '', _hy2KeepAlivePeriod: '',
    _hy2StreamRecvWindow: '', _hy2ConnRecvWindow: '', _hy2MaxConcurrentStreams: 0,
    // hysteria2 brutal_debug
    _hy2BrutalDebug: false,
    // hysteria2 realm (可选，P2P 代理功能)
    _hy2RealmEnabled: false,
    _hy2RealmServerUrl: '', _hy2RealmToken: '', _hy2RealmId: '',
    _hy2RealmStunServers: '', _hy2RealmIpVersion: 0,
    _hy2RealmPortMappingEnabled: false, _hy2RealmPortMappingTimeout: '', _hy2RealmPortMappingLifetime: '',
    _hy2RealmHttpClientTag: '', _hy2RealmStunDnsServer: '',
    // tuic
    congestion_control: 'bbr', auth_timeout: '', max_idle_time: '',
    // tun specific (Listable fields stored as comma-separated strings)
    _tunNetns: '', _tunMtu: 0, _tunAddress: '', _tunDnsMode: '', _tunDnsAddress: '',
    _tunIpRoute2Table: 0, _tunIpRoute2Rule: 0, _tunAutoRedirect: false,
    _tunAutoRedirectInputMark: '', _tunAutoRedirectOutputMark: '', _tunAutoRedirectResetMark: '', _tunAutoRedirectTproxyMark: '',
    _tunAutoRedirectNFQueue: 0, _tunAutoRedirectFallbackRule: 0,
    _tunExcludeMptcp: false, _tunLoopbackAddress: '',
    _tunRouteAddress: '', _tunRouteAddressSet: '', _tunRouteExcludeAddress: '', _tunRouteExcludeAddressSet: '',
    _tunIncludeInterface: '', _tunExcludeInterface: '',
    _tunIncludeUid: '', _tunIncludeUidRange: '', _tunExcludeUid: '', _tunExcludeUidRange: '',
    _tunIncludeAndroidUser: '', _tunIncludePackage: '', _tunExcludePackage: '',
    _tunIncludeMacAddress: '', _tunExcludeMacAddress: '',
    _tunUdpTimeout: '', _tunUdpMapping: '', _tunUdpFiltering: '', _tunUdpNatMax: 0,
    // platform http_proxy
    _tunPlatformHttpProxyEnabled: false, _tunPlatformHttpProxyServer: '', _tunPlatformHttpProxyPort: 0,
    _tunPlatformHttpProxyBypassDomain: '', _tunPlatformHttpProxyMatchDomain: '',
    // shadowtls / snell
    version: type === 'shadowtls' ? 3 : type === 'snell' ? 5 : 0,
    psk: '',
    obfs: '', obfs_host: '',
    _snellMode: '', // snell v6 mode: default / unshaped / unsafe-raw
    // tls
    _tlsEnabled: TLS_ENABLED_PROTOCOLS.includes(type),
    _tlsServerName: '', _tlsAlpn: type === 'hysteria2' || type === 'tuic' || type === 'hysteria' ? ['h3'] : ['h2', 'http/1.1'], _tlsMinVersion: '1.2', _tlsCertPath: '', _tlsKeyPath: '',
    _realityEnabled: false, _realityHandshakeServer: '', _realityHandshakePort: 443,
    _realityPrivateKey: '', _realityPublicKey: '', _realityShortIds: '', _realityMaxTimeDiff: '',
    _utlsEnabled: false, _utlsFingerprint: 'chrome',
    _certId: '',
    // transport（参考 docs/configuration/shared/v2ray-transport.md）
    // 默认 grpc，自动生成 service_name（对齐 xray 的 genId(12)）
    _transportType: TRANSPORT_PROTOCOLS.includes(type) ? 'grpc' : '',
    _transportPath: '/', _transportHost: '',
    _transportServiceName: TRANSPORT_PROTOCOLS.includes(type) ? genRandomId(12) : '',
    _transportMaxEarlyData: 0, _transportEarlyDataHeader: '', _transportIdleTimeout: '', _transportPingTimeout: '',
    _transportMethod: '', _transportHeaders: [],
    _transportPermitWithoutStream: false,
    // multiplex
    _multiplexEnabled: false, _multiplexProtocol: 'smux', _multiplexMaxConn: 0,
    _multiplexMinStreams: 0, _multiplexMaxStreams: 0, _multiplexPadding: false,
    // remark + enabled + access domain (对齐 xray)
    _remark: 'Node-' + Math.random().toString(36).substring(2, 8), _enabled: true,
    _accessDomainEnabled: false, _accessDomainType: 'direct',
    _directDomain: '', _directPortMode: 'config',
    _cfDomain: '', _cfIP: '', _customDomain: '',
    _customPortMode: 'custom', _accessCustomPort: 443,
  }
  if (!TRANSPORT_PROTOCOLS.includes(type)) base._transportType = ''
  if (type === 'shadowsocks') { base.network = 'tcp,udp' }
  if (type === 'hysteria') { base.obfs = 'salamander'; base.up = '100 Mbps'; base.down = '100 Mbps' }
  if (type === 'naive') { base.network = 'tcp,udp'; base.quic_congestion_control = 'bbr' }
  if (type === 'shadowtls') {
    base.version = 3
    // ShadowTLS password: 普通字符串（TLS 握手密码，官方文档示例为可读字符串）
    base.password = generatePassword()
    // handshake: {server, server_port, ...dial}
    base._shadowtlsHandshakeServer = ''
    base._shadowtlsHandshakePort = 443
    // handshake Dial 字段 (DialerOptions 常用子集)
    base._shadowtlsHandshakeDetour = ''
    base._shadowtlsHandshakeBindInterface = ''
    base._shadowtlsHandshakeRoutingMark = 0
    base._shadowtlsHandshakeReuseAddr = false
    base._shadowtlsHandshakeConnectTimeout = ''
    base._shadowtlsHandshakeTcpFastOpen = false
    base._shadowtlsHandshakeTcpMultiPath = false
    base._shadowtlsHandshakeDisableTcpKeepAlive = false
    base._shadowtlsHandshakeDomainResolver = ''
    // 顶层参数
    base._shadowtlsStrictMode = false
    base._shadowtlsWildcardSni = ''
    // handshake_for_server_name: 数组形式 [{sni, server, server_port, ...dial}]
    base._shadowtlsHsn = []
    // ShadowTLS 用户密码: base64url 编码的随机 32 字节
    if (base.users?.[0]) base.users[0].password = generateKey()
  }
  if (type === 'tuic') {
    base.congestion_control = 'bbr'
    base.auth_timeout = ''
    base.zero_rtt_handshake = false
    base.heartbeat = ''
    // QUIC options (= HTTP2Options + QUIC fields)
    base._tuicInitialPacketSize = 0
    base._tuicDisablePathMtu = false
    // HTTP2 fields
    base._tuicIdleTimeout = ''
    base._tuicKeepAlivePeriod = ''
    base._tuicStreamRecvWindow = ''
    base._tuicConnRecvWindow = ''
    base._tuicMaxConcurrentStreams = 0
  }
  if (type === 'snell') {
    base.psk = generateKey()
  }
  if (type === 'tun') {
    base.interface_name = 'tun0'
    base.stack = 'system'
    base.auto_route = true
    base.strict_route = true
    base._tunAddress = '172.18.0.1/30,fdfe:dcba:9876::1/126'
    base._tunDnsMode = 'hijack'
    base._tunDnsAddress = '172.18.0.1'
    base._tunAutoRoute = true
    base._tunUdpTimeout = ''
    base._tunUdpMapping = 'endpoint_independent'
    base._tunUdpFiltering = 'endpoint_independent'
  }
  if (type === 'tproxy') {
    base._tproxyNetwork = 'tcp,udp'
    base._tproxyUdpMapping = ''
    base._tproxyUdpFiltering = ''
    base._tproxyUdpNatMax = 0
  }
  if (type === 'cloudflared') {
    // Cloudflare Tunnel inbound - no local port needed (edge-initiated)
    base.listen_port = 0
    base.token = ''
    base.ha_connections = 0
    base.protocol = ''
    base.post_quantum = false
    base.edge_ip_version = 0
    base._edgeIpVersion = '0'
    base.datagram_version = ''
    base.grace_period = ''
    base.region = ''
    base._cfdControlDetour = ''
    base._cfdControlBindInterface = ''
    base._cfdControlRoutingMark = 0
    base._cfdTunnelDetour = ''
    base._cfdTunnelBindInterface = ''
    base._cfdTunnelRoutingMark = 0
  }
  return base
}

// 通用公网访问地址解析（镜像 xray 的 getAccessHostPort）
export function resolveAccessHost(ib, serverInfo) {
  // serverInfo: { address, domain } from parent if available
  return (serverInfo && (serverInfo.domain || serverInfo.address)) || (ib && ib.listen === '0.0.0.0' ? '' : (ib && ib.listen)) || ''
}