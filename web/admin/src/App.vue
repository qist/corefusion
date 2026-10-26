<template>
  <el-config-provider :locale="elLocale">
    <router-view />
  </el-config-provider>
</template>

<script setup>
import { computed, watch, ref } from 'vue'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import enUs from 'element-plus/es/locale/lang/en'
import i18n from './locales'

const lang = ref(i18n.global.locale.value)

watch(() => i18n.global.locale.value, (v) => { lang.value = v })

const elLocale = computed(() => lang.value === 'en' ? enUs : zhCn)
</script>

<style>
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}
body {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
}

/* WAF 配置表单：多语言长标签允许换行，避免与控件/提示文案重叠 */
.waf-config-form .el-form-item {
  align-items: flex-start;
}
.waf-config-form .el-form-item__label {
  display: block;
  white-space: normal;
  word-break: break-word;
  height: auto;
  min-height: 24px;
  padding-top: 2px;
  padding-bottom: 2px;
  line-height: 1.4;
}
.waf-config-form .el-form-item__content {
  row-gap: 4px;
}
.waf-config-form .el-form-item__content > span {
  flex-basis: 100%;
  margin-left: 0 !important;
  margin-top: 2px;
  line-height: 1.4;
}

/* ── 移动端全局适配（≤768px）── */
@media (max-width: 768px) {
  /* 弹窗/消息框不超出屏幕 */
  .el-dialog { max-width: 94vw !important; }
  .el-message-box { max-width: 92vw !important; }
  .el-message { max-width: 92vw; box-sizing: border-box; }
  /* 主内容区收紧内边距（覆盖 el-main 行内 20px） */
  .el-main { padding: 10px !important; }
  /* 卡片内边距收紧 */
  .el-card__body { padding: 12px; }
  /* 统计卡片（span=4 的列）每行两张 */
  .el-col-4 { flex: 0 0 50%; max-width: 50%; }
  /* 内容卡片（span=8 的列，如套餐模板卡）手机端全宽 */
  .el-col-8 { flex: 0 0 100%; max-width: 100%; }
  /* 仪表盘"集群资源/高负载节点"双列手机端上下堆叠 */
  .el-col-10, .el-col-14 { flex: 0 0 100%; max-width: 100%; }
  /* 移动端菜单抽屉去内边距 */
  .mobile-menu-drawer .el-drawer__body { padding: 0; }
  /* 表格更紧凑：按钮间距/单元格内边距缩小 */
  .el-table .el-button + .el-button { margin-left: 6px; }
  .el-table .cell { padding: 0 6px; }
  /* 表头按钮区允许换行 */
  .el-header { padding: 0 12px; }
  /* 页面工具行（标题+按钮的 space-between 行）允许换行，避免横向溢出 */
  .el-main div[style*="justify-content:space-between"] { flex-wrap: wrap; row-gap: 8px; }
  /* 弹窗内表单：窄屏标签上置，避免 100-140px 的 label 挤压内容 */
  .el-dialog .el-form-item { display: block; }
  .el-dialog .el-form-item__label { display: block; width: auto !important; text-align: left; padding-bottom: 4px; }
}
</style>
