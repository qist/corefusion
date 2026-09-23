package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"corefusion/internal/runtime/cert"
	certsvc "corefusion/internal/service/certificate"
)

// acmeIssueParams 是"在节点本机签一张公网证书"的全部输入 ✓。
//
// CLI（corefusion-agent cert issue）与 Agent 的线上指令（面板下发 IssuePublicCert）共用同一套逻辑 ✓，
// 避免两条路径的行为漂移 ✗。
type acmeIssueParams struct {
	Identifiers []string          // 域名或 **IP 字面量** ✓
	Email       string            // ACME 账号邮箱
	Challenge   string            // http-01（默认）| dns-01
	CA          string            // lets_encrypt | lets_encrypt_staging | ...
	Profile     string            // 空 = 自动（含 IP → shortlived ✓）
	Webroot     string            // http-01 且 80 被 nginx/caddy 占用时：挑战写进该站点根目录 ✓
	DataDir     string            // ACME 数据目录（账号 + 产物）
	DNSProvider string            // dns-01 用
	DNSEnvVars  map[string]string // dns-01 凭据
	Timeout     time.Duration     // 整体超时（0 = 5 分钟）
	// EAB：Google / ZeroSSL **注册 ACME 账号时必填** ✗（面板按 CA 自动关联后下发 ✓）。
	EABKeyID  string
	EABMACKey string
	// CertID 是面板侧的证书 id ✓（仅用于上报/对账 ✓；节点自续时从 meta 读回 ✓）。
	CertID int64
	// KeyType / KeyBits 决定**签出来的私钥类型** ✓（ecc|rsa + 位数 ✓）。
	// ⚠ 原来这里**没有这两个字段** ✗ → 节点侧永远走 EC256 ✗
	// （面板记录 rsa 也白搭 ✓）。空/0 = 历史默认（EC256 ✓）。
	KeyType string
	KeyBits int
}

// defaultAgentACMECertDir 是节点侧 ACME 数据目录的**固定绝对默认值** ✓。
// 里面存的是 ACME **账号**（每 CA 一份 `<ca>.json` ✓）+ 产物 ✓ ——
// 账号身份文件丢了就得重新注册 ✗（Google 要新 EAB ✗），所以路径必须稳定 ✓。
const defaultAgentACMECertDir = "/var/lib/corefusion-agent/acme"

// issuedCert 是一次成功签发的结果 ✓。
type issuedCert struct {
	Result       *cert.IssueResult
	CertPEM      string
	KeyPEM       string
	FullchainPEM string
}

// issueACMECert 在**本机**完成一次 ACME 签发/续签 ✓。
//
// 为什么必须在节点本机做 ✗：面板常位于 SNAT 之后（只有出站、没有入站），CA 无法回调面板的 80 端口
// 完成 http-01 验证 ✗；而节点有公网 IP 与 80 入站 ✓。IP 证书还要求 profile=shortlived ✓（RFC 8738 ✓）。
func issueACMECert(p acmeIssueParams) (*issuedCert, error) {
	domains := make([]string, 0, len(p.Identifiers))
	for _, d := range p.Identifiers {
		if d = strings.TrimSpace(d); d != "" {
			domains = append(domains, d)
		}
	}
	if len(domains) == 0 {
		return nil, fmt.Errorf("no identifiers to issue")
	}
	email := strings.TrimSpace(p.Email)
	if email == "" {
		return nil, fmt.Errorf("acme email is required")
	}
	challenge := strings.TrimSpace(p.Challenge)
	if challenge == "" {
		challenge = "http-01"
	}

	// 复用面板的校验：公网 IP 放行 ✓、私网/保留地址拒绝 ✓、通配符必须 dns-01 ✓
	if err := certsvc.ValidateACMEDomains(domains, challenge); err != nil {
		return nil, err
	}

	prof := strings.TrimSpace(p.Profile)
	if prof == "" {
		prof = certsvc.DefaultProfileFor(domains)
		if prof != "" {
			log.Printf("[acme] %s → 自动使用 profile=%s（LE 的 IP 证书是短时证书，约 6 天）", domains[0], prof)
		}
	}

	provider := cert.CAProvider(p.CA)
	if provider == "" {
		provider = cert.CAProviderLetsEncrypt
	}

	dataDir := strings.TrimSpace(p.DataDir)
	if dataDir == "" {
		dataDir = defaultAgentACMECertDir
	}
	// ⚠⚠ 必须**固定成绝对路径** ✗：相对路径（原来的 "./acme-certs" ✗）会随进程 CWD 漂移 ✗
	// → ACME **账号文件找不到** ✗ → 每次都重新注册 ✗ → Google 那种"7 天一次性"的 EAB
	// 会被反复消耗、必然失败 ✗（用户问到过这个隐患 ✓）。绝对路径 + MkdirAll 才能"注册一次、长期续期" ✓。
	if abs, aerr := filepath.Abs(dataDir); aerr == nil {
		dataDir = abs
	}
	if merr := os.MkdirAll(dataDir, 0o750); merr != nil {
		return nil, fmt.Errorf("create acme data dir %s: %w", dataDir, merr)
	}

	opts := cert.IssueOptions{
		Domains:       domains,
		ChallengeType: cert.ChallengeType(challenge),
		Profile:       prof,
		// ⚠ 必须传下去 ✗（原来这里也不带 ✗）→ 不带就由 createClient 回退 EC256 ✗
		KeyType: strings.TrimSpace(p.KeyType),
		KeyBits: p.KeyBits,
	}
	// 首次申请即"自动注册"✓：把面板下发的 EAB 交给 lego ✓（账号已在本地时它不会再用 EAB ✓）
	if strings.TrimSpace(p.EABKeyID) != "" && strings.TrimSpace(p.EABMACKey) != "" {
		opts.EAB = &cert.EABConfig{KeyID: strings.TrimSpace(p.EABKeyID), MACKey: strings.TrimSpace(p.EABMACKey)}
		log.Printf("[acme] 携带 EAB（仅注册用 ✓）kid=%s", strings.TrimSpace(p.EABKeyID))
	}

	var client *cert.ACMEClient
	cleanup := func() {}
	switch {
	case strings.EqualFold(challenge, "dns-01"):
		if strings.TrimSpace(p.DNSProvider) == "" {
			return nil, fmt.Errorf("dns-01 requires a dns provider")
		}
		env := map[string]string{}
		for k, v := range p.DNSEnvVars {
			env[k] = v
		}
		opts.DNSProvider = &cert.DNSProviderConfig{Provider: cert.DNSProvider(p.DNSProvider), EnvVars: env}
		client = cert.NewACMEClient(dataDir, email, provider)
	default:
		// http-01：**全自动**决定怎么提供挑战 ✓（产品的既定设计 ✓）：
		//   ① 80 空闲 → 临时起 80 服务 ✓
		//   ② 80 被占用 → 自动写一份只服务 /.well-known/acme-challenge/ 的配置并 reload ✓（用完自动还原 ✓）
		// 因此**不需要**任何人手工填 webroot ✗（`p.Webroot` 仅作显式覆盖 ✓）。
		root, cl, perr := prepareHTTP01(domains, dataDir, p.Webroot)
		if perr != nil {
			return nil, perr
		}
		cleanup = cl
		if root != "" {
			client = cert.NewACMEClientWithChallenge(dataDir, email, provider, cert.NewWebrootProvider(root))
		} else {
			client = cert.NewACMEClientWithHTTP01(dataDir, email, provider, cert.NewHTTP01Provider())
		}
	}
	defer cleanup()

	timeout := p.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	log.Printf("[acme] 开始签发：SAN=%v challenge=%s ca=%s profile=%q key=%s/%d data_dir=%s（账号会落在这里 ✓）",
		domains, challenge, provider, prof, cert.NormalizeKeyType(p.KeyType), p.KeyBits, dataDir)
	result, err := client.Issue(ctx, opts)
	if err != nil {
		return nil, err
	}

	certPEM, err := os.ReadFile(joinPath(dataDir, result.CertPath))
	if err != nil {
		return nil, fmt.Errorf("read issued cert: %w", err)
	}
	keyPEM, err := os.ReadFile(joinPath(dataDir, result.KeyPath))
	if err != nil {
		return nil, fmt.Errorf("read issued key: %w", err)
	}
	fullchain := certPEM
	if b, ferr := os.ReadFile(joinPath(dataDir, result.FullchainPath)); ferr == nil && len(b) > 0 {
		fullchain = b
	}

	// ── 落一份 meta ✓：这张证书从此**归节点自管**（自续期靠它 ✓）──────────────
	// ⚠ 这份文件就是"所有权"的分界线 ✓：面板 `cert_distributor` 推下来的证书**不会有 meta** ✗，
	// 于是自续期**不会**去动它们 ✓（那种证书归面板管 ✓，节点擅自续会绕开面板口径 ✗）。
	// 所以 meta 的存在本身就是"面板只管触发、节点持有并自续"这条架构的落点 ✓。
	// ⚠ 不能再用 `filepath.Base(filepath.Dir(...))` ✗：RSA 落在 `certs/<name>/rsa/cert.pem` ✓，
	// 取出来会是 "rsa" ✗ → meta 写到 `certs/rsa/meta.json` ✗（自续期再也找不到它 ✗）。
	certName := certNameFromRelPath(result.CertPath)
	meta := &certMeta{
		CertID:      p.CertID,
		Identifiers: domains,
		Email:       email,
		CA:          string(provider),
		Challenge:   challenge,
		Profile:     prof,
		Webroot:     strings.TrimSpace(p.Webroot),
		DNSProvider: strings.TrimSpace(p.DNSProvider),
		DNSEnvVars:  p.DNSEnvVars,
		// ⚠ 密钥类型也要记住 ✗：节点自续期是"照 meta 重签一遍" ✓，
		// 不记住就退回 EC256 ✗ → 等于换了一种密钥 ✗（每次自续都换密钥 ✗）。
		KeyType:  cert.NormalizeKeyType(p.KeyType),
		KeyBits:  p.KeyBits,
		IssuedAt: time.Now(),
	}
	// 续期要保留首次的签发时间与面板 id ✓（覆盖成"这次"会让历史信息丢失 ✗）
	if prev, perr := readCertMeta(dataDir, certName); perr == nil && prev != nil {
		if !prev.IssuedAt.IsZero() {
			meta.IssuedAt = prev.IssuedAt
		}
		if meta.CertID == 0 {
			meta.CertID = prev.CertID
		}
		meta.LastRenewAt = time.Now()
	}
	if merr := writeCertMeta(dataDir, certName, meta); merr != nil {
		// meta 写不进去只影响"以后能不能自续" ✓，**不影响本次签发成功** ✓ → 记日志不返回错误 ✓
		log.Printf("[acme] 写入自续期元数据失败（本次签发不受影响，但该证书将无法自续）: %v", merr)
	} else {
		log.Printf("[acme] 已记录自续期元数据：%s（节点持有 ✓，到期前自动续 ✓）", certMetaPath(dataDir, certName))
	}

	return &issuedCert{
		Result:       result,
		CertPEM:      string(certPEM),
		KeyPEM:       string(keyPEM),
		FullchainPEM: string(fullchain),
	}, nil
}

func joinPath(dir, rel string) string {
	if strings.HasPrefix(rel, "/") {
		return rel
	}
	return strings.TrimSuffix(dir, "/") + "/" + rel
}
