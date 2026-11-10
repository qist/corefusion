// 前置 TLS 的 ALPN 选择（访问域名 → 开启 TLS 时使用）。
//
// 存储形态是逗号分隔字符串（`_accessDomain.tls.alpn`），因为核心配置与旧数据
// 都是这个形态；界面用多选下拉，这里负责数组 ↔ 字符串的双向转换。

// ALPN 候选项：H3 用于 UDP/QUIC 前置终止，H2 与 HTTP/1.1 用于 TCP。
export const ALPN_OPTIONS = ['h3', 'h2', 'http/1.1']

// alpnToArray 把存储值转成下拉用的数组；兼容 "h2, http/1.1" 这种带空格的旧值。
export function alpnToArray(value) {
  if (Array.isArray(value)) return value.filter(Boolean)
  return String(value || '')
    .split(',')
    .map(s => s.trim())
    .filter(Boolean)
}

// alpnToString 把下拉选择转回存储值；空数组写空字符串（表示不显式指定）。
export function alpnToString(value) {
  return alpnToArray(value).join(',')
}
