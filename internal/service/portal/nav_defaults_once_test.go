package portal

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// fakeSettingStore 是 SettingStore 的内存实现。
type fakeSettingStore struct {
	vals map[string]string
}

func (f *fakeSettingStore) GetByKey(_ context.Context, key string) (string, error) {
	if f.vals == nil {
		return "", nil
	}
	return f.vals[key], nil
}

func (f *fakeSettingStore) SetByKey(_ context.Context, key, value string) error {
	if f.vals == nil {
		f.vals = map[string]string{}
	}
	f.vals[key] = value
	return nil
}

func navConfig(t *testing.T, enabledStore, enabledBilling bool) string {
	t.Helper()
	cfg := map[string]interface{}{
		"navigation": []interface{}{
			map[string]interface{}{"id": "nav-home", "target": "overview", "enabled": true},
			map[string]interface{}{"id": "nav-services", "target": "store", "enabled": enabledStore},
			map[string]interface{}{"id": "nav-billing", "target": "billing", "enabled": enabledBilling},
		},
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

func navEnabled(t *testing.T, config string, id string) bool {
	t.Helper()
	var cfg struct {
		Navigation []struct {
			ID      string `json:"id"`
			Enabled bool   `json:"enabled"`
		} `json:"navigation"`
	}
	if err := json.Unmarshal([]byte(config), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, n := range cfg.Navigation {
		if n.ID == id {
			return n.Enabled
		}
	}
	t.Fatalf("navigation %s not found", id)
	return false
}

// 导航「购买服务 / 我的账单」默认关闭迁移只跑一次：管理员之后打开，重启不会再被关回去。
func TestNavDefaultsOffRunsOnlyOnce(t *testing.T) {
	ctx := context.Background()
	store := &fakeSettingStore{}
	svc := NewService(nil).WithSettingStore(store)

	// ① 首次启动：把已开启的 nav.store / nav.billing 关掉
	out, changed := svc.applyNavDefaultsOffOnce(ctx, navConfig(t, true, true))
	if !changed {
		t.Fatal("首次迁移应修改配置")
	}
	if navEnabled(t, out, "nav-services") || navEnabled(t, out, "nav-billing") {
		t.Fatal("首次迁移应把 nav.store / nav.billing 置为关闭")
	}
	if store.vals[navDefaultsOffDoneKey] != "1" {
		t.Fatal("迁移完成后应写入 system_settings 标记")
	}

	// ② 管理员在编辑器里打开这两项
	adminOn := navConfig(t, true, true)

	// ③ 再次启动（补丁重跑）：必须原样保留，不能再关掉
	out2, changed2 := svc.applyNavDefaultsOffOnce(ctx, adminOn)
	if changed2 {
		t.Fatal("已迁移过就不应再修改导航（否则重启会覆盖管理员的开启）")
	}
	if !navEnabled(t, out2, "nav-services") || !navEnabled(t, out2, "nav-billing") {
		t.Fatal("管理员已打开的导航不应被重启流程关闭")
	}
}

// 账户模块「显示余额行 / 显示充值按钮」默认关闭：旧条件升级为显式开启才显示，缺键补 false。
func TestPatchChargeButtonDefaultsBalanceAndButtonOff(t *testing.T) {
	html := `<div class="acct-info-row" v-if="mod.data && mod.data.show_balance !== false">余额</div>
<el-button v-if="!mod.data || mod.data.show_charge_btn !== false">充值</el-button>`
	cfg := map[string]interface{}{
		"modules": []interface{}{
			map[string]interface{}{
				"id":   "mod-account",
				"type": "account_info",
				"data": map[string]interface{}{"html": html},
			},
		},
	}
	raw, _ := json.Marshal(cfg)

	out, changed := PatchChargeButtonConfig(string(raw))
	if !changed {
		t.Fatal("应发生修改")
	}
	var parsed struct {
		Modules []struct {
			Data map[string]interface{} `json:"data"`
		} `json:"modules"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	data := parsed.Modules[0].Data
	if v, ok := data["show_balance"].(bool); !ok || v {
		t.Errorf("show_balance 应补为 false，实际 %v", data["show_balance"])
	}
	if v, ok := data["show_charge_btn"].(bool); !ok || v {
		t.Errorf("show_charge_btn 应补为 false，实际 %v", data["show_charge_btn"])
	}
	gotHTML, _ := data["html"].(string)
	if !strings.Contains(gotHTML, `mod.data.show_balance === true`) {
		t.Error("余额行条件应升级为「显式开启才显示」")
	}
	if !strings.Contains(gotHTML, `mod.data.show_charge_btn === true`) {
		t.Error("充值按钮条件应升级为「显式开启才显示」")
	}

	// 管理员显式开启的值不能被覆盖（幂等）
	cfg2 := map[string]interface{}{
		"modules": []interface{}{
			map[string]interface{}{
				"id":   "mod-account",
				"type": "account_info",
				"data": map[string]interface{}{
					"html":            gotHTML,
					"show_balance":    true,
					"show_charge_btn": true,
				},
			},
		},
	}
	raw2, _ := json.Marshal(cfg2)
	if _, changed2 := PatchChargeButtonConfig(string(raw2)); changed2 {
		t.Fatal("已显式开启且条件已升级时不应再修改")
	}
}

// 内置默认模板本身必须是「默认关闭」。
func TestBuiltinDefaultAccountModuleDefaultsOff(t *testing.T) {
	var mods struct {
		Modules []struct {
			Type string                 `json:"type"`
			Data map[string]interface{} `json:"data"`
		} `json:"modules"`
	}
	var modules []interface{}
	if err := json.Unmarshal([]byte(defaultModulesJSON), &modules); err != nil {
		t.Fatalf("内置默认模块不是合法 JSON: %v", err)
	}
	wrapped, _ := json.Marshal(map[string]interface{}{"modules": modules})
	if err := json.Unmarshal(wrapped, &mods); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}
	found := false
	for _, m := range mods.Modules {
		if m.Type != "account_info" {
			continue
		}
		found = true
		if v, ok := m.Data["show_balance"].(bool); !ok || v {
			t.Errorf("默认模板 show_balance 应为 false，实际 %v", m.Data["show_balance"])
		}
		if v, ok := m.Data["show_charge_btn"].(bool); !ok || v {
			t.Errorf("默认模板 show_charge_btn 应为 false，实际 %v", m.Data["show_charge_btn"])
		}
		html, _ := m.Data["html"].(string)
		if strings.Contains(html, "show_balance !== false") {
			t.Error("默认模板余额行仍使用旧条件")
		}
		if !strings.Contains(html, "mod.data.show_balance === true") {
			t.Error("默认模板余额行应为「显式开启才显示」")
		}
	}
	if !found {
		t.Fatal("默认模板缺少 account_info 模块")
	}
}
