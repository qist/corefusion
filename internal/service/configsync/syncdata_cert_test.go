package configsync

import (
	"testing"

	"corefusion/internal/model/entity"
)

// TestMatchCertificateVariants 覆盖真机场景 ✓：
// 证书 name=gg / domain=example.com，规范路径是 certs/example.com/，
// 而站点配置引用的是历史目录 certs/gg/ —— 必须命中，并且按 **certs/gg/** 下发，
// 否则节点上的 certs/gg/ 永远是旧证书（2026-10-20 实测问题）。
func TestMatchCertificateVariants(t *testing.T) {
	c := entity.Certificate{
		ID:            2,
		Name:          "gg",
		Domain:        "example.com",
		KeyType:       "ecc",
		KeyBits:       384,
		CertPath:      "certs/example.com/cert.pem",
		KeyPath:       "certs/example.com/key.pem",
		FullchainPath: "certs/example.com/fullchain.pem",
	}

	// 站点只引用历史目录 → 必须产生 certs/gg/ 的落地路径 ✓
	got := matchCertificateVariants(c, map[string]bool{
		normalizeCertPath("../../../certs/gg/cert.pem"): true,
	})
	if len(got) != 1 {
		t.Fatalf("应命中 1 个别名目录，实际 %d：%+v", len(got), got)
	}
	if got[0].Dir != "certs/gg" ||
		got[0].Cert != "certs/gg/cert.pem" ||
		got[0].Key != "certs/gg/key.pem" ||
		got[0].Fullchain != "certs/gg/fullchain.pem" {
		t.Fatalf("下发路径必须是 certs/gg/ 这一套，实际 %+v", got[0])
	}

	// 规范目录同样能命中 ✓
	got = matchCertificateVariants(c, map[string]bool{
		normalizeCertPath("certs/example.com/fullchain.pem"): true,
	})
	if len(got) != 1 || got[0].Dir != "certs/example.com" {
		t.Fatalf("规范目录应命中 certs/example.com，实际 %+v", got)
	}

	// 两个目录都被引用 → 两个都要下发（两处都写新证书 ✓）
	got = matchCertificateVariants(c, map[string]bool{
		normalizeCertPath("certs/gg/cert.pem"):        true,
		normalizeCertPath("certs/example.com/cert.pem"): true,
	})
	if len(got) != 2 {
		t.Fatalf("两个目录都被引用时应返回 2 个形态，实际 %d：%+v", len(got), got)
	}

	// 无关路径不应误命中 ✓
	got = matchCertificateVariants(c, map[string]bool{
		normalizeCertPath("certs/other.com/cert.pem"): true,
	})
	if len(got) != 0 {
		t.Fatalf("无关路径不应命中，实际 %+v", got)
	}
}

// TestMatchCertificateVariantsRSA 断言 RSA 与 ECC 的别名目录不会串 ✓。
func TestMatchCertificateVariantsRSA(t *testing.T) {
	c := entity.Certificate{
		Name:          "gg",
		Domain:        "example.com",
		KeyType:       "rsa",
		CertPath:      "certs/example.com/rsa/cert.pem",
		KeyPath:       "certs/example.com/rsa/key.pem",
		FullchainPath: "certs/example.com/rsa/fullchain.pem",
	}
	// ECC 目录 certs/gg/cert.pem 不应命中 RSA 证书 ✓（否则会把 RSA 写进 ECC 目录）
	got := matchCertificateVariants(c, map[string]bool{
		normalizeCertPath("certs/gg/cert.pem"): true,
	})
	if len(got) != 0 {
		t.Fatalf("RSA 证书不应命中 ECC 目录，实际 %+v", got)
	}
	got = matchCertificateVariants(c, map[string]bool{
		normalizeCertPath("certs/gg/rsa/cert.pem"): true,
	})
	if len(got) != 1 || got[0].Dir != "certs/gg/rsa" {
		t.Fatalf("RSA 历史目录应命中 certs/gg/rsa，实际 %+v", got)
	}
}
