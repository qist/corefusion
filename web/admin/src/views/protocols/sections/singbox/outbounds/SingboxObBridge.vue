<template>
  <el-form label-width="150px" label-position="left">
    <!-- Bridge 出站：纯 tag 类型，无协议专属字段（仅基础 tag/base 由容器层提供） -->
    <div style="color:#909399;font-size:12px;padding:8px 0">
      {{ t('singbox.obBridge.hint') }}
    </div>
  </el-form>
</template>

<script setup>
// Bridge 出站：无协议专属字段（仅基础 tag 由容器层提供）
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>
