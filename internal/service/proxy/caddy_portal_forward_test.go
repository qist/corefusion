package proxy

import (
	"context"
	"testing"

	"corefusion/internal/model/entity"
)

// fakeSettingStore 是 SettingStore 的内存实现，仅用于派生字段解析测试。
type fakeSettingStore struct{ m map[string]string }

func (f fakeSettingStore) GetByKey(_ context.Context, key string) (string, error) {
	return f.m[key], nil
}

func (f fakeSettingStore) Set(_ context.Context, _, _ string) error { return nil }

// TestCaddyIsLocalTargetIgnoresSharedIsLocal 锁定：判断「配置落到哪个节点」必须按 serverID 参数，
// 绝不能依赖会被每请求改写的共享状态 m.isLocal。
//
// Why: 面板启动时只 SetServerID(1)、不设 m.isLocal（默认 false）。旧逻辑用 m.isLocal 判断
// → 启动生成的 Caddyfile 会**跳过 portal_forward 站点**（门户白屏），直到某次本机请求把它置回 true。
func TestCaddyIsLocalTargetIgnoresSharedIsLocal(t *testing.T) {
	m := &CaddyManager{}
	m.SetIsLocal(false) // 模拟「启动时 / 被并发请求改写」的状态
	if !m.isLocalTarget(context.Background(), 1) {
		t.Fatal("serverID=1（本机面板）应始终判为本机，与 m.isLocal 无关")
	}
}

// TestCaddyFillForwardDerivedFields 锁定 portal_forward / subscription_forward 的派生字段规则，
// 保证创建/更新/生成三处落地一致（DB 里 upstream/path 不再为空）。
func TestCaddyFillForwardDerivedFields(t *testing.T) {
	ctx := context.Background()

	// 有独立门户端口 → 整域名直通，path 留空
	m := &CaddyManager{settingStore: fakeSettingStore{m: map[string]string{
		"panel.portal_port": "8082",
		"panel.listen_port": "8080",
	}}}
	pf := &entity.CaddySite{Template: "portal_forward"}
	m.fillForwardDerivedFields(ctx, pf)
	if pf.Upstream != "127.0.0.1:8082" || pf.Path != "" {
		t.Fatalf("portal_forward: upstream=%q path=%q, want 127.0.0.1:8082 / empty", pf.Upstream, pf.Path)
	}

	// 无独立门户端口 → 挂 /p，转发面板主端口
	m2 := &CaddyManager{settingStore: fakeSettingStore{m: map[string]string{
		"panel.listen_port": "8080",
	}}}
	pf2 := &entity.CaddySite{Template: "portal_forward"}
	m2.fillForwardDerivedFields(ctx, pf2)
	if pf2.Upstream != "127.0.0.1:8080" || pf2.Path != "/p" {
		t.Fatalf("portal_forward(no portal port): upstream=%q path=%q, want 127.0.0.1:8080 / /p", pf2.Upstream, pf2.Path)
	}

	// 无独立订阅端口 → 面板主端口 + /sub
	sf := &entity.CaddySite{Template: "subscription_forward"}
	m.fillForwardDerivedFields(ctx, sf)
	if sf.Upstream != "127.0.0.1:8080" || sf.Path != "/sub" {
		t.Fatalf("subscription_forward: upstream=%q path=%q, want 127.0.0.1:8080 / /sub", sf.Upstream, sf.Path)
	}

	// 有独立订阅端口 → 独立订阅引擎在根路径服务 → 整域名直通，path 留空（不能再挂 /sub，否则 404）
	m3 := &CaddyManager{settingStore: fakeSettingStore{m: map[string]string{
		"panel.subscription_port": "8083",
		"panel.listen_port":       "8080",
	}}}
	sf2 := &entity.CaddySite{Template: "subscription_forward"}
	m3.fillForwardDerivedFields(ctx, sf2)
	if sf2.Upstream != "127.0.0.1:8083" || sf2.Path != "" {
		t.Fatalf("subscription_forward(dedicated): upstream=%q path=%q, want 127.0.0.1:8083 / empty", sf2.Upstream, sf2.Path)
	}

	// 面板转发 → 面板主端口 + 根路径（upstream 由 panel.listen_port 派生）
	pf3 := &entity.CaddySite{Template: "panel_forward"}
	m3.fillForwardDerivedFields(ctx, pf3)
	if pf3.Upstream != "127.0.0.1:8080" || pf3.Path != "" {
		t.Fatalf("panel_forward: upstream=%q path=%q, want 127.0.0.1:8080 / empty", pf3.Upstream, pf3.Path)
	}
}
