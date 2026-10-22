package portal

import (
	"encoding/json"
	"strings"
	"testing"
)

// 编辑器回传的语言包是「自定义 + 内置兜底」合并后的全量包 —— 保存时必须剥离与
// 内置包逐字相同的文案，否则 16 种语言全量 ≈ 420KB 会撑爆 300KB 配置上限
// （portal config too large ✗）。同时要保证 round-trip：剥离后由内置包补齐，
// 渲染出来的文案与原输入逐键一致 ✓。
func TestCompactBuiltinLangRoundTrip(t *testing.T) {
	full := MergeBuiltinLangFallback(json.RawMessage(defaultMultiLangJSON))
	var before map[string]interface{}
	if err := json.Unmarshal(full, &before); err != nil {
		t.Fatalf("合并后的语言包非法: %v", err)
	}

	compacted := CompactBuiltinLang(full)
	if len(compacted) >= len(full) {
		t.Fatalf("剥离后应变小：%d → %d", len(full), len(compacted))
	}

	// round-trip：剥离后再合并回去，必须与原始全量包等价
	restored := MergeBuiltinLangFallback(compacted)
	var after map[string]interface{}
	if err := json.Unmarshal(restored, &after); err != nil {
		t.Fatalf("还原后的语言包非法: %v", err)
	}
	b, _ := json.Marshal(before)
	a, _ := json.Marshal(after)
	if string(a) != string(b) {
		t.Error("Compact 后 Merge 应等价于原始全量包（round-trip 失败）")
	}

	// 幂等：再剥离一次不应再变化
	again := CompactBuiltinLang(compacted)
	if string(again) != string(compacted) {
		t.Error("CompactBuiltinLang 应幂等")
	}
}

// 管理员改过的文案必须保留；与内置相同的才剥离；_meta 一律保留 ✓。
func TestCompactBuiltinLangKeepsCustomText(t *testing.T) {
	builtin := matchBuiltinLangPack("zh-CN")
	sameKey := "common.confirm"
	sameVal, ok := builtin[sameKey].(string)
	if !ok || sameVal == "" {
		t.Fatalf("内置包缺少 %s，测试前提不成立", sameKey)
	}

	pack := map[string]interface{}{
		"default": "zh-CN",
		"locales": map[string]interface{}{
			"zh-CN": map[string]interface{}{
				sameKey:                 sameVal,
				"common.cancel":         "自定义取消文案",
				"_meta":                 map[string]interface{}{"flag": "🇨🇳", "label": "简体中文", "language": "zh-CN"},
				"custom.admin.only.key": "只存在于模板",
			},
		},
	}
	raw, _ := json.Marshal(pack)
	out := CompactBuiltinLang(raw)

	var root map[string]interface{}
	if err := json.Unmarshal(out, &root); err != nil {
		t.Fatalf("输出非法: %v", err)
	}
	zh := root["locales"].(map[string]interface{})["zh-CN"].(map[string]interface{})
	if _, exists := zh[sameKey]; exists {
		t.Errorf("与内置相同的 %s 应被剥离", sameKey)
	}
	if zh["common.cancel"] != "自定义取消文案" {
		t.Error("自定义文案不得被剥离")
	}
	if zh["custom.admin.only.key"] != "只存在于模板" {
		t.Error("模板独有 key 不得被剥离")
	}
	if _, exists := zh["_meta"]; !exists {
		t.Error("_meta 应保留")
	}
}

// 全量 16 语言包经 ValidatePortalConfig 必须能通过（不再报 portal config too large）。
func TestValidatePortalConfigAcceptsFullBuiltinLang(t *testing.T) {
	full := MergeBuiltinLangFallback(json.RawMessage(defaultMultiLangJSON))
	cfg, _ := json.Marshal(map[string]interface{}{
		"site_name":             "CoreFusion",
		"lang":                  json.RawMessage(full),
		"portal_editor_version": 1,
	})
	if len(cfg) <= maxConfigSize {
		t.Skip("内置全量语言包未超过上限，本用例前提不成立")
	}
	cleaned, err := ValidatePortalConfig(string(cfg))
	if err != nil {
		t.Fatalf("全量内置语言包应能通过校验（剥离后落库）: %v", err)
	}
	if len(cleaned) > maxConfigSize {
		t.Errorf("落库配置仍超限: %d > %d", len(cleaned), maxConfigSize)
	}
	if !strings.Contains(cleaned, `"_meta"`) {
		t.Error("_meta 应保留在落库配置里")
	}
}
