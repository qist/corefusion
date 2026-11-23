package repository

import (
	"context"
	"path/filepath"
	"testing"

	"corefusion/internal/model/entity"

	"github.com/qist/sqlite"
	"gorm.io/gorm"
)

// 删节点后不能留下孤儿证书关联：绑定行要删，issuer 指针要清，
// 但证书本身和其它节点的有效绑定必须保留。启动修复需幂等。
func TestRepairOrphanNodeCertificates(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "repair.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&entity.NodeServer{}, &entity.NodeCertificate{}, &entity.Certificate{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// 节点 1 存在；99 已删除。
	if err := db.Create(&entity.NodeServer{ID: 1, Name: "alive", Hostname: "127.0.0.1"}).Error; err != nil {
		t.Fatalf("seed node: %v", err)
	}
	for _, nc := range []entity.NodeCertificate{
		{NodeServerID: 1, CertificateID: 10},
		{NodeServerID: 99, CertificateID: 10},
	} {
		if err := db.Create(&nc).Error; err != nil {
			t.Fatalf("seed node_certificate: %v", err)
		}
	}
	for _, c := range []entity.Certificate{
		{ID: 10, Name: "shared", IssuerNodeID: 1},
		{ID: 11, Name: "from-deleted", IssuerNodeID: 99},
	} {
		if err := db.Create(&c).Error; err != nil {
			t.Fatalf("seed certificate: %v", err)
		}
	}

	repo := NewNodeCertificateRepository(db)
	n, err := repo.RepairOrphanNodeCertificates(context.Background())
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	if n != 2 {
		t.Fatalf("repaired = %d, want 2 (1 delete + 1 update)", n)
	}

	var links []entity.NodeCertificate
	db.Find(&links)
	if len(links) != 1 || links[0].NodeServerID != 1 {
		t.Fatalf("绑定清理错误: %+v", links)
	}
	var cert11 entity.Certificate
	if err := db.First(&cert11, 11).Error; err != nil {
		t.Fatalf("load cert 11: %v", err)
	}
	if cert11.IssuerNodeID != 0 {
		t.Fatalf("cert 11 issuer_node_id = %d, want 0", cert11.IssuerNodeID)
	}
	var cert10 entity.Certificate
	if err := db.First(&cert10, 10).Error; err != nil {
		t.Fatalf("load cert 10: %v", err)
	}
	if cert10.IssuerNodeID != 1 {
		t.Fatalf("cert 10 issuer 被误清: %d", cert10.IssuerNodeID)
	}

	if n2, err := repo.RepairOrphanNodeCertificates(context.Background()); err != nil || n2 != 0 {
		t.Fatalf("第二次修复应为 0，实际 n=%d err=%v", n2, err)
	}
}
