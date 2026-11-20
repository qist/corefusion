package charge

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/security/crypto"
	"corefusion/internal/store/repository"
)

func newSiteTestEnv(t *testing.T) (*Handler, *entity.ChargeSite, *repository.ChargeSiteRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "site.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&entity.ChargeSite{}, &entity.ChargeSiteSecret{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	enc, err := crypto.EncryptSecret("real-callback-secret")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	site := &entity.ChargeSite{
		Name: "mock", SiteURL: "http://127.0.0.1:18081", APIKey: "api-key-1",
		EncryptedSecret: enc, SecretVersion: 1, Currency: "CNY",
		SupportedMethods: `["alipay","wechat"]`, Enabled: true, SortOrder: 7,
	}
	repo := repository.NewChargeSiteRepository(db)
	if err := repo.Create(context.Background(), site); err != nil {
		t.Fatalf("create site: %v", err)
	}
	return &Handler{db: db, chargeSiteRepo: repo}, site, repo
}

// 列表开关只应改动 enabled，不能碰密钥/API Key/渠道。
func TestSetSiteEnabledOnlyTogglesFlag(t *testing.T) {
	h, site, repo := newSiteTestEnv(t)
	before, err := repo.GetByID(context.Background(), site.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}

	body, _ := json.Marshal(map[string]interface{}{"enabled": false})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/charge-sites/1/enable", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	h.SetSiteEnabled(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}

	after, err := repo.GetByID(context.Background(), site.ID)
	if err != nil {
		t.Fatalf("reload after: %v", err)
	}
	if after.Enabled {
		t.Error("enabled 应被关闭")
	}
	if after.EncryptedSecret != before.EncryptedSecret {
		t.Error("密钥不应被改动")
	}
	if after.APIKey != before.APIKey || after.SupportedMethods != before.SupportedMethods ||
		after.Currency != before.Currency || after.SortOrder != before.SortOrder {
		t.Error("其它字段不应被改动")
	}
}

// 缺少 enabled 字段应 400，而不是当成 false 静默关闭。
func TestSetSiteEnabledRequiresField(t *testing.T) {
	h, site, _ := newSiteTestEnv(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/charge-sites/1/enable", bytes.NewReader([]byte(`{}`)))
	c.Request.Header.Set("Content-Type", "application/json")
	h.SetSiteEnabled(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
	}
	got, _ := h.chargeSiteRepo.GetByID(context.Background(), site.ID)
	if !got.Enabled {
		t.Error("请求无效时不应改动状态")
	}
}

// 编辑弹窗会把掩码密钥原样回传，必须忽略，否则真实密钥被覆盖 → 回调验签失败。
func TestUpdateSiteIgnoresMaskedSecret(t *testing.T) {
	h, site, repo := newSiteTestEnv(t)
	before, _ := repo.GetByID(context.Background(), site.ID)

	payload := map[string]interface{}{
		"name": "mock", "site_url": "http://127.0.0.1:18081",
		"api_key":           "api-key-1",
		"callback_secret":   "mock****",
		"enabled":           true,
		"supported_methods": `["alipay"]`,
		"currency":          "CNY",
	}
	body, _ := json.Marshal(payload)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/charge-sites/1", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	h.UpdateSite(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}

	after, _ := repo.GetByID(context.Background(), site.ID)
	if after.EncryptedSecret != before.EncryptedSecret {
		t.Fatal("掩码密钥不应覆盖真实密钥")
	}
	if plain, err := crypto.DecryptSecret(after.EncryptedSecret); err != nil || plain != "real-callback-secret" {
		t.Fatalf("密钥应保持原文，实际 %q err=%v", plain, err)
	}

	// 明确传入新明文时才更新
	payload["callback_secret"] = "new-secret"
	body, _ = json.Marshal(payload)
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/charge-sites/1", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	h.UpdateSite(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	after, _ = repo.GetByID(context.Background(), site.ID)
	if plain, err := crypto.DecryptSecret(after.EncryptedSecret); err != nil || plain != "new-secret" {
		t.Fatalf("显式新密钥应生效，实际 %q err=%v", plain, err)
	}
}
