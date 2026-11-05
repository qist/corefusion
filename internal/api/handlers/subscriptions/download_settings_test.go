package subscriptions

import (
	"context"
	"encoding/json"
	"testing"

	"corefusion/internal/model/entity"
)

// TestResolveDownloadSettings 校验「引用式下行入口」的解析：
// 只存 _downloadInboundId，渲染时按目标入站生成完整 downloadSettings。
func TestResolveDownloadSettings(t *testing.T) {
	const priv = "CAP0iqPwrizd4edy_hLIKtlpA7V9WE2gAU0LxpfeCUY"
	targetCfg := map[string]interface{}{
		"streamSettings": map[string]interface{}{
			"network":  "xhttp",
			"security": "reality",
			"xhttpSettings": map[string]interface{}{
				"path": "/dl",
			},
			"realitySettings": map[string]interface{}{
				"privateKey":  priv,
				"serverNames": []interface{}{"www.oracle.com"},
				"shortIds":    []interface{}{"30b3ee1d"},
				"settings":    []interface{}{map[string]interface{}{"fingerprint": "chrome"}},
			},
		},
	}
	raw, _ := json.Marshal(targetCfg)
	target := entity.InboundConfig{
		ID: 2, ServerID: 1, CoreType: "xray", Enabled: true,
		Listen: "127.0.0.1", Port: 8443, ConfigJSON: string(raw),
	}
	uploadCfg := map[string]interface{}{
		"streamSettings": map[string]interface{}{
			"network": "xhttp",
			"xhttpSettings": map[string]interface{}{
				"path": "/dl",
				"extra": map[string]interface{}{
					downloadInboundKey: float64(2),
				},
			},
		},
	}

	h := &PublicHandler{}
	upload := entity.InboundConfig{ID: 1, ServerID: 1, CoreType: "xray", Listen: "127.0.0.1", Port: 443}
	h.resolveDownloadSettings(context.Background(), upload, uploadCfg, []entity.InboundConfig{target}, nil)

	dl := getMap(getMap(getMap(getMap(uploadCfg, "streamSettings"), "xhttpSettings"), "extra"), "downloadSettings")
	if dl == nil {
		t.Fatal("未生成 downloadSettings")
	}
	if dl["network"] != "xhttp" || dl["security"] != "reality" {
		t.Errorf("network/security 错误: %#v", dl)
	}
	if toInt64(dl["port"]) != 8443 {
		t.Errorf("port = %v, want 8443", dl["port"])
	}
	if dl["address"] == "" {
		t.Error("address 为空")
	}
	reality := getMap(dl, "realitySettings")
	if reality == nil || getString(reality, "password", "") == "" {
		t.Errorf("realitySettings 缺少公钥: %#v", dl)
	}
	if getString(reality, "shortId", "") != "30b3ee1d" {
		t.Errorf("shortId 错误: %#v", reality)
	}
	xh := getMap(dl, "xhttpSettings")
	if getString(xh, "path", "") != "/dl" {
		t.Errorf("下行 path 应取自目标入站: %#v", xh)
	}
}

// TestResolveDownloadSettingsSkipsInvalidTarget 目标被禁用 / security=none 时必须回退单入口。
func TestResolveDownloadSettingsSkipsInvalidTarget(t *testing.T) {
	mk := func(enabled bool, security string) entity.InboundConfig {
		cfg := map[string]interface{}{
			"streamSettings": map[string]interface{}{
				"network":       "xhttp",
				"security":      security,
				"xhttpSettings": map[string]interface{}{"path": "/dl"},
			},
		}
		raw, _ := json.Marshal(cfg)
		return entity.InboundConfig{ID: 2, ServerID: 1, CoreType: "xray", Enabled: enabled, Port: 8443, ConfigJSON: string(raw)}
	}
	cases := []struct {
		name       string
		uploadPath string
		target     entity.InboundConfig
	}{
		{"disabled", "/dl", mk(false, "tls")},
		{"security-none", "/dl", mk(true, "none")},
		{"path-mismatch", "/other", mk(true, "tls")},
	}
	for _, tc := range cases {
		uploadCfg := map[string]interface{}{
			"streamSettings": map[string]interface{}{
				"network": "xhttp",
				"xhttpSettings": map[string]interface{}{
					"path":  tc.uploadPath,
					"extra": map[string]interface{}{downloadInboundKey: float64(2)},
				},
			},
		}
		h := &PublicHandler{}
		h.resolveDownloadSettings(context.Background(),
			entity.InboundConfig{ID: 1, ServerID: 1, CoreType: "xray"},
			uploadCfg, []entity.InboundConfig{tc.target}, nil)
		if dl := getMap(getMap(getMap(uploadCfg, "streamSettings"), "xhttpSettings"), "downloadSettings"); dl != nil {
			t.Errorf("%s: 不应生成 downloadSettings: %#v", tc.name, dl)
		}
	}
}
