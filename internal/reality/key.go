// Package reality 提供 Reality 密钥的小工具。
//
// 背景：sing-box 的入站（服务端）只存 `private_key`，而客户端/分享链接需要公钥 `pbk`。
// x25519 私钥可以推导出公钥，因此面板不需要额外保存公钥也能随时还原它。
package reality

import (
	"crypto/ecdh"
	"encoding/base64"
	"strings"
)

// PublicKeyFromPrivate 由 Reality 私钥推导出公钥。
//
// 入参为 base64(raw-url, 无填充) 编码的 32 字节 x25519 私钥（xray `x25519` /
// sing-box `generate reality-keypair` 输出的格式）。私钥非法或长度不符时返回空串。
func PublicKeyFromPrivate(privB64 string) string {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(privB64))
	if err != nil || len(raw) != 32 {
		return ""
	}
	priv, err := ecdh.X25519().NewPrivateKey(raw)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(priv.PublicKey().Bytes())
}
