package main

import (
	"context"
	"fmt"
	"log"
	"os"
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
}

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
		dataDir = "./acme-certs"
	}

	opts := cert.IssueOptions{
		Domains:       domains,
		ChallengeType: cert.ChallengeType(challenge),
		Profile:       prof,
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

	log.Printf("[acme] 开始签发：SAN=%v challenge=%s ca=%s profile=%q", domains, challenge, provider, prof)
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
