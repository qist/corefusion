/**
 * 语言包修复脚本（不依赖任何翻译服务 / 可重复运行）
 *
 * 做三件事：
 *   1. zh-TW：用 OpenCC(s2tw) 从简体源重新生成，不再用错位的机器翻译结果
 *   2. appName：所有语言统一恢复为 CoreFusion（产品名不音译）
 *   3. 审计日志等关键键：用人工译文覆盖（chore: 修 audit / auditLogs / auditTitle / nodeAuditTitle / cleanupCatAudit）
 *
 * 用法：node scripts/i18n/fix-generated.mjs [--dry-run]
 */

import { readFileSync, writeFileSync } from 'node:fs'
import { join, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'
import OpenCC from 'opencc'

const ROOT = join(dirname(fileURLToPath(import.meta.url)), '..', '..')
const ADMIN_DIR = join(ROOT, 'web/admin/src/locales')
const PORTAL_DIR = join(ROOT, 'internal/service/portal/locales')

const DRY = process.argv.includes('--dry-run')

const NAMES = {
  ru: 'Русский', fa: 'فارسی', 'zh-TW': '繁體中文', fr: 'Français', es: 'Español',
  ar: 'العربية', de: 'Deutsch', ja: '日本語', ko: '한국어', pt: 'Português',
  tr: 'Türkçe', id: 'Bahasa Indonesia', th: 'ไทย', vi: 'Tiếng Việt',
}

/** 审计日志等关键键的人工译文（覆盖错位的机器结果） */
const AUDIT_FIX = {
  'zh-TW': { 'menu.audit': '審計日誌', 'nodeServer.auditLogs': '審計日誌', 'security.auditTitle': '審計日誌', 'logManage.cleanupCatAudit': '審計', 'security.nodeAuditTitle': '節點操作日誌' },
  ru: { 'menu.audit': 'Журнал аудита', 'nodeServer.auditLogs': 'Журналы аудита', 'security.auditTitle': 'Журнал аудита', 'logManage.cleanupCatAudit': 'Аудит', 'security.nodeAuditTitle': 'Журналы операций узла' },
  fa: { 'menu.audit': 'گزارش حسابرسی', 'nodeServer.auditLogs': 'گزارش‌های حسابرسی', 'security.auditTitle': 'گزارش حسابرسی', 'logManage.cleanupCatAudit': 'حسابرسی', 'security.nodeAuditTitle': 'گزارش عملیات نود' },
  ar: { 'menu.audit': 'سجل التدقيق', 'nodeServer.auditLogs': 'سجلات التدقيق', 'security.auditTitle': 'سجل التدقيق', 'logManage.cleanupCatAudit': 'التدقيق', 'security.nodeAuditTitle': 'سجلات عمليات العقد' },
  fr: { 'menu.audit': 'Journal d’audit', 'nodeServer.auditLogs': 'Journaux d’audit', 'security.auditTitle': 'Journal d’audit', 'logManage.cleanupCatAudit': 'Vérification', 'security.nodeAuditTitle': 'Journaux d’opérations des nœuds' },
  es: { 'menu.audit': 'Registro de auditoría', 'nodeServer.auditLogs': 'Registros de auditoría', 'security.auditTitle': 'Registro de auditoría', 'logManage.cleanupCatAudit': 'Auditoría', 'security.nodeAuditTitle': 'Registros de operaciones de nodos' },
  de: { 'menu.audit': 'Audit-Protokoll', 'nodeServer.auditLogs': 'Audit-Protokolle', 'security.auditTitle': 'Audit-Protokoll', 'logManage.cleanupCatAudit': 'Audit', 'security.nodeAuditTitle': 'Knoten-Betriebsprotokolle' },
  ja: { 'menu.audit': '監査ログ', 'nodeServer.auditLogs': '監査ログ', 'security.auditTitle': '監査ログ', 'logManage.cleanupCatAudit': '監査', 'security.nodeAuditTitle': 'ノード操作ログ' },
  ko: { 'menu.audit': '감사 로그', 'nodeServer.auditLogs': '감사 로그', 'security.auditTitle': '감사 로그', 'logManage.cleanupCatAudit': '감사', 'security.nodeAuditTitle': '노드 작업 로그' },
  pt: { 'menu.audit': 'Registro de auditoria', 'nodeServer.auditLogs': 'Registros de auditoria', 'security.auditTitle': 'Registro de auditoria', 'logManage.cleanupCatAudit': 'Auditoria', 'security.nodeAuditTitle': 'Registros de operação dos nós' },
  tr: { 'menu.audit': 'Denetim Günlüğü', 'nodeServer.auditLogs': 'Denetim Günlükleri', 'security.auditTitle': 'Denetim Günlüğü', 'logManage.cleanupCatAudit': 'Denetim', 'security.nodeAuditTitle': 'Düğüm İşlem Günlükleri' },
  th: { 'menu.audit': 'บันทึกการตรวจสอบ', 'nodeServer.auditLogs': 'บันทึกการตรวจสอบ', 'security.auditTitle': 'บันทึกการตรวจสอบ', 'logManage.cleanupCatAudit': 'การตรวจสอบ', 'security.nodeAuditTitle': 'บันทึกการทำงานของโหนด' },
  vi: { 'menu.audit': 'Nhật ký kiểm toán', 'nodeServer.auditLogs': 'Nhật ký kiểm toán', 'security.auditTitle': 'Nhật ký kiểm toán', 'logManage.cleanupCatAudit': 'Kiểm toán', 'security.nodeAuditTitle': 'Nhật ký thao tác nút' },
  id: { 'menu.audit': 'Log Audit', 'nodeServer.auditLogs': 'Log Audit', 'security.auditTitle': 'Log Audit', 'logManage.cleanupCatAudit': 'Audit', 'security.nodeAuditTitle': 'Log Operasi Node' },
}

/** 值里出现这些标记 = 串键/错位的机器结果 */
const BROKEN_MARKERS = ['// ctx', 'return {', '<template>', '=>', 'ctx.ref', 'console.']
const NO_TRANSLATE = new Set(['common.appName'])

async function loadJs(file) {
  return (await import(`file://${file}`)).default
}
const loadJson = f => JSON.parse(readFileSync(f, 'utf8'))

function flatten(obj, prefix = '') {
  const out = {}
  for (const [k, v] of Object.entries(obj)) {
    const key = prefix ? `${prefix}.${k}` : k
    if (v && typeof v === 'object' && !Array.isArray(v)) Object.assign(out, flatten(v, key))
    else out[key] = v
  }
  return out
}

function fixGrouped(srcTemplate, texts) {
  const out = {}
  for (const [g, tpl] of Object.entries(srcTemplate)) {
    out[g] = {}
    for (const k of Object.keys(tpl)) {
      out[g][k] = texts[`${g}.${k}`] ?? tpl[k]
    }
  }
  return out
}

const jsModule = (obj, code) => `// ${NAMES[code] || code}（${code}）\nexport default ${JSON.stringify(obj, null, 2)}\n`
const jsonModule = obj => `${JSON.stringify(obj, null, 2)}\n`

async function main() {
  const s2tw = new OpenCC('s2tw.json')

  const enJs = await loadJs(join(ADMIN_DIR, 'en.js'))
  const zhJs = await loadJs(join(ADMIN_DIR, 'zh-CN.js'))
  const enJson = loadJson(join(PORTAL_DIR, 'en.json'))
  const zhJson = loadJson(join(PORTAL_DIR, 'zh-CN.json'))

  const zhFlat = flatten(zhJs)

  const codes = ['zh-TW', ...Object.keys(NAMES).filter(c => c !== 'zh-TW')]

  for (const code of codes) {
    const jsPath = join(ADMIN_DIR, `${code}.js`)
    const jsonPath = join(PORTAL_DIR, `${code}.json`)

    let jsFixed = 0, jsonFixed = 0, auditFixed = 0, nameFixed = 0

    if (code === 'zh-TW') {
      // 从简体源用 OpenCC 全量重建，键集合天然一致
      const rebuilt = {}
      for (const [k, v] of Object.entries(zhFlat)) {
        rebuilt[k] = typeof v === 'string' ? s2tw.convertSync(v) : v
      }
      const js = fixGrouped(enJs, rebuilt)
      const rj = { _meta: { flag: ' 🇨🇳'.trim(), label: '繁體中文', language: 'zh-TW' } }
      for (const [k, v] of Object.entries(zhJson)) {
        if (k === '_meta') continue
        rj[k] = typeof v === 'string' ? s2tw.convertSync(v) : v
      }
      // _meta 置首位
      const json = { _meta: rj._meta }
      delete rj._meta
      Object.assign(json, rj)
      jsFixed = Object.keys(zhFlat).length
      jsonFixed = Object.keys(zhJson).length - 1
      if (!DRY) {
        writeFileSync(jsPath, jsModule(js, code))
        writeFileSync(jsonPath, jsonModule(json))
      }
    } else {
      let js = await loadJs(jsPath)
      const fix = AUDIT_FIX[code] || {}

      if (js.common && js.common.appName && js.common.appName !== 'CoreFusion') {
        js.common.appName = 'CoreFusion'
        nameFixed++
      }

      const cur = flatten(js)
      for (const [key, val] of Object.entries(fix)) {
        if (cur[key] !== undefined && cur[key] !== val) { cur[key] = val; auditFixed++ }
      }
      js = fixGrouped(enJs, cur)

      let json = null
      try { json = loadJson(jsonPath) } catch {}
      if (json) {
        for (const [key, val] of Object.entries(fix)) {
          if (json[key] !== undefined && json[key] !== val) { json[key] = val; auditFixed++ }
        }
      }

      if (!DRY) {
        writeFileSync(jsPath, jsModule(js, code))
        if (json) writeFileSync(jsonPath, jsonModule(json))
      }
    }

    console.log(`${code}: 前端重建 ${jsFixed} 键, 门户重建 ${jsonFixed}, 产品名修正 ${nameFixed}, 审计日志修正 ${auditFixed}`)
  }

  console.log('\n=== 代码片段残留扫描 ===')
  for (const code of codes) {
    const flat = flatten(await loadJs(join(ADMIN_DIR, `${code}.js`)))
    const broken = Object.entries(flat).filter(([k, v]) =>
      typeof v === 'string' && !NO_TRANSLATE.has(k) && BROKEN_MARKERS.some(m => v.includes(m)))

    if (broken.length) {
      console.log(`${code}: ${broken.length} 处残留`)
      broken.slice(0, 8).forEach(([k, v]) => console.log(`    ${k} = ${JSON.stringify(v.slice(0, 60))}`))
    } else {
      console.log(`${code}: 0 处`)
    }
  }
}

main().catch(e => { console.error(e); process.exit(1) })
