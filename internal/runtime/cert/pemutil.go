package cert

import (
	"bytes"
	"encoding/pem"
	"strings"
)

// SplitLeafAndChain 把一份证书 PEM 拆成"**仅叶证书**"与"**叶 + 中间链**"两部分 ✓。
//
// 为什么需要它 ✗：ACME 返回的 `certificate.Resource.Certificate` **本身就含中间链** ✓，
// 所以面板落盘时 `cert.pem` 与 `fullchain.pem` 写的是**同一份内容** ✓（真机实测 md5 相同 ✓）。
// 这对"内核/代理直接引用 cert.pem 做 TLS"是**正确的** ✓（少了中间链，浏览器/客户端会验不过 ✗），
// 但按惯例（acme.sh 等）用户期望 `cert.pem` 只含叶证书 ✓ —— 所以**下载打包**时按惯例拆分 ✓，
// 磁盘上依旧保持"含全链"不变 ✗（避免把正在服务的 TLS 链打断 ✗）。
//
// 返回的 full 是**规范化后的完整链** ✓（按 PEM 顺序拼接，忽略非 CERTIFICATE 段 ✓）；
// 输入为空或解析不出证书段时，返回 (nil, 原样) ✓（调用方自行判断 ✓）。
func SplitLeafAndChain(certPEM []byte) (leaf, full []byte) {
	blocks := certificateBlocks(certPEM)
	if len(blocks) == 0 {
		return nil, certPEM
	}
	return blocks[0], bytes.Join(blocks, nil)
}

// CountCertificateBlocks 返回 PEM 里的证书段数 ✓（诊断/测试用 ✓）。
func CountCertificateBlocks(certPEM []byte) int {
	return len(certificateBlocks(certPEM))
}

// certificateBlocks 按顺序取出所有 CERTIFICATE 段的**规范 PEM 编码** ✓。
func certificateBlocks(certPEM []byte) [][]byte {
	var out [][]byte
	rest := certPEM
	for {
		var b *pem.Block
		b, rest = pem.Decode(rest)
		if b == nil {
			break
		}
		if b.Type != "CERTIFICATE" {
			continue
		}
		out = append(out, pem.EncodeToMemory(b))
	}
	return out
}

// BuildChainPEM 生成"叶 + 中间链"的 PEM ✓：以 certPEM 为主 ✓，
// 若额外的 issuerPEM（lego 的 IssuerCertificate ✓）不在其中则**追加** ✓。
//
// 用于：节点回传时补写 fullchain.pem ✓（真机上节点签出来的证书**只有 cert.pem** ✗，
// 缺 fullchain.pem ✓ —— 而有些部署与工具会按惯例去找它 ✓）。
func BuildChainPEM(certPEM, issuerPEM []byte) []byte {
	blocks := certificateBlocks(certPEM)
	if len(blocks) == 0 {
		return certPEM
	}
	full := bytes.Join(blocks, nil)
	for _, extra := range certificateBlocks(issuerPEM) {
		if bytes.Contains(full, bytes.TrimSpace(extra)) {
			continue
		}
		full = append(full, extra...)
	}
	return full
}

// SanitizeFileName 把证书名/域名转成安全的文件名 ✓（下载打包用 ✓）：
// 只保留字母数字与 `. _ -` ✓，其余替换为 `_` ✓，并防住 `..` / 路径分隔符 ✗。
func SanitizeFileName(name string) string {
	s := strings.TrimSpace(name)
	if s == "" {
		return "certificate"
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.' || r == '_' || r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := b.String()
	// 再把残留的 ".." 折叠掉 ✓（上面的字符映射保留了 `.` ✓，必须单独处理 ✗）
	for strings.Contains(out, "..") {
		out = strings.ReplaceAll(out, "..", "_")
	}
	out = strings.Trim(out, ".")
	if out == "" || strings.Trim(out, "_") == "" {
		return "certificate"
	}
	return out
}
