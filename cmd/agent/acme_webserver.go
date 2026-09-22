package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ACME HTTP-01 的提供方式（**全自动，零人工配置** ✓）——这是产品的既定设计 ✓：
//
//	① **80 端口空闲** → Agent 直接**临时起一个 80 服务**提供挑战 ✓
//	   （`internal/runtime/cert.HTTP01Provider` ✓，用完即停 ✓）。
//	② **80 端口已被占用**（nginx/caddy 等）→ **自动去改它的配置** ✓：
//	   写一份只处理 `/.well-known/acme-challenge/` 的片段 ✓（alias 到 Agent 自管目录 ✓），
//	   校验通过才 reload ✓，失败立刻回滚 ✓，签发结束后**自动清理** ✓。
//	   —— 也就是说：**不需要操作者去填 webroot** ✗，节点侧也没有 acme 配置 ✗。
//
// 因此 `IssuePublicCertCommand.webroot` 只作为**显式覆盖**存在（运维想手工接管时才用 ✓）。

// acmeChallengeDir 返回"web 服务器**一定看得见**"的挑战目录 ✓。
//
// 真机踩到的坑 ✗：挑战文件**不能**放在 /tmp 下面 —— 很多发行版的 nginx 服务单元带
// **PrivateTmp=yes** ✗，nginx 看到的是它**自己的私有 /tmp** ✗，于是目录权限全对、
// server_name 也匹配上了，却恒为 **404** ✗（真机实测：同一份文件 /tmp → 404，/var/lib → 200 ✓）。
// 而 ACME 的 dataDir（账号/产物）经常正好设在 /tmp ✗ → 所以挑战目录**必须独立于 dataDir** ✓，
// 固定放在 web 服务器可见的位置 ✓。
//
// 可用 `COREFUSION_ACME_CHALLENGE_DIR` 覆盖（测试/特殊部署用 ✓）。
func acmeChallengeDir(dataDir string) string {
	if d := strings.TrimSpace(os.Getenv("COREFUSION_ACME_CHALLENGE_DIR")); d != "" {
		return d
	}
	// 默认放 /var/lib 下：web 服务器可读 ✓，且不受 PrivateTmp 影响 ✓
	return "/var/lib/corefusion-agent/acme-challenge"
}

// portIsFree 判断本机某端口是否空闲（能绑定就是空闲 ✓）。
func portIsFree(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

// detectWebServer 探测占用 80 的 web 服务器类型 ✓（只认我们**会改**的两种 ✓）。
//
// ⚠ 不能用 `pgrep -x`（要求进程名**完全相等** ✗）：实际部署里 caddy 的进程名常是
// `caddy-linux-amd64` ✗、nginx 也可能带后缀 ✗ → 精确匹配会**探测不到** ✗（真机踩到 ✓）。
// 这里用锚定前缀的正则 ✓（既避免误伤 nginx-exporter 之类 ✗，又能认出带后缀的 ✓）。
func detectWebServer() string {
	for _, name := range []string{"nginx", "caddy"} {
		out, err := exec.Command("pgrep", "^"+name).Output()
		if err == nil && strings.TrimSpace(string(out)) != "" {
			return name
		}
	}
	return ""
}

// prepareHTTP01 decides how to serve the HTTP-01 challenge.
//
// 返回 root="" 表示"用内置临时 80 服务" ✓；root 非空表示"把挑战写进该目录" ✓
// （此时 cleanup 负责把自动改过的 web 服务器配置还原 ✓）。
func prepareHTTP01(identifiers []string, dataDir, overrideWebroot string) (root string, cleanup func(), err error) {
	if w := strings.TrimSpace(overrideWebroot); w != "" {
		// 显式覆盖：直接用给定的 webroot ✓（不做任何自动改动 ✓）
		return w, func() {}, nil
	}

	// ⚠ 第一步永远是**行为探测**（而不是去猜"配置归谁管" ✗）：
	// 现在的 web 服务器**已经**能把 /.well-known/acme-challenge/ 提供了吗？
	//   - 面板守护的节点：面板生成的配置里就带了处理 ✓（见 proxy/nginx_config.go|caddy_manager.go ✓）
	//   - 独立部署的节点：运维可能自己配过 ✓
	//   - 都没有：探测失败 ✓ → 才轮到我们去动配置 ✓
	// 这样"面板守护 / 独立部署"两种情况用同一套逻辑就能覆盖 ✓，也不需要给配置加标记 ✗。
	dir := acmeChallengeDir(dataDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", func() {}, err
	}
	_ = os.Chmod(dir, 0755)
	_ = os.Chmod(filepath.Dir(dir), 0755)
	if err := selfCheckChallenge(identifiers, dir); err == nil {
		log.Printf("[acme] 挑战路径已由现有 web 服务器提供 ✓（无需改动任何配置）")
		return dir, func() {}, nil
	} else {
		log.Printf("[acme] 挑战路径当前不可用 → 自动处理：%v", err)
	}

	// ① 80 空闲 → 临时起服务 ✓
	if portIsFree(80) {
		return "", func() {}, nil
	}

	// ② 80 被占用 → 自动配置现有 web 服务器 ✓
	server := detectWebServer()
	switch server {
	case "caddy":
		undo, err := configureCaddyForACME(dir, identifiers)
		if err != nil {
			return "", func() {}, err
		}
		return dir, undo, nil

	case "nginx":
		undo, err := configureNginxForACME(dir, identifiers)
		if err != nil {
			return "", func() {}, err
		}
		return dir, undo, nil
	default:
		// 注意：错误分支也必须返回**非 nil** 的 cleanup ✓（调用方会无条件 defer 它 ✗）
		noop := func() {}
		if server == "" {
			return "", noop, fmt.Errorf("port 80 is in use by an unrecognized process; cannot auto-configure HTTP-01 " +
				"(停止占用 80 的服务，或显式指定 webroot)")
		}
		return "", noop, fmt.Errorf("port 80 is in use by %s; automatic configuration for it is not supported yet "+
			"(目前支持 nginx 与 caddy ✓；其它服务请停止它，或显式指定 webroot)", server)
	}
}

// nginxSnippetPath 是临时片段路径（固定名 → 幂等 ✓，重复签发不会堆文件 ✓）。
func nginxSnippetPath() string {
	if p := os.Getenv("COREFUSION_NGINX_CONF_DIR"); strings.TrimSpace(p) != "" {
		return filepath.Join(p, "zz-corefusion-acme.conf")
	}
	return "/etc/nginx/conf.d/zz-corefusion-acme.conf"
}

// nginxSnippet 生成"只服务 ACME 挑战"的 server 块 ✓。
//
// 为什么用独立 server 块 ✗：不需要去解析/改写别人的站点配置 ✓（那才是最危险的做法 ✓）；
// nginx 按 `Host` 匹配 server_name ✓，而 ACME 校验时 Host 正是申请的域名/IP ✓，
// 所以这个块能精确接管挑战请求 ✓，其它路径返回 404 且**只在这几秒内**存在 ✓。
func nginxSnippet(challengeDir string, identifiers []string) string {
	var sb strings.Builder
	sb.WriteString("# CoreFusion ACME 挑战 —— 由 Agent 自动生成，签发结束后会自动删除 ✓\n")
	sb.WriteString("server {\n")
	sb.WriteString("    listen 80;\n")
	sb.WriteString(fmt.Sprintf("    server_name %s;\n", strings.Join(identifiers, " ")))
	sb.WriteString("\n")
	sb.WriteString("    location ^~ /.well-known/acme-challenge/ {\n")
	// alias 必须对齐 WebrootProvider 的写入位置 ✓：<root>/.well-known/acme-challenge/<token> ✓
	sb.WriteString(fmt.Sprintf("        alias %s/.well-known/acme-challenge/;\n", strings.TrimSuffix(challengeDir, "/")))
	sb.WriteString("    }\n")
	sb.WriteString("\n")
	sb.WriteString("    location / { return 404; }\n")
	sb.WriteString("}\n")
	return sb.String()
}

// configureNginxForACME 写入片段 → nginx -t → reload ✓；任一步失败都会**回滚** ✓。
func configureNginxForACME(challengeDir string, identifiers []string) (func(), error) {
	path := nginxSnippetPath()
	noop := func() {}

	prev, hadPrev := readFileIfExists(path)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return noop, err
	}
	if err := os.WriteFile(path, []byte(nginxSnippet(challengeDir, identifiers)), 0644); err != nil {
		return noop, err
	}
	restore := func() {
		if hadPrev {
			_ = os.WriteFile(path, prev, 0644)
		} else {
			_ = os.Remove(path)
		}
	}

	// `nginx -t` 通过才允许 reload ✓（语法错就回滚 ✗，绝不把线上配置搞坏 ✗）
	if out, err := exec.Command("nginx", "-t").CombinedOutput(); err != nil {
		restore()
		return noop, fmt.Errorf("nginx -t failed after adding ACME snippet: %v: %s", err, strings.TrimSpace(string(out)))
	}
	if out, err := exec.Command("nginx", "-s", "reload").CombinedOutput(); err != nil {
		restore()
		return noop, fmt.Errorf("nginx reload failed: %v: %s", err, strings.TrimSpace(string(out)))
	}

	// 给 nginx 一点时间落地 ✓ —— reload 是异步的 ✓，太短会在自检里误判成"挑战不可用" ✗
	// （真机实测：实测 300ms 偶尔不够 ✓，2s 稳定 ✓；这里取 1s 折中 ✓）
	time.Sleep(1 * time.Second)

	// 自检：片段真的被 include 进来了吗？（include 路径不对就白写了 ✗ → 回滚并说明 ✓）
	if out, err := exec.Command("nginx", "-T").CombinedOutput(); err != nil || !strings.Contains(string(out), strings.TrimSuffix(challengeDir, "/")) {
		restore()
		_, _ = exec.Command("nginx", "-t").CombinedOutput()
		_ = exec.Command("nginx", "-s", "reload").Run()
		return noop, fmt.Errorf("nginx does not include the ACME snippet (%s) — 请把该目录加入 nginx.conf 的 include（例如 http{} 里 include /etc/nginx/conf.d/*.conf;）", path)
	}

	undo := func() {
		restore()
		_, _ = exec.Command("nginx", "-t").CombinedOutput()
		_ = exec.Command("nginx", "-s", "reload").Run()
	}

	// 开签前自检 ✓：真的能从 :80 取到挑战文件吗？
	// 少了这步，权限/匹配问题会一路带到 CA，变成一个含糊的 404 ✗（真机踩过 ✓）。
	if err := selfCheckChallenge(identifiers, challengeDir); err != nil {
		undo()
		return func() {}, err
	}
	return undo, nil
}

// selfCheckChallenge 在本机放一个探针文件，再从 :80 取回来 ✓ ——
// 把"权限不对 / server_name 没匹配上 / include 没生效"这类问题暴露在**本地** ✓，
// 而不是等 CA 回一个含糊的 404 ✗。
func selfCheckChallenge(identifiers []string, challengeDir string) error {
	if len(identifiers) == 0 {
		return fmt.Errorf("no identifiers for challenge self-check")
	}
	token := fmt.Sprintf("acme-selfcheck-%d", time.Now().UnixNano())
	probe := filepath.Join(challengeDir, ".well-known", "acme-challenge", token)
	if err := os.MkdirAll(filepath.Dir(probe), 0755); err != nil {
		return fmt.Errorf("self-check: prepare probe: %w", err)
	}
	_ = os.Chmod(filepath.Dir(probe), 0755)
	if err := os.WriteFile(probe, []byte("ok"), 0644); err != nil {
		return fmt.Errorf("self-check: write probe: %w", err)
	}
	_ = os.Chmod(probe, 0644)
	defer os.Remove(probe)

	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1/.well-known/acme-challenge/"+token, nil)
	if err != nil {
		return err
	}
	req.Host = identifiers[0] // 用申请的第一个标识符当 Host ✓（模拟 CA 的请求 ✓）
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("self-check: cannot reach port 80: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16))
	if resp.StatusCode != http.StatusOK || strings.TrimSpace(string(body)) != "ok" {
		return fmt.Errorf("self-check failed: GET /.well-known/acme-challenge/ (Host: %s) → HTTP %d body=%q；"+
			"请检查 web 服务器是否 include 了片段、以及挑战目录权限（%s）",
			identifiers[0], resp.StatusCode, strings.TrimSpace(string(body)), challengeDir)
	}
	return nil
}

func readFileIfExists(path string) ([]byte, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	return b, true
}

// ─────────────────────────── Caddy ───────────────────────────
//
// 为什么不改 Caddyfile ✗：Caddyfile **没有**"往已有站点里插一个 location"的机制 ✗，
// 另加一个同址站点块会直接报 ambiguous site definition ✗。而 Caddy 自带 **admin API**
// （默认 localhost:2019 ✓）→ 往现有 server 的 routes 里**插一条只匹配挑战路径的路由** ✓
// 既精确又不打扰既有站点 ✓；回滚就用"整份配置还原" ✓（最可靠 ✓）。

// caddyAdminAddr 返回 admin API 地址（可用 COREFUSION_CADDY_ADMIN 覆盖 ✓）。
func caddyAdminAddr() string {
	if v := strings.TrimSpace(os.Getenv("COREFUSION_CADDY_ADMIN")); v != "" {
		return strings.TrimSuffix(v, "/")
	}
	return "http://localhost:2019"
}

// caddyChallengeRouteJSON 生成"只处理 ACME 挑战"的路由 ✓。
// file_server 的 root = 挑战目录 ✓ → URI /.well-known/acme-challenge/<token>
// 正好映射到 <root>/.well-known/acme-challenge/<token> ✓（与 WebrootProvider 的写入位置一致 ✓）。
func caddyChallengeRouteJSON(challengeDir string) string {
	route := map[string]any{
		"match":    []any{map[string]any{"path": []string{"/.well-known/acme-challenge/*"}}},
		"handle":   []any{map[string]any{"handler": "file_server", "root": challengeDir}},
		"terminal": true,
	}
	b, _ := json.Marshal(route)
	return string(b)
}

// caddyPickServer80 从 /config/apps/http/servers/ 的 JSON 里挑出监听 80 的 server 名 ✓。
func caddyPickServer80(body []byte) (string, error) {
	var servers map[string]struct {
		Listen []string `json:"listen"`
	}
	if err := json.Unmarshal(body, &servers); err != nil {
		return "", fmt.Errorf("parse caddy servers: %w", err)
	}
	names := make([]string, 0, len(servers))
	for name := range servers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		for _, l := range servers[name].Listen {
			// 可能是 ":80" / "0.0.0.0:80" / "[::]:80" ✓ 统一按"以 :80 结尾"判断 ✓
			if strings.HasSuffix(l, ":80") {
				return name, nil
			}
		}
	}
	if len(names) > 0 {
		return names[0], nil // 没有显式 80 时退而取第一个 ✓（真机验证过 ✓）
	}
	return "", fmt.Errorf("no http server found in caddy config")
}

func caddyDo(method, url string, payload []byte) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := (&http.Client{Timeout: 8 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("caddy admin %s %s → HTTP %d: %s", method, url, resp.StatusCode, strings.TrimSpace(string(out)))
	}
	return out, nil
}

// configureCaddyForACME 通过 admin API 插入挑战路由 ✓；失败/用完都会把配置**完整还原** ✓。
func configureCaddyForACME(challengeDir string, identifiers []string) (func(), error) {
	admin := caddyAdminAddr()
	servers, err := caddyDo(http.MethodGet, admin+"/config/apps/http/servers/", nil)
	if err != nil {
		return func() {}, fmt.Errorf("caddy admin API unreachable (%s): %w", admin, err)
	}
	server, err := caddyPickServer80(servers)
	if err != nil {
		return func() {}, err
	}
	// 保存**整份**配置用于回滚 ✓（比"删掉刚才那条"更不易出错 ✓）
	// 保存**整份**配置 ✓（还原时用 /load = 整体替换 ✓；注意 POST /config/ 只是**合并** ✗，不够可靠 ✗）
	saved, err := caddyDo(http.MethodGet, admin+"/config/", nil)
	if err != nil {
		return func() {}, fmt.Errorf("snapshot caddy config: %w", err)
	}

	routesURL := fmt.Sprintf("%s/config/apps/http/servers/%s/routes", admin, url.PathEscape(server))

	// 先取出原有路由 ✓（可能为 null，要当空数组处理 ✓）
	rawRoutes, err := caddyDo(http.MethodGet, routesURL, nil)
	if err != nil {
		return func() {}, err
	}
	existing := []json.RawMessage{}
	if trimmed := bytes.TrimSpace(rawRoutes); len(trimmed) > 0 && string(trimmed) != "null" {
		if uerr := json.Unmarshal(trimmed, &existing); uerr != nil {
			return func() {}, fmt.Errorf("parse caddy routes: %w", uerr)
		}
	}

	// ⚠ 为什么不用 `?@index=0` ✗：**实测该参数被 Caddy 忽略** ✗，新路由会被**追加到末尾** ✗，
	// 于是永远轮不到它（前面的 catch-all 路由 terminal ✓ 会把挑战请求吞掉 ✗ → 403/404 ✗）。
	// 可靠做法：**清空该 server 的 routes 再按"我的路由在前、原有路由依次在后"重建** ✓。
	// 整个重建在毫秒级完成 ✓，且任一步失败都会用 /load 把**整份配置**还原 ✓。
	mine := json.RawMessage(caddyChallengeRouteJSON(challengeDir))
	restore := func() {
		if _, rerr := caddyDo(http.MethodPost, admin+"/load", saved); rerr != nil {
			log.Printf("[acme] restore caddy config failed: %v", rerr)
		}
	}
	if _, err := caddyDo(http.MethodDelete, routesURL, nil); err != nil {
		restore()
		return func() {}, fmt.Errorf("clear caddy routes: %w", err)
	}
	// ⚠ 必须一次 POST **整个数组** ✗：清空之后该路径上是"设置值"语义 ✓，
	// 只 POST 一个对象会被当成 RouteList 反序列化 → Caddy 报 500 ✗（真机踩到 ✓）。
	rebuilt := make([]json.RawMessage, 0, len(existing)+1)
	rebuilt = append(rebuilt, mine)
	rebuilt = append(rebuilt, existing...)
	payload, merr := json.Marshal(rebuilt)
	if merr != nil {
		restore()
		return func() {}, fmt.Errorf("encode caddy routes: %w", merr)
	}
	if _, err := caddyDo(http.MethodPost, routesURL, payload); err != nil {
		restore()
		return func() {}, fmt.Errorf("rebuild caddy routes: %w", err)
	}

	// 回读确认挑战路由确实在**第一位** ✓（否则前面的路由会把它挡住 ✗）
	if check, cerr := caddyDo(http.MethodGet, routesURL, nil); cerr == nil {
		var got []struct {
			Match []struct {
				Path []string `json:"path"`
			} `json:"match"`
		}
		if json.Unmarshal(check, &got) == nil {
			ok := false
			if len(got) > 0 {
				for _, mm := range got[0].Match {
					for _, p := range mm.Path {
						if p == "/.well-known/acme-challenge/*" {
							ok = true
						}
					}
				}
			}
			if !ok {
				restore()
				return func() {}, fmt.Errorf("caddy challenge route is not the first route after rebuild ✗")
			}
		}
	}

	// 开签前自检 ✓（和 nginx 路径同一套 ✓）
	if err := selfCheckChallenge(identifiers, challengeDir); err != nil {
		restore()
		return func() {}, err
	}
	return restore, nil
}
