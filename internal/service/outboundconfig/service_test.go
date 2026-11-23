package outboundconfig

import (
	"context"
	"testing"

	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

func newOutboundTestService(t *testing.T) *Service {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("内存 SQLite 不可用（跳过）：%v", err)
	}
	if err := db.AutoMigrate(&entity.OutboundConfig{}); err != nil {
		t.Skipf("迁移失败（跳过）：%v", err)
	}
	return NewService(repository.NewOutboundConfigRepository(db))
}

func tagsOf(rows []entity.OutboundConfig) map[string]entity.OutboundConfig {
	out := make(map[string]entity.OutboundConfig, len(rows))
	for _, r := range rows {
		out[r.Tag] = r
	}
	return out
}

// 历史节点没有 direct/blocked 行时，列表要幂等补齐，否则管理端看不到保底出站。
func TestListSeedsXrayDefaultOutbounds(t *testing.T) {
	svc := newOutboundTestService(t)
	ctx := context.Background()

	for i := 0; i < 2; i++ { // 第二次验证幂等
		rows, err := svc.List(ctx, 1, "xray")
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		m := tagsOf(rows)
		for _, tag := range []string{"direct", "blocked"} {
			row, ok := m[tag]
			if !ok {
				t.Fatalf("第 %d 次 list 缺少默认出站 %s（got %v）", i+1, tag, rows)
			}
			if !row.IsDefault || !row.Enabled {
				t.Fatalf("%s 应为 enabled+is_default，实际 %+v", tag, row)
			}
		}
		if len(rows) != 2 {
			t.Fatalf("第 %d 次 list 行数 = %d, want 2（不能重复创建）", i+1, len(rows))
		}
	}
	if m := tagsOf(mustList(t, svc)); m["direct"].Protocol != "freedom" || m["blocked"].Protocol != "blackhole" {
		t.Fatalf("默认出站协议不对: %+v", m)
	}
}

// 用户已经手动建过同名 tag（非 default）时不能覆盖、也不能重复插入。
func TestListKeepsUserDefinedDirectOutbound(t *testing.T) {
	svc := newOutboundTestService(t)
	ctx := context.Background()
	user := &entity.OutboundConfig{
		ServerID: 1, CoreType: "xray", Tag: "direct", Protocol: "freedom",
		ConfigJSON: `{"protocol":"freedom","tag":"direct","settings":{"domainStrategy":"UseIPv4"}}`,
		Enabled:    true, IsDefault: false, SortOrder: 5,
	}
	if err := svc.repo.Create(ctx, user); err != nil {
		t.Fatalf("seed user row: %v", err)
	}
	rows, err := svc.List(ctx, 1, "xray")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	m := tagsOf(rows)
	if d, ok := m["direct"]; !ok || d.IsDefault {
		t.Fatalf("用户自建 direct 不应被替换: %+v", d)
	}
	if _, ok := m["blocked"]; !ok {
		t.Fatalf("blocked 仍应补齐: %v", rows)
	}
	if len(rows) != 2 {
		t.Fatalf("行数 = %d, want 2", len(rows))
	}
	// 补进来的默认出站必须排在用户出站之后：xray 第一个出站是路由默认出口，
	// 插到最前面会静默改掉默认出口。
	if m["blocked"].SortOrder <= user.SortOrder {
		t.Fatalf("补行的 blocked 应排在用户出站之后，sort=%d", m["blocked"].SortOrder)
	}
}

// sing-box 出站存在 section JSON 里，不应被 xray 的补行逻辑影响。
func TestListDoesNotSeedSingbox(t *testing.T) {
	svc := newOutboundTestService(t)
	rows, err := svc.List(context.Background(), 1, "singbox")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("singbox 列表不应被补行: %v", rows)
	}
}

func mustList(t *testing.T, svc *Service) []entity.OutboundConfig {
	t.Helper()
	rows, err := svc.List(context.Background(), 1, "xray")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	return rows
}
