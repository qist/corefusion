<template>
  <div class="pagination-bar">
    <el-pagination
      :current-page="page"
      :page-size="pageSize"
      :total="total"
      :page-sizes="pageSizes"
      :layout="layout"
      background
      @current-change="onPageChange"
      @size-change="onSizeChange"
    />
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  page: { type: Number, required: true },
  pageSize: { type: Number, required: true },
  total: { type: Number, default: 0 },
  pageSizes: { type: Array, default: () => [20, 50, 100] },
  layout: { type: String, default: 'total, sizes, prev, pager, next, jumper' },
})

const emit = defineEmits(['update:page', 'update:pageSize', 'change'])

function onPageChange(p) {
  emit('update:page', p)
  emit('change')
}
function onSizeChange(s) {
  emit('update:pageSize', s)
  emit('update:page', 1)
  emit('change')
}
</script>

<style scoped>
.pagination-bar {
  display: flex;
  justify-content: flex-end;
  margin-top: 12px;
}
</style>
