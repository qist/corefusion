<template>
  <div>
    <el-form label-width="180px" label-position="left">
      <el-divider content-position="left">{{ t('protoCenter.policy.level0') }}</el-divider>
      <el-form-item :label="t('protoCenter.policy.handshake')">
        <div style="display:flex;align-items:center;gap:8px">
          <el-switch v-model="level0._handshakeEnabled" @change="onToggle('handshake')" />
          <el-input-number v-model="level0.handshake" :min="0" :disabled="!level0._handshakeEnabled" @change="emit" />
        </div>
      </el-form-item>
      <el-form-item :label="t('protoCenter.policy.connIdle')">
        <div style="display:flex;align-items:center;gap:8px">
          <el-switch v-model="level0._connIdleEnabled" @change="onToggle('connIdle')" />
          <el-input-number v-model="level0.connIdle" :min="0" :disabled="!level0._connIdleEnabled" @change="emit" />
        </div>
      </el-form-item>
      <el-form-item :label="t('protoCenter.policy.uplinkOnly')">
        <div style="display:flex;align-items:center;gap:8px">
          <el-switch v-model="level0._uplinkOnlyEnabled" @change="onToggle('uplinkOnly')" />
          <el-input-number v-model="level0.uplinkOnly" :min="0" :disabled="!level0._uplinkOnlyEnabled" @change="emit" />
        </div>
      </el-form-item>
      <el-form-item :label="t('protoCenter.policy.downlinkOnly')">
        <div style="display:flex;align-items:center;gap:8px">
          <el-switch v-model="level0._downlinkOnlyEnabled" @change="onToggle('downlinkOnly')" />
          <el-input-number v-model="level0.downlinkOnly" :min="0" :disabled="!level0._downlinkOnlyEnabled" @change="emit" />
        </div>
      </el-form-item>
      <el-form-item :label="t('protoCenter.policy.statsUserUplink')">
        <el-switch v-model="level0.statsUserUplink" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('protoCenter.policy.statsUserDownlink')">
        <el-switch v-model="level0.statsUserDownlink" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('protoCenter.policy.bufferSize')">
        <div style="display:flex;align-items:center;gap:8px">
          <el-switch v-model="level0._bufferSizeEnabled" @change="onToggle('bufferSize')" />
          <el-input-number v-model="level0.bufferSize" :min="0" :disabled="!level0._bufferSizeEnabled" @change="emit" />
        </div>
      </el-form-item>

      <el-divider content-position="left">{{ t('protoCenter.policy.system') }}</el-divider>
      <el-form-item :label="t('protoCenter.policy.statsInboundUplink')">
        <el-switch v-model="form.system.statsInboundUplink" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('protoCenter.policy.statsInboundDownlink')">
        <el-switch v-model="form.system.statsInboundDownlink" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('protoCenter.policy.statsOutboundUplink')">
        <el-switch v-model="form.system.statsOutboundUplink" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('protoCenter.policy.statsOutboundDownlink')">
        <el-switch v-model="form.system.statsOutboundDownlink" @change="emit" />
      </el-form-item>
    </el-form>
  </div>
</template>
<script setup>
import { reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
const props = defineProps({ data: Object })
const emitFn = defineEmits(['update'])
const form = reactive({
  levels: { '0': {
    handshake: 4, connIdle: 300, uplinkOnly: 2, downlinkOnly: 5,
    statsUserUplink: true, statsUserDownlink: true, bufferSize: 10240,
    _handshakeEnabled: false, _connIdleEnabled: false, _uplinkOnlyEnabled: false,
    _downlinkOnlyEnabled: false, _bufferSizeEnabled: false,
  } },
  system: { statsInboundUplink: true, statsInboundDownlink: true, statsOutboundUplink: true, statsOutboundDownlink: true },
})
const level0 = computed(() => form.levels['0'])

function onToggle(field) { emit() }

function sync() {
  if (!props.data) return
  if (props.data.levels) {
    const l = props.data.levels
    Object.keys(l).forEach(k => {
      if (!form.levels[k]) form.levels[k] = {}
      Object.assign(form.levels[k], l[k])
      form.levels[k]._handshakeEnabled = l[k].handshake !== undefined
      form.levels[k]._connIdleEnabled = l[k].connIdle !== undefined
      form.levels[k]._uplinkOnlyEnabled = l[k].uplinkOnly !== undefined
      form.levels[k]._downlinkOnlyEnabled = l[k].downlinkOnly !== undefined
      form.levels[k]._bufferSizeEnabled = l[k].bufferSize !== undefined
    })
  }
  if (props.data.system) Object.assign(form.system, props.data.system)
}

function build() {
  const result = { levels: {}, system: { ...form.system } }
  Object.keys(form.levels).forEach(k => {
    const l = form.levels[k]
    const level = {}
    if (l._handshakeEnabled) level.handshake = l.handshake
    if (l._connIdleEnabled) level.connIdle = l.connIdle
    if (l._uplinkOnlyEnabled) level.uplinkOnly = l.uplinkOnly
    if (l._downlinkOnlyEnabled) level.downlinkOnly = l.downlinkOnly
    if (l.statsUserUplink) level.statsUserUplink = l.statsUserUplink
    if (l.statsUserDownlink) level.statsUserDownlink = l.statsUserDownlink
    if (l._bufferSizeEnabled) level.bufferSize = l.bufferSize
    result.levels[k] = level
  })
  return result
}

function emit() { emitFn('update', build()) }
onMounted(sync)
</script>
