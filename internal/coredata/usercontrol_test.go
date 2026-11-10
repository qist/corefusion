package coredata

import (
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/xtls/xray-core/common/serial"
	hyAccount "github.com/xtls/xray-core/proxy/hysteria/account"
	"github.com/xtls/xray-core/proxy/shadowsocks"
	ss2022 "github.com/xtls/xray-core/proxy/shadowsocks_2022"
	"github.com/xtls/xray-core/proxy/trojan"
	"github.com/xtls/xray-core/proxy/vless"
	"github.com/xtls/xray-core/proxy/vmess"
)

// 协议必须由调用方显式给出：线上 tag 是 inbound-59026（只有端口），
// 只按 tag 推断会构造出错误类型的账号，热加必然失败。
func TestBuildXrayUserAccountByProtocol(t *testing.T) {
	cases := []struct {
		name     string
		protocol string
		method   string
		want     *serial.TypedMessage
	}{
		{"vless", "vless", "",
			serial.ToTypedMessage(&vless.Account{Id: "uuid-1", Flow: "xtls-rprx-vision"})},
		{"vmess", "vmess", "",
			serial.ToTypedMessage(&vmess.Account{Id: "uuid-1"})},
		{"trojan", "trojan", "",
			serial.ToTypedMessage(&trojan.Account{Password: "pwd-1"})},
		{"hysteria", "hysteria", "",
			serial.ToTypedMessage(&hyAccount.Account{Auth: "pwd-1"})},
		{"shadowsocks-2022", "shadowsocks", "2022-blake3-aes-128-gcm",
			serial.ToTypedMessage(&ss2022.Account{Key: "pwd-1"})},
		{"shadowsocks-legacy", "shadowsocks", "chacha20-poly1305",
			serial.ToTypedMessage(&shadowsocks.Account{
				Password:   "pwd-1",
				CipherType: shadowsocks.CipherType_CHACHA20_POLY1305,
			})},
	}
	for _, tc := range cases {
		got := BuildXrayUserAccount(tc.protocol, "uuid-1", "pwd-1", "xtls-rprx-vision", tc.method)
		if got == nil {
			t.Errorf("%s: BuildXrayUserAccount returned nil", tc.name)
			continue
		}
		if !proto.Equal(got, tc.want) {
			t.Errorf("%s: unexpected account payload", tc.name)
		}
	}

	if got := BuildXrayUserAccount("socks", "uuid-1", "pwd-1", "", ""); got != nil {
		t.Error("socks 入站没有账号，不应构造用户账户")
	}
	if got := BuildXrayUserAccount("", "uuid-1", "pwd-1", "", ""); got != nil {
		t.Error("协议为空时不应猜一个账户类型")
	}
}

func TestProtocolHasUsers(t *testing.T) {
	for _, p := range []string{"socks", "http", "dokodemo-door", "wireguard"} {
		if ProtocolHasUsers(p) {
			t.Errorf("%s should not have users", p)
		}
	}
	for _, p := range []string{"vless", "vmess", "trojan", "shadowsocks", "hysteria"} {
		if !ProtocolHasUsers(p) {
			t.Errorf("%s should have users", p)
		}
	}
}

func TestInferProtocolFromTag(t *testing.T) {
	cases := map[string]string{
		"inbound-vless-443": "vless",
		"inbound-59026":     "",
		"inbound-ss-8388":   "shadowsocks",
	}
	for tag, want := range cases {
		if got := InferProtocolFromTag(tag); got != want {
			t.Errorf("InferProtocolFromTag(%q) = %q, want %q", tag, got, want)
		}
	}
}
