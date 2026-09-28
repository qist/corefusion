package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	sqlitedrv "github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	agentpb "corefusion/internal/runtime/agent/proto"
	certpkg "corefusion/internal/runtime/cert"
	"corefusion/internal/store/repository"
)

type fakeIssuer struct {
	got      []*agentpb.IssuePublicCertCommand
	which    []int64
	envEmail string
	envErr   bool
}

// 续期时面板要按 CA 给出邮箱与 EAB ✓（真实实现由 router 的 publicCertEnvAdapter 提供 ✓）
func (f *fakeIssuer) IssuanceContextFor(ctx context.Context, caProvider string) (string, string, string) {
	if f.envErr {
		return "", "", ""
	}
	return f.envEmail, "kid-" + caProvider, "mac-" + caProvider
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

// 核心回归 ✓：续期指令**必须**自带 CA / 邮箱 / EAB ✗ ——
// 不带 CA → 节点用默认 CA（LE）续签 ✗（签发者被悄悄换掉 ✗）；
// 不带邮箱 → Google 注册被拒 ✗；不带 EAB → Google/ZeroSSL 首次注册失败 ✗。
func TestRenewCommandCarriesCAEmailEAB(t *testing.T) {
	certRepo, serverRepo := setupRenewerDB(t)
	for _, c := range []struct {
		domains string
		issuer  string
		wantCA  string
	}{
		{`["a.example.com"]`, "google", "google"},
		{`["b.example.com"]`, "zerossl", "zerossl"},
		{`["c.example.com"]`, "lets_encrypt", "lets_encrypt"},
		{`["d.example.com"]`, "Let's Encrypt", "lets_encrypt"}, // 历史格式也要归一 ✓
	} {
		cert := seedCert(t, certRepo, c.domains, 1, 2*24*time.Hour)
		cert.Issuer = c.issuer
		if err := certRepo.Update(context.Background(), cert); err != nil {
			t.Fatal(err)
		}
		if err := serverRepo.Create(context.Background(), &entity.NodeServer{Name: "n-" + c.wantCA, CertificateID: cert.ID}); err != nil {
			t.Fatal(err)
		}
	}

	f := &fakeIssuer{envEmail: "test@example.com"}
	r := NewPublicCertRenewer(certRepo, serverRepo, f)
	if err := r.checkAndRenew(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.got) != 4 {
		t.Fatalf("应下发 4 条续期指令，实际 %d", len(f.got))
	}
	for _, cmd := range f.got {
		if cmd.GetCa() == "" {
			t.Fatalf("指令必须带 CA ✗（否则节点会用自己的默认 CA 续签 ✗）：%+v", cmd)
		}
		if cmd.GetEmail() != "test@example.com" {
			t.Fatalf("指令必须带邮箱 ✗（Google 注册要求 contact ✗）：%+v", cmd)
		}
		if cmd.GetEabKeyId() != "kid-"+cmd.GetCa() || cmd.GetEabMacKey() != "mac-"+cmd.GetCa() {
			t.Fatalf("指令必须带**按 CA 自动关联**的 EAB ✗：ca=%s kid=%s", cmd.GetCa(), cmd.GetEabKeyId())
		}
	}
	// 顺带验 CA 归一 ✓（Let's Encrypt → lets_encrypt ✓）
	seen := map[string]bool{}
	for _, cmd := range f.got {
		seen[cmd.GetCa()] = true
	}
	for _, want := range []string{"google", "zerossl", "lets_encrypt"} {
		if !seen[want] {
			t.Fatalf("缺少 ca=%s（归一化有问题 ✗）：%v", want, seen)
		}
	}
}

// 没有环境时的兜底：不 panic ✓、字段为空 ✓（宁可不带，也不能假造 ✗）
func TestRenewCommandWithoutEnv(t *testing.T) {
	certRepo, serverRepo := setupRenewerDB(t)
	c := seedCert(t, certRepo, `["e.example.com"]`, 1, 2*24*time.Hour)
	c.Issuer = "google"
	_ = certRepo.Update(context.Background(), c)
	_ = serverRepo.Create(context.Background(), &entity.NodeServer{Name: "n-nil", CertificateID: c.ID})

	f := &fakeIssuer{envErr: true}
	r := NewPublicCertRenewer(certRepo, serverRepo, f)
	if err := r.checkAndRenew(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.got) != 1 {
		t.Fatalf("仍应下发 1 条，实际 %d", len(f.got))
	}
	if f.got[0].GetCa() != "google" {
		t.Fatalf("CA 仍应带上（来自证书 issuer ✓）：%s", f.got[0].GetCa())
	}
	if f.got[0].GetEabKeyId() != "" {
		t.Fatalf("没有环境时不该假造 EAB ✗：%s", f.got[0].GetEabKeyId())
	}
}

// 核心回归 ✓：续期指令**必须带密钥类型** ✗ —— 不带的话节点侧
// `resolveKeyType("",0)` 会回退 **EC256** ✗ → 库里登记 `key_type=rsa/key_bits=4096`
// 的证书，续期后**变成 EC 证书** ✗（真机现象：记录 rsa、文件却是 `EC PRIVATE KEY` ✓）。
func TestRenewCommandCarriesKeyType(t *testing.T) {
	certRepo, serverRepo := setupRenewerDB(t)
	for _, c := range []struct {
		keyType string
		keyBits int
	}{
		{"rsa", 4096},
		{"ecc", 256},
	} {
		cert := seedCert(t, certRepo, `["`+c.keyType+`.example.com"]`, 1, 2*24*time.Hour)
		cert.Issuer = "lets_encrypt"
		// ⚠ 密钥规格以**库记录**为准 ✗（面板/表单选的 ✓）→ 必须原样下发 ✗
		cert.KeyType, cert.KeyBits = c.keyType, c.keyBits
		if err := certRepo.Update(context.Background(), cert); err != nil {
			t.Fatal(err)
		}
		if err := serverRepo.Create(context.Background(), &entity.NodeServer{
			Name: "n-" + c.keyType, CertificateID: cert.ID,
		}); err != nil {
			t.Fatal(err)
		}
	}

	f := &fakeIssuer{envEmail: "test@example.com"}
	r := NewPublicCertRenewer(certRepo, serverRepo, f)
	if err := r.checkAndRenew(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.got) != 2 {
		t.Fatalf("应下发 2 条续期指令，实际 %d", len(f.got))
	}
	got := map[string]int{}
	for _, cmd := range f.got {
		got[cmd.GetKeyType()] = int(cmd.GetKeyBits())
	}
	if got["rsa"] != 4096 {
		t.Fatalf("rsa 证书的续期指令必须带 key_type=rsa + key_bits=4096 ✗（否则节点会签出 EC ✗）：%v", got)
	}
	if got["ecc"] != 256 {
		t.Fatalf("ecc 证书的续期指令必须带 key_type=ecc + key_bits=256 ✗：%v", got)
	}
}

// fakeDNSResolver 按 id 返回 dns-01 凭据 ✓（真实实现是面板的 DNS 通道服务 ✓）
type fakeDNSResolver struct {
	asked []int64
	cfg   *certpkg.DNSProviderConfig
	err   error
}

func (f *fakeDNSResolver) ResolveDNSChannel(_ context.Context, id int64) (*certpkg.DNSProviderConfig, error) {
	f.asked = append(f.asked, id)
	if f.err != nil {
		return nil, f.err
	}
	return f.cfg, nil
}

// 核心回归 ✓：**创建时选什么挑战方式，续期就用什么** ✗ ——
// 这是用户的原始要求 ✓（"创建的时候选择什么就应该用什么续期" ✓）。
//
// ⚠ 以前这里**根本不带 Challenge** ✗ → 节点侧 `firstNonEmpty(cmd.Challenge,"http-01")` 一律 http-01 ✗
// → 创建时选的 dns-01 在自动续期里整片丢失 ✗（通配符证书根本续不出来 ✗）。
// 同时也必须把**面板那条通道的 id**带上 ✓：节点回传证书时要靠它让面板记住"创建时选了哪条通道" ✓
// （不带 → 面板落库 dns_channel_id=0 ✗ → 下一轮续期直接跳过 ✗）。
func TestRenewCommandCarriesChallengeAndDNSChannel(t *testing.T) {
	certRepo, serverRepo := setupRenewerDB(t)

	// ① dns-01 + 通道 7：指令必须带 challenge=dns-01 + 通道 id + **凭据** ✓
	dnsCert := seedCert(t, certRepo, `["dns.example.com"]`, 1, 2*24*time.Hour)
	dnsCert.Issuer = "lets_encrypt"
	dnsCert.ChallengeType, dnsCert.DNSChannelID = "dns-01", 7
	if err := certRepo.Update(context.Background(), dnsCert); err != nil {
		t.Fatal(err)
	}
	_ = serverRepo.Create(context.Background(), &entity.NodeServer{Name: "n-dns", CertificateID: dnsCert.ID})

	// ② http-01：不能因为"没有 dnsProvider"就被猜成别的 ✓
	httpCert := seedCert(t, certRepo, `["http.example.com"]`, 1, 2*24*time.Hour)
	httpCert.Issuer, httpCert.ChallengeType = "lets_encrypt", "http-01"
	if err := certRepo.Update(context.Background(), httpCert); err != nil {
		t.Fatal(err)
	}
	_ = serverRepo.Create(context.Background(), &entity.NodeServer{Name: "n-http", CertificateID: httpCert.ID})

	// ③ 历史数据（挑战方式上线前签的 ✓）：库里是空的 ✗ → 按通配符兜底 dns-01 ✓
	legacy := seedCert(t, certRepo, `["*.example.com","example.com"]`, 1, 2*24*time.Hour)
	legacy.Issuer, legacy.DNSChannelID = "lets_encrypt", 7
	if err := certRepo.Update(context.Background(), legacy); err != nil {
		t.Fatal(err)
	}
	_ = serverRepo.Create(context.Background(), &entity.NodeServer{Name: "n-legacy", CertificateID: legacy.ID})

	resolver := &fakeDNSResolver{cfg: &certpkg.DNSProviderConfig{
		Provider: "cloudflare",
		EnvVars:  map[string]string{"CF_DNS_API_TOKEN": "tok"},
	}}
	f := &fakeIssuer{envEmail: "test@example.com"}
	r := NewPublicCertRenewer(certRepo, serverRepo, f).WithDNSChannelResolver(resolver)
	if err := r.checkAndRenew(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.got) != 3 {
		t.Fatalf("应下发 3 条续期指令，实际 %d", len(f.got))
	}

	byCertID := map[int64]*agentpb.IssuePublicCertCommand{}
	for _, cmd := range f.got {
		byCertID[cmd.GetCertId()] = cmd
	}

	dnsCmd := byCertID[dnsCert.ID]
	if dnsCmd == nil {
		t.Fatal("dns-01 的证书没有下发续期指令 ✗")
	}
	if got := dnsCmd.GetChallenge(); got != "dns-01" {
		t.Fatalf("续期**必须沿用创建时选的挑战方式** ✗：期望 dns-01，实际 %q", got)
	}
	if got := dnsCmd.GetDnsChannelId(); got != 7 {
		t.Fatalf("必须带上面板通道 id ✗（否则节点回传后面板 dns_channel_id 仍是 0 ✗）：实际 %d", got)
	}
	if dnsCmd.GetDnsProvider() != "cloudflare" || dnsCmd.GetDnsEnvVars()["CF_DNS_API_TOKEN"] != "tok" {
		t.Fatalf("dns-01 必须把**创建时选的那条通道**的凭据一起下发 ✗：%+v", dnsCmd)
	}
	// 两张 dns-01 都按**同一个通道 id=7** 解析 ✓（http-01 那张一次都不该解析 ✓）
	if len(resolver.asked) != 2 {
		t.Fatalf("应解析 2 次（两张 dns-01 ✓），实际 %v", resolver.asked)
	}
	for _, id := range resolver.asked {
		if id != 7 {
			t.Fatalf("必须按库里的通道 id 解析 ✗（不能另取一个 ✓）：%v", resolver.asked)
		}
	}

	if got := byCertID[httpCert.ID].GetChallenge(); got != "http-01" {
		t.Fatalf("http-01 的证书应沿用 http-01 ✗：实际 %q", got)
	}
	if got := byCertID[legacy.ID].GetChallenge(); got != "dns-01" {
		t.Fatalf("历史数据（无记录 ✗）+ 通配符 → 必须兜底 dns-01 ✓（http-01 根本签不了通配符 ✗）：实际 %q", got)
	}
}

// 反例 ✓：库记录是 dns-01 但**通道丢了 / 解析不出来** → 必须**明确报错跳过** ✗，
// 绝不能"没有 dnsProvider 就退回 http-01" ✗（那等于偷偷把用户的 dns-01 改掉 ✗）。
func TestRenewDNSFailsWhenChannelUnavailable(t *testing.T) {
	for _, tc := range []struct {
		name      string
		channelID int64
		resolver  DNSChannelResolver
	}{
		{"库里没有通道 id", 0, &fakeDNSResolver{cfg: &certpkg.DNSProviderConfig{Provider: "cloudflare"}}},
		{"解析器没注入", 7, nil},
		{"解析失败", 7, &fakeDNSResolver{err: errors.New("通道不存在")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			certRepo, serverRepo := setupRenewerDB(t)
			c := seedCert(t, certRepo, `["*.dns.example.com","dns.example.com"]`, 1, 2*24*time.Hour)
			c.Issuer, c.ChallengeType, c.DNSChannelID = "lets_encrypt", "dns-01", tc.channelID
			if err := certRepo.Update(context.Background(), c); err != nil {
				t.Fatal(err)
			}
			_ = serverRepo.Create(context.Background(), &entity.NodeServer{Name: "n", CertificateID: c.ID})

			f := &fakeIssuer{envEmail: "test@example.com"}
			r := NewPublicCertRenewer(certRepo, serverRepo, f)
			if tc.resolver != nil {
				r = r.WithDNSChannelResolver(tc.resolver)
			}
			// 调度本身不该崩 ✓（单张失败只跳过 ✓，不能拖垮整轮 ✓）
			if err := r.checkAndRenew(context.Background()); err != nil {
				t.Fatal(err)
			}
			if len(f.got) != 0 {
				t.Fatalf("拿不到 dns-01 凭据时必须**跳过** ✗（不能降级成 http-01 ✗）：%+v", f.got[0])
			}
		})
	}
}

// normalizeCertCA 归一化 ✓
func TestNormalizeCertCA(t *testing.T) {
	cases := map[string]string{
		"google": "google", "Google": "google", "zerossl": "zerossl",
		"lets_encrypt": "lets_encrypt", "Let's Encrypt": "lets_encrypt",
		"Let's Encrypt Staging": "lets_encrypt_staging",
		"lets_encrypt_staging":  "lets_encrypt_staging",
		"":                      "", "weird-ca": "",
	}
	for in, want := range cases {
		if got := normalizeCertCA(in); got != want {
			t.Fatalf("normalizeCertCA(%q)=%q 期望 %q", in, got, want)
		}
	}
}

// 用户口径：谁签发谁续期 ✓ —— 一张证书即使被多个节点"使用"，也只问它的签发节点 ✓
func TestIssuerNodesFilter(t *testing.T) {
	all := []entity.NodeServer{{ID: 1, Name: "user-a"}, {ID: 19, Name: "issuer"}, {ID: 30, Name: "user-b"}}

	got := issuerNodes(all, 19)
	if len(got) != 1 || got[0].ID != 19 {
		t.Fatalf("指定签发节点时应只留它 ✓，实际 %+v", got)
	}
	// issuerNodeID=0（面板本机签的 ✓）→ 原样返回 ✓（行为不变 ✓）
	if got := issuerNodes(all, 0); len(got) != 3 {
		t.Fatalf("0 应原样返回 ✓，实际 %d", len(got))
	}
	// 签发节点已不在绑定列表（被删/解绑 ✓）→ 原样返回兜底 ✓
	if got := issuerNodes(all, 99); len(got) != 3 {
		t.Fatalf("找不到签发节点时应兜底返回全部 ✓，实际 %d", len(got))
	}
	if got := issuerNodes(nil, 19); len(got) != 0 {
		t.Fatalf("空列表应原样返回 ✓")
	}
}

// 端到端：证书带 IssuerNodeID 时，续期只下发到该节点 ✓（避免多个使用节点重复签发 ✗）
func TestRenewOnlyAsksIssuerNode(t *testing.T) {
	certRepo, serverRepo := setupRenewerDB(t)
	c := seedCert(t, certRepo, `["issuer-only.example.com"]`, 1, 2*24*time.Hour)
	// ⚠ 先建节点、拿到**真实自增 ID** ✓ —— 内存库里 ID 从 1 开始 ✗，
	// 不能拿"名字里的数字"当 ID 用 ✗（我第一版就这么写错、导致断言失败 ✓）。
	issuerNode := &entity.NodeServer{Name: "issuer-1.8", CertificateID: c.ID}
	if err := serverRepo.Create(context.Background(), issuerNode); err != nil {
		t.Fatal(err)
	}
	otherNode := &entity.NodeServer{Name: "user-other", CertificateID: c.ID}
	if err := serverRepo.Create(context.Background(), otherNode); err != nil {
		t.Fatal(err)
	}
	if issuerNode.ID == otherNode.ID {
		t.Fatalf("测试前置不成立：两个节点 ID 相同")
	}

	// 把"签发节点"设为其中之一 ✓
	c.IssuerNodeID = issuerNode.ID
	if err := certRepo.Update(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	// 先确认真的落库了 ✓
	if back, gerr := certRepo.GetByID(context.Background(), c.ID); gerr != nil || back.IssuerNodeID != issuerNode.ID {
		t.Fatalf("IssuerNodeID 未落库：%+v (%v)", back, gerr)
	}

	f := &fakeIssuer{envEmail: "a@b.com"}
	r := NewPublicCertRenewer(certRepo, serverRepo, f)
	if err := r.checkAndRenew(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.got) != 1 {
		t.Fatalf("只该下发 1 条（给签发节点 ✓），实际 %d", len(f.got))
	}
	if f.which[0] != c.IssuerNodeID {
		t.Fatalf("应下发给签发节点 %d，实际 %d", c.IssuerNodeID, f.which[0])
	}
}

// 兜底 ✓：签发节点已不在绑定列表（被删/被解绑 ✓）→ 仍然让绑定它的节点续 ✓（不会因为过滤而彻底不续 ✗）
func TestRenewFallsBackWhenIssuerMissing(t *testing.T) {
	certRepo, serverRepo := setupRenewerDB(t)
	c := seedCert(t, certRepo, `["issuer-gone.example.com"]`, 1, 2*24*time.Hour)
	if err := serverRepo.Create(context.Background(), &entity.NodeServer{Name: "only-user", CertificateID: c.ID}); err != nil {
		t.Fatal(err)
	}
	c.IssuerNodeID = 999999 // 不存在 ✓
	if err := certRepo.Update(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	f := &fakeIssuer{envEmail: "a@b.com"}
	r := NewPublicCertRenewer(certRepo, serverRepo, f)
	if err := r.checkAndRenew(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.got) != 1 {
		t.Fatalf("签发节点缺失时应兜底给绑定节点 ✓，实际下发 %d 条", len(f.got))
	}
}
