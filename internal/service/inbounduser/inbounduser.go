// Package inbounduser 定位「某客户在某入站里的账号条目」。
//
// 入站账号的 email 规则是 {客户邮箱}_{tag 短后缀}（见 Email）。但入站 tag 可能被改过
// ——改端口时 tag 会一起变（inbound-vmess-36833 → inbound-vmess-36832），而存量账号的
// email 后缀仍是旧 tag。只按当前 tag 精确比对会误判「该客户在这个入站没有账号」，进而：
//   - 订阅里整台节点消失（findClientByEmail）
//   - 暂停/恢复失效（pauseResumeAll）
//   - 全量同步账号重复添加（SyncAccountsForNewInbound）
//   - 流量用尽封禁删不掉账号（usersync remove）
//
// 因此这里统一提供「先精确、再按客户前缀兜底」的定位逻辑，供上述各处复用。
package inbounduser

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Email 生成入站账号的 email：inbound-vless-443 → {base}_vless443；
// 不含协议段的 tag（inbound-443）→ {base}_inbound-443。
// 与历史实现（service/client、subscriptions 各自一份）保持完全一致。
func Email(baseEmail, tag string) string {
	parts := strings.Split(tag, "-")
	if len(parts) >= 3 {
		return fmt.Sprintf("%s_%s%s", baseEmail, parts[1], parts[len(parts)-1])
	}
	return fmt.Sprintf("%s_%s", baseEmail, tag)
}

// EntryEmail 返回账号条目的标识：xray 用 email，sing-box 用 name。
func EntryEmail(entry map[string]interface{}) string {
	if entry == nil {
		return ""
	}
	if v, ok := entry["email"].(string); ok && v != "" {
		return v
	}
	if v, ok := entry["name"].(string); ok && v != "" {
		return v
	}
	return ""
}

// FindEntry 在入站配置里找该客户的账号条目。
//
// 先按当前 tag 生成的 email 精确匹配；失败后按「同一客户」兜底：email 等于客户邮箱，
// 或以 {客户邮箱}_ 开头。兜底的匹配范围仍然只属于该客户（不会命中别人），
// 用于兼容改过 tag 的存量账号。
func FindEntry(configJSON, clientEmail, tag string) map[string]interface{} {
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return nil
	}
	return FindEntryInConfig(config, clientEmail, tag)
}

// FindEntryInConfig 是 FindEntry 的「已解析配置」版本（调用方已有 map 时免二次解析）。
func FindEntryInConfig(config map[string]interface{}, clientEmail, tag string) map[string]interface{} {
	if config == nil || clientEmail == "" {
		return nil
	}

	exact := Email(clientEmail, tag)
	prefix := clientEmail + "_"

	var fallback map[string]interface{}
	for _, list := range userLists(config) {
		for _, raw := range list {
			entry, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			email := EntryEmail(entry)
			if email == "" {
				continue
			}
			if email == exact {
				return entry
			}
			if fallback == nil && (email == clientEmail || strings.HasPrefix(email, prefix)) {
				fallback = entry
			}
		}
	}
	return fallback
}

// ResolveEmail 返回该客户在此入站里的实际账号 email；没有账号时返回空串。
//
// 与 FindEntry 不同，这里不会「按当前 tag 猜一个」：调用方（暂停/恢复/删号）
// 必须作用在真实存在的账号上，凭空构造 email 只会把操作打到不存在的账号上。
func ResolveEmail(configJSON, clientEmail, tag string) string {
	return EntryEmail(FindEntry(configJSON, clientEmail, tag))
}

// userLists 收集配置里的账号数组：xray（settings.clients / settings.users）
// 与 sing-box（顶层 users[]）。
func userLists(config map[string]interface{}) [][]interface{} {
	var lists [][]interface{}
	if arr, ok := config["users"].([]interface{}); ok {
		lists = append(lists, arr)
	}
	if settings, ok := config["settings"].(map[string]interface{}); ok {
		for _, key := range []string{"clients", "users"} {
			if arr, ok := settings[key].([]interface{}); ok {
				lists = append(lists, arr)
			}
		}
	}
	return lists
}

// Names 返回入站配置里全部账号标识（xray email / sing-box name）。
// 用于「按账号名反查入站」（如在线用户归属），兼容 tag 改过、后缀与当前 tag 不一致的账号。
func Names(configJSON string) []string {
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return nil
	}
	var names []string
	for _, list := range userLists(config) {
		for _, raw := range list {
			entry, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			if name := EntryEmail(entry); name != "" {
				names = append(names, name)
			}
		}
	}
	return names
}
