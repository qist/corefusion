<template>
  <el-form label-width="170px" label-position="left">
    <el-form-item :label="t('singbox.obProtocol.username')">
      <div style="display:flex;gap:8px;flex:1">
        <el-input v-model="model.username" @change="emit" />
        <el-button size="small" @click="model.username = generateUsername(); emit()">{{ t('singbox.users.generate') }}</el-button>
      </div>
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.password')">
      <div style="display:flex;gap:8px;flex:1">
        <el-input v-model="model.password" @change="emit" />
        <el-button size="small" @click="model.password = generatePassword(); emit()">{{ t('singbox.users.generate') }}</el-button>
      </div>
    </el-form-item>

    <template v-if="model.type === 'socks'">
      <el-form-item :label="t('singbox.obProtocol.socksVersion')">
        <el-select v-model="model.version" style="width:100%" @change="emit">
          <el-option label="5" value="5" />
          <el-option label="4" value="4" />
          <el-option label="4a" value="4a" />
        </el-select>
      </el-form-item>
      <el-form-item :label="t('singbox.obProtocol.udpOverTcp')">
        <el-switch v-model="model._udpOverTCP" @change="emit" />
      </el-form-item>
      <el-form-item v-if="model._udpOverTCP" :label="t('singbox.obProtocol.udpOverTcpVersion')">
        <el-select v-model="model._udpOverTCPVersion" style="width:100%" @change="emit">
          <el-option :label="t('singbox.obProtocol.uotDefault')" :value="0" />
          <el-option label="1" :value="1" />
          <el-option label="2" :value="2" />
        </el-select>
      </el-form-item>
    </template>

    <template v-if="model.type === 'http'">
      <el-form-item :label="t('singbox.obProtocol.httpVersion')">
        <el-select v-model="model.version" style="width:100%" @change="emit">
          <el-option label="auto" :value="0" />
          <el-option label="HTTP/1.1" :value="1" />
          <el-option label="HTTP/2" :value="2" />
          <el-option label="HTTP/3" :value="3" />
        </el-select>
      </el-form-item>
      <template v-if="Number(model.version) <= 1">
        <el-form-item :label="t('singbox.obProtocol.path')">
          <el-input v-model="model.path" placeholder="/" @change="emit" />
        </el-form-item>
        <el-form-item :label="t('singbox.obProtocol.headers')">
          <div v-for="(h, i) in (model._httpHeaders || [])" :key="i" style="display:flex;gap:8px;margin-bottom:6px">
            <el-input v-model="h.key" placeholder="Header" style="flex:1" @change="emit" />
            <el-input v-model="h.value" placeholder="Value" style="flex:1" @change="emit" />
            <el-button size="small" type="danger" @click="model._httpHeaders.splice(i, 1); emit()">X</el-button>
          </div>
          <el-button size="small" @click="addHeader">{{ t('singbox.obProtocol.addHeader') }}</el-button>
          <div class="hint">{{ t('singbox.obProtocol.http1OnlyHint') }}</div>
        </el-form-item>
      </template>
      <el-form-item :label="t('singbox.obProtocol.disableVersionFallback')">
        <el-switch v-model="model.disable_version_fallback" @change="emit" />
      </el-form-item>

      <el-divider content-position="left">{{ t('singbox.obProtocol.quicSection') }}</el-divider>
      <el-form-item :label="t('singbox.obProtocol.idleTimeout')">
        <el-input v-model="model._httpIdleTimeout" placeholder="30s" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.obProtocol.keepAlivePeriod')">
        <el-input v-model="model._httpKeepAlivePeriod" placeholder="15s" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.obProtocol.streamRecvWindow')">
        <el-input v-model="model._httpStreamRecvWindow" placeholder="8MB" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.obProtocol.connRecvWindow')">
        <el-input v-model="model._httpConnRecvWindow" placeholder="20MB" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.obProtocol.maxConcurrentStreams')">
        <el-input-number v-model="model._httpMaxConcurrentStreams" :min="0" style="width:100%" @change="emit" />
      </el-form-item>
      <template v-if="Number(model.version) === 3">
        <el-form-item :label="t('singbox.obProtocol.initialPacketSize')">
          <el-input-number v-model="model._httpInitialPacketSize" :min="0" style="width:100%" @change="emit" />
        </el-form-item>
        <el-form-item :label="t('singbox.obProtocol.disablePathMtu')">
          <el-switch v-model="model._httpDisablePathMtu" @change="emit" />
        </el-form-item>
      </template>
    </template>
  </el-form>
</template>

<script setup>
// http / socks 出站共用组件（按 type 显示各自字段）
import { useI18n } from 'vue-i18n'
import { generateUsername, generatePassword } from '../utils/singboxDefaults'
const { t } = useI18n()
const props = defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
function addHeader() {
  if (!Array.isArray(props.model._httpHeaders)) props.model._httpHeaders = []
  props.model._httpHeaders.push({ key: '', value: '' })
  emit()
}
</script>

<style scoped>
.hint { color: #909399; font-size: 12px; margin-top: 4px; }
</style>
