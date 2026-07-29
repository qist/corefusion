import { ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

/**
 * 让组件内的本地状态（tab / 分页 / 筛选 / 节点选择等）与 URL query 双向同步。
 * 刷新页面后从 query 恢复，避免回到默认状态。
 *
 * @param {string} key      query 参数名
 * @param {*}      fallback  默认值（刷新且 query 缺失时使用）
 * @param {object} [opts]
 * @param {('string'|'number')} [opts.type='string'] 值类型
 * @param {(v:any)=>boolean}    [opts.omit]  当值等于该返回 true 时不写入 query（通常是默认值）
 */
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

  watch(
    state,
    (val) => {
      const q = { ...route.query }
      const shouldOmit = omit ? omit(val) : val === fallback
      if (shouldOmit) {
        if (key in q) delete q[key]
      } else {
        q[key] = type === 'number' ? String(val) : String(val)
      }
      router.replace({ query: q }).catch(() => {})
    },
    { flush: 'sync' }
  )

  return state
}
