package georule

import (
	"context"
	"testing"

	"corefusion/internal/model/entity"
)

// 直连源（一个 URL = 一个文件）删掉「已选规则文件」后，点同步不能恒返回 0：
// 必须按源 URL 的文件名把记录补回来，否则用户只能删源重建。
func TestEnsureDirectSourceFileRecreatesDeletedRecord(t *testing.T) {
	svc, db := newBuiltinTestService(t)
	ctx := context.Background()

	src := &entity.RuleSource{
		ServerID: 0,
		Name:     "geosite-ir",
		RepoURL:  "https://raw.githubusercontent.com/Chocolate4U/Iran-sing-box-rules/rule-set/geosite-ir.srs",
		Enabled:  true,
		SyncCron: builtinSyncCron,
		Status:   "idle",
	}
	if err := db.Create(src).Error; err != nil {
		t.Fatalf("create source: %v", err)
	}

	// 模拟：文件被删除（记录与磁盘都没了）
	files, _ := svc.fileRepo.ListBySource(ctx, src.ID)
	if len(files) != 0 {
		t.Fatalf("初始应为空，got %d", len(files))
	}

	got, row := svc.ensureDirectSourceFile(ctx, src, files)
	if row == nil {
		t.Fatalf("应补回文件记录")
	}
	if row.FileName != "geosite-ir.srs" {
		t.Errorf("FileName = %q, want geosite-ir.srs", row.FileName)
	}
	if row.DownloadURL != src.RepoURL {
		t.Errorf("DownloadURL = %q, want %q", row.DownloadURL, src.RepoURL)
	}
	if !row.AutoSync {
		t.Errorf("补回的记录应参与自动同步")
	}
	if len(got) != 1 {
		t.Errorf("返回列表长度 = %d, want 1", len(got))
	}

	// 落库可见
	saved, err := svc.fileRepo.ListBySource(ctx, src.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(saved) != 1 || saved[0].FileName != "geosite-ir.srs" {
		t.Fatalf("记录未落库: %+v", saved)
	}

	// 幂等：已有记录时不再新建
	again, row2 := svc.ensureDirectSourceFile(ctx, src, saved)
	if row2 != nil {
		t.Errorf("已有记录时不应重复创建")
	}
	if len(again) != 1 {
		t.Errorf("列表长度 = %d, want 1", len(again))
	}
}

// 仓库模式（owner/repo）不做这种补建：取消勾选文件是用户明确意图。
func TestEnsureDirectSourceFileIgnoresRepoSource(t *testing.T) {
	svc, db := newBuiltinTestService(t)
	ctx := context.Background()

	src := &entity.RuleSource{ServerID: 0, Name: "iran", RepoURL: "chocolate4u/Iran-v2ray-rules", Status: "idle"}
	if err := db.Create(src).Error; err != nil {
		t.Fatalf("create source: %v", err)
	}

	got, row := svc.ensureDirectSourceFile(ctx, src, nil)
	if row != nil {
		t.Errorf("仓库模式不应自动建文件")
	}
	if len(got) != 0 {
		t.Errorf("列表应保持为空，got %d", len(got))
	}
}

// 同名文件已被别的源占用时不能抢（会覆盖别人的规则文件）。
func TestEnsureDirectSourceFileSkipsNameTakenByOtherSource(t *testing.T) {
	svc, db := newBuiltinTestService(t)
	ctx := context.Background()

	other := &entity.RuleSource{ServerID: 0, Name: "main", RepoURL: "Loyalsoldier/v2ray-rules-dat", Status: "idle"}
	if err := db.Create(other).Error; err != nil {
		t.Fatalf("create other source: %v", err)
	}
	if err := db.Create(&entity.GeoRuleFile{
		ServerID: 0, SourceID: other.ID, Name: "geoip.dat", FileName: "geoip.dat",
		DownloadURL: "https://example.com/geoip.dat", Status: "available",
	}).Error; err != nil {
		t.Fatalf("create existing file: %v", err)
	}

	src := &entity.RuleSource{ServerID: 0, Name: "iran", RepoURL: "https://example.com/geoip.dat", Status: "idle"}
	if err := db.Create(src).Error; err != nil {
		t.Fatalf("create direct source: %v", err)
	}

	got, row := svc.ensureDirectSourceFile(ctx, src, nil)
	if row != nil {
		t.Errorf("名字被占用时不应补建")
	}
	if len(got) != 0 {
		t.Errorf("列表应保持为空，got %d", len(got))
	}
}
