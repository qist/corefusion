<template>
  <div>
    <div v-for="(u, ui) in model.users" :key="ui" class="user-row">
      <el-input v-model="u.name" :placeholder="t('singbox.users.name')" style="width:150px" @change="emit" />
      <template v-if="UUID_PROTOCOLS.includes(model.type)">
        <el-input v-model="u.uuid" :placeholder="t('singbox.users.uuid')" style="width:190px" @change="emit" />
        <el-button size="small" @click="u.uuid = generateUuid(); emit()">{{ t('singbox.users.generate') }}</el-button>
      </template>
      <template v-if="AUTH_USER_PROTOCOLS.includes(model.type)">
        <el-input v-model="u.username" :placeholder="t('singbox.users.username')" style="width:130px" @change="emit" />
        <el-button size="small" @click="u.username = generateUsername(); emit()">{{ t('singbox.users.generate') }}</el-button>
      </template>
      <template v-if="PASSWORD_PROTOCOLS.includes(model.type) || AUTH_USER_PROTOCOLS.includes(model.type)">
        <el-input v-model="u.password" :placeholder="t('singbox.users.password')" style="width:150px" @change="emit" />
        <el-button size="small" @click="u.password = generatePassword(); emit()">{{ t('singbox.users.generate') }}</el-button>
      </template>
      <template v-if="USERKEY_PROTOCOLS.includes(model.type)">
        <el-input v-model="u.userkey" :placeholder="t('singbox.users.userkey')" style="width:150px" @change="emit" />
        <el-button size="small" @click="u.userkey = generateKey(); emit()">{{ t('singbox.users.generate') }}</el-button>
      </template>
      <el-input-number v-if="model.type === 'vmess'" v-model="u.alterId" :min="0" :max="65535" :placeholder="t('singbox.users.alterId')" style="width:100px" @change="emit" />
      <el-select v-if="model.type === 'vless'" v-model="u.flow" style="width:150px" clearable :placeholder="t('singbox.users.flow')" @change="emit">
        <el-option label="" value="" />
        <el-option label="xtls-rprx-vision" value="xtls-rprx-vision" />
      </el-select>
      <!-- Share per user -->
      <div style="display:flex;gap:4px">
        <el-button size="small" @click="copyShare(u, ui)">{{ t('singbox.users.share') }}</el-button>
        <el-button size="small" type="primary" plain @click="showQr(u, ui)">{{ t('singbox.users.qrCode') }}</el-button>
      </div>
      <el-button size="small" type="danger" text @click="model.users.splice(ui, 1); emit()">X</el-button>
    </div>
    <el-button size="small" @click="addUser">{{ t('singbox.users.addUser') }}</el-button>

    <!-- QR Dialog -->
    <el-dialog v-model="qrDlg" :title="qrTitle" width="400" top="10vh">
      <div style="text-align:center;padding:20px">
        <img v-if="qrCodeUrl" :src="qrCodeUrl" style="max-width:250px;margin-bottom:12px" />
        <el-input v-model="qrLink" type="textarea" :rows="3" readonly style="margin-top:8px" />
        <el-button size="small" style="margin-top:8px" @click="copyToClipboard(qrLink)">Copy Link</el-button>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { generateQrDataUrl, isQrDataTooLargeError } from '../../../../../utils/qrcode'
import { generateUuid, generatePassword, generateUsername, generateKey, generateEmail, UUID_PROTOCOLS, PASSWORD_PROTOCOLS, AUTH_USER_PROTOCOLS, USERKEY_PROTOCOLS } from '../utils/singboxDefaults'
import { generateShareLink } from '../utils/singboxShareLink'

const { t } = useI18n()
const props = defineProps({
  model: { type: Object, required: true },
  accessHost: { type: String, default: '' },
})
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }

function addUser() {
  const u = { name: generateEmail() }
  if (UUID_PROTOCOLS.includes(props.model.type)) { u.uuid = generateUuid(); if (props.model.type === 'vless') u.flow = 'xtls-rprx-vision'; if (props.model.type === 'vmess') u.alterId = 0 }
  if (PASSWORD_PROTOCOLS.includes(props.model.type)) u.password = generatePassword()
  if (AUTH_USER_PROTOCOLS.includes(props.model.type)) { u.username = generateUsername(); u.password = generatePassword() }
  if (USERKEY_PROTOCOLS.includes(props.model.type)) u.userkey = generateKey()
  props.model.users = props.model.users || []
  props.model.users.push(u)
  emit()
}

const qrDlg = ref(false)
const qrLink = ref('')
const qrCodeUrl = ref('')
const qrTitle = ref('')

function buildShareLink(u) {
  return generateShareLink(props.model, u, props.accessHost || props.model.listen || '', props.model.listen_port || 0, props.model.tag)
}

function copyShare(u) {
  const link = buildShareLink(u)
  if (!link) { ElMessage.warning(t('singbox.inbound.shareNotSupported')); return }
  copyToClipboard(link)
  ElMessage.success(t('singbox.inbound.shareCopied'))
}

async function showQr(u) {
  const link = buildShareLink(u)
  if (!link) { ElMessage.warning(t('singbox.inbound.shareNotSupported')); return }
  qrLink.value = link
  qrTitle.value = `${props.model.type} @ ${u.name || u.uuid || u.username || 'user'}`
  qrDlg.value = true
  qrCodeUrl.value = ''
  try {
    qrCodeUrl.value = await generateQrDataUrl(link)
  } catch (e) {
    if (isQrDataTooLargeError(e)) ElMessage.warning(t('singbox.inbound.linkTooLong'))
    else ElMessage.error(t('singbox.inbound.qrFailed'))
  }
}

function copyToClipboard(text) {
  if (!text) return
  const ta = document.createElement('textarea')
  ta.value = text
  document.body.appendChild(ta)
  ta.select()
  try { document.execCommand('copy'); ElMessage.success(t('singbox.inbound.shareCopied')) } catch { ElMessage.error(t('singbox.users.copyError')) }
  document.body.removeChild(ta)
}
</script>

<style scoped>
.user-row { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 8px; align-items: center; }
</style>