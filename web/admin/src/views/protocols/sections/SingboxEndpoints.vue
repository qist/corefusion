<template>
  <div>
    <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:16px">
      <div>
        <span style="font-size:15px;font-weight:600">{{ t('singbox.protocol.endpoint.title') }}</span>
        <span style="font-size:12px;color:#909399;margin-left:8px">{{ t('singbox.protocol.endpoint.subtitle') }}</span>
      </div>
      <el-button size="small" type="primary" @click="showAdd">{{ t('singbox.protocol.endpoint.add') }}</el-button>
    </div>

    <!-- Hint -->
    <div style="background:#f0f9eb;border-radius:6px;padding:8px 12px;margin-bottom:16px;font-size:12px;color:#67c23a">
      <el-icon style="vertical-align:middle"><InfoFilled /></el-icon>
      {{ t('singbox.protocol.endpoint.hint') }}
    </div>

    <!-- Table -->
    <el-table :data="epPaged" stripe style="width:100%" size="small">
      <el-table-column label="#" width="50" align="right">
        <template #default="{ $index }"><span style="font-size:11px;opacity:0.6">{{ (epPage - 1) * epSize + $index + 1 }}</span></template>
      </el-table-column>
      <el-table-column :label="t('common.actions')" width="70" align="center">
        <template #default="{ row }">
          <el-dropdown trigger="click" @command="(cmd) => handleAction(cmd, row)">
            <el-icon style="font-size:18px;cursor:pointer"><MoreFilled /></el-icon>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="edit"><el-icon><Edit /></el-icon> {{ t('common.edit') }}</el-dropdown-item>
                <el-dropdown-item command="delete" divided><span style="color:#f56c6c"><el-icon><Delete /></el-icon> {{ t('common.delete') }}</span></el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </template>
      </el-table-column>
      <el-table-column :label="t('common.enabled')" width="60" align="center">
        <template #default="{ row }">
          <el-switch v-model="row._enabled" size="small" @change="toggleEnabled(row)" />
        </template>
      </el-table-column>
      <el-table-column :label="t('singbox.protocol.endpoint.tag')" min-width="120" show-overflow-tooltip>
        <template #default="{row}"><span style="font-weight:600">{{ row.tag || '-' }}</span></template>
      </el-table-column>
      <el-table-column :label="t('singbox.protocol.endpoint.type')" width="100" align="center">
        <template #default="{row}"><el-tag size="small">{{ row.type || 'wireguard' }}</el-tag></template>
      </el-table-column>
      <el-table-column :label="t('singbox.protocol.endpoint.listenPort')" width="90" align="center">
        <template #default="{row}"><span>{{ row.listen_port || '-' }}</span></template>
      </el-table-column>
      <el-table-column :label="t('singbox.protocol.endpoint.address')" min-width="160" show-overflow-tooltip>
        <template #default="{row}"><span style="font-size:12px">{{ (row.address || []).join?.(', ') || row.address || '-' }}</span></template>
      </el-table-column>
      <el-table-column :label="t('singbox.protocol.endpoint.publicKey')" min-width="200" show-overflow-tooltip>
        <template #default="{row}">
          <span style="font-size:11px;font-family:monospace">{{ row.public_key ? row.public_key.substring(0, 20) + '...' : '-' }}</span>
        </template>
      </el-table-column>
      <el-table-column :label="t('singbox.protocol.endpoint.peers')" width="70" align="center">
        <template #default="{row}">{{ row.peers?.length || 0 }}</template>
      </el-table-column>
    </el-table>
    <PaginationBar v-model:page="epPage" v-model:page-size="epSize" :total="epTotal" />
    <div v-if="!form.length" style="text-align:center;color:#909399;padding:24px">{{ t('singbox.protocol.endpoint.noEndpoints') }}</div>

    <!-- Edit Dialog -->
    <el-dialog v-model="dlg" :title="editIdx >= 0 ? t('singbox.protocol.endpoint.edit') : t('singbox.protocol.endpoint.add')" width="900" :close-on-click-modal="false" top="3vh">
      <el-tabs v-model="activeTab" type="border-card" style="min-height:520px">
        <!-- ========== Basic Tab ========== -->
        <el-tab-pane :label="t('singbox.protocol.endpoint.tabs.basic')" name="basic">
          <el-form label-width="150px" label-position="left">
            <el-form-item :label="t('singbox.protocol.endpoint.type')">
              <el-select v-model="rf.type" style="width:100%" @change="onTypeChange">
                <el-option v-for="tp in ENDPOINT_TYPES" :key="tp" :label="ENDPOINT_TYPE_LABELS[tp] || tp" :value="tp" />
              </el-select>
            </el-form-item>
            <el-form-item :label="t('protoCenter.inbounds.remark')">
              <el-input v-model="rf._remark" @change="onRemarkChange" />
            </el-form-item>
            <el-form-item :label="t('singbox.listen.tag')">
              <el-input v-model="rf.tag" />
            </el-form-item>

            <!-- ===== WireGuard ===== -->
            <template v-if="rf.type === 'wireguard'">
              <el-divider content-position="left">{{ t('singbox.protocol.endpoint.networkSection') }}</el-divider>
              <el-form-item :label="t('singbox.protocol.endpoint.listenPort')">
                <el-input-number v-model="rf.listen_port" :min="0" :max="65535" style="width:100%" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.address')">
                <el-input v-model="rf._addressStr" :placeholder="t('singbox.protocol.wireguard.addressPlaceholder')" @change="onAddressChange" />
                <div class="hint">{{ t('singbox.protocol.endpoint.addressHint') }}</div>
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.mtu')">
                <el-input-number v-model="rf.mtu" :min="0" />
                <span class="hint">{{ t('singbox.protocol.wireguard.mtuHint') }}</span>
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.workers')">
                <el-input-number v-model="rf.workers" :min="0" />
                <span class="hint">{{ t('singbox.protocol.endpoint.workersHint') }}</span>
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.system')">
                <el-switch v-model="rf.system" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.name')">
                <el-input v-model="rf.name" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ts.onDemand')">
                <el-switch v-model="rf.on_demand" />
              </el-form-item>
            </template>

            <!-- ===== Tailscale ===== -->
            <template v-if="rf.type === 'tailscale'">
              <el-form-item :label="t('singbox.protocol.endpoint.ts.authKey')">
                <el-input v-model="rf.auth_key" :placeholder="t('singbox.protocol.endpoint.ts.authKeyPlaceholder')" show-password />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ts.controlUrl')">
                <el-input v-model="rf.control_url" :placeholder="t('singbox.protocol.endpoint.ts.controlUrlPlaceholder')" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ts.hostname')">
                <el-input v-model="rf.hostname" :placeholder="t('singbox.protocol.endpoint.ts.hostnamePlaceholder')" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ts.ephemeral')">
                <el-switch v-model="rf.ephemeral" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ts.acceptRoutes')">
                <el-switch v-model="rf.accept_routes" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ts.exitNode')">
                <el-input v-model="rf.exit_node" :placeholder="t('singbox.protocol.endpoint.ts.exitNodePlaceholder')" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ts.exitNodeAllowLan')">
                <el-switch v-model="rf.exit_node_allow_lan_access" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ts.advertiseRoutes')">
                <el-input v-model="rf._advertiseRoutesStr" placeholder="10.0.0.0/8, 192.168.0.0/24" @change="onAdvertiseRoutesChange" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ts.advertiseTags')">
                <el-input v-model="rf._advertiseTagsStr" placeholder="tag:server,tag:prod" @change="onAdvertiseTagsChange" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ts.advertiseExitNode')">
                <el-switch v-model="rf.advertise_exit_node" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.listenPort')">
                <el-input-number v-model="rf.listen_port" :min="0" :max="65535" style="width:100%" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ts.relayServerPort')">
                <el-input-number v-model="rf.relay_server_port" :min="0" :max="65535" style="width:100%" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ts.relayServerStaticEndpoints')">
                <el-input v-model="rf._relayServerStaticEndpointsStr" placeholder="v6:[...]:port, relay.example.com:443" @change="relayStaticEndpointsChange" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ts.systemInterface')">
                <el-switch v-model="rf.system_interface" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ts.systemInterfaceName')">
                <el-input v-model="rf.system_interface_name" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ts.systemInterfaceMtu')">
                <el-input-number v-model="rf.system_interface_mtu" :min="0" style="width:100%" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ts.udpTimeout')">
                <el-input v-model="rf.udp_timeout" placeholder="5m" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ts.sshServer')">
                <el-switch v-model="rf.ssh_server" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ts.taildropDirectory')">
                <el-input v-model="rf.taildrop_directory" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ts.stateDirectory')">
                <el-input v-model="rf.state_directory" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ts.onDemand')">
                <el-switch v-model="rf.on_demand" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.name')">
                <el-input v-model="rf.name" />
              </el-form-item>
            </template>

            <!-- ===== OpenConnect ===== -->
            <template v-if="rf.type === 'openconnect'">
              <el-divider content-position="left">{{ t('singbox.protocol.endpoint.oc.serverConfig') }}</el-divider>
              <el-form-item :label="t('singbox.protocol.endpoint.oc.server')">
                <el-input v-model="rf.server" :placeholder="t('singbox.protocol.endpoint.oc.serverPlaceholder')" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.oc.username')">
                <el-input v-model="rf.username" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.oc.password')">
                <el-input v-model="rf.password" show-password />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.oc.authGroup')">
                <el-input v-model="rf.auth_group" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.oc.flavor')">
                <el-select v-model="rf.flavor" style="width:100%">
                  <el-option label="anyconnect" value="anyconnect" />
                  <el-option label="nc" value="nc" />
                  <el-option label="gp" value="gp" />
                </el-select>
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.oc.userAgent')">
                <el-input v-model="rf.user_agent" :placeholder="t('singbox.protocol.endpoint.oc.userAgentPlaceholder')" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.oc.reported_os')">
                <el-select v-model="rf.reported_os" style="width:100%" allow-create>
                  <el-option label="Windows" value="win" />
                  <el-option label="Mac OS X" value="mac" />
                  <el-option label="Linux" value="linux" />
                  <el-option label="Android" value="android" />
                  <el-option label="iOS" value="ios" />
                </el-select>
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.oc.cookie')">
                <el-input v-model="rf.cookie" show-password />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.oc.version')">
                <el-input v-model="rf.version" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.oc.localHostname')">
                <el-input v-model="rf.local_hostname" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.oc.system')">
                <el-switch v-model="rf.system" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.oc.name')">
                <el-input v-model="rf.name" />
              </el-form-item>

              <!-- OpenConnect Token settings -->
              <el-collapse>
                <el-collapse-item :title="t('singbox.protocol.endpoint.oc.token') || 'Token Settings'">
                  <el-form-item :label="t('singbox.protocol.endpoint.oc.tokenMode')">
                    <el-input v-model="rf.token.mode" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.oc.tokenSecret')">
                    <el-input v-model="rf.token.secret" show-password />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.oc.tokenSecretPath')">
                    <el-input v-model="rf.token.secret_path" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.oc.tokenPin')">
                    <el-input v-model="rf.token.pin" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.oc.tokenPassword')">
                    <el-input v-model="rf.token.password" show-password />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.oc.tokenDeviceId')">
                    <el-input v-model="rf.token.device_id" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.oc.tokenCounter')">
                    <el-input-number v-model="rf.token.counter" :min="0" />
                  </el-form-item>
                </el-collapse-item>

                <el-collapse-item :title="t('singbox.protocol.endpoint.oc.mobile') || 'Mobile Device'">
                  <el-form-item :label="t('singbox.protocol.endpoint.oc.mobilePlatformVersion')">
                    <el-input v-model="rf.mobile_platform_version" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.oc.mobileDeviceType')">
                    <el-input v-model="rf.mobile_device_type" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.oc.mobileDeviceUniqueId')">
                    <el-input v-model="rf.mobile_device_unique_id" />
                  </el-form-item>
                </el-collapse-item>

                <el-collapse-item :title="t('singbox.protocol.endpoint.oc.advancedOptions') || 'Advanced Options'">
                  <el-form-item :label="t('singbox.protocol.endpoint.oc.noUDP')">
                    <el-switch v-model="rf.no_udp" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.oc.dtlsLocalPort')">
                    <el-input-number v-model="rf.dtls_local_port" :min="0" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.oc.compressionDisabled')">
                    <el-switch v-model="rf.compression_disabled" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.oc.compressionMode')">
                    <el-input v-model="rf.compression_mode" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.oc.ipv6Disabled')">
                    <el-switch v-model="rf.ipv6_disabled" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.oc.httpKeepaliveDisabled')">
                    <el-switch v-model="rf.http_keepalive_disabled" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.oc.xmlPostDisabled')">
                    <el-switch v-model="rf.xml_post_disabled" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.oc.externalAuthDisabled')">
                    <el-switch v-model="rf.external_auth_disabled" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.oc.passwordAuthDisabled')">
                    <el-switch v-model="rf.password_authentication_disabled" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.oc.tcpKeepAliveEnabled')">
                    <el-switch v-model="rf.tcp_keep_alive_enabled" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.oc.pfs')">
                    <el-switch v-model="rf.pfs" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.oc.mtu')">
                    <el-input-number v-model="rf.mtu" :min="0" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.oc.baseMtu')">
                    <el-input-number v-model="rf.base_mtu" :min="0" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.oc.dpdInterval')">
                    <el-input v-model="rf.dpd_interval" placeholder="300s" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.oc.reconnectTimeout')">
                    <el-input v-model="rf.reconnect_timeout" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.oc.trojanInterval')">
                    <el-input v-model="rf.trojan_interval" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.oc.queueLength')">
                    <el-input-number v-model="rf.queue_length" :min="0" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.oc.allowInsecureCrypto')">
                    <el-switch v-model="rf.allow_insecure_crypto" />
                  </el-form-item>
                </el-collapse-item>

                <el-collapse-item :title="t('singbox.protocol.endpoint.oc.tls') || 'TLS Settings'">
                  <el-form-item :label="t('singbox.protocol.endpoint.oc.tlsInsecure')">
                    <el-switch v-model="rf.tls_insecure" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.oc.tlsServerName')">
                    <el-input v-model="rf.tls_server_name" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.oc.tlsPeerFingerprint')">
                    <el-input v-model="rf.tls_peer_fingerprint" placeholder="xx:xx:xx:..." />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.oc.tlsSystemTrustDisabled')">
                    <el-switch v-model="rf.tls_system_trust_disabled" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.oc.tlsCert')">
                    <el-input v-model="rf.tls_certificate_authority" type="textarea" :rows="3" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.oc.tlsCertPath')">
                    <el-input v-model="rf.tls_certificate_authority_path" />
                  </el-form-item>
                </el-collapse-item>
              </el-collapse>
            </template>

            <!-- ===== OpenVPN Client ===== -->
            <template v-if="rf.type === 'openvpn-client'">
              <el-divider content-position="left">{{ t('singbox.protocol.endpoint.oc.serverConfig') }}</el-divider>
              <el-form-item :label="t('singbox.protocol.endpoint.ovpn.server')">
                <el-input v-model="rf.server" :placeholder="t('singbox.protocol.endpoint.ovpn.serverPlaceholder')" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ovpn.serverPort')">
                <el-input-number v-model="rf.server_port" :min="1" :max="65535" style="width:100%" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ovpn.username')">
                <el-input v-model="rf.username" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ovpn.password')">
                <el-input v-model="rf.password" show-password />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ovpn.mode')">
                <el-select v-model="rf.mode" style="width:100%">
                  <el-option label="TLS" value="tls" />
                  <el-option label="Mux" value="mux" />
                </el-select>
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ovpn.network')">
                <el-select v-model="rf.network" style="width:100%">
                  <el-option label="UDP" value="udp" />
                  <el-option label="TCP" value="tcp" />
                </el-select>
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ovpn.topology')">
                <el-select v-model="rf.topology" style="width:100%" allow-create>
                  <el-option label="net30" value="net30" />
                  <el-option label="p2p" value="p2p" />
                  <el-option label="subnet" value="subnet" />
                </el-select>
              </el-form-item>

              <el-collapse>
                <el-collapse-item :title="t('singbox.protocol.endpoint.ovpn.advancedOptions') || 'Advanced'">
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.address')">
                    <el-input v-model="rf._addressStr" placeholder="10.0.0.1/24" @change="onAddressChange" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.peerAddress')">
                    <el-input v-model="rf.peer_address" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.peerAddressIPv6')">
                    <el-input v-model="rf.peer_address_ipv6" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.cipher')">
                    <el-input v-model="rf.cipher" placeholder="AES-256-GCM" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.dataCiphers')">
                    <el-input v-model="rf._dataCiphersStr" placeholder="AES-256-GCM:AES-128-GCM" @change="onDataCiphersChange" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.dataCiphersFallback')">
                    <el-input v-model="rf.data_ciphers_fallback" placeholder="AES-256-CBC" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.auth')">
                    <el-input v-model="rf.auth" placeholder="SHA256" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.authRetry')">
                    <el-select v-model="rf.auth_retry" style="width:100%">
                      <el-option :label="t('singbox.protocol.endpoint.ovpn.authRetryNone')" value="none" />
                      <el-option :label="t('singbox.protocol.endpoint.ovpn.authRetryInteracted')" value="interacted" />
                      <el-option :label="t('singbox.protocol.endpoint.ovpn.authRetryUninteracted')" value="uninteracted" />
                    </el-select>
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.staticChallenge')">
                    <el-input v-model="rf.static_challenge" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.staticChallengeEcho')">
                    <el-switch v-model="rf.static_challenge_echo" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.staticKeyPath')">
                    <el-input v-model="rf.static_key_path" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.keyDirection')">
                    <el-input v-model="rf.key_direction" placeholder="0 or 1" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.mssFix')">
                    <el-input-number v-model="rf.mss_fix" :min="0" style="width:100%" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.mssFixDisabled')">
                    <el-switch v-model="rf.mss_fix_disabled" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.mssFixMode')">
                    <el-input v-model="rf.mss_fix_mode" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.fragment')">
                    <el-input-number v-model="rf.fragment" :min="0" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.replayWindow')">
                    <el-input-number v-model="rf.replay_window" :min="0" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.replayWindowTime')">
                    <el-input v-model="rf.replay_window_time" placeholder="15s" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.compression')">
                    <el-input v-model="rf.compression" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.compressionLzo')">
                    <el-input v-model="rf.compression_lzo" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.allowCompression')">
                    <el-select v-model="rf.allow_compression" style="width:100%">
                      <el-option label="no" value="no" />
                      <el-option label="yes" value="yes" />
                      <el-option label="asym" value="asym" />
                    </el-select>
                  </el-form-item>
                </el-collapse-item>

                <el-collapse-item :title="t('singbox.protocol.endpoint.ovpn.routeSettings') || 'Routes'">
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.routeNoPull')">
                    <el-switch v-model="rf.route_no_pull" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.routes')">
                    <el-input v-model="rf._routesStr" placeholder="10.0.0.0/24, 192.168.0.0/16" @change="onOvpnRoutesChange" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.routeGateway')">
                    <el-input v-model="rf.route_gateway" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.routeMetric')">
                    <el-input-number v-model="rf.route_metric" :min="0" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.redirectGateway')">
                    <el-switch v-model="rf.redirect_gateway" />
                  </el-form-item>
                </el-collapse-item>
              </el-collapse>
              <el-form-item :label="t('singbox.protocol.endpoint.name')">
                <el-input v-model="rf.name" />
              </el-form-item>
            </template>

            <!-- ===== OpenVPN Server ===== -->
            <template v-if="rf.type === 'openvpn-server'">
              <el-divider content-position="left">{{ t('singbox.protocol.endpoint.ovpn.listenConfig') }}</el-divider>
              <el-form-item :label="t('singbox.protocol.endpoint.ovpn.listen')">
                <el-input v-model="rf.listen" :placeholder="t('singbox.protocol.endpoint.ovpn.listenPlaceholder')" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.listenPort')">
                <el-input-number v-model="rf.listen_port" :min="1" :max="65535" style="width:100%" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ovpn.mode')">
                <el-select v-model="rf.mode" style="width:100%">
                  <el-option label="TLS" value="tls" />
                  <el-option label="Mux" value="mux" />
                </el-select>
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ovpn.network')">
                <el-select v-model="rf.network" style="width:100%">
                  <el-option label="UDP" value="udp" />
                  <el-option label="TCP" value="tcp" />
                </el-select>
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ovpn.topology')">
                <el-select v-model="rf.topology" style="width:100%">
                  <el-option label="subnet" value="subnet" />
                  <el-option label="net30" value="net30" />
                  <el-option label="p2p" value="p2p" />
                </el-select>
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ovpn.mtu')">
                <el-input-number v-model="rf.mtu" :min="0" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.address')">
                <el-input v-model="rf._addressStr" placeholder="10.0.0.1/24" @change="onAddressChange" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ovpn.peerAddress')">
                <el-input v-model="rf.peer_address" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ovpn.peerAddressIPv6')">
                <el-input v-model="rf.peer_address_ipv6" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ovpn.maxClients')">
                <el-input-number v-model="rf.max_clients" :min="1" style="width:100%" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ovpn.duplicateCN')">
                <el-switch v-model="rf.duplicate_cn" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ovpn.cipher')">
                <el-input v-model="rf.cipher" placeholder="AES-256-GCM" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ovpn.dataCiphers')">
                <el-input v-model="rf._dataCiphersStr" @change="onDataCiphersChange" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ovpn.dataCiphersFallback')">
                <el-input v-model="rf.data_ciphers_fallback" placeholder="AES-256-CBC" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ovpn.auth')">
                <el-input v-model="rf.auth" placeholder="SHA256" />
              </el-form-item>

              <el-collapse>
                <el-collapse-item :title="t('singbox.protocol.endpoint.ovpn.advancedOptions') || 'Advanced'">
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.remote')">
                    <el-input v-model="rf.remote" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.remotePort')">
                    <el-input-number v-model="rf.remote_port" :min="0" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.staticKeyPath')">
                    <el-input v-model="rf.static_key_path" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.keyDirection')">
                    <el-input v-model="rf.key_direction" placeholder="0 or 1" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.mssFix')">
                    <el-input-number v-model="rf.mss_fix" :min="0" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.mssFixDisabled')">
                    <el-switch v-model="rf.mss_fix_disabled" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.mssFixMode')">
                    <el-input v-model="rf.mss_fix_mode" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.replayWindow')">
                    <el-input-number v-model="rf.replay_window" :min="0" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.replayWindowTime')">
                    <el-input v-model="rf.replay_window_time" />
                  </el-form-item>
                </el-collapse-item>

                <el-collapse-item :title="t('singbox.protocol.endpoint.ovpn.push') || 'Push Settings'">
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.pushRoutes')">
                    <el-input v-model="rf._pushRoutesStr" @change="onPushRoutesChange" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.pushDns')">
                    <el-input v-model="rf._pushDnsStr" @change="onPushDnsChange" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.pushDnsServers')">
                    <el-input v-model="rf._pushDnsServersStr" @change="onPushDnsServersChange" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.pushSearchDomains')">
                    <el-input v-model="rf._pushSearchDomainsStr" @change="onPushSearchDomainsChange" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.pushRedirectGateway')">
                    <el-switch v-model="rf.push_redirect_gateway" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.pushBlockOutsideDns')">
                    <el-switch v-model="rf.push_block_outside_dns" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.pushPingInterval')">
                    <el-input v-model="rf.push_ping_interval" placeholder="60s" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.pushPingRestart')">
                    <el-input v-model="rf.push_ping_restart" placeholder="300s" />
                  </el-form-item>
                </el-collapse-item>

                <el-collapse-item :title="t('singbox.protocol.endpoint.ovpn.timing') || 'Timing'">
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.pingInterval')">
                    <el-input v-model="rf.ping_interval" placeholder="60s" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.pingRestart')">
                    <el-input v-model="rf.ping_restart" placeholder="300s" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.renegotiateInterval')">
                    <el-input v-model="rf.renegotiate_interval" placeholder="3600s" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.renegotiateDisabled')">
                    <el-switch v-model="rf.renegotiate_disabled" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.renegotiateBytes')">
                    <el-input-number v-model="rf.renegotiate_bytes" :min="0" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.renegotiatePackets')">
                    <el-input-number v-model="rf.renegotiate_packets" :min="0" />
                  </el-form-item>
                  <el-form-item :label="t('singbox.protocol.endpoint.ovpn.handshakeWindow')">
                    <el-input v-model="rf.handshake_window" placeholder="1m" />
                  </el-form-item>
                </el-collapse-item>
              </el-collapse>
              <el-form-item :label="t('singbox.protocol.endpoint.name')">
                <el-input v-model="rf.name" />
              </el-form-item>
            </template>
          </el-form>
        </el-tab-pane>

        <!-- ========== Key Tab - WireGuard only ========== -->
        <el-tab-pane v-if="rf.type === 'wireguard'" :label="t('singbox.protocol.endpoint.tabs.key')" name="key">
          <el-form label-width="150px" label-position="left">
            <el-form-item :label="t('singbox.protocol.wireguard.privateKey')">
              <div style="display:flex;gap:8px;flex:1">
                <el-input v-model="rf.private_key" @change="onPrivateKeyChange" />
                <el-button size="small" @click="genWgKeyPair">{{ t('singbox.users.generate') }}</el-button>
              </div>
            </el-form-item>
            <el-form-item :label="t('singbox.protocol.wireguard.publicKey')">
              <el-input v-model="rf.public_key" readonly />
              <div class="hint">{{ t('singbox.protocol.wireguard.publicKeyHint') }}</div>
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <!-- ========== Peers Tab - WireGuard only ========== -->
        <el-tab-pane v-if="rf.type === 'wireguard'" :label="t('singbox.protocol.endpoint.tabs.peers')" name="peers">
          <div v-for="(p, pi) in rf.peers" :key="pi" class="peer-item">
            <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:8px">
              <span style="font-weight:600">Peer #{{ pi + 1 }}</span>
              <el-button size="small" type="danger" text @click="rf.peers.splice(pi,1)">{{ t('common.delete') }}</el-button>
            </div>
            <el-form label-width="130px" label-position="left" size="small">
              <el-form-item :label="t('singbox.protocol.wireguard.peerPublicKey')">
                <el-input v-model="p.public_key" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.wireguard.peerAddress') || 'Address'">
                <el-input v-model="p.address" placeholder="127.0.0.1" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.listenPort')">
                <el-input-number v-model="p.port" :min="0" :max="65535" style="width:100%" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.wireguard.allowedIPs')">
                <el-input v-model="p._allowedIPsStr" :placeholder="'0.0.0.0/0, ::/0'" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.wireguard.preSharedKey') || 'Pre-shared Key'">
                <el-input v-model="p.pre_shared_key" show-password />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.wireguard.keepAlive')">
                <el-input-number v-model="p.persistent_keepalive_interval" :min="0" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.wireguard.reserved') || 'Reserved'">
                <el-input v-model="p.reserved" placeholder="0, 0, 0" />
              </el-form-item>
            </el-form>
          </div>
          <el-button size="small" @click="addPeer">+ {{ t('singbox.protocol.wireguard.addPeer') }}</el-button>
        </el-tab-pane>

        <!-- ========== TLS Tab - OpenVPN Client ========== -->
        <el-tab-pane v-if="rf.type === 'openvpn-client'" :label="t('singbox.protocol.endpoint.oc.tls')" name="ovpn-tls">
          <el-form label-width="160px" label-position="left" size="small">
            <el-form-item :label="t('singbox.protocol.endpoint.ovpn.tlsServerName')">
              <el-input v-model="rf.tls_server_name" />
            </el-form-item>
            <el-form-item :label="t('singbox.protocol.endpoint.ovpn.tlsServerNameType')">
              <el-select v-model="rf.tls_server_name_type" style="width:100%">
                <el-option label="name" value="name" />
                <el-option label="ip" value="ip" />
                <el-option label="srv" value="srv" />
              </el-select>
            </el-form-item>
            <el-form-item :label="t('singbox.protocol.endpoint.ovpn.tlsCert')">
              <el-input v-model="rf.tls_certificate" type="textarea" :rows="3" />
            </el-form-item>
            <el-form-item :label="t('singbox.protocol.endpoint.ovpn.tlsCertPath')">
              <el-input v-model="rf.tls_certificate_path" />
            </el-form-item>
            <el-form-item :label="t('singbox.protocol.endpoint.ovpn.tlsClientCert')">
              <el-input v-model="rf.tls_client_certificate" type="textarea" :rows="3" />
            </el-form-item>
            <el-form-item :label="t('singbox.protocol.endpoint.ovpn.tlsClientCertPath')">
              <el-input v-model="rf.tls_client_certificate_path" />
            </el-form-item>
            <el-form-item :label="t('singbox.protocol.endpoint.ovpn.tlsClientKey')">
              <el-input v-model="rf.tls_client_key" show-password />
            </el-form-item>
            <el-form-item :label="t('singbox.protocol.endpoint.ovpn.tlsClientKeyPath')">
              <el-input v-model="rf.tls_client_key_path" />
            </el-form-item>
            <el-form-item :label="t('singbox.protocol.endpoint.ovpn.tlsPeerFingerprint')">
              <el-input v-model="rf.tls_peer_fingerprint" placeholder="xx:xx:xx:..." />
            </el-form-item>
            <el-form-item :label="t('singbox.protocol.endpoint.ovpn.tlsCrlPath')">
              <el-input v-model="rf.tls_crl_path" />
            </el-form-item>
            <el-form-item :label="t('singbox.protocol.endpoint.ovpn.tlsVersionMin')">
              <el-select v-model="rf.tls_version_min" style="width:100%">
                <el-option label="1.2" value="1.2" />
                <el-option label="1.3" value="1.3" />
              </el-select>
            </el-form-item>
            <el-form-item :label="t('singbox.protocol.endpoint.ovpn.tlsCipher')">
              <el-input v-model="rf.tls_cipher" />
            </el-form-item>
            <el-form-item :label="t('singbox.protocol.endpoint.ovpn.tlsGroups')">
              <el-input v-model="rf.tls_groups" />
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <!-- ========== TLS Tab - OpenVPN Server ========== -->
        <el-tab-pane v-if="rf.type === 'openvpn-server'" :label="t('singbox.protocol.endpoint.oc.tls')" name="ovpn-srv-tls">
          <el-form label-width="170px" label-position="left" size="small">
            <el-form-item :label="t('singbox.protocol.endpoint.ovpn.tlsCert')">
              <el-input v-model="rf.tls_certificate" type="textarea" :rows="3" />
            </el-form-item>
            <el-form-item :label="t('singbox.protocol.endpoint.ovpn.tlsCertPath')">
              <el-input v-model="rf.tls_certificate_path" />
            </el-form-item>
            <el-form-item :label="t('singbox.protocol.endpoint.ovpn.tlsKey') || 'Server Key'">
              <el-input v-model="rf.tls_key" show-password />
            </el-form-item>
            <el-form-item :label="t('singbox.protocol.endpoint.ovpn.tlsKeyPath') || 'Server Key Path'">
              <el-input v-model="rf.tls_key_path" />
            </el-form-item>
            <el-form-item :label="t('singbox.protocol.endpoint.ovpn.tlsClientCert')">
              <el-input v-model="rf.tls_client_certificate" type="textarea" :rows="3" />
            </el-form-item>
            <el-form-item :label="t('singbox.protocol.endpoint.ovpn.tlsClientCertPath')">
              <el-input v-model="rf.tls_client_certificate_path" />
            </el-form-item>
            <el-form-item :label="t('singbox.protocol.endpoint.ovpn.verifyClientCert')">
              <el-select v-model="rf.tls_verify_client_certificate" style="width:100%">
                <el-option label="none" value="none" />
                <el-option label="request" value="request" />
                <el-option label="require" value="require" />
              </el-select>
            </el-form-item>
            <el-form-item :label="t('singbox.protocol.endpoint.ovpn.clientName')">
              <el-input v-model="rf.tls_client_name" />
            </el-form-item>
            <el-form-item :label="t('singbox.protocol.endpoint.ovpn.tlsPeerFingerprint')">
              <el-input v-model="rf.tls_peer_fingerprint" />
            </el-form-item>
            <el-form-item :label="t('singbox.protocol.endpoint.ovpn.tlsVersionMin')">
              <el-select v-model="rf.tls_version_min" style="width:100%">
                <el-option label="1.2" value="1.2" />
                <el-option label="1.3" value="1.3" />
              </el-select>
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <!-- ========== Users Tab - OpenVPN Server ========== -->
        <el-tab-pane v-if="rf.type === 'openvpn-server'" :label="t('singbox.protocol.endpoint.ovpn.users')" name="users">
          <div v-for="(u, ui) in rf.users" :key="ui" class="peer-item">
            <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:8px">
              <span style="font-weight:600">{{ t('singbox.protocol.endpoint.ovpn.user') }} #{{ ui + 1 }}</span>
              <el-button size="small" type="danger" text @click="rf.users.splice(ui,1)">{{ t('common.delete') }}</el-button>
            </div>
            <el-form label-width="100px" label-position="left" size="small">
              <el-form-item :label="t('singbox.protocol.endpoint.ovpn.username')">
                <el-input v-model="u.username" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ovpn.password')">
                <el-input v-model="u.password" show-password />
              </el-form-item>
            </el-form>
          </div>
          <el-button size="small" @click="addOvpnUser">+ {{ t('singbox.protocol.endpoint.ovpn.addUser') }}</el-button>
        </el-tab-pane>

        <!-- ========== Servers Tab - OpenVPN Client ========== -->
        <el-tab-pane v-if="rf.type === 'openvpn-client'" :label="t('singbox.protocol.endpoint.ovpn.servers')" name="ovpn-servers">
          <div class="hint" style="margin-bottom:12px">{{ t('singbox.protocol.endpoint.ovpn.serversHint') }}</div>
          <el-form-item :label="t('singbox.protocol.endpoint.ovpn.remoteRandom')">
            <el-switch v-model="rf.remote_random" />
          </el-form-item>
          <div v-for="(s, si) in rf.servers" :key="si" class="peer-item">
            <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:8px">
              <span style="font-weight:600">Server #{{ si + 1 }}</span>
              <el-button size="small" type="danger" text @click="rf.servers.splice(si,1)">{{ t('common.delete') }}</el-button>
            </div>
            <el-form label-width="110px" label-position="left" size="small">
              <el-form-item :label="t('singbox.protocol.endpoint.ovpn.serverAddress')">
                <el-input v-model="s.server" placeholder="vpn.example.com" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ovpn.serverPortLabel')">
                <el-input-number v-model="s.server_port" :min="1" :max="65535" style="width:100%" />
              </el-form-item>
              <el-form-item :label="t('singbox.protocol.endpoint.ovpn.serverNetwork')">
                <el-select v-model="s.network" style="width:100%">
                  <el-option label="UDP" value="udp" />
                  <el-option label="TCP" value="tcp" />
                </el-select>
              </el-form-item>
            </el-form>
          </div>
          <el-button size="small" @click="addOvpnServer">+ {{ t('singbox.protocol.endpoint.ovpn.addServer') }}</el-button>
        </el-tab-pane>
      </el-tabs>
      <template #footer>
        <div style="display:flex;justify-content:space-between;align-items:center;width:100%">
          <el-form-item :label="t('common.enabled')" label-width="auto" style="margin-bottom:0">
            <el-switch v-model="rf._enabled" />
          </el-form-item>
          <div>
            <el-button @click="dlg = false">{{ t('singbox.inbound.cancel') }}</el-button>
            <el-button type="primary" @click="save">{{ t('singbox.inbound.save') }}</el-button>
          </div>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, ref, reactive, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { MoreFilled, Edit, Delete, InfoFilled } from '@element-plus/icons-vue'
import client from '../../../api/client'
import { useClientPagination } from '../../../composables/useClientPagination'
import { useQueryState } from '../../../composables/useQueryState'
import PaginationBar from '../../../components/PaginationBar.vue'
import { buildEndpoint, parseEndpoint, endpointDefaultsForType, ENDPOINT_TYPES, ENDPOINT_TYPE_LABELS } from './singbox/utils/singboxEndpointBuild'
import { prefetchFreePort, takeFreePort } from './utils/freePort'

const props = defineProps({ data: Array, serverId: Number })
const emitFn = defineEmits(['update'])
const { t } = useI18n()

const nodeBase = computed(() => `/node-servers/${props.serverId}`)
const form = reactive([])
const { paged: epPaged, page: epPage, pageSize: epSize, total: epTotal } = useClientPagination(computed(() => form), { pageKey: 'sbEpPage', pageSizeKey: 'sbEpSize' })

const dlg = ref(false)
const editIdx = ref(-1)
const rf = reactive(endpointDefaultsForType('wireguard'))
const activeTab = useQueryState('sbetab', 'basic')

// Reset tab to valid one when type changes
watch(() => rf.type, (newType) => {
  const validTabs = {
    wireguard: ['basic', 'key', 'peers'],
    tailscale: ['basic'],
    openconnect: ['basic'],
    'openvpn-client': ['basic', 'ovpn-tls', 'ovpn-servers'],
    'openvpn-server': ['basic', 'users', 'ovpn-srv-tls'],
  }
  if (!validTabs[newType]?.includes(activeTab.value)) {
    activeTab.value = 'basic'
  }
})

async function showAdd() {
  editIdx.value = -1
  activeTab.value = 'basic'
  // 端口与入站编辑器对齐：取该节点未被占用的端口（含其它入站/端点占用）
  await prefetchFreePort(props.serverId)
  Object.assign(rf, endpointDefaultsForType('wireguard', takeFreePort(props.serverId)))
  dlg.value = true
  // 自动生成 WireGuard 密钥对
  if (rf.type === 'wireguard') {
    try {
      const data = await client.post(`${nodeBase.value}/binaries/wg-key?core_type=singbox`)
      if (data.privateKey) rf.private_key = data.privateKey
      if (data.publicKey) rf.public_key = data.publicKey
    } catch (e) {
      console.error('Generate WG keypair failed:', e)
    }
  }
}

function showEdit(row) {
  editIdx.value = form.indexOf(row)
  activeTab.value = 'basic'
  const parsed = parseEndpoint(JSON.parse(JSON.stringify(row)))
  Object.assign(rf, parsed)
  rf._addressStr = Array.isArray(rf.address) ? rf.address.join(', ') : ''
  dlg.value = true
}

async function onTypeChange() {
  const oldTag = rf.tag
  const oldRemark = rf._remark
  Object.assign(rf, endpointDefaultsForType(rf.type, takeFreePort(props.serverId)))
  rf.tag = oldTag || rf.tag
  rf._remark = oldRemark || ''
  // 切换到 WireGuard 时自动生成密钥对
  if (rf.type === 'wireguard' && !rf.private_key) {
    try {
      const data = await client.post(`${nodeBase.value}/binaries/wg-key?core_type=singbox`)
      if (data.privateKey) rf.private_key = data.privateKey
      if (data.publicKey) rf.public_key = data.publicKey
    } catch (e) {
      console.error('Generate WG keypair failed:', e)
    }
  }
}

function onAddressChange(val) {
  rf.address = (val || '').split(',').map(s => s.trim()).filter(Boolean)
}

function onAdvertiseRoutesChange(val) {
  rf.advertise_routes = (val || '').split(',').map(s => s.trim()).filter(Boolean)
}

function onAdvertiseTagsChange(val) {
  rf.advertise_tags = (val || '').split(',').map(s => s.trim()).filter(Boolean)
}

function relayStaticEndpointsChange(val) {
  rf.relay_server_static_endpoints = (val || '').split(',').map(s => s.trim()).filter(Boolean)
}

function onDataCiphersChange(val) {
  rf.data_ciphers = (val || '').split(',').map(s => s.trim()).filter(Boolean)
}

function onOvpnRoutesChange(val) {
  rf.routes = (val || '').split(',').map(s => s.trim()).filter(Boolean)
}

function onPushRoutesChange(val) {
  rf.push_routes = (val || '').split(',').map(s => s.trim()).filter(Boolean)
}

function onPushDnsChange(val) {
  rf.push_dns = (val || '').split(',').map(s => s.trim()).filter(Boolean)
}

function onPushDnsServersChange(val) {
  rf.push_dns_servers = (val || '').split(',').map(s => s.trim()).filter(Boolean)
}

function onPushSearchDomainsChange(val) {
  rf.push_search_domains = (val || '').split(',').map(s => s.trim()).filter(Boolean)
}

function onRemarkChange(val) {
  const prefix = rf.type === 'tailscale' ? 'ts-' :
                 rf.type === 'openconnect' ? 'oc-' :
                 rf.type === 'openvpn-client' ? 'ovpn-c-' :
                 rf.type === 'openvpn-server' ? 'ovpn-s-' : 'wg-'
  const suffix = val ? val.toLowerCase().replace(/\s+/g, '-').substring(0, 20) : Math.random().toString(36).substring(2, 6)
  if (!rf.tag || rf.tag.match(/^(wg|ts|oc|ovpn)-/)) {
    rf.tag = prefix + suffix
  }
}

async function genWgKeyPair() {
  try {
    const data = await client.post(`${nodeBase.value}/binaries/wg-key?core_type=singbox`)
    rf.private_key = data.privateKey || ''
    rf.public_key = data.publicKey || ''
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || 'Generate failed')
  }
}

async function onPrivateKeyChange(val) {
  if (!val) { rf.public_key = ''; return }
  try {
    const data = await client.post(`${nodeBase.value}/binaries/wg-key?core_type=singbox`, { private_key: val })
    if (data.publicKey) rf.public_key = data.publicKey
  } catch {
    rf.public_key = ''
  }
}

function addPeer() {
  if (!rf.peers) rf.peers = []
  rf.peers.push({
    public_key: '',
    address: '127.0.0.1',
    port: 0,
    _allowedIPsStr: '0.0.0.0/0, ::/0',
    allowed_ips: ['0.0.0.0/0', '::/0'],
    pre_shared_key: '',
    persistent_keepalive_interval: 0,
    reserved: '',
  })
}

function addOvpnUser() {
  if (!rf.users) rf.users = []
  rf.users.push({ username: '', password: '' })
}

function addOvpnServer() {
  if (!rf.servers) rf.servers = []
  rf.servers.push({ server: '127.0.0.1', server_port: 1194, network: 'udp' })
}

async function save() {
  if (!rf.tag) { ElMessage.warning(t('singbox.inbound.tagRequired')); return }
  const cfg = buildEndpoint(rf)
  const payload = {
    core_type: 'singbox',
    endpoint_type: rf.type,
    remark: rf._remark || rf.tag || '',
    tag: rf.tag,
    enabled: rf._enabled !== false,
    config_json: cfg,
  }
  try {
    if (editIdx.value >= 0 && form[editIdx.value]?._id) {
      await client.put(`${nodeBase.value}/endpoint-configs/${form[editIdx.value]._id}`, payload)
    } else {
      await client.post(`${nodeBase.value}/endpoint-configs`, payload)
    }
    await restartCore()
    ElMessage.success(t('common.success'))
    dlg.value = false
    await loadEndpoints()
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || t('common.failed'))
  }
}

function handleAction(cmd, row) {
  if (cmd === 'edit') showEdit(row)
  else if (cmd === 'delete') {
    const i = form.indexOf(row)
    const ep = form[i]
    if (!ep) return
    ElMessageBox.confirm(t('common.deleteConfirm'), t('common.confirm'), { type: 'warning' })
      .then(async () => {
        try {
          if (ep._id) {
            await client.delete(`${nodeBase.value}/endpoint-configs/${ep._id}`)
            await restartCore()
          }
          form.splice(i, 1)
          ElMessage.success(t('common.deleteSuccess'))
        } catch (e) {
          ElMessage.error(e?.response?.data?.message || t('common.failed'))
        }
      })
      .catch(() => {})
  }
}

async function toggleEnabled(row) {
  const i = form.indexOf(row)
  if (i < 0) return
  const ep = form[i]
  if (!ep._id) return
  try {
    const cfg = buildEndpoint(ep)
    await client.put(`${nodeBase.value}/endpoint-configs/${ep._id}`, {
      core_type: 'singbox',
      endpoint_type: ep.type,
      remark: ep._remark || ep.tag || '',
      tag: ep.tag,
      enabled: ep._enabled !== false,
      config_json: cfg,
    })
    await restartCore()
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || t('common.failed'))
    ep._enabled = !ep._enabled
  }
}

async function restartCore() {
  try {
    await client.post(`${nodeBase.value}/control`, { core_type: 'singbox', action: 'restart' })
  } catch (e) {
    console.error('restart failed:', e)
  }
}

async function loadEndpoints() {
  let apiItems = []
  const loaded = new Set()
  try {
    const res = await client.get(`${nodeBase.value}/endpoint-configs`, { params: { core_type: 'singbox' } })
    apiItems = res?.items || (Array.isArray(res) ? res : [])
  } catch {
    apiItems = []
  }

  form.splice(0, form.length)

  for (const ep of apiItems) {
    let cfg
    try { cfg = JSON.parse(ep.config_json) } catch { cfg = ep.config_json || {} }
    const parsed = parseEndpoint(cfg)
    parsed._id = ep.id
    parsed._remark = ep.remark || parsed._remark || ''
    parsed._enabled = ep.enabled !== false
    if (parsed.tag) loaded.add(parsed.tag)
    form.push(parsed)
  }

  if (!apiItems.length && Array.isArray(props.data)) {
    for (const ep of props.data) {
      if (ENDPOINT_TYPES.includes(ep.type)) {
        const parsed = parseEndpoint(JSON.parse(JSON.stringify(ep)))
        if (parsed.tag && !loaded.has(parsed.tag)) {
          parsed._id = null
          parsed._enabled = parsed._enabled !== false
          form.push(parsed)
          loaded.add(parsed.tag)
        }
      }
    }
  }
  emitFn('update', form.map(r => buildEndpoint(r)))
}

onMounted(() => {
  loadEndpoints()
  prefetchFreePort(props.serverId) // 预热空闲端口，新建端点时不再撞端口
})

defineExpose({ loadEndpoints })
</script>

<style scoped>
.peer-item {
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 4px;
  padding: 8px;
  margin-bottom: 8px;
}
.hint {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 4px;
}
</style>
