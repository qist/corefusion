package certificate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"corefusion/internal/model/entity"
)

// AgentIssuedCertInput 是"节点本机 ACME 签发完成"后回传的入库输入 ✓。
type AgentIssuedCertInput struct {
	CertID       int64    // 面板证书 ID（可选：有则按 ID 更新 ✓）
	Identifiers  []string // 域名或 IP ✓
	CertPEM      string
	KeyPEM       string
	FullchainPEM string
	ExpiresAt    time.Time
	CA           string
}

// StoreAgentIssuedCert 把 Agent 在**节点本机**签好的公网证书落盘 + 落库 ✓。
//
// 与 Upload 的区别 ✓：来源是节点自动签发（provider_type=acme ✓、auto_renew=1 ✓、
// 保留原证书的 name/id ✓），因此续期后**配置引用不会被换掉** ✓。
func (s *Service) StoreAgentIssuedCert(ctx context.Context, in AgentIssuedCertInput) (*entity.Certificate, error) {
	if strings.TrimSpace(in.CertPEM) == "" || strings.TrimSpace(in.KeyPEM) == "" {
		return nil, errors.New("cert and key content are required")
	}

	tmpDir := filepath.Join(s.dataDir, "certs_tmp")
	if err := os.MkdirAll(tmpDir, 0750); err != nil {
		return nil, err
	}
	suffix := fmt.Sprintf("%x", time.Now().UnixNano())
	tmpCert := filepath.Join(tmpDir, "agent_cert_"+suffix+".pem")
	tmpKey := filepath.Join(tmpDir, "agent_key_"+suffix+".pem")
	if err := os.WriteFile(tmpCert, []byte(in.CertPEM), 0600); err != nil {
		return nil, err
	}
	if err := os.WriteFile(tmpKey, []byte(in.KeyPEM), 0600); err != nil {
		os.Remove(tmpCert)
		return nil, err
	}
	defer os.Remove(tmpCert)
	defer os.Remove(tmpKey)

	// 解析：CN/SAN/issuer/有效期/密钥类型 —— 全部用**文件**版本（与 Upload 同一套解析 ✓）
	expiresAt, err := parseCertExpiry(tmpCert)
	if err != nil {
		return nil, fmt.Errorf("parse certificate: %w", err)
	}
	if !in.ExpiresAt.IsZero() {
		// 以 Agent 上报的有效期为准时也做一次合理性判断（解析失败才用上报值 ✓）
		expiresAt = in.ExpiresAt
	}
	issuer := parseCertIssuer(tmpCert)
	sans := parseCertDomains(tmpCert)
	keyType, keyBits, _ := detectKeyType(tmpKey)
	parsed := parseCertDomainList(tmpCert)

	// 目标行：优先按 CertID 找，其次按第一个标识符找 ✓（就地更新 → 引用不失效 ✓）
	var existing *entity.Certificate
	if in.CertID > 0 {
		if c, gerr := s.repo.GetByID(ctx, in.CertID); gerr == nil {
			existing = c
		}
	}
	if existing == nil && len(parsed) > 0 {
		if c, gerr := s.repo.GetByDomain(ctx, parsed[0]); gerr == nil {
			existing = c
		}
	}
	if existing == nil && len(in.Identifiers) > 0 {
		if c, gerr := s.repo.GetByDomain(ctx, in.Identifiers[0]); gerr == nil {
			existing = c
		}
	}

	domain := ""
	name := ""
	if existing != nil {
		domain = existing.Domain
		name = existing.Name
	}
	if domain == "" && len(parsed) > 0 {
		domain = parsed[0]
	}
	if domain == "" && len(in.Identifiers) > 0 {
		domain = in.Identifiers[0]
	}
	if domain == "" {
		domain = "unknown"
	}
	if name == "" {
		name = "acme-" + domain
	}

	certDir := filepath.Join(s.dataDir, "certs", sanitizeDomain(name))
	if err := os.MkdirAll(certDir, 0750); err != nil {
		return nil, err
	}
	certRel := filepath.Join("certs", sanitizeDomain(name), "cert.pem")
	keyRel := filepath.Join("certs", sanitizeDomain(name), "key.pem")
	if err := os.WriteFile(filepath.Join(s.dataDir, certRel), []byte(in.CertPEM), 0600); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(s.dataDir, keyRel), []byte(in.KeyPEM), 0600); err != nil {
		return nil, err
	}

	if existing == nil {
		existing = &entity.Certificate{
			Name:         name,
			Domain:       domain,
			Domains:      sans,
			Issuer:       issuer,
			ProviderType: "acme",
			KeyType:      keyType,
			KeyBits:      keyBits,
			CertPath:     certRel,
			KeyPath:      keyRel,
			CertContent:  in.CertPEM,
			KeyContent:   in.KeyPEM,
			Status:       "active",
			AutoRenew:    1,
			ExpiresAt:    expiresAt,
		}
		if err := s.repo.Create(ctx, existing); err != nil {
			return nil, err
		}
		return existing, nil
	}

	existing.Name = name
	existing.Domains = sans
	existing.Issuer = issuer
	existing.ProviderType = "acme"
	existing.KeyType = keyType
	existing.KeyBits = keyBits
	existing.CertPath = certRel
	existing.KeyPath = keyRel
	existing.CertContent = in.CertPEM
	existing.KeyContent = in.KeyPEM
	existing.Status = "active"
	existing.AutoRenew = 1
	existing.ExpiresAt = expiresAt
	now := time.Now()
	existing.LastRenewedAt = &now
	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, err
	}
	return existing, nil
}
