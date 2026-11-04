package proxy

import (
	"regexp"
	"strings"
	"testing"

	"corefusion/internal/model/entity"
)

// 多个 stream 监听地址时，每组都会生成 upstream/map；
// 名字重复会让 nginx -t 直接失败（duplicate upstream "direct_http"）。
func TestNginxConfigMultiListenSNINoDuplicateNames(t *testing.T) {
	sites := []entity.NginxSite{
		{ServerID: 2, Domain: "a.example.com", Template: "sni", Path: "192.0.2.1:443", SNITarget: "127.0.0.1:9001", Enabled: true},
		{ServerID: 2, Domain: "b.example.com", Template: "sni", Path: "192.0.2.1:8443", SNITarget: "127.0.0.1:9002", Enabled: true},
		{ServerID: 2, Domain: "fallback", Template: "sni", Path: "192.0.2.1:443", SNITarget: "127.0.0.1:9003", Enabled: true},
	}
	cfg := generateFor(t, newNginxTestManager(t, sites,
		`{"stream_sni_enabled": true, "stream_nontls_target": "127.0.0.1:8444"}`))

	upstreamRe := regexp.MustCompile(`(?m)^upstream\s+(\S+)\s*\{`)
	seen := map[string]int{}
	for _, m := range upstreamRe.FindAllStringSubmatch(cfg, -1) {
		seen[m[1]]++
	}
	for name, n := range seen {
		if n > 1 {
			t.Fatalf("upstream %q 重复定义 %d 次：\n%s", name, n, cfg)
		}
	}

	mapRe := regexp.MustCompile(`(?m)^map\s+(\S+)\s+(\S+)\s*\{`)
	seenMap := map[string]int{}
	for _, m := range mapRe.FindAllStringSubmatch(cfg, -1) {
		seenMap[m[2]]++
	}
	for name, n := range seenMap {
		if n > 1 {
			t.Fatalf("map 目标变量 %q 重复定义 %d 次：\n%s", name, n, cfg)
		}
	}

	// 两个 server 必须各自引用自己那一组的 guard 变量
	if strings.Count(cfg, "proxy_pass $sni_backend_guarded_") != 2 {
		t.Fatalf("每个 listen 组应引用各自的 guard map：\n%s", cfg)
	}
	if !strings.Contains(cfg, "listen 192.0.2.1:443 ") || !strings.Contains(cfg, "listen 192.0.2.1:8443 ") {
		t.Fatalf("缺少多 listen：\n%s", cfg)
	}
}
