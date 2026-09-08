<template>
  <el-form label-width="140px" label-position="left">
    <el-form-item :label="t('singbox.multiplex.multiplex')">
      <el-switch v-model="model._multiplexEnabled" @change="emit" />
    </el-form-item>
    <template v-if="model._multiplexEnabled">
      <el-form-item :label="t('singbox.multiplex.protocol')">
        <el-select v-model="model._multiplexProtocol" style="width:100%" @change="emit">
          <el-option label="smux" value="smux" />
          <el-option label="yamux" value="yamux" />
          <el-option label="h2mux" value="h2mux" />
        </el-select>
      </el-form-item>
      <el-form-item :label="t('singbox.multiplex.maxConnections')">
        <el-input-number v-model="model._multiplexMaxConn" :min="0" style="width:100%" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.multiplex.minStreams')">
        <el-input-number v-model="model._multiplexMinStreams" :min="0" style="width:100%" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.multiplex.maxStreams')">
        <el-input-number v-model="model._multiplexMaxStreams" :min="0" style="width:100%" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.multiplex.padding')">
        <el-switch v-model="model._multiplexPadding" @change="emit" />
      </el-form-item>
    </template>
  </el-form>
</template>

<script setup>
// 通用 Multiplex（smux/yamux/h2mux）组件
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>