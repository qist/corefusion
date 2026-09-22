<template>
  <div>
    <!-- Standard protocols -->
    <el-form v-if="['vmess','vless','trojan','shadowsocks','socks','http','hysteria'].includes(rf.protocol)" label-width="120px" label-position="left">
      <template v-if="['vmess','vless','trojan','shadowsocks','hysteria'].includes(rf.protocol)">
        <el-form-item :label="t('protoCenter.outbounds.server')">
          <el-input v-model="rf._server" placeholder="example.com" />
        </el-form-item>
        <el-form-item :label="t('protoCenter.outbounds.port')">
          <el-input-number v-model="rf._port" :min="1" :max="65535" />
        </el-form-item>
      </template>
      <template v-if="rf.protocol === 'vmess' || rf.protocol === 'vless'">
        <el-form-item label="UUID">
          <el-input v-model="rf._uuid" />
        </el-form-item>
        <el-form-item v-if="rf.protocol === 'vless'" label="Encryption">
          <el-input v-model="rf._vlessEncryption" placeholder="none" />
          <div class="hint">{{ t('protoCenter.outbounds.vlessEncryptionHint') }}</div>
        </el-form-item>
        <el-form-item v-if="rf.protocol === 'vless'" label="Flow">
          <el-select v-model="rf._flow" style="width:100%" clearable>
            <el-option label="none" value="" />
            <el-option label="xtls-rprx-vision" value="xtls-rprx-vision" />
            <el-option label="xtls-rprx-vision-udp443" value="xtls-rprx-vision-udp443" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="rf.protocol === 'vless'" label="Reverse Proxy">
          <el-switch v-model="rf._vlessReverseEnabled" />
        </el-form-item>
        <template v-if="rf.protocol === 'vless' && rf._vlessReverseEnabled">
          <el-form-item label="Reverse Tag">
            <el-input v-model="rf._vlessReverseTag" placeholder="r-inbound" />
            <div class="hint">{{ t('protoCenter.outbounds.vlessReverseTagHint') }}</div>
          </el-form-item>
          <el-divider content-position="left">Reverse Sniffing</el-divider>
          <el-form-item :label="t('protoCenter.outbounds.reverseSniffingEnable')">
            <el-switch v-model="rf._vlessReverseSniffing" />
          </el-form-item>
          <template v-if="rf._vlessReverseSniffing">
            <el-form-item :label="t('protoCenter.outbounds.reverseSniffingOverride')">
              <el-checkbox-group v-model="rf._vlessReverseSniffingOverride">
                <el-checkbox value="http" label="http" /><el-checkbox value="tls" label="tls" />
                <el-checkbox value="quic" label="quic" /><el-checkbox value="fakedns" label="fakedns" />
              </el-checkbox-group>
            </el-form-item>
            <el-form-item :label="t('protoCenter.outbounds.reverseSniffingMetadataOnly')">
              <el-switch v-model="rf._vlessReverseSniffingMetadata" />
            </el-form-item>
            <el-form-item :label="t('protoCenter.outbounds.reverseSniffingRouteOnly')">
              <el-switch v-model="rf._vlessReverseSniffingRouteOnly" />
            </el-form-item>
            <el-form-item :label="t('protoCenter.outbounds.reverseSniffingDomainsExcluded')">
              <el-input v-model="rf._vlessReverseSniffingDomainsExcluded" type="textarea" :rows="2" placeholder="courier.push.apple.com&#10;domain:example.com" />
              <div class="hint">{{ t('protoCenter.outbounds.reverseSniffingDomainsExcludedHint') }}</div>
            </el-form-item>
            <el-form-item :label="t('protoCenter.outbounds.reverseSniffingIpsExcluded')">
              <el-input v-model="rf._vlessReverseSniffingIpsExcluded" type="textarea" :rows="2" placeholder="10.0.0.0/8" />
              <div class="hint">{{ t('protoCenter.outbounds.reverseSniffingIpsExcludedHint') }}</div>
            </el-form-item>
          </template>
        </template>
        <el-form-item v-if="rf.protocol === 'vmess'" :label="t('protoCenter.outbounds.security')">
          <el-select v-model="rf._security" style="width:100%">
            <el-option label="auto" value="auto" /><el-option label="aes-128-gcm" value="aes-128-gcm" />
            <el-option label="chacha20-poly1305" value="chacha20-poly1305" />
            <el-option label="none" value="none" /><el-option label="zero" value="zero" />
          </el-select>
        </el-form-item>
      </template>
      <template v-if="rf.protocol === 'trojan'">
        <el-form-item :label="t('protoCenter.outbounds.password')">
          <el-input v-model="rf._password" />
        </el-form-item>
      </template>
      <template v-if="rf.protocol === 'shadowsocks'">
        <el-form-item :label="t('protoCenter.outbounds.method')">
          <el-select v-model="rf._ssMethod" style="width:100%" filterable allow-create>
            <el-option label="2022-blake3-aes-128-gcm" value="2022-blake3-aes-128-gcm" />
            <el-option label="2022-blake3-aes-256-gcm" value="2022-blake3-aes-256-gcm" />
            <el-option label="2022-blake3-chacha20-poly1305" value="2022-blake3-chacha20-poly1305" />
            <el-option label="aes-256-gcm" value="aes-256-gcm" />
            <el-option label="aes-128-gcm" value="aes-128-gcm" />
            <el-option label="chacha20-poly1305" value="chacha20-poly1305" />
            <el-option label="xchacha20-poly1305" value="xchacha20-poly1305" />
            <el-option label="none" value="none" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('protoCenter.outbounds.password')">
          <el-input v-model="rf._password" />
          <div class="hint">{{ t('protoCenter.outbounds.ssPasswordHint') }}</div>
        </el-form-item>
      </template>
      <template v-if="rf.protocol === 'hysteria'">
        <el-form-item label="Version">
          <el-input-number v-model="rf._hysteriaVersion" :min="2" :max="2" disabled />
          <div class="hint">{{ t('protoCenter.outbounds.hysteriaVersionHint') }}</div>
        </el-form-item>
      </template>
      <template v-if="rf.protocol === 'socks' || rf.protocol === 'http'">
        <el-form-item :label="t('protoCenter.outbounds.server')">
          <el-input v-model="rf._server" placeholder="127.0.0.1" />
        </el-form-item>
        <el-form-item :label="t('protoCenter.outbounds.port')">
          <el-input-number v-model="rf._port" :min="1" :max="65535" />
        </el-form-item>
        <el-form-item :label="t('protoCenter.outbounds.username')">
          <el-input v-model="rf._username" />
        </el-form-item>
        <el-form-item :label="t('protoCenter.outbounds.password')">
          <el-input v-model="rf._password" />
        </el-form-item>
        <el-form-item label="Email">
          <el-input v-model="rf._email" placeholder="user@example.com" />
        </el-form-item>
        <el-form-item label="User Level">
          <el-input-number v-model="rf._level" :min="0" />
        </el-form-item>
      </template>
    </el-form>

    <!-- WireGuard -->
    <div v-if="rf.protocol === 'wireguard'">
      <div style="margin-bottom:12px">
        <el-button size="small" type="primary" @click="importWarp">{{ t('protoCenter.outbounds.importWarp') }}</el-button>
      </div>
      <el-form label-width="140px" label-position="left">
        <el-form-item label="Secret Key">
          <div style="display:flex;gap:8px;width:100%">
            <el-input v-model="rf._wgSecretKey" style="flex:1" show-password />
            <el-button size="small" @click="genWgKey">{{ t('protoCenter.outbounds.generate') }}</el-button>
          </div>
        </el-form-item>
        <el-form-item v-if="rf._wgPublicKey" label="Public Key">
          <el-input :model-value="rf._wgPublicKey" readonly />
          <div class="hint">{{ t('protoCenter.outbounds.wgPublicKeyHint') }}</div>
        </el-form-item>
        <el-form-item label="Address (IPv4)">
          <el-input v-model="rf._wgAddressIPv4" placeholder="172.16.0.2/32" @change="emit" />
        </el-form-item>
        <el-form-item label="Address (IPv6)">
          <el-input v-model="rf._wgAddressIPv6" placeholder="fd00::2/128" @change="emit" />
        </el-form-item>
        <el-form-item label="MTU">
          <el-input-number v-model="rf._wgMtu" :min="0" />
        </el-form-item>
        <el-form-item label="Workers">
          <el-input-number v-model="rf._wgWorkers" :min="0" />
        </el-form-item>
        <el-form-item label="Reserved">
          <el-input v-model="rf._wgReservedStr" placeholder="0, 0, 0" />
        </el-form-item>
        <el-form-item label="No Kernel TUN">
          <el-switch v-model="rf._wgNoKernelTun" />
        </el-form-item>
        <el-form-item label="Domain Strategy">
          <el-select v-model="rf._wgDomainStrategy" style="width:100%" clearable>
            <el-option :label="t('protoCenter.outbounds.wgDomainStrategyDefault')" value="ForceIP" />
            <el-option label="ForceIPv4" value="ForceIPv4" />
            <el-option label="ForceIPv6" value="ForceIPv6" />
            <el-option label="ForceIPv4v6" value="ForceIPv4v6" />
            <el-option label="ForceIPv6v4" value="ForceIPv6v4" />
          </el-select>
          <div class="hint">{{ t('protoCenter.outbounds.wgDomainStrategyHint') }}</div>
        </el-form-item>
        <el-form-item label="Remote DNS">
          <el-input v-model="rf._wgRemoteDNS" placeholder="1.1.1.1, 8.8.8.8" @change="emit" />
          <div class="hint">WireGuard 隧道内使用的 DNS 服务器，逗号分隔（写入 settings.remoteDNS；替代已废弃的 dns 字段）</div>
        </el-form-item>
      </el-form>
      <el-divider content-position="left">Peers</el-divider>
      <div v-for="(p, pi) in rf._wgPeers" :key="pi" style="border:1px solid #e4e7ed;border-radius:6px;padding:12px;margin-bottom:12px">
        <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:8px">
          <span style="font-weight:500">Peer {{ pi + 1 }}</span>
          <el-button size="small" type="danger" text @click="rf._wgPeers.splice(pi,1)">X</el-button>
        </div>
        <el-form label-width="140px" label-position="left" size="small">
          <el-form-item label="Public Key"><el-input v-model="p.publicKey" /></el-form-item>
          <el-form-item label="Endpoint"><el-input v-model="p.endpoint" placeholder="1.2.3.4:51820" /></el-form-item>
          <el-form-item label="Pre-Shared Key"><el-input v-model="p.preSharedKey" show-password /></el-form-item>
          <el-form-item label="Keep Alive (s)"><el-input-number v-model="p.keepAlive" :min="0" /></el-form-item>
          <el-form-item label="Allowed IPs (IPv4)">
            <el-input v-model="p._allowedIPv4" placeholder="0.0.0.0/0" @change="emit" />
          </el-form-item>
          <el-form-item label="Allowed IPs (IPv6)">
            <el-input v-model="p._allowedIPv6" placeholder="::/0" @change="emit" />
          </el-form-item>
        </el-form>
      </div>
      <el-button size="small" @click="rf._wgPeers.push({publicKey:'',endpoint:'',preSharedKey:'',keepAlive:0,_allowedIPv4:'',_allowedIPv6:''})">+ Add Peer</el-button>
    </div>
  </div>
</template>

<script setup>
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import client from '../../../../../api/client'
const props = defineProps({ rf: { type: Object, required: true }, serverId: Number, coreType: String })
const { t } = useI18n()
const apiBase = `/node-servers/${props.serverId}/binaries`

function emit() {}

async function genWgKey() {
  try {
    const data = await client.post(`${apiBase}/wg-key?core_type=` + (props.coreType || 'xray'))
    if (data && data.privateKey) {
      props.rf._wgSecretKey = data.privateKey
      props.rf._wgPublicKey = data.publicKey || ''
      ElMessage.success(t('common.success'))
    }
  } catch (e) { ElMessage.error(t('common.failed') + ': ' + (e.message || e)) }
}

async function importWarp() {
  try {
    const coreType = props.coreType || 'xray'
    const warpBase = `/node-servers/${props.serverId}/warp`
    let status
    try { status = await client.get(`${warpBase}/status`, { params: { core_type: coreType } }) } catch { status = null }
    if (!status || !status.registered) {
      ElMessage.info(t('protoCenter.outbounds.warpRegistering'))
      status = await client.post(`${warpBase}/register`, { core_type: coreType })
      if (!status || !status.registered) {
        ElMessage.error(t('protoCenter.outbounds.warpRegisterFailed'))
        return
      }
    }
    props.rf.tag = 'warp-' + Math.random().toString(36).substring(2, 6)
    props.rf._wgSecretKey = status.private_key || ''
    let reserved = status.reserved || ''
    if (reserved.startsWith('[')) {
      try { reserved = JSON.parse(reserved).join(', ') } catch {}
    }
    props.rf._wgReservedStr = reserved
    props.rf._wgMtu = 1280
    props.rf._wgAddressIPv4 = status.address_ipv4 ? status.address_ipv4 + '/32' : ''
    props.rf._wgAddressIPv6 = status.address_ipv6 ? status.address_ipv6 + '/128' : ''
    props.rf._wgPeers = [{
      publicKey: status.public_key || '',
      endpoint: status.endpoint || 'engage.cloudflareclient.com:2408',
      preSharedKey: '',
      keepAlive: 0,
      _allowedIPv4: '0.0.0.0/0',
      _allowedIPv6: '::/0',
    }]
    ElMessage.success(t('protoCenter.outbounds.warpImportSuccess'))
  } catch (e) { ElMessage.error(t('protoCenter.outbounds.warpImportFailed') + ': ' + (e.message || e)) }
}
</script>