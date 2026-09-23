package cert

import (
	"bytes"
	"encoding/pem"
	"strings"
	"testing"
)

// mkPEM 用 Go 自己的 pem 编码生成**合法的**证书段 ✓。
// ⚠ 不要手写 base64 夹具 ✗：长度不是 4 的倍数时 pem.Decode 会直接判非法 ✗
// （我在这一版之前就是这么踩的 ✓，导致"明明 3 段却只解析出 1 段" ✗）。
func mkPEM(marker string) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte(marker)})
}

func mkOtherPEM(marker string) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte(marker)})
}

// 叶 + 链：cert.pem 只留 1 段、fullchain 保留全部 ✓（顺序不变 ✓）
func TestSplitLeafAndChain(t *testing.T) {
	in := bytes.Join([][]byte{mkPEM("leaf"), mkPEM("intermediate"), mkPEM("root")}, nil)
	leaf, full := SplitLeafAndChain(in)
	if got := CountCertificateBlocks(leaf); got != 1 {
		t.Fatalf("leaf 应只有 1 段 ✓，实际 %d（输入共 %d 段）", got, CountCertificateBlocks(in))
	}
	if got := CountCertificateBlocks(full); got != 3 {
		t.Fatalf("full 应有 3 段 ✓，实际 %d", got)
	}
	// ⚠ PEM 体是 base64 ✗（`leaf` 会变成 `bGVhZg==` ✓），所以比对**解码后的段** ✓
	if !bytes.Equal(bytes.TrimSpace(leaf), bytes.TrimSpace(mkPEM("leaf"))) {
		t.Fatalf("leaf 应等于第一段 ✗：%q", leaf)
	}
	// 顺序：叶在前 ✓
	if !bytes.HasPrefix(bytes.TrimSpace(full), bytes.TrimSpace(mkPEM("leaf"))) {
		t.Fatalf("full 应保持原顺序（叶在前 ✓）")
	}
}

// 只有叶：两份都是 1 段 ✓
func TestSplitLeafAndChainSingle(t *testing.T) {
	in := mkPEM("only")
	leaf, full := SplitLeafAndChain(in)
	if CountCertificateBlocks(leaf) != 1 || CountCertificateBlocks(full) != 1 {
		t.Fatalf("单段应两份都 1 段 ✓：leaf=%d full=%d", CountCertificateBlocks(leaf), CountCertificateBlocks(full))
	}
}

// 非证书段要被忽略 ✓
func TestSplitLeafAndChainIgnoresNonCert(t *testing.T) {
	in := bytes.Join([][]byte{mkOtherPEM("privkey"), mkPEM("leaf"), mkPEM("ca")}, nil)
	leaf, full := SplitLeafAndChain(in)
	if strings.Contains(string(full), "PRIVATE KEY") {
		t.Fatalf("不该把私钥混进来 ✗")
	}
	if CountCertificateBlocks(leaf) != 1 || CountCertificateBlocks(full) != 2 {
		t.Fatalf("段数不对：leaf=%d full=%d（期望 1/2）", CountCertificateBlocks(leaf), CountCertificateBlocks(full))
	}
}

// 空/垃圾输入：不 panic ✓，full 原样 ✓
func TestSplitLeafAndChainEmpty(t *testing.T) {
	leaf, full := SplitLeafAndChain(nil)
	if leaf != nil || len(full) != 0 {
		t.Fatalf("空输入应返回 nil/空 ✓")
	}
	if _, got := SplitLeafAndChain([]byte("garbage")); string(got) != "garbage" {
		t.Fatalf("非 PEM 应原样返回 ✓")
	}
}

// 节点回传：只有 cert.pem（可能含链 ✓）也要能拼 fullchain ✓；重复 issuer 不重复追加 ✓
func TestBuildChainPEM(t *testing.T) {
	certOnly := mkPEM("leaf")
	issuer := mkPEM("intermediate")

	// ① 叶 + 额外 issuer → 2 段 ✓
	full := BuildChainPEM(certOnly, issuer)
	if got := CountCertificateBlocks(full); got != 2 {
		t.Fatalf("应拼成 2 段 ✓，实际 %d", got)
	}
	// ② 已包含时不得重复 ✓
	if got := CountCertificateBlocks(BuildChainPEM(full, issuer)); got != 2 {
		t.Fatalf("重复 issuer 不该再加一次 ✗，实际 %d", got)
	}
	// ③ 空 issuer → 原样 ✓
	if got := CountCertificateBlocks(BuildChainPEM(certOnly, nil)); got != 1 {
		t.Fatalf("空 issuer 应原样 ✓，实际 %d", got)
	}
	// ④ 非 PEM → 原样 ✓
	if got := string(BuildChainPEM([]byte("garbage"), issuer)); got != "garbage" {
		t.Fatalf("非 PEM 应原样返回 ✓，实际 %q", got)
	}
}

// 文件名消毒：防目录穿越 ✓（打包 zip 用 ✓）
func TestSanitizeFileName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"a.example.com", "a.example.com"},
		{"host:port", "host_port"},
		{"a b/c", "a_b_c"},
		{"wildcard-*.example.com", "wildcard-_.example.com"},
		{"", "certificate"},
		{"...", "certificate"},
	}
	for _, c := range cases {
		if got := SanitizeFileName(c.in); got != c.want {
			t.Fatalf("SanitizeFileName(%q)=%q 期望 %q", c.in, got, c.want)
		}
	}

	// 安全性质（不写死具体形态 ✓）：结果里绝不能有分隔符，也不能出现 `..` ✗
	for _, in := range []string{"../../etc/passwd", "/etc/shadow", `..\..\win`, "a/../../b", "中文域名.com"} {
		got := SanitizeFileName(in)
		if got == "" {
			t.Fatalf("(%q) 不该变成空串 ✗", in)
		}
		if strings.ContainsAny(got, `/\`) {
			t.Fatalf("消毒后仍有路径分隔符 ✗：%q → %q", in, got)
		}
		if strings.Contains(got, "..") {
			t.Fatalf("消毒后仍有 `..` ✗：%q → %q", in, got)
		}
	}
}
