package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writePEM(t *testing.T, path, blockType string, der []byte) {
	t.Helper()
	raw := pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der})
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

// writeServerCert 生成一张自签服务器证书（SAN=panel.test），serial 用于区分新旧。
func writeServerCert(t *testing.T, dir string, serial int64) (certPath, keyPath string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject:      pkix.Name{CommonName: "panel.test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"panel.test"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPath = filepath.Join(dir, "cert.pem")
	keyPath = filepath.Join(dir, "key.pem")
	writePEM(t, certPath, "CERTIFICATE", der)
	kder, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	writePEM(t, keyPath, "EC PRIVATE KEY", kder)
	return certPath, keyPath
}

// 证书续期后必须**无需重启**就用上新证书：文件变化 → 下一次握手重读。
func TestPanelTLSReloadsServerCertificate(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath := writeServerCert(t, dir, 1001)

	p, err := newPanelTLS(certPath, keyPath, "", false)
	if err != nil {
		t.Fatalf("newPanelTLS: %v", err)
	}
	leaf, err := p.getCertificate(nil)
	if err != nil {
		t.Fatalf("getCertificate: %v", err)
	}
	if leaf.Leaf == nil || leaf.Leaf.SerialNumber.Int64() != 1001 {
		t.Fatalf("初始证书序列号不对：%+v", leaf.Leaf)
	}

	// 模拟"证书中心重签"：覆盖同一路径下的 cert/key
	newDir := t.TempDir()
	newCert, newKey := writeServerCert(t, newDir, 2002)
	certRaw, _ := os.ReadFile(newCert)
	keyRaw, _ := os.ReadFile(newKey)
	if err := os.WriteFile(certPath, certRaw, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, keyRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	// 显式推进 mtime，避免同一秒内写入导致时间戳相同
	future := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(certPath, future, future)
	_ = os.Chtimes(keyPath, future, future)

	leaf2, err := p.getCertificate(nil)
	if err != nil {
		t.Fatalf("getCertificate after renew: %v", err)
	}
	if leaf2.Leaf == nil || leaf2.Leaf.SerialNumber.Int64() != 2002 {
		t.Fatalf("续期后仍用旧证书（serial=%v），热加载未生效", leaf2.Leaf.SerialNumber)
	}
}

// 端到端：真实 http.Server + ServeTLS 也必须换证书即生效，
// 且不能在 ServeTLS 里又把 certFile/keyFile 读死（那样热加载就白做了）。
func TestHTTPServerServesRenewedCertificateWithoutRestart(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath := writeServerCert(t, dir, 11)

	p, err := newPanelTLS(certPath, keyPath, "", false)
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})}
	srv.TLSConfig = p.Config()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.ServeTLS(ln, "", "") }()
	defer srv.Close()
	addr := ln.Addr().String()

	serial := func() int64 {
		conn, err := tls.Dial("tcp", addr, &tls.Config{InsecureSkipVerify: true})
		if err != nil {
			t.Fatalf("TLS dial: %v", err)
		}
		defer conn.Close()
		return conn.ConnectionState().PeerCertificates[0].SerialNumber.Int64()
	}

	// 首次连接要等监听就绪（ServeTLS 在 goroutine 里）
	var first int64
	for i := 0; i < 50; i++ {
		conn, derr := tls.Dial("tcp", addr, &tls.Config{InsecureSkipVerify: true})
		if derr == nil {
			first = conn.ConnectionState().PeerCertificates[0].SerialNumber.Int64()
			conn.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if first != 11 {
		t.Fatalf("初始证书 serial=%d，应为 11", first)
	}

	// 覆盖证书文件 = 证书中心重签
	newDir := t.TempDir()
	newCert, newKey := writeServerCert(t, newDir, 22)
	certRaw, _ := os.ReadFile(newCert)
	keyRaw, _ := os.ReadFile(newKey)
	if err := os.WriteFile(certPath, certRaw, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, keyRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(certPath, future, future)
	_ = os.Chtimes(keyPath, future, future)

	if got := serial(); got != 22 {
		t.Fatalf("续期后新连接仍拿到旧证书 serial=%d（应为 22），说明仍需要重启", got)
	}
}

func writeCA(t *testing.T, dir, name string) (*x509.Certificate, *ecdsa.PrivateKey, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	pair, err := tls.X509KeyPair(
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: mustECKey(t, key)}),
	)
	if err != nil {
		t.Fatal(err)
	}
	_ = pair
	return cert, key, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func mustECKey(t *testing.T, key *ecdsa.PrivateKey) []byte {
	t.Helper()
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return der
}

func clientLeafSignedBy(t *testing.T, caCert *x509.Certificate, caKey *ecdsa.PrivateKey) *x509.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "agent"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return leaf
}

func verifyWithPool(leaf *x509.Certificate, pool *x509.CertPool) error {
	_, err := leaf.Verify(x509.VerifyOptions{
		Roots:     pool,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	})
	return err
}

// mTLS 客户端 CA 换掉后必须**无需重启**生效（否则旧 agent 证书被续成新 CA 后会连不上）。
func TestPanelTLSReloadsMTLSClientCA(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath := writeServerCert(t, dir, 1)

	caA, caAKey, caAPEM := writeCA(t, dir, "CA-A")
	caB, caBKey, caBPEM := writeCA(t, dir, "CA-B")
	_ = caAKey

	caPath := filepath.Join(dir, "mtls-ca.pem")
	if err := os.WriteFile(caPath, caAPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := newPanelTLS(certPath, keyPath, caPath, true)
	if err != nil {
		t.Fatalf("newPanelTLS: %v", err)
	}
	cfg, err := p.getConfigForClient(&tls.Config{NextProtos: []string{"h2", "http/1.1"}})
	if err != nil || cfg == nil {
		t.Fatalf("mTLS 配置应为非空：cfg=%v err=%v", cfg, err)
	}
	if cfg.ClientAuth != tls.RequireAndVerifyClientCert || cfg.ClientCAs == nil {
		t.Fatalf("mTLS 配置不正确: auth=%v pool=%v", cfg.ClientAuth, cfg.ClientCAs)
	}
	if len(cfg.NextProtos) == 0 || cfg.NextProtos[0] != "h2" {
		t.Fatalf("GetConfigForClient 返回的配置必须带上 h2（否则 ALPN 退化）: %v", cfg.NextProtos)
	}

	leafA := clientLeafSignedBy(t, caA, caAKey)
	leafB := clientLeafSignedBy(t, caB, caBKey)
	if err := verifyWithPool(leafA, cfg.ClientCAs); err != nil {
		t.Fatalf("CA-A 签发的客户端证书应通过：%v", err)
	}
	if err := verifyWithPool(leafB, cfg.ClientCAs); err == nil {
		t.Fatal("CA-B 签发的证书不该被 CA-A 接受")
	}

	// 轮换 CA：覆盖文件并推进 mtime
	if err := os.WriteFile(caPath, caBPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(caPath, future, future)

	cfg2, err := p.getConfigForClient(&tls.Config{NextProtos: []string{"h2", "http/1.1"}})
	if err != nil || cfg2 == nil {
		t.Fatalf("轮换后配置为空：cfg=%v err=%v", cfg2, err)
	}
	if err := verifyWithPool(leafB, cfg2.ClientCAs); err != nil {
		t.Fatalf("轮换后新的客户端 CA 应生效：%v", err)
	}
	if err := verifyWithPool(leafA, cfg2.ClientCAs); err == nil {
		t.Fatal("轮换后旧 CA 不应再被接受")
	}
}

// mTLS CA 路径误配成不存在时，必须继续用旧 CA，不能静默降级成"不验证客户端证书"。
func TestPanelTLSApplyKeepsPreviousCAOnBadPath(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath := writeServerCert(t, dir, 2)
	caA, caAKey, caAPEM := writeCA(t, dir, "CA-A")
	caPath := filepath.Join(dir, "mtls-ca.pem")
	if err := os.WriteFile(caPath, caAPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := newPanelTLS(certPath, keyPath, caPath, true)
	if err != nil {
		t.Fatal(err)
	}
	p.apply(certPath, keyPath, filepath.Join(dir, "missing-ca.pem"), true)
	cfg, err := p.getConfigForClient(&tls.Config{NextProtos: []string{"h2", "http/1.1"}})
	if err != nil || cfg == nil || cfg.ClientCAs == nil {
		t.Fatalf("坏 CA 路径不应让 mTLS 失效: cfg=%v err=%v", cfg, err)
	}
	if err := verifyWithPool(clientLeafSignedBy(t, caA, caAKey), cfg.ClientCAs); err != nil {
		t.Fatalf("坏 CA 路径后旧 CA 仍应生效: %v", err)
	}
}

// 设置里路径为空（未配置）时不能被应用成"没有证书"，否则面板直接不可用。
func TestPanelTLSApplyIgnoresEmptyPaths(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath := writeServerCert(t, dir, 7)
	p, err := newPanelTLS(certPath, keyPath, "", false)
	if err != nil {
		t.Fatal(err)
	}
	p.apply("", "", "", false)
	leaf, err := p.getCertificate(nil)
	if err != nil || leaf.Leaf == nil || leaf.Leaf.SerialNumber.Int64() != 7 {
		t.Fatalf("空路径不应清掉当前证书: err=%v", err)
	}
}

// 切到不存在的坏路径时，必须继续用旧证书（否则一次误配会让面板 HTTPS 全挂）。
func TestPanelTLSApplyKeepsPreviousCertOnBadPath(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath := writeServerCert(t, dir, 8)
	p, err := newPanelTLS(certPath, keyPath, "", false)
	if err != nil {
		t.Fatal(err)
	}
	p.apply(filepath.Join(dir, "missing-cert.pem"), filepath.Join(dir, "missing-key.pem"), "", false)
	leaf, err := p.getCertificate(nil)
	if err != nil || leaf.Leaf == nil || leaf.Leaf.SerialNumber.Int64() != 8 {
		t.Fatalf("坏路径不应清掉当前证书: err=%v", err)
	}
	if p.certPath != certPath || p.keyPath != keyPath {
		t.Fatalf("坏路径不应覆盖当前路径: cert=%s key=%s", p.certPath, p.keyPath)
	}

	// 改回正确的新证书后仍应正常热加载 ✓
	newDir := t.TempDir()
	newCert, newKey := writeServerCert(t, newDir, 9)
	p.apply(newCert, newKey, "", false)
	leaf2, err := p.getCertificate(nil)
	if err != nil || leaf2.Leaf == nil || leaf2.Leaf.SerialNumber.Int64() != 9 {
		t.Fatalf("恢复正确路径后应加载新证书: err=%v", err)
	}
}
