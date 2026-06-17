<template>
  <div>
    <el-form label-width="140px" label-position="left">
      <el-form-item :label="t('protoCenter.fakedns.enable')">
        <el-switch v-model="enabled" @change="onToggle" />
      </el-form-item>
    </el-form>

    <template v-if="enabled">
      <!-- IPv4 Pool -->
      <el-divider content-position="left">IPv4 Pool</el-divider>
      <el-form label-width="140px" label-position="left">
        <el-form-item label="Enable IPv4">
          <el-switch v-model="ipv4Enabled" @change="emit" />
        </el-form-item>
        <template v-if="ipv4Enabled">
          <el-form-item :label="t('protoCenter.fakedns.ipPool')">
            <el-input v-model="ipv4.ipPool" placeholder="198.18.0.0/15" @change="emit" />
            <div class="hint">{{ t('protoCenter.fakedns.ipPoolHint') }}</div>
          </el-form-item>
          <el-form-item :label="t('protoCenter.fakedns.poolSize')">
            <el-input-number v-model="ipv4.poolSize" :min="0" @change="emit" />
            <div class="hint">{{ t('protoCenter.fakedns.poolSizeHint') }}</div>
          </el-form-item>
        </template>
      </el-form>

      <!-- IPv6 Pool -->
      <el-divider content-position="left">IPv6 Pool</el-divider>
      <el-form label-width="140px" label-position="left">
        <el-form-item label="Enable IPv6">
          <el-switch v-model="ipv6Enabled" @change="emit" />
        </el-form-item>
        <template v-if="ipv6Enabled">
          <el-form-item :label="t('protoCenter.fakedns.ipPool')">
            <el-input v-model="ipv6.ipPool" placeholder="fc00::/18" @change="emit" />
            <div class="hint">{{ t('protoCenter.fakedns.ipPoolHint') }}</div>
          </el-form-item>
          <el-form-item :label="t('protoCenter.fakedns.poolSize')">
            <el-input-number v-model="ipv6.poolSize" :min="0" @change="emit" />
            <div class="hint">{{ t('protoCenter.fakedns.poolSizeHint') }}</div>
          </el-form-item>
        </template>
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
const ipv4Enabled = ref(true)
const ipv6Enabled = ref(false)
const ipv4 = reactive({ ipPool: '198.18.0.0/15', poolSize: 65535 })
const ipv6 = reactive({ ipPool: 'fc00::/18', poolSize: 65535 })

function sync() {
  if (!props.data || Object.keys(props.data).length === 0) { enabled.value = false; return }
  if (props.data._disabled) { enabled.value = false; return }
  enabled.value = true

  // Handle array format (v4+v6 pools)
  if (Array.isArray(props.data)) {
    const v4 = props.data.find(p => p.ipPool && p.ipPool.includes('.'))
    const v6 = props.data.find(p => p.ipPool && p.ipPool.includes(':'))
    if (v4) { ipv4Enabled.value = true; ipv4.ipPool = v4.ipPool; ipv4.poolSize = v4.poolSize || 65535 }
    else { ipv4Enabled.value = false }
    if (v6) { ipv6Enabled.value = true; ipv6.ipPool = v6.ipPool; ipv6.poolSize = v6.poolSize || 65535 }
    else { ipv6Enabled.value = false }
  } else {
    // Single object format
    if (props.data.ipPool) {
      if (props.data.ipPool.includes(':')) {
        ipv6Enabled.value = true; ipv6.ipPool = props.data.ipPool; ipv6.poolSize = props.data.poolSize || 65535
        ipv4Enabled.value = false
      } else {
        ipv4Enabled.value = true; ipv4.ipPool = props.data.ipPool; ipv4.poolSize = props.data.poolSize || 65535
        ipv6Enabled.value = false
      }
    }
  }
}

function build() {
  const pools = []
  if (ipv4Enabled.value && ipv4.ipPool) pools.push({ ipPool: ipv4.ipPool, poolSize: ipv4.poolSize })
  if (ipv6Enabled.value && ipv6.ipPool) pools.push({ ipPool: ipv6.ipPool, poolSize: ipv6.poolSize })
  if (pools.length === 0) return {}
  if (pools.length === 1) return pools[0]
  return pools
}

function onToggle() {
  if (!enabled.value) {
    emitFn('update', { _disabled: true })
  } else {
    emit()
  }
}

function emit() { if (enabled.value) emitFn('update', build()) }
onMounted(sync)
</script>
<style scoped>.hint { font-size: 12px; color: var(--el-text-color-secondary); margin-top: 2px; }</style>
