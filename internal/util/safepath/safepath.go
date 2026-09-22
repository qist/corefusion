// Package safepath 提供"把来自对端的字符串当作路径片段/路径使用时"的最小校验。
//
// 背景（安全审计 v5 H2/H3/H4）：面板会指挥 Agent 落盘/读取文件，而 **Agent 以 root 运行**。
// 之前的实现里，面板下发的字符串有几种用法完全没校验：
//
//	filepath.Join(rulesDir, req.FileName)          // DownloadGeoRule
//	filepath.Join(wafDir, ".."+ruleName)           // ReadConfigFile("waf_rule:../../etc/shadow")
//	certPath := certEntry.CertPath                 // 绝对路径原样写入
//
// 一旦有 `..` 或分隔符，`filepath.Join` 会直接**逃出目标目录** ✗ —— 等价于"以 root 写/读任意文件"。
//
// 设计原则：**只拒绝明显非法的形态，不限制合法用法** —— 管理员确实会配置绝对路径
// （例如把证书写到 nginx 自己的目录），所以这里不做"必须落在某个目录内"的限制（那会打断
// 现有功能），只做两件确定安全的事：
//  1. SafeName：要求是"单纯的名字"（不含分隔符、不是 . / ..）—— 用于文件名、核心名等片段；
//  2. RejectTraversal：拒绝含 `..` 的路径 —— 用于允许绝对路径的场景。
package safepath

import (
	"fmt"
	"path/filepath"
	"strings"
)

// SafeName 判断 s 是否可以作为"单纯的路径片段"（文件名、核心名、规则名）使用。
// 不含路径分隔符、不是 "." / ".."、非空、不含 NUL。
func SafeName(s string) bool {
	if s == "" || s == "." || s == ".." {
		return false
	}
	if strings.ContainsAny(s, `/\`+"\x00") {
		return false
	}
	return true
}

// CheckName 在 SafeName 不通过时返回带上下文的错误（便于日志定位）。
func CheckName(what, s string) error {
	if !SafeName(s) {
		return fmt.Errorf("invalid %s %q: must be a plain name (no path separator, not '.' or '..')", what, s)
	}
	return nil
}

// RejectTraversal 拒绝含上跳（`..`）或空字节的路径；允许相对与绝对路径。
// 仅用于"管理员本来就可以指定任意路径"的场景，作为纵深防御。
func RejectTraversal(what, p string) error {
	if p == "" {
		return fmt.Errorf("empty %s", what)
	}
	if strings.ContainsRune(p, '\x00') {
		return fmt.Errorf("invalid %s: contains NUL", what)
	}
	for _, seg := range strings.Split(filepath.ToSlash(p), "/") {
		if seg == ".." {
			return fmt.Errorf("invalid %s %q: path traversal ('..') is not allowed", what, p)
		}
	}
	return nil
}

// ResolveUnder 把 p 解析到 root 之下：相对路径按 root 拼接，绝对路径原样使用，
// 但**最终必须仍在 root 内**（防止 `..` 与绝对路径逃逸）。用于"只允许写在数据目录内"的场景。
func ResolveUnder(root, p string) (string, error) {
	if err := RejectTraversal("path", p); err != nil {
		return "", err
	}
	cleanRoot := filepath.Clean(root)
	joined := filepath.Clean(p)
	if !filepath.IsAbs(joined) {
		joined = filepath.Join(cleanRoot, joined)
	}
	rel, err := filepath.Rel(cleanRoot, joined)
	if err != nil {
		return "", fmt.Errorf("resolve %q under %q: %w", p, root, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes %q", p, root)
	}
	return joined, nil
}
