package georule

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

type memSettingStore struct {
	vals map[string]string
}

func (m *memSettingStore) GetByKey(_ context.Context, key string) (string, error) {
	return m.vals[key], nil
}

func (m *memSettingStore) Set(_ context.Context, key, value string) error {
	if m.vals == nil {
		m.vals = map[string]string{}
	}
	m.vals[key] = value
	return nil
}

func newRenameHintTestService(t *testing.T) (*Service, *entity.RuleSource) {
	t.Helper()
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
		&memSettingStore{},
	)
	src := &entity.RuleSource{Name: "iran", RepoURL: "chocolate4u/Iran-v2ray-rules"}
	if err := svc.sourceRepo.Create(context.Background(), src); err != nil {
		t.Fatalf("create source: %v", err)
	}
	return svc, src
}

// 重命名要在删除文件记录之后仍然保留（重新勾选时由接口带回前端预填）。
func TestRenameHintsSurviveFileDeletion(t *testing.T) {
	ctx := context.Background()
	svc, src := newRenameHintTestService(t)

	file := &entity.GeoRuleFile{
		SourceID:    src.ID,
		Name:        "geoip.dat",
		FileName:    "geoip_IR.dat",
		SourceAsset: "geoip.dat",
		DownloadURL: "https://example.com/geoip.dat",
		Status:      "available",
	}
	if err := svc.fileRepo.Create(ctx, file); err != nil {
		t.Fatalf("create file: %v", err)
	}
	if err := svc.DeleteFile(ctx, file.ID); err != nil {
		t.Fatalf("delete file: %v", err)
	}
	if got := svc.SourceRenameHints(ctx, src.ID)["geoip.dat"]; got != "geoip_IR.dat" {
		t.Fatalf("删除文件后重命名记忆 = %q, want geoip_IR.dat", got)
	}
}

// 用户显式改回原始资产名时，记忆要清掉，否则下次勾选还会带出旧名字。
func TestRenameHintsClearedWhenReNamedBack(t *testing.T) {
	ctx := context.Background()
	svc, src := newRenameHintTestService(t)

	svc.rememberRenames(ctx, src.RepoURL, []selectPlan{
		{asset: ReleaseAsset{Name: "geosite.dat"}, rename: "geosite_IR.dat"},
	})
	svc.rememberRenames(ctx, src.RepoURL, []selectPlan{
		{asset: ReleaseAsset{Name: "geosite.dat"}},
	})
	if hints := svc.SourceRenameHints(ctx, src.ID); len(hints) != 0 {
		t.Fatalf("改回原名后仍有记忆: %v", hints)
	}
}

// 记忆按仓库区分：不同仓库的同名资产互不影响。
func TestRenameHintsScopedByRepo(t *testing.T) {
	ctx := context.Background()
	svc, iran := newRenameHintTestService(t)

	ru := &entity.RuleSource{Name: "ru", RepoURL: "runetfreedom/russia-v2ray-rules-dat"}
	if err := svc.sourceRepo.Create(ctx, ru); err != nil {
		t.Fatalf("create source: %v", err)
	}
	svc.rememberRenames(ctx, iran.RepoURL, []selectPlan{
		{asset: ReleaseAsset{Name: "geoip.dat"}, rename: "geoip_IR.dat"},
	})
	if hints := svc.SourceRenameHints(ctx, ru.ID); len(hints) != 0 {
		t.Fatalf("其它仓库不应共享记忆: %v", hints)
	}
	if got := svc.SourceRenameHints(ctx, iran.ID)["geoip.dat"]; got != "geoip_IR.dat" {
		t.Fatalf("伊朗仓库记忆 = %q, want geoip_IR.dat", got)
	}
}
