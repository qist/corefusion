// 本地 Reality 握手目标的自动 SNI 生成（xray / sing-box 编辑器共用）。
// 配置存在面板 system_settings（reality.local_sni）：
//   { enabled: true, prefix: 'speedtest' }
// 本地 Reality 目标本来就支持任意随机域名；开启后不改变这一点，
// 只是在原有随机域名上加前缀：
//   speedtest.<随机域名>          如 speedtest.haya.cc
//   <随机标签>.<prefix>.<随机域名>  如 hk2.speedtest.miar.app
// 关闭时调用方直接使用原有随机域名（与改造前行为一致）。
import client from '../../../../api/client'

export function defaultLocalSniConfig() {
  return { enabled: true, prefix: 'speedtest' }
}

// 拉取配置；接口异常时按默认值处理
export async function loadLocalSniConfig() {
  try {
    const data = await client.get('/system/reality-local-sni')
    const cfg = data && typeof data === 'object' && !Array.isArray(data) ? data : {}
    const def = defaultLocalSniConfig()
    return {
      enabled: cfg.enabled !== false,
      prefix: String(cfg.prefix || def.prefix).trim().toLowerCase() || def.prefix,
    }
  } catch {
    return defaultLocalSniConfig()
  }
}

// 随机短标签（2-4 位小写字母数字），用于 xxx.speedtest.<随机域名> 变体
function randLabel() {
  const chars = 'abcdefghijklmnopqrstuvwxyz0123456789'
  const len = 2 + Math.floor(Math.random() * 3)
  let s = ''
  for (let i = 0; i < len; i++) s += chars[Math.floor(Math.random() * chars.length)]
  return s
}

// 生成 count 个 SNI。genBase：调用方注入的随机域名生成函数（各编辑器原有的 randomDomain），
// 不在这里重新实现随机逻辑。跳过 usedSet 里已有的；
// usedSet：本节点其它入站已占用的 SNI（含跨核心登记），同一节点多个 Reality 入站互斥。
// 返回空数组表示功能关闭/生成失败，调用方回退为原有随机域名。
export function genLocalSniDomains(count, usedSet, cfg, genBase) {
  if (!cfg || cfg.enabled === false) return []
  const prefix = String(cfg.prefix || 'speedtest').trim().toLowerCase() || 'speedtest'
  const used = usedSet || new Set()
  const out = []
  let attempts = 0
  while (out.length < count && attempts < count * 100) {
    attempts++
    const base = String((genBase && genBase()) || '').trim().toLowerCase()
    if (!base) continue
    const d = (out.length === 0 ? prefix : randLabel() + '.' + prefix) + '.' + base
    if (!used.has(d) && !out.includes(d)) out.push(d)
  }
  return out
}
