<template>
  <div>
    <el-form label-width="140px" label-position="left">
      <el-form-item :label="t('protoCenter.geodata.enable')">
        <el-switch v-model="enabled" @change="onToggle" />
      </el-form-item>
    </el-form>

    <template v-if="enabled">
      <el-form label-width="140px" label-position="left">
        <el-form-item :label="t('protoCenter.geodata.cron')">
          <el-input v-model="form.cron" placeholder="0 4 * * *" @change="emit" />
          <div class="hint">{{ t('protoCenter.geodata.cronHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('protoCenter.geodata.outbound')">
          <el-select v-model="form.outbound" style="width:100%" clearable @change="emit">
            <el-option v-for="t in outboundTags" :key="t" :label="t" :value="t" />
          </el-select>
          <div class="hint">{{ t('protoCenter.geodata.outboundHint') }}</div>
        </el-form-item>
      </el-form>

      <el-divider content-position="left">{{ t('protoCenter.geodata.assets') }}</el-divider>
      <div v-for="(a, i) in form.assets" :key="i" style="display:flex;gap:8px;margin-bottom:8px">
        <el-input v-model="a.url" placeholder="https://github.com/.../geoip.dat" style="width:55%" @change="emit" />
        <el-input v-model="a.file" placeholder="geoip.dat" style="width:25%" @change="emit" />
        <el-button size="small" type="danger" text @click="rmAsset(i)">X</el-button>
      </div>
      <el-button size="small" type="primary" @click="addAsset">{{ t('protoCenter.geodata.addAsset') }}</el-button>
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
const form = reactive({ cron: '0 4 * * *', outbound: 'direct', assets: [
  { url: 'https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat', file: 'geoip.dat' },
  { url: 'https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat', file: 'geosite.dat' },
] })
const outboundTags = ref([])

const builtinOutboundTags = ['direct', 'blocked']
async function loadOutbounds() {
  try {
    const data = await client.get(`/node-servers/${props.serverId}/outbound-configs`, { params: { core_type: props.coreType || 'xray' } })
    const tags = (data || []).filter(o => o?.enabled !== false).map(o => o.tag).filter(Boolean)
    outboundTags.value = [...new Set([...builtinOutboundTags, ...tags])]
  } catch {}
}

function sync() {
  if (!props.data || Object.keys(props.data).length === 0) { enabled.value = false; return }
  if (props.data._disabled) { enabled.value = false; return }
  enabled.value = true
  form.cron = props.data.cron || '0 4 * * *'
  form.outbound = props.data.outbound || 'direct'
  form.assets = (props.data.assets || []).map(a => ({ ...a }))
}

function onToggle() {
  if (!enabled.value) emitFn('update', { ...form, _disabled: true })
  else emit()
}

function emit() { if (enabled.value) emitFn('update', JSON.parse(JSON.stringify(form))) }
function addAsset() { form.assets.push({ url: '', file: '' }); emit() }
function rmAsset(i) { form.assets.splice(i, 1); emit() }
onMounted(function() { sync(); loadOutbounds() })
</script>
<style scoped>.hint { font-size: 12px; color: var(--el-text-color-secondary); margin-top: 2px; }</style>
