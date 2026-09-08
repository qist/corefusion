<template>
  <el-form label-width="180px" label-position="left">
    <el-form-item label="Name">
      <el-input v-model="rf.settings.name" placeholder="xray0" />
      <div class="hint">{{ t('protoCenter.transport.tunNameHint') }}</div>
    </el-form-item>
    <el-form-item label="MTU">
      <el-input-number v-model="rf.settings.mtu" :min="0" />
      <div class="hint">{{ t('protoCenter.transport.tunMtu') }}</div>
    </el-form-item>
    <el-form-item label="Gateway">
      <el-input v-model="rf._tunGatewayStr" placeholder="10.0.0.1/16, fc00::1/64" />
      <div class="hint">{{ t('protoCenter.transport.tunAddresses') }}</div>
    </el-form-item>
    <el-form-item label="DNS">
      <el-input v-model="rf._tunDnsStr" placeholder="1.1.1.1, 8.8.8.8" />
      <div class="hint">{{ t('protoCenter.transport.tunDns') }}</div>
    </el-form-item>
    <el-form-item label="User Level">
      <el-input-number v-model="rf.settings.userLevel" :min="0" />
    </el-form-item>
    <el-form-item label="Auto System Routing Table">
      <el-input v-model="rf._tunAutoRoutingStr" placeholder="0.0.0.0/0, ::/0" />
      <div class="hint">{{ t('protoCenter.transport.tunAutoRoute') }}</div>
    </el-form-item>
    <el-form-item label="Auto Outbounds Interface">
      <el-input v-model="rf.settings.autoOutboundsInterface" placeholder="auto" />
      <div class="hint">{{ t('protoCenter.transport.tunStrictRoute') }}</div>
    </el-form-item>
  </el-form>
</template>

<script setup>
import { useI18n } from 'vue-i18n'
defineProps({ rf: { type: Object, required: true }, serverId: Number, coreType: String })
const { t } = useI18n()
</script>