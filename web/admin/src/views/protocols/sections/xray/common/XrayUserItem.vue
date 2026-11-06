<template>
  <div class="user-item">
    <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:8px">
      <span style="font-weight:600">{{ title }} #{{ index + 1 }}</span>
      <el-button size="small" type="danger" text @click="$emit('remove')">X</el-button>
    </div>
    <el-form label-width="100px" label-position="left" size="small">
      <!-- Credentials / protocol-specific fields injected by parent -->
      <slot />
      <el-divider content-position="left">Traffic &amp; Expiry</el-divider>
      <el-form-item :label="t('menu.trafficQuota')">
        <el-input v-model="user._quota" placeholder="100GB, unlimited" />
      </el-form-item>
      <el-form-item :label="t('proto2.trafficUsed')">
        <div style="display:flex;gap:8px;align-items:center">
          <span style="font-size:13px">↑ {{ formatTraffic(user._usedUp || 0) }} ↓ {{ formatTraffic(user._usedDown || 0) }}</span>
          <el-button size="small" type="warning" plain @click="resetTraffic(user)">Reset</el-button>
        </div>
      </el-form-item>
      <el-form-item :label="t('client.expiryTime')">
        <el-date-picker v-model="user._expiry" type="datetime" :placeholder="t('proto2.selectDate')" style="width:100%" />
      </el-form-item>
    </el-form>
  </div>
</template>

<script setup>
import { formatTraffic, resetTraffic } from '../utils/xrayInboundUtils'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
defineProps({ user: { type: Object, required: true }, title: { type: String, default: 'User' }, index: { type: Number, default: 0 } })
defineEmits(['remove'])
</script>