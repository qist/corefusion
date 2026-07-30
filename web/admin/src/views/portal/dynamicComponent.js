import { ref, reactive, computed, watch, onMounted, onUnmounted, defineComponent, h, compile } from 'vue'
import { ElMessage } from 'element-plus'

// Dynamic component compiler for the builder's live preview.
//
// Two problems with the naive approach:
//  1. Re-compiling on every render is expensive — compiled components are
//     cached by content signature so typing in a form doesn't rebuild the world.
//  2. The old `with (ctx) { ... }` wrapper is sloppy-mode only and diverges
//     from the real portal runtime. User JS must use `ctx.xxx`, exactly like
//     the production portal (see internal/service/portal/template.go).

const MAX_CACHE = 100
const cache = new Map()

function signature(mod) {
  const d = mod.data || {}
  return `${mod.id}${d.html || ''}${d.js || ''}`
}

function formatBytes(bytes) {
  if (!bytes && bytes !== 0) return '-'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let v = Number(bytes)
  while (v >= 1024 && i < units.length - 1) { v /= 1024; i++ }
  return v.toFixed(v >= 100 || i === 0 ? 0 : 1) + ' ' + units[i]
}

function formatDate(ts) {
  if (!ts) return '-'
  const d = new Date(typeof ts === 'number' ? ts * (ts < 1e12 ? 1000 : 1) : ts)
  return Number.isNaN(d.getTime()) ? String(ts) : d.toLocaleDateString()
}

function formatDateTime(ts) {
  if (!ts) return '-'
  const d = new Date(typeof ts === 'number' ? ts * (ts < 1e12 ? 1000 : 1) : ts)
  return Number.isNaN(d.getTime()) ? String(ts) : d.toLocaleString()
}

function formatMoney(cents) {
  return '¥' + (Number(cents || 0) / 100).toFixed(2)
}

function copyText(text) {
  return navigator.clipboard
    ? navigator.clipboard.writeText(text).then(() => ElMessage.success('Copied')).catch(() => {})
    : Promise.resolve()
}

const apiUnavailable = () => Promise.reject(new Error('API calls are not available in the builder preview'))

// Context exposed to freeform/custom JS in the preview — mirrors the shape of
// the production portal runtime ctx, with API calls stubbed out (the builder
// runs under an admin session, client APIs are not meaningful here).
function buildPreviewCtx(mod, t) {
  return {
    // Vue APIs
    ref, reactive, computed, watch, onMounted, onUnmounted,
    // Module data
    data: mod.data || {},
    mod,
    // i18n
    t: t || ((k) => k),
    lang: computed(() => ({})),
    // Utilities
    formatBytes, formatDate, formatDateTime, formatMoney, copyText,
    elMessage: ElMessage,
    ElMessage,
    renderMd: (md) => md || '',
    // API stub
    api: { get: apiUnavailable, post: apiUnavailable, put: apiUnavailable, delete: apiUnavailable, raw: apiUnavailable },
    // Navigation stub
    goToPage: () => {},
    activeTab: ref(''),
    // Config
    config: {},
    moduleId: mod.id,
    moduleTitle: mod.title || '',
  }
}

// Build (or fetch from cache) the preview component for a module.
// Returns null when the module has no custom HTML/JS to render.
export function getDynamicComponent(mod, t) {
  if (!mod) return null
  const html = mod.data?.html || ''
  const js = mod.data?.js || ''
  if (!html && !js && !mod.data?.css) return null

  const sig = signature(mod)
  const hit = cache.get(sig)
  if (hit) return hit

  let component = null
  try {
    const renderFn = html ? compile(html) : () => h('div')

    let componentLogic = {}
    if (js) {
      try {
        const ctx = buildPreviewCtx(mod, t)
        const factory = new Function('ctx', js)
        const result = factory(ctx)
        if (result && typeof result === 'object') {
          // Auto-merge common bindings like the production runtime does
          if (!result.t) result.t = ctx.t
          if (!result.mod) result.mod = ctx.mod
          componentLogic = result
        }
      } catch (jsErr) {
        console.error('[portal-builder] module JS error:', jsErr)
        componentLogic = { __error: ref(jsErr.message || String(jsErr)) }
      }
    }

    component = defineComponent({
      name: `DynamicModule-${mod.id}`,
      render: renderFn,
      setup() {
        return componentLogic
      },
    })
  } catch (err) {
    console.error('[portal-builder] component compile error:', err)
    return null
  }

  // Bound the cache — drop oldest entries first (Map preserves insertion order)
  if (cache.size >= MAX_CACHE) {
    cache.delete(cache.keys().next().value)
  }
  cache.set(sig, component)
  return component
}

export function clearDynamicComponentCache() {
  cache.clear()
}
