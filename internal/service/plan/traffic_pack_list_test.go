package plan

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// 「流量叠加包」列表只能返回 variant_type=traffic_pack：
// 旧实现直接列全部变体，套餐规格也会出现，点「推荐」会被 UpdateTrafficPack
// 的类型校验拒绝（variant is not a traffic pack）。
func TestListTrafficPacksPagedExcludesPlanVariants(t *testing.T) {
	ctx := context.Background()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "plans.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&entity.PlanVariant{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	variantRepo := repository.NewPlanVariantRepository(db)

	planVariant := &entity.PlanVariant{TemplateID: 3, VariantType: entity.VariantTypePlan, Name: "流量叠加", Price: 500, Enabled: true}
	packA := &entity.PlanVariant{TemplateID: 0, VariantType: entity.VariantTypeTrafficPack, Name: "10G", Price: 500, Enabled: true}
	packB := &entity.PlanVariant{TemplateID: 0, VariantType: entity.VariantTypeTrafficPack, Name: "100G", Price: 990, Enabled: true}
	for _, v := range []*entity.PlanVariant{planVariant, packA, packB} {
		if err := variantRepo.Create(ctx, v); err != nil {
			t.Fatalf("create variant: %v", err)
		}
	}

	svc := &Service{variantRepo: variantRepo}
	items, total, err := svc.ListTrafficPacksPaged(ctx, 50, 0)
	if err != nil {
		t.Fatalf("ListTrafficPacksPaged: %v", err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2（只统计 traffic_pack）", total)
	}
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	for _, v := range items {
		if v.VariantType != entity.VariantTypeTrafficPack {
			t.Fatalf("流量包列表混入了 %s 类型：id=%d name=%s", v.VariantType, v.ID, v.Name)
		}
	}

	// 分页也要按过滤后的集合生效
	page, total, err := svc.ListTrafficPacksPaged(ctx, 1, 1)
	if err != nil {
		t.Fatalf("paged: %v", err)
	}
	if total != 2 || len(page) != 1 {
		t.Fatalf("分页结果 total=%d len=%d, want total=2 len=1", total, len(page))
	}

	// ListAllVariantsPaged 保持「全部变体」语义（供内部使用）
	all, allTotal, err := svc.ListAllVariantsPaged(ctx, 50, 0)
	if err != nil {
		t.Fatalf("ListAllVariantsPaged: %v", err)
	}
	if allTotal != 3 || len(all) != 3 {
		t.Fatalf("全部变体应为 3，得到 total=%d len=%d", allTotal, len(all))
	}
}
