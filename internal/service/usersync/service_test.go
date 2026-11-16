package usersync

import (
	"testing"

	"corefusion/internal/model/entity"
)

// add/update 必须用入站里现存账号的真实 email 与凭据（而不是裸客户邮箱 + clientID）：
// 否则恢复/续费推下去的是一个客户端根本不会用的账号。
func TestBuildUserOpUsesExistingEntry(t *testing.T) {
	inbound := &entity.InboundConfig{Tag: "inbound-vmess-36832", Protocol: "vmess"}
	entry := map[string]interface{}{
		"email":    "user@example.com_vmess28307",
		"id":       "11111111-1111-1111-1111-111111111111",
		"security": "auto",
	}
	op := buildUserOp(entry, inbound, "add")
	if op.Email != "user@example.com_vmess28307" {
		t.Errorf("email = %q, want 现存账号（旧 tag 后缀）", op.Email)
	}
	if op.Uuid != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("uuid = %q, want 账号真实 uuid", op.Uuid)
	}
}

// remove 只需要 email（xray RemoveUser 按 email 匹配），不能带 clientID 当 uuid。
func TestBuildUserOpRemoveOnlyNeedsEmail(t *testing.T) {
	inbound := &entity.InboundConfig{Tag: "inbound-vless-1"}
	entry := map[string]interface{}{"email": "a@b.com_vless1", "id": "uuid-1"}
	op := buildUserOp(entry, inbound, "remove")
	if op.Email != "a@b.com_vless1" || op.Uuid != "" {
		t.Fatalf("remove op = %+v", op)
	}
}

// hysteria 的凭据字段是 auth。
func TestBuildUserOpHysteriaAuth(t *testing.T) {
	inbound := &entity.InboundConfig{Tag: "inbound-hysteria-4443"}
	entry := map[string]interface{}{"email": "a@b.com_hysteria4443", "auth": "secret"}
	op := buildUserOp(entry, inbound, "add")
	if op.Password != "secret" {
		t.Fatalf("password = %q, want auth", op.Password)
	}
}

// shadowsocks 的 cipher method 在入站 settings 级，不在账号条目里。
func TestBuildUserOpShadowsocksMethod(t *testing.T) {
	inbound := &entity.InboundConfig{
		Tag:        "inbound-ss-1",
		ConfigJSON: `{"protocol":"shadowsocks","settings":{"method":"aes-128-gcm","clients":[{"email":"a@b.com_ss1","password":"p"}]}}`,
	}
	entry := map[string]interface{}{"email": "a@b.com_ss1", "password": "p"}
	op := buildUserOp(entry, inbound, "add")
	if op.Encryption != "aes-128-gcm" {
		t.Fatalf("encryption = %q, want settings.method", op.Encryption)
	}
	if op.Password != "p" {
		t.Fatalf("password = %q", op.Password)
	}
}
