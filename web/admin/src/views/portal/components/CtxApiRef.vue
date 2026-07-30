<template>
  <div class="freeform-api-ref">
    <el-collapse>
      <el-collapse-item :title="t('portal.ctxApiRef')">
        <pre>{{ t('portal.ctxApiRefBody') }}</pre>
      </el-collapse-item>
    </el-collapse>
  </div>
</template>

<script setup>
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
</script>

<style scoped>
.freeform-api-ref { margin-top: 8px; }
.freeform-api-ref pre { font-size: 12px; line-height: 1.6; color: #606266; white-space: pre-wrap; word-break: break-all; margin: 0; }
.freeform-api-ref :deep(.el-collapse) { border: none; }
.freeform-api-ref :deep(.el-collapse-item__header) { font-size: 12px; height: 32px; line-height: 32px; }
</style>
