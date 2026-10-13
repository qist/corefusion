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

// newNginxTestManager 构造一个基于内存 SQLite 的 NginxManager，用于验证配置生成。
func newNginxTestManager(t *testing.T, sites []entity.NginxSite, settingsJSON string) *NginxManager {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("内存 SQLite 不可用（跳过）：%v", err)
	}
	if err := db.AutoMigrate(&entity.NginxSite{}); err != nil {
		t.Skipf("迁移失败（跳过）：%v", err)
	}
	repo := repository.NewNginxSiteRepository(db)
	for i := range sites {
		if err := repo.Create(context.Background(), &sites[i]); err != nil {
			t.Fatalf("创建站点失败：%v", err)
		}
	}
	dataDir := t.TempDir()
	m := &NginxManager{
		dataDir:   dataDir,
		configDir: filepath.Join(dataDir, "proxy"),
		siteRepo:  repo,
		settingStore: fakeSettingStore{m: map[string]string{
			"proxy.nginx_settings": settingsJSON,
			"panel.listen_port":    "8080",
		}},
	}
	m.SetServerID(2)
	return m
}

func generateFor(t *testing.T, m *NginxManager) string {
	t.Helper()
	cfg, err := m.GenerateConfigForServer(context.Background(), 2)
	if err != nil {
		t.Fatalf("生成配置失败：%v", err)
	}
	return cfg
}

// TestNginxConfigIPv6Switch 锁定 nginx IPv6 开关的行为：
//   - 全局与站点都关闭 → 不生成任何 [::] 监听（仅 IPv4）；
//   - 全局开启 → 站点 server 与 HTTP→HTTPS 跳转 server 都追加 [::] 监听；
//   - 全局关闭但站点级 enable_ipv6 打开 → 该站点 server 单独追加 [::] 监听。
func TestNginxConfigIPv6Switch(t *testing.T) {
	baseSites := func() []entity.NginxSite {
		return []entity.NginxSite{
			{ServerID: 2, Domain: "v6.example.com", ListenPort: "443", CertPath: "cert.pem", KeyPath: "key.pem", Enabled: true, Path: "/", Upstream: "127.0.0.1:8080"},
			{ServerID: 2, Domain: "v6.example.com", ListenPort: "80", Enabled: true, Path: "/", Upstream: "127.0.0.1:8080"},
		}
	}

	// 1) 全局关 + 站点关 → 无 IPv6
	cfg := generateFor(t, newNginxTestManager(t, baseSites(), `{}`))
	if strings.Contains(cfg, "[::]") {
		t.Fatalf("全局与站点均关闭时不应生成 [::] 监听：\n%s", cfg)
	}

	// 2) 全局开 → 站点 server 与跳转 server 都有 [::]
	cfg = generateFor(t, newNginxTestManager(t, baseSites(), `{"ipv6":true}`))
	if !strings.Contains(cfg, "listen [::]:443 ssl;") {
		t.Fatalf("全局 IPv6 开启时站点 server 应生成 listen [::]:443 ssl：\n%s", cfg)
	}
	if !strings.Contains(cfg, "listen [::]:80;") {
		t.Fatalf("全局 IPv6 开启时 HTTP→HTTPS 跳转 server 应生成 listen [::]:80：\n%s", cfg)
	}

	// 3) 全局关 + 站点开 → 该域名组（含 HTTP→HTTPS 跳转）有 [::]
	//    跳转 server 是按域名组生成的，只要组内任一站点开了 IPv6，就应同时监听 ipv6，
	//    否则 ipv6 客户端访问 http:// 拿不到跳转。
	sites := baseSites()
	sites[0].EnableIpv6 = true
	cfg = generateFor(t, newNginxTestManager(t, sites, `{}`))
	if !strings.Contains(cfg, "listen [::]:443 ssl;") {
		t.Fatalf("站点级 enable_ipv6 打开时应生成 listen [::]:443 ssl：\n%s", cfg)
	}
	if !strings.Contains(cfg, "listen [::]:80;") {
		t.Fatalf("域名组内站点开启 IPv6 时，跳转 server 应同时生成 listen [::]:80：\n%s", cfg)
	}
}
