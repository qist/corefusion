<template>
  <el-form label-width="160px" label-position="left">
    <el-form-item :label="t('singbox.protocol.httpMixed.domainResolver')">
      <el-input v-model="model.domain_resolver" :placeholder="t('singbox.outboundBase.domainResolver')" @change="emit" />
      <div class="hint">{{ t('singbox.protocol.httpMixed.domainResolverHint') }}</div>
    </el-form-item>
    <el-form-item v-if="model.type !== 'socks'" :label="t('singbox.protocol.httpMixed.setSystemProxy')">
      <el-switch v-model="model.set_system_proxy" @change="emit" />
    </el-form-item>

    <template v-if="model.type === 'http'">
      <el-divider content-position="left">{{ t('singbox.protocol.httpMixed.httpSection') }}</el-divider>
      <el-form-item :label="t('singbox.protocol.httpMixed.versions')">
        <el-select :model-value="httpVersions" style="width:100%" multiple collapse-tags @update:model-value="onVersionChange">
          <el-option label="HTTP/1.1" value="1" />
          <el-option label="HTTP/2" value="2" />
          <el-option label="HTTP/3" value="3" />
        </el-select>
      </el-form-item>
      <el-form-item :label="t('singbox.protocol.httpMixed.idleTimeout')">
        <el-input v-model="model._httpIdleTimeout" placeholder="30s" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.protocol.httpMixed.keepAlivePeriod')">
        <el-input v-model="model._httpKeepAlivePeriod" placeholder="15s" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.protocol.httpMixed.streamRecvWindow')">
        <el-input v-model="model._httpStreamRecvWindow" placeholder="8MB" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.protocol.httpMixed.connRecvWindow')">
        <el-input v-model="model._httpConnRecvWindow" placeholder="20MB" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.protocol.httpMixed.maxConcurrentStreams')">
        <el-input-number v-model="model._httpMaxConcurrentStreams" :min="0" style="width:100%" @change="emit" />
      </el-form-item>
      <template v-if="hasH3">
        <el-form-item :label="t('singbox.protocol.httpMixed.initialPacketSize')">
          <el-input-number v-model="model._httpInitialPacketSize" :min="0" style="width:100%" @change="emit" />
        </el-form-item>
        <el-form-item :label="t('singbox.protocol.httpMixed.disablePathMtu')">
          <el-switch v-model="model._httpDisablePathMtu" @change="emit" />
        </el-form-item>
      </template>
    </template>
  </el-form>
</template>

<script setup>
// http / mixed / socks 入站专属设置
// sing-box 源码：option/simple.go（SocksInboundOptions / HTTPMixedInboundOptions / HTTPInboundOptions）
// 注意：只有 http 有 version/HTTP2/QUIC 字段；mixed 只有 set_system_proxy；socks 只有 domain_resolver。
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
const props = defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }

const hasH3 = computed(() => String(props.model._httpVersion || '').split(',').map(s => s.trim()).includes('3'))
const httpVersions = computed(() => String(props.model._httpVersion || '').split(',').map(s => s.trim()).filter(Boolean))

function onVersionChange(v) {
  // el-select multiple + collapse-tags 需要使用数组；表单内部统一存逗号字符串
  props.model._httpVersion = Array.isArray(v) ? v.join(',') : String(v || '')
  emit()
}
</script>

<style scoped>
.hint { color: #909399; font-size: 12px; margin-top: 4px; }
</style>
