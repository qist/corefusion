/**
 * 多语言转换脚本（可重复运行 / 幂等）
 *
 * 输入：web/admin/src/locales/{en,zh-CN}.js，internal/service/portal/locales/{en,zh-CN}.json
 * 输出：指定目标语言的完整语言包，键集合与源语言**完全一致**。
 *
 * 用法：
 *   node scripts/i18n/translate.mjs --targets ru,fa,zh-TW             # dry-run，只打印前几条
 *   node scripts/i18n/translate.mjs --targets ru,fa,zh-TW --write     # 写入语言包
 *   node scripts/i18n/translate.mjs --targets ru --force              # 忽略本地缓存，全部重译
 *
 * ⚠ 逐条翻译（一次一次发），**绝不**用 \n 拼接后按 \n 拆回：
 *   早期版本用拼接法，遇到自带换行的文案就整批错位（account.email 拿到 account.plan 的值）。
 *   代价是请求数 = 文案数，因此内置了本地缓存（.i18n-cache/<lang>.json），
 *   同一份原文只翻一次，增量更新成本接近 0。
 */

import { readFileSync, writeFileSync, existsSync, mkdirSync } from 'node:fs'
import { join, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'

const ROOT = join(dirname(fileURLToPath(import.meta.url)), '..', '..')
const ADMIN_DIR = join(ROOT, 'web/admin/src/locales')
const PORTAL_DIR = join(ROOT, 'internal/service/portal/locales')
const CACHE_DIR = join(ROOT, '.i18n-cache')

/** 这些键在任何语言下都必须保持原值（产品名 / 商标） */
const NO_TRANSLATE_KEYS = new Set([
  'common.appName',
  '_meta.flag',
  '_meta.label',
  '_meta.language',
])

/** 语言码 → 母语名称（写入语言包头部注释 / UI 下拉） */
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

// ── 参数 ────────────────────────────────────────────────────
function parseArgs() {
  const a = process.argv.slice(2)
  const out = { targets: [], write: false, force: false }
  for (let i = 0; i < a.length; i++) {
    if (a[i] === '--targets') out.targets = a[++i].split(',').map(s => s.trim()).filter(Boolean)
    else if (a[i] === '--write') out.write = true
    else if (a[i] === '--force') out.force = true
  }
  return out
}

// ── 源语言 ──────────────────────────────────────────────────
async function loadSourceJs(file) {
  const mod = await import(`file://${file}`)
  return mod.default
}
const loadSourceJson = file => JSON.parse(readFileSync(file, 'utf8'))

/** grouped → flat（a.b.c），保证键集合与源语言逐一对齐 */
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
// {count} / {{name}} / %s / ${expr} 不能进翻译器，否则会被改写
const PLACEHOLDER = /(\{[a-zA-Z0-9_]+\}|\{\{\s*[a-zA-Z0-9_.]+\s*\}\}|%[sd]|\$\{[^}]+\})/g
const SENTINEL = '\u0001'

function protect(text) {
  const store = []
  const masked = String(text).replace(PLACEHOLDER, m => {
    store.push(m)
    return `${SENTINEL}${store.length - 1}${SENTINEL}`
  })
  return { masked, store }
}

function restore(masked, store) {
  return String(masked).replace(new RegExp(`${SENTINEL}(\\d+)${SENTINEL}`, 'g'), (_, i) => store[+i])
}

// ── 翻译实现 ────────────────────────────────────────────────
/** 单条翻译（唯一实现，绝不拼接） */
async function translateOne(text, target) {
  const { masked, store } = protect(text)
  const url = `https://translate.googleapis.com/translate_a/single?client=gtx&sl=zh-CN&tl=${encodeURIComponent(target)}&dt=t&q=${encodeURIComponent(masked)}`
  const res = await fetch(url)
  if (!res.ok) throw new Error(`translate HTTP ${res.status} for ${target}`)
  const data = await res.json()
  const segs = data[0] || []
  const out = segs.map(s => s[0]).join('')
  if (!out) throw new Error('empty translation')
  return restore(out, store)
}

/** 并发受控的逐条翻译：带缓存、重试、进度打印 */
async function translateAll(texts, target, { force = false, noTranslateKeys = null } = {}) {
  const cacheFile = join(CACHE_DIR, `${target}.json`)
  let cache = {}
  if (existsSync(cacheFile)) {
    try { cache = JSON.parse(readFileSync(cacheFile, 'utf8')) } catch { cache = {} }
  }
  if (force) cache = {}

  const results = new Array(texts.length).fill(null)
  const todo = []
  texts.forEach((t, i) => {
    if (!t || typeof t !== 'string') results[i] = t
    else if (cache[t] != null) results[i] = cache[t]
    else todo.push({ i, t })
  })

  console.log(`  [${target}] 共 ${texts.length} 条，待翻译 ${todo.length} 条`)

  const CONCURRENCY = 2
  let done = 0
  async function worker(item) {
    for (let attempt = 1; attempt <= 5; attempt++) {
      try {
        const v = await translateOne(item.t, target)
        cache[item.t] = v
        results[item.i] = v
        return
      } catch (e) {
        if (attempt === 5) {
          console.error(`    失败(保留原文): ${JSON.stringify(item.t.slice(0, 40))} → ${e.message}`)
          return
        }
        const backoff = e.message.includes('429') ? 5000 * attempt * 4 : 500 * attempt
        await new Promise(r => setTimeout(r, backoff))
      }
    }
  }
  const idx = { i: 0 }
  async function pump() {
    while (idx.i < todo.length) {
      const item = todo[idx.i++]
      await worker(item)
      if (++done % 100 === 0) {
        console.log(`    进度 ${done}/${todo.length}`)
        writeFileSync(cacheFile, JSON.stringify(cache))
      }
    }
  }
  await Promise.all(Array.from({ length: CONCURRENCY }, pump))

  mkdirSync(CACHE_DIR, { recursive: true })
  writeFileSync(cacheFile, JSON.stringify(cache))
  return results
}

// ── 输出序列化 ───────────────────────────────────────────────
/** 按 template 的结构还原 nested 对象，键集合与 template 完全一致 */
function flatToNested(flat, template) {
  function build(tpl, prefix) {
    const out = {}
    for (const [k, v] of Object.entries(tpl)) {
      const key = prefix ? `${prefix}.${k}` : k
      if (v && typeof v === 'object' && !Array.isArray(v)) out[k] = build(v, key)
      else if (key in flat) out[k] = flat[key]
      else if (typeof v === 'string') out[k] = v
    }
    return out
  }
  return build(template, '')
}

const jsModule = (obj, locale) => `// ${LOCALE_NAMES[locale] || locale}（${locale}）\nexport default ${JSON.stringify(obj, null, 2)}\n`
const jsonModule = obj => `${JSON.stringify(obj, null, 2)}\n`

// ── 主流程 ──────────────────────────────────────────────────
async function main() {
  const { targets, write, force } = parseArgs()
  if (!targets.length) { console.error('--targets 必填'); process.exit(1) }

  // 源语言（以 en 为结构基准；缺省值用 zh-CN 兜底）
  const enJsTpl = await loadSourceJs(join(ADMIN_DIR, 'en.js'))
  const zhJsFlat = flatten(await loadSourceJs(join(ADMIN_DIR, 'zh-CN.js')))
  const enJsFlat = flatten(enJsTpl)
  const enObjTpl = loadSourceJson(join(PORTAL_DIR, 'en.json'))
  const zhObjFlat = flatten(loadSourceJson(join(PORTAL_DIR, 'zh-CN.json')))
  const enObjFlat = flatten(enObjTpl)

  // 前端：优先 zh-CN 的文案（更贴近本项目术语），缺失键用 en 兜底
  const adminSrc = {}
  for (const k of Object.keys(enJsFlat)) adminSrc[k] = zhJsFlat[k] ?? enJsFlat[k]
  // 门户：直接用 en 的文案
  const portalSrc = {}
  for (const k of Object.keys(enObjFlat)) portalSrc[k] = enObjFlat[k]

  const adminKeys = Object.keys(enJsTpl.length ? enJsFlat : enJsFlat)
  for (const target of targets) {
    console.log(`\n=== ${target} (${LOCALE_NAMES[target] || target}) ===`)
    const adminOrder = Object.keys(adminSrc)
    const portalOrder = Object.keys(portalSrc)

    const adminNoTr = new Set(adminOrder.filter(k => NO_TRANSLATE_KEYS.has(k)))
    const portalNoTr = new Set(portalOrder.filter(k => NO_TRANSLATE_KEYS.has(k)))
    // 免翻键：直接透传原值（不进翻译器，也不会被缓存下来的旧译文覆盖）
    const tAdmin = await translateAll(
      adminOrder.map(k => adminNoTr.has(k) ? null : adminSrc[k]), target, { force })
    const tPortal = await translateAll(
      portalOrder.map(k => portalNoTr.has(k) ? null : portalSrc[k]), target, { force })

    const adminFlat = {}
    adminOrder.forEach((k, i) => { adminFlat[k] = tAdmin[i] != null ? tAdmin[i] : (adminNoTr.has(k) ? adminSrc[k] : adminSrc[k]) })
    // 非字符串值（数组/对象）原样保留
    for (const k of Object.keys(enJsFlat)) if (typeof enJsFlat[k] !== 'string') adminFlat[k] = enJsFlat[k]

    const portalFlat = {}
    portalOrder.forEach((k, i) => { portalFlat[k] = tPortal[i] != null ? tPortal[i] : (portalNoTr.has(k) ? portalSrc[k] : portalSrc[k]) })
    for (const k of Object.keys(enObjFlat)) if (typeof enObjFlat[k] !== 'string') portalFlat[k] = enObjFlat[k]

    const adminNested = flatToNested(adminFlat, enJsTpl)
    const portalNested = flatToNested(portalFlat, enObjTpl)

    const adminOut = join(ADMIN_DIR, `${target}.js`)
    const portalOut = join(PORTAL_DIR, `${target}.json`)
    if (write) {
      writeFileSync(adminOut, jsModule(adminNested, target))
      writeFileSync(portalOut, jsonModule(portalNested))
      console.log(`  已写入 ${adminOut}\n  已写入 ${portalOut}`)
    } else {
      console.log(`  [dry-run] 将生成 ${adminOut} / ${portalOut}`)
      adminOrder.slice(0, 8).forEach((k, i) => console.log(`    ${k}: ${adminSrc[k]}  →  ${tAdmin[i]}`))
    }
  }
}

main().catch(e => { console.error(e); process.exit(1) })
