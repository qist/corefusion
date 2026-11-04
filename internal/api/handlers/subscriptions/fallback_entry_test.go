package subscriptions

import (
	"testing"

	"corefusion/internal/model/entity"
)

const fallbackEntryCfg = `{
  "settings": {"fallbacks": [
    {"name": "", "dest": "@vless-grpc", "xver": 1},
    {"name": "qomvix.com", "dest": "@trojan-grpc", "xver": 1},
    {"name": "ravexo.com", "dest": "unix:/dev/shm/VLESSH2.sock", "xver": 1}
  ]},
  "streamSettings": {
    "security": "reality",
    "realitySettings": {
      "publicKey": "PBK",
      "privateKey": "PRIV",
      "mldsa65Seed": "SEED",
      "serverNames": ["activate.apple.com", ""],
      "shortIds": ["", "sid1"],
      "target": "/dev/shm/nginx.sock"
    }
  }
}`

// 内部入站（@abstract / unix.sock）被外层 REALITY 入站的 fallback 引用时，
// 对外分享必须用外层入口的地址/端口/security + fallback.name 作为 SNI。
func TestFallbackEntryIndexAndShareView(t *testing.T) {
	entry := entity.InboundConfig{ID: 20, ServerID: 1, CoreType: "xray", Port: 443, ConfigJSON: fallbackEntryCfg}
	grpcInner := entity.InboundConfig{ID: 21, ServerID: 1, CoreType: "xray", Listen: "@vless-grpc", Port: 22554,
		ConfigJSON: `{"streamSettings":{"network":"grpc","security":"none","grpcSettings":{"serviceName":"SVC"},"sockopt":{"acceptProxyProtocol":true}}}`}
	trojanInner := entity.InboundConfig{ID: 22, ServerID: 1, CoreType: "xray", Listen: "@trojan-grpc", Port: 55019,
		ConfigJSON: `{"streamSettings":{"network":"grpc","security":"none","grpcSettings":{"serviceName":"SVC2"}}}`}
	h2Inner := entity.InboundConfig{ID: 23, ServerID: 1, CoreType: "xray", Listen: "/dev/shm/VLESSH2.sock,0666", Port: 44357,
		ConfigJSON: `{"streamSettings":{"network":"xhttp","security":"none","xhttpSettings":{"path":"/","mode":"stream-one"}}}`}

	idx := buildFallbackEntryIndex([]entity.InboundConfig{entry, grpcInner, trojanInner, h2Inner})
	if got := idx[21]; got.Entry == nil || got.Entry.ID != 20 || got.Name != "" {
		t.Fatalf("inbound 21 应指向入口 20 且 SNI 为空: %+v", got)
	}
	if got := idx[22]; got.Entry == nil || got.Name != "qomvix.com" {
		t.Fatalf("inbound 22 的 SNI 应为 qomvix.com: %+v", got)
	}
	// dest 带 unix: 前缀、listen 带 ,0666 后缀也要能对上
	if got := idx[23]; got.Entry == nil || got.Name != "ravexo.com" {
		t.Fatalf("inbound 23 应匹配 unix socket（去前缀/权限后）: %+v", got)
	}

	inner := parseConfigMap(trojanInner.ConfigJSON)
	view := fallbackShareView(inner, parseConfigMap(entry.ConfigJSON), idx[22].Name)
	ss := getMap(view, "streamSettings")
	if getString(ss, "security", "") != "reality" {
		t.Errorf("security 应继承入口 REALITY: %v", ss["security"])
	}
	if getString(ss, "network", "") != "grpc" {
		t.Errorf("传输必须保留内部入站自己的: %v", ss["network"])
	}
	if !containsString(getStringSlice(getMap(ss, "realitySettings"), "serverNames"), "qomvix.com") {
		t.Errorf("客户端 SNI 应为 fallback.name: %v", getMap(ss, "realitySettings")["serverNames"])
	}
	if getMap(ss, "realitySettings")["privateKey"] != nil || getMap(ss, "realitySettings")["mldsa65Seed"] != nil {
		t.Error("服务端私钥/抗量子种子不应进入客户端视图")
	}
	if ss["sockopt"] != nil {
		t.Error("服务端 sockopt 不应进入客户端视图")
	}

	// 不在 fallback 引用里的入站：原样返回（不能改别人的配置）
	own := parseConfigMap(grpcInner.ConfigJSON)
	cfg2, view2 := (&PublicHandler{}).applyFallbackEntry(entry, own, idx)
	if cfg2.ID != entry.ID || view2["streamSettings"] == nil {
		t.Errorf("入口自身不应被改写: %+v", cfg2.ID)
	}
}

func containsString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// 入口入站（any_SNI_No_SNI）只允许"无 SNI + 未被 fallback 认领的 SNI"：
// fallback.name 是内部入站的唯一入口 SNI，外层客户端不能拿它当入口。
func TestEntryShareViewRemovesFallbackClaimedSNI(t *testing.T) {
	entry := map[string]interface{}{
		"settings": map[string]interface{}{"fallbacks": []interface{}{
			map[string]interface{}{"name": "", "dest": "@vless-grpc"},
			map[string]interface{}{"name": "qomvix.com", "dest": "@trojan-grpc"},
			map[string]interface{}{"name": "Ravexo.com", "dest": "@vless-h2"},
		}},
		"streamSettings": map[string]interface{}{"security": "reality", "realitySettings": map[string]interface{}{
			"serverNames": []interface{}{"", "activate.apple.com", "qomvix.com", "ravexo.com", "adrepository.apple.com"},
			"serverName":  "qomvix.com",
			"shortIds":    []interface{}{"", "32"},
		}},
	}
	view := entryShareView(entry)
	rs := getMap(getMap(view, "streamSettings"), "realitySettings")
	names := getStringSliceKeepEmptyLocal(rs, "serverNames")
	if len(names) != 3 || names[0] != "" || names[1] != "activate.apple.com" || names[2] != "adrepository.apple.com" {
		t.Fatalf("serverNames 过滤结果不对: %v", names)
	}
	if _, ok := rs["serverName"]; ok {
		t.Error("残留的标量 serverName 会让随机失效，应删除")
	}
	// 原 config 不能被改动（服务端仍需要完整 serverNames）
	if len(getStringSliceKeepEmptyLocal(getMap(getMap(entry, "streamSettings"), "realitySettings"), "serverNames")) != 5 {
		t.Error("entryShareView 不应修改原始 config")
	}
	// 全被认领时保留空串（无 SNI 仍可用）
	only := map[string]interface{}{
		"settings": map[string]interface{}{"fallbacks": []interface{}{
			map[string]interface{}{"name": "qomvix.com", "dest": "@trojan-grpc"},
		}},
		"streamSettings": map[string]interface{}{"realitySettings": map[string]interface{}{
			"serverNames": []interface{}{"qomvix.com"},
		}},
	}
	got := getStringSliceKeepEmptyLocal(getMap(getMap(entryShareView(only), "streamSettings"), "realitySettings"), "serverNames")
	if len(got) != 1 || got[0] != "" {
		t.Fatalf("全被认领时应只剩空串: %v", got)
	}
}

func getStringSliceKeepEmptyLocal(m map[string]interface{}, key string) []string {
	var out []string
	switch v := m[key].(type) {
	case []interface{}:
		for _, raw := range v {
			if s, ok := raw.(string); ok {
				out = append(out, s)
			}
		}
	case []string:
		out = append(out, v...)
	}
	return out
}
