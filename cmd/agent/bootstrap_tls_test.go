package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newBootstrapApp(t *testing.T, panelTLS bool) *AgentApp {
	t.Helper()
	dir := t.TempDir()
	cfg := &AgentConfig{}
	cfg.Panel.Host = "192.0.2.10"
	cfg.Panel.Port = 8080
	cfg.Panel.TLS = panelTLS
	return &AgentApp{cfg: cfg, dataDir: dir, caPath: filepath.Join(dir, "certs", "ca-cert.pem")}
}

// 安全审计 v5 M6：引导阶段原本 `InsecureSkipVerify: true` ✗ —— 任何人 MITM 都能冒充面板，
// 把 Agent 引导到假面板上并拿到它的私钥。修复后必须：
//   - 面板走 http → 不需要 TLS 配置；
//   - 有面板 CA（安装脚本自动下发）→ 用它做正常链校验（不再跳过校验）；
//   - 没 CA → TOFU：首次自动固定指纹，之后不一致就拒绝（同样无需人工填写）。
func TestBootstrapTLSConfigPrefersPanelCA(t *testing.T) {
	a := newBootstrapApp(t, false)
	if cfg, err := a.bootstrapTLSConfig(); err != nil || cfg != nil {
		t.Fatalf("http 面板不应有 TLS 配置：cfg=%v err=%v", cfg, err)
	}

	a = newBootstrapApp(t, true)
	// 没有 CA 文件 → TOFU 路径
	cfg, err := a.bootstrapTLSConfig()
	if err != nil {
		t.Fatalf("TOFU 路径不应报错：%v", err)
	}
	if cfg.VerifyPeerCertificate == nil {
		t.Fatal("没有 CA 时必须自己做证书校验（TOFU），不能什么都不校验")
	}
	if cfg.ServerName != panelCertServerName {
		t.Errorf("ServerName 应为面板证书名 %q，得到 %q", panelCertServerName, cfg.ServerName)
	}

	// 写入 CA 文件 → 必须走正常链校验（不再 InsecureSkipVerify）
	caPEM := selfSignedCAPEM(t)
	if err := os.MkdirAll(filepath.Dir(a.caPath), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.caPath, caPEM, 0640); err != nil {
		t.Fatal(err)
	}
	cfg, err = a.bootstrapTLSConfig()
	if err != nil {
		t.Fatalf("有 CA 时不应报错：%v", err)
	}
	if cfg.InsecureSkipVerify {
		t.Error("有面板 CA 时不得跳过校验")
	}
	if cfg.RootCAs == nil {
		t.Error("必须用面板 CA 作为根证书")
	}
	if cfg.MinVersion != tls.VersionTLS13 {
		t.Errorf("最低 TLS 版本应为 1.3，得到 %x", cfg.MinVersion)
	}
}

func TestTOFUPinAcceptsOnceThenRequiresSameCert(t *testing.T) {
	a := newBootstrapApp(t, true)
	cfg, err := a.bootstrapTLSConfig()
	if err != nil {
		t.Fatal(err)
	}

	certA := []byte("panel-cert-A")
	certB := []byte("panel-cert-B")

	// ① 首次：接受并落盘指纹
	if err := cfg.VerifyPeerCertificate([][]byte{certA}, nil); err != nil {
		t.Fatalf("首次应接受并固定：%v", err)
	}
	pinPath := a.panelTLSPinPath()
	if _, err := os.Stat(pinPath); err != nil {
		t.Fatalf("指纹未落盘：%v", err)
	}
	if info, _ := os.Stat(pinPath); info.Mode().Perm() != 0600 {
		t.Errorf("指纹文件权限应为 0600，实际 %v", info.Mode().Perm())
	}

	// ② 再次相同证书：继续接受
	if err := cfg.VerifyPeerCertificate([][]byte{certA}, nil); err != nil {
		t.Errorf("同一证书应继续接受：%v", err)
	}

	// ③ 换了证书（中间人换了面板）：必须拒绝
	if err := cfg.VerifyPeerCertificate([][]byte{certB}, nil); err == nil {
		t.Error("证书变了必须拒绝（否则 MITM 可以随时替换假面板）")
	}

	// ④ 没有证书：拒绝
	if err := cfg.VerifyPeerCertificate(nil, nil); err == nil {
		t.Error("对端没给证书必须拒绝")
	}
}

func TestCertChainFingerprintUsesRoot(t *testing.T) {
	if certChainFingerprint(nil) != "" {
		t.Error("空链应返回空")
	}
	leaf := []byte("leaf")
	root := []byte("root")
	// 用链上最后一环（根/签发者）：面板服务端证书每次启动重签，叶子指纹每次都变
	sum := sha256.Sum256(root)
	want := hex.EncodeToString(sum[:])
	if got := certChainFingerprint([][]byte{leaf, root}); got != want {
		t.Errorf("应取链上最后一环的指纹：got=%s want=%s", got, want)
	}
	if got := certChainFingerprint([][]byte{root}); got != want {
		t.Errorf("单证书链应取自身：got=%s want=%s", got, want)
	}
}

// selfSignedCAPEM 生成一张真实的自签 CA 证书（AppendCertsFromPEM 会拒绝假 PEM）。
func selfSignedCAPEM(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
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
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}
