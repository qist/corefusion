package proxy

import (
	"fmt"
	"strconv"
	"strings"

	"corefusion/internal/model/entity"
)

// NginxImportResult 是从一份 nginx 配置文本解析出来的站点导入结果。
type NginxImportResult struct {
	Sites    []entity.NginxSite `json:"sites"`
	Warnings []string           `json:"warnings"`
}

// nginxDirective 是 nginx 配置的语法树节点：`name arg1 arg2 { ... }` 或 `name arg1;`
type nginxDirective struct {
	Name     string
	Args     []string
	Children []*nginxDirective
}

// ParseNginxSites 把 nginx 配置文本解析成面板的站点列表（不落库）。
//
// 支持的范围（覆盖现网常见写法）：
//   - http { server { ... } } 或直接 server { ... }
//   - listen 80 / 443 ssl http2 / [::]:443 / 127.0.0.1:8080 / 443 quic / 80 proxy_protocol
//   - server_name 多值（含通配符）
//   - location 里的 proxy_pass / grpc_pass / return / root / echo 探测页
//   - server 级 return 301/302 → redirect 模板
//   - ssl_certificate / ssl_certificate_key 路径
//
// 不支持的（给出 warning，由人工处理）：lua/rewrite/if/complex map 等。
func ParseNginxSites(content string) NginxImportResult {
	res := NginxImportResult{}
	tokens := tokenizeNginx(content)
	pos := 0
	tree := parseNginxDirectives(tokens, &pos)

	var servers []*nginxDirective
	upstreams := make(map[string][]string)
	collectUpstream := func(dirs []*nginxDirective) {
		for _, c := range dirs {
			if c.Name != "upstream" || len(c.Args) == 0 {
				continue
			}
			var addrs []string
			for _, srv := range c.Children {
				if srv.Name == "server" && len(srv.Args) > 0 {
					addrs = append(addrs, srv.Args[0]) // 忽略 weight/backup 等参数
				}
			}
			if len(addrs) > 0 {
				upstreams[c.Args[0]] = addrs
			}
		}
	}
	for _, d := range tree {
		switch d.Name {
		case "http":
			collectUpstream(d.Children)
			for _, c := range d.Children {
				if c.Name == "server" {
					servers = append(servers, c)
				}
			}
		case "upstream":
			collectUpstream([]*nginxDirective{d})
		case "server":
			servers = append(servers, d)
		}
	}
	if len(servers) == 0 {
		res.Warnings = append(res.Warnings, "未找到 server 块")
		return res
	}

	// 同一组域名可能有 80 与 443 两个 server 块：优先取带 ssl 的那个，
	// 避免导入后生成重复站点（生成器本来就会自动做 HTTP→HTTPS 跳转）。
	type group struct {
		domains []string
		server  *nginxDirective
		hasTLS  bool
	}
	order := make([]string, 0)
	groups := make(map[string]*group)
	for _, srv := range servers {
		domains := allDirectiveArgs(srv, "server_name")
		if len(domains) == 0 {
			res.Warnings = append(res.Warnings, "跳过：server 块没有 server_name")
			continue
		}
		normalized := SplitDomains(strings.Join(domains, " "))
		if len(normalized) == 0 {
			continue
		}
		key := DomainGroupKey(normalized)
		tls := listenHasTLS(srv)
		g, ok := groups[key]
		if !ok {
			groups[key] = &group{domains: normalized, server: srv, hasTLS: tls}
			order = append(order, key)
			continue
		}
		// 已有组：优先保留带 TLS 的 server（证书/端口以它为准）
		if tls && !g.hasTLS {
			g.server = srv
			g.hasTLS = tls
		}
	}

	for _, key := range order {
		g := groups[key]
		sites, warnings := convertNginxServer(g.server, g.domains, upstreams)
		res.Sites = append(res.Sites, sites...)
		res.Warnings = append(res.Warnings, warnings...)
	}
	return res
}

// convertNginxServer 把一个 server 块转成 1..N 个站点（每个 location 一个站点）。
func convertNginxServer(srv *nginxDirective, domains []string, upstreams map[string][]string) ([]entity.NginxSite, []string) {
	var warnings []string
	port, socket, flags := parseListen(srv)
	domain := strings.Join(domains, ", ")
	cert, key := "", ""
	if v := directiveArgs(srv, "ssl_certificate"); len(v) > 0 {
		cert = v[0]
	}
	if v := directiveArgs(srv, "ssl_certificate_key"); len(v) > 0 {
		key = v[0]
	}

	if port == "" {
		port = "443"
		if !hasTLSDirective(srv) {
			port = "80"
		}
	}
	base := entity.NginxSite{
		Domain:         domain,
		ListenPort:     port,
		ListenSocket:   socket,
		CertPath:       cert,
		KeyPath:        key,
		EnableHttp2:    flags["http2"] || (flags["ssl"] && port == "443"),
		EnableQuic:     flags["quic"],
		ProxyProtocol:  flags["proxy_protocol"],
		EnableIpv6:     strings.Contains(listenLine(srv), "[::]"),
		Enabled:        true,
		Priority:       100,
		DualCert:       boolPtr(true),
		WafEnabled:     boolPtr(true),
		RedirectHTTPS:  boolPtr(true),
		ServerConfig:   "",
		LocationConfig: "",
	}

	// server 级 return：整站跳转/状态码
	if ret := directiveArgs(srv, "return"); len(ret) > 0 {
		site := base
		site.Path = "/"
		code := ret[0]
		target := ""
		if len(ret) > 1 {
			target = ret[1]
		}
		if isRedirectStatus(code) {
			site.Template = "redirect"
			site.BackendPath = cleanReturnTarget(target)
		} else if isHTTPStatusCode(code) {
			site.Template = "return_code"
			site.BackendPath = code
		}
		return []entity.NginxSite{site}, warnings
	}

	var sites []entity.NginxSite
	for _, loc := range srv.Children {
		if loc.Name != "location" {
			continue
		}
		site, ok, warn := convertNginxLocation(base, loc, upstreams)
		if warn != "" {
			warnings = append(warnings, warn)
		}
		if ok {
			sites = append(sites, site)
		}
	}
	if len(sites) == 0 {
		warnings = append(warnings, fmt.Sprintf("[%s] 跳过：没有可识别的 location", domain))
	}
	return sites, warnings
}

// convertNginxLocation 把一个 location 块转成站点。
func convertNginxLocation(base entity.NginxSite, loc *nginxDirective, upstreams map[string][]string) (entity.NginxSite, bool, string) {
	site := base
	site.Path = locationPath(loc.Args)
	if site.Path == "" {
		return site, false, ""
	}

	// return 优先（探测页 204 / 跳转）
	if ret := directiveArgs(loc, "return"); len(ret) > 0 {
		code := ret[0]
		target := ""
		if len(ret) > 1 {
			target = ret[1]
		}
		if isRedirectStatus(code) {
			if hasRegexCapture(target) {
				return site, false, fmt.Sprintf("[%s %s] 跳过：return 目标含正则捕获（$1…），需手工配置", base.Domain, site.Path)
			}
			site.Template = "redirect"
			site.BackendPath = cleanReturnTarget(target)
			return site, true, ""
		}
		if isHTTPStatusCode(code) {
			// echo 探测页：default_type text/plain + return 200 "$clientRealIp"
			if strings.Contains(target, "clientRealIp") || strings.Contains(target, "remote_addr") {
				site.Template = "echo_ip"
			} else {
				site.Template = "return_code"
				site.BackendPath = code
			}
			return site, true, ""
		}
	}

	if v := directiveArgs(loc, "proxy_pass"); len(v) > 0 {
		// 含变量的上游（proxy_pass https://$website）依赖同文件里的 map/set，
		// 面板无法一并迁移 → 跳过并提示，避免导入后配置不可用。
		if strings.Contains(v[0], "$") {
			return site, false, fmt.Sprintf("[%s %s] 跳过：proxy_pass 含变量（需手工配置 map/set）", base.Domain, site.Path)
		}
		resolved, ok := resolveUpstreamRef(v[0], upstreams)
		if !ok {
			return site, false, fmt.Sprintf("[%s %s] 跳过：proxy_pass 引用了未定义的 upstream 组 %q", base.Domain, site.Path, v[0])
		}
		site.Template = "proxy"
		site.Upstream = resolved
		return site, true, ""
	}
	if v := directiveArgs(loc, "grpc_pass"); len(v) > 0 {
		if strings.Contains(v[0], "$") {
			return site, false, fmt.Sprintf("[%s %s] 跳过：grpc_pass 含变量", base.Domain, site.Path)
		}
		resolved, ok := resolveUpstreamRef(v[0], upstreams)
		if !ok {
			return site, false, fmt.Sprintf("[%s %s] 跳过：grpc_pass 引用了未定义的 upstream 组 %q", base.Domain, site.Path, v[0])
		}
		site.Template = "grpc"
		site.Upstream = resolved
		return site, true, ""
	}
	if v := directiveArgs(loc, "root"); len(v) > 0 {
		site.Template = "static"
		site.Upstream = v[0]
		return site, true, ""
	}
	// 只有 return/echo 之外的复杂 location（lua/rewrite/if…）
	if len(loc.Children) > 0 && hasDirective(loc, "default_type") {
		site.Template = "static"
		site.Upstream = "/var/www"
		return site, true, ""
	}
	return site, false, fmt.Sprintf("[%s %s] 跳过：不支持的 location（可能含 lua/rewrite/if）", base.Domain, site.Path)
}

// ────────────────────────── 解析工具 ──────────────────────────

// tokenizeNginx 去掉注释与引号后按 nginx 语法切词：
// { } ; 单独成 token，空白分隔参数，引号内的内容作为一个 token。
func tokenizeNginx(content string) []string {
	var tokens []string
	var cur strings.Builder
	var quote rune
	inComment := false
	flush := func() {
		if cur.Len() > 0 {
			tokens = append(tokens, cur.String())
			cur.Reset()
		}
	}
	for _, r := range content {
		if inComment {
			if r == '\n' {
				inComment = false
			}
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
				flush()
				continue
			}
			cur.WriteRune(r)
			continue
		}
		switch r {
		case '#':
			flush()
			inComment = true
		case '"', '\'':
			flush()
			quote = r
		case '{', '}', ';':
			flush()
			tokens = append(tokens, string(r))
		case ' ', '\t', '\r', '\n':
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return tokens
}

// parseNginxDirectives 把 token 流解析成指令树。
func parseNginxDirectives(tokens []string, pos *int) []*nginxDirective {
	var out []*nginxDirective
	for *pos < len(tokens) {
		t := tokens[*pos]
		if t == "}" {
			*pos++
			return out
		}
		if t == "{" || t == ";" {
			*pos++
			continue
		}
		d := &nginxDirective{Name: t}
		*pos++
		for *pos < len(tokens) {
			a := tokens[*pos]
			if a == ";" {
				*pos++
				break
			}
			if a == "{" {
				*pos++
				d.Children = parseNginxDirectives(tokens, pos)
				break
			}
			if a == "}" {
				break
			}
			d.Args = append(d.Args, strings.Trim(a, `"'`))
			*pos++
		}
		out = append(out, d)
	}
	return out
}

func directiveArgs(d *nginxDirective, name string) []string {
	for _, c := range d.Children {
		if c.Name == name {
			return c.Args
		}
	}
	return nil
}

// allDirectiveArgs 合并同名指令的所有参数（server_name 允许写多行）。
func allDirectiveArgs(d *nginxDirective, name string) []string {
	var out []string
	for _, c := range d.Children {
		if c.Name == name {
			out = append(out, c.Args...)
		}
	}
	return out
}

func hasDirective(d *nginxDirective, name string) bool {
	for _, c := range d.Children {
		if c.Name == name {
			return true
		}
	}
	return false
}

// parseListen 解析 listen 指令，返回端口、unix socket 与开关标记。
func parseListen(srv *nginxDirective) (port, socket string, flags map[string]bool) {
	flags = map[string]bool{}
	var listens [][]string
	for _, c := range srv.Children {
		if c.Name != "listen" {
			continue
		}
		listens = append(listens, c.Args)
	}
	if len(listens) == 0 {
		return "", "", flags
	}
	// 优先 443/带 ssl 的 listen
	chosen := listens[0]
	for _, l := range listens {
		for _, a := range l {
			if a == "ssl" {
				chosen = l
			}
		}
	}
	for _, a := range chosen {
		switch a {
		case "ssl", "http2", "quic", "proxy_protocol", "default_server", "reuseport", "so_keepalive=on", "ipv6only=off":
			flags[a] = true
			continue
		}
		if strings.HasPrefix(a, "unix:") {
			socket = strings.TrimPrefix(a, "unix:")
			continue
		}
		if p := listenPortOf(a); p != "" {
			port = p
		}
	}
	return port, socket, flags
}

func listenPortOf(addr string) string {
	if strings.HasPrefix(addr, "[") {
		// [::]:443
		if i := strings.LastIndex(addr, "]:"); i >= 0 {
			return addr[i+2:]
		}
		return ""
	}
	if i := strings.LastIndex(addr, ":"); i >= 0 {
		return addr[i+1:]
	}
	if _, err := strconv.Atoi(addr); err == nil {
		return addr
	}
	return ""
}

func listenHasTLS(srv *nginxDirective) bool {
	return hasTLSDirective(srv)
}

func hasTLSDirective(srv *nginxDirective) bool {
	for _, c := range srv.Children {
		if c.Name != "listen" {
			continue
		}
		for _, a := range c.Args {
			if a == "ssl" {
				return true
			}
		}
	}
	return hasDirective(srv, "ssl_certificate")
}

func listenLine(srv *nginxDirective) string {
	for _, c := range srv.Children {
		if c.Name == "listen" {
			return strings.Join(c.Args, " ")
		}
	}
	return ""
}

// locationPath 取 location 的匹配路径（去掉 = ~ ~* ^~ 修饰符）。
func locationPath(args []string) string {
	if len(args) == 0 {
		return ""
	}
	switch args[0] {
	case "=", "~", "~*", "^~":
		if len(args) > 1 {
			return args[1]
		}
		return ""
	}
	return args[0]
}

func isRedirectStatus(code string) bool {
	switch code {
	case "301", "302", "303", "307", "308":
		return true
	}
	return false
}

// cleanReturnTarget 把 nginx 的 return 目标转成面板的跳转目标：
// 去掉 $request_uri（生成器会自动追加），保留 https://$host 这类变量。
func cleanReturnTarget(t string) string {
	t = strings.TrimSpace(t)
	t = strings.ReplaceAll(t, "$request_uri", "")
	t = strings.ReplaceAll(t, "$uri", "")
	return strings.TrimSpace(t)
}

func boolPtr(b bool) *bool { return &b }

// resolveUpstreamRef 把 `http://php` 这种对 upstream 组的引用替换成组内第一台服务器地址。
// 现网 conf 里 upstream 组写在 http 块（或 upstream.conf 并被 include），面板不迁移 upstream 组，
// 因此在导入时内联，避免生成 `proxy_pass https://dns;` 这种依赖运行时 DNS 解析的配置。
func resolveUpstreamRef(target string, upstreams map[string][]string) (string, bool) {
	i := strings.Index(target, "://")
	if i < 0 {
		return target, true
	}
	scheme, rest := target[:i+3], target[i+3:]
	host, suffix := rest, ""
	if j := strings.IndexAny(rest, "/?#"); j >= 0 {
		host, suffix = rest[:j], rest[j:]
	}
	if addrs, ok := upstreams[host]; ok && len(addrs) > 0 {
		return scheme + addrs[0] + suffix, true
	}
	// 不含点、不含端口的裸名字（php/dns/vnStat…）在 nginx 里几乎都是 upstream 组名：
	// 组定义没随这份配置带过来 → 直接跳过，避免生成 `proxy_pass http://vnStat;`
	// 让 nginx 启动时做 DNS 解析而失败。
	if !strings.Contains(host, ".") && !strings.Contains(host, ":") && host != "localhost" && host != "" {
		return target, false
	}
	return target, true
}

// hasRegexCapture 判断跳转目标是否使用了 nginx 正则捕获变量（$1…$9）。
// 这类 location 依赖原配置里的 location ~ 正则，面板站点没有对应字段，必须手工迁移。
func hasRegexCapture(target string) bool {
	for i := 1; i <= 9; i++ {
		if strings.Contains(target, fmt.Sprintf("$%d", i)) {
			return true
		}
	}
	return false
}
