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

// 核心回归 ✓：节点回传"我实际是怎么签的" → **必须落库** ✗ ——
// 这就是"创建的时候选择什么就应该用什么续期"里**创建时的那次记录** ✓。
//
// ⚠ 以前这两个字段**根本不存** ✗ → 节点本机签出来的证书在面板里"不知道自己是怎么签的" ✗ →
// 自动续期只能按"含通配符 → dns-01 / 否则 http-01"猜 ✗，
// 而 dns-01 的证书还会因为 `dns_channel_id=0` 而**直接跳过续期** ✗（真机现象 ✓）。
func TestStoreAgentIssuedCertPersistsChallengeAndDNSChannel(t *testing.T) {
	dataDir := t.TempDir()
	svc := newAgentIssuedSvc(t, dataDir)
	ctx := context.Background()
	certPEM, keyPEM := makeAgentCert(t, []string{"*.example.com", "example.com"}, 90)

	got, err := svc.StoreAgentIssuedCert(ctx, AgentIssuedCertInput{
		Identifiers:   []string{"*.example.com", "example.com"},
		CertPEM:       certPEM,
		KeyPEM:        keyPEM,
		CA:            "lets_encrypt",
		ServerID:      19,
		ChallengeType: "dns-01",
		DNSChannelID:  7,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ChallengeType != "dns-01" || got.DNSChannelID != 7 {
		t.Fatalf("节点回传的挑战方式/通道必须落库 ✗（否则续期无从还原 ✗）：challenge=%q channel=%d",
			got.ChallengeType, got.DNSChannelID)
	}
	// 真落库 ✓（不是内存对象 ✓）
	again, rerr := svc.repo.GetByID(ctx, got.ID)
	if rerr != nil {
		t.Fatal(rerr)
	}
	if again.ChallengeType != "dns-01" || again.DNSChannelID != 7 {
		t.Fatalf("库里没存下来 ✗：challenge=%q channel=%d", again.ChallengeType, again.DNSChannelID)
	}
}

// ⚠ 老 Agent 不带这两个字段 ✗ → **必须保持空** ✗，绝不能归一成 `http-01` ✗：
// 通配符证书被记成 http-01 之后，续期会按 http-01 去签 ✗（HTTP-01 根本签不了通配符 ✗ → 必失败 ✗）。
func TestStoreAgentIssuedCertKeepsEmptyChallengeEmpty(t *testing.T) {
	svc := newAgentIssuedSvc(t, t.TempDir())
	certPEM, keyPEM := makeAgentCert(t, []string{"*.example.com", "example.com"}, 90)

	got, err := svc.StoreAgentIssuedCert(context.Background(), AgentIssuedCertInput{
		Identifiers: []string{"*.example.com", "example.com"},
		CertPEM:     certPEM,
		KeyPEM:      keyPEM,
		CA:          "lets_encrypt",
		ServerID:    19,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ChallengeType != "" || got.DNSChannelID != 0 {
		t.Fatalf("老 Agent 回传为空时必须留空 ✗（留空 = 交给续期侧按通配符兜底 ✓）：challenge=%q channel=%d",
			got.ChallengeType, got.DNSChannelID)
	}
}

// 重报/续期回传：**只补空、不覆盖** ✓ ——
// 库里已有的是"用户创建时的选择" ✓（权威 ✓），节点回传不能把它改掉 ✗。
func TestStoreAgentIssuedCertDoesNotOverwriteStoredChoice(t *testing.T) {
	svc := newAgentIssuedSvc(t, t.TempDir())
	ctx := context.Background()

	// ① 库里已有 dns-01 + 通道 7（创建时选的 ✓）；节点却回传 http-01 / 0 → 库里必须不变 ✓
	keep := &entity.Certificate{
		Name: "keep.example.com", Domain: "keep.example.com", Domains: `["keep.example.com"]`,
		Issuer: "lets_encrypt", ProviderType: "acme",
		ChallengeType: "dns-01", DNSChannelID: 7,
		CertPath: "certs/keep.example.com/cert.pem", KeyPath: "certs/keep.example.com/key.pem",
		Status: "active", AutoRenew: 1, ExpiresAt: time.Now().Add(48 * time.Hour),
	}
	if err := svc.repo.Create(ctx, keep); err != nil {
		t.Fatal(err)
	}
	certPEM, keyPEM := makeAgentCert(t, []string{"keep.example.com"}, 90)
	got, err := svc.StoreAgentIssuedCert(ctx, AgentIssuedCertInput{
		Identifiers: []string{"keep.example.com"}, CertPEM: certPEM, KeyPEM: keyPEM,
		CA: "lets_encrypt", ServerID: 19,
		ChallengeType: "http-01", DNSChannelID: 0, // 节点说的与库里不一致 ✗
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ChallengeType != "dns-01" || got.DNSChannelID != 7 {
		t.Fatalf("库里的选择是权威 ✓，不能被回传覆盖 ✗：challenge=%q channel=%d",
			got.ChallengeType, got.DNSChannelID)
	}

	// ② 库里是空的（历史数据 ✗）、节点回传 dns-01 + 7 → **补上** ✓（下次续期就有依据了 ✓）
	fill := &entity.Certificate{
		Name: "fill.example.com", Domain: "fill.example.com", Domains: `["fill.example.com"]`,
		Issuer: "lets_encrypt", ProviderType: "acme",
		CertPath: "certs/fill.example.com/cert.pem", KeyPath: "certs/fill.example.com/key.pem",
		Status: "active", AutoRenew: 1, ExpiresAt: time.Now().Add(48 * time.Hour),
	}
	if err := svc.repo.Create(ctx, fill); err != nil {
		t.Fatal(err)
	}
	certPEM2, keyPEM2 := makeAgentCert(t, []string{"fill.example.com"}, 90)
	gotFill, err := svc.StoreAgentIssuedCert(ctx, AgentIssuedCertInput{
		Identifiers: []string{"fill.example.com"}, CertPEM: certPEM2, KeyPEM: keyPEM2,
		CA: "lets_encrypt", ServerID: 19,
		ChallengeType: "dns-01", DNSChannelID: 7,
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotFill.ChallengeType != "dns-01" || gotFill.DNSChannelID != 7 {
		t.Fatalf("空记录应被回传值补上 ✓：challenge=%q channel=%d", gotFill.ChallengeType, gotFill.DNSChannelID)
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
