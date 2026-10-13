package system

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qist/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"corefusion/internal/model/entity"
	serviceSystem "corefusion/internal/service/system"
	"corefusion/internal/store/repository"
)

// newBootstrapHandler 构造一个挂载了 FinishBootstrap 的 system.Handler（内存 SQLite）。
func newBootstrapHandler(t *testing.T) (*Handler, *gorm.DB) {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Skipf("in-memory sqlite unavailable: %v", err)
	}
	if err := gdb.AutoMigrate(&entity.SystemSetting{}, &entity.User{}); err != nil {
		t.Skipf("automigrate failed: %v", err)
	}
	settingSvc := serviceSystem.NewSettingService(
		repository.NewSystemSettingRepository(gdb),
		repository.NewNotificationTemplateRepository(gdb),
	)
	return NewHandler(gdb, settingSvc, t.TempDir(), "test"), gdb
}

// chdirTempDir 把工作目录切到临时目录：UpdatePanelConfig 会写 .env，不能污染仓库。
func chdirTempDir(t *testing.T) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
}

func doBootstrap(t *testing.T, h *Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.PUT("/api/v1/system/bootstrap-finish", h.FinishBootstrap)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/system/bootstrap-finish", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func settingValue(t *testing.T, gdb *gorm.DB, key string) string {
	t.Helper()
	var s entity.SystemSetting
	if err := gdb.Where("`key` = ?", key).First(&s).Error; err != nil {
		t.Fatalf("setting %q not found: %v", key, err)
	}
	return s.Value
}

func TestFinishBootstrapAppliesConfigAndIsSingleUse(t *testing.T) {
	h, gdb := newBootstrapHandler(t)
	chdirTempDir(t)

	body := `{"listen_addr":"127.0.0.1","listen_port":"18080","web_path":"/a7x9k2m8p","subscription_port":"0"}`
	w := doBootstrap(t, h, body)
	if w.Code != http.StatusOK {
		t.Fatalf("first finish: want 200, got %d (%s)", w.Code, w.Body.String())
	}

	// 完成标记、面板配置落库
	if v := settingValue(t, gdb, bootstrapCompletedKey); v != "true" {
		t.Errorf("marker = %q, want true", v)
	}
	if v := settingValue(t, gdb, "panel.web_path"); v != "/a7x9k2m8p" {
		t.Errorf("panel.web_path = %q, want /a7x9k2m8p", v)
	}
	if v := settingValue(t, gdb, "panel.listen_port"); v != "18080" {
		t.Errorf("panel.listen_port = %q, want 18080", v)
	}

	// 数据目录写入 .env 的 SAFE_PATHS
	data, err := os.ReadFile(".env")
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}
	if !strings.Contains(string(data), "SAFE_PATHS=") {
		t.Errorf(".env missing SAFE_PATHS: %q", data)
	}

	// 单次有效：再次调用返回 409
	w2 := doBootstrap(t, h, body)
	if w2.Code != http.StatusConflict {
		t.Fatalf("second finish: want 409, got %d (%s)", w2.Code, w2.Body.String())
	}
}

func TestFinishBootstrapAllowsEmptyBody(t *testing.T) {
	h, gdb := newBootstrapHandler(t)
	chdirTempDir(t)

	w := doBootstrap(t, h, "")
	if w.Code != http.StatusOK {
		t.Fatalf("empty body: want 200, got %d (%s)", w.Code, w.Body.String())
	}
	if v := settingValue(t, gdb, bootstrapCompletedKey); v != "true" {
		t.Errorf("marker = %q, want true", v)
	}
}

func TestFinishBootstrapRejectsInvalidConfig(t *testing.T) {
	cases := []string{
		`{"web_path":"bad path"}`,
		`{"listen_port":"99999"}`,
		`{"listen_port":"abc"}`,
		`{"tls_enabled":true}`,
		`{"listen_addr":""}`,
	}
	for _, body := range cases {
		h, gdb := newBootstrapHandler(t)
		chdirTempDir(t)
		w := doBootstrap(t, h, body)
		if w.Code != http.StatusBadRequest {
			t.Errorf("body %s: want 400, got %d (%s)", body, w.Code, w.Body.String())
		}
		// 校验失败不得写入完成标记
		var n int64
		gdb.Model(&entity.SystemSetting{}).Where("`key` = ?", bootstrapCompletedKey).Count(&n)
		if n != 0 {
			t.Errorf("body %s: marker persisted despite invalid config", body)
		}
	}
}
