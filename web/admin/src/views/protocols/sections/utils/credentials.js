// 客户端凭据生成器（xray / sing-box 入站编辑器共用）。
//
// 口径必须与后端 internal/service/client/credentials.go 保持一致：
//   - UUID：RFC4122 v4（crypto 随机）；
//   - 通用密码（trojan/hysteria2/hysteria/anytls/tuic/naive/mixed/http/socks）：32 字节随机数的 base64；
//   - shadowsocks 2022：base64 密钥，长度由 method 决定（aes-128=16B，其余=32B）；
//   - psk（snell/shadowtls）：32 字节随机数的 base64url。
//
// 以前各处各自实现（12 位字符 / 16 位字符 / Math.random），
// 与「客户管理创建用户」自动下发的凭据强度不一致（用户反馈），这里统一收口。

export function randomBytes(n) {
  const bytes = new Uint8Array(n)
  if (globalThis.crypto?.getRandomValues) {
    globalThis.crypto.getRandomValues(bytes)
  } else {
    // 极老浏览器兜底：仅用于非安全上下文，熵不如 crypto。
    for (let i = 0; i < n; i++) bytes[i] = Math.floor(Math.random() * 256)
  }
  return bytes
}

export function bytesToBase64(bytes) {
  return btoa(String.fromCharCode(...bytes))
}

export function generateUuid() {
  if (globalThis.crypto?.randomUUID) return globalThis.crypto.randomUUID()
  const b = randomBytes(16)
  b[6] = (b[6] & 0x0f) | 0x40
  b[8] = (b[8] & 0x3f) | 0x80
  const h = Array.from(b, (x) => x.toString(16).padStart(2, '0')).join('')
  return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20)}`
}

export function generatePassword(byteLen = 32) {
  return bytesToBase64(randomBytes(byteLen))
}

export function generatePsk() {
  const bytes = randomBytes(32)
  return bytesToBase64(bytes).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

// shadowsocks 密码：2022-blake3-* 必须是长度精确匹配的 base64 密钥，
// 旧式加密用普通随机密码（与后端同一口径）。
export function generateShadowsocksKey(method) {
  if (method && method.startsWith('2022-blake3')) {
    const length = method === '2022-blake3-aes-128-gcm' ? 16 : 32
    return bytesToBase64(randomBytes(length))
  }
  return generatePassword()
}
