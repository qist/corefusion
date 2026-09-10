<template>
  <el-form label-width="170px" label-position="left">
    <!-- 基础 -->
    <el-form-item :label="t('singbox.protocol.shadowtls.version')">
      <el-select v-model="model.version" style="width:100%" @change="emit">
        <el-option label="1" :value="1" />
        <el-option label="2" :value="2" />
        <el-option label="3" :value="3" />
      </el-select>
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.shadowtls.password')">
      <div style="display:flex;gap:8px;flex:1">
        <el-input v-model="model.password" @change="emit" />
        <el-button size="small" @click="model.password = generatePassword(); emit()">{{ t('singbox.users.generate') }}</el-button>
      </div>
    </el-form-item>

    <!-- 握手服务器 -->
    <el-divider content-position="left">{{ t('singbox.protocol.shadowtls.handshakeSection') }}</el-divider>
    <el-form-item :label="t('singbox.protocol.shadowtls.handshakeServer')">
      <el-input v-model="model._shadowtlsHandshakeServer" placeholder="google.com" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.shadowtls.handshakePort')">
      <el-input-number v-model="model._shadowtlsHandshakePort" :min="1" :max="65535" style="width:100%" @change="emit" />
    </el-form-item>

    <!-- 握手 Dial 字段 -->
    <el-divider content-position="left">{{ t('singbox.protocol.shadowtls.dialSection') }}</el-divider>
    <el-form-item :label="t('singbox.protocol.shadowtls.detour')">
      <el-input v-model="model._shadowtlsHandshakeDetour" placeholder="" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.shadowtls.bindInterface')">
      <el-input v-model="model._shadowtlsHandshakeBindInterface" placeholder="eth0" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.shadowtls.routingMark')">
      <el-input-number v-model="model._shadowtlsHandshakeRoutingMark" :min="0" style="width:100%" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.shadowtls.reuseAddr')">
      <el-switch v-model="model._shadowtlsHandshakeReuseAddr" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.shadowtls.connectTimeout')">
      <el-input v-model="model._shadowtlsHandshakeConnectTimeout" placeholder="5s" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.shadowtls.tcpFastOpen')">
      <el-switch v-model="model._shadowtlsHandshakeTcpFastOpen" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.shadowtls.tcpMultiPath')">
      <el-switch v-model="model._shadowtlsHandshakeTcpMultiPath" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.shadowtls.disableTcpKeepAlive')">
      <el-switch v-model="model._shadowtlsHandshakeDisableTcpKeepAlive" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.shadowtls.domainResolver')">
      <el-input v-model="model._shadowtlsHandshakeDomainResolver" placeholder="" @change="emit" />
    </el-form-item>

    <!-- 按 SNI 握手 -->
    <el-divider content-position="left">{{ t('singbox.protocol.shadowtls.hsnSection') }}</el-divider>
    <div v-for="(e, i) in (model._shadowtlsHsn || [])" :key="i" class="hsn-item">
      <el-row :gutter="8">
        <el-col :span="8">
          <el-input v-model="e.sni" placeholder="SNI (e.g. example.com)" @change="emit" />
        </el-col>
        <el-col :span="8">
          <el-input v-model="e.server" placeholder="handshake server" @change="emit" />
        </el-col>
        <el-col :span="6">
          <el-input-number v-model="e.server_port" :min="1" :max="65535" style="width:100%" @change="emit" />
        </el-col>
        <el-col :span="2">
          <el-button size="small" type="danger" @click="model._shadowtlsHsn.splice(i, 1); emit()">
            <el-icon><Delete /></el-icon>
          </el-button>
        </el-col>
      </el-row>
      <el-row :gutter="8" style="margin-top:4px">
        <el-col :span="6">
          <el-input v-model="e.detour" placeholder="detour" @change="emit" />
        </el-col>
        <el-col :span="6">
          <el-input v-model="e.bind_interface" placeholder="bind interface" @change="emit" />
        </el-col>
        <el-col :span="6">
          <el-input v-model="e.connect_timeout" placeholder="connect timeout" @change="emit" />
        </el-col>
        <el-col :span="6">
          <el-input v-model="e.domain_resolver" placeholder="domain resolver" @change="emit" />
        </el-col>
      </el-row>
    </div>
    <el-button size="small" type="primary" plain @click="addHsn">{{ t('singbox.protocol.shadowtls.addHsn') }}</el-button>

    <!-- 高级 -->
    <el-divider content-position="left">{{ t('singbox.protocol.shadowtls.advanced') }}</el-divider>
    <el-form-item :label="t('singbox.protocol.shadowtls.strictMode')">
      <el-switch v-model="model._shadowtlsStrictMode" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.shadowtls.wildcardSni')">
      <el-select v-model="model._shadowtlsWildcardSni" style="width:100%" clearable @change="emit">
        <el-option label="Off" value="" />
        <el-option label="Authed" value="authed" />
        <el-option label="All" value="all" />
      </el-select>
    </el-form-item>
  </el-form>
</template>

<script setup>
// ShadowTLS 协议专属设置 — 完整参数
// sing-box 源码: ShadowTLSInboundOptions { ListenOptions, Version, Password, Users,
//   Handshake (ServerOptions + DialerOptions), HandshakeForServerName (map[string]Handshake),
//   StrictMode, WildcardSNI }
// 注意：shadowtls 无 TLS/Transport/Multiplex 字段（不在 TLS_ENABLED/TRANSPORT/MULTIPLEX 协议列表中）
// 通用 Listen/Users 由容器层提供
import { useI18n } from 'vue-i18n'
import { Delete } from '@element-plus/icons-vue'
import { generatePassword } from '../utils/singboxDefaults'
const { t } = useI18n()
const props = defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
function addHsn() {
  if (!Array.isArray(props.model._shadowtlsHsn)) props.model._shadowtlsHsn = []
  props.model._shadowtlsHsn.push({ sni: '', server: '', server_port: 443, detour: '', bind_interface: '', connect_timeout: '', tcp_fast_open: false, domain_resolver: '' })
  emit()
}
</script>

<style scoped>
.hsn-item {
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 4px;
  padding: 8px;
  margin-bottom: 8px;
}
</style>
