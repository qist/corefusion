<template>
  <el-form label-width="140px" label-position="left">
    <el-form-item :label="t('singbox.protocol.anytls.paddingScheme')">
      <el-input
        v-model="model._anytlsPaddingScheme"
        type="textarea"
        :rows="8"
        :placeholder="t('singbox.protocol.anytls.paddingSchemePlaceholder')"
        @change="emit"
      />
      <div class="hint">{{ t('singbox.protocol.anytls.paddingSchemeHint') }}</div>
      <div style="display:flex;gap:8px">
        <el-button size="small" @click="fillDefault">{{ t('singbox.protocol.anytls.fillDefault') }}</el-button>
        <el-button size="small" @click="model._anytlsPaddingScheme = ''; emit()">{{ t('singbox.protocol.anytls.clear') }}</el-button>
      </div>
    </el-form-item>
  </el-form>
</template>

<script setup>
// AnyTLS 入站专属设置（sing-box option/anytls.go: AnyTLSInboundOptions.PaddingScheme）
// 用户列表（name/password）与 TLS 由容器层统一提供。
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
const props = defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }

// sing-box 文档给出的默认填充方案，留空即使用该方案
const DEFAULT_PADDING = [
  'stop=8',
  '0=30-30',
  '1=100-400',
  '2=400-500,c,500-1000,c,500-1000,c,500-1000,c,500-1000',
  '3=9-9,500-1000',
  '4=500-1000',
  '5=500-1000',
  '6=500-1000',
  '7=500-1000',
]
function fillDefault() {
  props.model._anytlsPaddingScheme = DEFAULT_PADDING.join('\n')
  emit()
}
</script>

<style scoped>
.hint { color: #909399; font-size: 12px; margin-top: 4px; margin-bottom: 8px; }
</style>
