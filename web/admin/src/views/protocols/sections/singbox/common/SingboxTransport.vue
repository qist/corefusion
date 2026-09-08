<template>
  <div>
    <el-form label-width="140px" label-position="left">
      <el-form-item label="Transport">
        <el-select v-model="model._transportType" style="width:100%" clearable @change="onChange">
          <el-option label="none" value="" />
          <el-option label="ws" value="ws" />
          <el-option label="grpc" value="grpc" />
          <el-option label="http" value="http" />
          <el-option label="httpupgrade" value="httpupgrade" />
          <el-option label="kcp" value="kcp" />
          <el-option label="quic" value="quic" />
        </el-select>
      </el-form-item>
    </el-form>

    <!-- ws -->
    <template v-if="model._transportType === 'ws'">
      <el-form label-width="140px" label-position="left">
        <el-form-item label="Path">
          <el-input v-model="model._transportPath" placeholder="/" @change="emit" />
        </el-form-item>
        <el-form-item label="Host">
          <el-input v-model="model._transportHost" placeholder="example.com" @change="emit" />
        </el-form-item>
        <el-form-item label="Max Early Data">
          <el-input-number v-model="model._transportMaxEarlyData" :min="0" style="width:100%" @change="emit" />
        </el-form-item>
        <el-form-item label="Early Data Header">
          <el-input v-model="model._transportEarlyDataHeader" placeholder="Sec-WebSocket-Protocol" @change="emit" />
        </el-form-item>
      </el-form>
    </template>

    <!-- grpc -->
    <template v-if="model._transportType === 'grpc'">
      <el-form label-width="140px" label-position="left">
        <el-form-item label="Service Name">
          <el-input v-model="model._transportServiceName" @change="emit" />
        </el-form-item>
        <el-form-item label="Idle Timeout">
          <el-input v-model="model._transportIdleTimeout" placeholder="15s" @change="emit" />
        </el-form-item>
        <el-form-item label="Ping Timeout">
          <el-input v-model="model._transportPingTimeout" placeholder="15s" @change="emit" />
        </el-form-item>
      </el-form>
    </template>

    <!-- http / httpupgrade -->
    <template v-if="model._transportType === 'http' || model._transportType === 'httpupgrade'">
      <el-form label-width="140px" label-position="left">
        <el-form-item label="Host">
          <el-input v-model="model._transportHost" @change="emit" />
        </el-form-item>
        <el-form-item label="Path">
          <el-input v-model="model._transportPath" placeholder="/" @change="emit" />
        </el-form-item>
      </el-form>
    </template>

    <!-- kcp -->
    <template v-if="model._transportType === 'kcp'">
      <el-form label-width="140px" label-position="left">
        <el-form-item label="Seed">
          <el-input v-model="model._transportSeed" @change="emit" />
        </el-form-item>
      </el-form>
    </template>
  </div>
</template>

<script setup>
// 通用 V2Ray Transport 组件（ws/grpc/http/httpupgrade/kcp/quic）
// model: 协议 form 响应式对象
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
function onChange() {
  if (model._transportType === 'ws' && !model._transportPath) model._transportPath = '/'
  if ((model._transportType === 'http' || model._transportType === 'httpupgrade') && !model._transportPath) model._transportPath = '/'
  emit()
}
</script>