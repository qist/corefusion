package certificate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CertMaterial 是"下载/打包"需要的证书材料 ✓。
//
// ⚠ CertPEM 里通常是**叶 + 中间链** ✓（ACME 返回的就是这样 ✓，面板与内核也都按全链使用 ✓）——
// 想拿"仅叶证书"请用 `cert.SplitLeafAndChain` ✓ 在**下载打包**时拆分 ✓，
// **不要**去改磁盘上的 cert.pem ✗（内核配置直接引用它做 TLS ✗，拆掉链会让客户端验不过 ✗）。
type CertMaterial struct {
	Name      string
	Domain    string
	Issuer    string
	ExpiresAt time.Time
	CertPEM   string
	KeyPEM    string
}

// ReadCertMaterial 读取证书材料 ✓：**优先用库里存的** ✓（节点签发回传的只有内容 ✓、磁盘文件在节点上 ✓），
// 库里没有（或为空 ✗）再退回面板本机的文件 ✓（老数据/手工上传 ✓）。
func (s *Service) ReadCertMaterial(ctx context.Context, id int64) (*CertMaterial, error) {
	c, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, fmt.Errorf("certificate %d not found", id)
	}

	certPEM := strings.TrimSpace(c.CertContent)
	keyPEM := strings.TrimSpace(c.KeyContent)

	if certPEM == "" && strings.TrimSpace(c.CertPath) != "" {
		if b, rerr := os.ReadFile(filepath.Join(s.dataDir, c.CertPath)); rerr == nil {
			certPEM = strings.TrimSpace(string(b))
		}
	}
	if keyPEM == "" && strings.TrimSpace(c.KeyPath) != "" {
		if b, rerr := os.ReadFile(filepath.Join(s.dataDir, c.KeyPath)); rerr == nil {
			keyPEM = strings.TrimSpace(string(b))
		}
	}
	// 有些老记录只落了 fullchain 文件 ✓
	if certPEM == "" && strings.TrimSpace(c.FullchainPath) != "" {
		if b, rerr := os.ReadFile(filepath.Join(s.dataDir, c.FullchainPath)); rerr == nil {
			certPEM = strings.TrimSpace(string(b))
		}
	}

	if certPEM == "" {
		return nil, fmt.Errorf("certificate %d has no stored content", id)
	}
	if keyPEM == "" {
		return nil, fmt.Errorf("certificate %d has no stored private key", id)
	}

	name := strings.TrimSpace(c.Name)
	if name == "" {
		name = c.Domain
	}
	return &CertMaterial{
		Name:      name,
		Domain:    c.Domain,
		Issuer:    c.Issuer,
		ExpiresAt: c.ExpiresAt,
		CertPEM:   certPEM + "\n",
		KeyPEM:    keyPEM + "\n",
	}, nil
}
