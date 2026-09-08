<template>
  <div>
    <div v-for="(u, ui) in model.users" :key="ui" class="user-row">
      <el-input v-model="u.name" placeholder="Email/Name" style="width:150px" @change="emit" />
      <template v-if="UUID_PROTOCOLS.includes(model.type)">
        <el-input v-model="u.uuid" placeholder="UUID" style="width:190px" @change="emit" />
        <el-button size="small" @click="u.uuid = generateUuid(); emit()">Generate</el-button>
      </template>
      <template v-if="AUTH_USER_PROTOCOLS.includes(model.type)">
        <el-input v-model="u.username" placeholder="Username" style="width:130px" @change="emit" />
        <el-button size="small" @click="u.username = generateUsername(); emit()">Gen</el-button>
      </template>
      <template v-if="PASSWORD_PROTOCOLS.includes(model.type) || AUTH_USER_PROTOCOLS.includes(model.type)">
        <el-input v-model="u.password" placeholder="Password" style="width:150px" @change="emit" />
        <el-button size="small" @click="u.password = generatePassword(); emit()">Gen</el-button>
      </template>
      <template v-if="USERKEY_PROTOCOLS.includes(model.type)">
        <el-input v-model="u.userkey" placeholder="UserKey" style="width:150px" @change="emit" />
        <el-button size="small" @click="u.userkey = generateKey(); emit()">Gen</el-button>
      </template>
      <el-input-number v-if="model.type === 'vmess'" v-model="u.alterId" :min="0" :max="65535" placeholder="AlterID" style="width:100px" @change="emit" />
      <el-select v-if="model.type === 'vless'" v-model="u.flow" style="width:150px" clearable placeholder="Flow" @change="emit">
        <el-option label="" value="" />
        <el-option label="xtls-rprx-vision" value="xtls-rprx-vision" />
      </el-select>
      <!-- Share per user -->
      <div style="display:flex;gap:4px">
        <el-button size="small" @click="copyShare(u, ui)">分享</el-button>
        <el-button size="small" type="primary" plain @click="showQr(u, ui)">二维码</el-button>
      </div>
      <el-button size="small" type="danger" text @click="model.users.splice(ui, 1); emit()">X</el-button>
    </div>
    <el-button size="small" @click="addUser">Add User</el-button>

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
import { ElMessage } from 'element-plus'
import { generateQrDataUrl, isQrDataTooLargeError } from '../../../../../utils/qrcode'
import { generateUuid, generatePassword, generateUsername, generateKey, UUID_PROTOCOLS, PASSWORD_PROTOCOLS, AUTH_USER_PROTOCOLS, USERKEY_PROTOCOLS } from '../utils/singboxDefaults'
import { generateShareLink } from '../utils/singboxShareLink'

const props = defineProps({
  model: { type: Object, required: true },
  accessHost: { type: String, default: '' },
})
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }

function addUser() {
  const u = { name: '' }
  if (UUID_PROTOCOLS.includes(model.type)) { u.uuid = generateUuid(); if (model.type === 'vless') u.flow = 'xtls-rprx-vision'; if (model.type === 'vmess') u.alterId = 0 }
  if (PASSWORD_PROTOCOLS.includes(model.type)) u.password = generatePassword()
  if (AUTH_USER_PROTOCOLS.includes(model.type)) { u.username = generateUsername(); u.password = generatePassword() }
  if (USERKEY_PROTOCOLS.includes(model.type)) u.userkey = generateKey()
  model.users = model.users || []
  model.users.push(u)
  emit()
}

const qrDlg = ref(false)
const qrLink = ref('')
const qrCodeUrl = ref('')
const qrTitle = ref('')

function buildShareLink(u) {
  return generateShareLink(model, u, props.accessHost || model.listen || '', model.listen_port || 0, model.tag)
}

function copyShare(u) {
  const link = buildShareLink(u)
  if (!link) { ElMessage.warning('该协议暂不支持分享链接'); return }
  copyToClipboard(link)
  ElMessage.success('已复制分享链接')
}

async function showQr(u) {
  const link = buildShareLink(u)
  if (!link) { ElMessage.warning('该协议暂不支持分享链接'); return }
  qrLink.value = link
  qrTitle.value = `${model.type} @ ${u.name || u.uuid || u.username || 'user'}`
  qrDlg.value = true
  qrCodeUrl.value = ''
  try {
    qrCodeUrl.value = await generateQrDataUrl(link)
  } catch (e) {
    if (isQrDataTooLargeError(e)) ElMessage.warning('链接过长，二维码无法生成')
    else ElMessage.error('二维码生成失败')
  }
}

function copyToClipboard(text) {
  if (!text) return
  const ta = document.createElement('textarea')
  ta.value = text
  document.body.appendChild(ta)
  ta.select()
  try { document.execCommand('copy'); ElMessage.success('已复制') } catch { ElMessage.error('复制失败') }
  document.body.removeChild(ta)
}
</script>

<style scoped>
.user-row { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 8px; align-items: center; }
</style>