/**
 * 多语言转换脚本（一次性 / 可重复运行）
 *
 * 输入：web/admin/src/locales/{en,zh-CN}.js，internal/service/portal/locales/{en,zh-CN}.json
 * 输出：指定目标语言的完整语言包，键集合与源语言完全一致。
 *
 * 用法：
 *   node scripts/i18n/translate.mjs --targets ru,fa,zh-TW,fr,es,ar,de,ja,ko,pt,tr,id,th,vi --dry-run
 *   node scripts/i18n/translate.mjs --targets ru,fa,zh-TW --write
 *
 * 默认走 Google 免费 translate endpoint（无需 API key）。
 * 如需换成自己的翻译服务，改 TRANSLATE_IMPL 一处即可。
 */

import { readFileSync, writeFileSync, existsSync } from 'node:fs'
import { join, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'

const ROOT = join(dirname(fileURLToPath(import.meta.url)), '..', '..')
const ADMIN_DIR = join(ROOT, 'web/admin/src/locales')
const PORTAL_DIR = join(ROOT, 'internal/service/portal/locales')

// 目标语言 → 各自母语名称（用于 UI 下拉展示）
const LOCALE_NAMES = {
  ru: 'Русский',
  fa: 'فارسی',
  'zh-TW': '繁體中文',
  fr: 'Français',
  es: 'Español',
  ar: 'العربية',
  de: 'Deutsch',
  ja: '日本語',
  ko: '한국어',
  pt: 'Português',
  tr: 'Türkçe',
  id: 'Bahasa Indonesia',
  th: 'ไทย',
  vi: 'Tiếng Việt',
}

const LOCALE_FLAGS = {
  ru: '🇷🇺', fa: '🇮🇷', 'zh-TW': '🇨🇳', fr: '🇫🇷', es: '🇪🇸', ar: '🇸🇦',
  de: '🇩🇪', ja: '🇯🇵', ko: '🇰🇷', pt: '🇵🇹', tr: '🇹🇷', id: '🇮🇩', th: '🇹🇭', vi: '🇻🇳',
}

// 语言码 → Google translate 目标语言码（zh-TW 是 google 支持的，法语 es 等一致）
const GOOGLE_LANG = { ...LOCALE_NAMES }

// ── 解析参数 ────────────────────────────────────────────────
function parseArgs() {
  const a = process.argv.slice(2)
  const out = { targets: [], write: false, limit: 0 }
  for (let i = 0; i < a.length; i++) {
    if (a[i] === '--targets') out.targets = a[++i].split(',').map(s => s.trim()).filter(Boolean)
    else if (a[i] === '--write') out.write = true
    else if (a[i] === '--limit') out.limit = parseInt(a[++i], 10) || 0
  }
  return out
}

// ── 读取源语言 ──────────────────────────────────────────────
// en.js / zh-CN.js 是 export default { ... }，用 node 直接 import 拿对象
async function loadSourceJs(file) {
  const mod = await import(`file://${file}`)
  return mod.default
}

async function loadSourceJson(file) {
  return JSON.parse(readFileSync(file, 'utf8'))
}

// ── 收集待翻译键（扁平化 grouped → flat，保证键集合一致） ─────
function flatten(obj, prefix = '') {
  const out = {}
  for (const [k, v] of Object.entries(obj)) {
    const key = prefix ? `${prefix}.${k}` : k
    if (v && typeof v === 'object' && !Array.isArray(v)) Object.assign(out, flatten(v, key))
    else out[key] = v
  }
  return out
}

// ── 占位符保护 ───────────────────────────────────────────────
// {count} {name} 等占位符不能进翻译器
const PLACEHOLDER = /(\{[a-zA-Z0-9_]+\}|\{\{\s*[a-zA-Z0-9_.]+\s*\}\}|%[sd]|\$\{[^}]+\})/g

function protect(text) {
  const store = []
  const masked = String(text).replace(PLACEHOLDER, m => {
    store.push(m)
    return `\u0001${store.length - 1}\u0001`
  })
  return { masked, store }
}

function restore(masked, store) {
  return String(masked).replace(/\u0001(\d+)\u0001/g, (_, i) => store[+i])
}

// ── 翻译实现（Google 免费 endpoint，一次最多 128 段） ─────────
const TRANSLATE_IMPL = {
  async google(texts, target) {
    const chunks = []
    for (let i = 0; i < texts.length; i += 100) chunks.push(texts.slice(i, i + 100))
    const results = []
    for (const chunk of chunks) {
      const url = 'https://translate.googleapis.com/translate_a/single?client=gtx&sl=zh-CN&tl=%s&dt=t&q=%s'
      const q = encodeURIComponent(chunk.join('\n'))
      const res = await fetch(url.replace('%s', encodeURIComponent(target)).replace('%s', q))
      if (!res.ok) throw new Error(`translate HTTP ${res.status} for ${target}`)
      const data = await res.json()
      // data[0] = [[translated, original, ...], ...]
      const segs = data[0] || []
      const joined = segs.map(s => s[0]).join('')
      results.push(joined.split('\n'))
    }
    return results.flat()
  },
}

// ── 一组键的翻译（带缓存与重试） ──────────────────────────────
async function translateGroup(texts, target) {
  const cacheKey = `${target}:${texts.length}`
  const cacheFile = join(ROOT, '.i18n-cache', `${target}.json`)
  let cache = {}
  if (existsSync(cacheFile)) cache = JSON.parse(readFileSync(cacheFile, 'utf8'))
  const todo = texts.map((t, i) => ({ i, t })).filter(x => x.t && !(x.t in cache))
  if (todo.length) {
    console.log(`  [${target}] 待翻译 ${todo.length} 条（缓存命中 ${texts.length - todo.length}）`)
    const BATCH = 60
    for (let i = 0; i < todo.length; i += BATCH) {
      const batch = todo.slice(i, i + BATCH)
      for (let attempt = 1; attempt <= 3; attempt++) {
        try {
          const masked = batch.map(x => protect(x.t).masked)
          const translated = await TRANSLATE_IMPL.google(masked, target)
          batch.forEach((x, j) => {
            const { store } = protect(x.t)
            cache[x.t] = restore(translated[j] ?? x.t, store)
          })
          break
        } catch (e) {
          console.error(`    第 ${attempt} 次尝试失败: ${e.message}`)
          if (attempt === 3) { /* 保留原值 */ }
          else await new Promise(r => setTimeout(r, 2000 * attempt))
        }
      }
    }
    const { mkdirSync } = await import('node:fs')
    mkdirSync(dirname(cacheFile), { recursive: true })
    writeFileSync(cacheFile, JSON.stringify(cache))
  }
  return texts.map(t => (t ? cache[t] ?? t : t))
}

// ── 输出序列化 ───────────────────────────────────────────────
function flatToNested(flat, template) {
  function build(tpl, src, prefix) {
    const out = {}
    for (const [k, v] of Object.entries(tpl)) {
      const key = prefix ? `${prefix}.${k}` : k
      if (v && typeof v === 'object' && !Array.isArray(v)) out[k] = build(v, src, key)
      else if (key in src) out[k] = src[key]
      else if (typeof v === 'string') out[k] = v
    }
    return out
  }
  return build(template, flat, '')
}

function jsModule(obj, locale) {
  const name = LOCALE_NAMES[locale] || locale
  return `// ${name}（${locale}）\nexport default ${JSON.stringify(obj, null, 2)}\n`
}

function jsonModule(obj, locale) {
  return `${JSON.stringify(obj, null, 2)}\n`
}

// ── 主流程 ──────────────────────────────────────────────────
async function main() {
  const { targets, write, limit } = parseArgs()
  if (!targets.length) { console.error('--targets 必填'); process.exit(1) }

  // 源语言（以 en 的键集合为基准，缺失键在 zh-CN 里补）
  const enJs = flatten(await loadSourceJs(join(ADMIN_DIR, 'en.js')))
  const zhJs = flatten(await loadSourceJs(join(ADMIN_DIR, 'zh-CN.js')))
  const enObj = await loadSourceJson(join(PORTAL_DIR, 'en.json'))
  const zhObj = await loadSourceJson(join(PORTAL_DIR, 'zh-CN.json'))

  // 前端：以 zh-CN 的键为基准（更全），缺省值用 en 兜底
  const flatAdmin = {}
  for (const k of Object.keys(zhJs)) flatAdmin[k] = zhJs[k] || enJs[k]
  for (const k of Object.keys(enJs)) if (!(k in flatAdmin)) flatAdmin[k] = enJs[k]

  const adminOrder = Object.keys(enJs)
  const portalOrder = Object.keys(enObj)

  for (const target of targets) {
    console.log(`\n=== ${target} (${LOCALE_NAMES[target] || ''}) ===`)
    const adminTexts = adminOrder.map(k => flatAdmin[k]).filter(t => typeof t === 'string')
    const portalTexts = portalOrder.map(k => (typeof enObj[k] === 'string' ? enObj[k] : '')).filter(Boolean)
    const limitAdmin = limit ? adminTexts.slice(0, limit) : adminTexts
    const limitPortal = limit ? portalTexts.slice(0, limit) : portalTexts

    const tAdmin = await translateGroup(limitAdmin, target)
    const tPortal = await translateGroup(limitPortal, target)

    // 还原 nested 结构（用源对象当模板，保证键完全一致）
    const adminFlat = {}
    adminOrder.forEach((k, i) => { if (typeof flatAdmin[k] === 'string') adminFlat[k] = tAdmin[limit ? i : i] ?? flatAdmin[k] })
    const portalFlat = {}
    portalOrder.forEach((k, i) => { portalFlat[k] = tPortal[i] ?? enObj[k] })
    // 非字符串值（数组/对象）原样保留
    for (const k of Object.keys(enObj)) if (typeof enObj[k] !== 'string') portalFlat[k] = enObj[k]
    for (const k of Object.keys(flatAdmin)) if (typeof flatAdmin[k] !== 'string') adminFlat[k] = flatAdmin[k]

    const adminNested = flatToNested(adminFlat, await loadSourceJs(join(ADMIN_DIR, 'en.js')))
    const portalNested = flatToNested(portalFlat, enObj)

    const adminOut = join(ADMIN_DIR, `${target}.js`)
    const portalOut = join(PORTAL_DIR, `${target}.json`)
    if (write) {
      writeFileSync(adminOut, jsModule(adminNested, target))
      writeFileSync(portalOut, jsonModule(portalNested, target))
      console.log(`  已写入 ${adminOut} / ${portalOut}`)
    } else {
      console.log(`  [dry-run] 将生成 ${adminOut} / ${portalOut}`)
      const sample = adminTexts.slice(0, 5)
      console.log('  示例：')
      sample.forEach((s, i) => console.log(`    ${s}  →  ${tAdmin[i]}`))
    }
  }
}

main().catch(e => { console.error(e); process.exit(1) })
