package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"corefusion/internal/model/entity"
	agentpb "corefusion/internal/runtime/agent/proto"
	certpkg "corefusion/internal/runtime/cert"
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
	certRepo   *repository.CertificateRepository
	serverRepo *repository.NodeServerRepository
	// nodeCerts 是"节点↔证书"多对多 ✓（一个节点可绑多张 ✓，都要能续 ✓）
	nodeCerts   *repository.NodeCertificateRepository
	issuer      PublicCertIssuer
	interval    time.Duration
	renewBefore time.Duration // >0 时**同时覆盖**两类阈值（测试/自定义用 ✓）
	// dnsResolver 把库里的 `certificates.dns_channel_id` 解析成 dns-01 凭据 ✓（可选注入 ✓）。
	//
	// ⚠ 节点侧自动续期也必须"按创建时的选择" ✗：不下发 Challenge 时，
	// 节点侧 `firstNonEmpty(cmd.Challenge, "http-01")` 一律按 http-01 走 ✗
	// → 创建时选的 dns-01 在自动续期里同样丢失 ✗（通配符证书直接续不出来 ✓）。
	dnsResolver DNSChannelResolver
}

// DNSChannelResolver 是"按面板 DNS 通道 ID 取 dns-01 凭据"的能力 ✓
// （由面板的 DNS 通道服务实现 ✓）—— 与证书服务用的是同一个接口口径 ✓。
type DNSChannelResolver interface {
	ResolveDNSChannel(ctx context.Context, id int64) (*certpkg.DNSProviderConfig, error)
}

// WithDNSChannelResolver 注入 DNS 通道解析器 ✓（不注入也能跑 ✓ —— 但 dns-01 的证书会被**明确跳过** ✗，
// 不会偷偷换成 http-01 ✓）。
func (r *PublicCertRenewer) WithDNSChannelResolver(resolver DNSChannelResolver) *PublicCertRenewer {
	r.dnsResolver = resolver
	return r
}

// PublicCertIssuer 是"把签发指令下发给节点"的能力（由 NodeServerManager 实现 ✓）。
type PublicCertIssuer interface {
	// IssuanceContextFor 返回"给这个 CA 签发/续期时所需的注册上下文"（邮箱 + EAB ✓）。
	// ⚠ 续期也必须带上这些 ✗：否则节点会用**自己的默认 CA**去签 ✗（真机审计发现过 ✓），
	// 而且 Google/ZeroSSL 首次注册需要 EAB ✗。返回空串表示没有 ✓（尽力而为 ✓）。
	IssuanceContextFor(ctx context.Context, caProvider string) (email, eabKID, eabMAC string)
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
	// Challenge / DNSChannelID 是"节点**实际用的**挑战方式与面板通道 id" ✓
	// （节点把下发的值原样带回 ✓）→ 面板落库时记下 ✓ = 续期时"创建时选什么就用什么" ✓。
	// 老 Agent 不带这两个字段 → 空/0 ✓，落库时按"库里已有的值优先"处理 ✓（不清空已有记录 ✓）。
	Challenge    string
	DNSChannelID int64
}

// PublicCertSink 由面板的证书服务实现：把 Agent 签好的证书落盘落库 ✓，
// 并返回面板侧的证书 ID（调用方据此把证书绑定到签发它的节点 ✓ → 自动续期才有目标 ✓）。
// PublicCertIssuanceEnv 由面板注入：按 CA 给出邮箱与 EAB ✓（与"签发时按 CA 自动关联"同源 ✓）。
type PublicCertIssuanceEnv interface {
	IssuanceContextFor(ctx context.Context, caProvider string) (email, eabKID, eabMAC string)
}

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

// WithNodeCertificates 注入多对多仓库 ✓（缺失时退回历史的单值字段 ✓）。
func (r *PublicCertRenewer) WithNodeCertificates(repo *repository.NodeCertificateRepository) *PublicCertRenewer {
	r.nodeCerts = repo
	return r
}

// nodesForCertificate 返回"绑定了这张证书的全部节点" ✓。
//
// 优先用多对多表 ✓（一个节点多张证书 ✓ / 一张证书多个节点 ✓）；
// 表里没有（历史数据还没迁过来 ✓）才退回 `node_servers.certificate_id` 单值查询 ✓。
func (r *PublicCertRenewer) nodesForCertificate(ctx context.Context, certID int64) ([]entity.NodeServer, error) {
	if r.nodeCerts != nil {
		ids, err := r.nodeCerts.ListNodeIDsByCertificate(ctx, certID)
		if err == nil && len(ids) > 0 {
			out := make([]entity.NodeServer, 0, len(ids))
			for _, id := range ids {
				srv, gerr := r.serverRepo.GetByID(ctx, id)
				if gerr == nil && srv != nil {
					out = append(out, *srv)
				}
			}
			if len(out) > 0 {
				return out, nil
			}
		}
	}
	return r.serverRepo.ListByCertificateID(ctx, certID)
}

// issuerNodes 只保留"**签发该证书的那个节点**" ✓ —— 用户口径：谁签发谁续期 ✓。
//
// 为什么必须限定 ✗：一张证书可能被多个节点"使用"（node_certificates 是多对多 ✓），
// 若对它们逐个下发续期指令，会**重复签发** ✗；对 Google/ZeroSSL 还会**重复消耗
// 一次性 EAB** ✗（真机已能看出这个风险 ✓）。
//
// issuerNodeID<=0（面板本机签的 ✓）或该节点已不在绑定列表里 ✓ → 原样返回 ✓（由其它绑定节点兜底 ✓）。
func issuerNodes(servers []entity.NodeServer, issuerNodeID int64) []entity.NodeServer {
	if issuerNodeID <= 0 {
		return servers
	}
	for _, s := range servers {
		if s.ID == issuerNodeID {
			return []entity.NodeServer{s}
		}
	}
	return servers
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

// buildIssueCommand 组装"让节点去签发/续期"的指令 ✓。
//
// ⚠ 必须带上 CA / 邮箱 / EAB ✗ —— 原来只带 CertId + Identifiers ✗，导致三个真问题 ✓：
//
//	① 不带 CA：节点会用**自己的默认 CA（Let's Encrypt）**去续签 ✗ → 签发者被悄悄换掉 ✗
//	   （Google 的证书续成 LE 的 ✗，审计时发现 ✓）；
//	② 不带邮箱：Google 注册直接拒 ✗（`Accounts must have at least one contact.` ✓）；
//	③ 不带 EAB：Google / ZeroSSL 在节点上**首次注册就会失败** ✗；
//	④ 不带密钥类型 ✗：节点侧 `resolveKeyType("",0)` 回退 **EC256** ✗
//	   → 库里登记 `key_type=rsa` 的证书，续期**永远只能续出 EC 证书** ✗
//	   （真机现象：记录 rsa、文件却是 `EC PRIVATE KEY` ✓）。
//
// 带上之后：节点**第一次申请即自动注册** ✓，账号落在节点固定目录 ✓，
// 之后续期只用账号密钥 ✓ —— **再也不会消耗新 EAB** ✓（正是要的效果 ✓）。
func (r *PublicCertRenewer) buildIssueCommand(ctx context.Context, certID int64, issuer string, identifiers []string, keyType string, keyBits int, challengeType string, dnsChannelID int64) (*agentpb.IssuePublicCertCommand, error) {
	ca := normalizeCertCA(issuer)
	email, eabKID, eabMAC := "", "", ""
	if r.issuer != nil {
		email, eabKID, eabMAC = r.issuer.IssuanceContextFor(ctx, ca)
	}

	// ── 挑战方式：**按创建时的选择** ✓ ──
	// ⚠ 原来这里**根本不带 Challenge** ✗（那个字段只有"面板选节点签发"那条路在用 ✗）→
	// 节点侧 `firstNonEmpty(cmd.Challenge, "http-01")` 一律走 http-01 ✗ →
	// 创建时选的 dns-01 在自动续期里**同样丢失** ✗（与面板本机那条路径是同一类 bug ✓）。
	challenge := strings.TrimSpace(challengeType)
	if challenge == "" {
		// 历史数据（挑战方式上线前签的 ✓）没有记录 ✗ → 按标识符兜底 ✓：
		// 含通配符 → 只可能 dns-01 ✓（HTTP-01 不支持通配符 ✗）；其余按历史行为 http-01 ✓。
		challenge = "http-01"
		if hasWildcardIdentifier(identifiers) {
			challenge = "dns-01"
		}
	}

	cmd := &agentpb.IssuePublicCertCommand{
		CertId:      certID,
		Identifiers: identifiers,
		Ca:          ca,
		Email:       email,
		EabKeyId:    eabKID,
		EabMacKey:   eabMAC,
		// ⚠ 密钥类型必须随指令下发 ✗：节点侧拿不到就退回 EC256 ✗（见上方 ④ ✓）。
		KeyType: keyType,
		KeyBits: int32(keyBits),
		// 挑战方式 + profile：与创建时**同一套规则** ✓
		//（IP 证书必须 shortlived ✓，不带 profile 续 IP 证书会被 LE 拒 ✓）
		Challenge: challenge,
		Profile:   profileForIdentifiers(identifiers),
		// ⚠ 面板那条 DNS 通道的 id 也要下发 ✗：节点签发**用不到**它 ✓（凭据就在下面 ✓），
		// 但节点回传证书时要把它带回来 ✓ → 面板落库保持"创建时选的是哪条通道" ✓
		//（不回传 → dns_channel_id 一直是 0 ✗ → 之后自动续期直接跳过 ✗）。
		DnsChannelId: dnsChannelID,
	}

	// dns-01：把**创建时选的那条 DNS 通道**的凭据一起下发 ✓（节点侧零配置 ✓，路径与签发一致 ✓）
	if challenge == "dns-01" {
		if dnsChannelID <= 0 {
			return nil, fmt.Errorf("证书是按 dns-01 签发/记录的，但库里没有 DNS 通道（dns_channel_id=%d）", dnsChannelID)
		}
		if r.dnsResolver == nil {
			return nil, fmt.Errorf("DNS 通道解析器未注入，无法还原 dns-01 凭据（dns_channel_id=%d）", dnsChannelID)
		}
		cfg, err := r.dnsResolver.ResolveDNSChannel(ctx, dnsChannelID)
		if err != nil {
			return nil, fmt.Errorf("解析 DNS 通道 %d 失败: %w", dnsChannelID, err)
		}
		if cfg == nil || strings.TrimSpace(string(cfg.Provider)) == "" {
			return nil, fmt.Errorf("DNS 通道 %d 没有可用的 provider", dnsChannelID)
		}
		cmd.DnsProvider = string(cfg.Provider)
		cmd.DnsEnvVars = cfg.EnvVars
	}

	return cmd, nil
}

// profileForIdentifiers 取 ACME profile ✓：含 IP 的证书必须 `shortlived` ✓（否则 LE 直接拒 ✓），
// 纯域名证书返回空串 ✓（不传 profile ✓）。
//
// ⚠ 与 `service/certificate.DefaultProfileFor` 是**同一套规则** ✓：
// 这里没有直接复用是因为 agent 包不该反向依赖 service 包 ✗（保持调用方向单一 ✓）。
func profileForIdentifiers(identifiers []string) string {
	for _, id := range identifiers {
		if net.ParseIP(strings.TrimSpace(id)) != nil {
			return "shortlived"
		}
	}
	return ""
}

// hasWildcardIdentifier 判断标识符里是否含通配符 ✓（含 → 只可能走 dns-01 ✓）。
func hasWildcardIdentifier(identifiers []string) bool {
	for _, id := range identifiers {
		if strings.HasPrefix(strings.TrimSpace(id), "*.") {
			return true
		}
	}
	return false
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
		servers, err := r.nodesForCertificate(ctx, c.ID)
		// ⚠ 只让**签发节点**续期 ✓（避免多个"使用节点"重复签发、重复消耗一次性 EAB ✗）
		servers = issuerNodes(servers, c.IssuerNodeID)
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
			cmd, cerr := r.buildIssueCommand(ctx, c.ID, c.Issuer, identifiers, c.KeyType, c.KeyBits, c.ChallengeType, c.DNSChannelID)
			if cerr != nil {
				// ✗ 不静默换成 http-01 ✓：原样报出来、本轮跳过 ✓（下次修好通道/记录后自动恢复 ✓）
				log.Printf("[PublicCertRenewer] cert %d (%v) skip renew: %v", c.ID, identifiers, cerr)
				continue
			}
			if err := r.issuer.IssuePublicCert(ctx, srv.ID, cmd); err != nil {
				log.Printf("[PublicCertRenewer] cert %d (%v) → server %d: issue command failed: %v",
					c.ID, identifiers, srv.ID, err)
				continue
			}
			log.Printf("[PublicCertRenewer] cert %d (%v) expiring %s (剩 %s，阈值 %s) → asked server %d (%s) to renew on-node (key=%s/%d challenge=%s dns_channel=%d)",
				c.ID, identifiers, c.ExpiresAt.Format(time.RFC3339),
				time.Until(c.ExpiresAt).Truncate(time.Hour), threshold, srv.ID, srv.Name,
				c.KeyType, c.KeyBits, cmd.Challenge, c.DNSChannelID)
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

// RenewCertsForServer 在 **Agent 重连**时补做一次续期检查 ✓。
//
// 为什么需要它 ✗：RunLoop 在面板启动时会**立即**检查一次 ✓，而那一刻各 Agent 的 gRPC 流
// 通常还没建立 ✗ → 指令发不出去（`server N not connected` ✗），然后就得等下一个周期（6h ✗）。
// 真机踩到过：改了证书有效期想验证续期，重启面板后一直没动静 ✓ —— 正是这个原因 ✓。
func (r *PublicCertRenewer) RenewCertsForServer(ctx context.Context, serverID int64) {
	if r == nil || r.issuer == nil || serverID <= 0 {
		return
	}
	certs, err := r.certRepo.FindExpiringSoon(ctx, r.window())
	if err != nil {
		log.Printf("[PublicCertRenewer] on-reconnect list: %v", err)
		return
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
		if c.ExpiresAt.After(now.Add(r.thresholdFor(identifiers))) {
			continue // 还早 ✓
		}
		servers, serr := r.nodesForCertificate(ctx, c.ID)
		if serr != nil {
			continue
		}
		// ⚠ 同上：只让**签发节点**续期 ✓
		servers = issuerNodes(servers, c.IssuerNodeID)
		for _, srv := range servers {
			if srv.ID != serverID || srv.IsLocal {
				continue
			}
			log.Printf("[PublicCertRenewer] (on-reconnect) cert %d (%v) expiring %s → server %d",
				c.ID, identifiers, c.ExpiresAt.Format(time.RFC3339), srv.ID)
			cmd, cerr := r.buildIssueCommand(ctx, c.ID, c.Issuer, identifiers, c.KeyType, c.KeyBits, c.ChallengeType, c.DNSChannelID)
			if cerr != nil {
				log.Printf("[PublicCertRenewer] (on-reconnect) cert %d (%v) skip renew: %v", c.ID, identifiers, cerr)
				continue
			}
			if err := r.issuer.IssuePublicCert(ctx, srv.ID, cmd); err != nil {
				log.Printf("[PublicCertRenewer] (on-reconnect) cert %d → server %d failed: %v", c.ID, srv.ID, err)
			}
		}
	}
}

// normalizeCertCA 把面板库里存的 issuer 归一成 CAProvider 取值 ✓。
//
// 库里历史数据格式不一 ✗：ACME 签发存的是 `lets_encrypt` / `google` / `zerossl` ✓，
// 早期上传的存的是 `Let's Encrypt` ✗ → 这里统一成小写下划线形式 ✓。
// 认不出来的返回空串 ✓ —— 此时**不强行下发 CA** ✗（宁可让节点用默认值 ✓，
// 也不能把 Google 的证书错续成 LE ✗），同时打日志提醒 ✓。
func normalizeCertCA(issuer string) string {
	v := strings.ToLower(strings.TrimSpace(issuer))
	v = strings.ReplaceAll(v, "'", "")
	v = strings.ReplaceAll(v, " ", "_")
	switch {
	case v == "":
		return ""
	case strings.Contains(v, "lets_encrypt") || strings.Contains(v, "letsencrypt"):
		if strings.Contains(v, "staging") {
			return "lets_encrypt_staging"
		}
		return "lets_encrypt"
	case strings.Contains(v, "zerossl"):
		return "zerossl"
	case strings.Contains(v, "google"):
		return "google"
	default:
		log.Printf("[PublicCertRenewer] 无法从 issuer=%q 判断 CA ✗ → 不下发 CA（节点用自己的默认值 ✓）", issuer)
		return ""
	}
}
