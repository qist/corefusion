import { ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

/**
 * 让组件内的本地状态与 URL query 双向同步（刷新后恢复）。
 * - 读：setup 时从 route.query 初始化
 * - 写：状态变化时（用户交互）用 router.replace 把值合并写回 query —— replace 不产生历史记录
 *
 * 安全说明（历史教训）：只能在「state 变化的 watch」里写，绝不能在 setup 阶段主动触发
 * router.replace —— 当年在 setup/watch 同步 route 导致响应式循环、页面卡死。
 * 同一 tick 内多个 key 变更（如分页的 size+page）合并为一次 replace，避免快照互相覆盖。
 */
let pendingReplace = null
function scheduleQueryReplace(router, route, key, val, remove) {
  if (!pendingReplace) {
    pendingReplace = { query: { ...route.query } }
    Promise.resolve().then(() => {
      const payload = pendingReplace
      pendingReplace = null
      router.replace({ query: payload.query }).catch(() => {})
    })
  }
  if (remove) delete pendingReplace.query[key]
  else pendingReplace.query[key] = val
}

export function useQueryState(key, fallback, opts = {}) {
  const { type = 'string', omit } = opts
  const route = useRoute()
  const router = useRouter()

  const raw = route.query[key]
  let initial = fallback
  if (raw !== undefined && raw !== '') {
    initial = type === 'number' ? Number(raw) : String(raw)
  }

  const state = ref(initial)

  watch(state, (val) => {
    const isDefault = val === '' || val == null || String(val) === String(fallback)
    const remove = isDefault || (omit ? omit(val) : false)
    const next = remove ? undefined : String(val)
    // 与当前 URL 一致则不写，避免多余导航
    const cur = route.query[key]
    const curStr = cur === undefined ? undefined : String(cur)
    if (curStr === next) return
    scheduleQueryReplace(router, route, key, next, remove)
  })

  return state
}
