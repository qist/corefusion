<template>
  <el-form label-width="120px" label-position="left">
    <el-form-item :label="t('protoCenter.masque.address')">
      <el-input
        v-model="rf.settings._addressStr"
        type="textarea"
        :rows="2"
        placeholder="10.0.0.1/24&#10;fd00::1/64"
      />
      <div class="hint">{{ t('protoCenter.masque.addressHint') }}</div>
    </el-form-item>
    <el-form-item :label="t('protoCenter.masque.mtu')">
      <el-input-number v-model="rf.settings.mtu" :min="1280" :max="65535" />
      <div class="hint">{{ t('protoCenter.masque.mtuHint') }}</div>
    </el-form-item>
    <el-divider content-position="left">
      {{ t('protoCenter.masque.users') }}
      <el-button size="small" type="primary" @click="addUser" style="margin-left:12px">+ {{ t('protoCenter.masque.users') }}</el-button>
    </el-divider>
    <div v-for="(u, ui) in rf.settings.clients" :key="ui">
      <XrayUserItem :user="u" :title="t('protoCenter.masque.users')" :index="ui" @remove="rf.settings.clients.splice(ui, 1)">
        <el-form-item :label="t('protoCenter.masque.email')">
          <el-input v-model="u.email" placeholder="love@xray.com" />
        </el-form-item>
        <el-form-item :label="t('protoCenter.masque.password')">
          <div style="display:flex;gap:8px">
            <el-input v-model="u.pass" style="flex:1" />
            <el-button size="small" @click="u.pass = genBase64Password()">{{ t('protoCenter.tls.generate') }}</el-button>
          </div>
        </el-form-item>
        <el-form-item :label="t('protoCenter.masque.level')">
          <el-input-number v-model="u.level" :min="0" />
        </el-form-item>
      </XrayUserItem>
    </div>
  </el-form>
</template>

<script setup>
import { useI18n } from 'vue-i18n'
import XrayUserItem from '../common/XrayUserItem.vue'
import { genEmail, genBase64Password } from '../utils/xrayInboundUtils'

const props = defineProps({ rf: { type: Object, required: true }, serverId: Number, coreType: String })
const { t } = useI18n()

function addUser() {
  props.rf.settings.clients.push({
    email: genEmail(), pass: genBase64Password(), level: 0,
    _quota: 'unlimited', _expiry: null, _usedUp: 0, _usedDown: 0,
  })
}
</script>
