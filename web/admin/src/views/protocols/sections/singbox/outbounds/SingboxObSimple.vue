<template>
  <el-form label-width="150px" label-position="left">
    <el-form-item :label="t('singbox.outboundSimple.tag')">
      <el-input v-model="model.tag" :placeholder="t('singbox.outboundSimple.tagPlaceholder')" @change="emit" />
    </el-form-item>
  </el-form>
</template>

<script setup>
// direct / block / dns 简单出站：仅需 tag
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>