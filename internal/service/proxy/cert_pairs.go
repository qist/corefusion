package proxy

import (
	"context"
	"log"
	"strings"

	"corefusion/internal/store/repository"
)

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
	domain, siteCert, siteKey string, dualEnabled bool) []certPair {
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

	add(siteCert, siteKey)
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
		add(certs[i].CertPath, certs[i].KeyPath)
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
