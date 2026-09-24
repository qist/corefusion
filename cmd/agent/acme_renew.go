package main

import (
	"context"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"corefusion/internal/runtime/cert"

	agentpb "corefusion/internal/runtime/agent/proto"
)

// 节点自续期的阈值（**比面板晚一步** ✓，这是刻意的 ✓）：
//
//	面板侧：IP = 3 天 / 域名 = 20 天（`internal/runtime/agent/public_cert_renewer.go` ✓）
//	节点侧：IP = 2 天 / 域名 = 10 天（本文件 ✓）
//
// 为什么不直接一样 ✗：两边同时续会**重复下单** ✗（LE 的"同名证书"配额是 5 份/周 ✓，
// IP 证书只有 ~6 天寿命 ✓、续得频繁 ✓ → 很容易撞上限 ✗）。
// 所以正常情况下面板先续 ✓（20 天/3 天 ✓），节点只在"面板没续成"时兜底 ✓
// （面板不可达 ✓、面板没记录 ✓、面板此前压根没托管这张 ✓）→ 证书不会等到过期才动 ✗。
const (
	agentIPRenewBefore     = 2 * 24 * time.Hour
	agentDomainRenewBefore = 10 * 24 * time.Hour
)

// agentRenewInterval 是自续期的检查间隔 ✓（6 小时 ✓：IP 证书也只有 ~6 天寿命 ✓，
// 留出足够重试次数 ✓）。
const agentRenewInterval = 6 * time.Hour

// agentRenewFirstDelay 是启动后首次检查的延迟 ✓（避开启动时与配置同步抢资源 ✓）。
const agentRenewFirstDelay = 90 * time.Second

// agentRenewThreshold 按标识符类型给阈值 ✓（含 IP → 按短时证书 ✓，与面板同口径 ✓）。
func agentRenewThreshold(identifiers []string) time.Duration {
	for _, id := range identifiers {
		if net.ParseIP(strings.TrimSpace(id)) != nil {
			return agentIPRenewBefore
		}
	}
	return agentDomainRenewBefore
}

// needsRenewAt 判断"该不该续" ✓（纯函数 ✓ 便于单测 ✓）。
func needsRenewAt(now, expiry time.Time, identifiers []string) bool {
	if expiry.IsZero() {
		return true // 解析不到到期时间 → 宁可信其快过期 ✓（续一次总比断线好 ✓）
	}
	return expiry.Sub(now) < agentRenewThreshold(identifiers)
}

// acmeDataDir 是节点侧 ACME 数据目录 ✓（与 acme_command.go 的口径一致 ✓）：
// 里面存 ACME **账号**（每 CA 一份 ✓）+ 产物（`certs/<name>/` ✓）+ 本文件的 meta ✓。
func (a *AgentApp) acmeDataDir() string {
	if dir := strings.TrimSpace(a.cfg.Agent.DataDir); dir != "" {
		return filepath.Join(strings.TrimSuffix(dir, "/"), "acme")
	}
	return defaultAgentACMECertDir
}

// renewDueCertsOnce 扫一遍本机证书 ✓，把"快到期"的**就地续掉** ✓，返回要上报的结果 ✓。
//
// ⚠ 只处理**有 meta.json 的** ✓ —— 那才是"节点自己签的、归节点自管"的证书 ✓；
// 面板分发给节点的证书（`cert_distributor` 写进来的 ✓）没有 meta ✓ → **跳过** ✓：
// 那种证书的所有权在面板 ✓（面板负责续 ✓），节点擅自续会绕开面板的管理口径 ✗。
func (a *AgentApp) renewDueCertsOnce(ctx context.Context) []*agentpb.PublicCertIssuedReport {
	dataDir := a.acmeDataDir()
	names, err := listLocalCertDirs(dataDir)
	if err != nil {
		log.Printf("[Agent] cert self-renew: list %s failed: %v", dataDir, err)
		return nil
	}
	if len(names) == 0 {
		return nil
	}

	var reports []*agentpb.PublicCertIssuedReport
	for _, name := range names {
		if ctx.Err() != nil {
			return reports
		}
		meta, merr := readCertMeta(dataDir, name)
		if merr != nil {
			if !os.IsNotExist(merr) {
				log.Printf("[Agent] cert self-renew: read meta for %s failed: %v", name, merr)
			}
			continue // 无 meta = 不归节点自管 ✓（例如面板下发的 ✓）
		}

		// ⚠ 按 meta 里的**密钥类型**定位 ✗：RSA 在 `certs/<name>/rsa/cert.pem` ✓，
		// 写死 `certs/<name>/cert.pem` 会读不到 ✗ → RSA 证书在节点侧永不自续 ✗。
		certPath := localCertPathFor(dataDir, name, meta.KeyType)
		pemBytes, rerr := os.ReadFile(certPath)
		if rerr != nil {
			log.Printf("[Agent] cert self-renew: read %s failed: %v", certPath, rerr)
			continue
		}
		info, perr := cert.ParseCertificateInfo(pemBytes)
		if perr != nil {
			log.Printf("[Agent] cert self-renew: parse %s failed: %v", certPath, perr)
			continue
		}

		// 标识符：优先用**证书自身的 SAN** ✓（最可信 ✓），meta 只作兜底 ✓。
		ids := certIdentifiersFromInfo(info)
		if len(ids) == 0 {
			ids = meta.Identifiers
		}
		if !needsRenewAt(time.Now(), info.NotAfter, ids) {
			continue
		}

		log.Printf("[Agent] cert self-renew: %s 将于 %s 到期（剩余 %s < 阈值 %s）→ 由节点自行续期 ✓",
			name, info.NotAfter.Format(time.RFC3339), time.Until(info.NotAfter).Truncate(time.Second), agentRenewThreshold(ids))

		meta.Identifiers = ids
		issued, ierr := issueACMECert(meta.issueParams(a, dataDir))
		if ierr != nil {
			// 续期失败**不影响下一轮** ✓（下一轮还会再试 ✓）；也绝不删掉现有证书 ✗（现有证书还能用 ✓）。
			log.Printf("[Agent] cert self-renew failed for %s: %v", name, ierr)
			reports = append(reports, &agentpb.PublicCertIssuedReport{
				CertId:      meta.CertID,
				Identifiers: ids,
				Success:     false,
				Error:       ierr.Error(),
				// 失败也要带挑战方式 ✓：面板据此知道"这条记录该按什么续" ✓
				Challenge:    meta.Challenge,
				DnsChannelId: meta.DNSChannelID,
			})
			continue
		}

		log.Printf("[Agent] cert self-renew OK: %s 新到期时间 %s", name, issued.Result.ExpiresAt.Format(time.RFC3339))
		reports = append(reports, &agentpb.PublicCertIssuedReport{
			CertId:       meta.CertID,
			Identifiers:  ids,
			Success:      true,
			CertPem:      issued.CertPEM,
			KeyPem:       issued.KeyPEM,
			FullchainPem: issued.FullchainPEM,
			ExpiresAt:    issued.Result.ExpiresAt.Unix(),
			Ca:           meta.CA,
			// 同上：挑战方式 + 面板通道 id 原样带回 ✓（面板落库保持"创建时的选择" ✓）
			Challenge:    meta.Challenge,
			DnsChannelId: meta.DNSChannelID,
		})
	}
	return reports
}

// certIdentifiersFromInfo 取证书的标识符（域名 + IP ✓，去重 ✓）。
func certIdentifiersFromInfo(info *cert.CertificateInfo) []string {
	if info == nil {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(info.DNSNames)+len(info.IPAddresses))
	for _, v := range append(append([]string{}, info.DNSNames...), info.IPAddresses...) {
		if v = strings.TrimSpace(v); v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

// runCertRenewLoop 是自续期的**独立 goroutine** ✓。
//
// ⚠ 为什么必须独立 ✗：一次 ACME 签发经常要 1~2 分钟 ✓（等 DNS 传播/CA 验证 ✓），
// 如果在心跳循环里同步跑 ✓ 会把这段时间的心跳饿死 ✗ → 面板把节点判成离线 ✗。
// 所以这里只"干活" ✓，结果投进 channel ✓，由心跳循环负责发（沿用 stream + streamSendMu ✓）。
func (a *AgentApp) runCertRenewLoop(ctx context.Context, out chan<- *agentpb.PublicCertIssuedReport) {
	first := time.NewTimer(agentRenewFirstDelay)
	defer first.Stop()
	ticker := time.NewTicker(agentRenewInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-first.C:
		case <-ticker.C:
		}
		for _, rep := range a.renewDueCertsOnce(ctx) {
			select {
			case out <- rep:
			case <-ctx.Done():
				return
			default:
				// 队列满：**证书已经续好了** ✓，只是这次上报丢了 ✓ → 记日志即可 ✓
				// （下一轮/下一次面板查询会重新对齐 ✓）。
				log.Printf("[Agent] cert self-renew: report queue full, dropping report for %v", rep.Identifiers)
			}
		}
	}
}
