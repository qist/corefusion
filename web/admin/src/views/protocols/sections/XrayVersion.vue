<template>
  <div>
    <el-form label-width="140px" label-position="left">
      <el-form-item :label="t('protoCenter.version.enable')">
        <el-switch v-model="enabled" @change="onToggle" />
      </el-form-item>
    </el-form>

    <template v-if="enabled">
      <el-form label-width="140px" label-position="left">
        <el-form-item :label="t('protoCenter.version.min')">
          <el-input v-model="form.min" placeholder="25.8.3" @change="emit" />
          <div class="hint">{{ t('protoCenter.version.minHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('protoCenter.version.max')">
          <el-input v-model="form.max" placeholder="26.0.0" @change="emit" />
          <div class="hint">{{ t('protoCenter.version.maxHint') }}</div>
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
const form = reactive({ min: '', max: '' })

function sync() {
  if (!props.data || Object.keys(props.data).length === 0) { enabled.value = false; return }
  if (props.data._disabled) { enabled.value = false; return }
  if (!props.data.min && !props.data.max) { enabled.value = false; return }
  enabled.value = true
  form.min = props.data.min || ''
  form.max = props.data.max || ''
}

function onToggle() {
  if (!enabled.value) emitFn('update', { ...form, _disabled: true })
  else emit()
}

function emit() { if (enabled.value) emitFn('update', { ...form }) }
onMounted(sync)
</script>
<style scoped>.hint { font-size: 12px; color: var(--el-text-color-secondary); margin-top: 2px; }</style>
