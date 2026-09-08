<template>
  <el-form label-width="140px" label-position="left">
    <el-form-item :label="t('singbox.protocol.hysteria.up')">
      <el-input v-model="model.up" placeholder="100 Mbps" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.hysteria.down')">
      <el-input v-model="model.down" placeholder="100 Mbps" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.hysteria.obfs')">
      <el-select v-model="model.obfs" style="width:100%" clearable @change="emit">
        <el-option label="" value="" />
        <el-option label="salamander" value="salamander" />
        <el-option label="none" value="none" />
      </el-select>
    </el-form-item>
  </el-form>
</template>

<script setup>
// Hysteria 协议专属设置（通用 Listen/Users/TLS 由容器层提供）
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>