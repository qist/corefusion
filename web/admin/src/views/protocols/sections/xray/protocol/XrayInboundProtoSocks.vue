<template>
  <el-form label-width="120px" label-position="left">
    <el-form-item label="Auth">
      <el-select v-model="rf.settings.auth" style="width:100%" @change="onSocksAuthChange">
        <el-option label="noauth" value="noauth" />
        <el-option label="password" value="password" />
      </el-select>
    </el-form-item>
    <template v-if="rf.settings.auth === 'password'">
      <el-divider content-position="left">Accounts</el-divider>
      <div v-for="(a, ai) in rf.settings.accounts" :key="ai">
        <XrayUserItem :user="a" title="Account" :index="ai" @remove="rf.settings.accounts.splice(ai,1)">
          <el-form-item label="User"><el-input v-model="a.user" /></el-form-item>
          <el-form-item label="Pass"><el-input v-model="a.pass" /></el-form-item>
        </XrayUserItem>
      </div>
      <el-button size="small" @click="addAccount">+ Add Account</el-button>
    </template>
    <el-form-item label="UDP">
      <el-switch v-model="rf.settings.udp" />
    </el-form-item>
  </el-form>
</template>

<script setup>
import XrayUserItem from '../common/XrayUserItem.vue'
import { genEmail, genPassword } from '../utils/xrayInboundUtils'
const props = defineProps({ rf: { type: Object, required: true }, serverId: Number, coreType: String })
function onSocksAuthChange(val) {
  if (val === 'password' && (!props.rf.settings.accounts || !props.rf.settings.accounts.length)) {
    props.rf.settings.accounts = [{ user: genEmail().split('@')[0], pass: genPassword(), _quota: 'unlimited', _expiry: null, _usedUp: 0, _usedDown: 0 }]
  }
}
function addAccount() {
  props.rf.settings.accounts.push({ user: genEmail().split('@')[0], pass: genPassword(), _quota: 'unlimited', _expiry: null, _usedUp: 0, _usedDown: 0 })
}
</script>