<template>
  <div>
    <el-form label-width="160px" label-position="left">
      <el-form-item :label="t('protoCenter.observatory.enable')">
        <el-switch v-model="enabled" @change="onToggle" />
      </el-form-item>
    </el-form>

    <template v-if="enabled">
      <el-form label-width="160px" label-position="left">
        <el-form-item :label="t('protoCenter.observatory.subjectSelector')">
          <el-select v-model="form.subjectSelector" multiple filterable allow-create default-first-option :placeholder="t('protoCenter.observatory.selectOrType')" style="width:100%" @change="emit">
            <el-option v-for="t in outboundTags" :key="t" :label="t" :value="t" />
          </el-select>
          <div class="hint">{{ t('protoCenter.observatory.subjectSelectorHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('protoCenter.observatory.probeURL')">
          <el-input v-model="form.probeURL" placeholder="http://www.google.com/generate_204" @change="emit" />
        </el-form-item>
        <el-form-item :label="t('protoCenter.observatory.probeInterval')">
          <el-input v-model="form.probeInterval" placeholder="120s" @change="emit" />
        </el-form-item>
        <el-form-item :label="t('protoCenter.observatory.enableConcurrency')">
          <el-switch v-model="form.enableConcurrency" @change="emit" />
          <div class="hint">{{ t('protoCenter.observatory.enableConcurrencyHint') }}</div>
        </el-form-item>
      </el-form>
    </template>
  </div>
</template>
<script setup>
import { reactive, ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import client from '../../../api/client'
const { t } = useI18n()
const props = defineProps({ data: Object, coreType: String })
const emitFn = defineEmits(['update'])
const enabled = ref(false)
const form = reactive({ subjectSelector: [], probeURL: 'http://www.google.com/generate_204', probeInterval: '120s', enableConcurrency: false })
const outboundTags = ref([])

async function loadOutbounds() {
  try {
    const data = await client.get(`/protocols/${props.coreType}/section/outbounds`)
    if (Array.isArray(data)) outboundTags.value = data.map(o => o.tag).filter(Boolean)
  } catch {}
}

function sync() {
  if (!props.data || Object.keys(props.data).length === 0) { enabled.value = false; return }
  if (props.data._disabled) { enabled.value = false; return }
  enabled.value = true
  form.subjectSelector = props.data.subjectSelector || []
  form.probeURL = props.data.probeURL || 'http://www.google.com/generate_204'
  form.probeInterval = props.data.probeInterval || '120s'
  form.enableConcurrency = props.data.enableConcurrency || false
}

function onToggle() {
  if (!enabled.value) emitFn('update', { ...form, _disabled: true })
  else emit()
}

function emit() { if (enabled.value) emitFn('update', { ...form }) }
onMounted(function() { sync(); loadOutbounds() })
</script>
<style scoped>.hint { font-size: 12px; color: var(--el-text-color-secondary); margin-top: 2px; }</style>
