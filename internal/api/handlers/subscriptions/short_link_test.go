package subscriptions

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	shortlinkSvc "corefusion/internal/service/shortlink"
	"corefusion/internal/store/repository"
)

type shortLinkTestEnv struct {
	db      *gorm.DB
	subRepo *repository.SubscriptionRepository
	setRepo *repository.SystemSettingRepository
	linkSvc *shortlinkSvc.Service
	handler *PublicHandler
	sub     *entity.Subscription
}

func newShortLinkTestEnv(t *testing.T) *shortLinkTestEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "shortlink_handler.db")), &gorm.Config{})
	if err != nil {
		t.Skipf("SQLite 不可用（跳过）：%v", err)
	}
	if err := db.AutoMigrate(
		&entity.Subscription{},
		&entity.SubscriptionShortLink{},
		&entity.SystemSetting{},
		&entity.InboundConfig{},
	); err != nil {
		t.Skipf("迁移失败（跳过）：%v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	subRepo := repository.NewSubscriptionRepository(db)
	setRepo := repository.NewSystemSettingRepository(db)
	linkSvc := shortlinkSvc.NewService(
		repository.NewSubscriptionShortLinkRepository(db),
		subRepo,
		nil,
		nil,
		nil,
		nil,
		setRepo,
	)
	sub := &entity.Subscription{
		ClientID:     0,
		Format:       "base64_uri",
		Token:        "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		TokenVersion: 1,
		Status:       "active",
		SecurityMode: "token_only",
		BindMode:     "none",
		QuotaState:   "normal",
	}
	if err := subRepo.Create(context.Background(), sub); err != nil {
		t.Fatal(err)
	}
	return &shortLinkTestEnv{
		db:      db,
		subRepo: subRepo,
		setRepo: setRepo,
		linkSvc: linkSvc,
		handler: &PublicHandler{
			subRepo:           subRepo,
			inboundRepo:       repository.NewInboundConfigRepository(db),
			systemSettingRepo: setRepo,
			shortSvc:          linkSvc,
		},
		sub: sub,
	}
}

func (e *shortLinkTestEnv) enable(t *testing.T, enabled bool) {
	t.Helper()
	if err := e.linkSvc.SaveSettings(context.Background(), shortlinkSvc.Settings{
		Enabled: enabled, AutoCreate: true, FollowTokenRotation: false,
	}); err != nil {
		t.Fatal(err)
	}
}

func (e *shortLinkTestEnv) createLink(t *testing.T, format string) *entity.SubscriptionShortLink {
	t.Helper()
	link, err := e.linkSvc.Create(context.Background(), shortlinkSvc.CreateInput{
		SubscriptionID: e.sub.ID,
		Format:         format,
	})
	if err != nil {
		t.Fatal(err)
	}
	return link
}

func (e *shortLinkTestEnv) request(code string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/sub/"+code, nil)
	c.Params = gin.Params{{Key: "token", Value: code}}
	e.handler.ServeSubscription(c)
	return w
}

func TestShortLinkDisabledReturns404(t *testing.T) {
	env := newShortLinkTestEnv(t)
	link := env.createLink(t, "base64_uri")
	env.enable(t, false)

	w := env.request(link.Code)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "" {
		t.Fatalf("must not redirect, Location = %q", loc)
	}
}

func TestShortLinkServesContentDirectly(t *testing.T) {
	env := newShortLinkTestEnv(t)
	link := env.createLink(t, "base64_uri")
	env.enable(t, true)

	w := env.request(link.Code)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if loc := w.Header().Get("Location"); loc != "" {
		t.Fatalf("must not redirect, Location = %q", loc)
	}
	if got := w.Header().Get("X-Robots-Tag"); got != "noindex" {
		t.Fatalf("X-Robots-Tag = %q", got)
	}
	if got := w.Header().Get("Cache-Control"); got == "" {
		t.Fatal("missing Cache-Control")
	}
}

func TestShortLinkUnknownCodeReturns404(t *testing.T) {
	env := newShortLinkTestEnv(t)
	env.enable(t, true)
	w := env.request("0123456789abcdefghjk")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestShortLinkInvalidAfterTokenRotation(t *testing.T) {
	env := newShortLinkTestEnv(t)
	link := env.createLink(t, "base64_uri")
	env.enable(t, true)

	env.sub.Token = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
	env.sub.TokenVersion = 2
	if err := env.subRepo.Update(context.Background(), env.sub); err != nil {
		t.Fatal(err)
	}
	w := env.request(link.Code)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 after rotation", w.Code)
	}
}

func TestShortLinkFollowsTokenRotationWhenConfigured(t *testing.T) {
	env := newShortLinkTestEnv(t)
	link, err := env.linkSvc.Create(context.Background(), shortlinkSvc.CreateInput{
		SubscriptionID:      env.sub.ID,
		Format:              "base64_uri",
		FollowTokenRotation: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	env.enable(t, true)
	// 模拟 Rotate：先改订阅版本，再同步短链快照（生产代码由 subscription.Service 调用）。
	env.sub.TokenVersion = 2
	if err := env.subRepo.Update(context.Background(), env.sub); err != nil {
		t.Fatal(err)
	}
	if err := env.linkSvc.SyncTokenVersion(context.Background(), env.sub.ID, 2); err != nil {
		t.Fatal(err)
	}
	w := env.request(link.Code)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
}

func TestShortLinkInactiveSubscriptionReturns403(t *testing.T) {
	env := newShortLinkTestEnv(t)
	link := env.createLink(t, "base64_uri")
	env.enable(t, true)

	env.sub.Status = "revoked"
	now := time.Now()
	env.sub.RevokedAt = &now
	if err := env.subRepo.Update(context.Background(), env.sub); err != nil {
		t.Fatal(err)
	}
	w := env.request(link.Code)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

func TestShortLinkDisabledLinkReturns404(t *testing.T) {
	env := newShortLinkTestEnv(t)
	link := env.createLink(t, "base64_uri")
	env.enable(t, true)

	link.Enabled = false
	if err := env.linkSvc.Update(context.Background(), link); err != nil {
		t.Fatal(err)
	}
	w := env.request(link.Code)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

// Provider 回源地址必须继续用短码，不能把真实 token 写进订阅响应。
func TestShortLinkProviderURLDoesNotLeakToken(t *testing.T) {
	env := newShortLinkTestEnv(t)
	// provider 回源地址只依赖短码，与短链格式无关；mihomo_yaml 现在必须绑配置。
	link := env.createLink(t, "base64_uri")
	env.enable(t, true)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/sub/"+link.Code, nil)
	c.Request.Header.Set("X-Forwarded-Proto", "https")
	c.Request.Header.Set("X-Forwarded-Host", "panel.example.com")

	got := env.handler.buildProviderURL(c, link.Code, "group-1", serveOptions{
		configName: "office",
		platform:   "",
		credential: link.Code,
	})
	want := "https://panel.example.com/sub/" + link.Code + "?format=mihomo_proxies_yaml&config=office&provider=group-1"
	if got != want {
		t.Fatalf("provider url = %q, want %q", got, want)
	}
	if strings.Contains(got, env.sub.Token) {
		t.Fatalf("provider url leaked token: %q", got)
	}
}

func TestLongTokenStillWorks(t *testing.T) {
	env := newShortLinkTestEnv(t)
	env.enable(t, true)
	w := env.request(env.sub.Token)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
}
