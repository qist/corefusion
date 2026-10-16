package portal

import (
	"encoding/json"
	"strings"
	"testing"
)

// 首次启动（库为空）种子出的内置模板必须满足三条口径 ✓：
//  1. 登录页自带 OAuth 第三方登录入口（后台启用哪个 provider 就显示哪个 ✓）；
//  2. 推广邀请（导航 + 模块）默认关闭 ✓（管理员可在编辑器里打开 ✓）；
//  3. 账户模块的"充值"按钮可开关（show_charge_btn ✓）。
func TestBuiltInTemplateDefaults(t *testing.T) {
	// 1. OAuth：主页面兜底 auth 卡 + 默认 auth 区域 + JS 逻辑都要有 ✓
	if !strings.Contains(defaultPortalHTML, "oauth-btn") || !strings.Contains(defaultPortalCSS, ".oauth-btn") {
		t.Fatalf("内置模板（兜底登录卡/CSS）缺少 OAuth 登录入口 ✗")
	}
	if !strings.Contains(defaultPortalJS, "fetchOAuthProviders") || !strings.Contains(defaultPortalJS, "oauth-callback") {
		t.Fatalf("内置 JS 缺少 OAuth providers 拉取/回调逻辑 ✗")
	}
	var region map[string]interface{}
	if err := json.Unmarshal([]byte(defaultAuthRegionJSON), &region); err != nil {
		t.Fatalf("默认 auth 区域 JSON 非法 ✗: %v", err)
	}
	if html, _ := region["html"].(string); !strings.Contains(html, "oauth-btn") {
		t.Fatalf("默认 auth 区域缺少 OAuth 登录按钮 ✗")
	}
	// P3-3：默认 auth 区域（真正渲染的登录卡）必须自带解封卡 + ctx 透传 ✓
	if html, _ := region["html"].(string); !strings.Contains(html, "self-unban-card") || !strings.Contains(html, `v-if="!selfUnban.visible"`) {
		t.Fatalf("默认 auth 区域缺少自助解封卡 ✗")
	}
	if js, _ := region["js"].(string); !strings.Contains(js, "selfUnban: ctx.selfUnban") {
		t.Fatalf("默认 auth 区域 JS 未透传 selfUnban/handleSelfUnban ✗")
	}
	if js, _ := region["js"].(string); !strings.Contains(js, "isOAuthEnabled: ctx.isOAuthEnabled") {
		t.Fatalf("默认 auth 区域 JS 未透传 OAuth 能力 ✗")
	}

	// 2. 推广邀请默认关闭 ✓：导航 enabled=false + 模块 visible=false
	if !strings.Contains(defaultNavigationJSON, `"id":"nav-referral","label":"nav.referral","icon":"🎁","target":"referral","href":"","enabled":false`) {
		t.Fatalf("推广邀请导航应默认关闭 ✗")
	}
	var mods []map[string]interface{}
	if err := json.Unmarshal([]byte(defaultModulesJSON), &mods); err != nil {
		t.Fatalf("默认模块 JSON 非法 ✗: %v", err)
	}
	var referralVisible interface{} = "missing"
	var accountHasChargeToggle bool
	var accountChargeDefaultOff bool
	for _, m := range mods {
		if m["id"] == "mod-referral" {
			referralVisible = m["visible"]
		}
		if m["id"] == "mod-account" {
			if data, ok := m["data"].(map[string]interface{}); ok {
				html, _ := data["html"].(string)
				if strings.Contains(html, "show_charge_btn") {
					accountHasChargeToggle = true
				}
				// 充值按钮默认关闭：data.show_charge_btn=false 且渲染条件要求 === true 才显示 ✓
				if v, ok := data["show_charge_btn"].(bool); ok && !v && strings.Contains(html, "mod.data.show_charge_btn === true") {
					accountChargeDefaultOff = true
				}
			}
		}
	}
	if referralVisible != false {
		t.Fatalf("推广邀请模块应默认隐藏（visible=false）✗，实际 %v", referralVisible)
	}

	// 3. 充值按钮可开关，且默认关闭、后台手动开启才显示 ✓
	if !accountHasChargeToggle {
		t.Fatalf("账户模块充值按钮缺少 show_charge_btn 开关 ✗")
	}
	if !accountChargeDefaultOff {
		t.Fatalf("账户模块充值按钮应默认关闭，需后台手动开启才显示 ✗")
	}

	// 4. 端到端渲染口径 ✓：种子常量 → renderPortalHTML 后，OAuth 入口与"推广邀请默认关"
	//    必须原样到达浏览器（防渲染/注入环节把标记弄丢 ✗）。
	rendered := renderPortalHTML(defaultPortalHTML, defaultPortalCSS, defaultPortalJS, "", MergeConfig(nil, false))
	if !strings.Contains(rendered, "oauth-btn") {
		t.Fatalf("渲染产物缺少 OAuth 登录按钮 ✗")
	}
	if !strings.Contains(rendered, `"id":"nav-referral"`) || !strings.Contains(rendered, `"enabled":false`) {
		t.Fatalf("渲染产物中推广邀请导航未默认关闭 ✗")
	}
	// ⚠ PORTAL_CONFIG 快照经 json.Marshal 会把 RawMessage **压缩**（去空格）✓
	if !strings.Contains(rendered, `"id":"mod-referral"`) || !strings.Contains(rendered, `"visible":false`) {
		t.Fatalf("渲染产物中推广邀请模块未默认隐藏 ✗")
	}
}

// 默认门户模板：订阅链接的「购买套餐」必须随「购买服务」导航开关联动。
func TestDefaultPortalSubStoreActionGate(t *testing.T) {
	if !strings.Contains(defaultModulesJSON, "visibleSubActions(mod.data.actions)") {
		t.Error("默认门户订阅链接动作按钮应使用 visibleSubActions 过滤")
	}
	if !strings.Contains(defaultPortalJS, "var storeEnabled = computed(function() {") {
		t.Error("默认门户 shell 应定义 storeEnabled")
	}
	if !strings.Contains(defaultPortalJS, "var visibleSubActions = function(") {
		t.Error("默认门户 shell 应定义 visibleSubActions")
	}
	if !strings.Contains(defaultPortalJS, "storeEnabled: storeEnabled, visibleSubActions: visibleSubActions") {
		t.Error("默认门户 shell 应在 ext 里透传 storeEnabled/visibleSubActions")
	}
}
