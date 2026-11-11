package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"corefusion/internal/model/entity"
)

// nodeACMEChallengeDir 是**节点上** ACME 挑战文件的落盘目录 ✓。
// 必须与 Agent 侧保持一致 ✗（cmd/agent/acme_webserver.go 的 acmeChallengeDir ✓），
// 且**不能在 /tmp** ✗ —— nginx 服务单元常带 PrivateTmp=yes ✗，放 /tmp 会恒 404 ✗（真机踩过 ✓）。
const nodeACMEChallengeDir = "/var/lib/corefusion-agent/acme-challenge"

// proxy_headers_hash 的可用下限：低于这个值时 nginx 会告警并忽略
// （"could not build optimal proxy_headers_hash ... ignoring proxy_headers_hash_bucket_size"）。
const (
	minProxyHeadersHashMaxSize    = 2048
	minProxyHeadersHashBucketSize = 128
)

// nginxHashSize 确保 hash size 是合法整数且不低于 min；非法值按 min 处理。
func nginxHashSize(v string, min int) string {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n < min {
		return strconv.Itoa(min)
	}
	return strings.TrimSpace(v)
}

// hasRootOrAlias 判断 location 级配置里是否已显式写了 root / alias。
func hasRootOrAlias(locationConfig string) bool {
	for _, line := range strings.Split(locationConfig, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "root ") || strings.HasPrefix(line, "alias ") {
			return true
		}
	}
	return false
}

// hasLocationDirective 判断站点自定义 location 配置里是否已经写了某条指令。
// 用于"默认值只在用户没写时才补"，避免生成器把参数写死、用户改不动。
func hasLocationDirective(locationConfig, name string) bool {
	for _, line := range strings.Split(locationConfig, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if strings.TrimSuffix(fields[0], ";") == name {
			return true
		}
	}
	return false
}

// isDAVLocation 判断是否是 WebDAV 目录（dav_methods / dav_ext_methods）。
// 这类 location 不能加 index/try_files：try_files 在 precontent 阶段会把
// PUT/MKCOL/DELETE 判成 404，配合 error_page 404=200 会静默吞掉写入失败。
func isDAVLocation(locationConfig string) bool {
	return hasLocationDirective(locationConfig, "dav_methods") ||
		hasLocationDirective(locationConfig, "dav_ext_methods")
}

// writeLocationConfig 写出站点自定义的 location 指令：
//   - 自动补分号；
//   - already 里列出的指令（生成器刚写过）跳过，避免 `set $waf_enable off;` 出现两次；
//   - 同一段里完全相同的行只保留一条（历史导入/复制粘贴造成的重复）。
func writeLocationConfig(sb *strings.Builder, locationConfig string, already map[string]bool) {
	if strings.TrimSpace(locationConfig) == "" {
		return
	}
	written := make(map[string]bool, len(already)+8)
	for k := range already {
		written[k] = true
	}
	for _, line := range strings.Split(strings.TrimSpace(locationConfig), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if !strings.HasSuffix(trimmed, ";") && !strings.HasSuffix(trimmed, "{") && !strings.HasSuffix(trimmed, "}") {
			trimmed += ";"
		}
		key := strings.Join(strings.Fields(trimmed), " ")
		if written[key] {
			continue
		}
		written[key] = true
		sb.WriteString("            " + trimmed + "\n")
	}
}

// ensureRegularLogFile ensures path is a real, regular log file.
// 旧实现把 stdout 模式做成 `logs/stdout_*.log -> /dev/stdout` 软链：
// 面板/Agent 重启后软链解析到**新进程自己的 stdout**，既读不到 nginx 日志，
// 旧 nginx 也还在往已断开的管道写。现在统一成真实文件，父进程重启不影响读写。
func ensureRegularLogFile(path string) {
	if info, err := os.Lstat(path); err == nil {
		if info.Mode().IsRegular() {
			return
		}
		_ = os.Remove(path) // symlink / device / dir
	}
	if f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0640); err == nil {
		_ = f.Close()
	}
}

// nginxUpstreamKey converts a domain/name to a safe upstream block name.
// GenerateConfig assembles an nginx.conf from settings and sites.
func (m *NginxManager) GenerateConfig(ctx context.Context, serverID int64) (string, error) {
	return m.GenerateConfigForServer(ctx, serverID)
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
// ⚠ 不能用共享状态 ✗：历史上 handlers 每请求调 SetIsLocal 改写单例 ✓，本机/远程请求
// **交错**时会互相覆盖 ✗（实测踩到 ✓：对 node 1 生成配置时被别的请求置为 false ✗，
// 文件校验没生效 ✗，缺文件的证书照样写进配置 ✗）。该字段已彻底移除 ✓。
//
// 判断只认**本次调用的 serverID 参数** ✓，与任何共享状态无关 ✓。
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
	sites = m.withDerivedSNISites(ctx, sourceServerID, sites)
	return m.GenerateConfigForSites(ctx, serverID, sourceServerID, sites)
}

// withDerivedSNISites 把 REALITY 入站的 serverNames 补进 SNI 转发规则。
//
// 不落库：入站改了 serverNames，下次生成配置就自动跟上；显式站点（用户在
// 「Nginx 代理」里配过的同域名规则）保持优先，不会被覆盖。
func (m *NginxManager) withDerivedSNISites(ctx context.Context, serverID int64, sites []entity.NginxSite) []entity.NginxSite {
	if m.inboundRepo == nil {
		return sites
	}
	cfgs, err := m.inboundRepo.ListByServerID(ctx, serverID)
	if err != nil {
		log.Printf("[NginxManager] list inbounds for SNI derivation failed: %v", err)
		return sites
	}
	var rules []derivedSNIRule
	for _, cfg := range cfgs {
		rules = append(rules, SNIRulesFromInbound(cfg)...)
	}
	return mergeDerivedNginxSNISites(serverID, sites, rules)
}

// PreviewSiteConfig 用「库里已有站点 + 正在编辑的草稿」生成完整配置。
//
// 预览必须和真实落盘同源：以前前端自己拼预览，样式/默认值（index、try_files、
// server_name 分隔符）和生成器各写一套 → 预览说一套、实际生成另一套 ✗。
// 这里直接复用 GenerateConfigForSites，草稿与库中同 domain/path/template 的站点互斥
// （编辑时覆盖，新建时追加），所以预览内容就是保存后真正会写出的内容。
func (m *NginxManager) PreviewSiteConfig(ctx context.Context, serverID int64, draft entity.NginxSite) (string, error) {
	sourceServerID, _ := m.resolveSourceServerID(ctx, serverID)
	sites, err := m.siteRepo.ListEnabled(ctx, sourceServerID)
	if err != nil {
		return "", err
	}
	merged := make([]entity.NginxSite, 0, len(sites)+1)
	for _, s := range sites {
		// 编辑已有站点：必须按 ID 替换。若按 domain/path/template 匹配，
		// 一旦改了监听地址或域名，旧行不会被剔除 → 预览里同时出现新旧两份配置。
		if draft.ID > 0 {
			if s.ID == draft.ID {
				continue
			}
		} else if s.Domain == draft.Domain && s.Path == draft.Path && s.Template == draft.Template {
			// 新建站点：同 domain/path/template 视为同一草稿，避免重复。
			continue
		}
		merged = append(merged, s)
	}
	draft.ServerID = sourceServerID
	draft.Enabled = true
	merged = append(merged, draft)
	return m.GenerateConfigForSites(ctx, serverID, sourceServerID, merged)
}

// GenerateConfigForSites 是配置生成的主体：预览与正式生成共用，保证两者完全一致。
func (m *NginxManager) GenerateConfigForSites(ctx context.Context, serverID, sourceServerID int64, sites []entity.NginxSite) (string, error) {
	s, err := m.GetNginxSettingsForServer(ctx, sourceServerID)
	if err != nil {
		s = DefaultNginxSettings()
	}
	// 全局 IPv6 开关：作用于所有生成的 listen（站点 server / HTTP→HTTPS 跳转 /
	// 默认 server / stream）。站点级 enable_ipv6 可在全局关闭时单独为该站点开启。
	globalIPv6 := s.IPv6
	// 全局监听拆成 http/https × v4/v6 四项，避免一个 ipv6_listen_addr 被 80/443 复用。
	httpListen := globalListen(s.HTTPListen, "80")
	httpsListen := globalListen(s.HTTPSListen, "443")
	httpListenV6 := globalListenV6(s.HTTPListenV6, httpListen)
	httpsListenV6 := globalListenV6(s.HTTPSListenV6, httpsListen)

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
				logOutputMode = normalizeProxyLogMode(cfg.Nginx.Mode)
			}
		}
	}
	switch logOutputMode {
	case "stdout":
		ensureRegularLogFile(filepath.Join(m.dataDir, "logs", "stdout_error.log"))
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
	// 自定义/历史配置扩展点：面板每次重新生成都会覆盖 nginx.conf，
	// 旧机迁移过来的手写 server（如 CDN 回源 8080）放这里才不会被冲掉。
	// ⚠ 相对 include 是相对 **nginx.conf 所在目录**（data/proxy）解析的，
	// 所以这里必须是 conf.d/*.conf，不能写 proxy/conf.d/*.conf。
	// 目录不存在时 nginx 对通配 include 不报错 ✓。
	sb.WriteString("    include conf.d/*.conf;\n")
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
	// $proxy_protocol_addr 在连接 accept 阶段就有值（PROXY protocol 头），而 realip
	// 改写的 $remote_addr 要等 post-read 阶段才生效：CONNECT/畸形请求在请求行解析
	// 阶段就被 405/400 拒绝，此时 realip 还没跑，$remote_addr 仍是原始 peer ——
	// unix socket 前置（reality 回落等）下就是字面量 "unix:"，日志里会记成 unix:。
	// 所以先用 $pp_addr 兜底（无 PROXY 头时回退 $remote_addr），再让 $clientRealIp
	// 在 XFF 为空时回退 $pp_addr。与上游 nginxguard 2026-10-26 的修复一致。
	sb.WriteString("    map $proxy_protocol_addr $pp_addr {\n")
	sb.WriteString("        \"\" $remote_addr;\n")
	sb.WriteString("        default $proxy_protocol_addr;\n")
	sb.WriteString("    }\n\n")
	sb.WriteString("    map $http_x_forwarded_for $clientRealIp {\n")
	sb.WriteString("        \"\" $pp_addr;\n")
	sb.WriteString("        ~^(?P<firstAddr>[0-9\\.\\:a-fA-F]+),?.*$ $firstAddr;\n")
	// XFF 存在但格式不匹配（unknown / 带空格等）时回退到 $pp_addr，避免空串。
	sb.WriteString("        default $pp_addr;\n")
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
		ensureRegularLogFile(filepath.Join(m.dataDir, "logs", "stdout_access.log"))
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
	// nginx 默认 512/64；站点多、proxy_set_header 多时会告警
	// "could not build optimal proxy_headers_hash ... ignoring proxy_headers_hash_bucket_size"。
	// 历史安装的 DB 里存的就是 512/64，所以这里兜底抬到可用下限（配置值本身仍可调大）。
	proxyHeadersMaxSize := nginxHashSize(s.ProxyHeadersHashMaxSize, minProxyHeadersHashMaxSize)
	proxyHeadersBucketSize := nginxHashSize(s.ProxyHeadersHashBucketSize, minProxyHeadersHashBucketSize)
	if proxyHeadersMaxSize != s.ProxyHeadersHashMaxSize || proxyHeadersBucketSize != s.ProxyHeadersHashBucketSize {
		log.Printf("[NginxManager] proxy_headers_hash %s/%s is too small, using %s/%s",
			s.ProxyHeadersHashMaxSize, s.ProxyHeadersHashBucketSize, proxyHeadersMaxSize, proxyHeadersBucketSize)
	}
	sb.WriteString(fmt.Sprintf("    proxy_headers_hash_max_size %s;\n", proxyHeadersMaxSize))
	sb.WriteString(fmt.Sprintf("    proxy_headers_hash_bucket_size %s;\n", proxyHeadersBucketSize))
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
		// 含 scheme 的值（如 http://php、https://host:port）是直接上游，不生成 upstream 块。
		if strings.Contains(site.Upstream, "://") {
			continue
		}
		upstreams := strings.Split(site.Upstream, ",")
		upstreamKey := nginxUpstreamKey(site.Upstream)
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

	// Group by domain set (HTTP sites only) — 一个站点可绑多个域名 ✓
	// 键 = 排序后的域名组，保证 a.com,b.com 与 b.com,a.com 落到同一个 server 块 ✓，
	// 同时生成结果与输入顺序无关（确定性）。
	domainSites := make(map[string][]entity.NginxSite)
	domainOrder := make([]string, 0)
	groupDomains := make(map[string][]string)
	for _, site := range httpSites {
		domains := SplitDomains(site.Domain)
		key := DomainGroupKey(domains)
		if _, ok := domainSites[key]; !ok {
			domainOrder = append(domainOrder, key)
			groupDomains[key] = domains
		}
		domainSites[key] = append(domainSites[key], site)
	}
	sort.Strings(domainOrder)

	// 80 端口是否开启 ✓：存在 listen 80 的**启用**站点 ✓
	//（ListenPort="80" ✓ / 非证书站点未写端口（默认 80 ✓））。
	// 没开 80 就**不生成** HTTP→HTTPS 跳转 ✓（跳转 server 抢不到 :80、跳了也白跳 ✗）。
	port80Open := false
	httpPortOnly := listenPortOnly(httpListen)
	for _, s := range httpSites {
		p := s.ListenPort
		if p == "" && s.CertPath == "" && s.KeyPath == "" && s.ListenSocket == "" {
			p = httpListen
		}
		if listenPortOnly(p) == httpPortOnly {
			port80Open = true
			break
		}
	}

	for _, groupKey := range domainOrder {
		siteList := domainSites[groupKey]
		domains := groupDomains[groupKey]
		// primary 用于证书查找/跳转目标/日志；server_name 用整组域名 ✓
		primary := ""
		if len(domains) > 0 {
			primary = domains[0]
		}
		serverNames := strings.Join(domains, " ")
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
		if grpHasTLS && port80Open && redirectEnabled && primary != "" {
			target := "https://" + primary
			if tlsPort != "443" {
				target = fmt.Sprintf("https://%s:%s", primary, tlsPort)
			}
			sb.WriteString("    # HTTP → HTTPS redirect (auto-generated; per-site switch)\n")
			sb.WriteString("    server {\n")
			sb.WriteString(fmt.Sprintf("        listen %s;\n", httpListen))
			if grpIPv6 {
				sb.WriteString(fmt.Sprintf("        listen %s;\n", httpListenV6))
			}
			sb.WriteString(fmt.Sprintf("        server_name %s;\n", serverNames))
			if !m.isLocalTarget(ctx, serverID) {
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

		sb.WriteString(fmt.Sprintf("    # %s\n", primary))
		sb.WriteString("    server {\n")

		// Collect settings from all sites for this domain
		hasTLS := false
		hasSocket := false
		socketProxyProtocol := false
		hasProxyProtocol := false
		enableIpv6 := globalIPv6
		// 系统内部站点（non-tls.invalid / unix-nginx-0.invalid 等）不继承全局 IPv6：
		// 它们只被本机前置层回源，多开一个公网 v6 监听没有意义，也容易和 TLS 端口冲突。
		if strings.HasSuffix(primary, ".invalid") {
			enableIpv6 = false
		}
		// 站点级 IPv6 监听地址（可含端口）优先于全局；同域名多 location 取第一个显式配置的。
		groupV6Addr := ""
		listenPort := ""
		httpsPortExplicit := false
		httpPortExplicit := false
		var tlsCert, tlsKey string
		var tlsSite *entity.NginxSite
		for _, site := range siteList {
			if site.ListenSocket != "" && !hasSocket {
				// unix socket 也要按站点开关声明 proxy_protocol：否则前置层发来的
				// PROXY 头会被当成请求行 → 400，且 $proxy_protocol_addr 恒为空。
				//
				// 站点自带证书时还必须带 ssl：REALITY 的回落目标就是这么用的
				// （unix:/dev/shm/nginx.sock ssl proxy_protocol）。漏了 ssl，target 变成
				// 明文 HTTP —— REALITY 要把客户端的 ClientHello 转发给 target、再把
				// target 的 ServerHello 回给客户端，target 不吐 TLS 时**已认证的连接也握手失败**
				// （客户端报 "first record does not look like a TLS handshake" 或证书不匹配）。
				sslParam := ""
				if site.CertPath != "" && site.KeyPath != "" {
					sslParam = " ssl"
				}
				if site.ProxyProtocol {
					sb.WriteString(fmt.Sprintf("        listen unix:%s%s proxy_protocol;\n", site.ListenSocket, sslParam))
					socketProxyProtocol = true
				} else {
					sb.WriteString(fmt.Sprintf("        listen unix:%s%s;\n", site.ListenSocket, sslParam))
				}
				hasSocket = true
			}
			if site.ListenPort != "" {
				if site.CertPath != "" && site.KeyPath != "" {
					httpsPortExplicit = true
				} else {
					httpPortExplicit = true
				}
				if listenPort == "" {
					listenPort = site.ListenPort
				}
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
			if groupV6Addr == "" && strings.TrimSpace(site.IPv6ListenAddr) != "" {
				groupV6Addr = site.IPv6ListenAddr
			}
		}
		// 旧的全局 ipv6_listen_addr 已在 mergeNginxSettings 中迁移为
		// http_listen_v6 / https_listen_v6，这里只保留站点级覆盖。

		// Default port: 443 for TLS, 80 for non-TLS
		if listenPort == "" || (hasTLS && listenPortOnly(listenPort) == "80") {
			if hasTLS {
				listenPort = httpsListen
			} else {
				listenPort = httpListen
			}
		}
		// 通配/兜底域名（"" 或 "_"）显式 default_server，避免"默认站点"随生成顺序漂移
		ds := ""
		if primary == "" || primary == "_" {
			ds = " default_server"
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

		// socket-only：站点只挂 unix socket（如 REALITY 回落 server：listen unix:... ssl proxy_protocol，
		// 无 server_name/无 443）。这类站点不能再按默认端口补 listen 443/80 —— 否则会凭空多出
		// 一个 HTTPS 监听（还可能带 default_server），抢走真实站点的流量。
		socketOnly := len(siteList) > 0
		for _, s := range siteList {
			if s.ListenSocket == "" || s.ListenPort != "" {
				socketOnly = false
				break
			}
		}

		// Listen directives - IPv4 and IPv6 separately
		if enableQuic && hasTLS && !socketOnly {
			// QUIC 监听优先级：站点 quic_listen > 全局 quic_listen > HTTPS 监听。
			quicListen := listenPort
			if strings.TrimSpace(s.QUICListen) != "" {
				quicListen = globalListen(s.QUICListen, listenPort)
			}
			if tlsSite != nil && tlsSite.QuicListen != "" {
				quicListen = tlsSite.QuicListen
			}
			quicV6 := httpsListenV6
			if strings.TrimSpace(s.QUICListenV6) != "" {
				quicV6 = globalListenV6(s.QUICListenV6, quicListen)
			}
			sb.WriteString(fmt.Sprintf("        listen %s quic%s;\n", quicListen, ds))
			if enableIpv6 {
				sb.WriteString(fmt.Sprintf("        listen %s quic%s;\n", v6ListenWith(quicV6, groupV6Addr, quicListen, true), ds))
			}
		}
		if hasTLS {
			// HTTPS listen（socket-only 站点只监听 unix，不补 443）
			if !socketOnly {
				if hasProxyProtocol {
					sb.WriteString(fmt.Sprintf("        listen %s ssl proxy_protocol%s;\n", listenPort, ds))
					if enableIpv6 {
						sb.WriteString(fmt.Sprintf("        listen %s ssl proxy_protocol%s;\n", v6ListenWith(httpsListenV6, groupV6Addr, listenPort, httpsPortExplicit), ds))
					}
				} else {
					sb.WriteString(fmt.Sprintf("        listen %s ssl%s;\n", listenPort, ds))
					if enableIpv6 {
						sb.WriteString(fmt.Sprintf("        listen %s ssl%s;\n", v6ListenWith(httpsListenV6, groupV6Addr, listenPort, httpsPortExplicit), ds))
					}
				}
			}
			if enableHttp2 {
				sb.WriteString("        http2 on;\n")
			}
			if len(domains) == 0 {
				sb.WriteString("        server_name _;\n")
			} else {
				sb.WriteString(fmt.Sprintf("        server_name %s;\n", serverNames))
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
			for _, cp := range collectCertPairsForDomains(ctx, m.certRepo, domains, tlsCert, tlsKey, dual, checker) {
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
			// HTTP listen（socket-only 站点不补 80）
			if !socketOnly {
				if hasProxyProtocol {
					sb.WriteString(fmt.Sprintf("        listen %s proxy_protocol%s;\n", listenPort, ds))
					if enableIpv6 {
						sb.WriteString(fmt.Sprintf("        listen %s proxy_protocol%s;\n", v6ListenWith(httpListenV6, groupV6Addr, listenPort, httpPortExplicit), ds))
					}
				} else {
					sb.WriteString(fmt.Sprintf("        listen %s%s;\n", listenPort, ds))
					if enableIpv6 {
						sb.WriteString(fmt.Sprintf("        listen %s%s;\n", v6ListenWith(httpListenV6, groupV6Addr, listenPort, httpPortExplicit), ds))
					}
				}
			}
			if len(domains) == 0 {
				sb.WriteString("        server_name _;\n")
			} else {
				sb.WriteString(fmt.Sprintf("        server_name %s;\n", serverNames))
			}
		}

		// PROXY Protocol real_ip settings
		if hasProxyProtocol {
			sb.WriteString("        set_real_ip_from 127.0.0.1;\n")
			if enableIpv6 {
				sb.WriteString("        set_real_ip_from ::1;\n")
			}
			if socketProxyProtocol {
				// AF_UNIX 的 peer 不是 IP，127.0.0.1 匹配不上 → realip 不会改写，
				// $remote_addr 对所有 unix 连接恒为 "unix:"（WAF/日志都拿不到真实 IP）。
				// 必须显式信任 unix:（nginx 1.13.8+）。
				sb.WriteString("        set_real_ip_from unix:;\n")
			}
			sb.WriteString("        real_ip_header proxy_protocol;\n")
		}

		// Server-level custom directives (between server { and location)
		// 合并本域名组内**所有**站点的 server_config（相同行去重），不再只取第一个
		serverDirectiveSeen := make(map[string]bool)
		for _, site := range siteList {
			if site.ServerConfig == "" {
				continue
			}
			for _, line := range strings.Split(strings.TrimSpace(site.ServerConfig), "\n") {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" || strings.HasPrefix(trimmed, "#") {
					continue
				}
				// 先补分号再当去重键：`root /x` 与 `root /x;` 是同一条指令，
				// 否则导入+手工编辑后会各写一遍（真机出现过重复参数）。
				if !strings.HasSuffix(trimmed, ";") && !strings.HasSuffix(trimmed, "{") && !strings.HasSuffix(trimmed, "}") {
					trimmed += ";"
				}
				key := strings.Join(strings.Fields(trimmed), " ")
				if serverDirectiveSeen[key] {
					continue
				}
				serverDirectiveSeen[key] = true
				sb.WriteString("        " + trimmed + "\n")
			}
		}

		// ACME HTTP-01 challenge support
		//
		// 分两种，必须区分 ✗：
		//   - **本机（面板）**：面板自己驱动签发 ✓ → 挑战**代理回面板** ✓（面板内存里就有挑战记录 ✓）；
		//   - **远程节点**：签发是**节点上的 Agent 在本机做**的 ✓ → 它的挑战记录在 Agent 进程内存里，
		//     面板并不知道 ✗ → 所以**不能代理回面板** ✗，必须直接服务**节点本地的挑战目录** ✓
		//     （Agent 会把挑战文件写在那里 ✓，见 cmd/agent/acme_webserver.go ✓）。
		if !m.isLocalTarget(ctx, serverID) {
			sb.WriteString("        # ACME HTTP-01 challenge support (node-local, served from disk)\n")
			sb.WriteString("        location ^~ /.well-known/acme-challenge/ {\n")
			sb.WriteString(fmt.Sprintf("            alias %s/.well-known/acme-challenge/;\n", nodeACMEChallengeDir))
			sb.WriteString("        }\n")
		}
		if m.isLocalTarget(ctx, serverID) {
			panelAddr := m.panelAddr(ctx)
			sb.WriteString("        # ACME HTTP-01 challenge support\n")
			sb.WriteString("        location ^~ /.well-known/acme-challenge/ {\n")
			sb.WriteString(fmt.Sprintf("            proxy_pass http://%s;\n", panelAddr))
			sb.WriteString("            proxy_set_header Host $host;\n")
			sb.WriteString("            proxy_set_header X-Real-IP $remote_addr;\n")
			sb.WriteString("            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n")
			sb.WriteString("            proxy_set_header X-Forwarded-Proto $scheme;\n")
			sb.WriteString("        }\n")
		}

		// Locations（去重：同域名同 location 只保留第一个，其余跳过并记日志）
		seenLoc := make(map[string]bool)
		for _, site := range siteList {
			path := normalizeLocationPath(site.Path)
			if seenLoc[path] {
				log.Printf("[NginxManager] %s: duplicate location %q (id=%d) skipped", primary, path, site.ID)
				continue
			}
			seenLoc[path] = true
			upstreamKey := nginxUpstreamKey(site.Upstream)
			// 直接上游（含 scheme，如 http://php / https://host:port）：proxy_pass 原样使用
			directUpstream := ""
			if strings.Contains(site.Upstream, "://") && !strings.Contains(site.Upstream, ",") {
				directUpstream = strings.TrimSpace(site.Upstream)
			}

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
				if ds := SplitDomains(site.Domain); len(ds) > 0 {
					upstreamSNI = ds[0]
				}
			}
			upstreamTarget := fmt.Sprintf("%s://backend_%s", upstreamScheme, upstreamKey)
			if directUpstream != "" {
				upstreamTarget = directUpstream
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
				grpcTarget := fmt.Sprintf("%s://backend_%s", grpcScheme, upstreamKey)
				if directUpstream != "" {
					grpcTarget = directUpstream
				}
				sb.WriteString(fmt.Sprintf("            grpc_pass %s;\n", grpcTarget))
				// Location-level directives
				// 本分支上方已经写过 wafBypass（set $waf_enable off），
				// 站点配置里再带这条时要去重，否则同一 location 出现两次。
				var grpcEmitted map[string]bool
				if strings.TrimSpace(wafBypass) != "" {
					grpcEmitted = map[string]bool{"set $waf_enable off;": true}
				}
				writeLocationConfig(&sb, site.LocationConfig, grpcEmitted)
				sb.WriteString("        }\n")
				continue
			}

			sb.WriteString(fmt.Sprintf("        location %s {\n", path))
			sb.WriteString(wafBypass)

			switch site.Template {
			case "static":
				// location_config 里已经写了 root/alias（迁移存量站点时常见）就不再重复写，
				// 否则 alias 会被生成的 root 干扰（两者语义不同，alias 是前缀替换）。
				if !hasRootOrAlias(site.LocationConfig) {
					rootDir := strings.TrimSpace(site.Upstream)
					if rootDir == "" {
						rootDir = "/var/www"
					}
					sb.WriteString(fmt.Sprintf("            root %s;\n", rootDir))
				}
				// 默认只补 index（目录默认页，本地静态站点必备），且用户写了就不重复。
				// ⚠ try_files 一律不默认生成 ✗：
				//   - 普通静态目录/下载目录根本不需要（nginx 默认就是找不到返回 404）；
				//   - 只有 SPA（前端路由）才需要 fallback，应由站点在 location_config 里显式写；
				//   - WebDAV/上传目录带上它会让 PUT/MKCOL/DELETE 在 precontent 阶段被判 404，
				//     叠加 server 级 `error_page 404 =200 /index.html` 就变成
				//     "返回 200 但文件没落盘"，客户端随后 MOVE 覆盖还会删掉原文件 ✗（真机踩过）。
				davLocation := isDAVLocation(site.LocationConfig)
				if !davLocation && !hasLocationDirective(site.LocationConfig, "index") {
					sb.WriteString("            index index.html index.htm;\n")
				}
			case "echo_ip":
				// 回显真实客户端 IP：落地页/探测页常用（现网 nginx 里就是 echo -n $clientRealIp）。
				// $clientRealIp 由生成器顶部的 map 定义（无 XFF 时回退 PROXY protocol / remote_addr）。
				sb.WriteString("            default_type text/plain;\n")
				sb.WriteString("            return 200 \"$clientRealIp\\n\";\n")
			case "return_code":
				// 直接返回状态码（探测页常用 204）。状态码放 backend_path（其次 upstream）。
				code := strings.TrimSpace(site.BackendPath)
				if code == "" {
					code = strings.TrimSpace(site.Upstream)
				}
				if !isHTTPStatusCode(code) {
					code = "204"
				}
				sb.WriteString(fmt.Sprintf("            return %s;\n", code))
			case "redirect":
				// 跳转：目标 URL 放 backend_path（其次 upstream）；都没填就跳 https://同域名。
				target := strings.TrimSpace(site.BackendPath)
				if target == "" {
					target = strings.TrimSpace(site.Upstream)
				}
				if target == "" {
					target = "https://$host"
				}
				sb.WriteString(fmt.Sprintf("            return 301 %s$request_uri;\n", target))
			case "websocket", "xhttp", "httpupgrade":
				// 回源 URI 交给 nginx 原生语义：proxy_pass 末尾带 / → 用 location 之后的
				// 剩余路径替换；末尾不带 / → 原样透传完整 URI。
				// 不能再用 backend_path 猜：迁移来的 xray ws/httpupgrade 入站需要完整路径
				// （如 /efnxuvlqws），强行补 / 会把它剥成 / → 上游 404（TROJANWS/VMESSWS 全挂）。
				sb.WriteString(fmt.Sprintf("            proxy_pass %s;\n", upstreamTarget))
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
				sb.WriteString(fmt.Sprintf("            proxy_pass %s;\n", upstreamTarget))
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
				sb.WriteString(fmt.Sprintf("            proxy_pass %s;\n", upstreamTarget))
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
			// 已由生成器写过的指令（例如 set $waf_enable off）不再重复，
			// 导入多次/手工复制产生的重复行也只保留一条。
			var emitted map[string]bool
			if strings.TrimSpace(wafBypass) != "" {
				emitted = map[string]bool{"set $waf_enable off;": true}
			}
			writeLocationConfig(&sb, site.LocationConfig, emitted)

			sb.WriteString("        }\n")
		}

		sb.WriteString("    }\n\n")
	}

	// Reality server blocks: forced TLS, HTTP/2, no domain required.
	// 两个入口可以并存（PROXY Protocol 是 listen 级参数）：
	//   - Xray 入口：unix:<path> 或 127.0.0.1:<port>，带 proxy_protocol（xver=1）
	//   - sing-box 入口：127.0.0.1:<singbox_listen_port>，**不带** proxy_protocol
	//     （sing-box 只支持 host:port，且不发 PROXY protocol）
	// 必须带 ssl：REALITY 会把客户端 ClientHello 转给 target，target 不吐 TLS 就握手失败。
	for _, rs := range realitySites {
		socket := normalizeNginxUnixSocket(rs.ListenSocket)
		useSocket := socket != ""
		port := strings.TrimSpace(rs.ListenPort)
		if port != "" {
			port = listenPortOnly(port)
		}
		sbPort := strings.TrimSpace(rs.SingboxListenPort)
		if sbPort != "" {
			sbPort = listenPortOnly(sbPort)
		}
		if port == "" && !useSocket {
			port = "9443"
		}
		// 同一端口不可能同时「要 PROXY 头」和「不要」。
		if sbPort != "" && sbPort == port {
			sbPort = ""
		}
		if useSocket {
			sb.WriteString(fmt.Sprintf("    # Reality unix:%s\n", socket))
		} else {
			sb.WriteString(fmt.Sprintf("    # Reality :%s\n", port))
		}
		if sbPort != "" {
			sb.WriteString(fmt.Sprintf("    # Reality sing-box :%s (no proxy_protocol)\n", sbPort))
		}
		sb.WriteString("    server {\n")
		if useSocket {
			sb.WriteString(fmt.Sprintf("        listen unix:%s ssl proxy_protocol;\n", socket))
		} else {
			sb.WriteString(fmt.Sprintf("        listen 127.0.0.1:%s ssl proxy_protocol;\n", port))
			if rs.EnableIpv6 || globalIPv6 {
				sb.WriteString(fmt.Sprintf("        listen [::1]:%s ssl proxy_protocol;\n", port))
			}
		}
		// sing-box 入口：只支持 host:port，且不发 PROXY protocol → 单独一条不带 PP 的 listen。
		if sbPort != "" {
			sb.WriteString(fmt.Sprintf("        listen 127.0.0.1:%s ssl;\n", sbPort))
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
		if useSocket {
			// AF_UNIX 的 peer 不是 IP，127.0.0.1 匹配不上 → 必须显式信任 unix:，
			// 否则 $remote_addr 恒为 "unix:"（与普通 socket 站点一致）。
			sb.WriteString("        set_real_ip_from unix:;\n")
		} else if rs.EnableIpv6 || globalIPv6 {
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
		sb.WriteString(fmt.Sprintf("        listen %s;\n", httpListen))
		if globalIPv6 {
			sb.WriteString(fmt.Sprintf("        listen %s;\n", httpListenV6))
		}
		sb.WriteString("        location / {\n")
		sb.WriteString("            default_type text/plain;\n")
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
			ensureRegularLogFile(filepath.Join(m.dataDir, "logs", "stdout_stream.log"))
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
			defaultListen := canonicalListenAddr(s.SNIListenAddr)
			if defaultListen == "" {
				defaultListen = "443"
			}
			sniByPort := make(map[string][]entity.NginxSite)
			listens := make([]string, 0, len(sniSites))
			for _, site := range sniSites {
				// 监听地址优先级：站点 Path → 站点 ListenPort → 全局 SNI 默认地址。
				// 统一 canonical 化（"" / "443" / "0.0.0.0:443" / ":443" 都归一成同一个 key），
				// 否则编辑一条规则就可能多出一个 listen 组，生成两份 SNI server。
				addr := canonicalListenAddr(site.Path)
				if addr == "" {
					addr = canonicalListenAddr(site.ListenPort)
				}
				if addr == "" {
					addr = defaultListen
				}
				if _, ok := sniByPort[addr]; !ok {
					listens = append(listens, addr)
				}
				sniByPort[addr] = append(sniByPort[addr], site)
			}
			// 固定顺序，避免 map 随机迭代导致每次生成的配置内容/顺序抖动。
			sort.Strings(listens)

			// 通配监听（"443"）与同端口的指定 IPv4 监听（"127.0.0.1:443"）不能共存：
			// nginx 会报 duplicate listen / bind() 冲突。通配优先，把指定 IP 组并入
			// 通配组（域名映射保留），避免历史数据里混用两种写法时生成两个 SNI 监听。
			wildcardPorts := make(map[string]bool)
			for _, addr := range listens {
				if !strings.Contains(addr, ":") {
					wildcardPorts[addr] = true
				}
			}
			mergedListens := make([]string, 0, len(listens))
			for _, addr := range listens {
				// 仅合并 IPv4 指定地址；IPv6（以 [ 开头）是独立监听，不冲突。
				if !strings.HasPrefix(addr, "[") && strings.Contains(addr, ":") {
					if p := listenPortOnly(addr); wildcardPorts[p] {
						log.Printf("[NginxConfig] SNI listen %s 与通配监听 %s 冲突，已并入通配组", addr, p)
						sniByPort[p] = append(sniByPort[p], sniByPort[addr]...)
						delete(sniByPort, addr)
						continue
					}
				}
				mergedListens = append(mergedListens, addr)
			}
			listens = mergedListens

			multiPort := len(listens) > 1
			// 同一端口的 [::]:port 通配 IPv6 监听只能出现一次：多个指定 IPv4 监听
			// （如 10.0.0.1:443 与 10.0.0.2:443）各自再追加 [::]:443 会被 nginx
			// 判为 duplicate listen options。
			v6Emitted := make(map[string]bool)
			for _, port := range listens {
				portSites := sniByPort[port]
				// 多个监听地址时，map / upstream 名字必须带端口后缀：否则每组都会写出
				// 同名的 `upstream direct_http` / `map ... $sni_backend`，
				// nginx -t 直接报 duplicate upstream "direct_http"。
				suffix := ""
				if multiPort {
					suffix = "_" + nginxUpstreamKey(port)
				}
				sniBackendVar := "$sni_backend" + suffix
				guardVar := "$sni_backend_guarded" + suffix
				fallbackUpstream := "sni_fallback" + suffix
				directUpstream := "direct_http" + suffix
				// 上游名同样按监听组区分：同一个 SNI 域名在不同监听地址上可能指向不同后端。
				sniUpstreamName := func(domain string) string {
					name := "sni_" + nginxUpstreamKey(domain)
					if multiPort {
						name += suffix
					}
					return name
				}

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
					upName := sniUpstreamName(domain)
					if streamUpstreamSeen[upName] {
						continue
					}
					streamUpstreamSeen[upName] = true
					sb.WriteString(fmt.Sprintf("upstream %s {\n", upName))
					for _, up := range strings.Split(target, ",") {
						up = strings.TrimSpace(up)
						if up != "" {
							sb.WriteString(fmt.Sprintf("    server %s;\n", up))
						}
					}
					sb.WriteString("}\n\n")
				}

				// Fallback upstream（SNI map 的 default：未知 SNI 的 TLS 流量）
				// 优先级：设置里显式选择的「默认后端」> fallback 规则 > 127.0.0.1:8443。
				var siteFallback string
				var fallbackPP bool
				for _, site := range portSites {
					if site.Domain == "fallback" || site.Domain == "" {
						siteFallback = site.SNITarget
						if siteFallback == "" {
							siteFallback = site.Upstream
						}
						if siteFallback == "" {
							localPort := site.LocalPort
							if localPort == "" {
								localPort = "8443"
							}
							siteFallback = fmt.Sprintf("127.0.0.1:%s", localPort)
						}
						fallbackPP = site.ProxyProtocol
						break
					}
				}
				fallbackTarget := strings.TrimSpace(s.StreamDefaultTarget)
				if fallbackTarget == "" {
					fallbackTarget = siteFallback
				}
				if fallbackTarget == "" {
					fallbackTarget = "127.0.0.1:8443"
				}
				if !streamUpstreamSeen[fallbackUpstream] {
					streamUpstreamSeen[fallbackUpstream] = true
					sb.WriteString(fmt.Sprintf("upstream %s {\n", fallbackUpstream))
					for _, up := range strings.Split(fallbackTarget, ",") {
						up = strings.TrimSpace(up)
						if up != "" {
							sb.WriteString(fmt.Sprintf("    server %s;\n", up))
						}
					}
					sb.WriteString("}\n\n")
				}

				// ssl_preread map
				sb.WriteString(fmt.Sprintf("# SNI routing on %s\n", port))
				sb.WriteString(fmt.Sprintf("map $ssl_preread_server_name %s {\n", sniBackendVar))
				// 合并监听组后可能出现同一域名两条规则：map 里重复 key 会让
				// nginx -t 直接报 duplicate parameter，这里只保留第一条。
				mapSeen := make(map[string]bool, len(portSites))
				for _, site := range portSites {
					domain := site.Domain
					if domain == "fallback" || domain == "" || mapSeen[domain] {
						continue
					}
					mapSeen[domain] = true
					sb.WriteString(fmt.Sprintf("    %s %s;\n", domain, sniUpstreamName(domain)))
				}
				sb.WriteString(fmt.Sprintf("    default %s;\n", fallbackUpstream))
				sb.WriteString("}\n\n")

				// 非 TLS 流量（没有 ClientHello / ssl_preread_protocol 为空）单独分流。
				// 历史实现把非 TLS 和"未知 SNI"一起丢给 sni_fallback（通常是 REALITY 入站），
				// 探测流量会落到 REALITY；这里按协议类型先摘出去，语义等同生产 nginx 的
				// `map $ssl_preread_protocol $stream_map_guarded`。
				backendVar := sniBackendVar
				if strings.TrimSpace(s.StreamNonTLSTarget) != "" {
					sb.WriteString(fmt.Sprintf("upstream %s {\n", directUpstream))
					for _, up := range strings.Split(s.StreamNonTLSTarget, ",") {
						up = strings.TrimSpace(up)
						if up != "" {
							sb.WriteString(fmt.Sprintf("    server %s;\n", up))
						}
					}
					sb.WriteString("}\n\n")
					sb.WriteString(fmt.Sprintf("map $ssl_preread_protocol %s {\n", guardVar))
					sb.WriteString(fmt.Sprintf("    \"\"      %s;\n", directUpstream))
					sb.WriteString(fmt.Sprintf("    default %s;\n", sniBackendVar))
					sb.WriteString("}\n\n")
					backendVar = guardVar
				}

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
				// IPv6 监听：全局开关 / 全局 IPv6 / 本组任一条规则开了 IPv6，任一成立即追加。
				// 若监听地址本身已是具体 IPv6（如 [::1]:443），不再追加 [::]:端口（会冲突）。
				groupIPv6 := s.StreamIPv6 || globalIPv6
				if !groupIPv6 {
					for _, site := range portSites {
						if site.EnableIpv6 {
							groupIPv6 = true
							break
						}
					}
				}
				sb.WriteString("server {\n")
				sb.WriteString(fmt.Sprintf("    listen %s reuseport fastopen=4096 so_keepalive=on backlog=4096;\n", port))
				if groupIPv6 && !listenHostIsIPv6(port) {
					// port 可能是 "203.0.113.10:443"（只监听指定 IPv4）——
					// IPv6 行必须只取端口，否则会生成 listen [::]:203.0.113.10:443 这种非法写法。
					p6 := listenPortOnly(port)
					// Stream IPv6 监听地址可写成 "[2001:db8::1]:8443"：带端口时用该端口。
					v6Listen := ipv6ListenFor(s.StreamIPv6ListenAddr, p6)
					if k := listenPortOnly(v6Listen); !v6Emitted[k] {
						v6Emitted[k] = true
						sb.WriteString(fmt.Sprintf("    listen %s reuseport fastopen=4096 so_keepalive=on backlog=4096;\n", v6Listen))
					}
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
				sb.WriteString(fmt.Sprintf("    proxy_pass %s;\n", backendVar))
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
						sb.WriteString(fmt.Sprintf("    listen %s udp ipv6only=on so_keepalive=on;\n", ipv6ListenFor(s.StreamIPv6ListenAddr, port)))
					}
				} else {
					sb.WriteString(fmt.Sprintf("    listen %s so_keepalive=on;\n", port))
					if s.StreamIPv6 || globalIPv6 {
						sb.WriteString(fmt.Sprintf("    listen %s ipv6only=on so_keepalive=on;\n", ipv6ListenFor(s.StreamIPv6ListenAddr, port)))
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

// listenPortOnly 从 "443" / "203.0.113.10:443" / "[::1]:443" 中取出纯端口，
// 供 IPv6 listen 复用（IPv6 行只接受 [::]:<port>）。
// canonicalListenAddr 归一 SNI/L4 监听地址：
//   - "" / ":443" / "0.0.0.0:443" / "*:443" / "[::]:443" → "443"（通配监听统一成纯端口）
//   - "127.0.0.1:443" → "127.0.0.1:443"
//   - "[::1]:443" → "[::1]:443"
//
// 目的：同一监听的等价写法必须落进同一个 SNI 分组，避免生成两个 listen 443。
func canonicalListenAddr(addr string) string {
	a := strings.TrimSpace(addr)
	if a == "" {
		return ""
	}
	// ":443" → "443"；但裸 IPv6（"::1" / "::"）不能剥前导冒号，否则会变成 "1" / ""。
	if strings.HasPrefix(a, ":") && !strings.HasPrefix(a, "::") {
		a = strings.TrimPrefix(a, ":")
	}
	if a == "" {
		return ""
	}
	// 纯端口
	if _, err := strconv.Atoi(a); err == nil {
		return a
	}
	host, port, err := net.SplitHostPort(a)
	if err != nil || port == "" {
		return a
	}
	if host == "" || host == "0.0.0.0" || host == "*" || host == "::" {
		return port
	}
	return net.JoinHostPort(host, port)
}

// ipv6ListenHost 归一 IPv6 监听地址，只取主机部分，默认 "::"。
// 支持 "::" / "[::]" / "::1" / "[::1]" / "2001:db8::1" / "[2001:db8::1]:443"。
// 非 IPv6 写法（含 IPv4 / 纯端口）一律回退 "::"，避免生成非法 listen。
func ipv6ListenHost(addr string) string {
	c := canonicalListenAddr(addr)
	if c == "" {
		return "::"
	}
	if host, _, err := net.SplitHostPort(c); err == nil && host != "" {
		c = host
	}
	c = strings.Trim(strings.TrimSpace(c), "[]")
	if !strings.Contains(c, ":") {
		return "::"
	}
	return c
}

// ipv6Listen 生成 nginx 的 IPv6 监听地址（形如 [::1]:443）。
// port 允许传纯端口（"443"）或带 IPv4 地址（"127.0.0.1:443"）——必须只取端口，
// 否则会生成 listen [::1]:127.0.0.1:443 这种非法写法（nginx: invalid port）。
func ipv6Listen(host, port string) string {
	return fmt.Sprintf("[%s]:%s", ipv6ListenHost(host), listenPortOnly(port))
}

// globalListen 归一全局监听地址（v4）：空 → 默认值；"80" / "127.0.0.1:80" 均可。
func globalListen(addr, def string) string {
	a := strings.TrimSpace(addr)
	if a == "" {
		a = def
	}
	if c := canonicalListenAddr(a); c != "" {
		return c
	}
	return def
}

// globalListenV6 归一全局 IPv6 监听地址：支持 "[::]:80" / "[::1]:80" / "::1"（只填主机）。
// 只填主机时端口跟随同协议 v4 监听（fallbackAddr）。
func globalListenV6(addr, fallbackAddr string) string {
	host := ipv6ListenHost(fallbackAddr)
	port := listenPortOnly(fallbackAddr)
	a := strings.TrimSpace(addr)
	if a != "" {
		if h, p, err := net.SplitHostPort(canonicalListenAddr(a)); err == nil && p != "" {
			return fmt.Sprintf("[%s]:%s", ipv6ListenHost(h), listenPortOnly(p))
		}
		if strings.Contains(strings.Trim(a, "[]"), ":") {
			host = ipv6ListenHost(a)
		}
	}
	return fmt.Sprintf("[%s]:%s", host, port)
}

// v6ListenWith 生成站点级 IPv6 监听：
//   - 站点显式 ipv6_listen_addr（可带端口）完整优先；
//   - 只写主机时端口跟随站点；
//   - 未配置时用对应协议（HTTP/HTTPS）的全局 IPv6 主机；
//     站点未显式 listen_port 时端口也用全局 IPv6 地址里的端口，显式时跟随站点端口。
func v6ListenWith(globalV6, siteV6, sitePort string, sitePortExplicit bool) string {
	a := strings.TrimSpace(siteV6)
	if a != "" {
		if h, p, err := net.SplitHostPort(canonicalListenAddr(a)); err == nil && p != "" {
			return fmt.Sprintf("[%s]:%s", ipv6ListenHost(h), listenPortOnly(p))
		}
		return fmt.Sprintf("[%s]:%s", ipv6ListenHost(a), listenPortOnly(sitePort))
	}
	if sitePortExplicit {
		return fmt.Sprintf("[%s]:%s", ipv6ListenHost(globalV6), listenPortOnly(sitePort))
	}
	if strings.TrimSpace(globalV6) != "" {
		return canonicalIPv6Listen(globalV6)
	}
	return fmt.Sprintf("[%s]:%s", ipv6ListenHost(globalV6), listenPortOnly(sitePort))
}

// canonicalIPv6Listen 归一 "[host]:port"；非法时退化为 [::]:port。
func canonicalIPv6Listen(addr string) string {
	if h, p, err := net.SplitHostPort(canonicalListenAddr(addr)); err == nil && p != "" {
		return fmt.Sprintf("[%s]:%s", ipv6ListenHost(h), listenPortOnly(p))
	}
	return fmt.Sprintf("[%s]:%s", ipv6ListenHost(addr), listenPortOnly(addr))
}

// ipv6ListenFor 生成站点/组使用的 IPv6 listen：addr 可只写主机（"::1"）或带端口
// （"[::1]:8443"）。带端口时用该端口，否则用 fallbackPort。
func ipv6ListenFor(addr, fallbackPort string) string {
	host, port := ipv6HostPort(addr, fallbackPort)
	return fmt.Sprintf("[%s]:%s", host, listenPortOnly(port))
}

func ipv6HostPort(addr, fallbackPort string) (string, string) {
	if strings.TrimSpace(addr) == "" {
		return "::", fallbackPort
	}
	c := canonicalListenAddr(addr)
	if host, port, err := net.SplitHostPort(c); err == nil && port != "" {
		return ipv6ListenHost(host), port
	}
	return ipv6ListenHost(c), fallbackPort
}

// listenHostIsIPv6 reports whether the listen address already pins a specific
// IPv6 host (e.g. "[::1]:443"). In that case adding "[::]:port" would conflict.
func listenHostIsIPv6(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	return strings.Contains(host, ":")
}

// normalizeNginxUnixSocket 归一 unix socket 监听路径：
// 支持 "/dev/shm/nginx.sock"、"unix:/dev/shm/nginx.sock"、"unix:///dev/shm/nginx.sock"。
// 返回空串表示「不是 unix 监听」。
func normalizeNginxUnixSocket(addr string) string {
	s := strings.TrimSpace(addr)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "unix://") {
		s = strings.TrimPrefix(s, "unix://")
	} else if strings.HasPrefix(s, "unix:") {
		s = strings.TrimPrefix(s, "unix:")
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// 纯端口/IP:端口 不是 socket 路径（防止把 "9443" 当 socket）。
	if !strings.Contains(s, "/") {
		return ""
	}
	return s
}

func listenPortOnly(listen string) string {
	listen = strings.TrimSpace(listen)
	if listen == "" {
		return "443"
	}
	if _, port, err := net.SplitHostPort(listen); err == nil && port != "" {
		return port
	}
	if i := strings.LastIndex(listen, ":"); i >= 0 && i+1 < len(listen) {
		if p := listen[i+1:]; p != "" && !strings.Contains(p, ":") {
			return p
		}
	}
	return listen
}
