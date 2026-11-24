package dnsrecord

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	secretcrypto "corefusion/internal/security/crypto"
	"corefusion/internal/store/repository"
)

// fakeCF 模拟 Cloudflare 的 zone / A 记录接口，记录收到的写请求。
type fakeCF struct {
	records map[string]string // name → content
	puts    int
	posts   int
	deletes int
}

func (f *fakeCF) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/zones", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		writeJSON(w, map[string]any{"success": true, "result": []map[string]any{
			{"id": "z-" + name, "name": name},
		}})
	})
	mux.HandleFunc("/zones/", func(w http.ResponseWriter, r *http.Request) {
		// /zones/{id}/dns_records 或 /zones/{id}/dns_records/{rec}
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/zones/"), "/")
		if len(parts) >= 2 && parts[1] == "dns_records" {
			switch r.Method {
			case http.MethodGet:
				var out []map[string]any
				for name, content := range f.records {
					out = append(out, map[string]any{"id": "r-" + name, "name": name, "type": "A", "content": content})
				}
				writeJSON(w, map[string]any{"success": true, "result": out})
				return
			case http.MethodPost, http.MethodPut:
				var body struct {
					Name    string `json:"name"`
					Content string `json:"content"`
				}
				_ = json.NewDecoder(r.Body).Decode(&body)
				f.records[body.Name] = body.Content
				if r.Method == http.MethodPost {
					f.posts++
				} else {
					f.puts++
				}
				writeJSON(w, map[string]any{"success": true, "result": map[string]any{
					"id": "r-" + body.Name, "name": body.Name, "content": body.Content,
				}})
				return
			case http.MethodDelete:
				f.deletes++
				writeJSON(w, map[string]any{"success": true, "result": map[string]any{"id": "deleted"}})
				return
			}
		}
		writeJSON(w, map[string]any{"success": false, "errors": []map[string]any{{"code": 7003, "message": "not found"}}})
	})
	return mux
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func newTestChannel(t *testing.T) (*repository.DNSChannelRepository, int64) {
	t.Helper()
	if err := secretcrypto.SetKey("00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"); err != nil {
		t.Fatalf("set key: %v", err)
	}
	raw, _ := json.Marshal(map[string]string{"CF_Token": "test-token"})
	enc, err := secretcrypto.EncryptSecret(base64.StdEncoding.EncodeToString(raw))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite 不可用（跳过）：%v", err)
	}
	if err := db.AutoMigrate(&entity.DNSChannel{}); err != nil {
		t.Skipf("迁移失败（跳过）：%v", err)
	}
	repo := repository.NewDNSChannelRepository(db)
	ch := &entity.DNSChannel{Name: "cf-test", Provider: "cloudflare", EnvVars: enc, Status: "active"}
	if err := repo.Create(context.Background(), ch); err != nil {
		t.Fatalf("创建通道失败：%v", err)
	}
	return repo, ch.ID
}

// 记录不存在 → 创建；再跑一次 → 更新；都指向新 IP。
func TestUpsertACloudflareCreateThenUpdate(t *testing.T) {
	repo, chID := newTestChannel(t)
	cf := &fakeCF{records: map[string]string{}}
	srv := httptest.NewServer(cf.handler())
	defer srv.Close()
	svc := NewService(repo).WithAPIBase(srv.URL)

	ctx := context.Background()
	if err := svc.UpsertA(ctx, chID, "entry.tycng.com", "192.168.2.175", false); err != nil {
		t.Fatalf("首次 upsert：%v", err)
	}
	if cf.posts != 1 || cf.puts != 0 {
		t.Fatalf("首次应 POST 一次，实际 posts=%d puts=%d", cf.posts, cf.puts)
	}
	if got := cf.records["entry.tycng.com"]; got != "192.168.2.175" {
		t.Fatalf("content = %q", got)
	}

	if err := svc.UpsertA(ctx, chID, "entry.tycng.com", "192.168.2.176", false); err != nil {
		t.Fatalf("二次 upsert：%v", err)
	}
	if cf.puts != 1 || cf.posts != 1 {
		t.Fatalf("二次应 PUT 一次，实际 posts=%d puts=%d", cf.posts, cf.puts)
	}
	if got := cf.records["entry.tycng.com"]; got != "192.168.2.176" {
		t.Fatalf("更新后 content = %q", got)
	}
}

// 非 IPv4 / 空记录名 / 未验证服务商都要明确报错。
func TestUpsertARejectsInvalidInput(t *testing.T) {
	repo, chID := newTestChannel(t)
	svc := NewService(repo)
	ctx := context.Background()

	if err := svc.UpsertA(ctx, chID, "entry.tycng.com", "not-an-ip", false); err == nil {
		t.Error("非法 IP 应报错")
	}
	if err := svc.UpsertA(ctx, chID, "", "192.168.2.175", false); err == nil {
		t.Error("空记录名应报错")
	}
	if err := svc.UpsertA(ctx, 0, "entry.tycng.com", "192.168.2.175", false); err == nil {
		t.Error("未选通道应报错")
	}
}

// zone 候选从最精确到最宽，支持子 zone（ugigc.dpdns.org）。
func TestZoneCandidates(t *testing.T) {
	got := zoneCandidates("entry.ugigc.dpdns.org")
	want := []string{"ugigc.dpdns.org", "dpdns.org"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
