<template>
  <el-form label-width="120px" label-position="left">
    <el-divider content-position="left">
      Users
      <el-button size="small" type="primary" @click="addUser" style="margin-left:12px">+ Add User</el-button>
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
        <el-form-item label="AlterID">
          <el-input-number v-model="u.alterId" :min="0" />
        </el-form-item>
        <el-form-item label="Security">
          <el-select v-model="u.security" style="width:100%">
            <el-option label="auto" value="auto" />
            <el-option label="aes-128-gcm" value="aes-128-gcm" />
            <el-option label="chacha20-poly1305" value="chacha20-poly1305" />
            <el-option label="none" value="none" />
          </el-select>
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
import { genUUID, genEmail } from '../utils/xrayInboundUtils'
const props = defineProps({ rf: { type: Object, required: true }, serverId: Number, coreType: String })
function addUser() {
  props.rf.settings.clients.push({ id: genUUID(), email: genEmail(), alterId: 0, security: 'auto', level: 0, _quota: 'unlimited', _expiry: null, _usedUp: 0, _usedDown: 0 })
}
</script>