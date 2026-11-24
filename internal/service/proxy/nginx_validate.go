package proxy

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	"corefusion/internal/model/entity"
)

// 安全审计 v7 M-6：nginx 配置由管理端参数拼接生成，参数里出现换行、分号或
// 不配对的花括号就能「结束当前指令 / 闭合 http{} 作用域」后注入任意指令或
// 任意路径日志，把「配置编辑」升级为持久化 root 控制。这里做最小校验：
//   - 所有字符串字段：禁止换行/NUL（换行是最直接的指令注入）；
//   - 裸写进指令的标量：额外禁止 `;` `{` `}`；
//   - 自定义片段：允许换行，但花括号必须配平（不能逃逸当前作用域）。

// validateNginxScalar 校验会直接拼进单条 nginx 指令的标量参数。
func validateNginxScalar(field, v string) error {
	if v == "" {
		return nil
	}
	if strings.ContainsAny(v, "\r\n\x00") {
		return fmt.Errorf("nginx 参数 %s 不能包含换行或 NUL", field)
	}
	if strings.ContainsAny(v, ";{}") {
		return fmt.Errorf("nginx 参数 %s 不能包含 ';' '{' '}'（会拆成额外指令）", field)
	}
	return nil
}

// validateNginxSnippet 校验多行自定义片段：允许换行，但花括号必须配平。
func validateNginxSnippet(field, v string) error {
	if strings.ContainsRune(v, '\x00') {
		return fmt.Errorf("nginx 参数 %s 不能包含 NUL", field)
	}
	balance := 0
	inSingle, inDouble := false, false
	for _, r := range v {
		switch {
		case inSingle:
			if r == '\'' {
				inSingle = false
			}
		case inDouble:
			if r == '"' {
				inDouble = false
			}
		case r == '\'':
			inSingle = true
		case r == '"':
			inDouble = true
		case r == '{':
			balance++
		case r == '}':
			balance--
			if balance < 0 {
				return fmt.Errorf("nginx 参数 %s 花括号不配平（出现多余的 '}'）", field)
			}
		}
	}
	if balance != 0 {
		return fmt.Errorf("nginx 参数 %s 花括号不配平（缺少 %d 个 '}'）", field, balance)
	}
	return nil
}

// validateSingleLineFields 对结构体所有字符串字段做单行校验：禁止换行/NUL 与
// `;` `{` `}` —— 这些字符能把单个参数拆成额外指令。skip 里的多行片段
// （custom_config / server_config / location_config）由 validateNginxSnippet
// 单独校验（允许换行，但花括号必须配平）。
func validateSingleLineFields(v interface{}, label string, skip ...string) error {
	skipSet := make(map[string]bool, len(skip))
	for _, name := range skip {
		skipSet[name] = true
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return nil
	}
	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return nil
	}
	rt := rv.Type()
	for i := 0; i < rv.NumField(); i++ {
		f := rv.Field(i)
		if f.Kind() != reflect.String {
			continue
		}
		if skipSet[rt.Field(i).Name] {
			continue
		}
		val := f.String()
		if strings.ContainsAny(val, "\r\n\x00") {
			return fmt.Errorf("nginx %s 字段 %s 不能包含换行或 NUL", label, rt.Field(i).Name)
		}
		if strings.ContainsAny(val, ";{}") {
			return fmt.Errorf("nginx %s 字段 %s 不能包含 ';' '{' '}'（会拆成额外指令）", label, rt.Field(i).Name)
		}
	}
	return nil
}

// nginxLogRoots 是 nginx 日志绝对路径的白名单根目录。
// 相对路径以 nginx prefix（面板数据目录）为基准，天然受限，放行。
var nginxLogRoots = []string{"/var/log", "/www/wwwlogs"}

// validateNginxLogPath 校验日志路径：禁止上跳；绝对路径只能落在白名单目录。
// 安全审计 v7 M-6：nginx master 以 root 打开日志文件，任意路径等于任意文件创建/追加。
func validateNginxLogPath(field, v string) error {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	switch v {
	case "/dev/null", "/dev/stdout", "/dev/stderr":
		return nil
	}
	if strings.Contains(v, "..") {
		return fmt.Errorf("nginx 日志路径 %s 不能包含 '..'", field)
	}
	if !filepath.IsAbs(v) {
		return nil // 相对路径写在 nginx prefix（数据目录）内
	}
	clean := filepath.Clean(v)
	for _, root := range nginxLogRoots {
		if clean == root || strings.HasPrefix(clean, root+string(filepath.Separator)) {
			return nil
		}
	}
	return fmt.Errorf("nginx 日志路径 %s=%q 不在白名单目录（%s，或使用相对路径）",
		field, v, strings.Join(nginxLogRoots, "、"))
}

// ValidateNginxSettings 校验全局 nginx 设置。
func ValidateNginxSettings(s *NginxSettings) error {
	if s == nil {
		return nil
	}
	if err := validateSingleLineFields(s, "设置", "CustomConfig"); err != nil {
		return err
	}
	logPaths := map[string]string{
		"error_log_path":  s.ErrorLogPath,
		"access_log_path": s.AccessLogPath,
		"stream_log_path": s.StreamLogPath,
	}
	for field, val := range logPaths {
		if err := validateNginxLogPath(field, val); err != nil {
			return err
		}
	}
	scalars := map[string]string{
		"http_listen":           s.HTTPListen,
		"http_listen_v6":        s.HTTPListenV6,
		"https_listen":          s.HTTPSListen,
		"https_listen_v6":       s.HTTPSListenV6,
		"quic_listen":           s.QUICListen,
		"quic_listen_v6":        s.QUICListenV6,
		"ipv6_listen_addr":      s.IPv6ListenAddr,
		"ssl_resolver":          s.SslResolver,
		"error_log_path":        s.ErrorLogPath,
		"access_log_path":       s.AccessLogPath,
		"error_log_level":       s.ErrorLogLevel,
		"stream_log_path":       s.StreamLogPath,
		"stream_log_format":     s.StreamLogFormat,
		"limit_conn_zone":       s.LimitConnZone,
		"limit_conn":            s.LimitConn,
		"sni_listen_addr":       s.SNIListenAddr,
		"sni_local_port":        s.SNILocalPort,
		"stream_ipv6_listen":    s.StreamIPv6ListenAddr,
		"stream_nontls_target":  s.StreamNonTLSTarget,
		"stream_default_target": s.StreamDefaultTarget,
	}
	for field, val := range scalars {
		if err := validateNginxScalar(field, val); err != nil {
			return err
		}
	}
	return validateNginxSnippet("custom_config", s.CustomConfig)
}

// ValidateNginxSite 校验站点参数（server/location 自定义段与裸写字段）。
func ValidateNginxSite(site *entity.NginxSite) error {
	if site == nil {
		return nil
	}
	if err := validateSingleLineFields(site, "站点", "ServerConfig", "LocationConfig"); err != nil {
		return err
	}
	scalars := map[string]string{
		"domain":              site.Domain,
		"upstream":            site.Upstream,
		"path":                site.Path,
		"backend_path":        site.BackendPath,
		"listen_port":         site.ListenPort,
		"quic_listen":         site.QuicListen,
		"listen_socket":       site.ListenSocket,
		"singbox_listen_port": site.SingboxListenPort,
		"cert_path":           site.CertPath,
		"key_path":            site.KeyPath,
		"l4_protocol":         site.L4Protocol,
		"sni_target":          site.SNITarget,
		"upstream_sni":        site.UpstreamSNI,
		"local_port":          site.LocalPort,
		"ipv6_listen_addr":    site.IPv6ListenAddr,
		"ssl_protocols":       site.SslProtocols,
		"ssl_ciphers":         site.SslCiphers,
		"ssl_ecdh_curve":      site.SslEcdhCurve,
	}
	for field, val := range scalars {
		if err := validateNginxScalar(field, val); err != nil {
			return err
		}
	}
	if err := validateNginxSnippet("server_config", site.ServerConfig); err != nil {
		return err
	}
	return validateNginxSnippet("location_config", site.LocationConfig)
}
