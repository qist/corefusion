<template>
  <div>
    <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:16px">
      <span style="font-size:15px;font-weight:600">{{ t('protoCenter.outbounds.title') }}</span>
      <el-button size="small" type="primary" @click="showAdd">{{ t('protoCenter.outbounds.add') }}</el-button>
    </div>

    <!-- List -->
    <div v-for="(ob, i) in form" :key="i" class="outbound-item">
      <div style="display:flex;justify-content:space-between;align-items:center">
        <div style="display:flex;align-items:center;gap:8px">
          <el-switch v-if="!ob._isDefault" v-model="ob._enabled" @change="toggleEnabled(i)" />
          <span style="font-weight:600">{{ ob.tag || 'outbound-' + (i+1) }}</span>
          <el-tag size="small">{{ ob.protocol }}</el-tag>
          <el-tag v-if="ob._isDefault" size="small" type="warning">默认</el-tag>
          <el-tag v-if="ob._server" size="small" type="info">{{ ob._server }}:{{ ob._port }}</el-tag>
        </div>
        <div>
          <el-button size="small" text @click="showEdit(i)">{{ t('common.edit') }}</el-button>
          <el-button v-if="!ob._isDefault" size="small" type="danger" text @click="removeOutbound(i)">{{ t('common.delete') }}</el-button>
        </div>
      </div>
    </div>
    <div v-if="!form.length" style="text-align:center;color:#909399;padding:24px">{{ t('common.noData') }}</div>

    <!-- Edit Dialog -->
    <el-dialog v-model="dlg" :title="editIdx >= 0 ? t('protoCenter.outbounds.edit') : t('protoCenter.outbounds.add')" width="750px" :close-on-click-modal="false" top="5vh">
      <el-tabs v-model="activeTab">
        <!-- Basic Tab -->
        <el-tab-pane :label="t('protoCenter.outbounds.basic')" name="basic">
          <el-form label-width="120px" label-position="left">
            <el-form-item :label="t('protoCenter.outbounds.tag')">
              <el-input v-model="rf.tag" placeholder="outbound-tag" />
            </el-form-item>
            <el-form-item :label="t('protoCenter.outbounds.protocol')">
              <el-select v-model="rf.protocol" style="width:100%" @change="onProtocolChange">
                <el-option label="Freedom (direct)" value="freedom" /><el-option label="Blackhole (block)" value="blackhole" />
                <el-option label="DNS" value="dns" /><el-option label="Loopback" value="loopback" />
                <el-option label="VMess" value="vmess" /><el-option label="VLESS" value="vless" />
                <el-option label="Trojan" value="trojan" /><el-option label="Shadowsocks" value="shadowsocks" />
                <el-option label="SOCKS" value="socks" /><el-option label="HTTP" value="http" />
                <el-option label="WireGuard" value="wireguard" />
              </el-select>
            </el-form-item>
            <template v-if="rf.protocol === 'freedom'">
              <el-form-item :label="t('protoCenter.outbounds.advancedSettings')">
                <el-switch v-model="rf._freedomAdvanced" />
              </el-form-item>
              <template v-if="rf._freedomAdvanced">
                <el-form-item :label="t('protoCenter.outbounds.domainStrategy')">
                  <el-select v-model="rf._domainStrategy" style="width:100%">
                    <el-option label="AsIs" value="AsIs" /><el-option label="UseIP" value="UseIP" />
                    <el-option label="UseIPv4" value="UseIPv4" /><el-option label="UseIPv6" value="UseIPv6" />
                    <el-option label="ForceIP" value="ForceIP" /><el-option label="ForceIPv4" value="ForceIPv4" /><el-option label="ForceIPv6" value="ForceIPv6" />
                  </el-select>
                </el-form-item>
                <el-form-item :label="t('protoCenter.outbounds.redirect')">
                  <el-input v-model="rf._freedomRedirect" placeholder="127.0.0.1:80 或 :443" @change="emit" />
                  <div class="hint">{{ t('protoCenter.outbounds.redirectHint') }}</div>
                </el-form-item>
                <el-form-item :label="t('protoCenter.outbounds.userLevel')">
                  <el-input-number v-model="rf._freedomUserLevel" :min="0" />
                </el-form-item>
                <el-form-item :label="t('protoCenter.outbounds.proxyProtocol')">
                  <el-select v-model="rf._freedomProxyProtocol" style="width:100%">
                    <el-option label="关闭 (0)" :value="0" /><el-option label="v1 (1)" :value="1" /><el-option label="v2 (2)" :value="2" />
                  </el-select>
                </el-form-item>

                <!-- Fragment -->
                <el-divider content-position="left">{{ t('protoCenter.outbounds.fragment') }}</el-divider>
                <el-form-item :label="t('protoCenter.outbounds.fragmentEnable')">
                  <el-switch v-model="rf._freedomFragmentEnabled" />
                </el-form-item>
                <template v-if="rf._freedomFragmentEnabled">
                  <el-form-item label="Packets">
                    <el-input v-model="rf._freedomFragmentPackets" placeholder="tlshello 或 1-3" />
                  </el-form-item>
                  <el-form-item label="Length">
                    <el-input v-model="rf._freedomFragmentLength" placeholder="100-200" />
                  </el-form-item>
                  <el-form-item label="Interval (ms)">
                    <el-input v-model="rf._freedomFragmentInterval" placeholder="10-20" />
                  </el-form-item>
                </template>

                <!-- Noises -->
                <el-divider content-position="left">{{ t('protoCenter.outbounds.noises') }}</el-divider>
                <el-form-item :label="t('protoCenter.outbounds.noisesEnable')">
                  <el-switch v-model="rf._freedomNoisesEnabled" />
                </el-form-item>
                <template v-if="rf._freedomNoisesEnabled">
                  <div v-for="(n, ni) in rf._freedomNoises" :key="ni" style="border:1px dashed #dcdfe6;border-radius:4px;padding:8px;margin-bottom:8px">
                    <div style="display:flex;gap:8px;margin-bottom:4px;align-items:center">
                      <el-select v-model="n.type" style="width:100px" @change="genNoisePacket(n)">
                        <el-option label="rand" value="rand" /><el-option label="str" value="str" /><el-option label="base64" value="base64" />
                      </el-select>
                      <el-input v-model="n.packet" :placeholder="n.type === 'rand' ? '长度或范围 (50-150)' : n.type === 'base64' ? 'Base64 数据' : '字符串内容'" style="flex:1" />
                      <el-button size="small" @click="genNoisePacket(n)">生成</el-button>
                      <el-input v-model="n.delay" placeholder="10-20" style="width:100px" />
                      <el-button size="small" type="danger" text @click="rf._freedomNoises.splice(ni,1)">X</el-button>
                    </div>
                  </div>
                  <el-button size="small" @click="addNoise()">{{ t('protoCenter.outbounds.addNoise') }}</el-button>
                </template>

                <!-- Final Rules -->
                <el-divider content-position="left">{{ t('protoCenter.outbounds.finalRules') }}</el-divider>
                <el-form-item :label="t('protoCenter.outbounds.finalRulesEnable')">
                  <el-switch v-model="rf._freedomFinalRulesEnabled" />
                </el-form-item>
                <template v-if="rf._freedomFinalRulesEnabled">
                  <div v-for="(rule, ri) in rf._freedomFinalRules" :key="ri" style="border:1px dashed #dcdfe6;border-radius:4px;padding:8px;margin-bottom:8px">
                    <div style="display:flex;gap:8px;margin-bottom:6px;align-items:center">
                      <el-select v-model="rule.action" style="width:80px" @change="onRuleActionChange(rule)">
                        <el-option label="allow" value="allow" /><el-option label="block" value="block" />
                      </el-select>
                      <el-select v-model="rule.network" style="width:120px" clearable>
                        <el-option label="(全部)" value="" />
                        <el-option label="tcp" value="tcp" /><el-option label="udp" value="udp" /><el-option label="tcp,udp" value="tcp,udp" />
                      </el-select>
                      <el-input v-model="rule.port" placeholder="22,25,465,587" style="flex:1" />
                      <el-button size="small" type="danger" text @click="rf._freedomFinalRules.splice(ri,1)">X</el-button>
                    </div>
                    <el-input v-model="rule._ipStr" type="textarea" :rows="1" placeholder="IP/CIDR/geoip:cn，逗号分隔" style="margin-bottom:6px" />
                    <div style="display:flex;gap:8px;align-items:center">
                      <span style="font-size:12px;color:#909399;width:80px">blockDelay:</span>
                      <el-input v-model="rule.blockDelay" placeholder="30-90" style="width:120px" />
                      <span style="font-size:12px;color:#909399">秒，省略默认 30-90</span>
                    </div>
                  </div>
                  <el-button size="small" @click="addFinalRule()">{{ t('protoCenter.outbounds.addRule') }}</el-button>
                </template>
              </template>
            </template>
            <template v-if="rf.protocol === 'blackhole'">
              <el-form-item label="高级设置">
                <el-switch v-model="rf._blackholeAdvanced" />
              </el-form-item>
              <template v-if="rf._blackholeAdvanced">
                <el-form-item label="Response Type">
                  <el-select v-model="rf._blackholeResponse" style="width:100%">
                    <el-option label="none" value="none" /><el-option label="http" value="http" />
                  </el-select>
                </el-form-item>
              </template>
            </template>

            <!-- DNS Settings -->
            <template v-if="rf.protocol === 'dns'">
              <el-form-item label="Rewrite Network">
                <el-select v-model="rf._dnsRewriteNetwork" style="width:100%" clearable>
                  <el-option label="(保持原样)" value="" /><el-option label="tcp" value="tcp" /><el-option label="udp" value="udp" />
                </el-select>
              </el-form-item>
              <el-form-item label="Rewrite Address">
                <el-input v-model="rf._dnsRewriteAddress" placeholder="如 8.8.8.8" />
              </el-form-item>
              <el-form-item label="Rewrite Port">
                <el-input-number v-model="rf._dnsRewritePort" :min="0" :max="65535" />
                <div class="hint">0 = 保持原端口</div>
              </el-form-item>
              <el-form-item label="User Level">
                <el-input-number v-model="rf._dnsUserLevel" :min="0" />
              </el-form-item>

              <el-divider content-position="left">Rules</el-divider>
              <div v-for="(rule, ri) in rf._dnsRules" :key="ri" style="border:1px dashed #dcdfe6;border-radius:4px;padding:8px;margin-bottom:8px">
                <div style="display:flex;gap:8px;margin-bottom:6px;align-items:center">
                  <el-select v-model="rule.action" style="width:100px">
                    <el-option label="direct" value="direct" /><el-option label="hijack" value="hijack" />
                    <el-option label="drop" value="drop" /><el-option label="return" value="return" />
                  </el-select>
                  <el-input v-model="rule.qType" placeholder="qType (如 1, 28, 65)" style="width:150px" />
                  <el-input v-if="rule.action === 'return'" v-model="rule.rCode" placeholder="rCode (0-65535)" style="width:130px" />
                  <el-button size="small" type="danger" text @click="rf._dnsRules.splice(ri,1)">X</el-button>
                </div>
                <el-input v-model="rule._domainsStr" type="textarea" :rows="1" placeholder="domain:example.com, geosite:geolocation-!cn" />
              </div>
              <el-button size="small" @click="rf._dnsRules.push({action:'direct',qType:'',rCode:'0',_domainsStr:''})">+ Add Rule</el-button>
            </template>

            <!-- Loopback Settings -->
            <template v-if="rf.protocol === 'loopback'">
              <el-form-item label="Inbound Tag">
                <el-select v-model="rf._loopbackInboundTag" style="width:100%" filterable allow-create clearable>
                  <el-option v-for="t in inboundTags" :key="t" :label="t" :value="t" />
                </el-select>
                <div class="hint">用于重新进入路由时的入站标识</div>
              </el-form-item>
            </template>
          </el-form>
        </el-tab-pane>

        <!-- Protocol Tab -->
        <el-tab-pane v-if="['vmess','vless','trojan','shadowsocks','socks','http'].includes(rf.protocol)" :label="t('protoCenter.outbounds.protocolSettings')" name="protocol">
          <el-form label-width="120px" label-position="left">
            <template v-if="['vmess','vless','trojan','shadowsocks'].includes(rf.protocol)">
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
              <el-form-item v-if="rf.protocol === 'vless'" label="Flow">
                <el-select v-model="rf._flow" style="width:100%" clearable>
                  <el-option label="none" value="" /><el-option label="xtls-rprx-vision" value="xtls-rprx-vision" />
                </el-select>
              </el-form-item>
              <el-form-item v-if="rf.protocol === 'vmess'" :label="t('protoCenter.outbounds.security')">
                <el-select v-model="rf._security" style="width:100%">
                  <el-option label="auto" value="auto" /><el-option label="aes-128-gcm" value="aes-128-gcm" />
                  <el-option label="chacha20-poly1305" value="chacha20-poly1305" />
                </el-select>
              </el-form-item>
            </template>
            <template v-if="rf.protocol === 'trojan' || rf.protocol === 'shadowsocks'">
              <el-form-item :label="t('protoCenter.outbounds.password')">
                <el-input v-model="rf._password" />
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
              <template v-if="rf.protocol === 'http'">
                <el-form-item label="Email">
                  <el-input v-model="rf._email" placeholder="user@example.com" />
                </el-form-item>
                <el-form-item label="User Level">
                  <el-input-number v-model="rf._level" :min="0" />
                </el-form-item>
              </template>
            </template>
          </el-form>
        </el-tab-pane>

        <!-- WireGuard Tab -->
        <el-tab-pane v-if="rf.protocol === 'wireguard'" label="WireGuard" name="protocol">
          <div style="margin-bottom:12px">
            <el-button size="small" type="primary" @click="importWarp">{{ t('protoCenter.outbounds.importWarp') }}</el-button>
          </div>
          <el-form label-width="140px" label-position="left">
            <el-form-item label="Secret Key">
              <div style="display:flex;gap:8px;width:100%">
                <el-input v-model="rf._wgSecretKey" style="flex:1" show-password />
                <el-button size="small" @click="genWgKey">生成</el-button>
              </div>
            </el-form-item>
            <el-form-item v-if="rf._wgPublicKey" label="Public Key">
              <el-input :model-value="rf._wgPublicKey" readonly />
              <div class="hint">配置服务端时需要此公钥</div>
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
                <el-option label="ForceIP (默认)" value="ForceIP" />
                <el-option label="ForceIPv4" value="ForceIPv4" />
                <el-option label="ForceIPv6" value="ForceIPv6" />
                <el-option label="ForceIPv4v6" value="ForceIPv4v6" />
                <el-option label="ForceIPv6v4" value="ForceIPv6v4" />
              </el-select>
              <div class="hint">域名解析策略，控制 WireGuard 目标地址解析</div>
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
        </el-tab-pane>

        <!-- Stream Settings Tab -->
        <el-tab-pane v-if="['vmess','vless','trojan','shadowsocks'].includes(rf.protocol)" :label="t('protoCenter.outbounds.stream')" name="stream">
          <el-form label-width="140px" label-position="left">
            <el-form-item :label="t('protoCenter.outbounds.network')">
              <el-select v-model="rf._network" style="width:100%" clearable>
                <el-option label="TCP" value="tcp" /><el-option label="WebSocket" value="ws" />
                <el-option label="gRPC" value="grpc" /><el-option label="HTTP/2" value="http" />
                <el-option label="HTTPUpgrade" value="httpupgrade" /><el-option label="XHTTP" value="xhttp" />
              </el-select>
            </el-form-item>
            <el-form-item :label="t('protoCenter.outbounds.securityMode')">
              <el-select v-model="rf._securityMode" style="width:100%">
                <el-option label="None" value="none" /><el-option label="TLS" value="tls" /><el-option label="Reality" value="reality" />
              </el-select>
            </el-form-item>

            <!-- TLS Settings -->
            <template v-if="rf._securityMode === 'tls'">
              <el-divider content-position="left">TLS</el-divider>
              <el-form-item label="SNI">
                <el-input v-model="rf._sni" placeholder="example.com" />
              </el-form-item>
              <el-form-item label="ALPN">
                <el-select v-model="rf._alpn" style="width:100%" multiple clearable>
                  <el-option label="h2" value="h2" /><el-option label="http/1.1" value="http/1.1" /><el-option label="h3" value="h3" />
                </el-select>
              </el-form-item>
              <el-form-item label="Fingerprint">
                <el-select v-model="rf._fingerprint" style="width:100%" clearable>
                  <el-option label="chrome" value="chrome" /><el-option label="firefox" value="firefox" />
                  <el-option label="safari" value="safari" /><el-option label="ios" value="ios" />
                  <el-option label="android" value="android" /><el-option label="random" value="random" />
                </el-select>
              </el-form-item>
              <el-form-item label="Allow Insecure">
                <el-switch v-model="rf._allowInsecure" />
              </el-form-item>
            </template>

            <!-- Reality Settings -->
            <template v-if="rf._securityMode === 'reality'">
              <el-divider content-position="left">REALITY</el-divider>
              <el-form-item label="Server Name">
                <el-input v-model="rf._realitySni" placeholder="example.com" />
              </el-form-item>
              <el-form-item label="Password (Public Key)">
                <el-input v-model="rf._realityPublicKey" />
              </el-form-item>
              <el-form-item label="Short ID">
                <el-input v-model="rf._realityShortId" />
              </el-form-item>
              <el-form-item label="Fingerprint">
                <el-select v-model="rf._realityFingerprint" style="width:100%">
                  <el-option label="chrome" value="chrome" /><el-option label="firefox" value="firefox" />
                  <el-option label="safari" value="safari" /><el-option label="ios" value="ios" />
                  <el-option label="android" value="android" /><el-option label="random" value="random" />
                </el-select>
              </el-form-item>
              <el-form-item label="SpiderX">
                <el-input v-model="rf._realitySpiderX" placeholder="crawler initial path" />
              </el-form-item>
            </template>

            <!-- WebSocket Settings -->
            <template v-if="rf._network === 'ws'">
              <el-divider content-position="left">WebSocket</el-divider>
              <el-form-item label="Path">
                <el-input v-model="rf._wsPath" placeholder="/" />
              </el-form-item>
              <el-form-item label="Host">
                <el-input v-model="rf._wsHost" />
              </el-form-item>
            </template>

            <!-- gRPC Settings -->
            <template v-if="rf._network === 'grpc'">
              <el-divider content-position="left">gRPC</el-divider>
              <el-form-item label="Service Name">
                <el-input v-model="rf._grpcServiceName" />
              </el-form-item>
            </template>
          </el-form>
        </el-tab-pane>
      </el-tabs>

      <template #footer>
        <el-button @click="dlg=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveOutbound">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, watch, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import client from '../../../api/client'
const { t } = useI18n()
const props = defineProps({ data: Array, coreType: String })

const form = ref([])
const dlg = ref(false)
const inboundTags = ref([])
const editIdx = ref(-1)
const activeTab = ref('basic')
const rf = reactive(getDefaultOutbound())

function getDefaultOutbound() {
  return {
    tag: '', protocol: 'freedom',
    _server: '', _port: 443, _uuid: '', _flow: '', _security: 'auto', _password: '', _username: '', _email: genEmail(), _level: 0,
    _freedomAdvanced: false, _domainStrategy: 'AsIs', _freedomRedirect: '', _freedomUserLevel: 0, _freedomProxyProtocol: 0,
    _freedomFragmentEnabled: false, _freedomFragmentPackets: 'tlshello', _freedomFragmentLength: '100-200', _freedomFragmentInterval: '10-20',
    _freedomNoisesEnabled: false, _freedomNoises: [{ type: 'rand', packet: '100', delay: '10-20' }],
    _freedomFinalRulesEnabled: false, _freedomFinalRules: [],
    _blackholeAdvanced: false, _blackholeResponse: 'none',
    _dnsRewriteNetwork: '', _dnsRewriteAddress: '', _dnsRewritePort: 0, _dnsUserLevel: 0, _dnsRules: [],
    _loopbackInboundTag: '',
    _network: 'tcp', _securityMode: 'none',
    _sni: '', _alpn: [], _fingerprint: '', _allowInsecure: false,
    _realitySni: '', _realityPublicKey: '', _realityShortId: '', _realityFingerprint: 'chrome', _realitySpiderX: '',
    _wsPath: '/', _wsHost: '', _grpcServiceName: '',
    _wgSecretKey: '', _wgPublicKey: '', _wgAddressIPv4: '', _wgAddressIPv6: '', _wgMtu: 1280, _wgWorkers: 2, _wgReservedStr: '', _wgNoKernelTun: false, _wgDomainStrategy: 'ForceIP',
    _wgPeers: [],
  }
}

function parseOutbound(ob) {
  const ss = ob.streamSettings || {}
  const vnext = ob.settings && ob.settings.vnext && ob.settings.vnext[0] ? ob.settings.vnext[0] : {}
  const srv = ob.settings && ob.settings.servers && ob.settings.servers[0] ? ob.settings.servers[0] : {}
  const user0 = vnext.users && vnext.users[0] ? vnext.users[0] : {}
  const tls = ss.tlsSettings || {}
  const reality = ss.realitySettings || {}
  const realitySettings = (reality.settings || [])[0] || {}
  const ws = ss.wsSettings || {}
  const grpc = ss.grpcSettings || {}
  var peers = (ob.settings && ob.settings.peers) || []
  return {
    tag: ob.tag || '', protocol: ob.protocol || 'freedom',
    _server: vnext.address || srv.address || (ob.settings && ob.settings.address) || '',
    _port: vnext.port || srv.port || (ob.settings && ob.settings.port) || 443,
    _uuid: user0.id || '',
    _flow: user0.flow || '',
    _security: user0.security || 'auto',
    _password: srv.password || (ob.settings && ob.settings.pass) || '',
    _username: srv.users && srv.users[0] ? srv.users[0].user || '' : (ob.settings && ob.settings.user) || '',
    _email: (ob.settings && ob.settings.email) || '',
    _level: (ob.settings && ob.settings.level) || 0,
    _freedomAdvanced: !!(ob.settings && (ob.settings.domainStrategy || ob.settings.redirect || ob.settings.fragment || ob.settings.noises || ob.settings.finalRules)),
    _domainStrategy: (ob.settings && ob.settings.domainStrategy) || 'AsIs',
    _freedomRedirect: (ob.settings && ob.settings.redirect) || '',
    _freedomUserLevel: (ob.settings && ob.settings.userLevel) || 0,
    _freedomProxyProtocol: (ob.settings && ob.settings.proxyProtocol) || 0,
    _freedomFragmentEnabled: !!(ob.settings && ob.settings.fragment),
    _freedomFragmentPackets: (ob.settings && ob.settings.fragment && ob.settings.fragment.packets) || 'tlshello',
    _freedomFragmentLength: (ob.settings && ob.settings.fragment && ob.settings.fragment.length) || '100-200',
    _freedomFragmentInterval: (ob.settings && ob.settings.fragment && ob.settings.fragment.interval) || '10-20',
    _freedomNoisesEnabled: !!(ob.settings && ob.settings.noises && ob.settings.noises.length),
    _freedomNoises: (ob.settings && ob.settings.noises) ? ob.settings.noises.map(n => ({ type: n.type || 'rand', packet: n.packet || '', delay: n.delay || '' })) : [],
    _freedomFinalRulesEnabled: !!(ob.settings && ob.settings.finalRules && ob.settings.finalRules.length),
    _freedomFinalRules: (ob.settings && ob.settings.finalRules) ? ob.settings.finalRules.map(r => ({
      action: r.action || 'block', network: r.network || '', port: r.port || '',
      _ipStr: (r.ip || []).join(', '), blockDelay: r.blockDelay || '',
    })) : [],
    _blackholeAdvanced: !!(ob.settings && ob.settings.response && ob.settings.response.type && ob.settings.response.type !== 'none'),
    _blackholeResponse: (ob.settings && ob.settings.response && ob.settings.response.type) || 'none',
    _dnsRewriteNetwork: (ob.settings && ob.settings.rewriteNetwork) || '',
    _dnsRewriteAddress: (ob.settings && ob.settings.rewriteAddress) || '',
    _dnsRewritePort: (ob.settings && ob.settings.rewritePort) || 0,
    _dnsUserLevel: (ob.settings && ob.settings.userLevel) || 0,
    _dnsRules: (ob.settings && ob.settings.rules) ? ob.settings.rules.map(r => ({
      action: r.action || 'direct', qType: r.qType || '', rCode: r.rCode || '0',
      _domainsStr: (r.domain || []).join(', '),
    })) : [],
    _loopbackInboundTag: (ob.settings && ob.settings.inboundTag) || '',
    _network: ss.network || 'tcp',
    _securityMode: ss.security || 'none',
    _sni: tls.serverName || '',
    _alpn: tls.alpn || [],
    _fingerprint: tls.fingerprint || '',
    _allowInsecure: tls.allowInsecure || false,
    _realitySni: reality.serverName || '',
    _realityPublicKey: reality.password || realitySettings.password || '',
    _realityShortId: reality.shortId || '',
    _realityFingerprint: reality.fingerprint || realitySettings.fingerprint || 'chrome',
    _realitySpiderX: reality.spiderX || '',
    _wsPath: ws.path || '/',
    _wsHost: ws.host || '',
    _grpcServiceName: grpc.serviceName || '',
    _wgSecretKey: (ob.settings && ob.settings.secretKey) || '',
    _wgAddressIPv4: Array.isArray(ob.settings && ob.settings.address) ? ob.settings.address.filter(a => !a.includes(':')).join(', ') : '',
    _wgAddressIPv6: Array.isArray(ob.settings && ob.settings.address) ? ob.settings.address.filter(a => a.includes(':')).join(', ') : '',
    _wgMtu: (ob.settings && ob.settings.mtu) || 1280,
    _wgDomainStrategy: (ob.settings && ob.settings.domainStrategy) || 'ForceIP',
    _wgWorkers: (ob.settings && ob.settings.workers) || 0,
    _wgReservedStr: ((ob.settings && ob.settings.reserved) || []).join(', '),
    _wgNoKernelTun: (ob.settings && ob.settings.noKernelTun) || false,
    _wgPeers: peers.map(function(p) {
      const ips = p.allowedIPs || []
      return {
        publicKey: p.publicKey || '', endpoint: p.endpoint || '', preSharedKey: p.preSharedKey || '', keepAlive: p.keepAlive || 0,
        _allowedIPv4: ips.filter(ip => !ip.includes(':')).join(', '),
        _allowedIPv6: ips.filter(ip => ip.includes(':')).join(', '),
      }
    }),
  }
}

function buildOutbound(ob) {
  const result = { tag: ob.tag, protocol: ob.protocol, settings: {}, streamSettings: {} }
  if (['vmess', 'vless'].includes(ob.protocol)) {
    const user = ob.protocol === 'vmess'
      ? { id: ob._uuid, alterId: 0, security: ob._security }
      : { id: ob._uuid, encryption: 'none', flow: ob._flow }
    result.settings.vnext = [{ address: ob._server, port: ob._port, users: [user] }]
    result.streamSettings = buildStreamSettings(ob)
  } else if (ob.protocol === 'trojan') {
    result.settings.servers = [{ address: ob._server, port: ob._port, password: ob._password }]
    result.streamSettings = buildStreamSettings(ob)
  } else if (ob.protocol === 'shadowsocks') {
    result.settings.servers = [{ address: ob._server, port: ob._port, password: ob._password }]
    result.streamSettings = buildStreamSettings(ob)
  } else if (ob.protocol === 'socks') {
    const srv = { address: ob._server, port: ob._port }
    if (ob._username) srv.users = [{ user: ob._username, pass: ob._password }]
    result.settings.servers = [srv]
    delete result.streamSettings
  } else if (ob.protocol === 'http') {
    result.settings = { address: ob._server, port: ob._port }
    if (ob._username) result.settings.user = ob._username
    if (ob._password) result.settings.pass = ob._password
    if (ob._level) result.settings.level = ob._level
    if (ob._email) result.settings.email = ob._email
    result.streamSettings = buildStreamSettings(ob)
  } else if (ob.protocol === 'wireguard') {
    result.settings.secretKey = ob._wgSecretKey
    const addresses = []
    if (ob._wgAddressIPv4) addresses.push(...ob._wgAddressIPv4.split(',').map(s => s.trim()).filter(Boolean))
    if (ob._wgAddressIPv6) addresses.push(...ob._wgAddressIPv6.split(',').map(s => s.trim()).filter(Boolean))
    result.settings.address = addresses
    result.settings.mtu = ob._wgMtu || 1280
    if (ob._wgDomainStrategy && ob._wgDomainStrategy !== 'ForceIP') result.settings.domainStrategy = ob._wgDomainStrategy
    if (ob._wgWorkers) result.settings.workers = ob._wgWorkers
    if (ob._wgReservedStr) result.settings.reserved = ob._wgReservedStr.split(',').map(function(s) { return parseInt(s.trim()) })
    result.settings.noKernelTun = ob._wgNoKernelTun || false
    result.settings.peers = (ob._wgPeers || []).map(function(p) {
      const allowedIPs = []
      if (p._allowedIPv4) allowedIPs.push(...p._allowedIPv4.split(',').map(s => s.trim()).filter(Boolean))
      if (p._allowedIPv6) allowedIPs.push(...p._allowedIPv6.split(',').map(s => s.trim()).filter(Boolean))
      return { publicKey: p.publicKey, endpoint: p.endpoint, preSharedKey: p.preSharedKey || undefined, keepAlive: p.keepAlive || 0, allowedIPs: allowedIPs.length ? allowedIPs : ['0.0.0.0/0'] }
    })
    delete result.streamSettings
  } else if (ob.protocol === 'freedom') {
    if (ob._freedomAdvanced) {
      result.settings = {}
      result.settings.domainStrategy = ob._domainStrategy || 'AsIs'
      if (ob._freedomRedirect) result.settings.redirect = ob._freedomRedirect
      result.settings.userLevel = ob._freedomUserLevel || 0
      if (ob._freedomProxyProtocol) result.settings.proxyProtocol = ob._freedomProxyProtocol
      if (ob._freedomFragmentEnabled) {
        result.settings.fragment = {}
        if (ob._freedomFragmentPackets) result.settings.fragment.packets = ob._freedomFragmentPackets
        if (ob._freedomFragmentLength) result.settings.fragment.length = ob._freedomFragmentLength
        if (ob._freedomFragmentInterval) result.settings.fragment.interval = ob._freedomFragmentInterval
      }
      if (ob._freedomNoisesEnabled && ob._freedomNoises && ob._freedomNoises.length) {
        result.settings.noises = ob._freedomNoises.filter(n => n.packet).map(n => {
          const noise = { type: n.type || 'rand', packet: n.packet }
          if (n.delay) noise.delay = n.delay
          return noise
        })
      }
      if (ob._freedomFinalRulesEnabled && ob._freedomFinalRules && ob._freedomFinalRules.length) {
        result.settings.finalRules = ob._freedomFinalRules.map(r => {
          const rule = { action: r.action || 'block' }
          if (r.network) rule.network = r.network
          if (r.port) rule.port = r.port
          if (r._ipStr) rule.ip = r._ipStr.split(',').map(s => s.trim()).filter(Boolean)
          if (r.blockDelay) rule.blockDelay = r.blockDelay
          return rule
        })
      }
    } else {
      delete result.settings
    }
    delete result.streamSettings
  } else if (ob.protocol === 'blackhole') {
    if (ob._blackholeAdvanced && ob._blackholeResponse !== 'none') {
      result.settings = { response: { type: ob._blackholeResponse } }
    } else {
      result.settings = { response: { type: 'none' } }
    }
    delete result.streamSettings
  } else if (ob.protocol === 'dns') {
    result.settings = {}
    if (ob._dnsRewriteNetwork) result.settings.rewriteNetwork = ob._dnsRewriteNetwork
    if (ob._dnsRewriteAddress) result.settings.rewriteAddress = ob._dnsRewriteAddress
    if (ob._dnsRewritePort) result.settings.rewritePort = ob._dnsRewritePort
    if (ob._dnsUserLevel) result.settings.userLevel = ob._dnsUserLevel
    if (ob._dnsRules && ob._dnsRules.length) {
      result.settings.rules = ob._dnsRules.map(r => {
        const rule = { action: r.action || 'direct' }
        if (r.qType) rule.qType = r.qType
        if (r.action === 'return' && r.rCode) rule.rCode = parseInt(r.rCode) || 0
        if (r._domainsStr) rule.domain = r._domainsStr.split(',').map(s => s.trim()).filter(Boolean)
        return rule
      })
    }
    if (!Object.keys(result.settings).length) delete result.settings
    delete result.streamSettings
  } else if (ob.protocol === 'loopback') {
    if (ob._loopbackInboundTag) {
      result.settings = { inboundTag: ob._loopbackInboundTag }
    }
    delete result.streamSettings
  }
  return result
}

function buildStreamSettings(ob) {
  const ss = { network: ob._network || 'tcp', security: ob._securityMode || 'none' }
  if (ob._securityMode === 'tls') {
    const tls = {}
    if (ob._sni) tls.serverName = ob._sni
    if (ob._alpn && ob._alpn.length) tls.alpn = ob._alpn
    if (ob._fingerprint) tls.fingerprint = ob._fingerprint
    if (ob._allowInsecure) tls.allowInsecure = true
    ss.tlsSettings = tls
  }
  if (ob._securityMode === 'reality') {
    const reality = {}
    if (ob._realitySni) reality.serverName = ob._realitySni
    if (ob._realityPublicKey) reality.password = ob._realityPublicKey
    if (ob._realityShortId) reality.shortId = ob._realityShortId
    if (ob._realityFingerprint) reality.fingerprint = ob._realityFingerprint
    if (ob._realitySpiderX) reality.spiderX = ob._realitySpiderX
    ss.realitySettings = reality
  }
  if (ob._network === 'ws') {
    ss.wsSettings = { path: ob._wsPath || '/', host: ob._wsHost || undefined }
  }
  if (ob._network === 'grpc') {
    ss.grpcSettings = { serviceName: ob._grpcServiceName || '' }
  }
  return ss
}

async function loadInboundTags() {
  try {
    const data = await client.get('/inbound-configs', { params: { core_type: props.coreType || 'xray' } })
    inboundTags.value = (data || []).map(i => i.tag).filter(Boolean)
  } catch {}
}

async function loadOutbounds() {
  try {
    const data = await client.get('/outbound-configs', { params: { core_type: props.coreType || 'xray' } })
    form.value = (data || []).map(item => {
      try {
        const cfg = typeof item.config_json === 'string' ? JSON.parse(item.config_json) : (item.config_json || {})
        return { ...parseOutbound(cfg), _id: item.id, _enabled: item.enabled, _isDefault: item.is_default }
      } catch (e) { console.error('parseOutbound error:', e, item); return null }
    }).filter(Boolean)
  } catch (e) { console.error('loadOutbounds error:', e); form.value = [] }
}

function genTag(protocol) {
  return 'outbound-' + protocol + '-' + Math.random().toString(36).substring(2, 8)
}

function genEmail() {
  return Math.random().toString(36).substring(2, 8) + '@xray.com'
}

function showAdd() {
  editIdx.value = -1
  const d = getDefaultOutbound()
  d.tag = genTag(d.protocol)
  Object.assign(rf, d)
  activeTab.value = 'basic'
  dlg.value = true
}

function showEdit(i) {
  editIdx.value = i
  const ob = form.value[i]
  Object.assign(rf, JSON.parse(JSON.stringify(ob)))
  activeTab.value = 'basic'
  dlg.value = true
}

async function saveOutbound() {
  const outbound = buildOutbound(rf)
  const payload = {
    core_type: props.coreType || 'xray',
    remark: rf.tag || outbound.tag || '',
    protocol: outbound.protocol,
    tag: outbound.tag || 'outbound-' + Date.now(),
    config_json: outbound,
    enabled: true,
    is_default: false,
  }
  try {
    if (editIdx.value >= 0 && form.value[editIdx.value]._id) {
      await client.put(`/outbound-configs/${form.value[editIdx.value]._id}`, payload)
    } else {
      await client.post('/outbound-configs', payload)
    }
    ElMessage.success(t('common.success'))
    dlg.value = false
    loadOutbounds()
  } catch (e) { ElMessage.error(t('common.failed') + ': ' + (e.message || e)) }
}

async function removeOutbound(i) {
  const ob = form.value[i]
  if (ob._id) {
    try { await client.delete(`/outbound-configs/${ob._id}`) } catch {}
  }
  form.value.splice(i, 1)
  loadOutbounds()
}

async function toggleEnabled(i) {
  const ob = form.value[i]
  if (!ob._id) return
  try {
    const outbound = buildOutbound(ob)
    await client.put(`/outbound-configs/${ob._id}`, {
      remark: ob.tag || outbound.tag || '',
      protocol: outbound.protocol,
      tag: outbound.tag || ob.tag,
      config_json: outbound,
      enabled: ob._enabled,
      is_default: ob._isDefault || false,
    })
    ElMessage.success(ob._enabled ? t('common.enabled') : t('common.disabled'))
  } catch (e) { ElMessage.error(t('common.failed')) }
}

function onProtocolChange() {
  rf.tag = genTag(rf.protocol)
  if (rf.protocol === 'freedom') rf._domainStrategy = 'AsIs'
  if (rf.protocol === 'wireguard') {
    rf._wgPeers = []
  }
}

function genNoisePacket(n) {
  if (n.type === 'rand') {
    const len = Math.floor(Math.random() * 100) + 50
    n.packet = String(len)
    n.delay = '10-20'
  } else if (n.type === 'str') {
    const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789'
    let r = ''
    for (let i = 0; i < 32; i++) r += chars.charAt(Math.floor(Math.random() * chars.length))
    n.packet = r
    n.delay = '10-20'
  } else if (n.type === 'base64') {
    const bytes = new Uint8Array(48)
    crypto.getRandomValues(bytes)
    n.packet = btoa(String.fromCharCode(...bytes))
    n.delay = '5-15'
  }
}

function addNoise() {
  const n = { type: 'rand', packet: '100', delay: '10-20' }
  genNoisePacket(n)
  rf._freedomNoises.push(n)
}

function addFinalRule() {
  rf._freedomFinalRules.push({ action: 'block', network: '', port: '', _ipStr: '', blockDelay: '30-90' })
}

function onRuleActionChange(rule) {
  if (rule.action === 'block' && !rule.blockDelay) rule.blockDelay = '30-90'
  if (rule.action === 'allow') rule.blockDelay = ''
}

async function genWgKey() {
  try {
    const data = await client.post('/binaries/wg-key?core_type=' + (props.coreType || 'xray'))
    if (data && data.privateKey) {
      rf._wgSecretKey = data.privateKey
      rf._wgPublicKey = data.publicKey || ''
      ElMessage.success(t('common.success'))
    }
  } catch (e) { ElMessage.error(t('common.failed') + ': ' + (e.message || e)) }
}



async function importWarp() {
  try {
    // Check WARP status
    let status
    try { status = await client.get('/warp/status') } catch { status = null }
    // Register if not registered
    if (!status || !status.registered) {
      ElMessage.info(t('protoCenter.outbounds.warpRegistering'))
      status = await client.post('/warp/register')
      if (!status || !status.registered) {
        ElMessage.error(t('protoCenter.outbounds.warpRegisterFailed'))
        return
      }
    }
    // Fill form from status data
    rf.tag = 'warp-' + Math.random().toString(36).substring(2, 6)
    rf._wgSecretKey = status.private_key || ''
    // Parse reserved: "[206,167,159]" → "206,167,159"
    let reserved = status.reserved || ''
    if (reserved.startsWith('[')) {
      try { reserved = JSON.parse(reserved).join(', ') } catch {}
    }
    rf._wgReservedStr = reserved
    rf._wgMtu = 1280
    // Import addresses
    rf._wgAddressIPv4 = status.address_ipv4 ? status.address_ipv4 + '/32' : ''
    rf._wgAddressIPv6 = status.address_ipv6 ? status.address_ipv6 + '/128' : ''
    rf._wgPeers = [{
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


watch(() => props.data, loadOutbounds, { deep: true })
onMounted(() => { loadOutbounds(); loadInboundTags() })
</script>
<style scoped>
.outbound-item { background: var(--el-fill-color-light); border-radius: 6px; padding: 12px; margin-bottom: 8px; }
.hint { font-size: 12px; color: #909399; margin-top: 2px; }
:deep(.el-dialog__body) { max-height: calc(100vh - 200px); overflow-y: auto; }
</style>
