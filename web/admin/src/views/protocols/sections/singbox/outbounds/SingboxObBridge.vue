<template>
  <el-form label-width="170px" label-position="left">
    <el-form-item :label="t('singbox.obBridge.interface')">
      <el-input v-model="model.interface" placeholder="br0" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obBridge.bridgeName')">
      <el-input v-model="model.bridge_name" placeholder="sing-box" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obBridge.iproute2TableIndex')">
      <el-input-number v-model="model.iproute2_table_index" :min="0" style="width:100%" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obBridge.iproute2RuleIndex')">
      <el-input-number v-model="model.iproute2_rule_index" :min="0" style="width:100%" @change="emit" />
    </el-form-item>
  </el-form>
</template>

<script setup>
// Bridge 出站：option/bridge.go BridgeOutboundOptions
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>
