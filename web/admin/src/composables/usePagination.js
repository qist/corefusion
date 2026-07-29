import { ref, computed } from 'vue'
import { useQueryState } from './useQueryState'

/**
 * 列表分页统一封装。
 * - 页码 page 与每页大小 pageSize 持久化到 URL query（刷新不减）
 * - 每页大小封顶 100，默认 20，切换档位时回到第 1 页
 * - 对外暴露 limit/offset 给接口调用，items/total 给表格与分页组件
 *
 * @param {object} [opts]
 * @param {number} [opts.defaultSize=20]  默认每页条数
 * @param {number} [opts.maxSize=100]     每页上限
 * @param {string} [opts.pageKey='page']         页码 query 参数名
 * @param {string} [opts.pageSizeKey='psize']     每页大小 query 参数名
 */
export function usePagination(opts = {}) {
  const { defaultSize = 20, maxSize = 100, pageKey = 'page', pageSizeKey = 'psize' } = opts

  const page = useQueryState(pageKey, 1, { type: 'number' })
  const pageSizeRaw = useQueryState(pageSizeKey, defaultSize, { type: 'number' })

  const total = ref(0)
  const items = ref([])

  const pageSize = computed({
    get() {
      let s = Number(pageSizeRaw.value)
      if (!s || s < 1) s = defaultSize
      return Math.min(s, maxSize)
    },
    set(val) {
      pageSizeRaw.value = val
    },
  })
  const limit = computed(() => pageSize.value)
  const offset = computed(() => (Math.max(Number(page.value) || 1, 1) - 1) * limit.value)

  /** 请求返回后调用：把 {items,total} 写回 */
  function applyResult(result) {
    items.value = result?.items || []
    total.value = result?.total || 0
  }

  /** 每页大小变化：回到第 1 页 */
  function onSizeChange(size) {
    pageSizeRaw.value = size
    page.value = 1
  }

  /** 页码变化 */
  function onPageChange(p) {
    page.value = p
  }

  return {
    page,
    pageSize,
    limit,
    offset,
    total,
    items,
    applyResult,
    onSizeChange,
    onPageChange,
  }
}
