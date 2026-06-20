<template>
  <el-form label-width="140px" label-position="left">
    <el-form-item :label="t('protoCenter.api.tag')">
      <el-input v-model="form.tag" placeholder="api" @change="emit" />
      <div class="hint">{{ t('protoCenter.api.tagHint') }}</div>
    </el-form-item>
    <el-form-item :label="t('protoCenter.api.listen')">
      <el-select v-model="form.listen" style="width:100%" clearable filterable allow-create @change="emit">
        <el-option :label="t('protoCenter.api.listenDisabled')" value="" />
        <el-option label="127.0.0.1:62789" value="127.0.0.1:62789" />
        <el-option label="127.0.0.1:10085" value="127.0.0.1:10085" />
      </el-select>
      <div class="hint">{{ t('protoCenter.api.listenHint') }}</div>
      <div class="hint">{{ t('protoCenter.api.listenDesc') }}</div>
    </el-form-item>
    <el-form-item :label="t('protoCenter.api.services')">
      <el-checkbox-group v-model="form.services" @change="emit">
        <el-checkbox value="HandlerService" label="HandlerService" />
        <el-checkbox value="LoggerService" label="LoggerService" />
        <el-checkbox value="StatsService" label="StatsService" />
        <el-checkbox value="ObservatoryService" label="ObservatoryService" />
        <el-checkbox value="RoutingService" label="RoutingService" />
        <el-checkbox value="BalancerService" label="BalancerService" />
      </el-checkbox-group>
      <div class="hint">{{ t('protoCenter.api.servicesHint') }}</div>
    </el-form-item>
  </el-form>
</template>
<script setup>
import { reactive, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
const props = defineProps({ data: Object })
const emitFn = defineEmits(['update'])
const form = reactive({ tag: 'api', listen: '', services: ['HandlerService', 'LoggerService', 'StatsService'] })
function sync() {
  if (!props.data) return
  form.tag = props.data.tag || 'api'
  form.listen = props.data.listen || ''
  form.services = props.data.services || ['HandlerService', 'LoggerService', 'StatsService']
}
function emit() {
  const out = { tag: form.tag, services: form.services }
  if (form.listen) out.listen = form.listen
  emitFn('update', out)
}
onMounted(sync)
</script>
<style scoped>.hint { font-size: 12px; color: var(--el-text-color-secondary); margin-top: 2px; }</style>
