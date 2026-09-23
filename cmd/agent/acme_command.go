package main

import (
	"log"
	"strings"
	"time"

	agentpb "corefusion/internal/runtime/agent/proto"
)

// handleIssuePublicCert 处理面板下发的"在节点本机签发公网证书"指令 ✓。
//
// 为什么由 Agent 做 ✗：面板常在 SNAT 后面，CA 无法回调它的 80 端口做 http-01 验证 ✗；
// 而节点有公网 IP 与 80 入站 ✓。签完把 PEM **回传给面板** ✓（面板负责落库/落盘 ✓），
// 于是"签发 → 续期 → 上传"整条链在节点侧自动闭环 ✓。
func (a *AgentApp) handleIssuePublicCert(cmd *agentpb.IssuePublicCertCommand, send func(*agentpb.AgentReport)) {
	report := &agentpb.PublicCertIssuedReport{
		CertId:      cmd.CertId,
		Identifiers: cmd.Identifiers,
	}
	if cmd == nil || len(cmd.Identifiers) == 0 {
		report.Error = "no identifiers"
		send(a.publicCertReport(report))
		return
	}

	// 全部来自面板指令 ✓（节点侧零配置 ✗）：邮箱/CA 来自面板的 ACME 账号 ✓，
	// webroot 只有当面板明确给了才用 ✓（面板自己生成的 web 配置已经会处理挑战路径 ✓）。
	dataDir := strings.TrimSpace(cmd.DataDir)
	if dataDir == "" {
		dataDir = strings.TrimSuffix(a.cfg.Agent.DataDir, "/") + "/acme"
	}
	params := acmeIssueParams{
		Identifiers: cmd.Identifiers,
		Email:       strings.TrimSpace(cmd.Email),
		Challenge:   firstNonEmpty(cmd.Challenge, "http-01"),
		CA:          strings.TrimSpace(cmd.Ca),
		Profile:     cmd.Profile,
		Webroot:     strings.TrimSpace(cmd.Webroot),
		DataDir:     dataDir,
		DNSProvider: cmd.DnsProvider,
		DNSEnvVars:  cmd.DnsEnvVars,
		// 面板按 CA 自动关联好的 EAB ✓ → 节点"首次申请即自动注册" ✓
		EABKeyID:  strings.TrimSpace(cmd.GetEabKeyId()),
		EABMACKey: strings.TrimSpace(cmd.GetEabMacKey()),
		Timeout:   5 * time.Minute,
	}
	if params.Email == "" {
		// 面板必须在指令里带上邮箱（来自面板的 ACME 账号 ✓）——没有就不能签 ✗
		report.Error = "panel did not supply an ACME account email"
		send(a.publicCertReport(report))
		return
	}

	issued, err := issueACMECert(params)
	if err != nil {
		log.Printf("[Agent] public cert issue failed for %v: %v", cmd.Identifiers, err)
		report.Error = err.Error()
		send(a.publicCertReport(report))
		return
	}

	report.Success = true
	report.CertPem = issued.CertPEM
	report.KeyPem = issued.KeyPEM
	report.FullchainPem = issued.FullchainPEM
	report.ExpiresAt = issued.Result.ExpiresAt.Unix()
	report.Ca = string(params.CA)
	log.Printf("[Agent] public cert issued for %v (expires %s), reporting to panel",
		cmd.Identifiers, issued.Result.ExpiresAt.Format(time.RFC3339))
	send(a.publicCertReport(report))
}

// publicCertReport 包装成 AgentReport ✓（面板据 Type=PUBLIC_CERT_ISSUED 落库 ✓）。
func (a *AgentApp) publicCertReport(r *agentpb.PublicCertIssuedReport) *agentpb.AgentReport {
	return &agentpb.AgentReport{
		Type:             agentpb.AgentReport_PUBLIC_CERT_ISSUED,
		ServerId:         a.cfg.Panel.ServerID,
		Timestamp:        time.Now().Unix(),
		PublicCertIssued: r,
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func parseTimeout(s string) time.Duration {
	if d, err := time.ParseDuration(strings.TrimSpace(s)); err == nil && d > 0 {
		return d
	}
	return 5 * time.Minute
}
