// sing-box 端点(Endpoint)通用 build/parse（表单模型 <-> sing-box 配置）
// 端点是 sing-box 1.13+ 的顶层概念，与 inbounds/outbounds 并列
// 支持的类型：wireguard, tailscale, openconnect, openvpn-client, openvpn-server

// 支持的端点类型列表
export const ENDPOINT_TYPES = ['wireguard', 'tailscale', 'openconnect', 'openvpn-client', 'openvpn-server']

// 端点类型显示名
export const ENDPOINT_TYPE_LABELS = {
  wireguard: 'WireGuard',
  tailscale: 'Tailscale',
  openconnect: 'OpenConnect',
  'openvpn-client': 'OpenVPN Client',
  'openvpn-server': 'OpenVPN Server',
}

// 生成短随机后缀
function randSuffix() {
  return Math.random().toString(36).substring(2, 8)
}

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
    _remark: 'Node-' + randSuffix(),
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
        taildrop_directory: '',
        on_demand: false,
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
        system: false,
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
        push_dns_servers: [],
        _pushDnsServersStr: '',
        push_search_domains: [],
        _pushSearchDomainsStr: '',
        push_redirect_gateway: false,
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
      }

    default:
      return base
  }
}

// 表单模型 -> sing-box 配置
export function buildEndpoint(ep) {
  switch (ep.type) {
    case 'wireguard': {
      const out = { type: 'wireguard' }
      if (ep.tag) out.tag = ep.tag
      if (ep.system) out.system = true
      if (ep.name) out.name = ep.name
      if (ep.mtu > 0) out.mtu = ep.mtu
      if (ep.address?.length) out.address = ep.address
      if (ep.private_key) out.private_key = ep.private_key
      if (ep.public_key) out.public_key = ep.public_key
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
          if (p.reserved) peer.reserved = p.reserved.split(',').map(s => parseInt(s.trim())).filter(n => !isNaN(n))
          return peer
        })
      }
      if (ep.workers > 0) out.workers = ep.workers
      if (ep.on_demand) out.on_demand = true
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
      if (ep.ssh_server) out.ssh_server = true
      if (ep.taildrop_directory) out.taildrop_directory = ep.taildrop_directory
      if (ep.on_demand) out.on_demand = true
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
      // TLS
      const tls = {}
      if (ep.tls_insecure) tls.insecure = true
      if (ep.tls_server_name) tls.server_name = ep.tls_server_name
      if (ep.tls_peer_fingerprint) tls.peer_fingerprint = ep.tls_peer_fingerprint.split(',').map(s => s.trim()).filter(Boolean)
      if (ep.tls_system_trust_disabled) tls.system_trust_disabled = true
      if (ep.tls_certificate_authority) tls.certificate_authority = ep.tls_certificate_authority
      if (ep.tls_certificate_authority_path) tls.certificate_authority_path = ep.tls_certificate_authority_path
      if (Object.keys(tls).length) out.tls = tls
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
      if (ep.address?.length) out.address = ep.address
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
      if (Object.keys(tls).length) out.tls = tls
      // Cipher
      if (ep.cipher) out.cipher = ep.cipher
      if (ep.data_ciphers?.length) out.data_ciphers = ep.data_ciphers
      if (ep.data_ciphers_fallback) out.data_ciphers_fallback = ep.data_ciphers_fallback
      if (ep.auth) out.auth = ep.auth
      if (ep.mss_fix > 0) out.mss_fix = ep.mss_fix
      if (ep.mss_fix_disabled) out.mss_fix_disabled = true
      if (ep.mss_fix_mode) out.mss_fix_mode = ep.mss_fix_mode
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
      if (ep.system) out.system = true
      if (ep.name) out.name = ep.name
      return out
    }

    case 'openvpn-server': {
      const out = { type: 'openvpn-server' }
      if (ep.tag) out.tag = ep.tag
      if (ep.system) out.system = true
      if (ep.name) out.name = ep.name
      if (ep.mtu > 0) out.mtu = ep.mtu
      if (ep.mode) out.mode = ep.mode
      if (ep.network) out.network = ep.network
      if (ep.remote) out.remote = ep.remote
      if (ep.remote_port > 0) out.remote_port = ep.remote_port
      if (ep.max_clients > 0) out.max_clients = ep.max_clients
      if (ep.address?.length) out.address = ep.address
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
      if (Object.keys(tls).length) out.tls = tls
      // Cipher
      if (ep.cipher) out.cipher = ep.cipher
      if (ep.data_ciphers?.length) out.data_ciphers = ep.data_ciphers
      if (ep.data_ciphers_fallback) out.data_ciphers_fallback = ep.data_ciphers_fallback
      if (ep.auth) out.auth = ep.auth
      if (ep.mss_fix > 0) out.mss_fix = ep.mss_fix
      if (ep.mss_fix_disabled) out.mss_fix_disabled = true
      if (ep.mss_fix_mode) out.mss_fix_mode = ep.mss_fix_mode
      if (ep.replay_window > 0) out.replay_window = ep.replay_window
      if (ep.replay_window_time) out.replay_window_time = ep.replay_window_time
      // Push
      const push = {}
      if (ep.push_routes?.length) push.routes = ep.push_routes
      if (ep.push_dns?.length) push.dns = ep.push_dns
      if (ep.push_dns_servers?.length) push.dns_servers = ep.push_dns_servers
      if (ep.push_search_domains?.length) push.search_domains = ep.push_search_domains
      if (ep.push_redirect_gateway) push.redirect_gateway = true
      if (ep.push_block_outside_dns) push.block_outside_dns = true
      if (ep.push_ping_interval) push.ping_interval = ep.push_ping_interval
      if (ep.push_ping_restart) push.ping_restart = ep.push_ping_restart
      if (Object.keys(push).length) out.push = push
      // Timing
      if (ep.ping_interval) out.ping_interval = ep.ping_interval
      if (ep.ping_restart) out.ping_restart = ep.ping_restart
      if (ep.renegotiate_interval) out.renegotiate_interval = ep.renegotiate_interval
      if (ep.renegotiate_disabled) out.renegotiate_disabled = true
      if (ep.renegotiate_bytes > 0) out.renegotiate_bytes = ep.renegotiate_bytes
      if (ep.renegotiate_packets > 0) out.renegotiate_packets = ep.renegotiate_packets
      if (ep.handshake_window && ep.handshake_window !== '1m') out.handshake_window = ep.handshake_window
      return out
    }

    default:
      return { type: ep.type, tag: ep.tag }
  }
}

// sing-box 配置 -> 表单模型
export function parseEndpoint(ep) {
  const base = {
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
        public_key: ep.public_key || '',
        listen_port: ep.listen_port || 0,
        peers: (ep.peers || []).map(p => ({
          ...p,
          _allowedIPsStr: Array.isArray(p.allowed_ips) ? p.allowed_ips.join(', ') : (p.allowed_ips || '0.0.0.0/0, ::/0'),
        })),
        workers: ep.workers || 0,
        on_demand: ep.on_demand || false,
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
        ssh_server: ep.ssh_server || false,
        taildrop_directory: ep.taildrop_directory || '',
        on_demand: ep.on_demand || false,
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
        tls_certificate_authority: ep.tls?.certificate_authority || '',
        tls_certificate_authority_path: ep.tls?.certificate_authority_path || '',
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
        system: ep.system || false,
        name: ep.name || '',
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
        _pushDnsServersStr: (ep.push?.dns_servers || []).join(', '),
        push_search_domains: ep.push?.search_domains || [],
        _pushSearchDomainsStr: (ep.push?.search_domains || []).join(', '),
        push_redirect_gateway: ep.push?.redirect_gateway || false,
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

    default:
      return base
  }
}
