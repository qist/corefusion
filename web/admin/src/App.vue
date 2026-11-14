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
  html, body { overflow-x: hidden; -webkit-text-size-adjust: 100%; }

  /* 主内容区收紧内边距（覆盖 el-main 行内 20px） */
  .el-main { padding: 8px !important; }
  .el-card__body { padding: 10px; }
  .el-card__header { padding: 12px 10px; }
  .el-header { padding: 0 10px !important; gap: 10px !important; font-size: 13px; }
  .el-footer { padding: 0; }

  /* 弹窗统一占满可视宽度，避免固定 width=700/800 溢出屏幕 */
  .el-overlay-dialog { padding: 0 8px; }
  .el-dialog {
    width: calc(100vw - 16px) !important;
    max-width: calc(100vw - 16px) !important;
    margin: 6vh auto !important;
    border-radius: 10px;
  }
  .el-dialog__header { padding: 14px 14px 6px; }
  .el-dialog__body { padding: 10px 14px; }
  .el-dialog__footer { padding: 8px 14px 14px; display: flex; flex-wrap: wrap; gap: 8px; justify-content: flex-end; }
  .el-drawer { max-width: 100vw; }
  .el-message-box { width: calc(100vw - 28px) !important; max-width: 420px; }
  .el-message { min-width: 0; max-width: 92vw; box-sizing: border-box; }
  /* 确认框底部按钮等宽排布，避免长外语按钮挤成两行 */
  .el-message-box__btns { display: flex; gap: 8px; }
  .el-message-box__btns .el-button { flex: 1; margin-left: 0; }

  /* 表单：窄屏标签一律上置，内容占满整行 */
  .el-form-item { display: block; }
  .el-form-item__label {
    display: block;
    width: auto !important;
    text-align: left !important;
    padding: 0 0 4px !important;
    height: auto;
    line-height: 1.4;
    white-space: normal;
    word-break: break-word;
  }
  .el-form-item__content { margin-left: 0 !important; }
  .el-form--inline .el-form-item { display: block; width: 100%; margin-right: 0; }
  .el-form--label-top .el-form-item__label { padding-bottom: 4px !important; }

  /* 输入类控件不超出屏幕（覆盖页面里 style="width:240px" 之类的固定宽度） */
  .el-input, .el-select, .el-cascader, .el-date-editor, .el-autocomplete,
  .el-input-number, .el-textarea { max-width: 100%; }
  .el-input-number { width: 100%; }

  /* 表格更紧凑：按钮间距/单元格内边距缩小 */
  .el-table { font-size: 12px; }
  .el-table .el-button + .el-button { margin-left: 6px; }
  .el-table .cell { padding: 0 6px; }

  /* 统计卡片（span=4/6 的列）每行两张 */
  .el-col-4, .el-col-6 { flex: 0 0 50%; max-width: 50%; }
  /* 内容卡片（span=8 的列，如套餐模板卡）手机端全宽 */
  .el-col-8 { flex: 0 0 100%; max-width: 100%; }
  /* 双栏内容（span=12/16）手机端上下堆叠 */
  .el-col-12, .el-col-16 { flex: 0 0 100%; max-width: 100%; }
  /* 仪表盘"集群资源/高负载节点"双列手机端上下堆叠 */
  .el-col-10, .el-col-14 { flex: 0 0 100%; max-width: 100%; }

  /* 页面工具行（标题 + 操作/筛选）允许换行，避免横向溢出 */
  .el-main div[style*="justify-content:space-between"],
  .el-main div[style*="justify-content: space-between"] { flex-wrap: wrap !important; row-gap: 8px; }
  /* 工具行里的固定宽度控件在手机上占满一行 */
  .el-main div[style*="display:flex"] > .el-input,
  .el-main div[style*="display:flex"] > .el-select {
    width: auto !important;
    flex: 1 1 140px;
    min-width: 0;
  }

  /* 表单提示文案：默认 32px 行高会让小字间距过大 */
  .el-form-item__content { line-height: 1.5; }

  /* 标签页：缩小左右内边距并允许横向滚动，避免标签被挤成两行 */
  .el-tabs__header { margin-bottom: 10px; }
  .el-tabs__item { padding: 0 10px; font-size: 13px; height: 36px; line-height: 36px; }
  .el-tabs--border-card > .el-tabs__header .el-tabs__item { padding: 0 10px; }

  /* 手机上标签过多时改为自动换行，而不是用左右箭头遮住标签文字 */
  .el-tabs__nav-wrap { overflow: visible; }
  .el-tabs__nav-scroll { overflow: visible; }
  .el-tabs__nav { flex-wrap: wrap; white-space: normal; transform: none !important; }
  .el-tabs__nav-prev, .el-tabs__nav-next { display: none; }
  .el-tabs__active-bar { display: none; }
  .el-tabs__item.is-active { font-weight: 600; }

  /* 超窄屏再收紧一档，让 4~5 个中文标签尽量一屏放下 */
  @media (max-width: 480px) {
    .el-tabs__item { padding: 0 8px; font-size: 12px; }
  }

  /* 页面内标题统一字号，避免默认 h2 在手机上过大 */
  .el-main h2 { font-size: 18px; }
  .el-main h3 { font-size: 15px; }

  /* 分页在窄屏居中换行 */
  .el-pagination { flex-wrap: wrap; row-gap: 6px; justify-content: center; gap: 4px; }
  .el-pager li { min-width: 28px; height: 28px; line-height: 28px; }
  .el-pagination button { min-width: 28px; height: 28px; }
  /* 手机上页码/总数足够用，隐藏每页条数选择与跳页输入，避免挤成两行 */
  .el-pagination__sizes, .el-pagination__jump { display: none !important; }
  .pagination-bar { justify-content: center !important; }

  /* 移动端菜单抽屉去内边距 */
  .mobile-menu-drawer .el-drawer__body { padding: 0; }
}

/* 次要数据表在窄屏保底：保留可读列宽，横向滚动而不是把列挤成条状。
   用法：<div class="table-scroll"><el-table .../></div> */
.table-scroll { width: 100%; overflow-x: auto; -webkit-overflow-scrolling: touch; }
@media (max-width: 768px) {
  .table-scroll .el-table { min-width: 620px; }
}
</style>
