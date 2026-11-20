package portal

import (
	"strings"
	"testing"
)

// 线上门户模板存在数据库里，旧模板的流量包支付 payload 不带 gift_card_code，
// 后端会报 "gift_card_code is required for gift_card payment"（流量包卡完全不可用）。
// PatchGiftCardPreview 必须把卡号补进 payload，并加上卡号预览。
func TestPatchGiftCardPreview(t *testing.T) {
	html := `<el-input v-model="orderForm.gift_card_code" :placeholder="t('plan.card_code_placeholder')" prefix-icon="Ticket" clearable />`
	js := `      var apiPayload = isTrafficPack ? {
        variant_id: orderForm.value.variant_id,
        pay_method: orderForm.value.pay_method,
        charge_site_id: orderForm.value.pay_method === 'charge_site' ? orderForm.value.charge_site_id : null
      } : payload

                purchaseVariant: purchaseVariant, submitOrder: submitOrder,
    // get plan name
    var getPlanName = function(planId) { return '' }`

	newHTML, newJS, changed := PatchGiftCardPreview(html, js)
	if !changed {
		t.Fatal("旧模板应命中补丁")
	}
	if !strings.Contains(newHTML, `@change="applyGiftCardPreview"`) {
		t.Error("卡号输入框应绑定预览事件")
	}
	if !strings.Contains(newJS, `gift_card_code: orderForm.value.pay_method === 'gift_card' ? orderForm.value.gift_card_code : null`) {
		t.Error("流量包支付 payload 缺少 gift_card_code（就是这次报错的根因）")
	}
	if !strings.Contains(newJS, `var applyGiftCardPreview = function`) {
		t.Error("缺少卡号预览函数")
	}
	if !strings.Contains(newJS, `applyGiftCardPreview: applyGiftCardPreview`) {
		t.Error("预览函数未透传到模板作用域，@change 会报未定义")
	}
	// 注入的代码必须在新版 getPlanName 之前，且语法结构未被破坏
	if strings.Index(newJS, "var applyGiftCardPreview") > strings.Index(newJS, "var getPlanName") {
		t.Error("预览函数应插在 getPlanName 之前")
	}

	// 幂等：第二次不应再改
	_, again, changed2 := PatchGiftCardPreview(newHTML, newJS)
	if changed2 {
		t.Fatal("重复打补丁不应再修改")
	}
	if again != newJS {
		t.Fatal("幂等结果应完全一致")
	}
}

// 已是新版的模板（已带卡号与预览）不能被重复注入。
func TestPatchGiftCardPreviewSkipsPatchedTemplate(t *testing.T) {
	html := `<el-input v-model="orderForm.gift_card_code" :placeholder="t('plan.card_code_placeholder')" prefix-icon="Ticket" clearable @change="applyGiftCardPreview" />`
	js := `      var apiPayload = isTrafficPack ? {
        variant_id: orderForm.value.variant_id,
        pay_method: orderForm.value.pay_method,
        charge_site_id: orderForm.value.pay_method === 'charge_site' ? orderForm.value.charge_site_id : null,
        gift_card_code: orderForm.value.pay_method === 'gift_card' ? orderForm.value.gift_card_code : null
      } : payload
    var applyGiftCardPreview = function() {}
    purchaseVariant: purchaseVariant, submitOrder: submitOrder, applyGiftCardPreview: applyGiftCardPreview,
    // get plan name`

	newHTML, newJS, changed := PatchGiftCardPreview(html, js)
	if changed {
		t.Fatal("新版模板不应被修改")
	}
	if newHTML != html || newJS != js {
		t.Fatal("未命中时内容必须原样返回")
	}
}
