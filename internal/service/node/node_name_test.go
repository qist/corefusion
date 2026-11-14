package node

import (
	"testing"

	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// 在线用户的 serverID 是 node_servers 的主键；旧实现查的是 nodes 表，
// 查不到就直接显示 "#<id>"，面板上只能看到数字。
func TestNodeNameUsesNodeServerTable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("内存 SQLite 不可用（跳过）：%v", err)
	}
	if err := db.AutoMigrate(&entity.Node{}, &entity.NodeServer{}); err != nil {
		t.Skipf("迁移失败（跳过）：%v", err)
	}
	if err := db.Create(&entity.NodeServer{ID: 3, Name: "香港-01", Hostname: "hk1"}).Error; err != nil {
		t.Fatal(err)
	}

	svc := NewService(repository.NewNodeRepository(db), nil, nil, nil, nil)
	svc.SetNodeServerRepo(repository.NewNodeServerRepository(db))
	if got := svc.nodeName(3); got != "香港-01" {
		t.Errorf("nodeName(3) = %q, want 香港-01", got)
	}
	if got := svc.nodeName(99); got != "#99" {
		t.Errorf("未知节点应回退为 #id，得到 %q", got)
	}
}
