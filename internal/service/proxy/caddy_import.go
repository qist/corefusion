package proxy

import (
	"fmt"
	"strconv"
	"strings"

	"corefusion/internal/model/entity"
)

// CaddyImportResult 是从一份 Caddyfile 文本解析出来的站点导入结果。
type CaddyImportResult struct {
	Sites    []entity.CaddySite `json:"sites"`
	Warnings []string           `json:"warnings"`
}

// caddyBlock 是 Caddyfile 的块：site 地址块 / handle 等子块。
// lines 是该块里的普通指令行（不含子块），children 是带 `{` 的子块。
type caddyBlock struct {
	Label    string
	Lines    []string
	Children []*caddyBlock
}

// ParseCaddyfileSites 把 Caddyfile 文本解析成面板的站点列表（不落库）。
//
// 支持的范围（覆盖现网常见写法）：
//   - 站点块：example.com / a.com, b.com / *.example.com / http://example.com
//   - reverse_proxy（含 `to`、transport http { tls }）→ http 模板
//   - handle_path /api/*（去前缀）/ handle /ws/*（保路径）/ route → 按路径拆站点
//   - root * /dir + file_server → static
//   - redir → redirect；respond（状态码/回显 {remote_host}）→ return_code / echo_ip
//   - tls cert key → 证书路径；@name 命名匹配器的 path
//
// 不支持的（给出 warning）：:8443 这类无主机名的 L4 块、非 80/443 端口、
// import、handle_errors、任意文本 respond 等。
func ParseCaddyfileSites(content string) CaddyImportResult {
	res := CaddyImportResult{}
	root := parseCaddyBlocks(content)
	if len(root) == 0 {
		res.Warnings = append(res.Warnings, "未找到任何站点块")
		return res
	}

	namedMatchers := make(map[string]string) // @name -> path 前缀
	var siteBlocks []*caddyBlock
	for _, b := range root {
		if b.Label == "" {
			continue // 全局选项块
		}
		if strings.HasPrefix(b.Label, "@") {
			// 顶层命名匹配器少见，但顺手收下
			if p := caddyMatcherPath(b); p != "" {
				namedMatchers[strings.Fields(b.Label)[0]] = p
			}
			continue
		}
		first := caddyFirstWord(b.Label)
		switch first {
		case "import", "log", "default_sni", "order", "storage", "admin", "auto_https", "servers", "email", "acme_dns", "on_demand_tls", "http_port", "https_port", "grace_period", "key_type":
			// 全局选项写在块里的形式（Caddy v2.7+ 支持 label 为指令的全局块）
			continue
		}
		if strings.HasPrefix(b.Label, ":") {
			res.Warnings = append(res.Warnings, fmt.Sprintf("跳过：%s 是无主机名的监听/L4 块，需在面板 L4 转发里手工配置", b.Label))
			continue
		}
		if first == "" {
			res.Warnings = append(res.Warnings, fmt.Sprintf("跳过：无法识别的块 %q", b.Label))
			continue
		}
		siteBlocks = append(siteBlocks, b)
	}
	if len(siteBlocks) == 0 {
		if len(res.Warnings) == 0 {
			res.Warnings = append(res.Warnings, "未找到任何站点块")
		}
		return res
	}

	for _, b := range siteBlocks {
		sites, warnings := convertCaddySite(b, namedMatchers)
		res.Sites = append(res.Sites, sites...)
		res.Warnings = append(res.Warnings, warnings...)
	}
	return res
}

// convertCaddySite 把一个站点块转成 1..N 个站点（每个路径一条记录）。
func convertCaddySite(b *caddyBlock, namedMatchers map[string]string) ([]entity.CaddySite, []string) {
	var warnings []string
	domains, tlsByScheme, warns := parseCaddySiteLabels(b.Label)
	warnings = append(warnings, warns...)
	if len(domains) == 0 {
		return nil, warnings
	}
	domain := strings.Join(domains, ", ")
	base := entity.CaddySite{
		Domain:        domain,
		Enabled:       true,
		Priority:      100,
		WafEnabled:    boolPtr(true),
		RedirectHTTPS: boolPtr(tlsByScheme),
		DualCert:      boolPtr(true),
	}

	type entry struct {
		path      string
		strip     bool // handle_path：后端收到的是去前缀后的路径
		kind      string
		upstreams []string
		root      string
		target    string // redirect 目标 / return_code 的状态码
		tls       bool
		lb        string
	}
	var entries []entry
	siteRoot := ""
	hasFileServer := false
	siteCert, siteKey := "", ""

	addEntry := func(e entry) {
		// 同一路径出现多次时只保留第一条，避免生成重复 location
		for _, ex := range entries {
			if NormalizeLocationPath(ex.path) == NormalizeLocationPath(e.path) {
				warnings = append(warnings, fmt.Sprintf("[%s %s] 跳过：该路径已有匹配的指令块（只保留第一个）", domain, e.path))
				return
			}
		}
		entries = append(entries, e)
	}

	// 解析一个 reverse_proxy（来自指令行参数或子块），返回上游等选项。
	// targets 里的路径/命名匹配器（/api/*、@name）不是上游：剔除并把匹配器路径一并返回。
	parseProxy := func(targets []string, sub *caddyBlock) (upstreams []string, matcherPath string, tls bool, lb string) {
		for _, t := range targets {
			if strings.HasPrefix(t, "@") {
				if matcherPath == "" && namedMatchers[t] != "" {
					matcherPath = caddyPathPrefix(namedMatchers[t])
				}
				continue
			}
			if strings.HasPrefix(t, "/") || strings.HasPrefix(t, "*") {
				if matcherPath == "" {
					matcherPath = caddyPathPrefix(t)
				}
				continue
			}
			upstreams = append(upstreams, t)
		}
		if sub == nil {
			return
		}
		for _, ln := range sub.Lines {
			f := caddyFields(ln)
			if len(f) == 0 {
				continue
			}
			switch f[0] {
			case "to":
				upstreams = append(upstreams, f[1:]...)
			case "lb_policy":
				if len(f) > 1 {
					lb = f[1]
				}
			}
		}
		for _, ch := range sub.Children {
			// transport http { tls ... } → 回源 TLS（https 协议族）
			tf := caddyFields(ch.Label)
			if len(tf) == 0 || tf[0] != "transport" || (len(tf) > 1 && tf[1] != "http") {
				continue
			}
			for _, ln := range ch.Lines {
				if strings.TrimSpace(ln) == "tls" || strings.HasPrefix(strings.TrimSpace(ln), "tls ") {
					tls = true
				}
			}
		}
		return
	}

	// 处理 handle/route 子块里的站点逻辑
	walkHandleBlock := func(path string, strip bool, blk *caddyBlock) {
		e := entry{path: path, strip: strip, kind: ""}
		for _, ln := range blk.Lines {
			f := caddyFields(ln)
			if len(f) == 0 {
				continue
			}
			switch f[0] {
			case "root":
				// root * /dir 或 root /dir
				if len(f) >= 2 && f[1] == "*" && len(f) >= 3 {
					e.root = f[2]
				} else if len(f) >= 2 {
					e.root = f[1]
				}
			case "file_server":
				hasFileServer = true
				e.kind = "static"
			case "reverse_proxy":
				ups, mp, tls, lb := parseProxy(f[1:], nil)
				if len(ups) == 0 {
					warnings = append(warnings, fmt.Sprintf("[%s %s] 跳过：reverse_proxy 没有上游地址", domain, path))
					continue
				}
				e.kind = "http"
				e.upstreams = ups
				e.tls = tls
				e.lb = lb
				// 块内 reverse_proxy 还带了更细的路径匹配器：以更具体的为准
				if mp != "" && mp != "/" {
					e.path = mp
					e.strip = false
				}
			case "redir":
				args := f[1:]
				if len(args) > 0 {
					target := args[len(args)-1]
					if len(args) >= 2 && caddyIsRedirCode(target) {
						target = args[len(args)-2]
					}
					if t := caddyCleanRedirTarget(target); t != "" {
						e.kind = "redirect"
						e.target = t
					}
				}
			case "respond":
				if kind, code := caddyRespondKind(f[1:]); kind != "" {
					e.kind = kind
					e.target = code
				}
			}
		}
		for _, ch := range blk.Children {
			f := caddyFields(ch.Label)
			if len(f) == 0 {
				continue
			}
			if f[0] == "reverse_proxy" {
				ups, _, tls, lb := parseProxy(f[1:], ch)
				if len(ups) == 0 {
					warnings = append(warnings, fmt.Sprintf("[%s %s] 跳过：reverse_proxy 没有上游地址", domain, path))
					continue
				}
				e.kind = "http"
				e.upstreams = ups
				e.tls = tls
				e.lb = lb
			} else if caddyBenignDirective(f[0]) {
				// header / encode / log 等装饰性指令：生成器会补标准头，忽略即可
			} else {
				warnings = append(warnings, fmt.Sprintf("[%s %s] 跳过子指令 %q（面板不支持）", domain, path, ch.Label))
			}
		}
		switch {
		case e.kind == "":
			warnings = append(warnings, fmt.Sprintf("[%s %s] 跳过：块里没有可识别的指令", domain, path))
		default:
			addEntry(e)
		}
	}

	// ── 先收集 site 级 root/file_server ──
	for _, ln := range b.Lines {
		f := caddyFields(ln)
		if len(f) == 0 {
			continue
		}
		switch f[0] {
		case "root":
			if len(f) >= 2 && f[1] == "*" && len(f) >= 3 {
				siteRoot = f[2]
			} else if len(f) >= 2 {
				siteRoot = f[1]
			}
		case "file_server":
			hasFileServer = true
		case "tls":
			// tls <cert> <key>（tls internal 交给面板自动证书）
			if len(f) >= 3 && f[1] != "internal" {
				siteCert, siteKey = f[1], f[2]
			}
		}
	}
	base.CertPath, base.KeyPath = siteCert, siteKey
	if hasFileServer {
		root := siteRoot
		if root == "" {
			root = "/var/www"
		}
		addEntry(entry{path: "/", kind: "static", root: root})
	}

	// ── site 级指令与子块 ──
	for _, ln := range b.Lines {
		f := caddyFields(ln)
		if len(f) == 0 {
			continue
		}
		switch f[0] {
		case "redir":
			// redir [<matcher>] <to> [<code>]：末参是状态码/permanent 才视为 code
			args := f[1:]
			path, target := "/", ""
			if len(args) >= 2 && caddyIsRedirCode(args[len(args)-1]) {
				target = args[len(args)-2]
				if len(args) == 3 {
					path = caddyPathPrefix(args[0])
				}
			} else if len(args) == 1 {
				target = args[0]
			} else if len(args) >= 2 {
				// 无 code：[matcher] <to>
				target = args[len(args)-1]
				if len(args) == 3 {
					path = caddyPathPrefix(args[0])
				} else if len(args) == 2 && strings.HasPrefix(args[0], "/") && strings.HasPrefix(args[1], "/") {
					// redir /old /new：两个路径，前者是匹配器
					path = caddyPathPrefix(args[0])
					target = args[1]
				}
			}
			target = caddyCleanRedirTarget(target)
			if target != "" {
				addEntry(entry{path: path, kind: "redirect", target: target})
			}
		case "respond":
			if kind, code := caddyRespondKind(f[1:]); kind != "" {
				addEntry(entry{path: "/", kind: kind, target: code})
			}
		case "reverse_proxy":
			ups, mp, tls, lb := parseProxy(f[1:], nil)
			if len(ups) == 0 {
				warnings = append(warnings, fmt.Sprintf("[%s] 跳过：reverse_proxy 没有上游地址", domain))
				continue
			}
			p := mp
			if p == "" {
				p = "/"
			}
			addEntry(entry{path: p, kind: "http", upstreams: ups, tls: tls, lb: lb})
		case "import":
			warnings = append(warnings, fmt.Sprintf("[%s] 跳过：import %q 需手工处理", domain, strings.Join(f[1:], " ")))
		}
	}
	// site 级 reverse_proxy 子块（带 to/transport 的写法）
	for _, ch := range b.Children {
		f := caddyFields(ch.Label)
		if len(f) == 0 {
			continue
		}
		switch f[0] {
		case "reverse_proxy":
			ups, mp, tls, lb := parseProxy(f[1:], ch)
			if len(ups) == 0 {
				warnings = append(warnings, fmt.Sprintf("[%s] 跳过：reverse_proxy 没有上游地址", domain))
				continue
			}
			p := mp
			if p == "" {
				p = "/"
			}
			addEntry(entry{path: p, kind: "http", upstreams: ups, tls: tls, lb: lb})
		case "handle_path", "handle", "route":
			path := ""
			if len(f) > 1 {
				path = caddyPathPrefix(f[1])
			}
			if path == "" {
				path = "/"
			}
			strip := f[0] == "handle_path"
			walkHandleBlock(path, strip, ch)
		default:
			if strings.HasPrefix(f[0], "@") || caddyBenignDirective(f[0]) {
				continue // 命名匹配器 / header·encode 等装饰性指令：导入时不需要单独处理
			}
			warnings = append(warnings, fmt.Sprintf("[%s] 跳过子块 %q（面板不支持）", domain, ch.Label))
		}
	}

	// ── 组装站点 ──
	var sites []entity.CaddySite
	for _, e := range entries {
		site := base
		site.Path = NormalizeLocationPath(e.path)
		if site.Path == "" {
			site.Path = "/"
		}
		site.Template = e.kind
		switch e.kind {
		case "static":
			site.Upstream = e.root
		case "redirect":
			site.BackendPath = e.target
		case "return_code", "echo_ip":
			site.BackendPath = e.target
		default:
			// http（reverse_proxy）
			site.Upstream = strings.Join(e.upstreams, ", ")
			site.UpstreamTLS = e.tls
			site.LbPolicy = e.lb
			if e.strip {
				// handle_path：后端收到去前缀路径 → 生成器用 handle_path 自动去前缀 ✓
				site.BackendPath = "/"
			} else if site.Path != "/" {
				// handle/route：保留原始路径 → 生成器用 reverse_proxy @matcher 保路径 ✓
				site.BackendPath = site.Path
			}
		}
		sites = append(sites, site)
	}
	if len(sites) == 0 {
		warnings = append(warnings, fmt.Sprintf("[%s] 跳过：没有可识别的指令", domain))
	}
	return sites, warnings
}

// parseCaddySiteLabels 解析站点地址（example.com, www.x.com:8443 → 域名列表）。
// 返回域名列表、是否带 https（http:// 显式声明的为 false）、告警。
func parseCaddySiteLabels(label string) (domains []string, tls bool, warnings []string) {
	// 只有显式声明 https:// 或无 scheme（Caddy 默认 auto-HTTPS）才算 TLS；
	// 纯 http:// 地址不强制跳 https。
	tls = false
	for _, addr := range strings.Split(label, ",") {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}
		isTLS := true
		if strings.HasPrefix(addr, "https://") {
			addr = strings.TrimPrefix(addr, "https://")
		} else if strings.HasPrefix(addr, "http://") {
			addr = strings.TrimPrefix(addr, "http://")
			isTLS = false
		}
		host := addr
		if i := strings.LastIndex(addr, ":"); i >= 0 && !strings.Contains(addr, "]") {
			p := addr[i+1:]
			if _, err := strconv.Atoi(p); err == nil {
				host = addr[:i]
				if p != "80" && p != "443" {
					warnings = append(warnings, fmt.Sprintf("站点地址 %q 带非 80/443 端口 %s，导入后按面板默认端口生成（如需保留请手工调整）", addr, p))
				}
			}
		}
		host = strings.Trim(host, ".")
		if host == "" {
			warnings = append(warnings, fmt.Sprintf("跳过：站点地址 %q 没有主机名", addr))
			continue
		}
		if isTLS {
			tls = true
		}
		domains = append(domains, host)
	}
	return domains, tls, warnings
}

// caddyMatcherPath 从 @name { path /x/* } 里取 path 模式。
func caddyMatcherPath(b *caddyBlock) string {
	for _, ln := range b.Lines {
		f := caddyFields(ln)
		if len(f) >= 2 && f[0] == "path" {
			return f[1]
		}
	}
	return ""
}

// caddyPathPrefix 把 /api/* 归一化成 /api（去通配符与尾斜杠）；"/" 保持为 "/"。
func caddyPathPrefix(p string) string {
	p = strings.TrimSuffix(strings.TrimSpace(p), "*")
	p = strings.TrimSuffix(p, "/")
	if p == "" {
		return "/"
	}
	return p
}

// caddyIsRedirCode 判断 redir 末参是否为跳转状态码（300-399 / permanent / temporary）。
func caddyIsRedirCode(s string) bool {
	if s == "permanent" || s == "temporary" {
		return true
	}
	n, err := strconv.Atoi(s)
	return err == nil && n >= 300 && n < 400
}

// caddyCleanRedirTarget 清理跳转目标（去掉 {uri}，生成器会自动补）。
// 含 Caddy 占位符变量的目标返回空（面板不支持，需手工配置）。
func caddyCleanRedirTarget(target string) string {
	target = strings.TrimSpace(target)
	target = strings.TrimSuffix(target, "{uri}")
	target = strings.TrimSpace(target)
	if target == "" || strings.Contains(target, "{") {
		return ""
	}
	return target
}

// caddyRespondKind 把 respond 指令映射成 return_code / echo_ip。
// 返回 ("", "") 表示无法导入。
func caddyRespondKind(args []string) (kind, code string) {
	if len(args) == 0 {
		return "", ""
	}
	// respond [<matcher>] <status>|<body> [<status>]
	first := strings.Trim(args[0], `"'`)
	last := ""
	if len(args) >= 2 {
		last = strings.Trim(args[len(args)-1], `"'`)
	}
	body, status := "", ""
	if _, err := strconv.Atoi(first); err == nil && (len(args) == 1 || last == "") {
		// respond 204 → 纯状态码
		return "return_code", first
	}
	if _, err := strconv.Atoi(last); err == nil {
		body, status = first, last
	} else {
		body = first
	}
	if strings.Contains(body, "{remote_host}") || strings.Contains(body, "{http.request.remote.host}") {
		return "echo_ip", ""
	}
	if status != "" && body == "" {
		return "return_code", status
	}
	// 任意文本响应面板不支持 → 跳过
	return "", ""
}

// ────────────────────────── 语法解析 ──────────────────────────

// caddyStripComment 去掉行内注释（引号外的 # 之后）。
func caddyStripComment(line string) string {
	var quote rune
	for i, r := range line {
		if quote != 0 {
			if r == quote {
				quote = 0
			}
			continue
		}
		switch r {
		case '"', '\'', '`':
			quote = r
		case '#':
			return strings.TrimSpace(line[:i])
		}
	}
	return strings.TrimSpace(line)
}

// parseCaddyBlocks 按行把 Caddyfile 解析成块树（顶层 = 站点块与全局选项块）。
func parseCaddyBlocks(content string) []*caddyBlock {
	var root []*caddyBlock
	var stack []*caddyBlock
	// pending 是"已见到 label 行、还没遇到 {"的顶层块
	scanner := func(line string) {
		line = caddyStripComment(line)
		if line == "" {
			return
		}
		if line == "}" {
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			return
		}
		if strings.HasSuffix(line, "{") {
			blk := &caddyBlock{Label: strings.TrimSpace(strings.TrimSuffix(line, "{"))}
			if len(stack) == 0 {
				root = append(root, blk)
			} else {
				parent := stack[len(stack)-1]
				parent.Children = append(parent.Children, blk)
			}
			stack = append(stack, blk)
			return
		}
		if len(stack) == 0 {
			// 顶层裸指令（无块的）不成站点，忽略
			return
		}
		cur := stack[len(stack)-1]
		cur.Lines = append(cur.Lines, line)
	}
	// 支持一行内多个 token（含引号包起来的 { }，例如 respond "{"）：
	// 简单按行处理即可覆盖现网写法；引号里的 { } 不在行尾，不受影响。
	for _, raw := range strings.Split(content, "\n") {
		scanner(raw)
	}
	return root
}

// caddyFields 按空白切分一行（保留引号内整体，去掉引号本身）。
func caddyFields(line string) []string {
	var out []string
	var cur strings.Builder
	var quote rune
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	for _, r := range line {
		if quote != 0 {
			if r == quote {
				quote = 0
				continue
			}
			cur.WriteRune(r)
			continue
		}
		switch r {
		case '"', '\'', '`':
			quote = r
		case ' ', '\t':
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return out
}

func caddyFirstWord(s string) string {
	f := caddyFields(s)
	if len(f) == 0 {
		return ""
	}
	return f[0]
}

// caddyBenignDirective 是导入时可以安全忽略的指令（生成器会补标准头/无行为差异）。
func caddyBenignDirective(name string) bool {
	switch name {
	case "header", "encode", "log", "request_body", "basicauth", "basic_auth",
		"tls", "push", "uri", "header_regexp", "var", "handle_errors", "try_files":
		return true
	}
	return false
}
