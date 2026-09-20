package client

import (
	"encoding/json"
	"strings"

	"corefusion/internal/model/entity"
)

// xray 与 sing-box 的用户容器结构不同，这里集中处理差异，避免各处重复判断：
//
//	xray    : settings.clients[] / settings.users[]，字段 id / email / password / flow / level
//	sing-box: 顶层 users[]，字段 name / uuid / password / flow（随协议不同）
//
// 关键差异：sing-box 的入站**没有 settings 字段**，用户直接挂在顶层 users[]，
// 因此凡是「只认 settings」的旧逻辑对 sing-box 入站都会静默失败（订阅用户加不进去）。

// isSingboxInbound 判断入站是否属于 sing-box 核心。
// 优先看 core_type；兼容历史数据（无 core_type / 无 settings 但有顶层 users）。
func isSingboxInbound(inbound *entity.InboundConfig) bool {
	if inbound == nil {
		return false
	}
	if strings.EqualFold(inbound.CoreType, "singbox") {
		return true
	}
	var config map[string]interface{}
	if json.Unmarshal([]byte(inbound.ConfigJSON), &config) != nil {
		return false
	}
	if _, hasSettings := config["settings"]; hasSettings {
		return false
	}
	_, hasUsers := config["users"]
	return hasUsers
}

// userContainers 返回可能存放用户列表的容器（sing-box 顶层优先，xray settings 优先）。
func userContainers(config map[string]interface{}) []map[string]interface{} {
	containers := make([]map[string]interface{}, 0, 2)
	if config == nil {
		return containers
	}
	if _, hasUsers := config["users"]; hasUsers {
		containers = append(containers, config) // sing-box：顶层 users[]
	}
	if settings, ok := config["settings"].(map[string]interface{}); ok {
		containers = append(containers, settings)
	}
	if len(containers) == 0 {
		containers = append(containers, config)
	}
	return containers
}

// userNameOf 取用户标识：xray 用 email，sing-box 用 name。
func userNameOf(user map[string]interface{}) string {
	if v, ok := user["email"].(string); ok && v != "" {
		return v
	}
	if v, ok := user["name"].(string); ok && v != "" {
		return v
	}
	return ""
}

// findUserEntry 在入站配置中查找指定用户的条目（兼容 email / name 两种标识）。
func findUserEntry(config map[string]interface{}, email string) map[string]interface{} {
	for _, container := range userContainers(config) {
		for _, key := range []string{"users", "clients"} {
			arr, ok := container[key].([]interface{})
			if !ok {
				continue
			}
			for _, u := range arr {
				if user, ok := u.(map[string]interface{}); ok && userNameOf(user) == email {
					return user
				}
			}
		}
	}
	return nil
}

// deleteUserFromConfig 从所有用户容器中删除指定用户，返回是否有改动。
func deleteUserFromConfig(config map[string]interface{}, email string) bool {
	changed := false
	for _, container := range userContainers(config) {
		for _, key := range []string{"users", "clients"} {
			arr, ok := container[key].([]interface{})
			if !ok {
				continue
			}
			filtered := make([]interface{}, 0, len(arr))
			for _, u := range arr {
				user, ok := u.(map[string]interface{})
				if !ok {
					continue
				}
				if userNameOf(user) == email {
					changed = true
					continue
				}
				filtered = append(filtered, u)
			}
			if len(filtered) != len(arr) {
				container[key] = filtered
			}
		}
	}
	return changed
}

// setUserPausedInConfig 设置用户的 _paused 标记，返回是否找到该用户。
func setUserPausedInConfig(config map[string]interface{}, email string, paused bool) bool {
	if user := findUserEntry(config, email); user != nil {
		user["_paused"] = paused
		return true
	}
	return false
}

// singboxProtocolOf 取 sing-box 入站的协议类型：优先配置里的 type 字段，其次从 tag 推断。
func singboxProtocolOf(config map[string]interface{}, tag string) string {
	if t, ok := config["type"].(string); ok && t != "" {
		return strings.ToLower(t)
	}
	return detectProtocol(tag)
}

// singboxUserEntry 按协议生成 sing-box 的 users[] 条目。
//
// 字段口径（sing-box 文档）：
//
//	vless/vmess/tuic        → uuid（vless 另带 flow）
//	trojan/hysteria2/anytls → password
//	shadowsocks             → password（2022-blake3-* 多用户按人分配）
//	snell                   → psk
//	naive/mixed/http/socks  → username + password
func singboxUserEntry(protocol, email, uuid, password, flow, method string) map[string]interface{} {
	// _paused 是面板约定字段，配置下发前会被 coreconfig 的清洗逻辑剔除/过滤
	user := map[string]interface{}{
		"name":    email,
		"_paused": false,
	}

	switch protocol {
	case "vless":
		user["uuid"] = uuid
		if flow != "" {
			user["flow"] = flow
		}
	case "vmess":
		user["uuid"] = uuid
	case "tuic":
		user["uuid"] = uuid
		user["password"] = password
	case "trojan", "hysteria2", "hysteria", "anytls":
		user["password"] = password
	case "shadowsocks":
		user["password"] = password
		if method != "" {
			user["method"] = method
		}
	case "snell":
		user["psk"] = password
	case "naive", "mixed", "http", "socks":
		user["username"] = email
		user["password"] = password
	default:
		// 未知协议：uuid/password 都给上，避免下发后无法认证
		if uuid != "" {
			user["uuid"] = uuid
		}
		if password != "" {
			user["password"] = password
		}
	}
	return user
}
