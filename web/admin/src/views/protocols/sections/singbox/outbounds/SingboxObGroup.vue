<template>
  <el-form label-width="150px" label-position="left">
    <el-form-item :label="t('singbox.outboundGroup.tag')">
      <el-input v-model="model.tag" :placeholder="t('singbox.outboundGroup.tagPlaceholder')" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.outbounds')">
      <el-input v-model="model._outboundsStr" :placeholder="t('singbox.outboundGroup.outboundsPlaceholder')" @change="emit" />
    </el-form-item>
    <el-form-item v-if="model.type === 'selector'" :label="t('singbox.obProtocol.default')">
      <el-input v-model="model.default" :placeholder="t('singbox.outboundGroup.default')" @change="emit" />
    </el-form-item>
    <template v-if="model.type === 'urltest'">
      <el-form-item :label="t('singbox.obProtocol.url')">
        <el-input v-model="model.url" placeholder="https://www.gstatic.com/generate_204" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.obProtocol.interval')">
        <el-input v-model="model.interval" placeholder="3m" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.obProtocol.tolerance')">
        <el-input-number v-model="model.tolerance" :min="0" style="width:100%" @change="emit" />
      </el-form-item>
    </template>
    <el-form-item :label="t('singbox.obProtocol.interruptConns')">
      <el-switch v-model="model.interrupt_exist_connections" @change="emit" />
    </el-form-item>
  </el-form>
</template>

<script setup>
// selector / urltest 出站（组类型专属设置）
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>