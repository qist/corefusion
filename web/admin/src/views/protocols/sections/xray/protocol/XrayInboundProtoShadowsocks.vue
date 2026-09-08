<template>
  <el-form label-width="120px" label-position="left">
    <el-form-item label="Method">
      <el-select v-model="rf.settings.method" style="width:100%" @change="onSSMethodChange">
        <el-option label="2022-blake3-aes-256-gcm" value="2022-blake3-aes-256-gcm" />
        <el-option label="2022-blake3-aes-128-gcm" value="2022-blake3-aes-128-gcm" />
        <el-option label="2022-blake3-chacha20-poly1305" value="2022-blake3-chacha20-poly1305" />
        <el-option label="aes-256-gcm" value="aes-256-gcm" />
        <el-option label="chacha20-poly1305" value="chacha20-poly1305" />
        <el-option label="chacha20-ietf-poly1305" value="chacha20-ietf-poly1305" />
        <el-option label="xchacha20-ietf-poly1305" value="xchacha20-ietf-poly1305" />
      </el-select>
    </el-form-item>
    <el-form-item label="Password">
      <div style="display:flex;gap:8px">
        <el-input v-model="rf.settings.password" style="flex:1" />
        <el-button size="small" @click="rf.settings.password = genSSPassword(rf.settings.method)">Generate</el-button>
      </div>
      <div class="hint">{{ isSS2022(rf.settings.method) ? 'Base64 encoded key (auto-generated)' : 'Plain text password' }}</div>
    </el-form-item>
    <el-form-item label="Network">
      <el-select v-model="rf.settings.network" style="width:100%">
        <el-option label="tcp,udp" value="tcp,udp" />
        <el-option label="tcp" value="tcp" />
        <el-option label="udp" value="udp" />
      </el-select>
    </el-form-item>
    <el-form-item label="ivCheck">
      <el-switch v-model="rf.settings.ivCheck" />
      <div class="hint">Enable IV check for legacy methods</div>
    </el-form-item>
  </el-form>

  <!-- Multi-user clients (only for 2022-blake3-aes-*-gcm methods) -->
  <template v-if="isSSMultiUser(rf.settings.method)">
    <el-divider content-position="left">
      Clients
      <el-button size="small" type="primary" @click="addUser" style="margin-left:12px">+ Add Client</el-button>
    </el-divider>
    <div v-for="(c, ci) in rf.settings.clients" :key="ci">
      <XrayUserItem :user="c" title="Client" :index="ci" @remove="rf.settings.clients.splice(ci,1)">
        <el-form-item label="Password">
          <div style="display:flex;gap:8px">
            <el-input v-model="c.password" style="flex:1" />
            <el-button size="small" @click="c.password = genBase64Password()">Generate</el-button>
          </div>
        </el-form-item>
        <el-form-item label="Email">
          <el-input v-model="c.email" placeholder="user@xray.com" />
        </el-form-item>
      </XrayUserItem>
    </div>
  </template>
</template>

<script setup>
import XrayUserItem from '../common/XrayUserItem.vue'
import { genEmail, genBase64Password, genSSPassword, isSS2022, isSSMultiUser } from '../utils/xrayInboundUtils'
const props = defineProps({ rf: { type: Object, required: true }, serverId: Number, coreType: String })
function onSSMethodChange(method) {
  props.rf.settings.password = genSSPassword(method)
  if (isSSMultiUser(method)) {
    if (!props.rf.settings.clients || props.rf.settings.clients.length === 0) {
      props.rf.settings.clients = [{ password: genBase64Password(), email: genEmail(), _quota: 'unlimited', _expiry: null, _usedUp: 0, _usedDown: 0 }]
    }
  } else {
    props.rf.settings.clients = []
  }
}
function addUser() {
  props.rf.settings.clients.push({ password: genBase64Password(), email: genEmail(), _quota: 'unlimited', _expiry: null, _usedUp: 0, _usedDown: 0 })
}
</script>