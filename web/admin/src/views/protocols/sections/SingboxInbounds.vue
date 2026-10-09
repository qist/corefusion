<template>
  <div>
    <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:16px">
      <span style="font-size:15px;font-weight:600">{{ t('singbox.inbound.title') }}</span>
      <div style="display:flex;gap:8px">
        <el-button size="small" type="primary" @click="showAdd">{{ t('singbox.inbound.add') }}</el-button>
      </div>
    </div>

    <!-- Stats Summary (对齐 xray 设计) -->
    <div style="display:flex;gap:24px;margin-bottom:16px;padding:12px 16px;background:var(--el-fill-color-light);border-radius:8px;font-size:13px;align-items:center;flex-wrap:wrap">
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
      <el-button size="small" @click="sync" :loading="false">
        <el-icon><Refresh /></el-icon> {{ t('common.refresh') }}
      </el-button>
    </div>

    <!-- Table List -->
    <el-table v-if="!isMobile" :data="inbPaged" stripe style="width:100%" size="small">
      <el-table-column label="#" width="50" align="right">
        <template #default="{ $index }">
          <span style="font-size:11px;opacity:0.6">{{ $index + 1 }}</span>
        </template>
      </el-table-column>
      <el-table-column :label="t('common.actions')" width="70" align="center">
        <template #default="{ row }">
          <el-dropdown trigger="click" @command="(cmd) => handleAction(cmd, row)">
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
                <el-dropdown-item v-if="USER_PROTOCOLS.includes(row.type)" command="add-user" divided>
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
        <template #default="{ row }">
          <el-switch v-model="row._enabled" size="small" @change="toggleEnabled(row)" />
        </template>
      </el-table-column>
      <el-table-column :label="t('protoCenter.inbounds.remark')" min-width="100" show-overflow-tooltip>
        <template #default="{row}"><span>{{ row._remark || row.tag || '-' }}</span></template>
      </el-table-column>
      <el-table-column :label="t('singbox.inbound.tag')" min-width="100" show-overflow-tooltip>
        <template #default="{row}"><span>{{ row.tag || '-' }}</span></template>
      </el-table-column>
      <el-table-column :label="t('singbox.inbound.port')" width="70" align="center">
        <template #default="{row}"><span style="font-weight:500">{{ row.listen_port || '-' }}</span></template>
      </el-table-column>
      <el-table-column :label="t('singbox.inbound.protocol')" min-width="170" align="left">
        <template #default="{row}">
          <div style="display:inline-flex;flex-wrap:wrap;gap:4px">
            <el-tag size="small" type="primary">{{ (row.type || '').toUpperCase() }}</el-tag>
            <el-tag v-if="row._transportType" size="small" type="success">{{ row._transportType }}</el-tag>
            <el-tag v-if="row._tlsEnabled" size="small" type="warning">TLS</el-tag>
            <el-tag v-if="row._realityEnabled" size="small" type="danger">Reality</el-tag>
          </div>
        </template>
      </el-table-column>
      <el-table-column :label="t('singbox.inbound.users')" width="120" align="left">
        <template #default="{row}">
          <div style="display:flex;gap:6px;align-items:center">
            <el-tag size="small">
              <el-icon><User /></el-icon> {{ getUserCount(row) }}
            </el-tag>
            <el-tag v-if="getActiveUserCount(row) > 0" size="small" type="success">{{ getActiveUserCount(row) }}</el-tag>
            <el-tag v-if="getExpiredUserCount(row) > 0" size="small" type="danger">{{ getExpiredUserCount(row) }}</el-tag>
          </div>
        </template>
      </el-table-column>
      <el-table-column :label="t('protoCenter.inbounds.traffic')" width="120" align="center">
        <template #default="{row}">
          <el-popover trigger="hover" width="180">
            <template #reference>
              <el-tag :type="getTrafficColor(row)" style="cursor:pointer;min-width:80px">
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
          <el-tag :type="getExpiryColor(row)" size="small" style="min-width:50px">
            {{ getExpiryDisplay(row) }}
          </el-tag>
        </template>
      </el-table-column>
    </el-table>
    <MobileCards v-else :rows="inbPaged" title-field="tag" :fields="sbInboundCardFields">
      <template #actions="{ row }">
        <el-switch v-model="row._enabled" size="small" @change="toggleEnabled(row)" />
        <el-dropdown trigger="click" @command="(cmd) => handleAction(cmd, row)">
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
              <el-dropdown-item v-if="USER_PROTOCOLS.includes(row.type)" command="add-user" divided>
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
    </MobileCards>
    <PaginationBar v-model:page="inbPage" v-model:page-size="inbSize" :total="inbTotal" />
    <div v-if="!form.length" style="text-align:center;color:#909399;padding:24px">{{ t('singbox.inbound.noInbounds') }}</div>

    <!-- Edit Dialog: 对齐 xray 弹窗模型（基础/协议/传输/TLS/Multiplex 统一 tabs） -->
    <el-dialog v-model="dlg" :title="editIdx >= 0 ? t('singbox.inbound.edit') : t('singbox.inbound.add')" width="760" :close-on-click-modal="false" top="5vh">
      <el-tabs v-model="activeTab">
        <!-- Basic Tab: 备注 + 开关 + 类型 + Tag + Listen + Port + 域名策略 + Sniff + 访问域名 -->
        <el-tab-pane :label="t('singbox.inbound.tabs.basic')" name="basic">
          <el-form label-width="140px" label-position="left">
            <el-form-item :label="t('protoCenter.inbounds.remark')">
              <el-input v-model="rf._remark" />
            </el-form-item>
            <el-form-item :label="t('singbox.inbound.type')">
              <el-select v-model="rf.type" style="width:100%" @change="onTypeChange">
                <el-option v-for="tp in INBOUND_TYPES" :key="tp" :label="tp" :value="tp" />
              </el-select>
            </el-form-item>
            <el-form-item :label="t('singbox.listen.tag')">
              <el-input v-model="rf.tag" :placeholder="t('singbox.listen.tagPlaceholder')" />
            </el-form-item>
            <el-form-item :label="t('singbox.listen.listen')">
              <el-input v-model="rf.listen" :placeholder="t('singbox.listen.listenPlaceholder')" />
            </el-form-item>
            <el-form-item :label="t('singbox.listen.listenPort')">
              <el-input-number v-model="rf.listen_port" :min="0" :max="65535" :disabled="rf.type==='tun' || rf.type==='cloudflared'" style="width:100%" />
            </el-form-item>
            <el-form-item :label="t('singbox.listen.domainStrategy')">
              <el-select v-model="rf.domain_strategy" style="width:100%" clearable>
                <el-option label="" value="" />
                <el-option label="prefer_ipv4" value="prefer_ipv4" />
                <el-option label="prefer_ipv6" value="prefer_ipv6" />
                <el-option label="ipv4_only" value="ipv4_only" />
                <el-option label="ipv6_only" value="ipv6_only" />
              </el-select>
            </el-form-item>
            <el-form-item :label="t('singbox.listen.sniff')">
              <el-switch v-model="rf.sniff" />
            </el-form-item>
            <el-form-item v-if="rf.sniff" :label="t('singbox.listen.sniffOverride')">
              <el-switch v-model="rf.sniff_override_destination" />
            </el-form-item>

            <!-- direct inbound：静态转发目标（sing-box inbound/direct） -->
            <template v-if="rf.type === 'direct'">
              <el-form-item label="Override Address">
                <el-input v-model="rf.override_address" placeholder="1.2.3.4" />
                <div class="hint">把命中该入站的流量转发到该地址</div>
              </el-form-item>
              <el-form-item label="Override Port">
                <el-input-number v-model="rf.override_port" :min="0" :max="65535" style="width:100%" />
              </el-form-item>
            </template>

            <!-- 通用监听字段（sing-box Listen Fields） -->
            <!-- Port Hopping（端口跳跃）：sing-box 入站不支持 server_ports（那是出站
                 字段），服务端靠防火墙把端口段 DNAT 到入站端口（与 xray 一致）；
                 客户端跳跃范围通过分享链接的 mport 下发。保存时自动增删规则。 -->
            <template v-if="rf.type === 'hysteria2' || rf.type === 'hysteria'">
              <el-divider content-position="left">Port Hopping</el-divider>
              <el-form-item label="Enable">
                <el-switch v-model="rf._portHopEnabled" />
                <span class="hint" style="margin-left:8px">端口段转发到本入站端口（保存时自动增删防火墙规则）</span>
              </el-form-item>
              <template v-if="rf._portHopEnabled">
                <el-form-item label="Target Port">
                  <el-input-number v-model="rf.listen_port" :min="1" :max="65535" disabled />
                  <div class="hint">当前入站监听端口（端口段会转发到这里）</div>
                </el-form-item>
                <el-form-item label="Range Mode">
                  <el-select v-model="rf._portHopMode" style="width:100%">
                    <el-option label="Preset" value="preset" />
                    <el-option label="Manual" value="manual" />
                  </el-select>
                </el-form-item>
                <el-form-item v-if="rf._portHopMode === 'preset'" label="Port Range">
                  <el-select v-model="rf._portHopRange" style="width:100%">
                    <!-- 预设段互不重叠；已被本节点其它入站占用的段直接标出来（端口段是节点级资源） -->
                    <el-option v-for="r in portHopRangeOptions" :key="r.value" :label="r.label" :value="r.value">
                      <span>{{ r.label }}</span>
                      <el-tag v-if="r.conflict" size="small" type="warning" style="margin-left:8px">
                        已被 {{ r.conflict.port_range }} → {{ r.conflict.target_port }} 占用
                      </el-tag>
                    </el-option>
                  </el-select>
                  <div v-if="portHopUsageHint" class="hint">{{ portHopUsageHint }}</div>
                </el-form-item>
                <el-form-item v-else label="Port Range">
                  <div style="display:flex;gap:8px;align-items:center">
                    <el-input-number v-model="rf._portHopStart" :min="1" :max="65535" style="width:120px" />
                    <span>-</span>
                    <el-input-number v-model="rf._portHopEnd" :min="1" :max="65535" style="width:120px" />
                  </div>
                </el-form-item>
                <el-form-item label="Interface">
                  <div style="display:flex;gap:8px;width:100%">
                    <el-select v-model="rf._portHopInterface" style="flex:1" clearable filterable allow-create placeholder="All interfaces">
                      <el-option v-for="iface in ifaceList" :key="iface" :label="iface" :value="iface" />
                    </el-select>
                    <el-button size="small" @click="loadInterfaces">Refresh</el-button>
                  </div>
                  <div class="hint">留空表示所有网卡</div>
                </el-form-item>
              </template>
            </template>
            <el-divider content-position="left">Advanced Listen</el-divider>
            <el-form-item label="Detour">
              <el-input v-model="rf.detour" placeholder="direct" />
              <div class="hint">inbound 级 detour（把该入站流量交给指定 outbound）</div>
            </el-form-item>
            <el-row :gutter="12">
              <el-col :span="8"><el-form-item label="TCP Fast Open"><el-switch v-model="rf.tcp_fast_open" /></el-form-item></el-col>
              <el-col :span="8"><el-form-item label="TCP Multi Path"><el-switch v-model="rf.tcp_multi_path" /></el-form-item></el-col>
              <el-col :span="8"><el-form-item label="UDP Fragment"><el-switch v-model="rf.udp_fragment" /></el-form-item></el-col>
            </el-row>
            <el-form-item label="UDP Timeout">
              <el-input v-model="rf.udp_timeout" placeholder="5m" />
              <div class="hint">UDP NAT 超时，如 5m / 300s</div>
            </el-form-item>
          </el-form>

          <!-- Access Domain (对齐 xray) -->
          <el-divider content-position="left">{{ t('protoCenter.inbounds.accessDomain') }}</el-divider>
          <el-form label-width="140px" label-position="left">
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
              <div v-if="!accessDomainSelected" class="hint warn-hint" style="margin-bottom:12px">
                {{ t('protoCenter.inbounds.accessDomainNotSelected') }}
              </div>
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

        <!-- Protocol: 协议专属设置（仅当协议有专属参数时渲染） -->
        <el-tab-pane v-if="hasProtoTab(rf.type)" :label="t('singbox.inbound.tabs.protocol')" name="protocol">
          <component
            :is="getProtocolComp(rf.type)"
            :key="rf.type"
            :model="rf"
            :server-id="serverId"
            @update="noop"
          />
        </el-tab-pane>

        <!-- Users -->
        <el-tab-pane v-if="USER_PROTOCOLS.includes(rf.type)" :label="t('singbox.inbound.tabs.users')" name="users">
          <SingboxUsers :model="rf" :access-host="accessHost" @update="noop" />
        </el-tab-pane>

        <!-- Transport -->
        <el-tab-pane v-if="TRANSPORT_PROTOCOLS.includes(rf.type)" :label="t('singbox.inbound.tabs.transport')" name="transport">
          <SingboxTransport :model="rf" @update="noop" />
        </el-tab-pane>

        <!-- TLS -->
        <el-tab-pane v-if="TLS_ENABLED_PROTOCOLS.includes(rf.type)" :label="t('singbox.inbound.tabs.tls')" name="tls">
          <SingboxTls :model="rf" :server-id="serverId" :inbounds="form" :used-server-names="usedServerNames" @update="noop" />
        </el-tab-pane>

        <!-- Multiplex -->
        <el-tab-pane v-if="MULTIPLEX_PROTOCOLS.includes(rf.type)" :label="t('singbox.inbound.tabs.multiplex')" name="multiplex">
          <SingboxMultiplex :model="rf" @update="noop" />
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

    <!-- QR Code Dialog（用户列表与内容全部来自后端 /share-links，与订阅完全同源；
         前端只渲染二维码与文本） -->
    <el-dialog v-model="qrDlg" :title="t('protoCenter.inbounds.qrCode')" width="400" top="10vh">
      <div v-if="qrUsers.length > 1" style="margin-bottom:12px">
        <el-select v-model="qrSelectedUser" style="width:100%" @change="onQrUserChange">
          <el-option v-for="u in qrUsers" :key="u.key" :label="u.label" :value="u.key" />
        </el-select>
      </div>
      <div v-else-if="qrUsers.length === 1" style="margin-bottom:12px;color:#909399;font-size:12px">
        {{ qrUsers[0].label }}
      </div>
      <el-radio-group v-model="qrFormat" size="small" style="margin-bottom:12px" @change="onQrFormatChange">
        <el-radio-button value="uri">{{ t('protoCenter.inbounds.shareFormatUri') }}</el-radio-button>
        <el-radio-button value="mihomo">{{ t('protoCenter.inbounds.shareFormatMihomo') }}</el-radio-button>
        <el-radio-button value="singbox">{{ t('protoCenter.inbounds.shareFormatSingbox') }}</el-radio-button>
        <el-radio-button value="xray">{{ t('protoCenter.inbounds.shareFormatXray') }}</el-radio-button>
      </el-radio-group>
      <div style="text-align:center;padding:20px">
        <!-- 有内容 / 没内容、为什么没内容，全部由后端决定 -->
        <div v-if="qrUnsupported" style="margin-bottom:8px;font-size:12px;text-align:left;color:#E6A23C">
          {{ shareUnsupportedMessage({ reason: qrUnsupportedReason }, qrFormat) }}
        </div>
        <template v-if="qrFormat === 'uri'">
          <div v-if="qrCodeUrl" style="margin-bottom:16px">
            <img :src="qrCodeUrl" style="max-width:250px" />
          </div>
          <div v-else-if="qrGenerating" style="margin-bottom:16px;height:250px;display:flex;align-items:center;justify-content:center;color:#909399">
            {{ t('protoCenter.inbounds.qrGenerating') }}
          </div>
          <el-input v-model="qrLink" type="textarea" :rows="3" readonly style="margin-top:12px" />
        </template>
        <template v-else>
          <div v-if="!qrUnsupported" style="margin-bottom:8px;font-size:12px;text-align:left;color:#909399">
            {{ t('protoCenter.inbounds.snippetHint', { client: qrFormatName }) }}
          </div>
          <el-input v-model="qrSnippet" type="textarea" :rows="10" readonly />
        </template>
        <el-button size="small" style="margin-top:8px" @click="copyQrLink">{{ t('protoCenter.inbounds.copyLink') }}</el-button>
      </div>
    </el-dialog>

    <!-- Reset Traffic Dialog -->
    <el-dialog v-model="resetDlg" :title="t('protoCenter.inbounds.resetTraffic')" width="400">
      <div v-if="resetUsers.length">
        <p style="margin-bottom:12px">{{ t('protoCenter.inbounds.selectUserToReset') }}:</p>
        <el-radio-group v-model="resetSelectedIdx" style="display:flex;flex-direction:column;gap:8px">
          <el-radio v-for="(u, i) in resetUsers" :key="i" :value="i" style="margin:0">
            <span>{{ u.name || u.uuid?.substring(0, 8) || ('User ' + (i + 1)) }}</span>
            <span v-if="u._usedUp || u._usedDown" style="color:#909399;font-size:12px;margin-left:8px">↑ {{ formatTraffic(u._usedUp || 0) }} ↓ {{ formatTraffic(u._usedDown || 0) }}</span>
          </el-radio>
        </el-radio-group>
      </div>
      <template #footer>
        <el-button @click="resetDlg = false">{{ t('common.cancel') }}</el-button>
        <el-button type="warning" @click="confirmResetTraffic">{{ t('common.confirm') }}</el-button>
      </template>
    </el-dialog>

    <!-- Manage Users Dialog -->
    <el-dialog v-model="manageDlg" :title="t('protoCenter.inbounds.manageUsers')" width="600" top="8vh">
      <div v-if="manageInbound">
        <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:12px">
          <span style="font-weight:500">{{ manageInbound.tag }} ({{ manageInbound.type?.toUpperCase() }})</span>
        </div>
        <div v-for="(u, i) in manageUsers" :key="i" style="border:1px solid #eee;border-radius:6px;padding:12px;margin-bottom:8px">
          <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:8px">
            <div>
              <span style="font-weight:500">{{ u.name || u.uuid?.substring(0, 8) || ('User ' + (i + 1)) }}</span>
            </div>
            <div style="display:flex;gap:6px">
              <el-button size="small" type="primary" @click="copyShareForUser(manageInbound, u)">{{ t('protoCenter.inbounds.copyLink') }}</el-button>
              <el-button size="small" type="danger" plain @click="removeUser(i)">{{ t('common.remove') }}</el-button>
            </div>
          </div>
          <div style="font-size:12px;color:#909399;display:flex;gap:12px;flex-wrap:wrap">
            <span v-if="u.uuid">UUID: {{ u.uuid.substring(0, 16) }}...</span>
            <span v-if="u._usedUp || u._usedDown">↑ {{ formatTraffic(u._usedUp || 0) }} ↓ {{ formatTraffic(u._usedDown || 0) }}</span>
          </div>
        </div>
        <el-button size="small" style="margin-top:8px" @click="addUser(manageInbound)">{{ t('protoCenter.inbounds.addUser') }}</el-button>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, ref, reactive, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { MoreFilled, Edit, Delete, View, Refresh, RefreshRight, Plus, User } from '@element-plus/icons-vue'
const { t } = useI18n()
import client from '../../../api/client'
import { prefetchFreePort, takeFreePort } from './utils/freePort'
import { snisOfInbound, collectUsedSnis, findNewConflicts, mergeUsedSnis } from './utils/sniUsage'
import { PORT_HOP_PRESETS, findConflict } from './utils/portHopPresets'
import { useClientPagination } from '../../../composables/useClientPagination'
import { useQueryState } from '../../../composables/useQueryState'
import PaginationBar from '../../../components/PaginationBar.vue'
import MobileCards from '../../../components/MobileCards.vue'
import { useIsMobile } from '../../../composables/useIsMobile'
import { generateQrDataUrl, isQrDataTooLargeError } from '../../../utils/qrcode'
import { buildInbound, parseInbound } from './singbox/utils/singboxBuild'
import {
  inboundDefaultsForType, INBOUND_TYPES,
  TLS_ENABLED_PROTOCOLS, TRANSPORT_PROTOCOLS, MULTIPLEX_PROTOCOLS, USER_PROTOCOLS,
} from './singbox/utils/singboxDefaults'
import SingboxUsers from './singbox/common/SingboxUsers.vue'
import SingboxTransport from './singbox/common/SingboxTransport.vue'
import SingboxTls from './singbox/common/SingboxTls.vue'
import SingboxMultiplex from './singbox/common/SingboxMultiplex.vue'
// 仅含协议专属设置的协议组件（其余协议的全部参数都由通用 tab 提供，无需专属组件）
import SingboxShadowsocks from './singbox/inbounds/SingboxShadowsocks.vue'
import SingboxHysteria2 from './singbox/inbounds/SingboxHysteria2.vue'
import SingboxTuic from './singbox/inbounds/SingboxTuic.vue'
import SingboxShadowtls from './singbox/inbounds/SingboxShadowtls.vue'
import SingboxHysteria from './singbox/inbounds/SingboxHysteria.vue'
import SingboxSnell from './singbox/inbounds/SingboxSnell.vue'
import SingboxNaive from './singbox/inbounds/SingboxNaive.vue'
import SingboxTun from './singbox/inbounds/SingboxTun.vue'
import SingboxTproxy from './singbox/inbounds/SingboxTproxy.vue'
import SingboxCloudflared from './singbox/inbounds/SingboxCloudflared.vue'

const props = defineProps({ data: Array, coreType: String, serverId: Number, serverInfo: { type: Object, default: () => ({}) } })
const emitFn = defineEmits(['update'])

const form = reactive([])
const { paged: inbPaged, page: inbPage, pageSize: inbSize, total: inbTotal } = useClientPagination(computed(() => form), { pageKey: 'sbInbPage', pageSizeKey: 'sbInbSize' })
const { isMobile } = useIsMobile()

const PROTOCOL_COMPS = {
  shadowsocks: SingboxShadowsocks, hysteria2: SingboxHysteria2, tuic: SingboxTuic,
  shadowtls: SingboxShadowtls, hysteria: SingboxHysteria, snell: SingboxSnell,
  naive: SingboxNaive, tun: SingboxTun, tproxy: SingboxTproxy,
  cloudflared: SingboxCloudflared,
}
function getProtocolComp(t) { return PROTOCOL_COMPS[t] || null }

// 有协议专属 tab 的类型（其余参数全部由通用 tab 提供）
const PROTO_SPECIFIC_TYPES = ['shadowsocks', 'hysteria2', 'tuic', 'shadowtls', 'hysteria', 'snell', 'naive', 'tun', 'tproxy', 'cloudflared']
function hasProtoTab(t) { return PROTO_SPECIFIC_TYPES.includes(t) }

// 访问地址：优先使用 server 域名/IP（由父级传入 data 时可附带），无则用 listen
const accessHost = computed(() => props.serverInfo?.domain || props.serverInfo?.address || '')

const dlg = ref(false)
const editIdx = ref(-1)
const rf = reactive({})
const activeTab = useQueryState('sbitab', 'basic')

// Access Domain data
const cfDomains = ref([])
const cfIPEntries = ref([])
const directDomains = ref([])

// 接入域名启用后是否真的选了域名/IP：没选时不能从面板域名池里随机抓一个
// （那样分享链接/预览里的 server 与下拉选择对不上），而应退回节点自身地址。
const accessDomainSelected = computed(() => {
  switch (rf._accessDomainType) {
    case 'direct': return !!rf._directDomain
    case 'cf_domain': return !!rf._cfDomain
    case 'cf_ip': return !!rf._cfIP
    case 'custom': return !!rf._customDomain
    default: return false
  }
})

// QR Dialog
const qrDlg = ref(false)
const qrLink = ref('')
const qrCodeUrl = ref('')
const qrGenerating = ref(false)
// 用户列表由后端给出（后端已过滤暂停用户）：[{ key: '后端 index', label: 'name/username' }]
const qrUsers = ref([])
const qrSelectedUser = ref('')
const qrCurrentIb = ref(null)
// 分享数据**全部来自后端** /share-links（与订阅同源）：uri=分享链接；
// mihomo/singbox/xray=可直接粘贴的配置片段。前端不再自己生成，也不做兜底。
// 缓存按格式分开：format → { id, supported, reason, users: [{ key, label }], items: [{ key, label, text }] }
const qrCache = ref({})
const qrSnippet = ref('')
const qrUnsupported = ref(false)
// 后端给出的不支持原因（如 transport=kcp / users / protocol=xxx）
const qrUnsupportedReason = ref('')
const qrFormat = ref('uri')      // uri | mihomo | singbox | xray
const qrFormatName = computed(() => shareFormatName(qrFormat.value))

// Reset Traffic Dialog
const resetDlg = ref(false)
const resetUsers = ref([])
const resetSelectedIdx = ref(0)
const resetInboundIdx = ref(-1)

// Manage Users Dialog
const manageDlg = ref(false)
const manageInbound = ref(null)
const manageUsers = ref([])
const manageInboundIdx = ref(-1)

function noop() {}

function showAdd() {
  editIdx.value = -1
  activeTab.value = 'basic'
  // 先清空：rf 是复用的 reactive 对象，上次【编辑】写入而 defaults 里没有的键
  // 会残留进新建对话框（表现为“新建还显示已创建入站的参数”）。
  for (const k of Object.keys(rf)) delete rf[k]
  Object.assign(rf, inboundDefaultsForType('vless', takeFreePort(props.serverId)))
  rf._id = null
  dlg.value = true
}

function showEdit(row) {
  editIdx.value = form.indexOf(row)
  activeTab.value = 'basic'
  Object.assign(rf, parseInbound(JSON.parse(JSON.stringify(row))))
  // 带上自身 id：SNI 互斥比较时要排除自己（utils/sniUsage 按 id 判断）
  rf._id = row._id ?? row.id ?? null
  dlg.value = true
}

function onTypeChange() {
  // 切协议 = 整套默认重置：同 showAdd，先清空再赋值，避免上一协议的字段残留
  const type = rf.type
  for (const k of Object.keys(rf)) delete rf[k]
  rf.type = type
  Object.assign(rf, inboundDefaultsForType(type, takeFreePort(props.serverId)))
}

async function save() {
  const cfg = buildInbound(rf)
  if (!cfg.tag) { ElMessage.warning(t('singbox.inbound.tagRequired')); return }

  // Reality SNI（Server Name）互斥：同节点内不能重复、跨节点允许重复；
  // 只拦截「本次新增的重复」（存量重复不阻塞编辑），与后端保持一致。
  if (rf._realityEnabled) {
    const mySni = String(rf._tlsServerName || '').trim().toLowerCase()
    if (mySni) {
      const self = editIdx.value >= 0 ? form[editIdx.value] : null
      // 本核心内存中的其它入站 + 后端登记的跨核心占用（同节点 xray 入站也要算）
      const used = mergeUsedSnis(collectUsedSnis(form, self), usedServerNames.value, self?._id ?? null)
      const added = findNewConflicts([mySni], self ? snisOfInbound(self) : [], used)
      if (added.length) {
        ElMessage.error(t('singbox.tls.sniConflict', { sni: mySni, tag: used.get(mySni) || '' }))
        return
      }
    }
  }
  // Build payload (same structure as xray inbound-configs)
  const configJson = { ...cfg }
  delete configJson._remark
  delete configJson._enabled
  delete configJson._id
  const payload = {
    core_type: 'singbox',
    remark: cfg._remark || cfg.tag || '',
    protocol: cfg.type,
    tag: cfg.tag,
    listen: cfg.listen || '0.0.0.0',
    port: cfg.listen_port || 0,
    enabled: cfg._enabled !== false,
    config_json: configJson,
  }
  try {
    if (editIdx.value >= 0 && form[editIdx.value]?._id) {
      await client.put(`${nodeBase.value}/inbound-configs/${form[editIdx.value]._id}`, payload)
    } else {
      await client.post(`${nodeBase.value}/inbound-configs`, payload)
    }
    // 端口跳跃：保存时自动增删防火墙规则（端口段 DNAT → 入站端口）。
    // 规则与核心无关，但 sing-box 入站不能写 server_ports，只能靠这套规则实现。
    if (rf.type === 'hysteria2' || rf.type === 'hysteria') {
      const isEnabled = rf._portHopEnabled || false
      const newRange = rf._portHopMode === 'preset' ? (rf._portHopRange || '20000-50000') : `${rf._portHopStart || 20000}-${rf._portHopEnd || 50000}`
      const newIface = rf._portHopInterface || ''
      const newPort = rf.listen_port || 0
      const fwBase = `${nodeBase.value}/singbox/control/firewall`
      try {
        if (editIdx.value >= 0) {
          const oldIb = form[editIdx.value] || {}
          const wasEnabled = oldIb._portHopEnabled || false
          const oldRange = oldIb._portHopMode === 'preset' ? (oldIb._portHopRange || '20000-50000') : `${oldIb._portHopStart || 20000}-${oldIb._portHopEnd || 50000}`
          const oldIface = oldIb._portHopInterface || ''
          const oldPort = oldIb.listen_port || 0
          const changed = oldRange !== newRange || oldIface !== newIface || oldPort !== newPort
          if (wasEnabled && (!isEnabled || changed)) {
            await client.post(`${fwBase}/remove`, { port_range: oldRange, target_port: oldPort, interface: oldIface })
          }
          if (isEnabled && (!wasEnabled || changed)) {
            await client.post(`${fwBase}/apply`, { port_range: newRange, target_port: newPort, interface: newIface })
          }
        } else if (isEnabled) {
          await client.post(`${fwBase}/apply`, { port_range: newRange, target_port: newPort, interface: newIface })
        }
      } catch (fwErr) {
        // 端口段冲突等错误必须让用户看到（否则以为保存成功但规则没生效）
        ElMessage.error(fwErr?.response?.data?.message || '端口跳跃防火墙规则应用失败')
      }
    }

    // Restart sing-box to apply changes (no hot-reload API)
    await restartCore()
    ElMessage.success(t('common.success'))
    dlg.value = false
    await loadInbounds()
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || t('common.failed'))
  }
}

function handleAction(cmd, row) {
  if (cmd === 'edit') showEdit(row)
  else if (cmd === 'qrcode') showQr(row)
  else if (cmd === 'reset-traffic') showResetTraffic(row)
  else if (cmd === 'add-user') showAddUser(row)
  else if (cmd === 'manage-users') showManageUsers(row)
  else if (cmd === 'delete') {
    const i = form.indexOf(row)
    const ib = form[i]
    if (!ib) return
    ElMessageBox.confirm(t('common.deleteConfirm'), t('common.confirm'), { type: 'warning' })
      .then(async () => {
        try {
          if (ib._id) {
            await client.delete(`${nodeBase.value}/inbound-configs/${ib._id}`)
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

// Toggle enabled state of an inbound — saves to API and restarts sing-box
async function toggleEnabled(row) {
  const i = form.indexOf(row)
  if (i < 0) return
  const ib = form[i]
  if (!ib._id) return
  const cfg = buildInbound(ib)
  const configJson = { ...cfg }
  delete configJson._remark
  delete configJson._enabled
  delete configJson._id
  try {
    await client.put(`${nodeBase.value}/inbound-configs/${ib._id}`, {
      remark: ib._remark || ib.tag || '',
      protocol: cfg.type,
      tag: cfg.tag,
      listen: cfg.listen || '0.0.0.0',
      port: cfg.listen_port || 0,
      enabled: ib._enabled !== false,
      config_json: configJson,
    })
    await restartCore()
  } catch (e) {
    // Revert on failure
    ib._enabled = !ib._enabled
    ElMessage.error(e?.response?.data?.message || t('common.failed'))
  }
}

// ==================== Share / QR ====================
// 链接与片段**只有一处生成**：后端 /share-links（与订阅同源）。前端以前自己也拼一套
// （singbox/utils/singboxShareLink.js），两套实现必然漂移，还得靠 name/email 做兜底匹配。
// 现在前端只做两件事：渲染二维码 + 展示后端给的文本。

// 分享格式 → i18n 里的显示名
const SHARE_FORMAT_I18N = {
  uri: 'shareFormatUri',
  mihomo: 'shareFormatMihomo',
  singbox: 'shareFormatSingbox',
  xray: 'shareFormatXray',
}

function shareFormatName(format) {
  return t('protoCenter.inbounds.' + (SHARE_FORMAT_I18N[format] || 'shareFormatUri'))
}

// 后端逐用户生成（与订阅同源）：uri=分享链接，其余=可直接粘贴的配置片段。
// entry.users 与 format 无关（弹窗的用户选择框直接用它）；items 的 index 指回 users。
async function loadShareLinks(ib, format = 'uri') {
  const id = inboundKey(ib)
  if (!id) return null
  const entry = { id, supported: true, reason: '', items: [], users: [] }
  try {
    const data = await client.get(`${nodeBase.value}/inbound-configs/${id}/share-links`, { params: { format } })
    // 协议/传输是否受该客户端支持（后端按订阅同一份矩阵判定）
    entry.supported = data?.supported !== false
    entry.reason = data?.reason || ''
    // 用户列表由**后端**给出（后端已过滤暂停用户），前端不再自己枚举用户
    entry.users = (data?.users || []).map(u => ({ key: String(u.index), label: String(u.label || '') }))
    entry.items = (data?.items || []).map((it, i) => ({
      key: it.index === undefined || it.index === null ? `#${i}` : String(it.index),
      label: String(it.user || ''),
      text: it.text || it.link || '',
    }))
  } catch {
    // 后端拿不到就**不再本地兜底生成**（另一套实现必然漂移）：如实报错，别装作有内容
    entry.supported = false
    entry.reason = 'request-failed'
  }
  qrCache.value = { ...qrCache.value, [format]: entry }
  return entry
}

// 入站的唯一标识（缓存归属用它判定）
function inboundKey(ib) {
  return ib?._id ?? ib?.id ?? null
}

// shareEntry 取「该入站」在某格式下的缓存；id 不匹配视为未缓存。
// 曾经只按 format 缓存 → 打开另一个入站的分享弹窗时 mihomo/singbox/xray 标签
// 一直显示**上一个入站**的内容（uri 因为每次都重新拉所以看不出来）。
function shareEntry(format, ib) {
  const entry = qrCache.value[format]
  if (!entry || entry.id !== inboundKey(ib)) return null
  return entry
}

// 取某用户在某格式下的条目：userKey = 后端 users[].index（跨格式稳定）；
// 单用户入站再取唯一一条。
function pickShareEntry(format, ib, userKey) {
  const entry = shareEntry(format, ib)
  if (!entry || !entry.items.length) return null
  const hit = entry.items.find(it => it.key === userKey)
  if (hit) return hit
  return entry.items.length === 1 ? entry.items[0] : null
}

// 表格/详情弹窗按**本地用户对象**复制链接时的匹配：后端 label 就是 name|username，
// 与本地用户字段同源，按 label 对上即可。
function pickShareEntryByUser(format, ib, user) {
  const entry = shareEntry(format, ib)
  if (!entry || !entry.items.length) return null
  const label = String(user?.name || user?.username || user?.email || '').trim().toLowerCase()
  const hit = label ? entry.items.find(it => it.label.toLowerCase() === label) : null
  if (hit) return hit
  return entry.items.length === 1 ? entry.items[0] : null
}

// 该用户的分享链接：**只从后端取**（与订阅同源、参数完整）
function shareUriForUser(ib, user) {
  return pickShareEntryByUser('uri', ib, user)?.text || ''
}

// 后端给的"为什么没有内容" → 展示文案。
// reason: protocol=vless / transport=kcp / plain-vless|plain-trojan / vless-encryption /
//         users（没有可用用户，含全部已暂停）/ request-failed
function shareUnsupportedMessage(entry, format) {
  const reason = entry?.reason || ''
  if (reason === 'request-failed') return t('protoCenter.inbounds.shareLoadFailed')
  if (reason === 'users') return t('protoCenter.inbounds.shareNoUsers')
  const client = shareFormatName(format)
  if (reason === 'vless-encryption') return t('protoCenter.inbounds.snippetUnsupportedVlessEncryption', { client })
  if (reason.startsWith('plain-')) {
    return t('protoCenter.inbounds.snippetUnsupportedPlain', { client, protocol: reason.slice('plain-'.length) })
  }
  const transport = reason.startsWith('transport=') ? reason.slice('transport='.length) : ''
  return transport
    ? t('protoCenter.inbounds.snippetUnsupportedTransport', { client, transport })
    : t('protoCenter.inbounds.snippetUnsupported', { client })
}

async function renderQr(link) {
  qrLink.value = link || ''
  qrCodeUrl.value = ''
  // 没有链接时不再弹 toast —— 原因已经在弹窗里显示（shareUnsupportedMessage）
  if (!link) return
  qrGenerating.value = true
  try {
    qrCodeUrl.value = await generateQrDataUrl(link)
  } catch (e) {
    if (isQrDataTooLargeError(e)) ElMessage.warning(t('singbox.inbound.linkTooLong'))
    else ElMessage.error(t('singbox.inbound.qrFailed'))
  } finally {
    qrGenerating.value = false
  }
}

// 打开分享弹窗：用户列表**完全来自后端**（后端已过滤暂停用户），
// 前端不再看本地配置自己枚举用户，也不再做标识兜底匹配。
async function showQr(row) {
  qrCurrentIb.value = row
  qrUsers.value = []
  qrSelectedUser.value = ''
  qrFormat.value = 'uri'
  qrLink.value = ''
  qrSnippet.value = ''
  qrCodeUrl.value = ''
  qrUnsupported.value = false
  qrUnsupportedReason.value = ''
  const entry = await loadShareLinks(row, 'uri')
  if (!entry || !entry.users.length) {
    // 一条可用数据都没有：把原因说清楚（没有用户 / 全部已暂停 / 请求失败）
    ElMessage.warning(shareUnsupportedMessage(entry, 'uri'))
    return
  }
  qrUsers.value = entry.users
  qrSelectedUser.value = qrUsers.value[0].key
  qrDlg.value = true
  await renderCurrentShare()
}

async function onQrUserChange() {
  await renderCurrentShare()
}

function onQrFormatChange() {
  void renderCurrentShare()
}

async function renderCurrentShare() {
  const row = qrCurrentIb.value
  if (!row) return
  const format = qrFormat.value
  // 缓存按「入站 + 格式」判定：换入站后必须重新拉，否则显示的是上一个入站的片段
  let entry = shareEntry(format, row)
  if (!entry) entry = await loadShareLinks(row, format)
  qrUnsupported.value = entry?.supported === false
  qrUnsupportedReason.value = entry?.reason || ''
  const item = pickShareEntry(format, row, qrSelectedUser.value)
  if (format === 'uri') {
    qrSnippet.value = ''
    await renderQr(item?.text || '')
    return
  }
  qrCodeUrl.value = ''
  qrLink.value = ''
  qrSnippet.value = item?.text || ''
}

function copyQrLink() {
  const text = qrFormat.value === 'uri' ? qrLink.value : qrSnippet.value
  if (!text) return
  copyToClipboard(text)
  ElMessage.success(t('singbox.inbound.shareCopied'))
}

// ==================== Reset Traffic ====================

function showResetTraffic(row) {
  const users = row.users || []
  if (!users.length) return
  resetInboundIdx.value = form.indexOf(row)
  resetUsers.value = users.map(u => ({ ...u }))
  resetSelectedIdx.value = 0
  resetDlg.value = true
}

function confirmResetTraffic() {
  const ib = form[resetInboundIdx.value]
  if (!ib || !ib.users) { resetDlg.value = false; return }
  const u = ib.users[resetSelectedIdx.value]
  if (u) {
    u._usedUp = 0
    u._usedDown = 0
  }
  emit()
  resetDlg.value = false
  ElMessage.success(t('protoCenter.inbounds.resetTraffic') + ' ✓')
}

// ==================== Manage Users ====================

function showManageUsers(row) {
  manageInboundIdx.value = form.indexOf(row)
  manageInbound.value = row
  manageUsers.value = (row.users || []).map(u => ({ ...u }))
  manageDlg.value = true
}

function showAddUser(row) {
  // 切到编辑弹窗的 users tab
  showEdit(row)
  activeTab.value = 'users'
}

function removeUser(idx) {
  manageUsers.value.splice(idx, 1)
  // 同步回 form
  const ib = form[manageInboundIdx.value]
  if (ib && ib.users) {
    ib.users.splice(idx, 1)
    emit()
  }
}

function addUser(ib) {
  if (!ib.users) ib.users = []
  showAddUser(ib)
}

async function copyShareForUser(ib, user) {
  // 管理弹窗里点复制时惰性加载后端链接（与订阅同源）；链接只从后端取
  if (!shareEntry('uri', ib)) await loadShareLinks(ib, 'uri')
  const link = shareUriForUser(ib, user)
  if (!link) { ElMessage.warning(t('singbox.inbound.shareNotSupported')); return }
  copyToClipboard(link)
  ElMessage.success(t('singbox.inbound.shareCopied'))
}

// ==================== Stats helpers (对齐 xray) ====================

function formatTraffic(bytes) {
  if (!bytes) return '0 B'
  const u = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0, b = bytes
  while (b >= 1024 && i < u.length - 1) { b /= 1024; i++ }
  return b.toFixed(1) + ' ' + u[i]
}

const totalUpload = computed(() => {
  let total = 0
  for (const ib of form) total += getTotalTraffic(ib, 'up')
  return total
})

const totalDownload = computed(() => {
  let total = 0
  for (const ib of form) total += getTotalTraffic(ib, 'down')
  return total
})

function getTotalTraffic(ib, direction) {
  const users = ib.users || []
  return users.reduce((sum, u) => sum + (direction === 'up' ? (u._usedUp || 0) : (u._usedDown || 0)), 0)
}

function getUserCount(ib) {
  return (ib.users || []).length
}

function getActiveUserCount(ib) {
  const users = ib.users || []
  return users.filter(u => {
    if (!u._expiry) return true
    return new Date(u._expiry) > new Date()
  }).length
}

function getExpiredUserCount(ib) {
  const users = ib.users || []
  return users.filter(u => {
    if (!u._expiry) return false
    return new Date(u._expiry) <= new Date()
  }).length
}

function getTrafficColor(ib) {
  const total = getTotalTraffic(ib, 'up') + getTotalTraffic(ib, 'down')
  if (total === 0) return 'info'
  if (total > 1073741824) return 'warning'
  return 'success'
}

function getExpiryColor(ib) {
  const users = ib.users || []
  if (!users.length) return 'info'
  const expiry = users[0]?._expiry
  if (!expiry) return 'info'
  const date = new Date(expiry)
  if (isNaN(date.getTime())) return 'info'
  if (date < new Date()) return 'danger'
  const daysLeft = (date - new Date()) / (1000 * 60 * 60 * 24)
  if (daysLeft < 7) return 'warning'
  return 'success'
}

function getExpiryDisplay(ib) {
  const users = ib.users || []
  if (!users.length) return '∞'
  const expiry = users[0]?._expiry
  if (!expiry) return '∞'
  const date = new Date(expiry)
  if (isNaN(date.getTime())) return expiry
  const now = new Date()
  if (date < now) return t('protoCenter.inbounds.expired')
  const diff = date - now
  const days = Math.floor(diff / (1000 * 60 * 60 * 24))
  if (days > 365) return Math.floor(days / 365) + 'y'
  if (days > 30) return Math.floor(days / 30) + 'mo'
  return days + 'd'
}

function hasUsers(ib) {
  return (ib.users?.length || 0) > 0
}

function hasShareableUsers(ib) {
  return hasUsers(ib)
}

function hasUserList(ib) {
  return (ib.users?.length || 0) > 0
}

// 手机端卡片字段（与桌面表格列一一对应；启用开关在 actions 插槽里）
const sbInboundCardFields = computed(() => [
  { label: t('protoCenter.inbounds.remark'), text: (row) => row._remark || row.tag || '-' },
  { label: t('singbox.inbound.tag'), text: (row) => row.tag || '-' },
  { label: t('singbox.inbound.port'), text: (row) => row.listen_port || '-' },
  { label: t('singbox.inbound.protocol'), text: (row) => [(row.type || '').toUpperCase(), row._transportType || '', row._tlsEnabled ? 'TLS' : '', row._realityEnabled ? 'Reality' : ''].filter(Boolean).join(', ') || '-' },
  { label: t('singbox.inbound.users'), text: (row) => {
    const parts = [String(getUserCount(row))]
    if (getActiveUserCount(row) > 0) parts.push(String(getActiveUserCount(row)))
    if (getExpiredUserCount(row) > 0) parts.push(String(getExpiredUserCount(row)))
    return parts.join(' / ')
  } },
  { label: t('protoCenter.inbounds.traffic'), text: (row) => '↑ ' + formatTraffic(getTotalTraffic(row, 'up')) + ' / ↓ ' + formatTraffic(getTotalTraffic(row, 'down')) },
  { label: t('protoCenter.inbounds.expiry'), text: (row) => getExpiryDisplay(row) },
])

// ==================== Clipboard ====================

function copyToClipboard(text) {
  if (navigator.clipboard && window.isSecureContext) {
    navigator.clipboard.writeText(text).then(() => {
      ElMessage.success(t('singbox.inbound.shareCopied'))
    }).catch(() => fallbackCopy(text))
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
  try { document.execCommand('copy') } catch {}
  document.body.removeChild(textarea)
}

// ==================== Sync ====================

// Load inbounds from backend API (inbound-configs)
// Also merges inbounds from props.data (core_configs table) that are not
// in the inbound_configs table yet — this handles legacy/migrated configs.
async function loadInbounds() {
  let apiItems = []
  try {
    const res = await client.get(`${nodeBase.value}/inbound-configs`, { params: { core_type: 'singbox' } })
    apiItems = res?.items || (Array.isArray(res) ? res : [])
  } catch (e) {
    console.error('loadInbounds API failed:', e)
  }

  form.splice(0, form.length)

  // First, load from inbound_configs table (API)
  const apiTags = new Set()
  for (const ib of apiItems) {
    let cfg
    try { cfg = JSON.parse(ib.config_json) } catch { cfg = ib.config_json || {} }
    const parsed = parseInbound(cfg)
    parsed._id = ib.id
    parsed._remark = ib.remark || parsed._remark || ''
    parsed._enabled = ib.enabled !== false
    form.push(parsed)
    if (parsed.tag) apiTags.add(parsed.tag)
  }

  // Then, merge inbounds from props.data (core_configs table) that are
  // not already in the inbound_configs table
  if (Array.isArray(props.data)) {
    for (const ib of props.data) {
      const parsed = parseInbound(JSON.parse(JSON.stringify(ib)))
      if (parsed.tag && !apiTags.has(parsed.tag)) {
        parsed._id = null  // no DB record yet
        parsed._enabled = parsed._enabled !== false
        form.push(parsed)
      }
    }
  }
}

function sync() {
  loadInbounds()
}

// emit is now a no-op — saves go directly to the API
function emit() {}

// Restart sing-box core to apply config changes
async function restartCore() {
  try {
    await client.post(`${nodeBase.value}/control`, { core_type: 'singbox', action: 'restart' })
  } catch (e) {
    console.warn('restart singbox failed:', e)
  }
}

// ==================== Access Domain data loading ====================

const nodeBase = computed(() => `/node-servers/${props.serverId}`)
// 端口跳跃预设段来自 utils/portHopPresets（与 xray 编辑器同一份定义）；历史值也显示。
const portHopRules = ref([])
const portHopSuggest = ref('')
async function loadPortHopRules() {
  try {
    const data = await client.get(`${nodeBase.value}/port-hop/rules`)
    portHopRules.value = data?.items || (Array.isArray(data) ? data : [])
    portHopSuggest.value = data?.suggest || ''
  } catch { portHopRules.value = []; portHopSuggest.value = '' }
}
const portHopRangeOptions = computed(() => {
  const cur = rf._portHopRange
  const base = [...PORT_HOP_PRESETS]
  if (cur && !base.includes(cur)) base.push(cur)
  const selfId = rf._id ?? null
  const selfTarget = Number(rf.listen_port) || 0
  return base.map(v => ({ value: v, label: v, conflict: findConflict(portHopRules.value, { portRange: v, targetPort: selfTarget }, selfId) }))
})
const portHopUsageHint = computed(() => {
  const parts = []
  const used = portHopRules.value.map(r => `${r.port_range} → ${r.target_port}`)
  if (used.length) parts.push('本节点已用端口段：' + used.join('，'))
  if (portHopSuggest.value) parts.push('空闲可用段：' + portHopSuggest.value)
  return parts.join('；')
})
// 端口跳跃：网卡列表（留空 = 全部网卡）
const ifaceList = ref([])
async function loadInterfaces() {
  try {
    // 网卡列表接口在 /binaries 下（与 xray 编辑器一致）：/node-servers/:id/binaries/interfaces
    const data = await client.get(`${nodeBase.value}/binaries/interfaces`)
    if (Array.isArray(data)) ifaceList.value = data
  } catch { ifaceList.value = ['eth0', 'wg0', 'lo'] }
}

async function loadCFDomains() {
  try {
    const globalData = await client.get('/share-domains')
    const global = globalData?.items || (Array.isArray(globalData) ? globalData : [])
    cfDomains.value = global.filter(d => d.type === 'cf_domain')
    const nodeData = await client.get(`${nodeBase.value}/share-domains`)
    const direct = nodeData?.items || (Array.isArray(nodeData) ? nodeData : [])
    directDomains.value = direct.filter(d => d.type === 'direct')
  } catch {}
}

async function loadCFIPEntries() {
  try {
    const data = await client.get('/cfip/entries')
    cfIPEntries.value = data?.items || (Array.isArray(data) ? data : [])
  } catch {}
}

// 该节点上（xray + sing-box）已占用的 Server Names（后端登记，用于跨核心标记与互斥判断）
const usedServerNames = ref([])
async function loadUsedServerNames() {
  try {
    const data = await client.get(`${nodeBase.value}/used-server-names`)
    usedServerNames.value = data?.items || (Array.isArray(data) ? data : [])
  } catch { usedServerNames.value = [] }
}

onMounted(() => {
  prefetchFreePort(props.serverId) // 预热空闲端口，新建入站时不再撞已有端口
  loadInterfaces()
  sync()
  loadCFDomains()
  loadCFIPEntries()
  loadUsedServerNames()
  loadPortHopRules() // 该节点已用的端口跳跃段，用于标记冲突段
})
</script>

<style scoped>
.hint { color: #909399; font-size: 12px; margin-top: 4px; }
</style>