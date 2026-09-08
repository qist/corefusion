<template>
  <el-form label-width="120px" label-position="left">
    <el-form-item label="Secret Key">
      <div style="display:flex;gap:8px">
        <el-input v-model="rf.settings.secretKey" show-password style="flex:1" />
        <el-button size="small" @click="genWgKeyPair">Generate</el-button>
      </div>
    </el-form-item>
    <el-form-item label="Public Key">
      <el-input v-model="rf._wgPublicKey" readonly />
      <div class="hint">{{ t('protoCenter.transport.autoDerive') }}</div>
    </el-form-item>
    <el-form-item label="MTU">
      <el-input-number v-model="rf.settings.mtu" :min="0" />
      <div class="hint">{{ t('protoCenter.transport.defaultMtu') }}</div>
    </el-form-item>
    <el-form-item label="No Kernel TUN">
      <el-switch v-model="rf.settings.noKernelTun" />
    </el-form-item>
    <el-form-item label="Workers">
      <el-input-number v-model="rf.settings.workers" :min="0" />
    </el-form-item>
    <el-divider content-position="left">Peers</el-divider>
    <div v-for="(p, pi) in rf.settings.peers" :key="pi" class="user-item">
      <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:8px">
        <span style="font-weight:600">Peer #{{ pi + 1 }}</span>
        <el-button size="small" type="danger" text @click="rf.settings.peers.splice(pi,1)">X</el-button>
      </div>
      <el-form label-width="100px" label-position="left" size="small">
        <el-form-item label="Public Key"><el-input v-model="p.publicKey" /></el-form-item>
        <el-form-item label="Allowed IPs"><el-input v-model="p._allowedIPsStr" placeholder="0.0.0.0/0, ::/0" /></el-form-item>
        <el-form-item label="Keep Alive"><el-input-number v-model="p.keepAlive" :min="0" /></el-form-item>
        <el-form-item label="Comment"><el-input v-model="p.comment" placeholder="e.g. iPhone, Laptop" /></el-form-item>
      </el-form>
    </div>
    <el-button size="small" @click="addPeer">+ Add Peer</el-button>
  </el-form>
</template>

<script setup>
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import client from '../../../../../api/client'
const props = defineProps({ rf: { type: Object, required: true }, serverId: Number, coreType: String })
const { t } = useI18n()
const apiBase = `/node-servers/${props.serverId}/binaries`

async function genWgKeyPair() {
  try {
    const data = await client.post(`${apiBase}/wg-key?core_type=` + (props.coreType || 'xray'))
    props.rf.settings.secretKey = data.privateKey || ''
    props.rf._wgPublicKey = data.publicKey || ''
  } catch {
    ElMessage.error(t('protoCenter.reality.generateFailed'))
  }
}
function addPeer() {
  props.rf.settings.peers.push({ publicKey: '', _allowedIPsStr: '0.0.0.0/0, ::/0', keepAlive: 0, comment: '' })
}
</script>