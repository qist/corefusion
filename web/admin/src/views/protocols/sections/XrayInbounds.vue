<template>
  <div>
    <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:16px">
      <span style="font-size:15px;font-weight:600">{{ t('protoCenter.inbounds.title') }}</span>
      <div style="display:flex;gap:8px">
        <el-button size="small" type="primary" @click="showAdd">{{ t('protoCenter.inbounds.add') }}</el-button>
      </div>
    </div>

    <!-- Stats Summary -->
    <div style="display:flex;gap:24px;margin-bottom:16px;padding:12px 16px;background:var(--el-fill-color-light);border-radius:8px;font-size:13px">
      <div>
        <span style="color:#909399">{{ t('protoCenter.inbounds.totalUploadDownload') }}: </span>
        <span style="font-weight:500">↑ {{ formatTraffic(totalUpload) }} / ↓ {{ formatTraffic(totalDownload) }}</span>
      </div>
      <div>
        <span style="color:#909399">{{ t('protoCenter.inbounds.totalUsage') }}: </span>
        <span style="font-weight:500">{{ formatTraffic(totalUpload + totalDownload) }}</span>
      </div>
      <div>
        <span style="color:#909399">{{ t('protoCenter.inbounds.inboundCount') }}: </span>
        <span style="font-weight:500">{{ form.length }}</span>
      </div>
      <el-button size="small" type="warning" @click="resetAllTraffic">{{ t('protoCenter.inbounds.resetAllTraffic') }}</el-button>
      <el-button size="small" @click="loadInbounds" :loading="statsLoading">
        <el-icon><Refresh /></el-icon> {{ t('common.refresh') }}
      </el-button>
    </div>

    <!-- Table List -->
    <el-table :data="form" stripe style="width:100%" size="small">
      <el-table-column label="ID" width="50" align="right">
        <template #default="{row, $index}">
          <span style="font-size:11px;opacity:0.6">{{ row._id || ($index + 1) }}</span>
        </template>
      </el-table-column>
      <el-table-column :label="t('common.actions')" width="70" align="center">
        <template #default="{row, $index}">
          <el-dropdown trigger="click" @command="(cmd) => handleAction(cmd, row, $index)">
            <el-icon style="font-size:18px;cursor:pointer"><MoreFilled /></el-icon>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="edit">
                  <el-icon><Edit /></el-icon> {{ t('common.edit') }}
                </el-dropdown-item>
                <el-dropdown-item v-if="hasShareableUsers(row)" command="qrcode" divided>
                  <el-icon><View /></el-icon> {{ t('protoCenter.inbounds.qrCode') }}
                </el-dropdown-item>
                <el-dropdown-item v-if="hasUsers(row)" command="reset-traffic">
                  <el-icon><RefreshRight /></el-icon> {{ t('protoCenter.inbounds.resetTraffic') }}
                </el-dropdown-item>
                <el-dropdown-item v-if="canAddUsers(row)" command="add-user" divided>
                  <el-icon><Plus /></el-icon> {{ t('protoCenter.inbounds.addUser') }}
                </el-dropdown-item>
                <el-dropdown-item v-if="hasUserList(row)" command="manage-users">
                  <el-icon><User /></el-icon> {{ t('protoCenter.inbounds.manageUsers') }}
                </el-dropdown-item>
                <el-dropdown-item command="delete" divided>
                  <span style="color:#f56c6c"><el-icon><Delete /></el-icon> {{ t('common.delete') }}</span>
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </template>
      </el-table-column>
      <el-table-column :label="t('common.enabled')" width="60" align="center">
        <template #default="{row, $index}">
          <el-switch v-model="row._enabled" size="small" @change="toggleEnabled($index)" />
        </template>
      </el-table-column>
      <el-table-column :label="t('protoCenter.inbounds.remark')" min-width="120" show-overflow-tooltip>
        <template #default="{row}">
          <span>{{ row._remark || '-' }}</span>
        </template>
      </el-table-column>
      <el-table-column :label="t('protoCenter.inbounds.port')" width="70" align="center">
        <template #default="{row}">
          <span style="font-weight:500">{{ row.port }}</span>
        </template>
      </el-table-column>
      <el-table-column :label="t('protoCenter.inbounds.protocol')" min-width="180" align="left">
        <template #default="{row}">
          <div style="display:inline-flex;flex-wrap:wrap;gap:4px">
            <el-tag size="small" color="purple" style="color:#fff">{{ row.protocol.toUpperCase() }}</el-tag>
            <el-tag size="small" color="success">{{ row.streamSettings?.network || 'raw' }}</el-tag>
            <el-tag v-if="row.streamSettings?.security === 'tls'" size="small" color="primary">TLS</el-tag>
            <el-tag v-if="row.streamSettings?.security === 'reality'" size="small" color="primary">Reality</el-tag>
          </div>
        </template>
      </el-table-column>
      <el-table-column :label="t('protoCenter.inbounds.users')" width="120" align="left">
        <template #default="{row}">
          <div style="display:flex;gap:6px;align-items:center">
            <el-tag size="small" style="margin:0">
              <el-icon><User /></el-icon> {{ getUserCount(row) }}
            </el-tag>
            <el-tag v-if="getActiveUserCount(row) > 0" size="small" color="success" style="margin:0">{{ getActiveUserCount(row) }}</el-tag>
            <el-tag v-if="getExpiredUserCount(row) > 0" size="small" color="danger" style="margin:0">{{ getExpiredUserCount(row) }}</el-tag>
          </div>
        </template>
      </el-table-column>
      <el-table-column :label="t('protoCenter.inbounds.traffic')" width="120" align="center">
        <template #default="{row}">
          <el-popover trigger="hover" width="180">
            <template #reference>
              <el-tag :color="getTrafficColor(row)" style="cursor:pointer;min-width:80px">
                {{ formatTraffic(getTotalTraffic(row, 'up') + getTotalTraffic(row, 'down')) }}
              </el-tag>
            </template>
            <div style="font-size:12px">
              <div>↑ {{ formatTraffic(getTotalTraffic(row, 'up')) }}</div>
              <div>↓ {{ formatTraffic(getTotalTraffic(row, 'down')) }}</div>
            </div>
          </el-popover>
        </template>
      </el-table-column>
      <el-table-column :label="t('protoCenter.inbounds.expiry')" width="80" align="center">
        <template #default="{row}">
          <el-tag :color="getExpiryColor(row)" size="small" style="min-width:50px">
            {{ getExpiryDisplay(row) }}
          </el-tag>
        </template>
      </el-table-column>
    </el-table>

    <!-- Details Dialog -->
    <el-dialog v-model="detailsDlg" :title="t('protoCenter.inbounds.details')" width="700" top="5vh">
      <div v-if="detailsInbound">
        <!-- Config Info -->
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item :label="t('protoCenter.inbounds.tag')">{{ detailsInbound.tag }}</el-descriptions-item>
          <el-descriptions-item :label="t('protoCenter.inbounds.protocol')">{{ detailsInbound.protocol?.toUpperCase() }}</el-descriptions-item>
          <el-descriptions-item :label="t('protoCenter.inbounds.port')">{{ detailsInbound.port }}</el-descriptions-item>
          <el-descriptions-item :label="t('protoCenter.inbounds.transport')">{{ detailsInbound.streamSettings?.network || 'raw' }} {{ detailsInbound.streamSettings?.security !== 'none' ? detailsInbound.streamSettings?.security : '' }}</el-descriptions-item>
          <el-descriptions-item :label="t('protoCenter.inbounds.remark')">{{ detailsInbound._remark || '-' }}</el-descriptions-item>
          <el-descriptions-item :label="t('common.status')">
            <el-tag :type="detailsInbound._enabled ? 'success' : 'danger'" size="small">{{ detailsInbound._enabled ? t('common.enabled') : t('common.disabled') }}</el-tag>
          </el-descriptions-item>
        </el-descriptions>

        <!-- Users -->
        <div v-if="getUsers(detailsInbound).length" style="margin-top:16px">
          <h4 style="margin-bottom:12px">{{ t('protoCenter.inbounds.users') }}</h4>
          <div v-for="(u, ui) in getUsers(detailsInbound)" :key="ui" style="border:1px solid #eee;border-radius:6px;padding:12px;margin-bottom:8px">
            <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:8px">
              <div>
                <span style="font-weight:500">{{ u.email || u.user || u.id?.substring(0, 8) || 'User ' + (ui + 1) }}</span>
                <el-tag v-if="u.flow" size="small" style="margin-left:8px">{{ u.flow }}</el-tag>
              </div>
              <el-button size="small" type="primary" @click="copyShareLink(detailsInbound, u)">
                {{ t('protoCenter.inbounds.copyLink') }}
              </el-button>
            </div>
            <div style="font-size:12px;color:#909399">
              <span v-if="u.id">UUID: {{ u.id }}</span>
              <span v-if="u._usedUp || u._usedDown" style="margin-left:12px">↑ {{ formatTraffic(u._usedUp || 0) }} ↓ {{ formatTraffic(u._usedDown || 0) }}</span>
            </div>
          </div>
        </div>

        <!-- Raw Config -->
        <el-collapse style="margin-top:16px">
          <el-collapse-item :title="t('protoCenter.inbounds.rawConfig')">
            <pre style="font-size:12px;max-height:300px;overflow:auto;background:#f5f5f5;padding:12px;border-radius:4px">{{ formatJson(detailsInbound) }}</pre>
          </el-collapse-item>
        </el-collapse>
      </div>
    </el-dialog>

    <!-- QR Code Dialog -->
    <el-dialog v-model="qrDlg" :title="t('protoCenter.inbounds.qrCode')" width="400" top="10vh">
      <div v-if="qrUsers.length > 1" style="margin-bottom:16px">
        <el-select v-model="qrSelectedUser" style="width:100%" @change="onQrUserChange">
          <el-option v-for="u in qrUsers" :key="u.email || u.id" :label="u.email || u.id" :value="u.email || u.id" />
        </el-select>
      </div>
      <div style="text-align:center;padding:20px">
        <div v-if="qrCodeUrl" style="margin-bottom:16px">
          <img :src="qrCodeUrl" style="max-width:250px" />
        </div>
        <div v-else-if="qrGenerating" style="margin-bottom:16px;height:250px;display:flex;align-items:center;justify-content:center;color:#909399">
          {{ t('protoCenter.inbounds.qrGenerating') }}
        </div>
        <el-input v-model="qrLink" type="textarea" :rows="3" readonly style="margin-top:12px" />
        <el-button size="small" style="margin-top:8px" @click="copyQrLink">{{ t('protoCenter.inbounds.copyLink') }}</el-button>
      </div>
    </el-dialog>

    <!-- Reset Traffic Dialog -->
    <el-dialog v-model="resetDlg" :title="t('protoCenter.inbounds.resetTraffic')" width="400">
      <div v-if="resetUsers.length">
        <p style="margin-bottom:12px">{{ t('protoCenter.inbounds.selectUserToReset') }}:</p>
        <el-radio-group v-model="resetSelectedUser" style="display:flex;flex-direction:column;gap:8px">
          <el-radio v-for="u in resetUsers" :key="u.email" :value="u.email" style="margin:0">
            <span>{{ u.email || u.id }}</span>
            <span style="color:#909399;font-size:12px;margin-left:8px">↑ {{ formatTraffic(u._usedUp || 0) }} ↓ {{ formatTraffic(u._usedDown || 0) }}</span>
          </el-radio>
        </el-radio-group>
      </div>
      <template #footer>
        <el-button @click="resetDlg=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="confirmResetTraffic">{{ t('common.confirm') }}</el-button>
      </template>
    </el-dialog>

    <!-- Add User Dialog -->
    <el-dialog v-model="addUserDlg" :title="t('protoCenter.inbounds.addUser')" width="500">
      <el-form label-width="100px" label-position="left">
        <el-form-item label="Email">
          <el-input v-model="newUser.email" placeholder="user@example.com" />
        </el-form-item>
        <el-form-item v-if="['vless','vmess'].includes(addUserProtocol)" label="UUID">
          <div style="display:flex;gap:8px;width:100%">
            <el-input v-model="newUser.id" style="flex:1" />
            <el-button size="small" @click="newUser.id = genUUID()">Generate</el-button>
          </div>
        </el-form-item>
        <el-form-item v-if="addUserProtocol === 'vless' && canEnableFlowForInbound(addUserTag)" label="Flow">
          <el-select v-model="newUser.flow" style="width:100%" clearable>
            <el-option label="none" value="" />
            <el-option label="xtls-rprx-vision" value="xtls-rprx-vision" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="['trojan','hysteria'].includes(addUserProtocol)" label="Password">
          <div style="display:flex;gap:8px;width:100%">
            <el-input v-model="newUser.password" style="flex:1" />
            <el-button size="small" @click="newUser.password = genBase64Password()">Generate</el-button>
          </div>
        </el-form-item>
        <el-form-item v-if="addUserProtocol === 'shadowsocks' && isSSMultiUser(addUserSSMethod)" label="Password">
          <div style="display:flex;gap:8px;width:100%">
            <el-input v-model="newUser.password" style="flex:1" />
            <el-button size="small" @click="newUser.password = genSSPassword(addUserSSMethod)">Generate</el-button>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="addUserDlg=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="confirmAddUser">{{ t('common.confirm') }}</el-button>
      </template>
    </el-dialog>

    <!-- Manage Users Dialog -->
    <el-dialog v-model="manageUsersDlg" :title="t('protoCenter.inbounds.manageUsers')" width="600">
      <div v-if="manageUsersList.length">
        <div v-for="(u, ui) in manageUsersList" :key="ui" style="display:flex;justify-content:space-between;align-items:center;padding:12px;border-bottom:1px solid #eee">
          <div>
            <div style="display:flex;align-items:center;gap:8px">
              <span style="font-weight:500">{{ u.email || u.id }}</span>
              <el-tag v-if="u._paused" size="small" type="warning">Paused</el-tag>
              <el-tag v-else size="small" type="success">Active</el-tag>
            </div>
            <div style="font-size:12px;color:#909399">
              <span v-if="u.id">UUID: {{ u.id.substring(0, 8) }}...</span>
              <span v-if="u.flow"> | Flow: {{ u.flow }}</span>
            </div>
          </div>
          <div style="display:flex;gap:8px">
            <el-button v-if="u._paused" size="small" type="success" @click="resumeUser(u)">{{ t('protoCenter.inbounds.resume') }}</el-button>
            <el-button v-else size="small" type="warning" @click="pauseUser(u)">{{ t('protoCenter.inbounds.pause') }}</el-button>
            <el-button size="small" type="danger" plain @click="removeUserFromXray(u)">{{ t('protoCenter.inbounds.removeFromXray') }}</el-button>
          </div>
        </div>
      </div>
      <div v-else style="text-align:center;color:#909399;padding:20px">
        No users found
      </div>
    </el-dialog>

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

            <!-- Access Domain Switch -->
            <el-divider content-position="left">{{ t('protoCenter.inbounds.accessDomain') }}</el-divider>
            <el-form-item :label="t('protoCenter.inbounds.enableAccessDomain')">
              <el-switch v-model="rf._accessDomainEnabled" />
              <span style="font-size:12px;color:#909399;margin-left:8px">{{ t('protoCenter.inbounds.accessDomainHint') }}</span>
            </el-form-item>
            <template v-if="rf._accessDomainEnabled">
              <el-form-item :label="t('protoCenter.inbounds.accessDomainType')">
                <el-select v-model="rf._accessDomainType" style="width:100%">
                  <el-option :label="t('protoCenter.inbounds.accessTypeDirect')" value="direct" />
                  <el-option :label="t('protoCenter.inbounds.accessTypeCFDomain')" value="cf_domain" />
                  <el-option :label="t('protoCenter.inbounds.accessTypeCFIP')" value="cf_ip" />
                  <el-option :label="t('protoCenter.inbounds.accessTypeCustom')" value="custom" />
                </el-select>
              </el-form-item>
              <!-- Direct -->
              <template v-if="rf._accessDomainType === 'direct'">
                <el-form-item :label="t('protoCenter.inbounds.directDomain')">
                  <el-select v-model="rf._directDomain" style="width:100%" filterable clearable :placeholder="t('protoCenter.inbounds.directDomainPlaceholder')">
                    <el-option v-for="d in directDomains" :key="d.id" :label="d.value" :value="d.value" />
                  </el-select>
                  <div class="hint">{{ t('protoCenter.inbounds.directDomainHint') }}</div>
                </el-form-item>
                <el-form-item :label="t('protoCenter.inbounds.directPortMode')">
                  <el-radio-group v-model="rf._directPortMode">
                    <el-radio value="config">{{ t('protoCenter.inbounds.directPortConfig') }}</el-radio>
                    <el-radio value="inbound">{{ t('protoCenter.inbounds.directPortInbound') }}</el-radio>
                  </el-radio-group>
                </el-form-item>
              </template>
              <!-- CF Domain -->
              <el-form-item v-if="rf._accessDomainType === 'cf_domain'" :label="t('protoCenter.inbounds.cfDomainSelect')">
                <el-select v-model="rf._cfDomain" style="width:100%" filterable clearable :placeholder="t('protoCenter.inbounds.cfDomainPlaceholder')">
                  <el-option v-for="d in cfDomains" :key="d.id" :label="d.value" :value="d.value" />
                </el-select>
                <div class="hint">{{ t('protoCenter.inbounds.cfDomainHint') }}</div>
              </el-form-item>
              <!-- CF IP -->
              <el-form-item v-if="rf._accessDomainType === 'cf_ip'" :label="t('protoCenter.inbounds.cfIPSelect')">
                <el-select v-model="rf._cfIP" style="width:100%" filterable clearable :placeholder="t('protoCenter.inbounds.cfIPPlaceholder')">
                  <el-option v-for="e in cfIPEntries" :key="e.id" :label="e.ip + (e.country ? ' (' + e.country + ')' : '')" :value="e.ip" />
                </el-select>
                <div class="hint">{{ t('protoCenter.inbounds.cfIPHint') }}</div>
              </el-form-item>
              <!-- Custom -->
              <template v-if="rf._accessDomainType === 'custom'">
                <el-form-item :label="t('protoCenter.inbounds.customDomain')">
                  <el-input v-model="rf._customDomain" :placeholder="t('protoCenter.inbounds.customDomainPlaceholder')" />
                </el-form-item>
                <el-form-item :label="t('protoCenter.inbounds.directPortMode')">
                  <el-radio-group v-model="rf._customPortMode">
                    <el-radio value="custom">{{ t('protoCenter.inbounds.customPortCustom') }}</el-radio>
                    <el-radio value="inbound">{{ t('protoCenter.inbounds.directPortInbound') }}</el-radio>
                  </el-radio-group>
                </el-form-item>
                <el-form-item v-if="rf._customPortMode === 'custom'" :label="t('protoCenter.inbounds.customPort')">
                  <el-input-number v-model="rf._accessCustomPort" :min="1" :max="65535" />
                </el-form-item>
              </template>
            </template>
          </el-form>
        </el-tab-pane>

        <!-- Protocol Tab -->
        <el-tab-pane :label="t('protoCenter.inbounds.protocolSettings')" name="protocol">
          <!-- VLESS -->
          <template v-if="rf.protocol === 'vless'">
            <el-form label-width="120px" label-position="left">
              <el-form-item label="Encryption">
                <div style="display:flex;gap:8px;width:100%">
                  <el-select v-model="rf.settings._encryptionType" style="flex:1" clearable @change="onVlessEncryptionTypeChange">
                    <el-option label="None" value="" />
                    <el-option label="X25519 (Classic)" value="x25519" />
                    <el-option label="ML-KEM-768 (Post-Quantum)" value="mlkem768" />
                  </el-select>
                  <el-button size="small" @click="generateVlessEnc" :disabled="!rf.settings._encryptionType">{{ t('protoCenter.tls.generate') }}</el-button>
                </div>
                <div class="hint">VLESS-level encryption (vlessenc). Select type first, then generate keys.</div>
              </el-form-item>
              <el-form-item label="Encryption Key">
                <el-input v-model="rf.settings.encryption" placeholder="none" readonly />
                <div class="hint">Client-side encryption key (auto-generated)</div>
              </el-form-item>
              <el-form-item label="Decryption Key">
                <el-input v-model="rf.settings.decryption" placeholder="none" readonly />
                <div class="hint">Server-side decryption key (auto-generated)</div>
              </el-form-item>
              <el-divider content-position="left">
                Users
                <el-button size="small" type="primary" @click="addUser('vless')" style="margin-left:12px">+ Add User</el-button>
              </el-divider>
              <div v-for="(u, ui) in rf.settings.clients" :key="ui" class="user-item">
                <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:8px">
                  <span style="font-weight:600">User #{{ ui + 1 }}</span>
                  <el-button size="small" type="danger" text @click="rf.settings.clients.splice(ui,1)">X</el-button>
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
                  <el-form-item v-if="canEnableFlow" label="Flow">
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
              <!-- Vision Seed (settings level, only when any user has flow) -->
              <el-form-item v-if="canEnableFlow && hasFlowUser" label="Vision Seed">
                <div style="display:flex;gap:4px;align-items:center">
                  <el-input-number v-for="(_, si) in 4" :key="si" v-model="getTestseed(rf.settings)[si]" :min="1" size="small" style="width:25%" />
                  <el-button size="small" @click="randomSeed(rf.settings)">Rand</el-button>
                  <el-button size="small" @click="resetSeed(rf.settings)">Reset</el-button>
                </div>
                <div class="hint">Applied to settings level for xtls-rprx-vision flow</div>
              </el-form-item>
            </el-form>
          </template>

          <!-- VMess -->
          <template v-if="rf.protocol === 'vmess'">
            <el-form label-width="120px" label-position="left">
              <el-divider content-position="left">
                Users
                <el-button size="small" type="primary" @click="addUser('vmess')" style="margin-left:12px">+ Add User</el-button>
              </el-divider>
              <div v-for="(u, ui) in rf.settings.clients" :key="ui" class="user-item">
                <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:8px">
                  <span style="font-weight:600">User #{{ ui + 1 }}</span>
                  <el-button size="small" type="danger" text @click="rf.settings.clients.splice(ui,1)">X</el-button>
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
              <div v-for="(u, ui) in rf.settings.clients" :key="ui" class="user-item">
                <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:8px">
                  <span style="font-weight:600">User #{{ ui + 1 }}</span>
                  <el-button size="small" type="danger" text @click="rf.settings.clients.splice(ui,1)">X</el-button>
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
                <el-select v-model="rf.settings.method" style="width:100%" @change="onSSMethodChange">
                  <el-option label="2022-blake3-aes-256-gcm" value="2022-blake3-aes-256-gcm" />
                  <el-option label="2022-blake3-aes-128-gcm" value="2022-blake3-aes-128-gcm" />
                  <el-option label="2022-blake3-chacha20-poly1305" value="2022-blake3-chacha20-poly1305" />
                  <el-option label="aes-256-gcm" value="aes-256-gcm" />
                  <el-option label="chacha20-poly1305" value="chacha20-poly1305" />
                  <el-option label="chacha20-ietf-poly1305" value="chacha20-ietf-poly1305" />
                  <el-option label="xchacha20-ietf-poly1305" value="xchacha20-ietf-poly1305" />
                </el-select>
              </el-form-item>
              <el-form-item label="Password">
                <div style="display:flex;gap:8px">
                  <el-input v-model="rf.settings.password" style="flex:1" />
                  <el-button size="small" @click="rf.settings.password = genSSPassword(rf.settings.method)">Generate</el-button>
                </div>
                <div class="hint">{{ isSS2022(rf.settings.method) ? 'Base64 encoded key (auto-generated)' : 'Plain text password' }}</div>
              </el-form-item>
              <el-form-item label="Network">
                <el-select v-model="rf.settings.network" style="width:100%">
                  <el-option label="tcp,udp" value="tcp,udp" />
                  <el-option label="tcp" value="tcp" />
                  <el-option label="udp" value="udp" />
                </el-select>
              </el-form-item>
              <el-form-item label="ivCheck">
                <el-switch v-model="rf.settings.ivCheck" />
                <div class="hint">Enable IV check for legacy methods</div>
              </el-form-item>
            </el-form>

            <!-- Multi-user clients (only for 2022-blake3-aes-*-gcm methods) -->
            <template v-if="isSSMultiUser(rf.settings.method)">
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
                <div class="hint">{{ t('protoCenter.transport.autoDerive') }}</div>
              </el-form-item>
              <el-form-item label="MTU">
                <el-input-number v-model="rf.settings.mtu" :min="0" />
                <div class="hint">{{ t('protoCenter.transport.defaultMtu') }}</div>
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
                  <el-form-item label="Comment"><el-input v-model="p.comment" placeholder="e.g. iPhone, Laptop" /></el-form-item>
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
              <div v-for="(u, ui) in rf.settings.clients" :key="ui" class="user-item">
                <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:8px">
                  <span style="font-weight:600">User #{{ ui + 1 }}</span>
                  <el-button size="small" type="danger" text @click="rf.settings.clients.splice(ui,1)">X</el-button>
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
                <div class="hint">{{ t('protoCenter.transport.tunnelDest') }}</div>
              </el-form-item>
              <el-form-item label="Rewrite Port">
                <el-input-number v-model="rf.settings.rewritePort" :min="0" :max="65535" />
                <div class="hint">{{ t('protoCenter.transport.tunnelPort') }}</div>
              </el-form-item>
              <el-form-item label="Follow Redirect">
                <el-switch v-model="rf.settings.followRedirect" />
                <div class="hint">{{ t('protoCenter.transport.tunnelFollowRedirect') }}</div>
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
                <div class="hint">{{ t('protoCenter.transport.tunnelPortMap') }}</div>
              </el-form-item>
            </el-form>
          </template>
        </el-tab-pane>

        <!-- Stream Tab (not for HTTP/SOCKS/Tunnel/WireGuard/TUN) -->
        <el-tab-pane v-if="!['http','socks','tunnel','dokodemo-door','wireguard','tun'].includes(rf.protocol)" :label="t('protoCenter.inbounds.stream')" name="stream">
          <el-form label-width="120px" label-position="left">
            <el-form-item label="Network">
              <el-select v-model="rf.streamSettings.network" style="width:100%" :disabled="rf.protocol === 'hysteria'" @change="onNetworkChange">
                <el-option label="raw" value="raw" /><el-option label="ws" value="ws" />
                <el-option label="mKCP" value="mkcp" /><el-option label="grpc" value="grpc" />
                <el-option label="httpupgrade" value="httpupgrade" />
                <el-option v-if="rf.protocol === 'vless'" label="xhttp" value="xhttp" />
                <el-option v-if="rf.protocol === 'hysteria'" label="Hysteria" value="hysteria" />
              </el-select>
            </el-form-item>
            <el-form-item label="Security">
              <el-select v-model="rf.streamSettings.security" style="width:100%" @change="onSecurityChange">
                <el-option v-if="rf.protocol !== 'hysteria'" label="none" value="none" />
                <el-option label="tls" value="tls" />
                <el-option v-if="['vless','trojan'].includes(rf.protocol) && ['raw','xhttp','grpc'].includes(rf.streamSettings.network)" label="reality" value="reality" />
              </el-select>
              <div v-if="rf.streamSettings.security === 'reality'" class="hint">{{ t('protoCenter.transport.realityOnly') }}</div>
              <div v-if="rf.protocol === 'hysteria'" class="hint">Hysteria requires TLS</div>
            </el-form-item>
            <template v-if="rf.streamSettings.security === 'tls'">
              <el-form-item :label="t('protoCenter.tls.serverName')"><el-input v-model="rf.streamSettings.tls.serverName" @change="emit" /></el-form-item>
              <el-form-item :label="t('protoCenter.tls.alpn')">
                <el-select v-model="rf.streamSettings.tls._alpn" style="width:100%" multiple clearable @change="emit">
                  <el-option label="h2" value="h2" /><el-option label="http/1.1" value="http/1.1" /><el-option label="h3" value="h3" />
                </el-select>
              </el-form-item>
              <el-form-item :label="t('protoCenter.tls.minVersion')">
                <el-select v-model="rf.streamSettings.tls.minVersion" style="width:100%" clearable @change="emit">
                  <el-option label="TLS 1.0" value="1.0" /><el-option label="TLS 1.1" value="1.1" />
                  <el-option label="TLS 1.2" value="1.2" /><el-option label="TLS 1.3" value="1.3" />
                </el-select>
              </el-form-item>
              <el-form-item :label="t('protoCenter.tls.maxVersion')">
                <el-select v-model="rf.streamSettings.tls.maxVersion" style="width:100%" clearable @change="emit">
                  <el-option label="TLS 1.2" value="1.2" /><el-option label="TLS 1.3" value="1.3" />
                </el-select>
              </el-form-item>
              <el-form-item :label="t('protoCenter.tls.cipherSuites')">
                <el-select v-model="rf.streamSettings.tls._cipherSuites" style="width:100%" multiple clearable @change="emit" :placeholder="t('protoCenter.tls.cipherSuitesHint')">
                  <el-option label="TLS_AES_128_GCM_SHA256" value="TLS_AES_128_GCM_SHA256" />
                  <el-option label="TLS_AES_256_GCM_SHA384" value="TLS_AES_256_GCM_SHA384" />
                  <el-option label="TLS_CHACHA20_POLY1305_SHA256" value="TLS_CHACHA20_POLY1305_SHA256" />
                  <el-option label="TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA" value="TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA" />
                  <el-option label="TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA" value="TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA" />
                  <el-option label="TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256" value="TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256" />
                  <el-option label="TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384" value="TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384" />
                  <el-option label="TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256" value="TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256" />
                  <el-option label="TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384" value="TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384" />
                  <el-option label="TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256" value="TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256" />
                  <el-option label="TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256" value="TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256" />
                </el-select>
              </el-form-item>
              <el-form-item :label="t('protoCenter.tls.curvePreferences')">
                <el-select v-model="rf.streamSettings.tls._curves" style="width:100%" multiple clearable @change="onCurvesChange">
                  <el-option label="CurveP256" value="CurveP256" /><el-option label="CurveP384" value="CurveP384" />
                  <el-option label="CurveP521" value="CurveP521" /><el-option label="X25519" value="X25519" />
                  <el-option label="X25519MLKEM768" value="X25519MLKEM768" />
                </el-select>
              </el-form-item>
              <el-form-item :label="t('protoCenter.tls.rejectUnknownSni')">
                <el-switch v-model="rf.streamSettings.tls.rejectUnknownSni" @change="emit" />
                <div class="hint">{{ t('protoCenter.tls.rejectUnknownSniHint') }}</div>
              </el-form-item>
              <el-form-item :label="t('protoCenter.tls.disableSystemRoot')">
                <el-switch v-model="rf.streamSettings.tls.disableSystemRoot" @change="emit" />
                <div class="hint">{{ t('protoCenter.tls.disableSystemRootHint') }}</div>
              </el-form-item>
              <el-form-item :label="t('protoCenter.tls.enableSessionResumption')">
                <el-switch v-model="rf.streamSettings.tls.enableSessionResumption" @change="emit" />
              </el-form-item>
              <el-form-item :label="t('protoCenter.tls.masterKeyLog')">
                <el-input v-model="rf.streamSettings.tls.masterKeyLog" @change="emit" />
                <div class="hint">{{ t('protoCenter.tls.masterKeyLogHint') }}</div>
              </el-form-item>
              <el-form-item :label="t('protoCenter.tls.fingerprint')">
                <el-select v-model="rf.streamSettings.tls.fingerprint" style="width:100%" clearable @change="emit">
                  <el-option label="chrome" value="chrome" /><el-option label="firefox" value="firefox" />
                  <el-option label="safari" value="safari" /><el-option label="edge" value="edge" />
                  <el-option label="random" value="random" /><el-option label="randomized" value="randomized" />
                  <el-option label="unsafe" value="unsafe" />
                </el-select>
              </el-form-item>
            </template>

            <!-- TLS Certificates -->
            <template v-if="rf.streamSettings.security === 'tls'">
              <el-divider content-position="left">{{ t('protoCenter.tls.certsSection') }}</el-divider>
              <el-form label-width="160px" label-position="left">
                <el-form-item :label="t('protoCenter.tls.certSource')">
                  <el-radio-group v-model="rf.streamSettings.tls._certMode" @change="onCertModeChange(rf)">
                    <el-radio value="file">{{ t('protoCenter.tls.certSourceFile') }}</el-radio>
                    <el-radio value="content">{{ t('protoCenter.tls.certSourceContent') }}</el-radio>
                  </el-radio-group>
                </el-form-item>
                <el-form-item :label="t('protoCenter.tls.selectCert')">
                  <el-select v-model="rf.streamSettings.tls._certId" style="width:100%" clearable @change="onCertSelect(rf)" :placeholder="t('protoCenter.tls.selectCertPlaceholder')">
                    <el-option v-for="c in certList" :key="c.id" :label="c.name + ' (' + c.domain + ')'" :value="c.id" />
                  </el-select>
                </el-form-item>
                <template v-if="rf.streamSettings.tls._certMode === 'file'">
                  <el-form-item :label="t('protoCenter.tls.certificateFile')">
                    <el-input v-model="rf.streamSettings.tls._certificateFile" placeholder="certs/example.com/cert.pem" @change="emit" />
                  </el-form-item>
                  <el-form-item :label="t('protoCenter.tls.keyFile')">
                    <el-input v-model="rf.streamSettings.tls._keyFile" placeholder="certs/example.com/key.pem" @change="emit" />
                  </el-form-item>
                </template>
                <template v-if="rf.streamSettings.tls._certMode === 'content'">
                  <el-form-item :label="t('protoCenter.tls.certificate')">
                    <el-input v-model="rf.streamSettings.tls._certificateContent" type="textarea" :rows="4" placeholder="-----BEGIN CERTIFICATE-----&#10;...&#10;-----END CERTIFICATE-----" @change="emit" />
                  </el-form-item>
                  <el-form-item :label="t('protoCenter.tls.key')">
                    <el-input v-model="rf.streamSettings.tls._keyContent" type="textarea" :rows="4" placeholder="-----BEGIN EC PRIVATE KEY-----&#10;...&#10;-----END EC PRIVATE KEY-----" @change="emit" />
                  </el-form-item>
                </template>
                <el-form-item :label="t('protoCenter.tls.usage')">
                  <el-select v-model="rf.streamSettings.tls._certUsage" style="width:100%" clearable @change="emit">
                    <el-option :label="t('protoCenter.tls.usageEncipherment')" value="encipherment" /><el-option :label="t('protoCenter.tls.usageVerify')" value="verify" /><el-option :label="t('protoCenter.tls.usageIssue')" value="issue" />
                  </el-select>
                </el-form-item>
                <el-form-item v-if="rf.streamSettings.tls._certUsage === 'issue'" :label="t('protoCenter.tls.buildChain')">
                  <el-switch v-model="rf.streamSettings.tls._buildChain" @change="emit" />
                  <div class="hint">{{ t('protoCenter.tls.buildChainHint') }}</div>
                </el-form-item>
                <el-form-item :label="t('protoCenter.tls.ocspStapling')">
                  <el-input-number v-model="rf.streamSettings.tls._ocspStapling" :min="0" @change="emit" />
                  <div class="hint">{{ t('protoCenter.tls.ocspStaplingHint') }}</div>
                </el-form-item>
                <el-form-item :label="t('protoCenter.tls.oneTimeLoading')">
                  <el-switch v-model="rf.streamSettings.tls._oneTimeLoading" @change="emit" />
                  <div class="hint">{{ t('protoCenter.tls.oneTimeLoadingHint') }}</div>
                </el-form-item>
              </el-form>
            </template>

            <!-- ECH -->
            <template v-if="rf.streamSettings.security === 'tls'">
              <el-divider content-position="left">
                {{ t('protoCenter.tls.echSection') }}
                <el-button v-if="rf.streamSettings.tls.echServerKeys || rf.streamSettings.tls.echConfigList" size="small" type="danger" text style="margin-left:12px" @click="rf.streamSettings.tls.echServerKeys='';rf.streamSettings.tls.echConfigList='';emit()">{{ t('protoCenter.tls.clearEch') }}</el-button>
              </el-divider>
              <el-form label-width="160px" label-position="left">
                <el-form-item :label="t('protoCenter.tls.echServerKeys')">
                  <div style="display:flex;gap:8px;flex:1">
                    <el-input v-model="rf.streamSettings.tls.echServerKeys" style="flex:1" @change="emit" />
                    <el-button size="small" @click="generateEchKeys(rf)">{{ t('protoCenter.tls.generate') }}</el-button>
                  </div>
                  <div class="hint">{{ t('protoCenter.tls.echServerKeysHint') }}</div>
                </el-form-item>
                <el-form-item :label="t('protoCenter.tls.echConfigList')">
                  <el-input v-model="rf.streamSettings.tls.echConfigList" type="textarea" :rows="2" @change="emit" />
                  <div class="hint">{{ t('protoCenter.tls.echConfigListHint') }}</div>
                </el-form-item>
              </el-form>
            </template>

            <template v-if="rf.streamSettings.security === 'reality'">
              <el-form label-width="160px" label-position="left">
                <el-form-item label="Target">
                  <div style="display:flex;gap:8px;width:100%">
                    <el-select v-model="rf.streamSettings.reality.target" style="flex:1" filterable allow-create default-first-option @change="onTargetChange">
                      <el-option v-for="t in realityTargets" :key="t.target" :label="t.target + ' (' + t.sni + ')'" :value="t.target" />
                    </el-select>
                    <el-button size="small" @click="showTargetMgr=true">{{ t('protoCenter.reality.manage') }}</el-button>
                  </div>
                  <div v-if="defaultTarget" class="hint">{{ t('protoCenter.reality.defaultPreset') }}：{{ defaultTarget.target }}（{{ t('protoCenter.reality.defaultPresetHint') }}）</div>
                </el-form-item>
                <el-form-item label="Server Names">
                  <el-input v-model="rf.streamSettings.reality.serverNamesStr" type="textarea" :rows="3" :placeholder="t('protoCenter.reality.serverNamesPlaceholder')" @change="emit" />
                  <div class="hint">{{ t('protoCenter.reality.serverNamesHint') }}</div>
                </el-form-item>
                <el-form-item label="Private Key">
                  <div style="display:flex;gap:8px;flex:1">
                    <el-input v-model="rf.streamSettings.reality.privateKey" style="flex:1" @change="emit" />
                    <el-button size="small" @click="genRealityKey(rf)">{{ t('protoCenter.reality.generate') }}</el-button>
                  </div>
                </el-form-item>
                <el-form-item :label="t('protoCenter.reality.password')">
                  <el-input v-model="rf.streamSettings.reality._password" readonly />
                  <div class="hint">{{ t('protoCenter.reality.passwordHint') }}</div>
                </el-form-item>
                <el-form-item label="Short IDs">
                  <el-input v-model="rf.streamSettings.reality.shortIdsStr" type="textarea" :rows="4" :placeholder="t('protoCenter.reality.shortIdsPlaceholder')" @change="emit" />
                  <el-button size="small" style="margin-top:4px" @click="genRandomShortId(rf)">{{ t('protoCenter.reality.randomGenerate') }}</el-button>
                  <div class="hint">{{ t('protoCenter.reality.shortIdsHint') }}</div>
                </el-form-item>
                <el-form-item label="Fingerprint">
                  <el-select v-model="rf.streamSettings.reality._fingerprint" style="width:100%" clearable @change="emit">
                    <el-option label="chrome" value="chrome" /><el-option label="firefox" value="firefox" />
                    <el-option label="safari" value="safari" /><el-option label="ios" value="ios" />
                    <el-option label="android" value="android" /><el-option label="edge" value="edge" />
                    <el-option label="random" value="random" /><el-option label="randomized" value="randomized" />
                  </el-select>
                </el-form-item>
                <el-form-item label="Show">
                  <el-switch v-model="rf.streamSettings.reality.show" @change="emit" />
                </el-form-item>
                <el-form-item label="Xver">
                  <el-select v-model="rf.streamSettings.reality.xver" style="width:100%" filterable allow-create default-first-option @change="emit">
                    <el-option label="0 (disabled)" :value="0" />
                    <el-option label="1" :value="1" />
                    <el-option label="2" :value="2" />
                  </el-select>
                </el-form-item>
                <el-form-item label="Min Client Ver">
                  <el-input v-model="rf.streamSettings.reality.minClientVer" placeholder="x.y.z" @change="emit" />
                </el-form-item>
                <el-form-item label="Max Client Ver">
                  <el-input v-model="rf.streamSettings.reality.maxClientVer" placeholder="x.y.z" @change="emit" />
                </el-form-item>
                <el-form-item label="Max Time Diff (ms)">
                  <el-input-number v-model="rf.streamSettings.reality.maxTimeDiff" :min="0" @change="emit" />
                </el-form-item>
              </el-form>

              <!-- ML-DSA-65 -->
              <el-divider content-position="left">{{ t('protoCenter.reality.mldsa65') }}</el-divider>
              <el-form label-width="160px" label-position="left">
                <el-form-item label="Seed">
                  <div style="display:flex;gap:8px;flex:1">
                    <el-input v-model="rf.streamSettings.reality.mldsa65Seed" style="flex:1" @change="emit" />
                    <el-button size="small" @click="genMldsa65(rf)">{{ t('protoCenter.reality.generate') }}</el-button>
                    <el-button v-if="rf.streamSettings.reality.mldsa65Seed" size="small" type="danger" text @click="rf.streamSettings.reality.mldsa65Seed='';rf.streamSettings.reality._mldsa65Verify='';emit()">{{ t('protoCenter.reality.clear') }}</el-button>
                  </div>
                </el-form-item>
                <el-form-item :label="t('protoCenter.reality.mldsa65Verify')">
                  <el-input v-model="rf.streamSettings.reality._mldsa65Verify" type="textarea" :rows="2" readonly />
                  <div class="hint">{{ t('protoCenter.reality.mldsa65VerifyHint') }}</div>
                </el-form-item>
              </el-form>

              <!-- Limit Fallback -->
              <el-divider content-position="left">{{ t('protoCenter.reality.limitFallback') }}</el-divider>
              <el-form label-width="160px" label-position="left">
                <el-form-item label="Limit Upload">
                  <el-switch v-model="rf.streamSettings.reality._showLimitUpload" @change="emit" />
                </el-form-item>
              </el-form>
              <el-form v-if="rf.streamSettings.reality._showLimitUpload" label-width="180px" label-position="left" style="margin-bottom:12px;padding-left:20px">
                <el-form-item label="After Bytes"><el-input-number v-model="rf.streamSettings.reality.limitFallbackUpload.afterBytes" :min="0" @change="emit" /></el-form-item>
                <el-form-item label="Bytes Per Sec"><el-input-number v-model="rf.streamSettings.reality.limitFallbackUpload.bytesPerSec" :min="0" @change="emit" /></el-form-item>
                <el-form-item label="Burst Bytes/Sec"><el-input-number v-model="rf.streamSettings.reality.limitFallbackUpload.burstBytesPerSec" :min="0" @change="emit" /></el-form-item>
              </el-form>
              <el-form label-width="160px" label-position="left">
                <el-form-item label="Limit Download">
                  <el-switch v-model="rf.streamSettings.reality._showLimitDownload" @change="emit" />
                </el-form-item>
              </el-form>
              <el-form v-if="rf.streamSettings.reality._showLimitDownload" label-width="180px" label-position="left" style="padding-left:20px">
                <el-form-item label="After Bytes"><el-input-number v-model="rf.streamSettings.reality.limitFallbackDownload.afterBytes" :min="0" @change="emit" /></el-form-item>
                <el-form-item label="Bytes Per Sec"><el-input-number v-model="rf.streamSettings.reality.limitFallbackDownload.bytesPerSec" :min="0" @change="emit" /></el-form-item>
                <el-form-item label="Burst Bytes/Sec"><el-input-number v-model="rf.streamSettings.reality.limitFallbackDownload.burstBytesPerSec" :min="0" @change="emit" /></el-form-item>
              </el-form>
            </template>
          </el-form>

          <!-- Port Hopping (Hysteria only) -->
          <template v-if="rf.protocol === 'hysteria'">
          <el-divider content-position="left">Port Hopping</el-divider>
          <el-form label-width="140px" label-position="left">
            <el-form-item label="Enable">
              <el-switch v-model="rf._portHopEnabled" @change="onPortHopChange" />
            </el-form-item>
            <template v-if="rf._portHopEnabled">
              <el-form-item label="Target Port">
                <el-input-number v-model="rf.port" :min="1" :max="65535" disabled />
                <div class="hint">Current server port</div>
              </el-form-item>
              <el-form-item label="Range Mode">
                <el-select v-model="rf._portHopMode" style="width:100%" @change="onPortHopModeChange">
                  <el-option label="Preset" value="preset" />
                  <el-option label="Manual" value="manual" />
                </el-select>
              </el-form-item>
              <el-form-item v-if="rf._portHopMode === 'preset'" label="Port Range">
                <el-select v-model="rf._portHopRange" style="width:100%" @change="onPortHopRangeChange">
                  <el-option label="10000-20000" value="10000-20000" />
                  <el-option label="20000-50000" value="20000-50000" />
                  <el-option label="30000-60000" value="30000-60000" />
                  <el-option label="40000-50000" value="40000-50000" />
                  <el-option label="40000-65000" value="40000-65000" />
                </el-select>
              </el-form-item>
              <el-form-item v-if="rf._portHopMode === 'manual'" label="Port Range">
                <div style="display:flex;gap:8px;align-items:center">
                  <el-input-number v-model="rf._portHopStart" :min="1" :max="65535" style="width:120px" @change="onPortHopManualChange" />
                  <span>-</span>
                  <el-input-number v-model="rf._portHopEnd" :min="1" :max="65535" style="width:120px" @change="onPortHopManualChange" />
                </div>
              </el-form-item>
              <el-form-item label="Interface">
                <div style="display:flex;gap:8px;width:100%">
                  <el-select v-model="rf._portHopInterface" style="flex:1" clearable filterable allow-create placeholder="All interfaces">
                    <el-option v-for="iface in ifaceList" :key="iface" :label="iface" :value="iface" />
                  </el-select>
                  <el-button size="small" @click="loadInterfaces">Refresh</el-button>
                </div>
                <div class="hint">Leave empty for all interfaces. Firewall rules will be applied automatically on save.</div>
              </el-form-item>
            </template>
          </el-form>
          </template>

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
                <div class="hint">{{ t('protoCenter.transport.wsEarlyData') }}</div>
              </el-form-item>
              <el-form-item label="Host">
                <el-input v-model="rf.streamSettings.ws.host" />
              </el-form-item>
            </el-form>
            <el-divider content-position="left">Client Settings</el-divider>
            <el-form label-width="140px" label-position="left">
              <el-form-item label="Headers">
                <div v-for="(h, i) in rf.streamSettings.ws.settings._headers" :key="i" style="display:flex;gap:8px;margin-bottom:4px">
                  <el-input v-model="h.key" placeholder="Header name" style="width:30%" />
                  <el-input v-model="h.value" placeholder="Header value" style="width:50%" />
                  <el-button size="small" type="danger" text @click="rf.streamSettings.ws.settings._headers.splice(i,1)">X</el-button>
                </div>
                <el-button size="small" @click="rf.streamSettings.ws.settings._headers.push({key:'',value:''})">+ Add Header</el-button>
              </el-form-item>
              <el-form-item label="Heartbeat Period">
                <el-input-number v-model="rf.streamSettings.ws.settings.heartbeatPeriod" :min="0" />
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
                <div class="hint">{{ t('protoCenter.transport.httpupgradeEarlyData') }}</div>
              </el-form-item>
              <el-form-item label="Host">
                <el-input v-model="rf.streamSettings.httpupgrade.host" />
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
                <el-input-number v-model="rf.settings.version" :min="2" :max="2" disabled />
              </el-form-item>
              <el-form-item label="UDP Idle Timeout">
                <el-input-number v-model="rf.streamSettings.hysteria.udpIdleTimeout" :min="0" @change="emit" />
                <div class="hint">{{ t('protoCenter.transport.hysteriaIdleTimeout') }}</div>
              </el-form-item>
            </el-form>
            <el-divider content-position="left">Masquerade</el-divider>
            <el-form label-width="140px" label-position="left">
              <el-form-item label="Type">
                <el-select v-model="rf.streamSettings.hysteria.masquerade.type" style="width:100%" clearable @change="emit">
                  <el-option label="Default 404" value="" />
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
                <el-input v-model="rf.streamSettings.xhttp.host" @change="emit" />
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
                <el-checkbox v-model="rf.streamSettings.xhttp.extra._showMethod" @change="emit">{{ t('protoCenter.transport.xhttpEnable') }}</el-checkbox>
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
                <el-checkbox v-model="rf.streamSettings.xhttp.extra._showMaxHeader" @change="emit">{{ t('protoCenter.transport.xhttpEnable') }}</el-checkbox>
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
              <el-form-item label="x Padding Bytes">
                <el-input v-model="rf.streamSettings.xhttp.extra.xPaddingBytes" placeholder="100-1000" style="width:200px" @change="emit" />
                <div class="hint">{{ t('protoCenter.transport.xhttpPaddingHint') }}</div>
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
                <el-input v-model="rf.streamSettings.xhttp.extra.xmux.maxConcurrency" placeholder="16-32" style="width:200px" :disabled="xmuxConnActive" @change="onXmuxConcurrency" />
                <div class="hint">{{ t('protoCenter.transport.xmuxConcurrencyHint') }}</div>
              </el-form-item>
              <el-form-item label="Max Connections">
                <el-input v-model="rf.streamSettings.xhttp.extra.xmux.maxConnections" placeholder="0" style="width:200px" :disabled="xmuxConcurrencyActive" @change="onXmuxConnections" />
                <div class="hint">{{ t('protoCenter.transport.xmuxConnectionsHint') }}</div>
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
              <el-form-item label="H Keep Alive Period">
                <el-input-number v-model="rf.streamSettings.xhttp.extra.xmux.hKeepAlivePeriod" :min="-1" @change="emit" />
                <div class="hint">{{ t('protoCenter.transport.xmuxKeepAliveHint') }}</div>
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
                  <el-button size="small" @click="rf.streamSettings.grpc.serviceName = genId(12)">{{ t('protoCenter.transport.grpcChange') }}</el-button>
                </div>
              </el-form-item>
              <el-form-item label="Authority">
                <el-input v-model="rf.streamSettings.grpc.authority" placeholder="grpc.example.com" />
                <div class="hint">{{ t('protoCenter.transport.grpcAuthority') }}</div>
              </el-form-item>
            </el-form>
            <el-divider content-position="left">Client Settings</el-divider>
            <el-form label-width="140px" label-position="left">
              <el-form-item label="Multi Mode">
                <el-switch v-model="rf.streamSettings.grpc.settings.multiMode" />
                <div class="hint">{{ t('protoCenter.transport.grpcMultiMode') }}</div>
              </el-form-item>
              <el-form-item label="User Agent">
                <el-select v-model="rf.streamSettings.grpc.settings.user_agent" filterable allow-create style="width:100%" @change="emit">
                  <el-option label="Default (Go)" value="" />
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
                <div class="hint">{{ t('protoCenter.transport.grpcHealthCheck') }}</div>
              </el-form-item>
              <el-form-item label="Health Check Timeout">
                <el-input-number v-model="rf.streamSettings.grpc.settings.health_check_timeout" :min="0" />
                <div class="hint">{{ t('protoCenter.transport.grpcHealthTimeout') }}</div>
              </el-form-item>
              <el-form-item label="Permit Without Stream">
                <el-switch v-model="rf.streamSettings.grpc.settings.permit_without_stream" />
              </el-form-item>
              <el-form-item label="Initial Window Size">
                <el-input-number v-model="rf.streamSettings.grpc.settings.initial_windows_size" :min="0" />
                <div class="hint">{{ t('protoCenter.transport.grpcInitialWindows') }}</div>
              </el-form-item>
            </el-form>
          </template>
        </el-tab-pane>

        <!-- Sniffing Tab (not for TUN) -->
        <el-tab-pane v-if="!['tun'].includes(rf.protocol)" :label="t('protoCenter.inbounds.sniffing')" name="sniffing">
          <el-form label-width="140px" label-position="left">
            <el-form-item label="Enabled">
              <el-switch v-model="rf.sniffing.enabled" />
            </el-form-item>
            <template v-if="rf.sniffing.enabled">
              <el-form-item label="Dest Override">
                <el-checkbox-group v-model="rf.sniffing.destOverride">
                  <el-checkbox value="http" /><el-checkbox value="tls" />
                  <el-checkbox value="quic" /><el-checkbox value="fakedns" />
                </el-checkbox-group>
              </el-form-item>
              <el-form-item label="Metadata Only">
                <el-switch v-model="rf.sniffing.metadataOnly" />
                <div class="hint">{{ t('protoCenter.transport.sniffingMetadata') }}</div>
              </el-form-item>
              <el-form-item label="Route Only">
                <el-switch v-model="rf.sniffing.routeOnly" />
                <div class="hint">{{ t('protoCenter.transport.sniffingRouteOnly') }}</div>
              </el-form-item>
              <el-form-item label="Domains Excluded">
                <el-input v-model="rf.sniffing._domainsExcludedStr" type="textarea" :rows="2" placeholder="courier.push.apple.com&#10;domain:example.com" @change="emit" />
                <div class="hint">{{ t('protoCenter.transport.sniffingDomainsExcluded') }}</div>
              </el-form-item>
              <el-form-item label="IPs Excluded">
                <el-input v-model="rf.sniffing._ipsExcludedStr" type="textarea" :rows="2" placeholder="10.0.0.0/8" @change="emit" />
                <div class="hint">{{ t('protoCenter.transport.sniffingIpsExcluded') }}</div>
              </el-form-item>
            </template>
          </el-form>
        </el-tab-pane>

        <!-- Sockopt Tab (not for http, socks, tun) -->
        <el-tab-pane v-if="!['http','socks','tun'].includes(rf.protocol)" :label="t('protoCenter.sockopt.title')" name="sockopt">
          <div style="margin-bottom:16px;display:flex;align-items:center;gap:12px">
            <span style="font-weight:500">{{ t('protoCenter.sockopt.enable') }}</span>
            <el-switch v-model="rf.sockopt._enabled" />
            <span v-if="!rf.sockopt._enabled" style="font-size:12px;color:#909399">{{ t('protoCenter.sockopt.enableHint') }}</span>
          </div>
          <el-form v-if="rf.sockopt._enabled" label-width="180px" label-position="left">
            <el-form-item label="Mark">
              <el-input-number v-model="rf.sockopt.mark" :min="0" @change="emit" />
            </el-form-item>
            <el-form-item label="TCP Fast Open">
              <el-switch v-model="rf.sockopt.tcpFastOpen" @change="emit" />
            </el-form-item>
            <el-form-item label="TProxy">
              <el-select v-model="rf.sockopt.tproxy" style="width:100%" clearable @change="emit">
                <el-option label="off" value="off" /><el-option label="redirect" value="redirect" /><el-option label="tproxy" value="tproxy" />
              </el-select>
            </el-form-item>
            <el-form-item label="Domain Strategy">
              <el-select v-model="rf.sockopt.domainStrategy" style="width:100%" clearable @change="emit">
                <el-option label="AsIs" value="AsIs" /><el-option label="UseIP" value="UseIP" />
                <el-option label="UseIPv4" value="UseIPv4" /><el-option label="UseIPv6" value="UseIPv6" />
                <el-option label="ForceIP" value="ForceIP" /><el-option label="ForceIPv4" value="ForceIPv4" /><el-option label="ForceIPv6" value="ForceIPv6" />
              </el-select>
            </el-form-item>
            <el-form-item label="Interface">
              <div style="display:flex;gap:8px;width:100%">
                <el-select v-model="rf.sockopt.interface" style="flex:1" filterable allow-create clearable @change="emit" :placeholder="t('protoCenter.sockopt.interfaceHint')">
                  <el-option v-for="iface in ifaceList" :key="iface" :label="iface" :value="iface" />
                </el-select>
                <el-button size="small" @click="loadInterfaces">{{ t('protoCenter.transport.refresh') }}</el-button>
              </div>
            </el-form-item>
            <el-form-item label="V6 Only">
              <el-switch v-model="rf.sockopt.v6only" @change="emit" />
            </el-form-item>
            <el-form-item label="TCP Keep Alive Idle (s)">
              <el-input-number v-model="rf.sockopt.tcpKeepAliveIdle" :min="0" @change="emit" />
            </el-form-item>
            <el-form-item label="TCP Keep Alive Interval (s)">
              <el-input-number v-model="rf.sockopt.tcpKeepAliveInterval" :min="0" @change="emit" />
            </el-form-item>
            <el-form-item label="TCP User Timeout (ms)">
              <el-input-number v-model="rf.sockopt.tcpUserTimeout" :min="0" @change="emit" />
            </el-form-item>
            <el-form-item label="TCP Congestion">
              <el-select v-model="rf.sockopt.tcpcongestion" style="width:100%" clearable @change="emit">
                <el-option label="(default)" value="" /><el-option label="bbr" value="bbr" /><el-option label="cubic" value="cubic" /><el-option label="reno" value="reno" />
              </el-select>
            </el-form-item>
            <el-form-item label="TCP Max Seg">
              <el-input-number v-model="rf.sockopt.tcpMaxSeg" :min="0" @change="emit" />
            </el-form-item>
            <el-form-item label="TCP Window Clamp">
              <el-input-number v-model="rf.sockopt.tcpWindowClamp" :min="0" @change="emit" />
            </el-form-item>
            <el-form-item label="TCP MPTCP">
              <el-switch v-model="rf.sockopt.tcpMptcp" @change="emit" />
            </el-form-item>
            <el-form-item label="Accept Proxy Protocol">
              <el-switch v-model="rf.sockopt.acceptProxyProtocol" @change="emit" />
            </el-form-item>
            <el-form-item label="Dialer Proxy">
              <el-input v-model="rf.sockopt.dialerProxy" @change="emit" />
            </el-form-item>
            <el-form-item label="Address Port Strategy">
              <el-select v-model="rf.sockopt.addressPortStrategy" style="width:100%" clearable @change="emit">
                <el-option label="(none)" value="" /><el-option label="SrvPortOnly" value="SrvPortOnly" />
                <el-option label="SrvAddressOnly" value="SrvAddressOnly" /><el-option label="SrvPortAndAddress" value="SrvPortAndAddress" />
                <el-option label="TxtPortOnly" value="TxtPortOnly" /><el-option label="TxtAddressOnly" value="TxtAddressOnly" />
                <el-option label="TxtPortAndAddress" value="TxtPortAndAddress" />
              </el-select>
            </el-form-item>
            <el-form-item label="Trusted X-Forwarded-For">
              <el-input v-model="rf.sockopt._trustedXffStr" type="textarea" :rows="2" placeholder="IP/CIDR" @change="emit" />
            </el-form-item>
          </el-form>

          <!-- Happy Eyeballs -->
          <template v-if="rf.sockopt._enabled">
          <el-divider content-position="left">
            {{ t('protoCenter.sockopt.happyEyeballs') }}
            <el-switch v-model="rf.sockopt._showHappyEyeballs" style="margin-left:12px" @change="onHappyEyeballsToggle" />
          </el-divider>
          <el-form v-if="rf.sockopt._showHappyEyeballs" label-width="180px" label-position="left">
            <el-form-item :label="t('protoCenter.sockopt.tryDelayMs')">
              <el-input-number v-model="rf.sockopt.happyEyeballs.tryDelayMs" :min="1" @change="emit" />
              <div class="hint">{{ t('protoCenter.sockopt.tryDelayMsHint') }}</div>
            </el-form-item>
            <el-form-item :label="t('protoCenter.sockopt.prioritizeIPv6')">
              <el-switch v-model="rf.sockopt.happyEyeballs.prioritizeIPv6" @change="emit" />
              <div class="hint">{{ t('protoCenter.sockopt.prioritizeIPv6Hint') }}</div>
            </el-form-item>
            <el-form-item :label="t('protoCenter.sockopt.interleave')">
              <el-input-number v-model="rf.sockopt.happyEyeballs.interleave" :min="1" @change="emit" />
              <div class="hint">{{ t('protoCenter.sockopt.interleaveHint') }}</div>
            </el-form-item>
            <el-form-item :label="t('protoCenter.sockopt.maxConcurrentTry')">
              <el-input-number v-model="rf.sockopt.happyEyeballs.maxConcurrentTry" :min="1" @change="emit" />
              <div class="hint">{{ t('protoCenter.sockopt.maxConcurrentTryHint') }}</div>
            </el-form-item>
          </el-form>

          <!-- Custom Sockopt -->
          <el-divider content-position="left">
            {{ t('protoCenter.sockopt.customSockopt') }}
            <el-switch v-model="rf.sockopt._showCustomSockopt" style="margin-left:12px" />
          </el-divider>
          <template v-if="rf.sockopt._showCustomSockopt">
            <div style="font-size:12px;color:#909399;margin-bottom:8px">
              {{ t('protoCenter.sockopt.customSockoptHint') }}
            </div>
            <el-form label-width="180px" label-position="left">
              <div v-for="(cs, i) in rf.sockopt._customSockopt" :key="i" style="border:1px solid #e4e7ed;border-radius:4px;padding:8px;margin-bottom:8px">
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
                  <el-button size="small" type="danger" text @click="rf.sockopt._customSockopt.splice(i,1);emit()">X</el-button>
                </div>
                <div style="display:flex;gap:8px">
                  <el-input v-model="cs.level" :placeholder="t('protoCenter.sockopt.customLevelHint')" style="flex:1" />
                  <el-input v-model="cs.opt" :placeholder="t('protoCenter.sockopt.customOptHint')" style="flex:1" />
                  <el-input v-model="cs.value" :placeholder="cs.type === 'int' ? '12345' : 'bbr'" style="flex:1" />
                </div>
              </div>
              <el-button size="small" @click="rf.sockopt._customSockopt.push({system:'',network:'',type:'str',level:'6',opt:'',value:''})">+ Add</el-button>
            </el-form>
          </template>
          </template>
        </el-tab-pane>

        <!-- FinalMask Tab (not for http, socks, tun, tunnel, dokodemo-door) -->
        <el-tab-pane v-if="!['http','socks','tun','tunnel','dokodemo-door'].includes(rf.protocol)" :label="t('protoCenter.finalmask.title')" name="finalmask">
          <div style="margin-bottom:16px;display:flex;align-items:center;gap:12px">
            <span style="font-weight:500">{{ t('protoCenter.finalmask.enable') }}</span>
            <el-switch v-model="rf.finalmask._enabled" />
            <span v-if="!rf.finalmask._enabled" style="font-size:12px;color:#909399">{{ t('protoCenter.finalmask.enableHint') }}</span>
          </div>
          <template v-if="rf.finalmask._enabled">

            <!-- TCP Mask -->
            <el-divider content-position="left">{{ t('protoCenter.finalmask.tcpMask') }}</el-divider>
            <div class="hint" style="margin-bottom:8px">{{ t('protoCenter.finalmask.tcpMaskHint') }}</div>
            <div v-for="(entry, i) in rf.finalmask.tcp" :key="'tcp-'+i" style="border:1px solid #e4e7ed;border-radius:6px;padding:12px;margin-bottom:12px">
              <div style="display:flex;gap:8px;align-items:center;margin-bottom:8px">
                <el-select v-model="entry.type" style="flex:1" @change="onMaskTypeChange(entry, 'tcp')">
                  <el-option label="(none)" value="" />
                  <el-option label="fragment" value="fragment" />
                  <el-option label="sudoku" value="sudoku" />
                  <el-option label="header-custom" value="header-custom" />
                </el-select>
                <el-button size="small" type="danger" text @click="rf.finalmask.tcp.splice(i,1)">X</el-button>
              </div>
              <!-- fragment -->
              <template v-if="entry.type === 'fragment'">
                <el-form label-width="140px" label-position="left" size="small">
                  <el-form-item :label="t('protoCenter.finalmask.packets')">
                    <el-input v-model="entry.settings.packets" placeholder="tlshello or 1-3" @change="emit" />
                    <div class="hint">{{ t('protoCenter.finalmask.packetsHint') }}</div>
                  </el-form-item>
                  <el-form-item :label="t('protoCenter.finalmask.length')">
                    <el-input v-model="entry.settings.length" placeholder="100-200" @change="emit" />
                    <div class="hint">{{ t('protoCenter.finalmask.lengthHint') }}</div>
                  </el-form-item>
                  <el-form-item :label="t('protoCenter.finalmask.delay')">
                    <el-input v-model="entry.settings.delay" placeholder="10-20" @change="emit" />
                    <div class="hint">{{ t('protoCenter.finalmask.delayHint') }}</div>
                  </el-form-item>
                  <el-form-item :label="t('protoCenter.finalmask.maxSplit')">
                    <el-input v-model="entry.settings.maxSplit" placeholder="3-6" @change="emit" />
                    <div class="hint">{{ t('protoCenter.finalmask.maxSplitHint') }}</div>
                  </el-form-item>
                </el-form>
              </template>
              <!-- sudoku -->
              <template v-if="entry.type === 'sudoku'">
                <el-form label-width="140px" label-position="left" size="small">
                  <el-form-item :label="t('protoCenter.finalmask.password')">
                    <div style="display:flex;gap:8px;width:100%">
                      <el-input v-model="entry.settings.password" style="flex:1" @change="emit" />
                      <el-button size="small" @click="entry.settings.password = genPassword(); emit()">{{ t('protoCenter.reality.randomGenerate') }}</el-button>
                    </div>
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
              <!-- header-custom (TCP) -->
              <template v-if="entry.type === 'header-custom'">
                <div v-for="(group, gKey) in {clients:t('protoCenter.finalmask.clients'), servers:t('protoCenter.finalmask.servers'), errors:t('protoCenter.finalmask.errors')}" :key="gKey">
                  <div style="display:flex;align-items:center;gap:8px;margin:8px 0 4px">
                    <span style="font-weight:500;font-size:13px">{{ group }}</span>
                    <el-button size="small" text @click="(entry.settings[gKey] = entry.settings[gKey] || []).push([newClientServerItem()]);emit()">{{ t('protoCenter.finalmask.addGroup') }}</el-button>
                  </div>
                  <div v-for="(grp, gi) in (entry.settings[gKey] || [])" :key="gi" style="border:1px dashed #dcdfe6;border-radius:4px;padding:8px;margin-bottom:6px;margin-left:12px">
                    <div style="display:flex;align-items:center;gap:8px;margin-bottom:4px">
                      <span style="font-size:12px;color:#909399">{{ t('protoCenter.finalmask.groupNum') }} {{ gi+1 }}</span>
                      <el-button size="small" text type="danger" @click="entry.settings[gKey].splice(gi,1);emit()">X</el-button>
                      <el-button size="small" text @click="grp.push(newClientServerItem());emit()">{{ t('protoCenter.finalmask.addItem') }}</el-button>
                    </div>
                    <div v-for="(item, ii) in grp" :key="ii" style="display:flex;gap:6px;margin-bottom:4px;align-items:center">
                      <el-input-number v-model="item.delay" :min="0" size="small" :placeholder="t('protoCenter.finalmask.delay')" style="width:90px" @change="emit" />
                      <el-input-number v-model="item.rand" :min="0" size="small" :placeholder="t('protoCenter.finalmask.rand')" style="width:90px" @change="onRandChange(item)" />
                      <el-input v-model="item.randRange" size="small" placeholder="0-255" style="width:80px" @change="emit" />
                      <el-select v-model="item.type" size="small" style="width:80px" @change="emit">
                        <el-option label="array" value="array" /><el-option label="str" value="str" /><el-option label="hex" value="hex" /><el-option label="base64" value="base64" />
                      </el-select>
                      <el-input v-model="item._packetStr" size="small" :placeholder="item.rand ? t('protoCenter.finalmask.randSet') : t('protoCenter.finalmask.packet')" :disabled="!!item.rand" style="flex:1" @change="onPacketChange(item)" />
                      <el-button size="small" type="danger" text @click="grp.splice(ii,1);emit()">X</el-button>
                    </div>
                  </div>
                </div>
              </template>
            </div>
            <el-button size="small" @click="rf.finalmask.tcp.push({type:'',settings:{},_json:''})">{{ t('protoCenter.finalmask.addTcpMask') }}</el-button>

            <!-- UDP Mask -->
            <el-divider content-position="left">{{ t('protoCenter.finalmask.udpMask') }}</el-divider>
            <div class="hint" style="margin-bottom:8px">{{ t('protoCenter.finalmask.udpMaskHint') }}</div>
            <div v-for="(entry, i) in rf.finalmask.udp" :key="'udp-'+i" style="border:1px solid #e4e7ed;border-radius:6px;padding:12px;margin-bottom:12px">
              <div style="display:flex;gap:8px;align-items:center;margin-bottom:8px">
                <el-select v-model="entry.type" style="flex:1" @change="onMaskTypeChange(entry, 'udp')">
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
                <el-button size="small" type="danger" text @click="rf.finalmask.udp.splice(i,1)">X</el-button>
              </div>
              <!-- header-custom (UDP) -->
              <template v-if="entry.type === 'header-custom'">
                <div v-for="(group, gKey) in {client:t('protoCenter.finalmask.client'), server:t('protoCenter.finalmask.server')}" :key="gKey">
                  <div style="display:flex;align-items:center;gap:8px;margin:8px 0 4px">
                    <span style="font-weight:500;font-size:13px">{{ group }}</span>
                    <el-button size="small" text @click="(entry.settings[gKey] = entry.settings[gKey] || []).push(newUdpClientServerItem());emit()">{{ t('protoCenter.finalmask.addItem') }}</el-button>
                  </div>
                  <div v-for="(item, ii) in (entry.settings[gKey] || [])" :key="ii" style="display:flex;gap:6px;margin-bottom:4px;margin-left:12px;align-items:center">
                    <el-input-number v-model="item.rand" :min="0" size="small" :placeholder="t('protoCenter.finalmask.rand')" style="width:90px" @change="onRandChange(item)" />
                    <el-input v-model="item.randRange" size="small" placeholder="0-255" style="width:80px" @change="emit" />
                    <el-select v-model="item.type" size="small" style="width:80px" @change="emit">
                      <el-option label="array" value="array" /><el-option label="str" value="str" /><el-option label="hex" value="hex" /><el-option label="base64" value="base64" />
                    </el-select>
                    <el-input v-model="item._packetStr" size="small" :placeholder="item.rand ? t('protoCenter.finalmask.randSet') : t('protoCenter.finalmask.packet')" :disabled="!!item.rand" style="flex:1" @change="onPacketChange(item)" />
                    <el-button size="small" type="danger" text @click="entry.settings[gKey].splice(ii,1);emit()">X</el-button>
                  </div>
                </div>
              </template>
              <!-- mkcp-legacy -->
              <template v-if="entry.type === 'mkcp-legacy'">
                <el-form label-width="140px" label-position="left" size="small">
                  <el-form-item :label="t('protoCenter.finalmask.header')">
                    <el-select v-model="entry.settings.header" style="width:100%" clearable @change="onMkcpHeaderChange(entry)">
                      <el-option :label="t('protoCenter.finalmask.headerDefault')" value="" />
                      <el-option label="dns" value="dns" />
                      <el-option label="dtls" value="dtls" />
                      <el-option label="srtp" value="srtp" />
                      <el-option label="utp" value="utp" />
                      <el-option label="wechat" value="wechat" />
                      <el-option label="wireguard" value="wireguard" />
                    </el-select>
                  </el-form-item>
                  <!-- AES-128-GCM: show key input with generate button -->
                  <el-form-item v-if="!entry.settings.header" :label="t('protoCenter.finalmask.value')">
                    <div style="display:flex;gap:8px;width:100%">
                      <el-input v-model="entry.settings.value" placeholder="AES-128-GCM key (empty = xor)" @change="emit" />
                      <el-button size="small" @click="entry.settings.value = genAES128Key(); emit()">Generate</el-button>
                    </div>
                    <div class="hint">AES-128-GCM key, empty = default xor obfuscation</div>
                  </el-form-item>
                  <!-- dns: show domain input -->
                  <el-form-item v-if="entry.settings.header === 'dns'" :label="t('protoCenter.finalmask.value')">
                    <el-input v-model="entry.settings.value" placeholder="www.baidu.com" @change="emit" />
                    <div class="hint">{{ t('protoCenter.finalmask.valueHint') }}</div>
                  </el-form-item>
                </el-form>
              </template>
              <!-- noise -->
              <template v-if="entry.type === 'noise'">
                <el-form label-width="140px" label-position="left" size="small">
                  <el-form-item :label="t('protoCenter.finalmask.reset')">
                    <el-input v-model="entry.settings.reset" placeholder="30-60" @change="emit" />
                    <div class="hint">{{ t('protoCenter.finalmask.resetHint') }}</div>
                  </el-form-item>
                </el-form>
                <div v-for="(n, ni) in entry.settings._noise" :key="ni" style="border:1px dashed #dcdfe6;border-radius:4px;padding:8px;margin-bottom:6px;margin-left:12px">
                  <div style="display:flex;gap:6px;margin-bottom:4px;align-items:center">
                    <span style="font-size:12px;color:#909399">{{ t('protoCenter.finalmask.noiseNum') }} {{ ni+1 }}</span>
                    <el-button size="small" type="danger" text @click="entry.settings._noise.splice(ni,1);emit()">X</el-button>
                  </div>
                  <div style="display:flex;gap:6px;align-items:center">
                    <el-select v-model="n.type" size="small" style="width:80px" @change="emit">
                      <el-option label="array" value="array" /><el-option label="str" value="str" /><el-option label="hex" value="hex" /><el-option label="base64" value="base64" />
                    </el-select>
                    <el-input v-model="n.rand" size="small" placeholder="rand (1-8192)" style="flex:1" @change="onRandChange(n)" />
                    <el-input v-model="n.randRange" size="small" placeholder="0-255" style="width:80px" @change="emit" />
                    <el-input v-model="n.delay" size="small" placeholder="delay (10-20)" style="width:100px" @change="emit" />
                  </div>
                  <div style="margin-top:4px">
                    <el-input v-model="n._packetStr" size="small" :placeholder="n.rand ? t('protoCenter.finalmask.randSet') : t('protoCenter.finalmask.packetHint')" :disabled="!!n.rand" @change="onPacketChange(n)" />
                  </div>
                </div>
                <el-button size="small" style="margin-left:12px" @click="entry.settings._noise.push({type:'array',rand:'1-8192',randRange:'0-255',delay:'10-20',_packetStr:''})">{{ t('protoCenter.finalmask.addNoise') }}</el-button>
              </template>
              <!-- salamander -->
              <template v-if="entry.type === 'salamander'">
                <el-form label-width="140px" label-position="left" size="small">
                  <el-form-item :label="t('protoCenter.finalmask.password')">
                    <div style="display:flex;gap:8px;width:100%">
                      <el-input v-model="entry.settings.password" style="flex:1" @change="emit" />
                      <el-button size="small" @click="entry.settings.password = genPassword(); emit()">{{ t('protoCenter.reality.randomGenerate') }}</el-button>
                    </div>
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
              <!-- sudoku -->
              <template v-if="entry.type === 'sudoku'">
                <el-form label-width="140px" label-position="left" size="small">
                  <el-form-item :label="t('protoCenter.finalmask.password')">
                    <div style="display:flex;gap:8px;width:100%">
                      <el-input v-model="entry.settings.password" style="flex:1" @change="emit" />
                      <el-button size="small" @click="entry.settings.password = genPassword(); emit()">{{ t('protoCenter.reality.randomGenerate') }}</el-button>
                    </div>
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
            </div>
            <el-button size="small" @click="rf.finalmask.udp.push({type:'',settings:{},_json:''})">{{ t('protoCenter.finalmask.addUdpMask') }}</el-button>

            <!-- QUIC Params -->
            <el-divider content-position="left">{{ t('protoCenter.finalmask.quicParams') }}</el-divider>
            <el-form label-width="180px" label-position="left">
              <el-form-item :label="t('protoCenter.finalmask.congestion')">
                <el-select v-model="rf.finalmask.quicParams.congestion" style="width:100%" clearable @change="onCongestionChange">
                  <el-option label="reno" value="reno" /><el-option label="bbr" value="bbr" />
                  <el-option label="brutal" value="brutal" /><el-option label="force-brutal" value="force-brutal" />
                </el-select>
              </el-form-item>
              <el-form-item v-if="rf.finalmask.quicParams.congestion === 'bbr'" :label="t('protoCenter.finalmask.bbrProfile')">
                <el-select v-model="rf.finalmask.quicParams.bbrProfile" style="width:100%" clearable @change="emit">
                  <el-option label="conservative" value="conservative" /><el-option label="standard" value="standard" /><el-option label="aggressive" value="aggressive" />
                </el-select>
              </el-form-item>
              <el-form-item :label="t('protoCenter.finalmask.debug')">
                <el-switch v-model="rf.finalmask.quicParams.debug" @change="emit" />
              </el-form-item>
              <el-form-item :label="t('protoCenter.finalmask.brutalUp')">
                <el-input v-model="rf.finalmask.quicParams.brutalUp" placeholder="200 mbps" @change="emit" />
              </el-form-item>
              <el-form-item :label="t('protoCenter.finalmask.brutalDown')">
                <el-input v-model="rf.finalmask.quicParams.brutalDown" placeholder="200 mbps" @change="emit" />
              </el-form-item>
              <!-- UDP Hop -->
              <el-form-item :label="t('protoCenter.finalmask.udpHop')">
                <el-switch v-model="rf.finalmask.quicParams._showUdpHop" @change="onQuicUdpHopToggle" />
              </el-form-item>
              <template v-if="rf.finalmask.quicParams._showUdpHop">
                <el-form-item :label="t('protoCenter.finalmask.hopPorts')">
                  <el-input v-model="rf.finalmask.quicParams.udpHop.ports" :placeholder="t('protoCenter.finalmask.hopPortsHint')" @change="emit" />
                </el-form-item>
                <el-form-item :label="t('protoCenter.finalmask.hopInterval')">
                  <el-input v-model="rf.finalmask.quicParams.udpHop.interval" :placeholder="t('protoCenter.finalmask.hopIntervalHint')" @change="emit" />
                </el-form-item>
              </template>
              <el-form-item :label="t('protoCenter.finalmask.maxIdleTimeout')">
                <el-input-number v-model="rf.finalmask.quicParams.maxIdleTimeout" :min="4" :max="120" @change="emit" />
              </el-form-item>
              <el-form-item :label="t('protoCenter.finalmask.keepAlivePeriod')">
                <el-input-number v-model="rf.finalmask.quicParams.keepAlivePeriod" :min="2" :max="60" @change="emit" />
              </el-form-item>
              <el-form-item :label="t('protoCenter.finalmask.disablePathMTU')">
                <el-switch v-model="rf.finalmask.quicParams.disablePathMTUDiscovery" @change="emit" />
              </el-form-item>
              <el-form-item :label="t('protoCenter.finalmask.maxIncomingStreams')">
                <el-input-number v-model="rf.finalmask.quicParams.maxIncomingStreams" :min="8" @change="emit" />
              </el-form-item>
              <el-form-item :label="t('protoCenter.finalmask.initStreamWindow')">
                <el-input-number v-model="rf.finalmask.quicParams.initStreamReceiveWindow" :min="16384" @change="emit" />
                <div class="hint">{{ t('protoCenter.finalmask.initStreamWindowHint') }}</div>
              </el-form-item>
              <el-form-item :label="t('protoCenter.finalmask.maxStreamWindow')">
                <el-input-number v-model="rf.finalmask.quicParams.maxStreamReceiveWindow" :min="16384" @change="emit" />
                <div class="hint">{{ t('protoCenter.finalmask.maxStreamWindowHint') }}</div>
              </el-form-item>
              <el-form-item :label="t('protoCenter.finalmask.initConnWindow')">
                <el-input-number v-model="rf.finalmask.quicParams.initConnectionReceiveWindow" :min="16384" @change="emit" />
                <div class="hint">{{ t('protoCenter.finalmask.initConnWindowHint') }}</div>
              </el-form-item>
              <el-form-item :label="t('protoCenter.finalmask.maxConnWindow')">
                <el-input-number v-model="rf.finalmask.quicParams.maxConnectionReceiveWindow" :min="16384" @change="emit" />
                <div class="hint">{{ t('protoCenter.finalmask.maxConnWindowHint') }}</div>
              </el-form-item>
            </el-form>
          </template>
        </el-tab-pane>

        <!-- Fallback Tab (only for vless, trojan) -->
        <el-tab-pane v-if="['vless','trojan'].includes(rf.protocol)" :label="t('protoCenter.fallback.title')" name="fallback">
          <div style="margin-bottom:16px;display:flex;align-items:center;gap:12px">
            <span style="font-weight:500">{{ t('protoCenter.fallback.enable') }}</span>
            <el-switch v-model="rf._fallbackEnabled" />
            <span v-if="!rf._fallbackEnabled" style="font-size:12px;color:#909399">{{ t('protoCenter.fallback.enableHint') }}</span>
          </div>
          <template v-if="rf._fallbackEnabled">
            <div class="hint" style="margin-bottom:12px">{{ t('protoCenter.fallback.desc') }}</div>
            <div v-for="(fb, i) in rf.fallbacks" :key="i" style="border:1px solid #e4e7ed;border-radius:6px;padding:12px;margin-bottom:12px">
              <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:8px">
                <span style="font-weight:500;font-size:13px">{{ t('protoCenter.fallback.entry') }} {{ i+1 }}</span>
                <el-button size="small" type="danger" text @click="rf.fallbacks.splice(i,1);emit()">X</el-button>
              </div>
              <el-form label-width="140px" label-position="left" size="small">
                <el-form-item :label="t('protoCenter.fallback.name')">
                  <el-input v-model="fb.name" :placeholder="t('protoCenter.fallback.nameHint')" @change="emit" />
                  <div class="hint">{{ t('protoCenter.fallback.nameDesc') }}</div>
                </el-form-item>
                <el-form-item :label="t('protoCenter.fallback.alpn')">
                  <el-select v-model="fb._alpn" style="width:100%" clearable @change="emit">
                    <el-option label="(any)" value="" />
                    <el-option label="h2" value="h2" /><el-option label="http/1.1" value="http/1.1" /><el-option label="h3" value="h3" />
                  </el-select>
                  <div class="hint">{{ t('protoCenter.fallback.alpnDesc') }}</div>
                </el-form-item>
                <el-form-item :label="t('protoCenter.fallback.path')">
                  <el-input v-model="fb.path" placeholder="/path" @change="emit" />
                  <div class="hint">{{ t('protoCenter.fallback.pathDesc') }}</div>
                </el-form-item>
                <el-form-item :label="t('protoCenter.fallback.dest')">
                  <el-input v-model="fb.dest" placeholder="80 / addr:port / ＠vless-grpc" @change="emit" />
                  <div class="hint">{{ t('protoCenter.fallback.destDesc') }}</div>
                </el-form-item>
                <el-form-item :label="t('protoCenter.fallback.xver')">
                  <el-select v-model="fb.xver" style="width:100%" @change="emit">
                    <el-option label="0 (disabled)" :value="0" />
                    <el-option label="1" :value="1" />
                    <el-option label="2" :value="2" />
                  </el-select>
                  <div class="hint">{{ t('protoCenter.fallback.xverDesc') }}</div>
                </el-form-item>
              </el-form>
            </div>
            <el-button size="small" @click="rf.fallbacks.push({name:'',_alpn:'',path:'',dest:80,xver:0})">+ {{ t('protoCenter.fallback.add') }}</el-button>
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
            <el-button type="primary" @click="saveInbound">{{ t('common.save') }}</el-button>
          </div>
        </div>
      </template>
    </el-dialog>

    <!-- Target Management Dialog -->
    <el-dialog v-model="showTargetMgr" :title="t('protoCenter.reality.manageTitle')" width="600px" top="10vh">
      <div style="margin-bottom:8px;font-size:12px;color:#909399">
        {{ t('protoCenter.reality.manageDesc') }}
      </div>
      <div style="margin-bottom:12px;display:flex;gap:8px;flex-wrap:wrap">
        <el-input v-model="newTarget.target" placeholder="host:port" style="flex:2" />
        <el-input v-model="newTarget.sni" placeholder="SNI" style="flex:1" />
        <el-select v-model="newTarget.type" style="width:90px">
          <el-option label="TCP" value="tcp" /><el-option label="Sock" value="sock" />
        </el-select>
        <el-button size="small" type="primary" @click="addTarget">{{ t('protoCenter.reality.addTarget') }}</el-button>
        <el-button size="small" @click="resetTargets">{{ t('protoCenter.reality.resetTargets') }}</el-button>
      </div>
      <div style="max-height:400px;overflow-y:auto">
        <div v-for="(target, i) in realityTargets" :key="i" style="display:flex;gap:8px;align-items:center;margin-bottom:6px;padding:4px 0" :style="target.default ? 'background:#ecf5ff;border-radius:4px;padding:4px 8px' : ''">
          <el-button size="small" :type="target.default ? 'primary' : 'info'" text @click="setDefaultTarget(i)" :title="t('protoCenter.reality.setDefault')">
            {{ target.default ? '★' : '☆' }}
          </el-button>
          <span style="flex:1;font-size:13px">{{ target.target }}</span>
          <span style="flex:1;font-size:13px;color:#909399">{{ target.sni }}</span>
          <el-tag size="small" :type="target.type === 'sock' ? 'warning' : ''">{{ target.type || 'tcp' }}</el-tag>
          <el-button size="small" type="danger" text @click="removeTarget(i)">{{ t('common.delete') }}</el-button>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { reactive, ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { More, MoreFilled, Edit, View, RefreshRight, Delete, Refresh, Plus, User, ArrowDown } from '@element-plus/icons-vue'
import client from '../../../api/client'
import { ensureXrayRunning, notifyXrayRuntimeApplyFailure } from '../../../utils/xrayRuntime'
import { generateQrDataUrl, isQrDataTooLargeError } from '../../../utils/qrcode'
const { t } = useI18n()
const props = defineProps({ data: Array, coreType: String, serverId: Number })
defineEmits(['update'])

// Helper to build node-server-scoped API paths
const nodeBase = computed(() => `/node-servers/${props.serverId}`)
const apiBase = computed(() => `${nodeBase.value}/binaries`)

const form = ref([])
const dlg = ref(false)
const editIdx = ref(-1)
const activeTab = ref('basic')
const certList = ref([])
const realityTargets = ref([])
const defaultTarget = computed(() => realityTargets.value.find(t => t.default))
const showTargetMgr = ref(false)
const newTarget = ref({ target: '', sni: '', type: 'tcp', default: false })
const vlessEncAuths = ref([])
// Details dialog
const detailsDlg = ref(false)
const detailsInbound = ref(null)

const qrDlg = ref(false)
const qrLink = ref('')
const qrCodeUrl = ref('')
const qrUsers = ref([])
const qrSelectedUser = ref('')
const qrCurrentIb = ref(null)
const qrGenerating = ref(false)
let qrGenerationToken = 0
const resetDlg = ref(false)
const resetUsers = ref([])
const resetSelectedUser = ref('')
const currentResetIndex = ref(-1)

// Add User dialog
const addUserDlg = ref(false)
const addUserTag = ref('')
const addUserProtocol = ref('')
const addUserSSMethod = ref('')
const newUser = reactive({
  email: '',
  id: '',
  password: '',
  flow: ''
})
const newUserTestseed = ref([900, 500, 900, 256])

// Manage Users dialog
const manageUsersDlg = ref(false)
const manageUsersList = ref([])
const manageUsersTag = ref('')
const xrayStats = ref({ users: [], inbounds: [] })
const statsLoading = ref(false)

// Access Domain
const cfDomains = ref([])
const cfIPEntries = ref([])
const directDomains = ref([])

// Computed stats (from database only, exclude API tag)
const totalUpload = computed(() => {
  let total = 0
  for (const ib of form.value) {
    if (ib.tag === 'api') continue
    total += getTotalTraffic(ib, 'up')
  }
  return total
})
const totalDownload = computed(() => {
  let total = 0
  for (const ib of form.value) {
    if (ib.tag === 'api') continue
    total += getTotalTraffic(ib, 'down')
  }
  return total
})

// VLESS Flow can only be enabled when network is raw and security is tls/reality
const canEnableFlow = computed(() => {
  const network = rf.streamSettings?.network || 'raw'
  const security = rf.streamSettings?.security || 'none'
  return network === 'raw' && (security === 'tls' || security === 'reality')
})

// Check if any user has flow enabled
const hasFlowUser = computed(() => {
  return (rf.settings?.clients || []).some(u => u.flow)
})

// Check if a specific inbound supports flow
function canEnableFlowForInbound(tag) {
  const ib = form.value.find(i => i.tag === tag)
  if (!ib) return false
  const network = ib.streamSettings?.network || 'raw'
  const security = ib.streamSettings?.security || 'none'
  return network === 'raw' && (security === 'tls' || security === 'reality')
}

function getProtocolTagType(protocol) {
  const types = { vless: '', vmess: 'success', trojan: 'warning', shadowsocks: 'info', socks: 'danger', http: 'danger', wireguard: 'success', hysteria: 'warning' }
  return types[protocol] || ''
}

async function fetchXrayStats() {
  try {
    statsLoading.value = true
    const data = await client.get(`${nodeBase.value}/xray/stats`)
    if (data) {
      xrayStats.value = data
    }
  } catch (e) {
    console.error('Failed to fetch xray stats:', e)
  } finally {
    statsLoading.value = false
  }
}

async function resetAllTraffic() {
  try {
    await ElMessageBox.confirm('Are you sure you want to reset all traffic stats?', 'Confirm', { type: 'warning' })
    // Reset all user stats
    const users = xrayStats.value.users || []
    for (const u of users) {
      try {
        await client.post(`${nodeBase.value}/xray/stats/reset/${u.email}`)
      } catch {}
    }
    // Refresh stats
    await fetchXrayStats()
    ElMessage.success('All traffic stats reset')
  } catch {}
}

function getTotalTraffic(ib, direction) {
  // Only use database traffic (synced periodically from xray)
  const users = ib.settings?.clients || ib.settings?.users || ib.settings?.accounts || []
  return users.reduce((sum, u) => sum + (direction === 'up' ? (u._usedUp || 0) : (u._usedDown || 0)), 0)
}

function getFirstUserExpiry(ib) {
  const users = ib.settings?.clients || ib.settings?.users || []
  if (!users.length) return '-'
  const expiry = users[0]._expiry
  if (!expiry) return t('common.unlimited')
  const date = new Date(expiry)
  if (isNaN(date.getTime())) return expiry
  return date.toLocaleDateString()
}

function isExpired(ib) {
  const users = ib.settings?.clients || ib.settings?.users || []
  if (!users.length) return false
  const expiry = users[0]._expiry
  if (!expiry) return false
  const date = new Date(expiry)
  return !isNaN(date.getTime()) && date < new Date()
}

function getUserCount(ib) {
  return (ib.settings?.clients || ib.settings?.users || []).length
}

function getActiveUserCount(ib) {
  const users = ib.settings?.clients || ib.settings?.users || []
  return users.filter(u => {
    if (!u._expiry) return true
    return new Date(u._expiry) > new Date()
  }).length
}

function getExpiredUserCount(ib) {
  const users = ib.settings?.clients || ib.settings?.users || []
  return users.filter(u => {
    if (!u._expiry) return false
    return new Date(u._expiry) <= new Date()
  }).length
}

function getTrafficColor(ib) {
  const total = getTotalTraffic(ib, 'up') + getTotalTraffic(ib, 'down')
  if (total === 0) return 'info'
  if (total > 1073741824) return 'warning' // > 1GB
  return 'success'
}

function getExpiryColor(ib) {
  const users = ib.settings?.clients || ib.settings?.users || []
  if (!users.length) return 'purple'
  const expiry = users[0]._expiry
  if (!expiry) return 'purple'
  const date = new Date(expiry)
  if (isNaN(date.getTime())) return 'purple'
  if (date < new Date()) return 'danger'
  const daysLeft = (date - new Date()) / (1000 * 60 * 60 * 24)
  if (daysLeft < 7) return 'warning'
  return 'success'
}

function getExpiryDisplay(ib) {
  const users = ib.settings?.clients || ib.settings?.users || []
  if (!users.length) return '∞'
  const expiry = users[0]._expiry
  if (!expiry) return '∞'
  const date = new Date(expiry)
  if (isNaN(date.getTime())) return expiry
  const now = new Date()
  if (date < now) return 'Expired'
  const diff = date - now
  const days = Math.floor(diff / (1000 * 60 * 60 * 24))
  if (days > 365) return Math.floor(days / 365) + 'y'
  if (days > 30) return Math.floor(days / 30) + 'mo'
  return days + 'd'
}

function hasUsers(ib) {
  if (ib.settings?.users?.length > 0) return true
  if (ib.settings?.clients?.length > 0) return true
  // Shadowsocks single-user methods have password in settings
  if (ib.protocol === 'shadowsocks' && ib.settings?.password) return true
  return false
}

// Can share (has at least one user with credentials)
function hasShareableUsers(ib) {
  return hasUsers(ib)
}

// Can add users (protocols that support gRPC add user)
// Supported: VLESS, VMess, Trojan, Shadowsocks (multi-user only), Hysteria
function canAddUsers(ib) {
  if (['vless', 'vmess', 'trojan', 'hysteria'].includes(ib.protocol)) return true
  // Shadowsocks: only multi-user methods (2022-blake3-aes-*) support adding users
  if (ib.protocol === 'shadowsocks' && isSSMultiUser(ib.settings?.method)) return true
  return false
}

// Has user list (clients/users array with items)
function hasUserList(ib) {
  return (ib.settings?.users?.length > 0) || (ib.settings?.clients?.length > 0)
}

function getUsers(ib) {
  if (ib.settings?.clients?.length > 0) return ib.settings.clients
  if (ib.settings?.users?.length > 0) return ib.settings.users
  // Shadowsocks single-user: create virtual user from settings
  if (ib.protocol === 'shadowsocks' && ib.settings?.password) {
    return [{ email: 'default', password: ib.settings.password }]
  }
  return []
}

function handleAction(cmd, row, index) {
  if (cmd === 'edit') {
    showEdit(index)
  } else if (cmd === 'qrcode') {
    showQrCode(row)
  } else if (cmd === 'reset-traffic') {
    showResetTraffic(row, index)
  } else if (cmd === 'add-user') {
    showAddUser(row)
  } else if (cmd === 'manage-users') {
    showManageUsers(row)
  } else if (cmd === 'delete') {
    rmInbound(index)
  }
}

// ==================== Details Dialog ====================
function showDetails(ib) {
  detailsInbound.value = ib
  detailsDlg.value = true
}

function formatJson(obj) {
  try {
    const clean = { ...obj }
    // Remove internal fields
    delete clean._id
    delete clean._enabled
    delete clean._listenType
    delete clean._socketPath
    delete clean._socketPerm
    delete clean._encryptionType
    delete clean._portHopEnabled
    delete clean._portHopMode
    delete clean._portHopRange
    delete clean._portHopStart
    delete clean._portHopEnd
    delete clean._portHopInterface
    return JSON.stringify(clean, null, 2)
  } catch {
    return '{}'
  }
}

function copyToClipboard(text) {
  // Try modern API first
  if (navigator.clipboard && window.isSecureContext) {
    navigator.clipboard.writeText(text).then(() => {
      ElMessage.success('Copied!')
    }).catch(() => {
      fallbackCopy(text)
    })
  } else {
    fallbackCopy(text)
  }
}

function fallbackCopy(text) {
  const textarea = document.createElement('textarea')
  textarea.value = text
  textarea.style.position = 'fixed'
  textarea.style.left = '-9999px'
  document.body.appendChild(textarea)
  textarea.select()
  try {
    document.execCommand('copy')
    ElMessage.success('Copied!')
  } catch (err) {
    ElMessage.error('Copy failed')
  }
  document.body.removeChild(textarea)
}

function copyShareLink(ib, user) {
  const link = generateShareLink(ib, user)
  copyToClipboard(link)
}

function showQrCode(ib) {
  qrCurrentIb.value = ib
  // Filter out paused users
  qrUsers.value = getUsers(ib).filter(u => !u._paused)
  if (qrUsers.value.length === 0) return
  qrSelectedUser.value = qrUsers.value[0].email || qrUsers.value[0].id
  void updateQrCode()
  qrDlg.value = true
}

function onQrUserChange() {
  void updateQrCode()
}

async function updateQrCode() {
  const ib = qrCurrentIb.value
  if (!ib) return
  const user = qrUsers.value.find(u => (u.email || u.id) === qrSelectedUser.value)
  if (!user) return
  qrLink.value = generateShareLink(ib, user)
  qrCodeUrl.value = ''
  const token = ++qrGenerationToken
  qrGenerating.value = true
  try {
    const dataUrl = await generateQrDataUrl(qrLink.value, { width: 250 })
    if (token !== qrGenerationToken) return
    qrCodeUrl.value = dataUrl
  } catch (error) {
    if (token !== qrGenerationToken) return
    const message = isQrDataTooLargeError(error) ? t('protoCenter.inbounds.qrDataTooLarge') : t('protoCenter.inbounds.qrGenerateFailed')
    ElMessage.error(message)
  } finally {
    if (token === qrGenerationToken) {
      qrGenerating.value = false
    }
  }
}

function getRandomItem(arr) {
  if (!arr || arr.length === 0) return null
  return arr[Math.floor(Math.random() * arr.length)]
}

function getRandomDomainPort(domains) {
  if (!domains || domains.length === 0) return null
  const item = getRandomItem(domains)
  if (!item) return null
  const ports = item.ports ? JSON.parse(item.ports) : [item.port || 443]
  return { host: item.value, port: getRandomItem(ports) || 443 }
}

function getRandomIPPort(entries) {
  if (!entries || entries.length === 0) return null
  const item = getRandomItem(entries)
  if (!item) return null
  const ports = item.ports ? JSON.parse(item.ports) : [443]
  return { host: item.ip, port: getRandomItem(ports) || 443 }
}

function getAccessHostPort(ib) {
  // If access domain is enabled, use configured host/port
  if (ib._accessDomainEnabled) {
    let result = null

    switch (ib._accessDomainType) {
      case 'direct':
        // Direct: from direct domains pool
        if (ib._directDomain) {
          // Use selected domain
          const domain = directDomains.value.find(d => d.value === ib._directDomain)
          if (domain) {
            if (ib._directPortMode === 'inbound') {
              // Use inbound port
              result = { host: domain.value, port: ib.port }
            } else {
              // Use configured ports
              const ports = domain.ports ? JSON.parse(domain.ports) : [443]
              result = { host: domain.value, port: getRandomItem(ports) || 443 }
            }
          }
        } else {
          // Random from all direct domains
          const domain = getRandomItem(directDomains.value)
          if (domain) {
            if (ib._directPortMode === 'inbound') {
              result = { host: domain.value, port: ib.port }
            } else {
              const ports = domain.ports ? JSON.parse(domain.ports) : [443]
              result = { host: domain.value, port: getRandomItem(ports) || 443 }
            }
          }
        }
        break
      case 'cf_domain':
        // CF Domain: random domain + its ports
        if (ib._cfDomain) {
          // Use selected domain
          const domain = cfDomains.value.find(d => d.value === ib._cfDomain)
          if (domain) {
            const ports = domain.ports ? JSON.parse(domain.ports) : [443]
            result = { host: domain.value, port: getRandomItem(ports) || 443 }
          }
        } else {
          // Random from all CF domains
          result = getRandomDomainPort(cfDomains.value)
        }
        break
      case 'cf_ip':
        // CF IP: random IP + its ports
        if (ib._cfIP) {
          // Use selected IP
          const entry = cfIPEntries.value.find(e => e.ip === ib._cfIP)
          if (entry) {
            const ports = entry.ports ? JSON.parse(entry.ports) : [443]
            result = { host: entry.ip, port: getRandomItem(ports) || 443 }
          }
        } else {
          // Random from all CF IPs
          result = getRandomIPPort(cfIPEntries.value)
        }
        break
      case 'custom':
        // Custom: use custom domain, port depends on mode
        if (ib._customDomain) {
          const port = ib._customPortMode === 'inbound' ? ib.port : (ib._accessCustomPort || 443)
          result = { host: ib._customDomain, port }
        }
        break
    }

    if (result && result.host) {
      return result
    }
  }

  // Default: use current host + inbound port
  // If listen is 0.0.0.0, ::, socket, or empty, use current hostname
  let host = ib.listen || ''
  if (!host || host === '0.0.0.0' || host === '::' || host === '[::]' || host.startsWith('/') || host.startsWith('./')) {
    host = window.location.hostname
  }
  return {
    host: host,
    port: ib.port || 443
  }
}

function serializeFinalMask(finalmask) {
  if (!finalmask || !finalmask._enabled) return ''
  const data = {}
  // TCP mask
  if (finalmask.tcp && finalmask.tcp.length) {
    const tcp = finalmask.tcp.filter(e => e.type).map(e => {
      const settings = { ...e.settings }
      // Clean internal fields
      Object.keys(settings).forEach(k => {
        if (k.startsWith('_')) delete settings[k]
      })
      return { type: e.type, settings }
    })
    if (tcp.length) data.tcp = tcp
  }
  // UDP mask
  if (finalmask.udp && finalmask.udp.length) {
    const udp = finalmask.udp.filter(e => e.type).map(e => {
      const settings = { ...e.settings }
      Object.keys(settings).forEach(k => {
        if (k.startsWith('_')) delete settings[k]
      })
      // Ensure mkcp-legacy always has header field
      if (e.type === 'mkcp-legacy' && !('header' in settings)) {
        settings.header = ''
      }
      return { type: e.type, settings }
    })
    if (udp.length) data.udp = udp
  }
  // QUIC params
  if (finalmask.quicParams && finalmask.quicParams.congestion) {
    const qp = { ...finalmask.quicParams }
    Object.keys(qp).forEach(k => {
      if (k.startsWith('_')) delete qp[k]
    })
    data.quicParams = qp
  }
  return Object.keys(data).length > 0 ? JSON.stringify(data) : ''
}

function generateShareLink(ib, user) {
  const protocol = ib.protocol || 'vless'
  const { host, port } = getAccessHostPort(ib)
  const remark = ib._remark || ib.tag || protocol + '-' + port

  if (protocol === 'vmess') {
    return generateVmessLink(ib, user, host, port, remark)
  } else if (protocol === 'vless') {
    return generateVlessLink(ib, user, host, port, remark)
  } else if (protocol === 'trojan') {
    return generateTrojanLink(ib, user, host, port, remark)
  } else if (protocol === 'shadowsocks') {
    return generateShadowsocksLink(ib, user, host, port, remark)
  } else if (protocol === 'hysteria') {
    return generateHysteriaLink(ib, user, host, port, remark)
  }
  return ''
}

function generateVmessLink(ib, user, host, port, remark) {
  const network = ib.streamSettings?.network || 'raw'
  const security = ib.streamSettings?.security || 'none'

  const obj = {
    v: '2',
    ps: remark,
    add: host,
    port: port,
    id: user.id || '',
    aid: user.alterId || 0,
    scy: user.security || 'auto',
    net: network === 'mkcp' ? 'kcp' : network,
    type: 'none',
    host: '',
    path: '',
    tls: security === 'tls' ? 'tls' : '',
    sni: '',
    alpn: '',
    fp: '',
  }

  // Network-specific params
  if (network === 'tcp' || network === 'raw') {
    const tcp = ib.streamSettings?.raw || {}
    if (tcp.headerType === 'http') {
      obj.type = 'http'
      obj.host = tcp.request?._headers?.find(h => h.key.toLowerCase() === 'host')?.value || ''
      obj.path = tcp.request?._pathsStr || '/'
    }
  } else if (network === 'kcp' || network === 'mkcp') {
    const kcp = ib.streamSettings?.kcp || {}
    obj.mtu = kcp.mtu || 1350
    obj.tti = kcp.tti || 50
    // Read from FinalMask mkcp-legacy if enabled
    const finalmask = ib.finalmask || {}
    if (finalmask._enabled && finalmask.udp?.length) {
      const mkcpLegacy = finalmask.udp.find(m => m.type === 'mkcp-legacy')
      if (mkcpLegacy?.settings) {
        if (mkcpLegacy.settings.header) {
          obj.headerType = mkcpLegacy.settings.header
        }
        if (mkcpLegacy.settings.value) {
          obj.seed = mkcpLegacy.settings.value
        }
      }
    }
  } else if (network === 'ws') {
    const ws = ib.streamSettings?.ws || {}
    obj.path = ws.path || '/'
    // Host: use ws.host or first Host header
    let wsHost = ws.host || ''
    if (!wsHost && ws.settings?._headers?.length) {
      const hostHeader = ws.settings._headers.find(h => h.key?.toLowerCase() === 'host')
      if (hostHeader?.value) wsHost = hostHeader.value
    }
    obj.host = wsHost
  } else if (network === 'grpc') {
    const grpc = ib.streamSettings?.grpc || {}
    obj.path = grpc.serviceName || ''
    obj.authority = grpc.authority || ''
    if (grpc.settings?.multiMode) obj.type = 'multi'
  } else if (network === 'httpupgrade') {
    const hu = ib.streamSettings?.httpupgrade || {}
    obj.path = hu.path || '/'
    obj.host = hu.host || ''
  } else if (network === 'xhttp') {
    const xhttp = ib.streamSettings?.xhttp || {}
    obj.path = xhttp.path || '/'
    obj.host = xhttp.host || ''
    obj.type = xhttp.mode || 'auto'
    if (xhttp.extra?.xPaddingBytes) obj.x_padding_bytes = xhttp.extra.xPaddingBytes
    // xhttp extra
    const extra = buildXhttpExtra(xhttp)
    if (extra) obj.extra = extra
  }

  // TLS params
  if (security === 'tls') {
    const tls = ib.streamSettings?.tls || {}
    obj.tls = 'tls'
    obj.sni = tls.serverName || ''
    obj.alpn = (tls._alpn || []).join(',')
    obj.fp = tls.fingerprint || ''
    if (tls.echConfigList) obj.ech = tls.echConfigList
    if (tls._verifyPeerCertByName) obj.vcn = tls._verifyPeerCertByName
    if (tls._pinnedPeerCertSha256) obj.pcs = tls._pinnedPeerCertSha256
  }

  // FinalMask
  const finalmask = ib.finalmask || {}
  if (finalmask._enabled) {
    const fmData = serializeFinalMask(finalmask)
    if (fmData) obj.fm = fmData
  }

  return 'vmess://' + btoa(JSON.stringify(obj))
}

function generateVlessLink(ib, user, host, port, remark) {
  const uuid = user.id || ''
  const flow = user.flow || ''
  const encryption = ib.settings?.encryption || 'none'
  const network = ib.streamSettings?.network || 'raw'
  const security = ib.streamSettings?.security || 'none'

  const params = new URLSearchParams()
  params.set('type', network === 'mkcp' ? 'kcp' : network)
  params.set('encryption', encryption)

  // Network-specific params
  if (network === 'tcp' || network === 'raw') {
    const tcp = ib.streamSettings?.raw || {}
    if (tcp.headerType === 'http') {
      params.set('headerType', 'http')
      params.set('path', tcp.request?._pathsStr || '/')
      const hostHeader = tcp.request?._headers?.find(h => h.key.toLowerCase() === 'host')?.value
      if (hostHeader) params.set('host', hostHeader)
    }
  } else if (network === 'kcp' || network === 'mkcp') {
    const kcp = ib.streamSettings?.kcp || {}
    params.set('mtu', String(kcp.mtu || 1350))
    params.set('tti', String(kcp.tti || 50))
    // Read from FinalMask mkcp-legacy if enabled
    const fm = ib.finalmask || {}
    if (fm._enabled && fm.udp?.length) {
      const mkcpLegacy = fm.udp.find(m => m.type === 'mkcp-legacy')
      if (mkcpLegacy?.settings) {
        if (mkcpLegacy.settings.header) params.set('headerType', mkcpLegacy.settings.header)
        if (mkcpLegacy.settings.value) params.set('seed', mkcpLegacy.settings.value)
      }
    }
  } else if (network === 'ws') {
    const ws = ib.streamSettings?.ws || {}
    params.set('path', ws.path || '/')
    // Host: use ws.host or first Host header
    let wsHost = ws.host || ''
    if (!wsHost && ws.settings?._headers?.length) {
      const hostHeader = ws.settings._headers.find(h => h.key?.toLowerCase() === 'host')
      if (hostHeader?.value) wsHost = hostHeader.value
    }
    params.set('host', wsHost)
  } else if (network === 'grpc') {
    const grpc = ib.streamSettings?.grpc || {}
    params.set('serviceName', grpc.serviceName || '')
    if (grpc.authority) params.set('authority', grpc.authority)
    if (grpc.settings?.multiMode) params.set('mode', 'multi')
  } else if (network === 'httpupgrade') {
    const hu = ib.streamSettings?.httpupgrade || {}
    params.set('path', hu.path || '/')
    params.set('host', hu.host || '')
  } else if (network === 'xhttp') {
    const xhttp = ib.streamSettings?.xhttp || {}
    params.set('path', xhttp.path || '/')
    params.set('host', xhttp.host || '')
    params.set('mode', xhttp.mode || 'auto')
    if (xhttp.extra?.xPaddingBytes) params.set('x_padding_bytes', xhttp.extra.xPaddingBytes)
    // xhttp extra
    const extra = buildXhttpExtra(xhttp)
    if (extra) params.set('extra', JSON.stringify(extra))
  }

  // Security params
  if (security === 'tls') {
    params.set('security', 'tls')
    const tls = ib.streamSettings?.tls || {}
    if (tls.fingerprint) params.set('fp', tls.fingerprint)
    if (tls._alpn?.length) params.set('alpn', tls._alpn.join(','))
    if (tls.serverName) params.set('sni', tls.serverName)
    if (tls.echConfigList) params.set('ech', tls.echConfigList)
    if (tls._verifyPeerCertByName) params.set('vcn', tls._verifyPeerCertByName)
    if (tls._pinnedPeerCertSha256) params.set('pcs', tls._pinnedPeerCertSha256)
  } else if (security === 'reality') {
    params.set('security', 'reality')
    const reality = ib.streamSettings?.reality || {}
    if (reality._password) params.set('pbk', reality._password)
    if (reality._fingerprint) params.set('fp', reality._fingerprint)
    // Random sni from serverNames
    const sniList = (reality.serverNamesStr || '').split(',').map(s => s.trim()).filter(Boolean)
    const sni = sniList.length > 0 ? getRandomItem(sniList) : ''
    if (sni) params.set('sni', sni)
    // Random sid from shortIds
    if (reality.shortIdsStr) {
      const sidList = reality.shortIdsStr.split('\n').map(s => s.trim()).filter(Boolean)
      const sid = sidList.length > 0 ? getRandomItem(sidList) : ''
      if (sid) params.set('sid', sid)
    }
    if (reality._mldsa65Verify) params.set('pqv', reality._mldsa65Verify)
    if (reality.spiderX) params.set('spx', reality.spiderX)
  } else {
    params.set('security', 'none')
  }

  // Flow
  if (flow) params.set('flow', flow)

  // FinalMask
  const finalmask = ib.finalmask || {}
  if (finalmask._enabled) {
    const fmData = serializeFinalMask(finalmask)
    if (fmData) params.set('fm', fmData)
  }

  const url = new URL(`vless://${uuid}@${formatUrlHost(host)}:${port}`)
  for (const [key, value] of params) url.searchParams.set(key, value)
  url.hash = encodeURIComponent(remark)
  return url.toString().replaceAll('+', '%20')
}

function buildXhttpExtra(xhttp) {
  if (!xhttp) return null
  const extra = {}
  if (xhttp.mode) extra.mode = xhttp.mode
  if (xhttp.xPaddingBytes) extra.xPaddingBytes = xhttp.xPaddingBytes
  // xPaddingObfsMode sub-fields only when obfs is enabled
  if (xhttp.extra?.xPaddingObfsMode) {
    extra.xPaddingObfsMode = true
    for (const k of ['xPaddingKey', 'xPaddingHeader', 'xPaddingPlacement', 'xPaddingMethod']) {
      if (xhttp.extra[k]) extra[k] = xhttp.extra[k]
    }
  }
  // String fields - filter out xray core defaults to avoid DPI fingerprint
  const coreDefaults = { scMaxEachPostBytes: '1000000', scMinPostsIntervalMs: '30', uplinkHTTPMethod: '' }
  for (const k of ['uplinkHTTPMethod', 'sessionIDPlacement', 'sessionIDKey', 'seqPlacement', 'seqKey', 'uplinkDataPlacement', 'uplinkDataKey', 'scMaxEachPostBytes', 'scMinPostsIntervalMs']) {
    const v = xhttp.extra?.[k]
    if (typeof v === 'string' && v.length > 0 && v !== coreDefaults[k]) extra[k] = v
  }
  // Headers
  if (xhttp.extra?._headers?.length) {
    const headers = {}
    xhttp.extra._headers.forEach(h => {
      if (h.key && h.value) headers[h.key] = h.value
    })
    if (Object.keys(headers).length) extra.headers = headers
  }
  return Object.keys(extra).length > 0 ? extra : null
}

function generateTrojanLink(ib, user, host, port, remark) {
  const password = user.password || ''
  const network = ib.streamSettings?.network || 'raw'
  const security = ib.streamSettings?.security || 'tls'

  const params = new URLSearchParams()
  params.set('type', network === 'mkcp' ? 'kcp' : network)

  // Network-specific params
  if (network === 'tcp' || network === 'raw') {
    const tcp = ib.streamSettings?.raw || {}
    if (tcp.headerType === 'http') {
      params.set('headerType', 'http')
      params.set('path', tcp.request?._pathsStr || '/')
      const hostHeader = tcp.request?._headers?.find(h => h.key.toLowerCase() === 'host')?.value
      if (hostHeader) params.set('host', hostHeader)
    }
  } else if (network === 'kcp' || network === 'mkcp') {
    const kcp = ib.streamSettings?.kcp || {}
    params.set('mtu', String(kcp.mtu || 1350))
    params.set('tti', String(kcp.tti || 50))
    // Read from FinalMask mkcp-legacy if enabled
    const fm = ib.finalmask || {}
    if (fm._enabled && fm.udp?.length) {
      const mkcpLegacy = fm.udp.find(m => m.type === 'mkcp-legacy')
      if (mkcpLegacy?.settings) {
        if (mkcpLegacy.settings.header) params.set('headerType', mkcpLegacy.settings.header)
        if (mkcpLegacy.settings.value) params.set('seed', mkcpLegacy.settings.value)
      }
    }
  } else if (network === 'ws') {
    const ws = ib.streamSettings?.ws || {}
    params.set('path', ws.path || '/')
    // Host: use ws.host or first Host header
    let wsHost = ws.host || ''
    if (!wsHost && ws.settings?._headers?.length) {
      const hostHeader = ws.settings._headers.find(h => h.key?.toLowerCase() === 'host')
      if (hostHeader?.value) wsHost = hostHeader.value
    }
    params.set('host', wsHost)
  } else if (network === 'grpc') {
    const grpc = ib.streamSettings?.grpc || {}
    params.set('serviceName', grpc.serviceName || '')
    if (grpc.authority) params.set('authority', grpc.authority)
    if (grpc.settings?.multiMode) params.set('mode', 'multi')
  } else if (network === 'httpupgrade') {
    const hu = ib.streamSettings?.httpupgrade || {}
    params.set('path', hu.path || '/')
    params.set('host', hu.host || '')
  } else if (network === 'xhttp') {
    const xhttp = ib.streamSettings?.xhttp || {}
    params.set('path', xhttp.path || '/')
    params.set('host', xhttp.host || '')
    params.set('mode', xhttp.mode || 'auto')
    if (xhttp.extra?.xPaddingBytes) params.set('x_padding_bytes', xhttp.extra.xPaddingBytes)
    const extra = buildXhttpExtra(xhttp)
    if (extra) params.set('extra', JSON.stringify(extra))
  }

  // Security params
  if (security === 'tls') {
    params.set('security', 'tls')
    const tls = ib.streamSettings?.tls || {}
    if (tls.fingerprint) params.set('fp', tls.fingerprint)
    if (tls._alpn?.length) params.set('alpn', tls._alpn.join(','))
    if (tls.serverName) params.set('sni', tls.serverName)
    if (tls.echConfigList) params.set('ech', tls.echConfigList)
    if (tls._verifyPeerCertByName) params.set('vcn', tls._verifyPeerCertByName)
    if (tls._pinnedPeerCertSha256) params.set('pcs', tls._pinnedPeerCertSha256)
  } else if (security === 'reality') {
    params.set('security', 'reality')
    const reality = ib.streamSettings?.reality || {}
    if (reality._password) params.set('pbk', reality._password)
    if (reality._fingerprint) params.set('fp', reality._fingerprint)
    // Random sni from serverNames
    const sniList = (reality.serverNamesStr || '').split(',').map(s => s.trim()).filter(Boolean)
    const sni = sniList.length > 0 ? getRandomItem(sniList) : ''
    if (sni) params.set('sni', sni)
    // Random sid from shortIds
    if (reality.shortIdsStr) {
      const sidList = reality.shortIdsStr.split('\n').map(s => s.trim()).filter(Boolean)
      const sid = sidList.length > 0 ? getRandomItem(sidList) : ''
      if (sid) params.set('sid', sid)
    }
    if (reality._mldsa65Verify) params.set('pqv', reality._mldsa65Verify)
    if (reality.spiderX) params.set('spx', reality.spiderX)
  } else {
    params.set('security', 'none')
  }

  // FinalMask
  const finalmask = ib.finalmask || {}
  if (finalmask._enabled) {
    const fmData = serializeFinalMask(finalmask)
    if (fmData) params.set('fm', fmData)
  }

  const url = new URL(`trojan://${encodeURIComponent(password)}@${formatUrlHost(host)}:${port}`)
  for (const [key, value] of params) url.searchParams.set(key, value)
  url.hash = encodeURIComponent(remark)
  return url.toString().replaceAll('+', '%20')
}

function generateShadowsocksLink(ib, user, host, port, remark) {
  const method = ib.settings?.method || 'aes-256-gcm'
  const network = ib.streamSettings?.network || 'raw'
  const security = ib.streamSettings?.security || 'none'

  // Get password based on method type
  // For multi-user (2022-blake3-aes-*-gcm): use user.password
  // For single-user: use settings.password
  const isMultiUser = method === '2022-blake3-aes-256-gcm' || method === '2022-blake3-aes-128-gcm'
  const password = isMultiUser ? (user.password || '') : (ib.settings?.password || '')

  const params = new URLSearchParams()
  params.set('type', network === 'mkcp' ? 'kcp' : network)

  // Network-specific params
  if (network === 'tcp' || network === 'raw') {
    const tcp = ib.streamSettings?.raw || {}
    if (tcp.headerType === 'http') {
      params.set('headerType', 'http')
      params.set('path', tcp.request?._pathsStr || '/')
      const hostHeader = tcp.request?._headers?.find(h => h.key.toLowerCase() === 'host')?.value
      if (hostHeader) params.set('host', hostHeader)
    }
  } else if (network === 'kcp' || network === 'mkcp') {
    const kcp = ib.streamSettings?.kcp || {}
    params.set('mtu', String(kcp.mtu || 1350))
    params.set('tti', String(kcp.tti || 50))
    // Read from FinalMask mkcp-legacy if enabled
    const fm = ib.finalmask || {}
    if (fm._enabled && fm.udp?.length) {
      const mkcpLegacy = fm.udp.find(m => m.type === 'mkcp-legacy')
      if (mkcpLegacy?.settings) {
        if (mkcpLegacy.settings.header) params.set('headerType', mkcpLegacy.settings.header)
        if (mkcpLegacy.settings.value) params.set('seed', mkcpLegacy.settings.value)
      }
    }
  } else if (network === 'ws') {
    const ws = ib.streamSettings?.ws || {}
    params.set('path', ws.path || '/')
    let wsHost = ws.host || ''
    if (!wsHost && ws.settings?._headers?.length) {
      const hostHeader = ws.settings._headers.find(h => h.key?.toLowerCase() === 'host')
      if (hostHeader?.value) wsHost = hostHeader.value
    }
    if (wsHost) params.set('host', wsHost)
  } else if (network === 'grpc') {
    const grpc = ib.streamSettings?.grpc || {}
    params.set('serviceName', grpc.serviceName || '')
    if (grpc.authority) params.set('authority', grpc.authority)
    if (grpc.settings?.multiMode) params.set('mode', 'multi')
  } else if (network === 'httpupgrade') {
    const hu = ib.streamSettings?.httpupgrade || {}
    params.set('path', hu.path || '/')
    if (hu.host) params.set('host', hu.host)
  } else if (network === 'xhttp') {
    const xhttp = ib.streamSettings?.xhttp || {}
    params.set('path', xhttp.path || '/')
    if (xhttp.host) params.set('host', xhttp.host)
    params.set('mode', xhttp.mode || 'auto')
    if (xhttp.extra?.xPaddingBytes) params.set('x_padding_bytes', xhttp.extra.xPaddingBytes)
    const extra = buildXhttpExtra(xhttp)
    if (extra) params.set('extra', JSON.stringify(extra))
  }

  // Security params
  if (security === 'tls') {
    params.set('security', 'tls')
    const tls = ib.streamSettings?.tls || {}
    if (tls.fingerprint) params.set('fp', tls.fingerprint)
    if (tls._alpn?.length) params.set('alpn', tls._alpn.join(','))
    if (tls.serverName) params.set('sni', tls.serverName)
    if (tls.echConfigList) params.set('ech', tls.echConfigList)
  }

  // FinalMask
  const finalmask = ib.finalmask || {}
  if (finalmask._enabled) {
    const fmData = serializeFinalMask(finalmask)
    if (fmData) params.set('fm', fmData)
  }

  // Build link
  const isSS2022 = method.startsWith('2022')
  if (isSS2022) {
    // SIP022: method:password (not base64)
    const passwords = [ib.settings?.password]
    if (isMultiUser && password) passwords.push(password)
    const userinfo = [method, ...passwords].map(encodeURIComponent).join(':')
    let link = `ss://${userinfo}@${formatUrlHost(host)}:${port}`
    const query = params.toString()
    if (query) link += `?${query}`
    link += `#${encodeURIComponent(remark)}`
    return link
  }

  // Legacy: base64(method:password)
  const userInfo = btoa(method + ':' + password)
  let link = `ss://${userInfo}@${formatUrlHost(host)}:${port}`
  const query = params.toString()
  if (query) link += `?${query}`
  link += `#${encodeURIComponent(remark)}`
  return link
}

function generateHysteriaLink(ib, user, host, port, remark) {
  const version = ib.settings?.version || ib.streamSettings?.hysteria?.version || 2
  const scheme = version === 2 ? 'hysteria2' : 'hysteria'
  // Hysteria2 auth from user.auth (clients[].auth)
  const auth = user.auth || ''
  const params = new URLSearchParams()

  // Security
  const security = ib.streamSettings?.security || 'tls'
  if (security === 'tls') {
    params.set('security', 'tls')
    const tls = ib.streamSettings?.tls || {}
    if (tls.fingerprint) params.set('fp', tls.fingerprint)
    if (tls._alpn?.length) params.set('alpn', tls._alpn.join(','))
    if (tls.serverName) params.set('sni', tls.serverName)
    if (tls.echConfigList) params.set('ech', tls.echConfigList)
    if (tls._verifyPeerCertByName) params.set('vcn', tls._verifyPeerCertByName)
  }

  // Only process finalmask if enabled
  if (ib.finalmask?._enabled) {
    const finalmask = ib.finalmask

    // Salamander obfuscation
    if (finalmask.udp?.length) {
      const salamander = finalmask.udp.find(m => m.type === 'salamander')
      if (salamander?.settings?.password) {
        params.set('obfs', 'salamander')
        params.set('obfs-password', salamander.settings.password)
      }
    }

    // UDP hop ports
    if (finalmask.quicParams?._showUdpHop && finalmask.quicParams?.udpHop?.ports) {
      params.set('mport', finalmask.quicParams.udpHop.ports)
    }

    // FinalMask data
    try {
      const fmData = serializeFinalMask(finalmask)
      if (fmData) params.set('fm', fmData)
    } catch (e) {
      // ignore serialization errors
    }
  }

  const encodedAuth = encodeURIComponent(auth)
  const url = new URL(`${scheme}://${encodedAuth}@${formatUrlHost(host)}:${port}`)
  for (const [key, value] of params) url.searchParams.set(key, value)
  url.hash = encodeURIComponent(remark)
  return url.toString().replaceAll('+', '%20')
}

function formatUrlHost(address) {
  const bare = (address || '').replace(/^\[|\]$/g, '')
  return bare.includes(':') ? `[${bare}]` : bare
}

function copyQrLink() {
  copyToClipboard(qrLink.value)
}

function showResetTraffic(ib, index) {
  resetUsers.value = getUsers(ib).map(u => ({ ...u }))
  resetSelectedUser.value = resetUsers.value[0]?.email || resetUsers.value[0]?.id || ''
  currentResetIndex.value = index
  resetDlg.value = true
}

async function confirmResetTraffic() {
  if (currentResetIndex.value < 0) return
  const ib = form.value[currentResetIndex.value]
  const users = ib.settings?.clients || ib.settings?.users || []
  const user = users.find(u => (u.email || u.id) === resetSelectedUser.value)
  if (user) {
    user._usedUp = 0
    user._usedDown = 0
    // Save to API
    try {
      const configJson = buildInbound(ib)
      delete configJson._remark
      delete configJson._enabled
      delete configJson._id
      await client.put(`${nodeBase.value}/inbound-configs/${ib._id}`, {
        core_type: props.coreType || 'xray',
        remark: ib._remark || '',
        protocol: ib.protocol,
        tag: ib.tag,
        listen: ib.listen || '0.0.0.0',
        port: ib.port || 0,
        enabled: ib._enabled,
        config_json: configJson,
      })
      ElMessage.success('Traffic reset successfully')
    } catch (e) {
      ElMessage.error('Failed to reset traffic')
    }
  }
  resetDlg.value = false
}

// ==================== User Management ====================

function showAddUser(ib) {
  addUserTag.value = ib.tag
  addUserProtocol.value = ib.protocol
  addUserSSMethod.value = ib.settings?.method || ''
  newUser.email = genEmail()
  newUser.id = genUUID()
  // Generate password based on protocol
  if (ib.protocol === 'trojan' || ib.protocol === 'hysteria') {
    newUser.password = genBase64Password()
  } else if (ib.protocol === 'shadowsocks') {
    newUser.password = genSSPassword(ib.settings?.method || 'aes-256-gcm')
  } else {
    newUser.password = genPassword()
  }
  newUser.flow = ''
  newUserTestseed.value = [900, 500, 900, 256]
  addUserDlg.value = true
}

async function confirmAddUser() {
  if (!newUser.email) {
    ElMessage.error('Email is required')
    return
  }

  try {
    // Add user to xray runtime via gRPC
    const apiPayload = {
      email: newUser.email
    }
    // Build payload based on protocol
    if (addUserProtocol.value === 'vless') {
      apiPayload.id = newUser.id
      if (newUser.flow) apiPayload.flow = newUser.flow
    } else if (addUserProtocol.value === 'vmess') {
      apiPayload.id = newUser.id
    } else if (addUserProtocol.value === 'trojan') {
      apiPayload.password = newUser.password
    } else if (addUserProtocol.value === 'shadowsocks') {
      apiPayload.password = newUser.password
      // Send current method from inbound config
      const ib = form.value.find(i => i.tag === addUserTag.value)
      if (ib?.settings?.method) {
        apiPayload.method = ib.settings.method
      }
    }
    await client.post(`${nodeBase.value}/xray/control/inbound/${addUserTag.value}/user`, apiPayload)

    // Also add to DB config
    const ib = form.value.find(i => i.tag === addUserTag.value)
    if (ib) {
      const users = ib.settings?.clients || ib.settings?.users || []
      const userEntry = {
        email: newUser.email,
        level: 0,
        _quota: 'unlimited',
        _expiry: null,
        _usedUp: 0,
        _usedDown: 0
      }
      // Add protocol-specific fields
      if (addUserProtocol.value === 'vless') {
        userEntry.id = newUser.id
        userEntry.flow = newUser.flow || ''
      } else if (addUserProtocol.value === 'vmess') {
        userEntry.id = newUser.id
        userEntry.alterId = 0
        userEntry.security = 'auto'
      } else if (addUserProtocol.value === 'trojan') {
        userEntry.password = newUser.password
      } else if (addUserProtocol.value === 'shadowsocks') {
        userEntry.password = newUser.password
      } else if (addUserProtocol.value === 'hysteria') {
        userEntry.auth = newUser.password
      }
      users.push(userEntry)
      // Add testseed at settings level for VLESS with flow
      if (addUserProtocol.value === 'vless' && newUser.flow) {
        ib.settings.testseed = [...newUserTestseed.value]
      }

      // Save to DB
      const configJson = buildInbound(ib)
      delete configJson._remark
      delete configJson._enabled
      delete configJson._id
      await client.put(`${nodeBase.value}/inbound-configs/${ib._id}`, {
        core_type: props.coreType || 'xray',
        remark: ib._remark || '',
        protocol: ib.protocol,
        tag: ib.tag,
        listen: ib.listen || '0.0.0.0',
        port: ib.port || 0,
        enabled: ib._enabled,
        config_json: configJson,
      })
    }

    ElMessage.success('User added successfully')
    addUserDlg.value = false
  } catch (e) {
    ElMessage.error('Failed to add user: ' + (e.message || e))
  }
}

function showManageUsers(ib) {
  manageUsersTag.value = ib.tag
  manageUsersList.value = ib.settings?.clients || ib.settings?.users || []
  manageUsersDlg.value = true
}

async function pauseUser(user) {
  // Find inbound ID from manageUsersTag
  const inbound = form.value.find(ib => ib.tag === manageUsersTag.value)
  if (!inbound || !inbound._id) {
    ElMessage.error('Inbound not found')
    return
  }
  try {
    // Step 1: Remove from xray runtime first
    try {
      await client.delete(`${nodeBase.value}/xray/control/inbound/${manageUsersTag.value}/user/${encodeURIComponent(user.email)}`)
    } catch (xrayErr) {
      console.warn('Failed to remove from xray runtime:', xrayErr)
      // Continue even if xray removal fails (user might not be in xray)
    }
    // Step 2: Update DB to mark as paused
    await client.post(`${nodeBase.value}/inbound-configs/${inbound._id}/users/${user.email}/pause`)
    user._paused = true
    ElMessage.success(`User ${user.email} paused`)
  } catch (e) {
    ElMessage.error('Failed to pause user: ' + (e.message || e))
  }
}

async function resumeUser(user) {
  // Find inbound ID from manageUsersTag
  const inbound = form.value.find(ib => ib.tag === manageUsersTag.value)
  if (!inbound || !inbound._id) {
    ElMessage.error('Inbound not found')
    return
  }
  try {
    // Step 1: Update DB to mark as active
    await client.post(`${nodeBase.value}/inbound-configs/${inbound._id}/users/${user.email}/resume`)
    // Step 2: Add back to xray runtime
    const apiPayload = { email: user.email }
    if (inbound.protocol === 'vless') {
      apiPayload.id = user.id
      if (user.flow) apiPayload.flow = user.flow
    } else if (inbound.protocol === 'vmess') {
      apiPayload.id = user.id
    } else if (inbound.protocol === 'trojan') {
      apiPayload.password = user.password
    } else if (inbound.protocol === 'shadowsocks') {
      apiPayload.password = user.password
      apiPayload.method = inbound.settings?.method || ''
    } else if (inbound.protocol === 'hysteria') {
      apiPayload.password = user.auth || user.password
    }
    await client.post(`${nodeBase.value}/xray/control/inbound/${manageUsersTag.value}/user`, apiPayload)
    user._paused = false
    ElMessage.success(`User ${user.email} resumed`)
  } catch (e) {
    ElMessage.error('Failed to resume user: ' + (e.message || e))
  }
}

async function removeUserFromXray(user) {
  try {
    // Remove from xray runtime
    await client.delete(`${nodeBase.value}/xray/control/inbound/${manageUsersTag.value}/user/${user.email}`)

    // Remove from DB config
    const ib = form.value.find(i => i.tag === manageUsersTag.value)
    if (ib) {
      const users = ib.settings?.clients || ib.settings?.users || []
      const idx = users.findIndex(u => u.email === user.email)
      if (idx >= 0) {
        users.splice(idx, 1)
        // Save to DB
        const configJson = buildInbound(ib)
        delete configJson._remark
        delete configJson._enabled
        delete configJson._id
        await client.put(`${nodeBase.value}/inbound-configs/${ib._id}`, {
          core_type: 'xray',
          remark: ib._remark || '',
          protocol: ib.protocol,
          tag: ib.tag,
          listen: ib.listen || '0.0.0.0',
          port: ib.port || 0,
          enabled: ib._enabled,
          config_json: configJson,
        })
      }
    }
    // Update manageUsersList
    manageUsersList.value = manageUsersList.value.filter(u => u.email !== user.email)
    ElMessage.success(`User ${user.email} removed`)
  } catch (e) {
    ElMessage.error('Failed to remove user: ' + (e.message || e))
  }
}

async function loadTargets() {
  try {
    const data = await client.get('/system/reality-targets')
    if (Array.isArray(data)) realityTargets.value = data
  } catch {}
}
async function loadCFDomains() {
  try {
    // CF domains are global
    const globalData = await client.get('/share-domains')
    if (Array.isArray(globalData)) {
      cfDomains.value = globalData.filter(d => d.type === 'cf_domain')
    }
    // Direct domains are per-node — only show domains bound to this server
    const nodeData = await client.get(`${nodeBase.value}/share-domains`)
    directDomains.value = Array.isArray(nodeData) ? nodeData.filter(d => d.type === 'direct') : []
  } catch {}
}
async function loadCFIPEntries() {
  try {
    const data = await client.get('/cfip/entries')
    if (Array.isArray(data)) cfIPEntries.value = data
  } catch {}
}
function toggleCfDomainPort(port, checked) {
  if (checked) {
    if (!rf._cfDomainPorts.includes(port)) rf._cfDomainPorts.push(port)
  } else {
    rf._cfDomainPorts = rf._cfDomainPorts.filter(p => p !== port)
  }
}
function toggleCfIPPort(port, checked) {
  if (checked) {
    if (!rf._cfIPPorts.includes(port)) rf._cfIPPorts.push(port)
  } else {
    rf._cfIPPorts = rf._cfIPPorts.filter(p => p !== port)
  }
}
async function saveTargets() {
  try { await client.put('/system/reality-targets', realityTargets.value) } catch {}
}
function addTarget() {
  if (!newTarget.value.target) return
  if (!realityTargets.value.find(t => t.target === newTarget.value.target)) {
    realityTargets.value.push({ ...newTarget.value })
    saveTargets()
  }
  newTarget.value = { target: '', sni: '', type: 'tcp', default: false }
}
function removeTarget(i) { realityTargets.value.splice(i, 1); saveTargets() }
function setDefaultTarget(i) {
  const isDefault = realityTargets.value[i].default
  realityTargets.value.forEach(t => { t.default = false })
  if (!isDefault) realityTargets.value[i].default = true
  saveTargets()
}
async function resetTargets() {
  try {
    const data = await client.post('/system/reality-targets/reset')
    if (Array.isArray(data)) realityTargets.value = data
  } catch {}
}
const rf = reactive(getDefaultInbound())

function genRemark() { return 'Node-' + Math.random().toString(36).substring(2, 8) }
function genPort() { return Math.floor(Math.random() * 50000 + 10000) }
function genId(len) { const chars = 'abcdefghijklmnopqrstuvwxyz0123456789'; let r = ''; for (let i = 0; i < len; i++) r += chars.charAt(Math.floor(Math.random() * chars.length)); return r }
async function genWgKeyPair() {
  try {
    const data = await client.post(`${apiBase.value}/wg-key?core_type=` + (props.coreType || 'xray'))
    rf.settings.secretKey = data.privateKey || ''
    rf._wgPublicKey = data.publicKey || ''
  } catch {
    ElMessage.error(t('protoCenter.reality.generateFailed'))
  }
}
function genUUID() { return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) { var r = Math.random()*16|0, v = c === 'x' ? r : (r&0x3|0x8); return v.toString(16) }) }
function genPassword() { const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789'; let r = ''; for (let i = 0; i < 12; i++) r += chars.charAt(Math.floor(Math.random() * chars.length)); return r }
function genBase64Password() { const bytes = new Uint8Array(32); crypto.getRandomValues(bytes); return btoa(String.fromCharCode(...bytes)) }
function genAES128Key() { const bytes = new Uint8Array(16); crypto.getRandomValues(bytes); return btoa(String.fromCharCode(...bytes)) }
function isSS2022(method) { return method && method.startsWith('2022') }
function isSSMultiUser(method) {
  // Only 2022-blake3-aes-*-gcm support multi-user
  return method === '2022-blake3-aes-256-gcm' || method === '2022-blake3-aes-128-gcm'
}
function genSSPassword(method) {
  if (isSS2022(method)) {
    // 2022-blake3 methods use base64 encoded key
    const length = method === '2022-blake3-aes-128-gcm' ? 16 : 32
    const bytes = new Uint8Array(length)
    crypto.getRandomValues(bytes)
    return btoa(String.fromCharCode(...bytes))
  }
  // Legacy methods use plain text password
  const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789'
  let r = ''
  for (let i = 0; i < 16; i++) r += chars.charAt(Math.floor(Math.random() * chars.length))
  return r
}
function onSSMethodChange(method) {
  rf.settings.password = genSSPassword(method)
  if (isSSMultiUser(method)) {
    // Switching to multi-user: add a client if empty
    if (!rf.settings.clients || rf.settings.clients.length === 0) {
      rf.settings.clients = [{ password: genBase64Password(), email: genEmail(), _quota: 'unlimited', _expiry: null, _usedUp: 0, _usedDown: 0 }]
    }
  } else {
    // Switching to single-user: clear clients
    rf.settings.clients = []
  }
}
function genEmail() { return Math.random().toString(36).substring(2, 8) + '@xray.com' }
function formatTraffic(bytes) { if (!bytes) return '0 B'; const u = ['B', 'KB', 'MB', 'GB', 'TB']; let i = 0, b = bytes; while (b >= 1024 && i < u.length - 1) { b /= 1024; i++ } return b.toFixed(1) + ' ' + u[i] }
function resetTraffic(u) { u._usedUp = 0; u._usedDown = 0 }
function getTestseed(obj) {
  if (!obj.testseed) obj.testseed = [900, 500, 900, 256]
  return obj.testseed
}
function randomSeed(obj) {
  if (!obj.testseed) obj.testseed = [900, 500, 900, 256]
  obj.testseed = obj.testseed.map(() => Math.floor(Math.random() * 1000) + 1)
}
function resetSeed(obj) {
  obj.testseed = [900, 500, 900, 256]
}

function addUser(proto) {
  if (proto === 'vless') rf.settings.clients.push({ id: genUUID(), email: genEmail(), flow: '', level: 0, _quota: 'unlimited', _expiry: null, _usedUp: 0, _usedDown: 0 })
  else if (proto === 'vmess') rf.settings.clients.push({ id: genUUID(), email: genEmail(), alterId: 0, security: 'auto', level: 0, _quota: 'unlimited', _expiry: null, _usedUp: 0, _usedDown: 0 })
  else if (proto === 'trojan') rf.settings.clients.push({ password: genBase64Password(), email: genEmail(), level: 0, _quota: 'unlimited', _expiry: null, _usedUp: 0, _usedDown: 0 })
  else if (proto === 'hysteria') rf.settings.clients.push({ auth: genBase64Password(), email: genEmail(), level: 0, _quota: 'unlimited', _expiry: null, _usedUp: 0, _usedDown: 0 })
  else if (proto === 'shadowsocks') rf.settings.clients.push({ password: genBase64Password(), email: genEmail(), _quota: 'unlimited', _expiry: null, _usedUp: 0, _usedDown: 0 })
}

function getDefaultInbound() {
  const port = genPort()
  return {
    _remark: genRemark(), tag: 'inbound-vless-' + port, protocol: 'vless',
    listen: '0.0.0.0', port: port, _enabled: true,
    _listenType: 'tcp', _socketPath: '', _socketPerm: '0666',
    settings: { clients: [{ id: genUUID(), email: genEmail(), flow: '', level: 0, _quota: 'unlimited', _expiry: null }], _encryptionType: '', encryption: 'none', decryption: 'none' },
    streamSettings: { network: 'raw', security: 'reality', tls: { serverName: '', _alpn: ['h2', 'http/1.1', 'h3'], minVersion: '1.2', maxVersion: '1.3', _cipherSuites: [], _curves: [], rejectUnknownSni: false, disableSystemRoot: false, enableSessionResumption: false, masterKeyLog: '', fingerprint: 'chrome', _certMode: 'file', _certId: '', _certName: '', _certificateFile: '', _keyFile: '', _certificateContent: '', _keyContent: '', _certUsage: 'encipherment', _buildChain: false, _ocspStapling: 3600, _oneTimeLoading: false, echServerKeys: '', echConfigList: '' }, reality: { target: 'www.amazon.com:443', serverNamesStr: '', privateKey: '', _password: '', shortIdsStr: '', _fingerprint: 'chrome', show: false, xver: 0, minClientVer: '', maxClientVer: '', maxTimeDiff: 0, mldsa65Seed: '', _mldsa65Verify: '', _showLimitUpload: false, limitFallbackUpload: { afterBytes: 0, bytesPerSec: 0, burstBytesPerSec: 0 }, _showLimitDownload: false, limitFallbackDownload: { afterBytes: 0, bytesPerSec: 0, burstBytesPerSec: 0 } }, raw: { acceptProxyProtocol: false, headerType: 'none', request: { version: '1.1', method: 'GET', _pathsStr: '/', _headers: [] }, response: { version: '1.1', status: '200', reason: 'OK', _headers: [] } }, ws: { acceptProxyProtocol: false, path: '/?ed=2560', host: '', settings: { _headers: [], heartbeatPeriod: 0 } }, kcp: { mtu: 1350, tti: 50, uplinkCapacity: 5, downlinkCapacity: 20, congestion: false, readBufferSize: 2, writeBufferSize: 2 }, grpc: { serviceName: genId(12), authority: '', settings: { multiMode: false, user_agent: '', idle_timeout: 0, health_check_timeout: 20, permit_without_stream: false, initial_windows_size: 0 } }, httpupgrade: { acceptProxyProtocol: false, path: '/?ed=2560', host: '', settings: { _headers: [] } }, xhttp: { path: '/', host: '', mode: 'auto', extra: { _showMethod: false, uplinkHTTPMethod: '', noGRPCHeader: false, noSSEHeader: false, _showMaxHeader: false, serverMaxHeaderBytes: 0, _headers: [], xPaddingBytes: '100-1000', xPaddingObfsMode: false, scMaxEachPostBytes: 1000000, scMinPostsIntervalMs: 30, scMaxBufferedPosts: 30, scStreamUpServerSecs: '20-80', xmux: { maxConcurrency: '16-32', maxConnections: 0, cMaxReuseTimes: 0, hMaxRequestTimes: '600-900', hMaxReusableSecs: '1800-3000', hKeepAlivePeriod: 0 } } }, hysteria: { version: 2, udpIdleTimeout: 60, masquerade: { type: '', dir: '', url: '', rewriteHost: false, insecure: false, content: '', _headers: [], statusCode: 0 } }, http: { _hostStr: '', path: '/' } },
    sniffing: { enabled: true, destOverride: ['http', 'tls', 'quic'], metadataOnly: false, _domainsExcludedStr: '', _ipsExcludedStr: '', routeOnly: false },
    _fallbackEnabled: false, fallbacks: [],
    sockopt: { _enabled: false, mark: 0, tcpFastOpen: false, tproxy: 'off', domainStrategy: '', interface: '', v6only: false, tcpKeepAliveIdle: 0, tcpKeepAliveInterval: 0, tcpUserTimeout: 10000, tcpcongestion: '', tcpMaxSeg: 1440, tcpWindowClamp: 600, tcpMptcp: false, acceptProxyProtocol: false, dialerProxy: '', addressPortStrategy: '', _trustedXffStr: '', _showHappyEyeballs: false, happyEyeballs: { tryDelayMs: 0, prioritizeIPv6: false, interleave: 1, maxConcurrentTry: 4 }, _showCustomSockopt: false, _customSockopt: [] },
    finalmask: { _enabled: false, tcp: [], udp: [], quicParams: { congestion: '', bbrProfile: 'standard', debug: false, brutalUp: '200 mbps', brutalDown: '200 mbps', _showUdpHop: false, udpHop: { ports: '20000-50000', interval: '5-10' }, maxIdleTimeout: 30, keepAlivePeriod: 10, disablePathMTUDiscovery: false, maxIncomingStreams: 1024, initStreamReceiveWindow: 8388608, maxStreamReceiveWindow: 8388608, initConnectionReceiveWindow: 20971520, maxConnectionReceiveWindow: 20971520 } },
    _wgAddressesStr: '', _tunGatewayStr: '', _tunDnsStr: '', _tunAutoRoutingStr: '',
    // Access Domain
    _accessDomainEnabled: false, _accessDomainType: 'direct',
    _directDomain: '', _directPortMode: 'config',
    _cfDomain: '', _cfIP: '', _customDomain: '',
    _customPortMode: 'custom',
    _accessCustomPort: 443,
    // Port Hop
    _portHopEnabled: false, _portHopMode: 'preset', _portHopRange: '20000-50000',
    _portHopStart: 20000, _portHopEnd: 50000, _portHopInterface: '',
  }
}

function onProtocolChange() {
  const p = rf.protocol
  const port = rf.port && rf.port > 0 ? rf.port : genPort()
  rf.port = port
  rf.tag = 'inbound-' + p + '-' + port
  rf._socketPath = '/dev/shm/' + p + '.sock'
  // Reset transport when switching from hysteria
  if (p !== 'hysteria' && rf.streamSettings.network === 'hysteria') {
    rf.streamSettings.network = 'raw'
  }
  // Reset security to none when switching protocols (except hysteria which needs tls)
  if (p === 'hysteria') {
    rf.streamSettings.security = 'tls'
  } else if (p === 'vless' || p === 'trojan') {
    // Keep current security for vless/trojan
  } else {
    rf.streamSettings.security = 'none'
  }
  // Reset FinalMask when switching from hysteria
  if (p !== 'hysteria' && rf.finalmask._enabled) {
    rf.finalmask._enabled = false
    rf.finalmask.tcp = []
    rf.finalmask.udp = []
    rf.finalmask.quicParams = { congestion: '', bbrProfile: 'standard', debug: false, brutalUp: '200 mbps', brutalDown: '200 mbps', _showUdpHop: false, udpHop: { ports: '20000-50000', interval: '5-10' }, maxIdleTimeout: 30, keepAlivePeriod: 10, disablePathMTUDiscovery: false, maxIncomingStreams: 1024, initStreamReceiveWindow: 8388608, maxStreamReceiveWindow: 8388608, initConnectionReceiveWindow: 20971520, maxConnectionReceiveWindow: 20971520 }
  }
  if (p === 'vless') rf.settings = { clients: [{ id: genUUID(), email: genEmail(), flow: '', level: 0, _quota: 'unlimited', _expiry: null }], _encryptionType: '', encryption: 'none', decryption: 'none' }
  else if (p === 'vmess') rf.settings = { clients: [{ id: genUUID(), email: genEmail(), alterId: 0, security: 'auto', level: 0, _quota: 'unlimited', _expiry: null }] }
  else if (p === 'trojan') rf.settings = { clients: [{ password: genBase64Password(), email: genEmail(), level: 0, _quota: 'unlimited', _expiry: null }] }
  else if (p === 'shadowsocks') rf.settings = { method: 'aes-256-gcm', password: genSSPassword('aes-256-gcm'), network: 'tcp', clients: [{ password: genBase64Password(), email: genEmail(), _quota: 'unlimited', _expiry: null, _usedUp: 0, _usedDown: 0 }] }
  else if (p === 'socks') rf.settings = { auth: 'noauth', accounts: [], udp: false }
  else if (p === 'http') rf.settings = { accounts: [], allowTransparent: false }
  else if (p === 'wireguard') { rf.settings = { secretKey: '', peers: [{ publicKey: '', _allowedIPsStr: '0.0.0.0/0, ::/0', keepAlive: 0, comment: '' }], address: [], mtu: 1420, noKernelTun: true, workers: 0, domainStrategy: '' }; rf._wgAddressesStr = ''; rf._wgPublicKey = '' }
  else if (p === 'hysteria') {
    rf.settings = { version: 2, clients: [{ auth: genBase64Password(), email: genEmail(), level: 0, _quota: 'unlimited', _expiry: null }] }
    rf.streamSettings.network = 'hysteria'
    rf.streamSettings.security = 'tls'
    rf.streamSettings.tls._alpn = ['h3']
    // FinalMask defaults for Hysteria
    rf.finalmask._enabled = true
    rf.finalmask.quicParams.congestion = 'bbr'
    rf.finalmask.quicParams.bbrProfile = 'conservative'
    // UDP Mask salamander
    rf.finalmask.udp.push({
      type: 'salamander',
      settings: { password: genPassword(), _gecko: false, _geckoMin: 1, _geckoMax: 2048 }
    })
    // Port hopping defaults
    rf._portHopEnabled = false
    rf._portHopMode = 'preset'
    rf._portHopRange = '20000-50000'
    rf._portHopStart = 20000
    rf._portHopEnd = 50000
    rf._portHopInterface = ''
  }
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

// ==================== Port Hopping ====================
const presetPortRanges = ['10000-20000', '20000-50000', '30000-60000', '40000-50000', '40000-65000']

function onPortHopChange(enabled) {
  if (enabled) {
    rf._portHopMode = 'preset'
    rf._portHopRange = '20000-50000'
    rf._portHopStart = 20000
    rf._portHopEnd = 50000
    rf._portHopInterface = ''
    // Enable FinalMask UDP Hop
    updateUdpHopFromPortHop()
  } else {
    // Disable FinalMask UDP Hop
    rf.finalmask.quicParams._showUdpHop = false
    rf.finalmask.quicParams.udpHop = { ports: '', interval: '5-10' }
  }
  emit()
}

function onPortHopModeChange(mode) {
  if (mode === 'preset') {
    rf._portHopRange = '20000-50000'
    rf._portHopStart = 20000
    rf._portHopEnd = 50000
  } else {
    rf._portHopStart = 20000
    rf._portHopEnd = 50000
  }
  updateUdpHopFromPortHop()
  emit()
}

function onPortHopRangeChange(range) {
  const [start, end] = range.split('-').map(Number)
  rf._portHopStart = start
  rf._portHopEnd = end
  updateUdpHopFromPortHop()
  emit()
}

function onPortHopManualChange() {
  if (rf._portHopStart && rf._portHopEnd) {
    rf._portHopRange = `${rf._portHopStart}-${rf._portHopEnd}`
    updateUdpHopFromPortHop()
  }
  emit()
}

function updateUdpHopFromPortHop() {
  const portRange = rf._portHopMode === 'preset' ? rf._portHopRange : `${rf._portHopStart}-${rf._portHopEnd}`
  rf.finalmask._enabled = true
  rf.finalmask.quicParams._showUdpHop = true
  rf.finalmask.quicParams.udpHop = {
    ports: portRange,
    interval: '5-10'
  }
}

function onSecurityChange() {
  emit()
}

async function generateVlessEnc() {
  const encType = rf.settings._encryptionType
  if (!encType) {
    ElMessage.warning('Please select encryption type first')
    return
  }

  try {
    const data = await client.post(`${apiBase.value}/vlessenc`)
    if (data && data.auths && data.auths.length > 0) {
      vlessEncAuths.value = data.auths
      // Find matching auth by type
      const auth = data.auths.find(a => a.id === encType)
      if (auth) {
        rf.settings.encryption = auth.encryption || ''
        rf.settings.decryption = auth.decryption || ''
        emit()
        ElMessage.success(t('protoCenter.tls.generateSuccess'))
      } else {
        ElMessage.error('No matching encryption type found')
      }
    }
  } catch (e) {
    ElMessage.error(t('protoCenter.reality.generateFailed'))
  }
}

async function onVlessEncryptionTypeChange(val) {
  if (val) {
    // Auto-generate keys when type is selected
    await generateVlessEnc()
  } else {
    // Clear keys when type is cleared
    rf.settings.encryption = 'none'
    rf.settings.decryption = 'none'
  }
  emit()
}

function onSocksAuthChange(val) {
  if (val === 'password' && (!rf.settings.accounts || !rf.settings.accounts.length)) {
    rf.settings.accounts = [{ user: genEmail().split('@')[0], pass: genPassword(), _quota: 'unlimited', _expiry: null, _usedUp: 0, _usedDown: 0 }]
  }
}

function xmuxFieldActive(val) {
  if (val === null || val === undefined) return false
  const s = String(val).trim()
  return s !== '' && s !== '0'
}
const xmuxConcurrencyActive = computed(() => xmuxFieldActive(rf.streamSettings?.xhttp?.extra?.xmux?.maxConcurrency))
const xmuxConnActive = computed(() => xmuxFieldActive(rf.streamSettings?.xhttp?.extra?.xmux?.maxConnections))

function onXmuxConcurrency() {
  const v = rf.streamSettings.xhttp.extra.xmux.maxConcurrency
  if (xmuxFieldActive(v)) {
    rf.streamSettings.xhttp.extra.xmux.maxConnections = ''
  }
  emit()
}

function onXmuxConnections() {
  const v = rf.streamSettings.xhttp.extra.xmux.maxConnections
  if (xmuxFieldActive(v)) {
    rf.streamSettings.xhttp.extra.xmux.maxConcurrency = ''
  }
  emit()
}

function onCurvesChange() { emit() }

async function genRealityKey(rf) {
  try {
    const data = await client.post(`${apiBase.value}/x25519-key`)
    if (data && data.privateKey) {
      rf.streamSettings.reality.privateKey = data.privateKey
      rf.streamSettings.reality._password = data.password || ''
      // Also generate short IDs
      genRandomShortId(rf)
      emit()
      ElMessage.success(t('protoCenter.tls.generateSuccess'))
    }
  } catch (e) { ElMessage.error(t('protoCenter.tls.generateFailed') + ': ' + (e.message || e)) }
}

function genRandomShortId(rf) {
  const ids = [''] // first line empty
  for (let i = 0; i < 12; i++) {
    const len = Math.random() > 0.5 ? 8 : 16
    ids.push(Array.from(crypto.getRandomValues(new Uint8Array(len / 2))).map(b => b.toString(16).padStart(2, '0')).join(''))
  }
  rf.streamSettings.reality.shortIdsStr = ids.join('\n')
  emit()
}

function onTargetChange() {
  const t = realityTargets.value.find(rt => rt.target === rf.streamSettings.reality.target)
  if (t) rf.streamSettings.reality.serverNamesStr = t.sni
  emit()
}

const ifaceList = ref([])
const dataDir = ref('/opt/corefusion/data') // default fallback
async function loadInterfaces() {
  try {
    const data = await client.get(`${apiBase.value}/interfaces`)
    if (Array.isArray(data)) ifaceList.value = data
  } catch { ifaceList.value = ['eth0', 'wg0', 'lo'] }
}
async function loadDataDir() {
  try {
    const data = await client.get('/system/panel-config')
    if (data && data.data_dir) dataDir.value = data.data_dir
  } catch {}
}

function onHappyEyeballsToggle(val) {
  if (val && rf.sockopt.happyEyeballs.tryDelayMs === 0) {
    rf.sockopt.happyEyeballs.tryDelayMs = 250
  }
}

function onCustomSockoptToggle(val) {
  if (val && rf.sockopt._customSockopt && !rf.sockopt._customSockopt.length) {
    rf.sockopt._customSockopt.push({ system: 'linux', network: 'tcp', type: 'str', level: '6', opt: '13', value: 'bbr' })
  }
}

async function genMldsa65(rf) {
  try {
    const data = await client.post(`${apiBase.value}/mldsa65-key`)
    if (data && data.seed) {
      rf.streamSettings.reality.mldsa65Seed = data.seed
      rf.streamSettings.reality._mldsa65Verify = data.verify || ''
      emit()
      ElMessage.success(t('protoCenter.tls.generateSuccess'))
    }
  } catch (e) { ElMessage.error(t('protoCenter.tls.generateFailed') + ': ' + (e.message || e)) }
}

function onCertModeChange(rf) {
  rf.streamSettings.tls._certId = ''
  rf.streamSettings.tls._certificateFile = ''
  rf.streamSettings.tls._keyFile = ''
  rf.streamSettings.tls._certificateContent = ''
  rf.streamSettings.tls._keyContent = ''
  emit()
}

function onCertSelect(rf) {
  const certId = rf.streamSettings.tls._certId
  if (!certId) { rf.streamSettings.tls._certName = ''; emit(); return }
  const cert = certList.value.find(c => c.id === certId)
  if (!cert) return
  const tls = rf.streamSettings.tls
  tls._certName = cert.name + ' (' + cert.domain + ')'
  if (tls._certMode === 'file') {
    tls._certificateFile = cert.cert_path || cert.fullchain_path || ''
    tls._keyFile = cert.key_path || ''
    tls._certificateContent = ''
    tls._keyContent = ''
  } else {
    tls._certificateContent = cert.cert_content || ''
    tls._keyContent = cert.key_content || ''
    tls._certificateFile = ''
    tls._keyFile = ''
  }
  emit()
}

async function generateEchKeys(rf) {
  try {
    const sni = rf.streamSettings.tls.serverName || 'example.com'
    const data = await client.post(`${apiBase.value}/ech-generate`, { server_name: sni })
    if (data) {
      if (data.echServerKeys) rf.streamSettings.tls.echServerKeys = data.echServerKeys
      if (data.echConfigList) rf.streamSettings.tls.echConfigList = data.echConfigList
      emit()
      ElMessage.success(t('protoCenter.tls.generateSuccess'))
    }
  } catch (e) {
    ElMessage.error(t('protoCenter.tls.generateFailed') + ': ' + (e.message || e))
  }
}

async function loadCertList() {
  try {
    const res = await client.get('/certificates', { params: { limit: 100 } })
    const items = (res && res.items) || (res && res.data && res.data.items) || (Array.isArray(res) ? res : [])
    certList.value = items
    // Match cert ID for existing configs so select shows the name
    form.value.forEach(ib => {
      const tls = ib.streamSettings && ib.streamSettings.tls
      if (!tls || tls._certId) return
      const certFile = tls._certificateFile || ''
      if (!certFile) return
      const matchPath = certFile.replace(/^(\.\.\/)+/, '')
      const found = items.find(c => c.cert_path === matchPath || c.fullchain_path === matchPath)
      if (found) {
        tls._certId = found.id
        tls._certName = found.name + ' (' + found.domain + ')'
      }
    })
  } catch (e) { console.error('loadCertList failed:', e) }
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

function parseTlsSettings(tls) {
  const certs = tls.certificates || []
  const cert = certs[0] || {}
  const settingsArr = tls.settings || []
  const clientSettings = settingsArr[0] || {}
  let certMode = 'content'
  if (cert.certificateFile) certMode = 'file'
  // Strip ../../../ prefix for display
  const stripPrefix = (p) => p ? p.replace(/^(\.\.\/)+/, '') : ''
  return {
    serverName: tls.serverName || '', _alpn: tls.alpn || [],
    minVersion: tls.minVersion || '', maxVersion: tls.maxVersion || '',
    _cipherSuites: tls.cipherSuites ? tls.cipherSuites.split(':') : [], _curves: tls.curvePreferences || [],
    rejectUnknownSni: tls.rejectUnknownSni || false, disableSystemRoot: tls.disableSystemRoot || false,
    enableSessionResumption: tls.enableSessionResumption || false, masterKeyLog: tls.masterKeyLog || '',
    fingerprint: clientSettings.fingerprint || '',
    _certMode: certMode, _certId: '', _certName: '',
    _certificateFile: stripPrefix(cert.certificateFile || ''), _keyFile: stripPrefix(cert.keyFile || ''),
    _certificateContent: (cert.certificate || []).join('\n'), _keyContent: (cert.key || []).join('\n'),
    _certUsage: cert.usage || 'encipherment', _buildChain: cert.buildChain || false,
    _ocspStapling: cert.ocspStapling || 3600, _oneTimeLoading: cert.oneTimeLoading || false,
    echServerKeys: tls.echServerKeys || '', echConfigList: clientSettings.echConfigList || '',
  }
}

function parseRealitySettings(reality) {
  const settingsArr = reality.settings || []
  const cs = settingsArr[0] || {}
  const lfu = reality.limitFallbackUpload || {}
  const lfd = reality.limitFallbackDownload || {}
  return {
    target: reality.target || 'www.amazon.com:443',
    serverNamesStr: (reality.serverNames || []).join('\n'),
    privateKey: reality.privateKey || '',
    _password: cs.password || '',
    _fingerprint: cs.fingerprint || 'chrome',
    shortIdsStr: (reality.shortIds || []).join('\n'),
    show: reality.show || false,
    xver: reality.xver || 0,
    minClientVer: reality.minClientVer || '',
    maxClientVer: reality.maxClientVer || '',
    maxTimeDiff: reality.maxTimeDiff || 0,
    mldsa65Seed: reality.mldsa65Seed || '',
    _mldsa65Verify: cs.mldsa65Verify || '',
    _showLimitUpload: !!reality.limitFallbackUpload,
    limitFallbackUpload: { afterBytes: lfu.afterBytes || 0, bytesPerSec: lfu.bytesPerSec || 0, burstBytesPerSec: lfu.burstBytesPerSec || 0 },
    _showLimitDownload: !!reality.limitFallbackDownload,
    limitFallbackDownload: { afterBytes: lfd.afterBytes || 0, bytesPerSec: lfd.bytesPerSec || 0, burstBytesPerSec: lfd.burstBytesPerSec || 0 },
  }
}

function parseFinalmask(fm) {
  const defQuic = { congestion: '', bbrProfile: 'standard', debug: false, brutalUp: '200 mbps', brutalDown: '200 mbps', _showUdpHop: false, udpHop: { ports: '20000-50000', interval: '5-10' }, maxIdleTimeout: 30, keepAlivePeriod: 10, disablePathMTUDiscovery: false, maxIncomingStreams: 1024, initStreamReceiveWindow: 8388608, maxStreamReceiveWindow: 8388608, initConnectionReceiveWindow: 20971520, maxConnectionReceiveWindow: 20971520 }
  if (!fm || Object.keys(fm).length === 0) return { _enabled: false, tcp: [], udp: [], quicParams: defQuic }
  const qp = fm.quicParams || {}
  // Parse TCP entry
  const parseTcpEntry = e => {
    const s = e.settings || {}
    if (e.type === 'header-custom') {
      return { type: 'header-custom', settings: {
        clients: (s.clients || []).map(g => (g || []).map(i => ({ delay: i.delay || 0, rand: i.rand || 0, randRange: i.randRange || '0-255', type: i.type || 'array', _packetStr: Array.isArray(i.packet) ? i.packet.join(',') : (i.packet || '') }))),
        servers: (s.servers || []).map(g => (g || []).map(i => ({ delay: i.delay || 0, rand: i.rand || 0, randRange: i.randRange || '0-255', type: i.type || 'array', _packetStr: Array.isArray(i.packet) ? i.packet.join(',') : (i.packet || '') }))),
        errors: (s.errors || []).map(g => (g || []).map(i => ({ delay: i.delay || 0, rand: i.rand || 0, randRange: i.randRange || '0-255', type: i.type || 'array', _packetStr: Array.isArray(i.packet) ? i.packet.join(',') : (i.packet || '') }))),
      }, _json: '' }
    }
    return { type: e.type || '', settings: s, _json: '' }
  }
  // Parse UDP entry
  const parseUdpEntry = e => {
    const s = e.settings || {}
    if (e.type === 'header-custom') {
      return { type: 'header-custom', settings: {
        client: (s.client || []).map(i => ({ rand: i.rand || 0, randRange: i.randRange || '0-255', type: i.type || 'array', _packetStr: Array.isArray(i.packet) ? i.packet.join(',') : (i.packet || '') })),
        server: (s.server || []).map(i => ({ rand: i.rand || 0, randRange: i.randRange || '0-255', type: i.type || 'array', _packetStr: Array.isArray(i.packet) ? i.packet.join(',') : (i.packet || '') })),
      }, _json: '' }
    }
    const base = { type: e.type || '', settings: { ...s }, _json: '' }
    if (e.type === 'noise') base.settings._noise = (s.noise || []).map(n => ({ type: n.type || 'array', rand: n.rand || '', randRange: n.randRange || '0-255', delay: n.delay || '', _packetStr: Array.isArray(n.packet) ? n.packet.join(',') : (n.packet || '') }))
    if (e.type === 'xdns') { base.settings._domainsStr = (s.domains || []).join('\n'); base.settings._resolversStr = (s.resolvers || []).join('\n') }
    if (e.type === 'xicmp') base.settings._ipsStr = (s.ips || []).join('\n')
    if (e.type === 'realm') {
      const tls = s.tlsConfig || {}
      base.settings._stunServersStr = (s.stunServers || []).join('\n')
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
      congestion: qp.congestion || '', bbrProfile: qp.bbrProfile || 'standard', debug: qp.debug || false,
      brutalUp: qp.brutalUp || '200 mbps', brutalDown: qp.brutalDown || '200 mbps',
      _showUdpHop: !!qp.udpHop,
      udpHop: qp.udpHop || { ports: '20000-50000', interval: '5-10' },
      maxIdleTimeout: qp.maxIdleTimeout || 30, keepAlivePeriod: qp.keepAlivePeriod || 10,
      disablePathMTUDiscovery: qp.disablePathMTUDiscovery || false, maxIncomingStreams: qp.maxIncomingStreams || 1024,
      initStreamReceiveWindow: qp.initStreamReceiveWindow || 8388608, maxStreamReceiveWindow: qp.maxStreamReceiveWindow || 8388608,
      initConnectionReceiveWindow: qp.initConnectionReceiveWindow || 20971520, maxConnectionReceiveWindow: qp.maxConnectionReceiveWindow || 20971520,
    },
  }
}

function newClientServerItem() { return { delay: 0, rand: 0, randRange: '0-255', type: 'array', _packetStr: '' } }
function newUdpClientServerItem() { return { rand: 0, randRange: '0-255', type: 'array', _packetStr: '' } }
function newNoiseItem() { return { type: 'array', rand: '1-8192', randRange: '0-255', delay: '10-20', _packetStr: '' } }
// rand 与 packet 互斥
function onRandChange(item) { if (item.rand) item._packetStr = ''; emit() }
function onPacketChange(item) { if (item._packetStr) item.rand = 0; emit() }

function onMaskTypeChange(entry, protocol) {
  entry.settings = {}
  entry._json = ''
  if (entry.type === 'header-custom') {
    if (protocol === 'tcp') entry.settings = { clients: [[newClientServerItem()]], servers: [[]], errors: [[]] }
    else entry.settings = { client: [newUdpClientServerItem()], server: [newUdpClientServerItem()] }
  }
  if (entry.type === 'fragment') entry.settings = { packets: 'tlshello', length: '100-200', delay: '', maxSplit: '' }
  if (entry.type === 'sudoku') entry.settings = { password: genPassword(), ascii: '', customTable: '', _customTablesStr: '', paddingMin: 0, paddingMax: 0 }
  if (entry.type === 'salamander') entry.settings = { password: genPassword(), _gecko: false, _geckoMin: 512, _geckoMax: 1200 }
  if (entry.type === 'mkcp-legacy') entry.settings = { header: '', value: genAES128Key() }
  if (entry.type === 'noise') entry.settings = { reset: 0, _noise: [newNoiseItem()] }
  if (entry.type === 'xdns') entry.settings = { _domainsStr: '', _resolversStr: '' }
  if (entry.type === 'xicmp') entry.settings = { dgram: false, _ipsStr: '' }
  if (entry.type === 'realm') entry.settings = { url: '', _stunServersStr: '', _tlsServerName: '', _tlsAlpn: [], _tlsFingerprint: '', _tlsAllowInsecure: false }
}

function onMkcpHeaderChange(entry) {
  // Clear value when header changes
  if (entry.settings.header === 'dns') {
    entry.settings.value = 'www.baidu.com'
  } else if (!entry.settings.header) {
    // Default (AES-128-GCM): generate new key
    entry.settings.value = genAES128Key()
  } else {
    // dtls, srtp, utp, wechat, wireguard: clear value
    entry.settings.value = ''
  }
  emit()
}

function onQuicUdpHopToggle(val) {
  if (val && !rf.finalmask.quicParams.udpHop) {
    rf.finalmask.quicParams.udpHop = { ports: '20000-50000', interval: '5-10' }
  }
}

function onCongestionChange(val) {
  if (val === 'brutal' || val === 'force-brutal') {
    if (!rf.finalmask.quicParams.brutalUp) rf.finalmask.quicParams.brutalUp = '60 mbps'
    if (!rf.finalmask.quicParams.brutalDown && rf.finalmask.quicParams.brutalDown !== 0) rf.finalmask.quicParams.brutalDown = '100 mbps'
  }
  emit()
}

function onGeckoToggle(entry) {
  if (entry.settings._gecko) {
    entry.settings._geckoMin = 512
    entry.settings._geckoMax = 1200
    entry.settings.packetSize = '512-1200'
  } else {
    delete entry.settings.packetSize
    delete entry.settings._geckoMin
    delete entry.settings._geckoMax
  }
  emit()
}

function onGeckoSizeChange(entry) {
  const min = entry.settings._geckoMin || 1
  const max = entry.settings._geckoMax || 2048
  entry.settings.packetSize = min + '-' + max
  emit()
}

function parseSockopt(s) {
  if (!s || Object.keys(s).length === 0) return { _enabled: false, mark: 0, tcpFastOpen: false, tproxy: 'off', domainStrategy: '', interface: '', v6only: false, tcpKeepAliveIdle: 0, tcpKeepAliveInterval: 0, tcpUserTimeout: 10000, tcpcongestion: '', tcpMaxSeg: 1440, tcpWindowClamp: 600, tcpMptcp: false, acceptProxyProtocol: false, dialerProxy: '', addressPortStrategy: '', _trustedXffStr: '', _showHappyEyeballs: false, happyEyeballs: { tryDelayMs: 250, prioritizeIPv6: false, interleave: 1, maxConcurrentTry: 4 }, _showCustomSockopt: false, _customSockopt: [] }
  const he = s.happyEyeballs || {}
  const cs = s.customSockopt || []
  const hasHe = he.tryDelayMs > 0
  const hasCs = Array.isArray(cs) && cs.length > 0
  return {
    _enabled: true,
    mark: s.mark !== undefined ? s.mark : 0,
    tcpFastOpen: s.tcpFastOpen === true,
    tproxy: s.tproxy || 'off',
    domainStrategy: s.domainStrategy !== undefined ? s.domainStrategy : '',
    interface: s.interface || '',
    v6only: s.V6Only === true || s.v6only === true,
    tcpKeepAliveIdle: s.tcpKeepAliveIdle !== undefined ? s.tcpKeepAliveIdle : 0,
    tcpKeepAliveInterval: s.tcpKeepAliveInterval !== undefined ? s.tcpKeepAliveInterval : 0,
    tcpUserTimeout: s.tcpUserTimeout !== undefined ? s.tcpUserTimeout : 10000,
    tcpcongestion: s.tcpcongestion || '',
    tcpMaxSeg: s.tcpMaxSeg !== undefined ? s.tcpMaxSeg : 1440,
    tcpWindowClamp: s.tcpWindowClamp !== undefined ? s.tcpWindowClamp : 600,
    tcpMptcp: s.tcpMptcp === true,
    acceptProxyProtocol: s.acceptProxyProtocol === true,
    dialerProxy: s.dialerProxy || '',
    addressPortStrategy: s.addressPortStrategy || '',
    _trustedXffStr: (s.trustedXForwardedFor || []).join('\n'),
    _showHappyEyeballs: hasHe,
    happyEyeballs: hasHe ? { tryDelayMs: he.tryDelayMs, prioritizeIPv6: he.prioritizeIPv6 || false, interleave: he.interleave || 1, maxConcurrentTry: he.maxConcurrentTry || 4 } : { tryDelayMs: 250, prioritizeIPv6: false, interleave: 1, maxConcurrentTry: 4 },
    _showCustomSockopt: hasCs,
    _customSockopt: Array.isArray(cs) ? cs.map(c => ({ system: c.system || '', network: c.network || '', type: c.type || 'str', level: c.level || '6', opt: c.opt || '', value: c.value || '' })) : [],
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
      uplinkHTTPMethod: extra.uplinkHTTPMethod || '',
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
  // Convert old 'users' to 'clients' for compatibility
  if (settings.users && !settings.clients) {
    settings.clients = settings.users
    delete settings.users
  }
  // Ensure clients array exists for protocols that use it
  if (['vless', 'vmess', 'trojan', 'hysteria'].includes(ib.protocol) && !settings.clients) {
    settings.clients = []
  }
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
  // Detect VLESS encryption type from stored encryption value
  if (ib.protocol === 'vless') {
    const enc = settings.encryption || ''
    if (enc.includes('mlkem768')) {
      settings._encryptionType = 'mlkem768'
    } else if (enc.includes('x25519') && enc !== 'none') {
      settings._encryptionType = 'x25519'
    } else {
      settings._encryptionType = ''
    }
    if (!settings.encryption) settings.encryption = 'none'
    if (!settings.decryption) settings.decryption = 'none'
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
      tls: parseTlsSettings(tls),
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
    sockopt: parseSockopt(ib.streamSettings?.sockopt),
    finalmask: parseFinalmask(ib.streamSettings?.finalmask),
    _fallbackEnabled: !!(ib.settings?.fallbacks && ib.settings.fallbacks.length),
    fallbacks: (ib.settings?.fallbacks || []).map(f => ({ name: f.name || '', _alpn: f.alpn || '', path: f.path || '', dest: f.dest || 80, xver: f.xver || 0 })),
    _wgAddressesStr: (ib.settings?.address || []).join(', '),
    _wgPublicKey: ib._wgPublicKey || '',
    _tunGatewayStr: (ib.settings?.gateway || []).join(', '),
    _tunDnsStr: (ib.settings?.dns || []).join(', '),
    _tunAutoRoutingStr: (ib.settings?.autoSystemRoutingTable || []).join(', '),
    // Port Hop config
    _portHopEnabled: ib._portHop?.enabled || ib._portHopEnabled || false,
    _portHopMode: ib._portHop?.mode || ib._portHopMode || 'preset',
    _portHopRange: ib._portHop?.range || ib._portHopRange || '20000-50000',
    _portHopStart: ib._portHop?.start || ib._portHopStart || 20000,
    _portHopEnd: ib._portHop?.end || ib._portHopEnd || 50000,
    _portHopInterface: ib._portHop?.interface || ib._portHopInterface || '',
    // Access Domain config
    _accessDomainEnabled: ib._accessDomain?.enabled || false,
    _accessDomainType: ib._accessDomain?.type || 'direct',
    _directDomain: ib._accessDomain?.directDomain || '',
    _directPortMode: ib._accessDomain?.directPortMode || 'config',
    _cfDomain: ib._accessDomain?.cfDomain || '',
    _cfIP: ib._accessDomain?.cfIP || '',
    _customDomain: ib._accessDomain?.customDomain || '',
    _customPortMode: ib._accessDomain?.customPortMode || 'custom',
    _accessCustomPort: ib._accessDomain?.customPort || 443,
  }
}

function buildInbound(ib, isApiCall = false) {
  const settings = JSON.parse(JSON.stringify(ib.settings))
  // Remove frontend-only internal fields from settings
  delete settings._encryptionType
  // For Shadowsocks single-user methods, remove clients
  if (ib.protocol === 'shadowsocks' && !isSSMultiUser(settings.method)) {
    delete settings.clients
  }
  // Filter out paused users from xray config (keep in DB config)
  if (isApiCall && settings.clients) {
    settings.clients = settings.clients.filter(u => !u._paused)
  }
  if (isApiCall && settings.users) {
    settings.users = settings.users.filter(u => !u._paused)
  }
  const result = {
    _remark: ib._remark || undefined,
    tag: ib.tag || undefined, protocol: ib.protocol,
    settings: settings,
    sniffing: {
      enabled: ib.sniffing.enabled || false,
      destOverride: ib.sniffing.destOverride || ['http', 'tls', 'quic'],
      metadataOnly: ib.sniffing.metadataOnly || false,
      domainsExcluded: ib.sniffing._domainsExcludedStr ? ib.sniffing._domainsExcludedStr.split('\n').map(d => d.trim()).filter(Boolean) : undefined,
      ipsExcluded: ib.sniffing._ipsExcludedStr ? ib.sniffing._ipsExcludedStr.split('\n').map(d => d.trim()).filter(Boolean) : undefined,
      routeOnly: ib.sniffing.routeOnly || false,
    },
    // Access Domain config
    _accessDomain: {
      enabled: ib._accessDomainEnabled || false,
      type: ib._accessDomainType || 'direct',
      directDomain: ib._directDomain || '',
      directPortMode: ib._directPortMode || 'config',
      cfDomain: ib._cfDomain || '',
      cfIP: ib._cfIP || '',
      customDomain: ib._customDomain || '',
      customPortMode: ib._customPortMode || 'custom',
      customPort: ib._accessCustomPort || 443,
    },
    // Port Hop config
    _portHop: {
      enabled: ib._portHopEnabled || false,
      mode: ib._portHopMode || 'preset',
      range: ib._portHopRange || '20000-50000',
      start: ib._portHopStart || 20000,
      end: ib._portHopEnd || 50000,
      interface: ib._portHopInterface || '',
    },
  }
  // Add sockopt only if enabled
  const sk = ib.sockopt || {}
  const sockopt = {}
  if (sk._enabled) {
  if (sk.mark) sockopt.mark = sk.mark
  if (sk.tcpFastOpen) sockopt.tcpFastOpen = true
  if (sk.tproxy && sk.tproxy !== 'off') sockopt.tproxy = sk.tproxy
  if (sk.domainStrategy) sockopt.domainStrategy = sk.domainStrategy
  if (sk.interface) sockopt.interface = sk.interface
  if (sk.v6only) sockopt.V6Only = true
  sockopt.tcpKeepAliveIdle = sk.tcpKeepAliveIdle || 0
  sockopt.tcpKeepAliveInterval = sk.tcpKeepAliveInterval || 0
  sockopt.tcpUserTimeout = sk.tcpUserTimeout || 10000
  if (sk.tcpcongestion) sockopt.tcpcongestion = sk.tcpcongestion
  sockopt.tcpMaxSeg = sk.tcpMaxSeg || 1440
  sockopt.tcpWindowClamp = sk.tcpWindowClamp || 600
  if (sk.tcpMptcp) sockopt.tcpMptcp = true
  if (sk.acceptProxyProtocol) sockopt.acceptProxyProtocol = true
  if (sk.dialerProxy) sockopt.dialerProxy = sk.dialerProxy
  if (sk.addressPortStrategy) sockopt.addressPortStrategy = sk.addressPortStrategy
  if (sk._trustedXffStr) sockopt.trustedXForwardedFor = sk._trustedXffStr.split('\n').map(s => s.trim()).filter(Boolean)
  // happyEyeballs: write full config when enabled
  if (sk._showHappyEyeballs && sk.happyEyeballs && sk.happyEyeballs.tryDelayMs > 0) {
    sockopt.happyEyeballs = {
      tryDelayMs: sk.happyEyeballs.tryDelayMs || 250,
      prioritizeIPv6: sk.happyEyeballs.prioritizeIPv6 || false,
      interleave: sk.happyEyeballs.interleave || 1,
      maxConcurrentTry: sk.happyEyeballs.maxConcurrentTry || 4,
    }
  }
  // customSockopt: only write when enabled and has entries
  if (sk._showCustomSockopt && sk._customSockopt && sk._customSockopt.filter(c => c.opt).length) {
    sockopt.customSockopt = sk._customSockopt.filter(c => c.opt).map(c => {
      const obj = { type: c.type || 'str', level: c.level || '6', opt: c.opt, value: c.value || '' }
      if (c.system) obj.system = c.system
      if (c.network) obj.network = c.network
      return obj
    })
  }
  } // end sockopt._enabled
  // Only add streamSettings for protocols that use transport layer
  const noStreamProtocols = ['wireguard', 'tun', 'socks', 'http', 'tunnel', 'dokodemo-door']
  if (!noStreamProtocols.includes(ib.protocol)) {
    result.streamSettings = {
      network: ib.streamSettings.network,
      security: ib.streamSettings.security,
    }
    if (sk._enabled && Object.keys(sockopt).length) result.streamSettings.sockopt = sockopt
  }
  // Add finalmask only if enabled
  const fm = ib.finalmask || {}
  if (fm._enabled) {
    const finalmask = {}
    // Helper: convert _packetStr to packet array
    const toPacket = (s) => { const p = s._packetStr; if (!p) return []; return p.split(',').map(v => { const n = Number(v); return isNaN(n) ? v : n }) }
    const cleanInternal = (s) => { const r = {}; Object.keys(s).forEach(k => { if (!k.startsWith('_') && s[k] !== '' && s[k] !== undefined) r[k] = s[k] }); return r }
    // TCP mask
    if (fm.tcp && fm.tcp.length) {
      finalmask.tcp = fm.tcp.filter(e => e.type).map(e => {
        const s = { ...e.settings }
        if (e.type === 'header-custom') {
          const convertGroup = grp => (grp || []).map(items => (items || []).map(i => { const item = cleanInternal(i); if (i._packetStr) item.packet = toPacket(i); return item }))
          return { type: 'header-custom', settings: { clients: convertGroup(s.clients), servers: convertGroup(s.servers), errors: convertGroup(s.errors) } }
        }
        delete s._customTablesStr
        return { type: e.type, settings: s }
      })
    }
    // UDP mask
    if (fm.udp && fm.udp.length) {
      finalmask.udp = fm.udp.filter(e => e.type).map(e => {
        const s = { ...e.settings }
        if (e.type === 'header-custom') {
          const convertArr = arr => (arr || []).map(i => { const item = cleanInternal(i); if (i._packetStr) item.packet = toPacket(i); return item })
          return { type: 'header-custom', settings: { client: convertArr(s.client), server: convertArr(s.server) } }
        }
        if (e.type === 'noise') {
          s.noise = (s._noise || []).filter(n => n.rand).map(n => { const item = cleanInternal(n); if (n._packetStr) item.packet = toPacket(n); return item })
          delete s._noise
        }
        if (e.type === 'xdns') {
          if (s._domainsStr) s.domains = s._domainsStr.split('\n').map(d => d.trim()).filter(Boolean)
          if (s._resolversStr) s.resolvers = s._resolversStr.split('\n').map(d => d.trim()).filter(Boolean)
          delete s._domainsStr; delete s._resolversStr
        }
        if (e.type === 'xicmp') { if (s._ipsStr) s.ips = s._ipsStr.split('\n').map(d => d.trim()).filter(Boolean); delete s._ipsStr }
        if (e.type === 'realm') {
          if (s._stunServersStr) s.stunServers = s._stunServersStr.split('\n').map(d => d.trim()).filter(Boolean)
          const tlsCfg = {}
          if (s._tlsServerName) tlsCfg.serverName = s._tlsServerName
          if (s._tlsAlpn && s._tlsAlpn.length) tlsCfg.alpn = s._tlsAlpn
          if (s._tlsFingerprint) tlsCfg.fingerprint = s._tlsFingerprint
          if (s._tlsAllowInsecure) tlsCfg.allowInsecure = true
          if (Object.keys(tlsCfg).length) s.tlsConfig = tlsCfg
          delete s._stunServersStr; delete s._tlsServerName; delete s._tlsAlpn; delete s._tlsFingerprint; delete s._tlsAllowInsecure
        }
        if (e.type === 'sudoku' && s._customTablesStr) { s.customTables = s._customTablesStr.split('\n').map(d => d.trim()).filter(Boolean); delete s._customTablesStr }
        // mkcp-legacy: preserve header field even when empty
        if (e.type === 'mkcp-legacy') {
          const header = s.header || ''
          const value = s.value || ''
          return { type: 'mkcp-legacy', settings: { header, value } }
        }
        return { type: e.type, settings: cleanInternal(s) }
      })
    }
    // QUIC params
    // quicParams: always write when finalmask is enabled
    const qp = fm.quicParams || {}
    const quicObj = {}
    if (qp.congestion) quicObj.congestion = qp.congestion
    if (qp.bbrProfile) quicObj.bbrProfile = qp.bbrProfile
    if (qp.debug) quicObj.debug = true
    quicObj.brutalUp = qp.brutalUp || '200 mbps'
    quicObj.brutalDown = qp.brutalDown || '200 mbps'
    if (qp._showUdpHop && qp.udpHop) quicObj.udpHop = qp.udpHop
    quicObj.maxIdleTimeout = qp.maxIdleTimeout || 30
    quicObj.keepAlivePeriod = qp.keepAlivePeriod || 10
    if (qp.disablePathMTUDiscovery) quicObj.disablePathMTUDiscovery = true
    quicObj.maxIncomingStreams = qp.maxIncomingStreams || 1024
    quicObj.initStreamReceiveWindow = qp.initStreamReceiveWindow || 8388608
    quicObj.maxStreamReceiveWindow = qp.maxStreamReceiveWindow || 8388608
    quicObj.initConnectionReceiveWindow = qp.initConnectionReceiveWindow || 20971520
    quicObj.maxConnectionReceiveWindow = qp.maxConnectionReceiveWindow || 20971520
    finalmask.quicParams = quicObj
    if (Object.keys(finalmask).length) result.streamSettings.finalmask = finalmask
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
      if (tls._alpn && tls._alpn.length) tlsSettings.alpn = tls._alpn
      if (tls.minVersion) tlsSettings.minVersion = tls.minVersion
      if (tls.maxVersion) tlsSettings.maxVersion = tls.maxVersion
      if (tls._cipherSuites && tls._cipherSuites.length) tlsSettings.cipherSuites = tls._cipherSuites.join(':')
      if (tls._curves && tls._curves.length) tlsSettings.curvePreferences = tls._curves
      if (tls.rejectUnknownSni) tlsSettings.rejectUnknownSni = true
      if (tls.disableSystemRoot) tlsSettings.disableSystemRoot = true
      if (tls.enableSessionResumption) tlsSettings.enableSessionResumption = true
      if (tls.masterKeyLog) tlsSettings.masterKeyLog = tls.masterKeyLog
      if (tls.echServerKeys) tlsSettings.echServerKeys = tls.echServerKeys
      // Client params in settings array
      const settingsObj = {}
      if (tls.fingerprint) settingsObj.fingerprint = tls.fingerprint
      if (tls.echConfigList) settingsObj.echConfigList = tls.echConfigList
      if (Object.keys(settingsObj).length) tlsSettings.settings = [settingsObj]
      // Certificates with ../../../ prefix
      const hasCert = (tls._certMode === 'file' && (tls._certificateFile || tls._keyFile)) || (tls._certMode === 'content' && (tls._certificateContent || tls._keyContent))
      if (hasCert) {
        const certObj = {}
        certObj.usage = tls._certUsage || 'encipherment'
        if (tls._buildChain) certObj.buildChain = true
        certObj.ocspStapling = tls._ocspStapling || 3600
        if (tls._oneTimeLoading) certObj.oneTimeLoading = true
        if (tls._certMode === 'file') {
          // For API calls, use absolute paths; for config file, use ../../../ prefix
          if (isApiCall) {
            // Absolute path for API calls
            if (tls._certificateFile) {
              const certPath = tls._certificateFile.replace(/^(\.\.\/)+/, '').replace(/^\//, '')
              certObj.certificateFile = dataDir.value + '/' + certPath
            }
            if (tls._keyFile) {
              const keyPath = tls._keyFile.replace(/^(\.\.\/)+/, '').replace(/^\//, '')
              certObj.keyFile = dataDir.value + '/' + keyPath
            }
          } else {
            // Relative path for config file (../../../ prefix)
            if (tls._certificateFile) {
              const certPath = tls._certificateFile.replace(/^(\.\.\/)+/, '').replace(/^\//, '')
              certObj.certificateFile = '../../../' + certPath
            }
            if (tls._keyFile) {
              const keyPath = tls._keyFile.replace(/^(\.\.\/)+/, '').replace(/^\//, '')
              certObj.keyFile = '../../../' + keyPath
            }
          }
        } else {
          if (tls._certificateContent) certObj.certificate = tls._certificateContent.split('\n').filter(Boolean)
          if (tls._keyContent) certObj.key = tls._keyContent.split('\n').filter(Boolean)
        }
        tlsSettings.certificates = [certObj]
      }
      result.streamSettings.tlsSettings = tlsSettings
    }
    // Add Reality settings if security is reality
    if (ib.streamSettings.security === 'reality') {
      const r = ib.streamSettings.reality || {}
      const rs = {}
      if (r.target) rs.target = r.target
      const names = r.serverNamesStr ? r.serverNamesStr.split('\n').map(s => s.trim()).filter(Boolean) : []
      if (names.length) rs.serverNames = names
      if (r.privateKey) rs.privateKey = r.privateKey
      const sids = r.shortIdsStr ? r.shortIdsStr.split('\n').map(s => s.trim()).filter(s => s !== '') : []
      if (sids.length) rs.shortIds = sids
      if (r.show) rs.show = true
      if (r.xver) rs.xver = r.xver
      if (r.minClientVer) rs.minClientVer = r.minClientVer
      if (r.maxClientVer) rs.maxClientVer = r.maxClientVer
      if (r.maxTimeDiff) rs.maxTimeDiff = r.maxTimeDiff
      if (r.mldsa65Seed) rs.mldsa65Seed = r.mldsa65Seed
      if (r._showLimitUpload) rs.limitFallbackUpload = r.limitFallbackUpload
      if (r._showLimitDownload) rs.limitFallbackDownload = r.limitFallbackDownload
      // Client params in settings array
      const settingsObj = {}
      if (r._fingerprint) settingsObj.fingerprint = r._fingerprint
      if (r._password) settingsObj.password = r._password
      if (r._mldsa65Verify) settingsObj.mldsa65Verify = r._mldsa65Verify
      if (Object.keys(settingsObj).length) rs.settings = [settingsObj]
      result.streamSettings.realitySettings = rs
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
      xhSettings.mode = xh.mode || 'auto'
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
  // Add fallbacks if enabled, otherwise remove
  if (ib._fallbackEnabled && ib.fallbacks && ib.fallbacks.length) {
    result.settings.fallbacks = ib.fallbacks.filter(f => f.dest).map(f => {
      const fb = { dest: f.dest }
      if (f.name) fb.name = f.name
      if (f._alpn) fb.alpn = f._alpn
      if (f.path) fb.path = f.path
      if (f.xver) fb.xver = f.xver
      return fb
    })
    if (!result.settings.fallbacks.length) delete result.settings.fallbacks
  } else {
    delete result.settings.fallbacks
  }
  return result
}

function showAdd() {
  editIdx.value = -1
  rf._id = null
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
  rf.sockopt = JSON.parse(JSON.stringify(d.sockopt))
  rf.finalmask = JSON.parse(JSON.stringify(d.finalmask))
  rf._accessDomainEnabled = d._accessDomainEnabled || false
  rf._accessDomainType = d._accessDomainType || 'direct'
  rf._directDomain = d._directDomain || ''
  rf._directPortMode = d._directPortMode || 'config'
  rf._cfDomain = d._cfDomain || ''
  rf._cfIP = d._cfIP || ''
  rf._customDomain = d._customDomain || ''
  rf._customPortMode = d._customPortMode || 'custom'
  rf._accessCustomPort = d._accessCustomPort || 443
  rf._fallbackEnabled = d._fallbackEnabled || false
  rf.fallbacks = JSON.parse(JSON.stringify(d.fallbacks || []))
  rf._wgAddressesStr = d._wgAddressesStr
  rf._wgPublicKey = ''
  rf._tunGatewayStr = d._tunGatewayStr
  rf._tunDnsStr = d._tunDnsStr
  rf._tunAutoRoutingStr = d._tunAutoRoutingStr
  // Port Hop
  rf._portHopEnabled = d._portHopEnabled
  rf._portHopMode = d._portHopMode
  rf._portHopRange = d._portHopRange
  rf._portHopStart = d._portHopStart
  rf._portHopEnd = d._portHopEnd
  rf._portHopInterface = d._portHopInterface
  activeTab.value = 'basic'
  dlg.value = true
  // Auto-select default target or random
  const def = realityTargets.value.find(t => t.default)
  const rt = def || realityTargets.value[Math.floor(Math.random() * realityTargets.value.length)]
  rf.streamSettings.reality.target = rt.target
  rf.streamSettings.reality.serverNamesStr = rt.sni
  genRealityKey(rf)
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
  rf.sockopt = JSON.parse(JSON.stringify(ib.sockopt || { _enabled: false, mark: 0, tcpFastOpen: false, tproxy: 'off', domainStrategy: '', interface: '', v6only: false, tcpKeepAliveIdle: 0, tcpKeepAliveInterval: 0, tcpUserTimeout: 10000, tcpcongestion: '', tcpMaxSeg: 1440, tcpWindowClamp: 600, tcpMptcp: false, acceptProxyProtocol: false, dialerProxy: '', addressPortStrategy: '', _trustedXffStr: '', _showHappyEyeballs: false, happyEyeballs: { tryDelayMs: 250, prioritizeIPv6: false, interleave: 1, maxConcurrentTry: 4 }, _showCustomSockopt: false, _customSockopt: [] }))
  rf.finalmask = JSON.parse(JSON.stringify(ib.finalmask || { _enabled: false, tcp: [], udp: [], quicParams: { congestion: '', bbrProfile: 'standard', debug: false, brutalUp: '200 mbps', brutalDown: '200 mbps', _showUdpHop: false, udpHop: { ports: '20000-50000', interval: '5-10' }, maxIdleTimeout: 30, keepAlivePeriod: 10, disablePathMTUDiscovery: false, maxIncomingStreams: 1024, initStreamReceiveWindow: 8388608, maxStreamReceiveWindow: 8388608, initConnectionReceiveWindow: 20971520, maxConnectionReceiveWindow: 20971520 } }))
  rf._fallbackEnabled = ib._fallbackEnabled || false
  rf.fallbacks = JSON.parse(JSON.stringify(ib.fallbacks || []))
  rf._wgAddressesStr = ib._wgAddressesStr || ''
  rf._wgPublicKey = ib._wgPublicKey || ''
  rf._tunGatewayStr = ib._tunGatewayStr || ''
  rf._tunDnsStr = ib._tunDnsStr || ''
  rf._tunAutoRoutingStr = ib._tunAutoRoutingStr || ''
  // Access Domain config
  rf._accessDomainEnabled = ib._accessDomainEnabled || false
  rf._accessDomainType = ib._accessDomainType || 'direct'
  rf._directDomain = ib._directDomain || ''
  rf._directPortMode = ib._directPortMode || 'config'
  rf._cfDomain = ib._cfDomain || ''
  rf._cfIP = ib._cfIP || ''
  rf._customDomain = ib._customDomain || ''
  rf._customPortMode = ib._customPortMode || 'custom'
  rf._accessCustomPort = ib._accessCustomPort || 443
  // Port Hop config
  rf._portHopEnabled = ib._portHopEnabled || false
  rf._portHopMode = ib._portHopMode || 'preset'
  rf._portHopRange = ib._portHopRange || '20000-50000'
  rf._portHopStart = ib._portHopStart || 20000
  rf._portHopEnd = ib._portHopEnd || 50000
  rf._portHopInterface = ib._portHopInterface || ''
  activeTab.value = 'basic'
  dlg.value = true
}

async function saveInbound() {
  // Build config for database (with relative paths)
  const ib = buildInbound(rf, false)
  // Build config for API call (with absolute paths)
  const apiConfig = buildInbound(rf, true)

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
      await client.put(`${nodeBase.value}/inbound-configs/${form.value[editIdx.value]._id}`, payload)
    } else {
      await client.post(`${nodeBase.value}/inbound-configs`, payload)
    }

    // Apply to running xray via gRPC (no restart) - use absolute paths
    try {
      if (!(await ensureXrayRunning(client, props.serverId, t('protoCenter.inbounds.savedToDb')))) {
        ElMessage.success(t('common.success'))
        dlg.value = false
        loadInbounds()
        return
      }
      const apiJson = { ...apiConfig }
      delete apiJson._remark
      delete apiJson._enabled
      delete apiJson._id
      // Remove internal fields before sending to xray
      delete apiJson._accessDomain
      delete apiJson._portHop
      delete apiJson._wgPublicKey
      // Only remove existing inbound when editing (not creating new)
      if (editIdx.value >= 0 && form.value[editIdx.value]._id) {
        try {
          await client.delete(`${nodeBase.value}/xray/control/inbound/${apiJson.tag}`, { suppressErrorMessage: true })
        } catch {}
      }
      // Add new inbound only if enabled
      if (rf._enabled !== false) {
        await client.post(`${nodeBase.value}/xray/control/inbound`, { inbound: apiJson }, { suppressErrorMessage: true })
      }
    } catch (xrayErr) {
      console.warn('Failed to apply to xray runtime:', xrayErr)
      // Don't fail the whole operation if xray apply fails
      notifyXrayRuntimeApplyFailure(xrayErr, t('protoCenter.inbounds.savedToDb'))
    }

    // Auto-manage firewall rules for port hopping
    if (rf.protocol === 'hysteria') {
      const isEnabled = rf._portHopEnabled || false
      const newPortRange = rf._portHopMode === 'preset' ? rf._portHopRange : `${rf._portHopStart}-${rf._portHopEnd}`
      const newIface = rf._portHopInterface || ''

      if (editIdx.value >= 0) {
        // Editing existing inbound
        const oldIb = form.value[editIdx.value]
        const wasEnabled = oldIb?._portHopEnabled || false
        const oldPortRange = oldIb?._portHopMode === 'preset'
          ? (oldIb?._portHopRange || '20000-50000')
          : `${oldIb?._portHopStart || 20000}-${oldIb?._portHopEnd || 50000}`
        const oldIface = oldIb?._portHopInterface || ''
        const oldPort = oldIb?.port || 0

        if (wasEnabled && !isEnabled) {
          // Port hop disabled: remove old firewall rule
          try {
            await client.post(`${nodeBase.value}/xray/control/firewall/remove`, {
              port_range: oldPortRange,
              target_port: oldPort,
              interface: oldIface
            })
          } catch (fwErr) {
            console.warn('Failed to remove firewall rules:', fwErr)
          }
        } else if (isEnabled && !wasEnabled) {
          // Port hop enabled: apply new firewall rule
          try {
            await client.post(`${nodeBase.value}/xray/control/firewall/apply`, {
              port_range: newPortRange || '20000-50000',
              target_port: rf.port,
              interface: newIface
            })
          } catch (fwErr) {
            console.warn('Failed to apply firewall rules:', fwErr)
          }
        } else if (isEnabled && wasEnabled) {
          // Port hop stayed enabled: check if port range, target port, or interface changed
          const configChanged = oldPortRange !== (newPortRange || '20000-50000') ||
            oldPort !== rf.port || oldIface !== newIface
          if (configChanged) {
            // Remove old rule then apply new rule
            try {
              await client.post(`${nodeBase.value}/xray/control/firewall/remove`, {
                port_range: oldPortRange,
                target_port: oldPort,
                interface: oldIface
              })
            } catch (fwErr) {
              console.warn('Failed to remove old firewall rules:', fwErr)
            }
            try {
              await client.post(`${nodeBase.value}/xray/control/firewall/apply`, {
                port_range: newPortRange || '20000-50000',
                target_port: rf.port,
                interface: newIface
              })
            } catch (fwErr) {
              console.warn('Failed to apply new firewall rules:', fwErr)
            }
          }
        }
      } else if (isEnabled) {
        // New inbound with port hopping enabled: apply firewall rule
        try {
          await client.post(`${nodeBase.value}/xray/control/firewall/apply`, {
            port_range: newPortRange || '20000-50000',
            target_port: rf.port,
            interface: newIface
          })
        } catch (fwErr) {
          console.warn('Failed to apply firewall rules:', fwErr)
        }
      }
    }

    ElMessage.success(t('common.success'))
    dlg.value = false
    await loadInbounds()
  } catch (e) {
    console.error('Save inbound failed:', e)
    ElMessage.error(e?.response?.data?.message || t('common.failed'))
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
    await client.put(`${nodeBase.value}/inbound-configs/${ib._id}`, {
      core_type: props.coreType || 'xray',
      remark: ib._remark || '',
      protocol: ib.protocol,
      tag: ib.tag,
      listen: ib.listen || '0.0.0.0',
      port: ib.port || 0,
      enabled: ib._enabled,
      config_json: configJson,
    })

    // Apply to running xray via gRPC
    try {
      if (await ensureXrayRunning(client, props.serverId, '')) {
        if (ib._enabled) {
          // Enable: add inbound to xray runtime
          const apiConfig = buildInbound(ib, true)
          delete apiConfig._remark
          delete apiConfig._enabled
          delete apiConfig._id
          delete apiConfig._accessDomain
          delete apiConfig._portHop
          delete apiConfig._wgPublicKey
          // Remove existing first to avoid duplicate
          try {
            await client.delete(`${nodeBase.value}/xray/control/inbound/${ib.tag}`, { suppressErrorMessage: true })
          } catch {}
          await client.post(`${nodeBase.value}/xray/control/inbound`, { inbound: apiConfig }, { suppressErrorMessage: true })
        } else {
          // Disable: remove inbound from xray runtime
          await client.delete(`${nodeBase.value}/xray/control/inbound/${ib.tag}`, { suppressErrorMessage: true })
        }
      }
    } catch (xrayErr) {
      console.warn('Failed to apply toggle to xray runtime:', xrayErr)
    }

    ElMessage.success(ib._enabled ? t('common.enabled') : t('common.disabled'))
  } catch (e) {
    ib._enabled = !ib._enabled
    ElMessage.error(t('common.failed'))
  }
}

async function rmInbound(i) {
  const ib = form.value[i]
  if (ib._id) {
    try { await client.delete(`${nodeBase.value}/inbound-configs/${ib._id}`) } catch {}
  }
  // Also remove from xray runtime
  if (ib.tag) {
    try {
      if (await ensureXrayRunning(client, props.serverId, t('protoCenter.inbounds.deletedFromDb'))) {
        await client.delete(`${nodeBase.value}/xray/control/inbound/${ib.tag}`, { suppressErrorMessage: true })
      }
    } catch (e) {
      notifyXrayRuntimeApplyFailure(e, t('protoCenter.inbounds.deletedFromDb'))
    }
  }
  // Remove firewall rules if port hopping was enabled
  if (ib.protocol === 'hysteria' && ib._portHopEnabled) {
    const portRange = ib._portHopMode === 'preset'
      ? (ib._portHopRange || '20000-50000')
      : `${ib._portHopStart || 20000}-${ib._portHopEnd || 50000}`
    try {
      await client.post(`${nodeBase.value}/xray/control/firewall/remove`, {
        port_range: portRange,
        target_port: ib.port || 0,
        interface: ib._portHopInterface || ''
      })
    } catch (fwErr) {
      console.warn('Failed to remove firewall rules on delete:', fwErr)
    }
  }
  form.value.splice(i, 1)
  ElMessage.success(t('common.success'))
}

async function loadInbounds() {
  try {
    const data = await client.get(`${nodeBase.value}/inbound-configs`, { params: { core_type: props.coreType || 'xray' } })
    form.value = (data || []).map(item => {
      const cfg = typeof item.config_json === 'string' ? JSON.parse(item.config_json) : (item.config_json || {})
      const parsed = parseInbound(cfg)
      // Use remark from API response (stored separately in DB)
      if (item.remark) {
        parsed._remark = item.remark
      }
      return { ...parsed, _id: item.id, _enabled: item.enabled }
    })
  } catch { form.value = [] }
}

function emit() { /* no longer needed, saves directly to API */ }

onMounted(() => { loadInbounds(); loadCertList(); loadTargets(); loadInterfaces(); fetchXrayStats(); loadDataDir(); loadCFDomains(); loadCFIPEntries() })
</script>

<style scoped>
.inbound-item { background: var(--el-fill-color-light); border-radius: 6px; padding: 12px; margin-bottom: 8px; }
.user-item { background: var(--el-fill-color); border-radius: 4px; padding: 8px; margin-bottom: 8px; border: 1px solid var(--el-border-color-lighter); }
:deep(.el-dialog__body) { max-height: calc(100vh - 200px); overflow-y: auto; }
</style>
