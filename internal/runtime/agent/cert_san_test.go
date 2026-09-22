package agent

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
)

// 用户："检查证书签发，特别是基于节点 IP 证书签发"（测试环境节点 IP 192.168.1.8，在防火墙后面）。
// 旧实现：节点证书 `DNSNames: [hostname]` + **`IPAddresses: []`（空）** ✗、面板服务证书只有
// 127.0.0.1/0.0.0.0 ✗ → 按 IP 校验证书必然失败。这里把 SAN 规则与真实签发都钉住。
func TestBuildCertSANs(t *testing.T) {
	// ① 普通主机名 + 节点 IP（user 的场景：hostname 是名字，IP 是 192.168.1.8）
	dns, ips := BuildCertSANs("node-12", "192.168.1.8", "192.168.2.50")
	if len(dns) != 1 || dns[0] != "node-12" {
		t.Errorf("DNS SAN 应为 [node-12]，实际 %v", dns)
	}
	want := map[string]bool{"192.168.1.8": true, "192.168.2.50": true, "127.0.0.1": true}
	if len(ips) != len(want) {
		t.Errorf("IP SAN 数量不对：%v", ips)
	}
	for _, ip := range ips {
		if !want[ip.String()] {
			t.Errorf("出现意外 IP SAN：%v", ip)
		}
	}

	// ② hostname 本身就是 IP（库里就有节点这么填）→ 必须是 **IP SAN**，不能当成 DNS 名 ✗
	dns, ips = BuildCertSANs("192.168.2.50")
	if len(dns) != 0 {
		t.Errorf("IP 形式的 hostname 不应进 DNS SAN：%v", dns)
	}
	found := false
	for _, ip := range ips {
		if ip.String() == "192.168.2.50" {
			found = true
		}
	}
	if !found {
		t.Errorf("IP 形式的 hostname 必须进 IP SAN：%v", ips)
	}

	// ③ 去重 / 跳过空值与非法值（非法值不得静默变成 DNS SAN ✗）
	dns, ips = BuildCertSANs("host", "192.168.1.8", "192.168.1.8", "", "  ", "not-an-ip", "127.0.0.1")
	if len(dns) != 1 || dns[0] != "host" {
		t.Errorf("DNS SAN 应只有 host：%v", dns)
	}
	if len(ips) != 2 { // 192.168.1.8 + 127.0.0.1（自动补）
		t.Errorf("去重/过滤后应只有 2 个 IP：%v", ips)
	}

	// ④ 空主机名也要能出（至少带 127.0.0.1）
	dns, ips = BuildCertSANs("")
	if len(dns) != 0 || len(ips) != 1 || ips[0].String() != "127.0.0.1" {
		t.Errorf("空主机名应只留 127.0.0.1：dns=%v ips=%v", dns, ips)
	}
}

// newTestCA 造一个测试用 CA（内存里，不碰面板真实 CA）。
func newTestCA(t *testing.T) (*CertificateAuthority, *x509.Certificate) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{Organization: []string{"CoreFusion"}, CommonName: "Test CA"},
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
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, _ := x509.MarshalECPrivateKey(key)
	ca := &CertificateAuthority{caDir: t.TempDir()}
	ca.entries = []caEntry{{
		cert:    cert,
		key:     key,
		certPEM: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		keyPEM:  pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}),
	}}
	return ca, cert
}

func parseFirstCert(t *testing.T, pemBytes []byte) *x509.Certificate {
	t.Helper()
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		t.Fatal("证书 PEM 解析失败")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	return cert
}

// 用户的实际场景：节点 **内网 IP = 192.168.1.8**（在防火墙后面），那台服务器**另有出口 IP**。
// 证书必须**两个 IP 都带上** ✓ —— 否则无论面板/客户端用哪个地址连它，总有一边校验失败 ✗。
// 签发后用 x509 做真实校验：
//   - 内网 IP 与出口 IP 都必须**能通过**；
//   - 没放进 SAN 的 IP 必须**失败**（否则 SAN 形同虚设 ✗）。
func TestIssueAgentCertificateForNodeIP(t *testing.T) {
	ca, caCert := newTestCA(t)

	// 内网 192.168.1.8 + 该机出口 IP 203.0.113.10（node_servers.public_ip）
	res, err := ca.IssueAgentCertificate(context.Background(), 12, "node-12", "192.168.1.8", "203.0.113.10")
	if err != nil {
		t.Fatalf("签发失败：%v", err)
	}
	cert := parseFirstCert(t, res.CertPEM)

	hasIP := func(want string) bool {
		for _, ip := range cert.IPAddresses {
			if ip.String() == want {
				return true
			}
		}
		return false
	}
	for _, want := range []string{"192.168.1.8", "203.0.113.10", "127.0.0.1"} {
		if !hasIP(want) {
			t.Errorf("IP SAN 缺少 %s（节点 IP 签发就是靠它）：%v", want, cert.IPAddresses)
		}
	}
	if len(cert.DNSNames) != 1 || cert.DNSNames[0] != "node-12" {
		t.Errorf("DNS SAN 应为 [node-12]：%v", cert.DNSNames)
	}
	// 身份绑定（审计 M3）：OU 必须是 server-<id>，面板据此拒绝冒充
	if len(cert.Subject.OrganizationalUnit) != 1 || cert.Subject.OrganizationalUnit[0] != "server-12" {
		t.Errorf("OU 应为 server-12：%v", cert.Subject.OrganizationalUnit)
	}

	pool := x509.NewCertPool()
	pool.AddCert(caCert)

	// ① 用 192.168.1.8 校验 → 通过
	if _, err := cert.Verify(x509.VerifyOptions{Roots: pool, DNSName: "192.168.1.8"}); err != nil {
		t.Errorf("用节点 IP 192.168.1.8 校验证书应通过：%v", err)
	}
	// ② 用该机的**出口 IP** 校验 → 也必须通过（服务器有出口 IP，客户端可能按它连）
	if _, err := cert.Verify(x509.VerifyOptions{Roots: pool, DNSName: "203.0.113.10"}); err != nil {
		t.Errorf("用出口 IP 203.0.113.10 校验应通过：%v", err)
	}
	// ③ 用一个**没有**放进 SAN 的 IP 校验 → 必须失败（否则等于没校验 ✗）
	if _, err := cert.Verify(x509.VerifyOptions{Roots: pool, DNSName: "192.168.1.9"}); err == nil {
		t.Error("未放入 SAN 的 IP 必须校验失败（否则 SAN 形同虚设）")
	}
	// ④ 用 hostname 校验 → 通过
	if _, err := cert.Verify(x509.VerifyOptions{Roots: pool, DNSName: "node-12"}); err != nil {
		t.Errorf("用 hostname 校验应通过：%v", err)
	}
}

// 面板自身的服务证书要带**本机 IP**（否则按局域网 IP 访问面板时校验失败）。
func TestIssueServerCertificateIncludesLocalIPs(t *testing.T) {
	ca, _ := newTestCA(t) // 注意：这里只验证 SAN 形状，不需要真实 CA 校验
	cert, err := ca.IssueServerCertificate()
	if err != nil {
		t.Fatalf("签发面板服务证书失败：%v", err)
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	has := func(want string) bool {
		for _, ip := range leaf.IPAddresses {
			if ip.String() == want {
				return true
			}
		}
		return false
	}
	if !has("127.0.0.1") {
		t.Error("服务证书必须含 127.0.0.1")
	}
	// 本机至少有一个非回环 IP（容器/局域网），必须出现在 SAN 里
	local := localIPStrings()
	if len(local) > 0 && !has(local[0]) {
		t.Errorf("服务证书应含本机 IP %s：%v", local[0], leaf.IPAddresses)
	}
	t.Logf("服务证书 IP SAN：%v / DNS SAN：%v / 本机 IP：%v", leaf.IPAddresses, leaf.DNSNames, local)
}

// 用**面板真实 CA**（磁盘上那份）给节点 IP 192.168.1.8 签一枚证书，并校验它。
//
// 默认跳过（不碰真实数据）—— 要跑就指定数据目录：
//
//	CF_TEST_DATA_DIR=/opt/corefusion/data go test ./internal/runtime/agent/ -run RealCA -v
func TestIssueAgentCertificateWithRealCA(t *testing.T) {
	dataDir := os.Getenv("CF_TEST_DATA_DIR")
	if dataDir == "" {
		t.Skip("未设置 CF_TEST_DATA_DIR，跳过真实 CA 签发测试")
	}
	caDir := filepath.Join(dataDir, "certs", "mtls")
	certPEM, err := os.ReadFile(filepath.Join(caDir, "ca-cert.pem"))
	if err != nil {
		t.Skipf("读不到面板 CA（%v），跳过", err)
	}
	keyPEM, err := os.ReadFile(filepath.Join(caDir, "ca-key.pem"))
	if err != nil {
		t.Skipf("读不到面板 CA 私钥（%v），跳过", err)
	}
	// ca-cert.pem 是 bundle（可能多个 CA）→ 取最后一个（最新的 incoming CA）
	var caCert *x509.Certificate
	var caCertDER []byte
	rest := certPEM
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if c, perr := x509.ParseCertificate(block.Bytes); perr == nil {
			caCert, caCertDER = c, block.Bytes
		}
	}
	if caCert == nil {
		t.Skip("CA bundle 里没有可用证书，跳过")
	}
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		t.Skip("CA 私钥格式不支持，跳过")
	}
	caKey, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		t.Skipf("CA 私钥解析失败（%v），跳过", err)
	}

	ca := &CertificateAuthority{caDir: caDir}
	ca.entries = []caEntry{{cert: caCert, key: caKey, certPEM: caCertDER, keyPEM: keyPEM}}

	// 用户场景：内网 192.168.1.8 + 该机出口 IP（这里用文档保留地址示例）
	res, err := ca.IssueAgentCertificate(context.Background(), 9999, "node-192-168-1-8", "192.168.1.8", "203.0.113.10")
	if err != nil {
		t.Fatalf("用真实 CA 签发失败：%v", err)
	}
	leaf := parseFirstCert(t, res.CertPEM)
	t.Logf("真实 CA 签发结果：subject=%s", leaf.Subject)
	t.Logf("  DNS SAN: %v", leaf.DNSNames)
	t.Logf("  IP  SAN: %v", leaf.IPAddresses)
	t.Logf("  有效期: %v → %v", leaf.NotBefore, leaf.NotAfter)

	for _, want := range []string{"192.168.1.8", "203.0.113.10"} {
		ok := false
		for _, ip := range leaf.IPAddresses {
			if ip.String() == want {
				ok = true
			}
		}
		if !ok {
			t.Fatalf("真实 CA 签发的证书缺少 IP SAN %s ✗：%v", want, leaf.IPAddresses)
		}
	}
	pool := x509.NewCertPool()
	pool.AddCert(caCert)
	if _, err := leaf.Verify(x509.VerifyOptions{Roots: pool, DNSName: "192.168.1.8"}); err != nil {
		t.Fatalf("真实 CA 签发的证书用 192.168.1.8 校验失败：%v", err)
	}
	if _, err := leaf.Verify(x509.VerifyOptions{Roots: pool, DNSName: net.IPv4(192, 168, 1, 9).String()}); err == nil {
		t.Fatal("未在 SAN 里的 IP 竟然校验通过 ✗")
	}
	t.Log("✓ 真实 CA 签发的节点 IP 证书校验通过（192.168.1.8），未列入的 IP 校验失败 ✓")
}
