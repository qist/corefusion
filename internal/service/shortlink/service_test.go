package shortlink

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

func TestIsCode(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"7kf2m9qxp3va7kf2m9qx", true},
		{"7KF2M9QXP3VA7KF2M9QX", true},  // 查询时统一转小写，输入大写也认
		{"s_7kf2m9qxp3va7kf2m", false},  // 不再使用保留前缀
		{"0123456789abcdefghij", false}, // i 不在字符集
		{"0123456789abcdefghjk", true},
		{"", false},
		{"0123456789abcdefghj", false},   // 19 位
		{"0123456789abcdefghjkl", false}, // 21 位
		{"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcd", false}, // 64 hex token
	}
	for _, c := range cases {
		if got := IsCode(c.in); got != c.want {
			t.Errorf("IsCode(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestRandomCode(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		code, err := randomCode(CodeLength)
		if err != nil {
			t.Fatal(err)
		}
		if len(code) != CodeLength {
			t.Fatalf("len = %d, want %d", len(code), CodeLength)
		}
		if !IsCode(code) {
			t.Fatalf("generated code %q is not valid", code)
		}
		if seen[code] {
			t.Fatalf("duplicate code %q", code)
		}
		seen[code] = true
	}
}

func TestCredentialURL(t *testing.T) {
	db := openTestDB(t)
	subRepo := repository.NewSubscriptionRepository(db)
	endpointRepo := repository.NewSubscriptionEndpointRepository(db)
	settingRepo := repository.NewSystemSettingRepository(db)
	svc := NewService(repository.NewSubscriptionShortLinkRepository(db), subRepo, nil, nil, nil, endpointRepo, settingRepo)

	// 无端点：回退 panel.external_url + /sub
	_ = settingRepo.Set(context.Background(), "panel.external_url", "https://panel.example.com")
	got := svc.CredentialURL(context.Background(), "abc123", "")
	if want := "https://panel.example.com/sub/abc123"; got != want {
		t.Fatalf("fallback url = %q, want %q", got, want)
	}

	// 有独立订阅域名 + path=/ → 根路径
	_ = endpointRepo.Create(context.Background(), &entity.SubscriptionEndpoint{
		Domain: "sub.example.com", Path: "/", Enabled: true,
	})
	got = svc.CredentialURL(context.Background(), "abc123", "")
	if want := "https://sub.example.com/abc123"; got != want {
		t.Fatalf("endpoint url = %q, want %q", got, want)
	}

	// 多域名：优先本次请求命中的域名
	_ = endpointRepo.Create(context.Background(), &entity.SubscriptionEndpoint{
		Domain: "sub2.example.com", Path: "/sub", Enabled: true,
	})
	got = svc.CredentialURL(context.Background(), "abc123", "sub2.example.com")
	if want := "https://sub2.example.com/sub/abc123"; got != want {
		t.Fatalf("request-domain url = %q, want %q", got, want)
	}
}

func TestSettingsDefaultAndSave(t *testing.T) {
	db := openTestDB(t)
	svc := NewService(
		repository.NewSubscriptionShortLinkRepository(db),
		repository.NewSubscriptionRepository(db),
		nil,
		nil,
		nil,
		repository.NewSubscriptionEndpointRepository(db),
		repository.NewSystemSettingRepository(db),
	)
	cfg := svc.GetSettings(context.Background())
	if cfg.Enabled || !cfg.AutoCreate || cfg.FollowTokenRotation {
		t.Fatalf("unexpected default: %+v", cfg)
	}
	if err := svc.SaveSettings(context.Background(), Settings{Enabled: true, AutoCreate: false, FollowTokenRotation: true}); err != nil {
		t.Fatal(err)
	}
	cfg = svc.GetSettings(context.Background())
	if !cfg.Enabled || cfg.AutoCreate || !cfg.FollowTokenRotation {
		t.Fatalf("settings not persisted: %+v", cfg)
	}
}

func TestAutoCreateAndBackfill(t *testing.T) {
	db := openTestDB(t)
	subRepo := repository.NewSubscriptionRepository(db)
	linkRepo := repository.NewSubscriptionShortLinkRepository(db)
	setRepo := repository.NewSystemSettingRepository(db)
	svc := NewService(linkRepo, subRepo, nil, nil, nil, repository.NewSubscriptionEndpointRepository(db), setRepo)

	sub := &entity.Subscription{
		ClientID: 1, Format: "base64_uri", Token: "tok-auto", TokenVersion: 1,
		Status: "active", SecurityMode: "token_only", BindMode: "none", QuotaState: "normal",
	}
	if err := subRepo.Create(context.Background(), sub); err != nil {
		t.Fatal(err)
	}
	if err := svc.SaveSettings(context.Background(), Settings{Enabled: true, AutoCreate: true}); err != nil {
		t.Fatal(err)
	}
	if err := svc.AutoCreateForSubscription(context.Background(), sub); err != nil {
		t.Fatal(err)
	}
	links, err := svc.ListBySubscription(context.Background(), sub.ID)
	if err != nil || len(links) != 1 {
		t.Fatalf("auto create links = %d, err=%v", len(links), err)
	}

	// 幂等：再跑 backfill 不应重复创建
	created, skipped, failed, err := svc.Backfill(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if created != 0 || skipped != 1 || failed != 0 {
		t.Fatalf("backfill created=%d skipped=%d failed=%d, want 0/1/0", created, skipped, failed)
	}
}

// 每个「格式 × 配置」组合都要有独立短链，URL 不带任何 query；重复调用幂等。
func TestEnsureLinksCoversCombosWithoutQuery(t *testing.T) {
	db := openTestDB(t)
	subRepo := repository.NewSubscriptionRepository(db)
	linkRepo := repository.NewSubscriptionShortLinkRepository(db)
	configRepo := repository.NewMihomoConfigRepository(db)
	fmtRepo := repository.NewSubscriptionFormatRepository(db)
	setRepo := repository.NewSystemSettingRepository(db)

	cfg := &entity.MihomoConfig{Name: "win", Format: "mihomo_yaml", Platform: "win", Enabled: true}
	if err := configRepo.Create(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	if err := fmtRepo.Create(context.Background(), &entity.SubscriptionFormat{
		Name: "V2RayN", ClientType: "v2ray", Format: "base64_uri", Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	// 与 Mihomo 配置同名的订阅格式：门户按名字去重，短链也不能多出「无配置」条目。
	if err := configRepo.Create(context.Background(), &entity.MihomoConfig{
		Name: "Sing-Box", Format: "singbox_json", Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := fmtRepo.Create(context.Background(), &entity.SubscriptionFormat{
		Name: "Sing-Box", ClientType: "singbox", Format: "singbox_json", Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}

	sub := &entity.Subscription{
		ClientID: 0, Format: "mihomo_yaml", MihomoConfigID: &cfg.ID,
		Token: "tok-ensure", TokenVersion: 1, Status: "active",
		SecurityMode: "token_only", BindMode: "none", QuotaState: "normal",
	}
	if err := subRepo.Create(context.Background(), sub); err != nil {
		t.Fatal(err)
	}

	svc := NewService(linkRepo, subRepo, configRepo, fmtRepo, nil,
		repository.NewSubscriptionEndpointRepository(db), setRepo)
	if err := svc.SaveSettings(context.Background(), Settings{Enabled: true}); err != nil {
		t.Fatal(err)
	}

	links, err := svc.EnsureLinks(context.Background(), sub.ID)
	if err != nil {
		t.Fatal(err)
	}
	has := func(format, config string, providers bool) bool {
		for _, l := range links {
			if l.Format == format && l.ConfigName == config && l.Providers == providers {
				return true
			}
		}
		return false
	}
	if !has("mihomo_yaml", "win", false) || !has("mihomo_yaml", "win", true) {
		t.Fatalf("缺少配置组合：%+v", links)
	}
	if !has("base64_uri", "", false) || !has("raw_uri", "", false) {
		t.Fatalf("缺少基础格式：%+v", links)
	}
	// sing-box 有同名配置时只应有配置链接，不能多出无配置链接。
	for _, l := range links {
		if l.Format == "singbox_json" && l.ConfigName == "" {
			t.Fatalf("sing-box 不应出现无配置短链：%+v", l)
		}
	}
	if !has("singbox_json", "Sing-Box", false) {
		t.Fatalf("缺少 sing-box 配置组合：%+v", links)
	}

	for _, v := range svc.EnsureViews(context.Background(), sub.ID) {
		if strings.Contains(v.URL, "?") {
			t.Fatalf("短链 URL 仍带参数：%s", v.URL)
		}
	}

	before := len(links)
	again, err := svc.EnsureLinks(context.Background(), sub.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != before {
		t.Fatalf("EnsureLinks 不幂等：before=%d after=%d", before, len(again))
	}
	all, _ := linkRepo.ListBySubscription(context.Background(), sub.ID)
	if len(all) != before {
		t.Fatalf("重复补齐产生了新行：links=%d rows=%d", before, len(all))
	}
}

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "shortlink.db")), &gorm.Config{})
	if err != nil {
		t.Skipf("SQLite 不可用（跳过）：%v", err)
	}
	if err := db.AutoMigrate(
		&entity.Subscription{},
		&entity.SubscriptionShortLink{},
		&entity.SubscriptionEndpoint{},
		&entity.SystemSetting{},
		&entity.MihomoConfig{},
		&entity.SubscriptionFormat{},
	); err != nil {
		t.Skipf("迁移失败（跳过）：%v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}
