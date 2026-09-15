<template>
  <el-form label-width="150px" label-position="left">
    <el-form-item :label="t('singbox.obTor.executablePath')">
      <el-input v-model="model.executable_path" placeholder="tor" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obTor.extraArgs')">
      <el-input v-model="model._torExtraArgs" :placeholder="t('singbox.obTor.extraArgsPlaceholder')" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obTor.dataDirectory')">
      <el-input v-model="model.data_directory" placeholder="/var/lib/tor" @change="emit" />
    </el-form-item>
  </el-form>
</template>

<script setup>
// Tor 出站：专属设置(ExecutablePath/ExtraArgs/DataDirectory)；Base 由容器层提供
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>
