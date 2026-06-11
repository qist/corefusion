package proxy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"corefusion/internal/model/entity"
)

// nginxUpstreamKey converts a domain/name to a safe upstream block name.
// GenerateConfig assembles an nginx.conf from settings and sites.
func (m *NginxManager) GenerateConfig(ctx context.Context) (string, error) {
	sites, err := m.siteRepo.ListEnabled(ctx)
	if err != nil {
		return "", err
	}

	s, err := m.GetNginxSettings(ctx)
	if err != nil {
		s = DefaultNginxSettings()
	}

	var sb strings.Builder

	// Ensure directories
	os.MkdirAll(filepath.Join(m.dataDir, "logs"), 0750)
	os.MkdirAll(filepath.Join(m.dataDir, "runtime"), 0750)
	os.MkdirAll(filepath.Join(m.dataDir, "temp"), 0750)

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
	if s.MasterProcess {
		sb.WriteString("master_process on;\n")
	} else {
		sb.WriteString("master_process off;\n")
	}
	// Error log
	if s.ErrorLog {
		sb.WriteString(fmt.Sprintf("error_log %s %s;\n", s.ErrorLogPath, s.ErrorLogLevel))
	} else {
		sb.WriteString("error_log /dev/null;\n")
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
	if s.AccessLog {
		sb.WriteString("    log_format main escape=json '$clientRealIp $remote_addr $proxy_protocol_addr - $remote_user [$time_local] \"$request\" '\n")
		sb.WriteString("    '$status $body_bytes_sent \"$http_referer\" '\n")
		sb.WriteString("    '\"$http_user_agent\" \"$http_x_forwarded_for\" \"$http_host\" \"$http3\" '\n")
		sb.WriteString("    'ups_add: $upstream_addr ups_resp_time: $upstream_response_time '\n")
		sb.WriteString("    'request_time: $request_time ups_status: $upstream_status ';\n")
		sb.WriteString(fmt.Sprintf("    access_log  %s main;\n", s.AccessLogPath))
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
	if s.LimitConnZone != "" {
		sb.WriteString(fmt.Sprintf("    limit_conn_zone %s;\n", s.LimitConnZone))
	}
	if s.LimitConn != "" {
		sb.WriteString(fmt.Sprintf("    limit_conn %s;\n", s.LimitConn))
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

	// Separate SNI, L4, and HTTP sites
	var sniSites []entity.NginxSite
	var l4Sites []entity.NginxSite
	var httpSites []entity.NginxSite
	for _, site := range sites {
		switch site.Template {
		case "sni":
			sniSites = append(sniSites, site)
		case "l4":
			l4Sites = append(l4Sites, site)
		default:
			httpSites = append(httpSites, site)
		}
	}

	// Collect upstreams (HTTP sites only)
	upstreamSeen := make(map[string]bool)
	for _, site := range httpSites {
		if site.Upstream == "" {
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

	for domain, siteList := range domainSites {
		sb.WriteString(fmt.Sprintf("    # %s\n", domain))
		sb.WriteString("    server {\n")

		// Collect settings from all sites for this domain
		hasTLS := false
		hasSocket := false
		hasProxyProtocol := false
		enableIpv6 := false
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
			sb.WriteString(fmt.Sprintf("        ssl_certificate ../%s;\n", stripDataDirPrefix(tlsCert, m.dataDir)))
			sb.WriteString(fmt.Sprintf("        ssl_certificate_key ../%s;\n", stripDataDirPrefix(tlsKey, m.dataDir)))
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

		// Locations
		for _, site := range siteList {
			path := site.Path
			if path == "" {
				path = "/"
			}
			upstreamKey := strings.ReplaceAll(site.Upstream, ".", "_")
			upstreamKey = strings.ReplaceAll(upstreamKey, ":", "_")
			upstreamKey = strings.ReplaceAll(upstreamKey, ",", "_")

			// gRPC uses a different location block syntax
			if site.Template == "grpc" {
				sb.WriteString(fmt.Sprintf("        location %s {\n", path))
				sb.WriteString("            client_max_body_size 0;\n")
				sb.WriteString("            keepalive_requests 4294967296;\n")
				sb.WriteString("            client_body_timeout 1h;\n")
				sb.WriteString("            send_timeout 1h;\n")
				sb.WriteString("            lingering_close always;\n")
				sb.WriteString("            grpc_set_header X-Real-IP $clientRealIp;\n")
				sb.WriteString("            grpc_read_timeout 1h;\n")
				sb.WriteString("            grpc_send_timeout 1h;\n")
				sb.WriteString(fmt.Sprintf("            grpc_pass grpc://backend_%s;\n", upstreamKey))
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

			switch site.Template {
			case "websocket":
				sb.WriteString(fmt.Sprintf("            proxy_pass http://backend_%s/;\n", upstreamKey))
				sb.WriteString("            proxy_http_version 1.1;\n")
				sb.WriteString("            proxy_set_header Upgrade $http_upgrade;\n")
				sb.WriteString("            proxy_set_header Connection $connection_upgrade;\n")
				sb.WriteString("            proxy_set_header Host $host;\n")
				sb.WriteString("            proxy_set_header X-Forwarded-Proto $scheme;\n")
				sb.WriteString("            proxy_set_header X-Real-IP $remote_addr;\n")
				sb.WriteString("            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n")
				sb.WriteString("            proxy_set_header X-NginX-Proxy true;\n")
				sb.WriteString("            proxy_ssl_session_reuse off;\n")
				sb.WriteString("            proxy_redirect off;\n")
			default:
				sb.WriteString(fmt.Sprintf("            proxy_pass http://backend_%s;\n", upstreamKey))
				sb.WriteString("            proxy_http_version 1.1;\n")
				sb.WriteString("            proxy_set_header Accept-Encoding \"\";\n")
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

	// Default server if no sites
	if len(sites) == 0 {
		sb.WriteString("    server {\n")
		sb.WriteString("        listen 80;\n")
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

		// Stream log format
		logFmt := s.StreamLogFormat
		if logFmt == "" {
			logFmt = "basic"
		}
		logPath := s.StreamLogPath
		if logPath == "" {
			logPath = "logs/stream.log"
		}
		if s.StreamLog {
			if logFmt == "json" {
				sb.WriteString("    log_format stream_json escape=json\n")
				sb.WriteString("        '$realip_remote_addr [$remote_addr] [$time_local] '\n")
				sb.WriteString("        '$protocol $status $bytes_sent $bytes_received '\n")
				sb.WriteString("        '$session_time \"$upstream_addr\" '\n")
				sb.WriteString("        '\"$upstream_bytes_sent\" \"$upstream_bytes_received\" '\n")
				sb.WriteString("        '\"$upstream_connect_time\" $ssl_preread_server_name;\n")
			} else {
				sb.WriteString("    log_format stream_basic\n")
				sb.WriteString("        '$realip_remote_addr [$remote_addr] [$time_local] '\n")
				sb.WriteString("        '$protocol $status $bytes_sent $bytes_received '\n")
				sb.WriteString("        '$session_time $ssl_preread_server_name';\n")
			}
			sb.WriteString(fmt.Sprintf("    access_log %s stream_%s buffer=32k;\n", logPath, logFmt))
			sb.WriteString("    open_log_file_cache off;\n\n")
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
					streamTimeout = "3s"
				}
				sb.WriteString("server {\n")
				sb.WriteString(fmt.Sprintf("    listen %s reuseport fastopen=4096 so_keepalive=on backlog=4096;\n", port))
				if s.StreamIPv6 {
					sb.WriteString(fmt.Sprintf("    listen [::]:%s reuseport fastopen=4096 so_keepalive=on backlog=4096;\n", port))
				}
				sb.WriteString("    ssl_preread on;\n")
				sb.WriteString("    proxy_half_close on;\n")
				if s.StreamProxyProtocol || fallbackPP {
					sb.WriteString("    proxy_protocol on;\n")
				}
				sb.WriteString("    tcp_nodelay on;\n")
				sb.WriteString("    preread_buffer_size 128k;\n")
				sb.WriteString(fmt.Sprintf("    proxy_connect_timeout %s;\n", connectTimeout))
				sb.WriteString(fmt.Sprintf("    proxy_timeout %s;\n", streamTimeout))
				if s.StreamLog {
					sb.WriteString(fmt.Sprintf("    access_log logs/stream_%s.log stream_%s buffer=32k;\n", port, logFmt))
				}
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
					if s.StreamIPv6 {
						sb.WriteString(fmt.Sprintf("    listen [::]:%s udp ipv6only=on so_keepalive=on;\n", port))
					}
				} else {
					sb.WriteString(fmt.Sprintf("    listen %s so_keepalive=on;\n", port))
					if s.StreamIPv6 {
						sb.WriteString(fmt.Sprintf("    listen [::]:%s ipv6only=on so_keepalive=on;\n", port))
					}
				}
				sb.WriteString("    proxy_socket_keepalive on;\n")
				sb.WriteString("    proxy_buffer_size 512k;\n")
				if s.StreamLog {
					sb.WriteString(fmt.Sprintf("    access_log logs/stream.log stream_%s buffer=32k;\n", logFmt))
				}
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
