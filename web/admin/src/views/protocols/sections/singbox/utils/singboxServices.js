// sing-box 顶层 services[] 编辑支持。
//
// 目前只有 hysteria-realm 做了结构化表单（它是 hysteria2 的 rendezvous 服务，
// 面板以前只能手写 JSON）；其它服务类型仍沿用「按类型添加 + JSON 编辑」。
//
// 字段来源：option/hysteria2.go HysteriaRealmServiceOptions
//   ListenOptions + InboundTLSOptionsContainer + HTTP2Options + users[]
import { generatePsk } from '../../utils/credentials'

// 支持的服务类型（与 sing-box constant/proxy.go 对齐）
export const SERVICE_TYPES = [
  'hysteria-realm', 'api', 'ccm', 'derp', 'ocm', 'oom-killer',
  'resolved', 'ssm-api', 'usbip-client', 'usbip-server',
]

// 走结构化表单的类型
export const STRUCTURED_SERVICE_TYPES = ['hysteria-realm']

function randSuffix() {
  return Math.random().toString(36).substring(2, 6)
}

function splitCsv(v) {
  if (Array.isArray(v)) return v
  return String(v || '').split(',').map(s => s.trim()).filter(Boolean)
}

// 表单模型默认值（新建时用）
export function serviceDefaultsForType(type, listenPort = 0) {
  if (type === 'hysteria-realm') {
    return {
      type,
      tag: 'realm-' + randSuffix(),
      listen: '0.0.0.0',
      listen_port: listenPort || 0,
      // TLS（realm 客户端用 https://server_url 连接，实际部署必须开）
      _tlsEnabled: true,
      _tlsCertificate: '',
      _tlsCertificatePath: '',
      _tlsKey: '',
      _tlsKeyPath: '',
      _tlsAlpnStr: 'h2',
      _tlsMinVersion: '',
      _tlsMaxVersion: '',
      // HTTP2Options
      idle_timeout: '',
      keep_alive_period: '',
      stream_receive_window: '',
      connection_receive_window: '',
      max_concurrent_streams: 0,
      // users: HysteriaRealmUser {name, token, max_realms}
      users: [{ name: 'realm1', token: generatePsk(), max_realms: 0 }],
      // ListenOptions 其余字段
      bind_interface: '',
      routing_mark: 0,
      reuse_addr: false,
      netns: '',
      disable_tcp_keep_alive: false,
      tcp_keep_alive: '',
      tcp_keep_alive_interval: '',
      tcp_fast_open: false,
      tcp_multi_path: false,
      udp_fragment: false,
      detour: '',
    }
  }
  return { type, tag: type ? type + '-' + randSuffix() : '' }
}

// 表单模型 -> sing-box 配置
export function buildService(svc) {
  const type = svc.type
  const out = { type }
  if (svc.tag) out.tag = svc.tag

  if (type === 'hysteria-realm') {
    if (svc.listen) out.listen = svc.listen
    if (svc.listen_port) out.listen_port = svc.listen_port
    // ListenOptions 其余字段
    if (svc.bind_interface) out.bind_interface = svc.bind_interface
    if (svc.routing_mark > 0) out.routing_mark = svc.routing_mark
    if (svc.reuse_addr) out.reuse_addr = true
    if (svc.netns) out.netns = svc.netns
    if (svc.disable_tcp_keep_alive) out.disable_tcp_keep_alive = true
    if (svc.tcp_keep_alive) out.tcp_keep_alive = svc.tcp_keep_alive
    if (svc.tcp_keep_alive_interval) out.tcp_keep_alive_interval = svc.tcp_keep_alive_interval
    if (svc.tcp_fast_open) out.tcp_fast_open = true
    if (svc.tcp_multi_path) out.tcp_multi_path = true
    if (svc.udp_fragment) out.udp_fragment = true
    if (svc.detour) out.detour = svc.detour

    // TLS（InboundTLSOptions）
    if (svc._tlsEnabled) {
      const tls = { enabled: true }
      // 内联 PEM 是多行块，必须整体作为单条（按行拆会被核心判为无效证书）
      if (String(svc._tlsCertificate || '').trim()) tls.certificate = [svc._tlsCertificate.trim()]
      if (svc._tlsCertificatePath) tls.certificate_path = svc._tlsCertificatePath
      if (String(svc._tlsKey || '').trim()) tls.key = [svc._tlsKey.trim()]
      if (svc._tlsKeyPath) tls.key_path = svc._tlsKeyPath
      const alpn = splitCsv(svc._tlsAlpnStr)
      if (alpn.length) tls.alpn = alpn
      if (svc._tlsMinVersion) tls.min_version = svc._tlsMinVersion
      if (svc._tlsMaxVersion) tls.max_version = svc._tlsMaxVersion
      out.tls = tls
    }

    // HTTP2Options
    if (svc.idle_timeout) out.idle_timeout = svc.idle_timeout
    if (svc.keep_alive_period) out.keep_alive_period = svc.keep_alive_period
    if (svc.stream_receive_window) out.stream_receive_window = svc.stream_receive_window
    if (svc.connection_receive_window) out.connection_receive_window = svc.connection_receive_window
    if (svc.max_concurrent_streams > 0) out.max_concurrent_streams = svc.max_concurrent_streams

    // users（必填，name/token 不能为空，否则核心拒载）
    out.users = (svc.users || [])
      .filter(u => u && u.name && u.token)
      .map(u => {
        const user = { name: u.name, token: u.token }
        if (u.max_realms > 0) user.max_realms = u.max_realms
        return user
      })
    return out
  }

  // 其它类型：原样透传（表单侧以 JSON 编辑）
  const { type: _t, tag: _g, ...rest } = svc
  return { ...out, ...rest }
}

// sing-box 配置 -> 表单模型
export function parseService(cfg) {
  const type = cfg.type || ''
  const base = { ...cfg, type, tag: cfg.tag || '' }

  if (type !== 'hysteria-realm') {
    return base
  }

  const tls = cfg.tls || {}
  return {
    ...base,
    listen: cfg.listen || '0.0.0.0',
    listen_port: cfg.listen_port || 0,
    _tlsEnabled: tls.enabled === true,
    _tlsCertificate: Array.isArray(tls.certificate) ? tls.certificate.join('\n') : (tls.certificate || ''),
    _tlsCertificatePath: tls.certificate_path || '',
    _tlsKey: Array.isArray(tls.key) ? tls.key.join('\n') : (tls.key || ''),
    _tlsKeyPath: tls.key_path || '',
    _tlsAlpnStr: Array.isArray(tls.alpn) ? tls.alpn.join(', ') : '',
    _tlsMinVersion: tls.min_version || '',
    _tlsMaxVersion: tls.max_version || '',
    idle_timeout: cfg.idle_timeout || '',
    keep_alive_period: cfg.keep_alive_period || '',
    stream_receive_window: cfg.stream_receive_window || '',
    connection_receive_window: cfg.connection_receive_window || '',
    max_concurrent_streams: cfg.max_concurrent_streams || 0,
    users: (cfg.users || []).map(u => ({
      name: u.name || '',
      token: u.token || '',
      max_realms: u.max_realms || 0,
    })),
    bind_interface: cfg.bind_interface || '',
    routing_mark: cfg.routing_mark || 0,
    reuse_addr: cfg.reuse_addr || false,
    netns: cfg.netns || '',
    disable_tcp_keep_alive: cfg.disable_tcp_keep_alive || false,
    tcp_keep_alive: cfg.tcp_keep_alive || '',
    tcp_keep_alive_interval: cfg.tcp_keep_alive_interval || '',
    tcp_fast_open: cfg.tcp_fast_open || false,
    tcp_multi_path: cfg.tcp_multi_path || false,
    udp_fragment: cfg.udp_fragment || false,
    detour: cfg.detour || '',
  }
}
