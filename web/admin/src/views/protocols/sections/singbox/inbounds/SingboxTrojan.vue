<template>
  <el-form label-width="160px" label-position="left">
    <el-divider content-position="left">{{ t('singbox.protocol.trojan.fallbackSection') }}</el-divider>
    <el-form-item :label="t('singbox.protocol.trojan.fallbackServer')">
      <el-input v-model="model._trojanFallbackServer" placeholder="127.0.0.1" @change="emit" />
      <div class="hint">{{ t('singbox.protocol.trojan.fallbackHint') }}</div>
    </el-form-item>
    <el-form-item v-if="model._trojanFallbackServer" :label="t('singbox.protocol.trojan.fallbackPort')">
      <el-input-number v-model="model._trojanFallbackPort" :min="1" :max="65535" style="width:100%" @change="emit" />
    </el-form-item>

    <el-divider content-position="left">{{ t('singbox.protocol.trojan.fallbackAlpnSection') }}</el-divider>
    <div v-for="(e, i) in (model._trojanFallbackHsn || [])" :key="i" style="display:flex;gap:8px;margin-bottom:8px">
      <el-input v-model="e.alpn" placeholder="h2" style="flex:1" @change="emit" />
      <el-input v-model="e.server" placeholder="127.0.0.1" style="flex:1" @change="emit" />
      <el-input-number v-model="e.server_port" :min="1" :max="65535" style="width:130px" @change="emit" />
      <el-button size="small" type="danger" @click="model._trojanFallbackHsn.splice(i, 1); emit()">X</el-button>
    </div>
    <el-button size="small" type="primary" plain @click="addFallback">{{ t('singbox.protocol.trojan.addFallback') }}</el-button>
  </el-form>
</template>

<script setup>
// Trojan 入站专属设置（option/trojan.go：fallback / fallback_for_alpn）
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
const props = defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }

function addFallback() {
  if (!Array.isArray(props.model._trojanFallbackHsn)) props.model._trojanFallbackHsn = []
  props.model._trojanFallbackHsn.push({ alpn: '', server: '', server_port: 80 })
  emit()
}
</script>

<style scoped>
.hint { color: #909399; font-size: 12px; margin-top: 4px; }
</style>
