package router

import (
	"testing"

	proxySvc "corefusion/internal/service/proxy"
)

// 证书刷新后的动作必须是：nginx reload（重读证书文件、不断连接），caddy restart（内存缓存）。
// 锁死这条约束，避免以后又被改回"两个都重启"，导致每次续期都断流几秒。
func TestCertRefreshAction(t *testing.T) {
	if got := certRefreshAction(proxySvc.ProxyNginx); got != "reload" {
		t.Fatalf("nginx 证书刷新应使用 reload（重读 ssl_certificate，不中断连接），got %q", got)
	}
	if got := certRefreshAction(proxySvc.ProxyCaddy); got != "restart" {
		t.Fatalf("caddy 证书缓存在内存，刷新必须 restart，got %q", got)
	}
}
