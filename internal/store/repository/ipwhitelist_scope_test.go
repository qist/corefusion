package repository

import (
	"context"
	"testing"

	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("内存 SQLite 不可用（跳过）：%v", err)
	}
	if err := db.AutoMigrate(&entity.IPWhitelistEntry{}); err != nil {
		t.Skipf("迁移失败（跳过）：%v", err)
	}
	return db
}

// 安全审计 v5 M1：删除白名单条目必须限定 client_id —— 否则任何已登录客户端遍历 id
// 就能删掉**别人**的白名单条目（越权破坏他人的访问控制）。
func TestDeleteByIDForClientEnforcesOwnership(t *testing.T) {
	db := newTestDB(t)
	repo := NewIPWhitelistEntryRepository(db)
	ctx := context.Background()

	mine := &entity.IPWhitelistEntry{ClientID: 1, IPAddress: "1.1.1.1"}
	others := &entity.IPWhitelistEntry{ClientID: 2, IPAddress: "2.2.2.2"}
	if err := repo.Create(ctx, mine); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, others); err != nil {
		t.Fatal(err)
	}

	// 用"客户端 1"的身份去删"客户端 2"的条目 → 必须失败
	if err := repo.DeleteByIDForClient(ctx, others.ID, 1); err == nil {
		t.Error("跨客户端删除必须被拒（IDOR）")
	}
	// 且那条记录必须**还在**
	var cnt int64
	db.Model(&entity.IPWhitelistEntry{}).Where("id = ?", others.ID).Count(&cnt)
	if cnt != 1 {
		t.Fatalf("他人的白名单条目被删掉了（count=%d）", cnt)
	}

	// 删自己的 → 成功
	if err := repo.DeleteByIDForClient(ctx, mine.ID, 1); err != nil {
		t.Errorf("删自己的条目应成功，得到 %v", err)
	}
	db.Model(&entity.IPWhitelistEntry{}).Where("id = ?", mine.ID).Count(&cnt)
	if cnt != 0 {
		t.Errorf("自己的条目未被删除（count=%d）", cnt)
	}
}
