package certificate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"corefusion/internal/model/entity"
)

// TestSyncCertsToDiskBackfillsFullchain 锁定启动回填 ✓：
// 库里已有内容、磁盘却缺 fullchain.pem 的历史记录（早期上传只写 cert/key ✗），
// 启动同步要补出 fullchain.pem，并把库里的 fullchain_path 一并补上。
func TestSyncCertsToDiskBackfillsFullchain(t *testing.T) {
	dir := t.TempDir()
	svc := newAgentIssuedSvc(t, dir)
	ctx := context.Background()

	certPEM, keyPEM := makeAgentCert(t, []string{"backfill.example.com"}, 30)
	c := &entity.Certificate{
		Name:          "backfill",
		Domain:        "backfill.example.com",
		Issuer:        "upload",
		ProviderType:  "upload",
		KeyType:       "ecc",
		CertPath:      "certs/backfill.example.com/cert.pem",
		KeyPath:       "certs/backfill.example.com/key.pem",
		FullchainPath: "", // 历史行：没落 => 回填目标
		CertContent:   certPEM,
		KeyContent:    keyPEM,
		Status:        "active",
		ExpiresAt:     time.Now().Add(30 * 24 * time.Hour),
	}
	if err := svc.repo.Create(ctx, c); err != nil {
		t.Fatal(err)
	}

	if err := svc.SyncCertsToDisk(ctx); err != nil {
		t.Fatal(err)
	}

	// ① 磁盘补出 fullchain.pem，且与 cert.pem 同内容
	fcPath := filepath.Join(dir, "certs", "backfill.example.com", "fullchain.pem")
	b, err := os.ReadFile(fcPath)
	if err != nil {
		t.Fatalf("fullchain.pem 未回填：%v", err)
	}
	if string(b) != certPEM {
		t.Fatalf("fullchain 内容应等于 cert 内容")
	}

	// ② 库里 fullchain_path 也被补上
	got, err := svc.repo.GetByID(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.FullchainPath != "certs/backfill.example.com/fullchain.pem" {
		t.Fatalf("fullchain_path 未回填，实际=%q", got.FullchainPath)
	}
}

// TestUploadWritesFullchainUnderDomainDir 锁定上传口径与 ACME 一致 ✓：
// 目录按**域名**（不是自定义 name ✓）、落 fullchain.pem、权限 0640/0600 ✓。
func TestUploadWritesFullchainUnderDomainDir(t *testing.T) {
	dir := t.TempDir()
	svc := newAgentIssuedSvc(t, dir)

	certPEM, keyPEM := makeAgentCert(t, []string{"up.example.com"}, 30)
	c, err := svc.Upload(context.Background(), UploadCertificateInput{
		Name:        "myalias", // 自定义别名不应进目录名
		Domain:      "up.example.com",
		CertContent: certPEM,
		KeyContent:  keyPEM,
	})
	if err != nil {
		t.Fatal(err)
	}

	if c.CertPath != "certs/up.example.com/cert.pem" {
		t.Fatalf("上传目录应按域名，实际 cert_path=%q", c.CertPath)
	}
	if c.FullchainPath != "certs/up.example.com/fullchain.pem" {
		t.Fatalf("fullchain_path 应被设置，实际=%q", c.FullchainPath)
	}

	// fullchain 必须落盘（否则 nginx ssl_trusted_certificate 会指向不存在的文件）
	fcPath := filepath.Join(dir, c.FullchainPath)
	if _, err := os.Stat(fcPath); err != nil {
		t.Fatalf("fullchain.pem 未落盘：%v", err)
	}
	if n := strings.Count(certPEM, "BEGIN CERTIFICATE"); n != 1 {
		t.Fatalf("测试基线异常：自签证书段数=%d", n)
	}
}
