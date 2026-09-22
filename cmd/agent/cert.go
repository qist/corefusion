package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"corefusion/internal/runtime/cert"
	certsvc "corefusion/internal/service/certificate"
)

// certSubcommandUsage 是 `corefusion-agent cert` 的帮助文本。
const certSubcommandUsage = `CoreFusion Agent — 证书签发（在**节点本机**执行 ACME）

为什么要在节点上签发：面板常位于 SNAT 后面（只有出站、没有入站），
公信 CA 无法回连面板完成 HTTP-01 验证；而节点自己有公网/端口映射，能完成验证。

用法：
  corefusion-agent cert issue -san <IP或域名>[,<...>] -email <邮箱> [选项]
  corefusion-agent cert info  -cert <证书路径>          # 只看已有证书的 SAN / 有效期 / 签发者

issue 选项：
  -san            必填；多个用逗号分隔。**IP 与域名都可**（IP 证书由 LE 2026-01-15 起 GA 支持，
                  但**必须是公网可达的 IP**；私网 IP 任何公信 CA 都签不了）
  -email          必填；ACME 账号邮箱（LE 必需）
  -challenge      http-01（默认，需 80 端口可被公网访问）或 dns-01（无需入站 ✓ 可穿防火墙 ✓）
  -ca             lets_encrypt（默认）| lets_encrypt_staging（测试用，不受浏览器信任但流程一致）
    -webroot        http-01 用：把挑战文件写进该目录（80 端口已被 nginx/caddy 占用时用；
                  你只需在站点里放行 /.well-known/acme-challenge/，无需停服务）
  -dns-provider   dns-01 用：cloudflare|alidns|dnspod|route53|google|godaddy|namecheap
  -dns-env        dns-01 用：KEY=VALUE，可重复（如 -dns-env CF_DNS_API_TOKEN=xxx）
  -profile        ACME profile；**IP 证书会自动用 shortlived**（LE 策略要求短时证书，约 6 天）
  -data-dir       产物目录（默认 ./acme-certs）→ 证书落在 <dir>/certs/<第一个SAN>/
  -timeout        整体超时（默认 5m）

示例：
  # 公网 IP 证书（先 staging 验证流程，再换 lets_encrypt 正式签）
  corefusion-agent cert issue -san 1.2.3.4 -email admin@example.com -ca lets_encrypt_staging

  # 域名 + DNS-01（无需入站）
  corefusion-agent cert issue -san sub.example.com -email admin@example.com \
      -challenge dns-01 -dns-provider cloudflare -dns-env CF_DNS_API_TOKEN=xxxx
`

// runCertSubcommand 处理 `corefusion-agent cert ...`（不需要面板配置 ✓）。
func runCertSubcommand(args []string) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Print(certSubcommandUsage)
		return 0
	}

	switch args[0] {
	case "issue":
		return runCertIssue(args[1:])
	case "info":
		return runCertInfo(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown cert subcommand: %s\n\n", args[0])
		fmt.Print(certSubcommandUsage)
		return 2
	}
}

// repeatedFlag 支持 `-dns-env A=B -dns-env C=D` 这种重复参数。
type repeatedFlag []string

func (r *repeatedFlag) String() string { return strings.Join(*r, ",") }
func (r *repeatedFlag) Set(v string) error {
	*r = append(*r, v)
	return nil
}

func runCertIssue(args []string) int {
	fs := flag.NewFlagSet("cert issue", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var (
		sans        = fs.String("san", "", "证书 SAN：IP 或域名，多个用逗号分隔（必填）")
		email       = fs.String("email", "", "ACME 账号邮箱（必填）")
		challenge   = fs.String("challenge", "http-01", "http-01 | dns-01")
		caProvider  = fs.String("ca", "lets_encrypt", "lets_encrypt | lets_encrypt_staging")
		dnsProvider = fs.String("dns-provider", "", "dns-01 使用的 DNS 提供商")
		profile     = fs.String("profile", "", "ACME profile（IP 证书自动使用 shortlived）")
		webroot     = fs.String("webroot", "", "http-01 用：把挑战文件写进该 webroot（80 被 nginx/caddy 占用时用）")
		dataDir     = fs.String("data-dir", "./acme-certs", "产物目录")
		timeout     = fs.Duration("timeout", 5*time.Minute, "整体超时")
	)
	var dnsEnv repeatedFlag
	fs.Var(&dnsEnv, "dns-env", "dns-01 的凭据：KEY=VALUE（可重复）")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	domains := splitSANs(*sans)
	if len(domains) == 0 {
		fmt.Fprintln(os.Stderr, "✗ -san 必填（IP 或域名，多个用逗号分隔）")
		return 2
	}
	if strings.TrimSpace(*email) == "" {
		fmt.Fprintln(os.Stderr, "✗ -email 必填（Let's Encrypt 要求 ACME 账号邮箱）")
		return 2
	}

	// 复用面板的校验：公网 IP 放行 ✓、私网/保留地址拒绝并说明原因 ✓、通配符必须 dns-01 ✓
	if err := certsvc.ValidateACMEDomains(domains, *challenge); err != nil {
		fmt.Fprintf(os.Stderr, "✗ %v\n", err)
		return 1
	}

	timeoutDur := *timeout
	if timeoutDur <= 0 {
		timeoutDur = 5 * time.Minute
	}
	issued, err := issueACMECert(acmeIssueParams{
		Identifiers: domains,
		Email:       *email,
		Challenge:   *challenge,
		CA:          *caProvider,
		Profile:     *profile,
		Webroot:     *webroot,
		DataDir:     *dataDir,
		DNSProvider: *dnsProvider,
		DNSEnvVars:  dnsEnvMap(dnsEnv),
		Timeout:     timeoutDur,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ 签发失败：%v\n", err)
		printCertHint(domains)
		return 1
	}

	result := issued.Result
	dir := joinPath(*dataDir, result.CertPath)
	fmt.Printf("✓ 签发成功\n")
	fmt.Printf("  证书：%s\n  私钥：%s\n  全链：%s\n", dir, joinPath(*dataDir, result.KeyPath), joinPath(*dataDir, result.FullchainPath))
	fmt.Printf("  到期：%s（剩余 %s）\n", result.ExpiresAt.Format(time.RFC3339), time.Until(result.ExpiresAt).Truncate(time.Minute))
	if time.Until(result.ExpiresAt) < 10*24*time.Hour {
		fmt.Printf("  ⚠ 这是**短时证书**（LE 的 IP 证书约 6 天）→ 必须在到期前自动续，否则会失效 ✗\n")
	}
	// 自检：把签发结果解析一遍，确认 SAN 真的包含申请的 IP/域名 ✓（"签出来了"和"签对了"是两件事）
	if err := printCertSANs(dir); err != nil {
		fmt.Fprintf(os.Stderr, "  ⚠ 自检解析失败：%v\n", err)
		return 1
	}
	return 0
}

func runCertInfo(args []string) int {
	fs := flag.NewFlagSet("cert info", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	certPath := fs.String("cert", "", "证书文件路径（必填）")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *certPath == "" {
		fmt.Fprintln(os.Stderr, "✗ -cert 必填")
		return 2
	}
	if err := printCertSANs(*certPath); err != nil {
		fmt.Fprintf(os.Stderr, "✗ %v\n", err)
		return 1
	}
	return 0
}

// printCertSANs 打印证书的 subject / SAN / 签发者 / 有效期（自检用）。
func printCertSANs(path string) error {
	pemBytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	info, err := cert.ParseCertificateInfo(pemBytes)
	if err != nil {
		return err
	}
	fmt.Printf("  Subject：%s\n", info.Subject)
	fmt.Printf("  签发者：%s\n", info.Issuer)
	fmt.Printf("  DNS SAN：%v\n", info.DNSNames)
	fmt.Printf("  IP  SAN：%v\n", info.IPAddresses)
	fmt.Printf("  有效期：%s → %s\n", info.NotBefore.Format(time.RFC3339), info.NotAfter.Format(time.RFC3339))
	return nil
}

// printCertHint 在失败时给出最可能的下一步（SNAT/私网 IP/端口占用是最常见原因）。
func printCertHint(sans []string) {
	for _, s := range sans {
		if ip := net.ParseIP(strings.TrimSpace(s)); ip != nil && !certsvc.IsPubliclyReachableIP(ip) {
			fmt.Fprintf(os.Stderr, "  提示：%s 是私网/保留地址，公信 CA 无法验证它 ✗ → 内部互信请用面板内置 CA，或改用公网 IP/域名\n", s)
			return
		}
	}
	fmt.Fprintln(os.Stderr, "  提示：http-01 需要该 IP/域名的 80 端口能被公网直连（SNAT/防火墙后不行 ✗）→")
	fmt.Fprintln(os.Stderr, "        要么放通 80/443 入站并确保本机 80 端口空闲，要么改用 dns-01（-challenge dns-01 -dns-provider ...）")
}

func splitSANs(s string) []string {
	out := make([]string, 0, 4)
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// dnsEnvMap 把 -dns-env KEY=VALUE 列表转成 map ✓。
func dnsEnvMap(kvs []string) map[string]string {
	out := map[string]string{}
	for _, kv := range kvs {
		if k, v, ok := strings.Cut(kv, "="); ok && strings.TrimSpace(k) != "" {
			out[strings.TrimSpace(k)] = v
		}
	}
	return out
}
