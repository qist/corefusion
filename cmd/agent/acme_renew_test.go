package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"corefusion/internal/runtime/cert"
)

// IP 证书用 2 天阈值 ✓、域名证书用 10 天阈值 ✓（都比面板晚一步 ✓）
func TestAgentRenewThreshold(t *testing.T) {
	cases := []struct {
		ids  []string
		want time.Duration
	}{
		{[]string{"1.2.3.4"}, agentIPRenewBefore},
		{[]string{"example.com", "203.0.113.48"}, agentIPRenewBefore}, // 混合 → 按短时证书 ✓
		{[]string{"example.com"}, agentDomainRenewBefore},
		{[]string{"*.example.com"}, agentDomainRenewBefore},
		{nil, agentDomainRenewBefore},
	}
	for _, c := range cases {
		if got := agentRenewThreshold(c.ids); got != c.want {
			t.Fatalf("agentRenewThreshold(%v)=%v 期望 %v", c.ids, got, c.want)
		}
	}
}

func TestNeedsRenewAt(t *testing.T) {
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		left time.Duration
		ids  []string
		want bool
	}{
		{"IP 只剩 1.5 天 → 要续", 36 * time.Hour, []string{"1.2.3.4"}, true},
		{"IP 还有 5 天 → 不续", 5 * 24 * time.Hour, []string{"1.2.3.4"}, false},
		{"域名只剩 9 天 → 要续", 9 * 24 * time.Hour, []string{"a.com"}, true},
		{"域名还有 30 天 → 不续", 30 * 24 * time.Hour, []string{"a.com"}, false},
		{"已过期 → 要续", -time.Hour, []string{"a.com"}, true},
	}
	for _, c := range cases {
		got := needsRenewAt(now, now.Add(c.left), c.ids)
		if got != c.want {
			t.Fatalf("%s：needsRenewAt=%v 期望 %v", c.name, got, c.want)
		}
	}
	// 解析不到到期时间 → 宁可信其快过期 ✓
	if !needsRenewAt(now, time.Time{}, []string{"a.com"}) {
		t.Fatalf("到期时间未知时应当续 ✓（续一次总比断线好 ✓）")
	}
}

// meta 落盘/读回 ✓；没有 meta 必须是 os.IsNotExist ✓（这是"不归节点自管"的判据 ✓）
func TestCertMetaRoundTrip(t *testing.T) {
	dir := t.TempDir()
	m := &certMeta{
		CertID:      7,
		Identifiers: []string{"a.com", "203.0.113.48"},
		Email:       "x@y.z",
		CA:          "lets_encrypt",
		Challenge:   "dns-01",
		DNSProvider: "cloudflare",
		DNSEnvVars:  map[string]string{"CF_Token": "secret"},
		IssuedAt:    time.Now().Add(-time.Hour).Truncate(time.Second),
	}
	if err := writeCertMeta(dir, "a.com", m); err != nil {
		t.Fatal(err)
	}
	got, err := readCertMeta(dir, "a.com")
	if err != nil {
		t.Fatal(err)
	}
	if got.CertID != 7 || got.CA != "lets_encrypt" || got.DNSEnvVars["CF_Token"] != "secret" {
		t.Fatalf("读回内容不符：%+v", got)
	}
	// 权限必须是 0600 ✓（里面有 DNS 凭据 ✗）
	st, serr := os.Stat(certMetaPath(dir, "a.com"))
	if serr != nil {
		t.Fatal(serr)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("meta 权限应为 0600 ✓，实际 %v", st.Mode().Perm())
	}
	// 不存在的 → os.IsNotExist ✓
	if _, err := readCertMeta(dir, "nope"); !os.IsNotExist(err) {
		t.Fatalf("缺 meta 应返回 os.IsNotExist ✓，实际 %v", err)
	}
}

// 列出本机证书目录 ✓：没有 cert.pem 的目录要跳过 ✓、目录不存在不算错 ✓
func TestListLocalCertDirs(t *testing.T) {
	dir := t.TempDir()
	if names, err := listLocalCertDirs(dir); err != nil || len(names) != 0 {
		t.Fatalf("目录不存在时应返回空且无错 ✓：%v %v", names, err)
	}
	mk := func(name string, withCert bool) {
		d := filepath.Join(dir, "certs", name)
		if err := os.MkdirAll(d, 0o750); err != nil {
			t.Fatal(err)
		}
		if withCert {
			if err := os.WriteFile(filepath.Join(d, "cert.pem"), []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	mk("b.com", true)
	mk("a.com", true)
	mk("halfway", false) // 没签完的目录 ✓ 跳过 ✓
	names, err := listLocalCertDirs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 || names[0] != "a.com" || names[1] != "b.com" {
		t.Fatalf("应只列出签好的目录且有序 ✓，实际 %v", names)
	}
}

// 标识符取自证书 SAN ✓（域名 + IP ✓，去重 ✓）
func TestCertIdentifiersFromInfo(t *testing.T) {
	info := &cert.CertificateInfo{
		DNSNames:    []string{"a.com", "a.com", " "},
		IPAddresses: []string{"1.2.3.4"},
	}
	got := certIdentifiersFromInfo(info)
	if len(got) != 2 || got[0] != "a.com" || got[1] != "1.2.3.4" {
		t.Fatalf("标识符提取不对 ✓：%v", got)
	}
	if certIdentifiersFromInfo(nil) != nil {
		t.Fatalf("nil 应返回 nil ✓")
	}
}

// meta → 签发参数：关键点是**续期不带 EAB** ✓（账号已在本地 ✓，不再消耗新 EAB ✓）
func TestIssueParamsFromMeta(t *testing.T) {
	app := &AgentApp{cfg: &AgentConfig{}}
	m := &certMeta{
		CertID:      3,
		Identifiers: []string{" 1.2.3.4 ", "a.com"},
		Email:       "e@x.y",
		CA:          "google",
		Challenge:   "dns-01",
		DNSProvider: "cloudflare",
		DNSEnvVars:  map[string]string{"CF_Token": "t"},
	}
	p := m.issueParams(app, "/tmp/acme")
	if len(p.Identifiers) != 2 || p.Identifiers[0] != "1.2.3.4" {
		t.Fatalf("标识符应去空白 ✓：%v", p.Identifiers)
	}
	if p.CertID != 3 || p.CA != "google" || p.DNSEnvVars["CF_Token"] != "t" {
		t.Fatalf("参数还原不对 ✓：%+v", p)
	}
	if p.EABKeyID != "" || p.EABMACKey != "" {
		t.Fatalf("续期**不该**带 EAB ✗（账号已在节点本地 ✓）：%+v", p)
	}
	// 修改返回的 env 不应影响 meta 本体 ✓（避免调用方意外改坏内存里的 meta ✓）
	p.DNSEnvVars["CF_Token"] = "changed"
	if m.DNSEnvVars["CF_Token"] != "t" {
		t.Fatalf("map 应做拷贝 ✓")
	}
}
