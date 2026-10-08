package repository

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"strings"
)

// ============================================================
// ss-2022（2022-blake3）密钥长度强制修正
//
// 加密方式对密钥字节长度的要求是**强制**的：aes-128-gcm 必须 16 字节，
// aes-256-gcm / chacha8/20-poly1305 必须 32 字节。长度不匹配时：
//   - sing-box / mihomo 客户端以 `bad key length, required 16, got 32` 拒绝加载；
//   - xray 过去能宽容接受（所以线上很隐蔽），**新版 xray 同样会拒绝**。
//
// 场景：面板切换加密方式时只重新生成服务端密钥，存量客户端密钥长度不变，
// 于是出现 aes-128-gcm 配 32 字节客户端密钥的组合。长度不匹配的密钥本来就
// 无法使用，这里按当前加密方式重新生成。
//
// 收口在**仓储写入路径**（Create / Update / Upsert）：无论数据来自面板编辑、
// 前端导入，还是 Agent 侧 SyncData 落库，写库前一律强制校正 —— 只在 service
// 层修正是堵不住全部入口的（Create 曾漏掉，SyncData 也完全绕过 service）。
// ============================================================

// Shadowsocks2022KeyLen 返回该 2022-blake3 加密方式要求的密钥字节长度。
func Shadowsocks2022KeyLen(method string) int {
	if strings.TrimSpace(method) == "2022-blake3-aes-128-gcm" {
		return 16
	}
	return 32 // aes-256-gcm / chacha20-poly1305 / chacha8-poly1305
}

// decodedKeyLen 返回 base64 密钥解码后的字节数（非 base64 返回 -1）。
func decodedKeyLen(v string) int {
	v = strings.TrimSpace(v)
	if v == "" {
		return -1
	}
	if b, err := base64.StdEncoding.DecodeString(v); err == nil {
		return len(b)
	}
	if b, err := base64.RawStdEncoding.DecodeString(v); err == nil {
		return len(b)
	}
	return -1
}

// randomBase64Key 生成 n 字节随机密钥（base64 标准编码）。
func randomBase64Key(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(buf)
}

// NormalizeShadowsocks2022Keys 保证 ss-2022 的密钥长度与加密方式匹配。
// 非 ss-2022 / 解析失败 / 无需修正时原样返回（nil 表示没有改动）。
//
// 返回（可能被修正的 JSON, 被重新生成密钥的标识列表）。标识为空表示无需改动。
func NormalizeShadowsocks2022Keys(configJSON string) (string, []string) {
	if !strings.Contains(configJSON, "2022-blake3") {
		return configJSON, nil
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return configJSON, nil
	}
	settings, _ := cfg["settings"].(map[string]interface{})
	method := ""
	if settings != nil {
		method, _ = settings["method"].(string)
	}
	if strings.TrimSpace(method) == "" {
		method, _ = cfg["method"].(string) // sing-box：method 在顶层
	}
	if !strings.HasPrefix(strings.TrimSpace(method), "2022-blake3") {
		return configJSON, nil
	}
	want := Shadowsocks2022KeyLen(method)
	fixed := make([]string, 0, 2)

	fixKey := func(holder map[string]interface{}, label string) {
		if holder == nil {
			return
		}
		raw, exists := holder["password"]
		if !exists {
			// 该位置本来就没有密钥（如 xray 的顶层、sing-box 的 settings）——不能凭空创建
			return
		}
		cur, _ := raw.(string)
		if decodedKeyLen(cur) == want {
			return
		}
		key := randomBase64Key(want)
		if key == "" {
			return
		}
		holder["password"] = key
		fixed = append(fixed, label)
	}

	// 服务端密钥（xray 在 settings.password，sing-box 在顶层 password）
	fixKey(settings, "<server>")
	fixKey(cfg, "<server>")

	// 用户/客户端密钥（xray: settings.clients[]；sing-box: 顶层 users[]）
	fixUsers := func(arr []interface{}) {
		for _, it := range arr {
			m, ok := it.(map[string]interface{})
			if !ok {
				continue
			}
			label, _ := m["email"].(string)
			if label == "" {
				label, _ = m["name"].(string)
			}
			if label == "" {
				label, _ = m["username"].(string)
			}
			fixKey(m, label)
		}
	}
	if arr, ok := cfg["users"].([]interface{}); ok {
		fixUsers(arr)
	}
	if settings != nil {
		if arr, ok := settings["clients"].([]interface{}); ok {
			fixUsers(arr)
		}
	}

	if len(fixed) == 0 {
		return configJSON, nil
	}
	out, err := json.Marshal(cfg)
	if err != nil {
		return configJSON, fixed
	}
	return string(out), fixed
}
