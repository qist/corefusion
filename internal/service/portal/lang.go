package portal

import (
	"embed"
	"encoding/json"
	"path"
	"reflect"
	"strings"
	"sync"
)

// 内置门户语言包按语言拆分为独立文件（locales/<code>.json），编译时嵌入。
// 新增一种语言只需新增 locales/<code>.json 文件，无需改动任何代码 ✓。
//
//go:embed locales/*.json
var localesFS embed.FS

// defaultLangCode 是默认语言码（内置语言包的基准 / 缺省兜底）。
const defaultLangCode = "zh-CN"

// builtinLangPacks 缓存内置语言包（code → key→value，解析一次）。
var (
	builtinLangPacksOnce sync.Once
	builtinLangPacks     map[string]map[string]interface{}
)

// loadBuiltinLangRaw 读取全部内置语言包原文（code → JSON 文本），
// code 取文件名（去 .json 后缀）。
func loadBuiltinLangRaw() map[string]string {
	raw := map[string]string{}
	entries, err := localesFS.ReadDir("locales")
	if err != nil {
		return raw
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := localesFS.ReadFile(path.Join("locales", e.Name()))
		if err != nil {
			continue
		}
		raw[strings.TrimSuffix(e.Name(), ".json")] = string(b)
	}
	return raw
}

func builtinLangPacksMap() map[string]map[string]interface{} {
	builtinLangPacksOnce.Do(func() {
		m := map[string]map[string]interface{}{}
		for code, raw := range loadBuiltinLangRaw() {
			pack := map[string]interface{}{}
			if json.Unmarshal([]byte(raw), &pack) == nil {
				m[code] = pack
			}
		}
		builtinLangPacks = m
	})
	return builtinLangPacks
}

// defaultMultiLangJSON 由内置语言包文件在启动时组装：
// {"default":"zh-CN","locales":{"zh-CN":{...},"en":{...}}}。
var defaultMultiLangJSON = buildDefaultMultiLangJSON()

func buildDefaultMultiLangJSON() string {
	raw := loadBuiltinLangRaw()
	locales := map[string]json.RawMessage{}
	for code, body := range raw {
		if json.Valid([]byte(body)) {
			locales[code] = json.RawMessage(body)
		}
	}
	if _, ok := locales[defaultLangCode]; !ok {
		locales[defaultLangCode] = json.RawMessage(`{}`)
	}
	b, err := json.Marshal(map[string]interface{}{
		"default": defaultLangCode,
		"locales": locales,
	})
	if err != nil {
		return `{"default":"` + defaultLangCode + `","locales":{"` + defaultLangCode + `":{}}}`
	}
	return string(b)
}

// DefaultLangCode 返回内置默认语言码（zh-CN）。
func DefaultLangCode() string { return defaultLangCode }

// BuiltinLangPacks 返回全部内置语言包（code → key→value）。
// 供后台「文案」tab 做缺失 key 检测与一键补齐。
func BuiltinLangPacks() map[string]map[string]interface{} {
	return builtinLangPacksMap()
}

// BuiltinHasLangKey 判断内置语言包里是否已有该 key（按与 matchBuiltinLangPack
// 相同的匹配规则找语言包）。供「补键」补丁跳过内置已提供的文案 ✓ —— 否则会给
// ja/ko 等语言补上英文兜底串，反而覆盖掉内置的正确译文 ✗。
func BuiltinHasLangKey(code, key string) bool {
	pack := matchBuiltinLangPack(code)
	if pack == nil {
		return false
	}
	v, ok := pack[key]
	return ok && v != nil && v != ""
}

// matchBuiltinLangPack 按语言码匹配内置语言包：精确 → 前缀（zh-*→zh-CN、en-*→en）
// → 兜底 defaultLangCode。
func matchBuiltinLangPack(code string) map[string]interface{} {
	packs := builtinLangPacksMap()
	if pack, ok := packs[code]; ok {
		return pack
	}
	prefix := strings.Split(code, "-")[0]
	for c, pack := range packs {
		if strings.Split(c, "-")[0] == prefix {
			return pack
		}
	}
	return packs[defaultLangCode]
}

// MergeBuiltinLangFallback 把内置语言包作为**兜底层**合并进模板自定义语言包：
// 自定义 key 一律优先，仅缺失的 key 用内置补齐 —— 新增内置 key（如自助解封
// auth.self_unban_*）无需动存量模板的语言包即可到达已部署门户 ✓。
// 仅支持 multi-lang {default, locales:{...}} 结构；解析失败时原样返回（不影响渲染）。
func MergeBuiltinLangFallback(langRaw json.RawMessage) json.RawMessage {
	if len(langRaw) == 0 {
		return langRaw
	}
	var root map[string]interface{}
	if err := json.Unmarshal(langRaw, &root); err != nil {
		return langRaw
	}
	locales, ok := root["locales"].(map[string]interface{})
	if !ok {
		return langRaw
	}
	for code, loc := range locales {
		lm, ok := loc.(map[string]interface{})
		if !ok {
			continue
		}
		pack := matchBuiltinLangPack(code)
		for k, v := range pack {
			if cur, exists := lm[k]; !exists || cur == nil || cur == "" {
				lm[k] = v
			}
		}
	}
	out, err := json.Marshal(root)
	if err != nil {
		return langRaw
	}
	return out
}

// CompactConfigLang 对整份 portal config JSON 的 lang 字段做 CompactBuiltinLang
// （剥离与内置包完全相同的文案）。用于存量模板启动时的瘦身迁移，幂等 ✓。
func CompactConfigLang(configStr string) (string, bool) {
	if strings.TrimSpace(configStr) == "" {
		return configStr, false
	}
	var cfg map[string]json.RawMessage
	if err := json.Unmarshal([]byte(configStr), &cfg); err != nil {
		return configStr, false
	}
	langRaw, ok := cfg["lang"]
	if !ok || len(langRaw) == 0 || string(langRaw) == "null" {
		return configStr, false
	}
	compacted := CompactBuiltinLang(langRaw)
	if len(compacted) >= len(langRaw) {
		return configStr, false
	}
	cfg["lang"] = compacted
	out, err := json.Marshal(cfg)
	if err != nil {
		return configStr, false
	}
	return string(out), true
}

// CompactBuiltinLang 是 MergeBuiltinLangFallback 的**逆操作**：把语言包里与内置包
// 完全相同的键剥离掉，只保留管理员真正改过的文案（diff）。
//
// 为什么必须这样做：前端编辑器拿到的是「自定义 + 内置兜底」合并后的**全量**语言包，
// 保存时会原样回传 —— 16 种语言 × 493 键 ≈ 420KB 全部落进 portal_templates.config，
// 既白白撑爆 300KB 的配置上限（报错 portal config too large），又让内置包的每次
// 文案升级对存量模板失效（DB 里的旧副本优先级更高）。
//
// 剥离后：DB 只存 diff，渲染/编辑时由 MergeBuiltinLangFallback 用内置包补齐 ✓。
// 保证 round-trip：Compact 之后 Merge 出来的结果与原输入逐键一致。
// 仅支持 {default, locales:{...}}；解析失败时原样返回（交给 validateLang 报错）。
func CompactBuiltinLang(langRaw json.RawMessage) json.RawMessage {
	if len(langRaw) == 0 {
		return langRaw
	}
	var root map[string]interface{}
	if err := json.Unmarshal(langRaw, &root); err != nil {
		return langRaw
	}
	locales, ok := root["locales"].(map[string]interface{})
	if !ok {
		return langRaw
	}
	for code, loc := range locales {
		lm, ok := loc.(map[string]interface{})
		if !ok {
			continue
		}
		pack := matchBuiltinLangPack(code)
		for k, v := range lm {
			// _meta（flag/label/language）体积极小且要保证编辑器语言页签稳定 → 一律保留
			if k == "_meta" {
				continue
			}
			if builtin, exists := pack[k]; exists && reflect.DeepEqual(builtin, v) {
				delete(lm, k)
			}
		}
	}
	out, err := json.Marshal(root)
	if err != nil {
		return langRaw
	}
	return out
}
