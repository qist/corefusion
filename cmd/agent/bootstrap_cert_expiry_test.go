package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeCertPair 写一对证书/私钥 PEM 到 certPath/keyPath。
func writeCertPair(t *testing.T, certPath, keyPath string, notBefore, notAfter time.Time) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: panelCertServerName},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	if err := os.MkdirAll(filepath.Dir(certPath), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(certPath, certPEM, 0640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		t.Fatal(err)
	}
}

// 面板关停太久（超过证书有效期）后：证书存在但已过期。
// 需要区分出"有证书但过期"和"首次安装没证书"，前者不应让进程直接退出。
func TestExpiredAgentCertIsDetectedNotTreatedAsFirstInstall(t *testing.T) {
	dir := t.TempDir()
	certs := filepath.Join(dir, "certs")
	a := &AgentApp{
		cfg:      &AgentConfig{},
		dataDir:  dir,
		certPath: filepath.Join(certs, "agent-cert.pem"),
		keyPath:  filepath.Join(certs, "agent-key.pem"),
		caPath:   filepath.Join(certs, "ca-cert.pem"),
	}

	// 首次安装（什么都没有）
	if a.HasCertFiles() {
		t.Fatal("HasCertFiles() 在没有任何证书时不应为 true")
	}

	expired := time.Now().Add(-48 * time.Hour)
	writeCertPair(t, a.certPath, a.keyPath, expired.Add(-365*24*time.Hour), expired)
	if err := os.WriteFile(a.caPath, []byte("ca"), 0640); err != nil {
		t.Fatal(err)
	}

	if !a.HasCertFiles() {
		t.Fatal("证书文件齐全时 HasCertFiles() 应为 true")
	}
	if a.hasValidCert() {
		t.Fatal("过期证书不能被当成有效证书")
	}
	got, ok := a.agentCertExpiry()
	if !ok {
		t.Fatal("agentCertExpiry() 应能读出过期时间")
	}
	if time.Since(got) < 47*time.Hour {
		t.Fatalf("证书过期时间 = %v，期望约 48 小时前", got)
	}
}

// 有效证书：agentCertExpiry 正常返回，hasValidCert 为 true。
func TestValidAgentCert(t *testing.T) {
	dir := t.TempDir()
	certs := filepath.Join(dir, "certs")
	a := &AgentApp{
		cfg:      &AgentConfig{},
		dataDir:  dir,
		certPath: filepath.Join(certs, "agent-cert.pem"),
		keyPath:  filepath.Join(certs, "agent-key.pem"),
		caPath:   filepath.Join(certs, "ca-cert.pem"),
	}
	writeCertPair(t, a.certPath, a.keyPath, time.Now().Add(-time.Hour), time.Now().Add(30*24*time.Hour))
	if err := os.WriteFile(a.caPath, []byte("ca"), 0640); err != nil {
		t.Fatal(err)
	}
	if !a.hasValidCert() {
		t.Fatal("有效证书应通过 hasValidCert")
	}
	if _, ok := a.agentCertExpiry(); !ok {
		t.Fatal("agentCertExpiry() 应能读出过期时间")
	}
}
