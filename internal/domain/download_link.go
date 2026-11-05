package domain

import (
	"encoding/json"
	"strconv"
	"strings"
)

// DownloadInboundKey 是 XHTTP 上下行分离里「下行入口」的引用键，存放在
// streamSettings.xhttpSettings.extra 下。只存目标入站 ID（引用式），
// 渲染订阅时再按目标入站的当前配置解析出完整 downloadSettings。
const DownloadInboundKey = "_downloadInboundId"

// DownloadTargetID 从入站 config_json 读出被引用的下行入站 ID；没有引用返回 0。
func DownloadTargetID(configJSON string) int64 {
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return 0
	}
	ss, _ := cfg["streamSettings"].(map[string]interface{})
	if ss == nil {
		return 0
	}
	xh, _ := ss["xhttpSettings"].(map[string]interface{})
	if xh == nil {
		return 0
	}
	ext, _ := xh["extra"].(map[string]interface{})
	if ext == nil {
		return 0
	}
	switch v := ext[DownloadInboundKey].(type) {
	case float64:
		return int64(v)
	case int:
		return int64(v)
	case int64:
		return v
	case json.Number:
		n, _ := v.Int64()
		return n
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		return n
	}
	return 0
}

// IsXHTTPInbound 判断入站 config_json 的传输是否为 XHTTP。
func IsXHTTPInbound(configJSON string) bool {
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return false
	}
	ss, _ := cfg["streamSettings"].(map[string]interface{})
	if ss == nil {
		return false
	}
	network, _ := ss["network"].(string)
	return strings.EqualFold(network, "xhttp")
}

// NormalizeClientEmail 把 xray stats / 入站用户里的 "user@x.com_<tagSuffix>"
// 归一成 "user@x.com"。
//
// 入站账号的 email 是「客户邮箱 + 入站后缀」（后缀 = 协议+端口），同一客户在
// 不同入站后缀不同；统计归户时必须去掉后缀才能落到同一个 client/订阅。
// 仅当去掉最后一段下划线后的前缀仍含 "@" 时才认为它是后缀，避免误伤
// 本地部分带下划线的邮箱（"a_b@x.com"）。
func NormalizeClientEmail(statsEmail string) string {
	if idx := strings.LastIndex(statsEmail, "_"); idx > 0 {
		candidate := statsEmail[:idx]
		if strings.Contains(candidate, "@") {
			return candidate
		}
	}
	return statsEmail
}
