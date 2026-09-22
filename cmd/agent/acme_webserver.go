package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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
func detectWebServer() string {
	for _, name := range []string{"nginx", "caddy"} {
		if out, err := exec.Command("pgrep", "-x", name).Output(); err == nil && strings.TrimSpace(string(out)) != "" {
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

	// ① 80 空闲 → 临时起服务 ✓
	if portIsFree(80) {
		return "", func() {}, nil
	}

	// ② 80 被占用 → 自动配置现有 web 服务器 ✓
	server := detectWebServer()
	switch server {
	case "nginx":
		dir := acmeChallengeDir(dataDir)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", func() {}, err
		}
		// web 服务器以非 root 运行 ✗：目录链必须可穿越 ✓
		// （显式 chmod ✓，否则会被 umask 削减成 0700 ✗ → 挑战恒为 404 ✗，真机踩过 ✓）
		_ = os.Chmod(dir, 0755)
		_ = os.Chmod(filepath.Dir(dir), 0755)
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
			"(停止占用 80 的服务，或显式指定 webroot)", server)
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

	// 给 nginx 一点时间落地 ✓
	time.Sleep(300 * time.Millisecond)

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
