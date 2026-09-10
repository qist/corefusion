<template>
  <div>
    <el-form label-width="140px" label-position="left">
      <el-form-item :label="t('singbox.transport.transport')">
        <el-select v-model="model._transportType" style="width:100%" clearable @change="onChange" :placeholder="t('singbox.transport.transportPlaceholder')">
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

    <!-- ==================== WebSocket ==================== -->
    <!-- 官方参数: path, headers, max_early_data, early_data_header_name -->
    <template v-if="model._transportType === 'ws'">
      <el-form label-width="140px" label-position="left">
        <el-form-item :label="t('singbox.transport.path')">
          <div style="display:flex;gap:8px;width:100%">
            <el-input v-model="model._transportPath" :placeholder="t('singbox.transport.pathPlaceholder')" style="flex:1" @change="emit" />
            <el-button size="small" @click="genRandomPath">{{ t('singbox.transport.randomGen') }}</el-button>
          </div>
          <div class="hint">{{ t('singbox.transport.wsPathHint') }}</div>
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
          <div class="hint">{{ t('singbox.transport.maxEarlyDataHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('singbox.transport.earlyDataHeader')">
          <el-input v-model="model._transportEarlyDataHeader" placeholder="Sec-WebSocket-Protocol" @change="emit" />
          <div class="hint">{{ t('singbox.transport.earlyDataHeaderHint') }}</div>
        </el-form-item>
      </el-form>
    </template>

    <!-- ==================== gRPC ==================== -->
    <!-- 官方参数: service_name, idle_timeout, ping_timeout, permit_without_stream -->
    <template v-if="model._transportType === 'grpc'">
      <el-form label-width="140px" label-position="left">
        <el-form-item :label="t('singbox.transport.serviceName')">
          <div style="display:flex;gap:8px;width:100%">
            <el-input v-model="model._transportServiceName" style="flex:1" @change="emit" />
            <el-button size="small" @click="genRandomServiceName">{{ t('singbox.transport.randomGen') }}</el-button>
          </div>
          <div class="hint">{{ t('singbox.transport.grpcServiceNameHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('singbox.transport.idleTimeout')">
          <el-input v-model="model._transportIdleTimeout" placeholder="15s" @change="emit" />
          <div class="hint">{{ t('singbox.transport.idleTimeoutHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('singbox.transport.pingTimeout')">
          <el-input v-model="model._transportPingTimeout" placeholder="15s" @change="emit" />
          <div class="hint">{{ t('singbox.transport.pingTimeoutHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('singbox.transport.permitWithoutStream')">
          <el-switch v-model="model._transportPermitWithoutStream" @change="emit" />
          <div class="hint">{{ t('singbox.transport.permitWithoutStreamHint') }}</div>
        </el-form-item>
      </el-form>
    </template>

    <!-- ==================== HTTP ==================== -->
    <!-- 官方参数: host[], path, method, headers, idle_timeout, ping_timeout -->
    <template v-if="model._transportType === 'http'">
      <el-form label-width="140px" label-position="left">
        <el-form-item :label="t('singbox.transport.path')">
          <div style="display:flex;gap:8px;width:100%">
            <el-input v-model="model._transportPath" :placeholder="t('singbox.transport.pathPlaceholder')" style="flex:1" @change="emit" />
            <el-button size="small" @click="genRandomPath">{{ t('singbox.transport.randomGen') }}</el-button>
          </div>
          <div class="hint">{{ t('singbox.transport.httpPathHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('singbox.transport.host')">
          <el-input v-model="model._transportHost" :placeholder="t('singbox.transport.httpHostPlaceholder')" @change="emit" />
          <div class="hint">{{ t('singbox.transport.httpHostHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('singbox.transport.method')">
          <el-select v-model="model._transportMethod" style="width:100%" clearable @change="emit">
            <el-option label="GET" value="GET" />
            <el-option label="POST" value="POST" />
            <el-option label="PUT" value="PUT" />
            <el-option label="DELETE" value="DELETE" />
            <el-option label="HEAD" value="HEAD" />
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
          <div class="hint">{{ t('singbox.transport.idleTimeoutHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('singbox.transport.pingTimeout')">
          <el-input v-model="model._transportPingTimeout" placeholder="15s" @change="emit" />
          <div class="hint">{{ t('singbox.transport.pingTimeoutHint') }}</div>
        </el-form-item>
      </el-form>
    </template>

    <!-- ==================== HTTPUpgrade ==================== -->
    <!-- 官方参数: host, path, headers -->
    <template v-if="model._transportType === 'httpupgrade'">
      <el-form label-width="140px" label-position="left">
        <el-form-item :label="t('singbox.transport.path')">
          <div style="display:flex;gap:8px;width:100%">
            <el-input v-model="model._transportPath" :placeholder="t('singbox.transport.pathPlaceholder')" style="flex:1" @change="emit" />
            <el-button size="small" @click="genRandomPath">{{ t('singbox.transport.randomGen') }}</el-button>
          </div>
        </el-form-item>
        <el-form-item :label="t('singbox.transport.host')">
          <el-input v-model="model._transportHost" :placeholder="t('singbox.transport.httpHostPlaceholder')" @change="emit" />
          <div class="hint">{{ t('singbox.transport.httpupgradeHostHint') }}</div>
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

    <!-- ==================== QUIC ==================== -->
    <!-- 官方文档：QUIC 传输无额外参数 -->
    <template v-if="model._transportType === 'quic'">
      <el-alert :title="t('singbox.transport.quicNoParams')" type="info" :closable="false" />
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

// 生成随机 ID（小写字母+数字，参考 xray genId）
function genId(len = 12) {
  const chars = 'abcdefghijklmnopqrstuvwxyz0123456789'
  let r = ''
  for (let i = 0; i < len; i++) r += chars.charAt(Math.floor(Math.random() * chars.length))
  return r
}

// 随机生成 WS/HTTP/HTTPUpgrade 路径（带 early data 参数，参考 xray）
function genRandomPath() {
  props.model._transportPath = '/' + genId(12) + '?ed=2560'
  emit()
}

// 随机生成 gRPC service_name（参考 xray）
function genRandomServiceName() {
  props.model._transportServiceName = genId(12)
  emit()
}

// 传输类型切换时设置合理默认值
function onChange() {
  const tp = props.model._transportType
  if (tp === 'ws') {
    if (!props.model._transportPath || props.model._transportPath === '/') {
      props.model._transportPath = '/' + genId(12) + '?ed=2560'
    }
    if (!props.model._transportMaxEarlyData) props.model._transportMaxEarlyData = 0
  }
  if (tp === 'grpc') {
    if (!props.model._transportServiceName) props.model._transportServiceName = genId(12)
  }
  if (tp === 'http' || tp === 'httpupgrade') {
    if (!props.model._transportPath || props.model._transportPath === '/') {
      props.model._transportPath = '/' + genId(12)
    }
  }
  if (!Array.isArray(props.model._transportHeaders)) props.model._transportHeaders = []
  emit()
}

const realityTransportOk = computed(() => supportsReality(props.model.type, props.model._transportType))
</script>

<style scoped>
.hint { color: #909399; font-size: 12px; margin-top: 4px; }
.warn-hint { color: #e6a23c; }
</style>
