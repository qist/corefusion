<template>
  <el-form label-width="120px" label-position="left">
    <el-form-item label="Encryption">
      <div style="display:flex;gap:8px;width:100%">
        <el-select v-model="rf.settings._encryptionType" style="flex:1" clearable @change="onVlessEncryptionTypeChange">
          <el-option label="None" value="" />
          <el-option label="X25519 (Classic)" value="x25519" />
          <el-option label="ML-KEM-768 (Post-Quantum)" value="mlkem768" />
        </el-select>
        <el-button size="small" @click="generateVlessEnc" :disabled="!rf.settings._encryptionType">{{ t('protoCenter.tls.generate') }}</el-button>
      </div>
      <div class="hint">VLESS-level encryption (vlessenc). Select type first, then generate keys.</div>
    </el-form-item>
    <el-form-item label="Encryption Key">
      <el-input v-model="rf.settings.encryption" placeholder="none" readonly />
      <div class="hint">Client-side encryption key (auto-generated)</div>
    </el-form-item>
    <el-form-item label="Decryption Key">
      <el-input v-model="rf.settings.decryption" placeholder="none" readonly />
      <div class="hint">Server-side decryption key (auto-generated)</div>
    </el-form-item>
    <el-divider content-position="left">
      Users
      <el-button size="small" type="primary" @click="addUser('vless')" style="margin-left:12px">+ Add User</el-button>
    </el-divider>
    <div v-for="(u, ui) in rf.settings.clients" :key="ui">
      <XrayUserItem :user="u" title="User" :index="ui" @remove="rf.settings.clients.splice(ui,1)">
        <el-form-item label="UUID">
          <div style="display:flex;gap:8px">
            <el-input v-model="u.id" placeholder="UUID" style="flex:1" />
            <el-button size="small" @click="u.id = genUUID()">Generate</el-button>
          </div>
        </el-form-item>
        <el-form-item label="Email">
          <el-input v-model="u.email" placeholder="user@xray.com" />
        </el-form-item>
        <el-form-item v-if="canEnableFlow" label="Flow">
          <el-select v-model="u.flow" style="width:100%" clearable>
            <el-option label="none" value="" />
            <el-option label="xtls-rprx-vision" value="xtls-rprx-vision" />
          </el-select>
        </el-form-item>
        <el-form-item label="Level">
          <el-input-number v-model="u.level" :min="0" />
        </el-form-item>
      </XrayUserItem>
    </div>
    <!-- Vision Seed (settings level, only when any user has flow) -->
    <el-form-item v-if="canEnableFlow && hasFlowUser" label="Vision Seed">
      <div style="display:flex;gap:4px;align-items:center">
        <el-input-number v-for="(_, si) in 4" :key="si" v-model="getTestseed(rf.settings)[si]" :min="1" size="small" style="width:25%" />
        <el-button size="small" @click="randomSeed(rf.settings)">Rand</el-button>
        <el-button size="small" @click="resetSeed(rf.settings)">Reset</el-button>
      </div>
      <div class="hint">Applied to settings level for xtls-rprx-vision flow</div>
    </el-form-item>
  </el-form>
</template>

<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import client from '../../../../../api/client'
import XrayUserItem from '../common/XrayUserItem.vue'
import { genUUID, genEmail, genBase64Password, getTestseed, randomSeed, resetSeed } from '../utils/xrayInboundUtils'

const props = defineProps({ rf: { type: Object, required: true }, serverId: Number, coreType: String })
const { t } = useI18n()

const apiBase = `/node-servers/${props.serverId}/binaries`

const canEnableFlow = computed(() => {
  const network = props.rf.streamSettings?.network || 'raw'
  const security = props.rf.streamSettings?.security || 'none'
  return network === 'raw' && (security === 'tls' || security === 'reality')
})

const hasFlowUser = computed(() => {
  return (props.rf.settings?.clients || []).some(u => u.flow)
})

function addUser(proto) {
  props.rf.settings.clients.push({ id: genUUID(), email: genEmail(), flow: '', level: 0, _quota: 'unlimited', _expiry: null, _usedUp: 0, _usedDown: 0 })
}

async function generateVlessEnc() {
  const encType = props.rf.settings._encryptionType
  if (!encType) {
    ElMessage.warning('Please select encryption type first')
    return
  }
  try {
    const data = await client.post(`${apiBase}/vlessenc`)
    if (data && data.auths && data.auths.length > 0) {
      const auth = data.auths.find(a => a.id === encType)
      if (auth) {
        props.rf.settings.encryption = auth.encryption || ''
        props.rf.settings.decryption = auth.decryption || ''
        ElMessage.success(t('protoCenter.tls.generateSuccess'))
      } else {
        ElMessage.error('No matching encryption type found')
      }
    }
  } catch (e) {
    ElMessage.error(t('protoCenter.reality.generateFailed'))
  }
}

async function onVlessEncryptionTypeChange(val) {
  if (val) {
    await generateVlessEnc()
  } else {
    props.rf.settings.encryption = 'none'
    props.rf.settings.decryption = 'none'
  }
}
</script>