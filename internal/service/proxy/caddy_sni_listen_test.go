package proxy

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// newCaddyTestManager 构造一个基于内存 SQLite 的 CaddyManager，用于验证 Caddyfile 生成。
func newCaddyTestManager(t *testing.T, sites []entity.CaddySite, settings map[string]string) *CaddyManager {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("内存 SQLite 不可用（跳过）：%v", err)
	}
	if err := db.AutoMigrate(&entity.CaddySite{}); err != nil {
		t.Skipf("迁移失败（跳过）：%v", err)
	}
	repo := repository.NewCaddySiteRepository(db)
	for i := range sites {
		if err := repo.Create(context.Background(), &sites[i]); err != nil {
			t.Fatalf("创建站点失败：%v", err)
		}
	}
	dataDir := t.TempDir()
	if settings == nil {
		settings = map[string]string{}
	}
	return &CaddyManager{
		dataDir:      dataDir,
		configDir:    filepath.Join(dataDir, "proxy"),
		siteRepo:     repo,
		settingStore: fakeSettingStore{m: settings},
	}
}

func generateCaddyFor(t *testing.T, m *CaddyManager) string {
	t.Helper()
	cfg, err := m.GenerateConfigForServer(context.Background(), 2)
	if err != nil {
		t.Fatalf("生成 Caddyfile 失败：%v", err)
	}
	return cfg
}

// Caddy 的 layer4 站点地址只接受 `:端口`。迁移 nginx 规则时 Path 可能带 IP
// （203.0.113.10:443 / [::]:443 / 0.0.0.0:443），必须归一到同一端口，
// 否则会生成 `:203.0.113.10:443` 这种非法地址，或同一端口拆成多个 server。
func TestCaddySNIListenAddressNormalized(t *testing.T) {
	sites := []entity.CaddySite{
		{ServerID: 2, Domain: "a.example.com", Template: "sni", Path: "203.0.113.10:443", SNITarget: "127.0.0.1:9001", Enabled: true},
		{ServerID: 2, Domain: "b.example.com", Template: "sni", Path: "443", SNITarget: "127.0.0.1:9002", Enabled: true},
		{ServerID: 2, Domain: "c.example.com", Template: "sni", Path: "0.0.0.0:443", SNITarget: "127.0.0.1:9003", Enabled: true},
		{ServerID: 2, Domain: "d.example.com", Template: "sni", Path: "[::]:443", SNITarget: "127.0.0.1:9004", Enabled: true},
	}
	cfg := generateCaddyFor(t, newCaddyTestManager(t, sites, nil))

	if n := strings.Count(cfg, "\t\t:443 {"); n != 1 {
		t.Fatalf("同一端口的 SNI 规则应只有一个 layer4 server，实际 %d 个：\n%s", n, cfg)
	}
	if strings.Contains(cfg, ":203.0.113.10:443") || strings.Contains(cfg, ":0.0.0.0:443") {
		t.Fatalf("Caddy 不应生成带 IP 的 layer4 地址：\n%s", cfg)
	}
	for _, domain := range []string{"a.example.com", "b.example.com", "c.example.com", "d.example.com"} {
		if !strings.Contains(cfg, "SNI: "+domain) {
			t.Fatalf("归一后 SNI 规则 %s 不应丢失：\n%s", domain, cfg)
		}
	}
}

// 编辑已有 SNI 站点并修改监听端口时，预览必须按 ID 替换旧行，
// 否则旧的 :443 与新的 :8445 会同时出现。
func TestCaddyPreviewSiteReplacesByID(t *testing.T) {
	sites := []entity.CaddySite{
		{ServerID: 2, Domain: "a.example.com", Template: "sni", Path: "443", SNITarget: "127.0.0.1:9001", Enabled: true},
	}
	m := newCaddyTestManager(t, sites, nil)
	stored, err := m.siteRepo.List(context.Background(), 2)
	if err != nil || len(stored) != 1 {
		t.Fatalf("读取测试站点失败：%v %+v", err, stored)
	}

	draft := stored[0]
	draft.Path = "127.0.0.1:8445"
	cfg, err := m.PreviewSiteConfig(context.Background(), 2, draft)
	if err != nil {
		t.Fatalf("预览失败：%v", err)
	}
	if n := strings.Count(cfg, "\t\t:8445 {"); n != 1 {
		t.Fatalf("预览应只有一份新的 layer4 配置，实际 %d 份：\n%s", n, cfg)
	}
	if strings.Contains(cfg, "\t\t:443 {") {
		t.Fatalf("编辑端口后不应保留旧 :443 分组：\n%s", cfg)
	}
}

// Reality 站点可以额外给 sing-box 一个本地端口：
// 该端口的 servers 配置不挂 proxy_protocol wrapper（sing-box 不发 PP）。
func TestCaddyRealitySingboxPortWithoutProxyProtocol(t *testing.T) {
	sites := []entity.CaddySite{
		{ServerID: 2, Domain: ":9443", Template: "reality", SingboxListenPort: "9444",
			CertPath: "certs/example.com/cert.pem", KeyPath: "certs/example.com/key.pem", Enabled: true},
	}
	cfg := generateCaddyFor(t, newCaddyTestManager(t, sites, nil))

	if !strings.Contains(cfg, "\tservers 127.0.0.1:9443 {") {
		t.Fatalf("缺少 Xray 端口的 servers 配置：\n%s", cfg)
	}
	if !strings.Contains(cfg, "\tservers 127.0.0.1:9444 {") {
		t.Fatalf("缺少 sing-box 端口的 servers 配置：\n%s", cfg)
	}
	if !strings.Contains(cfg, ":9443, :9444 {") {
		t.Fatalf("站点块应同时绑定两个地址：\n%s", cfg)
	}
	// sing-box 的 servers 块里不能出现 proxy_protocol
	idx := strings.Index(cfg, "\tservers 127.0.0.1:9444 {")
	tail := cfg[idx:]
	end := strings.Index(tail, "\n\t}")
	if end < 0 || strings.Contains(tail[:end], "proxy_protocol") {
		t.Fatalf("sing-box 端口不能挂 proxy_protocol wrapper：\n%s", cfg)
	}
}

// 显式设置默认后端时优先于 fallback 规则（与 nginx 一致）。
func TestCaddySNIDefaultTargetOverridesFallback(t *testing.T) {
	sites := []entity.CaddySite{
		{ServerID: 2, Domain: "fallback", Template: "sni", Path: "443", SNITarget: "127.0.0.1:8443", LocalPort: "8443", Enabled: true},
		{ServerID: 2, Domain: "a.example.com", Template: "sni", Path: "443", SNITarget: "127.0.0.1:9001", Enabled: true},
	}
	cfg := generateCaddyFor(t, newCaddyTestManager(t, sites, map[string]string{
		"proxy.sni_default_target.2": "127.0.0.1:443",
	}))
	if !strings.Contains(cfg, "# fallback -> 127.0.0.1:443") {
		t.Fatalf("默认后端应使用设置里的 127.0.0.1:443：\n%s", cfg)
	}
}

// 规则未填 local_port 时，兜底目标应使用 SNI 设置里的本地 HTTPS 端口。
func TestCaddySNIFallbackUsesSettingsLocalPort(t *testing.T) {
	sites := []entity.CaddySite{
		{ServerID: 2, Domain: "a.example.com", Template: "sni", Path: "443", SNITarget: "127.0.0.1:9001", Enabled: true},
	}
	cfg := generateCaddyFor(t, newCaddyTestManager(t, sites, map[string]string{
		"proxy.sni_local_port.2": "9443",
	}))
	if !strings.Contains(cfg, "# fallback -> 127.0.0.1:9443") {
		t.Fatalf("未设置规则 local_port 时应使用设置的 9443：\n%s", cfg)
	}
}

// 未设置默认后端时回落到 fallback 规则目标。
func TestCaddySNIDefaultTargetUnsetUsesFallback(t *testing.T) {
	sites := []entity.CaddySite{
		{ServerID: 2, Domain: "fallback", Template: "sni", Path: "443", SNITarget: "127.0.0.1:8443", LocalPort: "8443", Enabled: true},
		{ServerID: 2, Domain: "a.example.com", Template: "sni", Path: "443", SNITarget: "127.0.0.1:9001", Enabled: true},
	}
	cfg := generateCaddyFor(t, newCaddyTestManager(t, sites, nil))
	if !strings.Contains(cfg, "# fallback -> 127.0.0.1:8443") {
		t.Fatalf("未设置默认后端时应使用 fallback 规则：\n%s", cfg)
	}
}
