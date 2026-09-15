<template>
  <el-form label-width="140px" label-position="left">
    <el-form-item :label="t('singbox.protocol.wireguard.privateKey')">
      <div style="display:flex;gap:8px;flex:1">
        <el-input v-model="model._wg_private_key" @change="onPrivateKeyChange" />
        <el-button size="small" @click="genWgKeyPair">{{ t('singbox.users.generate') }}</el-button>
      </div>
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.wireguard.publicKey')">
      <el-input v-model="model._wg_public_key" readonly style="flex:1" />
      <div class="hint">{{ t('singbox.protocol.wireguard.publicKeyHint') }}</div>
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.wireguard.mtu')">
      <el-input-number v-model="model._wg_mtu" :min="0" @change="emit" />
      <div class="hint">{{ t('singbox.protocol.wireguard.mtuHint') }}</div>
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.wireguard.address')">
      <el-input v-model="model._wg_address" :placeholder="t('singbox.protocol.wireguard.addressPlaceholder')" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.wireguard.workers')">
      <el-input-number v-model="model._wg_workers" :min="0" @change="emit" />
      <div class="hint">{{ t('singbox.protocol.wireguard.workersHint') }}</div>
    </el-form-item>
    <el-divider content-position="left">{{ t('singbox.protocol.wireguard.peers') }}</el-divider>
    <div v-for="(p, pi) in model._wg_peers" :key="pi" class="user-item">
      <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:8px">
        <span style="font-weight:600">Peer #{{ pi + 1 }}</span>
        <el-button size="small" type="danger" text @click="model._wg_peers.splice(pi,1);emit">{{ t('common.delete') || 'X' }}</el-button>
      </div>
      <el-form label-width="120px" label-position="left" size="small">
        <el-form-item :label="t('singbox.protocol.wireguard.peerPublicKey')">
          <el-input v-model="p.public_key" @change="emit" />
        </el-form-item>
        <el-form-item :label="t('singbox.protocol.wireguard.allowedIPs')">
          <el-input v-model="p._allowedIPsStr" :placeholder="'0.0.0.0/0, ::/0'" @change="onAllowedIPsChange(p)" />
        </el-form-item>
        <el-form-item :label="t('singbox.protocol.wireguard.keepAlive')">
          <el-input-number v-model="p.persistent_keepalive_interval" :min="0" @change="emit" />
        </el-form-item>
      </el-form>
    </div>
    <el-button size="small" @click="addPeer">+ {{ t('singbox.protocol.wireguard.addPeer') }}</el-button>
  </el-form>
</template>

<script setup>
// WireGuard 协议专属设置（通用 Listen 由容器层提供）
import { useI18n } from 'vue-i18n'
import client from '../../../../../api/client'

const props = defineProps({
  model: { type: Object, required: true },
  serverId: { type: Number, required: true },
  coreType: { type: String, default: 'singbox' }
})
const { t } = useI18n()
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }

const apiBase = `/node-servers/${props.serverId}/binaries`

// 生成 WireGuard 密钥对
async function genWgKeyPair() {
  try {
    const data = await client.post(`${apiBase}/wg-key?core_type=${props.coreType}`)
    props.model._wg_private_key = data.privateKey || ''
    props.model._wg_public_key = data.publicKey || ''
    emit()
  } catch {
    // show error
  }
}

// 私钥变化时尝试派生公钥
async function onPrivateKeyChange(val) {
  if (!val) return
  try {
    const data = await client.post(`${apiBase}/wg-derive?core_type=${props.coreType}`, { private_key: val })
    if (data.publicKey) props.model._wg_public_key = data.publicKey
  } catch {
    // clear public key if derive fails
    props.model._wg_public_key = ''
  }
  emit()
}

// 添加 Peer
function addPeer() {
  if (!props.model._wg_peers) props.model._wg_peers = []
  props.model._wg_peers.push({
    public_key: '',
    _allowedIPsStr: '0.0.0.0/0, ::/0',
    allowed_ips: ['0.0.0.0/0', '::/0'],
    persistent_keepalive_interval: 0,
  })
  emit()
}

// Allowed IPs 字符串 → 数组转换
function onAllowedIPsChange(peer) {
  peer.allowed_ips = (peer._allowedIPsStr || '').split(',').map(s => s.trim()).filter(Boolean)
  emit()
}
</script>

<style scoped>
.user-item {
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 4px;
  padding: 8px;
  margin-bottom: 8px;
}
.hint {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 4px;
}
</style>
