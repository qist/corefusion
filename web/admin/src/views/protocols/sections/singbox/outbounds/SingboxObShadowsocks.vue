<template>
  <el-form label-width="150px" label-position="left">
    <el-form-item :label="t('singbox.obProtocol.method')">
      <el-select v-model="model.method" style="width:100%" filterable @change="emit">
        <el-option v-for="m in ssMethods" :key="m" :label="m" :value="m" />
      </el-select>
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.password')">
      <div style="display:flex;gap:8px;flex:1">
        <el-input v-model="model.password" @change="emit" />
        <el-button size="small" @click="model.password = is2022 ? generateKey() : generatePassword(); emit()">{{ t('singbox.users.generate') }}</el-button>
      </div>
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.plugin')">
      <el-input v-model="model.plugin" placeholder="obfs-local / v2ray-plugin" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.pluginOpts')">
      <el-input v-model="model.plugin_opts" :placeholder="t('singbox.obProtocol.pluginOptsHint')" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.udpOverTcp')">
      <el-switch v-model="model._udpOverTCP" @change="emit" />
    </el-form-item>
    <el-form-item v-if="model._udpOverTCP" :label="t('singbox.obProtocol.udpOverTcpVersion')">
      <el-select v-model="model._udpOverTCPVersion" style="width:100%" @change="emit">
        <el-option :label="t('singbox.obProtocol.uotDefault')" :value="0" />
        <el-option label="1" :value="1" />
        <el-option label="2" :value="2" />
      </el-select>
    </el-form-item>
  </el-form>
</template>

<script setup>
// Shadowsocks / Shadowsocks2022 出站：Method / Password / Plugin / UDP-over-TCP
// sing-box 源码 option/shadowsocks.go（没有 packet_encoding 字段）
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { generatePassword, generateKey } from '../utils/singboxDefaults'
const { t } = useI18n()
const props = defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
const is2022 = computed(() => String(props.model.method || '').startsWith('2022-') || props.model.type === 'shadowsocks2022')
const ssMethods = [
  'none',
  'aes-128-gcm', 'aes-192-gcm', 'aes-256-gcm',
  'chacha20-ietf-poly1305', 'xchacha20-ietf-poly1305',
  '2022-blake3-aes-128-gcm', '2022-blake3-aes-256-gcm', '2022-blake3-chacha20-poly1305',
  'aes-128-ctr', 'aes-192-ctr', 'aes-256-ctr',
  'aes-128-cfb', 'aes-192-cfb', 'aes-256-cfb',
  'rc4-md5', 'chacha20-ietf', 'xchacha20',
]
</script>
