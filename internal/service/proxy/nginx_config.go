package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"corefusion/internal/model/entity"
)

// nodeACMEChallengeDir 是**节点上** ACME 挑战文件的落盘目录 ✓。
// 必须与 Agent 侧保持一致 ✗（cmd/agent/acme_webserver.go 的 acmeChallengeDir ✓），
// 且**不能在 /tmp** ✗ —— nginx 服务单元常带 PrivateTmp=yes ✗，放 /tmp 会恒 404 ✗（真机踩过 ✓）。
const nodeACMEChallengeDir = "/var/lib/corefusion-agent/acme-challenge"

// ensureSymlink creates a symlink, removing existing file/symlink first.
func ensureSymlink(path, target string) {
	os.Remove(path)
	os.Symlink(target, path)
}

// nginxUpstreamKey converts a domain/name to a safe upstream block name.
// GenerateConfig assembles an nginx.conf from settings and sites.
func (m *NginxManager) GenerateConfig(ctx context.Context) (string, error) {
	return m.GenerateConfigForServer(ctx, m.serverID)
}

// resolveSourceServerID checks if the given serverID is a mirror node.
// If it is, returns the primary server's ID and true.
// Otherwise, returns the original serverID and false.
func (m *NginxManager) resolveSourceServerID(ctx context.Context, serverID int64) (int64, bool) {
	if m.nodeServerRepo == nil {
		return serverID, false
	}
	server, err := m.nodeServerRepo.GetByID(ctx, serverID)
	if err != nil || server == nil {
		return serverID, false
	}
	if server.NodeType == entity.NodeTypeMirror && server.PartnerServerID > 0 {
		return server.PartnerServerID, true
	}
	return serverID, false
}

// GenerateConfigForServer assembles an nginx.conf from settings and sites for a specific server.
// If the target server is a mirror node, sites and settings are fetched from
// the primary server's data (partner_server_id). This ensures mirrors get the
// same nginx.conf as the primary without needing separate DB records.
// isLocalTarget 判断「**这份配置要落到哪个节点**」是不是本机（面板）✓。
//
// ⚠ 不能用 m.isLocal ✗：它是**每请求被改写**的共享状态 ✓（handlers 的 applyServerID 会按
// URL 里的 node id 调 SetIsLocal ✓），本机请求与远程请求**交错**时会被别的请求改掉 ✗
// → 拿它决定"要不要校验证书文件在不在本地"会时好时坏 ✗
//
//	（实测踩到 ✓：对 node 1 生成配置时 m.isLocal 被别的请求置为 false ✗，
//	 文件校验没生效 ✗，缺文件的证书照样写进配置 ✗）。
//
// 这里改为**按本次调用的 serverID 参数**判断 ✓，与共享状态无关 ✓。
func (m *NginxManager) isLocalTarget(ctx context.Context, serverID int64) bool {
	if serverID == 1 { // NodeServer 表里 id=1 就是 "Local (Panel)" ✓
		return true
	}
	if m.nodeServerRepo == nil {
		return false
	}
	srv, err := m.nodeServerRepo.GetByID(ctx, serverID)
	if err != nil || srv == nil {
		return false
	}
	return srv.IsLocal
}

func (m *NginxManager) GenerateConfigForServer(ctx context.Context, serverID int64) (string, error) {
	// Determine the source server ID for site/settings queries.
	// For mirror nodes, use the primary's server_id.
	sourceServerID, isMirror := m.resolveSourceServerID(ctx, serverID)
	if isMirror {
		log.Printf("[NginxManager] server %d is mirror, using source server %d for sites", serverID, sourceServerID)
	}

	sites, err := m.siteRepo.ListEnabled(ctx, sourceServerID)
	if err != nil {
		return "", err
	}

	s, err := m.GetNginxSettingsForServer(ctx, sourceServerID)
	if err != nil {
		s = DefaultNginxSettings()
	}
	// 全局 IPv6 开关：作用于所有生成的 listen（站点 server / HTTP→HTTPS 跳转 /
	// 默认 server / stream）。站点级 enable_ipv6 可在全局关闭时单独为该站点开启。
	globalIPv6 := s.IPv6

	// 全局 TLS 素材：开启时从库里的 base64 还原到 data/ssl/（生成在保存开关/点按钮时完成，
	// 不在此处触发耗时生成）；远程节点由 Agent 按同一份 base64 还原，保证多节点一致。
	if (s.SslDhparam || s.SslSessionTicket) && m.isLocalTarget(ctx, serverID) {
		if err := WriteGlobalSslFiles(m.dataDir, s.SslDhparamB64, s.SslSessionTicketB64, s.SslSessionTicketBB64); err != nil {
			log.Printf("[NginxManager] 还原本机全局 TLS 文件失败：%v", err)
		}
	}

	var sb strings.Builder

	// Ensure directories
	os.MkdirAll(filepath.Join(m.dataDir, "logs"), 0755)
	os.MkdirAll(filepath.Join(m.dataDir, "runtime"), 0755)
	os.MkdirAll(filepath.Join(m.dataDir, "temp"), 0755)

	// Main block
	sb.WriteString(fmt.Sprintf("worker_processes  %s;\n", s.WorkerProcesses))
	sb.WriteString(fmt.Sprintf("worker_rlimit_nofile %s;\n", s.WorkerRlimitNofile))
	sb.WriteString(fmt.Sprintf("worker_priority %s;\n", s.WorkerPriority))
	sb.WriteString(fmt.Sprintf("worker_shutdown_timeout %s;\n", s.WorkerShutdownTimeout))
	if s.Daemon {
		sb.WriteString("daemon on;\n")
	} else {
		sb.WriteString("daemon off;\n")
	}
	// master_process 强制 on ✓：nginx 由面板/Guardian 托管运行（daemon off ✓），
	// 而**单进程模式（master_process off）在这套自编译 nginx 上处理 reload（SIGHUP）会段错误** ✗
	// （dmesg 固定崩溃地址，2026-10-13 真机复现 ✓）→ 一改配置就崩、healthcheck 反复重启 ✗。
	// 该开关前端从未暴露 ✓（用户改不了 ✓），所以不再读设置里的旧值，直接写死 ✓。
	sb.WriteString("master_process on;\n")
	// Error log - check log output mode (per-node first, then global)
	logOutputMode := "file"
	if m.settingStore != nil {
		// Try per-node key first
		logOutputKey := perNodeNginxSettingsKey(serverID)
		logOutputKey = strings.Replace(logOutputKey, "proxy.nginx_settings", "proxy.log_output_config", 1)
		v, err := m.settingStore.GetByKey(ctx, logOutputKey)
		if err != nil || v == "" {
			// Fallback to global key
			v, err = m.settingStore.GetByKey(ctx, "proxy.log_output_config")
		}
		if err == nil && v != "" {
			var cfg struct {
				Nginx struct {
					Mode string `json:"mode"`
				} `json:"nginx"`
			}
			if json.Unmarshal([]byte(v), &cfg) == nil && cfg.Nginx.Mode != "" {
				logOutputMode = cfg.Nginx.Mode
			}
		}
	}
	switch logOutputMode {
	case "stdout":
		ensureSymlink(filepath.Join(m.dataDir, "logs", "stdout_error.log"), "/dev/stderr")
		if s.ErrorLogLevel != "" {
			sb.WriteString(fmt.Sprintf("error_log logs/stdout_error.log %s;\n", s.ErrorLogLevel))
		} else {
			sb.WriteString("error_log logs/stdout_error.log;\n")
		}
	case "off":
		sb.WriteString("error_log /dev/null;\n")
	default: // "file"
		if s.ErrorLog {
			if s.ErrorLogLevel != "" {
				sb.WriteString(fmt.Sprintf("error_log %s %s;\n", s.ErrorLogPath, s.ErrorLogLevel))
			} else {
				sb.WriteString(fmt.Sprintf("error_log %s;\n", s.ErrorLogPath))
			}
		} else {
			sb.WriteString("error_log /dev/null;\n")
		}
	}
	sb.WriteString("pid runtime/nginx.pid;\n")
	sb.WriteString("\n")

	// Events
	sb.WriteString("events {\n")
	sb.WriteString(fmt.Sprintf("    worker_connections  %s;\n", s.WorkerConnections))
	if s.MultiAccept {
		sb.WriteString("    multi_accept on;\n")
	}
	if s.UseEpoll {
		sb.WriteString("    use epoll;\n")
	}
	sb.WriteString("}\n\n")

	// HTTP
	sb.WriteString("http {\n")
	sb.WriteString("    include mime.types;\n")
	sb.WriteString("    default_type application/octet-stream;\n")

	// 全局 TLS：DH 参数与会话票据密钥（各站点 TLS server 共用 data/ssl/，文件已由
	// GenerateConfigForServer / Agent 在加载前生成）。
	if s.SslDhparam {
		sb.WriteString(fmt.Sprintf("    ssl_dhparam %s;\n", nginxSslPath(sslDhparamName)))
	}
	if s.SslSessionTicket {
		sb.WriteString(fmt.Sprintf("    ssl_session_ticket_key %s;\n", nginxSslPath(sslTicketKeyName)))
		sb.WriteString(fmt.Sprintf("    ssl_session_ticket_key %s;\n", nginxSslPath(sslTicketKeyBName)))
	}

	// Lua module paths — required: the nginx binary has LuaJIT compiled in,
	// without lua_package_path the Lua VM cannot locate modules and nginx may
	// fail to start when any lua_* directive is encountered.
	// Always emit these unconditionally.
	sb.WriteString("    lua_package_path \"proxy/lua/?.lua;proxy/lua/?/init.lua;proxy/lua/waf/?.lua;;\";\n")
	sb.WriteString("    lua_code_cache on;\n")

	// WAF directives (only when WAF is enabled)
	if s.WAFEnabled {
		sb.WriteString("    lua_shared_dict limit 10m;\n")
		sb.WriteString("    lua_shared_dict badGuys 10m;\n")
		sb.WriteString("    lua_need_request_body off;\n")
		sb.WriteString("    init_by_lua_file proxy/lua/waf/init.lua;\n")
		sb.WriteString("    access_by_lua_file proxy/lua/waf/access.lua;\n")
	}

	// Maps
	sb.WriteString("    map $http_x_forwarded_for $clientRealIp {\n")
	sb.WriteString("        \"\" $remote_addr;\n")
	sb.WriteString("        ~^(?P<firstAddr>[0-9\\.\\:a-f\\.\\:A-F\\.\\:]+),?.*$ $firstAddr;\n")
	sb.WriteString("    }\n\n")
	sb.WriteString("    map $http_upgrade $connection_upgrade {\n")
	sb.WriteString("        default upgrade;\n")
	sb.WriteString("        '' close;\n")
	sb.WriteString("    }\n\n")

	// Log format
	logFmt := "    log_format main escape=json '$clientRealIp $remote_addr $proxy_protocol_addr - $remote_user [$time_local] \"$request\" '\n" +
		"    '$status $body_bytes_sent \"$http_referer\" '\n" +
		"    '\"$http_user_agent\" \"$http_x_forwarded_for\" \"$http_host\" \"$http3\" '\n" +
		"    'ups_add: $upstream_addr ups_resp_time: $upstream_response_time '\n" +
		"    'request_time: $request_time ups_status: $upstream_status ';\n"

	switch logOutputMode {
	case "stdout":
		ensureSymlink(filepath.Join(m.dataDir, "logs", "stdout_access.log"), "/dev/stdout")
		sb.WriteString(logFmt)
		sb.WriteString("    access_log logs/stdout_access.log main;\n")
	case "off":
		// no access log
	default: // "file"
		if s.AccessLog {
			sb.WriteString(logFmt)
			sb.WriteString(fmt.Sprintf("    access_log  %s main;\n", s.AccessLogPath))
		}
	}

	// Vhost traffic status (not on Windows)
	if s.VhostTrafficStatus {
		sb.WriteString("    vhost_traffic_status_zone;\n")
	}

	// Basic settings
	sb.WriteString(fmt.Sprintf("    client_header_buffer_size %s;\n", s.ClientHeaderBufferSize))
	sb.WriteString(fmt.Sprintf("    large_client_header_buffers %s;\n", s.LargeClientHeaderBuffers))
	sb.WriteString(fmt.Sprintf("    client_header_timeout %s;\n", s.ClientHeaderTimeout))
	sb.WriteString(fmt.Sprintf("    client_body_timeout %s;\n", s.ClientBodyTimeout))
	sb.WriteString(fmt.Sprintf("    client_max_body_size %s;\n", s.ClientMaxBodySize))
	sb.WriteString(fmt.Sprintf("    client_body_buffer_size %s;\n", s.ClientBodyBufferSize))
	if !s.ServerTokens {
		sb.WriteString("    server_tokens off;\n")
	}
	if s.Sendfile {
		sb.WriteString("    sendfile on;\n")
	}
	if !s.LogNotFound {
		sb.WriteString("    log_not_found off;\n")
	}
	sb.WriteString(fmt.Sprintf("    sendfile_max_chunk %s;\n", s.SendfileMaxChunk))
	if s.TcpNopush {
		sb.WriteString("    tcp_nopush on;\n")
	}
	if s.TcpNodelay {
		sb.WriteString("    tcp_nodelay on;\n")
	}
	if s.DisableSymlinks {
		sb.WriteString("    disable_symlinks off;\n")
	}
	if s.ProxyStore {
		sb.WriteString("    proxy_store off;\n")
	}
	sb.WriteString("\n")

	// HTTP/2
	sb.WriteString(fmt.Sprintf("    http2_chunk_size %s;\n", s.Http2ChunkSize))
	sb.WriteString(fmt.Sprintf("    http2_body_preread_size %s;\n", s.Http2BodyPrereadSize))
	sb.WriteString(fmt.Sprintf("    http2_max_concurrent_streams %s;\n", s.Http2MaxConcurrentStreams))
	sb.WriteString(fmt.Sprintf("    http2_recv_buffer_size %s;\n", s.Http2RecvBufferSize))
	sb.WriteString("\n")

	// HTTP/3 (not on Windows)
	if s.Http3 {
		sb.WriteString("    quic_retry on;\n")
		sb.WriteString("    http3 on;\n")
		sb.WriteString("    http3_hq on;\n")
		sb.WriteString(fmt.Sprintf("    http3_max_concurrent_streams %s;\n", s.Http3MaxConcurrentStreams))
		sb.WriteString(fmt.Sprintf("    http3_stream_buffer_size %s;\n", s.Http3StreamBufferSize))
		sb.WriteString(fmt.Sprintf("    quic_active_connection_id_limit %s;\n", s.QuicActiveConnectionIdLimit))
		sb.WriteString("    quic_gso on;\n")
	}
	sb.WriteString("\n")

	// Performance
	sb.WriteString(fmt.Sprintf("    keepalive_requests %s;\n", s.KeepaliveRequests))
	sb.WriteString(fmt.Sprintf("    keepalive_timeout %s;\n", s.KeepaliveTimeout))
	if s.Aio {
		sb.WriteString("    aio threads;\n")
		if s.AioWrite {
			sb.WriteString("    aio_write on;\n")
		}
	}
	sb.WriteString(fmt.Sprintf("    types_hash_max_size %s;\n", s.TypesHashMaxSize))
	sb.WriteString(fmt.Sprintf("    server_names_hash_max_size %s;\n", s.ServerNamesHashMaxSize))
	sb.WriteString(fmt.Sprintf("    server_names_hash_bucket_size %s;\n", s.ServerNamesHashBucketSize))
	sb.WriteString(fmt.Sprintf("    proxy_headers_hash_max_size %s;\n", s.ProxyHeadersHashMaxSize))
	sb.WriteString(fmt.Sprintf("    proxy_headers_hash_bucket_size %s;\n", s.ProxyHeadersHashBucketSize))
	sb.WriteString(fmt.Sprintf("    variables_hash_bucket_size %s;\n", s.VariablesHashBucketSize))
	sb.WriteString(fmt.Sprintf("    variables_hash_max_size %s;\n", s.VariablesHashMaxSize))
	if s.UnderscoresInHeaders {
		sb.WriteString("    underscores_in_headers on;\n")
	}
	if s.IgnoreInvalidHeaders {
		sb.WriteString("    ignore_invalid_headers on;\n")
	}
	if s.ResetTimedoutConnection {
		sb.WriteString("    reset_timedout_connection on;\n")
	}
	sb.WriteString(fmt.Sprintf("    send_timeout %s;\n", s.SendTimeout))
	// limit_conn_zone / limit_conn are skipped when WAF is enabled —
	// the WAF Lua module handles rate limiting and concurrency control.
	if !s.WAFEnabled {
		if s.LimitConnZone != "" {
			sb.WriteString(fmt.Sprintf("    limit_conn_zone %s;\n", s.LimitConnZone))
		}
		if s.LimitConn != "" {
			sb.WriteString(fmt.Sprintf("    limit_conn %s;\n", s.LimitConn))
		}
	}
	if s.Charset != "" {
		sb.WriteString(fmt.Sprintf("    charset '%s';\n", s.Charset))
	}
	sb.WriteString("\n")

	// Proxy settings
	if s.ProxyIgnoreClientAbort {
		sb.WriteString("    proxy_ignore_client_abort on;\n")
	}
	sb.WriteString(fmt.Sprintf("    proxy_connect_timeout %s;\n", s.ProxyConnectTimeout))
	sb.WriteString(fmt.Sprintf("    proxy_read_timeout %s;\n", s.ProxyReadTimeout))
	sb.WriteString(fmt.Sprintf("    proxy_send_timeout %s;\n", s.ProxySendTimeout))
	sb.WriteString(fmt.Sprintf("    proxy_buffer_size %s;\n", s.ProxyBufferSize))
	sb.WriteString(fmt.Sprintf("    proxy_buffers %s;\n", s.ProxyBuffers))
	sb.WriteString(fmt.Sprintf("    proxy_busy_buffers_size %s;\n", s.ProxyBusyBuffersSize))
	sb.WriteString("\n")

	// FastCGI settings
	sb.WriteString(fmt.Sprintf("    fastcgi_connect_timeout %s;\n", s.FastcgiConnectTimeout))
	sb.WriteString(fmt.Sprintf("    fastcgi_send_timeout %s;\n", s.FastcgiSendTimeout))
	sb.WriteString(fmt.Sprintf("    fastcgi_read_timeout %s;\n", s.FastcgiReadTimeout))
	sb.WriteString(fmt.Sprintf("    fastcgi_buffer_size %s;\n", s.FastcgiBufferSize))
	sb.WriteString(fmt.Sprintf("    fastcgi_buffers %s;\n", s.FastcgiBuffers))
	sb.WriteString(fmt.Sprintf("    fastcgi_busy_buffers_size %s;\n", s.FastcgiBusyBuffersSize))
	sb.WriteString(fmt.Sprintf("    fastcgi_temp_file_write_size %s;\n", s.FastcgiTempFileWriteSize))
	if s.FastcgiInterceptErrors {
		sb.WriteString("    fastcgi_intercept_errors on;\n")
	}
	if s.FastcgiIgnoreClientAbort {
		sb.WriteString("    fastcgi_ignore_client_abort on;\n")
	}
	sb.WriteString("\n")

	// Gzip
	if s.Gzip {
		sb.WriteString("    gzip on;\n")
		sb.WriteString(fmt.Sprintf("    gzip_http_version %s;\n", s.GzipHttpVersion))
		sb.WriteString(fmt.Sprintf("    gzip_buffers %s;\n", s.GzipBuffers))
		if s.GzipDisable != "" {
			sb.WriteString(fmt.Sprintf("    gzip_disable \"%s\";\n", s.GzipDisable))
		}
		sb.WriteString(fmt.Sprintf("    gzip_proxied %s;\n", s.GzipProxied))
		sb.WriteString(fmt.Sprintf("    gzip_min_length %s;\n", s.GzipMinLength))
		sb.WriteString(fmt.Sprintf("    gzip_comp_level %s;\n", s.GzipCompLevel))
		if s.GzipTypes != "" {
			sb.WriteString(fmt.Sprintf("    gzip_types %s;\n", s.GzipTypes))
		}
		if s.GzipVary {
			sb.WriteString("    gzip_vary on;\n")
		}
	}
	sb.WriteString("\n")

	// Brotli
	if s.Brotli {
		sb.WriteString("    brotli on;\n")
		sb.WriteString(fmt.Sprintf("    brotli_comp_level %s;\n", s.BrotliCompLevel))
		sb.WriteString(fmt.Sprintf("    brotli_buffers %s;\n", s.BrotliBuffers))
		sb.WriteString(fmt.Sprintf("    brotli_min_length %s;\n", s.BrotliMinLength))
		if s.BrotliStatic {
			sb.WriteString("    brotli_static on;\n")
		}
		sb.WriteString(fmt.Sprintf("    brotli_window %s;\n", s.BrotliWindow))
		if s.BrotliTypes != "" {
			sb.WriteString(fmt.Sprintf("    brotli_types %s;\n", s.BrotliTypes))
		}
	}
	sb.WriteString("\n")

	// Custom global config
	if s.CustomConfig != "" {
		for _, line := range strings.Split(strings.TrimSpace(s.CustomConfig), "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				if !strings.HasSuffix(trimmed, ";") && !strings.HasSuffix(trimmed, "{") && !strings.HasSuffix(trimmed, "}") {
					trimmed += ";"
				}
				sb.WriteString("    " + trimmed + "\n")
			}
		}
		sb.WriteString("\n")
	}

	// Separate SNI, L4, Reality, and HTTP sites
	var sniSites []entity.NginxSite
	var l4Sites []entity.NginxSite
	var realitySites []entity.NginxSite
	var httpSites []entity.NginxSite
	for _, site := range sites {
		switch site.Template {
		case "sni":
			sniSites = append(sniSites, site)
		case "l4":
			l4Sites = append(l4Sites, site)
		case "reality":
			realitySites = append(realitySites, site)
		default:
			// portal_forward / subscription_forward / panel_forward：仅面板节点有效 ✓（远程节点没有
			// 面板/门户进程 ✗）。派生字段（upstream/path）统一走 fillForwardDerivedFields ✓。
			//
			// ⚠ 用按参数判断的 isLocalTarget ✓，**不是**共享的 m.isLocal ✗
			//   （m.isLocal 每请求被改写 ✓ → 本机/远程请求交错时会误跳过本机站点 ✗，
			//    表现为门户转发配置时有时无、重置或换端口访问又正常 ✗）。
			if site.Template == "portal_forward" || site.Template == "subscription_forward" || site.Template == "panel_forward" {
				if !m.isLocalTarget(ctx, serverID) {
					continue
				}
				m.fillForwardDerivedFields(ctx, &site)
			}
			httpSites = append(httpSites, site)
		}
	}

	// Collect upstreams (HTTP sites only)
	upstreamSeen := make(map[string]bool)
	for _, site := range httpSites {
		if site.Upstream == "" || site.Template == "static" {
			continue
		}
		upstreams := strings.Split(site.Upstream, ",")
		upstreamKey := strings.ReplaceAll(site.Upstream, ".", "_")
		upstreamKey = strings.ReplaceAll(upstreamKey, ":", "_")
		upstreamKey = strings.ReplaceAll(upstreamKey, ",", "_")
		if upstreamSeen[upstreamKey] {
			continue
		}
		upstreamSeen[upstreamKey] = true
		sb.WriteString(fmt.Sprintf("    upstream backend_%s {\n", upstreamKey))
		for _, up := range upstreams {
			up = strings.TrimSpace(up)
			if up != "" {
				sb.WriteString(fmt.Sprintf("        server %s;\n", up))
			}
		}
		sb.WriteString("    }\n\n")
	}

	// Group by domain (HTTP sites only)
	domainSites := make(map[string][]entity.NginxSite)
	for _, site := range httpSites {
		domainSites[site.Domain] = append(domainSites[site.Domain], site)
	}

	// 80 端口是否开启 ✓：存在 listen 80 的**启用**站点 ✓
	//（ListenPort="80" ✓ / 非证书站点未写端口（默认 80 ✓））。
	// 没开 80 就**不生成** HTTP→HTTPS 跳转 ✓（跳转 server 抢不到 :80、跳了也白跳 ✗）。
	port80Open := false
	for _, s := range httpSites {
		p := s.ListenPort
		if p == "" && s.CertPath == "" && s.KeyPath == "" && s.ListenSocket == "" {
			p = "80"
		}
		if p == "80" {
			port80Open = true
			break
		}
	}

	for domain, siteList := range domainSites {
		// HTTP → HTTPS 自动跳转 ✓ —— 三个条件**同时满足**才生成 ✓：
		//   ① 域名组有 TLS 证书 ✓；② 节点开了 80 端口 ✓；③ 站点开关打开 ✓（nil=开 ✓）。
		// 生成独立的 `server { listen 80; ... }` 块 ✓：ACME 挑战留在 :80 ✓，其余 301 到 https ✓。
		grpHasTLS := false
		redirectEnabled := false
		grpIPv6 := globalIPv6
		tlsPort := "443"
		for _, s := range siteList {
			if s.CertPath != "" && s.KeyPath != "" {
				grpHasTLS = true
				if s.ListenPort != "" && s.ListenPort != "80" {
					tlsPort = s.ListenPort
				}
			}
			if s.RedirectHTTPS == nil || *s.RedirectHTTPS {
				redirectEnabled = true
			}
			if s.EnableIpv6 {
				grpIPv6 = true
			}
		}
		if grpHasTLS && port80Open && redirectEnabled && domain != "" {
			target := "https://" + domain
			if tlsPort != "443" {
				target = fmt.Sprintf("https://%s:%s", domain, tlsPort)
			}
			sb.WriteString("    # HTTP → HTTPS redirect (auto-generated; per-site switch)\n")
			sb.WriteString("    server {\n")
			sb.WriteString("        listen 80;\n")
			if grpIPv6 {
				sb.WriteString("        listen [::]:80;\n")
			}
			sb.WriteString(fmt.Sprintf("        server_name %s;\n", domain))
			if !m.isLocal {
				sb.WriteString("        # ACME HTTP-01 challenge support (node-local, served from disk)\n")
				sb.WriteString("        location ^~ /.well-known/acme-challenge/ {\n")
				sb.WriteString(fmt.Sprintf("            alias %s/.well-known/acme-challenge/;\n", nodeACMEChallengeDir))
				sb.WriteString("        }\n")
			} else {
				sb.WriteString("        # ACME HTTP-01 challenge support\n")
				sb.WriteString("        location ^~ /.well-known/acme-challenge/ {\n")
				sb.WriteString(fmt.Sprintf("            proxy_pass http://%s;\n", m.panelAddr(ctx)))
				sb.WriteString("            proxy_set_header Host $host;\n")
				sb.WriteString("            proxy_set_header X-Real-IP $remote_addr;\n")
				sb.WriteString("            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n")
				sb.WriteString("            proxy_set_header X-Forwarded-Proto $scheme;\n")
				sb.WriteString("        }\n")
			}
			sb.WriteString(fmt.Sprintf("        return 301 %s$request_uri;\n", target))
			sb.WriteString("    }\n\n")
		}

		sb.WriteString(fmt.Sprintf("    # %s\n", domain))
		sb.WriteString("    server {\n")

		// Collect settings from all sites for this domain
		hasTLS := false
		hasSocket := false
		hasProxyProtocol := false
		enableIpv6 := globalIPv6
		listenPort := ""
		var tlsCert, tlsKey string
		var tlsSite *entity.NginxSite
		for _, site := range siteList {
			if site.ListenSocket != "" && !hasSocket {
				sb.WriteString(fmt.Sprintf("        listen unix:%s;\n", site.ListenSocket))
				hasSocket = true
			}
			if site.ListenPort != "" && listenPort == "" {
				listenPort = site.ListenPort
			}
			if site.CertPath != "" && site.KeyPath != "" {
				hasTLS = true
				tlsCert = site.CertPath
				tlsKey = site.KeyPath
				if tlsSite == nil {
					tlsSite = &site
				}
			}
			if site.ProxyProtocol {
				hasProxyProtocol = true
			}
			if site.EnableIpv6 {
				enableIpv6 = true
			}
		}

		// Default port: 443 for TLS, 80 for non-TLS
		if listenPort == "" || (hasTLS && listenPort == "80") {
			if hasTLS {
				listenPort = "443"
			} else {
				listenPort = "80"
			}
		}

		// TLS options from site
		enableHttp2 := true
		enableQuic := false
		enableEarlyData := false
		enableStapling := false
		sslProtocols := "TLSv1.2 TLSv1.3"
		sslCiphers := ""
		sslEcdhCurve := ""
		if tlsSite != nil {
			enableHttp2 = tlsSite.EnableHttp2
			enableQuic = tlsSite.EnableQuic
			enableEarlyData = tlsSite.EnableEarlyData
			enableStapling = tlsSite.EnableStapling
			if tlsSite.SslProtocols != "" {
				sslProtocols = tlsSite.SslProtocols
			}
			sslCiphers = tlsSite.SslCiphers
			sslEcdhCurve = tlsSite.SslEcdhCurve
		}

		// Listen directives - IPv4 and IPv6 separately
		if enableQuic && hasTLS {
			sb.WriteString(fmt.Sprintf("        listen %s quic;\n", listenPort))
			if enableIpv6 {
				sb.WriteString(fmt.Sprintf("        listen [::]:%s quic;\n", listenPort))
			}
		}
		if hasTLS {
			// HTTPS listen
			if hasProxyProtocol {
				sb.WriteString(fmt.Sprintf("        listen %s ssl proxy_protocol;\n", listenPort))
				if enableIpv6 {
					sb.WriteString(fmt.Sprintf("        listen [::]:%s ssl proxy_protocol;\n", listenPort))
				}
			} else {
				sb.WriteString(fmt.Sprintf("        listen %s ssl;\n", listenPort))
				if enableIpv6 {
					sb.WriteString(fmt.Sprintf("        listen [::]:%s ssl;\n", listenPort))
				}
			}
			if enableHttp2 {
				sb.WriteString("        http2 on;\n")
			}
			if domain == "" {
				sb.WriteString("        server_name _;\n")
			} else {
				sb.WriteString(fmt.Sprintf("        server_name %s;\n", domain))
			}
			// ✅ 双证书 ✓：nginx 允许在同一个 server 块里写**多组** ssl_certificate ✓，
			// 它会按客户端支持能力自动挑 ECC / RSA ✓（以前只写一组 ✗ → 另一张白签 ✗）。
			dual := true // 缺省开 ✓
			if tlsSite != nil && tlsSite.DualCert != nil {
				dual = *tlsSite.DualCert
			}
			var checker certPairChecker
			// ⚠ 用按参数判断的 isLocalTarget ✓，**不是**共享的 m.isLocal ✗（见其注释 ✓）
			if m.isLocalTarget(ctx, serverID) {
				checker = localCertPairChecker(m.dataDir)
			}
			for _, cp := range collectCertPairs(ctx, m.certRepo, domain, tlsCert, tlsKey, dual, checker) {
				sb.WriteString(fmt.Sprintf("        ssl_certificate ../%s;\n", stripDataDirPrefix(cp.Cert, m.dataDir)))
				sb.WriteString(fmt.Sprintf("        ssl_certificate_key ../%s;\n", stripDataDirPrefix(cp.Key, m.dataDir)))
			}
			sb.WriteString(fmt.Sprintf("        ssl_protocols %s;\n", sslProtocols))
			if sslCiphers != "" {
				sb.WriteString(fmt.Sprintf("        ssl_ciphers %s;\n", sslCiphers))
			}
			if sslEcdhCurve != "" {
				sb.WriteString(fmt.Sprintf("        ssl_ecdh_curve %s;\n", sslEcdhCurve))
			}
			if enableEarlyData {
				sb.WriteString("        ssl_early_data on;\n")
				sb.WriteString("        proxy_set_header Early-Data $ssl_early_data;\n")
			}
			if enableStapling {
				sb.WriteString("        ssl_stapling on;\n")
				sb.WriteString("        ssl_stapling_verify on;\n")
				// OCSP stapling 校验需要可信链（fullchain），见 trustChainRel。
				sb.WriteString(fmt.Sprintf("        ssl_trusted_certificate ../%s;\n",
					stripDataDirPrefix(trustChainRel(tlsCert), m.dataDir)))
				// resolver 可选 ✓：解析 CA 的 OCSP 地址用 ✓。**留空 = 不写该指令** ✓，
				// 即关闭本功能、改用系统 resolver ✓（缺省值是 8.8.8.8 1.1.1.1，见 DefaultNginxSettings ✓）。
				if resolver := strings.TrimSpace(s.SslResolver); resolver != "" {
					sb.WriteString(fmt.Sprintf("        resolver %s;\n", resolver))
					sb.WriteString("        resolver_timeout 10s;\n")
				}
			}
			if enableQuic {
				sb.WriteString("        add_header Alt-Svc 'h3=\":443\"; ma=86400, h3-29=\":443\"; ma=86400' always;\n")
			}
		} else {
			// HTTP listen
			if hasProxyProtocol {
				sb.WriteString(fmt.Sprintf("        listen %s proxy_protocol;\n", listenPort))
				if enableIpv6 {
					sb.WriteString(fmt.Sprintf("        listen [::]:%s proxy_protocol;\n", listenPort))
				}
			} else {
				sb.WriteString(fmt.Sprintf("        listen %s;\n", listenPort))
				if enableIpv6 {
					sb.WriteString(fmt.Sprintf("        listen [::]:%s;\n", listenPort))
				}
			}
			if domain == "" {
				sb.WriteString("        server_name _;\n")
			} else {
				sb.WriteString(fmt.Sprintf("        server_name %s;\n", domain))
			}
		}

		// PROXY Protocol real_ip settings
		if hasProxyProtocol {
			sb.WriteString("        set_real_ip_from 127.0.0.1;\n")
			if enableIpv6 {
				sb.WriteString("        set_real_ip_from ::1;\n")
			}
			sb.WriteString("        real_ip_header proxy_protocol;\n")
		}

		// Server-level custom directives (between server { and location)
		for _, site := range siteList {
			if site.ServerConfig != "" {
				for _, line := range strings.Split(strings.TrimSpace(site.ServerConfig), "\n") {
					trimmed := strings.TrimSpace(line)
					if trimmed != "" {
						if !strings.HasSuffix(trimmed, ";") && !strings.HasSuffix(trimmed, "{") && !strings.HasSuffix(trimmed, "}") {
							trimmed += ";"
						}
						sb.WriteString("        " + trimmed + "\n")
					}
				}
				break
			}
		}

		// ACME HTTP-01 challenge support
		//
		// 分两种，必须区分 ✗：
		//   - **本机（面板）**：面板自己驱动签发 ✓ → 挑战**代理回面板** ✓（面板内存里就有挑战记录 ✓）；
		//   - **远程节点**：签发是**节点上的 Agent 在本机做**的 ✓ → 它的挑战记录在 Agent 进程内存里，
		//     面板并不知道 ✗ → 所以**不能代理回面板** ✗，必须直接服务**节点本地的挑战目录** ✓
		//     （Agent 会把挑战文件写在那里 ✓，见 cmd/agent/acme_webserver.go ✓）。
		if !m.isLocal {
			sb.WriteString("        # ACME HTTP-01 challenge support (node-local, served from disk)\n")
			sb.WriteString("        location ^~ /.well-known/acme-challenge/ {\n")
			sb.WriteString(fmt.Sprintf("            alias %s/.well-known/acme-challenge/;\n", nodeACMEChallengeDir))
			sb.WriteString("        }\n")
		}
		if m.isLocal {
			panelAddr := m.panelAddr(ctx)
			sb.WriteString("        # ACME HTTP-01 challenge support\n")
			sb.WriteString("        location /.well-known/acme-challenge/ {\n")
			sb.WriteString(fmt.Sprintf("            proxy_pass http://%s;\n", panelAddr))
			sb.WriteString("            proxy_set_header Host $host;\n")
			sb.WriteString("            proxy_set_header X-Real-IP $remote_addr;\n")
			sb.WriteString("            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n")
			sb.WriteString("            proxy_set_header X-Forwarded-Proto $scheme;\n")
			sb.WriteString("        }\n")
		}

		// Locations
		for _, site := range siteList {
			path := site.Path
			if path == "" {
				path = "/"
			}
			backendPath := site.BackendPath
			if backendPath == "" {
				backendPath = "/"
			}
			upstreamKey := strings.ReplaceAll(site.Upstream, ".", "_")
			upstreamKey = strings.ReplaceAll(upstreamKey, ":", "_")
			upstreamKey = strings.ReplaceAll(upstreamKey, ",", "_")

			// Per-location WAF bypass: set $waf_enable off; when WafEnabled is false
			wafBypass := ""
			if site.WafEnabled != nil && !*site.WafEnabled {
				wafBypass = "            set $waf_enable off;\n"
			}

			// gRPC uses a different location block syntax
			// 回源（nginx → 本地内核）是否走 TLS ✓：由站点 upstream_tls 决定 ✓
			grpcScheme := "grpc"
			upstreamScheme := "http"
			if site.UpstreamTLS {
				grpcScheme = "grpcs"
				upstreamScheme = "https"
			}
			// 回源 SNI ✓：优先 sni_target ✓，否则退回域名 ✓
			upstreamSNI := strings.TrimSpace(site.SNITarget)
			if upstreamSNI == "" {
				upstreamSNI = strings.TrimSpace(site.Domain)
			}

			if site.Template == "grpc" {
				sb.WriteString(fmt.Sprintf("        location %s {\n", path))
				sb.WriteString(wafBypass)
				sb.WriteString("            client_max_body_size 0;\n")
				sb.WriteString("            keepalive_requests 4294967296;\n")
				sb.WriteString("            client_body_timeout 1h;\n")
				sb.WriteString("            send_timeout 1h;\n")
				sb.WriteString("            lingering_close always;\n")
				sb.WriteString("            grpc_set_header X-Real-IP $clientRealIp;\n")
				sb.WriteString("            grpc_read_timeout 1h;\n")
				sb.WriteString("            grpc_send_timeout 1h;\n")
				// ⚠ 原来写死 `grpc://`（h2c 明文）✗：入站自带 TLS 时必须用 `grpcs://` ✗，
				// 并补 SNI 与"不校验回源证书"（回源是本机内核 ✓，多为自签/内部签发 ✓）。
				if site.UpstreamTLS {
					if upstreamSNI != "" {
						sb.WriteString("            grpc_ssl_server_name on;\n")
						sb.WriteString(fmt.Sprintf("            grpc_ssl_name %s;\n", upstreamSNI))
					}
					sb.WriteString("            grpc_ssl_verify off;\n")
				}
				sb.WriteString(fmt.Sprintf("            grpc_pass %s://backend_%s;\n", grpcScheme, upstreamKey))
				// Location-level directives
				if site.LocationConfig != "" {
					for _, line := range strings.Split(strings.TrimSpace(site.LocationConfig), "\n") {
						trimmed := strings.TrimSpace(line)
						if trimmed != "" {
							if !strings.HasSuffix(trimmed, ";") && !strings.HasSuffix(trimmed, "{") && !strings.HasSuffix(trimmed, "}") {
								trimmed += ";"
							}
							sb.WriteString("            " + trimmed + "\n")
						}
					}
				}
				sb.WriteString("        }\n")
				continue
			}

			sb.WriteString(fmt.Sprintf("        location %s {\n", path))
			sb.WriteString(wafBypass)

			switch site.Template {
			case "static":
				rootDir := strings.TrimSpace(site.Upstream)
				if rootDir == "" {
					rootDir = "/var/www"
				}
				sb.WriteString(fmt.Sprintf("            root %s;\n", rootDir))
				sb.WriteString("            index index.html index.htm;\n")
				sb.WriteString("            try_files $uri $uri/ =404;\n")
			case "websocket", "xhttp", "httpupgrade":
				// If backend_path is root ("/"), add trailing slash to strip the location prefix
				// If backend_path has a specific path, no trailing slash to preserve the full URI
				if backendPath == "/" {
					sb.WriteString(fmt.Sprintf("            proxy_pass %s://backend_%s/;\n", upstreamScheme, upstreamKey))
				} else {
					sb.WriteString(fmt.Sprintf("            proxy_pass %s://backend_%s;\n", upstreamScheme, upstreamKey))
				}
				sb.WriteString("            proxy_http_version 1.1;\n")
				sb.WriteString("            proxy_set_header Upgrade $http_upgrade;\n")
				sb.WriteString("            proxy_set_header Connection $connection_upgrade;\n")
				sb.WriteString("            proxy_set_header Host $host;\n")
				sb.WriteString("            proxy_set_header X-Forwarded-Proto $scheme;\n")
				sb.WriteString("            proxy_set_header X-Real-IP $remote_addr;\n")
				sb.WriteString("            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n")
				sb.WriteString("            proxy_set_header X-NginX-Proxy true;\n")
				if site.UpstreamTLS {
					writeProxySSLUpstream(&sb, upstreamSNI)
				}
				sb.WriteString("            proxy_ssl_session_reuse off;\n")
				sb.WriteString("            proxy_redirect off;\n")
			case "panel_forward":
				// 面板转发（专用模板）：把面板主端口整域名直通。必须按 SSE/流式响应配置 ——
				// 面板的实时日志（/api/v1/log-manage/*/log-stream 等）是长连接事件流，
				// 若沿用默认 location（缓冲 + 60s 读超时）会被 nginx 断开/缓冲住 ✗。
				sb.WriteString(fmt.Sprintf("            proxy_pass %s://backend_%s;\n", upstreamScheme, upstreamKey))
				sb.WriteString("            proxy_http_version 1.1;\n")
				sb.WriteString("            proxy_set_header Accept-Encoding \"\";\n")
				if site.UpstreamTLS {
					writeProxySSLUpstream(&sb, upstreamSNI)
				}
				sb.WriteString("            proxy_set_header Host $host;\n")
				sb.WriteString("            proxy_set_header X-Real-IP $remote_addr;\n")
				sb.WriteString("            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n")
				sb.WriteString("            proxy_set_header X-Forwarded-Proto $scheme;\n")
				sb.WriteString("            proxy_set_header Connection \"\";\n")
				sb.WriteString("            proxy_buffering off;\n")
				sb.WriteString("            proxy_cache off;\n")
				sb.WriteString("            proxy_read_timeout 3600s;\n")
				sb.WriteString("            proxy_send_timeout 3600s;\n")
			default:
				sb.WriteString(fmt.Sprintf("            proxy_pass %s://backend_%s;\n", upstreamScheme, upstreamKey))
				sb.WriteString("            proxy_http_version 1.1;\n")
				sb.WriteString("            proxy_set_header Accept-Encoding \"\";\n")
				if site.UpstreamTLS {
					writeProxySSLUpstream(&sb, upstreamSNI)
				}
				sb.WriteString("            proxy_set_header Host $host;\n")
				sb.WriteString("            proxy_set_header X-Real-IP $remote_addr;\n")
				sb.WriteString("            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n")
				sb.WriteString("            proxy_set_header X-Forwarded-Proto $scheme;\n")
			}

			// Location-level directives (auto-add semicolons)
			if site.LocationConfig != "" {
				for _, line := range strings.Split(strings.TrimSpace(site.LocationConfig), "\n") {
					trimmed := strings.TrimSpace(line)
					if trimmed != "" {
						if !strings.HasSuffix(trimmed, ";") && !strings.HasSuffix(trimmed, "{") && !strings.HasSuffix(trimmed, "}") {
							trimmed += ";"
						}
						sb.WriteString("            " + trimmed + "\n")
					}
				}
			}

			sb.WriteString("        }\n")
		}

		sb.WriteString("    }\n\n")
	}

	// Reality server blocks: forced TLS, HTTP/2, PROXY Protocol, no domain required
	for _, rs := range realitySites {
		port := rs.ListenPort
		if port == "" {
			port = "9443"
		}
		sb.WriteString(fmt.Sprintf("    # Reality :%s\n", port))
		sb.WriteString("    server {\n")
		sb.WriteString(fmt.Sprintf("        listen 127.0.0.1:%s ssl proxy_protocol;\n", port))
		if rs.EnableIpv6 || globalIPv6 {
			sb.WriteString(fmt.Sprintf("        listen [::1]:%s ssl proxy_protocol;\n", port))
		}
		sb.WriteString("        http2 on;\n")
		if rs.Domain == "" {
			sb.WriteString("        server_name _;\n")
		} else {
			sb.WriteString(fmt.Sprintf("        server_name %s;\n", rs.Domain))
		}
		if rs.CertPath != "" && rs.KeyPath != "" {
			sb.WriteString(fmt.Sprintf("        ssl_certificate ../%s;\n", stripDataDirPrefix(rs.CertPath, m.dataDir)))
			sb.WriteString(fmt.Sprintf("        ssl_certificate_key ../%s;\n", stripDataDirPrefix(rs.KeyPath, m.dataDir)))
		}
		sslProtocols := rs.SslProtocols
		if sslProtocols == "" {
			sslProtocols = "TLSv1.2 TLSv1.3"
		}
		sb.WriteString(fmt.Sprintf("        ssl_protocols %s;\n", sslProtocols))
		if rs.SslCiphers != "" {
			sb.WriteString(fmt.Sprintf("        ssl_ciphers %s;\n", rs.SslCiphers))
		}
		if rs.SslEcdhCurve != "" {
			sb.WriteString(fmt.Sprintf("        ssl_ecdh_curve %s;\n", rs.SslEcdhCurve))
		}
		// PROXY Protocol real_ip settings
		sb.WriteString("        set_real_ip_from 127.0.0.1;\n")
		if rs.EnableIpv6 || globalIPv6 {
			sb.WriteString("        set_real_ip_from ::1;\n")
		}
		sb.WriteString("        real_ip_header proxy_protocol;\n")
		// Server-level custom config
		if rs.ServerConfig != "" {
			for _, line := range strings.Split(strings.TrimSpace(rs.ServerConfig), "\n") {
				trimmed := strings.TrimSpace(line)
				if trimmed != "" {
					if !strings.HasSuffix(trimmed, ";") && !strings.HasSuffix(trimmed, "{") && !strings.HasSuffix(trimmed, "}") {
						trimmed += ";"
					}
					sb.WriteString("        " + trimmed + "\n")
				}
			}
		}
		// Default location
		sb.WriteString("        location = / {\n")
		sb.WriteString("            default_type 'text/html';\n")
		sb.WriteString("            return 200 'Success';\n")
		sb.WriteString("        }\n")
		sb.WriteString("    }\n\n")
	}

	// Default server if no HTTP sites (ensures :80 always responds)
	if len(httpSites) == 0 {
		sb.WriteString("    server {\n")
		sb.WriteString("        listen 80;\n")
		if globalIPv6 {
			sb.WriteString("        listen [::]:80;\n")
		}
		sb.WriteString("        location / {\n")
		sb.WriteString("            return 200 \"CoreFusion Proxy - OK\";\n")
		sb.WriteString("        }\n")
		sb.WriteString("    }\n")
	}

	sb.WriteString("}\n")

	// Stream block — SNI has toggle, L4 auto-enabled when rules exist
	needStream := (s.StreamSNIEnabled && len(sniSites) > 0) || len(l4Sites) > 0
	if needStream {
		sb.WriteString("\nstream {\n")
		// Lua module paths in stream context too — needed so resty.core can be
		// found when lua-nginx-module initializes in the stream block.
		sb.WriteString("    lua_package_path \"proxy/lua/?.lua;proxy/lua/?/init.lua;proxy/lua/waf/?.lua;;\";\n")

		// Stream log format
		logFmt := s.StreamLogFormat
		if logFmt == "" {
			logFmt = "basic"
		}
		logPath := s.StreamLogPath
		if logPath == "" {
			logPath = "logs/stream.log"
		}
		// Stream log format
		streamFmtStr := ""
		if logFmt == "json" {
			streamFmtStr = "    log_format stream_json escape=json\n" +
				"        '$realip_remote_addr [$remote_addr] [$time_local] '\n" +
				"        '$protocol $status $bytes_sent $bytes_received '\n" +
				"        '$session_time \"$upstream_addr\" '\n" +
				"        '\"$upstream_bytes_sent\" \"$upstream_bytes_received\" '\n" +
				"        '\"$upstream_connect_time\" $ssl_preread_server_name;\n"
		} else {
			streamFmtStr = "    log_format stream_basic\n" +
				"        '$realip_remote_addr [$remote_addr] [$time_local] '\n" +
				"        '$protocol $status $bytes_sent $bytes_received '\n" +
				"        '$session_time $ssl_preread_server_name';\n"
		}

		switch logOutputMode {
		case "stdout":
			ensureSymlink(filepath.Join(m.dataDir, "logs", "stdout_stream.log"), "/dev/stdout")
			sb.WriteString(streamFmtStr)
			sb.WriteString(fmt.Sprintf("    access_log logs/stdout_stream.log stream_%s buffer=32k;\n", logFmt))
		case "off":
			// no stream log
		default: // "file"
			if s.StreamLog {
				sb.WriteString(streamFmtStr)
				sb.WriteString(fmt.Sprintf("    access_log %s stream_%s buffer=32k;\n", logPath, logFmt))
			}
		}

		// Collect all stream upstreams
		streamUpstreamSeen := make(map[string]bool)

		// SNI routing via ssl_preread
		if s.StreamSNIEnabled && len(sniSites) > 0 {
			sniByPort := make(map[string][]entity.NginxSite)
			for _, site := range sniSites {
				port := site.Path
				if port == "" {
					port = "443"
				}
				sniByPort[port] = append(sniByPort[port], site)
			}

			for port, portSites := range sniByPort {
				// Build upstreams for SNI targets
				for _, site := range portSites {
					domain := site.Domain
					if domain == "fallback" || domain == "" {
						continue
					}
					target := site.SNITarget
					if target == "" {
						target = site.Upstream
					}
					if target == "" {
						continue
					}
					upKey := nginxUpstreamKey(domain)
					if streamUpstreamSeen[upKey] {
						continue
					}
					streamUpstreamSeen[upKey] = true
					sb.WriteString(fmt.Sprintf("upstream sni_%s {\n", upKey))
					for _, up := range strings.Split(target, ",") {
						up = strings.TrimSpace(up)
						if up != "" {
							sb.WriteString(fmt.Sprintf("    server %s;\n", up))
						}
					}
					sb.WriteString("}\n\n")
				}

				// Fallback upstream
				var fallbackTarget string
				var fallbackPP bool
				for _, site := range portSites {
					if site.Domain == "fallback" || site.Domain == "" {
						fallbackTarget = site.SNITarget
						if fallbackTarget == "" {
							fallbackTarget = site.Upstream
						}
						if fallbackTarget == "" {
							localPort := site.LocalPort
							if localPort == "" {
								localPort = "8443"
							}
							fallbackTarget = fmt.Sprintf("127.0.0.1:%s", localPort)
						}
						fallbackPP = site.ProxyProtocol
						break
					}
				}
				if fallbackTarget == "" {
					fallbackTarget = "127.0.0.1:8443"
				}
				if !streamUpstreamSeen["sni_fallback"] {
					streamUpstreamSeen["sni_fallback"] = true
					sb.WriteString("upstream sni_fallback {\n")
					for _, up := range strings.Split(fallbackTarget, ",") {
						up = strings.TrimSpace(up)
						if up != "" {
							sb.WriteString(fmt.Sprintf("    server %s;\n", up))
						}
					}
					sb.WriteString("}\n\n")
				}

				// ssl_preread map
				sb.WriteString(fmt.Sprintf("# SNI routing on :%s\n", port))
				sb.WriteString("map $ssl_preread_server_name $sni_backend {\n")
				for _, site := range portSites {
					domain := site.Domain
					if domain == "fallback" || domain == "" {
						continue
					}
					sb.WriteString(fmt.Sprintf("    %s sni_%s;\n", domain, nginxUpstreamKey(domain)))
				}
				sb.WriteString("    default sni_fallback;\n")
				sb.WriteString("}\n\n")

				// SNI server block
				connectTimeout := s.StreamConnectTimeout
				if connectTimeout == "" {
					connectTimeout = "1s"
				}
				streamTimeout := s.StreamTimeout
				if streamTimeout == "" {
					streamTimeout = "1h"
				}
				prereadTimeout := s.StreamPrereadTimeout
				if prereadTimeout == "" {
					prereadTimeout = "3s"
				}
				sb.WriteString("server {\n")
				sb.WriteString(fmt.Sprintf("    listen %s reuseport fastopen=4096 so_keepalive=on backlog=4096;\n", port))
				if s.StreamIPv6 || globalIPv6 {
					sb.WriteString(fmt.Sprintf("    listen [::]:%s reuseport fastopen=4096 so_keepalive=on backlog=4096;\n", port))
				}
				sb.WriteString("    ssl_preread on;\n")
				sb.WriteString("    proxy_half_close on;\n")
				if s.StreamProxyProtocol || fallbackPP {
					sb.WriteString("    proxy_protocol on;\n")
				}
				sb.WriteString("    tcp_nodelay on;\n")
				sb.WriteString("    preread_buffer_size 128k;\n")
				// 未发 ClientHello/SNI 的探测连接用短超时快速丢弃（防扫描）；已建立连接允许长时间空闲，
				// 以承载面板 SSE/日志流等长连接（proxy_timeout 过短会在空闲 3s 后把流切断）。
				sb.WriteString(fmt.Sprintf("    preread_timeout %s;\n", prereadTimeout))
				sb.WriteString(fmt.Sprintf("    proxy_connect_timeout %s;\n", connectTimeout))
				sb.WriteString(fmt.Sprintf("    proxy_timeout %s;\n", streamTimeout))
				sb.WriteString("    proxy_pass $sni_backend;\n")
				sb.WriteString("}\n\n")
			}
		}

		// L4 forwarding with upstream blocks
		if len(l4Sites) > 0 {
			for _, site := range l4Sites {
				upKey := nginxUpstreamKey(site.Domain)
				if streamUpstreamSeen[upKey] {
					continue
				}
				streamUpstreamSeen[upKey] = true
				sb.WriteString(fmt.Sprintf("upstream l4_%s {\n", upKey))
				for _, up := range strings.Split(site.Upstream, ",") {
					up = strings.TrimSpace(up)
					if up != "" {
						sb.WriteString(fmt.Sprintf("    server %s;\n", up))
					}
				}
				sb.WriteString("}\n\n")
			}

			for _, site := range l4Sites {
				port := site.Path
				if port == "" {
					port = "443"
				}
				proto := site.L4Protocol
				if proto == "" {
					proto = "tcp"
				}
				upKey := nginxUpstreamKey(site.Domain)
				sb.WriteString(fmt.Sprintf("# L4 %s on :%s -> %s\n", proto, port, site.Upstream))
				sb.WriteString("server {\n")
				if proto == "udp" {
					sb.WriteString(fmt.Sprintf("    listen %s udp so_keepalive=on;\n", port))
					if s.StreamIPv6 || globalIPv6 {
						sb.WriteString(fmt.Sprintf("    listen [::]:%s udp ipv6only=on so_keepalive=on;\n", port))
					}
				} else {
					sb.WriteString(fmt.Sprintf("    listen %s so_keepalive=on;\n", port))
					if s.StreamIPv6 || globalIPv6 {
						sb.WriteString(fmt.Sprintf("    listen [::]:%s ipv6only=on so_keepalive=on;\n", port))
					}
				}
				sb.WriteString("    proxy_socket_keepalive on;\n")
				sb.WriteString("    proxy_buffer_size 512k;\n")
				if site.ProxyProtocol {
					sb.WriteString("    proxy_protocol on;\n")
				}
				sb.WriteString(fmt.Sprintf("    proxy_pass l4_%s;\n", upKey))
				sb.WriteString("    proxy_timeout 5m;\n")
				sb.WriteString("    proxy_connect_timeout 2s;\n")
				sb.WriteString("}\n\n")
			}
		}

		sb.WriteString("}\n")
	}

	return sb.String(), nil
}

// writeProxySSLUpstream 回源走 TLS 时补 SNI 与"不校验回源证书" ✓（http/https 转发用 ✓）。
//
// 回源对象是本机内核（127.0.0.1）✓，证书多为自签/内部签发 ✓ → 默认不校验链 ✓；
// 需要强校验时，用站点的 LocationConfig 追加 `proxy_ssl_verify on;` 覆盖 ✓。
func writeProxySSLUpstream(sb *strings.Builder, sni string) {
	if sni != "" {
		sb.WriteString("            proxy_ssl_server_name on;\n")
		sb.WriteString(fmt.Sprintf("            proxy_ssl_name %s;\n", sni))
	}
	sb.WriteString("            proxy_ssl_verify off;\n")
}
