package proxy

import (
	"context"
	"testing"

	"corefusion/internal/model/entity"
)

// TestNginxIsLocalTargetIgnoresSharedIsLocal 锁定：判断「配置落到哪个节点」必须按 serverID 参数，
// 绝不能依赖会被每请求改写的共享状态 m.isLocal（与 CaddyManager 对称）。
func TestNginxIsLocalTargetIgnoresSharedIsLocal(t *testing.T) {
	m := &NginxManager{}
	m.SetIsLocal(false) // 模拟「启动时 / 被并发请求改写」的状态
	if !m.isLocalTarget(context.Background(), 1) {
		t.Fatal("serverID=1（本机面板）应始终判为本机，与 m.isLocal 无关")
	}
}

// TestNginxFillForwardDerivedFields 锁定 portal_forward / subscription_forward 的派生字段规则，
// 保证创建/更新/生成三处落地一致（DB 里 upstream/path 不再为空）。与 CaddyManager 对称。
func TestNginxFillForwardDerivedFields(t *testing.T) {
	ctx := context.Background()

	// 有独立门户端口 → 整域名直通，path 留空
	m := &NginxManager{settingStore: fakeSettingStore{m: map[string]string{
		"panel.portal_port": "8082",
		"panel.listen_port": "8080",
	}}}
	pf := &entity.NginxSite{Template: "portal_forward"}
	m.fillForwardDerivedFields(ctx, pf)
	if pf.Upstream != "127.0.0.1:8082" || pf.Path != "" {
		t.Fatalf("portal_forward: upstream=%q path=%q, want 127.0.0.1:8082 / empty", pf.Upstream, pf.Path)
	}

	// 无独立门户端口 → 挂 /p，转发面板主端口
	m2 := &NginxManager{settingStore: fakeSettingStore{m: map[string]string{
		"panel.listen_port": "8080",
	}}}
	pf2 := &entity.NginxSite{Template: "portal_forward"}
	m2.fillForwardDerivedFields(ctx, pf2)
	if pf2.Upstream != "127.0.0.1:8080" || pf2.Path != "/p" {
		t.Fatalf("portal_forward(no portal port): upstream=%q path=%q, want 127.0.0.1:8080 / /p", pf2.Upstream, pf2.Path)
	}

	// subscription_forward → 面板主端口 + /sub
	sf := &entity.NginxSite{Template: "subscription_forward"}
	m.fillForwardDerivedFields(ctx, sf)
	if sf.Upstream != "127.0.0.1:8080" || sf.Path != "/sub" {
		t.Fatalf("subscription_forward: upstream=%q path=%q, want 127.0.0.1:8080 / /sub", sf.Upstream, sf.Path)
	}
}
