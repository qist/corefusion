package subscriptions

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// 失败访问也要落库：以前只有成功下发才写 subscription_access_logs，
// token 过期/暂停/未知 token 在管理端完全看不到（"curl 拉了但日志里没记录"）。
func TestLogDeniedAccessRecordsFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// 用临时文件库而不是 :memory:（连接池下 :memory: 每个连接一个库）。
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "subaccess.db")), &gorm.Config{})
	if err != nil {
		t.Skipf("SQLite 不可用（跳过）：%v", err)
	}
	if err := db.AutoMigrate(&entity.SubscriptionAccessLog{}); err != nil {
		t.Skipf("迁移失败（跳过）：%v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("取连接池失败：%v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	h := &PublicHandler{accessLog: repository.NewAccessLogRepository(db)}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/sub/bad-token", nil)
	c.Request.Header.Set("X-Real-IP", "203.0.113.9")
	c.Request.Header.Set("User-Agent", "curl/8.18.0")

	h.logDeniedAccess(c, 0, http.StatusNotFound, "not_found")

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var row entity.SubscriptionAccessLog
		if err := db.Order("id DESC").First(&row).Error; err == nil {
			if row.StatusCode != http.StatusNotFound {
				t.Fatalf("status_code = %d, want 404", row.StatusCode)
			}
			if row.HitResult != "not_found" {
				t.Fatalf("hit_result = %q, want not_found", row.HitResult)
			}
			if row.RiskLevel != "high" {
				t.Fatalf("risk_level = %q, want high", row.RiskLevel)
			}
			// 无可信代理时 X-Real-IP 是任意来源都能伪造的 → 必须忽略；
			// 三个 IP 字段统一落在 TCP 直连地址上（展示层用 ip）。
			if row.IP != "192.0.2.1" {
				t.Fatalf("ip = %q, want 192.0.2.1", row.IP)
			}
			if row.RealIP != "192.0.2.1" {
				t.Fatalf("real_ip = %q, want the trusted-proxy-aware IP", row.RealIP)
			}
			if row.CDNIP != "" {
				t.Fatalf("cdn_ip = %q, want empty for a direct request", row.CDNIP)
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("失败访问没有落库")
}

// 风险分级：未知 token / 限流最高，其它 4xx 次之，成功且无 UA 记 medium。
func TestSubscriptionAccessRisk(t *testing.T) {
	cases := []struct {
		status   int
		hit      string
		ua       string
		expected string
	}{
		{200, "hit", "clash-verge/2.0", "low"},
		{200, "hit", "", "medium"},
		{404, "not_found", "curl/8", "high"},
		{429, "rate_limited", "curl/8", "high"},
		{403, "expired", "clash-verge/2.0", "medium"},
		{403, "suspended", "", "medium"},
	}
	for _, tc := range cases {
		if got := subscriptionAccessRisk(tc.status, tc.hit, tc.ua); got != tc.expected {
			t.Errorf("status=%d hit=%s ua=%q => %s, want %s", tc.status, tc.hit, tc.ua, got, tc.expected)
		}
	}
}
