package bootstrap

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/db"
)

func TestIsFirstBoot(t *testing.T) {
	dir := t.TempDir()
	if !IsFirstBoot(map[string]string{}, dir+"/missing") {
		t.Error("no env + no data dir should be first boot")
	}
	if IsFirstBoot(map[string]string{}, dir) {
		t.Error("no env + existing data dir should NOT be first boot")
	}
	if !IsFirstBoot(map[string]string{"CF_DB_DSN": "x"}, dir) {
		t.Error("missing SAFE_PATHS should be first boot")
	}
	if !IsFirstBoot(map[string]string{"SAFE_PATHS": dir}, dir) {
		t.Error("missing CF_DB_DSN should be first boot")
	}
	if IsFirstBoot(map[string]string{"SAFE_PATHS": dir, "CF_DB_DSN": "x"}, dir) {
		t.Error("complete config should NOT be first boot")
	}
}

func TestDraftValidate(t *testing.T) {
	d := NewDraft(t.TempDir())
	if err := d.Validate(); err != nil {
		t.Fatalf("default draft should be valid: %v", err)
	}
	d.WebPath = "bad path"
	if err := d.Validate(); err == nil {
		t.Error("invalid web_path should fail")
	}
	d = NewDraft(t.TempDir())
	d.ListenPort = "70000"
	if err := d.Validate(); err == nil {
		t.Error("invalid port should fail")
	}
	d = NewDraft(t.TempDir())
	d.AdminPassword = "weak"
	if err := d.Validate(); err == nil {
		t.Error("weak password should fail")
	}
	d = NewDraft(t.TempDir())
	d.DBType = "mysql"
	if err := d.Validate(); err == nil {
		t.Error("mysql without host/user/name should fail")
	}
	d = NewDraft(t.TempDir())
	d.Cores = []string{"bogus"}
	if err := d.Validate(); err == nil {
		t.Error("unsupported core should fail")
	}
}

func TestPasswordStrength(t *testing.T) {
	if _, errs := PasswordStrength("abc"); len(errs) == 0 {
		t.Error("weak password should report errors")
	}
	if _, errs := PasswordStrength("Abcdef1!"); len(errs) != 0 {
		t.Errorf("strong password should pass, got %v", errs)
	}
}

func TestSessionSaveLoadExpire(t *testing.T) {
	dir := t.TempDir()
	s := &Session{Port: 18080, Token: "abc", StartTime: time.Now()}
	if err := s.Save(dir); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, ok := LoadSession(dir)
	if !ok || got.Port != 18080 || got.Token != "abc" {
		t.Fatalf("load mismatch: %+v ok=%v", got, ok)
	}
	if got.Expired() {
		t.Error("fresh session should not be expired")
	}
	old := &Session{Port: 1, Token: "x", StartTime: time.Now().Add(-3 * time.Hour)}
	if !old.Expired() {
		t.Error("3h-old session should be expired")
	}
	if _, ok := LoadSession(t.TempDir()); ok {
		t.Error("missing session file should return ok=false")
	}
}

func chdir(t *testing.T) {
	t.Helper()
	wd, _ := os.Getwd()
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
}

func TestComplete(t *testing.T) {
	chdir(t)
	dataDir := t.TempDir()
	d := NewDraft(dataDir)
	d.DataDir = dataDir
	d.ListenPort = "18080"
	d.WebPath = "/a7x9k2m8p"
	d.AdminUsername = "cf_test"
	d.AdminPassword = "StrongPass1!"
	d.PanelDomain = "panel.example.com"
	d.PortalDomain = "portal.example.com"
	d.SubscriptionDomain = "sub.example.com"
	// 选 nginx 且订阅/门户不设独立端口（0）→ 回退面板端口，订阅走 /sub、门户走 /p。
	d.ProxyType = ProxyNginx
	d.SubscriptionPort = "0"
	d.PortalPort = "0"
	d.ProxyAddPanel, d.ProxyAddSubscription, d.ProxyAddPortal = true, true, true

	if err := Complete(context.Background(), dataDir, d); err != nil {
		t.Fatalf("complete: %v", err)
	}

	// 管理员已创建
	var u entity.User
	if err := db.DB.Where("username = ?", "cf_test").First(&u).Error; err != nil {
		t.Errorf("admin user not created: %v", err)
	}
	// 完成标记
	var mark entity.SystemSetting
	if err := db.DB.Where("`key` = ?", BootstrapCompletedKey).First(&mark).Error; err != nil || mark.Value != "true" {
		t.Errorf("marker missing/!=true: %+v err=%v", mark, err)
	}
	// 面板路径 / 监听地址（无证书 → 回环）
	var wp entity.SystemSetting
	if err := db.DB.Where("`key` = ?", "panel.web_path").First(&wp).Error; err != nil || wp.Value != "/a7x9k2m8p" {
		t.Errorf("panel.web_path = %q err=%v", wp.Value, err)
	}
	var la entity.SystemSetting
	if err := db.DB.Where("`key` = ?", "panel.listen_addr").First(&la).Error; err != nil || la.Value != LoopbackAddr {
		t.Errorf("panel.listen_addr = %q, want %q", la.Value, LoopbackAddr)
	}
	// 域名
	var cb entity.SystemSetting
	if err := db.DB.Where("`key` = ?", "oauth.callback_domain").First(&cb).Error; err != nil || cb.Value != "portal.example.com" {
		t.Errorf("oauth.callback_domain = %q err=%v", cb.Value, err)
	}
	var cur entity.SystemSetting
	if err := db.DB.Where("`key` = ?", "currency.default").First(&cur).Error; err != nil || cur.Value != "CNY" {
		t.Errorf("currency.default = %q err=%v", cur.Value, err)
	}
	var ep entity.SubscriptionEndpoint
	if err := db.DB.Where("domain = ?", "sub.example.com").First(&ep).Error; err != nil {
		t.Errorf("subscription endpoint not created: %v", err)
	} else if ep.Path != "/sub" {
		t.Errorf("subscription endpoint path = %q, want /sub (panel serves /sub/:token)", ep.Path)
	}

	// 反代站点写入真实生成器读取的 nginx_sites（不是 proxy_sites），
	// 且只有面板有端口时订阅/门户仍带正确 upstream + path。
	var panelSite entity.NginxSite
	if err := db.DB.Where("domain = ?", "panel.example.com").First(&panelSite).Error; err != nil {
		t.Errorf("panel nginx site not created: %v", err)
	} else if panelSite.Template != "panel_forward" || panelSite.Path != "" || panelSite.Upstream != "127.0.0.1:18080" {
		t.Errorf("panel site mismatch: %+v", panelSite)
	}
	var subSite entity.NginxSite
	if err := db.DB.Where("domain = ?", "sub.example.com").First(&subSite).Error; err != nil {
		t.Errorf("subscription nginx site not created: %v", err)
	} else if subSite.Template != "subscription_forward" || subSite.Path != "/sub" || subSite.Upstream != "127.0.0.1:18080" {
		t.Errorf("subscription site mismatch: %+v", subSite)
	}
	var portalSite entity.NginxSite
	if err := db.DB.Where("domain = ?", "portal.example.com").First(&portalSite).Error; err != nil {
		t.Errorf("portal nginx site not created: %v", err)
	} else if portalSite.Template != "portal_forward" || portalSite.Path != "/p" || portalSite.Upstream != "127.0.0.1:18080" {
		t.Errorf("portal site mismatch: %+v", portalSite)
	}
	// .env 写入 SAFE_PATHS 与 CF_DB_DSN 与密钥
	raw, err := os.ReadFile(".env")
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}
	s := string(raw)
	if !strings.Contains(s, "SAFE_PATHS="+dataDir) || !strings.Contains(s, "CF_DB_DSN=") ||
		!strings.Contains(s, "JWT_SECRET=") || !strings.Contains(s, "CF_SECRET_ENCRYPTION_KEY=") {
		t.Errorf(".env missing keys: %s", s)
	}
	db.Close()
}
