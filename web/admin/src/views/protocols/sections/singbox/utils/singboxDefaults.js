// sing-box 公用工具：协议常量 + 自动填参 + 密钥/用户生成
// sing-box 源码：/opt/sing-box/option/*.go（权威字段来源）

// 需要 TLS 的 Inbound 协议（InboundTLSOptionsContainer）
export const TLS_ENABLED_PROTOCOLS = ['vless', 'vmess', 'trojan', 'hysteria2', 'tuic', 'shadowsocks', 'mixed', 'http', 'socks', 'naive', 'anytls', 'shadowtls', 'hysteria']

// 支持 V2Ray Transport 的协议
export const TRANSPORT_PROTOCOLS = ['vless', 'vmess', 'trojan', 'shadowsocks', 'mixed', 'http', 'socks', 'naive']

// 支持 Multiplex 的协议
export const MULTIPLEX_PROTOCOLS = ['vless', 'vmess', 'trojan', 'shadowsocks']

// 有用户数组的协议
export const USER_PROTOCOLS = ['vless', 'vmess', 'trojan', 'hysteria2', 'tuic', 'shadowsocks', 'mixed', 'http', 'socks', 'naive', 'anytls', 'shadowtls', 'hysteria', 'snell']

// 用户使用 uuid 的协议（VLESS/VMess/TUIC 用户有 uuid 字段）
export const UUID_PROTOCOLS = ['vless', 'vmess', 'tuic']

// 用户使用 password 的协议
export const PASSWORD_PROTOCOLS = ['trojan', 'hysteria2', 'shadowsocks', 'hysteria', 'anytls', 'shadowtls']

// 用户使用 username+password（auth.User: socks/http/mixed/naive）
export const AUTH_USER_PROTOCOLS = ['mixed', 'http', 'socks', 'naive']

// 用户使用 userkey 的协议（snell）
export const USERKEY_PROTOCOLS = ['snell']

// 非用户型（wireguard 用 peers）
export const NON_USER_PROTOCOLS = ['wireguard']

// 该协议支持的 Inbound type 白名单
export const INBOUND_TYPES = [
  'mixed', 'http', 'socks', 'naive', 'tun',
  'vless', 'vmess', 'trojan', 'hysteria2', 'shadowsocks', 'tuic',
  'anytls', 'shadowtls', 'hysteria', 'wireguard', 'snell',
]

// 每种协议默认监听端口（打开就自动填入）
export const DEFAULT_PORTS = {
  vless: 443, vmess: 443, trojan: 443, shadowsocks: 8388, hysteria2: 8443,
  tuic: 8443, mixed: 1080, http: 8080, socks: 1080, naive: 80,
  anytls: 8443, shadowtls: 8443, hysteria: 36712, wireguard: 51820, snell: 6153,
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

// 按协议类型给单个用户生成默认凭证字段
export function userDefaultsForType(type) {
  const u = { name: generateEmail() }
  if (UUID_PROTOCOLS.includes(type)) u.uuid = generateUuid()
  if (PASSWORD_PROTOCOLS.includes(type)) u.password = generatePassword()
  if (AUTH_USER_PROTOCOLS.includes(type)) { u.username = generateUsername(); u.password = generatePassword() }
  if (USERKEY_PROTOCOLS.includes(type)) u.userkey = generateKey()
  if (type === 'vless') u.flow = 'xtls-rprx-vision'
  if (type === 'vmess') u.alterId = 0
  return u
}

// 按协议类型生成完整 inbound 默认参数（打开新入站就自动填入，可手动修改）
export function inboundDefaultsForType(type) {
  const base = {
    type,
    tag: type + '-in',
    listen: '0.0.0.0',
    listen_port: DEFAULT_PORTS[type] || 1080,
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
    // tuic
    congestion_control: 'bbr', auth_timeout: '', max_idle_time: '',
    // shadowtls / snell
    version: type === 'shadowtls' ? 3 : type === 'snell' ? 4 : 0,
    psk: '',
    obfs: '', obfs_host: '',
    // tls
    _tlsEnabled: !['tun', 'wireguard', 'redirect', 'tproxy'].includes(type),
    _tlsServerName: '', _tlsAlpn: ['h2', 'http/1.1'], _tlsMinVersion: '1.2', _tlsCertPath: '', _tlsKeyPath: '',
    _realityEnabled: false, _realityHandshakeServer: '', _realityHandshakePort: 443,
    _realityPrivateKey: '', _realityPublicKey: '', _realityShortIds: '', _realityMaxTimeDiff: '',
    _utlsEnabled: false, _utlsFingerprint: 'chrome',
    // transport
    _transportType: '', _transportPath: '/', _transportHost: '', _transportServiceName: '',
    _transportMaxEarlyData: 0, _transportEarlyDataHeader: '', _transportIdleTimeout: '', _transportPingTimeout: '', _transportSeed: '',
    // multiplex
    _multiplexEnabled: false, _multiplexProtocol: 'smux', _multiplexMaxConn: 0,
    _multiplexMinStreams: 0, _multiplexMaxStreams: 0, _multiplexPadding: false,
  }
  if (!TRANSPORT_PROTOCOLS.includes(type)) base._transportType = ''
  if (type === 'shadowsocks') { base.network = 'tcp,udp' }
  if (type === 'hysteria') { base.obfs = 'salamander'; base.up = '100 Mbps'; base.down = '100 Mbps' }
  return base
}

// 通用公网访问地址解析（镜像 xray 的 getAccessHostPort）
export function resolveAccessHost(ib, serverInfo) {
  // serverInfo: { address, domain } from parent if available
  return (serverInfo && (serverInfo.domain || serverInfo.address)) || (ib && ib.listen === '0.0.0.0' ? '' : (ib && ib.listen)) || ''
}