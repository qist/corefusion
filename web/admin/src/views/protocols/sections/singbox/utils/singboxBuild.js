// sing-box inbound 通用 build/parse（表单 模型 <-> sing-box 配置）
// 表单里用 _ 前缀字段表达配置，build 时映射为标准 sing-box 字段。
import { supportsReality, TRANSPORT_PROTOCOLS, AUTH_USER_PROTOCOLS } from './singboxDefaults'

export function buildInbound(ib) {
  const out = { type: ib.type, tag: ib.tag }
  if (ib.listen) out.listen = ib.listen
  if (ib.listen_port) out.listen_port = ib.listen_port
  if (ib.domain_strategy) out.domain_strategy = ib.domain_strategy
  // 通用监听字段（sing-box Listen Fields）
  if (ib.tcp_fast_open) out.tcp_fast_open = true
  if (ib.tcp_multi_path) out.tcp_multi_path = true
  if (ib.udp_fragment) out.udp_fragment = true
  if (ib.udp_timeout) out.udp_timeout = ib.udp_timeout
  if (ib.detour) out.detour = ib.detour
  // sniff 相关：sing-box 1.13 起入站不再接受这些字段，但后端
  // coreconfig.MigrateSingboxLegacyFields 会据此生成等价的 route action
  // （action: sniff / hijack-dns），因此这里仍按 legacy 字段写出。
  if (ib.sniff) {
    out.sniff = true
    if (ib.sniff_override_destination) out.sniff_override_destination = true
  }
  // direct inbound：静态转发目标
  if (ib.type === 'direct') {
    if (ib.override_address) out.override_address = ib.override_address
    if (ib.override_port) out.override_port = ib.override_port
  }
  // NetworkList: sing-box 要求 ["tcp","udp"] 数组或单个 "tcp"/"udp" 字符串
  // 表单用逗号分隔字符串表达（如 "tcp,udp"），build 时转换为数组
  if (ib.network && (ib.type === 'shadowsocks' || ib.type === 'naive')) {
    const nets = ib.network.split(',').map(s => s.trim()).filter(Boolean)
    if (nets.length === 1) {
      out.network = nets[0]
    } else if (nets.length > 1) {
      out.network = nets
    }
  }

  // TUN
  if (ib.type === 'tun') {
    if (ib.interface_name) out.interface_name = ib.interface_name
    if (ib.stack) out.stack = ib.stack
    out.auto_route = !!ib.auto_route
    out.strict_route = !!ib.strict_route
    if (ib._tunNetns) out.netns = ib._tunNetns
    if (ib._tunMtu) out.mtu = ib._tunMtu
    // Listable fields: comma-separated string → array
    if (ib._tunAddress) out.address = ib._tunAddress.split(',').map(s => s.trim()).filter(Boolean)
    if (ib._tunDnsMode) out.dns_mode = ib._tunDnsMode
    if (ib._tunDnsAddress) out.dns_address = ib._tunDnsAddress.split(',').map(s => s.trim()).filter(Boolean)
    if (ib._tunIpRoute2Table) out.iproute2_table_index = ib._tunIpRoute2Table
    if (ib._tunIpRoute2Rule) out.iproute2_rule_index = ib._tunIpRoute2Rule
    if (ib._tunAutoRedirect) out.auto_redirect = true
    if (ib._tunAutoRedirectInputMark) out.auto_redirect_input_mark = ib._tunAutoRedirectInputMark
    if (ib._tunAutoRedirectOutputMark) out.auto_redirect_output_mark = ib._tunAutoRedirectOutputMark
    if (ib._tunAutoRedirectResetMark) out.auto_redirect_reset_mark = ib._tunAutoRedirectResetMark
    if (ib._tunAutoRedirectTproxyMark) out.auto_redirect_tproxy_mark = ib._tunAutoRedirectTproxyMark
    if (ib._tunAutoRedirectNFQueue) out.auto_redirect_nfqueue = ib._tunAutoRedirectNFQueue
    if (ib._tunAutoRedirectFallbackRule) out.auto_redirect_iproute2_fallback_rule_index = ib._tunAutoRedirectFallbackRule
    if (ib._tunExcludeMptcp) out.exclude_mptcp = true
    if (ib._tunLoopbackAddress) out.loopback_address = ib._tunLoopbackAddress.split(',').map(s => s.trim()).filter(Boolean)
    if (ib._tunRouteAddress) out.route_address = ib._tunRouteAddress.split(',').map(s => s.trim()).filter(Boolean)
    if (ib._tunRouteAddressSet) out.route_address_set = ib._tunRouteAddressSet.split(',').map(s => s.trim()).filter(Boolean)
    if (ib._tunRouteExcludeAddress) out.route_exclude_address = ib._tunRouteExcludeAddress.split(',').map(s => s.trim()).filter(Boolean)
    if (ib._tunRouteExcludeAddressSet) out.route_exclude_address_set = ib._tunRouteExcludeAddressSet.split(',').map(s => s.trim()).filter(Boolean)
    if (ib._tunIncludeInterface) out.include_interface = ib._tunIncludeInterface.split(',').map(s => s.trim()).filter(Boolean)
    if (ib._tunExcludeInterface) out.exclude_interface = ib._tunExcludeInterface.split(',').map(s => s.trim()).filter(Boolean)
    if (ib._tunIncludeUid) out.include_uid = ib._tunIncludeUid.split(',').map(s => parseInt(s.trim())).filter(n => !isNaN(n))
    if (ib._tunIncludeUidRange) out.include_uid_range = ib._tunIncludeUidRange.split(',').map(s => s.trim()).filter(Boolean)
    if (ib._tunExcludeUid) out.exclude_uid = ib._tunExcludeUid.split(',').map(s => parseInt(s.trim())).filter(n => !isNaN(n))
    if (ib._tunExcludeUidRange) out.exclude_uid_range = ib._tunExcludeUidRange.split(',').map(s => s.trim()).filter(Boolean)
    if (ib._tunIncludeAndroidUser) out.include_android_user = ib._tunIncludeAndroidUser.split(',').map(s => parseInt(s.trim())).filter(n => !isNaN(n))
    if (ib._tunIncludePackage) out.include_package = ib._tunIncludePackage.split('\n').map(s => s.trim()).filter(Boolean)
    if (ib._tunExcludePackage) out.exclude_package = ib._tunExcludePackage.split('\n').map(s => s.trim()).filter(Boolean)
    if (ib._tunIncludeMacAddress) out.include_mac_address = ib._tunIncludeMacAddress.split(',').map(s => s.trim()).filter(Boolean)
    if (ib._tunExcludeMacAddress) out.exclude_mac_address = ib._tunExcludeMacAddress.split(',').map(s => s.trim()).filter(Boolean)
    if (ib._tunUdpTimeout) out.udp_timeout = ib._tunUdpTimeout
    if (ib._tunUdpMapping) out.udp_mapping = ib._tunUdpMapping
    if (ib._tunUdpFiltering) out.udp_filtering = ib._tunUdpFiltering
    if (ib._tunUdpNatMax) out.udp_nat_max = ib._tunUdpNatMax
    // Platform HTTP Proxy
    if (ib._tunPlatformHttpProxyEnabled) {
      const hp = { enabled: true }
      if (ib._tunPlatformHttpProxyServer) hp.server = ib._tunPlatformHttpProxyServer
      if (ib._tunPlatformHttpProxyPort) hp.server_port = ib._tunPlatformHttpProxyPort
      if (ib._tunPlatformHttpProxyBypassDomain) hp.bypass_domain = ib._tunPlatformHttpProxyBypassDomain.split('\n').map(s => s.trim()).filter(Boolean)
      if (ib._tunPlatformHttpProxyMatchDomain) hp.match_domain = ib._tunPlatformHttpProxyMatchDomain.split('\n').map(s => s.trim()).filter(Boolean)
      out.platform = { http_proxy: hp }
    }
  }

  // TProxy
  // sing-box TProxyInboundOptions: Network, UDPMapping, UDPFiltering, UDPNATMax
  if (ib.type === 'tproxy') {
    if (ib._tproxyNetwork) {
      const nets = ib._tproxyNetwork.split(',').map(s => s.trim()).filter(Boolean)
      if (nets.length === 1) out.network = nets[0]
      else if (nets.length > 1) out.network = nets
    }
    if (ib._tproxyUdpMapping) out.udp_mapping = ib._tproxyUdpMapping
    if (ib._tproxyUdpFiltering) out.udp_filtering = ib._tproxyUdpFiltering
    if (ib._tproxyUdpNatMax) out.udp_nat_max = ib._tproxyUdpNatMax
  }

  // Cloudflared (Cloudflare Tunnel)
  if (ib.type === 'cloudflared') {
    if (ib.token) out.token = ib.token
    if (ib.ha_connections > 0) out.ha_connections = ib.ha_connections
    if (ib.protocol) out.protocol = ib.protocol
    if (ib.post_quantum) out.post_quantum = true
    if (ib.edge_ip_version) out.edge_ip_version = ib.edge_ip_version
    if (ib.datagram_version) out.datagram_version = ib.datagram_version
    if (ib.grace_period) out.grace_period = ib.grace_period
    if (ib.region) out.region = ib.region
    // ControlDialer
    if (ib._cfdControlDetour || ib._cfdControlBindInterface || ib._cfdControlRoutingMark) {
      const cd = {}
      if (ib._cfdControlDetour) cd.detour = ib._cfdControlDetour
      if (ib._cfdControlBindInterface) cd.bind_interface = ib._cfdControlBindInterface
      if (ib._cfdControlRoutingMark) cd.routing_mark = ib._cfdControlRoutingMark
      if (Object.keys(cd).length) out.control_dialer = cd
    }
    // TunnelDialer
    if (ib._cfdTunnelDetour || ib._cfdTunnelBindInterface || ib._cfdTunnelRoutingMark) {
      const td = {}
      if (ib._cfdTunnelDetour) td.detour = ib._cfdTunnelDetour
      if (ib._cfdTunnelBindInterface) td.bind_interface = ib._cfdTunnelBindInterface
      if (ib._cfdTunnelRoutingMark) td.routing_mark = ib._cfdTunnelRoutingMark
      if (Object.keys(td).length) out.tunnel_dialer = td
    }
  }

  // Users
  // 注意：naive/mixed/http/socks 使用 auth.User，只有 username+password，没有 name 字段
  // 其他协议（vless/vmess/trojan/hysteria2/tuic/ss 等）的 User 都有 name 字段
  if (ib.users?.length) {
    const isAuthUser = AUTH_USER_PROTOCOLS.includes(ib.type)
    out.users = ib.users.filter(u => u.name || u.uuid || u.username || u.password || u.userkey).map(u => {
      const user = {}
      if (!isAuthUser && u.name) user.name = u.name
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
    // obfs: sing-box 要求 {type, password} 结构体
    if (ib._hy2ObfsType) {
      out.obfs = { type: ib._hy2ObfsType }
      if (ib._hy2ObfsPassword) out.obfs.password = ib._hy2ObfsPassword
    }
    // masquerade: 可以是 URL 字符串或 JSON 对象
    if (ib.masquerade) {
      try { out.masquerade = JSON.parse(ib.masquerade) } catch { out.masquerade = ib.masquerade }
    }
    // bbr_profile
    if (ib._hy2BbrProfile) out.bbr_profile = ib._hy2BbrProfile
    // brutal_debug
    if (ib._hy2BrutalDebug) out.brutal_debug = true
    // realm (可选 P2P 代理)
    if (ib._hy2RealmEnabled) {
      const realm = {
        server_url: ib._hy2RealmServerUrl || '',
        realm_id: ib._hy2RealmId || '',
        stun_servers: ib._hy2RealmStunServers ? ib._hy2RealmStunServers.split(',').map(s => s.trim()).filter(Boolean) : [],
      }
      if (ib._hy2RealmToken) realm.token = ib._hy2RealmToken
      if (ib._hy2RealmIpVersion) realm.ip_version = ib._hy2RealmIpVersion
      // port_mapping
      if (ib._hy2RealmPortMappingEnabled) {
        const pm = { enabled: true }
        if (ib._hy2RealmPortMappingTimeout) pm.timeout = ib._hy2RealmPortMappingTimeout
        if (ib._hy2RealmPortMappingLifetime) pm.lifetime = ib._hy2RealmPortMappingLifetime
        realm.port_mapping = pm
      }
      // http_client (可以是 tag 字符串或对象)
      if (ib._hy2RealmHttpClientTag) realm.http_client = ib._hy2RealmHttpClientTag
      // stun_domain_resolver
      if (ib._hy2RealmStunDnsServer) {
        realm.stun_domain_resolver = { server: ib._hy2RealmStunDnsServer }
      }
      out.realm = realm
    }
    // QUIC options (QUICOptions = HTTP2Options + QUIC fields)
    if (ib._hy2InitialPacketSize) out.initial_packet_size = ib._hy2InitialPacketSize
    if (ib._hy2DisablePathMtu) out.disable_path_mtu_discovery = true
    // HTTP2 fields (embedded in QUICOptions)
    if (ib._hy2IdleTimeout) out.idle_timeout = ib._hy2IdleTimeout
    if (ib._hy2KeepAlivePeriod) out.keep_alive_period = ib._hy2KeepAlivePeriod
    if (ib._hy2StreamRecvWindow) out.stream_receive_window = ib._hy2StreamRecvWindow
    if (ib._hy2ConnRecvWindow) out.connection_receive_window = ib._hy2ConnRecvWindow
    if (ib._hy2MaxConcurrentStreams) out.max_concurrent_streams = ib._hy2MaxConcurrentStreams
  }
  // Hysteria (old)
  if (ib.type === 'hysteria') {
    if (ib.up) out.up = ib.up
    if (ib.down) out.down = ib.down
    if (ib.obfs) out.obfs = ib.obfs
  }

  // TUIC
  // sing-box TUICInboundOptions: congestion_control, auth_timeout, zero_rtt_handshake, heartbeat, QUICOptions
  if (ib.type === 'tuic') {
    if (ib.congestion_control) out.congestion_control = ib.congestion_control
    if (ib.auth_timeout) out.auth_timeout = ib.auth_timeout
    if (ib.zero_rtt_handshake) out.zero_rtt_handshake = true
    if (ib.heartbeat) out.heartbeat = ib.heartbeat
    // QUIC options (QUICOptions = HTTP2Options + QUIC fields)
    if (ib._tuicInitialPacketSize) out.initial_packet_size = ib._tuicInitialPacketSize
    if (ib._tuicDisablePathMtu) out.disable_path_mtu_discovery = true
    // HTTP2 fields (embedded in QUICOptions)
    if (ib._tuicIdleTimeout) out.idle_timeout = ib._tuicIdleTimeout
    if (ib._tuicKeepAlivePeriod) out.keep_alive_period = ib._tuicKeepAlivePeriod
    if (ib._tuicStreamRecvWindow) out.stream_receive_window = ib._tuicStreamRecvWindow
    if (ib._tuicConnRecvWindow) out.connection_receive_window = ib._tuicConnRecvWindow
    if (ib._tuicMaxConcurrentStreams) out.max_concurrent_streams = ib._tuicMaxConcurrentStreams
  }

  // ShadowTLS
  // sing-box ShadowTLSInboundOptions: version, password, users, handshake, handshake_for_server_name, strict_mode, wildcard_sni
  // 注意: 顶层 password 是 TLS 握手密码（普通字符串），users[].password 是用户认证密码（base64url）
  if (ib.type === 'shadowtls') {
    if (ib.version) out.version = ib.version
    if (ib.password) out.password = ib.password
    // handshake: {server, server_port, ...DialerOptions} (ShadowTLSHandshakeOptions = ServerOptions + DialerOptions)
    if (ib._shadowtlsHandshakeServer) {
      const hs = { server: ib._shadowtlsHandshakeServer }
      if (ib._shadowtlsHandshakePort) hs.server_port = ib._shadowtlsHandshakePort
      // Dial 字段
      if (ib._shadowtlsHandshakeDetour) hs.detour = ib._shadowtlsHandshakeDetour
      if (ib._shadowtlsHandshakeBindInterface) hs.bind_interface = ib._shadowtlsHandshakeBindInterface
      if (ib._shadowtlsHandshakeRoutingMark) hs.routing_mark = ib._shadowtlsHandshakeRoutingMark
      if (ib._shadowtlsHandshakeReuseAddr) hs.reuse_addr = true
      if (ib._shadowtlsHandshakeConnectTimeout) hs.connect_timeout = ib._shadowtlsHandshakeConnectTimeout
      if (ib._shadowtlsHandshakeTcpFastOpen) hs.tcp_fast_open = true
      if (ib._shadowtlsHandshakeTcpMultiPath) hs.tcp_multi_path = true
      if (ib._shadowtlsHandshakeDisableTcpKeepAlive) hs.disable_tcp_keep_alive = true
      if (ib._shadowtlsHandshakeDomainResolver) hs.domain_resolver = ib._shadowtlsHandshakeDomainResolver
      out.handshake = hs
    }
    // handshake_for_server_name: [{sni, server, server_port, ...dial}] → map
    if (Array.isArray(ib._shadowtlsHsn) && ib._shadowtlsHsn.length) {
      const map = {}
      for (const e of ib._shadowtlsHsn) {
        if (!e.sni || !e.server) continue
        const hs = { server: e.server }
        if (e.server_port) hs.server_port = e.server_port
        if (e.detour) hs.detour = e.detour
        if (e.bind_interface) hs.bind_interface = e.bind_interface
        if (e.connect_timeout) hs.connect_timeout = e.connect_timeout
        if (e.tcp_fast_open) hs.tcp_fast_open = true
        if (e.domain_resolver) hs.domain_resolver = e.domain_resolver
        map[e.sni] = hs
      }
      if (Object.keys(map).length) out.handshake_for_server_name = map
    }
    // strict_mode
    if (ib._shadowtlsStrictMode) out.strict_mode = true
    // wildcard_sni: "off" | "authed" | "all"（空串不输出）
    if (ib._shadowtlsWildcardSni && ib._shadowtlsWildcardSni !== 'off') out.wildcard_sni = ib._shadowtlsWildcardSni
  }
  // Snell
  // sing-box SnellInboundOptions (discriminated union by version):
  //   v5: SnellObfsServerOptions { obfs_mode: none|http|tls }
  //   v6: SnellV6Options { mode: default|unshaped|unsafe-raw }
  //   Abstract: { ListenOptions, PSK, Users[] }
  if (ib.type === 'snell') {
    if (ib.version) out.version = ib.version
    if (ib.psk) out.psk = ib.psk
    // v5: obfs_mode (source field is obfs_mode, not obfs)
    if (ib.obfs) out.obfs_mode = ib.obfs
    if (ib.obfs_host) out.obfs_host = ib.obfs_host
    // v6: mode
    if (ib._snellMode) out.mode = ib._snellMode
  }
  // AnyTLS
  if (ib.type === 'anytls') { /* password in users */ }

  // Naive
  if (ib.type === 'naive') {
    if (ib.quic_congestion_control) out.quic_congestion_control = ib.quic_congestion_control
  }

  // TLS（Reality 仅支持 vless / vmess / trojan + tcp/http/ws/httpupgrade/grpc）
  const realityOk = supportsReality(ib.type, ib._transportType)
  if (ib._tlsEnabled) {
    const tls = { enabled: true }
    if (ib._tlsServerName) tls.server_name = ib._tlsServerName
    if (ib._tlsAlpn?.length) tls.alpn = ib._tlsAlpn
    if (ib._tlsMinVersion) tls.min_version = ib._tlsMinVersion
    // Reality 与标准证书模式互斥：启用 Reality 时不输出证书字段
    if (ib._realityEnabled && realityOk) {
      tls.reality = { enabled: true }
      if (ib._realityHandshakeServer) tls.reality.handshake = { server: ib._realityHandshakeServer, server_port: ib._realityHandshakePort }
      if (ib._realityPrivateKey) tls.reality.private_key = ib._realityPrivateKey
      // 入站(服务端) reality 不含 public_key（仅给客户端/分享链接使用）
      const sids = ib._realityShortIds ? ib._realityShortIds.split('\n').map(s => s.trim()).filter(Boolean) : []
      if (sids.length) tls.reality.short_id = sids
      if (ib._realityMaxTimeDiff) tls.reality.max_time_difference = ib._realityMaxTimeDiff
    } else {
      // 标准 TLS（含协议/传输不支持 Reality 时的回退）
      // 注意：utls 是出站(客户端)TLS 字段，入站(服务端)不支持，不输出
      if (ib._tlsCertPath) tls.certificate_path = ib._tlsCertPath
      if (ib._tlsKeyPath) tls.key_path = ib._tlsKeyPath
    }
    out.tls = tls
  }
  // Reality 公钥（pbk）：sing-box 入站不接受 public_key 字段（DisallowUnknownFields 会 FATAL），
  // 所以存成顶层 _realityPublicKey —— 顶层 `_` 字段生成核心配置时会被剥离，不会进 sing-box。
  // 「公钥（客户端）」只读框与后端 sharelink/singbox_share.go 都从这里取；
  // 缺失时两端都会由 private_key 推导（x25519）。⚠ 以前这行被误写在 parseInbound 里，从未落库，
  // 且会把 parse 从 tls.reality.public_key 读到的旧值覆盖成空串。
  if (out.tls && out.tls.reality && out.tls.reality.enabled) {
    out._realityPublicKey = ib._realityPublicKey || ''
  }

  // Transport（仅 vless/vmess/trojan 支持 V2Ray Transport）
  // 每种 transport 只输出该类型支持的合法字段，参考 sing-box 文档：
  //   http: host, path, method, headers
  //   ws: path, headers, max_early_data, early_data_header_name
  //   quic: (无额外字段)
  //   grpc: service_name, idle_timeout, permit_without_stream, ping_timeout
  //   httpupgrade: host, path, headers
  if (ib._transportType && TRANSPORT_PROTOCOLS.includes(ib.type)) {
    const tp = { type: ib._transportType }
    switch (ib._transportType) {
      case 'http':
        if (ib._transportHost) {
          tp.host = ib._transportHost.split(',').map(s => s.trim()).filter(Boolean)
          if (!tp.host.length) delete tp.host
        }
        if (ib._transportPath) tp.path = ib._transportPath
        if (ib._transportMethod) tp.method = ib._transportMethod
        if (Array.isArray(ib._transportHeaders) && ib._transportHeaders.length) {
          const hdrs = {}
          for (const h of ib._transportHeaders) { if (h.key) hdrs[h.key] = h.value || '' }
          if (Object.keys(hdrs).length) tp.headers = hdrs
        }
        break
      case 'ws':
        if (ib._transportPath) tp.path = ib._transportPath
        if (Array.isArray(ib._transportHeaders) && ib._transportHeaders.length) {
          const hdrs = {}
          for (const h of ib._transportHeaders) { if (h.key) hdrs[h.key] = h.value || '' }
          if (Object.keys(hdrs).length) tp.headers = hdrs
        }
        if (ib._transportMaxEarlyData) tp.max_early_data = ib._transportMaxEarlyData
        if (ib._transportEarlyDataHeader) tp.early_data_header_name = ib._transportEarlyDataHeader
        break
      case 'quic':
        // quic transport has no additional fields
        break
      case 'grpc':
        if (ib._transportServiceName) tp.service_name = ib._transportServiceName
        if (ib._transportIdleTimeout) tp.idle_timeout = ib._transportIdleTimeout
        if (ib._transportPingTimeout) tp.ping_timeout = ib._transportPingTimeout
        if (ib._transportPermitWithoutStream) tp.permit_without_stream = true
        break
      case 'httpupgrade':
        if (ib._transportHost) tp.host = ib._transportHost
        if (ib._transportPath) tp.path = ib._transportPath
        if (Array.isArray(ib._transportHeaders) && ib._transportHeaders.length) {
          const hdrs = {}
          for (const h of ib._transportHeaders) { if (h.key) hdrs[h.key] = h.value || '' }
          if (Object.keys(hdrs).length) tp.headers = hdrs
        }
        break
    }
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

  // ── 面板内部字段（顶层 _ 前缀，生成核心配置时由后端 CleanInboundFields 剥离）──
  // 访问域名：必须写成与 xray 同款的 _accessDomain **对象** —— 订阅渲染与分享链接
  // （resolveHostPort / ensureAccessDomain）读的都是 config["_accessDomain"]；只写扁平
  // 字段会被后端当内部字段剔除，表现为「订阅/二维码里没有域名，回退成 127.0.0.1」。
  // ⚠ 这段以前被误写进了 parseInbound（读取方向），导致 sing-box 的访问域名从未落库。
  out._accessDomainEnabled = ib._accessDomainEnabled || false
  out._accessDomainType = ib._accessDomainType || 'direct'
  out._directDomain = ib._directDomain || ''
  out._directPortMode = ib._directPortMode || 'config'
  out._cfDomain = ib._cfDomain || ''
  out._cfIP = ib._cfIP || ''
  out._customDomain = ib._customDomain || ''
  out._customPortMode = ib._customPortMode || 'custom'
  out._accessCustomPort = ib._accessCustomPort || 443
  out._accessDomain = {
    enabled: out._accessDomainEnabled,
    type: out._accessDomainType,
    directDomain: out._directDomain,
    directPortMode: out._directPortMode,
    cfDomain: out._cfDomain,
    cfIP: out._cfIP,
    customDomain: out._customDomain,
    customPortMode: out._customPortMode,
    customPort: out._accessCustomPort,
  }
  // 面板内部字段（顶层 `_`，生成核心配置时由 CleanInboundFields 剥离）：**必须落库**，
  // 否则重开编辑器就不知道用户当初选了什么（只在生成时过滤，不在保存时丢弃）。
  // _utlsFingerprint：客户端 uTLS 指纹。sing-box 入站不支持 tls.utls（服务端会报未知字段），
  //   所以只能存内部字段 —— 分享链接的 fp 取它，缺失时后端默认 chrome。
  // _certId：TLS 证书选择（界面回显；证书文件路径已在 tls.certificate_path/key_path）。
  out._utlsFingerprint = ib._utlsFingerprint || ''
  out._certId = ib._certId || ''
  // 端口跳跃：同样以对象形态落库 —— 面板启动时按它恢复 DNAT 规则
  // （firewall.ParsePortHopFromConfig），分享链接据此下发 mport，编辑器回显也靠它。
  out._portHopEnabled = ib._portHopEnabled || false
  out._portHopMode = ib._portHopMode || 'preset'
  out._portHopRange = ib._portHopRange || '10000-19999'
  out._portHopStart = ib._portHopStart || 10000
  out._portHopEnd = ib._portHopEnd || 19999
  out._portHopInterface = ib._portHopInterface || ''
  out._portHop = {
    enabled: out._portHopEnabled,
    mode: out._portHopMode,
    range: out._portHopRange,
    start: out._portHopStart,
    end: out._portHopEnd,
    interface: out._portHopInterface,
  }
  return out
}

export function parseInbound(ib) {
  const o = { ...ib }
  if (!o.users) o.users = []
  // NetworkList: sing-box 存储为 ["tcp","udp"] 数组或 "tcp" 字符串，表单用逗号分隔字符串
  if (Array.isArray(ib.network)) {
    o.network = ib.network.join(',')
  } else {
    o.network = ib.network || ''
  }
  o._tlsEnabled = ib.tls?.enabled || false
  o._tlsServerName = ib.tls?.server_name || ''
  o._tlsAlpn = ib.tls?.alpn || []
  o._tlsMinVersion = ib.tls?.min_version || ''
  o._tlsCertPath = ib.tls?.certificate_path || ''
  o._tlsKeyPath = ib.tls?.key_path || ''
  // 客户端 uTLS 指纹：字段存在于配置就用它（允许为空 = 不下发 fp），
  // 老数据没有该字段时回退 tls.utls.fingerprint，仍没有则默认 chrome（与 xray 表单默认一致）。
  o._utlsFingerprint = ib._utlsFingerprint !== undefined
    ? (ib._utlsFingerprint || '')
    : (ib.tls?.utls?.fingerprint || 'chrome')
  o._certId = ib._certId || ''
  o._realityEnabled = ib.tls?.reality?.enabled || false
  o._realityHandshakeServer = ib.tls?.reality?.handshake?.server || ''
  o._realityHandshakePort = ib.tls?.reality?.handshake?.server_port || 443
  o._realityPrivateKey = ib.tls?.reality?.private_key || ''
  // 优先顶层 _realityPublicKey（编辑器保存的），兼容历史写在 tls.reality.public_key 的数据
  o._realityPublicKey = ib._realityPublicKey || ib.tls?.reality?.public_key || ''
  o._realityShortIds = (ib.tls?.reality?.short_id || []).join('\n')
  o._realityMaxTimeDiff = ib.tls?.reality?.max_time_difference || ''
  o._transportType = ib.transport?.type || ''
  o._transportPath = ib.transport?.path || ''
  // HTTP transport: host is array[] → comma-separated string; httpupgrade: host is string
  const rawHost = ib.transport?.host
  if (Array.isArray(rawHost)) {
    o._transportHost = rawHost.join(',')
  } else {
    o._transportHost = rawHost || ''
  }
  o._transportServiceName = ib.transport?.service_name || ''
  o._transportMaxEarlyData = ib.transport?.max_early_data || 0
  o._transportEarlyDataHeader = ib.transport?.early_data_header_name || ''
  o._transportIdleTimeout = ib.transport?.idle_timeout || ''
  o._transportPingTimeout = ib.transport?.ping_timeout || ''
  o._transportPermitWithoutStream = ib.transport?.permit_without_stream || false
  o._transportMethod = ib.transport?.method || ''
  // headers: {key:value} → [{key,value}]
  if (ib.transport?.headers && typeof ib.transport.headers === 'object') {
    o._transportHeaders = Object.entries(ib.transport.headers).map(([k, v]) => ({ key: k, value: v ? String(v) : '' }))
  } else {
    o._transportHeaders = []
  }
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
o.obfs = ib.obfs_mode || ''
o.obfs_host = ib.obfs_host || ''
o._snellMode = ib.mode || ''
  // Hysteria2 obfs: sing-box stores as {type, password} struct → recover to _hy2 fields
  if (ib.type === 'hysteria2' && ib.obfs && typeof ib.obfs === 'object') {
    o._hy2ObfsType = ib.obfs.type || ''
    o._hy2ObfsPassword = ib.obfs.password || ''
    // Clear the raw obfs so it doesn't get re-output as string
    o.obfs = ''
  }
  // Hysteria2 bbr_profile + QUIC options
  o._hy2BbrProfile = ib.bbr_profile || ''
  o._hy2BrutalDebug = ib.brutal_debug || false
  // Hysteria2 realm
  if (ib.realm && typeof ib.realm === 'object') {
    o._hy2RealmEnabled = true
    o._hy2RealmServerUrl = ib.realm.server_url || ''
    o._hy2RealmToken = ib.realm.token || ''
    o._hy2RealmId = ib.realm.realm_id || ''
    o._hy2RealmStunServers = Array.isArray(ib.realm.stun_servers) ? ib.realm.stun_servers.join(',') : ''
    o._hy2RealmIpVersion = ib.realm.ip_version || 0
    if (ib.realm.port_mapping) {
      o._hy2RealmPortMappingEnabled = ib.realm.port_mapping.enabled || false
      o._hy2RealmPortMappingTimeout = ib.realm.port_mapping.timeout || ''
      o._hy2RealmPortMappingLifetime = ib.realm.port_mapping.lifetime || ''
    }
    if (typeof ib.realm.http_client === 'string') {
      o._hy2RealmHttpClientTag = ib.realm.http_client
    }
    if (ib.realm.stun_domain_resolver) {
      o._hy2RealmStunDnsServer = ib.realm.stun_domain_resolver.server || ''
    }
  }
  o._hy2InitialPacketSize = ib.initial_packet_size || 0
  o._hy2DisablePathMtu = ib.disable_path_mtu_discovery || false
  o._hy2IdleTimeout = ib.idle_timeout || ''
  o._hy2KeepAlivePeriod = ib.keep_alive_period || ''
  o._hy2StreamRecvWindow = ib.stream_receive_window || ''
  o._hy2ConnRecvWindow = ib.connection_receive_window || ''
  o._hy2MaxConcurrentStreams = ib.max_concurrent_streams || 0
  // naive
  o.quic_congestion_control = ib.quic_congestion_control || ''
  // tuic (TUICInboundOptions + QUICOptions)
  o.zero_rtt_handshake = ib.zero_rtt_handshake || false
  o.heartbeat = ib.heartbeat || ''
  o._tuicInitialPacketSize = ib.initial_packet_size || 0
  o._tuicDisablePathMtu = ib.disable_path_mtu_discovery || false
  o._tuicIdleTimeout = ib.idle_timeout || ''
  o._tuicKeepAlivePeriod = ib.keep_alive_period || ''
  o._tuicStreamRecvWindow = ib.stream_receive_window || ''
  o._tuicConnRecvWindow = ib.connection_receive_window || ''
  o._tuicMaxConcurrentStreams = ib.max_concurrent_streams || 0
  // shadowtls
  o._shadowtlsHandshakeServer = ib.handshake?.server || ''
  o._shadowtlsHandshakePort = ib.handshake?.server_port || 443
  o._shadowtlsHandshakeDetour = ib.handshake?.detour || ''
  o._shadowtlsHandshakeBindInterface = ib.handshake?.bind_interface || ''
  o._shadowtlsHandshakeRoutingMark = ib.handshake?.routing_mark || 0
  o._shadowtlsHandshakeReuseAddr = ib.handshake?.reuse_addr || false
  o._shadowtlsHandshakeConnectTimeout = ib.handshake?.connect_timeout || ''
  o._shadowtlsHandshakeTcpFastOpen = ib.handshake?.tcp_fast_open || false
  o._shadowtlsHandshakeTcpMultiPath = ib.handshake?.tcp_multi_path || false
  o._shadowtlsHandshakeDisableTcpKeepAlive = ib.handshake?.disable_tcp_keep_alive || false
  o._shadowtlsHandshakeDomainResolver = ib.handshake?.domain_resolver || ''
  // handshake_for_server_name: map → 数组形式
  if (ib.handshake_for_server_name && typeof ib.handshake_for_server_name === 'object') {
    o._shadowtlsHsn = Object.entries(ib.handshake_for_server_name).map(([sni, hs]) => ({
      sni,
      server: hs?.server || '',
      server_port: hs?.server_port || 443,
      detour: hs?.detour || '',
      bind_interface: hs?.bind_interface || '',
      connect_timeout: hs?.connect_timeout || '',
      tcp_fast_open: hs?.tcp_fast_open || false,
      domain_resolver: hs?.domain_resolver || '',
    }))
  } else {
    o._shadowtlsHsn = []
  }
  o._shadowtlsStrictMode = ib.strict_mode || false
  o._shadowtlsWildcardSni = ib.wildcard_sni || ''
  if (typeof ib.masquerade === 'string') o.masquerade = ib.masquerade
  else if (ib.masquerade) o.masquerade = JSON.stringify(ib.masquerade)
  else o.masquerade = ''
  // remark + enabled + access domain (对齐 xray，表单专用字段)
  o._remark = ib._remark || ''
  o._enabled = ib._enabled !== false
  // tun parse
  o._tunNetns = ib.netns || ''
  o._tunMtu = ib.mtu || 0
  o._tunAddress = Array.isArray(ib.address) ? ib.address.join(', ') : (ib.address || '')
  o._tunDnsMode = ib.dns_mode || ''
  o._tunDnsAddress = Array.isArray(ib.dns_address) ? ib.dns_address.join(', ') : (ib.dns_address || '')
  o._tunIpRoute2Table = ib.iproute2_table_index || 0
  o._tunIpRoute2Rule = ib.iproute2_rule_index || 0
  o._tunAutoRedirect = ib.auto_redirect || false
  o._tunAutoRedirectInputMark = ib.auto_redirect_input_mark || ''
  o._tunAutoRedirectOutputMark = ib.auto_redirect_output_mark || ''
  o._tunAutoRedirectResetMark = ib.auto_redirect_reset_mark || ''
  o._tunAutoRedirectTproxyMark = ib.auto_redirect_tproxy_mark || ''
  o._tunAutoRedirectNFQueue = ib.auto_redirect_nfqueue || 0
  o._tunAutoRedirectFallbackRule = ib.auto_redirect_iproute2_fallback_rule_index || 0
  o._tunExcludeMptcp = ib.exclude_mptcp || false
  o._tunLoopbackAddress = Array.isArray(ib.loopback_address) ? ib.loopback_address.join(', ') : (ib.loopback_address || '')
  o._tunRouteAddress = Array.isArray(ib.route_address) ? ib.route_address.join(', ') : (ib.route_address || '')
  o._tunRouteAddressSet = Array.isArray(ib.route_address_set) ? ib.route_address_set.join(', ') : (ib.route_address_set || '')
  o._tunRouteExcludeAddress = Array.isArray(ib.route_exclude_address) ? ib.route_exclude_address.join(', ') : (ib.route_exclude_address || '')
  o._tunRouteExcludeAddressSet = Array.isArray(ib.route_exclude_address_set) ? ib.route_exclude_address_set.join(', ') : (ib.route_exclude_address_set || '')
  o._tunIncludeInterface = Array.isArray(ib.include_interface) ? ib.include_interface.join(', ') : (ib.include_interface || '')
  o._tunExcludeInterface = Array.isArray(ib.exclude_interface) ? ib.exclude_interface.join(', ') : (ib.exclude_interface || '')
  o._tunIncludeUid = Array.isArray(ib.include_uid) ? ib.include_uid.join(', ') : (ib.include_uid || '')
  o._tunIncludeUidRange = Array.isArray(ib.include_uid_range) ? ib.include_uid_range.join(', ') : (ib.include_uid_range || '')
  o._tunExcludeUid = Array.isArray(ib.exclude_uid) ? ib.exclude_uid.join(', ') : (ib.exclude_uid || '')
  o._tunExcludeUidRange = Array.isArray(ib.exclude_uid_range) ? ib.exclude_uid_range.join(', ') : (ib.exclude_uid_range || '')
  o._tunIncludeAndroidUser = Array.isArray(ib.include_android_user) ? ib.include_android_user.join(', ') : (ib.include_android_user || '')
  o._tunIncludePackage = Array.isArray(ib.include_package) ? ib.include_package.join('\n') : (ib.include_package || '')
  o._tunExcludePackage = Array.isArray(ib.exclude_package) ? ib.exclude_package.join('\n') : (ib.exclude_package || '')
  o._tunIncludeMacAddress = Array.isArray(ib.include_mac_address) ? ib.include_mac_address.join(', ') : (ib.include_mac_address || '')
  o._tunExcludeMacAddress = Array.isArray(ib.exclude_mac_address) ? ib.exclude_mac_address.join(', ') : (ib.exclude_mac_address || '')
  o._tunUdpTimeout = ib.udp_timeout || ''
  o._tunUdpMapping = ib.udp_mapping || ''
  o._tunUdpFiltering = ib.udp_filtering || ''
  o._tunUdpNatMax = ib.udp_nat_max || 0
  // platform http_proxy
  const hp = ib.platform?.http_proxy
  o._tunPlatformHttpProxyEnabled = hp?.enabled || false
  o._tunPlatformHttpProxyServer = hp?.server || ''
  o._tunPlatformHttpProxyPort = hp?.server_port || 0
  o._tunPlatformHttpProxyBypassDomain = Array.isArray(hp?.bypass_domain) ? hp.bypass_domain.join('\n') : (hp?.bypass_domain || '')
  o._tunPlatformHttpProxyMatchDomain = Array.isArray(hp?.match_domain) ? hp.match_domain.join('\n') : (hp?.match_domain || '')
  // tproxy parse
  o._tproxyNetwork = Array.isArray(ib.network) ? ib.network.join(',') : (ib.network || '')
  o._tproxyUdpMapping = ib.udp_mapping || ''
  o._tproxyUdpFiltering = ib.udp_filtering || ''
  o._tproxyUdpNatMax = ib.udp_nat_max || 0
  // cloudflared parse
  if (ib.type === 'cloudflared') {
    o.token = ib.token || ''
    o.ha_connections = ib.ha_connections || 0
    o.protocol = ib.protocol || ''
    o.post_quantum = ib.post_quantum || false
    o.edge_ip_version = ib.edge_ip_version || 0
    o._edgeIpVersion = String(ib.edge_ip_version || 0)
    o.datagram_version = ib.datagram_version || ''
    o.grace_period = ib.grace_period || ''
    o.region = ib.region || ''
    o._cfdControlDetour = ib.control_dialer?.detour || ''
    o._cfdControlBindInterface = ib.control_dialer?.bind_interface || ''
    o._cfdControlRoutingMark = ib.control_dialer?.routing_mark || 0
    o._cfdTunnelDetour = ib.tunnel_dialer?.detour || ''
    o._cfdTunnelBindInterface = ib.tunnel_dialer?.bind_interface || ''
    o._cfdTunnelRoutingMark = ib.tunnel_dialer?.routing_mark || 0
  }
  o._remark = ib._remark || ''
  // 访问域名：入库形态是 _accessDomain 对象（与 xray 一致），扁平字段仅作旧数据兜底。
  // 以前这里方向写反了（在 parse 里构造对象），所以访问域名从来没被写进数据库。
  {
    const ad = ib._accessDomain || {}
    const pick = (objKey, flatKey, def) => {
      if (ad[objKey] !== undefined && ad[objKey] !== null) return ad[objKey]
      if (ib[flatKey] !== undefined && ib[flatKey] !== null) return ib[flatKey]
      return def
    }
    o._accessDomainEnabled = pick('enabled', '_accessDomainEnabled', false)
    o._accessDomainType = pick('type', '_accessDomainType', 'direct')
    o._directDomain = pick('directDomain', '_directDomain', '')
    o._directPortMode = pick('directPortMode', '_directPortMode', 'config')
    o._cfDomain = pick('cfDomain', '_cfDomain', '')
    o._cfIP = pick('cfIP', '_cfIP', '')
    o._customDomain = pick('customDomain', '_customDomain', '')
    o._customPortMode = pick('customPortMode', '_customPortMode', 'custom')
    o._accessCustomPort = pick('customPort', '_accessCustomPort', 443)
  }
  // 端口跳跃：同样是对象形态（旧数据可能只有扁平字段）
  {
    const ph = ib._portHop || {}
    const pick = (objKey, flatKey, def) => {
      if (ph[objKey] !== undefined && ph[objKey] !== null) return ph[objKey]
      if (ib[flatKey] !== undefined && ib[flatKey] !== null) return ib[flatKey]
      return def
    }
    o._portHopEnabled = pick('enabled', '_portHopEnabled', false)
    o._portHopMode = pick('mode', '_portHopMode', 'preset')
    o._portHopRange = pick('range', '_portHopRange', '10000-19999')
    o._portHopStart = pick('start', '_portHopStart', 10000)
    o._portHopEnd = pick('end', '_portHopEnd', 19999)
    o._portHopInterface = pick('interface', '_portHopInterface', '')
  }
  return o
}