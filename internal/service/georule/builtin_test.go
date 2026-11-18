package georule

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

func newBuiltinTestService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	// 用临时文件库：SeedDefaults 会起异步 goroutine，`:memory:` 每条连接是
	// 独立库，goroutine 里会报 no such table。
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "geo.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&entity.RuleSource{}, &entity.GeoRuleFile{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	svc := NewService(
		repository.NewRuleSourceRepository(db),
		repository.NewGeoRuleRepository(db),
		t.TempDir(),
		nil,
	)
	return svc, db
}

func listSources(t *testing.T, svc *Service) []entity.RuleSource {
	t.Helper()
	srcs, err := svc.sourceRepo.ListByServer(context.Background(), 0)
	if err != nil {
		t.Fatalf("list sources: %v", err)
	}
	return srcs
}

func findSourceByRepo(srcs []entity.RuleSource, repo string) *entity.RuleSource {
	want := normalizeRepoKey(repo)
	for i := range srcs {
		if normalizeRepoKey(srcs[i].RepoURL) == want {
			return &srcs[i]
		}
	}
	return nil
}

// 内置规则源必须是标准写法：伊朗/俄罗斯各一条 GitHub 仓库源，每个 srs URL
// 一条直连源；不能再出现「内置路由规则（IR/RU/CN/广告）」这种聚合源。
func TestEnsureBuiltinFilesCreatesStandardSources(t *testing.T) {
	svc, _ := newBuiltinTestService(t)
	svc.ensureBuiltinSourcesInDB(context.Background())

	srcs := listSources(t, svc)
	if len(srcs) != len(builtinSources) {
		t.Fatalf("源数量 = %d, want %d", len(srcs), len(builtinSources))
	}
	for _, s := range srcs {
		if s.Name == legacyBuiltinSourceName {
			t.Errorf("不应再创建聚合源: %s", s.Name)
		}
		if !s.Enabled {
			t.Errorf("新建内置源应默认启用: %s", s.Name)
		}
		if s.SyncCron != builtinSyncCron {
			t.Errorf("内置源同步周期 = %q, want %q", s.SyncCron, builtinSyncCron)
		}
	}

	iran := findSourceByRepo(srcs, "chocolate4u/Iran-v2ray-rules")
	if iran == nil {
		t.Fatalf("缺少伊朗仓库源")
	}
	if iran.Name != "chocolate4u/Iran-v2ray-rules" {
		t.Errorf("伊朗源名称 = %q", iran.Name)
	}
	if isDirectURL(iran.RepoURL) {
		t.Errorf("伊朗源应是仓库模式（不带 http），实际 %q", iran.RepoURL)
	}

	// 每个 srs 一个源，名字是文件名主干（与线上规则库一致）
	for _, name := range []string{"geosite-cn", "geoip-cn", "geosite-category-ads-all", "geosite-ir", "geoip-ir", "geosite-category-ru", "geoip-ru"} {
		found := false
		for _, s := range srcs {
			if s.Name == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("缺少直连源 %q", name)
		}
	}
}

// 文件必须挂到对应标准源下，且仓库模式用 releases/latest/download/<asset>
// （伊朗/俄罗斯的资产名都叫 geoip.dat / geosite.dat，落盘要改名）。
func TestEnsureBuiltinFilesMapsFilesToSources(t *testing.T) {
	svc, _ := newBuiltinTestService(t)
	svc.ensureBuiltinSourcesInDB(context.Background())

	srcs := listSources(t, svc)
	files, err := svc.fileRepo.ListByServer(context.Background(), 0)
	if err != nil {
		t.Fatalf("list files: %v", err)
	}
	byName := map[string]entity.GeoRuleFile{}
	for _, f := range files {
		byName[f.FileName] = f
	}

	iran := findSourceByRepo(srcs, "chocolate4u/Iran-v2ray-rules")
	if iran == nil {
		t.Fatalf("缺少伊朗仓库源")
	}
	for local, asset := range map[string]string{"geoip_IR.dat": "geoip.dat", "geosite_IR.dat": "geosite.dat"} {
		f, ok := byName[local]
		if !ok {
			t.Fatalf("缺少文件 %s", local)
		}
		if f.SourceID != iran.ID {
			t.Errorf("%s 未挂到伊朗源（source_id=%d）", local, f.SourceID)
		}
		wantURL := "https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/" + asset
		if f.DownloadURL != wantURL {
			t.Errorf("%s 下载地址 = %q, want %q", local, f.DownloadURL, wantURL)
		}
	}

	cn := findSourceByRepo(srcs, "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-cn.srs")
	if cn == nil {
		t.Fatalf("缺少 geosite-cn 直连源")
	}
	f, ok := byName["geosite-cn.srs"]
	if !ok {
		t.Fatalf("缺少 geosite-cn.srs")
	}
	if f.SourceID != cn.ID {
		t.Errorf("geosite-cn.srs 未挂到直连源")
	}
	if f.DownloadURL != cn.RepoURL {
		t.Errorf("直连源文件地址应等于源 URL: %q", f.DownloadURL)
	}
}

// 重复执行必须幂等：不重复建源、不重复建文件。
func TestEnsureBuiltinFilesIsIdempotent(t *testing.T) {
	svc, _ := newBuiltinTestService(t)
	svc.ensureBuiltinSourcesInDB(context.Background())
	first, _ := svc.fileRepo.ListByServer(context.Background(), 0)

	created, _ := svc.ensureBuiltinSourcesInDB(context.Background())
	if len(created) != 0 {
		t.Errorf("第二次不应新建文件，got %d", len(created))
	}
	second, _ := svc.fileRepo.ListByServer(context.Background(), 0)
	if len(second) != len(first) {
		t.Errorf("文件数量变化: %d -> %d", len(first), len(second))
	}
	if got := len(listSources(t, svc)); got != len(builtinSources) {
		t.Errorf("源数量变化: %d", got)
	}
}

// 旧版聚合源：文件迁到标准源后旧源被删除；残留未知文件时保留旧源。
func TestEnsureBuiltinFilesMigratesLegacyAggregatedSource(t *testing.T) {
	svc, db := newBuiltinTestService(t)
	ctx := context.Background()

	legacy := &entity.RuleSource{ServerID: 0, Name: legacyBuiltinSourceName, RepoURL: "https://github.com/SagerNet/sing-geosite", SyncCron: "0 4 * * *", Status: "idle"}
	if err := db.Create(legacy).Error; err != nil {
		t.Fatalf("create legacy source: %v", err)
	}
	// 路由正在引用的文件（改名后的 IR dat）
	if err := db.Create(&entity.GeoRuleFile{
		ServerID: 0, SourceID: legacy.ID, Name: "geosite_IR.dat", FileName: "geosite_IR.dat",
		DownloadURL: "https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geosite.dat",
		AutoSync:    true, Status: "available",
	}).Error; err != nil {
		t.Fatalf("create legacy file: %v", err)
	}

	svc.ensureBuiltinSourcesInDB(ctx)

	var legacyCount int64
	db.Model(&entity.RuleSource{}).Where("name = ?", legacyBuiltinSourceName).Count(&legacyCount)
	if legacyCount != 0 {
		t.Errorf("旧聚合源应被删除（文件已迁移），仍有 %d 条", legacyCount)
	}

	srcs := listSources(t, svc)
	iran := findSourceByRepo(srcs, "chocolate4u/Iran-v2ray-rules")
	if iran == nil {
		t.Fatalf("缺少伊朗仓库源")
	}
	var row entity.GeoRuleFile
	if err := db.Where("file_name = ?", "geosite_IR.dat").First(&row).Error; err != nil {
		t.Fatalf("read migrated file: %v", err)
	}
	if row.SourceID != iran.ID {
		t.Errorf("迁移后 source_id = %d, want %d", row.SourceID, iran.ID)
	}
}

func TestEnsureBuiltinFilesKeepsLegacySourceWithUnknownFiles(t *testing.T) {
	svc, db := newBuiltinTestService(t)
	ctx := context.Background()

	legacy := &entity.RuleSource{ServerID: 0, Name: legacyBuiltinSourceName, RepoURL: "https://github.com/SagerNet/sing-geosite", SyncCron: "0 4 * * *", Status: "idle"}
	if err := db.Create(legacy).Error; err != nil {
		t.Fatalf("create legacy source: %v", err)
	}
	if err := db.Create(&entity.GeoRuleFile{
		ServerID: 0, SourceID: legacy.ID, Name: "user-own.dat", FileName: "user-own.dat",
		DownloadURL: "https://example.com/user-own.dat", AutoSync: true, Status: "available",
	}).Error; err != nil {
		t.Fatalf("create unknown file: %v", err)
	}

	svc.ensureBuiltinSourcesInDB(ctx)

	var legacyCount int64
	db.Model(&entity.RuleSource{}).Where("name = ?", legacyBuiltinSourceName).Count(&legacyCount)
	if legacyCount != 1 {
		t.Errorf("旧源还有未知文件时应保留，实际 %d 条", legacyCount)
	}
}

func TestRegionFiles(t *testing.T) {
	cases := []struct {
		core, region string
		want         []string
	}{
		{"xray", "ir", []string{"geoip_IR.dat", "geosite_IR.dat"}},
		{"xray", "ru", []string{"geoip_RU.dat", "geosite_RU.dat"}},
		{"singbox", "cn", []string{"geosite-cn.srs", "geoip-cn.srs"}},
		{"singbox", "ir", []string{"geosite-ir.srs", "geoip-ir.srs"}},
		{"singbox", "ru", []string{"geosite-category-ru.srs", "geoip-ru.srs"}},
	}
	for _, c := range cases {
		got := regionFiles(c.core, c.region)
		if strings.Join(got, ",") != strings.Join(c.want, ",") {
			t.Errorf("regionFiles(%s,%s) = %v, want %v", c.core, c.region, got, c.want)
		}
	}
}

func TestNormalizeRepoKey(t *testing.T) {
	cases := map[string]string{
		"chocolate4u/Iran-v2ray-rules":                        "chocolate4u/iran-v2ray-rules",
		"https://github.com/Chocolate4U/Iran-v2ray-rules":     "chocolate4u/iran-v2ray-rules",
		"https://github.com/Chocolate4U/Iran-v2ray-rules.git": "chocolate4u/iran-v2ray-rules",
		"Loyalsoldier/v2ray-rules-dat":                        "loyalsoldier/v2ray-rules-dat",
	}
	for in, want := range cases {
		if got := normalizeRepoKey(in); got != want {
			t.Errorf("normalizeRepoKey(%q) = %q, want %q", in, got, want)
		}
	}
}

// 全新安装：SeedDefaults 必须先播种主规则源（geoip.dat/geosite.dat），
// 内置地区源已经存在也不能再提前返回（否则 xray CN / sing-box 默认规则没文件）。
func TestSeedDefaultsStillSeedsMainSourceWhenBuiltinExists(t *testing.T) {
	svc, db := newBuiltinTestService(t)
	ctx := context.Background()

	// 先放一条「内置地区源」，模拟 EnsureBuiltinFiles 先跑过
	if err := db.Create(&entity.RuleSource{
		ServerID: 0, Name: "geosite-cn",
		RepoURL: "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-cn.srs",
		Enabled: true, SyncCron: builtinSyncCron, Status: "idle",
	}).Error; err != nil {
		t.Fatalf("create builtin source: %v", err)
	}

	if err := svc.SeedDefaults(ctx, 0); err != nil {
		t.Fatalf("SeedDefaults: %v", err)
	}

	srcs := listSources(t, svc)
	loyal := findSourceByRepo(srcs, "Loyalsoldier/v2ray-rules-dat")
	if loyal == nil {
		t.Fatalf("主规则源未播种（被 len(sources)>0 提前返回）")
	}
	files, _ := svc.fileRepo.ListBySource(ctx, loyal.ID)
	names := map[string]bool{}
	for _, f := range files {
		names[f.FileName] = true
	}
	if !names["geoip.dat"] || !names["geosite.dat"] {
		t.Fatalf("主规则文件缺失: %v", names)
	}
}
