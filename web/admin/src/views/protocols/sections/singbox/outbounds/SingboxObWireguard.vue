<template>
  <el-form label-width="150px" label-position="left">
    <el-form-item :label="t('singbox.obProtocol.privateKey')">
      <div style="display:flex;gap:8px;flex:1">
        <el-input v-model="model._wg_private_key" @change="emit" />
        <el-button size="small" @click="model._wg_private_key = generateKey(); emit()">{{ t('singbox.users.generate') }}</el-button>
      </div>
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.address')">
      <el-input v-model="model._wg_address" :placeholder="t('singbox.obProtocol.addressPlaceholder')" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.peers')">
      <div style="width:100%">
        <div v-for="(p, i) in (model.peers || [])" :key="i" style="display:flex;flex-wrap:wrap;gap:6px;margin-bottom:6px">
          <el-input v-model="p.server" :placeholder="t('singbox.obProtocol.peerServer')" style="width:140px" @change="emit" />
          <el-input v-model="p['public_key']" :placeholder="t('singbox.obProtocol.peerPublicKey')" style="flex:1" @change="emit" />
          <el-input v-model="p.preshared_key" :placeholder="t('singbox.obProtocol.peerPresharedKey')" style="width:160px" @change="emit" />
          <el-input v-model="p['allowed_ips']" :placeholder="t('singbox.obProtocol.peerAllowedIps')" style="width:150px" @change="emit" />
          <el-button size="small" type="danger" text @click="model.peers.splice(i, 1); emit()">X</el-button>
        </div>
        <el-button size="small" @click="(model.peers = model.peers || []).push({ server: '', public_key: '', allowed_ips: '0.0.0.0/0' }); emit()">{{ t('singbox.obProtocol.addPeer') }}</el-button>
      </div>
    </el-form-item>
  </el-form>
</template>

<script setup>
// WireGuard 出站：专属设置(PrivateKey/Address/Peers)；Base 由容器层提供
import { useI18n } from 'vue-i18n'
import { generateKey } from '../utils/singboxDefaults'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>