package agent

import (
	"context"
	"testing"
	"time"

	sqlitedrv "github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	agentpb "corefusion/internal/runtime/agent/proto"
	"corefusion/internal/store/repository"
)

type fakeIssuer struct {
	got   []*agentpb.IssuePublicCertCommand
	which []int64
}

func (f *fakeIssuer) IssuePublicCert(ctx context.Context, serverID int64, cmd *agentpb.IssuePublicCertCommand) error {
	f.got = append(f.got, cmd)
	f.which = append(f.which, serverID)
	return nil
}

func setupRenewerDB(t *testing.T) (*repository.CertificateRepository, *repository.NodeServerRepository) {
	t.Helper()
	db, err := gorm.Open(sqlitedrv.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&entity.Certificate{}, &entity.NodeServer{}, &entity.NodeCertificate{}); err != nil {
		t.Fatal(err)
	}
	return repository.NewCertificateRepository(db), repository.NewNodeServerRepository(db)
}

// setupNodeCertRepo 单独建一个带多对多仓库的环境 ✓。
func setupNodeCertRepo(t *testing.T) (*repository.CertificateRepository, *repository.NodeServerRepository, *repository.NodeCertificateRepository) {
	t.Helper()
	db, err := gorm.Open(sqlitedrv.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&entity.Certificate{}, &entity.NodeServer{}, &entity.NodeCertificate{}); err != nil {
		t.Fatal(err)
	}
	return repository.NewCertificateRepository(db),
		repository.NewNodeServerRepository(db),
		repository.NewNodeCertificateRepository(db)
}

func seedCert(t *testing.T, repo *repository.CertificateRepository, domains string, autoRenew int, expiresIn time.Duration) *entity.Certificate {
	t.Helper()
	c := &entity.Certificate{
		Name: "c", Domain: "203.0.113.48", Domains: domains,
		Issuer: "Let's Encrypt", ProviderType: "acme", KeyType: "ecc", KeyBits: 256,
		CertPath: "certs/c/cert.pem", KeyPath: "certs/c/key.pem",
		Status: "active", AutoRenew: autoRenew, ExpiresAt: time.Now().Add(expiresIn),
	}
	if err := repo.Create(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	return c
}

// 核心：快到期 + 开了自动续期 + 绑定了节点 → 必须下发"节点本机签发" ✓
func TestPublicCertRenewerAsksBoundNode(t *testing.T) {
	certRepo, serverRepo := setupRenewerDB(t)
	c := seedCert(t, certRepo, `["203.0.113.48"]`, 1, 2*24*time.Hour)
	if err := serverRepo.Create(context.Background(), &entity.NodeServer{Name: "1.8", CertificateID: c.ID}); err != nil {
		t.Fatal(err)
	}

	f := &fakeIssuer{}
	r := NewPublicCertRenewer(certRepo, serverRepo, f)
	if err := r.checkAndRenew(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.got) != 1 {
		t.Fatalf("期望下发 1 次签发指令，实际 %d 次", len(f.got))
	}
	if got := f.got[0].GetIdentifiers(); len(got) != 1 || got[0] != "203.0.113.48" {
		t.Fatalf("标识符错误: %v", got)
	}
	if f.got[0].GetCertId() != c.ID {
		t.Fatalf("cert_id 未带上: %d", f.got[0].GetCertId())
	}
}

func TestPublicCertRenewerSkips(t *testing.T) {
	// ① 还很远才到期 → 不下发
	certRepo, serverRepo := setupRenewerDB(t)
	c1 := seedCert(t, certRepo, `["a.com"]`, 1, 60*24*time.Hour)
	_ = serverRepo.Create(context.Background(), &entity.NodeServer{Name: "n1", CertificateID: c1.ID})

	// ② 关了自动续期 → 不下发
	c2 := seedCert(t, certRepo, `["b.com"]`, 0, time.Hour)
	_ = serverRepo.Create(context.Background(), &entity.NodeServer{Name: "n2", CertificateID: c2.ID})

	// ③ 快到期但没绑定节点 → 不下发（例如面板本机 dns-01 签的 ✓）
	seedCert(t, certRepo, `["c.com"]`, 1, time.Hour)

	f := &fakeIssuer{}
	r := NewPublicCertRenewer(certRepo, serverRepo, f)
	if err := r.checkAndRenew(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.got) != 0 {
		for i, g := range f.got {
			t.Logf("  意外下发: server=%d cert_id=%d identifiers=%v", f.which[i], g.GetCertId(), g.GetIdentifiers())
		}
		t.Fatalf("这三种情况都不该下发，实际 %d 次", len(f.got))
	}
}

func TestCertIdentifiers(t *testing.T) {
	cases := []struct {
		domains, domain string
		want            []string
	}{
		{`["203.0.113.48"]`, "203.0.113.48", []string{"203.0.113.48"}},
		{`["a.com","b.com"]`, "a.com", []string{"a.com", "b.com"}},
		{"", "only.com", []string{"only.com"}},
		{`[""]`, "", nil},
		{`not-json`, "d.com", []string{"d.com"}},
	}
	for _, c := range cases {
		got := certIdentifiers(c.domains, c.domain)
		if len(got) != len(c.want) {
			t.Fatalf("certIdentifiers(%q,%q)=%v 期望 %v", c.domains, c.domain, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("certIdentifiers(%q,%q)=%v 期望 %v", c.domains, c.domain, got, c.want)
			}
		}
	}
}

// 回归：显式 0 必须真的存成 0 ✓（GORM 曾因 default:1 把零值吞掉 ✗）
func TestCertAutoRenewZeroIsPersisted(t *testing.T) {
	certRepo, _ := setupRenewerDB(t)
	c := seedCert(t, certRepo, `["z.com"]`, 0, time.Hour)
	back, err := certRepo.GetByID(context.Background(), c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if back.AutoRenew != 0 {
		t.Fatalf("auto_renew 期望 0（显式关闭），实际 %d ✗", back.AutoRenew)
	}
	// 反向：1 仍然是 1 ✓
	c2 := seedCert(t, certRepo, `["y.com"]`, 1, time.Hour)
	back2, _ := certRepo.GetByID(context.Background(), c2.ID)
	if back2.AutoRenew != 1 {
		t.Fatalf("auto_renew 期望 1，实际 %d", back2.AutoRenew)
	}
}

// 阈值口径（用户要求）：IP 证书剩 3 天开始续；域名证书剩 20 天开始续 ✓
func TestRenewThresholdByType(t *testing.T) {
	if got := renewThresholdFor([]string{"203.0.113.48"}); got != PublicCertIPRenewBefore {
		t.Fatalf("IP 证书阈值 = %v，期望 %v", got, PublicCertIPRenewBefore)
	}
	if got := renewThresholdFor([]string{"test.example.com"}); got != PublicCertDomainRenewBefore {
		t.Fatalf("域名证书阈值 = %v，期望 %v", got, PublicCertDomainRenewBefore)
	}
	// 混合（域名 + IP）按更紧的短时证书处理 ✓
	if got := renewThresholdFor([]string{"test.example.com", "203.0.113.48"}); got != PublicCertIPRenewBefore {
		t.Fatalf("混合阈值 = %v，期望按 IP 的 %v", got, PublicCertIPRenewBefore)
	}
}

func TestRenewThresholdAppliesPerCertType(t *testing.T) {
	certRepo, serverRepo := setupRenewerDB(t)

	// IP：剩 10 天 → 3 天阈值 → 还早，不续 ✓
	ipFar := seedCert(t, certRepo, `["203.0.113.49"]`, 1, 10*24*time.Hour)
	_ = serverRepo.Create(context.Background(), &entity.NodeServer{Name: "s-ip-far", CertificateID: ipFar.ID})
	// IP：剩 2 天 → 该续 ✓
	ipNear := seedCert(t, certRepo, `["203.0.113.50"]`, 1, 2*24*time.Hour)
	_ = serverRepo.Create(context.Background(), &entity.NodeServer{Name: "s-ip-near", CertificateID: ipNear.ID})
	// 域名：剩 10 天 → 20 天阈值 → 该续 ✓
	domNear := seedCert(t, certRepo, `["near.example.com"]`, 1, 10*24*time.Hour)
	_ = serverRepo.Create(context.Background(), &entity.NodeServer{Name: "s-dom-near", CertificateID: domNear.ID})
	// 域名：剩 30 天 → 还早，不续 ✓
	domFar := seedCert(t, certRepo, `["far.example.com"]`, 1, 30*24*time.Hour)
	_ = serverRepo.Create(context.Background(), &entity.NodeServer{Name: "s-dom-far", CertificateID: domFar.ID})

	f := &fakeIssuer{}
	r := NewPublicCertRenewer(certRepo, serverRepo, f)
	if err := r.checkAndRenew(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := map[int64]bool{}
	for i, c := range f.got {
		got[c.GetCertId()] = true
		_ = i
	}
	if !got[ipNear.ID] {
		t.Fatalf("IP 证书剩 2 天必须续（阈值 3 天）")
	}
	if got[ipFar.ID] {
		t.Fatalf("IP 证书剩 10 天不该续（阈值 3 天）")
	}
	if !got[domNear.ID] {
		t.Fatalf("域名证书剩 10 天必须续（阈值 20 天）")
	}
	if got[domFar.ID] {
		t.Fatalf("域名证书剩 30 天不该续（阈值 20 天）")
	}
}

// 一个节点绑定**两张**证书 → 续期时必须两张都下发 ✓（这正是"支持多证书"的核心 ✓）。
func TestPublicCertRenewerRenewsAllBoundCerts(t *testing.T) {
	certRepo, serverRepo, nodeCertRepo := setupNodeCertRepo(t)

	// 一张 IP 证书（3 天阈值）+ 一张域名证书（20 天阈值）✓，都进入续期窗口 ✓
	ipCert := seedCert(t, certRepo, `["203.0.113.48"]`, 1, 2*24*time.Hour)
	domCert := seedCert(t, certRepo, `["multi.example.com"]`, 1, 10*24*time.Hour)

	srv := &entity.NodeServer{Name: "n-multi"}
	if err := serverRepo.Create(context.Background(), srv); err != nil {
		t.Fatal(err)
	}
	// **两张都绑到同一个节点** ✓
	for _, id := range []int64{ipCert.ID, domCert.ID} {
		if err := nodeCertRepo.Associate(context.Background(), srv.ID, id); err != nil {
			t.Fatal(err)
		}
	}
	// 主证书字段仍只放一张 ✓（兼容既有逻辑 ✓）
	if err := serverRepo.UpdateCertificateID(context.Background(), srv.ID, ipCert.ID); err != nil {
		t.Fatal(err)
	}

	f := &fakeIssuer{}
	r := NewPublicCertRenewer(certRepo, serverRepo, f).WithNodeCertificates(nodeCertRepo)
	if err := r.checkAndRenew(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := map[int64]bool{}
	for _, c := range f.got {
		got[c.GetCertId()] = true
	}
	if !got[ipCert.ID] || !got[domCert.ID] {
		t.Fatalf("两张绑定证书都该被续 ✓；实际下发 %v（IP=%d 域名=%d）", got, ipCert.ID, domCert.ID)
	}
}

// 多对多仓库：绑定幂等 ✓、精确解除 ✓、反向查询 ✓
func TestNodeCertificateRepositoryBehavior(t *testing.T) {
	_, serverRepo, nodeCertRepo := setupNodeCertRepo(t)
	ctx := context.Background()

	srv := &entity.NodeServer{Name: "n1"}
	if err := serverRepo.Create(ctx, srv); err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{3, 4, 5} {
		if err := nodeCertRepo.Associate(ctx, srv.ID, id); err != nil {
			t.Fatal(err)
		}
	}
	// 幂等 ✓
	if err := nodeCertRepo.Associate(ctx, srv.ID, 4); err != nil {
		t.Fatal(err)
	}
	ids, err := nodeCertRepo.ListCertificateIDsByNode(ctx, srv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 3 {
		t.Fatalf("应绑定 3 张（去重后），实际 %v", ids)
	}
	// 反向：这张证书被谁绑着 ✓
	nodes, err := nodeCertRepo.ListNodeIDsByCertificate(ctx, 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0] != srv.ID {
		t.Fatalf("证书 4 应绑定到节点 %d，实际 %v", srv.ID, nodes)
	}
	// 精确解除 ✓（只解这一张 ✓）
	if err := nodeCertRepo.Dissociate(ctx, srv.ID, 4); err != nil {
		t.Fatal(err)
	}
	ids, _ = nodeCertRepo.ListCertificateIDsByNode(ctx, srv.ID)
	if len(ids) != 2 {
		t.Fatalf("解除一张后应剩 2 张，实际 %v", ids)
	}
	if nodes, _ = nodeCertRepo.ListNodeIDsByCertificate(ctx, 4); len(nodes) != 0 {
		t.Fatalf("证书 4 应已无绑定，实际 %v", nodes)
	}
}

// 历史单值迁移 ✓：node_servers.certificate_id → 多对多表，且幂等 ✓
func TestMigrateLegacyCertificateID(t *testing.T) {
	_, serverRepo, nodeCertRepo := setupNodeCertRepo(t)
	ctx := context.Background()

	srv := &entity.NodeServer{Name: "legacy"}
	if err := serverRepo.Create(ctx, srv); err != nil {
		t.Fatal(err)
	}
	if err := serverRepo.UpdateCertificateID(ctx, srv.ID, 3); err != nil {
		t.Fatal(err)
	}
	n, err := nodeCertRepo.MigrateLegacyCertificateID(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("应迁移 1 条，实际 %d", n)
	}
	// 再跑一次不应重复 ✓
	if n, err = nodeCertRepo.MigrateLegacyCertificateID(ctx); err != nil || n != 0 {
		t.Fatalf("迁移必须幂等：n=%d err=%v", n, err)
	}
	ids, _ := nodeCertRepo.ListCertificateIDsByNode(ctx, srv.ID)
	if len(ids) != 1 || ids[0] != 3 {
		t.Fatalf("迁移结果不对：%v", ids)
	}
}
