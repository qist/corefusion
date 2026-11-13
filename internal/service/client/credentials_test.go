package client

import (
	"encoding/base64"
	"regexp"
	"testing"

	"corefusion/internal/model/entity"
)

func TestGeneratePasswordIsStrongBase64(t *testing.T) {
	pw := generatePassword()
	b, err := base64.StdEncoding.DecodeString(pw)
	if err != nil {
		t.Fatalf("密码应为标准 base64: %q (%v)", pw, err)
	}
	if len(b) != 32 {
		t.Fatalf("密码熵应为 32 字节, 实际 %d", len(b))
	}
}

func TestGenerateUUIDFormat(t *testing.T) {
	re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if got := generateUUID(); !re.MatchString(got) {
		t.Fatalf("UUID v4 格式不正确: %q", got)
	}
}

// ss-2022 的密钥长度必须与加密方式匹配，否则仓储会重新生成一份，
// 运行时热加的密码与配置/订阅里展示的密码就会不一致。
func TestCredentialsForInboundShadowsocks2022(t *testing.T) {
	cases := []struct {
		name   string
		core   string
		config string
		want   int
	}{
		{
			name:   "xray-aes-128",
			core:   "xray",
			config: `{"protocol":"shadowsocks","settings":{"method":"2022-blake3-aes-128-gcm"}}`,
			want:   16,
		},
		{
			name:   "singbox-aes-256",
			core:   "singbox",
			config: `{"type":"shadowsocks","method":"2022-blake3-aes-256-gcm","users":[]}`,
			want:   32,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inbound := &entity.InboundConfig{CoreType: tc.core, Protocol: "shadowsocks", ConfigJSON: tc.config}
			_, password, _, _ := credentialsForInbound(inbound)
			b, err := base64.StdEncoding.DecodeString(password)
			if err != nil {
				t.Fatalf("密钥应为 base64: %q (%v)", password, err)
			}
			if len(b) != tc.want {
				t.Fatalf("密钥长度应为 %d 字节, 实际 %d", tc.want, len(b))
			}
		})
	}
}

// vless/vmess 只需要 UUID，不能再塞一个未使用的密码字段。
func TestCredentialsForInboundUUIDOnly(t *testing.T) {
	for _, proto := range []string{"vless", "vmess"} {
		inbound := &entity.InboundConfig{Protocol: proto, ConfigJSON: `{"settings":{}}`}
		uuid, password, _, _ := credentialsForInbound(inbound)
		if uuid == "" || password != "" {
			t.Fatalf("%s 只应生成 UUID，实际 uuid=%q password=%q", proto, uuid, password)
		}
	}
}

// hysteria2 等密码协议：与入口编辑器一致的 32 字节 base64。
func TestCredentialsForInboundHysteria2Password(t *testing.T) {
	inbound := &entity.InboundConfig{CoreType: "singbox", Protocol: "hysteria2", ConfigJSON: `{"type":"hysteria2","users":[]}`}
	_, password, _, _ := credentialsForInbound(inbound)
	b, err := base64.StdEncoding.DecodeString(password)
	if err != nil || len(b) != 32 {
		t.Fatalf("hysteria2 密码应为 32 字节 base64, 实际 %q (err=%v)", password, err)
	}
}
