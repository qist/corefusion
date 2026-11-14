/**
 * 语言包英文残留修复（可重复运行 / 幂等）
 *
 * 场景：部分语言包存在“值仍然等于英文源”的键（整条未翻译）。
 * 做法：以 zh-CN 文案为源（缺失时回退 en），调用 Google dict-chrome-ex 免费端点
 *       只翻译这些残留键，其余键原样保留。
 *
 * 用法：
 *   node scripts/i18n/fix-english.mjs --langs de,fr --dry-run
 *   node scripts/i18n/fix-english.mjs                 # 处理全部目标语言并写回
 *   node scripts/i18n/fix-english.mjs --force         # 忽略缓存
 *   node scripts/i18n/fix-english.mjs --include-partial  # 同时处理“英文句子混排”条目（默认只处理整条等于英文）
 */

import { readFileSync, writeFileSync, existsSync, mkdirSync } from 'node:fs'
import { join, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'

const ROOT = join(dirname(fileURLToPath(import.meta.url)), '..', '..')
const ADMIN_DIR = join(ROOT, 'web/admin/src/locales')
const PORTAL_DIR = join(ROOT, 'internal/service/portal/locales')
const CACHE_DIR = join(ROOT, '.i18n-cache')

const ALL_LANGS = ['ar', 'de', 'es', 'fa', 'fr', 'id', 'ja', 'ko', 'pt', 'ru', 'th', 'tr', 'vi', 'zh-TW']

function parseArgs() {
  const a = process.argv.slice(2)
  const out = { langs: ALL_LANGS, dry: false, force: false }
  for (let i = 0; i < a.length; i++) {
    if (a[i] === '--langs') out.langs = a[++i].split(',').map(s => s.trim()).filter(Boolean)
    else if (a[i] === '--dry-run') out.dry = true
    else if (a[i] === '--force') out.force = true
  }
  return out
}

const loadJs = async f => (await import(`file://${f}`)).default
const loadJson = f => JSON.parse(readFileSync(f, 'utf8'))
const flatten = (obj, prefix = '') => {
  const out = {}
  for (const [k, v] of Object.entries(obj)) {
    const key = prefix ? `${prefix}.${k}` : k
    if (v && typeof v === 'object' && !Array.isArray(v)) Object.assign(out, flatten(v, key))
    else out[key] = v
  }
  return out
}
const unflatten = (flat, template) => {
  function build(tpl, prefix) {
    const out = {}
    for (const [k, v] of Object.entries(tpl)) {
      const key = prefix ? `${prefix}.${k}` : k
      if (v && typeof v === 'object' && !Array.isArray(v)) out[k] = build(v, key)
      else out[k] = key in flat ? flat[key] : v
    }
    return out
  }
  return build(template, '')
}

// ── 文本保护 ──────────────────────────────────────────────
// 整值即通用缩写/单位/品牌时保持原样（“ID” 不能译成“序列号”）
const KEEP_AS_IS = new Set([
  'id', 'url', 'uri', 'api', 'ip', 'ipv4', 'ipv6', 'dns', 'tls', 'ssl', 'sni', 'alpn', 'mtu',
  'tcp', 'udp', 'quic', 'http', 'https', 'http/1.1', 'http/2', 'http/3', 'json', 'yaml', 'csv',
  'sse', 'jwt', 'oauth', 'oauth2', 'totp', '2fa', 'mfa', 'cpu', 'ram', 'rom', 'ssd', 'hdd',
  'mb', 'gb', 'tb', 'kb', 'ms', 's', 'm', 'h', 'd', 'cny', 'usd', 'eur', 'btc', 'usdt',
  'ssh', 'vpn', 'vps', 'uuid', 'websocket', 'grpc', 'warp', 'masque', 'clash', 'mihomo',
  'xray', 'sing-box', 'corefusion', 'wireguard', 'cloudflare', 'reality', 'hysteria2',
  'tuic', 'trojan', 'shadowsocks', 'vmess', 'vless', 'snell', 'shadowtls', 'xmux',
])

// 占位符 / URL / 域名 / 反引号代码 / 产品与协议名 不送翻译
const PROTECT_RE = new RegExp([
  String.raw`\{[a-zA-Z0-9_]+\}`,
  String.raw`\{\{\s*[a-zA-Z0-9_.]+\s*\}\}`,
  String.raw`%[sd]`,
  String.raw`\$\{[^}]+\}`,
  String.raw`https?:\/\/[^\s"'\\)]+`,
  String.raw`\b[\w-]+\.(?:com|org|net|io|cn|dev|app|xyz|me|co|top|info)\b[^\s"'\\)]*`,
  String.raw`\x60[^\x60]+\x60`,
  String.raw`\b(?:CoreFusion|sing-?box|Xray|V2Ray|Mihomo|Clash|Cloudflare|WireGuard|WARP|MASQUE|Hysteria2?|TUIC|Trojan|Shadowsocks|VMess|VLESS|Reality|WebSocket|gRPC|HTTP\/[23]|HTTP|HTTPS|TLS|SSL|SNI|ALPN|UUID|DNS|DoH|DoT|IPv4|IPv6|TCP|UDP|QUIC|CDN|API|JSON|YAML|SSE|MTU|JWT|OAuth2?|TOTP|2FA|CPU|RAM|ROM|SSD|HDD)\b`,
].join('|'), 'g')

const SENTINEL = '[[['
function protect(text) {
  const store = []
  const masked = String(text).replace(PROTECT_RE, m => {
    store.push(m)
    return `${SENTINEL}${store.length - 1}]]]`
  })
  return { masked, store }
}
function restore(masked, store) {
  return String(masked).replace(/\[\[\[\s*(\d+)\s*\]\]\]/g, (_, i) => store[+i])
}

// ── Google 免费端点 ───────────────────────────────────────
const ENDPOINT = 'https://clients5.google.com/translate_a/t?client=dict-chrome-ex'

async function googleBatch(items, target, attempt = 1) {
  const body = new URLSearchParams()
  for (const it of items) body.append('q', it.masked)
  const url = `${ENDPOINT}&sl=zh-CN&tl=${encodeURIComponent(target)}`
  try {
    const res = await fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded; charset=UTF-8' },
      body: body.toString(),
    })
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    const data = await res.json()
    if (!Array.isArray(data) || data.length !== items.length) {
      throw new Error(`响应长度不匹配: 期望 ${items.length}, 实得 ${Array.isArray(data) ? data.length : typeof data}`)
    }
    return data.map((v, i) => restore(typeof v === 'string' ? v : String(v), items[i].store))
  } catch (e) {
    if (attempt >= 4) throw e
    await new Promise(r => setTimeout(r, 1200 * attempt))
    return googleBatch(items, target, attempt + 1)
  }
}

const BATCH = 16
const CONCURRENCY = 3

async function translateTexts(entries, target) {
  // entries: [{key, src}]
  const cacheFile = join(CACHE_DIR, `fix-en-${target}.json`)
  mkdirSync(CACHE_DIR, { recursive: true })
  let cache = {}
  if (existsSync(cacheFile)) { try { cache = JSON.parse(readFileSync(cacheFile, 'utf8')) } catch { cache = {} } }

  const results = new Map()
  const todo = []
  for (const e of entries) {
    if (cache[e.src] != null) results.set(e.key, cache[e.src])
    else todo.push(e)
  }

  const batchSize = target === 'zh-TW' ? 4 : BATCH
  const batches = []
  for (let i = 0; i < todo.length; i += batchSize) batches.push(todo.slice(i, i + batchSize))

  let idx = 0, done = 0, failed = 0
  async function pump() {
    while (idx < batches.length) {
      const batch = batches[idx++]
      const items = batch.map(e => protect(e.src))
      let outs = null
      try {
        outs = await googleBatch(items, target)
      } catch (err) {
        // 整批失败 → 逐条
        outs = []
        for (const e of batch) {
          try {
            const one = await googleBatch([protect(e.src)], target)
            outs.push(one[0])
          } catch (e2) {
            failed++
            console.error(`    失败: ${e.key} → ${e2.message}`)
            outs.push(null)
          }
        }
      }
      // 校验：英文整句原样返回 = 批量模式偶发不翻译，逐条重试
      for (let i = 0; i < batch.length; i++) {
        const e = batch[i]
        let out = outs[i]
        if (out == null) continue
        if (out.trim() === e.src.trim() && /[A-Za-z]{3,}[\s,]/.test(e.src)) {
          try {
            const one = await googleBatch([protect(e.src)], target)
            out = one[0]
          } catch { /* 保持原值 */ }
        }
        cache[e.src] = out
        results.set(e.key, out)
      }
      done += batch.length
      if (done % 80 < batchSize) {
        writeFileSync(cacheFile, JSON.stringify(cache))
        console.log(`    进度 ${done}/${todo.length} | 失败 ${failed}`)
      }
    }
  }
  await Promise.all(Array.from({ length: CONCURRENCY }, pump))
  writeFileSync(cacheFile, JSON.stringify(cache))
  return { results, failed }
}

// ── 主流程 ────────────────────────────────────────────────
async function main() {
  const opts = parseArgs()
  const enJsTpl = await loadJs(join(ADMIN_DIR, 'en.js'))
  const zhJsFlat = flatten(await loadJs(join(ADMIN_DIR, 'zh-CN.js')))
  const enJsFlat = flatten(enJsTpl)
  const enJsonTpl = loadJson(join(PORTAL_DIR, 'en.json'))
  const zhJsonFlat = flatten(loadJson(join(PORTAL_DIR, 'zh-CN.json')))
  const enJsonFlat = flatten(enJsonTpl)

  const isResidue = (key, val, enFlat) => {
    if (typeof val !== 'string' || !(key in enFlat)) return false
    if (val.trim() !== String(enFlat[key] ?? '').trim()) return false
    if (!/[A-Za-z]/.test(val)) return false
    if (KEEP_AS_IS.has(val.trim().toLowerCase())) return false
    // 去掉占位符/技术名词后几乎没有可译内容（如 "1 = HTTP/1.1, 2 = HTTP/2"）直接跳过
    const rest = val.replace(PROTECT_RE, ' ').replace(/[^A-Za-z]/g, '')
    return rest.length >= 2
  }

  let grand = 0
  for (const code of opts.langs) {
    // 繁中：以简体中文为源，产出地道繁体；其余语言以英文源（Google 英→X 质量更稳）
    const srcOf = (k, enFlat, zhFlat) => {
      if (code === 'zh-TW') {
        const z = zhFlat[k]
        if (typeof z === 'string' && z.trim() && z.trim() !== String(enFlat[k] ?? '').trim()) return z
      }
      return enFlat[k]
    }
    const jsPath = join(ADMIN_DIR, `${code}.js`)
    const jsonPath = join(PORTAL_DIR, `${code}.json`)
    const js = await loadJs(jsPath)
    const jsFlat = flatten(js)
    const json = loadJson(jsonPath)
    const jsonFlat = flatten(json)

    const adminEntries = []
    for (const [k, v] of Object.entries(jsFlat)) {
      if (isResidue(k, v, enJsFlat)) adminEntries.push({ key: k, src: srcOf(k, enJsFlat, zhJsFlat) })
    }
    const portalEntries = []
    for (const [k, v] of Object.entries(jsonFlat)) {
      if (isResidue(k, v, enJsonFlat)) portalEntries.push({ key: k, src: srcOf(k, enJsonFlat, zhJsonFlat) })
    }

    console.log(`\n=== ${code}: 残留 admin ${adminEntries.length} / portal ${portalEntries.length} ===`)
    if (!adminEntries.length && !portalEntries.length) continue
    grand += adminEntries.length + portalEntries.length

    if (opts.dry) {
      adminEntries.slice(0, 5).forEach(e => console.log(`  [dry] ${e.key}: ${JSON.stringify(e.src).slice(0, 80)}`))
      continue
    }

    const adminTr = await translateTexts(adminEntries, code)
    const portalTr = await translateTexts(portalEntries, code)

    for (const [k, v] of adminTr.results) jsFlat[k] = v
    for (const [k, v] of portalTr.results) jsonFlat[k] = v

    const jsOut = unflatten(jsFlat, enJsTpl)
    // 保留 _meta 等非模板键（若有）
    const mergedJs = { ...jsOut }
    for (const [k, v] of Object.entries(js)) if (!(k in mergedJs)) mergedJs[k] = v
    writeFileSync(jsPath, `// ${code}\nexport default ${JSON.stringify(mergedJs, null, 2)}\n`)

    const jsonOut = unflatten(jsonFlat, enJsonTpl)
    const mergedJson = { ...jsonOut }
    for (const [k, v] of Object.entries(json)) if (!(k in mergedJson)) mergedJson[k] = v
    writeFileSync(jsonPath, `${JSON.stringify(mergedJson, null, 2)}\n`)

    console.log(`  已写入 ${jsPath} (${adminTr.results.size} 键) / ${jsonPath} (${portalTr.results.size} 键)`)
  }

  console.log(`\n合计待修复 ${grand} 条`)
}

main().catch(e => { console.error(e); process.exit(1) })
