// i18n 审计：把前端所有 t('x.y') 用到的键 与 zh-CN / en 两个语言包比对
import fs from 'fs'
import path from 'path'
import { fileURLToPath } from 'url'

// ⚠ 不能写死绝对路径 ✗（原来写的是 '/opt/corefusion/web/admin/src'）：换目录/在别处跑时会
// 审计错误的位置，甚至"审了个不存在但恰好有旧文件"的树 → 给出**假通过** ✗。
// 从脚本自身位置推导：<web/admin>/scripts/i18n-audit.mjs → <web/admin>/src
const SRC = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', 'src')

function flatten(obj, prefix = '', out = new Set()) {
  for (const [k, v] of Object.entries(obj || {})) {
    const key = prefix ? `${prefix}.${k}` : k
    if (v && typeof v === 'object' && !Array.isArray(v)) flatten(v, key, out)
    else out.add(key)
  }
  return out
}

const zh = flatten((await import(`${SRC}/locales/zh-CN.js`)).default)
const en = flatten((await import(`${SRC}/locales/en.js`)).default)

// 1) 收集源码里用到的键
const used = new Map() // key -> [files]
function walk(dir) {
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    if (e.name === 'node_modules' || e.name === 'dist' || e.name === '.git') continue
    const p = path.join(dir, e.name)
    if (e.isDirectory()) { walk(p); continue }
    if (!/\.(vue|ts|js|jsx|tsx)$/.test(e.name)) continue
    if (/locales\//.test(p)) continue
    const src = fs.readFileSync(p, 'utf8')
    for (const m of src.matchAll(/(?<![\w$])\$?t\(\s*['"]([A-Za-z0-9_.\-]+)['"]/g)) {
      const k = m[1]
      if (!used.has(k)) used.set(k, new Set())
      used.get(k).add(path.relative(SRC, p))
    }
  }
}
walk(SRC)

// 有意为之的**动态拼接前缀**（都带 fallback，如 t('service.process.' + state, state)），不是缺失。
const DYNAMIC_PREFIXES = ['...', 'dashboard.', 'protoCenter.inbounds.', 'service.process.']
const isDynamic = k => DYNAMIC_PREFIXES.includes(k) || k.endsWith('.')

const missingZh = [...used.keys()].filter(k => !zh.has(k) && !isDynamic(k)).sort()
const missingEn = [...used.keys()].filter(k => !en.has(k) && !isDynamic(k)).sort()
const onlyZh = [...zh].filter(k => !en.has(k)).sort()
const onlyEn = [...en].filter(k => !zh.has(k)).sort()

console.log(`语言包键数: zh=${zh.size} en=${en.size} | 源码用到 ${used.size} 个键`)
console.log(`\n=== ① 用到但 zh-CN 缺失（${missingZh.length} 个，界面会显示原始键名！）===`)
for (const k of missingZh) console.log(`  ${k}   ← ${[...used.get(k)].slice(0, 3).join(', ')}`)
console.log(`\n=== ② 用到但 en 缺失（${missingEn.length} 个）===`)
for (const k of missingEn) console.log(`  ${k}   ← ${[...used.get(k)].slice(0, 3).join(', ')}`)
console.log(`\n=== ③ zh 有但 en 没有（${onlyZh.length} 个，切英文会显示键名）===`)
for (const k of onlyZh.slice(0, 60)) console.log(`  ${k}`)
if (onlyZh.length > 60) console.log(`  … 还有 ${onlyZh.length - 60} 个`)
console.log(`\n=== ④ en 有但 zh 没有（${onlyEn.length} 个）===`)
for (const k of onlyEn.slice(0, 40)) console.log(`  ${k}`)
if (onlyEn.length > 40) console.log(`  … 还有 ${onlyEn.length - 40} 个`)

// 5) 文案必须能被 vue-i18n 的消息编译器解析：像 `/{文件名}`、整段代码样例这类
//    字面花括号，若不转义成 {'{'} / {'}'} 或改用 tm() 取原文，运行时会在控制台
//    刷 SyntaxError（真机：WAF 页 "Invalid token in placeholder: '文件名'"）。
const { baseCompile } = await import('@intlify/message-compiler')
// 代码样例（HTML/JS）由 moduleDefs.js 的 rawMessage()（tm）取原文，不参与编译。
const RAW_KEYS = new Set(['portal.dft.freeform.html', 'portal.dft.freeform.js'])
const compileBad = []
for (const f of fs.readdirSync(`${SRC}/locales`)) {
  if (!f.endsWith('.js') || f === 'index.js') continue
  const msgs = (await import(`${SRC}/locales/${f}`)).default
  const stack = [[msgs, '']]
  while (stack.length) {
    const [obj, prefix] = stack.pop()
    for (const [k, v] of Object.entries(obj || {})) {
      const key = prefix ? `${prefix}.${k}` : k
      if (v && typeof v === 'object' && !Array.isArray(v)) { stack.push([v, key]); continue }
      if (typeof v !== 'string' || RAW_KEYS.has(key)) continue
      if (!v.includes('{') && !v.includes('}')) continue
      let err = ''
      try { baseCompile(v, { onError: e => { err = e.message || String(e) } }) } catch (e) { err = e.message || String(e) }
      if (err) compileBad.push(`${f}: ${key} :: ${err}`)
    }
  }
}
console.log(`\n=== ⑤ 消息编译器无法解析的文案（${compileBad.length} 个，字面花括号要转义）===`)
for (const line of compileBad.slice(0, 20)) console.log(`  ${line}`)
if (compileBad.length > 20) console.log(`  … 还有 ${compileBad.length - 20} 个`)

// 作为守卫用时给出退出码（CI / pre-commit 可直接调用）
if (missingZh.length || missingEn.length || onlyZh.length || onlyEn.length || compileBad.length) {
  console.log('\n❌ i18n 审计未通过（存在缺失/不对称的键，或无法编译的文案）')
  process.exit(1)
}
console.log('\n✅ i18n 审计通过：键齐全且对称，所有文案都能被消息编译器解析')
