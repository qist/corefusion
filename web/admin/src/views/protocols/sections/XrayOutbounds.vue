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
          <el-tag v-if="ob._isDefault" size="small" type="warning">{{ t('protoCenter.outbounds.default') }}</el-tag>
          <el-tag v-if="ob._server" size="small" type="info">{{ ob._server }}:{{ ob._port }}</el-tag>
        </div>
        <div>
          <el-button size="small" text :disabled="i === 0" @click="moveOutbound(i, -1)">↑</el-button>
          <el-button size="small" text :disabled="i === form.length - 1" @click="moveOutbound(i, 1)">↓</el-button>
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
                <el-option label="Hysteria" value="hysteria" />
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
                  <el-input v-model="rf._freedomRedirect" :placeholder="t('protoCenter.outbounds.freedomRedirectHint')" @change="emit" />
                  <div class="hint">{{ t('protoCenter.outbounds.redirectHint') }}</div>
                </el-form-item>
                <el-form-item :label="t('protoCenter.outbounds.userLevel')">
                  <el-input-number v-model="rf._freedomUserLevel" :min="0" />
                </el-form-item>
                <el-form-item :label="t('protoCenter.outbounds.proxyProtocol')">
                  <el-select v-model="rf._freedomProxyProtocol" style="width:100%">
                    <el-option :label="t('protoCenter.outbounds.proxyProtocolOff')" :value="0" /><el-option label="v1 (1)" :value="1" /><el-option label="v2 (2)" :value="2" />
                  </el-select>
                </el-form-item>

                <!-- Fragment -->
                <el-divider content-position="left">{{ t('protoCenter.outbounds.fragment') }}</el-divider>
                <el-form-item :label="t('protoCenter.outbounds.fragmentEnable')">
                  <el-switch v-model="rf._freedomFragmentEnabled" />
                </el-form-item>
                <template v-if="rf._freedomFragmentEnabled">
                  <el-form-item label="Packets">
                    <el-input v-model="rf._freedomFragmentPackets" :placeholder="t('protoCenter.outbounds.fragmentPacketsHint')" />
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
                      <el-input v-model="n.packet" :placeholder="n.type === 'rand' ? t('protoCenter.outbounds.noiseRandHint') : n.type === 'base64' ? t('protoCenter.outbounds.noiseBase64Hint') : t('protoCenter.outbounds.noiseStrHint')" style="flex:1" />
                      <el-button size="small" @click="genNoisePacket(n)">{{ t('protoCenter.outbounds.generate') }}</el-button>
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
                        <el-option :label="t('protoCenter.outbounds.allNetworks')" value="" />
                        <el-option label="tcp" value="tcp" /><el-option label="udp" value="udp" /><el-option label="tcp,udp" value="tcp,udp" />
                      </el-select>
                      <el-input v-model="rule.port" placeholder="22,25,465,587" style="flex:1" />
                      <el-button size="small" type="danger" text @click="rf._freedomFinalRules.splice(ri,1)">X</el-button>
                    </div>
                    <el-input v-model="rule._ipStr" type="textarea" :rows="1" :placeholder="t('protoCenter.outbounds.finalRuleIpHint')" style="margin-bottom:6px" />
                    <div style="display:flex;gap:8px;align-items:center">
                      <span style="font-size:12px;color:#909399;width:80px">blockDelay:</span>
                      <el-input v-model="rule.blockDelay" placeholder="30-90" style="width:120px" />
                      <span style="font-size:12px;color:#909399">{{ t('protoCenter.outbounds.finalRuleBlockDelay') }}</span>
                    </div>
                  </div>
                  <el-button size="small" @click="addFinalRule()">{{ t('protoCenter.outbounds.addRule') }}</el-button>
                </template>
              </template>
            </template>
            <template v-if="rf.protocol === 'blackhole'">
              <el-form-item :label="t('protoCenter.outbounds.advancedSettings')">
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
                  <el-option :label="t('protoCenter.outbounds.default')" value="" /><el-option label="tcp" value="tcp" /><el-option label="udp" value="udp" />
                </el-select>
              </el-form-item>
              <el-form-item label="Rewrite Address">
                <el-input v-model="rf._dnsRewriteAddress" :placeholder="t('protoCenter.outbounds.dnsRewriteHint')" />
              </el-form-item>
              <el-form-item label="Rewrite Port">
                <el-input-number v-model="rf._dnsRewritePort" :min="0" :max="65535" />
                <div class="hint">{{ t('protoCenter.outbounds.dnsPortHint') }}</div>
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
                  <el-input v-model="rule.qType" :placeholder="t('protoCenter.outbounds.dnsQtypeHint')" style="width:150px" />
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
                <div class="hint">{{ t('protoCenter.outbounds.reverseTagHint') }}</div>
              </el-form-item>
            </template>
          </el-form>
        </el-tab-pane>

        <!-- Protocol Tab -->
        <el-tab-pane v-if="['vmess','vless','trojan','shadowsocks','socks','http','hysteria'].includes(rf.protocol)" :label="t('protoCenter.outbounds.protocolSettings')" name="protocol">
          <el-form label-width="120px" label-position="left">
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
              <template v-if="rf.protocol === 'socks' || rf.protocol === 'http'">
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
        <el-tab-pane v-if="['vmess','vless','trojan','shadowsocks','hysteria'].includes(rf.protocol)" :label="t('protoCenter.outbounds.stream')" name="stream">
          <el-form label-width="140px" label-position="left">
            <el-form-item :label="t('protoCenter.outbounds.network')">
              <el-select v-model="rf._network" style="width:100%" :disabled="rf.protocol === 'hysteria'" clearable>
                <el-option label="Raw (TCP)" value="raw" />
                <el-option label="mKCP" value="kcp" />
                <el-option label="WebSocket" value="ws" />
                <el-option label="gRPC" value="grpc" />
                <el-option label="HTTPUpgrade" value="httpupgrade" />
                <el-option v-if="rf.protocol === 'vless'" label="XHTTP" value="xhttp" />
                <el-option v-if="rf.protocol === 'hysteria'" label="Hysteria" value="hysteria" />
              </el-select>
            </el-form-item>
            <el-form-item :label="t('protoCenter.outbounds.securityMode')">
              <el-select v-model="rf._securityMode" style="width:100%">
                <el-option v-if="rf.protocol !== 'hysteria'" label="None" value="none" />
                <el-option label="TLS" value="tls" />
                <el-option v-if="['vless','trojan'].includes(rf.protocol)" label="Reality" value="reality" />
              </el-select>
              <div v-if="rf.protocol === 'hysteria'" class="hint">Hysteria requires TLS</div>
            </el-form-item>

            <!-- TLS Settings -->
            <template v-if="rf._securityMode === 'tls'">
              <el-divider content-position="left">TLS</el-divider>
              <el-form-item label="Server Name">
                <el-input v-model="rf._sni" placeholder="example.com" />
              </el-form-item>
              <el-form-item label="Verify Peer Cert">
                <el-input v-model="rf._tlsVerifyPeerCertByName" placeholder="{{ t('protoCenter.outbounds.tlsVerifyHint') }}" />
                <div class="hint">{{ t('protoCenter.outbounds.tlsVerifyDesc') }}</div>
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
              <el-form-item label="Disable System Root">
                <el-switch v-model="rf._tlsDisableSystemRoot" />
              </el-form-item>
              <el-form-item label="ECH Config List">
                <el-input v-model="rf._tlsEchConfigList" :placeholder="t('protoCenter.outbounds.tlsEchHint')" />
              </el-form-item>
              <el-form-item label="Master Key Log">
                <el-input v-model="rf._tlsMasterKeyLog" :placeholder="t('protoCenter.outbounds.tlsMasterKeyHint')" />
              </el-form-item>
            </template>

            <!-- Reality Settings -->
            <template v-if="rf._securityMode === 'reality'">
              <el-divider content-position="left">REALITY</el-divider>
              <el-form-item label="Server Name">
                <el-input v-model="rf._realitySni" placeholder="example.com" />
                <div class="hint">{{ t('protoCenter.outbounds.realitySniHint') }}</div>
              </el-form-item>
              <el-form-item label="Password (Public Key)">
                <el-input v-model="rf._realityPublicKey" />
              </el-form-item>
              <el-form-item label="Short ID">
                <el-input v-model="rf._realityShortId" :placeholder="t('protoCenter.outbounds.realityShortIdHint')" />
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
              <el-form-item label="mldsa65Verify">
                <el-input v-model="rf._realityMldsa65Verify" type="textarea" :rows="2" :placeholder="t('protoCenter.outbounds.realityMldsa65Hint')" />
                <div class="hint">{{ t('protoCenter.outbounds.realityMldsa65Hint') }}</div>
              </el-form-item>
            </template>

            <!-- Raw TCP Settings -->
            <template v-if="rf._network === 'raw'">
              <el-divider content-position="left">RAW Settings</el-divider>
              <el-form label-width="140px" label-position="left">
                <el-form-item label="Header Type">
                  <el-select v-model="rf._raw.headerType" style="width:100%" @change="onRawHeaderChange">
                    <el-option label="none" value="none" /><el-option label="http" value="http" />
                  </el-select>
                </el-form-item>
              </el-form>
              <template v-if="rf._raw.headerType === 'http'">
                <el-divider content-position="left">HTTP Request</el-divider>
                <el-form label-width="140px" label-position="left" size="small">
                  <el-form-item label="Version">
                    <el-input v-model="rf._raw.request.version" placeholder="1.1" />
                  </el-form-item>
                  <el-form-item label="Method">
                    <el-input v-model="rf._raw.request.method" placeholder="GET" />
                  </el-form-item>
                  <el-form-item label="Path">
                    <el-input v-model="rf._raw.request._pathsStr" placeholder="/" />
                    <div class="hint">{{ t('protoCenter.outbounds.rawReqPathsHint') }}</div>
                  </el-form-item>
                  <el-form-item label="Host Headers">
                    <div v-for="(h, i) in rf._raw.request._headers" :key="i" style="display:flex;gap:8px;margin-bottom:4px">
                      <el-input v-model="h.key" placeholder="Header name" style="width:30%" />
                      <el-input v-model="h.value" placeholder="Header value" style="width:50%" />
                      <el-button size="small" type="danger" text @click="rf._raw.request._headers.splice(i,1);emit()">X</el-button>
                    </div>
                    <el-button size="small" @click="rf._raw.request._headers.push({key:'Host',value:''})">+ Add Header</el-button>
                  </el-form-item>
                </el-form>
                <el-divider content-position="left">HTTP Response</el-divider>
                <el-form label-width="140px" label-position="left" size="small">
                  <el-form-item label="Version">
                    <el-input v-model="rf._raw.response.version" placeholder="1.1" />
                  </el-form-item>
                  <el-form-item label="Status">
                    <el-input v-model="rf._raw.response.status" placeholder="200" />
                  </el-form-item>
                  <el-form-item label="Reason">
                    <el-input v-model="rf._raw.response.reason" placeholder="OK" />
                  </el-form-item>
                  <el-form-item label="Response Headers">
                    <div v-for="(h, i) in rf._raw.response._headers" :key="i" style="display:flex;gap:8px;margin-bottom:4px">
                      <el-input v-model="h.key" placeholder="Header name" style="width:30%" />
                      <el-input v-model="h.value" placeholder="Header value" style="width:50%" />
                      <el-button size="small" type="danger" text @click="rf._raw.response._headers.splice(i,1);emit()">X</el-button>
                    </div>
                    <el-button size="small" @click="rf._raw.response._headers.push({key:'',value:''})">+ Add Header</el-button>
                  </el-form-item>
                </el-form>
              </template>
            </template>

            <!-- mKCP Settings -->
            <template v-if="rf._network === 'kcp'">
              <el-divider content-position="left">mKCP</el-divider>
              <el-form label-width="160px" label-position="left">
                <el-form-item label="MTU">
                  <el-input-number v-model="rf._kcpMtu" :min="576" :max="1460" @change="emit" />
                  <div class="hint">{{ t('protoCenter.outbounds.kcpMtuHint') }}</div>
                </el-form-item>
                <el-form-item label="TTI (ms)">
                  <el-input-number v-model="rf._kcpTti" :min="10" :max="100" @change="emit" />
                  <div class="hint">{{ t('protoCenter.outbounds.kcpTtiHint') }}</div>
                </el-form-item>
                <el-form-item :label="t('protoCenter.outbounds.kcpUplinkCap')">
                  <el-input-number v-model="rf._kcpUplinkCapacity" :min="0" @change="emit" />
                  <div class="hint">{{ t('protoCenter.outbounds.kcpCapHint') }}</div>
                </el-form-item>
                <el-form-item :label="t('protoCenter.outbounds.kcpDownlinkCap')">
                  <el-input-number v-model="rf._kcpDownlinkCapacity" :min="0" @change="emit" />
                  <div class="hint">{{ t('protoCenter.outbounds.kcpCapHint') }}</div>
                </el-form-item>
                <el-form-item :label="t('protoCenter.outbounds.kcpCongestion')">
                  <el-switch v-model="rf._kcpCongestion" @change="emit" />
                  <div class="hint">{{ t('protoCenter.outbounds.kcpCongestionHint') }}</div>
                </el-form-item>
                <el-form-item label="Read Buffer Size">
                  <el-input-number v-model="rf._kcpReadBufferSize" :min="1" @change="emit" />
                  <div class="hint">MB</div>
                </el-form-item>
                <el-form-item label="Write Buffer Size">
                  <el-input-number v-model="rf._kcpWriteBufferSize" :min="1" @change="emit" />
                  <div class="hint">MB</div>
                </el-form-item>
              </el-form>
            </template>

            <!-- WebSocket Settings -->
            <template v-if="rf._network === 'ws'">
              <el-divider content-position="left">WebSocket</el-divider>
              <el-form label-width="160px" label-position="left">
                <el-form-item label="Path">
                  <el-input v-model="rf._wsPath" placeholder="/" />
                  <div class="hint">{{ t('protoCenter.outbounds.wsPathHint') }}</div>
                </el-form-item>
                <el-form-item label="Host">
                  <el-input v-model="rf._wsHost" />
                </el-form-item>
                <el-form-item label="Headers">
                  <div v-for="(h, i) in rf._wsHeaders" :key="i" style="display:flex;gap:8px;margin-bottom:4px">
                    <el-input v-model="h.key" placeholder="Header name" style="width:30%" />
                    <el-input v-model="h.value" placeholder="Header value" style="width:50%" />
                    <el-button size="small" type="danger" text @click="rf._wsHeaders.splice(i,1);emit()">X</el-button>
                  </div>
                  <el-button size="small" @click="rf._wsHeaders.push({key:'',value:''})">+ Add Header</el-button>
                </el-form-item>
                <el-form-item label="Heartbeat Period">
                  <el-input-number v-model="rf._wsHeartbeatPeriod" :min="0" @change="emit" />
                  <div class="hint">{{ t('protoCenter.outbounds.wsHeartbeatHint') }}</div>
                </el-form-item>
              </el-form>
            </template>

            <!-- gRPC Settings -->
            <template v-if="rf._network === 'grpc'">
              <el-divider content-position="left">gRPC</el-divider>
              <el-form label-width="160px" label-position="left">
                <el-form-item label="Service Name">
                  <el-input v-model="rf._grpcServiceName" />
                </el-form-item>
                <el-form-item label="Authority">
                  <el-input v-model="rf._grpcAuthority" placeholder="grpc.example.com" />
                  <div class="hint">{{ t('protoCenter.outbounds.grpcAuthorityHint') }}</div>
                </el-form-item>
                <el-form-item label="User Agent">
                  <el-input v-model="rf._grpcUserAgent" placeholder="custom user agent" />
                </el-form-item>
                <el-form-item label="Multi Mode">
                  <el-switch v-model="rf._grpcMultiMode" @change="emit" />
                  <div class="hint">{{ t('protoCenter.outbounds.grpcMultiModeHint') }}</div>
                </el-form-item>
                <el-form-item label="Idle Timeout (s)">
                  <el-input-number v-model="rf._grpcIdleTimeout" :min="0" @change="emit" />
                  <div class="hint">{{ t('protoCenter.outbounds.grpcIdleTimeoutHint') }}</div>
                </el-form-item>
                <el-form-item label="Health Check Timeout">
                  <el-input-number v-model="rf._grpcHealthCheckTimeout" :min="0" @change="emit" />
                  <div class="hint">{{ t('protoCenter.outbounds.grpcHealthTimeoutHint') }}</div>
                </el-form-item>
                <el-form-item label="Permit Without Stream">
                  <el-switch v-model="rf._grpcPermitWithoutStream" @change="emit" />
                </el-form-item>
                <el-form-item label="Initial Windows Size">
                  <el-input-number v-model="rf._grpcInitialWindowsSize" :min="0" @change="emit" />
                  <div class="hint">{{ t('protoCenter.outbounds.grpcWindowsSizeHint') }}</div>
                </el-form-item>
              </el-form>
            </template>

            <!-- HTTPUpgrade Settings -->
            <template v-if="rf._network === 'httpupgrade'">
              <el-divider content-position="left">HTTPUpgrade</el-divider>
              <el-form label-width="160px" label-position="left">
                <el-form-item label="Path">
                  <el-input v-model="rf._httpupgradePath" placeholder="/" />
                  <div class="hint">{{ t('protoCenter.outbounds.wsPathHint') }}</div>
                </el-form-item>
                <el-form-item label="Host">
                  <el-input v-model="rf._httpupgradeHost" />
                </el-form-item>
                <el-form-item label="Headers">
                  <div v-for="(h, i) in rf._httpupgradeHeaders" :key="i" style="display:flex;gap:8px;margin-bottom:4px">
                    <el-input v-model="h.key" placeholder="Header name" style="width:30%" />
                    <el-input v-model="h.value" placeholder="Header value" style="width:50%" />
                    <el-button size="small" type="danger" text @click="rf._httpupgradeHeaders.splice(i,1);emit()">X</el-button>
                  </div>
                  <el-button size="small" @click="rf._httpupgradeHeaders.push({key:'',value:''})">+ Add Header</el-button>
                </el-form-item>
              </el-form>
            </template>

            <!-- XHTTP Settings -->
            <template v-if="rf._network === 'xhttp'">
              <el-divider content-position="left">XHTTP</el-divider>
              <el-form label-width="160px" label-position="left">
                <el-form-item label="Path">
                  <el-input v-model="rf._xhttpPath" placeholder="/" />
                </el-form-item>
                <el-form-item label="Host">
                  <el-input v-model="rf._xhttpHost" />
                </el-form-item>
                <el-form-item label="Mode">
                  <el-select v-model="rf._xhttpMode" style="width:100%" clearable>
                    <el-option label="auto" value="" />
                    <el-option label="packet-up" value="packet-up" />
                    <el-option label="stream-up" value="stream-up" />
                    <el-option label="stream-one" value="stream-one" />
                  </el-select>
                  <div class="hint">{{ t('protoCenter.outbounds.xhttpModeHint') }}</div>
                </el-form-item>
              </el-form>

              <!-- Extra Settings -->
              <el-divider content-position="left">Extra</el-divider>
              <el-form label-width="180px" label-position="left">
                <el-form-item label="x Padding Bytes">
                  <el-input v-model="rf._xhttpPaddingBytes" placeholder="100-1000" @change="emit" />
                </el-form-item>
                <el-form-item v-if="['stream-up','stream-one'].includes(rf._xhttpMode)" label="No GRPC Header">
                  <el-switch v-model="rf._xhttpNoGRPCHeader" @change="emit" />
                </el-form-item>
                <el-form-item label="Headers">
                  <div v-for="(h, i) in rf._xhttpHeaders" :key="i" style="display:flex;gap:8px;margin-bottom:4px">
                    <el-input v-model="h.key" placeholder="Header name" style="width:30%" />
                    <el-input v-model="h.value" placeholder="Header value" style="width:50%" />
                    <el-button size="small" type="danger" text @click="rf._xhttpHeaders.splice(i,1);emit()">X</el-button>
                  </div>
                  <el-button size="small" @click="rf._xhttpHeaders.push({key:'',value:''})">+ Add Header</el-button>
                </el-form-item>
              </el-form>

              <!-- Packet-Up Settings -->
              <template v-if="rf._xhttpMode === 'packet-up'">
                <el-divider content-position="left">Packet-Up</el-divider>
                <el-form label-width="180px" label-position="left">
                  <el-form-item label="Max Each Post Bytes">
                    <el-input-number v-model="rf._xhttpScMaxEachPostBytes" :min="0" @change="emit" />
                  </el-form-item>
                  <el-form-item label="Min Posts Interval (ms)">
                    <el-input-number v-model="rf._xhttpScMinPostsIntervalMs" :min="0" @change="emit" />
                  </el-form-item>
                </el-form>
              </template>

              <!-- Xmux -->
              <el-divider content-position="left">Xmux</el-divider>
              <el-form label-width="180px" label-position="left">
                <el-form-item label="Max Concurrency">
                  <el-input v-model="rf._xhttpXmuxMaxConcurrency" placeholder="16-32" @change="emit" />
                  <div class="hint">{{ t('protoCenter.outbounds.xmuxConcurrencyHint') }}</div>
                </el-form-item>
                <el-form-item label="Max Connections">
                  <el-input v-model="rf._xhttpXmuxMaxConnections" placeholder="0" @change="emit" />
                  <div class="hint">{{ t('protoCenter.outbounds.xmuxConnectionsHint') }}</div>
                </el-form-item>
                <el-form-item label="C Max Reuse Times">
                  <el-input v-model="rf._xhttpXmuxCMaxReuseTimes" placeholder="0" @change="emit" />
                </el-form-item>
                <el-form-item label="H Max Request Times">
                  <el-input v-model="rf._xhttpXmuxHMaxRequestTimes" placeholder="600-900" @change="emit" />
                </el-form-item>
                <el-form-item label="H Max Reusable Secs">
                  <el-input v-model="rf._xhttpXmuxHMaxReusableSecs" placeholder="1800-3000" @change="emit" />
                </el-form-item>
                <el-form-item label="H Keep Alive Period">
                  <el-input-number v-model="rf._xhttpXmuxHKeepAlivePeriod" :min="-1" @change="emit" />
                  <div class="hint">{{ t('protoCenter.outbounds.xmuxKeepAliveHint') }}</div>
                </el-form-item>
              </el-form>

              <!-- Download Settings -->
              <el-divider content-position="left">
                {{ t('protoCenter.outbounds.downloadSettings') }}
                <el-switch v-model="rf._xhttpDownloadEnabled" style="margin-left:12px" />
              </el-divider>
              <template v-if="rf._xhttpDownloadEnabled">
                <el-form label-width="160px" label-position="left">
                  <el-form-item label="Address">
                    <el-input v-model="rf._xhttpDownloadAddress" @change="emit" />
                  </el-form-item>
                  <el-form-item label="Port">
                    <el-input-number v-model="rf._xhttpDownloadPort" :min="1" :max="65535" @change="emit" />
                  </el-form-item>
                  <el-form-item label="Security">
                    <el-select v-model="rf._xhttpDownloadSecurity" style="width:100%" @change="emit">
                      <el-option label="none" value="none" /><el-option label="tls" value="tls" /><el-option label="reality" value="reality" />
                    </el-select>
                  </el-form-item>
                  <!-- TLS -->
                  <template v-if="rf._xhttpDownloadSecurity === 'tls'">
                    <el-form-item label="Server Name">
                      <el-input v-model="rf._xhttpDownloadTlsServerName" @change="emit" />
                    </el-form-item>
                    <el-form-item label="ALPN">
                      <el-select v-model="rf._xhttpDownloadTlsAlpn" style="width:100%" multiple clearable @change="emit">
                        <el-option label="h2" value="h2" /><el-option label="http/1.1" value="http/1.1" /><el-option label="h3" value="h3" />
                      </el-select>
                    </el-form-item>
                    <el-form-item label="Fingerprint">
                      <el-select v-model="rf._xhttpDownloadTlsFingerprint" style="width:100%" clearable @change="emit">
                        <el-option label="chrome" value="chrome" /><el-option label="firefox" value="firefox" />
                        <el-option label="safari" value="safari" /><el-option label="random" value="random" />
                      </el-select>
                    </el-form-item>
                  </template>
                  <!-- REALITY -->
                  <template v-if="rf._xhttpDownloadSecurity === 'reality'">
                    <el-form-item label="Server Name">
                      <el-input v-model="rf._xhttpDownloadRealitySni" @change="emit" />
                    </el-form-item>
                    <el-form-item label="Password (Public Key)">
                      <el-input v-model="rf._xhttpDownloadRealityPassword" @change="emit" />
                    </el-form-item>
                    <el-form-item label="Short ID">
                      <el-input v-model="rf._xhttpDownloadRealityShortId" @change="emit" />
                    </el-form-item>
                    <el-form-item label="Fingerprint">
                      <el-select v-model="rf._xhttpDownloadRealityFingerprint" style="width:100%" @change="emit">
                        <el-option label="chrome" value="chrome" /><el-option label="firefox" value="firefox" />
                        <el-option label="safari" value="safari" /><el-option label="random" value="random" />
                      </el-select>
                    </el-form-item>
                  </template>
                  <el-form-item label="Path">
                    <el-input v-model="rf._xhttpDownloadPath" :placeholder="t('protoCenter.outbounds.downloadPathHint')" @change="emit" />
                  </el-form-item>
                </el-form>
                <!-- Download Xmux -->
                <el-divider content-position="left" style="margin:8px 0">Download Xmux</el-divider>
                <el-form label-width="180px" label-position="left">
                  <el-form-item label="Max Concurrency">
                    <el-input v-model="rf._xhttpDownloadXmuxMaxConcurrency" placeholder="16-32" @change="emit" />
                  </el-form-item>
                  <el-form-item label="Max Connections">
                    <el-input v-model="rf._xhttpDownloadXmuxMaxConnections" placeholder="0" @change="emit" />
                  </el-form-item>
                  <el-form-item label="C Max Reuse Times">
                    <el-input v-model="rf._xhttpDownloadXmuxCMaxReuseTimes" placeholder="0" @change="emit" />
                  </el-form-item>
                  <el-form-item label="H Max Request Times">
                    <el-input v-model="rf._xhttpDownloadXmuxHMaxRequestTimes" placeholder="600-900" @change="emit" />
                  </el-form-item>
                  <el-form-item label="H Max Reusable Secs">
                    <el-input v-model="rf._xhttpDownloadXmuxHMaxReusableSecs" placeholder="1800-3000" @change="emit" />
                  </el-form-item>
                  <el-form-item label="H Keep Alive Period">
                    <el-input-number v-model="rf._xhttpDownloadXmuxHKeepAlivePeriod" :min="-1" @change="emit" />
                  </el-form-item>
                </el-form>
              </template>
            </template>

            <!-- Hysteria Settings -->
            <template v-if="rf._network === 'hysteria' || rf.protocol === 'hysteria'">
              <el-divider content-position="left">Hysteria Settings</el-divider>
              <el-form label-width="160px" label-position="left">
                <el-form-item label="Version">
                  <el-input-number v-model="rf._hysteriaVersion" :min="2" :max="2" />
                  <div class="hint">{{ t('protoCenter.outbounds.hysteriaVersionHint') }}</div>
                </el-form-item>
                <el-form-item label="Auth">
                  <el-input v-model="rf._hysteriaAuth" :placeholder="t('protoCenter.outbounds.hysteriaAuthHint')" />
                </el-form-item>
              </el-form>
            </template>
          </el-form>
        </el-tab-pane>

        <!-- Sockopt Tab -->
        <el-tab-pane v-if="['vmess','vless','trojan','shadowsocks','hysteria'].includes(rf.protocol)" :label="t('protoCenter.sockopt.title')" name="sockopt">
          <div style="margin-bottom:16px;display:flex;align-items:center;gap:12px">
            <span style="font-weight:500">{{ t('protoCenter.sockopt.enable') }}</span>
            <el-switch v-model="rf._sockoptEnabled" />
            <span v-if="!rf._sockoptEnabled" style="font-size:12px;color:#909399">{{ t('protoCenter.sockopt.enableHint') }}</span>
          </div>
          <template v-if="rf._sockoptEnabled">
            <el-form label-width="180px" label-position="left">
              <el-form-item label="Mark"><el-input-number v-model="rf._sockoptMark" :min="0" @change="emit" /></el-form-item>
              <el-form-item label="TCP Fast Open"><el-switch v-model="rf._sockoptTcpFastOpen" @change="emit" /></el-form-item>
              <el-form-item label="TProxy">
                <el-select v-model="rf._sockoptTproxy" style="width:100%" clearable @change="emit">
                  <el-option label="off" value="off" /><el-option label="redirect" value="redirect" /><el-option label="tproxy" value="tproxy" />
                </el-select>
              </el-form-item>
              <el-form-item :label="t('protoCenter.sockopt.domainStrategy')">
                <el-select v-model="rf._sockoptDomainStrategy" style="width:100%" clearable @change="emit">
                  <el-option label="AsIs" value="AsIs" /><el-option label="UseIP" value="UseIP" />
                  <el-option label="UseIPv4" value="UseIPv4" /><el-option label="UseIPv6" value="UseIPv6" />
                  <el-option label="ForceIP" value="ForceIP" /><el-option label="ForceIPv4" value="ForceIPv4" /><el-option label="ForceIPv6" value="ForceIPv6" />
                </el-select>
              </el-form-item>
              <el-form-item :label="t('protoCenter.sockopt.interface')">
                <el-input v-model="rf._sockoptInterface" @change="emit" />
              </el-form-item>
              <el-form-item label="V6 Only"><el-switch v-model="rf._sockoptV6only" @change="emit" /></el-form-item>
              <el-form-item :label="t('protoCenter.sockopt.tcpKeepAliveIdle')"><el-input-number v-model="rf._sockoptTcpKeepAliveIdle" :min="0" @change="emit" /></el-form-item>
              <el-form-item :label="t('protoCenter.sockopt.tcpKeepAliveInterval')"><el-input-number v-model="rf._sockoptTcpKeepAliveInterval" :min="0" @change="emit" /></el-form-item>
              <el-form-item :label="t('protoCenter.sockopt.tcpUserTimeout')"><el-input-number v-model="rf._sockoptTcpUserTimeout" :min="0" @change="emit" /></el-form-item>
              <el-form-item :label="t('protoCenter.sockopt.tcpcongestion')">
                <el-select v-model="rf._sockoptTcpcongestion" style="width:100%" clearable @change="emit">
                  <el-option label="(default)" value="" /><el-option label="bbr" value="bbr" /><el-option label="cubic" value="cubic" /><el-option label="reno" value="reno" />
                </el-select>
              </el-form-item>
              <el-form-item :label="t('protoCenter.sockopt.tcpMaxSeg')"><el-input-number v-model="rf._sockoptTcpMaxSeg" :min="0" @change="emit" /></el-form-item>
              <el-form-item :label="t('protoCenter.sockopt.tcpWindowClamp')"><el-input-number v-model="rf._sockoptTcpWindowClamp" :min="0" @change="emit" /></el-form-item>
              <el-form-item label="TCP MPTCP"><el-switch v-model="rf._sockoptTcpMptcp" @change="emit" /></el-form-item>
              <el-form-item label="Accept Proxy Protocol"><el-switch v-model="rf._sockoptAcceptProxyProtocol" @change="emit" /></el-form-item>
              <el-form-item :label="t('protoCenter.sockopt.dialerProxy')">
                <el-input v-model="rf._sockoptDialerProxy" @change="emit" />
              </el-form-item>
              <el-form-item :label="t('protoCenter.sockopt.addressPortStrategy')">
                <el-select v-model="rf._sockoptAddressPortStrategy" style="width:100%" clearable @change="emit">
                  <el-option label="(none)" value="" /><el-option label="SrvPortOnly" value="SrvPortOnly" />
                  <el-option label="SrvAddressOnly" value="SrvAddressOnly" /><el-option label="SrvPortAndAddress" value="SrvPortAndAddress" />
                  <el-option label="TxtPortOnly" value="TxtPortOnly" /><el-option label="TxtAddressOnly" value="TxtAddressOnly" />
                  <el-option label="TxtPortAndAddress" value="TxtPortAndAddress" />
                </el-select>
              </el-form-item>
              <el-form-item :label="t('protoCenter.sockopt.trustedXff')">
                <el-input v-model="rf._sockoptTrustedXffStr" type="textarea" :rows="2" :placeholder="t('protoCenter.sockopt.trustedXffHint')" @change="emit" />
              </el-form-item>
            </el-form>

            <!-- Happy Eyeballs -->
            <el-divider content-position="left">
              {{ t('protoCenter.sockopt.happyEyeballs') }}
              <el-switch v-model="rf._sockoptShowHappyEyeballs" style="margin-left:12px" />
            </el-divider>
            <el-form v-if="rf._sockoptShowHappyEyeballs" label-width="180px" label-position="left">
              <el-form-item :label="t('protoCenter.sockopt.tryDelayMs')">
                <el-input-number v-model="rf._sockoptHappyEyeballsTryDelayMs" :min="1" @change="emit" />
                <div class="hint">{{ t('protoCenter.sockopt.tryDelayMsHint') }}</div>
              </el-form-item>
              <el-form-item :label="t('protoCenter.sockopt.prioritizeIPv6')">
                <el-switch v-model="rf._sockoptHappyEyeballsPrioritizeIPv6" @change="emit" />
                <div class="hint">{{ t('protoCenter.sockopt.prioritizeIPv6Hint') }}</div>
              </el-form-item>
              <el-form-item :label="t('protoCenter.sockopt.interleave')">
                <el-input-number v-model="rf._sockoptHappyEyeballsInterleave" :min="1" @change="emit" />
                <div class="hint">{{ t('protoCenter.sockopt.interleaveHint') }}</div>
              </el-form-item>
              <el-form-item :label="t('protoCenter.sockopt.maxConcurrentTry')">
                <el-input-number v-model="rf._sockoptHappyEyeballsMaxConcurrentTry" :min="1" @change="emit" />
                <div class="hint">{{ t('protoCenter.sockopt.maxConcurrentTryHint') }}</div>
              </el-form-item>
            </el-form>

            <!-- Custom Sockopt -->
            <el-divider content-position="left">
              {{ t('protoCenter.sockopt.customSockopt') }}
              <el-switch v-model="rf._sockoptShowCustomSockopt" style="margin-left:12px" />
            </el-divider>
            <template v-if="rf._sockoptShowCustomSockopt">
              <div style="font-size:12px;color:#909399;margin-bottom:8px">{{ t('protoCenter.sockopt.customSockoptHint') }}</div>
              <el-form label-width="180px" label-position="left">
                <div v-for="(cs, i) in rf._sockoptCustomSockopt" :key="i" style="border:1px solid #e4e7ed;border-radius:4px;padding:8px;margin-bottom:8px">
                  <div style="display:flex;gap:8px;margin-bottom:6px">
                    <el-select v-model="cs.system" style="flex:1" clearable filterable allow-create :placeholder="t('protoCenter.sockopt.customSystem')">
                      <el-option label="linux" value="linux" /><el-option label="windows" value="windows" />
                      <el-option label="darwin" value="darwin" /><el-option label="android" value="android" />
                    </el-select>
                    <el-select v-model="cs.network" style="flex:1" clearable filterable allow-create :placeholder="t('protoCenter.sockopt.customNetwork')">
                      <el-option label="tcp" value="tcp" /><el-option label="tcp4" value="tcp4" /><el-option label="tcp6" value="tcp6" />
                      <el-option label="udp" value="udp" /><el-option label="udp4" value="udp4" /><el-option label="udp6" value="udp6" />
                    </el-select>
                    <el-select v-model="cs.type" style="width:80px">
                      <el-option label="str" value="str" /><el-option label="int" value="int" />
                    </el-select>
                    <el-button size="small" type="danger" text @click="rf._sockoptCustomSockopt.splice(i,1);emit()">X</el-button>
                  </div>
                  <div style="display:flex;gap:8px">
                    <el-input v-model="cs.level" :placeholder="t('protoCenter.sockopt.customLevelHint')" style="flex:1" />
                    <el-input v-model="cs.opt" :placeholder="t('protoCenter.sockopt.customOptHint')" style="flex:1" />
                    <el-input v-model="cs.value" :placeholder="cs.type === 'int' ? '12345' : 'bbr'" style="flex:1" />
                  </div>
                </div>
                <el-button size="small" @click="rf._sockoptCustomSockopt.push({system:'',network:'',type:'str',level:'6',opt:'',value:''})">+ Add</el-button>
              </el-form>
            </template>
          </template>
        </el-tab-pane>

        <!-- FinalMask Tab -->
        <el-tab-pane v-if="['vmess','vless','trojan','shadowsocks','hysteria'].includes(rf.protocol)" :label="t('protoCenter.finalmask.title')" name="finalmask">
          <div style="margin-bottom:16px;display:flex;align-items:center;gap:12px">
            <span style="font-weight:500">{{ t('protoCenter.finalmask.enable') }}</span>
            <el-switch v-model="rf._finalmaskEnabled" />
          </div>
          <template v-if="rf._finalmaskEnabled">
            <!-- TCP Mask -->
            <el-divider content-position="left">{{ t('protoCenter.finalmask.tcpMask') }}</el-divider>
            <div class="hint" style="margin-bottom:8px">{{ t('protoCenter.finalmask.tcpMaskHint') }}</div>
            <div v-for="(entry, i) in rf._finalmaskTcp" :key="'tcp-'+i" style="border:1px solid #e4e7ed;border-radius:6px;padding:12px;margin-bottom:12px">
              <div style="display:flex;gap:8px;align-items:center;margin-bottom:8px">
                <el-select v-model="entry.type" style="flex:1" @change="onOutboundMaskTypeChange(entry, 'tcp')">
                  <el-option label="(none)" value="" />
                  <el-option label="fragment" value="fragment" />
                  <el-option label="sudoku" value="sudoku" />
                  <el-option label="header-custom" value="header-custom" />
                </el-select>
                <el-button size="small" type="danger" text @click="rf._finalmaskTcp.splice(i,1)">X</el-button>
              </div>
              <template v-if="entry.type === 'fragment'">
                <el-form label-width="140px" label-position="left" size="small">
                  <el-form-item :label="t('protoCenter.finalmask.packets')"><el-input v-model="entry.settings.packets" placeholder="tlshello or 1-3" @change="emit" /></el-form-item>
                  <el-form-item :label="t('protoCenter.finalmask.length')"><el-input v-model="entry.settings._lengthsStr" type="textarea" :rows="2" placeholder="3-5&#10;6-8&#10;10-20" @change="emit" /></el-form-item>
                  <el-form-item :label="t('protoCenter.finalmask.delay')"><el-input v-model="entry.settings._delaysStr" type="textarea" :rows="2" placeholder="10-20" @change="emit" /></el-form-item>
                  <el-form-item label="Max Split"><el-input v-model="entry.settings.maxSplit" placeholder="3-6" @change="emit" /></el-form-item>
                </el-form>
              </template>
              <template v-if="entry.type === 'sudoku'">
                <el-form label-width="140px" label-position="left" size="small">
                  <el-form-item :label="t('protoCenter.finalmask.password')">
                    <el-input v-model="entry.settings.password" style="width:100%" @change="emit" />
                  </el-form-item>
                  <el-form-item label="Padding Min"><el-input-number v-model="entry.settings.paddingMin" :min="0" @change="emit" /></el-form-item>
                  <el-form-item label="Padding Max"><el-input-number v-model="entry.settings.paddingMax" :min="0" @change="emit" /></el-form-item>
                </el-form>
              </template>
              <template v-if="entry.type === 'header-custom'">
                <div class="hint" style="margin-bottom:4px">clients / servers / errors</div>
                <el-input v-model="entry._json" type="textarea" :rows="4" placeholder='{"clients":[[]],"servers":[[]],"errors":[[]]}' @change="emit" />
              </template>
            </div>
            <el-button size="small" @click="rf._finalmaskTcp.push({type:'',settings:{},_json:''})">{{ t('protoCenter.finalmask.addTcpMask') }}</el-button>

            <!-- UDP Mask -->
            <el-divider content-position="left">{{ t('protoCenter.finalmask.udpMask') }}</el-divider>
            <div class="hint" style="margin-bottom:8px">{{ t('protoCenter.finalmask.udpMaskHint') }}</div>
            <div v-for="(entry, i) in rf._finalmaskUdp" :key="'udp-'+i" style="border:1px solid #e4e7ed;border-radius:6px;padding:12px;margin-bottom:12px">
              <div style="display:flex;gap:8px;align-items:center;margin-bottom:8px">
                <el-select v-model="entry.type" style="flex:1" @change="onOutboundMaskTypeChange(entry, 'udp')">
                  <el-option label="(none)" value="" />
                  <el-option label="header-custom" value="header-custom" />
                  <el-option label="mkcp-legacy" value="mkcp-legacy" />
                  <el-option label="noise" value="noise" />
                  <el-option label="salamander" value="salamander" />
                  <el-option label="sudoku" value="sudoku" />
                  <el-option label="xdns" value="xdns" />
                  <el-option label="xicmp" value="xicmp" />
                  <el-option label="realm" value="realm" />
                </el-select>
                <el-button size="small" type="danger" text @click="rf._finalmaskUdp.splice(i,1)">X</el-button>
              </div>
              <!-- salamander -->
              <template v-if="entry.type === 'salamander'">
                <el-form label-width="140px" label-position="left" size="small">
                  <el-form-item :label="t('protoCenter.finalmask.password')">
                    <el-input v-model="entry.settings.password" style="width:100%" @change="emit" />
                  </el-form-item>
                  <el-form-item :label="t('protoCenter.finalmask.geckoMode')">
                    <el-switch v-model="entry.settings._gecko" @change="onGeckoToggle(entry)" />
                    <div class="hint">{{ t('protoCenter.finalmask.geckoModeHint') }}</div>
                  </el-form-item>
                  <el-form-item v-if="entry.settings._gecko" :label="t('protoCenter.finalmask.packetSize')">
                    <div style="display:flex;gap:8px;align-items:center">
                      <el-input-number v-model="entry.settings._geckoMin" :min="1" :max="2048" size="small" style="width:100px" @change="onGeckoSizeChange(entry)" />
                      <span>-</span>
                      <el-input-number v-model="entry.settings._geckoMax" :min="1" :max="2048" size="small" style="width:100px" @change="onGeckoSizeChange(entry)" />
                    </div>
                    <div class="hint">{{ t('protoCenter.finalmask.packetSizeHint') }}</div>
                  </el-form-item>
                </el-form>
              </template>
              <!-- mkcp-legacy -->
              <template v-if="entry.type === 'mkcp-legacy'">
                <el-form label-width="140px" label-position="left" size="small">
                  <el-form-item label="Header">
                    <el-select v-model="entry.settings.header" style="width:100%" clearable @change="emit">
                      <el-option label="(default AES-128-GCM)" value="" /><el-option label="dns" value="dns" /><el-option label="dtls" value="dtls" />
                      <el-option label="srtp" value="srtp" /><el-option label="utp" value="utp" />
                      <el-option label="wechat" value="wechat" /><el-option label="wireguard" value="wireguard" />
                    </el-select>
                  </el-form-item>
                  <el-form-item label="Value"><el-input v-model="entry.settings.value" @change="emit" /></el-form-item>
                </el-form>
              </template>
              <!-- noise -->
              <template v-if="entry.type === 'noise'">
                <el-form label-width="140px" label-position="left" size="small">
                  <el-form-item label="Reset"><el-input v-model="entry.settings.reset" placeholder="30-60" @change="emit" /></el-form-item>
                </el-form>
                <div v-for="(n, ni) in entry.settings._noise" :key="ni" style="display:flex;gap:6px;margin-bottom:4px;margin-left:12px">
                  <el-input v-model="n.rand" size="small" placeholder="rand" style="flex:1" @change="emit" />
                  <el-input v-model="n.delay" size="small" placeholder="delay" style="width:100px" @change="emit" />
                  <el-button size="small" type="danger" text @click="entry.settings._noise.splice(ni,1);emit()">X</el-button>
                </div>
                <el-button size="small" style="margin-left:12px" @click="entry.settings._noise.push({rand:'1-8192',delay:'10-20'})">+ Noise</el-button>
              </template>
              <!-- header-custom (UDP) -->
              <template v-if="entry.type === 'header-custom'">
                <el-input v-model="entry._json" type="textarea" :rows="3" placeholder='{"client":[],"server":[]}' @change="emit" />
              </template>
              <!-- xdns -->
              <template v-if="entry.type === 'xdns'">
                <el-form label-width="140px" label-position="left" size="small">
                  <el-form-item :label="t('protoCenter.finalmask.domains')">
                    <el-input v-model="entry.settings._domainsStr" type="textarea" :rows="2" placeholder="t.example.com&#10;domain:method (txt/a/aaaa)" @change="emit" />
                    <div class="hint">{{ t('protoCenter.finalmask.domainsHint') }}</div>
                  </el-form-item>
                  <el-form-item :label="t('protoCenter.finalmask.resolvers')">
                    <el-input v-model="entry.settings._resolversStr" type="textarea" :rows="2" placeholder="t.example.com+udp://8.8.8.8:53" @change="emit" />
                    <div class="hint">{{ t('protoCenter.finalmask.resolversHint') }}</div>
                  </el-form-item>
                </el-form>
              </template>
              <!-- xicmp -->
              <template v-if="entry.type === 'xicmp'">
                <el-form label-width="140px" label-position="left" size="small">
                  <el-form-item :label="t('protoCenter.finalmask.dgram')">
                    <el-switch v-model="entry.settings.dgram" @change="emit" />
                    <div class="hint">{{ t('protoCenter.finalmask.dgramHint') }}</div>
                  </el-form-item>
                  <el-form-item :label="t('protoCenter.finalmask.ips')">
                    <el-input v-model="entry.settings._ipsStr" type="textarea" :rows="2" placeholder="IP/CIDR" @change="emit" />
                  </el-form-item>
                </el-form>
              </template>
              <!-- realm -->
              <template v-if="entry.type === 'realm'">
                <el-form label-width="140px" label-position="left" size="small">
                  <el-form-item :label="t('protoCenter.finalmask.realmUrl')">
                    <el-input v-model="entry.settings.url" :placeholder="t('protoCenter.finalmask.realmUrlHint')" @change="emit" />
                  </el-form-item>
                  <el-form-item :label="t('protoCenter.finalmask.stunServers')">
                    <el-input v-model="entry.settings._stunServersStr" type="textarea" :rows="2" placeholder="stun.nextcloud.com:3478" @change="emit" />
                  </el-form-item>
                </el-form>
                <el-divider content-position="left" style="margin:8px 0">{{ t('protoCenter.finalmask.realmTls') }}</el-divider>
                <el-form label-width="140px" label-position="left" size="small">
                  <el-form-item :label="t('protoCenter.finalmask.serverName')">
                    <el-input v-model="entry.settings._tlsServerName" @change="emit" />
                  </el-form-item>
                  <el-form-item :label="t('protoCenter.finalmask.alpn')">
                    <el-select v-model="entry.settings._tlsAlpn" style="width:100%" multiple clearable @change="emit">
                      <el-option label="h3" value="h3" /><el-option label="h2" value="h2" /><el-option label="http/1.1" value="http/1.1" />
                    </el-select>
                  </el-form-item>
                  <el-form-item :label="t('protoCenter.finalmask.fingerprint')">
                    <el-select v-model="entry.settings._tlsFingerprint" style="width:100%" clearable @change="emit">
                      <el-option label="chrome" value="chrome" /><el-option label="firefox" value="firefox" />
                      <el-option label="safari" value="safari" /><el-option label="random" value="random" />
                    </el-select>
                  </el-form-item>
                  <el-form-item :label="t('protoCenter.finalmask.allowInsecure')">
                    <el-switch v-model="entry.settings._tlsAllowInsecure" @change="emit" />
                  </el-form-item>
                </el-form>
              </template>
              <!-- sudoku (UDP) -->
              <template v-if="entry.type === 'sudoku'">
                <el-form label-width="140px" label-position="left" size="small">
                  <el-form-item :label="t('protoCenter.finalmask.password')">
                    <el-input v-model="entry.settings.password" style="width:100%" @change="emit" />
                  </el-form-item>
                  <el-form-item :label="t('protoCenter.finalmask.ascii')">
                    <el-input v-model="entry.settings.ascii" @change="emit" />
                  </el-form-item>
                  <el-form-item :label="t('protoCenter.finalmask.customTable')">
                    <el-input v-model="entry.settings.customTable" @change="emit" />
                  </el-form-item>
                  <el-form-item :label="t('protoCenter.finalmask.customTables')">
                    <el-input v-model="entry.settings._customTablesStr" type="textarea" :rows="2" :placeholder="t('protoCenter.finalmask.customTablesHint')" @change="emit" />
                  </el-form-item>
                  <el-form-item :label="t('protoCenter.finalmask.paddingMin')">
                    <el-input-number v-model="entry.settings.paddingMin" :min="0" @change="emit" />
                  </el-form-item>
                  <el-form-item :label="t('protoCenter.finalmask.paddingMax')">
                    <el-input-number v-model="entry.settings.paddingMax" :min="0" @change="emit" />
                  </el-form-item>
                </el-form>
              </template>
            </div>
            <el-button size="small" @click="rf._finalmaskUdp.push({type:'',settings:{},_json:''})">{{ t('protoCenter.finalmask.addUdpMask') }}</el-button>

            <!-- QUIC Params -->
            <el-divider content-position="left">QUIC Params</el-divider>
            <el-form label-width="180px" label-position="left">
              <el-form-item :label="t('protoCenter.finalmask.congestion')">
                <el-select v-model="rf._finalmaskCongestion" style="width:100%" clearable @change="onOutboundCongestionChange">
                  <el-option label="reno" value="reno" /><el-option label="bbr" value="bbr" />
                  <el-option label="brutal" value="brutal" /><el-option label="force-brutal" value="force-brutal" />
                </el-select>
              </el-form-item>
              <el-form-item v-if="rf._finalmaskCongestion === 'bbr'" :label="t('protoCenter.finalmask.bbrProfile')">
                <el-select v-model="rf._finalmaskBbrProfile" style="width:100%" clearable @change="emit">
                  <el-option label="conservative" value="conservative" /><el-option label="standard" value="standard" /><el-option label="aggressive" value="aggressive" />
                </el-select>
              </el-form-item>
              <el-form-item :label="t('protoCenter.finalmask.debug')">
                <el-switch v-model="rf._finalmaskDebug" @change="emit" />
              </el-form-item>
              <template v-if="rf._finalmaskCongestion === 'brutal' || rf._finalmaskCongestion === 'force-brutal'">
                <el-form-item :label="t('protoCenter.finalmask.brutalUp')">
                  <el-input v-model="rf._finalmaskBrutalUp" placeholder="60 mbps" @change="emit" />
                </el-form-item>
                <el-form-item :label="t('protoCenter.finalmask.brutalDown')">
                  <el-input v-model="rf._finalmaskBrutalDown" placeholder="100 mbps" @change="emit" />
                </el-form-item>
              </template>
              <el-form-item :label="t('protoCenter.finalmask.maxIdleTimeout')">
                <el-input-number v-model="rf._finalmaskMaxIdleTimeout" :min="4" :max="120" @change="emit" />
              </el-form-item>
              <el-form-item :label="t('protoCenter.finalmask.keepAlivePeriod')">
                <el-input-number v-model="rf._finalmaskKeepAlivePeriod" :min="2" :max="60" @change="emit" />
              </el-form-item>
              <el-form-item :label="t('protoCenter.finalmask.disablePathMTU')">
                <el-switch v-model="rf._finalmaskDisablePathMTUDiscovery" @change="emit" />
              </el-form-item>
              <el-form-item :label="t('protoCenter.finalmask.maxIncomingStreams')">
                <el-input-number v-model="rf._finalmaskMaxIncomingStreams" :min="8" @change="emit" />
              </el-form-item>
              <!-- UDP Hop -->
              <el-form-item :label="t('protoCenter.finalmask.udpHop')">
                <el-switch v-model="rf._finalmaskShowUdpHop" @change="onFinalmaskUdpHopToggle" />
              </el-form-item>
              <template v-if="rf._finalmaskShowUdpHop">
                <el-form-item :label="t('protoCenter.finalmask.hopPorts')">
                  <el-input v-model="rf._finalmaskUdpHopPorts" :placeholder="t('protoCenter.finalmask.hopPortsHint')" @change="emit" />
                </el-form-item>
                <el-form-item :label="t('protoCenter.finalmask.hopInterval')">
                  <el-input v-model="rf._finalmaskUdpHopInterval" :placeholder="t('protoCenter.finalmask.hopIntervalHint')" @change="emit" />
                </el-form-item>
              </template>
              <el-form-item :label="t('protoCenter.finalmask.initStreamWindow')">
                <el-input-number v-model="rf._finalmaskInitStreamWindow" :min="16384" @change="emit" />
                <div class="hint">{{ t('protoCenter.finalmask.initStreamWindowHint') }}</div>
              </el-form-item>
              <el-form-item :label="t('protoCenter.finalmask.maxStreamWindow')">
                <el-input-number v-model="rf._finalmaskMaxStreamWindow" :min="16384" @change="emit" />
                <div class="hint">{{ t('protoCenter.finalmask.maxStreamWindowHint') }}</div>
              </el-form-item>
              <el-form-item :label="t('protoCenter.finalmask.initConnWindow')">
                <el-input-number v-model="rf._finalmaskInitConnWindow" :min="16384" @change="emit" />
                <div class="hint">{{ t('protoCenter.finalmask.initConnWindowHint') }}</div>
              </el-form-item>
              <el-form-item :label="t('protoCenter.finalmask.maxConnWindow')">
                <el-input-number v-model="rf._finalmaskMaxConnWindow" :min="16384" @change="emit" />
                <div class="hint">{{ t('protoCenter.finalmask.maxConnWindowHint') }}</div>
              </el-form-item>
            </el-form>
          </template>
        </el-tab-pane>
      </el-tabs>

      <template #footer>
        <div style="display:flex;justify-content:space-between;align-items:center;width:100%">
          <el-form-item :label="t('common.enabled')" label-width="auto" style="margin-bottom:0">
            <el-switch v-model="rf._enabled" />
          </el-form-item>
          <div>
            <el-button @click="dlg=false">{{ t('common.cancel') }}</el-button>
            <el-button type="primary" @click="saveOutbound">{{ t('common.save') }}</el-button>
          </div>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, watch, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQueryState } from '../../../composables/useQueryState'
import { ElMessage } from 'element-plus'
import client from '../../../api/client'
import { ensureXrayRunning, notifyXrayRuntimeApplyFailure } from '../../../utils/xrayRuntime'
const { t } = useI18n()
const props = defineProps({ data: Array, coreType: String, serverId: Number })

// Helper to build node-server-scoped API paths
const nodeBase = computed(() => `/node-servers/${props.serverId}`)
const apiBase = computed(() => `${nodeBase.value}/binaries`)

const form = ref([])
const dlg = ref(false)
const inboundTags = ref([])
const editIdx = ref(-1)
const activeTab = useQueryState('otab', 'basic')
const rf = reactive(getDefaultOutbound())

function getDefaultOutbound() {
  return {
    tag: '', protocol: 'freedom',
    _server: '', _port: 443, _uuid: '', _flow: '', _security: 'auto', _password: '', _username: '', _email: genEmail(), _level: 0, _ssMethod: '2022-blake3-aes-256-gcm', _vlessEncryption: 'none', _vlessReverseEnabled: false, _vlessReverseTag: '', _vlessReverseSniffing: false, _vlessReverseSniffingOverride: ['http', 'tls', 'quic'], _vlessReverseSniffingMetadata: false, _vlessReverseSniffingRouteOnly: false, _vlessReverseSniffingDomainsExcluded: '', _vlessReverseSniffingIpsExcluded: '',
    _freedomAdvanced: false, _domainStrategy: 'AsIs', _freedomRedirect: '', _freedomUserLevel: 0, _freedomProxyProtocol: 0,
    _freedomFragmentEnabled: false, _freedomFragmentPackets: 'tlshello', _freedomFragmentLength: '100-200', _freedomFragmentInterval: '10-20',
    _freedomNoisesEnabled: false, _freedomNoises: [{ type: 'rand', packet: '100', delay: '10-20' }],
    _freedomFinalRulesEnabled: false, _freedomFinalRules: [],
    _blackholeAdvanced: false, _blackholeResponse: 'none',
    _dnsRewriteNetwork: '', _dnsRewriteAddress: '', _dnsRewritePort: 0, _dnsUserLevel: 0, _dnsRules: [],
    _loopbackInboundTag: '',
    _hysteriaVersion: 2, _hysteriaAuth: '',
    _network: 'raw', _securityMode: 'none',
    _raw: { headerType: 'none', request: { version: '1.1', method: 'GET', _pathsStr: '/', _headers: [] }, response: { version: '1.1', status: '200', reason: 'OK', _headers: [] } },
    _kcpMtu: 1350, _kcpTti: 50, _kcpUplinkCapacity: 5, _kcpDownlinkCapacity: 20, _kcpCongestion: false, _kcpReadBufferSize: 2, _kcpWriteBufferSize: 2,
    _sni: '', _alpn: [], _fingerprint: '', _allowInsecure: false,
    _tlsVerifyPeerCertByName: '', _tlsMinVersion: '', _tlsMaxVersion: '', _tlsCipherSuites: [], _tlsCurvePreferences: [], _tlsDisableSystemRoot: false, _tlsPinnedPeerCert: '', _tlsEchConfigList: '', _tlsMasterKeyLog: '',
    _realitySni: '', _realityPublicKey: '', _realityShortId: '', _realityFingerprint: 'chrome', _realitySpiderX: '', _realityMldsa65Verify: '',
    _wsPath: '/', _wsHost: '', _wsHeaders: [], _wsHeartbeatPeriod: 0,
    _grpcServiceName: '', _grpcAuthority: '', _grpcUserAgent: '', _grpcMultiMode: false, _grpcIdleTimeout: 0, _grpcHealthCheckTimeout: 20, _grpcPermitWithoutStream: false, _grpcInitialWindowsSize: 0,
    _httpupgradePath: '/', _httpupgradeHost: '', _httpupgradeHeaders: [],
    _xhttpPath: '/', _xhttpHost: '', _xhttpMode: '',
    _xhttpPaddingBytes: '100-1000', _xhttpNoGRPCHeader: false, _xhttpHeaders: [],
    _xhttpScMaxEachPostBytes: 1000000, _xhttpScMinPostsIntervalMs: 30,
    _xhttpXmuxMaxConcurrency: '16-32', _xhttpXmuxMaxConnections: '0', _xhttpXmuxCMaxReuseTimes: '0', _xhttpXmuxHMaxRequestTimes: '600-900', _xhttpXmuxHMaxReusableSecs: '1800-3000', _xhttpXmuxHKeepAlivePeriod: 0,
    _xhttpDownloadEnabled: false, _xhttpDownloadAddress: '', _xhttpDownloadPort: 443, _xhttpDownloadSecurity: 'none',
    _xhttpDownloadTlsServerName: '', _xhttpDownloadTlsAlpn: [], _xhttpDownloadTlsFingerprint: '',
    _xhttpDownloadRealitySni: '', _xhttpDownloadRealityPassword: '', _xhttpDownloadRealityShortId: '', _xhttpDownloadRealityFingerprint: 'chrome',
    _xhttpDownloadPath: '',
    _xhttpDownloadXmuxMaxConcurrency: '16-32', _xhttpDownloadXmuxMaxConnections: '0', _xhttpDownloadXmuxCMaxReuseTimes: '0',
    _xhttpDownloadXmuxHMaxRequestTimes: '600-900', _xhttpDownloadXmuxHMaxReusableSecs: '1800-3000', _xhttpDownloadXmuxHKeepAlivePeriod: 0,
    _hysteriaAuth: '', _hysteriaUdpIdleTimeout: 60, _hysteriaMasqType: '', _hysteriaMasqDir: '', _hysteriaMasqUrl: '', _hysteriaMasqRewriteHost: false, _hysteriaMasqInsecure: false, _hysteriaMasqContent: '', _hysteriaMasqStatusCode: 0, _hysteriaMasqHeaders: [],
    _sockoptEnabled: false, _sockoptMark: 0, _sockoptTcpFastOpen: false, _sockoptTproxy: 'off',
    _sockoptDomainStrategy: '', _sockoptInterface: '', _sockoptV6only: false,
    _sockoptTcpKeepAliveIdle: 0, _sockoptTcpKeepAliveInterval: 0, _sockoptTcpUserTimeout: 10000,
    _sockoptTcpcongestion: '', _sockoptTcpMaxSeg: 1440, _sockoptTcpWindowClamp: 600,
    _sockoptTcpMptcp: false, _sockoptAcceptProxyProtocol: false,
    _sockoptDialerProxy: '', _sockoptAddressPortStrategy: '', _sockoptTrustedXffStr: '',
    _sockoptShowHappyEyeballs: false, _sockoptHappyEyeballsTryDelayMs: 250, _sockoptHappyEyeballsPrioritizeIPv6: false, _sockoptHappyEyeballsInterleave: 1, _sockoptHappyEyeballsMaxConcurrentTry: 4,
    _sockoptShowCustomSockopt: false, _sockoptCustomSockopt: [],
    _finalmaskEnabled: false, _finalmaskTcp: [], _finalmaskUdp: [],
    _finalmaskCongestion: '', _finalmaskBbrProfile: '', _finalmaskDebug: false,
    _finalmaskBrutalUp: '', _finalmaskBrutalDown: '',
    _finalmaskShowUdpHop: false, _finalmaskUdpHopPorts: '', _finalmaskUdpHopInterval: '',
    _finalmaskMaxIdleTimeout: '', _finalmaskKeepAlivePeriod: '', _finalmaskDisablePathMTUDiscovery: false, _finalmaskMaxIncomingStreams: '',
    _finalmaskInitStreamWindow: '', _finalmaskMaxStreamWindow: '', _finalmaskInitConnWindow: '', _finalmaskMaxConnWindow: '',
    _wgSecretKey: '', _wgPublicKey: '', _wgAddressIPv4: '', _wgAddressIPv6: '', _wgMtu: 1280, _wgWorkers: 2, _wgReservedStr: '', _wgNoKernelTun: false, _wgDomainStrategy: 'ForceIP',
    _wgPeers: [],
    _enabled: true,
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
    _vlessEncryption: user0.encryption || 'none',
    _vlessReverseEnabled: !!(ob.settings && ob.settings.reverse),
    _vlessReverseTag: (ob.settings && ob.settings.reverse && ob.settings.reverse.tag) || '',
    _vlessReverseSniffing: !!(ob.settings && ob.settings.reverse && ob.settings.reverse.sniffing),
    _vlessReverseSniffingOverride: (ob.settings && ob.settings.reverse && ob.settings.reverse.sniffing && ob.settings.reverse.sniffing.destOverride) || ['http', 'tls', 'fakedns'],
    _vlessReverseSniffingMetadata: !!(ob.settings && ob.settings.reverse && ob.settings.reverse.sniffing && ob.settings.reverse.sniffing.metadataOnly),
    _vlessReverseSniffingRouteOnly: !!(ob.settings && ob.settings.reverse && ob.settings.reverse.sniffing && ob.settings.reverse.sniffing.routeOnly),
    _vlessReverseSniffingDomainsExcluded: ((ob.settings && ob.settings.reverse && ob.settings.reverse.sniffing && ob.settings.reverse.sniffing.domainsExcluded) || []).join('\n'),
    _vlessReverseSniffingIpsExcluded: ((ob.settings && ob.settings.reverse && ob.settings.reverse.sniffing && ob.settings.reverse.sniffing.ipsExcluded) || []).join('\n'),
    _password: srv.password || (ob.settings && ob.settings.password) || (ob.settings && ob.settings.pass) || '',
    _username: srv.users && srv.users[0] ? srv.users[0].user || '' : (ob.settings && ob.settings.user) || '',
    _email: (ob.settings && ob.settings.email) || '',
    _level: (ob.settings && ob.settings.level) || 0,
    _ssMethod: (ob.settings && ob.settings.method) || '2022-blake3-aes-256-gcm',
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
    _hysteriaVersion: (ob.settings && ob.settings.version) || 2,
    _hysteriaAuth: (ss.hysteriaSettings && ss.hysteriaSettings.auth) || (ob.settings && ob.settings.auth) || '',
    _network: ss.network || 'raw',
    _securityMode: ss.security || 'none',
    _raw: parseRawSettings(ss.rawSettings),
    _kcpMtu: (ss.kcpSettings && ss.kcpSettings.mtu) || 1350,
    _kcpTti: (ss.kcpSettings && ss.kcpSettings.tti) || 50,
    _kcpUplinkCapacity: (ss.kcpSettings && ss.kcpSettings.uplinkCapacity) || 5,
    _kcpDownlinkCapacity: (ss.kcpSettings && ss.kcpSettings.downlinkCapacity) || 20,
    _kcpCongestion: !!(ss.kcpSettings && ss.kcpSettings.congestion),
    _kcpReadBufferSize: (ss.kcpSettings && ss.kcpSettings.readBufferSize) || 2,
    _kcpWriteBufferSize: (ss.kcpSettings && ss.kcpSettings.writeBufferSize) || 2,
    _sockoptEnabled: !!(ss.sockopt && Object.keys(ss.sockopt).length),
    _sockoptMark: (ss.sockopt && ss.sockopt.mark) || 0,
    _sockoptTcpFastOpen: !!(ss.sockopt && ss.sockopt.tcpFastOpen),
    _sockoptTproxy: (ss.sockopt && ss.sockopt.tproxy) || 'off',
    _sockoptDomainStrategy: (ss.sockopt && ss.sockopt.domainStrategy) || '',
    _sockoptInterface: (ss.sockopt && ss.sockopt.interface) || '',
    _sockoptV6only: !!(ss.sockopt && (ss.sockopt.V6Only || ss.sockopt.v6only)),
    _sockoptTcpKeepAliveIdle: (ss.sockopt && ss.sockopt.tcpKeepAliveIdle) || 0,
    _sockoptTcpKeepAliveInterval: (ss.sockopt && ss.sockopt.tcpKeepAliveInterval) || 0,
    _sockoptTcpUserTimeout: (ss.sockopt && ss.sockopt.tcpUserTimeout) || 10000,
    _sockoptTcpcongestion: (ss.sockopt && ss.sockopt.tcpcongestion) || '',
    _sockoptTcpMaxSeg: (ss.sockopt && ss.sockopt.tcpMaxSeg) || 1440,
    _sockoptTcpWindowClamp: (ss.sockopt && ss.sockopt.tcpWindowClamp) || 600,
    _sockoptTcpMptcp: !!(ss.sockopt && ss.sockopt.tcpMptcp),
    _sockoptAcceptProxyProtocol: !!(ss.sockopt && ss.sockopt.acceptProxyProtocol),
    _sockoptDialerProxy: (ss.sockopt && ss.sockopt.dialerProxy) || '',
    _sockoptAddressPortStrategy: (ss.sockopt && ss.sockopt.addressPortStrategy) || '',
    _sockoptTrustedXffStr: ((ss.sockopt && ss.sockopt.trustedXForwardedFor) || []).join('\n'),
    _sockoptShowHappyEyeballs: !!(ss.sockopt && ss.sockopt.happyEyeballs),
    _sockoptHappyEyeballsTryDelayMs: (ss.sockopt && ss.sockopt.happyEyeballs && ss.sockopt.happyEyeballs.tryDelayMs) || 250,
    _sockoptHappyEyeballsPrioritizeIPv6: !!(ss.sockopt && ss.sockopt.happyEyeballs && ss.sockopt.happyEyeballs.prioritizeIPv6),
    _sockoptHappyEyeballsInterleave: (ss.sockopt && ss.sockopt.happyEyeballs && ss.sockopt.happyEyeballs.interleave) || 1,
    _sockoptHappyEyeballsMaxConcurrentTry: (ss.sockopt && ss.sockopt.happyEyeballs && ss.sockopt.happyEyeballs.maxConcurrentTry) || 4,
    _sockoptShowCustomSockopt: !!(ss.sockopt && ss.sockopt.customSockopt && ss.sockopt.customSockopt.length),
    _sockoptCustomSockopt: (ss.sockopt && ss.sockopt.customSockopt) ? ss.sockopt.customSockopt.map(c => ({ system: c.system || '', network: c.network || '', type: c.type || 'str', level: c.level || '6', opt: c.opt || '', value: c.value || '' })) : [],
    _finalmaskEnabled: !!(ss.finalmask && (ss.finalmask.tcp || ss.finalmask.udp || ss.finalmask.quicParams)),
    _finalmaskTcp: (ss.finalmask && ss.finalmask.tcp) ? ss.finalmask.tcp.map(e => {
      const s = e.settings || {}
      if (e.type === 'header-custom') {
        const fixGroup = g => (g || []).map(items => (items || []).map(i => ({ delay: i.delay || 0, rand: i.rand || 0, randRange: i.randRange || '0-255', type: i.type || 'array', _packetStr: Array.isArray(i.packet) ? i.packet.join(',') : (i.packet || '') })))
        const clients = fixGroup(s.clients), servers = fixGroup(s.servers), errors = fixGroup(s.errors)
        return { type: 'header-custom', settings: { clients, servers, errors }, _json: JSON.stringify({ clients: s.clients || [[]], servers: s.servers || [[]], errors: s.errors || [[]] }, null, 2) }
      }
      return { type: e.type || '', settings: s, _json: '' }
    }) : [],
    _finalmaskUdp: (ss.finalmask && ss.finalmask.udp) ? ss.finalmask.udp.map(e => {
      const s = e.settings || {}
      if (e.type === 'header-custom') {
        const fixArr = arr => (arr || []).map(i => ({ rand: i.rand || 0, randRange: i.randRange || '0-255', type: i.type || 'array', _packetStr: Array.isArray(i.packet) ? i.packet.join(',') : (i.packet || '') }))
        return { type: 'header-custom', settings: { client: fixArr(s.client), server: fixArr(s.server) }, _json: JSON.stringify({ client: s.client || [], server: s.server || [] }, null, 2) }
      }
      const base = { type: e.type || '', settings: { ...s }, _json: '' }
      if (e.type === 'noise') base.settings._noise = (s.noise || []).map(n => ({ type: n.type || 'array', rand: n.rand || '', randRange: n.randRange || '0-255', delay: n.delay || '', _packetStr: Array.isArray(n.packet) ? n.packet.join(',') : (n.packet || '') }))
      if (e.type === 'xdns') { base.settings._domainsStr = (s.domains || []).join('\n'); base.settings._resolversStr = (s.resolvers || []).join('\n') }
      if (e.type === 'xicmp') base.settings._ipsStr = (s.ips || []).join('\n')
      if (e.type === 'realm') { base.settings.url = s.url || ''; base.settings._stunServersStr = (s.stunServers || []).join('\n'); const tls = s.tlsConfig || {}; base.settings._tlsServerName = tls.serverName || ''; base.settings._tlsAlpn = tls.alpn || []; base.settings._tlsFingerprint = tls.fingerprint || ''; base.settings._tlsAllowInsecure = tls.allowInsecure || false }
      if (e.type === 'sudoku') base.settings._customTablesStr = (s.customTables || []).join('\n')
      if (e.type === 'salamander') { const ps = s.packetSize || ''; const match = /^(\d+)-(\d+)$/.exec(ps); base.settings._gecko = !!match; base.settings._geckoMin = match ? Number(match[1]) : 512; base.settings._geckoMax = match ? Number(match[2]) : 1200 }
      return base
    }) : [],
    _finalmaskCongestion: (ss.finalmask && ss.finalmask.quicParams && ss.finalmask.quicParams.congestion) || '',
    _finalmaskBbrProfile: (ss.finalmask && ss.finalmask.quicParams && ss.finalmask.quicParams.bbrProfile) || '',
    _finalmaskDebug: !!(ss.finalmask && ss.finalmask.quicParams && ss.finalmask.quicParams.debug),
    _finalmaskBrutalUp: (ss.finalmask && ss.finalmask.quicParams && ss.finalmask.quicParams.brutalUp) || '',
    _finalmaskBrutalDown: (ss.finalmask && ss.finalmask.quicParams && ss.finalmask.quicParams.brutalDown) || '',
    _finalmaskShowUdpHop: !!(ss.finalmask && ss.finalmask.quicParams && ss.finalmask.quicParams.udpHop),
    _finalmaskUdpHopPorts: (ss.finalmask && ss.finalmask.quicParams && ss.finalmask.quicParams.udpHop && ss.finalmask.quicParams.udpHop.ports) || '',
    _finalmaskUdpHopInterval: (ss.finalmask && ss.finalmask.quicParams && ss.finalmask.quicParams.udpHop && ss.finalmask.quicParams.udpHop.interval) || '',
    _finalmaskMaxIdleTimeout: ss.finalmask?.quicParams?.maxIdleTimeout ?? '',
    _finalmaskKeepAlivePeriod: ss.finalmask?.quicParams?.keepAlivePeriod ?? '',
    _finalmaskDisablePathMTUDiscovery: !!(ss.finalmask && ss.finalmask.quicParams && ss.finalmask.quicParams.disablePathMTUDiscovery),
    _finalmaskMaxIncomingStreams: ss.finalmask?.quicParams?.maxIncomingStreams ?? '',
    _finalmaskInitStreamWindow: ss.finalmask?.quicParams?.initStreamReceiveWindow ?? '',
    _finalmaskMaxStreamWindow: ss.finalmask?.quicParams?.maxStreamReceiveWindow ?? '',
    _finalmaskInitConnWindow: ss.finalmask?.quicParams?.initConnectionReceiveWindow ?? '',
    _finalmaskMaxConnWindow: ss.finalmask?.quicParams?.maxConnectionReceiveWindow ?? '',
    _sni: tls.serverName || '',
    _alpn: tls.alpn || [],
    _fingerprint: tls.fingerprint || '',
    _allowInsecure: tls.allowInsecure || false,
    _tlsVerifyPeerCertByName: tls.verifyPeerCertByName || '',
    _tlsMinVersion: tls.minVersion || '',
    _tlsMaxVersion: tls.maxVersion || '',
    _tlsCipherSuites: tls.cipherSuites ? tls.cipherSuites.split(':') : [],
    _tlsCurvePreferences: tls.curvePreferences || [],
    _tlsDisableSystemRoot: tls.disableSystemRoot || false,
    _tlsPinnedPeerCert: (tls.pinnedPeerCertSha256 || []).join(', '),
    _tlsEchConfigList: tls.echConfigList || '',
    _tlsMasterKeyLog: tls.masterKeyLog || '',
    _realitySni: reality.serverName || '',
    _realityPublicKey: reality.password || realitySettings.password || '',
    _realityShortId: reality.shortId || '',
    _realityFingerprint: reality.fingerprint || realitySettings.fingerprint || 'chrome',
    _realitySpiderX: reality.spiderX || '',
    _realityMldsa65Verify: realitySettings.mldsa65Verify || '',
    _wsPath: ws.path || '/',
    _wsHost: ws.host || '',
    _wsHeaders: Object.entries(ws.headers || {}).map(([k, v]) => ({ key: k, value: v || '' })),
    _wsHeartbeatPeriod: ws.heartbeatPeriod || 0,
    _grpcServiceName: grpc.serviceName || '',
    _grpcAuthority: grpc.authority || '',
    _grpcUserAgent: grpc.user_agent || '',
    _grpcMultiMode: grpc.multiMode || false,
    _grpcIdleTimeout: grpc.idle_timeout || 0,
    _grpcHealthCheckTimeout: grpc.health_check_timeout || 20,
    _grpcPermitWithoutStream: grpc.permit_without_stream || false,
    _grpcInitialWindowsSize: grpc.initial_windows_size || 0,
    _httpupgradePath: (ss.httpupgradeSettings && ss.httpupgradeSettings.path) || '/',
    _httpupgradeHost: (ss.httpupgradeSettings && ss.httpupgradeSettings.host) || '',
    _httpupgradeHeaders: Object.entries((ss.httpupgradeSettings && ss.httpupgradeSettings.headers) || {}).map(([k, v]) => ({ key: k, value: v || '' })),
    _xhttpPath: (ss.xhttpSettings && ss.xhttpSettings.path) || '/',
    _xhttpHost: (ss.xhttpSettings && ss.xhttpSettings.host) || '',
    _xhttpMode: (ss.xhttpSettings && ss.xhttpSettings.mode) || '',
    _xhttpPaddingBytes: (ss.xhttpSettings && ss.xhttpSettings.extra && ss.xhttpSettings.extra.xPaddingBytes) || '100-1000',
    _xhttpNoGRPCHeader: !!(ss.xhttpSettings && ss.xhttpSettings.extra && ss.xhttpSettings.extra.noGRPCHeader),
    _xhttpHeaders: Object.entries(((ss.xhttpSettings && ss.xhttpSettings.extra && ss.xhttpSettings.extra.headers) || {})).map(([k, v]) => ({ key: k, value: v || '' })),
    _xhttpScMaxEachPostBytes: (ss.xhttpSettings && ss.xhttpSettings.extra && ss.xhttpSettings.extra.scMaxEachPostBytes) || 1000000,
    _xhttpScMinPostsIntervalMs: (ss.xhttpSettings && ss.xhttpSettings.extra && ss.xhttpSettings.extra.scMinPostsIntervalMs) || 30,
    _xhttpXmuxMaxConcurrency: (ss.xhttpSettings && ss.xhttpSettings.extra && ss.xhttpSettings.extra.xmux && ss.xhttpSettings.extra.xmux.maxConcurrency) || '16-32',
    _xhttpXmuxMaxConnections: String((ss.xhttpSettings && ss.xhttpSettings.extra && ss.xhttpSettings.extra.xmux && ss.xhttpSettings.extra.xmux.maxConnections) || 0),
    _xhttpXmuxCMaxReuseTimes: String((ss.xhttpSettings && ss.xhttpSettings.extra && ss.xhttpSettings.extra.xmux && ss.xhttpSettings.extra.xmux.cMaxReuseTimes) || 0),
    _xhttpXmuxHMaxRequestTimes: (ss.xhttpSettings && ss.xhttpSettings.extra && ss.xhttpSettings.extra.xmux && ss.xhttpSettings.extra.xmux.hMaxRequestTimes) || '600-900',
    _xhttpXmuxHMaxReusableSecs: (ss.xhttpSettings && ss.xhttpSettings.extra && ss.xhttpSettings.extra.xmux && ss.xhttpSettings.extra.xmux.hMaxReusableSecs) || '1800-3000',
    _xhttpXmuxHKeepAlivePeriod: (ss.xhttpSettings && ss.xhttpSettings.extra && ss.xhttpSettings.extra.xmux && ss.xhttpSettings.extra.xmux.hKeepAlivePeriod) || 0,
    _xhttpDownloadEnabled: !!(ss.xhttpSettings && ss.xhttpSettings.extra && ss.xhttpSettings.extra.downloadSettings),
    _xhttpDownloadAddress: (ss.xhttpSettings && ss.xhttpSettings.extra && ss.xhttpSettings.extra.downloadSettings && ss.xhttpSettings.extra.downloadSettings.address) || '',
    _xhttpDownloadPort: (ss.xhttpSettings && ss.xhttpSettings.extra && ss.xhttpSettings.extra.downloadSettings && ss.xhttpSettings.extra.downloadSettings.port) || 443,
    _xhttpDownloadSecurity: (ss.xhttpSettings && ss.xhttpSettings.extra && ss.xhttpSettings.extra.downloadSettings && ss.xhttpSettings.extra.downloadSettings.security) || 'none',
    _xhttpDownloadTlsServerName: (ss.xhttpSettings && ss.xhttpSettings.extra && ss.xhttpSettings.extra.downloadSettings && ss.xhttpSettings.extra.downloadSettings.tlsSettings && ss.xhttpSettings.extra.downloadSettings.tlsSettings.serverName) || '',
    _xhttpDownloadTlsAlpn: (ss.xhttpSettings && ss.xhttpSettings.extra && ss.xhttpSettings.extra.downloadSettings && ss.xhttpSettings.extra.downloadSettings.tlsSettings && ss.xhttpSettings.extra.downloadSettings.tlsSettings.alpn) || [],
    _xhttpDownloadTlsFingerprint: (ss.xhttpSettings && ss.xhttpSettings.extra && ss.xhttpSettings.extra.downloadSettings && ss.xhttpSettings.extra.downloadSettings.tlsSettings && ss.xhttpSettings.extra.downloadSettings.tlsSettings.fingerprint) || '',
    _xhttpDownloadRealitySni: (ss.xhttpSettings && ss.xhttpSettings.extra && ss.xhttpSettings.extra.downloadSettings && ss.xhttpSettings.extra.downloadSettings.realitySettings && ss.xhttpSettings.extra.downloadSettings.realitySettings.serverName) || '',
    _xhttpDownloadRealityPassword: (ss.xhttpSettings && ss.xhttpSettings.extra && ss.xhttpSettings.extra.downloadSettings && ss.xhttpSettings.extra.downloadSettings.realitySettings && ss.xhttpSettings.extra.downloadSettings.realitySettings.password) || '',
    _xhttpDownloadRealityShortId: (ss.xhttpSettings && ss.xhttpSettings.extra && ss.xhttpSettings.extra.downloadSettings && ss.xhttpSettings.extra.downloadSettings.realitySettings && ss.xhttpSettings.extra.downloadSettings.realitySettings.shortId) || '',
    _xhttpDownloadRealityFingerprint: (ss.xhttpSettings && ss.xhttpSettings.extra && ss.xhttpSettings.extra.downloadSettings && ss.xhttpSettings.extra.downloadSettings.realitySettings && ss.xhttpSettings.extra.downloadSettings.realitySettings.fingerprint) || 'chrome',
    _xhttpDownloadPath: (ss.xhttpSettings && ss.xhttpSettings.extra && ss.xhttpSettings.extra.downloadSettings && ss.xhttpSettings.extra.downloadSettings.xhttpSettings && ss.xhttpSettings.extra.downloadSettings.xhttpSettings.path) || '',
    _xhttpDownloadXmuxMaxConcurrency: (ss.xhttpSettings && ss.xhttpSettings.extra && ss.xhttpSettings.extra.downloadSettings && ss.xhttpSettings.extra.downloadSettings.xhttpSettings && ss.xhttpSettings.extra.downloadSettings.xhttpSettings.extra && ss.xhttpSettings.extra.downloadSettings.xhttpSettings.extra.xmux && ss.xhttpSettings.extra.downloadSettings.xhttpSettings.extra.xmux.maxConcurrency) || '16-32',
    _xhttpDownloadXmuxMaxConnections: String((ss.xhttpSettings && ss.xhttpSettings.extra && ss.xhttpSettings.extra.downloadSettings && ss.xhttpSettings.extra.downloadSettings.xhttpSettings && ss.xhttpSettings.extra.downloadSettings.xhttpSettings.extra && ss.xhttpSettings.extra.downloadSettings.xhttpSettings.extra.xmux && ss.xhttpSettings.extra.downloadSettings.xhttpSettings.extra.xmux.maxConnections) || 0),
    _xhttpDownloadXmuxCMaxReuseTimes: String((ss.xhttpSettings && ss.xhttpSettings.extra && ss.xhttpSettings.extra.downloadSettings && ss.xhttpSettings.extra.downloadSettings.xhttpSettings && ss.xhttpSettings.extra.downloadSettings.xhttpSettings.extra && ss.xhttpSettings.extra.downloadSettings.xhttpSettings.extra.xmux && ss.xhttpSettings.extra.downloadSettings.xhttpSettings.extra.xmux.cMaxReuseTimes) || 0),
    _xhttpDownloadXmuxHMaxRequestTimes: (ss.xhttpSettings && ss.xhttpSettings.extra && ss.xhttpSettings.extra.downloadSettings && ss.xhttpSettings.extra.downloadSettings.xhttpSettings && ss.xhttpSettings.extra.downloadSettings.xhttpSettings.extra && ss.xhttpSettings.extra.downloadSettings.xhttpSettings.extra.xmux && ss.xhttpSettings.extra.downloadSettings.xhttpSettings.extra.xmux.hMaxRequestTimes) || '600-900',
    _xhttpDownloadXmuxHMaxReusableSecs: (ss.xhttpSettings && ss.xhttpSettings.extra && ss.xhttpSettings.extra.downloadSettings && ss.xhttpSettings.extra.downloadSettings.xhttpSettings && ss.xhttpSettings.extra.downloadSettings.xhttpSettings.extra && ss.xhttpSettings.extra.downloadSettings.xhttpSettings.extra.xmux && ss.xhttpSettings.extra.downloadSettings.xhttpSettings.extra.xmux.hMaxReusableSecs) || '1800-3000',
    _xhttpDownloadXmuxHKeepAlivePeriod: (ss.xhttpSettings && ss.xhttpSettings.extra && ss.xhttpSettings.extra.downloadSettings && ss.xhttpSettings.extra.downloadSettings.xhttpSettings && ss.xhttpSettings.extra.downloadSettings.xhttpSettings.extra && ss.xhttpSettings.extra.downloadSettings.xhttpSettings.extra.xmux && ss.xhttpSettings.extra.downloadSettings.xhttpSettings.extra.xmux.hKeepAlivePeriod) || 0,
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
      ? { id: ob._uuid, alterId: 0, security: ob._security || 'auto' }
      : { id: ob._uuid, encryption: ob._vlessEncryption || 'none', flow: ob._flow }
    if (ob._level) user.level = ob._level
    if (ob._email) user.email = ob._email
    result.settings.vnext = [{ address: ob._server, port: ob._port, users: [user] }]
    // VLESS reverse proxy
    if (ob.protocol === 'vless' && ob._vlessReverseEnabled) {
      const reverse = {}
      if (ob._vlessReverseTag) reverse.tag = ob._vlessReverseTag
      if (ob._vlessReverseSniffing) {
        reverse.sniffing = {
          enabled: true,
          destOverride: ob._vlessReverseSniffingOverride || ['http', 'tls', 'fakedns'],
        }
        if (ob._vlessReverseSniffingMetadata) reverse.sniffing.metadataOnly = true
        if (ob._vlessReverseSniffingRouteOnly) reverse.sniffing.routeOnly = true
        if (ob._vlessReverseSniffingDomainsExcluded) reverse.sniffing.domainsExcluded = ob._vlessReverseSniffingDomainsExcluded.split('\n').map(s => s.trim()).filter(Boolean)
        if (ob._vlessReverseSniffingIpsExcluded) reverse.sniffing.ipsExcluded = ob._vlessReverseSniffingIpsExcluded.split('\n').map(s => s.trim()).filter(Boolean)
      }
      if (Object.keys(reverse).length) result.settings.reverse = reverse
    }
    result.streamSettings = buildStreamSettings(ob)
  } else if (ob.protocol === 'trojan') {
    result.settings = { address: ob._server, port: ob._port, password: ob._password }
    if (ob._email) result.settings.email = ob._email
    if (ob._level) result.settings.level = ob._level
    result.streamSettings = buildStreamSettings(ob)
  } else if (ob.protocol === 'shadowsocks') {
    result.settings = { address: ob._server, port: ob._port, method: ob._ssMethod || '2022-blake3-aes-256-gcm', password: ob._password }
    if (ob._email) result.settings.email = ob._email
    if (ob._level) result.settings.level = ob._level
    result.streamSettings = buildStreamSettings(ob)
  } else if (ob.protocol === 'socks') {
    result.settings = { address: ob._server, port: ob._port }
    if (ob._username) result.settings.user = ob._username
    if (ob._password) result.settings.pass = ob._password
    if (ob._level) result.settings.level = ob._level
    if (ob._email) result.settings.email = ob._email
    result.streamSettings = buildStreamSettings(ob)
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
  } else if (ob.protocol === 'hysteria') {
    result.settings = { version: ob._hysteriaVersion || 2, address: ob._server, port: ob._port }
    const ss = buildStreamSettings(ob)
    ss.network = 'hysteria'
    if (ob._hysteriaAuth) {
      ss.hysteriaSettings = { version: ob._hysteriaVersion || 2, auth: ob._hysteriaAuth }
    }
    result.streamSettings = ss
  }
  return result
}

function parseRawSettings(raw) {
  const header = (raw && raw.header) || {}
  const req = header.request || {}
  const resp = header.response || {}
  return {
    headerType: header.type || 'none',
    request: {
      version: req.version || '1.1', method: req.method || 'GET',
      _pathsStr: (req.path || ['/']).join('\n'),
      _headers: Object.entries(req.headers || {}).map(([k, v]) => ({ key: k, value: Array.isArray(v) ? v[0] || '' : v || '' })),
    },
    response: {
      version: resp.version || '1.1', status: resp.status || '200', reason: resp.reason || 'OK',
      _headers: Object.entries(resp.headers || {}).map(([k, v]) => ({ key: k, value: Array.isArray(v) ? v[0] || '' : v || '' })),
    },
  }
}

function onRawHeaderChange() { emit() }

function buildStreamSettings(ob) {
  const ss = { network: ob._network || 'raw', security: ob._securityMode || 'none' }
  if (ob._securityMode === 'tls') {
    const tls = {}
    if (ob._sni) tls.serverName = ob._sni
    if (ob._tlsVerifyPeerCertByName) tls.verifyPeerCertByName = ob._tlsVerifyPeerCertByName
    if (ob._alpn && ob._alpn.length) tls.alpn = ob._alpn
    if (ob._fingerprint) tls.fingerprint = ob._fingerprint
    if (ob._allowInsecure) tls.allowInsecure = true
    if (ob._tlsDisableSystemRoot) tls.disableSystemRoot = true
    if (ob._tlsEchConfigList) tls.echConfigList = ob._tlsEchConfigList
    if (ob._tlsMasterKeyLog) tls.masterKeyLog = ob._tlsMasterKeyLog
    // Client params in settings array
    const settingsObj = {}
    if (ob._fingerprint) settingsObj.fingerprint = ob._fingerprint
    if (ob._tlsEchConfigList) settingsObj.echConfigList = ob._tlsEchConfigList
    if (Object.keys(settingsObj).length) tls.settings = [settingsObj]
    ss.tlsSettings = tls
  }
  if (ob._securityMode === 'reality') {
    const reality = {}
    if (ob._realitySni) reality.serverName = ob._realitySni
    if (ob._realityPublicKey) reality.password = ob._realityPublicKey
    if (ob._realityShortId) reality.shortId = ob._realityShortId
    if (ob._realityFingerprint) reality.fingerprint = ob._realityFingerprint
    if (ob._realitySpiderX) reality.spiderX = ob._realitySpiderX
    // Client params in settings array
    const realitySettingsObj = {}
    if (ob._realityFingerprint) realitySettingsObj.fingerprint = ob._realityFingerprint
    if (ob._realityMldsa65Verify) realitySettingsObj.mldsa65Verify = ob._realityMldsa65Verify
    if (Object.keys(realitySettingsObj).length) reality.settings = [realitySettingsObj]
    ss.realitySettings = reality
  }
  if (ob._network === 'kcp') {
    ss.kcpSettings = {
      mtu: ob._kcpMtu || 1350,
      tti: ob._kcpTti || 50,
      uplinkCapacity: ob._kcpUplinkCapacity || 5,
      downlinkCapacity: ob._kcpDownlinkCapacity || 20,
      congestion: ob._kcpCongestion || false,
      readBufferSize: ob._kcpReadBufferSize || 2,
      writeBufferSize: ob._kcpWriteBufferSize || 2,
    }
  }
  if (ob._network === 'raw' && ob._raw) {
    const raw = ob._raw
    const header = { type: raw.headerType || 'none' }
    if (raw.headerType === 'http') {
      header.request = {
        version: raw.request.version || '1.1',
        method: raw.request.method || 'GET',
        path: raw.request._pathsStr ? raw.request._pathsStr.split('\n').map(s => s.trim()).filter(Boolean) : ['/'],
      }
      if (raw.request._headers && raw.request._headers.length) {
        header.request.headers = {}
        raw.request._headers.forEach(h => { if (h.key) header.request.headers[h.key] = [h.value || ''] })
      }
      header.response = {
        version: raw.response.version || '1.1',
        status: raw.response.status || '200',
        reason: raw.response.reason || 'OK',
      }
      if (raw.response._headers && raw.response._headers.length) {
        header.response.headers = {}
        raw.response._headers.forEach(h => { if (h.key) header.response.headers[h.key] = [h.value || ''] })
      }
    }
    ss.rawSettings = { header }
  }
  if (ob._network === 'ws') {
    const ws = { path: ob._wsPath || '/' }
    if (ob._wsHost) ws.host = ob._wsHost
    if (ob._wsHeaders && ob._wsHeaders.length) {
      ws.headers = {}
      ob._wsHeaders.forEach(h => { if (h.key) ws.headers[h.key] = h.value || '' })
    }
    if (ob._wsHeartbeatPeriod) ws.heartbeatPeriod = ob._wsHeartbeatPeriod
    ss.wsSettings = ws
  }
  if (ob._network === 'grpc') {
    const grpc = { serviceName: ob._grpcServiceName || '' }
    if (ob._grpcAuthority) grpc.authority = ob._grpcAuthority
    if (ob._grpcUserAgent) grpc.user_agent = ob._grpcUserAgent
    if (ob._grpcMultiMode) grpc.multiMode = true
    if (ob._grpcIdleTimeout) grpc.idle_timeout = ob._grpcIdleTimeout
    if (ob._grpcHealthCheckTimeout && ob._grpcHealthCheckTimeout !== 20) grpc.health_check_timeout = ob._grpcHealthCheckTimeout
    if (ob._grpcPermitWithoutStream) grpc.permit_without_stream = true
    if (ob._grpcInitialWindowsSize) grpc.initial_windows_size = ob._grpcInitialWindowsSize
    ss.grpcSettings = grpc
  }
  if (ob._network === 'httpupgrade') {
    const hu = {}
    if (ob._httpupgradePath) hu.path = ob._httpupgradePath
    if (ob._httpupgradeHost) hu.host = ob._httpupgradeHost
    if (ob._httpupgradeHeaders && ob._httpupgradeHeaders.length) {
      hu.headers = {}
      ob._httpupgradeHeaders.forEach(h => { if (h.key) hu.headers[h.key] = h.value || '' })
    }
    if (Object.keys(hu).length) ss.httpupgradeSettings = hu
  }
  if (ob._network === 'xhttp') {
    const xh = {}
    if (ob._xhttpPath) xh.path = ob._xhttpPath
    if (ob._xhttpHost) xh.host = ob._xhttpHost
    xh.mode = ob._xhttpMode || 'auto'
    // Extra
    const extra = {}
    if (ob._xhttpPaddingBytes && ob._xhttpPaddingBytes !== '100-1000') extra.xPaddingBytes = ob._xhttpPaddingBytes
    if (ob._xhttpNoGRPCHeader) extra.noGRPCHeader = true
    if (ob._xhttpHeaders && ob._xhttpHeaders.length) {
      extra.headers = {}
      ob._xhttpHeaders.forEach(h => { if (h.key) extra.headers[h.key] = h.value || '' })
    }
    // Packet-up
    if (ob._xhttpMode === 'packet-up') {
      if (ob._xhttpScMaxEachPostBytes && ob._xhttpScMaxEachPostBytes !== 1000000) extra.scMaxEachPostBytes = ob._xhttpScMaxEachPostBytes
      if (ob._xhttpScMinPostsIntervalMs && ob._xhttpScMinPostsIntervalMs !== 30) extra.scMinPostsIntervalMs = ob._xhttpScMinPostsIntervalMs
    }
    // Xmux
    const xmux = {}
    if (ob._xhttpXmuxMaxConcurrency && ob._xhttpXmuxMaxConcurrency !== '16-32') xmux.maxConcurrency = ob._xhttpXmuxMaxConcurrency
    if (ob._xhttpXmuxMaxConnections && ob._xhttpXmuxMaxConnections !== '0') xmux.maxConnections = Number(ob._xhttpXmuxMaxConnections)
    if (ob._xhttpXmuxCMaxReuseTimes && ob._xhttpXmuxCMaxReuseTimes !== '0') xmux.cMaxReuseTimes = Number(ob._xhttpXmuxCMaxReuseTimes)
    if (ob._xhttpXmuxHMaxRequestTimes && ob._xhttpXmuxHMaxRequestTimes !== '600-900') xmux.hMaxRequestTimes = ob._xhttpXmuxHMaxRequestTimes
    if (ob._xhttpXmuxHMaxReusableSecs && ob._xhttpXmuxHMaxReusableSecs !== '1800-3000') xmux.hMaxReusableSecs = ob._xhttpXmuxHMaxReusableSecs
    if (ob._xhttpXmuxHKeepAlivePeriod) xmux.hKeepAlivePeriod = ob._xhttpXmuxHKeepAlivePeriod
    if (Object.keys(xmux).length) extra.xmux = xmux
    // Download settings
    if (ob._xhttpDownloadEnabled && ob._xhttpDownloadAddress) {
      const dl = { address: ob._xhttpDownloadAddress, port: ob._xhttpDownloadPort || 443, network: 'xhttp' }
      if (ob._xhttpDownloadSecurity && ob._xhttpDownloadSecurity !== 'none') {
        dl.security = ob._xhttpDownloadSecurity
        if (ob._xhttpDownloadSecurity === 'tls') {
          const dlTls = {}
          if (ob._xhttpDownloadTlsServerName) dlTls.serverName = ob._xhttpDownloadTlsServerName
          if (ob._xhttpDownloadTlsAlpn && ob._xhttpDownloadTlsAlpn.length) dlTls.alpn = ob._xhttpDownloadTlsAlpn
          if (ob._xhttpDownloadTlsFingerprint) dlTls.fingerprint = ob._xhttpDownloadTlsFingerprint
          if (Object.keys(dlTls).length) dl.tlsSettings = dlTls
        }
        if (ob._xhttpDownloadSecurity === 'reality') {
          const dlR = {}
          if (ob._xhttpDownloadRealitySni) dlR.serverName = ob._xhttpDownloadRealitySni
          if (ob._xhttpDownloadRealityPassword) dlR.password = ob._xhttpDownloadRealityPassword
          if (ob._xhttpDownloadRealityShortId) dlR.shortId = ob._xhttpDownloadRealityShortId
          if (ob._xhttpDownloadRealityFingerprint) dlR.fingerprint = ob._xhttpDownloadRealityFingerprint
          if (Object.keys(dlR).length) dl.realitySettings = dlR
        }
      }
      if (ob._xhttpDownloadPath) {
        const dlXhttp = { path: ob._xhttpDownloadPath }
        // Download Xmux
        const dlXmux = {}
        if (ob._xhttpDownloadXmuxMaxConcurrency && ob._xhttpDownloadXmuxMaxConcurrency !== '16-32') dlXmux.maxConcurrency = ob._xhttpDownloadXmuxMaxConcurrency
        if (ob._xhttpDownloadXmuxMaxConnections && ob._xhttpDownloadXmuxMaxConnections !== '0') dlXmux.maxConnections = Number(ob._xhttpDownloadXmuxMaxConnections)
        if (ob._xhttpDownloadXmuxCMaxReuseTimes && ob._xhttpDownloadXmuxCMaxReuseTimes !== '0') dlXmux.cMaxReuseTimes = Number(ob._xhttpDownloadXmuxCMaxReuseTimes)
        if (ob._xhttpDownloadXmuxHMaxRequestTimes && ob._xhttpDownloadXmuxHMaxRequestTimes !== '600-900') dlXmux.hMaxRequestTimes = ob._xhttpDownloadXmuxHMaxRequestTimes
        if (ob._xhttpDownloadXmuxHMaxReusableSecs && ob._xhttpDownloadXmuxHMaxReusableSecs !== '1800-3000') dlXmux.hMaxReusableSecs = ob._xhttpDownloadXmuxHMaxReusableSecs
        if (ob._xhttpDownloadXmuxHKeepAlivePeriod) dlXmux.hKeepAlivePeriod = ob._xhttpDownloadXmuxHKeepAlivePeriod
        if (Object.keys(dlXmux).length) dlXhttp.extra = { xmux: dlXmux }
        dl.xhttpSettings = dlXhttp
      }
      extra.downloadSettings = dl
    }
    if (Object.keys(extra).length) xh.extra = extra
    if (Object.keys(xh).length) ss.xhttpSettings = xh
  }
  // Sockopt
  if (ob._sockoptEnabled) {
    const sk = {}
    if (ob._sockoptMark) sk.mark = ob._sockoptMark
    if (ob._sockoptTcpFastOpen) sk.tcpFastOpen = true
    if (ob._sockoptTproxy && ob._sockoptTproxy !== 'off') sk.tproxy = ob._sockoptTproxy
    if (ob._sockoptDomainStrategy) sk.domainStrategy = sk._sockoptDomainStrategy
    if (ob._sockoptInterface) sk.interface = ob._sockoptInterface
    if (ob._sockoptV6only) sk.V6Only = true
    sk.tcpKeepAliveIdle = ob._sockoptTcpKeepAliveIdle || 0
    sk.tcpKeepAliveInterval = ob._sockoptTcpKeepAliveInterval || 0
    sk.tcpUserTimeout = ob._sockoptTcpUserTimeout || 10000
    if (ob._sockoptTcpcongestion) sk.tcpcongestion = ob._sockoptTcpcongestion
    sk.tcpMaxSeg = ob._sockoptTcpMaxSeg || 1440
    sk.tcpWindowClamp = ob._sockoptTcpWindowClamp || 600
    if (ob._sockoptTcpMptcp) sk.tcpMptcp = true
    if (ob._sockoptAcceptProxyProtocol) sk.acceptProxyProtocol = true
    if (ob._sockoptDialerProxy) sk.dialerProxy = ob._sockoptDialerProxy
    if (ob._sockoptAddressPortStrategy) sk.addressPortStrategy = ob._sockoptAddressPortStrategy
    if (ob._sockoptTrustedXffStr) sk.trustedXForwardedFor = ob._sockoptTrustedXffStr.split('\n').map(s => s.trim()).filter(Boolean)
    // Happy Eyeballs
    if (ob._sockoptShowHappyEyeballs && ob._sockoptHappyEyeballsTryDelayMs > 0) {
      sk.happyEyeballs = {
        tryDelayMs: ob._sockoptHappyEyeballsTryDelayMs || 250,
        prioritizeIPv6: ob._sockoptHappyEyeballsPrioritizeIPv6 || false,
        interleave: ob._sockoptHappyEyeballsInterleave || 1,
        maxConcurrentTry: ob._sockoptHappyEyeballsMaxConcurrentTry || 4,
      }
    }
    // Custom Sockopt
    if (ob._sockoptShowCustomSockopt && ob._sockoptCustomSockopt && ob._sockoptCustomSockopt.length) {
      sk.customSockopt = ob._sockoptCustomSockopt.filter(c => c.opt).map(c => {
        const obj = { type: c.type || 'str', level: c.level || '6', opt: c.opt, value: c.value || '' }
        if (c.system) obj.system = c.system
        if (c.network) obj.network = c.network
        return obj
      })
    }
    ss.sockopt = sk
  }
  // FinalMask
  if (ob._finalmaskEnabled) {
    const toPacket = s => { const p = s._packetStr; if (!p) return []; return p.split(',').map(v => { const n = Number(v); return isNaN(n) ? v : n }) }
    const cleanInternal = s => { const r = {}; Object.keys(s).forEach(k => { if (!k.startsWith('_') && s[k] !== '' && s[k] !== undefined) r[k] = s[k] }); return r }
    const fm = {}
    // TCP masks
    if (ob._finalmaskTcp && ob._finalmaskTcp.length) {
      fm.tcp = ob._finalmaskTcp.filter(e => e.type).map(e => {
        const s = { ...e.settings }
        if (e.type === 'fragment') {
          if (s._lengthsStr) s.lengths = s._lengthsStr.split(/[\n,]+/).map(v => v.trim()).filter(Boolean)
          if (s._delaysStr) s.delays = s._delaysStr.split(/[\n,]+/).map(v => v.trim()).filter(Boolean)
        }
        if (e.type === 'header-custom') {
          const convertGroup = grp => (grp || []).map(items => (items || []).map(i => { const item = cleanInternal(i); if (i._packetStr) item.packet = toPacket(i); return item }))
          return { type: 'header-custom', settings: { clients: convertGroup(s.clients), servers: convertGroup(s.servers), errors: convertGroup(s.errors) } }
        }
        delete s._customTablesStr
        return { type: e.type, settings: s }
      })
    }
    // UDP masks
    if (ob._finalmaskUdp && ob._finalmaskUdp.length) {
      fm.udp = ob._finalmaskUdp.filter(e => e.type).map(e => {
        const s = { ...e.settings }
        if (e.type === 'header-custom') {
          const convertArr = arr => (arr || []).map(i => { const item = cleanInternal(i); if (i._packetStr) item.packet = toPacket(i); return item })
          return { type: 'header-custom', settings: { client: convertArr(s.client), server: convertArr(s.server) } }
        }
        if (e.type === 'noise') { s.noise = (s._noise || []).filter(n => n.rand).map(n => { const item = cleanInternal(n); if (n._packetStr) item.packet = toPacket(n); return item }); delete s._noise }
        if (e.type === 'xdns') { if (s._domainsStr) s.domains = s._domainsStr.split('\n').map(d => d.trim()).filter(Boolean); if (s._resolversStr) s.resolvers = s._resolversStr.split('\n').map(d => d.trim()).filter(Boolean); delete s._domainsStr; delete s._resolversStr }
        if (e.type === 'xicmp') { if (s._ipsStr) s.ips = s._ipsStr.split('\n').map(d => d.trim()).filter(Boolean); delete s._ipsStr }
        if (e.type === 'realm') {
          if (s._stunServersStr) s.stunServers = s._stunServersStr.split('\n').map(d => d.trim()).filter(Boolean)
          const tlsCfg = {}; if (s._tlsServerName) tlsCfg.serverName = s._tlsServerName; if (s._tlsAlpn && s._tlsAlpn.length) tlsCfg.alpn = s._tlsAlpn; if (s._tlsFingerprint) tlsCfg.fingerprint = s._tlsFingerprint; if (s._tlsAllowInsecure) tlsCfg.allowInsecure = true
          if (Object.keys(tlsCfg).length) s.tlsConfig = tlsCfg
          delete s._stunServersStr; delete s._tlsServerName; delete s._tlsAlpn; delete s._tlsFingerprint; delete s._tlsAllowInsecure
        }
        if (e.type === 'sudoku' && s._customTablesStr) { s.customTables = s._customTablesStr.split('\n').map(d => d.trim()).filter(Boolean); delete s._customTablesStr }
        return { type: e.type, settings: cleanInternal(s) }
      })
    }
    // QUIC Params
    const quicObj = {}
    if (ob._finalmaskCongestion) quicObj.congestion = ob._finalmaskCongestion
    if (ob._finalmaskBbrProfile) quicObj.bbrProfile = ob._finalmaskBbrProfile
    if (ob._finalmaskDebug) quicObj.debug = true
    if (ob._finalmaskBrutalUp) quicObj.brutalUp = ob._finalmaskBrutalUp
    if (ob._finalmaskBrutalDown) quicObj.brutalDown = ob._finalmaskBrutalDown
    if (ob._finalmaskShowUdpHop && (ob._finalmaskUdpHopPorts || ob._finalmaskUdpHopInterval)) quicObj.udpHop = { ports: ob._finalmaskUdpHopPorts || '', interval: ob._finalmaskUdpHopInterval || '' }
    if (ob._finalmaskMaxIdleTimeout !== '' && ob._finalmaskMaxIdleTimeout !== null && ob._finalmaskMaxIdleTimeout !== undefined) quicObj.maxIdleTimeout = ob._finalmaskMaxIdleTimeout
    if (ob._finalmaskKeepAlivePeriod !== '' && ob._finalmaskKeepAlivePeriod !== null && ob._finalmaskKeepAlivePeriod !== undefined) quicObj.keepAlivePeriod = ob._finalmaskKeepAlivePeriod
    if (ob._finalmaskDisablePathMTUDiscovery) quicObj.disablePathMTUDiscovery = true
    if (ob._finalmaskMaxIncomingStreams !== '' && ob._finalmaskMaxIncomingStreams !== null && ob._finalmaskMaxIncomingStreams !== undefined) quicObj.maxIncomingStreams = ob._finalmaskMaxIncomingStreams
    if (ob._finalmaskInitStreamWindow !== '' && ob._finalmaskInitStreamWindow !== null && ob._finalmaskInitStreamWindow !== undefined) quicObj.initStreamReceiveWindow = ob._finalmaskInitStreamWindow
    if (ob._finalmaskMaxStreamWindow !== '' && ob._finalmaskMaxStreamWindow !== null && ob._finalmaskMaxStreamWindow !== undefined) quicObj.maxStreamReceiveWindow = ob._finalmaskMaxStreamWindow
    if (ob._finalmaskInitConnWindow !== '' && ob._finalmaskInitConnWindow !== null && ob._finalmaskInitConnWindow !== undefined) quicObj.initConnectionReceiveWindow = ob._finalmaskInitConnWindow
    if (ob._finalmaskMaxConnWindow !== '' && ob._finalmaskMaxConnWindow !== null && ob._finalmaskMaxConnWindow !== undefined) quicObj.maxConnectionReceiveWindow = ob._finalmaskMaxConnWindow
    if (Object.keys(quicObj).length) fm.quicParams = quicObj
    if (Object.keys(fm).length) ss.finalmask = fm
  }
  return ss
}

async function loadInboundTags() {
  try {
    const data = await client.get(`/node-servers/${props.serverId}/inbound-configs`, { params: { core_type: props.coreType || 'xray' } })
    inboundTags.value = (data || []).map(i => i.tag).filter(Boolean)
  } catch {}
}

async function loadOutbounds() {
  try {
    const data = await client.get(`/node-servers/${props.serverId}/outbound-configs`, { params: { core_type: props.coreType || 'xray' } })
    form.value = (data || []).map(item => {
      try {
        const cfg = typeof item.config_json === 'string' ? JSON.parse(item.config_json) : (item.config_json || {})
        return { ...parseOutbound(cfg), _id: item.id, _enabled: item.enabled, _isDefault: item.is_default, _sortOrder: item.sort_order || 0 }
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

function emit() { /* no-op, saves via API on dialog confirm */ }

async function saveOutbound() {
  const outbound = buildOutbound(rf)
  const payload = {
    core_type: props.coreType || 'xray',
    remark: rf.tag || outbound.tag || '',
    protocol: outbound.protocol,
    tag: outbound.tag || 'outbound-' + Date.now(),
    config_json: outbound,
    enabled: rf._enabled !== false,
    is_default: false,
  }
  try {
    if (editIdx.value >= 0 && form.value[editIdx.value]._id) {
      await client.put(`/node-servers/${props.serverId}/outbound-configs/${form.value[editIdx.value]._id}`, payload)
    } else {
      await client.post(`/node-servers/${props.serverId}/outbound-configs`, payload)
    }

    // Apply to running xray via gRPC
    try {
      if (!(await ensureXrayRunning(client, props.serverId, t('protoCenter.outbounds.savedToDb')))) {
        ElMessage.success(t('common.success'))
        dlg.value = false
        loadOutbounds()
        return
      }
      // Remove existing outbound first if editing
      if (editIdx.value >= 0 && form.value[editIdx.value]._id) {
        await client.delete(`${nodeBase.value}/xray/control/outbound/${outbound.tag}`, { suppressErrorMessage: true }).catch(() => {})
      }
      // Add new outbound only if enabled
      if (rf._enabled !== false) {
        await client.post(`${nodeBase.value}/xray/control/outbound`, { outbound }, { suppressErrorMessage: true })
      }
    } catch (xrayErr) {
      console.warn('Failed to apply outbound to xray:', xrayErr)
      notifyXrayRuntimeApplyFailure(xrayErr, t('protoCenter.outbounds.savedToDb'))
    }

    ElMessage.success(t('common.success'))
    dlg.value = false
    loadOutbounds()
  } catch (e) { ElMessage.error(t('common.failed') + ': ' + (e.message || e)) }
}

async function removeOutbound(i) {
  const ob = form.value[i]
  if (ob._id) {
    try { await client.delete(`/node-servers/${props.serverId}/outbound-configs/${ob._id}`) } catch {}
  }
  // Remove from xray runtime
  if (ob.tag) {
    try {
      if (await ensureXrayRunning(client, props.serverId, t('protoCenter.outbounds.deletedFromDb'))) {
        await client.delete(`${nodeBase.value}/xray/control/outbound/${ob.tag}`, { suppressErrorMessage: true })
      }
    } catch (e) {
      notifyXrayRuntimeApplyFailure(e, t('protoCenter.outbounds.deletedFromDb'))
    }
  }
  form.value.splice(i, 1)
  loadOutbounds()
}

async function toggleEnabled(i) {
  const ob = form.value[i]
  if (!ob._id) return
  try {
    const outbound = buildOutbound(ob)
    await client.put(`/node-servers/${props.serverId}/outbound-configs/${ob._id}`, {
      remark: ob.tag || outbound.tag || '',
      protocol: outbound.protocol,
      tag: outbound.tag || ob.tag,
      config_json: outbound,
      enabled: ob._enabled,
      is_default: ob._isDefault || false,
    })

    // Apply to running xray via gRPC
    try {
      if (await ensureXrayRunning(client, props.serverId, '')) {
        if (ob._enabled) {
          // Enable: add outbound to xray runtime
          // Remove existing first to avoid duplicate
          try {
            await client.delete(`${nodeBase.value}/xray/control/outbound/${outbound.tag}`, { suppressErrorMessage: true })
          } catch {}
          await client.post(`${nodeBase.value}/xray/control/outbound`, { outbound }, { suppressErrorMessage: true })
        } else {
          // Disable: remove outbound from xray runtime
          await client.delete(`${nodeBase.value}/xray/control/outbound/${outbound.tag}`, { suppressErrorMessage: true })
        }
      }
    } catch (xrayErr) {
      console.warn('Failed to apply toggle to xray runtime:', xrayErr)
    }

    ElMessage.success(ob._enabled ? t('common.enabled') : t('common.disabled'))
  } catch (e) { 
    ob._enabled = !ob._enabled
    ElMessage.error(t('common.failed')) 
  }
}

async function moveOutbound(i, dir) {
  const j = i + dir
  if (j < 0 || j >= form.value.length) return
  // Swap in the local array first for immediate UI feedback
  const arr = form.value
  const tmp = arr[i]
  arr[i] = arr[j]
  arr[j] = tmp
  // Build reorder payload from current array order
  const orders = arr.map((ob, idx) => ({ id: ob._id, sort_order: idx }))
  try {
    await client.put(`/node-servers/${props.serverId}/outbound-configs/reorder`, { orders }, { params: { core_type: props.coreType || 'xray' } })
    // Update local sort orders
    arr.forEach((ob, idx) => { ob._sortOrder = idx })
    // Trigger config sync to running xray
    try {
      if (await ensureXrayRunning(client, props.serverId, '')) {
        // Reload outbounds in xray runtime by removing and re-adding all
        for (const ob of arr) {
          if (ob.tag && ob._enabled !== false) {
            try { await client.delete(`${nodeBase.value}/xray/control/outbound/${ob.tag}`, { suppressErrorMessage: true }) } catch {}
          }
        }
        for (const ob of arr) {
          if (ob._enabled !== false) {
            const outbound = buildOutbound(ob)
            try { await client.post(`${nodeBase.value}/xray/control/outbound`, { outbound }, { suppressErrorMessage: true }) } catch {}
          }
        }
      }
    } catch (xrayErr) {
      console.warn('Failed to apply reorder to xray runtime:', xrayErr)
    }
  } catch (e) {
    ElMessage.error(t('common.failed') + ': ' + (e.message || e))
    // Revert on failure
    loadOutbounds()
  }
}

function onProtocolChange() {
  rf.tag = genTag(rf.protocol)
  if (rf.protocol === 'freedom') rf._domainStrategy = 'AsIs'
  if (rf.protocol === 'wireguard') {
    rf._wgPeers = []
  }
  // Hysteria: bind network to hysteria, security to tls
  if (rf.protocol === 'hysteria') {
    rf._network = 'hysteria'
    rf._securityMode = 'tls'
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
    const data = await client.post(`${apiBase.value}/wg-key?core_type=` + (props.coreType || 'xray'))
    if (data && data.privateKey) {
      rf._wgSecretKey = data.privateKey
      rf._wgPublicKey = data.publicKey || ''
      ElMessage.success(t('common.success'))
    }
  } catch (e) { ElMessage.error(t('common.failed') + ': ' + (e.message || e)) }
}



function onFinalmaskUdpHopToggle(val) {
  // Outbound FinalMask intentionally keeps optional fields empty until selected.
}

function onOutboundCongestionChange(val) {
  emit()
}

function parseFinalmask(fm) {
  const defQuic = { congestion: '', bbrProfile: '', debug: false, brutalUp: '', brutalDown: '', _showUdpHop: false, udpHop: { ports: '', interval: '' }, maxIdleTimeout: '', keepAlivePeriod: '', disablePathMTUDiscovery: false, maxIncomingStreams: '', initStreamReceiveWindow: '', maxStreamReceiveWindow: '', initConnectionReceiveWindow: '', maxConnectionReceiveWindow: '' }
  if (!fm || Object.keys(fm).length === 0) return { _enabled: false, tcp: [], udp: [], quicParams: defQuic }
  const qp = fm.quicParams || {}
  const parseTcpEntry = e => {
    const s = { ...(e.settings || {}) }
    if (e.type === 'header-custom') {
      const fixGroup = g => (g || []).map(items => (items || []).map(i => ({ delay: i.delay || 0, rand: i.rand || 0, randRange: i.randRange || '0-255', type: i.type || 'array', _packetStr: Array.isArray(i.packet) ? i.packet.join(',') : (i.packet || '') })))
      return { type: 'header-custom', settings: { clients: fixGroup(s.clients), servers: fixGroup(s.servers), errors: fixGroup(s.errors) }, _json: '' }
    }
    if (e.type === 'fragment') {
      const lengths = Array.isArray(s.lengths) ? s.lengths : (s.length ? [s.length] : [])
      const delays = Array.isArray(s.delays) ? s.delays : (s.delay ? [s.delay] : [])
      s._lengthsStr = lengths.join('\n')
      s._delaysStr = delays.join('\n')
      delete s.length
      delete s.delay
    }
    return { type: e.type || '', settings: s, _json: '' }
  }
  const parseUdpEntry = e => {
    const s = e.settings || {}
    if (e.type === 'header-custom') {
      const fixArr = arr => (arr || []).map(i => ({ rand: i.rand || 0, randRange: i.randRange || '0-255', type: i.type || 'array', _packetStr: Array.isArray(i.packet) ? i.packet.join(',') : (i.packet || '') }))
      return { type: 'header-custom', settings: { client: fixArr(s.client), server: fixArr(s.server) }, _json: '' }
    }
    const base = { type: e.type || '', settings: { ...s }, _json: '' }
    if (e.type === 'noise') base.settings._noise = (s.noise || []).map(n => ({ type: n.type || 'array', rand: n.rand || '', randRange: n.randRange || '0-255', delay: n.delay || '', _packetStr: Array.isArray(n.packet) ? n.packet.join(',') : (n.packet || '') }))
    if (e.type === 'xdns') { base.settings._domainsStr = (s.domains || []).join('\n'); base.settings._resolversStr = (s.resolvers || []).join('\n') }
    if (e.type === 'xicmp') base.settings._ipsStr = (s.ips || []).join('\n')
    if (e.type === 'realm') {
      base.settings._stunServersStr = (s.stunServers || []).join('\n')
      const tls = s.tlsConfig || {}
      base.settings._tlsServerName = tls.serverName || ''
      base.settings._tlsAlpn = tls.alpn || []
      base.settings._tlsFingerprint = tls.fingerprint || ''
      base.settings._tlsAllowInsecure = tls.allowInsecure || false
    }
    if (e.type === 'sudoku') base.settings._customTablesStr = (s.customTables || []).join('\n')
    if (e.type === 'salamander') {
      const ps = s.packetSize || ''
      const match = /^(\d+)-(\d+)$/.exec(ps)
      base.settings._gecko = !!match
      base.settings._geckoMin = match ? Number(match[1]) : 512
      base.settings._geckoMax = match ? Number(match[2]) : 1200
    }
    return base
  }
  return {
    _enabled: true,
    tcp: (fm.tcp || []).map(parseTcpEntry),
    udp: (fm.udp || []).map(parseUdpEntry),
    quicParams: {
      congestion: qp.congestion || '', bbrProfile: qp.bbrProfile || '', debug: qp.debug || false,
      brutalUp: qp.brutalUp || '', brutalDown: qp.brutalDown || '',
      _showUdpHop: !!qp.udpHop, udpHop: qp.udpHop || { ports: '', interval: '' },
      maxIdleTimeout: qp.maxIdleTimeout ?? '', keepAlivePeriod: qp.keepAlivePeriod ?? '',
      disablePathMTUDiscovery: qp.disablePathMTUDiscovery || false, maxIncomingStreams: qp.maxIncomingStreams ?? '',
      initStreamReceiveWindow: qp.initStreamReceiveWindow ?? '', maxStreamReceiveWindow: qp.maxStreamReceiveWindow ?? '',
      initConnectionReceiveWindow: qp.initConnectionReceiveWindow ?? '', maxConnectionReceiveWindow: qp.maxConnectionReceiveWindow ?? '',
    },
  }
}

function newClientServerItem() { return { delay: 0, rand: 0, randRange: '0-255', type: 'array', _packetStr: '' } }
function newUdpClientServerItem() { return { rand: 0, randRange: '0-255', type: 'array', _packetStr: '' } }
function onRandChange(item) { if (item.rand) item._packetStr = '' }
function onPacketChange(item) { if (item._packetStr) item.rand = 0 }
function onGeckoToggle(entry) {
  if (entry.settings._gecko) { entry.settings._geckoMin = 512; entry.settings._geckoMax = 1200; entry.settings.packetSize = '512-1200' }
  else { delete entry.settings.packetSize; delete entry.settings._geckoMin; delete entry.settings._geckoMax }
}
function onGeckoSizeChange(entry) { entry.settings.packetSize = (entry.settings._geckoMin || 1) + '-' + (entry.settings._geckoMax || 2048) }

function onOutboundMaskTypeChange(entry, protocol) {
  entry.settings = {}
  entry._json = ''
  if (entry.type === 'fragment') entry.settings = { packets: 'tlshello', _lengthsStr: '', _delaysStr: '', maxSplit: '' }
  if (entry.type === 'sudoku') entry.settings = { password: '', ascii: '', customTable: '', _customTablesStr: '', paddingMin: 0, paddingMax: 0 }
  if (entry.type === 'header-custom') {
    if (protocol === 'tcp') entry.settings = { clients: [[newClientServerItem()]], servers: [[]], errors: [[]] }
    else entry.settings = { client: [newUdpClientServerItem()], server: [newUdpClientServerItem()] }
  }
  if (entry.type === 'salamander') entry.settings = { password: '', _gecko: false, _geckoMin: 512, _geckoMax: 1200 }
  if (entry.type === 'mkcp-legacy') entry.settings = { header: '', value: '' }
  if (entry.type === 'noise') entry.settings = { reset: 0, _noise: [{ type: 'array', rand: '1-8192', randRange: '0-255', delay: '10-20', _packetStr: '' }] }
  if (entry.type === 'xdns') entry.settings = { _domainsStr: '', _resolversStr: '' }
  if (entry.type === 'xicmp') entry.settings = { dgram: false, _ipsStr: '' }
  if (entry.type === 'realm') entry.settings = { url: '', _stunServersStr: '', _tlsServerName: '', _tlsAlpn: [], _tlsFingerprint: '', _tlsAllowInsecure: false }
}

async function importWarp() {
  try {
    const coreType = props.coreType || 'xray'
    const warpBase = `/node-servers/${props.serverId}/warp`
    // Check WARP status for this node + core type
    let status
    try { status = await client.get(`${warpBase}/status`, { params: { core_type: coreType } }) } catch { status = null }
    // Register if not registered
    if (!status || !status.registered) {
      ElMessage.info(t('protoCenter.outbounds.warpRegistering'))
      status = await client.post(`${warpBase}/register`, { core_type: coreType })
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
