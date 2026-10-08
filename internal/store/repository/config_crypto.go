package repository

import (
	"log"
	"strings"

	"corefusion/internal/model/entity"
	"corefusion/internal/security/crypto"
)

// config_json **按需**加密落库（与充值卡、DNS 渠道、EAB 同一把密钥、同一 enc: 格式）：
// 只有真的含凭据的配置才密文化，其余保持明文可读。
//
// 为什么要整包而不是逐字段：
//   - 配置里的敏感叶子（UUID / 密码 / Reality 私钥 / WireGuard 私钥 / hy2 auth）
//     分布在 xray 与 sing-box 两种结构里，逐字段要枚举十几处且极易漏；
//   - 消费方（订阅生成、分享链接、核心渲染、SyncData、前端编辑）拿到的仍是明文，
//     全部零改动；
//   - 没有凭据的配置不动（按需加密），DB 保持可读、省掉无谓的加解密。
//
// 懒迁移：DecryptConfigJSON 对不带 enc: 前缀的存量明文原样返回，任何一次回写都会
// 经 EncryptConfigJSON 转成密文，因此不需要停机刷表。
//
// 注意边界：只防「数据库/备份/磁盘」静态泄露；下发到节点、渲染给核心、返回前端的
// 仍然是明文（节点跑核必须明文，属物理约束）。

// sensitiveKeys 是「配置里出现即视为含凭据」的 JSON 键（统一小写后匹配）。
// 判定方向是**宁可多加密**：误判只会多一次加解密，漏判则是明文落库。
var sensitiveKeys = []string{
	"uuid",          // vless/vmess 客户端 ID
	"id",            // 同上（clients[].id）+ sing-box users[].id
	"password",      // trojan/ss/hy2/tuic 密码、ss-2022 密钥
	"passwd",        // 部分协议的密码键
	"pass",          // xray socks/http accounts[].pass
	"auth",          // hysteria/tuic/naive auth
	"auth-str",      // mihomo ss 认证
	"token",         // 各类访问令牌
	"secret",        // secretKey / client_secret 等
	"privatekey",    // Reality 私钥（realitySettings.privateKey）
	"private_key",   // sing-box / WireGuard 私钥
	"private-key",   // mihomo WireGuard 私钥
	"secretkey",     // xray WireGuard settings.secretKey
	"psk",           // 预共享密钥
	"obfs-password", // ss obfs 密码
	"license",       // WARP license_key
	"credential",    // 凭据对象
	"key",           // 裸 key 键（其余如 keyFile / publicKey 不会匹配到）
}

// isSensitiveConfigJSON 判断一份配置文本是否含凭据。
//
// 直接做**子串匹配**而不是解析 JSON：绝大多数配置本来就没有凭据，
// 子串扫描可以在不解析的情况下快速判定；误匹配（例如备注里写了 "password"）
// 只是多加密一次，方向是安全的。
func isSensitiveConfigJSON(v string) bool {
	if v == "" {
		return false
	}
	if !looksLikeJSON(v) {
		// 非 JSON 的整段文本（如 SSH key / wg conf 片段）按敏感处理
		return true
	}
	lower := strings.ToLower(v)
	for _, k := range sensitiveKeys {
		if strings.Contains(lower, "\""+k) { // 形如 "uuid": / "password": ...
			return true
		}
	}
	return false
}

func looksLikeJSON(v string) bool {
	s := strings.TrimSpace(v)
	return strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[")
}

// EncryptConfigJSON 是**配置 JSON** 的落库收口：只在真的含凭据时加密。
//
// 没有凭据的配置（freedom/blackhole 出站、log/dns/routing 这类 section）
// 保持明文 —— 既省掉每次读写的加解密，也让 DB 内容仍然可读、可排查；
// 含凭据的（入站 clients、Reality 私钥、WireGuard secretKey…）整包加密。
// 空值与已加密的值原样返回（避免双重加密）；加密失败保留原值并告警。
func EncryptConfigJSON(v string) string {
	if v == "" || crypto.IsEncrypted(v) || !isSensitiveConfigJSON(v) {
		return v
	}
	return encryptForce(v)
}

// EncryptConfigField 与 EncryptConfigJSON 的区别：**无条件**加密。
// 用于整列就是凭据的场景（WARP 的 token / private_key / license_key、
// mihomo 的 TLS 私钥与 authentication）—— 它们不是 JSON，没有键名可判定。
func EncryptConfigField(v string) string {
	if v == "" || crypto.IsEncrypted(v) {
		return v
	}
	return encryptForce(v)
}

// IsSensitiveConfigJSON 供存量迁移判断「这份明文到底要不要加密」。
func IsSensitiveConfigJSON(v string) bool { return isSensitiveConfigJSON(v) }

func encryptForce(v string) string {
	enc, err := crypto.EncryptSecret(v)
	if err != nil {
		log.Printf("[config-crypto] encrypt failed, storing plaintext: %v", err)
		return v
	}
	return enc
}

// DecryptConfigJSON 把库里的 config_json 解密成明文。
// 不带 enc: 前缀的存量明文原样返回；解密失败（密钥不符）时保留原值并告警，
// 让上层的 JSON 解析错误直接暴露问题，而不是静默丢数据。
func DecryptConfigJSON(v string) string {
	if v == "" || !crypto.IsEncrypted(v) {
		return v
	}
	plain, err := crypto.DecryptSecret(v)
	if err != nil {
		log.Printf("[config-crypto] decrypt failed (key mismatch?): %v", err)
		return v
	}
	return plain
}

// encryptConfigs / decryptConfigs 是切片版收口，供各仓储的 List* 方法统一调用。
func encryptConfigs[T any](items []T, get func(*T) *string) {
	for i := range items {
		p := get(&items[i])
		*p = EncryptConfigJSON(*p)
	}
}

func decryptConfigs[T any](items []T, get func(*T) *string) {
	for i := range items {
		p := get(&items[i])
		*p = DecryptConfigJSON(*p)
	}
}

// DecryptInboundConfigs / EncryptInboundConfigs 供**绕过仓储直查库**的调用方
// （plan join 查询、nodeserver 换证书、commerce 等）显式收口。
func DecryptInboundConfigs(items []entity.InboundConfig) {
	decryptConfigs(items, func(c *entity.InboundConfig) *string { return &c.ConfigJSON })
}

func EncryptInboundConfig(cfg *entity.InboundConfig) {
	if cfg != nil {
		cfg.ConfigJSON = EncryptConfigJSON(cfg.ConfigJSON)
	}
}
