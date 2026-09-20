package inboundconfig

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func key(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i + 1)
	}
	return base64.StdEncoding.EncodeToString(b)
}

// 2022-blake3 的密钥长度是强制的（128→16B / 256→32B）。面板切换加密方式时只重生成
// 服务端密钥，存量客户端密钥长度不变 → sing-box / mihomo 客户端 `bad key length` 拒载。
func TestNormalizeShadowsocks2022KeysFixesMismatch(t *testing.T) {
	// xray 形状：aes-128 但其中一个客户端是 32 字节密钥
	cfg := map[string]interface{}{
		"protocol": "shadowsocks",
		"settings": map[string]interface{}{
			"method":   "2022-blake3-aes-128-gcm",
			"password": key(16),
			"clients": []interface{}{
				map[string]interface{}{"email": "bad@x.com", "password": key(32)},
				map[string]interface{}{"email": "ok@x.com", "password": key(16)},
			},
		},
	}
	raw, _ := json.Marshal(cfg)
	out, fixed := normalizeShadowsocks2022Keys(string(raw))
	if len(fixed) != 1 || fixed[0] != "bad@x.com" {
		t.Fatalf("应只修 bad@x.com, got %v", fixed)
	}
	var got map[string]interface{}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	clients := got["settings"].(map[string]interface{})["clients"].([]interface{})
	bad := clients[0].(map[string]interface{})["password"].(string)
	if n := decodedKeyLen(bad); n != 16 {
		t.Errorf("修复后密钥应为 16 字节, got %d", n)
	}
	ok := clients[1].(map[string]interface{})["password"].(string)
	if ok != key(16) {
		t.Error("长度正确的密钥不应被改动")
	}
	if got["settings"].(map[string]interface{})["password"] != key(16) {
		t.Error("服务端密钥长度正确时不应被改动")
	}
}

// aes-256 配 16 字节密钥同样要修（反向）。
func TestNormalizeShadowsocks2022KeysFixesShortKey(t *testing.T) {
	cfg := map[string]interface{}{
		"settings": map[string]interface{}{
			"method":   "2022-blake3-aes-256-gcm",
			"password": key(32),
			"clients": []interface{}{
				map[string]interface{}{"email": "short@x.com", "password": key(16)},
			},
		},
	}
	raw, _ := json.Marshal(cfg)
	out, fixed := normalizeShadowsocks2022Keys(string(raw))
	if len(fixed) != 1 {
		t.Fatalf("应修 1 条, got %v", fixed)
	}
	var got map[string]interface{}
	_ = json.Unmarshal([]byte(out), &got)
	pw := got["settings"].(map[string]interface{})["clients"].([]interface{})[0].(map[string]interface{})["password"].(string)
	if decodedKeyLen(pw) != 32 {
		t.Errorf("应为 32 字节, got %d", decodedKeyLen(pw))
	}
}

// sing-box 形状：method / password / users[] 都在顶层。
func TestNormalizeShadowsocks2022KeysSingboxShape(t *testing.T) {
	cfg := map[string]interface{}{
		"type":     "shadowsocks",
		"method":   "2022-blake3-aes-128-gcm",
		"password": key(16),
		"users": []interface{}{
			map[string]interface{}{"name": "u1", "password": key(32)},
		},
	}
	raw, _ := json.Marshal(cfg)
	out, fixed := normalizeShadowsocks2022Keys(string(raw))
	if len(fixed) != 1 || fixed[0] != "u1" {
		t.Fatalf("应修 u1, got %v", fixed)
	}
	var got map[string]interface{}
	_ = json.Unmarshal([]byte(out), &got)
	if decodedKeyLen(got["users"].([]interface{})[0].(map[string]interface{})["password"].(string)) != 16 {
		t.Error("sing-box users 密钥未修成 16 字节")
	}
}

// 无需改动时必须**原样返回字符串**（避免无谓地重排 JSON 键、改写 updated_at）。
func TestNormalizeShadowsocks2022KeysNoopKeepsJSON(t *testing.T) {
	cfg := `{"settings":{"method":"2022-blake3-aes-128-gcm","password":"` + key(16) + `","clients":[{"email":"a@x.com","password":"` + key(16) + `"}]}}`
	out, fixed := normalizeShadowsocks2022Keys(cfg)
	if len(fixed) != 0 || out != cfg {
		t.Errorf("不应有改动: fixed=%v out=%s", fixed, out)
	}
	// 非 2022 方法、非 base64 的旧式密码：不动
	legacy := `{"settings":{"method":"aes-256-gcm","password":"plainpw"}}`
	if out, fixed := normalizeShadowsocks2022Keys(legacy); out != legacy || len(fixed) != 0 {
		t.Error("旧式加密方式不应被改动")
	}
	// 非法 JSON：原样返回
	if out, _ := normalizeShadowsocks2022Keys("{not json"); !strings.Contains(out, "not json") {
		t.Error("非法 JSON 应原样返回")
	}
}
