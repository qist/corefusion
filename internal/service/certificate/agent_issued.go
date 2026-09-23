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
	// ServerID 是"回传这张证书的节点" ✓ = 签发节点 ✓（写进 IssuerNodeID ✓）
	ServerID int64
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
		// 名字直接用标识符 ✓（原来是 "acme-"+domain ✗，列表里会莫名多个前缀 ✓，
		// 与面板本机签出来的那批（名字=域名 ✓）不一致 ✓）
		name = domain
	}

	// ⚠ 这里以前还会 `MkdirAll(data/certs/<name>)` 建目录 ✗（为写文件做准备 ✓）——
	// 现在面板**不落地任何证书文件** ✗，建空目录只会让人误以为"面板上有这份证书" ✗ → 去掉 ✓。
	// ── 面板**不落地**这张证书 ✗（架构口径 ✓：面板只管触发 ✓，证书由节点持有并由节点自己续 ✓）──
	// 以前的实现是：把 PEM/私钥**写进面板的 `data/certs/`** ✗、**再整份存进数据库** ✗ ——
	// 于是"任何一个节点签的每一张证书都会落到面板" ✓（用户口径：面板并没用到这些证书 ✗）。
	// 现在这里**只保留元数据** ✓（上面的 tmp 文件仅用于解析 ✓ 用完即删 ✓）：
	//   · 路径字段保留为**节点侧引用** ✓（用于"这张证书被谁用了"的匹配 ✓）；
	//   · `CertContent/KeyContent` **一律留空** ✗ → 接口返回 `has_content=false` ✓。
	certRel := filepath.Join("certs", sanitizeDomain(name), "cert.pem")
	keyRel := filepath.Join("certs", sanitizeDomain(name), "key.pem")
	// fullchain 的路径同样只作为**节点侧引用** ✓（面板不写文件 ✗，见上）
	fullchainRel := filepath.Join("certs", sanitizeDomain(name), "fullchain.pem")

	if existing == nil {
		existing = &entity.Certificate{
			Name:          name,
			Domain:        domain,
			Domains:       sans,
			Issuer:        issuer,
			IssuerNodeID:  in.ServerID,
			ProviderType:  "acme",
			KeyType:       keyType,
			KeyBits:       keyBits,
			CertPath:      certRel,
			KeyPath:       keyRel,
			FullchainPath: fullchainRel,
			CertContent:   "",
			KeyContent:    "",
			Status:        "active",
			AutoRenew:     1,
			ExpiresAt:     expiresAt,
		}
		if err := s.repo.Create(ctx, existing); err != nil {
			return nil, err
		}
		return existing, nil
	}

	existing.Name = name
	existing.Domains = sans
	existing.Issuer = issuer
	if in.ServerID > 0 {
		existing.IssuerNodeID = in.ServerID
	}
	existing.ProviderType = "acme"
	existing.KeyType = keyType
	existing.KeyBits = keyBits
	existing.CertPath = certRel
	existing.KeyPath = keyRel
	existing.FullchainPath = fullchainRel
	existing.CertContent = ""
	existing.KeyContent = ""
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
