package certificate

import (
	"context"
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
	"testing"
	"time"

	sqlitedrv "github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// makeAgentCert 造一张自签证书（含 SAN ✓）+ 对应私钥 PEM ✓，供上报路径的测试用 ✓。
func makeAgentCert(t *testing.T, sans []string, days int) (certPEM, keyPEM string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: sans[0]},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Duration(days) * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	for _, s := range sans {
		if ip := net.ParseIP(s); ip != nil {
			tpl.IPAddresses = append(tpl.IPAddresses, ip)
		} else {
			tpl.DNSNames = append(tpl.DNSNames, s)
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	kb, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: kb}))
	return certPEM, keyPEM
}

func newAgentIssuedSvc(t *testing.T, dataDir string) *Service {
	t.Helper()
	db, err := gorm.Open(sqlitedrv.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&entity.Certificate{}); err != nil {
		t.Fatal(err)
	}
	return NewService(repository.NewCertificateRepository(db), dataDir, "", "")
}

// 核心口径 ✓：**完整数据入库**（库里存一份完整副本 ✓，下载/分发都从这一份出 ✓）+ **不写磁盘文件** ✗。
//
// 用户口径原话 ✓："证书是存数据库的，完整的数据，下载就是从库里出" ✓；
// "存数据库的目的就是**有人删除证书**[文件]了，库里还完整有一份" ✓。
func TestStoreAgentIssuedCertStoresFullDataInDB(t *testing.T) {
	dataDir := t.TempDir()
	svc := newAgentIssuedSvc(t, dataDir)
	certPEM, keyPEM := makeAgentCert(t, []string{"203.0.113.48"}, 6)

	got, err := svc.StoreAgentIssuedCert(context.Background(), AgentIssuedCertInput{
		Identifiers: []string{"203.0.113.48"},
		CertPEM:     certPEM,
		KeyPEM:      keyPEM,
		CA:          "lets_encrypt",
		ServerID:    19,
	})
	if err != nil {
		t.Fatalf("上报入库失败：%v", err)
	}
	if got == nil {
		t.Fatal("应返回记录 ✓")
	}
	// ① 元数据齐全 ✓
	if got.Domain != "203.0.113.48" || got.IssuerNodeID != 19 {
		t.Fatalf("元数据不对 ✓：domain=%q node=%d", got.Domain, got.IssuerNodeID)
	}
	if got.ExpiresAt.IsZero() || got.KeyType == "" {
		t.Fatalf("到期时间/密钥类型应解析出来 ✓：%+v", got)
	}
	// ② **完整数据入库** ✓（库里就是权威副本 ✓ → 下载直接从库里出 ✓）
	if got.CertContent != certPEM || got.KeyContent != keyPEM {
		t.Fatalf("库里应存**完整** PEM/私钥 ✓：cert=%d/%d key=%d/%d",
			len(got.CertContent), len(certPEM), len(got.KeyContent), len(keyPEM))
	}
	// ③ **不写磁盘文件** ✗（"不落地"指的是这个 ✓：磁盘副本只会多一处要同步的状态 ✗）
	if _, serr := os.Stat(filepath.Join(dataDir, "certs")); serr == nil {
		t.Fatalf("面板不该写 %s/certs ✗", dataDir)
	}
	// ④ 路径字段仍留作**节点侧引用** ✓
	if got.CertPath == "" || got.KeyPath == "" {
		t.Fatalf("节点侧引用路径应保留 ✓：%+v", got)
	}
}

// 续期/重报：库里的副本要**刷新成最新** ✓（这正是"库里有一份完整数据"的意义 ✓）
func TestStoreAgentIssuedCertRefreshesContent(t *testing.T) {
	dataDir := t.TempDir()
	svc := newAgentIssuedSvc(t, dataDir)
	ctx := context.Background()

	// 先塞一条"历史遗留"：面板持有 PEM ✓（模拟旧版本落地的存量 ✓）
	if err := svc.repo.Create(ctx, &entity.Certificate{
		Name:         "old.example.com",
		Domain:       "old.example.com",
		Issuer:       "lets_encrypt",
		ProviderType: "acme",
		CertPath:     "certs/old.example.com/cert.pem",
		KeyPath:      "certs/old.example.com/key.pem",
		CertContent:  "-----BEGIN CERTIFICATE-----\nOLD\n-----END CERTIFICATE-----\n",
		KeyContent:   "-----BEGIN PRIVATE KEY-----\nOLD\n-----END PRIVATE KEY-----\n",
		Status:       "active",
		AutoRenew:    1,
		ExpiresAt:    time.Now().Add(48 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	certPEM, keyPEM := makeAgentCert(t, []string{"old.example.com"}, 90)
	got, err := svc.StoreAgentIssuedCert(ctx, AgentIssuedCertInput{
		Identifiers: []string{"old.example.com"},
		CertPEM:     certPEM,
		KeyPEM:      keyPEM,
		CA:          "lets_encrypt",
		ServerID:    19,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.CertContent != certPEM || got.KeyContent != keyPEM {
		t.Fatalf("重报后库里的副本应是**最新**内容 ✓：cert=%d 期望 %d", len(got.CertContent), len(certPEM))
	}
	// 从库里再读一次确认是真落库了 ✓（不是内存对象 ✓）
	again, rerr := svc.repo.GetByID(ctx, got.ID)
	if rerr != nil {
		t.Fatal(rerr)
	}
	if again.CertContent != certPEM {
		t.Fatalf("库里没有刷新成最新内容 ✗：%d 期望 %d", len(again.CertContent), len(certPEM))
	}
	if again.CertContent == "-----BEGIN CERTIFICATE-----\nOLD\n-----END CERTIFICATE-----\n" {
		t.Fatalf("库里还是历史遗留的旧内容 ✗")
	}
}
