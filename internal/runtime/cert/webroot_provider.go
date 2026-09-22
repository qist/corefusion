package cert

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WebrootProvider 通过"往 webroot 写挑战文件"来满足 ACME HTTP-01。
//
// 为什么需要它：很多节点的 **80 端口已被 nginx/caddy 占用**（例如跑着 Harbor 之类服务 ✗），
// 而 ACME 的 HTTP-01 必须由 80 端口提供挑战 ✗ → 既不能停掉现有服务 ✗，也不该为了签发
// 去独占 80 ✗。把挑战文件写进**现有 web 服务器的站点根目录**是最小侵入的做法 ✓
// （只需在站点里放行 /.well-known/acme-challenge/ ✓，无需重启 ✓）。
//
// 对应 lego 的 challenge.Provider 接口（Present / CleanUp）✓。
type WebrootProvider struct {
	// Root 是站点根目录（挑战文件写到 <root>/.well-known/acme-challenge/<token>）
	Root string
	// PathPrefix 允许自定义前缀（默认 /.well-known/acme-challenge）
	PathPrefix string
}

// NewWebrootProvider 创建 webroot 挑战提供者。
func NewWebrootProvider(root string) *WebrootProvider {
	return &WebrootProvider{Root: root, PathPrefix: "/.well-known/acme-challenge"}
}

func (w *WebrootProvider) challengePath(token string) (string, error) {
	if strings.TrimSpace(w.Root) == "" {
		return "", fmt.Errorf("webroot is empty")
	}
	if strings.ContainsAny(token, "/\\") || token == "" {
		return "", fmt.Errorf("invalid challenge token")
	}
	return filepath.Join(w.Root, w.PathPrefix, token), nil
}

// Present 写入挑战文件（0644，web 服务器可读 ✓）。
func (w *WebrootProvider) Present(domain, token, keyAuth string) error {
	path, err := w.challengePath(token)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create challenge dir %s: %w", dir, err)
	}
	// ⚠ MkdirAll 的权限会被 **umask** 削减 ✗（umask 077 时 0755 变成 0700 ✗）→
	// 而 web 服务器（nginx/caddy）通常以**非 root** 运行 ✗ → 穿不过去就是 404 ✗。
	// 所以这里显式 chmod ✓（真机踩到过这个坑 ✓）。
	if err := os.Chmod(dir, 0755); err != nil {
		return fmt.Errorf("chmod challenge dir %s: %w", dir, err)
	}
	// 写临时文件再 rename：避免 web 服务器读到半截内容 ✗
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(keyAuth), 0644); err != nil {
		return fmt.Errorf("write challenge file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("publish challenge file: %w", err)
	}
	// 同样显式放开读权限 ✓（umask 不能影响 web 服务器读挑战文件 ✓）
	if err := os.Chmod(path, 0644); err != nil {
		return fmt.Errorf("chmod challenge file: %w", err)
	}
	return nil
}

// CleanUp 删除挑战文件（幂等 ✓）。
func (w *WebrootProvider) CleanUp(domain, token, keyAuth string) error {
	path, err := w.challengePath(token)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
