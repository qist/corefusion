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
      </el-form>

      <el-divider content-position="left">Ping Config</el-divider>
      <el-form label-width="160px" label-position="left">
        <el-form-item :label="t('protoCenter.observatory.destination')">
          <el-input v-model="form.pingConfig.destination" placeholder="http://www.google.com/generate_204" @change="emit" />
        </el-form-item>
        <el-form-item :label="t('protoCenter.observatory.connectivity')">
          <el-input v-model="form.pingConfig.connectivity" @change="emit" />
        </el-form-item>
        <el-form-item :label="t('protoCenter.observatory.interval')">
          <el-input v-model="form.pingConfig.interval" placeholder="120s" @change="emit" />
        </el-form-item>
        <el-form-item :label="t('protoCenter.observatory.sampling')">
          <el-input-number v-model="form.pingConfig.sampling" :min="1" @change="emit" />
        </el-form-item>
        <el-form-item :label="t('protoCenter.observatory.timeout')">
          <el-input v-model="form.pingConfig.timeout" placeholder="16s" @change="emit" />
        </el-form-item>
        <el-form-item :label="t('protoCenter.observatory.httpMethod')">
          <el-select v-model="form.pingConfig.httpMethod" style="width:100%" @change="emit">
            <el-option label="HEAD" value="HEAD" /><el-option label="GET" value="GET" />
          </el-select>
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
const props = defineProps({ data: Object, coreType: String, serverId: Number })
const emitFn = defineEmits(['update'])
const enabled = ref(false)
const form = reactive({
  subjectSelector: [],
  pingConfig: { destination: 'http://www.google.com/generate_204', connectivity: '', interval: '120s', sampling: 5, timeout: '16s', httpMethod: 'HEAD' },
})
const outboundTags = ref([])

async function loadOutbounds() {
  try {
    const data = await client.get(`/node-servers/${props.serverId}/protocols/${props.coreType}/section/outbounds`)
    if (Array.isArray(data)) outboundTags.value = data.map(o => o.tag).filter(Boolean)
  } catch {}
}

function sync() {
  if (!props.data || Object.keys(props.data).length === 0) { enabled.value = false; return }
  if (props.data._disabled) { enabled.value = false; return }
  enabled.value = true
  form.subjectSelector = props.data.subjectSelector || []
  if (props.data.pingConfig) Object.assign(form.pingConfig, props.data.pingConfig)
}

function onToggle() {
  if (!enabled.value) emitFn('update', { ...form, _disabled: true })
  else emit()
}

function emit() { if (enabled.value) emitFn('update', JSON.parse(JSON.stringify(form))) }
onMounted(function() { sync(); loadOutbounds() })
</script>
<style scoped>.hint { font-size: 12px; color: var(--el-text-color-secondary); margin-top: 2px; }</style>
