package nodeserver

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

func newStandbyTest(t *testing.T) (*StandbyReconciler, *repository.NodeServerRepository, int64, int64, *int) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("内存 SQLite 不可用（跳过）：%v", err)
	}
	if err := db.AutoMigrate(&entity.NodeServer{}, &entity.InboundConfig{}, &entity.CoreConfig{},
		&entity.NginxSite{}, &entity.CaddySite{}); err != nil {
		t.Skipf("迁移失败（跳过）：%v", err)
	}
	ctx := context.Background()
	serverRepo := repository.NewNodeServerRepository(db)

	primary := &entity.NodeServer{Name: "p", Hostname: "p", NodeType: entity.NodeTypePrimaryBackup,
		FailoverRole: entity.FailoverRolePrimary, Enabled: true}
	backup := &entity.NodeServer{Name: "b", Hostname: "b", NodeType: entity.NodeTypePrimaryBackup,
		FailoverRole: entity.FailoverRoleBackup, Enabled: true}
	if err := serverRepo.Create(ctx, primary); err != nil {
		t.Fatal(err)
	}
	if err := serverRepo.Create(ctx, backup); err != nil {
		t.Fatal(err)
	}
	primary.PartnerServerID = backup.ID
	backup.PartnerServerID = primary.ID
	_ = serverRepo.Update(ctx, primary)
	_ = serverRepo.Update(ctx, backup)

	pushed := 0
	r := NewStandbyReconciler(
		serverRepo,
		repository.NewInboundConfigRepository(db),
		repository.NewCoreConfigRepository(db),
		repository.NewNginxSiteRepository(db),
		repository.NewCaddySiteRepository(db),
		nil, // settingRepo=nil → 默认开启
		t.TempDir(),
		func(context.Context, int64) error { pushed++; return nil },
	)
	return r, serverRepo, primary.ID, backup.ID, &pushed
}

// 主备 reconcile：主有备无 → 建；两边都有 → 改；主无备有 → 删；主节点数据不动。
func TestStandbyReconcileInbounds(t *testing.T) {
	r, _, primaryID, backupID, pushed := newStandbyTest(t)
	ctx := context.Background()

	mk := func(serverID int64, tag, cfg string) *entity.InboundConfig {
		return &entity.InboundConfig{ServerID: serverID, CoreType: "xray", Protocol: "vless",
			Tag: tag, Listen: "0.0.0.0", Port: 10000, ConfigJSON: cfg, Enabled: true}
	}
	// 主：A（新增）、B（内容 v2）
	for _, c := range []*entity.InboundConfig{
		mk(primaryID, "A", `{"protocol":"vless","tag":"A"}`),
		mk(primaryID, "B", `{"protocol":"vless","tag":"B","v":2}`),
	} {
		if err := r.inboundRepo.Create(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	// 备：B（旧内容 v1）、C（多余）
	for _, c := range []*entity.InboundConfig{
		mk(backupID, "B", `{"protocol":"vless","tag":"B","v":1}`),
		mk(backupID, "C", `{"protocol":"vless","tag":"C"}`),
	} {
		if err := r.inboundRepo.Create(ctx, c); err != nil {
			t.Fatal(err)
		}
	}

	if err := r.ReconcileForPrimary(ctx, primaryID); err != nil {
		t.Fatalf("reconcile 失败：%v", err)
	}

	got, _ := r.inboundRepo.ListByServerID(ctx, backupID)
	byTag := map[string]entity.InboundConfig{}
	for _, c := range got {
		byTag[c.Tag] = c
	}
	if _, ok := byTag["A"]; !ok {
		t.Error("主有备无的入站 A 没有创建")
	}
	if _, ok := byTag["C"]; ok {
		t.Error("主无备有的入站 C 没有删除")
	}
	if b, ok := byTag["B"]; !ok {
		t.Error("入站 B 丢失")
	} else if want := `"v":2`; !contains(b.ConfigJSON, want) {
		t.Errorf("入站 B 未更新到主的内容：%s", b.ConfigJSON)
	}
	if *pushed != 1 {
		t.Errorf("应推送 1 次，实际 %d", *pushed)
	}

	// 主节点自身不应被改动。
	mainList, _ := r.inboundRepo.ListByServerID(ctx, primaryID)
	if len(mainList) != 2 {
		t.Errorf("主节点入站数被改动：%d", len(mainList))
	}
}

// 快照目录要落在 data/backups/failover 下（回滚依据）。
func TestStandbyReconcileWritesSnapshot(t *testing.T) {
	r, _, primaryID, backupID, _ := newStandbyTest(t)
	ctx := context.Background()
	if err := r.inboundRepo.Create(ctx, &entity.InboundConfig{
		ServerID: backupID, CoreType: "xray", Protocol: "vless", Tag: "old",
		Listen: "0.0.0.0", Port: 10001, ConfigJSON: `{"protocol":"vless","tag":"old"}`, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := r.ReconcileForPrimary(ctx, primaryID); err != nil {
		t.Fatalf("reconcile 失败：%v", err)
	}
	files, _ := filepath.Glob(filepath.Join(r.dataDir, "backups", "failover", "server-*.json"))
	if len(files) == 0 {
		t.Fatal("未生成配置快照")
	}
	if _, err := os.Stat(files[0]); err != nil {
		t.Fatalf("快照文件不可读：%v", err)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
