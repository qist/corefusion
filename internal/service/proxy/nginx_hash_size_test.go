package proxy

import (
	"strings"
	"testing"

	"corefusion/internal/model/entity"
)

// 历史默认值 512/64 会让 nginx 告警并忽略 bucket_size，生成时必须抬到下限。
func TestNginxConfigProxyHeadersHashMinimum(t *testing.T) {
	sites := []entity.NginxSite{
		{ServerID: 2, Domain: "a.example.com", Template: "http", Path: "/",
			Upstream: "127.0.0.1:8080", Enabled: true},
	}
	cfg := generateFor(t, newNginxTestManager(t, sites,
		`{"proxy_headers_hash_max_size":"512","proxy_headers_hash_bucket_size":"64"}`))
	if !strings.Contains(cfg, "proxy_headers_hash_max_size 2048;") {
		t.Fatalf("max_size 未抬到 2048：\n%s", firstLines(cfg, 90))
	}
	if !strings.Contains(cfg, "proxy_headers_hash_bucket_size 128;") {
		t.Fatalf("bucket_size 未抬到 128：\n%s", firstLines(cfg, 90))
	}
}

// 用户显式配得更大时保持原值，不覆盖。
func TestNginxConfigProxyHeadersHashKeepsLarger(t *testing.T) {
	sites := []entity.NginxSite{
		{ServerID: 2, Domain: "a.example.com", Template: "http", Path: "/",
			Upstream: "127.0.0.1:8080", Enabled: true},
	}
	cfg := generateFor(t, newNginxTestManager(t, sites,
		`{"proxy_headers_hash_max_size":"8192","proxy_headers_hash_bucket_size":"256"}`))
	if !strings.Contains(cfg, "proxy_headers_hash_max_size 8192;") {
		t.Fatalf("应保留 8192：\n%s", firstLines(cfg, 90))
	}
	if !strings.Contains(cfg, "proxy_headers_hash_bucket_size 256;") {
		t.Fatalf("应保留 256：\n%s", firstLines(cfg, 90))
	}
}

// 空值/非法值按下限处理，不能生成 "proxy_headers_hash_max_size ;"。
func TestNginxConfigProxyHeadersHashInvalid(t *testing.T) {
	sites := []entity.NginxSite{
		{ServerID: 2, Domain: "a.example.com", Template: "http", Path: "/",
			Upstream: "127.0.0.1:8080", Enabled: true},
	}
	cfg := generateFor(t, newNginxTestManager(t, sites,
		`{"proxy_headers_hash_max_size":"","proxy_headers_hash_bucket_size":"abc"}`))
	if !strings.Contains(cfg, "proxy_headers_hash_max_size 2048;") ||
		!strings.Contains(cfg, "proxy_headers_hash_bucket_size 128;") {
		t.Fatalf("非法值未回落到下限：\n%s", firstLines(cfg, 90))
	}
}

func firstLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
