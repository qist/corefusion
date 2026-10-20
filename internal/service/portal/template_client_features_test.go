package portal

import (
	"encoding/json"
	"strings"
	"testing"
)

// 存量模板必须补上 C-1/C-2/C-4：支付后轮询、推荐提现、白名单管理。
// 这里用各补丁锚点拼一份"旧模板"，验证一次补齐且第二次调用不再改动。
func TestPatchClientFeaturesAppliesAndIsIdempotent(t *testing.T) {
	js := "var withdrawRecords = ref([])\n" +
		clientFeatureJSAnchor + "\n" +
		clientFeatureReferralOld + "\n" +
		"      if (initHasReferral) fetchReferral()\n" +
		clientFeaturePageModsOld + "\n" +
		clientFeaturePageFetchOld + "\n" +
		clientFeatureUnmountOld + "\n" +
		clientFeatureModuleCtxOld + "\n" +
		clientFeatureExtOld + "\n" +
		clientFeatureReturnOld + "\n" +
		"window.open(res.payment_url, '_blank')\n"
	html := "    <!-- More Subscriptions Dialog -->\n"

	newHTML, newJS, changed := PatchClientFeatures(html, js)
	if !changed {
		t.Fatal("含旧写法的模板应被判定为需要修改")
	}
	for _, want := range []string{
		"var showWithdrawDialog = ref(false)",
		"var startPaymentPolling = function",
		"startPaymentPolling(res)",
		"/referral/withdrawals",
		"'security_settings' })) fetchWhitelistEntries()",
		"var hasSecurity = pageMods.some",
		"if (hasSecurity) fetchWhitelistEntries()",
		clientFeatureModuleCtxNew,
		clientFeatureExtNew,
		clientFeatureReturnNew,
	} {
		if !strings.Contains(newJS, want) {
			t.Errorf("JS 缺少补丁内容: %s", want)
		}
	}
	if !strings.Contains(newHTML, "<!-- Withdraw Dialog -->") {
		t.Error("shell HTML 应补提现弹窗")
	}
	// 原有内容不得被破坏
	for _, keep := range []string{"var withdrawRecords = ref([])", "window.open(res.payment_url, '_blank')"} {
		if !strings.Contains(newJS, keep) {
			t.Errorf("原有代码被破坏: %s", keep)
		}
	}
	if strings.Count(newJS, "startPaymentPolling(res)") != 1 {
		t.Error("支付跳转后只应追加一次轮询调用")
	}

	// 幂等
	html3, js3, changed2 := PatchClientFeatures(newHTML, newJS)
	if changed2 {
		t.Error("第二次调用不应再判定为需要修改")
	}
	if html3 != newHTML || js3 != newJS {
		t.Error("第二次调用不应改动任何内容")
	}
}

// 保守性：没有锚点的自定义模板一字不动。
func TestPatchClientFeaturesLeavesCustomTemplateAlone(t *testing.T) {
	html := "<div class=\"custom\">hello</div>\n"
	js := "var custom = 1\n"
	newHTML, newJS, changed := PatchClientFeatures(html, js)
	if changed {
		t.Error("无锚点的自定义模板不应被改动")
	}
	if newHTML != html || newJS != js {
		t.Error("内容必须原样返回")
	}
}

// config JSON 里的模块 HTML 补丁：提现按钮绑定 + 白名单列表；幂等。
func TestPatchClientFeaturesConfig(t *testing.T) {
	cfg := map[string]interface{}{
		"modules": []interface{}{
			map[string]interface{}{
				"id":   "mod-referral",
				"data": map[string]interface{}{"html": clientFeatureWithdrawBtnOld},
			},
			map[string]interface{}{
				"id":   "mod-sec",
				"data": map[string]interface{}{"html": "x\n" + clientFeatureSecAnchor + "\ny"},
			},
		},
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out, changed := PatchClientFeaturesConfig(string(raw))
	if !changed {
		t.Fatal("旧模块 HTML 应被判定为需要修改")
	}
	if !strings.Contains(out, "openWithdrawDialog") {
		t.Error("提现按钮应绑定 openWithdrawDialog")
	}
	if !strings.Contains(out, "security.ip_whitelist_list") {
		t.Error("安全模块应补白名单列表")
	}
	again, changed2 := PatchClientFeaturesConfig(out)
	if changed2 {
		t.Error("第二次调用不应再判定为需要修改")
	}
	if again != out {
		t.Error("第二次调用不应改动 config")
	}
}

// 默认模板（新装/重置时种子进库的那份）也必须自带这些功能。
func TestDefaultPortalTemplateHasClientFeatures(t *testing.T) {
	for _, want := range []string{
		"var startPaymentPolling = function",
		"startPaymentPolling(res)",
		"var openWithdrawDialog = function",
		"var submitWithdraw = function",
		"var fetchWhitelistEntries = function",
		"if (hasSecurity) fetchWhitelistEntries()",
	} {
		if !strings.Contains(defaultPortalJS, want) {
			t.Errorf("默认门户 JS 缺少: %s", want)
		}
	}
	if strings.Count(defaultPortalJS, "startPaymentPolling(res)") != 3 {
		t.Error("三处支付跳转（下单/升级/充值）都应启动轮询")
	}
	if !strings.Contains(defaultPortalHTML, "<!-- Withdraw Dialog -->") {
		t.Error("默认门户 HTML 应包含提现弹窗")
	}
	if !strings.Contains(defaultModulesJSON, `@click=\"openWithdrawDialog\"`) {
		t.Error("默认推荐模块的提现按钮应绑定 openWithdrawDialog")
	}
	if !strings.Contains(defaultModulesJSON, "security.ip_whitelist_list") {
		t.Error("默认安全模块应包含白名单列表")
	}
}
