import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

/**
 * 让组件内的本地状态从 URL query 初始化（刷新后恢复）。
 * 不自动同步回 URL —— 避免在 setup/watch 阶段触发 router.replace
 * 导致的响应式循环和页面卡死。
 */
export function useQueryState(key, fallback, opts = {}) {
  const { type = 'string' } = opts
  const route = useRoute()

  const raw = route.query[key]
  let initial = fallback
  if (raw !== undefined && raw !== '') {
    initial = type === 'number' ? Number(raw) : String(raw)
  }

  return ref(initial)
}
