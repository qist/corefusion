package inboundconfig

import (
	"context"
	"errors"
	"testing"

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
	return `{"protocol":"vless","streamSettings":{"security":"reality","reality":{"serverNames":[` +
		joinQuoted(quoted) + `]}}}`
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
			json: `{"streamSettings":{"security":"reality","reality":{"serverNames":["A.com","b.com","A.com"]}}}`,
			want: []string{"a.com", "b.com"},
		},
		{
			name: "string form newline/comma",
			json: `{"streamSettings":{"security":"reality","reality":{"serverNamesStr":"x.com\nY.com, z.com\nx.com"}}}`,
			want: []string{"x.com", "y.com", "z.com"},
		},
		{
			name: "not reality",
			json: `{"streamSettings":{"security":"tls","reality":{"serverNames":["a.com"]}}}`,
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
