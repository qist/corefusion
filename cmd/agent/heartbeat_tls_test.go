package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"testing"
	"time"
)

// 这组测试对应安全审计 v5 的 C2：Agent 原本用 `InsecureSkipVerify: true` 连面板 ✗ ——
// 任何能 MITM 的人都能用一张自签证书冒充面板，而面板可以指挥 Agent 以 **root** 写任意路径、
// 执行命令。修复后必须满足：① 配置里不允许关闭校验；② 真面板证书能握手成功；
// ③ **不受信 CA 签发的证书必须被拒**；④ 证书名字不匹配必须被拒。

func mintCA(t *testing.T) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("生成 CA 私钥: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{Organization: []string{"CoreFusion"}, CommonName: "CoreFusion Agent CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("签发 CA: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("解析 CA: %v", err)
	}
	return cert, key
}

func mintServerCert(t *testing.T, ca *x509.Certificate, caKey *ecdsa.PrivateKey, dnsNames []string) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("生成服务端私钥: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{Organization: []string{"CoreFusion"}, CommonName: dnsNames[0]},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     dnsNames,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca, &key.PublicKey, caKey)
	if err != nil {
		t.Fatalf("签发服务端证书: %v", err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("解析服务端证书: %v", err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}
}

// handshakeOK 起一个 TLS 服务端（用给定证书），并用**生产代码构造的客户端配置**去握手。
func handshakeOK(t *testing.T, serverCert tls.Certificate, pool *x509.CertPool) bool {
	t.Helper()
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		MinVersion:   tls.VersionTLS13,
	})
	if err != nil {
		t.Fatalf("起 TLS 监听: %v", err)
	}
	defer ln.Close()
	go func() {
		conn, aerr := ln.Accept()
		if aerr != nil {
			return
		}
		defer conn.Close()
		if c, ok := conn.(*tls.Conn); ok {
			_ = c.SetDeadline(time.Now().Add(3 * time.Second))
			_ = c.Handshake()
		}
	}()

	cfg := buildPanelTLSConfig(tls.Certificate{}, pool)
	dialer := &net.Dialer{Timeout: 3 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", ln.Addr().String(), cfg)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func TestBuildPanelTLSConfigNeverSkipsVerification(t *testing.T) {
	pool := x509.NewCertPool()
	cfg := buildPanelTLSConfig(tls.Certificate{}, pool)

	if cfg.InsecureSkipVerify {
		t.Fatal("不得关闭服务端证书校验 —— Agent 不校验对端身份就能被人冒充面板并以 root 接管节点")
	}
	if cfg.ServerName != panelCertServerName {
		t.Errorf("ServerName 应为面板证书名 %q，得到 %q", panelCertServerName, cfg.ServerName)
	}
	if cfg.RootCAs != pool {
		t.Error("必须使用 bootstrap 落盘的 CA 做校验")
	}
	if cfg.MinVersion != tls.VersionTLS13 {
		t.Errorf("最低 TLS 版本应为 1.3，得到 %x", cfg.MinVersion)
	}
}

func TestPanelTLSConfigAcceptsRealPanelAndRejectsImpostor(t *testing.T) {
	ca, caKey := mintCA(t)
	pool := x509.NewCertPool()
	pool.AddCert(ca)

	// ① 真面板：由该 CA 签发、SAN 含 corefusion-panel → 必须握手成功
	realPanel := mintServerCert(t, ca, caKey, []string{panelCertServerName, "localhost"})
	if !handshakeOK(t, realPanel, pool) {
		t.Fatal("正常面板证书应当握手成功（否则修复会打断所有节点连接）")
	}

	// ② 冒充者：另一套 CA 自签 → 必须被拒（这就是修复要挡住的攻击）
	rogueCA, rogueKey := mintCA(t)
	impostor := mintServerCert(t, rogueCA, rogueKey, []string{panelCertServerName})
	if handshakeOK(t, impostor, pool) {
		t.Fatal("不受信 CA 签发的证书被接受了 —— 攻击者可以冒充面板控制 Agent")
	}

	// ③ 受信 CA 签发但名字不对 → 必须被拒
	wrongName := mintServerCert(t, ca, caKey, []string{"evil.example.com"})
	if handshakeOK(t, wrongName, pool) {
		t.Fatal("证书名不匹配被接受了 —— ServerName 校验形同虚设")
	}
}
