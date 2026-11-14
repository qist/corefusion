// sing-box 端点(Endpoint)通用 build/parse（表单模型 <-> sing-box 配置）
// 端点是 sing-box 1.13+ 的顶层概念，与 inbounds/outbounds 并列
// 支持的类型：wireguard, tailscale, openconnect, openvpn-client, openvpn-server

// 支持的端点类型列表
// masque-client / masque-server 是 sing-box 1.13+ 的 MASQUE（RFC 9484 CONNECT-IP）端点：
// 客户端 endpoint 当出站用，服务端 endpoint 当入站用（挂到 route 即可）。
export const ENDPOINT_TYPES = [
  'wireguard', 'tailscale', 'openconnect', 'openvpn-client', 'openvpn-server',
  'masque-client', 'masque-server',
]

// 端点类型显示名
export const ENDPOINT_TYPE_LABELS = {
  wireguard: 'WireGuard',
  tailscale: 'Tailscale',
  openconnect: 'OpenConnect',
  'openvpn-client': 'OpenVPN Client',
  'openvpn-server': 'OpenVPN Server',
  'masque-client': 'MASQUE Client',
  'masque-server': 'MASQUE Server',
}

// 逗号/换行分隔的字符串 → 数组（端点表单的列表字段统一走它）
function splitCSV(v) {
  if (Array.isArray(v)) return v
  return String(v || '').split(/[,\n]/).map(s => s.trim()).filter(Boolean)
}

// 表单的 [{key,value}] → 配置的 {k:v}（headers 类型）
function headersFromList(list) {
  const out = {}
  if (!Array.isArray(list)) return out
  for (const h of list) {
    if (h && h.key) out[h.key] = h.value || ''
  }
  return out
}

// OpenVPN control_wrap（tls_auth / tls_crypt / tls_crypt_v2）
function buildControlWrap(ep) {
  if (!ep.control_wrap_type) return null
  const cw = { type: ep.control_wrap_type }
  if (ep.control_wrap_key) cw.key = splitCSV(ep.control_wrap_key)
  if (ep.control_wrap_key_path) cw.key_path = ep.control_wrap_key_path
  if (ep.control_wrap_direction) cw.direction = ep.control_wrap_direction
  // force_cookie 只有入站（服务端）的 control_wrap 才有
  if (ep.type === 'openvpn-server' && ep.control_wrap_force_cookie) cw.force_cookie = true
  return cw
}

// HTTP/2 + QUIC 选项（sing-box 把 http2/http3 变体字段平铺进同一对象）：
// version 1 没有额外字段；2 用 HTTP2Options；3（含 0=默认 3）用 QUICOptions。
function applyHTTPVersionOptions(out, ep, versions) {
  const list = versions || [ep.version > 0 ? ep.version : 3]
  if (!list.includes(2) && !list.includes(3)) return
  if (ep.idle_timeout) out.idle_timeout = ep.idle_timeout
  if (ep.keep_alive_period) out.keep_alive_period = ep.keep_alive_period
  if (ep.stream_receive_window) out.stream_receive_window = ep.stream_receive_window
  if (ep.connection_receive_window) out.connection_receive_window = ep.connection_receive_window
  if (ep.max_concurrent_streams > 0) out.max_concurrent_streams = ep.max_concurrent_streams
  if (list.includes(3)) {
    if (ep.initial_packet_size > 0) out.initial_packet_size = ep.initial_packet_size
    if (ep.disable_path_mtu_discovery) out.disable_path_mtu_discovery = true
  }
}

// MASQUE 端点共用的 UDP NAT 字段（MASQUEEndpointOptions）
function applyMasqueEndpointOptions(out, ep) {
  if (ep.system) out.system = true
  if (ep.name) out.name = ep.name
  if (ep.mtu > 0) out.mtu = ep.mtu
  if (ep.udp_mapping) out.udp_mapping = ep.udp_mapping
  if (ep.udp_filtering) out.udp_filtering = ep.udp_filtering
  if (ep.udp_nat_max > 0) out.udp_nat_max = ep.udp_nat_max
}

// 客户端 endpoint 的拨号字段（DialerOptions）
function applyDialOptions(out, ep) {
  if (ep.detour) out.detour = ep.detour
  if (ep.bind_interface) out.bind_interface = ep.bind_interface
  if (ep.inet4_bind_address) out.inet4_bind_address = ep.inet4_bind_address
  if (ep.inet6_bind_address) out.inet6_bind_address = ep.inet6_bind_address
  if (ep.bind_address_no_port) out.bind_address_no_port = true
  if (ep.protect_path) out.protect_path = ep.protect_path
  if (ep.routing_mark > 0) out.routing_mark = ep.routing_mark
  if (ep.reuse_addr) out.reuse_addr = true
  if (ep.netns) out.netns = ep.netns
  if (ep.connect_timeout) out.connect_timeout = ep.connect_timeout
  if (ep.tcp_fast_open) out.tcp_fast_open = true
  if (ep.tcp_multi_path) out.tcp_multi_path = true
  if (ep.disable_tcp_keep_alive) out.disable_tcp_keep_alive = true
  if (ep.tcp_keep_alive) out.tcp_keep_alive = ep.tcp_keep_alive
  if (ep.tcp_keep_alive_interval) out.tcp_keep_alive_interval = ep.tcp_keep_alive_interval
  if (ep.udp_fragment) out.udp_fragment = true
  if (ep.domain_resolver) out.domain_resolver = ep.domain_resolver
  if (ep.network_strategy) out.network_strategy = ep.network_strategy
  const netTypes = splitCSV(ep._networkType)
  if (netTypes.length) out.network_type = netTypes
  const fallbackTypes = splitCSV(ep._fallbackNetworkType)
  if (fallbackTypes.length) out.fallback_network_type = fallbackTypes
  if (ep.fallback_delay) out.fallback_delay = ep.fallback_delay
}

// 服务端 endpoint 的监听字段（ListenOptions）
function applyListenOptions(out, ep) {
  if (ep.listen) out.listen = ep.listen
  if (ep.listen_port) out.listen_port = ep.listen_port
  if (ep.bind_interface) out.bind_interface = ep.bind_interface
  if (ep.routing_mark > 0) out.routing_mark = ep.routing_mark
  if (ep.reuse_addr) out.reuse_addr = true
  if (ep.netns) out.netns = ep.netns
  if (ep.tcp_fast_open) out.tcp_fast_open = true
  if (ep.tcp_multi_path) out.tcp_multi_path = true
  if (ep.disable_tcp_keep_alive) out.disable_tcp_keep_alive = true
  if (ep.tcp_keep_alive) out.tcp_keep_alive = ep.tcp_keep_alive
  if (ep.tcp_keep_alive_interval) out.tcp_keep_alive_interval = ep.tcp_keep_alive_interval
  if (ep.udp_fragment) out.udp_fragment = true
  if (ep.udp_timeout) out.udp_timeout = ep.udp_timeout
  if (ep.detour) out.detour = ep.detour
}

// 入站 TLS（服务端 endpoint）：只有开启了才写 tls 对象
function buildInboundTLS(ep) {
  if (ep.tls_enabled === false) return null
  const tls = { enabled: true }
  if (ep.tls_server_name) tls.server_name = ep.tls_server_name
  const alpn = splitCSV(ep._tlsAlpnStr)
  if (alpn.length) tls.alpn = alpn
  if (ep.tls_certificate) tls.certificate = [ep.tls_certificate.trim()].filter(Boolean)
  if (ep.tls_certificate_path) tls.certificate_path = ep.tls_certificate_path
  if (ep.tls_key) tls.key = [ep.tls_key.trim()].filter(Boolean)
  if (ep.tls_key_path) tls.key_path = ep.tls_key_path
  if (ep.tls_client_authentication) tls.client_authentication = ep.tls_client_authentication
  if (ep.tls_client_certificate) tls.client_certificate = [ep.tls_client_certificate.trim()].filter(Boolean)
  if (ep.tls_client_certificate_path) tls.client_certificate_path = [ep.tls_client_certificate_path.trim()].filter(Boolean)
  if (ep.tls_version_min && ep.tls_version_min !== '1.2') tls.min_version = ep.tls_version_min
  if (ep.tls_version_max) tls.max_version = ep.tls_version_max
  return tls
}

// 出站 TLS（客户端 endpoint）
function buildOutboundTLS(ep) {
  if (ep.tls_enabled === false) return null
  const tls = { enabled: true }
  if (ep.tls_server_name) tls.server_name = ep.tls_server_name
  if (ep.tls_insecure) tls.insecure = true
  if (ep.tls_disable_sni) tls.disable_sni = true
  const alpn = splitCSV(ep._tlsAlpnStr)
  if (alpn.length) tls.alpn = alpn
  // 证书公钥 pin（base64 SHA256）：WARP MASQUE 边缘是自签证书，缺了它握手必失败。
  if (ep.tls_certificate_public_key_sha256) tls.certificate_public_key_sha256 = splitCSV(ep.tls_certificate_public_key_sha256)
  if (ep.tls_certificate) tls.certificate = [ep.tls_certificate.trim()].filter(Boolean)
  if (ep.tls_certificate_path) tls.certificate_path = ep.tls_certificate_path
  if (ep.tls_client_certificate) tls.client_certificate = [ep.tls_client_certificate.trim()].filter(Boolean)
  if (ep.tls_client_certificate_path) tls.client_certificate_path = ep.tls_client_certificate_path
  if (ep.tls_client_key) tls.client_key = [ep.tls_client_key.trim()].filter(Boolean)
  if (ep.tls_client_key_path) tls.client_key_path = ep.tls_client_key_path
  if (ep.tls_version_min) tls.min_version = ep.tls_version_min
  if (ep.tls_version_max) tls.max_version = ep.tls_version_max
  return tls
}

// 节点「名称 / 标签」生成器与 xray 共用（见 sections/utils/nodeNaming.js），不另起一套。
import { genRemark, randSuffix } from '../../utils/nodeNaming'

// 端点表单默认值
export function endpointDefaultsForType(type, customPort) {
  // customPort：由调用方传入（后端 free-port 接口返回的空闲端口），
  // 保证新建端点不会与已有入站/端点撞端口。
  const listenPort = customPort > 0 ? customPort : 0
  const base = {
    type,
    tag: '',
    // 与入站编辑器一致：备注（界面「节点名称」）自动生成，不留空 ——
    // 此前默认 ''，新建端点的「节点名称」永远要手填（2026-10-09）。
    _remark: genRemark(),
    _enabled: true,
  }

  switch (type) {
    case 'wireguard': {
      // 生成随机私有网段 10.x.x.1/24
      const octet1 = Math.floor(Math.random() * 254) + 1
      const octet2 = Math.floor(Math.random() * 254) + 1
      const defaultAddr = `10.${octet1}.${octet2}.1/24`
      return {
        ...base,
        tag: 'wg-ep-' + randSuffix(),
        name: 'WireGuard-' + randSuffix(),
        // WireGuard endpoint fields
        system: false,
        mtu: 1408,
        address: [defaultAddr],
        _addressStr: defaultAddr,
        private_key: '',
        public_key: '',
        listen_port: listenPort > 0 ? listenPort : 51820,
        peers: [],
        workers: 0,
        on_demand: false,
        // UDP NAT（option/wireguard.go WireGuardEndpointOptions）
        udp_timeout: '5m',
        udp_mapping: '',
        udp_filtering: '',
        udp_nat_max: 0,
        // Peer defaults
        peer_address: '',
        peer_port: 0,
        pre_shared_key: '',
        persistent_keepalive_interval: 0,
        reserved: '',
      }
    }

    case 'tailscale':
      return {
        ...base,
        tag: 'ts-ep-' + randSuffix(),
        name: 'Tailscale-' + randSuffix(),
        state_directory: '',
        auth_key: '',
        control_url: '',
        ephemeral: false,
        hostname: '',
        accept_routes: false,
        exit_node: '',
        exit_node_allow_lan_access: false,
        advertise_routes: [],
        _advertiseRoutesStr: '',
        advertise_exit_node: false,
        advertise_tags: [],
        _advertiseTagsStr: '',
        listen_port: 0,
        relay_server_port: 0,
        relay_server_static_endpoints: [],
        _relayServerStaticEndpointsStr: '',
        system_interface: false,
        system_interface_name: '',
        system_interface_mtu: 0,
        udp_timeout: '5m',
        ssh_server: false,
        // TailscaleSSHServerOptions：三项全 false 时 sing-box 写布尔值，否则写对象
        ssh_disable_pty: false,
        ssh_disable_sftp: false,
        ssh_disable_forwarding: false,
        taildrop_directory: '',
        on_demand: false,
        // 拨号字段（DialerOptions）
        detour: '', bind_interface: '', inet4_bind_address: '', inet6_bind_address: '',
        bind_address_no_port: false, protect_path: '', routing_mark: 0, reuse_addr: false,
        netns: '', connect_timeout: '', tcp_fast_open: false, tcp_multi_path: false,
        disable_tcp_keep_alive: false, tcp_keep_alive: '', tcp_keep_alive_interval: '',
        udp_fragment: false, domain_resolver: '', network_strategy: '',
        _networkType: '', _fallbackNetworkType: '', fallback_delay: '',
      }

    case 'openconnect':
      return {
        ...base,
        tag: 'oc-ep-' + randSuffix(),
        name: 'OpenConnect-' + randSuffix(),
        system: false,
        server: '',
        flavor: 'anyconnect',
        username: '',
        password: '',
        auth_group: '',
        cookie: '',
        token: {
          mode: '',
          secret: '',
          secret_path: '',
          pin: '',
          password: '',
          device_id: '',
          counter: 0,
        },
        reported_os: '',
        user_agent: '',
        version: '',
        local_hostname: '',
        mobile_platform_version: '',
        mobile_device_type: '',
        mobile_device_unique_id: '',
        csd_wrapper_path: '',
        hip_wrapper_path: '',
        tncc_wrapper_path: '',
        tncc_device_id: '',
        tncc_user_agent: '',
        tncc_machine_identification_enabled: false,
        fortinet_hostcheck: '',
        fortinet_check_virtual_desktop: '',
        no_udp: false,
        dtls_local_port: 0,
        compression_disabled: false,
        compression_mode: '',
        ipv6_disabled: false,
        http_keepalive_disabled: false,
        xml_post_disabled: false,
        external_auth_disabled: false,
        password_authentication_disabled: false,
        tcp_keep_alive_enabled: false,
        pfs: false,
        mtu: 0,
        base_mtu: 0,
        dpd_interval: '',
        reconnect_timeout: '',
        trojan_interval: '',
        queue_length: 0,
        allow_insecure_crypto: false,
        // TLS settings
        tls_insecure: false,
        tls_server_name: '',
        tls_peer_fingerprint: '',
        tls_system_trust_disabled: false,
        tls_certificate_authority: '',
        tls_certificate_authority_path: '',
        tls_client_certificate: '',
        tls_client_certificate_path: '',
        tls_client_key: '',
        tls_client_key_path: '',
        tls_client_key_password: '',
        tls_mca_certificate: '',
        tls_mca_certificate_path: '',
        tls_mca_key: '',
        tls_mca_key_path: '',
        tls_mca_key_password: '',
        // TNC 客户端证书 / 表单字段 / 按需连接 / UDP NAT
        tncc_certificates: [],
        form_entries: [],
        on_demand: false,
        udp_timeout: '5m',
        udp_mapping: '',
        udp_filtering: '',
        udp_nat_max: 0,
        // 拨号字段
        detour: '', bind_interface: '', inet4_bind_address: '', inet6_bind_address: '',
        bind_address_no_port: false, protect_path: '', routing_mark: 0, reuse_addr: false,
        netns: '', connect_timeout: '', tcp_fast_open: false, tcp_multi_path: false,
        disable_tcp_keep_alive: false, tcp_keep_alive: '', tcp_keep_alive_interval: '',
        udp_fragment: false, domain_resolver: '', network_strategy: '',
        _networkType: '', _fallbackNetworkType: '', fallback_delay: '',
      }

    case 'openvpn-client':
      return {
        ...base,
        tag: 'ovpn-c-' + randSuffix(),
        name: 'OVPN-Client-' + randSuffix(),
        mode: 'tls',
        server: '127.0.0.1',
        server_port: 1194,
        servers: [],
        remote_random: false,
        network: 'udp',
        address: [],
        _addressStr: '',
        peer_address: '',
        peer_address_ipv6: '',
        topology: '',
        username: '',
        password: '',
        auth_retry: 'none',
        static_challenge: '',
        static_challenge_echo: false,
        static_key: '',
        static_key_path: '',
        key_direction: '',
        // TLS settings
        tls_server_name: '',
        tls_server_name_type: 'name',
        tls_certificate: '',
        tls_certificate_path: '',
        tls_client_certificate: '',
        tls_client_certificate_path: '',
        tls_client_key: '',
        tls_client_key_path: '',
        tls_peer_fingerprint: '',
        tls_crl_path: '',
        tls_certificate_profile: '',
        tls_ns_certificate_type: '',
        tls_version_min: '1.2',
        tls_version_max: '',
        tls_cipher: '',
        tls_groups: '',
        // Cipher settings
        cipher: '',
        data_ciphers: [],
        _dataCiphersStr: '',
        data_ciphers_fallback: '',
        auth: '',
        mss_fix: 0,
        mss_fix_disabled: false,
        mss_fix_mode: '',
        fragment: 0,
        replay_window: 0,
        replay_window_time: '',
        compression: '',
        compression_lzo: '',
        allow_compression: 'no',
        route_no_pull: false,
        routes: [],
        _routesStr: '',
        route_gateway: '',
        route_metric: 0,
        redirect_gateway: false,
        redirect_gateway_flags: [],
        _redirectGatewayFlagsStr: '',
        redirect_private: false,
        block_ipv6: false,
        pull_filters: [],
        // 重协商 / 心跳
        ping_interval: '',
        ping_restart: '',
        ping_restart_disabled: false,
        renegotiate_interval: '',
        renegotiate_disabled: false,
        renegotiate_bytes: 0,
        renegotiate_packets: 0,
        tls_timeout: '',
        handshake_window: '',
        explicit_exit_notify: 0,
        // TLS 远端证书约束 + control wrap
        tls_remote_certificate_ku: '',
        tls_remote_certificate_eku: '',
        tls_remote_certificate_tls: '',
        control_wrap_type: '',
        control_wrap_key: '',
        control_wrap_key_path: '',
        control_wrap_direction: '',
        // UDP NAT / 按需
        udp_timeout: '5m',
        udp_mapping: '',
        udp_filtering: '',
        udp_nat_max: 0,
        on_demand: false,
        system: false,
        // 拨号字段
        detour: '', bind_interface: '', inet4_bind_address: '', inet6_bind_address: '',
        bind_address_no_port: false, protect_path: '', routing_mark: 0, reuse_addr: false,
        netns: '', connect_timeout: '', tcp_fast_open: false, tcp_multi_path: false,
        disable_tcp_keep_alive: false, tcp_keep_alive: '', tcp_keep_alive_interval: '',
        udp_fragment: false, domain_resolver: '', network_strategy: '',
        _networkType: '', _fallbackNetworkType: '', fallback_delay: '',
      }

    case 'openvpn-server':
      return {
        ...base,
        tag: 'ovpn-s-' + randSuffix(),
        name: 'OVPN-Server-' + randSuffix(),
        system: false,
        mtu: 1500,
        mode: 'tls',
        network: 'udp',
        // 监听端口默认给一个该节点空闲端口（与入站编辑器的取端口方式对齐）
        listen_port: listenPort,
        remote: '',
        remote_port: 0,
        max_clients: 1024,
        address: [],
        _addressStr: '',
        peer_address: '',
        peer_address_ipv6: '',
        topology: 'subnet',
        duplicate_cn: false,
        users: [],
        static_key: '',
        static_key_path: '',
        key_direction: '',
        // TLS settings
        tls_certificate: '',
        tls_certificate_path: '',
        tls_key: '',
        tls_key_path: '',
        tls_client_certificate: '',
        tls_client_certificate_path: '',
        tls_verify_client_certificate: 'require',
        tls_client_name: '',
        tls_client_name_type: 'name',
        tls_peer_fingerprint: '',
        tls_crl_path: '',
        tls_certificate_profile: '',
        tls_ns_certificate_type: '',
        tls_version_min: '1.2',
        tls_version_max: '',
        tls_cipher: '',
        tls_groups: '',
        // Cipher settings
        cipher: '',
        data_ciphers: [],
        _dataCiphersStr: '',
        data_ciphers_fallback: '',
        auth: '',
        mss_fix: 0,
        mss_fix_disabled: false,
        mss_fix_mode: '',
        replay_window: 0,
        replay_window_time: '',
        // Push settings
        push_routes: [],
        _pushRoutesStr: '',
        push_dns: [],
        _pushDnsStr: '',
        // []OpenVPNPushDNSServerOptions：{priority, addresses[], resolve_domains[], dnssec, transport, sni}
        push_dns_servers: [],
        push_search_domains: [],
        _pushSearchDomainsStr: '',
        push_dhcp_options: [],
        _pushDhcpOptionsStr: '',
        push_redirect_gateway: false,
        push_redirect_gateway_flags: [],
        _pushRedirectGatewayFlagsStr: '',
        push_block_outside_dns: false,
        push_ping_interval: '',
        push_ping_restart: '',
        // Timing
        ping_interval: '',
        ping_restart: '',
        renegotiate_interval: '',
        renegotiate_disabled: false,
        renegotiate_bytes: 0,
        renegotiate_packets: 0,
        handshake_window: '1m',
        // OpenVPNEndpointOptions 的 UDP NAT
        udp_mapping: '',
        udp_filtering: '',
        udp_nat_max: 0,
        // TLS 远端证书约束 + control wrap（入站多一个 force_cookie）
        tls_remote_certificate_ku: '',
        tls_remote_certificate_eku: '',
        tls_remote_certificate_tls: '',
        control_wrap_type: '',
        control_wrap_key: '',
        control_wrap_key_path: '',
        control_wrap_direction: '',
        control_wrap_force_cookie: false,
        // 监听字段（ListenOptions）
        bind_interface: '', routing_mark: 0, reuse_addr: false, netns: '',
        tcp_fast_open: false, tcp_multi_path: false, disable_tcp_keep_alive: false,
        tcp_keep_alive: '', tcp_keep_alive_interval: '', udp_fragment: false, detour: '',
      }

    case 'masque-client':
      return {
        ...base,
        tag: 'mq-c-' + randSuffix(),
        name: 'MASQUE-Client-' + randSuffix(),
        system: false,
        mtu: 0,
        server: '',
        server_port: 443,
        username: '',
        password: '',
        // 官方默认 path 是 "/"（WARP 风格）；RFC 9484 模板可自行改成
        // "/.well-known/masque/ip/{target}/{ipproto}/"
        path: '/',
        version: 3,
        disable_version_fallback: false,
        headers: [],
        advertise_routes: [],
        _advertiseRoutesStr: '',
        udp_timeout: '5m',
        udp_mapping: '',
        udp_filtering: '',
        udp_nat_max: 0,
        on_demand: false,
        // HTTP/2 与 QUIC 的平铺字段（按 version 落盘）
        idle_timeout: '', keep_alive_period: '10s',
        stream_receive_window: '', connection_receive_window: '',
        max_concurrent_streams: 0,
        initial_packet_size: 0, disable_path_mtu_discovery: false,
        // TLS（客户端）
        tls_enabled: true, tls_server_name: '', tls_insecure: false, tls_disable_sni: false,
        tls_alpn: [], _tlsAlpnStr: 'h3',
        tls_certificate: '', tls_certificate_path: '',
        tls_certificate_public_key_sha256: '',
        tls_client_certificate: '', tls_client_certificate_path: '',
        tls_client_key: '', tls_client_key_path: '',
        tls_version_min: '', tls_version_max: '',
        // 拨号字段（DialerOptions）：放进默认模型，避免 build 的透传逻辑把
        // 空值/零值当成"界面未建模的字段"原样写进配置
        detour: '', bind_interface: '', inet4_bind_address: '', inet6_bind_address: '',
        bind_address_no_port: false, protect_path: '', routing_mark: 0, reuse_addr: false,
        netns: '', connect_timeout: '', tcp_fast_open: false, tcp_multi_path: false,
        disable_tcp_keep_alive: false, tcp_keep_alive: '', tcp_keep_alive_interval: '',
        udp_fragment: false, domain_resolver: '', network_strategy: '',
        _networkType: '', _fallbackNetworkType: '', fallback_delay: '',
      }

    case 'masque-server':
      return {
        ...base,
        tag: 'mq-s-' + randSuffix(),
        name: 'MASQUE-Server-' + randSuffix(),
        system: false,
        mtu: 0,
        listen: '0.0.0.0',
        listen_port: listenPort,
        // 服务器自己的 TUN 地址池（必填，最多一个 IPv4 + 一个 IPv6 前缀）
        address: [],
        _addressStr: '',
        path: '/.well-known/masque/ip/{target}/{ipproto}/',
        version: [1, 2, 3],
        users: [],
        advertise_routes: [],
        _advertiseRoutesStr: '',
        udp_timeout: '5m',
        udp_mapping: '',
        udp_filtering: '',
        udp_nat_max: 0,
        idle_timeout: '', keep_alive_period: '',
        stream_receive_window: '', connection_receive_window: '',
        max_concurrent_streams: 0,
        initial_packet_size: 0, disable_path_mtu_discovery: false,
        // TLS（服务端，HTTP/3 必需）
        tls_enabled: true, tls_server_name: '', tls_alpn: [], _tlsAlpnStr: '',
        tls_certificate: '', tls_certificate_path: '',
        tls_key: '', tls_key_path: '',
        tls_client_authentication: '', tls_client_certificate: '', tls_client_certificate_path: '',
        tls_version_min: '1.2', tls_version_max: '',
        // 监听字段（ListenOptions）
        bind_interface: '', routing_mark: 0, reuse_addr: false, netns: '',
        tcp_fast_open: false, tcp_multi_path: false, disable_tcp_keep_alive: false,
        tcp_keep_alive: '', tcp_keep_alive_interval: '', udp_fragment: false, detour: '',
      }

    default:
      return base
  }
}

// 列表字段统一转成逗号分隔字符串给表单
function listToStr(v) {
  if (Array.isArray(v)) return v.join(', ')
  return v || ''
}

// DialerOptions 反向解析（tailscale / openconnect / openvpn-client）
function parseDialOptions(ep) {
  return {
    detour: ep.detour || '',
    bind_interface: ep.bind_interface || '',
    inet4_bind_address: ep.inet4_bind_address || '',
    inet6_bind_address: ep.inet6_bind_address || '',
    bind_address_no_port: ep.bind_address_no_port || false,
    protect_path: ep.protect_path || '',
    routing_mark: ep.routing_mark || 0,
    reuse_addr: ep.reuse_addr || false,
    netns: ep.netns || '',
    connect_timeout: ep.connect_timeout || '',
    tcp_fast_open: ep.tcp_fast_open || false,
    tcp_multi_path: ep.tcp_multi_path || false,
    disable_tcp_keep_alive: ep.disable_tcp_keep_alive || false,
    tcp_keep_alive: ep.tcp_keep_alive || '',
    tcp_keep_alive_interval: ep.tcp_keep_alive_interval || '',
    udp_fragment: ep.udp_fragment || false,
    domain_resolver: typeof ep.domain_resolver === 'string' ? ep.domain_resolver : (ep.domain_resolver?.server || ''),
    network_strategy: ep.network_strategy || '',
    _networkType: Array.isArray(ep.network_type) ? ep.network_type.join(', ') : '',
    _fallbackNetworkType: Array.isArray(ep.fallback_network_type) ? ep.fallback_network_type.join(', ') : '',
    fallback_delay: ep.fallback_delay || '',
  }
}

// 表单模型 -> sing-box 配置（各类型的字段映射）
function buildEndpointTyped(ep) {
  switch (ep.type) {
    case 'wireguard': {
      const out = { type: 'wireguard' }
      if (ep.tag) out.tag = ep.tag
      if (ep.system) out.system = true
      if (ep.name) out.name = ep.name
      if (ep.mtu > 0) out.mtu = ep.mtu
      if (ep.address?.length) out.address = ep.address
      if (ep.private_key) out.private_key = ep.private_key
      // 公钥按项目既有约束处理：落库但不落地配置 ——
      // sing-box wireguard endpoint 没有顶层 public_key（公钥只属于 peers[].public_key），
      // 直接写 public_key 会让核心 strict decode FATAL（unknown field）；
      // 用 _ 前缀内部字段存进 endpoint_configs，供界面/他人取用，
      // 渲染运行配置时由后端剥离（同 CleanInboundFields 对 inbounds 的约束）。
      if (ep.public_key) out._public_key = ep.public_key
      if (ep.listen_port) out.listen_port = ep.listen_port
      if (ep.peers?.length) {
        out.peers = ep.peers.map(p => {
          const peer = { public_key: p.public_key }
          const ips = p._allowedIPsStr
            ? p._allowedIPsStr.split(',').map(s => s.trim()).filter(Boolean)
            : p.allowed_ips
          if (ips?.length) peer.allowed_ips = ips
          if (p.persistent_keepalive_interval > 0) peer.persistent_keepalive_interval = p.persistent_keepalive_interval
          if (p.pre_shared_key) peer.pre_shared_key = p.pre_shared_key
          if (p.address) peer.address = p.address
          if (p.port > 0) peer.port = p.port
          // reserved 可能是表单字符串 "34, 51, 234"，也可能是配置里的 int 数组（直接粘贴/导入时）——统一转数组，避免 split is not a function
          if (Array.isArray(p.reserved)) {
            if (p.reserved.length) peer.reserved = p.reserved
          } else if (p.reserved) {
            peer.reserved = String(p.reserved).split(',').map(s => parseInt(s.trim())).filter(n => !isNaN(n))
          }
          return peer
        })
      }
      if (ep.workers > 0) out.workers = ep.workers
      if (ep.on_demand) out.on_demand = true
      // UDP NAT：UDPTimeout 是 Duration，UDPNATBehavior 是枚举字符串
      if (ep.udp_timeout) out.udp_timeout = ep.udp_timeout
      if (ep.udp_mapping) out.udp_mapping = ep.udp_mapping
      if (ep.udp_filtering) out.udp_filtering = ep.udp_filtering
      if (ep.udp_nat_max > 0) out.udp_nat_max = ep.udp_nat_max
      return out
    }

    case 'tailscale': {
      const out = { type: 'tailscale' }
      if (ep.tag) out.tag = ep.tag
      if (ep.state_directory) out.state_directory = ep.state_directory
      if (ep.auth_key) out.auth_key = ep.auth_key
      if (ep.control_url) out.control_url = ep.control_url
      if (ep.ephemeral) out.ephemeral = true
      if (ep.hostname) out.hostname = ep.hostname
      if (ep.accept_routes) out.accept_routes = true
      if (ep.exit_node) out.exit_node = ep.exit_node
      if (ep.exit_node_allow_lan_access) out.exit_node_allow_lan_access = true
      if (ep.advertise_routes?.length) out.advertise_routes = ep.advertise_routes
      if (ep.advertise_exit_node) out.advertise_exit_node = true
      if (ep.advertise_tags?.length) out.advertise_tags = ep.advertise_tags
      if (ep.listen_port > 0) out.listen_port = ep.listen_port
      if (ep.relay_server_port > 0) out.relay_server_port = ep.relay_server_port
      if (ep.relay_server_static_endpoints?.length) out.relay_server_static_endpoints = ep.relay_server_static_endpoints
      if (ep.system_interface) out.system_interface = true
      if (ep.system_interface_name) out.system_interface_name = ep.system_interface_name
      if (ep.system_interface_mtu > 0) out.system_interface_mtu = ep.system_interface_mtu
      if (ep.udp_timeout) out.udp_timeout = ep.udp_timeout
      // ssh_server 既接受布尔也接受对象；有 disable_* 时必须写对象，
      // 否则开关会被静默丢弃（TailscaleSSHServerOptions.MarshalJSON）
      if (ep.ssh_server) {
        if (ep.ssh_disable_pty || ep.ssh_disable_sftp || ep.ssh_disable_forwarding) {
          const ssh = { enabled: true }
          if (ep.ssh_disable_pty) ssh.disable_pty = true
          if (ep.ssh_disable_sftp) ssh.disable_sftp = true
          if (ep.ssh_disable_forwarding) ssh.disable_forwarding = true
          out.ssh_server = ssh
        } else {
          out.ssh_server = true
        }
      }
      if (ep.taildrop_directory) out.taildrop_directory = ep.taildrop_directory
      if (ep.on_demand) out.on_demand = true
      applyDialOptions(out, ep)
      return out
    }

    case 'openconnect': {
      const out = { type: 'openconnect' }
      if (ep.tag) out.tag = ep.tag
      if (ep.system) out.system = true
      if (ep.name) out.name = ep.name
      if (ep.server) out.server = ep.server
      if (ep.flavor) out.flavor = ep.flavor
      if (ep.username) out.username = ep.username
      if (ep.password) out.password = ep.password
      if (ep.auth_group) out.auth_group = ep.auth_group
      if (ep.cookie) out.cookie = ep.cookie
      if (ep.token) {
        const token = {}
        if (ep.token.mode) token.mode = ep.token.mode
        if (ep.token.secret) token.secret = ep.token.secret
        if (ep.token.secret_path) token.secret_path = ep.token.secret_path
        if (ep.token.pin) token.pin = ep.token.pin
        if (ep.token.password) token.password = ep.token.password
        if (ep.token.device_id) token.device_id = ep.token.device_id
        if (ep.token.counter > 0) token.counter = ep.token.counter
        if (Object.keys(token).length) out.token = token
      }
      if (ep.reported_os) out.reported_os = ep.reported_os
      if (ep.user_agent) out.user_agent = ep.user_agent
      if (ep.version) out.version = ep.version
      if (ep.local_hostname) out.local_hostname = ep.local_hostname
      if (ep.mobile_platform_version || ep.mobile_device_type || ep.mobile_device_unique_id) {
        const mobile = {}
        if (ep.mobile_platform_version) mobile.platform_version = ep.mobile_platform_version
        if (ep.mobile_device_type) mobile.device_type = ep.mobile_device_type
        if (ep.mobile_device_unique_id) mobile.device_unique_id = ep.mobile_device_unique_id
        out.mobile = mobile
      }
      if (ep.csd_wrapper_path) out.csd = { wrapper_path: ep.csd_wrapper_path }
      if (ep.hip_wrapper_path) out.hip = { wrapper_path: ep.hip_wrapper_path }
      if (ep.tncc_wrapper_path || ep.tncc_device_id || ep.tncc_user_agent) {
        const tncc = {}
        if (ep.tncc_wrapper_path) tncc.wrapper_path = ep.tncc_wrapper_path
        if (ep.tncc_device_id) tncc.device_id = ep.tncc_device_id
        if (ep.tncc_user_agent) tncc.user_agent = ep.tncc_user_agent
        if (ep.tncc_machine_identification_enabled !== undefined) tncc.machine_identification_enabled = ep.tncc_machine_identification_enabled
        out.tncc = tncc
      }
      if (ep.fortinet_hostcheck || ep.fortinet_check_virtual_desktop) {
        const fortinet = {}
        if (ep.fortinet_hostcheck) fortinet.hostcheck = ep.fortinet_hostcheck
        if (ep.fortinet_check_virtual_desktop) fortinet.check_virtual_desktop = ep.fortinet_check_virtual_desktop
        out.fortinet_host_check = fortinet
      }
      if (ep.no_udp) out.no_udp = true
      if (ep.dtls_local_port > 0) out.dtls_local_port = ep.dtls_local_port
      if (ep.compression_disabled) out.compression_disabled = true
      if (ep.compression_mode) out.compression_mode = ep.compression_mode
      if (ep.ipv6_disabled) out.ipv6_disabled = true
      if (ep.http_keepalive_disabled) out.http_keepalive_disabled = true
      if (ep.xml_post_disabled) out.xml_post_disabled = true
      if (ep.external_auth_disabled) out.external_auth_disabled = true
      if (ep.password_authentication_disabled) out.password_authentication_disabled = true
      if (ep.tcp_keep_alive_enabled) out.tcp_keep_alive_enabled = true
      if (ep.pfs) out.pfs = true
      if (ep.mtu > 0) out.mtu = ep.mtu
      if (ep.base_mtu > 0) out.base_mtu = ep.base_mtu
      if (ep.dpd_interval) out.dpd_interval = ep.dpd_interval
      if (ep.reconnect_timeout) out.reconnect_timeout = ep.reconnect_timeout
      if (ep.trojan_interval) out.trojan_interval = ep.trojan_interval
      if (ep.queue_length > 0) out.queue_length = ep.queue_length
      if (ep.allow_insecure_crypto) out.allow_insecure_crypto = true
      if (ep.on_demand) out.on_demand = true
      // UDP NAT
      if (ep.udp_timeout) out.udp_timeout = ep.udp_timeout
      if (ep.udp_mapping) out.udp_mapping = ep.udp_mapping
      if (ep.udp_filtering) out.udp_filtering = ep.udp_filtering
      if (ep.udp_nat_max > 0) out.udp_nat_max = ep.udp_nat_max
      // TLS
      const tls = {}
      if (ep.tls_insecure) tls.insecure = true
      if (ep.tls_server_name) tls.server_name = ep.tls_server_name
      if (ep.tls_peer_fingerprint) tls.peer_fingerprint = ep.tls_peer_fingerprint.split(',').map(s => s.trim()).filter(Boolean)
      if (ep.tls_system_trust_disabled) tls.system_trust_disabled = true
      if (ep.tls_certificate_authority) tls.certificate_authority = splitCSV(ep.tls_certificate_authority)
      if (ep.tls_certificate_authority_path) tls.certificate_authority_path = ep.tls_certificate_authority_path
      if (ep.tls_client_certificate) tls.client_certificate = splitCSV(ep.tls_client_certificate)
      if (ep.tls_client_certificate_path) tls.client_certificate_path = ep.tls_client_certificate_path
      if (ep.tls_client_key) tls.client_key = splitCSV(ep.tls_client_key)
      if (ep.tls_client_key_path) tls.client_key_path = ep.tls_client_key_path
      if (ep.tls_client_key_password) tls.client_key_password = ep.tls_client_key_password
      if (ep.tls_mca_certificate) tls.mca_certificate = splitCSV(ep.tls_mca_certificate)
      if (ep.tls_mca_certificate_path) tls.mca_certificate_path = ep.tls_mca_certificate_path
      if (ep.tls_mca_key) tls.mca_key = splitCSV(ep.tls_mca_key)
      if (ep.tls_mca_key_path) tls.mca_key_path = ep.tls_mca_key_path
      if (ep.tls_mca_key_password) tls.mca_key_password = ep.tls_mca_key_password
      if (Object.keys(tls).length) out.tls = tls
      // TNC 客户端证书（tncc.certificates）
      const tnccCerts = (ep.tncc_certificates || [])
        .filter(c => c && (c.certificate || c.certificate_path))
        .map(c => {
          const cert = {}
          if (c.certificate) cert.certificate = splitCSV(c.certificate)
          if (c.certificate_path) cert.certificate_path = c.certificate_path
          return cert
        })
      if (tnccCerts.length) out.tncc = { ...(out.tncc || {}), certificates: tnccCerts }
      // 表单字段（form_entries）
      const formEntries = (ep.form_entries || [])
        .filter(f => f && (f.form_id || f.name))
        .map(f => {
          const entry = { name: f.name || '' }
          if (f.form_id) entry.form_id = f.form_id
          if (f.submission_key) entry.submission_key = f.submission_key
          if (f.value) entry.value = f.value
          if (f.promote) entry.promote = true
          return entry
        })
      if (formEntries.length) out.form_entries = formEntries
      applyDialOptions(out, ep)
      return out
    }

    case 'openvpn-client': {
      const out = { type: 'openvpn-client' }
      if (ep.tag) out.tag = ep.tag
      if (ep.mode) out.mode = ep.mode
      if (ep.server) out.server = ep.server
      if (ep.server_port) out.server_port = ep.server_port
      if (ep.servers?.length) out.servers = ep.servers
      if (ep.remote_random) out.remote_random = true
      if (ep.network) out.network = ep.network
      if (ep.address?.length || ep._addressStr) out.address = ep.address?.length ? ep.address : splitCSV(ep._addressStr)
      if (ep.peer_address) out.peer_address = ep.peer_address
      if (ep.peer_address_ipv6) out.peer_address_ipv6 = ep.peer_address_ipv6
      if (ep.topology) out.topology = ep.topology
      if (ep.username) out.username = ep.username
      if (ep.password) out.password = ep.password
      if (ep.auth_retry && ep.auth_retry !== 'none') out.auth_retry = ep.auth_retry
      if (ep.static_challenge) out.static_challenge = ep.static_challenge
      if (ep.static_challenge_echo) out.static_challenge_echo = true
      if (ep.static_key) out.static_key = ep.static_key
      if (ep.static_key_path) out.static_key_path = ep.static_key_path
      if (ep.key_direction) out.key_direction = ep.key_direction
      // TLS
      const tls = {}
      if (ep.tls_server_name) tls.server_name = ep.tls_server_name
      if (ep.tls_server_name_type && ep.tls_server_name_type !== 'name') tls.server_name_type = ep.tls_server_name_type
      if (ep.tls_certificate) tls.certificate = ep.tls_certificate
      if (ep.tls_certificate_path) tls.certificate_path = ep.tls_certificate_path
      if (ep.tls_client_certificate) tls.client_certificate = ep.tls_client_certificate
      if (ep.tls_client_certificate_path) tls.client_certificate_path = ep.tls_client_certificate_path
      if (ep.tls_client_key) tls.client_key = ep.tls_client_key
      if (ep.tls_client_key_path) tls.client_key_path = ep.tls_client_key_path
      if (ep.tls_peer_fingerprint) tls.peer_fingerprint = ep.tls_peer_fingerprint.split(',').map(s => s.trim()).filter(Boolean)
      if (ep.tls_crl_path) tls.crl_path = ep.tls_crl_path
      if (ep.tls_certificate_profile) tls.certificate_profile = ep.tls_certificate_profile
      if (ep.tls_ns_certificate_type) tls.ns_certificate_type = ep.tls_ns_certificate_type
      if (ep.tls_version_min && ep.tls_version_min !== '1.2') tls.version_min = ep.tls_version_min
      if (ep.tls_version_max) tls.version_max = ep.tls_version_max
      if (ep.tls_cipher) tls.cipher = ep.tls_cipher
      if (ep.tls_groups) tls.groups = ep.tls_groups
      if (ep.tls_remote_certificate_ku) tls.remote_certificate_ku = splitCSV(ep.tls_remote_certificate_ku)
      if (ep.tls_remote_certificate_eku) tls.remote_certificate_eku = ep.tls_remote_certificate_eku
      if (ep.tls_remote_certificate_tls) tls.remote_certificate_tls = ep.tls_remote_certificate_tls
      const cw = buildControlWrap(ep)
      if (cw) tls.control_wrap = cw
      if (Object.keys(tls).length) out.tls = tls
      // Cipher
      if (ep.cipher) out.cipher = ep.cipher
      if (ep.data_ciphers?.length) out.data_ciphers = ep.data_ciphers
      if (ep.data_ciphers_fallback) out.data_ciphers_fallback = ep.data_ciphers_fallback
      if (ep.auth) out.auth = ep.auth
      // mss_fix 与 mss_fix_disabled 互斥（核心会直接 FATAL）
      if (ep.mss_fix_disabled) out.mss_fix_disabled = true
      else if (ep.mss_fix > 0) {
        out.mss_fix = ep.mss_fix
        if (ep.mss_fix_mode) out.mss_fix_mode = ep.mss_fix_mode
      }
      if (ep.fragment > 0) out.fragment = ep.fragment
      if (ep.replay_window > 0) out.replay_window = ep.replay_window
      if (ep.replay_window_time) out.replay_window_time = ep.replay_window_time
      if (ep.compression) out.compression = ep.compression
      if (ep.compression_lzo) out.compression_lzo = ep.compression_lzo
      if (ep.allow_compression && ep.allow_compression !== 'no') out.allow_compression = ep.allow_compression
      if (ep.route_no_pull) out.route_no_pull = true
      if (ep.routes?.length) out.routes = ep.routes
      if (ep.route_gateway) out.route_gateway = ep.route_gateway
      if (ep.route_metric > 0) out.route_metric = ep.route_metric
      if (ep.redirect_gateway) out.redirect_gateway = true
      if (ep.redirect_gateway_flags?.length) out.redirect_gateway_flags = ep.redirect_gateway_flags
      if (ep.redirect_private) out.redirect_private = true
      if (ep.block_ipv6) out.block_ipv6 = true
      const pullFilters = (ep.pull_filters || [])
        .filter(f => f && f.action && f.text)
        .map(f => ({ action: f.action, text: f.text }))
      if (pullFilters.length) out.pull_filters = pullFilters
      if (ep.ping_interval) out.ping_interval = ep.ping_interval
      if (ep.ping_restart_disabled) out.ping_restart_disabled = true
      else if (ep.ping_restart) out.ping_restart = ep.ping_restart
      if (ep.renegotiate_disabled) out.renegotiate_disabled = true
      else if (ep.renegotiate_interval) out.renegotiate_interval = ep.renegotiate_interval
      if (ep.renegotiate_bytes > 0) out.renegotiate_bytes = ep.renegotiate_bytes
      if (ep.renegotiate_packets > 0) out.renegotiate_packets = ep.renegotiate_packets
      if (ep.tls_timeout) out.tls_timeout = ep.tls_timeout
      if (ep.handshake_window) out.handshake_window = ep.handshake_window
      if (ep.explicit_exit_notify > 0) out.explicit_exit_notify = ep.explicit_exit_notify
      if (ep.udp_timeout) out.udp_timeout = ep.udp_timeout
      if (ep.udp_mapping) out.udp_mapping = ep.udp_mapping
      if (ep.udp_filtering) out.udp_filtering = ep.udp_filtering
      if (ep.udp_nat_max > 0) out.udp_nat_max = ep.udp_nat_max
      if (ep.on_demand) out.on_demand = true
      if (ep.system) out.system = true
      if (ep.name) out.name = ep.name
      applyDialOptions(out, ep)
      return out
    }

    case 'openvpn-server': {
      const out = { type: 'openvpn-server' }
      if (ep.tag) out.tag = ep.tag
      applyListenOptions(out, ep)
      if (ep.udp_mapping) out.udp_mapping = ep.udp_mapping
      if (ep.udp_filtering) out.udp_filtering = ep.udp_filtering
      if (ep.udp_nat_max > 0) out.udp_nat_max = ep.udp_nat_max
      if (ep.system) out.system = true
      if (ep.name) out.name = ep.name
      if (ep.mtu > 0) out.mtu = ep.mtu
      if (ep.mode) out.mode = ep.mode
      if (ep.network) out.network = ep.network
      if (ep.remote) out.remote = ep.remote
      if (ep.remote_port > 0) out.remote_port = ep.remote_port
      if (ep.max_clients > 0) out.max_clients = ep.max_clients
      if (ep.address?.length || ep._addressStr) out.address = ep.address?.length ? ep.address : splitCSV(ep._addressStr)
      if (ep.peer_address) out.peer_address = ep.peer_address
      if (ep.peer_address_ipv6) out.peer_address_ipv6 = ep.peer_address_ipv6
      if (ep.topology) out.topology = ep.topology
      if (ep.duplicate_cn) out.duplicate_cn = true
      if (ep.users?.length) out.users = ep.users
      if (ep.static_key) out.static_key = ep.static_key
      if (ep.static_key_path) out.static_key_path = ep.static_key_path
      if (ep.key_direction) out.key_direction = ep.key_direction
      // TLS
      const tls = {}
      if (ep.tls_certificate) tls.certificate = ep.tls_certificate
      if (ep.tls_certificate_path) tls.certificate_path = ep.tls_certificate_path
      if (ep.tls_key) tls.key = ep.tls_key
      if (ep.tls_key_path) tls.key_path = ep.tls_key_path
      if (ep.tls_client_certificate) tls.client_certificate = ep.tls_client_certificate
      if (ep.tls_client_certificate_path) tls.client_certificate_path = ep.tls_client_certificate_path
      if (ep.tls_verify_client_certificate && ep.tls_verify_client_certificate !== 'require') tls.verify_client_certificate = ep.tls_verify_client_certificate
      if (ep.tls_client_name) tls.client_name = ep.tls_client_name
      if (ep.tls_client_name_type && ep.tls_client_name_type !== 'name') tls.client_name_type = ep.tls_client_name_type
      if (ep.tls_peer_fingerprint) tls.peer_fingerprint = ep.tls_peer_fingerprint.split(',').map(s => s.trim()).filter(Boolean)
      if (ep.tls_crl_path) tls.crl_path = ep.tls_crl_path
      if (ep.tls_certificate_profile) tls.certificate_profile = ep.tls_certificate_profile
      if (ep.tls_ns_certificate_type) tls.ns_certificate_type = ep.tls_ns_certificate_type
      if (ep.tls_version_min && ep.tls_version_min !== '1.2') tls.version_min = ep.tls_version_min
      if (ep.tls_version_max) tls.version_max = ep.tls_version_max
      if (ep.tls_cipher) tls.cipher = ep.tls_cipher
      if (ep.tls_groups) tls.groups = ep.tls_groups
      if (ep.tls_remote_certificate_ku) tls.remote_certificate_ku = splitCSV(ep.tls_remote_certificate_ku)
      if (ep.tls_remote_certificate_eku) tls.remote_certificate_eku = ep.tls_remote_certificate_eku
      if (ep.tls_remote_certificate_tls) tls.remote_certificate_tls = ep.tls_remote_certificate_tls
      const cw = buildControlWrap(ep)
      if (cw) tls.control_wrap = cw
      if (Object.keys(tls).length) out.tls = tls
      // Cipher
      if (ep.cipher) out.cipher = ep.cipher
      if (ep.data_ciphers?.length) out.data_ciphers = ep.data_ciphers
      if (ep.data_ciphers_fallback) out.data_ciphers_fallback = ep.data_ciphers_fallback
      if (ep.auth) out.auth = ep.auth
      // mss_fix 与 mss_fix_disabled 互斥（核心会直接 FATAL）
      if (ep.mss_fix_disabled) out.mss_fix_disabled = true
      else if (ep.mss_fix > 0) {
        out.mss_fix = ep.mss_fix
        if (ep.mss_fix_mode) out.mss_fix_mode = ep.mss_fix_mode
      }
      if (ep.replay_window > 0) out.replay_window = ep.replay_window
      if (ep.replay_window_time) out.replay_window_time = ep.replay_window_time
      // Push
      const push = {}
      if (ep.push_routes?.length) push.routes = ep.push_routes
      if (ep.push_dns?.length) push.dns = ep.push_dns
      const dnsServers = (ep.push_dns_servers || [])
        .filter(d => d && (d.addresses?.length || d.addresses))
        .map(d => {
          const entry = { addresses: Array.isArray(d.addresses) ? d.addresses : splitCSV(d.addresses) }
          if (d.priority > 0) entry.priority = d.priority
          if (d.resolve_domains?.length) entry.resolve_domains = d.resolve_domains
          if (d.dnssec) entry.dnssec = d.dnssec
          if (d.transport) entry.transport = d.transport
          if (d.sni) entry.sni = d.sni
          return entry
        })
      if (dnsServers.length) push.dns_servers = dnsServers
      if (ep.push_search_domains?.length) push.search_domains = ep.push_search_domains
      if (ep.push_dhcp_options?.length) push.dhcp_options = ep.push_dhcp_options
      if (ep.push_redirect_gateway) push.redirect_gateway = true
      if (ep.push_redirect_gateway_flags?.length) push.redirect_gateway_flags = ep.push_redirect_gateway_flags
      if (ep.push_block_outside_dns) push.block_outside_dns = true
      if (ep.push_ping_interval) push.ping_interval = ep.push_ping_interval
      if (ep.push_ping_restart) push.ping_restart = ep.push_ping_restart
      if (Object.keys(push).length) out.push = push
      // Timing
      if (ep.ping_interval) out.ping_interval = ep.ping_interval
      if (ep.ping_restart) out.ping_restart = ep.ping_restart
      if (ep.renegotiate_disabled) out.renegotiate_disabled = true
      else if (ep.renegotiate_interval) out.renegotiate_interval = ep.renegotiate_interval
      if (ep.renegotiate_bytes > 0) out.renegotiate_bytes = ep.renegotiate_bytes
      if (ep.renegotiate_packets > 0) out.renegotiate_packets = ep.renegotiate_packets
      if (ep.handshake_window && ep.handshake_window !== '1m') out.handshake_window = ep.handshake_window
      return out
    }

    case 'masque-client': {
      const out = { type: 'masque-client' }
      if (ep.tag) out.tag = ep.tag
      applyMasqueEndpointOptions(out, ep)
      if (ep.server) out.server = ep.server
      if (ep.server_port) out.server_port = ep.server_port
      if (ep.username) out.username = ep.username
      if (ep.password) out.password = ep.password
      if (ep.path) out.path = ep.path
      if (ep.version > 0) out.version = ep.version
      if (ep.disable_version_fallback) out.disable_version_fallback = true
      const hdrs = headersFromList(ep.headers)
      if (Object.keys(hdrs).length) out.headers = hdrs
      const routes = ep._advertiseRoutesStr ? splitCSV(ep._advertiseRoutesStr) : (ep.advertise_routes || [])
      if (routes.length) out.advertise_routes = routes
      if (ep.udp_timeout) out.udp_timeout = ep.udp_timeout
      if (ep.on_demand) out.on_demand = true
      applyHTTPVersionOptions(out, ep)
      applyDialOptions(out, ep)
      const tls = buildOutboundTLS(ep)
      if (tls) out.tls = tls
      return out
    }

    case 'masque-server': {
      const out = { type: 'masque-server' }
      if (ep.tag) out.tag = ep.tag
      applyMasqueEndpointOptions(out, ep)
      applyListenOptions(out, ep)
      const addrs = splitCSV(ep._addressStr) || ep.address
      if (Array.isArray(addrs) && addrs.length) out.address = addrs
      if (ep.path) out.path = ep.path
      const versions = Array.isArray(ep.version) ? ep.version : splitCSV(ep.version)
      if (versions.length) out.version = versions.map(v => parseInt(v, 10)).filter(n => !isNaN(n))
      if (ep.users?.length) {
        out.users = ep.users
          .filter(u => u && (u.username || u.password))
          .map(u => ({ username: u.username || '', password: u.password || '' }))
      }
      const routes = ep._advertiseRoutesStr ? splitCSV(ep._advertiseRoutesStr) : (ep.advertise_routes || [])
      if (routes.length) out.advertise_routes = routes
      applyHTTPVersionOptions(out, ep, out.version)
      const tls = buildInboundTLS(ep)
      if (tls) out.tls = tls
      return out
    }

    default:
      return { type: ep.type, tag: ep.tag }
  }
}

// 表单模型 -> sing-box 配置
//
// 先按类型映射，再把「表单没建模」的原始字段原样透传：文档里有但界面没做的字段
//（wireguard 的 UDP NAT udp_mapping/udp_filtering/udp_nat_max、通用 dial 字段、
// openvpn 的 pull_filters/redirect_private/tls_timeout 等），编辑一次就保存的话
// 会被整个抹掉。判断依据：字段不在该类型表单默认模型里（说明不是界面建模的）。
// 「_」开头的是表单内部字段（_addressStr/_remark 等），绝不进配置。
export function buildEndpoint(ep) {
  const out = buildEndpointTyped(ep)
  let formKeys = {}
  try {
    formKeys = endpointDefaultsForType(ep.type, 0)
  } catch { /* 未知类型，走 default 分支只输出 type/tag */ }
  for (const k of Object.keys(ep)) {
    if (k.startsWith('_') || k in formKeys || k in out) continue
    const v = ep[k]
    if (v === undefined || v === null || v === '') continue
    out[k] = v
  }
  return out
}

// sing-box 配置 -> 表单模型
export function parseEndpoint(ep) {
  // ...ep：保留文档里有、表单没建模的字段（UDP NAT、dial、openvpn pull_filters 等），
  // 与 buildEndpoint 的透传配对，否则编辑保存一次这些字段就丢了
  const base = {
    ...ep,
    type: ep.type || 'wireguard',
    tag: ep.tag || '',
    _remark: ep.tag || '',
    _enabled: true,
  }

  switch (ep.type) {
    case 'wireguard':
      return {
        ...base,
        system: ep.system || false,
        name: ep.name || '',
        mtu: ep.mtu || 1408,
        address: Array.isArray(ep.address) ? ep.address : [],
        _addressStr: Array.isArray(ep.address) ? ep.address.join(', ') : '',
        private_key: ep.private_key || '',
        // 公钥：新数据存 _public_key（build 输出），兼容历史上直接写 public_key 的记录
        public_key: ep._public_key || ep.public_key || '',
        listen_port: ep.listen_port || 0,
        peers: (ep.peers || []).map(p => ({
          ...p,
          _allowedIPsStr: Array.isArray(p.allowed_ips) ? p.allowed_ips.join(', ') : (p.allowed_ips || '0.0.0.0/0, ::/0'),
          // 配置里 reserved 是 int 数组（sing-box 原生格式），表单要字符串 ——
          // 不转换的话保存时 build 侧 p.reserved.split 直接崩（split is not a function）
          reserved: Array.isArray(p.reserved) ? p.reserved.join(', ') : (p.reserved || ''),
        })),
        workers: ep.workers || 0,
        on_demand: ep.on_demand || false,
        udp_timeout: ep.udp_timeout || '5m',
        udp_mapping: ep.udp_mapping || '',
        udp_filtering: ep.udp_filtering || '',
        udp_nat_max: ep.udp_nat_max || 0,
      }

    case 'tailscale':
      return {
        ...base,
        state_directory: ep.state_directory || '',
        auth_key: ep.auth_key || '',
        control_url: ep.control_url || '',
        ephemeral: ep.ephemeral || false,
        hostname: ep.hostname || '',
        accept_routes: ep.accept_routes || false,
        exit_node: ep.exit_node || '',
        exit_node_allow_lan_access: ep.exit_node_allow_lan_access || false,
        advertise_routes: ep.advertise_routes || [],
        _advertiseRoutesStr: (ep.advertise_routes || []).join(', '),
        advertise_exit_node: ep.advertise_exit_node || false,
        advertise_tags: ep.advertise_tags || [],
        _advertiseTagsStr: (ep.advertise_tags || []).join(', '),
        listen_port: ep.listen_port || 0,
        relay_server_port: ep.relay_server_port || 0,
        relay_server_static_endpoints: ep.relay_server_static_endpoints || [],
        _relayServerStaticEndpointsStr: (ep.relay_server_static_endpoints || []).join(', '),
        system_interface: ep.system_interface || false,
        system_interface_name: ep.system_interface_name || '',
        system_interface_mtu: ep.system_interface_mtu || 0,
        udp_timeout: ep.udp_timeout || '5m',
        ssh_server: ep.ssh_server === true || (!!ep.ssh_server && ep.ssh_server.enabled !== false),
        ssh_disable_pty: ep.ssh_server?.disable_pty || false,
        ssh_disable_sftp: ep.ssh_server?.disable_sftp || false,
        ssh_disable_forwarding: ep.ssh_server?.disable_forwarding || false,
        taildrop_directory: ep.taildrop_directory || '',
        on_demand: ep.on_demand || false,
        ...parseDialOptions(ep),
      }

    case 'openconnect':
      return {
        ...base,
        system: ep.system || false,
        name: ep.name || '',
        server: ep.server || '',
        flavor: ep.flavor || 'anyconnect',
        username: ep.username || '',
        password: ep.password || '',
        auth_group: ep.auth_group || '',
        cookie: ep.cookie || '',
        token: ep.token || { mode: '', secret: '', secret_path: '', pin: '', password: '', device_id: '', counter: 0 },
        reported_os: ep.reported_os || '',
        user_agent: ep.user_agent || '',
        version: ep.version || '',
        local_hostname: ep.local_hostname || '',
        mobile_platform_version: ep.mobile?.platform_version || '',
        mobile_device_type: ep.mobile?.device_type || '',
        mobile_device_unique_id: ep.mobile?.device_unique_id || '',
        csd_wrapper_path: ep.csd?.wrapper_path || '',
        hip_wrapper_path: ep.hip?.wrapper_path || '',
        tncc_wrapper_path: ep.tncc?.wrapper_path || '',
        tncc_device_id: ep.tncc?.device_id || '',
        tncc_user_agent: ep.tncc?.user_agent || '',
        tncc_machine_identification_enabled: ep.tncc?.machine_identification_enabled || false,
        fortinet_hostcheck: ep.fortinet_host_check?.hostcheck || '',
        fortinet_check_virtual_desktop: ep.fortinet_host_check?.check_virtual_desktop || '',
        no_udp: ep.no_udp || false,
        dtls_local_port: ep.dtls_local_port || 0,
        compression_disabled: ep.compression_disabled || false,
        compression_mode: ep.compression_mode || '',
        ipv6_disabled: ep.ipv6_disabled || false,
        http_keepalive_disabled: ep.http_keepalive_disabled || false,
        xml_post_disabled: ep.xml_post_disabled || false,
        external_auth_disabled: ep.external_auth_disabled || false,
        password_authentication_disabled: ep.password_authentication_disabled || false,
        tcp_keep_alive_enabled: ep.tcp_keep_alive_enabled || false,
        pfs: ep.pfs || false,
        mtu: ep.mtu || 0,
        base_mtu: ep.base_mtu || 0,
        dpd_interval: ep.dpd_interval || '',
        reconnect_timeout: ep.reconnect_timeout || '',
        trojan_interval: ep.trojan_interval || '',
        queue_length: ep.queue_length || 0,
        allow_insecure_crypto: ep.allow_insecure_crypto || false,
        tls_insecure: ep.tls?.insecure || false,
        tls_server_name: ep.tls?.server_name || '',
        tls_peer_fingerprint: Array.isArray(ep.tls?.peer_fingerprint) ? ep.tls.peer_fingerprint.join(', ') : '',
        tls_system_trust_disabled: ep.tls?.system_trust_disabled || false,
        tls_certificate_authority: listToStr(ep.tls?.certificate_authority),
        tls_certificate_authority_path: ep.tls?.certificate_authority_path || '',
        tls_client_certificate: listToStr(ep.tls?.client_certificate),
        tls_client_certificate_path: ep.tls?.client_certificate_path || '',
        tls_client_key: listToStr(ep.tls?.client_key),
        tls_client_key_path: ep.tls?.client_key_path || '',
        tls_client_key_password: ep.tls?.client_key_password || '',
        tls_mca_certificate: listToStr(ep.tls?.mca_certificate),
        tls_mca_certificate_path: ep.tls?.mca_certificate_path || '',
        tls_mca_key: listToStr(ep.tls?.mca_key),
        tls_mca_key_path: ep.tls?.mca_key_path || '',
        tls_mca_key_password: ep.tls?.mca_key_password || '',
        tncc_certificates: (ep.tncc?.certificates || []).map(c => ({
          certificate: listToStr(c.certificate),
          certificate_path: c.certificate_path || '',
        })),
        form_entries: (ep.form_entries || []).map(f => ({
          form_id: f.form_id || '',
          submission_key: f.submission_key || '',
          name: f.name || '',
          value: f.value || '',
          promote: f.promote || false,
        })),
        on_demand: ep.on_demand || false,
        udp_timeout: ep.udp_timeout || '5m',
        udp_mapping: ep.udp_mapping || '',
        udp_filtering: ep.udp_filtering || '',
        udp_nat_max: ep.udp_nat_max || 0,
        ...parseDialOptions(ep),
      }

    case 'openvpn-client':
      return {
        ...base,
        mode: ep.mode || 'tls',
        server: ep.server || '127.0.0.1',
        server_port: ep.server_port || 1194,
        servers: ep.servers || [],
        remote_random: ep.remote_random || false,
        network: ep.network || 'udp',
        address: Array.isArray(ep.address) ? ep.address : [],
        _addressStr: Array.isArray(ep.address) ? ep.address.join(', ') : '',
        peer_address: ep.peer_address || '',
        peer_address_ipv6: ep.peer_address_ipv6 || '',
        topology: ep.topology || '',
        username: ep.username || '',
        password: ep.password || '',
        auth_retry: ep.auth_retry || 'none',
        static_challenge: ep.static_challenge || '',
        static_challenge_echo: ep.static_challenge_echo || false,
        static_key: ep.static_key || '',
        static_key_path: ep.static_key_path || '',
        key_direction: ep.key_direction || '',
        tls_server_name: ep.tls?.server_name || '',
        tls_server_name_type: ep.tls?.server_name_type || 'name',
        tls_certificate: ep.tls?.certificate || '',
        tls_certificate_path: ep.tls?.certificate_path || '',
        tls_client_certificate: ep.tls?.client_certificate || '',
        tls_client_certificate_path: ep.tls?.client_certificate_path || '',
        tls_client_key: ep.tls?.client_key || '',
        tls_client_key_path: ep.tls?.client_key_path || '',
        tls_peer_fingerprint: Array.isArray(ep.tls?.peer_fingerprint) ? ep.tls.peer_fingerprint.join(', ') : '',
        tls_crl_path: ep.tls?.crl_path || '',
        tls_certificate_profile: ep.tls?.certificate_profile || '',
        tls_ns_certificate_type: ep.tls?.ns_certificate_type || '',
        tls_version_min: ep.tls?.version_min || '1.2',
        tls_version_max: ep.tls?.version_max || '',
        tls_cipher: ep.tls?.cipher || '',
        tls_groups: ep.tls?.groups || '',
        tls_remote_certificate_ku: listToStr(ep.tls?.remote_certificate_ku),
        tls_remote_certificate_eku: ep.tls?.remote_certificate_eku || '',
        tls_remote_certificate_tls: ep.tls?.remote_certificate_tls || '',
        control_wrap_type: ep.tls?.control_wrap?.type || '',
        control_wrap_key: listToStr(ep.tls?.control_wrap?.key),
        control_wrap_key_path: ep.tls?.control_wrap?.key_path || '',
        control_wrap_direction: ep.tls?.control_wrap?.direction || '',
        cipher: ep.cipher || '',
        data_ciphers: ep.data_ciphers || [],
        _dataCiphersStr: (ep.data_ciphers || []).join(', '),
        data_ciphers_fallback: ep.data_ciphers_fallback || '',
        auth: ep.auth || '',
        mss_fix: ep.mss_fix || 0,
        mss_fix_disabled: ep.mss_fix_disabled || false,
        mss_fix_mode: ep.mss_fix_mode || '',
        fragment: ep.fragment || 0,
        replay_window: ep.replay_window || 0,
        replay_window_time: ep.replay_window_time || '',
        compression: ep.compression || '',
        compression_lzo: ep.compression_lzo || '',
        allow_compression: ep.allow_compression || 'no',
        route_no_pull: ep.route_no_pull || false,
        routes: ep.routes || [],
        _routesStr: (ep.routes || []).join(', '),
        route_gateway: ep.route_gateway || '',
        route_metric: ep.route_metric || 0,
        redirect_gateway: ep.redirect_gateway || false,
        redirect_gateway_flags: ep.redirect_gateway_flags || [],
        _redirectGatewayFlagsStr: (ep.redirect_gateway_flags || []).join(', '),
        redirect_private: ep.redirect_private || false,
        block_ipv6: ep.block_ipv6 || false,
        pull_filters: (ep.pull_filters || []).map(f => ({ action: f.action || '', text: f.text || '' })),
        ping_interval: ep.ping_interval || '',
        ping_restart: ep.ping_restart || '',
        ping_restart_disabled: ep.ping_restart_disabled || false,
        renegotiate_interval: ep.renegotiate_interval || '',
        renegotiate_disabled: ep.renegotiate_disabled || false,
        renegotiate_bytes: ep.renegotiate_bytes || 0,
        renegotiate_packets: ep.renegotiate_packets || 0,
        tls_timeout: ep.tls_timeout || '',
        handshake_window: ep.handshake_window || '',
        explicit_exit_notify: ep.explicit_exit_notify || 0,
        udp_timeout: ep.udp_timeout || '5m',
        udp_mapping: ep.udp_mapping || '',
        udp_filtering: ep.udp_filtering || '',
        udp_nat_max: ep.udp_nat_max || 0,
        on_demand: ep.on_demand || false,
        system: ep.system || false,
        name: ep.name || '',
        ...parseDialOptions(ep),
      }

    case 'openvpn-server':
      return {
        ...base,
        system: ep.system || false,
        name: ep.name || '',
        mtu: ep.mtu || 1500,
        mode: ep.mode || 'tls',
        network: ep.network || 'udp',
        remote: ep.remote || '',
        remote_port: ep.remote_port || 0,
        max_clients: ep.max_clients || 1024,
        address: Array.isArray(ep.address) ? ep.address : [],
        _addressStr: Array.isArray(ep.address) ? ep.address.join(', ') : '',
        peer_address: ep.peer_address || '',
        peer_address_ipv6: ep.peer_address_ipv6 || '',
        topology: ep.topology || 'subnet',
        duplicate_cn: ep.duplicate_cn || false,
        users: ep.users || [],
        static_key: ep.static_key || '',
        static_key_path: ep.static_key_path || '',
        key_direction: ep.key_direction || '',
        tls_certificate: ep.tls?.certificate || '',
        tls_certificate_path: ep.tls?.certificate_path || '',
        tls_key: ep.tls?.key || '',
        tls_key_path: ep.tls?.key_path || '',
        tls_client_certificate: ep.tls?.client_certificate || '',
        tls_client_certificate_path: ep.tls?.client_certificate_path || '',
        tls_verify_client_certificate: ep.tls?.verify_client_certificate || 'require',
        tls_client_name: ep.tls?.client_name || '',
        tls_client_name_type: ep.tls?.client_name_type || 'name',
        tls_peer_fingerprint: Array.isArray(ep.tls?.peer_fingerprint) ? ep.tls.peer_fingerprint.join(', ') : '',
        tls_crl_path: ep.tls?.crl_path || '',
        tls_certificate_profile: ep.tls?.certificate_profile || '',
        tls_ns_certificate_type: ep.tls?.ns_certificate_type || '',
        tls_version_min: ep.tls?.version_min || '1.2',
        tls_version_max: ep.tls?.version_max || '',
        tls_cipher: ep.tls?.cipher || '',
        tls_groups: ep.tls?.groups || '',
        tls_remote_certificate_ku: listToStr(ep.tls?.remote_certificate_ku),
        tls_remote_certificate_eku: ep.tls?.remote_certificate_eku || '',
        tls_remote_certificate_tls: ep.tls?.remote_certificate_tls || '',
        control_wrap_type: ep.tls?.control_wrap?.type || '',
        control_wrap_key: listToStr(ep.tls?.control_wrap?.key),
        control_wrap_key_path: ep.tls?.control_wrap?.key_path || '',
        control_wrap_direction: ep.tls?.control_wrap?.direction || '',
        control_wrap_force_cookie: ep.tls?.control_wrap?.force_cookie || false,
        cipher: ep.cipher || '',
        data_ciphers: ep.data_ciphers || [],
        _dataCiphersStr: (ep.data_ciphers || []).join(', '),
        data_ciphers_fallback: ep.data_ciphers_fallback || '',
        auth: ep.auth || '',
        mss_fix: ep.mss_fix || 0,
        mss_fix_disabled: ep.mss_fix_disabled || false,
        mss_fix_mode: ep.mss_fix_mode || '',
        replay_window: ep.replay_window || 0,
        replay_window_time: ep.replay_window_time || '',
        push_routes: ep.push?.routes || [],
        _pushRoutesStr: (ep.push?.routes || []).join(', '),
        push_dns: ep.push?.dns || [],
        _pushDnsStr: (ep.push?.dns || []).join(', '),
        push_dns_servers: ep.push?.dns_servers || [],
        push_dns_servers: (ep.push?.dns_servers || []).map(d => ({
          priority: d.priority || 0,
          addresses: d.addresses || [],
          resolve_domains: d.resolve_domains || [],
          dnssec: d.dnssec || '',
          transport: d.transport || '',
          sni: d.sni || '',
        })),
        push_search_domains: ep.push?.search_domains || [],
        _pushSearchDomainsStr: (ep.push?.search_domains || []).join(', '),
        push_dhcp_options: ep.push?.dhcp_options || [],
        _pushDhcpOptionsStr: (ep.push?.dhcp_options || []).join(', '),
        push_redirect_gateway: ep.push?.redirect_gateway || false,
        push_redirect_gateway_flags: ep.push?.redirect_gateway_flags || [],
        _pushRedirectGatewayFlagsStr: (ep.push?.redirect_gateway_flags || []).join(', '),
        push_block_outside_dns: ep.push?.block_outside_dns || false,
        push_ping_interval: ep.push?.ping_interval || '',
        push_ping_restart: ep.push?.ping_restart || '',
        ping_interval: ep.ping_interval || '',
        ping_restart: ep.ping_restart || '',
        renegotiate_interval: ep.renegotiate_interval || '',
        renegotiate_disabled: ep.renegotiate_disabled || false,
        renegotiate_bytes: ep.renegotiate_bytes || 0,
        renegotiate_packets: ep.renegotiate_packets || 0,
        handshake_window: ep.handshake_window || '1m',
      }

    case 'masque-client': {
      const tls = ep.tls || {}
      return {
        ...base,
        system: ep.system || false,
        name: ep.name || '',
        mtu: ep.mtu || 0,
        server: ep.server || '',
        server_port: ep.server_port || 443,
        username: ep.username || '',
        password: ep.password || '',
        path: ep.path || '/',
        version: ep.version || 3,
        disable_version_fallback: ep.disable_version_fallback || false,
        headers: Object.entries(ep.headers || {}).map(([k, v]) => ({ key: k, value: v || '' })),
        advertise_routes: ep.advertise_routes || [],
        _advertiseRoutesStr: (ep.advertise_routes || []).join(', '),
        udp_timeout: ep.udp_timeout || '5m',
        udp_mapping: ep.udp_mapping || '',
        udp_filtering: ep.udp_filtering || '',
        udp_nat_max: ep.udp_nat_max || 0,
        on_demand: ep.on_demand || false,
        idle_timeout: ep.idle_timeout || '',
        keep_alive_period: ep.keep_alive_period || '',
        stream_receive_window: ep.stream_receive_window || '',
        connection_receive_window: ep.connection_receive_window || '',
        max_concurrent_streams: ep.max_concurrent_streams || 0,
        initial_packet_size: ep.initial_packet_size || 0,
        disable_path_mtu_discovery: ep.disable_path_mtu_discovery || false,
        tls_enabled: tls.enabled === true,
        tls_server_name: tls.server_name || '',
        tls_insecure: tls.insecure || false,
        tls_disable_sni: tls.disable_sni || false,
        tls_alpn: Array.isArray(tls.alpn) ? tls.alpn : [],
        _tlsAlpnStr: Array.isArray(tls.alpn) ? tls.alpn.join(', ') : '',
        tls_certificate: Array.isArray(tls.certificate) ? tls.certificate.join('\n') : (tls.certificate || ''),
        tls_certificate_path: tls.certificate_path || '',
        tls_certificate_public_key_sha256: Array.isArray(tls.certificate_public_key_sha256) ? tls.certificate_public_key_sha256.join('\n') : (tls.certificate_public_key_sha256 || ''),
        tls_client_certificate: Array.isArray(tls.client_certificate) ? tls.client_certificate.join('\n') : (tls.client_certificate || ''),
        tls_client_certificate_path: tls.client_certificate_path || '',
        tls_client_key: Array.isArray(tls.client_key) ? tls.client_key.join('\n') : (tls.client_key || ''),
        tls_client_key_path: tls.client_key_path || '',
        tls_version_min: tls.min_version || '',
        tls_version_max: tls.max_version || '',
        detour: ep.detour || '',
        bind_interface: ep.bind_interface || '',
        inet4_bind_address: ep.inet4_bind_address || '',
        inet6_bind_address: ep.inet6_bind_address || '',
        bind_address_no_port: ep.bind_address_no_port || false,
        protect_path: ep.protect_path || '',
        routing_mark: ep.routing_mark || 0,
        reuse_addr: ep.reuse_addr || false,
        netns: ep.netns || '',
        connect_timeout: ep.connect_timeout || '',
        tcp_fast_open: ep.tcp_fast_open || false,
        tcp_multi_path: ep.tcp_multi_path || false,
        disable_tcp_keep_alive: ep.disable_tcp_keep_alive || false,
        tcp_keep_alive: ep.tcp_keep_alive || '',
        tcp_keep_alive_interval: ep.tcp_keep_alive_interval || '',
        udp_fragment: ep.udp_fragment || false,
        domain_resolver: ep.domain_resolver || '',
        network_strategy: ep.network_strategy || '',
        _networkType: Array.isArray(ep.network_type) ? ep.network_type.join(', ') : '',
        _fallbackNetworkType: Array.isArray(ep.fallback_network_type) ? ep.fallback_network_type.join(', ') : '',
        fallback_delay: ep.fallback_delay || '',
      }
    }

    case 'masque-server': {
      const tls = ep.tls || {}
      const versions = Array.isArray(ep.version) ? ep.version : (ep.version ? [ep.version] : [1, 2, 3])
      return {
        ...base,
        system: ep.system || false,
        name: ep.name || '',
        mtu: ep.mtu || 0,
        listen: ep.listen || '0.0.0.0',
        listen_port: ep.listen_port || 0,
        address: Array.isArray(ep.address) ? ep.address : [],
        _addressStr: Array.isArray(ep.address) ? ep.address.join(', ') : '',
        path: ep.path || '/.well-known/masque/ip/{target}/{ipproto}/',
        version: versions,
        users: (ep.users || []).map(u => ({ username: u.username || '', password: u.password || '' })),
        advertise_routes: ep.advertise_routes || [],
        _advertiseRoutesStr: (ep.advertise_routes || []).join(', '),
        udp_timeout: ep.udp_timeout || '5m',
        udp_mapping: ep.udp_mapping || '',
        udp_filtering: ep.udp_filtering || '',
        udp_nat_max: ep.udp_nat_max || 0,
        idle_timeout: ep.idle_timeout || '',
        keep_alive_period: ep.keep_alive_period || '',
        stream_receive_window: ep.stream_receive_window || '',
        connection_receive_window: ep.connection_receive_window || '',
        max_concurrent_streams: ep.max_concurrent_streams || 0,
        initial_packet_size: ep.initial_packet_size || 0,
        disable_path_mtu_discovery: ep.disable_path_mtu_discovery || false,
        tls_enabled: tls.enabled === true,
        tls_server_name: tls.server_name || '',
        tls_alpn: Array.isArray(tls.alpn) ? tls.alpn : [],
        _tlsAlpnStr: Array.isArray(tls.alpn) ? tls.alpn.join(', ') : '',
        tls_certificate: Array.isArray(tls.certificate) ? tls.certificate.join('\n') : (tls.certificate || ''),
        tls_certificate_path: tls.certificate_path || '',
        tls_key: Array.isArray(tls.key) ? tls.key.join('\n') : (tls.key || ''),
        tls_key_path: tls.key_path || '',
        tls_client_authentication: tls.client_authentication || '',
        tls_client_certificate: Array.isArray(tls.client_certificate) ? tls.client_certificate.join('\n') : (tls.client_certificate || ''),
        tls_client_certificate_path: Array.isArray(tls.client_certificate_path) ? tls.client_certificate_path.join('\n') : (tls.client_certificate_path || ''),
        tls_version_min: tls.min_version || '1.2',
        tls_version_max: tls.max_version || '',
        bind_interface: ep.bind_interface || '',
        routing_mark: ep.routing_mark || 0,
        reuse_addr: ep.reuse_addr || false,
        netns: ep.netns || '',
        tcp_fast_open: ep.tcp_fast_open || false,
        tcp_multi_path: ep.tcp_multi_path || false,
        disable_tcp_keep_alive: ep.disable_tcp_keep_alive || false,
        tcp_keep_alive: ep.tcp_keep_alive || '',
        tcp_keep_alive_interval: ep.tcp_keep_alive_interval || '',
        udp_fragment: ep.udp_fragment || false,
        detour: ep.detour || '',
      }
    }

    default:
      return base
  }
}
