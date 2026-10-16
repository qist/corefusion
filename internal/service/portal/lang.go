package portal

import (
	"embed"
	"encoding/json"
	"path"
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
