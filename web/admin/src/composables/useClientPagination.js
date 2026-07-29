import { computed } from 'vue'
import { useQueryState } from './useQueryState'

// Client-side (in-memory) pagination helper for lists that are fetched
// in full from the backend (e.g. node-scoped config lists that are not
// server-paginated). Keeps all items in the source ref/computed and slices
// the current page reactively.
//
// Usage:
//   const { paged: sniList, page: sniPage, pageSize: sniSize, total: sniTotal } =
//     useClientPagination(sniRules, { pageKey: 'sniPage', pageSizeKey: 'sniSize' })
//
//   <el-table :data="sniList" />
//   <PaginationBar v-model:page="sniPage" v-model:page-size="sniSize" :total="sniTotal" />
export function useClientPagination(source, opts = {}) {
  const defaultSize = opts.defaultSize || 20
  const maxSize = opts.maxSize || 100
  const pageKey = opts.pageKey || 'page'
  const pageSizeKey = opts.pageSizeKey || 'psize'

  // Writable refs bound to <PaginationBar v-model:page / v-model:page-size>.
  const page = useQueryState(pageKey, 1, { type: 'number' })
  const pageSize = useQueryState(pageSizeKey, defaultSize, { type: 'number' })

  // Clamped effective page size used for slicing (guards against bad URL params).
  const effectivePageSize = computed(() => {
    let s = Number(pageSize.value)
    if (!s || s < 1) s = defaultSize
    return Math.min(s, maxSize)
  })

  const total = computed(() => {
    const arr = source && source.value
    return Array.isArray(arr) ? arr.length : 0
  })

  const paged = computed(() => {
    const arr = (source && source.value) || []
    const p = Math.max(Number(page.value) || 1, 1)
    const start = (p - 1) * effectivePageSize.value
    return arr.slice(start, start + effectivePageSize.value)
  })

  return { page, pageSize, total, paged }
}
