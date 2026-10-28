package proxy

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"

	"corefusion/internal/store/repository"
)

// certPairChecker 判断一组证书文件**此刻在本地是否真的可用** ✓（nil = 不检查 ✓）。
type certPairChecker func(cert, key string) bool

// localCertPairChecker 造一个"本地文件存在性"检查器 ✓（路径相对 dataDir ✓，绝对路径直接用 ✓）。
//
// ⚠ 只在为**本地**服务器生成配置时使用 ✗：
//   - Caddy 加载配置时，`tls` 指向的文件**读不到**会让**整个配置加载失败** ✗✗
//     （实测 `caddy validate` 报 `unable to open ...` exit=1 ✓）—— 不只是少一张证书 ✗，
//     而是整个代理停在旧配置上 ✗。所以库里"状态是 active 但文件已被删/搬走"的脏记录
//     绝不能写进配置 ✗。
//   - 远程节点的证书文件由分发器同步 ✓，在面板侧 stat 必然报缺失 ✗ → 会把好证书误删 ✗✗
//     因此远程**刻意不检查** ✓（信任库状态 ✓）。
func localCertPairChecker(dataDir string) certPairChecker {
	return func(cert, key string) bool {
		for _, p := range []string{cert, key} {
			if strings.TrimSpace(p) == "" {
				return false
			}
			abs := p
			if !filepath.IsAbs(abs) {
				abs = filepath.Join(dataDir, abs)
			}
			fi, err := os.Stat(abs)
			if err != nil || fi.IsDir() {
				return false
			}
		}
		return true
	}
}

// certPair 是代理要用的一组「证书 + 私钥」（路径相对 dataDir ✓）。
type certPair struct {
	Cert string
	Key  string
}

// maxCertPairs 是同一站点最多挂几组证书 ✓。
//
// 代理（Caddy / nginx）会**按客户端支持的能力自动挑一组** ✓（RFC 8446/5246 的
// certificate selection ✓），所以 ECC + RSA 两组就够 ✓；再多没有意义 ✗
// —— 同一密钥类型只会有一组真正生效 ✓（记录身份本来就是 (域名, 密钥类型) ✓）。
const maxCertPairs = 2

// collectCertPairs 收集某域名要挂到代理上的**所有**证书组 ✓（最多 maxCertPairs 组 ✓）。
//
// 为什么需要它 ✗：生成器以前只取"该域名第一条带证书的站点"就 break ✓
// → 于是「同时签发 ECC + RSA」虽然两张都签出来了 ✓，站点配置里却只挂了一张 ✗（另一张白签 ✗）。
//
// 规则 ✓：
//  1. 站点自己配的那组 (siteCert, siteKey) 永远排第一 ✓ —— 保持既有行为 ✓，已有引用不断 ✗；
//  2. 再从 certificates 表按域名补齐其余 **active** 证书 ✓（ECC + RSA 各一组 ✓）；
//  3. 按 (cert,key) 去重 ✓（站点配的与表里同一条时不会写两遍 ✓）。
//
// ⚠ certRepo 为 nil 时安全退化 ✓：只用站点自己那一组 ✓（不崩、行为与改动前一致 ✓）。
// dualEnabled 是**站点级开关** ✓（caddy_sites/nginx_sites 的 dual_cert ✓，nil 视为 true ✓）：
// 显式关掉时**只挂站点自己那对** ✓。
func collectCertPairs(ctx context.Context, certRepo *repository.CertificateRepository,
	domain, siteCert, siteKey string, dualEnabled bool, checker certPairChecker) []certPair {
	pairs := make([]certPair, 0, maxCertPairs)
	seen := make(map[string]bool)
	add := func(cert, key string) {
		cert, key = strings.TrimSpace(cert), strings.TrimSpace(key)
		if cert == "" || key == "" || len(pairs) >= maxCertPairs {
			return
		}
		id := cert + "\x00" + key
		if seen[id] {
			return
		}
		seen[id] = true
		pairs = append(pairs, certPair{Cert: cert, Key: key})
	}

	// ⚠ 站点自配的那组**也要检查** ✗：它的文件不在时，写进配置**同样**会让整个代理配置
	// 加载失败 ✗✗（实测 `caddy validate` exit=1 ✓）→ 跳过它，让下面按域名补的其他证书顶上 ✓
	// （站点至少还有一张能用 ✓；实在没有才退化成没有 tls ✓）。
	if checker != nil && !checker(siteCert, siteKey) && strings.TrimSpace(siteCert) != "" {
		log.Printf("[proxy] WARNING: 站点自配的证书文件不在本地，**已跳过**（写进配置会让整个代理加载失败 ✗）：%s", siteCert)
	} else {
		add(siteCert, siteKey)
	}
	// ⚠ 护栏 ✗：站点自己**没配证书**（例如纯 HTTP 的 `example.com:80` ✓）时不做
	// "按域名补证书" ✓ —— 否则会凭空给纯 HTTP 站点加上 tls 指令 ✗。
	if strings.TrimSpace(siteCert) == "" {
		return pairs
	}
	if !dualEnabled || certRepo == nil || strings.TrimSpace(domain) == "" {
		return pairs
	}
	// domain 可能带端口（如 panel.example.com:8443 ✓）→ 库里存的是纯域名 ✓
	host := StripPort(domain)
	certs, err := certRepo.ListByDomain(ctx, host)
	if err != nil {
		log.Printf("[proxy] WARNING: 查询 %s 的证书列表失败（本次只用站点自配的那组）：%v", host, err)
		return pairs
	}
	for i := range certs {
		c, k := certs[i].CertPath, certs[i].KeyPath
		if checker != nil && !checker(c, k) {
			// 跳过比"写进配置让整体加载失败"好得多 ✓（站点还能用剩下那张 ✓）
			log.Printf("[proxy] WARNING: %s 的证书文件不在本地，**已跳过**（写进配置会让整个代理加载失败 ✗）：%s", host, c)
			continue
		}
		add(c, k)
	}
	if len(pairs) > 1 {
		names := make([]string, 0, len(pairs))
		for _, p := range pairs {
			names = append(names, p.Cert)
		}
		log.Printf("[proxy] %s 挂**双证书**（%d 组）：%s ✓", host, len(pairs), strings.Join(names, " + "))
	}
	return pairs
}

// collectCertPairsForDomains 为一个站点绑定的多个域名分别收集证书并去重。
// 一个站点绑多域名时，证书可能是「一个 SAN 证书覆盖全部」也可能是「每域名各一张」——
// 全部挂上后 nginx/Caddy 会按 SNI 自动挑对应的一张。
func collectCertPairsForDomains(ctx context.Context, certRepo *repository.CertificateRepository,
	domains []string, siteCert, siteKey string, dualEnabled bool, checker certPairChecker) []certPair {
	if len(domains) == 0 {
		return collectCertPairs(ctx, certRepo, "", siteCert, siteKey, dualEnabled, checker)
	}
	out := make([]certPair, 0, maxCertPairs)
	seen := make(map[string]bool)
	for _, d := range domains {
		for _, cp := range collectCertPairs(ctx, certRepo, d, siteCert, siteKey, dualEnabled, checker) {
			if len(out) >= maxCertPairs {
				return out
			}
			id := cp.Cert + "\x00" + cp.Key
			if seen[id] {
				continue
			}
			seen[id] = true
			out = append(out, cp)
		}
	}
	return out
}
