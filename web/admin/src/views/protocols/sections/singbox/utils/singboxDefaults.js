// sing-box 公用工具：协议常量 + 自动填参 + 密钥/用户生成
// sing-box 源码：/opt/sing-box/option/*.go（权威字段来源）
// 节点「名称 / 标签」生成器与 xray 共用（见 sections/utils/nodeNaming.js），不另起一套。
import { genRemark, genInboundTag } from '../../utils/nodeNaming'

// 需要 TLS 的 Inbound 协议（InboundTLSOptionsContainer，源码验证）
// vless/vmess/trojan/hysteria2/tuic/naive/anytls/mixed/hysteria 有 InboundTLSOptionsContainer
// socks/shadowsocks/shadowtls/snell/wireguard/tun 无 TLS 字段
export const TLS_ENABLED_PROTOCOLS = ['vless', 'vmess', 'trojan', 'hysteria2', 'tuic', 'naive', 'anytls', 'mixed', 'http', 'hysteria']

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

// 出站（客户端）协议：支持 TLS（源码校验：snell / socks 没有 OutboundTLSOptionsContainer）
export const OUTBOUND_TLS_PROTOCOLS = ['vmess', 'vless', 'trojan', 'hysteria2', 'hysteria', 'tuic', 'http', 'anytls', 'shadowtls', 'naive']
// 出站（客户端）协议：支持 V2Ray Transport
export const OUTBOUND_TRANSPORT_PROTOCOLS = ['vmess', 'vless', 'trojan']
// 出站（客户端）协议：支持 Multiplex
export const OUTBOUND_MULTIPLEX_PROTOCOLS = ['vmess', 'vless', 'trojan', 'shadowsocks']
// 出站（客户端）协议：使用服务器地址（server/server_port）
export const OUTBOUND_SERVER_PROTOCOLS = ['vmess', 'vless', 'trojan', 'hysteria2', 'hysteria', 'shadowsocks', 'shadowsocks2022', 'tuic', 'anytls', 'shadowtls', 'snell', 'naive', 'http', 'socks', 'ssh']
// 出站（客户端）协议：有 network 字段的协议（sing-box 源码校验）
// trojan/anytls/shadowtls/naive/http 没有 network 字段，写出去会 unknown field
export const OUTBOUND_NETWORK_PROTOCOLS = ['vmess', 'vless', 'shadowsocks', 'shadowsocks2022', 'hysteria2', 'hysteria', 'tuic', 'snell', 'socks']
// 出站（客户端）协议：协议专属设置（需要 Protocol tab）
export const OUTBOUND_PROTOCOL_PROTOCOLS = ['vmess', 'vless', 'trojan', 'shadowsocks', 'shadowsocks2022', 'hysteria2', 'hysteria', 'tuic', 'anytls', 'shadowtls', 'snell', 'naive', 'bridge', 'http', 'socks', 'selector', 'urltest', 'direct', 'block', 'dns', 'tor', 'ssh']

// 该协议支持的 Inbound type 白名单
// direct：静态端口转发（override_address/override_port），sing-box 官方文档 inbound/direct
export const INBOUND_TYPES = [
  'mixed', 'http', 'socks', 'naive', 'tun', 'redirect', 'tproxy', 'direct',
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

// 凭据生成统一走 sections/utils/credentials（与 xray 编辑器、后端 client/credentials.go 同口径）：
// UUID v4；密码 = 32 字节随机数 base64；psk = 32 字节随机数 base64url。
import { generateUuid, generatePassword, generatePsk } from '../../utils/credentials'
export { generateUuid, generatePassword }
// 注意：export { generatePsk as generateKey } 只建立导出别名，不会在本模块内定义
// generateKey；本文件里的 userDefaultsForType 需要本地绑定。
export const generateKey = generatePsk

export function generateUsername() { return 'user' + Math.random().toString(36).substring(2, 8) }

export function generateEmail() { return Math.random().toString(36).substring(2, 8) + '@singbox.com' }

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
  if (type === 'tuic') u.password = generatePassword()
  if (PSK_PROTOCOLS.includes(type)) u.password = generateKey()
  if (AUTH_USER_PROTOCOLS.includes(type)) { u.username = generateUsername(); u.password = generatePassword() }
  if (USERKEY_PROTOCOLS.includes(type)) u.userkey = generateKey()
  if (type === 'vless') u.flow = 'xtls-rprx-vision'
  if (type === 'vmess') u.alterId = 0
  return u
}

// 按协议类型生成完整 inbound 默认参数（打开新入站就自动填入，可手动修改）
export function inboundDefaultsForType(type, customPort) {
  // customPort 由调用方传入（通常来自后端 free-port 接口，保证不与已有入站重复）
  const port = customPort > 0 ? customPort : generatePort()
  const base = {
    type,
    tag: genInboundTag(type, port),
    // sing-box 默认监听地址就是 "::"（Go 的 IPv6 通配地址，Linux 下双栈同时收 v4/v6）。
    // 写 0.0.0.0 只监听 IPv4，IPv6 客户端会连不上。
    listen: '::',
    listen_port: port,
    domain_strategy: '',
    sniff: true,
    sniff_override_destination: false,
    // 通用监听字段（sing-box Listen Fields）
    tcp_fast_open: false,
    tcp_multi_path: false,
    udp_fragment: false,
    udp_timeout: '',
    detour: '',
    // Listen Fields（sing-box ListenOptions，源码 inbound.go）
    bind_interface: '', routing_mark: 0, reuse_addr: false, netns: '',
    disable_tcp_keep_alive: false, tcp_keep_alive: '', tcp_keep_alive_interval: '',
    // socks / http / mixed 入站支持 domain_resolver（字符串形式或对象）
    domain_resolver: '',
    // http / mixed
    set_system_proxy: false,
    _httpVersion: '1,2',
    _httpIdleTimeout: '', _httpKeepAlivePeriod: '', _httpStreamRecvWindow: '',
    _httpConnRecvWindow: '', _httpMaxConcurrentStreams: 0,
    _httpInitialPacketSize: 0, _httpDisablePathMtu: false,
    // AnyTLS padding_scheme（每行一条）
    _anytlsPaddingScheme: '',
    // Trojan fallback
    _trojanFallbackServer: '', _trojanFallbackPort: 0, _trojanFallbackHsn: [],
    // Port Hopping（端口跳跃）：字段与 xray 一致，供防火墙规则增删与恢复读取
    _portHopEnabled: false,
    _portHopMode: 'preset',
    _portHopRange: '10000-19999',
    _portHopStart: 20000,
    _portHopEnd: 50000,
    _portHopInterface: '',
    // direct inbound（静态转发）
    override_address: '',
    override_port: 0,
    interface_name: '',
    stack: 'system',
    auto_route: true,
    strict_route: false,
    network: '',
    users: USER_PROTOCOLS.includes(type) ? [userDefaultsForType(type)] : [],
    method: '2022-blake3-aes-256-gcm',
    password: '',
    // shadowsocks inbound：多目的地（destinations）与托管模式（option/shadowsocks.go）
    destinations: [], managed: false,
    // hysteria2
    up: '', down: '', up_mbps: '', down_mbps: '', ignore_client_bandwidth: false, masquerade: '',
    // hysteria2 obfs (sing-box: {type, password} + gecko {min_packet_size,max_packet_size})
    _hy2ObfsType: '', _hy2ObfsPassword: '',
    _hy2ObfsMinPacketSize: 0, _hy2ObfsMaxPacketSize: 0,
    // hysteria2 masquerade（type: none/proxy/file/string）
    _hy2MasqType: 'none', _hy2MasqUrl: '', _hy2MasqRewriteHost: false,
    _hy2MasqDirectory: '', _hy2MasqStatusCode: 0, _hy2MasqContent: '',
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
    // 入站 TLS 其余字段（option/tls.go InboundTLSOptions）
    _tlsMaxVersion: '', _tlsCipherSuites: '', _tlsCurvePreferences: [],
    _tlsHandshakeTimeout: '', _tlsClientAuth: '', _tlsClientCertPath: '',
    _tlsClientCertSha256: '', _tlsClientCertPubKeySha256: '',
    _tlsKernelTx: false, _tlsKernelRx: false,
    // 入站 ECH（InboundECHOptions）
    _tlsEchEnabled: false, _tlsEchKey: '', _tlsEchKeyPath: '',
    _realityEnabled: false, _realityHandshakeServer: '', _realityHandshakePort: 443,
    _realityHandshakeDetour: '', _realityHandshakeBindInterface: '', _realityHandshakeConnectTimeout: '', _realityHandshakeTcpFastOpen: false,
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
    _multiplexBrutalUp: 0, _multiplexBrutalDown: 0,
    // remark + enabled + access domain (对齐 xray)
    _remark: genRemark(), _enabled: true,
    _accessDomainEnabled: false, _accessDomainType: 'direct',
    _directDomain: '', _directPortMode: 'config',
    _cfDomain: '', _cfIP: '', _customDomain: '',
    _customPortMode: 'custom', _accessCustomPort: 443,
    _accessTlsEnabled: false, _accessTlsSni: '', _accessTlsAlpn: '', _accessTlsFingerprint: '',
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
    // handshake DialerOptions 其余字段
    base._shadowtlsHandshakeInet4BindAddress = ''
    base._shadowtlsHandshakeInet6BindAddress = ''
    base._shadowtlsHandshakeBindAddressNoPort = false
    base._shadowtlsHandshakeNetns = ''
    base._shadowtlsHandshakeTcpKeepAlive = ''
    base._shadowtlsHandshakeTcpKeepAliveInterval = ''
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
    base._cfdControlReuseAddr = false
    base._cfdControlConnectTimeout = ''
    base._cfdControlTcpFastOpen = false
    base._cfdControlTcpMultiPath = false
    base._cfdControlDisableTcpKeepAlive = false
    base._cfdControlTcpKeepAlive = ''
    base._cfdControlTcpKeepAliveInterval = ''
    base._cfdControlNetns = ''
    base._cfdControlInet4BindAddress = ''
    base._cfdControlInet6BindAddress = ''
    base._cfdControlBindAddressNoPort = false
    base._cfdTunnelDetour = ''
    base._cfdTunnelBindInterface = ''
    base._cfdTunnelRoutingMark = 0
    base._cfdTunnelReuseAddr = false
    base._cfdTunnelConnectTimeout = ''
    base._cfdTunnelTcpFastOpen = false
    base._cfdTunnelTcpMultiPath = false
    base._cfdTunnelDisableTcpKeepAlive = false
    base._cfdTunnelTcpKeepAlive = ''
    base._cfdTunnelTcpKeepAliveInterval = ''
    base._cfdTunnelNetns = ''
    base._cfdTunnelInet4BindAddress = ''
    base._cfdTunnelInet6BindAddress = ''
    base._cfdTunnelBindAddressNoPort = false
  }
  return base
}

// 通用公网访问地址解析（镜像 xray 的 getAccessHostPort）
export function resolveAccessHost(ib, serverInfo) {
  // serverInfo: { address, domain } from parent if available
  const listen = ib && ib.listen
  // 通配监听地址不能直接当访问地址展示（0.0.0.0 / :: / [::]）
  const wildcard = !listen || listen === '0.0.0.0' || listen === '::' || listen === '[::]'
  return (serverInfo && (serverInfo.domain || serverInfo.address)) || (wildcard ? '' : listen) || ''
}