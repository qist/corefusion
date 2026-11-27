package shortlink

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/security/crypto"
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
	// base64_uri + raw_uri（默认格式 base64 与第一条去重）
	if err != nil || len(links) != 2 {
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
	// 没有任何 xray 配置时，xray_json 订阅格式不应生成短链。
	if err := fmtRepo.Create(context.Background(), &entity.SubscriptionFormat{
		Name: "Xray-NoCfg", ClientType: "v2ray", Format: "xray_json", Enabled: true,
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
		if l.Format == "xray_json" {
			t.Fatalf("没有 xray 配置时不应生成 xray 短链：%+v", l)
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
		&entity.SubscriptionShortLinkRevocation{},
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

// Backfill 要补齐全部组合，而不是只建默认那条。
func TestBackfillCreatesAllCombos(t *testing.T) {
	db := openTestDB(t)
	subRepo := repository.NewSubscriptionRepository(db)
	linkRepo := repository.NewSubscriptionShortLinkRepository(db)
	setRepo := repository.NewSystemSettingRepository(db)
	svc := NewService(linkRepo, subRepo, nil, nil, nil,
		repository.NewSubscriptionEndpointRepository(db), setRepo)
	if err := svc.SaveSettings(context.Background(), Settings{Enabled: true, AutoCreate: false}); err != nil {
		t.Fatal(err)
	}
	sub := &entity.Subscription{
		ClientID: 0, Format: "base64_uri", Token: "tok-backfill", TokenVersion: 1,
		Status: "active", SecurityMode: "token_only", BindMode: "none", QuotaState: "normal",
	}
	if err := subRepo.Create(context.Background(), sub); err != nil {
		t.Fatal(err)
	}
	// 只有默认一条（模拟旧行为）
	if _, err := svc.Create(context.Background(), CreateInput{
		SubscriptionID: sub.ID, Format: "base64_uri",
	}); err != nil {
		t.Fatal(err)
	}

	created, skipped, failed, err := svc.Backfill(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if created != 1 || skipped != 0 || failed != 0 {
		t.Fatalf("backfill created=%d skipped=%d failed=%d, want 1/0/0", created, skipped, failed)
	}
	links, _ := svc.ListBySubscription(context.Background(), sub.ID)
	if len(links) != 2 {
		t.Fatalf("backfill 后应有 base64+raw 两条，得到 %d", len(links))
	}
}

// mihomo_yaml 没有配置时不生成短链：这类链接只有 proxies，没有 proxy-groups/rules。
func TestEnsureLinksSkipsUnconfiguredMihomo(t *testing.T) {
	db := openTestDB(t)
	subRepo := repository.NewSubscriptionRepository(db)
	linkRepo := repository.NewSubscriptionShortLinkRepository(db)
	configRepo := repository.NewMihomoConfigRepository(db)
	fmtRepo := repository.NewSubscriptionFormatRepository(db)
	setRepo := repository.NewSystemSettingRepository(db)

	if err := fmtRepo.Create(context.Background(), &entity.SubscriptionFormat{
		Name: "Mihomo", ClientType: "clash", Format: "mihomo_yaml", Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	sub := &entity.Subscription{
		ClientID: 0, Format: "mihomo_yaml",
		Token: "tok-nocfg", TokenVersion: 1, Status: "active",
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
	for _, l := range links {
		if l.Format == "mihomo_yaml" {
			t.Fatalf("无配置的 mihomo_yaml 不应生成短链：%+v", l)
		}
	}
	if _, err := svc.Create(context.Background(), CreateInput{
		SubscriptionID: sub.ID, Format: "mihomo_yaml",
	}); err == nil {
		t.Fatal("无配置的 mihomo_yaml 应拒绝创建")
	}
}

// 存量无配置 mihomo_yaml 链接要清理；带平台/配置名或其它格式的保留。
func TestPurgeUnconfiguredMihomo(t *testing.T) {
	db := openTestDB(t)
	subRepo := repository.NewSubscriptionRepository(db)
	linkRepo := repository.NewSubscriptionShortLinkRepository(db)
	setRepo := repository.NewSystemSettingRepository(db)

	sub := &entity.Subscription{
		ClientID: 0, Format: "base64_uri",
		Token: "tok-purge", TokenVersion: 1, Status: "active",
		SecurityMode: "token_only", BindMode: "none", QuotaState: "normal",
	}
	if err := subRepo.Create(context.Background(), sub); err != nil {
		t.Fatal(err)
	}
	rows := []*entity.SubscriptionShortLink{
		{SubscriptionID: sub.ID, Code: "empty-mihomo", Format: "mihomo_yaml", Enabled: true},
		{SubscriptionID: sub.ID, Code: "platform-mihomo", Format: "mihomo_yaml", Platform: "win", Enabled: true},
		{SubscriptionID: sub.ID, Code: "named-mihomo", Format: "mihomo_yaml", ConfigName: "out", Enabled: true},
		{SubscriptionID: sub.ID, Code: "proxies-only", Format: "mihomo_proxies", Enabled: true},
		{SubscriptionID: sub.ID, Code: "base64-only", Format: "base64_uri", Enabled: true},
	}
	for _, row := range rows {
		if err := linkRepo.Create(context.Background(), row); err != nil {
			t.Fatal(err)
		}
	}

	svc := NewService(linkRepo, subRepo, nil, nil, nil,
		repository.NewSubscriptionEndpointRepository(db), setRepo)
	if removed := svc.PurgeUnconfiguredMihomo(context.Background(), sub.ID); removed != 1 {
		t.Fatalf("应只清理 1 条无配置 mihomo_yaml，实际 %d", removed)
	}
	left, err := linkRepo.ListBySubscription(context.Background(), sub.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != len(rows)-1 {
		t.Fatalf("清理后应剩 %d 条，实际 %d", len(rows)-1, len(left))
	}
	for _, l := range left {
		if l.Code == "empty-mihomo" {
			t.Fatal("无配置 mihomo_yaml 未被清理")
		}
	}
}

// 轮换/删除都要把旧短码的 HMAC 指纹写进撤销表（删除后不得复用）。
func TestRevocationOnRotateAndDelete(t *testing.T) {
	db := openTestDB(t)
	subRepo := repository.NewSubscriptionRepository(db)
	linkRepo := repository.NewSubscriptionShortLinkRepository(db)
	revRepo := repository.NewSubscriptionShortLinkRevocationRepository(db)
	setRepo := repository.NewSystemSettingRepository(db)
	svc := NewService(linkRepo, subRepo, nil, nil, nil,
		repository.NewSubscriptionEndpointRepository(db), setRepo)
	svc.SetRevocationRepository(revRepo)
	ctx := context.Background()

	sub := &entity.Subscription{
		ClientID: 0, Format: "base64_uri", Token: "tok-revoke",
		TokenVersion: 1, Status: "active",
		SecurityMode: "token_only", BindMode: "none", QuotaState: "normal",
	}
	if err := subRepo.Create(ctx, sub); err != nil {
		t.Fatal(err)
	}
	link, err := svc.Create(ctx, CreateInput{SubscriptionID: sub.ID, Format: "base64_uri"})
	if err != nil {
		t.Fatal(err)
	}
	oldFP, err := crypto.FingerprintSecret(link.Code)
	if err != nil {
		t.Fatal(err)
	}

	rotated, err := svc.RotateCode(ctx, link.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rotated.Code == link.Code {
		t.Fatal("轮换后 code 未变化")
	}
	if revoked, err := revRepo.Exists(ctx, oldFP); err != nil || !revoked {
		t.Fatalf("轮换后旧 code 未撤销：revoked=%v err=%v", revoked, err)
	}

	newFP, err := crypto.FingerprintSecret(rotated.Code)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, rotated.ID); err != nil {
		t.Fatal(err)
	}
	if revoked, err := revRepo.Exists(ctx, newFP); err != nil || !revoked {
		t.Fatalf("删除后 code 未撤销：revoked=%v err=%v", revoked, err)
	}
}

// 绑定停用配置 / 跨格式配置必须在创建时拒绝：否则链接打开会退化成「只有 proxies」。
func TestCreateRejectsUnusableConfig(t *testing.T) {
	db := openTestDB(t)
	subRepo := repository.NewSubscriptionRepository(db)
	linkRepo := repository.NewSubscriptionShortLinkRepository(db)
	configRepo := repository.NewMihomoConfigRepository(db)
	setRepo := repository.NewSystemSettingRepository(db)
	svc := NewService(linkRepo, subRepo, configRepo, nil, nil,
		repository.NewSubscriptionEndpointRepository(db), setRepo)
	ctx := context.Background()

	sub := &entity.Subscription{
		ClientID: 0, Format: "mihomo_yaml", Token: "tok-cfg-check",
		TokenVersion: 1, Status: "active",
		SecurityMode: "token_only", BindMode: "none", QuotaState: "normal",
	}
	if err := subRepo.Create(ctx, sub); err != nil {
		t.Fatal(err)
	}

	// Enabled 带 default:true，直接用结构体 Create(false) 会被 GORM 忽略，
	// 这里先建再显式停用，模拟后台停用配置后的真实库状态。
	disabled := &entity.MihomoConfig{Name: "off", Format: "mihomo_yaml", Enabled: true}
	if err := configRepo.Create(ctx, disabled); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&entity.MihomoConfig{}).Where("id = ?", disabled.ID).Update("enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, CreateInput{
		SubscriptionID: sub.ID, Format: "mihomo_yaml", MihomoConfigID: &disabled.ID,
	}); err == nil {
		t.Fatal("停用的 mihomo 配置应拒绝创建短链")
	}

	singbox := &entity.MihomoConfig{Name: "sb", Format: "singbox_json", Enabled: true}
	if err := configRepo.Create(ctx, singbox); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, CreateInput{
		SubscriptionID: sub.ID, Format: "mihomo_yaml", MihomoConfigID: &singbox.ID,
	}); err == nil {
		t.Fatal("跨格式的配置应拒绝创建短链")
	}

	ok := &entity.MihomoConfig{Name: "ok", Format: "mihomo_yaml", Enabled: true}
	if err := configRepo.Create(ctx, ok); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, CreateInput{
		SubscriptionID: sub.ID, Format: "mihomo_yaml", MihomoConfigID: &ok.ID,
	}); err != nil {
		t.Fatalf("正常配置应允许：%v", err)
	}
}

// 清理无配置 mihomo 短链时要写撤销指纹，避免短码被重新签发后旧地址指向别的订阅。
func TestPurgeUnconfiguredMihomoRevokesCode(t *testing.T) {
	db := openTestDB(t)
	subRepo := repository.NewSubscriptionRepository(db)
	linkRepo := repository.NewSubscriptionShortLinkRepository(db)
	revRepo := repository.NewSubscriptionShortLinkRevocationRepository(db)
	setRepo := repository.NewSystemSettingRepository(db)
	svc := NewService(linkRepo, subRepo, nil, nil, nil,
		repository.NewSubscriptionEndpointRepository(db), setRepo)
	svc.SetRevocationRepository(revRepo)
	ctx := context.Background()

	const code = "abcdefghjk0123456789"
	link := &entity.SubscriptionShortLink{
		SubscriptionID: 1, Code: code, Format: "mihomo_yaml", Enabled: true,
	}
	if err := linkRepo.Create(ctx, link); err != nil {
		t.Fatal(err)
	}
	if n := svc.PurgeUnconfiguredMihomo(ctx, 1); n != 1 {
		t.Fatalf("应删除 1 条无配置 mihomo 短链，实际 %d", n)
	}
	fp, err := crypto.FingerprintSecret(code)
	if err != nil {
		t.Fatal(err)
	}
	if revoked, err := revRepo.Exists(ctx, fp); err != nil || !revoked {
		t.Fatalf("短码未写入撤销表：revoked=%v err=%v", revoked, err)
	}
}
