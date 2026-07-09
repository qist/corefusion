package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
)

// NginxSettings contains all Nginx global configuration.
type NginxSettings struct {
	// Worker
	WorkerProcesses    string `json:"worker_processes"`
	WorkerRlimitNofile string `json:"worker_rlimit_nofile"`
	WorkerPriority     string `json:"worker_priority"`
	WorkerShutdownTimeout string `json:"worker_shutdown_timeout"`
	Daemon             bool   `json:"daemon"`
	MasterProcess      bool   `json:"master_process"`

	// Default ports
	DefaultHttpPort  string `json:"default_http_port"`
	DefaultHttpsPort string `json:"default_https_port"`

	// Events
	WorkerConnections string `json:"worker_connections"`
	MultiAccept       bool   `json:"multi_accept"`
	UseEpoll          bool   `json:"use_epoll"`

	// Logs
	ErrorLog            bool   `json:"error_log"`
	ErrorLogPath        string `json:"error_log_path"`
	ErrorLogLevel       string `json:"error_log_level"`
	AccessLog           bool   `json:"access_log"`
	AccessLogPath       string `json:"access_log_path"`
	LogFormat           string `json:"log_format"`
	DisableSymlinks     bool   `json:"disable_symlinks"`
	ClientHeaderBufferSize string `json:"client_header_buffer_size"`
	LargeClientHeaderBuffers string `json:"large_client_header_buffers"`
	ClientHeaderTimeout string `json:"client_header_timeout"`
	ClientBodyTimeout   string `json:"client_body_timeout"`
	ClientMaxBodySize   string `json:"client_max_body_size"`
	ClientBodyBufferSize string `json:"client_body_buffer_size"`
	ServerTokens        bool   `json:"server_tokens"`
	Sendfile            bool   `json:"sendfile"`
	LogNotFound         bool   `json:"log_not_found"`
	SendfileMaxChunk    string `json:"sendfile_max_chunk"`
	TcpNopush           bool   `json:"tcp_nopush"`
	TcpNodelay          bool   `json:"tcp_nodelay"`
	ProxyStore          bool   `json:"proxy_store"`

	// HTTP/2 & HTTP/3
	Http2ChunkSize           string `json:"http2_chunk_size"`
	Http2BodyPrereadSize     string `json:"http2_body_preread_size"`
	Http2MaxConcurrentStreams string `json:"http2_max_concurrent_streams"`
	Http2RecvBufferSize      string `json:"http2_recv_buffer_size"`
	Http3                    bool   `json:"http3"`
	Http3Hq                  bool   `json:"http3_hq"`
	Http3MaxConcurrentStreams string `json:"http3_max_concurrent_streams"`
	Http3StreamBufferSize    string `json:"http3_stream_buffer_size"`
	QuicRetry                bool   `json:"quic_retry"`
	QuicActiveConnectionIdLimit string `json:"quic_active_connection_id_limit"`
	QuicGso                  bool   `json:"quic_gso"`

	// Performance
	KeepaliveRequests string `json:"keepalive_requests"`
	KeepaliveTimeout  string `json:"keepalive_timeout"`
	Aio               bool   `json:"aio"`
	AioWrite          bool   `json:"aio_write"`
	TypesHashMaxSize  string `json:"types_hash_max_size"`
	ServerNamesHashMaxSize    string `json:"server_names_hash_max_size"`
	ServerNamesHashBucketSize string `json:"server_names_hash_bucket_size"`
	ProxyHeadersHashMaxSize   string `json:"proxy_headers_hash_max_size"`
	ProxyHeadersHashBucketSize string `json:"proxy_headers_hash_bucket_size"`
	VariablesHashBucketSize   string `json:"variables_hash_bucket_size"`
	VariablesHashMaxSize      string `json:"variables_hash_max_size"`
	UnderscoresInHeaders      bool   `json:"underscores_in_headers"`
	IgnoreInvalidHeaders      bool   `json:"ignore_invalid_headers"`
	ResetTimedoutConnection   bool   `json:"reset_timedout_connection"`
	SendTimeout               string `json:"send_timeout"`
	Charset                   string `json:"charset"`

	// Proxy
	ProxyIgnoreClientAbort bool   `json:"proxy_ignore_client_abort"`
	ProxyConnectTimeout    string `json:"proxy_connect_timeout"`
	ProxyReadTimeout       string `json:"proxy_read_timeout"`
	ProxySendTimeout       string `json:"proxy_send_timeout"`
	ProxyBufferSize        string `json:"proxy_buffer_size"`
	ProxyBuffers           string `json:"proxy_buffers"`
	ProxyBusyBuffersSize   string `json:"proxy_busy_buffers_size"`

	// FastCGI
	FastcgiConnectTimeout     string `json:"fastcgi_connect_timeout"`
	FastcgiSendTimeout        string `json:"fastcgi_send_timeout"`
	FastcgiReadTimeout        string `json:"fastcgi_read_timeout"`
	FastcgiBufferSize         string `json:"fastcgi_buffer_size"`
	FastcgiBuffers            string `json:"fastcgi_buffers"`
	FastcgiBusyBuffersSize    string `json:"fastcgi_busy_buffers_size"`
	FastcgiTempFileWriteSize  string `json:"fastcgi_temp_file_write_size"`
	FastcgiInterceptErrors    bool   `json:"fastcgi_intercept_errors"`
	FastcgiIgnoreClientAbort  bool   `json:"fastcgi_ignore_client_abort"`

	// Gzip
	Gzip            bool   `json:"gzip"`
	GzipHttpVersion string `json:"gzip_http_version"`
	GzipBuffers     string `json:"gzip_buffers"`
	GzipDisable     string `json:"gzip_disable"`
	GzipProxied     string `json:"gzip_proxied"`
	GzipMinLength   string `json:"gzip_min_length"`
	GzipCompLevel   string `json:"gzip_comp_level"`
	GzipTypes       string `json:"gzip_types"`
	GzipVary        bool   `json:"gzip_vary"`

	// Brotli
	Brotli          bool   `json:"brotli"`
	BrotliCompLevel string `json:"brotli_comp_level"`
	BrotliBuffers   string `json:"brotli_buffers"`
	BrotliMinLength string `json:"brotli_min_length"`
	BrotliStatic    bool   `json:"brotli_static"`
	BrotliWindow    string `json:"brotli_window"`
	BrotliTypes     string `json:"brotli_types"`

	// Limit
	LimitConnZone string `json:"limit_conn_zone"`
	LimitConn     string `json:"limit_conn"`

	// Vhost traffic status
	VhostTrafficStatus bool `json:"vhost_traffic_status"`

	// Stream (SNI + L4)
	StreamSNIEnabled     bool   `json:"stream_sni_enabled"`
	StreamSNICacheSize   string `json:"stream_sni_cache_size"`
	StreamIPv6           bool   `json:"stream_ipv6"`
	StreamProxyProtocol  bool   `json:"stream_proxy_protocol"`
	StreamConnectTimeout string `json:"stream_connect_timeout"`
	StreamTimeout        string `json:"stream_timeout"`
	StreamLog            bool   `json:"stream_log"`
	StreamLogPath        string `json:"stream_log_path"`
	StreamLogFormat      string `json:"stream_log_format"`

	// Custom
	CustomConfig string `json:"custom_config"`
}

// DefaultNginxSettings returns default Nginx configuration.
func DefaultNginxSettings() *NginxSettings {
	isWindows := runtime.GOOS == "windows"
	return &NginxSettings{
		WorkerProcesses:    "auto",
		WorkerRlimitNofile: "1024000",
		WorkerPriority:     "1",
		WorkerShutdownTimeout: "10s",
		Daemon:             false,
		MasterProcess:      true,
		DefaultHttpPort:    "80",
		DefaultHttpsPort:   "443",
		WorkerConnections:  "1024000",
		MultiAccept:        true,
		UseEpoll:           !isWindows,
		ErrorLog:           true,
		ErrorLogPath:       "logs/error.log",
		ErrorLogLevel:      "warn",
		AccessLog:          true,
		AccessLogPath:      "logs/access.log",
		ClientHeaderBufferSize: "1024k",
		LargeClientHeaderBuffers: "4 128k",
		ClientHeaderTimeout: "90",
		ClientBodyTimeout:   "90",
		ClientMaxBodySize:   "2m",
		ClientBodyBufferSize: "128k",
		ServerTokens:        false,
		Sendfile:            true,
		LogNotFound:         false,
		SendfileMaxChunk:    "512k",
		TcpNopush:           true,
		TcpNodelay:          true,
		ProxyStore:          false,
		Http2ChunkSize:           "64k",
		Http2BodyPrereadSize:     "128k",
		Http2MaxConcurrentStreams: "512",
		Http2RecvBufferSize:      "512k",
		Http3:                    !isWindows,
		Http3Hq:                  !isWindows,
		Http3MaxConcurrentStreams: "256",
		Http3StreamBufferSize:    "256k",
		QuicRetry:                !isWindows,
		QuicActiveConnectionIdLimit: "8",
		QuicGso:                  !isWindows,
		KeepaliveRequests: "10000",
		KeepaliveTimeout:  "75s",
		Aio:               true,
		AioWrite:          true,
		TypesHashMaxSize:  "2048",
		ServerNamesHashMaxSize:    "1024",
		ServerNamesHashBucketSize: "128",
		ProxyHeadersHashMaxSize:   "512",
		ProxyHeadersHashBucketSize: "64",
		VariablesHashBucketSize:   "128",
		VariablesHashMaxSize:      "2048",
		UnderscoresInHeaders:      true,
		IgnoreInvalidHeaders:      true,
		ResetTimedoutConnection:   true,
		SendTimeout:               "90",
		Charset:                   "UTF-8",
		ProxyIgnoreClientAbort: true,
		ProxyConnectTimeout:    "90",
		ProxyReadTimeout:       "90",
		ProxySendTimeout:       "90",
		ProxyBufferSize:        "64k",
		ProxyBuffers:           "4 64k",
		ProxyBusyBuffersSize:   "128k",
		FastcgiConnectTimeout:     "30",
		FastcgiSendTimeout:        "30",
		FastcgiReadTimeout:        "30",
		FastcgiBufferSize:         "64k",
		FastcgiBuffers:            "4 32k",
		FastcgiBusyBuffersSize:    "64k",
		FastcgiTempFileWriteSize:  "64k",
		FastcgiInterceptErrors:    true,
		FastcgiIgnoreClientAbort:  true,
		Gzip:            true,
		GzipHttpVersion: "1.0",
		GzipBuffers:     "4 16k",
		GzipDisable:     "msie6",
		GzipProxied:     "any",
		GzipMinLength:   "1000",
		GzipCompLevel:   "6",
		GzipTypes:       "text/plain text/css text/xml text/javascript application/json application/x-javascript application/javascript application/xml application/xml+rss",
		GzipVary:        true,
		Brotli:          true,
		BrotliCompLevel: "6",
		BrotliBuffers:   "16 8k",
		BrotliMinLength: "20",
		BrotliStatic:    false,
		BrotliWindow:    "512k",
		BrotliTypes:     "text/plain text/css text/xml text/javascript application/json application/x-javascript application/javascript application/xml application/xml+rss",
		LimitConnZone: "$binary_remote_addr zone=addr:5m",
		LimitConn:     "addr 2000",
		VhostTrafficStatus: !isWindows,
		// Stream defaults
		StreamSNIEnabled:     false,
		StreamSNICacheSize:   "64",
		StreamIPv6:           false,
		StreamProxyProtocol:  false,
		StreamConnectTimeout: "1s",
		StreamTimeout:        "3s",
		StreamLog:            true,
		StreamLogPath:        "logs/stream.log",
		StreamLogFormat:      "basic",
	}
}

// perNodeNginxSettingsKey returns the settings key for a server ID.
// Local node (serverID=1) uses the legacy global key for backward compatibility.
// Remote nodes use a per-node key.
func perNodeNginxSettingsKey(serverID int64) string {
	if serverID <= 1 {
		return "proxy.nginx_settings"
	}
	return fmt.Sprintf("proxy.nginx_settings.%d", serverID)
}

// GetNginxSettings reads Nginx settings from database (local node only).
func (m *Manager) GetNginxSettings(ctx context.Context) (*NginxSettings, error) {
	return m.GetNginxSettingsForServer(ctx, 1)
}

// GetNginxSettingsForServer reads Nginx settings for a specific server.
// It reads the per-node key first, and falls back to the global key for backward compatibility.
func (m *Manager) GetNginxSettingsForServer(ctx context.Context, serverID int64) (*NginxSettings, error) {
	settings := DefaultNginxSettings()
	if m.settingStore == nil {
		return settings, nil
	}

	// Try per-node key first
	key := perNodeNginxSettingsKey(serverID)
	v, err := m.settingStore.GetByKey(ctx, key)
	if err != nil || v == "" {
		// Fallback to global key for backward compatibility
		if serverID > 1 {
			v, err = m.settingStore.GetByKey(ctx, "proxy.nginx_settings")
			if err != nil || v == "" {
				return settings, nil
			}
		} else {
			return settings, nil
		}
	}

	// Unmarshal into a temp struct, then merge non-empty values onto defaults
	var saved NginxSettings
	if err := json.Unmarshal([]byte(v), &saved); err != nil {
		return settings, nil
	}
	mergeNginxSettings(settings, &saved)
	return settings, nil
}

// mergeNginxSettings copies non-empty/non-zero values from src to dst (preserving defaults).
func mergeNginxSettings(dst *NginxSettings, src *NginxSettings) {
	// String fields: only overwrite if src is non-empty
	if src.WorkerProcesses != "" { dst.WorkerProcesses = src.WorkerProcesses }
	if src.WorkerRlimitNofile != "" { dst.WorkerRlimitNofile = src.WorkerRlimitNofile }
	if src.WorkerPriority != "" { dst.WorkerPriority = src.WorkerPriority }
	if src.WorkerShutdownTimeout != "" { dst.WorkerShutdownTimeout = src.WorkerShutdownTimeout }
	if src.DefaultHttpPort != "" { dst.DefaultHttpPort = src.DefaultHttpPort }
	if src.DefaultHttpsPort != "" { dst.DefaultHttpsPort = src.DefaultHttpsPort }
	if src.WorkerConnections != "" { dst.WorkerConnections = src.WorkerConnections }
	if src.ErrorLogPath != "" { dst.ErrorLogPath = src.ErrorLogPath }
	if src.ErrorLogLevel != "" { dst.ErrorLogLevel = src.ErrorLogLevel }
	if src.AccessLogPath != "" { dst.AccessLogPath = src.AccessLogPath }
	if src.ClientHeaderBufferSize != "" { dst.ClientHeaderBufferSize = src.ClientHeaderBufferSize }
	if src.LargeClientHeaderBuffers != "" { dst.LargeClientHeaderBuffers = src.LargeClientHeaderBuffers }
	if src.ClientHeaderTimeout != "" { dst.ClientHeaderTimeout = src.ClientHeaderTimeout }
	if src.ClientBodyTimeout != "" { dst.ClientBodyTimeout = src.ClientBodyTimeout }
	if src.ClientMaxBodySize != "" { dst.ClientMaxBodySize = src.ClientMaxBodySize }
	if src.ClientBodyBufferSize != "" { dst.ClientBodyBufferSize = src.ClientBodyBufferSize }
	if src.SendfileMaxChunk != "" { dst.SendfileMaxChunk = src.SendfileMaxChunk }
	if src.Http2ChunkSize != "" { dst.Http2ChunkSize = src.Http2ChunkSize }
	if src.Http2BodyPrereadSize != "" { dst.Http2BodyPrereadSize = src.Http2BodyPrereadSize }
	if src.Http2MaxConcurrentStreams != "" { dst.Http2MaxConcurrentStreams = src.Http2MaxConcurrentStreams }
	if src.Http2RecvBufferSize != "" { dst.Http2RecvBufferSize = src.Http2RecvBufferSize }
	if src.Http3MaxConcurrentStreams != "" { dst.Http3MaxConcurrentStreams = src.Http3MaxConcurrentStreams }
	if src.Http3StreamBufferSize != "" { dst.Http3StreamBufferSize = src.Http3StreamBufferSize }
	if src.QuicActiveConnectionIdLimit != "" { dst.QuicActiveConnectionIdLimit = src.QuicActiveConnectionIdLimit }
	if src.KeepaliveRequests != "" { dst.KeepaliveRequests = src.KeepaliveRequests }
	if src.KeepaliveTimeout != "" { dst.KeepaliveTimeout = src.KeepaliveTimeout }
	if src.TypesHashMaxSize != "" { dst.TypesHashMaxSize = src.TypesHashMaxSize }
	if src.ServerNamesHashMaxSize != "" { dst.ServerNamesHashMaxSize = src.ServerNamesHashMaxSize }
	if src.ServerNamesHashBucketSize != "" { dst.ServerNamesHashBucketSize = src.ServerNamesHashBucketSize }
	if src.ProxyHeadersHashMaxSize != "" { dst.ProxyHeadersHashMaxSize = src.ProxyHeadersHashMaxSize }
	if src.ProxyHeadersHashBucketSize != "" { dst.ProxyHeadersHashBucketSize = src.ProxyHeadersHashBucketSize }
	if src.VariablesHashBucketSize != "" { dst.VariablesHashBucketSize = src.VariablesHashBucketSize }
	if src.VariablesHashMaxSize != "" { dst.VariablesHashMaxSize = src.VariablesHashMaxSize }
	if src.SendTimeout != "" { dst.SendTimeout = src.SendTimeout }
	if src.Charset != "" { dst.Charset = src.Charset }
	if src.ProxyConnectTimeout != "" { dst.ProxyConnectTimeout = src.ProxyConnectTimeout }
	if src.ProxyReadTimeout != "" { dst.ProxyReadTimeout = src.ProxyReadTimeout }
	if src.ProxySendTimeout != "" { dst.ProxySendTimeout = src.ProxySendTimeout }
	if src.ProxyBufferSize != "" { dst.ProxyBufferSize = src.ProxyBufferSize }
	if src.ProxyBuffers != "" { dst.ProxyBuffers = src.ProxyBuffers }
	if src.ProxyBusyBuffersSize != "" { dst.ProxyBusyBuffersSize = src.ProxyBusyBuffersSize }
	if src.FastcgiConnectTimeout != "" { dst.FastcgiConnectTimeout = src.FastcgiConnectTimeout }
	if src.FastcgiSendTimeout != "" { dst.FastcgiSendTimeout = src.FastcgiSendTimeout }
	if src.FastcgiReadTimeout != "" { dst.FastcgiReadTimeout = src.FastcgiReadTimeout }
	if src.FastcgiBufferSize != "" { dst.FastcgiBufferSize = src.FastcgiBufferSize }
	if src.FastcgiBuffers != "" { dst.FastcgiBuffers = src.FastcgiBuffers }
	if src.FastcgiBusyBuffersSize != "" { dst.FastcgiBusyBuffersSize = src.FastcgiBusyBuffersSize }
	if src.FastcgiTempFileWriteSize != "" { dst.FastcgiTempFileWriteSize = src.FastcgiTempFileWriteSize }
	if src.GzipHttpVersion != "" { dst.GzipHttpVersion = src.GzipHttpVersion }
	if src.GzipBuffers != "" { dst.GzipBuffers = src.GzipBuffers }
	if src.GzipDisable != "" { dst.GzipDisable = src.GzipDisable }
	if src.GzipProxied != "" { dst.GzipProxied = src.GzipProxied }
	if src.GzipMinLength != "" { dst.GzipMinLength = src.GzipMinLength }
	if src.GzipCompLevel != "" { dst.GzipCompLevel = src.GzipCompLevel }
	if src.GzipTypes != "" { dst.GzipTypes = src.GzipTypes }
	if src.BrotliCompLevel != "" { dst.BrotliCompLevel = src.BrotliCompLevel }
	if src.BrotliBuffers != "" { dst.BrotliBuffers = src.BrotliBuffers }
	if src.BrotliMinLength != "" { dst.BrotliMinLength = src.BrotliMinLength }
	if src.BrotliWindow != "" { dst.BrotliWindow = src.BrotliWindow }
	if src.BrotliTypes != "" { dst.BrotliTypes = src.BrotliTypes }
	if src.LimitConnZone != "" { dst.LimitConnZone = src.LimitConnZone }
	if src.LimitConn != "" { dst.LimitConn = src.LimitConn }
	if src.CustomConfig != "" { dst.CustomConfig = src.CustomConfig }
	if src.StreamSNICacheSize != "" { dst.StreamSNICacheSize = src.StreamSNICacheSize }
	if src.StreamConnectTimeout != "" { dst.StreamConnectTimeout = src.StreamConnectTimeout }
	if src.StreamTimeout != "" { dst.StreamTimeout = src.StreamTimeout }
	if src.StreamLogPath != "" { dst.StreamLogPath = src.StreamLogPath }
	if src.StreamLogFormat != "" { dst.StreamLogFormat = src.StreamLogFormat }
	// Bool fields: always copy (can't distinguish "not set" from "false")
	dst.Daemon = src.Daemon
	dst.MasterProcess = src.MasterProcess
	dst.MultiAccept = src.MultiAccept
	dst.UseEpoll = src.UseEpoll
	dst.ErrorLog = src.ErrorLog
	dst.AccessLog = src.AccessLog
	dst.DisableSymlinks = src.DisableSymlinks
	dst.ServerTokens = src.ServerTokens
	dst.Sendfile = src.Sendfile
	dst.LogNotFound = src.LogNotFound
	dst.TcpNopush = src.TcpNopush
	dst.TcpNodelay = src.TcpNodelay
	dst.ProxyStore = src.ProxyStore
	dst.Http3 = src.Http3
	dst.Http3Hq = src.Http3Hq
	dst.QuicRetry = src.QuicRetry
	dst.QuicGso = src.QuicGso
	dst.Aio = src.Aio
	dst.AioWrite = src.AioWrite
	dst.UnderscoresInHeaders = src.UnderscoresInHeaders
	dst.IgnoreInvalidHeaders = src.IgnoreInvalidHeaders
	dst.ResetTimedoutConnection = src.ResetTimedoutConnection
	dst.ProxyIgnoreClientAbort = src.ProxyIgnoreClientAbort
	dst.FastcgiInterceptErrors = src.FastcgiInterceptErrors
	dst.FastcgiIgnoreClientAbort = src.FastcgiIgnoreClientAbort
	dst.Gzip = src.Gzip
	dst.GzipVary = src.GzipVary
	dst.Brotli = src.Brotli
	dst.BrotliStatic = src.BrotliStatic
	dst.VhostTrafficStatus = src.VhostTrafficStatus
	dst.StreamSNIEnabled = src.StreamSNIEnabled
	dst.StreamIPv6 = src.StreamIPv6
	dst.StreamProxyProtocol = src.StreamProxyProtocol
	dst.StreamLog = src.StreamLog
}

// SaveNginxSettings saves Nginx settings to database (local node only).
func (m *Manager) SaveNginxSettings(ctx context.Context, settings *NginxSettings) error {
	return m.SaveNginxSettingsForServer(ctx, 1, settings)
}

// SaveNginxSettingsForServer saves Nginx settings for a specific server.
func (m *Manager) SaveNginxSettingsForServer(ctx context.Context, serverID int64, settings *NginxSettings) error {
	if m.settingStore == nil {
		return nil
	}
	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	key := perNodeNginxSettingsKey(serverID)
	return m.settingStore.Set(ctx, key, string(data))
}
