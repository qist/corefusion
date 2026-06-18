<template>
  <div>
    <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:16px">
      <span style="font-size:15px;font-weight:600">{{ t('protoCenter.inbounds.title') }}</span>
      <el-button size="small" type="primary" @click="showAdd">{{ t('protoCenter.inbounds.add') }}</el-button>
    </div>

    <!-- List -->
    <div v-for="(ib, i) in form" :key="i" class="inbound-item">
      <div style="display:flex;justify-content:space-between;align-items:center">
        <div style="display:flex;align-items:center;gap:12px">
          <el-switch v-model="ib._enabled" @change="toggleEnabled(i)" />
          <el-tag size="small">{{ ib.protocol }}</el-tag>
          <span style="font-size:13px">{{ ib.listen || '0.0.0.0' }}:{{ ib.port }}</span>
          <el-tag v-if="ib._remark" size="small">{{ ib._remark }}</el-tag>
          <el-tag v-if="ib.tag" size="small" type="info">{{ ib.tag }}</el-tag>
        </div>
        <div>
          <el-button size="small" @click="showEdit(i)">{{ t('common.edit') }}</el-button>
          <el-button size="small" type="danger" text @click="rmInbound(i)">X</el-button>
        </div>
      </div>
    </div>

    <!-- Dialog -->
    <el-dialog v-model="dlg" :title="editIdx >= 0 ? t('protoCenter.inbounds.edit') : t('protoCenter.inbounds.add')" width="750" :close-on-click-modal="false" top="5vh">
      <el-tabs v-model="activeTab">
        <!-- Basic Tab -->
        <el-tab-pane :label="t('protoCenter.inbounds.basic')" name="basic">
          <el-form label-width="120px" label-position="left">
            <el-form-item :label="t('protoCenter.inbounds.remark')">
              <el-input v-model="rf._remark" />
            </el-form-item>
            <el-form-item :label="t('protoCenter.inbounds.tag')">
              <el-input v-model="rf.tag" />
            </el-form-item>
            <el-form-item :label="t('protoCenter.inbounds.protocol')">
              <el-select v-model="rf.protocol" style="width:100%" @change="onProtocolChange">
                <el-option label="VLESS" value="vless" />
                <el-option label="VMess" value="vmess" />
                <el-option label="Trojan" value="trojan" />
                <el-option label="Shadowsocks" value="shadowsocks" />
                <el-option label="SOCKS" value="socks" />
                <el-option label="HTTP" value="http" />
                <el-option label="WireGuard" value="wireguard" />
                <el-option label="Hysteria" value="hysteria" />
                <el-option label="TUN" value="tun" />
                <el-option label="Tunnel" value="tunnel" />
                <el-option label="Dokodemo-door" value="dokodemo-door" />
              </el-select>
            </el-form-item>
            <el-form-item :label="t('protoCenter.inbounds.listenType')">
              <el-radio-group v-model="rf._listenType" @change="onListenTypeChange">
                <el-radio value="tcp">TCP</el-radio>
                <el-radio value="sock">Unix Socket</el-radio>
              </el-radio-group>
            </el-form-item>
            <el-form-item v-if="rf._listenType === 'tcp'" :label="t('protoCenter.inbounds.listen')">
              <el-input v-model="rf.listen" placeholder="0.0.0.0" />
            </el-form-item>
            <el-form-item v-if="rf._listenType === 'tcp'" :label="t('protoCenter.inbounds.port')">
              <el-input-number v-model="rf.port" :min="1" :max="65535" />
            </el-form-item>
            <el-form-item v-if="rf._listenType === 'sock'" :label="t('protoCenter.inbounds.socketPath')">
              <el-input v-model="rf._socketPath" :placeholder="'/dev/shm/' + rf.protocol + '.sock'" />
            </el-form-item>
            <el-form-item v-if="rf._listenType === 'sock'" :label="t('protoCenter.inbounds.socketPerm')">
              <el-input v-model="rf._socketPerm" placeholder="0666" />
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <!-- Protocol Tab -->
        <el-tab-pane :label="t('protoCenter.inbounds.protocolSettings')" name="protocol">
          <!-- VLESS -->
          <template v-if="rf.protocol === 'vless'">
            <el-form label-width="120px" label-position="left">
              <el-form-item label="Decryption">
                <el-input v-model="rf.settings.decryption" placeholder="none" />
              </el-form-item>
              <el-divider content-position="left">
                Users
                <el-button size="small" type="primary" @click="addUser('vless')" style="margin-left:12px">+ Add User</el-button>
              </el-divider>
              <div v-for="(u, ui) in rf.settings.users" :key="ui" class="user-item">
                <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:8px">
                  <span style="font-weight:600">User #{{ ui + 1 }}</span>
                  <el-button size="small" type="danger" text @click="rf.settings.users.splice(ui,1)">X</el-button>
                </div>
                <el-form label-width="100px" label-position="left" size="small">
                  <el-form-item label="UUID">
                    <div style="display:flex;gap:8px">
                      <el-input v-model="u.id" placeholder="UUID" style="flex:1" />
                      <el-button size="small" @click="u.id = genUUID()">Generate</el-button>
                    </div>
                  </el-form-item>
                  <el-form-item label="Email">
                    <el-input v-model="u.email" placeholder="user@xray.com" />
                  </el-form-item>
                  <el-form-item label="Flow">
                    <el-select v-model="u.flow" style="width:100%" clearable>
                      <el-option label="none" value="" />
                      <el-option label="xtls-rprx-vision" value="xtls-rprx-vision" />
                    </el-select>
                  </el-form-item>
                  <el-form-item label="Level">
                    <el-input-number v-model="u.level" :min="0" />
                  </el-form-item>
                  <el-divider content-position="left">Traffic & Expiry</el-divider>
                  <el-form-item label="Traffic Quota">
                    <el-input v-model="u._quota" placeholder="100GB, unlimited" />
                  </el-form-item>
                  <el-form-item label="Traffic Used">
                    <div style="display:flex;gap:8px;align-items:center">
                      <span style="font-size:13px">↑ {{ formatTraffic(u._usedUp || 0) }} ↓ {{ formatTraffic(u._usedDown || 0) }}</span>
                      <el-button size="small" type="warning" plain @click="resetTraffic(u)">Reset</el-button>
                    </div>
                  </el-form-item>
                  <el-form-item label="Expiry Time">
                    <el-date-picker v-model="u._expiry" type="datetime" placeholder="Select date" style="width:100%" />
                  </el-form-item>
                </el-form>
              </div>
            </el-form>
          </template>

          <!-- VMess -->
          <template v-if="rf.protocol === 'vmess'">
            <el-form label-width="120px" label-position="left">
              <el-divider content-position="left">
                Users
                <el-button size="small" type="primary" @click="addUser('vmess')" style="margin-left:12px">+ Add User</el-button>
              </el-divider>
              <div v-for="(u, ui) in rf.settings.users" :key="ui" class="user-item">
                <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:8px">
                  <span style="font-weight:600">User #{{ ui + 1 }}</span>
                  <el-button size="small" type="danger" text @click="rf.settings.users.splice(ui,1)">X</el-button>
                </div>
                <el-form label-width="100px" label-position="left" size="small">
                  <el-form-item label="UUID">
                    <div style="display:flex;gap:8px">
                      <el-input v-model="u.id" placeholder="UUID" style="flex:1" />
                      <el-button size="small" @click="u.id = genUUID()">Generate</el-button>
                    </div>
                  </el-form-item>
                  <el-form-item label="Email">
                    <el-input v-model="u.email" placeholder="user@xray.com" />
                  </el-form-item>
                  <el-form-item label="AlterID">
                    <el-input-number v-model="u.alterId" :min="0" />
                  </el-form-item>
                  <el-form-item label="Security">
                    <el-select v-model="u.security" style="width:100%">
                      <el-option label="auto" value="auto" />
                      <el-option label="aes-128-gcm" value="aes-128-gcm" />
                      <el-option label="chacha20-poly1305" value="chacha20-poly1305" />
                      <el-option label="none" value="none" />
                    </el-select>
                  </el-form-item>
                  <el-form-item label="Level">
                    <el-input-number v-model="u.level" :min="0" />
                  </el-form-item>
                  <el-divider content-position="left">Traffic & Expiry</el-divider>
                  <el-form-item label="Traffic Quota">
                    <el-input v-model="u._quota" placeholder="100GB, unlimited" />
                  </el-form-item>
                  <el-form-item label="Traffic Used">
                    <div style="display:flex;gap:8px;align-items:center">
                      <span style="font-size:13px">↑ {{ formatTraffic(u._usedUp || 0) }} ↓ {{ formatTraffic(u._usedDown || 0) }}</span>
                      <el-button size="small" type="warning" plain @click="resetTraffic(u)">Reset</el-button>
                    </div>
                  </el-form-item>
                  <el-form-item label="Expiry Time">
                    <el-date-picker v-model="u._expiry" type="datetime" placeholder="Select date" style="width:100%" />
                  </el-form-item>
                </el-form>
              </div>
            </el-form>
          </template>

          <!-- Trojan -->
          <template v-if="rf.protocol === 'trojan'">
            <el-form label-width="120px" label-position="left">
              <el-divider content-position="left">
                Users
                <el-button size="small" type="primary" @click="addUser('trojan')" style="margin-left:12px">+ Add User</el-button>
              </el-divider>
              <div v-for="(u, ui) in rf.settings.users" :key="ui" class="user-item">
                <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:8px">
                  <span style="font-weight:600">User #{{ ui + 1 }}</span>
                  <el-button size="small" type="danger" text @click="rf.settings.users.splice(ui,1)">X</el-button>
                </div>
                <el-form label-width="100px" label-position="left" size="small">
                  <el-form-item label="Password">
                    <div style="display:flex;gap:8px">
                      <el-input v-model="u.password" style="flex:1" />
                      <el-button size="small" @click="u.password = genBase64Password()">Generate</el-button>
                    </div>
                  </el-form-item>
                  <el-form-item label="Email">
                    <el-input v-model="u.email" placeholder="user@xray.com" />
                  </el-form-item>
                  <el-form-item label="Level">
                    <el-input-number v-model="u.level" :min="0" />
                  </el-form-item>
                  <el-divider content-position="left">Traffic & Expiry</el-divider>
                  <el-form-item label="Traffic Quota">
                    <el-input v-model="u._quota" placeholder="100GB, unlimited" />
                  </el-form-item>
                  <el-form-item label="Traffic Used">
                    <div style="display:flex;gap:8px;align-items:center">
                      <span style="font-size:13px">↑ {{ formatTraffic(u._usedUp || 0) }} ↓ {{ formatTraffic(u._usedDown || 0) }}</span>
                      <el-button size="small" type="warning" plain @click="resetTraffic(u)">Reset</el-button>
                    </div>
                  </el-form-item>
                  <el-form-item label="Expiry Time">
                    <el-date-picker v-model="u._expiry" type="datetime" placeholder="Select date" style="width:100%" />
                  </el-form-item>
                </el-form>
              </div>
            </el-form>
          </template>

          <!-- Shadowsocks -->
          <template v-if="rf.protocol === 'shadowsocks'">
            <el-form label-width="120px" label-position="left">
              <el-form-item label="Method">
                <el-select v-model="rf.settings.method" style="width:100%">
                  <el-option label="aes-256-gcm" value="aes-256-gcm" />
                  <el-option label="aes-128-gcm" value="aes-128-gcm" />
                  <el-option label="chacha20-poly1305" value="chacha20-poly1305" />
                  <el-option label="2022-blake3-aes-128-gcm" value="2022-blake3-aes-128-gcm" />
                  <el-option label="2022-blake3-aes-256-gcm" value="2022-blake3-aes-256-gcm" />
                </el-select>
              </el-form-item>
              <el-form-item label="Password">
                <div style="display:flex;gap:8px">
                  <el-input v-model="rf.settings.password" style="flex:1" />
                  <el-button size="small" @click="rf.settings.password = genBase64Password()">Generate</el-button>
                </div>
              </el-form-item>
              <el-form-item label="Network">
                <el-select v-model="rf.settings.network" style="width:100%">
                  <el-option label="tcp" value="tcp" />
                  <el-option label="udp" value="udp" />
                  <el-option label="tcp,udp" value="tcp,udp" />
                </el-select>
              </el-form-item>
            </el-form>

            <!-- Multi-user clients (for 2022-blake3 methods) -->
            <el-divider content-position="left">
              Clients
              <el-button size="small" type="primary" @click="addUser('shadowsocks')" style="margin-left:12px">+ Add Client</el-button>
            </el-divider>
            <div v-for="(c, ci) in rf.settings.clients" :key="ci" class="user-item">
              <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:8px">
                <span style="font-weight:600">Client #{{ ci + 1 }}</span>
                <el-button size="small" type="danger" text @click="rf.settings.clients.splice(ci,1)">X</el-button>
              </div>
              <el-form label-width="100px" label-position="left" size="small">
                <el-form-item label="Password">
                  <div style="display:flex;gap:8px">
                    <el-input v-model="c.password" style="flex:1" />
                    <el-button size="small" @click="c.password = genBase64Password()">Generate</el-button>
                  </div>
                </el-form-item>
                <el-form-item label="Email">
                  <el-input v-model="c.email" placeholder="user@xray.com" />
                </el-form-item>
                <el-divider content-position="left">Traffic & Expiry</el-divider>
                <el-form-item label="Traffic Quota">
                  <el-input v-model="c._quota" placeholder="100GB, unlimited" />
                </el-form-item>
                <el-form-item label="Traffic Used">
                  <div style="display:flex;gap:8px;align-items:center">
                    <span style="font-size:13px">↑ {{ formatTraffic(c._usedUp || 0) }} ↓ {{ formatTraffic(c._usedDown || 0) }}</span>
                    <el-button size="small" type="warning" plain @click="resetTraffic(c)">Reset</el-button>
                  </div>
                </el-form-item>
                <el-form-item label="Expiry Time">
                  <el-date-picker v-model="c._expiry" type="datetime" placeholder="Select date" style="width:100%" />
                </el-form-item>
              </el-form>
            </div>
          </template>

          <!-- SOCKS -->
          <template v-if="rf.protocol === 'socks'">
            <el-form label-width="120px" label-position="left">
              <el-form-item label="Auth">
                <el-select v-model="rf.settings.auth" style="width:100%" @change="onSocksAuthChange">
                  <el-option label="noauth" value="noauth" />
                  <el-option label="password" value="password" />
                </el-select>
              </el-form-item>
              <template v-if="rf.settings.auth === 'password'">
                <el-divider content-position="left">Accounts</el-divider>
                <div v-for="(a, ai) in rf.settings.accounts" :key="ai" class="user-item">
                  <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:8px">
                    <span style="font-weight:600">Account #{{ ai + 1 }}</span>
                    <el-button size="small" type="danger" text @click="rf.settings.accounts.splice(ai,1)">X</el-button>
                  </div>
                  <el-form label-width="100px" label-position="left" size="small">
                    <el-form-item label="User"><el-input v-model="a.user" /></el-form-item>
                    <el-form-item label="Pass"><el-input v-model="a.pass" /></el-form-item>
                    <el-divider content-position="left">Traffic & Expiry</el-divider>
                    <el-form-item label="Traffic Quota"><el-input v-model="a._quota" placeholder="100GB, unlimited" /></el-form-item>
                    <el-form-item label="Traffic Used">
                      <div style="display:flex;gap:8px;align-items:center">
                        <span style="font-size:13px">↑ {{ formatTraffic(a._usedUp || 0) }} ↓ {{ formatTraffic(a._usedDown || 0) }}</span>
                        <el-button size="small" type="warning" plain @click="resetTraffic(a)">Reset</el-button>
                      </div>
                    </el-form-item>
                    <el-form-item label="Expiry Time"><el-date-picker v-model="a._expiry" type="datetime" placeholder="Select date" style="width:100%" /></el-form-item>
                  </el-form>
                </div>
                <el-button size="small" @click="rf.settings.accounts.push({user:genEmail().split('@')[0],pass:genPassword(),_quota:'unlimited',_expiry:null,_usedUp:0,_usedDown:0})">+ Add Account</el-button>
              </template>
              <el-form-item label="UDP">
                <el-switch v-model="rf.settings.udp" />
              </el-form-item>
            </el-form>
          </template>

          <!-- HTTP -->
          <template v-if="rf.protocol === 'http'">
            <el-divider content-position="left">Accounts</el-divider>
            <div v-for="(a, ai) in rf.settings.accounts" :key="ai" class="user-item">
              <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:8px">
                <span style="font-weight:600">Account #{{ ai + 1 }}</span>
                <el-button size="small" type="danger" text @click="rf.settings.accounts.splice(ai,1)">X</el-button>
              </div>
              <el-form label-width="100px" label-position="left" size="small">
                <el-form-item label="User"><el-input v-model="a.user" /></el-form-item>
                <el-form-item label="Pass"><el-input v-model="a.pass" /></el-form-item>
                <el-divider content-position="left">Traffic & Expiry</el-divider>
                <el-form-item label="Traffic Quota"><el-input v-model="a._quota" placeholder="100GB, unlimited" /></el-form-item>
                <el-form-item label="Traffic Used">
                  <div style="display:flex;gap:8px;align-items:center">
                    <span style="font-size:13px">↑ {{ formatTraffic(a._usedUp || 0) }} ↓ {{ formatTraffic(a._usedDown || 0) }}</span>
                    <el-button size="small" type="warning" plain @click="resetTraffic(a)">Reset</el-button>
                  </div>
                </el-form-item>
                <el-form-item label="Expiry Time"><el-date-picker v-model="a._expiry" type="datetime" placeholder="Select date" style="width:100%" /></el-form-item>
              </el-form>
            </div>
            <el-button size="small" @click="rf.settings.accounts.push({user:genEmail().split('@')[0],pass:genPassword(),_quota:'unlimited',_expiry:null,_usedUp:0,_usedDown:0})">+ Add Account</el-button>
            <el-divider content-position="left">Options</el-divider>
            <el-form label-width="140px" label-position="left">
              <el-form-item label="Allow Transparent">
                <el-switch v-model="rf.settings.allowTransparent" />
              </el-form-item>
            </el-form>
          </template>

          <!-- WireGuard -->
          <template v-if="rf.protocol === 'wireguard'">
            <el-form label-width="120px" label-position="left">
              <el-form-item label="Secret Key">
                <div style="display:flex;gap:8px">
                  <el-input v-model="rf.settings.secretKey" show-password style="flex:1" />
                  <el-button size="small" @click="genWgKeyPair()">Generate</el-button>
                </div>
              </el-form-item>
              <el-form-item label="Public Key">
                <el-input v-model="rf._wgPublicKey" readonly />
                <div class="hint">从私钥自动派生</div>
              </el-form-item>
              <el-form-item label="MTU">
                <el-input-number v-model="rf.settings.mtu" :min="0" />
                <div class="hint">默认 1420</div>
              </el-form-item>
              <el-form-item label="No Kernel TUN">
                <el-switch v-model="rf.settings.noKernelTun" />
              </el-form-item>
              <el-form-item label="Workers">
                <el-input-number v-model="rf.settings.workers" :min="0" />
              </el-form-item>
              <el-divider content-position="left">Peers</el-divider>
              <div v-for="(p, pi) in rf.settings.peers" :key="pi" class="user-item">
                <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:8px">
                  <span style="font-weight:600">Peer #{{ pi + 1 }}</span>
                  <el-button size="small" type="danger" text @click="rf.settings.peers.splice(pi,1)">X</el-button>
                </div>
                <el-form label-width="100px" label-position="left" size="small">
                  <el-form-item label="Public Key"><el-input v-model="p.publicKey" /></el-form-item>
                  <el-form-item label="Allowed IPs"><el-input v-model="p._allowedIPsStr" placeholder="0.0.0.0/0, ::/0" /></el-form-item>
                  <el-form-item label="Keep Alive"><el-input-number v-model="p.keepAlive" :min="0" /></el-form-item>
                  <el-form-item label="Comment"><el-input v-model="p.comment" placeholder="备注，如 iPhone、笔记本" /></el-form-item>
                </el-form>
              </div>
              <el-button size="small" @click="rf.settings.peers.push({publicKey:'',_allowedIPsStr:'0.0.0.0/0, ::/0',keepAlive:0,comment:''})">+ Add Peer</el-button>
            </el-form>
          </template>

          <!-- Hysteria -->
          <template v-if="rf.protocol === 'hysteria'">
            <el-form label-width="120px" label-position="left">
              <el-form-item label="Version">
                <el-input-number v-model="rf.settings.version" :min="2" :max="2" />
              </el-form-item>
              <el-divider content-position="left">
                Users
                <el-button size="small" type="primary" @click="addUser('hysteria')" style="margin-left:12px">+ Add User</el-button>
              </el-divider>
              <div v-for="(u, ui) in rf.settings.users" :key="ui" class="user-item">
                <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:8px">
                  <span style="font-weight:600">User #{{ ui + 1 }}</span>
                  <el-button size="small" type="danger" text @click="rf.settings.users.splice(ui,1)">X</el-button>
                </div>
                <el-form label-width="100px" label-position="left" size="small">
                  <el-form-item label="Auth">
                    <div style="display:flex;gap:8px">
                      <el-input v-model="u.auth" style="flex:1" />
                      <el-button size="small" @click="u.auth = genBase64Password()">Generate</el-button>
                    </div>
                  </el-form-item>
                  <el-form-item label="Email">
                    <el-input v-model="u.email" placeholder="user@xray.com" />
                  </el-form-item>
                  <el-form-item label="Level">
                    <el-input-number v-model="u.level" :min="0" />
                  </el-form-item>
                  <el-divider content-position="left">Traffic & Expiry</el-divider>
                  <el-form-item label="Traffic Quota">
                    <el-input v-model="u._quota" placeholder="100GB, unlimited" />
                  </el-form-item>
                  <el-form-item label="Traffic Used">
                    <div style="display:flex;gap:8px;align-items:center">
                      <span style="font-size:13px">↑ {{ formatTraffic(u._usedUp || 0) }} ↓ {{ formatTraffic(u._usedDown || 0) }}</span>
                      <el-button size="small" type="warning" plain @click="resetTraffic(u)">Reset</el-button>
                    </div>
                  </el-form-item>
                  <el-form-item label="Expiry Time">
                    <el-date-picker v-model="u._expiry" type="datetime" placeholder="Select date" style="width:100%" />
                  </el-form-item>
                </el-form>
              </div>
            </el-form>
          </template>

          <!-- TUN -->
          <template v-if="rf.protocol === 'tun'">
            <el-form label-width="180px" label-position="left">
              <el-form-item label="Name">
                <el-input v-model="rf.settings.name" placeholder="xray0" />
                <div class="hint">TUN 接口名，默认 xray0</div>
              </el-form-item>
              <el-form-item label="MTU">
                <el-input-number v-model="rf.settings.mtu" :min="0" />
                <div class="hint">默认 1500</div>
              </el-form-item>
              <el-form-item label="Gateway">
                <el-input v-model="rf._tunGatewayStr" placeholder="10.0.0.1/16, fc00::1/64" />
                <div class="hint">逗号分隔，IPv4/IPv6 地址前缀</div>
              </el-form-item>
              <el-form-item label="DNS">
                <el-input v-model="rf._tunDnsStr" placeholder="1.1.1.1, 8.8.8.8" />
                <div class="hint">逗号分隔，TUN 接口 DNS 服务器</div>
              </el-form-item>
              <el-form-item label="User Level">
                <el-input-number v-model="rf.settings.userLevel" :min="0" />
              </el-form-item>
              <el-form-item label="Auto System Routing Table">
                <el-input v-model="rf._tunAutoRoutingStr" placeholder="0.0.0.0/0, ::/0" />
                <div class="hint">逗号分隔，仅 Windows，自动写入系统路由表</div>
              </el-form-item>
              <el-form-item label="Auto Outbounds Interface">
                <el-input v-model="rf.settings.autoOutboundsInterface" placeholder="auto" />
                <div class="hint">避免回环，auto = 自动选择</div>
              </el-form-item>
            </el-form>
          </template>

          <!-- Tunnel / Dokodemo-door -->
          <template v-if="rf.protocol === 'tunnel' || rf.protocol === 'dokodemo-door'">
            <el-form label-width="140px" label-position="left">
              <el-form-item label="Allowed Network">
                <el-select v-model="rf.settings.allowedNetwork" style="width:100%">
                  <el-option label="tcp" value="tcp" /><el-option label="udp" value="udp" /><el-option label="tcp,udp" value="tcp,udp" />
                </el-select>
              </el-form-item>
              <el-form-item label="Rewrite Address">
                <el-input v-model="rf.settings.rewriteAddress" placeholder="8.8.8.8 or domain.com" />
                <div class="hint">转发目标地址，留空为 localhost</div>
              </el-form-item>
              <el-form-item label="Rewrite Port">
                <el-input-number v-model="rf.settings.rewritePort" :min="0" :max="65535" />
                <div class="hint">转发目标端口，0 = 监听端口</div>
              </el-form-item>
              <el-form-item label="Follow Redirect">
                <el-switch v-model="rf.settings.followRedirect" />
                <div class="hint">识别 iptables 转发的数据</div>
              </el-form-item>
              <el-form-item label="User Level">
                <el-input-number v-model="rf.settings.userLevel" :min="0" />
              </el-form-item>
              <el-form-item label="Port Map">
                <div v-for="(m, i) in rf._portMap" :key="i" style="display:flex;gap:8px;margin-bottom:4px">
                  <el-input v-model="m.local" placeholder="5555" style="width:30%" />
                  <el-input v-model="m.remote" placeholder="1.1.1.1:7777" style="width:50%" />
                  <el-button size="small" type="danger" text @click="rf._portMap.splice(i,1)">X</el-button>
                </div>
                <el-button size="small" @click="rf._portMap.push({local:'',remote:''})">+ Add Mapping</el-button>
                <div class="hint">本地端口 → 远程地址:端口</div>
              </el-form-item>
            </el-form>
          </template>
        </el-tab-pane>

        <!-- Stream Tab (not for HTTP/SOCKS/Tunnel/WireGuard/TUN) -->
        <el-tab-pane v-if="!['http','socks','tunnel','dokodemo-door','wireguard','tun'].includes(rf.protocol)" :label="t('protoCenter.inbounds.stream')" name="stream">
          <el-form label-width="120px" label-position="left">
            <el-form-item label="Network">
              <el-select v-model="rf.streamSettings.network" style="width:100%" @change="onNetworkChange">
                <el-option label="raw" value="raw" /><el-option label="ws" value="ws" />
                <el-option label="mKCP" value="mkcp" /><el-option label="grpc" value="grpc" />
                <el-option label="httpupgrade" value="httpupgrade" /><el-option label="xhttp" value="xhttp" />
                <el-option label="Hysteria" value="hysteria" />
              </el-select>
            </el-form-item>
            <el-form-item label="Security">
              <el-select v-model="rf.streamSettings.security" style="width:100%" @change="onSecurityChange">
                <el-option label="none" value="none" />
                <el-option label="tls" value="tls" />
                <el-option v-if="['raw','xhttp','grpc'].includes(rf.streamSettings.network)" label="reality" value="reality" />
              </el-select>
              <div v-if="rf.streamSettings.security === 'reality'" class="hint">Reality 仅支持 raw、xhttp、grpc</div>
            </el-form-item>
            <template v-if="rf.streamSettings.security === 'tls'">
              <el-form-item label="Server Name"><el-input v-model="rf.streamSettings.tls.serverName" @change="emit" /></el-form-item>
              <el-form-item label="ALPN"><el-input v-model="rf.streamSettings.tls.alpnStr" placeholder="h2, http/1.1" @change="emit" /></el-form-item>
              <el-form-item label="Fingerprint">
                <el-select v-model="rf.streamSettings.tls.fingerprint" style="width:100%" clearable @change="emit">
                  <el-option label="chrome" value="chrome" /><el-option label="firefox" value="firefox" />
                  <el-option label="safari" value="safari" /><el-option label="edge" value="edge" />
                  <el-option label="random" value="random" />
                </el-select>
              </el-form-item>
            </template>
            <template v-if="rf.streamSettings.security === 'reality'">
              <el-form-item label="Target"><el-input v-model="rf.streamSettings.reality.target" placeholder="www.microsoft.com:443" @change="emit" /></el-form-item>
              <el-form-item label="Server Names"><el-input v-model="rf.streamSettings.reality.serverNamesStr" placeholder="www.microsoft.com" @change="emit" /></el-form-item>
              <el-form-item label="Private Key"><el-input v-model="rf.streamSettings.reality.privateKey" @change="emit" /></el-form-item>
              <el-form-item label="Short ID"><el-input v-model="rf.streamSettings.reality.shortId" @change="emit" /></el-form-item>
              <el-form-item label="Fingerprint">
                <el-select v-model="rf.streamSettings.reality.fingerprint" style="width:100%" @change="emit">
                  <el-option label="chrome" value="chrome" /><el-option label="firefox" value="firefox" />
                </el-select>
              </el-form-item>
            </template>
          </el-form>

          <!-- RAW Settings -->
          <template v-if="rf.streamSettings.network === 'raw'">
            <el-divider content-position="left">RAW Settings</el-divider>
            <el-form label-width="140px" label-position="left">
              <el-form-item label="Accept Proxy Protocol">
                <el-switch v-model="rf.streamSettings.raw.acceptProxyProtocol" />
              </el-form-item>
              <el-form-item label="Header Type">
                <el-select v-model="rf.streamSettings.raw.headerType" style="width:100%" @change="onRawHeaderChange">
                  <el-option label="none" value="none" />
                  <el-option label="http" value="http" />
                </el-select>
              </el-form-item>
              <template v-if="rf.streamSettings.raw.headerType === 'http'">
                <el-divider content-position="left">HTTP Request</el-divider>
                <el-form-item label="Version">
                  <el-input v-model="rf.streamSettings.raw.request.version" placeholder="1.1" />
                </el-form-item>
                <el-form-item label="Method">
                  <el-input v-model="rf.streamSettings.raw.request.method" placeholder="GET" />
                </el-form-item>
                <el-form-item label="Paths">
                  <el-input v-model="rf.streamSettings.raw.request._pathsStr" placeholder="/" />
                  <div class="hint">Comma-separated, e.g. /,/path</div>
                </el-form-item>
                <el-form-item label="Headers">
                  <div v-for="(h, i) in rf.streamSettings.raw.request._headers" :key="i" style="display:flex;gap:8px;margin-bottom:4px">
                    <el-input v-model="h.key" placeholder="Host" style="width:30%" />
                    <el-input v-model="h.value" placeholder="www.example.com" style="width:50%" />
                    <el-button size="small" type="danger" text @click="rf.streamSettings.raw.request._headers.splice(i,1)">X</el-button>
                  </div>
                  <el-button size="small" @click="rf.streamSettings.raw.request._headers.push({key:'',value:''})">+ Add Header</el-button>
                </el-form-item>
                <el-divider content-position="left">HTTP Response</el-divider>
                <el-form-item label="Version">
                  <el-input v-model="rf.streamSettings.raw.response.version" placeholder="1.1" />
                </el-form-item>
                <el-form-item label="Status">
                  <el-input v-model="rf.streamSettings.raw.response.status" placeholder="200" />
                </el-form-item>
                <el-form-item label="Reason">
                  <el-input v-model="rf.streamSettings.raw.response.reason" placeholder="OK" />
                </el-form-item>
                <el-form-item label="Headers">
                  <div v-for="(h, i) in rf.streamSettings.raw.response._headers" :key="i" style="display:flex;gap:8px;margin-bottom:4px">
                    <el-input v-model="h.key" placeholder="Content-Type" style="width:30%" />
                    <el-input v-model="h.value" placeholder="application/octet-stream" style="width:50%" />
                    <el-button size="small" type="danger" text @click="rf.streamSettings.raw.response._headers.splice(i,1)">X</el-button>
                  </div>
                  <el-button size="small" @click="rf.streamSettings.raw.response._headers.push({key:'',value:''})">+ Add Header</el-button>
                </el-form-item>
              </template>
            </el-form>
          </template>

          <!-- WebSocket Settings -->
          <template v-if="rf.streamSettings.network === 'ws'">
            <el-divider content-position="left">WebSocket Settings</el-divider>
            <el-form label-width="140px" label-position="left">
              <el-form-item label="Accept Proxy Protocol">
                <el-switch v-model="rf.streamSettings.ws.acceptProxyProtocol" />
              </el-form-item>
              <el-form-item label="Path">
                <div style="display:flex;gap:8px">
                  <el-input v-model="rf.streamSettings.ws.path" placeholder="/" style="flex:1" />
                  <el-button size="small" @click="rf.streamSettings.ws.path = '/' + genId(12) + '?ed=2560'">Random</el-button>
                </div>
                <div class="hint">包含 ?ed=2560 启用 Early Data 降低延迟</div>
              </el-form-item>
              <el-form-item label="Host">
                <el-input v-model="rf.streamSettings.ws.host" placeholder="留空不验证" />
              </el-form-item>
            </el-form>
            <el-divider content-position="left">Client Settings</el-divider>
            <el-form label-width="140px" label-position="left">
              <el-form-item label="Headers">
                <div v-for="(h, i) in rf.streamSettings.ws._headers" :key="i" style="display:flex;gap:8px;margin-bottom:4px">
                  <el-input v-model="h.key" placeholder="Header name" style="width:30%" />
                  <el-input v-model="h.value" placeholder="Header value" style="width:50%" />
                  <el-button size="small" type="danger" text @click="rf.streamSettings.ws._headers.splice(i,1)">X</el-button>
                </div>
                <el-button size="small" @click="rf.streamSettings.ws._headers.push({key:'',value:''})">+ Add Header</el-button>
              </el-form-item>
              <el-form-item label="Heartbeat Period">
                <el-input-number v-model="rf.streamSettings.ws.heartbeatPeriod" :min="0" />
                <div class="hint">Seconds, 0 = disabled</div>
              </el-form-item>
            </el-form>
          </template>

          <!-- mKCP Settings -->
          <template v-if="rf.streamSettings.network === 'mkcp'">
            <el-divider content-position="left">mKCP Settings</el-divider>
            <el-form label-width="160px" label-position="left">
              <el-form-item label="MTU">
                <el-input-number v-model="rf.streamSettings.kcp.mtu" :min="576" :max="1460" @change="emit" />
                <div class="hint">576-1460, default 1350</div>
              </el-form-item>
              <el-form-item label="TTI (ms)">
                <el-input-number v-model="rf.streamSettings.kcp.tti" :min="10" :max="100" @change="emit" />
                <div class="hint">Transmission time interval, 10-100ms, default 50</div>
              </el-form-item>
              <el-form-item label="Uplink Capacity (MB/s)">
                <el-input-number v-model="rf.streamSettings.kcp.uplinkCapacity" :min="0" @change="emit" />
                <div class="hint">Upload bandwidth, 0 = very small</div>
              </el-form-item>
              <el-form-item label="Downlink Capacity (MB/s)">
                <el-input-number v-model="rf.streamSettings.kcp.downlinkCapacity" :min="0" @change="emit" />
                <div class="hint">Download bandwidth, 0 = very small</div>
              </el-form-item>
              <el-form-item label="Congestion">
                <el-switch v-model="rf.streamSettings.kcp.congestion" @change="emit" />
                <div class="hint">Auto reduce throughput on packet loss</div>
              </el-form-item>
              <el-form-item label="Read Buffer Size (MB)">
                <el-input-number v-model="rf.streamSettings.kcp.readBufferSize" :min="1" @change="emit" />
              </el-form-item>
              <el-form-item label="Write Buffer Size (MB)">
                <el-input-number v-model="rf.streamSettings.kcp.writeBufferSize" :min="1" @change="emit" />
              </el-form-item>
            </el-form>
          </template>

          <!-- HTTPUpgrade Settings -->
          <template v-if="rf.streamSettings.network === 'httpupgrade'">
            <el-divider content-position="left">HTTPUpgrade Settings</el-divider>
            <el-form label-width="140px" label-position="left">
              <el-form-item label="Accept Proxy Protocol">
                <el-switch v-model="rf.streamSettings.httpupgrade.acceptProxyProtocol" />
              </el-form-item>
              <el-form-item label="Path">
                <div style="display:flex;gap:8px">
                  <el-input v-model="rf.streamSettings.httpupgrade.path" placeholder="/" style="flex:1" />
                  <el-button size="small" @click="rf.streamSettings.httpupgrade.path = '/' + genId(12) + '?ed=2560'">Random</el-button>
                </div>
                <div class="hint">包含 ?ed=2560 启用 Early Data</div>
              </el-form-item>
              <el-form-item label="Host">
                <el-input v-model="rf.streamSettings.httpupgrade.host" placeholder="留空不验证" />
              </el-form-item>
            </el-form>
            <el-divider content-position="left">Client Settings</el-divider>
            <el-form label-width="140px" label-position="left">
              <el-form-item label="Headers">
                <div v-for="(h, i) in rf.streamSettings.httpupgrade.settings._headers" :key="i" style="display:flex;gap:8px;margin-bottom:4px">
                  <el-input v-model="h.key" placeholder="Header name" style="width:30%" />
                  <el-input v-model="h.value" placeholder="Header value" style="width:50%" />
                  <el-button size="small" type="danger" text @click="rf.streamSettings.httpupgrade.settings._headers.splice(i,1)">X</el-button>
                </div>
                <el-button size="small" @click="rf.streamSettings.httpupgrade.settings._headers.push({key:'',value:''})">+ Add Header</el-button>
              </el-form-item>
            </el-form>
          </template>

          <!-- Hysteria Settings -->
          <template v-if="rf.streamSettings.network === 'hysteria'">
            <el-divider content-position="left">Hysteria Settings</el-divider>
            <el-form label-width="140px" label-position="left">
              <el-form-item label="Version">
                <el-input-number v-model="rf.streamSettings.hysteria.version" :min="2" :max="2" @change="emit" />
              </el-form-item>
              <el-form-item label="Auth">
                <div style="display:flex;gap:8px">
                  <el-input v-model="rf.streamSettings.hysteria.auth" style="flex:1" />
                  <el-button size="small" @click="rf.streamSettings.hysteria.auth = genBase64Password()">Generate</el-button>
                </div>
                <div class="hint">搭配 hysteria inbound 时会被 users 覆盖</div>
              </el-form-item>
              <el-form-item label="UDP Idle Timeout">
                <el-input-number v-model="rf.streamSettings.hysteria.udpIdleTimeout" :min="0" @change="emit" />
                <div class="hint">秒，默认 60</div>
              </el-form-item>
            </el-form>
            <el-divider content-position="left">Masquerade</el-divider>
            <el-form label-width="140px" label-position="left">
              <el-form-item label="Type">
                <el-select v-model="rf.streamSettings.hysteria.masquerade.type" style="width:100%" clearable @change="emit">
                  <el-option label="默认 404" value="" />
                  <el-option label="file" value="file" />
                  <el-option label="proxy" value="proxy" />
                  <el-option label="string" value="string" />
                </el-select>
              </el-form-item>
              <el-form-item v-if="rf.streamSettings.hysteria.masquerade.type === 'file'" label="Directory">
                <el-input v-model="rf.streamSettings.hysteria.masquerade.dir" placeholder="/var/www/html" @change="emit" />
              </el-form-item>
              <template v-if="rf.streamSettings.hysteria.masquerade.type === 'proxy'">
                <el-form-item label="URL">
                  <el-input v-model="rf.streamSettings.hysteria.masquerade.url" placeholder="https://example.com" @change="emit" />
                </el-form-item>
                <el-form-item label="Rewrite Host">
                  <el-switch v-model="rf.streamSettings.hysteria.masquerade.rewriteHost" @change="emit" />
                </el-form-item>
                <el-form-item label="Insecure">
                  <el-switch v-model="rf.streamSettings.hysteria.masquerade.insecure" @change="emit" />
                </el-form-item>
              </template>
              <template v-if="rf.streamSettings.hysteria.masquerade.type === 'string'">
                <el-form-item label="Content">
                  <el-input v-model="rf.streamSettings.hysteria.masquerade.content" type="textarea" :rows="3" @change="emit" />
                </el-form-item>
                <el-form-item label="Status Code">
                  <el-input-number v-model="rf.streamSettings.hysteria.masquerade.statusCode" :min="0" @change="emit" />
                </el-form-item>
                <el-form-item label="Headers">
                  <div v-for="(h, i) in rf.streamSettings.hysteria.masquerade._headers" :key="i" style="display:flex;gap:8px;margin-bottom:4px">
                    <el-input v-model="h.key" placeholder="Header name" style="width:30%" />
                    <el-input v-model="h.value" placeholder="Header value" style="width:50%" />
                    <el-button size="small" type="danger" text @click="rf.streamSettings.hysteria.masquerade._headers.splice(i,1)">X</el-button>
                  </div>
                  <el-button size="small" @click="rf.streamSettings.hysteria.masquerade._headers.push({key:'',value:''})">+ Add Header</el-button>
                </el-form-item>
              </template>
            </el-form>
          </template>

          <!-- XHTTP Settings -->
          <template v-if="rf.streamSettings.network === 'xhttp'">
            <el-divider content-position="left">XHTTP Settings</el-divider>
            <el-form label-width="160px" label-position="left">
              <el-form-item label="Path">
                <div style="display:flex;gap:8px">
                  <el-input v-model="rf.streamSettings.xhttp.path" placeholder="/" style="flex:1" />
                  <el-button size="small" @click="rf.streamSettings.xhttp.path = '/' + genId(12)">Random</el-button>
                </div>
              </el-form-item>
              <el-form-item label="Host">
                <el-input v-model="rf.streamSettings.xhttp.host" placeholder="留空不配置" @change="emit" />
              </el-form-item>
              <el-form-item label="Mode">
                <el-select v-model="rf.streamSettings.xhttp.mode" style="width:100%" clearable @change="emit">
                  <el-option label="auto" value="" />
                  <el-option label="packet-up" value="packet-up" />
                  <el-option label="stream-up" value="stream-up" />
                  <el-option label="stream-one" value="stream-one" />
                </el-select>
              </el-form-item>
              <el-form-item label="Uplink HTTP Method">
                <el-checkbox v-model="rf.streamSettings.xhttp.extra._showMethod" @change="emit">启用</el-checkbox>
                <el-select v-if="rf.streamSettings.xhttp.extra._showMethod" v-model="rf.streamSettings.xhttp.extra.uplinkHTTPMethod" style="width:100%;margin-top:4px" @change="emit">
                  <el-option label="POST" value="POST" /><el-option label="GET" value="GET" />
                </el-select>
              </el-form-item>
              <el-form-item v-if="['stream-up','stream-one'].includes(rf.streamSettings.xhttp.mode)" label="No GRPC Header">
                <el-switch v-model="rf.streamSettings.xhttp.extra.noGRPCHeader" @change="emit" />
              </el-form-item>
              <el-form-item label="No SSE Header">
                <el-switch v-model="rf.streamSettings.xhttp.extra.noSSEHeader" @change="emit" />
                <div class="hint">Server only</div>
              </el-form-item>
              <el-form-item label="Server Max Header Bytes">
                <el-checkbox v-model="rf.streamSettings.xhttp.extra._showMaxHeader" @change="emit">启用</el-checkbox>
                <el-input-number v-if="rf.streamSettings.xhttp.extra._showMaxHeader" v-model="rf.streamSettings.xhttp.extra.serverMaxHeaderBytes" :min="0" style="margin-top:4px" @change="emit" />
              </el-form-item>
              <el-form-item label="Headers">
                <div v-for="(h, i) in rf.streamSettings.xhttp.extra._headers" :key="i" style="display:flex;gap:8px;margin-bottom:4px">
                  <el-input v-model="h.key" placeholder="Header name" style="width:30%" />
                  <el-input v-model="h.value" placeholder="Header value" style="width:50%" />
                  <el-button size="small" type="danger" text @click="rf.streamSettings.xhttp.extra._headers.splice(i,1)">X</el-button>
                </div>
                <el-button size="small" @click="rf.streamSettings.xhttp.extra._headers.push({key:'',value:''})">+ Add Header</el-button>
              </el-form-item>
            </el-form>
            <template v-if="!rf.streamSettings.xhttp.mode || rf.streamSettings.xhttp.mode === 'packet-up'">
              <el-divider content-position="left">Scamper (packet-up only)</el-divider>
              <el-form label-width="180px" label-position="left">
                <el-form-item label="Max Each Post Bytes">
                  <el-input-number v-model="rf.streamSettings.xhttp.extra.scMaxEachPostBytes" :min="0" @change="emit" />
                </el-form-item>
                <el-form-item label="Min Posts Interval (ms)">
                  <el-input-number v-model="rf.streamSettings.xhttp.extra.scMinPostsIntervalMs" :min="0" @change="emit" />
                </el-form-item>
                <el-form-item label="Max Buffered Posts">
                  <el-input v-model="rf.streamSettings.xhttp.extra.scMaxBufferedPosts" placeholder="30" style="width:200px" @change="emit" />
                </el-form-item>
              </el-form>
            </template>
            <template v-if="rf.streamSettings.xhttp.mode === 'stream-up'">
              <el-divider content-position="left">Stream-Up Settings</el-divider>
              <el-form label-width="180px" label-position="left">
                <el-form-item label="Stream Up Server Secs">
                  <el-input v-model="rf.streamSettings.xhttp.extra.scStreamUpServerSecs" placeholder="20-80" style="width:200px" @change="emit" />
                </el-form-item>
              </el-form>
            </template>
            <el-divider content-position="left">Xmux (Multiplexing)</el-divider>
            <el-form label-width="180px" label-position="left">
              <el-form-item label="Max Concurrency">
                <el-input v-model="rf.streamSettings.xhttp.extra.xmux.maxConcurrency" placeholder="16-32" style="width:200px" @change="onXmuxConcurrency" />
                <div class="hint">与 Max Connections 互斥</div>
              </el-form-item>
              <el-form-item label="Max Connections">
                <el-input v-model="rf.streamSettings.xhttp.extra.xmux.maxConnections" placeholder="0" style="width:200px" @change="onXmuxConnections" />
                <div class="hint">与 Max Concurrency 互斥</div>
              </el-form-item>
              <el-form-item label="C Max Reuse Times">
                <el-input v-model="rf.streamSettings.xhttp.extra.xmux.cMaxReuseTimes" placeholder="0" style="width:200px" @change="emit" />
              </el-form-item>
              <el-form-item label="H Max Request Times">
                <el-input v-model="rf.streamSettings.xhttp.extra.xmux.hMaxRequestTimes" placeholder="600-900" style="width:200px" @change="emit" />
              </el-form-item>
              <el-form-item label="H Max Reusable Secs">
                <el-input v-model="rf.streamSettings.xhttp.extra.xmux.hMaxReusableSecs" placeholder="1800-3000" style="width:200px" @change="emit" />
              </el-form-item>
            </el-form>
          </template>

          <!-- gRPC Settings -->
          <template v-if="rf.streamSettings.network === 'grpc'">
            <el-divider content-position="left">gRPC Settings</el-divider>
            <el-form label-width="140px" label-position="left">
              <el-form-item label="Service Name">
                <div style="display:flex;gap:8px">
                  <el-input v-model="rf.streamSettings.grpc.serviceName" style="flex:1" />
                  <el-button size="small" @click="rf.streamSettings.grpc.serviceName = genId(12)">更换</el-button>
                </div>
              </el-form-item>
              <el-form-item label="Authority">
                <el-input v-model="rf.streamSettings.grpc.authority" placeholder="grpc.example.com" />
                <div class="hint">可当 Host 使用，留空不配置</div>
              </el-form-item>
            </el-form>
            <el-divider content-position="left">Client Settings</el-divider>
            <el-form label-width="140px" label-position="left">
              <el-form-item label="Multi Mode">
                <el-switch v-model="rf.streamSettings.grpc.settings.multiMode" />
                <div class="hint">实验性，约 20% 性能提升</div>
              </el-form-item>
              <el-form-item label="User Agent">
                <el-select v-model="rf.streamSettings.grpc.settings.user_agent" filterable allow-create style="width:100%" @change="emit">
                  <el-option label="默认 (Go)" value="" />
                  <el-option label="Chrome Windows" value="Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36" />
                  <el-option label="Chrome macOS" value="Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36" />
                  <el-option label="Chrome Linux" value="Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36" />
                  <el-option label="Safari iOS" value="Mozilla/5.0 (iPhone; CPU iPhone OS 17_5_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1" />
                  <el-option label="Chrome Android" value="Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Mobile Safari/537.36" />
                  <el-option label="grpc-go/1.62.0" value="grpc-go/1.62.0" />
                  <el-option label="grpc-node/1.6.0" value="grpc-node/1.6.0" />
                  <el-option label="grpc-python/1.60.0" value="grpc-python/1.60.0" />
                </el-select>
              </el-form-item>
              <el-form-item label="Idle Timeout (s)">
                <el-input-number v-model="rf.streamSettings.grpc.settings.idle_timeout" :min="0" />
                <div class="hint">空闲健康检查，最小 10s，0=禁用</div>
              </el-form-item>
              <el-form-item label="Health Check Timeout">
                <el-input-number v-model="rf.streamSettings.grpc.settings.health_check_timeout" :min="0" />
                <div class="hint">健康检查超时，默认 20s</div>
              </el-form-item>
              <el-form-item label="Permit Without Stream">
                <el-switch v-model="rf.streamSettings.grpc.settings.permit_without_stream" />
              </el-form-item>
              <el-form-item label="Initial Window Size">
                <el-input-number v-model="rf.streamSettings.grpc.settings.initial_windows_size" :min="0" />
                <div class="hint">0=不生效，>65535 禁用动态窗口</div>
              </el-form-item>
            </el-form>
          </template>
        </el-tab-pane>

        <!-- Sniffing Tab -->
        <el-tab-pane :label="t('protoCenter.inbounds.sniffing')" name="sniffing">
          <el-form label-width="140px" label-position="left">
            <el-form-item label="Enabled">
              <el-switch v-model="rf.sniffing.enabled" />
            </el-form-item>
            <template v-if="rf.sniffing.enabled">
              <el-form-item label="Dest Override">
                <el-checkbox-group v-model="rf.sniffing.destOverride">
                  <el-checkbox label="http" /><el-checkbox label="tls" />
                  <el-checkbox label="quic" /><el-checkbox label="fakedns" />
                </el-checkbox-group>
              </el-form-item>
              <el-form-item label="Metadata Only">
                <el-switch v-model="rf.sniffing.metadataOnly" />
                <div class="hint">仅使用元数据嗅探，不深度检测</div>
              </el-form-item>
              <el-form-item label="Route Only">
                <el-switch v-model="rf.sniffing.routeOnly" />
                <div class="hint">仅用于路由，不替换目标地址</div>
              </el-form-item>
              <el-form-item label="Domains Excluded">
                <el-input v-model="rf.sniffing._domainsExcludedStr" type="textarea" :rows="2" placeholder="courier.push.apple.com&#10;domain:example.com" @change="emit" />
                <div class="hint">每行一个，嗅探结果匹配时不重置目标</div>
              </el-form-item>
              <el-form-item label="IPs Excluded">
                <el-input v-model="rf.sniffing._ipsExcludedStr" type="textarea" :rows="2" placeholder="10.0.0.0/8" @change="emit" />
                <div class="hint">每行一个，目标 IP 匹配时不重置</div>
              </el-form-item>
            </template>
          </el-form>
        </el-tab-pane>
      </el-tabs>

      <template #footer>
        <el-button @click="dlg=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveInbound">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { reactive, ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import client from '../../../api/client'
const { t } = useI18n()
const props = defineProps({ data: Array, coreType: String })
const emitFn = defineEmits(['update'])

const form = ref([])
const dlg = ref(false)
const editIdx = ref(-1)
const activeTab = ref('basic')
const rf = reactive(getDefaultInbound())

function genRemark() { return 'Node-' + Math.random().toString(36).substring(2, 8) }
function genPort() { return Math.floor(Math.random() * 50000 + 10000) }
function genId(len) { const chars = 'abcdefghijklmnopqrstuvwxyz0123456789'; let r = ''; for (let i = 0; i < len; i++) r += chars.charAt(Math.floor(Math.random() * chars.length)); return r }
async function genWgKeyPair() {
  try {
    const data = await client.post('/binaries/wg-key?core_type=' + (props.coreType || 'xray'))
    rf.settings.secretKey = data.privateKey || ''
    rf._wgPublicKey = data.publicKey || ''
  } catch {
    ElMessage.error('生成密钥失败')
  }
}
function genUUID() { return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) { var r = Math.random()*16|0, v = c === 'x' ? r : (r&0x3|0x8); return v.toString(16) }) }
function genPassword() { const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789'; let r = ''; for (let i = 0; i < 12; i++) r += chars.charAt(Math.floor(Math.random() * chars.length)); return r }
function genBase64Password() { const bytes = new Uint8Array(32); crypto.getRandomValues(bytes); return btoa(String.fromCharCode(...bytes)) }
function genEmail() { return Math.random().toString(36).substring(2, 8) + '@xray.com' }
function formatTraffic(bytes) { if (!bytes) return '0 B'; const u = ['B', 'KB', 'MB', 'GB', 'TB']; let i = 0, b = bytes; while (b >= 1024 && i < u.length - 1) { b /= 1024; i++ } return b.toFixed(1) + ' ' + u[i] }
function resetTraffic(u) { u._usedUp = 0; u._usedDown = 0 }

function addUser(proto) {
  if (proto === 'vless') rf.settings.users.push({ id: genUUID(), email: genEmail(), flow: '', level: 0, _quota: 'unlimited', _expiry: null, _usedUp: 0, _usedDown: 0 })
  else if (proto === 'vmess') rf.settings.users.push({ id: genUUID(), email: genEmail(), alterId: 0, security: 'auto', level: 0, _quota: 'unlimited', _expiry: null, _usedUp: 0, _usedDown: 0 })
  else if (proto === 'trojan') rf.settings.users.push({ password: genBase64Password(), email: genEmail(), level: 0, _quota: 'unlimited', _expiry: null, _usedUp: 0, _usedDown: 0 })
  else if (proto === 'hysteria') rf.settings.users.push({ auth: genBase64Password(), email: genEmail(), level: 0, _quota: 'unlimited', _expiry: null, _usedUp: 0, _usedDown: 0 })
  else if (proto === 'shadowsocks') rf.settings.clients.push({ password: genBase64Password(), email: genEmail(), _quota: 'unlimited', _expiry: null, _usedUp: 0, _usedDown: 0 })
}

function getDefaultInbound() {
  const port = genPort()
  return {
    _remark: genRemark(), tag: 'inbound-vless-' + port, protocol: 'vless',
    listen: '0.0.0.0', port: port, _enabled: true,
    _listenType: 'tcp', _socketPath: '', _socketPerm: '0666',
    settings: { users: [{ id: genUUID(), email: genEmail(), flow: '', level: 0, _quota: 'unlimited', _expiry: null }], decryption: 'none' },
    streamSettings: { network: 'raw', security: 'none', tls: { serverName: '', alpnStr: '', fingerprint: '' }, reality: { target: '', serverNamesStr: '', privateKey: '', shortId: '', fingerprint: 'chrome' }, raw: { acceptProxyProtocol: false, headerType: 'none', request: { version: '1.1', method: 'GET', _pathsStr: '/', _headers: [] }, response: { version: '1.1', status: '200', reason: 'OK', _headers: [] } }, ws: { acceptProxyProtocol: false, path: '/?ed=2560', host: '', settings: { _headers: [], heartbeatPeriod: 0 } }, kcp: { mtu: 1350, tti: 50, uplinkCapacity: 5, downlinkCapacity: 20, congestion: false, readBufferSize: 2, writeBufferSize: 2 }, grpc: { serviceName: genId(12), authority: '', settings: { multiMode: false, user_agent: '', idle_timeout: 0, health_check_timeout: 20, permit_without_stream: false, initial_windows_size: 0 } }, httpupgrade: { acceptProxyProtocol: false, path: '/?ed=2560', host: '', settings: { _headers: [] } }, xhttp: { path: '/', host: '', mode: 'auto', extra: { _showMethod: false, uplinkHTTPMethod: 'POST', noGRPCHeader: false, noSSEHeader: false, _showMaxHeader: false, serverMaxHeaderBytes: 0, _headers: [], xPaddingBytes: '100-1000', xPaddingObfsMode: false, scMaxEachPostBytes: 1000000, scMinPostsIntervalMs: 30, scMaxBufferedPosts: 30, scStreamUpServerSecs: '20-80', xmux: { maxConcurrency: '16-32', maxConnections: 0, cMaxReuseTimes: 0, hMaxRequestTimes: '600-900', hMaxReusableSecs: '1800-3000', hKeepAlivePeriod: 0 } } }, hysteria: { version: 2, auth: genBase64Password(), udpIdleTimeout: 60, masquerade: { type: '', dir: '', url: '', rewriteHost: false, insecure: false, content: '', _headers: [], statusCode: 0 } }, http: { _hostStr: '', path: '/' } },
    sniffing: { enabled: true, destOverride: ['http', 'tls', 'quic'], metadataOnly: false, _domainsExcludedStr: '', _ipsExcludedStr: '', routeOnly: false },
    _wgAddressesStr: '', _tunGatewayStr: '', _tunDnsStr: '', _tunAutoRoutingStr: '',
  }
}

function onProtocolChange() {
  const p = rf.protocol
  const port = rf.port && rf.port > 0 ? rf.port : genPort()
  rf.port = port
  rf.tag = 'inbound-' + p + '-' + port
  rf._socketPath = '/dev/shm/' + p + '.sock'
  if (p === 'vless') rf.settings = { users: [{ id: genUUID(), email: genEmail(), flow: '', level: 0, _quota: 'unlimited', _expiry: null }], decryption: 'none' }
  else if (p === 'vmess') rf.settings = { users: [{ id: genUUID(), email: genEmail(), alterId: 0, security: 'auto', level: 0, _quota: 'unlimited', _expiry: null }] }
  else if (p === 'trojan') rf.settings = { users: [{ password: genBase64Password(), email: genEmail(), level: 0, _quota: 'unlimited', _expiry: null }] }
  else if (p === 'shadowsocks') rf.settings = { method: 'aes-256-gcm', password: genBase64Password(), network: 'tcp', clients: [{ password: genBase64Password(), email: genEmail(), _quota: 'unlimited', _expiry: null, _usedUp: 0, _usedDown: 0 }] }
  else if (p === 'socks') rf.settings = { auth: 'noauth', accounts: [], udp: false }
  else if (p === 'http') rf.settings = { accounts: [], allowTransparent: false }
  else if (p === 'wireguard') { rf.settings = { secretKey: '', peers: [{ publicKey: '', _allowedIPsStr: '0.0.0.0/0, ::/0', keepAlive: 0, comment: '' }], address: [], mtu: 1420, noKernelTun: true, workers: 0, domainStrategy: '' }; rf._wgAddressesStr = ''; rf._wgPublicKey = '' }
  else if (p === 'hysteria') rf.settings = { version: 2, users: [{ auth: genBase64Password(), email: genEmail(), level: 0, _quota: 'unlimited', _expiry: null }] }
  else if (p === 'tun') { rf.settings = { name: 'xray0', mtu: 1500, gateway: [], dns: [], userLevel: 0, autoSystemRoutingTable: [], autoOutboundsInterface: '' }; rf._tunGatewayStr = ''; rf._tunDnsStr = ''; rf._tunAutoRoutingStr = '' }
  else if (p === 'tunnel' || p === 'dokodemo-door') { rf.settings = { allowedNetwork: 'tcp', rewriteAddress: '', rewritePort: 0, followRedirect: false, userLevel: 0, portMap: {} }; rf._portMap = []; rf.listen = '127.0.0.1'; rf.port = genPort() }
  else rf.settings = {}
}

function onListenTypeChange(val) {
  if (val === 'sock') {
    rf._savedPort = rf.port
    rf.listen = ''
    rf.port = 0
    if (!rf._socketPath) rf._socketPath = '/dev/shm/' + rf.protocol + '.sock'
  } else {
    rf.listen = '0.0.0.0'
    rf.port = rf._savedPort || genPort()
    rf._socketPath = ''
  }
}

function onNetworkChange() {
  // Reset incompatible security when network changes
  if (rf.streamSettings.security === 'reality' && !['raw', 'xhttp', 'grpc'].includes(rf.streamSettings.network)) {
    rf.streamSettings.security = 'none'
  }
  emit()
}

function onSecurityChange() {
  emit()
}

function onSocksAuthChange(val) {
  if (val === 'password' && (!rf.settings.accounts || !rf.settings.accounts.length)) {
    rf.settings.accounts = [{ user: genEmail().split('@')[0], pass: genPassword(), _quota: 'unlimited', _expiry: null, _usedUp: 0, _usedDown: 0 }]
  }
}

function onXmuxConcurrency() {
  if (rf.streamSettings.xhttp.extra.xmux.maxConcurrency > 0) {
    rf.streamSettings.xhttp.extra.xmux.maxConnections = 0
  }
  emit()
}

function onXmuxConnections() {
  if (rf.streamSettings.xhttp.extra.xmux.maxConnections > 0) {
    rf.streamSettings.xhttp.extra.xmux.maxConcurrency = 0
  }
  emit()
}

function onRawHeaderChange(val) {
  if (val === 'http') {
    rf.streamSettings.raw.request = { version: '1.1', method: 'GET', _pathsStr: '/', _headers: [{ key: 'Host', value: 'www.baidu.com' }, { key: 'User-Agent', value: 'Mozilla/5.0' }] }
    rf.streamSettings.raw.response = { version: '1.1', status: '200', reason: 'OK', _headers: [{ key: 'Content-Type', value: 'application/octet-stream' }] }
  } else {
    rf.streamSettings.raw.request = { version: '1.1', method: 'GET', _pathsStr: '/', _headers: [] }
    rf.streamSettings.raw.response = { version: '1.1', status: '200', reason: 'OK', _headers: [] }
  }
}

function parseRealitySettings(reality) {
  return {
    target: reality.target || '',
    serverNamesStr: (reality.serverNames || []).join(', '),
    privateKey: reality.privateKey || '',
    shortId: reality.shortId || '',
    fingerprint: reality.fingerprint || 'chrome',
  }
}

function parseXhttpSettings(xh) {
  const extra = xh.extra || xh
  const xmux = extra.xmux || {}
  return {
    path: xh.path || '/',
    host: xh.host || '',
    mode: xh.mode || 'auto',
    extra: {
      // Client settings
      _showMethod: !!extra.uplinkHTTPMethod,
      uplinkHTTPMethod: extra.uplinkHTTPMethod || 'POST',
      noGRPCHeader: extra.noGRPCHeader || false,
      noSSEHeader: extra.noSSEHeader || false,
      _showMaxHeader: !!extra.serverMaxHeaderBytes,
      serverMaxHeaderBytes: extra.serverMaxHeaderBytes || 0,
      _headers: Object.entries(extra.headers || {}).map(([k, v]) => ({ key: k, value: v })),
      // Padding
      xPaddingBytes: extra.xPaddingBytes || '100-1000',
      xPaddingObfsMode: extra.xPaddingObfsMode || false,
      // Scamper
      scMaxEachPostBytes: extra.scMaxEachPostBytes || '',
      scMinPostsIntervalMs: extra.scMinPostsIntervalMs || '',
      scMaxBufferedPosts: extra.scMaxBufferedPosts || 30,
      scStreamUpServerSecs: extra.scStreamUpServerSecs || '20-80',
      // Xmux
      xmux: {
        maxConcurrency: xmux.maxConcurrency || '16-32',
        maxConnections: xmux.maxConnections || 0,
        cMaxReuseTimes: xmux.cMaxReuseTimes || 0,
        hMaxRequestTimes: xmux.hMaxRequestTimes || '600-900',
        hMaxReusableSecs: xmux.hMaxReusableSecs || '1800-3000',
        hKeepAlivePeriod: xmux.hKeepAlivePeriod || 0,
      },
    },
  }
}

function parseHysteriaSettings(hy) {
  const masq = hy.masquerade || {}
  return {
    version: hy.version || 2,
    auth: hy.auth || '',
    udpIdleTimeout: hy.udpIdleTimeout || 60,
    masquerade: {
      type: masq.type || '',
      dir: masq.dir || '',
      url: masq.url || '',
      rewriteHost: masq.rewriteHost || false,
      insecure: masq.insecure || false,
      content: masq.content || '',
      _headers: Object.entries(masq.headers || {}).map(([k, v]) => ({ key: k, value: v })),
      statusCode: masq.statusCode || 0,
    },
  }
}

function parseHttpupgradeSettings(hu) {
  // Support both nested {settings: {headers}} and flat {headers}
  const s = hu.settings || hu
  return {
    acceptProxyProtocol: hu.acceptProxyProtocol || false,
    path: hu.path || '/',
    host: hu.host || '',
    _headers: Object.entries(s.headers || {}).map(([k, v]) => ({ key: k, value: v })),
  }
}

function parseKcpSettings(kcp) {
  return {
    mtu: kcp.mtu || 1350,
    tti: kcp.tti || 50,
    uplinkCapacity: kcp.uplinkCapacity || 5,
    downlinkCapacity: kcp.downlinkCapacity || 20,
    congestion: kcp.congestion || false,
    readBufferSize: kcp.readBufferSize || 2,
    writeBufferSize: kcp.writeBufferSize || 2,
  }
}

function parseHttpSettings(http) {
  return {
    _hostStr: (http.host || []).join(', '),
    path: http.path || '/',
  }
}

function parseGrpcSettings(grpc) {
  const s = grpc.settings || {}
  return {
    serviceName: grpc.serviceName || genId(12),
    authority: grpc.authority || '',
    settings: {
      multiMode: s.multiMode || false,
      user_agent: s.user_agent || '',
      idle_timeout: s.idle_timeout || 0,
      health_check_timeout: s.health_check_timeout || 20,
      permit_without_stream: s.permit_without_stream || false,
      initial_windows_size: s.initial_windows_size || 0,
    },
  }
}

function parseWsSettings(ws) {
  // Support both nested {settings: {headers, heartbeatPeriod}} and flat {headers, heartbeatPeriod}
  const s = ws.settings || ws
  return {
    acceptProxyProtocol: ws.acceptProxyProtocol || false,
    path: ws.path || '/',
    host: ws.host || '',
    _headers: Object.entries(s.headers || {}).map(([k, v]) => ({ key: k, value: v })),
    heartbeatPeriod: s.heartbeatPeriod || 0,
  }
}

function parseRawSettings(raw) {
  const header = raw.header || {}
  const request = header.request || {}
  const response = header.response || {}
  return {
    acceptProxyProtocol: raw.acceptProxyProtocol || false,
    headerType: header.type || 'none',
    request: {
      version: request.version || '1.1',
      method: request.method || 'GET',
      _pathsStr: (request.path || ['/']).join(', '),
      _headers: Object.entries(request.headers || {}).map(([k, v]) => ({ key: k, value: Array.isArray(v) ? v[0] : v })),
    },
    response: {
      version: response.version || '1.1',
      status: response.status || '200',
      reason: response.reason || 'OK',
      _headers: Object.entries(response.headers || {}).map(([k, v]) => ({ key: k, value: Array.isArray(v) ? v[0] : v })),
    },
  }
}

function parseSettings(ib) {
  const settings = JSON.parse(JSON.stringify(ib.settings || {}))
  // Ensure accounts array exists for socks/http protocols
  if (['socks', 'http'].includes(ib.protocol) && !settings.accounts) {
    settings.accounts = []
  }
  // Convert WireGuard peers allowedIPs to _allowedIPsStr for editing
  if (ib.protocol === 'wireguard' && settings.peers) {
    settings.peers = settings.peers.map(function(p) {
      return {
        publicKey: p.publicKey || '',
        _allowedIPsStr: (p.allowedIPs || ['0.0.0.0/0', '::/0']).join(', '),
        keepAlive: p.keepAlive || 0,
        comment: p.comment || '',
      }
    })
  }
  return settings
}

function parseInbound(ib) {
  const ss = ib.streamSettings || {}
  const tls = ss.tlsSettings || {}
  const isSocket = ib.listen && ib.listen.startsWith('/')
  return {
    _id: ib._id || null,
    _remark: ib._remark || ib.tag || '', tag: ib.tag || '', protocol: ib.protocol || 'vless',
    listen: ib.listen || '0.0.0.0', port: ib.port || 0,
    _enabled: ib._enabled !== false,
    _listenType: isSocket ? 'sock' : 'tcp',
    _socketPath: isSocket ? ib.listen.split(',')[0] : '',
    _socketPerm: isSocket && ib.listen.includes(',') ? ib.listen.split(',')[1] : '0666',
    settings: parseSettings(ib),
    _portMap: ib.settings && ib.settings.portMap ? Object.entries(ib.settings.portMap).map(([local, remote]) => ({ local, remote })) : [],
    streamSettings: {
      network: ss.network || 'raw', security: ss.security || 'none',
      tls: { serverName: tls.serverName || '', alpnStr: (tls.alpn || []).join(', '), fingerprint: tls.fingerprint || '' },
      reality: parseRealitySettings(ss.realitySettings || {}),
      raw: parseRawSettings(ss.rawSettings || ss.tcpSettings || {}),
      ws: parseWsSettings(ss.wsSettings || {}),
      kcp: parseKcpSettings(ss.kcpSettings || {}),
      grpc: parseGrpcSettings(ss.grpcSettings || {}),
      httpupgrade: parseHttpupgradeSettings(ss.httpupgradeSettings || {}),
      xhttp: parseXhttpSettings(ss.xhttpSettings || ss.splithttpSettings || {}),
      hysteria: parseHysteriaSettings(ss.hysteriaSettings || {}),
      http: parseHttpSettings(ss.httpSettings || {}),
    },
    sniffing: ib.sniffing ? {
      enabled: ib.sniffing.enabled !== false,
      destOverride: ib.sniffing.destOverride || ['http', 'tls', 'quic'],
      metadataOnly: ib.sniffing.metadataOnly || false,
      _domainsExcludedStr: (ib.sniffing.domainsExcluded || []).join('\n'),
      _ipsExcludedStr: (ib.sniffing.ipsExcluded || []).join('\n'),
      routeOnly: ib.sniffing.routeOnly || false,
    } : { enabled: true, destOverride: ['http', 'tls', 'quic'], metadataOnly: false, _domainsExcludedStr: '', _ipsExcludedStr: '', routeOnly: false },
    _wgAddressesStr: (ib.settings?.address || []).join(', '),
    _wgPublicKey: ib._wgPublicKey || '',
    _tunGatewayStr: (ib.settings?.gateway || []).join(', '),
    _tunDnsStr: (ib.settings?.dns || []).join(', '),
    _tunAutoRoutingStr: (ib.settings?.autoSystemRoutingTable || []).join(', '),
  }
}

function buildInbound(ib) {
  const result = {
    _remark: ib._remark || undefined,
    tag: ib.tag || undefined, protocol: ib.protocol,
    settings: JSON.parse(JSON.stringify(ib.settings)),
    sniffing: {
      enabled: ib.sniffing.enabled || false,
      destOverride: ib.sniffing.destOverride || ['http', 'tls', 'quic'],
      metadataOnly: ib.sniffing.metadataOnly || false,
      domainsExcluded: ib.sniffing._domainsExcludedStr ? ib.sniffing._domainsExcludedStr.split('\n').map(d => d.trim()).filter(Boolean) : undefined,
      ipsExcluded: ib.sniffing._ipsExcludedStr ? ib.sniffing._ipsExcludedStr.split('\n').map(d => d.trim()).filter(Boolean) : undefined,
      routeOnly: ib.sniffing.routeOnly || false,
    },
  }
  // Only add streamSettings for protocols that use transport layer
  const noStreamProtocols = ['wireguard', 'tun', 'socks', 'http', 'tunnel', 'dokodemo-door']
  if (!noStreamProtocols.includes(ib.protocol)) {
    result.streamSettings = {
      network: ib.streamSettings.network,
      security: ib.streamSettings.security,
    }
  }
  // Handle listen/port based on type
  if (ib._listenType === 'sock') {
    result.listen = ib._socketPath + (ib._socketPerm ? ',' + ib._socketPerm : '')
    result.port = 0
  } else {
    result.listen = ib.listen || undefined
    result.port = ib.port
  }
  // Only add transport-specific settings when streamSettings exists (protocols with transport layer)
  if (result.streamSettings) {
    // Add TLS settings if security is tls
    if (ib.streamSettings.security === 'tls') {
      const tls = ib.streamSettings.tls || {}
      const tlsSettings = {}
      if (tls.serverName) tlsSettings.serverName = tls.serverName
      if (tls.alpnStr) tlsSettings.alpn = tls.alpnStr.split(',').map(s => s.trim()).filter(Boolean)
      if (tls.fingerprint) tlsSettings.fingerprint = tls.fingerprint
      result.streamSettings.tlsSettings = tlsSettings
    }
    // Add Reality settings if security is reality
    if (ib.streamSettings.security === 'reality') {
      const r = ib.streamSettings.reality || {}
      const realitySettings = {}
      if (r.target) realitySettings.target = r.target
      if (r.serverNamesStr) realitySettings.serverNames = r.serverNamesStr.split(',').map(s => s.trim()).filter(Boolean)
      if (r.privateKey) realitySettings.privateKey = r.privateKey
      if (r.shortId) realitySettings.shortId = r.shortId
      if (r.fingerprint) realitySettings.fingerprint = r.fingerprint
      result.streamSettings.realitySettings = realitySettings
    }
    // Add RAW settings if network is raw
    if (ib.streamSettings.network === 'raw' && ib.streamSettings.raw) {
      const raw = ib.streamSettings.raw
      const rawSettings = {}
      if (raw.acceptProxyProtocol) rawSettings.acceptProxyProtocol = true
      if (raw.headerType === 'http') {
        const headersToMap = (headers) => {
          const map = {}
          headers.forEach(h => { if (h.key) map[h.key] = [h.value] })
          return Object.keys(map).length ? map : undefined
        }
        rawSettings.header = {
          type: 'http',
          request: {
            version: raw.request.version || '1.1',
            method: raw.request.method || 'GET',
            path: raw.request._pathsStr ? raw.request._pathsStr.split(',').map(s => s.trim()).filter(Boolean) : ['/'],
            headers: headersToMap(raw.request._headers),
          },
          response: {
            version: raw.response.version || '1.1',
            status: raw.response.status || '200',
            reason: raw.response.reason || 'OK',
            headers: headersToMap(raw.response._headers),
          },
        }
      } else {
        rawSettings.header = { type: 'none' }
      }
      result.streamSettings.rawSettings = rawSettings
    }
    // Add WebSocket settings if network is ws
    if (ib.streamSettings.network === 'ws' && ib.streamSettings.ws) {
      const ws = ib.streamSettings.ws
      const wsSettings = {}
      if (ws.acceptProxyProtocol) wsSettings.acceptProxyProtocol = true
      if (ws.path && ws.path !== '/') wsSettings.path = ws.path
      if (ws.host && ws.host.trim()) wsSettings.host = ws.host.trim()
      if (ws._headers && ws._headers.length) {
        const headers = {}
        ws._headers.forEach(h => { if (h.key) headers[h.key] = h.value })
        if (Object.keys(headers).length) wsSettings.headers = headers
      }
      if (ws.heartbeatPeriod) wsSettings.heartbeatPeriod = ws.heartbeatPeriod
      result.streamSettings.wsSettings = wsSettings
    }
    // Add XHTTP settings if network is xhttp
    if (ib.streamSettings.network === 'xhttp' && ib.streamSettings.xhttp) {
      const xh = ib.streamSettings.xhttp
      const xhSettings = {}
      if (xh.path && xh.path !== '/') xhSettings.path = xh.path
      if (xh.host && xh.host.trim()) xhSettings.host = xh.host.trim()
      if (xh.mode && xh.mode !== 'auto') xhSettings.mode = xh.mode
      // Extra settings
      const s = xh.extra || {}
      const extra = {}
      // Client settings (only write if explicitly set)
      if (s._showMethod && s.uplinkHTTPMethod) extra.uplinkHTTPMethod = s.uplinkHTTPMethod
      if (s.noGRPCHeader) extra.noGRPCHeader = true
      if (s.noSSEHeader) extra.noSSEHeader = true
      if (s._showMaxHeader && s.serverMaxHeaderBytes) extra.serverMaxHeaderBytes = s.serverMaxHeaderBytes
      if (s._headers && s._headers.length) {
        const headers = {}
        s._headers.forEach(h => { if (h.key) headers[h.key] = h.value })
        if (Object.keys(headers).length) extra.headers = headers
      }
      // Padding
      if (s.xPaddingBytes && s.xPaddingBytes !== '100-1000') extra.xPaddingBytes = s.xPaddingBytes
      if (s.xPaddingObfsMode) extra.xPaddingObfsMode = true
      // Scamper (only write if non-default)
      if (s.scMaxEachPostBytes) extra.scMaxEachPostBytes = s.scMaxEachPostBytes
      if (s.scMinPostsIntervalMs) extra.scMinPostsIntervalMs = s.scMinPostsIntervalMs
      if (s.scMaxBufferedPosts && s.scMaxBufferedPosts !== 30) extra.scMaxBufferedPosts = s.scMaxBufferedPosts
      if (s.scStreamUpServerSecs && s.scStreamUpServerSecs !== '20-80') extra.scStreamUpServerSecs = s.scStreamUpServerSecs
      // Xmux
      const xmux = s.xmux || {}
      const xmuxObj = {}
      if (xmux.maxConcurrency && xmux.maxConcurrency !== '16-32') xmuxObj.maxConcurrency = xmux.maxConcurrency
      if (xmux.maxConnections) xmuxObj.maxConnections = xmux.maxConnections
      if (xmux.cMaxReuseTimes) xmuxObj.cMaxReuseTimes = xmux.cMaxReuseTimes
      if (xmux.hMaxRequestTimes && xmux.hMaxRequestTimes !== '600-900') xmuxObj.hMaxRequestTimes = xmux.hMaxRequestTimes
      if (xmux.hMaxReusableSecs && xmux.hMaxReusableSecs !== '1800-3000') xmuxObj.hMaxReusableSecs = xmux.hMaxReusableSecs
      if (xmux.hKeepAlivePeriod) xmuxObj.hKeepAlivePeriod = xmux.hKeepAlivePeriod
      if (Object.keys(xmuxObj).length) extra.xmux = xmuxObj
      if (Object.keys(extra).length) xhSettings.extra = extra
      result.streamSettings.xhttpSettings = xhSettings
    }
    // Add Hysteria settings if network is hysteria
    if (ib.streamSettings.network === 'hysteria' && ib.streamSettings.hysteria) {
      const hy = ib.streamSettings.hysteria
      const hySettings = { version: 2 }
      if (hy.auth) hySettings.auth = hy.auth
      if (hy.udpIdleTimeout && hy.udpIdleTimeout !== 60) hySettings.udpIdleTimeout = hy.udpIdleTimeout
      const masq = hy.masquerade || {}
      if (masq.type) {
        const masqObj = { type: masq.type }
        if (masq.type === 'file' && masq.dir) masqObj.dir = masq.dir
        if (masq.type === 'proxy') {
          if (masq.url) masqObj.url = masq.url
          if (masq.rewriteHost) masqObj.rewriteHost = true
          if (masq.insecure) masqObj.insecure = true
        }
        if (masq.type === 'string') {
          if (masq.content) masqObj.content = masq.content
          if (masq.statusCode) masqObj.statusCode = masq.statusCode
          if (masq._headers && masq._headers.length) {
            const headers = {}
            masq._headers.forEach(h => { if (h.key) headers[h.key] = h.value })
            if (Object.keys(headers).length) masqObj.headers = headers
          }
        }
        hySettings.masquerade = masqObj
      }
      result.streamSettings.hysteriaSettings = hySettings
    }
    // Add HTTPUpgrade settings if network is httpupgrade
    if (ib.streamSettings.network === 'httpupgrade' && ib.streamSettings.httpupgrade) {
      const hu = ib.streamSettings.httpupgrade
      const huSettings = {}
      if (hu.acceptProxyProtocol) huSettings.acceptProxyProtocol = true
      if (hu.path && hu.path !== '/') huSettings.path = hu.path
      if (hu.host && hu.host.trim()) huSettings.host = hu.host.trim()
      if (hu._headers && hu._headers.length) {
        const headers = {}
        hu._headers.forEach(h => { if (h.key) headers[h.key] = h.value })
        if (Object.keys(headers).length) huSettings.headers = headers
      }
      result.streamSettings.httpupgradeSettings = huSettings
    }
    // Add mKCP settings if network is mkcp
    if (ib.streamSettings.network === 'mkcp' && ib.streamSettings.kcp) {
      const kcp = ib.streamSettings.kcp
      result.streamSettings.kcpSettings = {
        mtu: kcp.mtu || 1350,
        tti: kcp.tti || 50,
        uplinkCapacity: kcp.uplinkCapacity || 5,
        downlinkCapacity: kcp.downlinkCapacity || 20,
        congestion: kcp.congestion || false,
        readBufferSize: kcp.readBufferSize || 2,
        writeBufferSize: kcp.writeBufferSize || 2,
      }
    }
    // Add HTTP/2 settings if network is http
    if (ib.streamSettings.network === 'http' && ib.streamSettings.http) {
      const http = ib.streamSettings.http
      const httpSettings = {}
      if (http._hostStr) httpSettings.host = http._hostStr.split(',').map(s => s.trim()).filter(Boolean)
      if (http.path && http.path !== '/') httpSettings.path = http.path
      if (Object.keys(httpSettings).length) result.streamSettings.httpSettings = httpSettings
    }
    // Add gRPC settings if network is grpc
    if (ib.streamSettings.network === 'grpc' && ib.streamSettings.grpc) {
      const grpc = ib.streamSettings.grpc
      const grpcSettings = {}
      if (grpc.serviceName) grpcSettings.serviceName = grpc.serviceName
      if (grpc.authority) grpcSettings.authority = grpc.authority
      // Client settings nested under "settings"
      const s = grpc.settings || {}
      const clientSettings = {}
      if (s.multiMode) clientSettings.multiMode = true
      if (s.user_agent) clientSettings.user_agent = s.user_agent
      if (s.idle_timeout) clientSettings.idle_timeout = s.idle_timeout
      if (s.health_check_timeout && s.health_check_timeout !== 20) clientSettings.health_check_timeout = s.health_check_timeout
      if (s.permit_without_stream) clientSettings.permit_without_stream = true
      if (s.initial_windows_size) clientSettings.initial_windows_size = s.initial_windows_size
      if (Object.keys(clientSettings).length) grpcSettings.settings = clientSettings
      result.streamSettings.grpcSettings = grpcSettings
    }
  }
  // Handle tunnel portMap
  if (ib.protocol === 'tunnel' && ib._portMap && ib._portMap.length) {
    const portMap = {}
    ib._portMap.forEach(m => { if (m.local && m.remote) portMap[m.local] = m.remote })
    if (Object.keys(portMap).length) result.settings.portMap = portMap
  }
  // Handle WireGuard peers
  if (ib.protocol === 'wireguard') {
    // Convert peer _allowedIPsStr to allowedIPs array, filter out peers with empty publicKey
    if (result.settings.peers) {
      result.settings.peers = result.settings.peers.filter(function(p) { return p.publicKey && p.publicKey.trim() }).map(function(p) {
        const peer = { publicKey: p.publicKey }
        if (p._allowedIPsStr) peer.allowedIPs = p._allowedIPsStr.split(',').map(function(s) { return s.trim() }).filter(Boolean)
        if (p.keepAlive) peer.keepAlive = p.keepAlive
        return peer
      })
    }
    // Remove empty fields, keep mtu and noKernelTun per official docs
    if (!result.settings.secretKey) delete result.settings.secretKey
    // noKernelTun: true is required to avoid kernel UAPI errors, keep it
    if (result.settings.noKernelTun !== true) delete result.settings.noKernelTun
    // workers: 0 means auto, don't write to config but UI still shows it
    if (!result.settings.workers) delete result.settings.workers
    delete result.settings.address
    delete result.settings.domainStrategy
  }
  // Handle TUN settings
  if (ib.protocol === 'tun') {
    if (ib._tunGatewayStr) result.settings.gateway = ib._tunGatewayStr.split(',').map(s => s.trim()).filter(Boolean)
    if (ib._tunDnsStr) result.settings.dns = ib._tunDnsStr.split(',').map(s => s.trim()).filter(Boolean)
    if (ib._tunAutoRoutingStr) result.settings.autoSystemRoutingTable = ib._tunAutoRoutingStr.split(',').map(s => s.trim()).filter(Boolean)
    // Remove empty fields
    if (!result.settings.name) delete result.settings.name
    if (!result.settings.mtu || result.settings.mtu === 1500) delete result.settings.mtu
    if (!result.settings.userLevel) delete result.settings.userLevel
    if (!result.settings.autoOutboundsInterface) delete result.settings.autoOutboundsInterface
  }
  // Remove internal fields from settings
  delete result.settings._wgAddressesStr
  delete result.settings._tunGatewayStr
  delete result.settings._tunDnsStr
  // Remove internal fields from users/clients/accounts (keep in db but don't write to config)
  for (const key of ['users', 'clients', 'accounts']) {
    if (result.settings[key]) {
      result.settings[key] = result.settings[key].map(u => {
        const clean = { ...u }
        delete clean._quota
        delete clean._expiry
        return clean
      })
      // Remove empty arrays
      if (!result.settings[key].length) delete result.settings[key]
    }
  }
  return result
}

function showAdd() {
  editIdx.value = -1
  const d = getDefaultInbound()
  rf._remark = d._remark
  rf.tag = d.tag
  rf.protocol = d.protocol
  rf.listen = d.listen
  rf.port = d.port
  rf._listenType = d._listenType
  rf._socketPath = d._socketPath
  rf._socketPerm = d._socketPerm
  rf._enabled = d._enabled
  rf.settings = d.settings
  rf.streamSettings = d.streamSettings
  rf.sniffing = d.sniffing
  rf._wgAddressesStr = d._wgAddressesStr
  rf._tunGatewayStr = d._tunGatewayStr
  rf._tunDnsStr = d._tunDnsStr
  rf._tunAutoRoutingStr = d._tunAutoRoutingStr
  activeTab.value = 'basic'
  dlg.value = true
}

function showEdit(i) {
  editIdx.value = i
  // form.value[i] is already parsed, just assign directly
  const ib = form.value[i]
  rf._id = ib._id || null
  rf._remark = ib._remark || ''
  rf.tag = ib.tag || ''
  rf.protocol = ib.protocol || 'vless'
  rf.listen = ib.listen || '0.0.0.0'
  rf.port = ib.port || 0
  rf._enabled = ib._enabled !== false
  rf._listenType = ib._listenType || 'tcp'
  rf._socketPath = ib._socketPath || ''
  rf._socketPerm = ib._socketPerm || '0666'
  rf.settings = JSON.parse(JSON.stringify(ib.settings || {}))
  rf.streamSettings = JSON.parse(JSON.stringify(ib.streamSettings || {}))
  rf.sniffing = JSON.parse(JSON.stringify(ib.sniffing || { enabled: false, destOverride: ['http', 'tls'], routeOnly: false }))
  rf._wgAddressesStr = ib._wgAddressesStr || ''
  rf._wgPublicKey = ib._wgPublicKey || ''
  rf._tunGatewayStr = ib._tunGatewayStr || ''
  rf._tunDnsStr = ib._tunDnsStr || ''
  rf._tunAutoRoutingStr = ib._tunAutoRoutingStr || ''
  activeTab.value = 'basic'
  dlg.value = true
}

async function saveInbound() {
  const ib = buildInbound(rf)
  // Store the full inbound object as config_json
  const configJson = { ...ib }
  delete configJson._remark
  delete configJson._enabled
  delete configJson._id
  // Persist server public key (derived from secretKey) for display on reload
  if (ib.protocol === 'wireguard' && rf._wgPublicKey) {
    configJson._wgPublicKey = rf._wgPublicKey
  }

  const payload = {
    core_type: props.coreType || 'xray',
    remark: ib._remark || ib.tag || '',
    protocol: ib.protocol,
    tag: ib.tag,
    listen: ib.listen || '0.0.0.0',
    port: ib.port || 0,
    enabled: ib._enabled !== false,
    config_json: configJson,
  }
  try {
    if (editIdx.value >= 0 && form.value[editIdx.value]._id) {
      await client.put(`/inbound-configs/${form.value[editIdx.value]._id}`, payload)
    } else {
      await client.post('/inbound-configs', payload)
    }
    ElMessage.success('保存成功')
    dlg.value = false
    await loadInbounds()
  } catch (e) {
    console.error('Save inbound failed:', e)
    ElMessage.error(e?.response?.data?.message || '保存失败')
  }
}

async function toggleEnabled(i) {
  const ib = form.value[i]
  if (!ib._id) return
  const configJson = buildInbound(ib)
  delete configJson._remark
  delete configJson._enabled
  delete configJson._id
  try {
    await client.put(`/inbound-configs/${ib._id}`, {
      core_type: props.coreType || 'xray',
      remark: ib._remark || '',
      protocol: ib.protocol,
      tag: ib.tag,
      listen: ib.listen || '0.0.0.0',
      port: ib.port || 0,
      enabled: ib._enabled,
      config_json: configJson,
    })
    ElMessage.success(ib._enabled ? '已启用' : '已禁用')
  } catch (e) {
    ib._enabled = !ib._enabled
    ElMessage.error('操作失败')
  }
}

async function rmInbound(i) {
  const ib = form.value[i]
  if (ib._id) {
    try { await client.delete(`/inbound-configs/${ib._id}`) } catch {}
  }
  form.value.splice(i, 1)
  ElMessage.success('删除成功')
}

async function loadInbounds() {
  try {
    const data = await client.get('/inbound-configs', { params: { core_type: props.coreType || 'xray' } })
    form.value = (data || []).map(item => {
      const cfg = typeof item.config_json === 'string' ? JSON.parse(item.config_json) : (item.config_json || {})
      return { ...parseInbound(cfg), _id: item.id, _enabled: item.enabled }
    })
  } catch { form.value = [] }
}

function emit() { /* no longer needed, saves directly to API */ }

onMounted(loadInbounds)
</script>

<style scoped>
.inbound-item { background: var(--el-fill-color-light); border-radius: 6px; padding: 12px; margin-bottom: 8px; }
.user-item { background: var(--el-fill-color); border-radius: 4px; padding: 8px; margin-bottom: 8px; border: 1px solid var(--el-border-color-lighter); }
:deep(.el-dialog__body) { max-height: calc(100vh - 200px); overflow-y: auto; }
</style>
