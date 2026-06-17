<template>
  <div>
    <el-form label-width="140px" label-position="left">
      <el-form-item :label="t('protoCenter.metrics.enable')">
        <el-switch v-model="enabled" @change="onToggle" />
      </el-form-item>
    </el-form>

    <template v-if="enabled">
      <el-form label-width="140px" label-position="left">
        <el-form-item :label="t('protoCenter.metrics.tag')">
          <el-input v-model="form.tag" placeholder="metrics_out" @change="emit" />
          <div class="hint">{{ t('protoCenter.metrics.tagHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('protoCenter.metrics.listen')">
          <el-input v-model="form.listen" placeholder="127.0.0.1:11111" @change="emit" />
          <div class="hint">{{ t('protoCenter.metrics.listenHint') }}</div>
        </el-form-item>
      </el-form>
    </template>
  </div>
</template>
<script setup>
import { reactive, ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
const props = defineProps({ data: Object })
const emitFn = defineEmits(['update'])
const enabled = ref(false)
const form = reactive({ tag: 'metrics_out', listen: '127.0.0.1:11111' })

function sync() {
  if (!props.data || Object.keys(props.data).length === 0) { enabled.value = false; return }
  if (props.data._disabled) { enabled.value = false; return }
  if (!props.data.tag && !props.data.listen) { enabled.value = false; return }
  enabled.value = true
  form.tag = props.data.tag || 'metrics_out'
  form.listen = props.data.listen || '127.0.0.1:11111'
}

function onToggle() {
  if (!enabled.value) {
    const data = { ...form, _disabled: true }
    emitFn('update', data)
  } else {
    emit()
  }
}

function emit() { if (enabled.value) emitFn('update', { ...form }) }
onMounted(sync)
</script>
<style scoped>.hint { font-size: 12px; color: var(--el-text-color-secondary); margin-top: 2px; }</style>
