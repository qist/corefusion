<template>
  <el-form label-width="140px" label-position="left">
    <el-form-item :label="t('singbox.protocol.shadowsocks.method')">
      <el-select v-model="model.method" style="width:100%" filterable @change="emit">
        <el-option v-for="m in methodOptions" :key="m" :label="m" :value="m" />
      </el-select>
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.shadowsocks.network')">
      <el-select v-model="model.network" style="width:100%" clearable @change="emit">
        <el-option label="" value="" />
        <el-option label="tcp" value="tcp" />
        <el-option label="udp" value="udp" />
        <el-option label="tcp,udp" value="tcp,udp" />
      </el-select>
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.shadowsocks.managed')">
      <el-switch v-model="model.managed" @change="onManagedChange" />
      <div class="hint">{{ t('singbox.protocol.shadowsocks.managedHint') }}</div>
    </el-form-item>
    <el-divider content-position="left">{{ t('singbox.protocol.shadowsocks.destinations') }}</el-divider>
    <div class="hint" style="margin-bottom:6px">{{ t('singbox.protocol.shadowsocks.managedHint') }}</div>
    <el-form-item v-if="hasDestinations" :label="t('singbox.users.password')">
      <el-input v-model="model.password" placeholder="2022-blake3-... PSK" show-password @change="emit" />
    </el-form-item>
    <div v-for="(d, i) in (model.destinations || [])" :key="i" class="dest-row">
      <el-input v-model="d.name" :placeholder="t('singbox.protocol.shadowsocks.destinationName')" @change="emit" />
      <el-input v-model="d.server" :placeholder="t('singbox.protocol.shadowsocks.destinationServer')" @change="emit" />
      <el-input-number v-model="d.server_port" :min="1" :max="65535" controls-position="right" style="width:130px" @change="emit" />
      <el-input v-model="d.password" :placeholder="t('singbox.protocol.shadowsocks.destinationPassword')" show-password @change="emit" />
      <el-button size="small" type="danger" @click="removeDestination(i)">X</el-button>
    </div>
    <div v-if="!(model.destinations || []).length" class="hint">{{ t('singbox.protocol.shadowsocks.noDestinations') }}</div>
    <el-button size="small" style="margin-top:6px" @click="addDestination">{{ t('singbox.protocol.shadowsocks.addDestination') }}</el-button>
  </el-form>
</template>

<script setup>
// Shadowsocks 协议专属设置（通用 Listen/Users/Transport/TLS/Multiplex 由容器层提供）
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { generateKey } from '../utils/singboxDefaults'
const { t } = useI18n()
const props = defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
const ssMethods = ['2022-blake3-aes-128-gcm', '2022-blake3-aes-256-gcm', '2022-blake3-chacha20-poly1305', 'aes-256-gcm', 'aes-128-gcm', 'chacha20-ietf-poly1305', 'none']
const ss2022Methods = ssMethods.filter(m => m.startsWith('2022-'))
const hasDestinations = computed(() => (props.model.destinations || []).length > 0)
// relay 模式（destinations）只支持 2022-blake3-* 方法（源码 inbound_relay.go）
const methodOptions = computed(() => hasDestinations.value ? ss2022Methods : ssMethods)

// users / destinations / managed 三者互斥（sing-box 会直接拒载）
function onManagedChange(v) {
  if (v) props.model.destinations = []
  emit()
}
function addDestination() {
  props.model.managed = false
  if (!Array.isArray(props.model.destinations)) props.model.destinations = []
  // 目标服务器的 password 必须是合法 PSK（2022-blake3-* 要求 base64 32 字节）
  props.model.destinations.push({ name: '', server: '', server_port: 0, password: generateKey() })
  // relay 模式必须用 2022-blake3-* 方法，否则核心拒载
  if (!String(props.model.method || '').startsWith('2022-')) props.model.method = '2022-blake3-aes-256-gcm'
  emit()
}
function removeDestination(i) {
  if (!Array.isArray(props.model.destinations)) return
  props.model.destinations.splice(i, 1)
  emit()
}
</script>

<style scoped>
.hint { color: #909399; font-size: 12px; margin-top: 4px; }
.dest-row { display: flex; gap: 8px; margin-bottom: 8px; align-items: center; }
</style>
