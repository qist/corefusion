package clientauth

import (
	"context"
	"path/filepath"
	"testing"

	"corefusion/internal/auth/password"
	"corefusion/internal/model/entity"
	"corefusion/internal/store/db"
	"corefusion/internal/store/repository"
)

// 审计 v5 M8 明确标注的遗留项：**客户端侧**（Portal）的登出/刷新没有纳入令牌版本机制 ✗ ——
// 门户"退出登录"只清前端本地存储，服务端仍旧认那个 token（被窃取的 access/refresh 在有效期内
// 照样可用）。本测试用**真实 sqlite**跑一遍完整链路：登录 → 登出 → 旧 token 全失效 → 重登可用。
func TestClientLogoutRevokesAllTokens(t *testing.T) {
	dir := t.TempDir()
	// 真实 sqlite + AutoMigrate（同时验证 clients.token_version 这列能被自动建出来 ✓）
	if err := db.Init(db.Config{Type: db.DBTypeSQLite, DSN: filepath.Join(dir, "app.db")}); err != nil {
		t.Fatalf("db.Init: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	// db.Init 只负责连接；建表在 AutoMigrateOnDB（这一步同时验证 clients.token_version 这列能建出来）
	if err := db.AutoMigrateOnDB(db.DB); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}

	clientRepo := repository.NewClientRepository(db.DB)
	svc := NewService(clientRepo, nil, nil, nil, "client-logout-test-secret")
	svc.clientRepo = clientRepo

	ctx := context.Background()
	hash, err := password.Hash("s3cret-pw")
	if err != nil {
		t.Fatal(err)
	}
	client := &entity.Client{Email: "revoke@example.com", Password: hash, Status: "active"}
	if err := clientRepo.Create(ctx, client); err != nil {
		t.Fatalf("create client: %v", err)
	}

	// ① 登录 → token 可用
	tokens, err := svc.Login(ctx, LoginInput{Email: client.Email, Password: "s3cret-pw"})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if id, err := svc.ValidateClientAccessToken(tokens.AccessToken); err != nil || id != client.ID {
		t.Fatalf("登录后 access token 应可用：id=%d err=%v", id, err)
	}

	// ② 登出（版本 +1）
	if err := svc.Logout(ctx, client.ID); err != nil {
		t.Fatalf("logout: %v", err)
	}

	// ③ 旧 access 立刻失效（旧实现里它还能继续用 ✗）
	if _, err := svc.ValidateClientAccessToken(tokens.AccessToken); err == nil {
		t.Error("登出后旧 access token 必须立即失效")
	}

	// ④ 旧 refresh 也不能续签（否则"登出"等于没登出 ✗）
	if _, err := svc.RefreshToken(ctx, tokens.RefreshToken); err == nil {
		t.Error("登出后旧 refresh token 不得再续签")
	}

	// ⑤ 版本号确实 +1（可观测，便于排查）
	if v, err := clientRepo.GetTokenVersion(ctx, client.ID); err != nil || v != 1 {
		t.Errorf("token_version 应为 1，实际 %d (err=%v)", v, err)
	}

	// ⑥ 重新登录拿到的新 token 可用（不能把正常用户锁死 ✗）
	tokens2, err := svc.Login(ctx, LoginInput{Email: client.Email, Password: "s3cret-pw"})
	if err != nil {
		t.Fatalf("重新登录失败：%v", err)
	}
	if _, err := svc.ValidateClientAccessToken(tokens2.AccessToken); err != nil {
		t.Errorf("重新登录后的 token 应可用：%v", err)
	}
	if _, err := svc.RefreshToken(ctx, tokens2.RefreshToken); err != nil {
		t.Errorf("重新登录后的 refresh 应可用：%v", err)
	}
}

// 登出后**同一张 access token** 不能靠"再登出一次"复活；且不同客户端互不影响。
func TestClientLogoutIsPerClient(t *testing.T) {
	dir := t.TempDir()
	if err := db.Init(db.Config{Type: db.DBTypeSQLite, DSN: filepath.Join(dir, "app.db")}); err != nil {
		t.Fatalf("db.Init: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.AutoMigrateOnDB(db.DB); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}

	clientRepo := repository.NewClientRepository(db.DB)
	svc := NewService(clientRepo, nil, nil, nil, "client-logout-test-secret")
	svc.clientRepo = clientRepo
	ctx := context.Background()

	hash, _ := password.Hash("pw12345678")
	a := &entity.Client{Email: "a@example.com", Password: hash, Status: "active"}
	b := &entity.Client{Email: "b@example.com", Password: hash, Status: "active"}
	if err := clientRepo.Create(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := clientRepo.Create(ctx, b); err != nil {
		t.Fatal(err)
	}

	tokensA, err := svc.Login(ctx, LoginInput{Email: a.Email, Password: "pw12345678"})
	if err != nil {
		t.Fatal(err)
	}
	tokensB, err := svc.Login(ctx, LoginInput{Email: b.Email, Password: "pw12345678"})
	if err != nil {
		t.Fatal(err)
	}

	// 只登出 A
	if err := svc.Logout(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ValidateClientAccessToken(tokensA.AccessToken); err == nil {
		t.Error("A 登出后其 token 应失效")
	}
	// B 必须不受影响（版本是 per-client 的 ✓）
	if _, err := svc.ValidateClientAccessToken(tokensB.AccessToken); err != nil {
		t.Errorf("登出 A 不应影响 B：%v", err)
	}
}
