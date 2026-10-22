/**
 * 多语言转换脚本（可重复运行 / 幂等）
 *
 * 输入：web/admin/src/locales/{en,zh-CN}.js，internal/service/portal/locales/{en,zh-CN}.json
 * 输出：指定目标语言的完整语言包，键集合与源语言**完全一致**。
 *
 * 用法：
 *   node scripts/i18n/translate.mjs --targets ru,fa,zh-TW --provider deepseek --write
 *   node scripts/i18n/translate.mjs --targets ru --provider qwen --write
 *   node scripts/i18n/translate.mjs --targets ru --force              # google 免费端点 + 忽略缓存
 *
 * Provider（默认 google）：
 *   google    免费端点，无需 key，但会限流（429）
 *   deepseek  DEEPSEEK_API_KEY      https://api.deepseek.com
 *   qwen      DASHSCOPE_API_KEY     https://dashscope.aliyuncs.com
 *   glm       ZHIPU_API_KEY         https://open.bigmodel.cn
 *   moonshot / kimi  MOONSHOT_API_KEY
 *   openai    OPENAI_API_KEY
 * 也可 --base-url/--model/--api-key 指向任意 OpenAI 兼容端点（含自建）。
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

/** 语言码 → 旗帜（写入门户 _meta，避免所有语言都显示 🇺🇸 English） */
const LOCALE_FLAGS = {
  'zh-CN': '🇨🇳', 'zh-TW': '🇨🇳', en: '🇺🇸',
  ru: '🇷🇺', fa: '🇮🇷', ar: '🇸🇦', fr: '🇫🇷', es: '🇪🇸',
  de: '🇩🇪', pt: '🇵🇹', ja: '🇯🇵', ko: '🇰🇷',
  tr: '🇹🇷', id: '🇮🇩', th: '🇹🇭', vi: '🇻🇳',
}

// ── 参数 ────────────────────────────────────────────────────
function parseArgs() {
  const a = process.argv.slice(2)
  const out = { targets: [], write: false, force: false, provider: 'google', model: '', baseURL: '', apiKey: '' }
  for (let i = 0; i < a.length; i++) {
    if (a[i] === '--targets') out.targets = a[++i].split(',').map(s => s.trim()).filter(Boolean)
    else if (a[i] === '--write') out.write = true
    else if (a[i] === '--force') out.force = true
    else if (a[i] === '--provider') out.provider = a[++i]
    else if (a[i] === '--model') out.model = a[++i]
    else if (a[i] === '--base-url') out.baseURL = a[++i]
    else if (a[i] === '--api-key') out.apiKey = a[++i]
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

// ── Provider 定义 ──────────────────────────────────────────
/** OpenAI 兼容的 /chat/completions 厂商配置（baseURL + key + 模型） */
const LLM_PROVIDERS = {
  deepseek: {
    baseURL: 'https://api.deepseek.com/v1',
    model: 'deepseek-chat',
    envKey: 'DEEPSEEK_API_KEY',
  },
  qwen: {
    baseURL: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
    model: 'qwen-plus',
    envKey: 'DASHSCOPE_API_KEY',
  },
  glm: {
    baseURL: 'https://open.bigmodel.cn/api/paas/v4',
    model: 'glm-4-plus',
    envKey: 'ZHIPU_API_KEY',
  },
  moonshot: {
    baseURL: 'https://api.moonshot.cn/v1',
    model: 'moonshot-v1-8k',
    envKey: 'MOONSHOT_API_KEY',
  },
  kimi: {
    baseURL: 'https://api.moonshot.cn/v1',
    model: 'moonshot-v1-8k',
    envKey: 'MOONSHOT_API_KEY',
  },
  openai: {
    baseURL: 'https://api.openai.com/v1',
    model: 'gpt-4o-mini',
    envKey: 'OPENAI_API_KEY',
  },
}

/** provider 名称 → 环境变量；--provider 与 --model/--base-url 可覆盖 */
function resolveProvider(opts) {
  const name = opts.provider
  if (!name || name === 'google') {
    return { name: 'google', translateOne: googleTranslateOne }
  }
  const p = LLM_PROVIDERS[name]
  if (!p) throw new Error(`未知 provider: ${name}（可选: google | ${Object.keys(LLM_PROVIDERS).join(' | ')}）`)
  const apiKey = opts.apiKey || process.env[p.envKey]
  if (!apiKey) throw new Error(`${name} 需要 API Key：传 --api-key 或设置环境变量 ${p.envKey}`)
  return {
    name,
    baseURL: opts.baseURL || p.baseURL,
    model: opts.model || p.model,
    apiKey,
  }
}

/** 组装给 LLM 的系统提示：术语表 + 硬约束 */
function buildSystemPrompt(target) {
  return [
    '你是软件界面本地化翻译引擎，负责把中文界面文案译成目标语言。',
    `目标语言：${LOCALE_NAMES[target] || target}（语言码 ${target}）。`,
    '',
    '硬性要求：',
    '1. 只输出译文本身，不要引号、解释、注释、编号或前后缀。',
    '2. 译文中出现 \u0001数字\u0001 形式的标记是占位符，必须原样保留，不得翻译、改写或删改。',
    '3. 这是 UI 文案：译文要简短、自然、符合母语者习惯，遵循目标语言的大小写规范。',
    '4. 同一术语在全文件中必须统一译法（如 proxy→代理/прокси，node→节点/узел，plan→套餐/план）。',
    '5. 技术专有名词（HTTP、TLS、UUID、DNS、WebSocket 等）保持不译。',
    '6. 占位符周围的语序按目标语言调整，但标记本身不动。',
    '7. 按钮/菜单用词简洁（动词短语），提示/说明可用完整句。',
  ].join('\n')
}

/** LLM 批量翻译：一次发 BATCH 条，要求返回等长 JSON 数组（比逐条快一个数量级） */
const BATCH_SIZE = 24

async function llmTranslateBatch(items, target, cfg) {
  const maskedItems = items.map(it => protect(it.t))
  const payload = maskedItems.map(m => m.masked)
  const sys = buildSystemPrompt(target) + '\n\n8. 用户消息是一个 JSON 字符串数组，你必须返回**等长的 JSON 字符串数组**，第 i 个输出对应第 i 个输入，顺序严格一致，不要合并、拆分或省略。'
  const url = `${cfg.baseURL.replace(/\/$/, '')}/chat/completions`
  const body = JSON.stringify({
    model: cfg.model,
    temperature: 0.2,
    max_tokens: Math.min(16000, 512 + payload.join('').length * 4),
    reasoning_effort: 'none',
    chat_template_kwargs: { thinking: false, enable_thinking: false },
    response_format: { type: 'json_object' },
    messages: [
      { role: 'system', content: sys },
      { role: 'user', content: JSON.stringify(payload) },
    ],
  })
  const res = await fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${cfg.apiKey}` },
    body,
  })
  if (!res.ok) {
    const detail = (await res.text().catch(() => '')).slice(0, 200)
    throw new Error(`LLM HTTP ${res.status}: ${detail}`)
  }
  const data = await res.json()
  const msg = data.choices && data.choices[0] && data.choices[0].message
  let raw = (msg && msg.content || '').trim()
  if (!raw) throw new Error('empty translation')

  // 解析 JSON 数组（容忍模型包了 ```json 围栏或塞进对象里）
  raw = raw.replace(/^```(?:json)?\s*|\s*```$/g, '').trim()
  let arr = null
  try {
    const parsed = JSON.parse(raw)
    if (Array.isArray(parsed)) arr = parsed
    else if (parsed && typeof parsed === 'object') {
      const v = Object.values(parsed).find(x => Array.isArray(x))
      if (v) arr = v
    }
  } catch {
    const m = raw.match(/\[[\s\S]*\]/)
    if (m) { try { arr = JSON.parse(m[0]) } catch {} }
  }
  // 单条翻译时模型可能直接返回裸字符串（如 "월"），这不算错误
  if (!Array.isArray(arr) && items.length === 1 && typeof arr === 'string') arr = [arr]
  if (!Array.isArray(arr) && items.length === 1 && typeof raw === 'string' && !raw.startsWith('[')) {
    arr = [raw]
  }
  if (!Array.isArray(arr)) throw new Error(`响应不是 JSON 数组: ${raw.slice(0, 120)}`)
  if (arr.length !== items.length) throw new Error(`长度不匹配: 期望 ${items.length}，实得 ${arr.length}`)

  return items.map((it, i) => {
    let v = arr[i]
    if (typeof v !== 'string' || !v.trim()) throw new Error(`第 ${i} 条译文为空`)
    v = v.replace(/^["'`「」『』]+|["'`「」『』]+$/g, '').trim()
    return restore(v, maskedItems[i].store)
  })
}

/** Google 免费端点单条翻译 */
async function googleTranslateOne(text, target) {
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

/** 统一的单条翻译入口 */
/** 统一的批量翻译入口 */
async function translateBatch(items, target, provider) {
  if (provider.name === 'google') {
    // google 免费端点逐条（无 key、会限流，但逻辑简单可靠）
    const out = []
    for (const it of items) out.push(await googleTranslateOne(it.t, target))
    return out
  }
  return llmTranslateBatch(items, target, provider)
}

/** 并发受控的逐条翻译：带缓存、重试、进度打印 */
async function translateAll(texts, target, { force = false, noTranslateKeys = null, provider } = {}) {
  const cacheFile = join(CACHE_DIR, `${target}.json`)
  mkdirSync(CACHE_DIR, { recursive: true })
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

  const CONCURRENCY = provider && provider.name !== 'google' ? 4 : 2
  let done = 0
  const startedAt = Date.now()

  /** 把一批条目送进翻译；整批失败时逐条重试，避免一条坏数据拖垮整批 */
  async function runBatch(batch, attempt = 1) {
    try {
      const arr = await translateBatch(batch, target, provider)
      batch.forEach((it, j) => {
        cache[it.t] = arr[j]
        results[it.i] = arr[j]
      })
      done += batch.length
      return
    } catch (e) {
      // 批量解析失败（长度不匹配/非 JSON 数组）→ 拆成单条逐个试
      if (provider && provider.name !== 'google' && batch.length > 1) {
        for (const it of batch) await runOne(it)
        return
      }
      if (attempt >= 5) {
        console.error(`    失败(保留原文) ${batch.length} 条: ${e.message}`)
        failCount += batch.length
        done += batch.length
        return
      }
      const backoff = e.message.includes('429') || e.message.includes('529') ? 5000 * attempt * 3 : 1000 * attempt
      await new Promise(r => setTimeout(r, backoff))
      await runBatch(batch, attempt + 1)
    }
  }

  /** 单条兜底翻译 */
  async function runOne(item) {
    try {
      const out = await translateBatch([item], target, provider)
      cache[item.t] = out[0]
      results[item.i] = out[0]
    } catch (e) {
      console.error(`    失败(保留原文): ${JSON.stringify(item.t.slice(0, 40))} → ${e.message}`)
      failCount++
    }
    done += 1
    if (done % 50 === 0) {
      const secs = ((Date.now() - startedAt) / 1000).toFixed(0)
      const rate = (done / Math.max(1, (Date.now() - startedAt) / 1000)).toFixed(2)
      const eta = ((todo.length - done) / Math.max(0.01, done / Math.max(1, (Date.now() - startedAt) / 1000)) / 60).toFixed(1)
      console.log(`    进度 ${done}/${todo.length} | 已用 ${secs}s | ${rate} 条/s | 预计剩余 ${eta} 分钟 | 失败 ${failCount}`)
      writeFileSync(cacheFile, JSON.stringify(cache))
    }
  }

  let failCount = 0
  const batches = []
  const size = provider && provider.name !== 'google' ? BATCH_SIZE : 1
  for (let i = 0; i < todo.length; i += size) batches.push(todo.slice(i, i + size))

  const idx = { i: 0 }
  async function pump() {
    while (idx.i < batches.length) {
      const batch = batches[idx.i++]
      await runBatch(batch)
      if (done % 50 !== 0) {
        const secs = ((Date.now() - startedAt) / 1000).toFixed(0)
        const rate = (done / Math.max(1, (Date.now() - startedAt) / 1000)).toFixed(2)
        const eta = ((todo.length - done) / Math.max(0.01, done / Math.max(1, (Date.now() - startedAt) / 1000)) / 60).toFixed(1)
        console.log(`    进度 ${done}/${todo.length} | 已用 ${secs}s | ${rate} 条/s | 预计剩余 ${eta} 分钟 | 失败 ${failCount}`)
        writeFileSync(cacheFile, JSON.stringify(cache))
      }
    }
  }
  await Promise.all(Array.from({ length: CONCURRENCY }, pump))

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
  const opts = parseArgs()
  const { targets, write, force } = opts
  const provider = resolveProvider(opts)
  console.log(`翻译引擎: ${provider.name}${provider.model ? ` (${provider.model})` : ''}`)
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
      adminOrder.map(k => adminNoTr.has(k) ? null : adminSrc[k]), target, { force, provider })
    const tPortal = await translateAll(
      portalOrder.map(k => portalNoTr.has(k) ? null : portalSrc[k]), target, { force, provider })

    const adminFlat = {}
    adminOrder.forEach((k, i) => { adminFlat[k] = tAdmin[i] != null ? tAdmin[i] : (adminNoTr.has(k) ? adminSrc[k] : adminSrc[k]) })
    // 非字符串值（数组/对象）原样保留
    for (const k of Object.keys(enJsFlat)) if (typeof enJsFlat[k] !== 'string') adminFlat[k] = enJsFlat[k]

    const portalFlat = {}
    portalOrder.forEach((k, i) => { portalFlat[k] = tPortal[i] != null ? tPortal[i] : (portalNoTr.has(k) ? portalSrc[k] : portalSrc[k]) })
    for (const k of Object.keys(enObjFlat)) if (typeof enObjFlat[k] !== 'string') portalFlat[k] = enObjFlat[k]

    const adminNested = flatToNested(adminFlat, enJsTpl)
    const portalNested = flatToNested(portalFlat, enObjTpl)

    // ⚠ _meta 是"语言自身的名称/旗帜"，绝不能沿用 en 的值（否则每个语言下拉都显示 English）。
    // 这里按目标语言码强制写回正确的 label/flag/language。
    if (portalNested._meta) {
      portalNested._meta = {
        flag: LOCALE_FLAGS[target] || '🌐',
        label: LOCALE_NAMES[target] || target,
        language: target,
      }
    }

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
