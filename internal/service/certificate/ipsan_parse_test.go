package certificate

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeTestCert 生成一张带指定 DNS/IP SAN 的自签证书，返回 PEM 路径。
func writeTestCert(t *testing.T, dnsNames []string, ips []string) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: ""},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		DNSNames:     dnsNames,
	}
	for _, s := range ips {
		if ip := net.ParseIP(s); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "cert.pem")
	if err := os.WriteFile(p, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

// 关键回归：IP 证书没有 DNS SAN，身份在 IP SAN 里 —— 解析器必须认 ✓
func TestParseCertDomainsIncludesIPSAN(t *testing.T) {
	p := writeTestCert(t, nil, []string{"203.0.113.48"})

	if got := parseCertDomains(p); got != `["203.0.113.48"]` {
		t.Fatalf("parseCertDomains = %s，期望 [\"203.0.113.48\"]（不得退化成 [\"\"] ✗）", got)
	}
	list := parseCertDomainList(p)
	if len(list) != 1 || list[0] != "203.0.113.48" {
		t.Fatalf("parseCertDomainList = %v，期望 [203.0.113.48]", list)
	}
}

func TestParseCertDomainsDomainOnly(t *testing.T) {
	p := writeTestCert(t, []string{"test.example.com"}, nil)
	if got := parseCertDomains(p); got != `["test.example.com"]` {
		t.Fatalf("parseCertDomains = %s", got)
	}
}

// 混合（DNS + IP）时两者都要在 ✓
func TestParseCertDomainsMixed(t *testing.T) {
	p := writeTestCert(t, []string{"test.example.com"}, []string{"203.0.113.48"})
	got := parseCertDomains(p)
	if !strings.Contains(got, "test.example.com") || !strings.Contains(got, "203.0.113.48") {
		t.Fatalf("混合 SAN 解析不全: %s", got)
	}
}
