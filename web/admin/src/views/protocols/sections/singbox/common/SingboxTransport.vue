<template>
  <div>
    <el-form label-width="140px" label-position="left">
      <el-form-item :label="t('singbox.transport.transport')">
        <el-select v-model="model._transportType" style="width:100%" clearable @change="onChange">
          <el-option :label="t('singbox.transport.noneTcp')" value="" />
          <el-option label="http" value="http" />
          <el-option label="ws" value="ws" />
          <el-option label="quic" value="quic" />
          <el-option label="grpc" value="grpc" />
          <el-option label="httpupgrade" value="httpupgrade" />
        </el-select>
        <div v-if="model._realityEnabled && !realityTransportOk" class="hint warn-hint">
          {{ t('singbox.transport.realityOnly') }}
        </div>
      </el-form-item>
    </el-form>

    <!-- ws -->
    <template v-if="model._transportType === 'ws'">
      <el-form label-width="140px" label-position="left">
        <el-form-item :label="t('singbox.transport.path')">
          <el-input v-model="model._transportPath" :placeholder="t('singbox.transport.pathPlaceholder')" @change="emit" />
        </el-form-item>
        <el-form-item :label="t('singbox.transport.headers')">
          <div v-for="(h, i) in model._transportHeaders" :key="i" style="display:flex;gap:8px;margin-bottom:4px">
            <el-input v-model="h.key" placeholder="Name" style="width:40%" @change="emit" />
            <el-input v-model="h.value" placeholder="Value" style="width:50%" @change="emit" />
            <el-button size="small" type="danger" text @click="model._transportHeaders.splice(i,1);emit()">X</el-button>
          </div>
          <el-button size="small" @click="model._transportHeaders.push({key:'',value:''});emit()">+</el-button>
        </el-form-item>
        <el-form-item :label="t('singbox.transport.maxEarlyData')">
          <el-input-number v-model="model._transportMaxEarlyData" :min="0" style="width:100%" @change="emit" />
        </el-form-item>
        <el-form-item :label="t('singbox.transport.earlyDataHeader')">
          <el-input v-model="model._transportEarlyDataHeader" placeholder="Sec-WebSocket-Protocol" @change="emit" />
        </el-form-item>
      </el-form>
    </template>

    <!-- grpc -->
    <template v-if="model._transportType === 'grpc'">
      <el-form label-width="140px" label-position="left">
        <el-form-item :label="t('singbox.transport.serviceName')">
          <el-input v-model="model._transportServiceName" @change="emit" />
        </el-form-item>
        <el-form-item :label="t('singbox.transport.idleTimeout')">
          <el-input v-model="model._transportIdleTimeout" placeholder="15s" @change="emit" />
        </el-form-item>
        <el-form-item :label="t('singbox.transport.pingTimeout')">
          <el-input v-model="model._transportPingTimeout" placeholder="15s" @change="emit" />
        </el-form-item>
        <el-form-item :label="t('singbox.transport.permitWithoutStream')">
          <el-switch v-model="model._transportPermitWithoutStream" @change="emit" />
        </el-form-item>
      </el-form>
    </template>

    <!-- http -->
    <template v-if="model._transportType === 'http'">
      <el-form label-width="140px" label-position="left">
        <el-form-item :label="t('singbox.transport.host')">
          <el-input v-model="model._transportHost" @change="emit" />
        </el-form-item>
        <el-form-item :label="t('singbox.transport.path')">
          <el-input v-model="model._transportPath" :placeholder="t('singbox.transport.pathPlaceholder')" @change="emit" />
        </el-form-item>
        <el-form-item :label="t('singbox.transport.method')">
          <el-select v-model="model._transportMethod" style="width:100%" clearable @change="emit">
            <el-option label="POST" value="POST" />
            <el-option label="GET" value="GET" />
            <el-option label="PUT" value="PUT" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('singbox.transport.headers')">
          <div v-for="(h, i) in model._transportHeaders" :key="i" style="display:flex;gap:8px;margin-bottom:4px">
            <el-input v-model="h.key" placeholder="Name" style="width:40%" @change="emit" />
            <el-input v-model="h.value" placeholder="Value" style="width:50%" @change="emit" />
            <el-button size="small" type="danger" text @click="model._transportHeaders.splice(i,1);emit()">X</el-button>
          </div>
          <el-button size="small" @click="model._transportHeaders.push({key:'',value:''});emit()">+</el-button>
        </el-form-item>
        <el-form-item :label="t('singbox.transport.idleTimeout')">
          <el-input v-model="model._transportIdleTimeout" placeholder="15s" @change="emit" />
        </el-form-item>
        <el-form-item :label="t('singbox.transport.pingTimeout')">
          <el-input v-model="model._transportPingTimeout" placeholder="15s" @change="emit" />
        </el-form-item>
      </el-form>
    </template>

    <!-- httpupgrade -->
    <template v-if="model._transportType === 'httpupgrade'">
      <el-form label-width="140px" label-position="left">
        <el-form-item :label="t('singbox.transport.host')">
          <el-input v-model="model._transportHost" @change="emit" />
        </el-form-item>
        <el-form-item :label="t('singbox.transport.path')">
          <el-input v-model="model._transportPath" :placeholder="t('singbox.transport.pathPlaceholder')" @change="emit" />
        </el-form-item>
        <el-form-item :label="t('singbox.transport.headers')">
          <div v-for="(h, i) in model._transportHeaders" :key="i" style="display:flex;gap:8px;margin-bottom:4px">
            <el-input v-model="h.key" placeholder="Name" style="width:40%" @change="emit" />
            <el-input v-model="h.value" placeholder="Value" style="width:50%" @change="emit" />
            <el-button size="small" type="danger" text @click="model._transportHeaders.splice(i,1);emit()">X</el-button>
          </div>
          <el-button size="small" @click="model._transportHeaders.push({key:'',value:''});emit()">+</el-button>
        </el-form-item>
      </el-form>
    </template>

  </div>
</template>

<script setup>
// 通用 V2Ray Transport 组件（http/ws/quic/grpc/httpupgrade）
// 参考 sing-box 官方文档：docs/configuration/shared/v2ray-transport.md
// model: 协议 form 响应式对象
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { supportsReality } from '../utils/singboxDefaults'
const { t } = useI18n()
const props = defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
function onChange() {
  if (props.model._transportType === 'ws' && !props.model._transportPath) props.model._transportPath = '/'
  if ((props.model._transportType === 'http' || props.model._transportType === 'httpupgrade') && !props.model._transportPath) props.model._transportPath = '/'
  if (!Array.isArray(props.model._transportHeaders)) props.model._transportHeaders = []
  emit()
}
const realityTransportOk = computed(() => supportsReality(props.model.type, props.model._transportType))
</script>

<style scoped>
.hint { color: #909399; font-size: 12px; margin-top: 4px; }
.warn-hint { color: #e6a23c; }
</style>