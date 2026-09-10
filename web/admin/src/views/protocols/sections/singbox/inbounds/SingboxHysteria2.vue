<template>
  <el-form label-width="140px" label-position="left">
    <!-- 带宽 -->
    <el-divider content-position="left">{{ t('singbox.protocol.hysteria2.bandwidth') }}</el-divider>
    <el-form-item :label="t('singbox.protocol.hysteria2.upMbps')">
      <el-input v-model="model.up_mbps" placeholder="100" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.hysteria2.downMbps')">
      <el-input v-model="model.down_mbps" placeholder="100" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.hysteria2.ignoreClientBw')">
      <el-switch v-model="model.ignore_client_bandwidth" @change="emit" />
      <div class="hint">{{ t('singbox.protocol.hysteria2.ignoreClientBwHint') }}</div>
    </el-form-item>

    <!-- 混淆 -->
    <el-divider content-position="left">{{ t('singbox.protocol.hysteria2.obfsSection') }}</el-divider>
    <el-form-item :label="t('singbox.protocol.hysteria2.obfsType')">
      <el-select v-model="model._hy2ObfsType" clearable style="width:100%" @change="onObfsTypeChange">
        <el-option label="Salamander" value="salamander" />
      </el-select>
    </el-form-item>
    <el-form-item v-if="model._hy2ObfsType" :label="t('singbox.protocol.hysteria2.obfsPassword')">
      <div style="display:flex;gap:8px;flex:1">
        <el-input v-model="model._hy2ObfsPassword" @change="emit" />
        <el-button size="small" @click="genObfsPassword">{{ t('singbox.tls.generate') }}</el-button>
      </div>
    </el-form-item>

    <!-- 伪装 -->
    <el-divider content-position="left">{{ t('singbox.protocol.hysteria2.masquerade') }}</el-divider>
    <el-form-item :label="t('singbox.protocol.hysteria2.masquerade')">
      <el-input v-model="model.masquerade" :placeholder="t('singbox.protocol.hysteria2.masqueradePlaceholder')" @change="emit" />
      <div class="hint">{{ t('singbox.protocol.hysteria2.masqueradeHint') }}</div>
    </el-form-item>

    <!-- BBR & Debug -->
    <el-divider content-position="left">{{ t('singbox.protocol.hysteria2.advanced') }}</el-divider>
    <el-form-item :label="t('singbox.protocol.hysteria2.bbrProfile')">
      <el-select v-model="model._hy2BbrProfile" clearable style="width:100%" @change="emit">
        <el-option label="" value="" />
        <el-option label="Standard" value="standard" />
        <el-option label="Conservative" value="conservative" />
        <el-option label="Aggressive" value="aggressive" />
      </el-select>
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.hysteria2.brutalDebug')">
      <el-switch v-model="model._hy2BrutalDebug" @change="emit" />
    </el-form-item>

    <!-- QUIC 参数 -->
    <el-divider content-position="left">{{ t('singbox.protocol.hysteria2.quicSection') }}</el-divider>
    <el-form-item :label="t('singbox.protocol.hysteria2.initialPacketSize')">
      <el-input-number v-model="model._hy2InitialPacketSize" :min="0" style="width:100%" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.hysteria2.disablePathMtu')">
      <el-switch v-model="model._hy2DisablePathMtu" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.hysteria2.idleTimeout')">
      <el-input v-model="model._hy2IdleTimeout" placeholder="30s" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.hysteria2.keepAlivePeriod')">
      <el-input v-model="model._hy2KeepAlivePeriod" placeholder="10s" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.hysteria2.streamRecvWindow')">
      <el-input v-model="model._hy2StreamRecvWindow" placeholder="8MB" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.hysteria2.connRecvWindow')">
      <el-input v-model="model._hy2ConnRecvWindow" placeholder="20MB" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.hysteria2.maxConcurrentStreams')">
      <el-input-number v-model="model._hy2MaxConcurrentStreams" :min="0" style="width:100%" @change="emit" />
    </el-form-item>

    <!-- Realm (P2P 代理) -->
    <el-divider content-position="left">
      <div style="display:flex;align-items:center;gap:8px">
        <span>{{ t('singbox.protocol.hysteria2.realmSection') }}</span>
        <el-switch v-model="model._hy2RealmEnabled" size="small" @change="emit" />
      </div>
    </el-divider>
    <template v-if="model._hy2RealmEnabled">
      <el-form-item :label="t('singbox.protocol.hysteria2.realmServerUrl')">
        <el-input v-model="model._hy2RealmServerUrl" placeholder="https://realm.example.com" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.protocol.hysteria2.realmToken')">
        <el-input v-model="model._hy2RealmToken" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.protocol.hysteria2.realmId')">
        <el-input v-model="model._hy2RealmId" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.protocol.hysteria2.realmStunServers')">
        <el-input v-model="model._hy2RealmStunServers" placeholder="stun:stun.l.google.com:19302,stun:stun1.l.google.com:19302" @change="emit" />
        <div class="hint">{{ t('singbox.protocol.hysteria2.realmStunServersHint') }}</div>
      </el-form-item>
      <el-form-item :label="t('singbox.protocol.hysteria2.realmStunDnsServer')">
        <el-input v-model="model._hy2RealmStunDnsServer" placeholder="udp://dns.google:53" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.protocol.hysteria2.realmIpVersion')">
        <el-select v-model="model._hy2RealmIpVersion" style="width:100%" @change="emit">
          <el-option label="Auto" :value="0" />
          <el-option label="IPv4" :value="4" />
          <el-option label="IPv6" :value="6" />
        </el-select>
      </el-form-item>
      <el-form-item :label="t('singbox.protocol.hysteria2.realmHttpClientTag')">
        <el-input v-model="model._hy2RealmHttpClientTag" placeholder="outbound tag, e.g. http-out" @change="emit" />
      </el-form-item>

      <!-- Port Mapping -->
      <el-form-item :label="t('singbox.protocol.hysteria2.realmPortMapping')">
        <el-switch v-model="model._hy2RealmPortMappingEnabled" @change="emit" />
      </el-form-item>
      <template v-if="model._hy2RealmPortMappingEnabled">
        <el-form-item :label="t('singbox.protocol.hysteria2.realmPmTimeout')">
          <el-input v-model="model._hy2RealmPortMappingTimeout" placeholder="10s" @change="emit" />
        </el-form-item>
        <el-form-item :label="t('singbox.protocol.hysteria2.realmPmLifetime')">
          <el-input v-model="model._hy2RealmPortMappingLifetime" placeholder="300s" @change="emit" />
        </el-form-item>
      </template>
    </template>
  </el-form>
</template>

<script setup>
// Hysteria2 协议专属设置（通用 Listen/Users/TLS 由容器层提供）
import { useI18n } from 'vue-i18n'
import { generatePassword } from '../utils/singboxDefaults'
const { t } = useI18n()
const props = defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }

function genObfsPassword() {
  props.model._hy2ObfsPassword = generatePassword(32)
  emit()
}

// Auto-generate obfs password when type is selected
function onObfsTypeChange(val) {
  if (val && !props.model._hy2ObfsPassword) {
    props.model._hy2ObfsPassword = generatePassword(32)
  }
  emit()
}
</script>

<style scoped>
.hint { color: #909399; font-size: 12px; margin-top: 4px; }
</style>
