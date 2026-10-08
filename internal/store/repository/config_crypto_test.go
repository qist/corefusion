package repository

import (
	"strings"
	"testing"
)

// 没有凭据的配置必须**保持明文**（DB 可读、省掉无谓的加解密）。
func TestEncryptConfigJSONSkipsNonSensitive(t *testing.T) {
	plain := []string{
		`{"protocol":"freedom","tag":"direct"}`,
		`{"loglevel":"warning","log":{"access":""}}`,
		`{"dns":{"servers":["8.8.8.8"]}}`,
	}
	for _, v := range plain {
		if got := EncryptConfigJSON(v); got != v {
			t.Fatalf("无凭据配置不应被加密: in=%q out=%q", v, got)
		}
	}
}

// 含凭据的配置必须密文化，且解密后能还原原文。
func TestEncryptConfigJSONEncryptsSensitive(t *testing.T) {
	cases := map[string]string{
		"vless uuid":   `{"settings":{"clients":[{"id":"0523ad37-7653-4386-8dcc-303b5be8daf1"}]}}`,
		"trojan pass":  `{"settings":{"clients":[{"password":"5j6+zh8QKO1Z"}]}}`,
		"reality key":  `{"streamSettings":{"realitySettings":{"privateKey":"OFv9NeLcZ1p8"}}}`,
		"wireguard sk": `{"settings":{"secretKey":"mMSwxoLjOEpp"}}`,
		"socks pass":   `{"settings":{"accounts":[{"user":"u","pass":"p"}]}}`,
	}
	for name, v := range cases {
		enc := EncryptConfigJSON(v)
		if enc == v || !strings.HasPrefix(enc, "enc:") {
			t.Fatalf("%s: 应被加密，got %q", name, enc)
		}
		if got := DecryptConfigJSON(enc); got != v {
			t.Fatalf("%s: 解密不还原: %q", name, got)
		}
	}
}

// 整列凭据（WARP token / mihomo TLS 私钥）没有键名可判定 → 无条件加密。
func TestEncryptConfigFieldAlwaysEncrypts(t *testing.T) {
	for _, v := range []string{"7ac813aa-b3e0", "aGVsbG8gd29ybGQ="} {
		enc := EncryptConfigField(v)
		if !strings.HasPrefix(enc, "enc:") {
			t.Fatalf("EncryptConfigField 必须加密: %q", enc)
		}
		if got := DecryptConfigJSON(enc); got != v {
			t.Fatalf("解密不还原: %q", got)
		}
	}
}

// 幂等：已加密的值不能被二次加密。
func TestEncryptConfigJSONIdempotent(t *testing.T) {
	enc := EncryptConfigJSON(`{"password":"x"}`)
	again := EncryptConfigJSON(enc)
	if again != enc {
		t.Fatalf("双重加密: %q -> %q", enc, again)
	}
}
