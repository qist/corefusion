<template>
  <div class="mobile-cards">
    <el-card v-for="(row, i) in rows" :key="rowKey(row, i)" shadow="never" class="mobile-card">
      <div class="mc-title">{{ cardTitle(row, i) }}</div>
      <div v-for="f in visibleFields(row)" :key="f.label" class="mc-field">
        <span class="mc-label">{{ f.label }}</span>
        <span class="mc-value">{{ f.value }}</span>
      </div>
      <div v-if="$slots.actions" class="mc-actions">
        <slot name="actions" :row="row" />
      </div>
    </el-card>
  </div>
</template>

<script setup>
// 移动端表格替代品：把行渲染成卡片。
// fields: [{ label, text: (row)=>string, hide?: (row)=>bool }]
// titleField: 卡片标题取 row 的哪个字段；actions 插槽复用页面现有按钮。
const props = defineProps({
  rows: { type: Array, default: () => [] },
  fields: { type: Array, default: () => [] },
  titleField: { type: String, default: 'name' },
  // 标题也可以传函数（如按 client_id 反查邮箱）
  titleFn: { type: Function, default: null },
})

const props_ = props
function cardTitle(row, i) {
  if (props_.titleFn) return props_.titleFn(row) || '#' + (row.id ?? i)
  return row[props_.titleField] || '#' + (row.id ?? i)
}
function visibleFields(row) {
  return props_.fields
    .filter(f => !(f.hide && f.hide(row)))
    .map(f => ({ label: f.label, value: f.text(row) }))
    .filter(f => f.value !== '' && f.value != null)
}
function rowKey(row, i) {
  return row.id ?? row.ID ?? row.email ?? row.token ?? i
}
</script>

<style scoped>
.mobile-cards { display: flex; flex-direction: column; gap: 8px; }
.mobile-card { --el-card-padding: 10px 12px; }
.mc-title { font-weight: 600; font-size: 14px; margin-bottom: 6px; word-break: break-all; }
.mc-field { display: flex; justify-content: space-between; gap: 12px; font-size: 12px; line-height: 1.8; }
.mc-label { color: #909399; flex: 0 0 auto; }
.mc-value { text-align: right; word-break: break-all; }
.mc-actions { display: flex; flex-wrap: wrap; gap: 4px; margin-top: 8px; padding-top: 8px; border-top: 1px solid #ebeef5; }
.mc-actions :deep(.el-button) { margin-left: 0; }
</style>
