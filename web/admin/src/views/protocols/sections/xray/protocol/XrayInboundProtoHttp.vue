<template>
  <el-divider content-position="left">Accounts</el-divider>
  <div v-for="(a, ai) in rf.settings.accounts" :key="ai">
    <XrayUserItem :user="a" title="Account" :index="ai" @remove="rf.settings.accounts.splice(ai,1)">
      <el-form-item label="User"><el-input v-model="a.user" /></el-form-item>
      <el-form-item label="Pass"><el-input v-model="a.pass" /></el-form-item>
    </XrayUserItem>
  </div>
  <el-button size="small" @click="addAccount">+ Add Account</el-button>
  <el-divider content-position="left">Options</el-divider>
  <el-form label-width="140px" label-position="left">
    <el-form-item label="Allow Transparent">
      <el-switch v-model="rf.settings.allowTransparent" />
    </el-form-item>
  </el-form>
</template>

<script setup>
import XrayUserItem from '../common/XrayUserItem.vue'
import { genEmail, genPassword } from '../utils/xrayInboundUtils'
const props = defineProps({ rf: { type: Object, required: true }, serverId: Number, coreType: String })
function addAccount() {
  props.rf.settings.accounts.push({ user: genEmail().split('@')[0], pass: genPassword(), _quota: 'unlimited', _expiry: null, _usedUp: 0, _usedDown: 0 })
}
</script>