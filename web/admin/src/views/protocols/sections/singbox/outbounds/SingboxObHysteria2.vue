<template>
  <el-form label-width="160px" label-position="left">
    <el-form-item :label="t('singbox.obProtocol.password')">
      <div style="display:flex;gap:8px;flex:1">
        <el-input v-model="model.password" @change="emit" />
        <el-button size="small" @click="model.password = generatePassword(); emit()">{{ t('singbox.users.generate') }}</el-button>
      </div>
    </el-form-item>

    <el-divider content-position="left">{{ t('singbox.obProtocol.bandwidth') }}</el-divider>
    <el-form-item :label="t('singbox.obProtocol.upMbps')">
      <el-input v-model="model['up_mbps']" :placeholder="t('singbox.obProtocol.speedPlaceholder')" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.downMbps')">
      <el-input v-model="model['down_mbps']" :placeholder="t('singbox.obProtocol.speedPlaceholder')" @change="emit" />
    </el-form-item>

    <el-divider content-position="left">{{ t('singbox.obProtocol.portHopping') }}</el-divider>
    <el-form-item :label="t('singbox.obProtocol.serverPorts')">
      <el-input v-model="model.server_ports" :placeholder="t('singbox.obProtocol.serverPortsHint')" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.hopInterval')">
      <el-input v-model="model.hop_interval" placeholder="30s" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.hopIntervalMax')">
      <el-input v-model="model.hop_interval_max" placeholder="60s" @change="emit" />
    </el-form-item>

    <el-divider content-position="left">{{ t('singbox.obProtocol.obfs') }}</el-divider>
    <el-form-item :label="t('singbox.obProtocol.obfsType')">
      <el-select v-model="model._hy2ObfsType" clearable style="width:100%" @change="emit">
        <el-option label="" value="" />
        <el-option label="Salamander" value="salamander" />
        <el-option label="Gecko" value="gecko" />
      </el-select>
    </el-form-item>
    <el-form-item v-if="model._hy2ObfsType" :label="t('singbox.obProtocol.obfsPassword')">
      <el-input v-model="model._hy2ObfsPassword" @change="emit" />
    </el-form-item>
    <template v-if="model._hy2ObfsType === 'gecko'">
      <el-form-item :label="t('singbox.obProtocol.obfsMinPacketSize')">
        <el-input-number v-model="model._hy2ObfsMinPacketSize" :min="0" style="width:100%" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.obProtocol.obfsMaxPacketSize')">
        <el-input-number v-model="model._hy2ObfsMaxPacketSize" :min="0" style="width:100%" @change="emit" />
      </el-form-item>
    </template>

    <el-divider content-position="left">{{ t('singbox.obProtocol.advanced') }}</el-divider>
    <el-form-item :label="t('singbox.obProtocol.bbrProfile')">
      <el-select v-model="model._hy2BbrProfile" clearable style="width:100%" @change="emit">
        <el-option label="" value="" />
        <el-option label="Standard" value="standard" />
        <el-option label="Conservative" value="conservative" />
        <el-option label="Aggressive" value="aggressive" />
      </el-select>
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.brutalDebug')">
      <el-switch v-model="model._hy2BrutalDebug" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.disableChromeParrot')">
      <el-switch v-model="model._hy2DisableChromeParrot" @change="emit" />
    </el-form-item>

    <el-divider content-position="left">{{ t('singbox.obProtocol.quicSection') }}</el-divider>
    <el-form-item :label="t('singbox.obProtocol.initialPacketSize')">
      <el-input-number v-model="model._hy2InitialPacketSize" :min="0" style="width:100%" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.disablePathMtu')">
      <el-switch v-model="model._hy2DisablePathMtu" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.idleTimeout')">
      <el-input v-model="model._hy2IdleTimeout" placeholder="30s" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.keepAlivePeriod')">
      <el-input v-model="model._hy2KeepAlivePeriod" placeholder="10s" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.streamRecvWindow')">
      <el-input v-model="model._hy2StreamRecvWindow" placeholder="8MB" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.connRecvWindow')">
      <el-input v-model="model._hy2ConnRecvWindow" placeholder="20MB" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.maxConcurrentStreams')">
      <el-input-number v-model="model._hy2MaxConcurrentStreams" :min="0" style="width:100%" @change="emit" />
    </el-form-item>

    <el-divider content-position="left">
      <div style="display:flex;align-items:center;gap:8px">
        <span>{{ t('singbox.obProtocol.realmSection') }}</span>
        <el-switch v-model="model._hy2RealmEnabled" size="small" @change="emit" />
      </div>
    </el-divider>
    <template v-if="model._hy2RealmEnabled">
      <div class="hint">{{ t('singbox.obProtocol.realmConflictHint') }}</div>
      <el-form-item :label="t('singbox.obProtocol.realmServerUrl')">
        <el-input v-model="model._hy2RealmServerUrl" placeholder="https://realm.example.com" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.obProtocol.realmToken')">
        <el-input v-model="model._hy2RealmToken" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.obProtocol.realmId')">
        <el-input v-model="model._hy2RealmId" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.obProtocol.realmStunServers')">
        <el-input v-model="model._hy2RealmStunServers" placeholder="stun:stun.l.google.com:19302" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.obProtocol.realmIpVersion')">
        <el-select v-model="model._hy2RealmIpVersion" style="width:100%" @change="emit">
          <el-option label="Auto" :value="0" />
          <el-option label="IPv4" :value="4" />
          <el-option label="IPv6" :value="6" />
        </el-select>
      </el-form-item>
      <el-form-item :label="t('singbox.obProtocol.realmHttpClientTag')">
        <el-input v-model="model._hy2RealmHttpClientTag" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.obProtocol.realmPortMapping')">
        <el-switch v-model="model._hy2RealmPortMappingEnabled" @change="emit" />
      </el-form-item>
      <template v-if="model._hy2RealmPortMappingEnabled">
        <el-form-item :label="t('singbox.obProtocol.realmPmTimeout')">
          <el-input v-model="model._hy2RealmPortMappingTimeout" placeholder="10s" @change="emit" />
        </el-form-item>
        <el-form-item :label="t('singbox.obProtocol.realmPmLifetime')">
          <el-input v-model="model._hy2RealmPortMappingLifetime" placeholder="300s" @change="emit" />
        </el-form-item>
      </template>
    </template>
  </el-form>
</template>

<script setup>
// Hysteria2 出站：option/hysteria2.go Hysteria2OutboundOptions
// 注意：出站带宽字段是 up_mbps/down_mbps（up/down 是 v1 的字节格式字段）
import { useI18n } from 'vue-i18n'
import { generatePassword } from '../utils/singboxDefaults'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>

<style scoped>
.hint { color: #909399; font-size: 12px; margin-top: 4px; }
</style>
