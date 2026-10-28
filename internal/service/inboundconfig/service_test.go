package inboundconfig

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// newTestDB creates an in-memory SQLite database and migrates the schema.
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	if err := db.AutoMigrate(&entity.InboundConfig{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// newTestService builds a Service backed by an in-memory DB. The
// coreconfigSvc dependency is unused by Create/Update and left nil.
func newTestService(t *testing.T) *Service {
	t.Helper()
	return &Service{
		repo: repository.NewInboundConfigRepository(newTestDB(t)),
	}
}

func realityConfig(serverNames ...string) string {
	// build a reality inbound whose serverNames are stored as an array,
	// matching what buildInbound emits.
	quoted := make([]string, 0, len(serverNames))
	for _, s := range serverNames {
		quoted = append(quoted, `"`+s+`"`)
	}
	// 键名必须与真实数据一致：xray 生成的是 streamSettings.realitySettings
	// （历史上这里写成 reality，导致本条测试长期失效，重新对齐）
	return `{"protocol":"vless","streamSettings":{"security":"reality","realitySettings":{"serverNames":[` +
		joinQuoted(quoted) + `]}}}`
}

// singboxRealityConfig 造一个 sing-box reality 入站（SNI 存在 tls.server_name）。
func singboxRealityConfig(serverName string) string {
	return `{"type":"vless","tag":"sb","tls":{"enabled":true,"server_name":"` + serverName +
		`","reality":{"enabled":true,"handshake":{"server":"` + serverName + `","server_port":443}}}}`
}

func joinQuoted(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ","
		}
		out += p
	}
	return out
}

func TestExtractServerNames(t *testing.T) {
	cases := []struct {
		name string
		json string
		want []string
	}{
		{
			name: "array form",
			json: `{"streamSettings":{"security":"reality","realitySettings":{"serverNames":["A.com","b.com","A.com"]}}}`,
			want: []string{"a.com", "b.com"},
		},
		{
			name: "string form newline/comma",
			json: `{"streamSettings":{"security":"reality","realitySettings":{"serverNamesStr":"x.com\nY.com, z.com\nx.com"}}}`,
			want: []string{"x.com", "y.com", "z.com"},
		},
		{
			name: "not reality",
			json: `{"streamSettings":{"security":"tls","realitySettings":{"serverNames":["a.com"]}}}`,
			want: nil,
		},
		{
			name: "singbox reality server_name",
			json: `{"type":"vless","tls":{"enabled":true,"server_name":"A.com","reality":{"enabled":true,"handshake":{"server":"a.com","server_port":443}}}}`,
			want: []string{"a.com"},
		},
		{
			// 非 reality 的 TLS SNI 不算（与 xray 一侧口径一致）
			name: "singbox tls without reality",
			json: `{"type":"vless","tls":{"enabled":true,"server_name":"a.com"}}`,
			want: nil,
		},
		{
			name: "empty",
			json: `{"streamSettings":{"security":"reality"}}`,
			want: nil,
		},
		{
			name: "invalid json",
			json: `not json`,
			want: nil,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := extractServerNames(c.json)
			if len(got) != len(c.want) {
				t.Fatalf("got %v, want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("index %d: got %q want %q", i, got[i], c.want[i])
				}
			}
		})
	}
}

func TestCreateServerNamesConflict(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	if _, err := svc.Create(ctx, 1, "xray", "node-A", "vless", "tag-a", "0.0.0.0", 443, realityConfig("a.com", "b.com"), true); err != nil {
		t.Fatalf("seed create: %v", err)
	}

	// duplicate SNI "b.com" on the same machine -> conflict
	_, err := svc.Create(ctx, 1, "xray", "node-B", "vless", "tag-b", "0.0.0.0", 443, realityConfig("b.com", "c.com"), true)
	if err == nil {
		t.Fatal("expected conflict error, got nil")
	}
	if !errors.Is(err, ErrServerNamesConflict) {
		t.Fatalf("expected ErrServerNamesConflict, got %v", err)
	}

	// disjoint SNI on same machine -> ok
	if _, err := svc.Create(ctx, 1, "xray", "node-C", "vless", "tag-c", "0.0.0.0", 443, realityConfig("d.com"), true); err != nil {
		t.Fatalf("disjoint create should succeed: %v", err)
	}

	// different machine (serverID=2) -> ok even with duplicate SNI
	if _, err := svc.Create(ctx, 2, "xray", "node-D", "vless", "tag-d", "0.0.0.0", 443, realityConfig("b.com"), true); err != nil {
		t.Fatalf("different machine should allow duplicate: %v", err)
	}
}

func TestCreateNonRealitySkipsCheck(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	// non-reality inbound with a "serverNames" that collides should not conflict
	cfg := `{"protocol":"vless","streamSettings":{"security":"tls","reality":{"serverNames":["a.com"]}}}`
	if _, err := svc.Create(ctx, 1, "xray", "n1", "vless", "tag-e", "0.0.0.0", 443, cfg, true); err != nil {
		t.Fatalf("non-reality create: %v", err)
	}
	if _, err := svc.Create(ctx, 1, "xray", "n2", "vless", "tag-f", "0.0.0.0", 443, cfg, true); err != nil {
		t.Fatalf("non-reality should not conflict: %v", err)
	}
}

func TestServerNamesConflictSingbox(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	if _, err := svc.Create(ctx, 1, "singbox", "sb-A", "vless", "sb-a", "0.0.0.0", 443, singboxRealityConfig("a.com"), true); err != nil {
		t.Fatalf("seed singbox: %v", err)
	}
	// 同节点重复 SNI -> 冲突
	if _, err := svc.Create(ctx, 1, "singbox", "sb-B", "vless", "sb-b", "0.0.0.0", 443, singboxRealityConfig("a.com"), true); !errors.Is(err, ErrServerNamesConflict) {
		t.Fatalf("expected conflict for duplicate singbox SNI, got %v", err)
	}
	// 跨节点允许重复
	if _, err := svc.Create(ctx, 2, "singbox", "sb-C", "vless", "sb-c", "0.0.0.0", 443, singboxRealityConfig("a.com"), true); err != nil {
		t.Fatalf("cross-node duplicate should be allowed: %v", err)
	}
	// 同节点内 xray 与 sing-box 共用同一 SNI 也要冲突（同一台机器的 SNI 路由会打架）
	if _, err := svc.Create(ctx, 1, "xray", "x-A", "vless", "x-a", "0.0.0.0", 443, realityConfig("a.com"), true); !errors.Is(err, ErrServerNamesConflict) {
		t.Fatalf("expected cross-core conflict on same node, got %v", err)
	}
}

func TestUpdateLegacyDuplicateUnchangedAllowed(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	mk := func(remark, tag, cfg string) *entity.InboundConfig {
		c := &entity.InboundConfig{
			Remark: remark, CoreType: "xray", Protocol: "vless", Tag: tag,
			Listen: "0.0.0.0", Port: 443, ConfigJSON: cfg, Enabled: true, ServerID: 1,
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		if err := svc.repo.Create(ctx, c); err != nil {
			t.Fatalf("seed %s: %v", tag, err)
		}
		return c
	}
	// 绕过校验直接写库，模拟存量重复（同节点两个入站共用 dup.com）
	a := mk("legacy-A", "l-a", realityConfig("dup.com"))
	_ = mk("legacy-B", "l-b", realityConfig("dup.com", "used-elsewhere.com"))

	// 1) 域名没变（只改备注）-> 存量重复不阻塞
	if _, err := svc.Update(ctx, a.ID, "legacy-A-renamed", "vless", "l-a", "0.0.0.0", 443, realityConfig("dup.com"), true); err != nil {
		t.Fatalf("unchanged duplicate should not block unrelated edit: %v", err)
	}
	// 2) 新增一个已被占用域名 -> 必须拒绝
	if _, err := svc.Update(ctx, a.ID, "legacy-A", "vless", "l-a", "0.0.0.0", 443, realityConfig("dup.com", "used-elsewhere.com"), true); !errors.Is(err, ErrServerNamesConflict) {
		t.Fatalf("adding a conflicting name should be rejected, got %v", err)
	}
}

func TestUpdateServerNamesConflict(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	a, err := svc.Create(ctx, 1, "xray", "node-A", "vless", "tag-a", "0.0.0.0", 443, realityConfig("a.com", "b.com"), true)
	if err != nil {
		t.Fatalf("seed a: %v", err)
	}
	b, err := svc.Create(ctx, 1, "xray", "node-B", "vless", "tag-b", "0.0.0.0", 443, realityConfig("c.com"), true)
	if err != nil {
		t.Fatalf("seed b: %v", err)
	}

	// updating B to steal A's SNI "b.com" -> conflict
	_, err = svc.Update(ctx, b.ID, "node-B", "vless", "tag-b", "0.0.0.0", 443, realityConfig("b.com"), true)
	if err == nil {
		t.Fatal("expected conflict on update, got nil")
	}
	if !errors.Is(err, ErrServerNamesConflict) {
		t.Fatalf("expected ErrServerNamesConflict, got %v", err)
	}

	// updating A to reuse its own SNI -> no conflict (self excluded).
	// Note: must run after the steal attempt above, which leaves A still
	// holding b.com (failed update does not persist).
	if _, err := svc.Update(ctx, a.ID, "node-A", "vless", "tag-a", "0.0.0.0", 443, realityConfig("a.com"), true); err != nil {
		t.Fatalf("self-update should not conflict: %v", err)
	}
}

// TestEnsureDefaultAccessDomain 新建入站默认带「直连 + 开启」，否则订阅渲染会跳过该节点。
func TestEnsureDefaultAccessDomain(t *testing.T) {
	// 1) 完全没配 → 注入默认值
	in := `{"tag":"t","protocol":"vless","port":31080,"settings":{"clients":[]}}`
	out := ensureDefaultAccessDomain(in)
	if !strings.Contains(out, `"_accessDomainEnabled":true`) || !strings.Contains(out, `"_accessDomainType":"direct"`) {
		t.Fatalf("未注入默认接入域名: %s", out)
	}
	// 2) 显式关闭 → 保持原样
	off := `{"tag":"t","protocol":"vless","_accessDomainEnabled":false}`
	if got := ensureDefaultAccessDomain(off); got != off {
		t.Fatalf("显式关闭不应被覆盖: %s", got)
	}
	// 3) 已有对象 → 保持原样
	obj := `{"tag":"t","protocol":"vless","_accessDomain":{"enabled":true}}`
	if got := ensureDefaultAccessDomain(obj); got != obj {
		t.Fatalf("已有 _accessDomain 不应被覆盖: %s", got)
	}
	// 4) 非 xray 形态（无 protocol）→ 不动
	other := `{"inbounds":[{"type":"vless"}]}`
	if got := ensureDefaultAccessDomain(other); got != other {
		t.Fatalf("非 xray 形态不应修改: %s", got)
	}
}
