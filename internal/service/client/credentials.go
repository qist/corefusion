package client

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// 客户端凭据生成统一收口。
//
// 口径（与 web/admin 的入站编辑器保持一致，改一处必须两处同改）：
//   - UUID（vless/vmess/tuic）：RFC4122 v4，crypto/rand；
//   - 通用密码（trojan/hysteria2/hysteria/anytls/tuic/naive/mixed/http/socks）：32 字节随机数的 base64；
//   - shadowsocks 2022：base64 密钥，长度由 method 决定（aes-128=16B，其余=32B）；
//   - shadowsocks 旧式 / psk（snell/shadowtls）：32 字节随机数的 base64（url 安全）。
//
// 以前这里统一用 16 字节 hex，而入站编辑器里 hysteria2 用 16 位字符、trojan 用
// 32 字节 base64，导致「客户管理创建」与「入口处手动创建」的密钥强度/形态不一致；
// shadowsocks 2022 的 hex 密钥还会在仓储落库时被 NormalizeShadowsocks2022Keys
// 重新生成，运行时热加的密码与配置里展示的密码因此对不上（客户端认证失败）。

// generatePassword 返回 32 字节随机数的 base64 编码（44 字符）。
func generatePassword() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("failed to generate random password: " + err.Error())
	}
	return base64.StdEncoding.EncodeToString(b)
}

// generatePSK 返回 32 字节随机数的 base64url 编码（43 字符，无 padding）。
func generatePSK() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("failed to generate random psk: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// generateShadowsocksPassword 按加密方式生成 shadowsocks 密码。
// 2022-blake3-* 必须是长度精确匹配的 base64 密钥，旧式加密用普通随机密码。
func generateShadowsocksPassword(method string) string {
	method = strings.TrimSpace(method)
	if strings.HasPrefix(method, "2022-blake3") {
		b := make([]byte, repository.Shadowsocks2022KeyLen(method))
		if _, err := rand.Read(b); err != nil {
			panic("failed to generate shadowsocks-2022 key: " + err.Error())
		}
		return base64.StdEncoding.EncodeToString(b)
	}
	return generatePassword()
}

// inboundShadowsocksMethod 读取入站配置里的 shadowsocks 加密方式
// （xray 在 settings.method，sing-box 在顶层 method）。
func inboundShadowsocksMethod(configJSON string) string {
	var root map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &root); err != nil {
		return ""
	}
	if settings, ok := root["settings"].(map[string]interface{}); ok {
		if m, _ := settings["method"].(string); strings.TrimSpace(m) != "" {
			return strings.TrimSpace(m)
		}
	}
	if m, _ := root["method"].(string); strings.TrimSpace(m) != "" {
		return strings.TrimSpace(m)
	}
	return ""
}

// credentialsForInbound 按入站协议生成一套客户端凭据。
// 返回的 method 用于撰写 shadowsocks 用户条目（空表示沿用服务端配置）。
func credentialsForInbound(inbound *entity.InboundConfig) (uuid, password, flow, method string) {
	if inbound == nil {
		return generateUUID(), generatePassword(), "", ""
	}
	protocol := strings.ToLower(strings.TrimSpace(inbound.Protocol))
	if protocol == "" {
		protocol = detectProtocol(inbound.Tag)
	}
	if protocol == "shadowsocks" || protocol == "shadowsocks2022" {
		method = inboundShadowsocksMethod(inbound.ConfigJSON)
		return "", generateShadowsocksPassword(method), "", method
	}
	switch protocol {
	case "vless", "vmess":
		// 这两个协议只认 UUID，密码字段不参与认证。
		return generateUUID(), "", "", ""
	case "tuic":
		return generateUUID(), generatePassword(), "", ""
	case "snell", "shadowtls":
		return "", generatePSK(), "", ""
	default:
		return generateUUID(), generatePassword(), "", ""
	}
}

// generateUUID 生成 RFC4122 v4 UUID。
func generateUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("failed to generate uuid: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// randomHex 返回 n 字节随机数的 hex 编码（订阅 token 等仍沿用该形态）。
func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("failed to generate random token: " + err.Error())
	}
	return hex.EncodeToString(b)
}
