package clientauth

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/auth/password"
	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// 到期账号必须能登录/续签去续费（否则永远无法续费）；只有管理员封禁才拒绝。
func TestExpiredAccountCanLoginToRenew(t *testing.T) {
	ctx := context.Background()
	d, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "clients.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := d.AutoMigrate(&entity.Client{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	clientRepo := repository.NewClientRepository(d)
	svc := NewService(clientRepo, nil, nil, nil, "expired-login-test-secret")

	hash, err := password.Hash("Test#123456")
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name       string
		client     *entity.Client
		wantTokens bool
	}{
		{
			name:       "status=expired（退款/试用到期）",
			client:     &entity.Client{Email: "expired-a@test.local", Password: hash, Status: "expired"},
			wantTokens: true,
		},
		{
			name:       "suspended + auto_expired（配额巡检到期）",
			client:     &entity.Client{Email: "expired-b@test.local", Password: hash, Status: "suspended", SuspendReason: "auto_expired"},
			wantTokens: true,
		},
		{
			name: "历史遗留 suspended（reason 为空）+ 已过期",
			client: &entity.Client{
				Email: "legacy-expired@test.local", Password: hash, Status: "suspended",
				ExpiresAt: func() *time.Time { t := time.Now().Add(-30 * 24 * time.Hour); return &t }(),
			},
			wantTokens: true,
		},
		{
			name: "历史遗留 suspended（reason 为空）+ 未过期（按封禁处理）",
			client: &entity.Client{
				Email: "legacy-suspended@test.local", Password: hash, Status: "suspended",
				ExpiresAt: func() *time.Time { t := time.Now().Add(30 * 24 * time.Hour); return &t }(),
			},
			wantTokens: false,
		},
		{
			name:       "suspended + admin（管理员封禁）",
			client:     &entity.Client{Email: "banned@test.local", Password: hash, Status: "suspended", SuspendReason: "admin"},
			wantTokens: false,
		},
		{
			name:       "suspended + auto_quota（超额，走自助解封，不发常规 token）",
			client:     &entity.Client{Email: "quota@test.local", Password: hash, Status: "suspended", SuspendReason: "auto_quota"},
			wantTokens: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := clientRepo.Create(ctx, tc.client); err != nil {
				t.Fatalf("create client: %v", err)
			}
			res, err := svc.Login(ctx, LoginInput{Email: tc.client.Email, Password: "Test#123456"})
			if !tc.wantTokens {
				if err == nil && (res == nil || res.AccessToken == "") {
					return // 自助解封等分支不发常规 token，也算正确
				}
				if err == nil {
					t.Fatal("封禁账号不应拿到常规 token")
				}
				return
			}
			if err != nil {
				t.Fatalf("到期账号应能登录：%v", err)
			}
			if res == nil || res.AccessToken == "" || res.RefreshToken == "" {
				t.Fatalf("应签发 token，得到 %+v", res)
			}
			// 到期账号也要能续签（否则 24h 后又被踢出，续不了费）
			if _, err := svc.RefreshToken(ctx, res.RefreshToken); err != nil {
				t.Fatalf("到期账号应能刷新 token：%v", err)
			}
			// 门户与其它接口据此判断「已到期」，而不是「已封禁」
			me, err := svc.GetMe(ctx, tc.client.ID)
			if err != nil {
				t.Fatalf("GetMe: %v", err)
			}
			if me.Status != "expired" {
				t.Fatalf("门户状态 = %q, want expired", me.Status)
			}
		})
	}
}

// 封禁账号刷新 token 仍然要被拦住（旧 refresh 不能无限续命）。
func TestBannedAccountCannotRefresh(t *testing.T) {
	ctx := context.Background()
	d, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "clients.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := d.AutoMigrate(&entity.Client{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	clientRepo := repository.NewClientRepository(d)
	svc := NewService(clientRepo, nil, nil, nil, "expired-login-test-secret")
	hash, _ := password.Hash("Test#123456")

	client := &entity.Client{Email: "refresh-banned@test.local", Password: hash, Status: "active"}
	if err := clientRepo.Create(ctx, client); err != nil {
		t.Fatal(err)
	}
	res, err := svc.Login(ctx, LoginInput{Email: client.Email, Password: "Test#123456"})
	if err != nil {
		t.Fatal(err)
	}
	client.Status = "suspended"
	client.SuspendReason = "admin"
	if err := clientRepo.Update(ctx, client); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RefreshToken(ctx, res.RefreshToken); err == nil {
		t.Fatal("封禁账号不应能刷新 token")
	}
}
