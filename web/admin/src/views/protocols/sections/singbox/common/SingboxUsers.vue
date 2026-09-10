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
      <template v-if="PSK_PROTOCOLS.includes(model.type)">
        <el-input v-model="u.password" :placeholder="t('singbox.users.password')" style="width:150px" @change="emit" />
        <el-button size="small" @click="u.password = generateKey(); emit()">{{ t('singbox.users.generate') }}</el-button>
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
      <el-button size="small" type="danger" text @click="model.users.splice(ui, 1); emit()">X</el-button>
    </div>
    <el-button size="small" @click="addUser">{{ t('singbox.users.addUser') }}</el-button>
  </div>
</template>

<script setup>
import { useI18n } from 'vue-i18n'
import { UUID_PROTOCOLS, PASSWORD_PROTOCOLS, PSK_PROTOCOLS, AUTH_USER_PROTOCOLS, USERKEY_PROTOCOLS, generateUuid, generatePassword, generateUsername, generateKey, generateEmail } from '../utils/singboxDefaults'

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
  if (PSK_PROTOCOLS.includes(props.model.type)) u.password = generateKey()
  if (AUTH_USER_PROTOCOLS.includes(props.model.type)) { u.username = generateUsername(); u.password = generatePassword() }
  if (USERKEY_PROTOCOLS.includes(props.model.type)) u.userkey = generateKey()
  props.model.users = props.model.users || []
  props.model.users.push(u)
  emit()
}
</script>

<style scoped>
.user-row { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 8px; align-items: center; }
</style>
