package agent

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"strings"
	"time"

	agentpb "corefusion/internal/runtime/agent/proto"
	"corefusion/internal/store/repository"
)

// PublicCertRenewer 让**公网证书**（ACME 签的域名/IP 证书）也自动续期 ✓。
//
// 背景 ✗：
//   - 面板已有 CertRenewer，但它续的是 **Agent 的 mTLS 证书**（面板自签 CA ✓），
//     跟"对外提供服务用的公网证书"完全是两回事 ✗；
//   - 公网证书的 ACME 续期（Service.RenewACME）此前只挂在**手动接口**上 ✗ → 没人点就不会续 ✗。
//
// 为什么让节点签、而不是面板签 ✗：面板常在 SNAT 后面（只有出站、没有入站），
// CA 无法回调面板的 80 端口做 http-01 验证 ✗；节点有公网 IP 与 80 入站 ✓。
//
// 闭环：本调度器发现"快到期且绑定了节点"的证书 → 给该节点的 Agent 下发
// IssuePublicCertCommand → Agent 在本机签发 → 回传 PEM（AgentReport.PUBLIC_CERT_ISSUED）
// → 面板由 PublicCertSink 落盘落库 ✓。
// 续期触发阈值 —— 按证书类型区分 ✓（用户口径）：
//   - **IP 证书**：Let's Encrypt 的 IP 证书是**短时证书**（约 6 天 ✓，profile=shortlived ✓），
//     所以剩 **3 天**就要开始续 ✓，否则一不留神就过期 ✗；
//   - **域名证书**：常规证书约 90 天 ✓，剩 **20 天**开始续 ✓（留足重试与排查时间 ✓）。
const (
	PublicCertIPRenewBefore     = 3 * 24 * time.Hour
	PublicCertDomainRenewBefore = 20 * 24 * time.Hour
)

// renewThresholdFor 返回该证书该提前多久续 ✓（含 IP 标识符 → 按短时证书处理 ✓）。
func renewThresholdFor(identifiers []string) time.Duration {
	for _, id := range identifiers {
		if net.ParseIP(strings.TrimSpace(id)) != nil {
			return PublicCertIPRenewBefore
		}
	}
	return PublicCertDomainRenewBefore
}

type PublicCertRenewer struct {
	certRepo    *repository.CertificateRepository
	serverRepo  *repository.NodeServerRepository
	issuer      PublicCertIssuer
	interval    time.Duration
	renewBefore time.Duration // >0 时**同时覆盖**两类阈值（测试/自定义用 ✓）
}

// PublicCertIssuer 是"把签发指令下发给节点"的能力（由 NodeServerManager 实现 ✓）。
type PublicCertIssuer interface {
	IssuePublicCert(ctx context.Context, serverID int64, cmd *agentpb.IssuePublicCertCommand) error
}

// PublicCertResult 是 Agent 回传的签发结果（跨包传递用，避免循环依赖 ✓）。
type PublicCertResult struct {
	CertID       int64
	ServerID     int64
	Identifiers  []string
	Success      bool
	CertPEM      string
	KeyPEM       string
	FullchainPEM string
	ExpiresAt    time.Time
	CA           string
	Error        string
}

// PublicCertSink 由面板的证书服务实现：把 Agent 签好的证书落盘落库 ✓，
// 并返回面板侧的证书 ID（调用方据此把证书绑定到签发它的节点 ✓ → 自动续期才有目标 ✓）。
type PublicCertSink interface {
	StoreAgentIssuedCert(ctx context.Context, res *PublicCertResult) (int64, error)
}

// NewPublicCertRenewer 创建公网证书自动续期调度器 ✓。
func NewPublicCertRenewer(certRepo *repository.CertificateRepository, serverRepo *repository.NodeServerRepository, issuer PublicCertIssuer) *PublicCertRenewer {
	return &PublicCertRenewer{
		certRepo:   certRepo,
		serverRepo: serverRepo,
		issuer:     issuer,
		interval:   6 * time.Hour,
		// renewBefore 留 0 = **按类型自动判定** ✓（IP=3 天、域名=20 天 ✓）；
		// 查询窗口由 window() 取两者较大的那个 ✓。
	}
}

// WithInterval 设置检查间隔（测试/自定义用 ✓）。
func (r *PublicCertRenewer) WithInterval(d time.Duration) *PublicCertRenewer {
	if d > 0 {
		r.interval = d
	}
	return r
}

// WithRenewBefore 显式覆盖续期阈值（**同时作用于两类**；留空/0 则按类型自动判定 ✓）。
func (r *PublicCertRenewer) WithRenewBefore(d time.Duration) *PublicCertRenewer {
	if d > 0 {
		r.renewBefore = d
	}
	return r
}

// RunLoop 周期检查并续期，直到 ctx 取消 ✓。
func (r *PublicCertRenewer) RunLoop(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	if err := r.checkAndRenew(ctx); err != nil {
		log.Printf("[PublicCertRenewer] initial check error: %v", err)
	}
	for {
		select {
		case <-ctx.Done():
			log.Println("[PublicCertRenewer] stopped")
			return
		case <-ticker.C:
			if err := r.checkAndRenew(ctx); err != nil {
				log.Printf("[PublicCertRenewer] check error: %v", err)
			}
		}
	}
}

// window 是查询窗口：取两类阈值中**较大**的 ✓（否则只查短时证书的窗口会漏掉域名证书 ✗）。
func (r *PublicCertRenewer) window() time.Duration {
	if r.renewBefore > 0 {
		return r.renewBefore
	}
	if PublicCertDomainRenewBefore > PublicCertIPRenewBefore {
		return PublicCertDomainRenewBefore
	}
	return PublicCertIPRenewBefore
}

// thresholdFor 返回这张证书该提前多久续 ✓（显式覆盖优先 ✓，否则按类型 ✓）。
func (r *PublicCertRenewer) thresholdFor(identifiers []string) time.Duration {
	if r.renewBefore > 0 {
		return r.renewBefore
	}
	return renewThresholdFor(identifiers)
}

// checkAndRenew 找出"快到期 + 开了自动续期"的证书，交给绑定节点的 Agent 在本机续 ✓。
func (r *PublicCertRenewer) checkAndRenew(ctx context.Context) error {
	if r.issuer == nil {
		return nil
	}
	certs, err := r.certRepo.FindExpiringSoon(ctx, r.window())
	if err != nil {
		return err
	}
	now := time.Now()
	for i := range certs {
		c := &certs[i]
		if c.AutoRenew == 0 {
			continue
		}
		identifiers := certIdentifiers(c.Domains, c.Domain)
		if len(identifiers) == 0 {
			continue
		}
		// 按类型决定阈值 ✓：IP 证书剩 3 天、域名证书剩 20 天开始续 ✓
		threshold := r.thresholdFor(identifiers)
		if c.ExpiresAt.After(now.Add(threshold)) {
			continue // 还早，不用续 ✓
		}
		servers, err := r.serverRepo.ListByCertificateID(ctx, c.ID)
		if err != nil {
			log.Printf("[PublicCertRenewer] list servers for cert %d: %v", c.ID, err)
			continue
		}
		if len(servers) == 0 {
			// 没有绑定节点的证书（例如在面板本地用 dns-01 签的 ✓）不在这里处理 ✓
			continue
		}
		for _, srv := range servers {
			if srv.IsLocal {
				continue // 面板本机不需要远程签发 ✓
			}
			cmd := &agentpb.IssuePublicCertCommand{
				CertId:      c.ID,
				Identifiers: identifiers,
			}
			if err := r.issuer.IssuePublicCert(ctx, srv.ID, cmd); err != nil {
				log.Printf("[PublicCertRenewer] cert %d (%v) → server %d: issue command failed: %v",
					c.ID, identifiers, srv.ID, err)
				continue
			}
			log.Printf("[PublicCertRenewer] cert %d (%v) expiring %s (剩 %s，阈值 %s) → asked server %d (%s) to renew on-node",
				c.ID, identifiers, c.ExpiresAt.Format(time.RFC3339),
				time.Until(c.ExpiresAt).Truncate(time.Hour), threshold, srv.ID, srv.Name)
		}
	}
	return nil
}

// certIdentifiers 取证书的标识符：优先 domains（JSON 数组 ✓），退化为 domain ✓。
func certIdentifiers(domains, domain string) []string {
	out := make([]string, 0, 2)
	if strings.TrimSpace(domains) != "" {
		var arr []string
		if err := json.Unmarshal([]byte(domains), &arr); err == nil {
			for _, d := range arr {
				if d = strings.TrimSpace(d); d != "" {
					out = append(out, d)
				}
			}
		}
	}
	if len(out) == 0 && strings.TrimSpace(domain) != "" {
		out = append(out, strings.TrimSpace(domain))
	}
	return out
}
