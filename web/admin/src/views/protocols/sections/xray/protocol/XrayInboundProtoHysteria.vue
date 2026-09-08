<template>
  <el-form label-width="120px" label-position="left">
    <el-form-item label="Version">
      <el-input-number v-model="rf.settings.version" :min="2" :max="2" />
    </el-form-item>
    <el-divider content-position="left">
      Users
      <el-button size="small" type="primary" @click="addUser" style="margin-left:12px">+ Add User</el-button>
    </el-divider>
    <div v-for="(u, ui) in rf.settings.clients" :key="ui">
      <XrayUserItem :user="u" title="User" :index="ui" @remove="rf.settings.clients.splice(ui,1)">
        <el-form-item label="Auth">
          <div style="display:flex;gap:8px">
            <el-input v-model="u.auth" style="flex:1" />
            <el-button size="small" @click="u.auth = genBase64Password()">Generate</el-button>
          </div>
        </el-form-item>
        <el-form-item label="Email">
          <el-input v-model="u.email" placeholder="user@xray.com" />
        </el-form-item>
        <el-form-item label="Level">
          <el-input-number v-model="u.level" :min="0" />
        </el-form-item>
      </XrayUserItem>
    </div>
  </el-form>
</template>

<script setup>
import XrayUserItem from '../common/XrayUserItem.vue'
import { genEmail, genBase64Password } from '../utils/xrayInboundUtils'
const props = defineProps({ rf: { type: Object, required: true }, serverId: Number, coreType: String })
function addUser() {
  props.rf.settings.clients.push({ auth: genBase64Password(), email: genEmail(), level: 0, _quota: 'unlimited', _expiry: null, _usedUp: 0, _usedDown: 0 })
}
</script>