package services

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

type fakeNodeOperator struct {
	states   map[string]*CoreStateInfo
	binaries []*BinaryInfoItem
}

func (f *fakeNodeOperator) IsLocalServer(context.Context, int64) bool { return false }

func (f *fakeNodeOperator) ControlCore(context.Context, int64, string, string) error { return nil }

func (f *fakeNodeOperator) GetRemoteState(context.Context, int64) (map[string]*CoreStateInfo, []*BinaryInfoItem, error) {
	return f.states, f.binaries, nil
}

// 异常核心明细：看板「异常 N」悬浮时要能看到是哪台节点、哪个核心。
func TestCoreServiceAbnormalServices(t *testing.T) {
	ctx := context.Background()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "cores.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&entity.NodeServer{}, &entity.InboundService{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	node := &entity.NodeServer{Name: "e2e-node", Hostname: "10.0.0.9"}
	if err := db.Create(node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	svc := &entity.InboundService{ServerID: node.ID, CoreType: "xray", Name: "xray"}
	if err := db.Create(svc).Error; err != nil {
		t.Fatalf("create service: %v", err)
	}

	op := &fakeNodeOperator{
		states: map[string]*CoreStateInfo{
			"xray": {State: "crashed"},
		},
		binaries: []*BinaryInfoItem{
			{CoreType: "xray", Installed: true},
		},
	}
	h := &ProcessHandler{
		svcRepo:        repository.NewServiceRepository(db),
		nodeServerRepo: repository.NewNodeServerRepository(db),
		nodeOperator:   op,
	}

	// 直接验证单节点收集逻辑（避免依赖 processMgr 的全局统计入口）
	counts, direct := h.collectServerCores(ctx, *node, []entity.InboundService{*svc})
	if counts.total != 1 || counts.running != 0 || counts.crashed != 1 {
		t.Fatalf("counts total=%d running=%d crashed=%d, want 1/0/1", counts.total, counts.running, counts.crashed)
	}
	if len(direct) != 1 || direct[0].State != "crashed" {
		t.Fatalf("collectServerCores abnormal = %+v, want 1 crashed", direct)
	}

	list := h.CoreServiceAbnormalServices(ctx)
	if len(list) != 1 {
		t.Fatalf("abnormal list = %d, want 1", len(list))
	}
	item := list[0]
	if item["server_name"] != "e2e-node" {
		t.Errorf("server_name = %v, want e2e-node", item["server_name"])
	}
	if item["core_type"] != "xray" {
		t.Errorf("core_type = %v, want xray", item["core_type"])
	}
	if item["state"] != "crashed" {
		t.Errorf("state = %v, want crashed", item["state"])
	}
	if item["server_id"] != node.ID {
		t.Errorf("server_id = %v, want %d", item["server_id"], node.ID)
	}

	// 恢复正常后不应再有异常明细
	op.states["xray"] = &CoreStateInfo{State: "running"}
	if got := h.CoreServiceAbnormalServices(ctx); len(got) != 0 {
		t.Errorf("running 时不应返回异常明细，实际 %d 条", len(got))
	}
}
