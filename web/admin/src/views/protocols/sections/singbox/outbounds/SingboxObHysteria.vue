<template>
  <el-form label-width="150px" label-position="left">
    <el-form-item :label="t('singbox.obProtocol.upMbps')">
      <el-input v-model="model.up" placeholder="100 Mbps" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.downMbps')">
      <el-input v-model="model.down" placeholder="100 Mbps" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.obfs')">
      <el-input v-model="model.obfs" placeholder="salamander" @change="emit" />
    </el-form-item>
  </el-form>
</template>

<script setup>
// Hysteria 出站：专属设置(Up/Down/Obfs)；Base/TLS 由容器层提供
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>
