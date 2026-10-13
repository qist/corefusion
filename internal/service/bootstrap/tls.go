package bootstrap

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"path/filepath"
	"strings"
	"time"

	"corefusion/internal/model/entity"
	corecert "corefusion/internal/runtime/cert"
	certsvc "corefusion/internal/service/certificate"
)

// 面板 TLS 证书**复用面板既有的证书库**（internal/service/certificate）：
// 证书落盘到 dataDir/certs/<name>/ 并注册进 certificates 表，与「设置 → 面板 TLS」
// 里选证书走的是同一套机制（可管理、可替换、可在证书页看到），
// 初始化向导与 CLI 都不再自建证书目录。

// SelfSigned 生成自签名证书（P-256，有效期 1 年）并写入面板证书库。
// 适用内网/测试：浏览器会提示不受信任（设计 3.3.5）。
func SelfSigned(ctx context.Context, dataDir string, svc *certsvc.Service, domains []string) (certPath, keyPath string, err error) {
	certPEM, keyPEM, gerr := generateSelfSigned(domains)
	if gerr != nil {
		return "", "", gerr
	}
	return storePanelTLS(ctx, dataDir, svc, domains, certPEM, keyPEM)
}

// SaveUploaded 校验并保存用户上传的证书/私钥（设计 3.3.5「上传自定义证书」）。
func SaveUploaded(ctx context.Context, dataDir string, svc *certsvc.Service, certPEM, keyPEM []byte) (certPath, keyPath string, err error) {
	if strings.TrimSpace(string(certPEM)) == "" {
		return "", "", fmt.Errorf("certificate PEM is required")
	}
	if !strings.Contains(string(keyPEM), "PRIVATE KEY") {
		return "", "", fmt.Errorf("invalid private key PEM")
	}
	return storePanelTLS(ctx, dataDir, svc, nil, certPEM, keyPEM)
}

// ACMEDeps 是 ACME 签发的配套服务（均可为 nil，按需使用）：
// EAB —— ZeroSSL 邮箱换 EAB / Google 服务账号自动申请；DNS —— dns-01 通道。
type ACMEDeps struct {
	EAB *certsvc.EABConfigService
	DNS *certsvc.DNSChannelService
}

// IssueACMEOptions 是 ACME 签发的完整入参（向导与 CLI 共用）。
type IssueACMEOptions struct {
	Domains []string
	Email   string
	// Provider: lets_encrypt | lets_encrypt_staging | zerossl | google
	Provider string
	// Challenge: http-01（默认）| dns-01；dns-01 需 DNS 凭据。
	Challenge string
	// KeyType / KeyBits：签发私钥的密钥级别（ecc256/384、rsa2048/3072/4096；空= ecc）。
	KeyType string
	KeyBits int
	// dns-01 的两种来源：已有 DNS 通道 ID，或向导直填凭据（DNSProvider + DNSEnv，
	// 会先加密建成 DNS 通道再使用，续期可直接按通道还原）。
	DNSChannelID int64
	DNSProvider  string
	DNSEnv       map[string]string
	// IPCert：IP 证书 —— SAN 为本机公网 IPv4/IPv6（自动发现），仅 Let's Encrypt 支持。
	IPCert bool
	// EAB：Google 需手工 EAB（EABKeyID/EABMACKey）或 GCP 服务账号自动申请
	//（ProjectID + ServiceAccount）；ZeroSSL 用邮箱自动领取，无需手填。
	EABKeyID       string
	EABMACKey      string
	ProjectID      string
	ServiceAccount string
}

// IssueACME 通过 ACME 签发面板证书并写入面板既有证书库（设计 3.3.5）：
// 挑战方式与密钥级别随签发落库（自动续期按原选择走）；ZeroSSL/Google 的 EAB
// 在这一步自动解决。
func IssueACME(ctx context.Context, dataDir string, svc *certsvc.Service, deps ACMEDeps, opts IssueACMEOptions) (certPath, keyPath string, err error) {
	if len(opts.Domains) == 0 {
		return "", "", fmt.Errorf("at least one domain required")
	}
	in := certsvc.IssueACMEInput{
		Domains:       opts.Domains,
		KeyType:       corecert.NormalizeKeyType(opts.KeyType),
		KeyBits:       opts.KeyBits,
		ChallengeType: certsvc.NormalizeChallengeType(opts.Challenge),
		CAProvider:    normalizeCAProvider(opts.Provider),
		Email:         opts.Email,
		Profile:       certsvc.DefaultProfileFor(opts.Domains),
		DNSChannelID:  opts.DNSChannelID,
	}
	// dns-01：优先已有通道；否则把直填凭据先建成加密 DNS 通道。
	if in.ChallengeType == string(corecert.ChallengeDNS01) {
		switch {
		case opts.DNSChannelID > 0:
		case strings.TrimSpace(opts.DNSProvider) != "" && deps.DNS != nil:
			ch, cerr := deps.DNS.Create(ctx, "初始化 - "+strings.TrimSpace(opts.DNSProvider), strings.ToLower(strings.TrimSpace(opts.DNSProvider)), opts.DNSEnv)
			if cerr != nil {
				return "", "", fmt.Errorf("DNS 通道创建失败: %w", cerr)
			}
			in.DNSChannelID = ch.ID
		default:
			return "", "", fmt.Errorf("dns-01 需要 DNS 接入：选择 DNS 服务商并填写凭据，或选择已有 DNS 通道")
		}
	}
	// EAB：ZeroSSL 邮箱自动领取；Google 手工 EAB 或 GCP 服务账号自动申请。
	p := normalizeCAProvider(opts.Provider)
	if p == corecert.CAProviderZeroSSL || p == corecert.CAProviderGoogle {
		// 两家都必须有联系邮箱（Google 注册强制；ZeroSSL 用邮箱换 EAB）。
		if strings.TrimSpace(opts.Email) == "" {
			return "", "", fmt.Errorf("%s 必须填写 ACME 联系邮箱", p)
		}
		switch {
		case p == corecert.CAProviderZeroSSL:
			if deps.EAB == nil {
				return "", "", fmt.Errorf("ZeroSSL EAB 服务不可用")
			}
			cfg, ferr := deps.EAB.FetchZeroSSLEAB(ctx, opts.Email)
			if ferr != nil {
				return "", "", fmt.Errorf("ZeroSSL EAB 自动申请失败: %w", ferr)
			}
			in.EAB = eabFromEntity(cfg)
		case opts.EABKeyID != "" && opts.EABMACKey != "":
			in.EAB = &corecert.EABConfig{KeyID: opts.EABKeyID, MACKey: opts.EABMACKey}
		case strings.TrimSpace(opts.ProjectID) != "" && strings.TrimSpace(opts.ServiceAccount) != "" && deps.EAB != nil:
			cfg, ferr := deps.EAB.FetchGoogleEAB(ctx, opts.ProjectID, opts.ServiceAccount, "")
			if ferr != nil {
				return "", "", fmt.Errorf("Google EAB 自动申请失败: %w", ferr)
			}
			in.EAB = eabFromEntity(cfg)
		default:
			return "", "", fmt.Errorf("Google Public CA 需要外部账号绑定（EAB）：手工填入 EAB-KID/EAB-HMAC-Key，或提供 GCP 项目 ID 与服务账号 JSON 自动申请")
		}
	}
	cert, ierr := svc.IssueACME(ctx, in)
	if ierr != nil {
		return "", "", ierr
	}
	return absCertPath(dataDir, cert.CertPath), absCertPath(dataDir, cert.KeyPath), nil
}

// eabFromEntity 把库里加密存储的 EAB 配置还原成 runtime 需要的 EAB 凭据。
func eabFromEntity(e *entity.EABConfig) *corecert.EABConfig {
	if e == nil {
		return nil
	}
	return &corecert.EABConfig{KeyID: e.KeyID, MACKey: e.MACKey}
}

// LocalPublicIPs 返回本机所有公网 IPv4 + IPv6 地址（公信 CA 只能验证公网地址）：
// 选 IP 证书时自动取 v4；有 v6 一并取；只有 v6 没有就只取 v6。
func LocalPublicIPs() []string {
	var out []string
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		ip := ipnet.IP
		if ip == nil || !certsvc.IsPubliclyReachableIP(ip) {
			continue
		}
		out = append(out, ip.String())
	}
	return out
}

// storePanelTLS 把证书写入面板证书库，返回面板可直接使用的绝对路径
// （与 CLI/向导落库的 panel.tls_cert_path / panel.tls_key_path 对应）。
func storePanelTLS(ctx context.Context, dataDir string, svc *certsvc.Service, domains []string, certPEM, keyPEM []byte) (string, string, error) {
	in := certsvc.UploadCertificateInput{
		CertContent: string(certPEM),
		KeyContent:  string(keyPEM),
	}
	if len(domains) > 0 {
		in.Domain = strings.TrimSpace(domains[0])
	}
	cert, err := svc.Upload(ctx, in)
	if err != nil {
		return "", "", err
	}
	return absCertPath(dataDir, cert.CertPath), absCertPath(dataDir, cert.KeyPath), nil
}

// absCertPath 把证书库里相对 dataDir 的路径补成绝对路径。
func absCertPath(dataDir, rel string) string {
	if filepath.IsAbs(rel) {
		return rel
	}
	return filepath.Join(dataDir, rel)
}

func normalizeCAProvider(provider string) corecert.CAProvider {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "lets_encrypt_staging", "staging":
		return corecert.CAProviderLetsEncryptStaging
	case "zerossl":
		return corecert.CAProviderZeroSSL
	case "google":
		return corecert.CAProviderGoogle
	default:
		return corecert.CAProviderLetsEncrypt
	}
}

// generateSelfSigned 生成 P-256 自签名证书 PEM（有效期 1 年）。
func generateSelfSigned(domains []string) (certPEM, keyPEM []byte, err error) {
	if len(domains) == 0 {
		domains = []string{"localhost"}
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: strings.TrimSpace(domains[0])},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	for _, d := range domains {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		if ip := net.ParseIP(d); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else {
			tmpl.DNSNames = append(tmpl.DNSNames, d)
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, err
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, err
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return certPEM, keyPEM, nil
}
