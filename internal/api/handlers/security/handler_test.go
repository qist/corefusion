package security

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

func newSecurityTestHandler(t *testing.T) *Handler {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("内存 SQLite 不可用（跳过）：%v", err)
	}
	if err := db.AutoMigrate(
		&entity.User{}, &entity.Client{}, &entity.Subscription{},
		&entity.SubscriptionAccessLog{}, &entity.SecurityAuditEvent{},
	); err != nil {
		t.Skipf("迁移失败（跳过）：%v", err)
	}
	return NewHandler(db)
}

type auditItem struct {
	ID        int64  `json:"id"`
	UserName  string `json:"user_name"`
	UserEmail string `json:"user_email"`
}

// 审计日志的用户列以前只查 users 表：客户端事件（user_id 是 clients.id）只能显示 "#id"，
// 也无法按客户邮箱查询。这里验证邮箱筛选与角色解析（管理端/客户端/metadata 兜底）。
func TestListAuditEventsByEmail(t *testing.T) {
	h := newSecurityTestHandler(t)
	db := h.db

	if err := db.Create(&entity.User{ID: 1, Username: "admin", Email: "admin@example.com", PasswordHash: "x"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&entity.Client{ID: 2, Email: "sub@example.com", Nickname: "Bob"}).Error; err != nil {
		t.Fatal(err)
	}
	events := []entity.SecurityAuditEvent{
		{Category: "admin", EventType: "admin_login", UserID: ptrInt64(1), Result: "success"},
		{Category: "auth", EventType: "client_login", UserID: ptrInt64(2), Result: "success"},
		{Category: "auth", EventType: "client_login", Result: "failure", MetadataJSON: `{"email":"sub@example.com"}`},
	}
	if err := db.Create(&events).Error; err != nil {
		t.Fatal(err)
	}

	got := requestAuditEvents(t, h, "email=sub@example.com")
	if got.Total != 2 {
		t.Fatalf("按客户邮箱应命中 2 条（含 metadata 兜底），得到 %d: %s", got.Total, got.Body)
	}
	for _, it := range got.Items {
		if it.UserEmail != "sub@example.com" {
			t.Errorf("事件 %d 的用户邮箱应为 sub@example.com，得到 %q（name=%q）", it.ID, it.UserEmail, it.UserName)
		}
		if it.UserName != "Bob" && it.UserName != "sub@example.com" {
			t.Errorf("事件 %d 的用户名应解析为 Bob 或邮箱，得到 %q", it.ID, it.UserName)
		}
	}

	adminGot := requestAuditEvents(t, h, "email=admin@example.com")
	if adminGot.Total != 1 || len(adminGot.Items) != 1 {
		t.Fatalf("按管理员邮箱应命中 1 条，得到 %d", adminGot.Total)
	}
	if adminGot.Items[0].UserName != "admin" || adminGot.Items[0].UserEmail != "admin@example.com" {
		t.Errorf("管理员事件解析错误: %+v", adminGot.Items[0])
	}
}

type auditResp struct {
	Items []auditItem
	Total int64
	Body  string
}

func requestAuditEvents(t *testing.T, h *Handler, query string) auditResp {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/audit", h.ListAuditEvents)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/audit?"+query, nil))
	if w.Code != 200 {
		t.Fatalf("audit 返回 %d: %s", w.Code, w.Body.String())
	}
	var payload struct {
		Data struct {
			Items []auditItem `json:"items"`
			Total int64       `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("解析 audit 响应失败: %v (%s)", err, w.Body.String())
	}
	return auditResp{Items: payload.Data.Items, Total: payload.Data.Total, Body: w.Body.String()}
}

// 订阅访问日志以前只能按订阅 ID/IP 查，这里验证按客户邮箱筛选。
func TestListSubscriptionAccessLogsByEmail(t *testing.T) {
	h := newSecurityTestHandler(t)
	db := h.db

	if err := db.Create(&entity.Client{ID: 1, Email: "a@example.com"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&entity.Client{ID: 2, Email: "b@example.com"}).Error; err != nil {
		t.Fatal(err)
	}
	subs := []entity.Subscription{
		{ID: 10, ClientID: 1, Token: "t1", Format: "clash"},
		{ID: 20, ClientID: 2, Token: "t2", Format: "clash"},
	}
	if err := db.Create(&subs).Error; err != nil {
		t.Fatal(err)
	}
	logs := []entity.SubscriptionAccessLog{
		{SubscriptionID: 10, IP: "1.1.1.1", StatusCode: 200, HitResult: "ok"},
		{SubscriptionID: 20, IP: "2.2.2.2", StatusCode: 200, HitResult: "ok"},
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/sub-logs", h.ListSubscriptionAccessLogs)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/sub-logs?email=a@example.com", nil))
	if w.Code != 200 {
		t.Fatalf("返回 %d: %s", w.Code, w.Body.String())
	}
	var payload struct {
		Data struct {
			Items []struct {
				SubscriptionID int64  `json:"subscription_id"`
				ClientEmail    string `json:"client_email"`
			} `json:"items"`
			Total int64 `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if payload.Data.Total != 1 || len(payload.Data.Items) != 1 {
		t.Fatalf("按邮箱应命中 1 条，得到 total=%d", payload.Data.Total)
	}
	if payload.Data.Items[0].SubscriptionID != 10 || payload.Data.Items[0].ClientEmail != "a@example.com" {
		t.Errorf("命中的日志不对: %+v", payload.Data.Items[0])
	}
}

func ptrInt64(v int64) *int64 { return &v }
