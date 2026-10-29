<template>
  <div>
    <el-tabs v-model="activeTab" type="border-card">

      <!-- ════════════ 服务器列表 Tab ════════════ -->
      <el-tab-pane :label="t('nodeServer.servers')" name="servers">
        <div style="display:flex;justify-content:space-between;margin-bottom:16px;align-items:center">
          <div style="display:flex;gap:8px;align-items:center">
            <el-select v-model="filterRegion" :placeholder="t('nodeServer.region')" clearable size="small" style="width:140px" @change="loadServers">
              <el-option v-for="r in regionOptions" :key="r" :label="r" :value="r" />
            </el-select>
            <el-select v-model="filterNodeType" :placeholder="t('nodeServer.nodeType')" clearable size="small" style="width:140px" @change="loadServers">
              <el-option v-for="opt in nodeTypeOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
            </el-select>
            <el-button size="small" @click="loadServers"><el-icon><Refresh /></el-icon> {{ t('common.refresh') }}</el-button>
            <el-button v-if="hasRemoteAgents" size="small" type="warning" @click="openUpdateAllDialog" :loading="updateAllLoading">{{ t('nodeServer.updateAllAgents') }}</el-button>
          </div>
          <el-button type="primary" size="small" @click="showCreateDialog">+ {{ t('nodeServer.addServer') }}</el-button>
        </div>

        <el-table v-if="!isMobile" :data="servers" v-loading="loading" stripe size="small">
          <el-table-column prop="id" label="ID" width="45" />
          <el-table-column prop="name" :label="t('common.name')" min-width="115" show-overflow-tooltip />
          <el-table-column prop="hostname" :label="t('nodeServer.hostname')" min-width="130" show-overflow-tooltip />
          <el-table-column :label="t('nodeServer.region')" width="62">
            <template #default="{row}">
              <el-tag size="small" type="info">{{ row.region || '-' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="t('nodeServer.nodeType')" width="88" show-overflow-tooltip>
            <template #default="{row}">
              <el-tag size="small" :type="nodeTypeTag(row.node_type)">{{ nodeTypeLabel(row.node_type) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="t('nodeServer.relationship')" min-width="135" show-overflow-tooltip>
            <template #default="{row}">
              <div v-if="row.node_type === 'mirror' && row.partner_server_id" style="display:flex;align-items:center;gap:4px;font-size:12px">
                <el-tag size="small" type="warning" effect="plain">{{ t('nodeServer.mirrorOf') }}</el-tag>
                <el-link type="primary" underline="never" @click="goToPartner(row.partner_server_id)" style="font-size:12px">{{ serverMap[row.partner_server_id] || ('#'+row.partner_server_id) }}</el-link>
              </div>
              <div v-else-if="row.node_type === 'primary_backup' && row.partner_server_id" style="display:flex;align-items:center;gap:4px;font-size:12px">
                <el-tag size="small" :type="row.failover_role === 'primary' ? 'danger' : 'info'" effect="plain">{{ row.failover_role === 'primary' ? t('nodeServer.failoverPrimary') : t('nodeServer.failoverBackup') }}</el-tag>
                <span style="color:#909399">⇄</span>
                <el-link type="primary" underline="never" @click="goToPartner(row.partner_server_id)" style="font-size:12px">{{ serverMap[row.partner_server_id] || ('#'+row.partner_server_id) }}</el-link>
              </div>
              <div v-else-if="row.chain_entry_for && row.chain_entry_for.length" style="display:flex;align-items:center;gap:4px;font-size:12px;flex-wrap:wrap">
                <el-tag size="small" type="success" effect="plain">{{ t('nodeServer.chainEntry') }}</el-tag>
                <template v-for="exitId in row.chain_entry_for" :key="'e'+exitId">
                  <el-link type="primary" underline="never" @click="goToPartner(exitId)" style="font-size:12px">{{ serverMap[exitId] || ('#'+exitId) }}</el-link>
                </template>
              </div>
              <div v-else-if="row.chain_exit_for && row.chain_exit_for.length" style="display:flex;align-items:center;gap:4px;font-size:12px;flex-wrap:wrap">
                <el-tag size="small" type="success" effect="plain">{{ t('nodeServer.chainExit') }}</el-tag>
                <template v-for="entryId in row.chain_exit_for" :key="'x'+entryId">
                  <el-link type="primary" underline="never" @click="goToPartner(entryId)" style="font-size:12px">{{ serverMap[entryId] || ('#'+entryId) }}</el-link>
                </template>
              </div>
              <el-tag v-else-if="row.node_type === 'chain_entry'" size="small" type="success" effect="plain">{{ t('nodeServer.chainEntry') }}</el-tag>
              <el-tag v-else-if="row.node_type === 'chain_exit'" size="small" type="success" effect="plain">{{ t('nodeServer.chainExit') }}</el-tag>
              <span v-else style="color:#c0c4cc;font-size:12px">{{ t('nodeServer.noPartner') }}</span>
            </template>
          </el-table-column>
          <el-table-column :label="t('nodeServer.agentStatus')" width="72">
            <template #default="{row}">
              <el-tag size="small" :type="statusTag(row.agent_status)">
                <span :class="['status-dot', row.agent_status]"></span>
                {{ statusLabel(row.agent_status) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="agent_version" :label="t('nodeServer.agentVersion')" width="78" show-overflow-tooltip />
          <el-table-column label="CPU" width="76">
            <template #default="{row}">
              <div v-if="row.resource_usage" style="display:flex;align-items:center;gap:6px">
                <div style="flex:1;height:6px;background:#f0f0f0;border-radius:3px;overflow:hidden">
                  <div :style="{width: Math.min(row.resource_usage.cpu_usage,100)+'%', background: row.resource_usage.cpu_usage > 80 ? '#f56c6c' : '#409eff', height:'100%', borderRadius:'3px'}" />
                </div>
                <span style="font-size:11px;color:#606266;min-width:32px;text-align:right">{{ row.resource_usage.cpu_usage?.toFixed(1) || 0 }}%</span>
              </div>
              <span v-else style="color:#c0c4cc;font-size:11px">-</span>
            </template>
          </el-table-column>
          <el-table-column :label="t('dashboard.memory')" width="76">
            <template #default="{row}">
              <div v-if="row.resource_usage" style="display:flex;align-items:center;gap:6px">
                <div style="flex:1;height:6px;background:#f0f0f0;border-radius:3px;overflow:hidden">
                  <div :style="{width: Math.min(row.resource_usage.mem_usage,100)+'%', background: row.resource_usage.mem_usage > 80 ? '#f56c6c' : '#67c23a', height:'100%', borderRadius:'3px'}" />
                </div>
                <span style="font-size:11px;color:#606266;min-width:32px;text-align:right">{{ row.resource_usage.mem_usage?.toFixed(1) || 0 }}%</span>
              </div>
              <span v-else style="color:#c0c4cc;font-size:11px">-</span>
            </template>
          </el-table-column>
          <el-table-column :label="t('dashboard.disk')" width="76">
            <template #default="{row}">
              <div v-if="row.resource_usage" style="display:flex;align-items:center;gap:6px">
                <div style="flex:1;height:6px;background:#f0f0f0;border-radius:3px;overflow:hidden">
                  <div :style="{width: Math.min(row.resource_usage.disk_usage,100)+'%', background: row.resource_usage.disk_usage > 80 ? '#f56c6c' : '#e6a23c', height:'100%', borderRadius:'3px'}" />
                </div>
                <span style="font-size:11px;color:#606266;min-width:32px;text-align:right">{{ row.resource_usage.disk_usage?.toFixed(1) || 0 }}%</span>
              </div>
              <span v-else style="color:#c0c4cc;font-size:11px">-</span>
            </template>
          </el-table-column>
          <el-table-column :label="t('nodeServer.monthlyTraffic')" width="95">
            <template #default="{row}">
              <div v-if="row.traffic_quota_bytes > 0" style="display:flex;align-items:center;gap:6px">
                <div style="flex:1;height:6px;background:#f0f0f0;border-radius:3px;overflow:hidden">
                  <div :style="{width: Math.min((row.traffic_used_bytes/row.traffic_quota_bytes)*100, 100)+'%', background: row.traffic_suspended ? '#f56c6c' : (row.traffic_used_bytes/row.traffic_quota_bytes > 0.8 ? '#e6a23c' : '#409eff'), height:'100%', borderRadius:'3px'}" />
                </div>
                <span style="font-size:10px;color:#606266;min-width:38px;text-align:right">{{ formatBytes(row.traffic_used_bytes) }}/{{ formatBytes(row.traffic_quota_bytes) }}</span>
              </div>
              <span v-else-if="row.traffic_suspended" style="color:#f56c6c;font-size:11px">⚠ {{ t('nodeServer.trafficSuspended') }}</span>
              <span v-else style="color:#c0c4cc;font-size:11px">-</span>
            </template>
          </el-table-column>
          <el-table-column :label="t('nodeServer.lastHeartbeat')" width="88">
            <template #default="{row}">
              <span v-if="row.last_heartbeat_at" :title="formatTime(row.last_heartbeat_at)" style="font-size:12px;color:#909399">{{ formatAgo(row.last_heartbeat_at) }}</span>
              <span v-else style="color:#c0c4cc">-</span>
            </template>
          </el-table-column>
          <el-table-column :label="t('nodeServer.certStatus')" width="62">
            <template #default="{row}">
              <div v-if="row.agent_cert_expires_at" style="font-size:12px">
                <el-tag size="small" :type="certExpiringSoon(row.agent_cert_expires_at) ? 'warning' : 'success'">
                  {{ formatTime(row.agent_cert_expires_at, 'MM-DD') }}
                </el-tag>
              </div>
              <span v-else style="color:#c0c4cc;font-size:12px">-</span>
            </template>
          </el-table-column>
          <el-table-column :label="t('nodeServer.onlineSubscriptions')" width="55" align="center">
            <template #default="{row}">
              <el-tag v-if="onlineSummary[row.id] && onlineSummary[row.id].online_subscriptions > 0" type="success" size="small" effect="dark">
                {{ onlineSummary[row.id].online_subscriptions }}
              </el-tag>
              <span v-else style="color:#c0c4cc;font-size:12px">0</span>
            </template>
          </el-table-column>
          <el-table-column :label="t('common.actions')" width="140" fixed="right">
            <template #default="{row}">
              <el-button size="small" type="primary" @click="goToDetail(row)">{{ t('nodeServer.detail') }}</el-button>
              <el-dropdown trigger="click" @command="(cmd) => handleServerAction(cmd, row)" size="small">
                <el-button size="small">{{ t('nodeServer.actions') }} <el-icon><ArrowDown /></el-icon></el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item command="deploy" v-if="row.bootstrap_required && $can('admin')">{{ t('nodeServer.deployCommand') }}</el-dropdown-item>
                    <el-dropdown-item command="status" divided>{{ t('nodeServer.realtimeStatus') }}</el-dropdown-item>
                    <el-dropdown-item command="control" v-if="$can('super_admin')">{{ t('nodeServer.controlCore') }}</el-dropdown-item>
                    <el-dropdown-item command="sync" v-if="$can('super_admin')">{{ t('nodeServer.syncConfig') }}</el-dropdown-item>
                    <el-dropdown-item command="tags" divided>{{ t('nodeServer.tags') }}</el-dropdown-item>
                    <el-dropdown-item command="revoke" v-if="row.agent_cert_expires_at && $can('admin')">{{ t('nodeServer.revokeCert') }}</el-dropdown-item>
                    <el-dropdown-item command="reconnect" v-if="$can('admin')">{{ t('nodeServer.forceReconnect') }}</el-dropdown-item>
                    <el-dropdown-item command="resumeTraffic" v-if="row.traffic_suspended" divided style="color:#e6a23c">⚠ {{ t('nodeServer.resumeTraffic') }}</el-dropdown-item>
                    <el-dropdown-item command="updateAgent" v-if="!row.is_local && $can('admin')" divided>{{ t('nodeServer.updateAgent') }}</el-dropdown-item>
                    <el-dropdown-item command="edit" divided>{{ t('common.edit') }}</el-dropdown-item>
                    <el-dropdown-item command="delete" v-if="!row.is_local && $can('admin')">{{ t('common.delete') }}</el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </template>
          </el-table-column>
        </el-table>
        <MobileCards
          v-else
          :rows="servers"
          title-field="name"
          :fields="serverCardFields"
        >
          <template #actions="{ row }">
            <el-button size="small" type="primary" @click="goToDetail(row)">{{ t('nodeServer.detail') }}</el-button>
            <el-dropdown trigger="click" @command="(cmd) => handleServerAction(cmd, row)" size="small">
              <el-button size="small">{{ t('nodeServer.actions') }} <el-icon><ArrowDown /></el-icon></el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="deploy" v-if="row.bootstrap_required && $can('admin')">{{ t('nodeServer.deployCommand') }}</el-dropdown-item>
                  <el-dropdown-item command="status" divided>{{ t('nodeServer.realtimeStatus') }}</el-dropdown-item>
                  <el-dropdown-item command="control" v-if="$can('super_admin')">{{ t('nodeServer.controlCore') }}</el-dropdown-item>
                  <el-dropdown-item command="sync" v-if="$can('super_admin')">{{ t('nodeServer.syncConfig') }}</el-dropdown-item>
                  <el-dropdown-item command="tags" divided>{{ t('nodeServer.tags') }}</el-dropdown-item>
                  <el-dropdown-item command="revoke" v-if="row.agent_cert_expires_at && $can('admin')">{{ t('nodeServer.revokeCert') }}</el-dropdown-item>
                  <el-dropdown-item command="reconnect" v-if="$can('admin')">{{ t('nodeServer.forceReconnect') }}</el-dropdown-item>
                  <el-dropdown-item command="resumeTraffic" v-if="row.traffic_suspended" divided style="color:#e6a23c">⚠ {{ t('nodeServer.resumeTraffic') }}</el-dropdown-item>
                  <el-dropdown-item command="updateAgent" v-if="!row.is_local && $can('admin')" divided>{{ t('nodeServer.updateAgent') }}</el-dropdown-item>
                  <el-dropdown-item command="edit" divided>{{ t('common.edit') }}</el-dropdown-item>
                  <el-dropdown-item command="delete" v-if="!row.is_local && $can('admin')">{{ t('common.delete') }}</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>
        </MobileCards>
        <PaginationBar v-model:page="serverPage" v-model:page-size="serverPageSize" :total="serverTotal" @change="loadServers" />
      </el-tab-pane>

      <!-- ════════════ Dashboard Tab ════════════ -->
      <el-tab-pane :label="t('nodeServer.dashboard')" name="dashboard">
        <div v-loading="dashboardLoading">
          <el-row :gutter="16" style="margin-bottom:16px">
            <el-col :span="6">
              <el-card shadow="hover">
                <div style="text-align:center">
                  <div style="font-size:32px;font-weight:700;color:#409eff">{{ dashboard.total || 0 }}</div>
                  <div style="color:#909399">{{ t('nodeServer.totalServers') }}</div>
                </div>
              </el-card>
            </el-col>
            <el-col :span="6">
              <el-card shadow="hover">
                <div style="text-align:center">
                  <div style="font-size:32px;font-weight:700;color:#67c23a">{{ dashboard.online || 0 }}</div>
                  <div style="color:#909399">{{ t('nodeServer.onlineServers') }}</div>
                </div>
              </el-card>
            </el-col>
            <el-col :span="6">
              <el-card shadow="hover">
                <div style="text-align:center">
                  <div style="font-size:32px;font-weight:700;color:#f56c6c">{{ dashboard.offline || 0 }}</div>
                  <div style="color:#909399">{{ t('nodeServer.offlineServers') }}</div>
                </div>
              </el-card>
            </el-col>
            <el-col :span="6">
              <el-card shadow="hover">
                <div style="text-align:center">
                  <div style="font-size:32px;font-weight:700;color:#e6a23c">{{ dashboard.degraded || 0 }}</div>
                  <div style="color:#909399">{{ t('nodeServer.degradedServers') }}</div>
                </div>
              </el-card>
            </el-col>
          </el-row>

        <el-card shadow="hover" style="margin-bottom:16px">
          <template #header><span>{{ t('nodeServer.regionOverview') }}</span></template>
          <el-table :data="regions" v-loading="regionsLoading" size="small">
            <el-table-column prop="region" :label="t('nodeServer.region')" width="100" />
            <el-table-column prop="region_label" :label="t('nodeServer.regionLabel')" width="120" />
            <el-table-column prop="total" :label="t('nodeServer.total')" width="80" />
            <el-table-column prop="online" :label="t('nodeServer.online')" width="80" />
            <el-table-column prop="offline" :label="t('nodeServer.offline')" width="80" />
            <el-table-column prop="billable" :label="t('nodeServer.billable')" width="80" />
          </el-table>
        </el-card>

        <el-row :gutter="16">
            <el-col :span="12">
              <el-card shadow="hover">
                <template #header><span>{{ t('nodeServer.speedTest') }}</span></template>
                <div style="margin-bottom:8px">
                  <el-button size="small" type="primary" :loading="speedTestLoading" @click="runSpeedTestAll">{{ t('nodeServer.testAll') }}</el-button>
                  <el-button size="small" @click="loadSpeedTestResults">{{ t('common.refresh') }}</el-button>
                </div>
                <el-table :data="speedTestResults" size="small" max-height="300">
                  <el-table-column prop="server_id" label="ID" width="50" />
                  <el-table-column prop="server_name" :label="t('common.name')" width="120" show-overflow-tooltip />
                  <el-table-column :label="t('nodeServer.latency')" width="100">
                    <template #default="{row}">
                      <el-tag size="small" :type="latencyTag(row.latency_ms)">{{ row.latency_ms >= 0 ? row.latency_ms + 'ms' : 'N/A' }}</el-tag>
                    </template>
                  </el-table-column>
                  <el-table-column prop="error" :label="t('nodeServer.error')" show-overflow-tooltip />
                </el-table>
              </el-card>
            </el-col>
            <el-col :span="12">
              <el-card shadow="hover">
                <template #header><span>{{ t('nodeServer.trafficStats') }}</span></template>
                <div v-if="trafficStats && trafficStats.enabled" style="padding:2px 0">
                  <div v-if="trafficStats.stats">
                    <div v-for="(val, key) in trafficStats.stats" :key="key" style="margin-bottom:12px">
                      <div style="display:flex;justify-content:space-between;align-items:center;font-size:12px;margin-bottom:4px">
                        <span style="color:#606266;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;margin-right:8px">{{ key }}</span>
                        <span style="color:#303133;font-weight:500;white-space:nowrap">{{ val }}</span>
                      </div>
                      <el-progress :percentage="trafficPercent(val)" :show-text="false" :stroke-width="8"
                        :color="trafficPercent(val) >= 100 ? '#f56c6c' : '#409eff'" />
                    </div>
                  </div>
                </div>
                <el-empty v-else :description="t('common.noData')" :image-size="60" />
              </el-card>
            </el-col>
          </el-row>
        </div>
      </el-tab-pane>

      <!-- ════════════ CA 证书中心 Tab ════════════ -->
      <el-tab-pane :label="t('nodeServer.caCenter')" name="ca">
        <el-row :gutter="16" style="margin-bottom:16px">
          <el-col :span="12">
            <el-card shadow="hover">
              <template #header>
                <div style="display:flex;justify-content:space-between;align-items:center">
                  <span>{{ t('nodeServer.caStatus') }}</span>
                  <el-tag v-if="caStatus && caStatus.rotation_in_progress" type="warning" size="small">{{ t('nodeServer.caRotating') }}</el-tag>
                  <el-tag v-else-if="caStatus && caStatus.initialized" type="success" size="small">{{ t('nodeServer.caNormal') }}</el-tag>
                </div>
              </template>
              <div v-loading="caLoading" style="font-size:13px;color:#606266">
                <template v-if="caStatus && caStatus.initialized">
                  <div v-if="caStatus.rotation_in_progress" style="margin-bottom:8px;padding:6px 10px;background:#fdf6ec;border-radius:4px;font-size:12px;color:#e6a23c">
                    {{ t('nodeServer.caRotationHint', { days: caStatus.overlap_window_days }) }}
                  </div>
                  <div v-for="(root, idx) in (caStatus.ca_roots || [])" :key="idx" style="border:1px solid #ebeef5;border-radius:6px;padding:10px 12px;margin-bottom:8px">
                    <div style="display:flex;gap:6px;margin-bottom:6px;flex-wrap:wrap">
                      <el-tag v-if="root.is_incoming" type="success" size="small">{{ t('nodeServer.caIncoming') }}</el-tag>
                      <el-tag v-if="root.is_outgoing" type="warning" size="small">{{ t('nodeServer.caOutgoing') }}</el-tag>
                      <el-tag v-if="root.in_overlap" type="danger" size="small">{{ t('nodeServer.caInOverlap') }}</el-tag>
                    </div>
                    <div style="line-height:1.8">
                      <div>{{ t('nodeServer.subject') }}: <code style="font-size:12px">{{ root.subject }}</code></div>
                      <div>{{ t('nodeServer.serial') }}: <code style="font-size:12px">{{ root.serial_number }}</code></div>
                      <div>{{ t('nodeServer.notBefore') }}: {{ formatTime(root.not_before) }}</div>
                      <div>{{ t('nodeServer.notAfter') }}: <span :style="{color: certExpiringSoon(root.not_after) ? '#e6a23c' : '#67c23a'}">{{ formatTime(root.not_after) }}</span></div>
                      <div style="color:#909399;font-size:12px">{{ t('nodeServer.caExpiresIn', { remaining: root.expires_in }) }}</div>
                    </div>
                  </div>
                </template>
                <el-empty v-else :description="t('nodeServer.caNotInit')" :image-size="60" />
              </div>
            </el-card>
          </el-col>
          <el-col :span="12">
            <el-card shadow="hover">
              <template #header><span>{{ t('nodeServer.userSyncQueue') }}</span></template>
              <div style="display:flex;gap:8px;margin-bottom:12px;align-items:center">
                <el-tag size="small">{{ t('nodeServer.pendingCount') }}: {{ retryQueue.pending_count || 0 }}</el-tag>
                <el-button size="small" @click="processRetryQueueManual" :loading="retryProcessing">{{ t('nodeServer.processQueue') }}</el-button>
                <el-button size="small" @click="loadRetryQueue">{{ t('common.refresh') }}</el-button>
              </div>
            </el-card>
          </el-col>
        </el-row>

        <el-card shadow="hover" style="margin-bottom:16px">
          <template #header><span>{{ t('nodeServer.agentCerts') }}</span></template>
          <el-table :data="agentCerts" v-loading="agentCertsLoading" size="small">
            <el-table-column prop="server_id" label="ID" width="50" />
            <el-table-column prop="server_name" :label="t('common.name')" width="140" show-overflow-tooltip />
            <el-table-column prop="hostname" :label="t('nodeServer.hostname')" width="160" show-overflow-tooltip />
            <el-table-column prop="cert_serial" :label="t('nodeServer.certSerial')" width="140" show-overflow-tooltip />
            <el-table-column :label="t('nodeServer.certExpiry')" width="155">
              <template #default="{row}">
                <el-tag size="small" :type="certExpiringSoon(row.expires_at) ? 'warning' : 'success'" v-if="row.expires_at">
                  {{ formatTime(row.expires_at) }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="agent_status" :label="t('nodeServer.agentStatus')" width="90" />
            <el-table-column :label="t('common.actions')" width="120">
              <template #default="{row}">
                <el-button size="small" @click="renewCert(row.server_id)">{{ t('nodeServer.renew') }}</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>

        <el-card shadow="hover" style="margin-bottom:16px">
          <template #header><span>{{ t('nodeServer.certRevocations') }}</span></template>
          <el-table :data="revocations" v-loading="revocationsLoading" size="small">
            <el-table-column prop="server_id" label="ID" width="50" />
            <el-table-column prop="cert_serial" :label="t('nodeServer.certSerial')" width="160" show-overflow-tooltip />
            <el-table-column prop="reason" :label="t('nodeServer.reason')" width="140" />
            <el-table-column :label="t('nodeServer.revokedAt')" width="160">
              <template #default="{row}">{{ formatTime(row.revoked_at) }}</template>
            </el-table-column>
            <el-table-column :label="t('nodeServer.effectiveAt')" width="160">
              <template #default="{row}">{{ formatTime(row.effective_at) }}</template>
            </el-table-column>
          </el-table>
        </el-card>

        <el-card shadow="hover">
          <template #header>
            <div style="display:flex;justify-content:space-between;align-items:center">
              <span>{{ t('nodeServer.distributionQueue') }}</span>
              <el-select v-model="distQueueFilter" size="small" style="width:120px" @change="loadDistQueue">
                <el-option label="All" value="" />
                <el-option label="Pending" value="pending" />
                <el-option label="Delivered" value="delivered" />
                <el-option label="Failed" value="failed" />
              </el-select>
            </div>
          </template>
          <el-table :data="distQueue" v-loading="distQueueLoading" size="small">
            <el-table-column prop="id" label="ID" width="50" />
            <el-table-column prop="server_id" :label="t('nodeServer.serverId')" width="80" />
            <el-table-column prop="certificate_id" :label="t('nodeServer.certId')" width="80" />
            <el-table-column prop="cert_type" :label="t('nodeServer.certType')" width="100" />
            <el-table-column prop="status" :label="t('common.status')" width="90">
              <template #default="{row}">
                <el-tag size="small" :type="distStatusTag(row.status)">{{ row.status }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="attempts" :label="t('nodeServer.attempts')" width="80" />
            <el-table-column prop="last_error" :label="t('nodeServer.error')" show-overflow-tooltip />
            <el-table-column :label="t('common.actions')" width="140">
              <template #default="{row}">
                <el-button size="small" v-if="row.status === 'failed'" @click="retryDist(row.id)">{{ t('nodeServer.retry') }}</el-button>
                <el-button size="small" type="danger" @click="deleteDistItem(row.id)">{{ t('common.delete') }}</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>

      <!-- ════════════ 审计日志 Tab ════════════ -->
      <el-tab-pane :label="t('nodeServer.auditLogs')" name="audit">
        <div style="display:flex;gap:8px;margin-bottom:16px;align-items:center;flex-wrap:wrap">
          <el-select v-model="auditFilter.server_id" clearable filterable :placeholder="t('nodeServer.serverId')" size="small" style="width:160px" @change="resetAuditPage">
            <el-option v-for="s in auditServerOptions" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
          <el-select v-model="auditFilter.action" clearable filterable :placeholder="t('nodeServer.actionType')" size="small" style="width:180px" @change="resetAuditPage">
            <el-option v-for="a in auditActionOptions" :key="a.value" :label="a.label" :value="a.value" />
          </el-select>
          <el-button size="small" type="primary" @click="resetAuditPage">{{ t('common.search') }}</el-button>
          <el-button size="small" v-if="$can('admin')" @click="exportAuditLogs">{{ t('common.export') }}</el-button>
        </div>
        <el-table :data="auditLogs" v-loading="auditLoading" stripe size="small">
          <el-table-column prop="id" label="ID" width="50" />
          <el-table-column :label="t('nodeServer.serverId')" min-width="140">
            <template #default="{row}">{{ row.server_name || row.server_id || '-' }}</template>
          </el-table-column>
          <el-table-column :label="t('nodeServer.adminId')" min-width="100">
            <template #default="{row}">{{ row.admin_name || row.admin_id || '-' }}</template>
          </el-table-column>
          <el-table-column prop="action" :label="t('nodeServer.actionType')" width="140" />
          <el-table-column prop="detail" :label="t('nodeServer.detail')" show-overflow-tooltip />
          <el-table-column prop="source_ip" :label="t('nodeServer.sourceIp')" width="120" />
          <el-table-column :label="t('common.status')" width="70">
            <template #default="{row}">
              <el-tag size="small" :type="row.success ? 'success' : 'danger'">{{ row.success ? '✓' : '✗' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="t('common.time')" width="155">
            <template #default="{row}">{{ formatTime(row.created_at) }}</template>
          </el-table-column>
        </el-table>
        <PaginationBar v-model:page="auditPage" v-model:page-size="auditPageSize" :total="auditTotal" @change="loadAuditLogs" />
      </el-tab-pane>

    </el-tabs>

    <!-- ════════════ Quick Add Node Dialog (kubeadm join style) ════════════ -->
    <el-dialog v-model="quickAddVisible" :title="t('nodeServer.addNode')" width="480px">
      <el-alert type="info" :closable="false" style="margin-bottom:16px"
                :title="t('nodeServer.quickAddHint')" />
      <el-form :model="quickForm" label-width="80px" size="default">
        <el-form-item :label="t('nodeServer.name')" required>
          <el-input v-model="quickForm.name" :placeholder="t('nodeServer.namePlaceholder')" @keyup.enter="quickAddSave" />
        </el-form-item>
        <el-form-item :label="t('nodeServer.ipAddress')" required>
          <el-input v-model="quickForm.hostname" :placeholder="t('nodeServer.ipAddressPlaceholder')" @keyup.enter="quickAddSave" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="quickAddVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="quickAddSaving" @click="quickAddSave">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- ════════════ Create/Edit Dialog (full form, edit only) ════════════ -->
    <el-dialog v-model="formVisible" :title="formMode === 'create' ? t('nodeServer.addServer') : t('nodeServer.editServer')" width="680px" @close="onFormClose">
      <el-form :model="form" label-width="140px" size="small">
        <el-divider content-position="left">{{ t('nodeServer.basicInfo') }}</el-divider>
        <el-form-item :label="t('common.name')" required>
          <el-input v-model="form.name" :placeholder="t('nodeServer.namePlaceholder')" />
        </el-form-item>
        <el-form-item :label="t('nodeServer.hostname')" required>
          <el-input v-model="form.hostname" :placeholder="t('nodeServer.hostnamePlaceholder')" />
        </el-form-item>
        <el-form-item :label="t('nodeServer.region')">
          <el-input v-model="form.region" :placeholder="t('nodeServer.regionPlaceholder')" />
        </el-form-item>
        <el-form-item :label="t('nodeServer.regionLabel')">
          <el-input v-model="form.region_label" :placeholder="t('nodeServer.regionLabelPlaceholder')" />
        </el-form-item>

        <el-divider content-position="left">{{ t('nodeServer.nodeTypeSection') }}</el-divider>
        <el-form-item :label="t('nodeServer.nodeType')">
          <el-select v-model="form.node_type" style="width:100%">
            <el-option v-for="opt in nodeTypeOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
          </el-select>
        </el-form-item>
        <template v-if="form.node_type === 'primary_backup'">
          <el-form-item :label="t('nodeServer.partnerServerId')">
            <el-select v-model="form.partner_server_id" filterable clearable style="width:100%" :placeholder="t('nodeServer.partnerServerPlaceholder')">
              <el-option v-for="s in partnerOptions" :key="s.id" :label="`${s.name} (${s.hostname})`" :value="s.id" :disabled="s.id === editingId" />
            </el-select>
            <div style="color:#909399;font-size:11px;margin-top:4px">{{ t('nodeServer.primaryBackupHint') }}</div>
          </el-form-item>
          <el-form-item :label="t('nodeServer.failoverRole')">
            <el-select v-model="form.failover_role" style="width:100%" clearable>
              <el-option label="Primary" value="primary" />
              <el-option label="Backup" value="backup" />
            </el-select>
          </el-form-item>
          <el-form-item :label="t('nodeServer.failoverPriority')">
            <el-input-number v-model="form.failover_priority" :min="0" controls-position="right" style="width:100%" />
          </el-form-item>
          <el-form-item :label="t('nodeServer.recoverModeLabel')">
            <el-radio-group v-model="form.recover_mode" style="width:100%">
              <el-radio-button label="auto_switch_back">{{ t('nodeServer.recoverModeAuto') }}</el-radio-button>
              <el-radio-button label="stay_on_promoted">{{ t('nodeServer.recoverModeStay') }}</el-radio-button>
            </el-radio-group>
            <div style="color:#909399;font-size:11px;margin-top:4px">{{ t('nodeServer.recoverModeHint') }}</div>
          </el-form-item>
          <el-form-item v-if="form.partner_server_id">
            <el-checkbox v-model="form.auto_set_reverse">{{ t('nodeServer.autoSetReverse') }}</el-checkbox>
            <div style="color:#909399;font-size:11px;margin-top:4px">{{ t('nodeServer.autoSetReverseHint') }}</div>
          </el-form-item>
        </template>
        <template v-if="form.node_type === 'mirror'">
          <el-form-item :label="t('nodeServer.partnerServerId')">
            <el-select v-model="form.partner_server_id" filterable clearable style="width:100%" :placeholder="t('nodeServer.partnerServerPlaceholder')">
              <el-option v-for="s in mirrorTargetOptions" :key="s.id" :label="`${s.name} (${s.hostname})`" :value="s.id" :disabled="s.id === editingId" />
            </el-select>
            <div style="color:#909399;font-size:11px;margin-top:4px">{{ t('nodeServer.mirrorTargetHint') }}</div>
          </el-form-item>
          <el-form-item v-if="form.partner_server_id">
            <el-checkbox v-model="form.auto_set_reverse">{{ t('nodeServer.autoSetReverse') }}</el-checkbox>
            <div style="color:#909399;font-size:11px;margin-top:4px">{{ t('nodeServer.autoSetReverseHint') }}</div>
          </el-form-item>
        </template>
        <el-form-item :label="t('nodeServer.chainConfig')" v-if="form.node_type === 'chain_entry' || form.node_type === 'chain_exit'">
          <el-input v-model="form.chain_config" type="textarea" :rows="4" :placeholder="t('nodeServer.chainConfigPlaceholder')" />
        </el-form-item>

        <el-divider content-position="left">{{ t('nodeServer.serverInfo') }}</el-divider>
        <el-form-item :label="t('nodeServer.agentPort')">
          <el-input-number v-model="form.agent_port" :min="1" :max="65535" controls-position="right" style="width:100%" />
        </el-form-item>
        <el-form-item :label="t('nodeServer.supportedCores')">
          <el-select v-model="form.supported_cores_arr" multiple style="width:100%" :placeholder="t('nodeServer.supportedCoresPlaceholder')">
            <el-option label="Xray" value="xray" />
            <el-option label="sing-box" value="singbox" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('nodeServer.maxInbounds')">
          <el-input-number v-model="form.max_inbounds" :min="0" controls-position="right" style="width:100%" />
        </el-form-item>
        <el-form-item :label="t('nodeServer.publicIP')">
          <el-input v-model="form.public_ip" :placeholder="t('nodeServer.publicIPPlaceholder')" />
        </el-form-item>
        <el-form-item :label="t('nodeServer.internalIP')">
          <el-input v-model="form.internal_ip" :placeholder="t('nodeServer.internalIPPlaceholder')" />
        </el-form-item>
        <el-form-item :label="t('nodeServer.bandwidthMbps')">
          <el-input-number v-model="form.bandwidth_mbps" :min="0" controls-position="right" style="width:100%" />
        </el-form-item>
        <el-divider content-position="left">{{ t('nodeServer.subCapacity') }}</el-divider>
        <el-form-item :label="t('nodeServer.subMaxConnections')">
          <el-input-number v-model="form.sub_max_connections" :min="0" controls-position="right" style="width:100%" />
          <div style="color:#909399;font-size:12px;margin-top:4px">{{ t('nodeServer.subMaxConnectionsHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('nodeServer.subMaxBandwidthMbps')">
          <el-input-number v-model="form.sub_max_bandwidth_mbps" :min="0" controls-position="right" style="width:100%" />
          <div style="color:#909399;font-size:12px;margin-top:4px">{{ t('nodeServer.subMaxBandwidthMbpsHint') }}</div>
        </el-form-item>

        <el-divider content-position="left">{{ t('nodeServer.advanced') }}</el-divider>
        <el-form-item :label="t('nodeServer.billingEnabled')">
          <el-switch v-model="form.billing_enabled" />
          <span style="margin-left:8px;color:#909399;font-size:12px">{{ t('nodeServer.billingHint') }}</span>
        </el-form-item>
        <el-form-item :label="t('nodeServer.enabled')">
          <el-switch v-model="form.enabled" />
        </el-form-item>

        <el-divider content-position="left">{{ t('nodeServer.trafficQuotaSection') }}</el-divider>
        <el-form-item :label="t('nodeServer.egressInterface')">
          <div style="display:flex;gap:8px;width:100%">
            <el-select v-model="form.egress_interface" clearable filterable allow-create style="flex:1" :placeholder="t('nodeServer.egressInterfacePlaceholder')">
              <el-option v-for="iface in interfaceOptions" :key="iface" :label="iface" :value="iface" />
            </el-select>
            <el-button size="small" @click="loadInterfaces" :loading="interfacesLoading" style="flex-shrink:0">{{ t('nodeServer.refresh') }}</el-button>
          </div>
          <div style="color:#909399;font-size:11px;margin-top:4px">{{ t('nodeServer.egressInterfaceHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('nodeServer.trafficQuota')">
          <el-input-number v-model="form.traffic_quota_gb" :min="0" :step="500" controls-position="right" style="width:100%" />
          <div style="color:#909399;font-size:11px;margin-top:4px">{{ t('nodeServer.trafficQuotaUnit') }}</div>
        </el-form-item>
        <el-form-item :label="t('nodeServer.trafficQuotaResetDay')">
          <el-input-number v-model="form.traffic_quota_reset_day" :min="1" :max="28" controls-position="right" style="width:100%" />
          <div style="color:#909399;font-size:11px;margin-top:4px">{{ t('nodeServer.trafficResetDayHint') }}</div>
        </el-form-item>

        <el-form-item :label="t('nodeServer.sortOrder')">
          <el-input-number v-model="form.sort_order" :min="0" controls-position="right" style="width:100%" />
        </el-form-item>
        <el-form-item :label="t('nodeServer.metadata')">
          <el-input v-model="form.metadata" type="textarea" :rows="2" placeholder="{}" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="formVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="formSaving" @click="saveForm">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- ════════════ Detail Dialog ════════════ -->
    <el-dialog v-model="detailVisible" :title="t('nodeServer.serverDetail')" width="780px">
      <el-descriptions v-if="detailData" :column="2" border size="small">
        <el-descriptions-item label="ID">{{ detailData.id }}</el-descriptions-item>
        <el-descriptions-item :label="t('common.name')">{{ detailData.name }}</el-descriptions-item>
        <el-descriptions-item :label="t('nodeServer.hostname')">{{ detailData.hostname }}</el-descriptions-item>
        <el-descriptions-item :label="t('nodeServer.region')">{{ detailData.region || '-' }} ({{ detailData.region_label || '-' }})</el-descriptions-item>
        <el-descriptions-item :label="t('nodeServer.nodeType')">{{ nodeTypeLabel(detailData.node_type) }}</el-descriptions-item>
        <el-descriptions-item :label="t('nodeServer.agentStatus')">
          <el-tag size="small" :type="statusTag(detailData.agent_status)">{{ statusLabel(detailData.agent_status) }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item :label="t('nodeServer.agentVersion')">{{ detailData.agent_version || '-' }}</el-descriptions-item>
        <el-descriptions-item :label="t('nodeServer.lastHeartbeat')">{{ formatTime(detailData.last_heartbeat_at) || '-' }}</el-descriptions-item>
        <el-descriptions-item :label="t('nodeServer.agentPort')">{{ detailData.agent_port }}</el-descriptions-item>
        <el-descriptions-item :label="t('nodeServer.publicIP')">{{ detailData.public_ip || '-' }}</el-descriptions-item>
        <el-descriptions-item :label="t('nodeServer.internalIP')">{{ detailData.internal_ip || '-' }}</el-descriptions-item>
        <el-descriptions-item :label="t('nodeServer.bandwidthMbps')">{{ detailData.bandwidth_mbps || '-' }} Mbps</el-descriptions-item>
        <el-descriptions-item :label="t('nodeServer.billingEnabled')">
          <el-tag size="small" :type="detailData.billing_enabled ? 'success' : 'info'">{{ detailData.billing_enabled ? t('common.yes') : t('common.no') }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item :label="t('nodeServer.isLocal')">
          <el-tag size="small" :type="detailData.is_local ? 'warning' : 'info'">{{ detailData.is_local ? t('common.yes') : t('common.no') }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item :label="t('nodeServer.supportedCores')">{{ detailData.supported_cores }}</el-descriptions-item>
        <el-descriptions-item :label="t('nodeServer.maxInbounds')">{{ detailData.max_inbounds || t('common.unlimited') }}</el-descriptions-item>
        <el-descriptions-item :label="t('nodeServer.certExpiry')">
          <span v-if="detailData.agent_cert_expires_at">{{ formatTime(detailData.agent_cert_expires_at) }}</span>
          <span v-else>-</span>
        </el-descriptions-item>
        <el-descriptions-item :label="t('nodeServer.bootstrapStatus')">
          <el-tag v-if="detailData.bootstrap_required" size="small" type="warning">{{ t('nodeServer.bootstrapPending') }}</el-tag>
          <el-tag v-else-if="detailData.bootstrap_used_at" size="small" type="success">{{ t('nodeServer.bootstrapDone') }}</el-tag>
          <el-tag v-else size="small" type="info">{{ t('nodeServer.bootstrapNotGenerated') }}</el-tag>
        </el-descriptions-item>

        <el-descriptions-item :label="t('nodeServer.egressInterface')">{{ detailData.egress_interface || '-' }}</el-descriptions-item>
        <el-descriptions-item :label="t('nodeServer.trafficQuota')">
          <span v-if="detailData.traffic_quota_bytes > 0">{{ formatBytes(detailData.traffic_quota_bytes) }}</span>
          <span v-else>{{ t('nodeServer.trafficUnlimited') }}</span>
        </el-descriptions-item>
        <el-descriptions-item :label="t('nodeServer.monthlyUsedTraffic')">
          <span v-if="detailData.traffic_quota_bytes > 0">
            {{ formatBytes(detailData.traffic_used_bytes) }}
            <el-progress :percentage="Math.min((detailData.traffic_used_bytes / detailData.traffic_quota_bytes) * 100, 100)"
              :color="detailData.traffic_suspended ? '#f56c6c' : (detailData.traffic_used_bytes / detailData.traffic_quota_bytes > 0.8 ? '#e6a23c' : '#409eff')"
              :stroke-width="6" :show-text="true" style="width:120px;display:inline-block;vertical-align:middle;margin-left:8px" />
          </span>
          <span v-else>-</span>
        </el-descriptions-item>
        <el-descriptions-item :label="t('nodeServer.billingDay')">{{ detailData.traffic_quota_reset_day || 1 }}{{ t('nodeServer.billingDaySuffix') }}</el-descriptions-item>
        <el-descriptions-item :label="t('nodeServer.trafficStatus')">
          <el-tag v-if="detailData.traffic_suspended" size="small" type="danger">{{ t('nodeServer.trafficSuspendedOverQuota') }}</el-tag>
          <el-tag v-else-if="detailData.traffic_quota_bytes > 0" size="small" type="success">{{ t('nodeServer.trafficNormal') }}</el-tag>
          <el-tag v-else size="small" type="info">{{ t('nodeServer.notConfigured') }}</el-tag>
        </el-descriptions-item>
      </el-descriptions>

      <el-divider v-if="realtimeStatus" content-position="left">{{ t('nodeServer.realtimeStatus') }}</el-divider>
      <div v-if="realtimeStatus" style="font-size:13px;line-height:2;color:#606266">
        <div v-for="(core, idx) in (realtimeStatus.cores || realtimeStatus.core_states || [])" :key="idx">
          <el-tag size="small" :type="core.state === 'running' ? 'success' : 'danger'">{{ core.core_type || core.type }}</el-tag>
          PID: {{ core.pid }} | {{ t('nodeServer.uptime') }}: {{ formatUptime(core.uptime_seconds) }}
        </div>
        <div v-if="realtimeStatus.cpu_usage !== undefined">{{ t('dashboard.systemInfo') }} CPU: {{ realtimeStatus.cpu_usage?.toFixed?.(1) || realtimeStatus.cpu_usage }}%</div>
      </div>

      <template #footer>
        <el-button @click="detailVisible = false">{{ t('common.close') }}</el-button>
      </template>
    </el-dialog>

    <!-- ════════════ Bootstrap Command Dialog ════════════ -->
    <el-dialog v-model="bootstrapVisible" :title="t('nodeServer.bootstrapCommand')" width="680px">
      <el-alert type="warning" :title="t('nodeServer.bootstrapWarning')" :closable="false" style="margin-bottom:12px" />
      <el-input v-model="bootstrapCmd" type="textarea" :rows="12" readonly />
      <div style="margin-top:8px;text-align:right">
        <el-button size="small" @click="copyToClipboard(bootstrapCmd)">{{ t('nodeServer.copyCmd') }}</el-button>
      </div>
    </el-dialog>

    <!-- ════════════ Install Script Dialog ════════════ -->
    <el-dialog v-model="installVisible" :title="t('nodeServer.installScript')" width="680px">
      <el-input v-model="installScript" type="textarea" :rows="14" readonly />
      <div style="margin-top:8px;text-align:right">
        <el-button size="small" @click="copyToClipboard(installScript)">{{ t('nodeServer.copyCmd') }}</el-button>
      </div>
    </el-dialog>

    <!-- ════════════ One-Line Install Command Dialog ════════════ -->
    <el-dialog v-model="installCmdVisible" :title="t('nodeServer.nodeDeployCommand')" width="720px" :close-on-click-modal="false">
      <el-alert type="success" :closable="false" style="margin-bottom:16px"
                :title="t('nodeServer.deployCommandHint')" />
      <el-input v-model="installCommand" type="textarea" :rows="5" readonly style="font-family:monospace;font-size:13px" />
      <div style="margin-top:12px;display:flex;justify-content:space-between;align-items:center">
        <span style="color:#909399;font-size:12px">{{ t('nodeServer.deployCommandTokenHint') }}</span>
        <div>
          <el-button size="small" @click="copyToClipboard(installCommand)">{{ t('nodeServer.copyCmd') }}</el-button>
          <el-button size="small" type="primary" @click="installCmdVisible = false">{{ t('common.close') }}</el-button>
        </div>
      </div>
    </el-dialog>

    <!-- ════════════ Core Control Dialog ════════════ -->
    <el-dialog v-model="controlVisible" :title="t('nodeServer.controlCore')" width="420px">
      <el-form label-width="100px" size="small">
        <el-form-item :label="t('nodeServer.coreType')">
          <el-select v-model="controlForm.core_type" style="width:100%">
            <el-option label="Xray" value="xray" />
            <el-option label="sing-box" value="singbox" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('nodeServer.action')">
          <el-radio-group v-model="controlForm.action">
            <el-radio-button label="start">{{ t('nodeServer.start') }}</el-radio-button>
            <el-radio-button label="stop">{{ t('nodeServer.stop') }}</el-radio-button>
            <el-radio-button label="restart">{{ t('nodeServer.restart') }}</el-radio-button>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="controlVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="controlLoading" @click="doControl">{{ t('common.submit') }}</el-button>
      </template>
    </el-dialog>

    <!-- ════════════ Tags Dialog ════════════ -->
    <el-dialog v-model="tagsVisible" :title="t('nodeServer.tagsManagement')" width="580px">
      <div style="display:flex;gap:8px;margin-bottom:12px">
        <el-input v-model="newTagKey" placeholder="Key" size="small" style="width:120px" />
        <el-input v-model="newTagValue" placeholder="Value" size="small" style="flex:1" />
        <el-button size="small" type="primary" @click="addTag">+</el-button>
      </div>
      <el-table :data="tags" size="small">
        <el-table-column prop="tag_key" label="Key" width="140" />
        <el-table-column prop="tag_value" label="Value" />
        <el-table-column :label="t('common.actions')" width="80">
          <template #default="{row}">
            <el-button size="small" type="danger" @click="removeTag(row.tag_key)">×</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>

    <!-- ════════════ Update All Agents Dialog ════════════ -->
    <el-dialog v-model="updateAllVisible" :title="t('nodeServer.updateAllAgents')" width="560px">
      <div v-loading="updateAllOptionsLoading">
        <el-alert
          v-if="!hasLocalAgent && !updateAllOptionsLoading"
          type="info" :closable="false" style="margin-bottom:14px"
          :title="t('nodeServer.updateAllNoLocal')"
        />
        <el-radio-group v-model="updateAllSource" style="display:flex;flex-direction:column;gap:14px;align-items:flex-start">
          <el-radio v-if="hasLocalAgent" value="panel">
            {{ t('nodeServer.updateAllFromPanel') }}
            <el-tag size="small" type="info" style="margin-left:6px">{{ panelAgentVersion.version }}</el-tag>
          </el-radio>
          <div>
            <el-radio value="github">{{ t('nodeServer.updateAllFromGithub') }}</el-radio>
            <div v-if="updateAllSource === 'github'" style="margin-top:8px">
              <el-select
                v-model="updateAllVersion" size="small" style="width:260px"
                :placeholder="t('nodeServer.selectAgentVersion')"
              >
                <el-option v-for="rel in githubReleases" :key="rel.version" :label="rel.version" :value="rel.version">
                  <span>{{ rel.version }}</span>
                  <el-tag v-if="rel.prerelease" size="small" type="warning" style="margin-left:6px">pre</el-tag>
                </el-option>
              </el-select>
              <div style="font-size:11px;color:#999;margin-top:6px">{{ t('nodeServer.updateAllGithubHint') }}</div>
            </div>
          </div>
        </el-radio-group>
        <div v-if="updateAllSource === 'panel'" style="font-size:11px;color:#999;margin-top:10px">
          {{ t('nodeServer.updateAllPanelHint') }}
        </div>
      </div>
      <template #footer>
        <el-button @click="updateAllVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="warning" :loading="updateAllLoading" :disabled="!canUpdateAll" @click="confirmUpdateAll">
          {{ t('common.confirm') }}
        </el-button>
      </template>
    </el-dialog>

  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, onUnmounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import client from '../api/client'

// Default Agent gRPC port (matches constants.DefaultAgentGRPCPort in Go)
const DEFAULT_AGENT_GRPC_PORT = 18443

import { 
  listNodeServers, createNodeServer, updateNodeServer, deleteNodeServer, 
  getServerStatus, controlCore, generateBootstrapToken, getBootstrapCommand, 
  getAgentInstallScript, revokeAgentCertificate, forceReconnect, 
  listServerTags, addServerTag, removeServerTag, 
  getDashboardStatus, getRegionsOverview, getTrafficStats, 
  testAllServersSpeed, getSpeedTestResults, 
  getCAStatus, listAgentCerts, renewAgentCertificate, listRevocations, 
  getDistributionQueue, retryDistribution, deleteDistributionQueueItem, 
  getRetryQueueStatus, processRetryQueue, 
  listNodeAuditLogs, exportNodeAuditLogs, 
  syncServerConfig, 
  updateAllAgents, 
  getAgentVersion,
  getAgentReleases,
  resumeTrafficSuspended, listInterfaces, 
} from '../api/nodeServer'
import { useQueryState } from '../composables/useQueryState'
import { usePagination } from '../composables/usePagination'
import { useIsMobile } from '../composables/useIsMobile'
import PaginationBar from '../components/PaginationBar.vue'
import MobileCards from '../components/MobileCards.vue'

const { t } = useI18n()
const router = useRouter()

// ─── Active Tab ───
const activeTab = useQueryState('tab', 'servers')

// ─── Agent Update All ───
// 面板本地有可用的 Agent 二进制 + 版本号时，允许「用本地二进制」推送；
// 否则（未配置）只能从 GitHub Release 里选版本，避免本地没东西还点更新。
const updateAllLoading = ref(false)
const updateAllVisible = ref(false)
const updateAllOptionsLoading = ref(false)
const panelAgentVersion = ref(null)
const githubReleases = ref([])
const updateAllVersion = ref('')
const updateAllSource = ref('github')

const hasLocalAgent = computed(() => {
  const v = panelAgentVersion.value?.version
  return !!v && v !== 'unknown'
})
const canUpdateAll = computed(() =>
  updateAllSource.value === 'panel' ? hasLocalAgent.value : !!updateAllVersion.value
)

async function openUpdateAllDialog() {
  updateAllVisible.value = true
  await loadUpdateAllOptions()
}

async function loadUpdateAllOptions() {
  updateAllOptionsLoading.value = true
  try {
    const [ver, rel] = await Promise.allSettled([getAgentVersion(), getAgentReleases(20)])
    panelAgentVersion.value = ver.status === 'fulfilled' ? ver.value : null
    githubReleases.value = rel.status === 'fulfilled' ? (rel.value?.releases || []) : []
    if (!updateAllVersion.value && githubReleases.value.length > 0) {
      updateAllVersion.value = rel.value?.latest_version || githubReleases.value[0].version
    }
    updateAllSource.value = hasLocalAgent.value ? 'panel' : 'github'
  } finally {
    updateAllOptionsLoading.value = false
  }
}

async function confirmUpdateAll() {
  const source = updateAllSource.value
  const version = source === 'panel' ? '' : updateAllVersion.value
  if (source === 'github' && !version) return
  updateAllLoading.value = true
  try {
    await updateAllAgents({ version, download_url: '', checksum: '', source })
    ElMessage.success(t('nodeServer.updateAllAgentsPushed'))
    updateAllVisible.value = false
    loadServers()
  } catch (e) {
    // 拦截器已提示
  } finally {
    updateAllLoading.value = false
  }
}

// ─── Server List ───
const hasRemoteAgents = ref(false)
const servers = ref([])
const { page: serverPage, pageSize: serverPageSize } = usePagination({ pageKey: 'srvPage', pageSizeKey: 'srvSize' })
const serverTotal = ref(0)
const loading = ref(false)
const filterRegion = ref('')
const filterNodeType = ref('')
const regionOptions = ref(['HK', 'JP', 'US', 'SG', 'EU', 'CN'])

const nodeTypeOptions = computed(() => [
  { value: 'standalone', label: t('nodeServer.standaloneLabel') },
  { value: 'mirror', label: t('nodeServer.mirrorLabel') },
  { value: 'primary_backup', label: t('nodeServer.primaryBackupLabel') },
  { value: 'chain_entry', label: t('nodeServer.chainEntryLabel') },
  { value: 'chain_exit', label: t('nodeServer.chainExitLabel') },
])

// ─── Dashboard ───
const dashboard = ref({})
const dashboardLoading = ref(false)
const regions = ref([])
const regionsLoading = ref(false)
const trafficStats = ref(null)
const speedTestResults = ref([])
const speedTestLoading = ref(false)

// ─── CA Center ───
const caStatus = ref(null)
const caLoading = ref(false)
const agentCerts = ref([])
const agentCertsLoading = ref(false)
const revocations = ref([])
const revocationsLoading = ref(false)
const distQueue = ref([])
const distQueueLoading = ref(false)
const distQueueFilter = ref('')
const retryQueue = ref({})
const retryProcessing = ref(false)

// ─── Audit Logs ───
const auditLogs = ref([])
const auditLoading = ref(false)
const auditTotal = ref(0)
const { page: auditPage, pageSize: auditPageSize, limit: auditLimit, offset: auditOffset } = usePagination({ defaultSize: 20, pageKey: 'auditPage', pageSizeKey: 'auditSize' })
const auditFilter = reactive({ server_id: '', action: '' })
const auditServerOptions = ref([])
const auditActionOptions = [
  { value: 'create', label: 'create' },
  { value: 'update', label: 'update' },
  { value: 'delete', label: 'delete' },
  { value: 'control_core', label: 'control_core' },
  { value: 'sync_config', label: 'sync_config' },
  { value: 'generate_token', label: 'generate_token' },
  { value: 'revoke_cert', label: 'revoke_cert' },
  { value: 'renew_cert', label: 'renew_cert' },
  { value: 'distribute_cert', label: 'distribute_cert' },
  { value: 'distribute_cert_all', label: 'distribute_cert_all' },
  { value: 'associate_cert', label: 'associate_cert' },
  { value: 'dissociate_cert', label: 'dissociate_cert' },
  { value: 'update_agent', label: 'update_agent' },
  { value: 'update_all_agents', label: 'update_all_agents' },
  { value: 'update_node_type', label: 'update_node_type' },
  { value: 'sync_mirror', label: 'sync_mirror' },
  { value: 'trigger_failover', label: 'trigger_failover' },
  { value: 'save_protocol', label: 'save_protocol' },
  { value: 'save_protocol_section', label: 'save_protocol_section' },
  { value: 'control_proxy', label: 'control_proxy' },
  { value: 'sync_proxy', label: 'sync_proxy' },
  { value: 'update_ratelimit', label: 'update_ratelimit' },
  { value: 'user_create', label: 'user_create' },
  { value: 'user_update', label: 'user_update' },
  { value: 'user_delete', label: 'user_delete' },
  { value: 'cert_create', label: 'cert_create' },
  { value: 'cert_update', label: 'cert_update' },
  { value: 'cert_delete', label: 'cert_delete' },
  { value: 'cert_issue', label: 'cert_issue' },
  { value: 'cert_renew', label: 'cert_renew' },
  { value: 'xray_add_inbound', label: 'xray_add_inbound' },
  { value: 'xray_remove_inbound', label: 'xray_remove_inbound' },
  { value: 'xray_add_outbound', label: 'xray_add_outbound' },
  { value: 'xray_remove_outbound', label: 'xray_remove_outbound' },
  { value: 'xray_add_user', label: 'xray_add_user' },
  { value: 'xray_remove_user', label: 'xray_remove_user' },
  { value: 'xray_apply_routing', label: 'xray_apply_routing' },
  { value: 'xray_add_routing_rule', label: 'xray_add_routing_rule' },
  { value: 'xray_remove_routing_rule', label: 'xray_remove_routing_rule' },
  { value: 'xray_set_balancer', label: 'xray_set_balancer' },
  { value: 'xray_reset_stats', label: 'xray_reset_stats' },
]

function loadAuditServerOptions() {
  listNodeServers({ limit: 500 }).then((r) => {
    auditServerOptions.value = (r.items || []).map((s) => ({ id: s.id, name: s.name }))
  }).catch(() => {})
}

// ─── Create/Edit Form ───
const formVisible = ref(false)
const formMode = ref('edit')
const formSaving = ref(false)
const editingId = ref(null)
const form = reactive({
  name: '', hostname: '', region: '', region_label: '',
  node_type: 'standalone', partner_server_id: 0, failover_role: '', failover_priority: 0,
  recover_mode: 'auto_switch_back', // 故障转移回切模式
  chain_config: '', agent_port: DEFAULT_AGENT_GRPC_PORT,
  supported_cores_arr: ['xray', 'singbox'],
  max_inbounds: 0, public_ip: '', internal_ip: '', bandwidth_mbps: 0,
  sub_max_connections: 0, sub_max_bandwidth_mbps: 0,
  billing_enabled: true, enabled: true, sort_order: 0, metadata: '',
  // 流量配额
  egress_interface: '', traffic_quota_gb: 0, traffic_quota_reset_day: 1,
  // 关系配置
  auto_set_reverse: true,
})

// ─── All servers list for partner dropdown ───
const allServersList = ref([])

// Partner options for primary_backup: all remote servers except self
const partnerOptions = computed(() => {
  return allServersList.value.filter(s => !s.is_local && s.id !== editingId.value)
})

// Mirror target options: only standalone and primary_backup nodes
const mirrorTargetOptions = computed(() => {
  return allServersList.value.filter(s => 
    !s.is_local && s.id !== editingId.value && 
    (s.node_type === 'standalone' || s.node_type === 'primary_backup')
  )
})

async function loadAllServersList() {
  try {
    const res = await listNodeServers({ limit: 500, offset: 0 })
    allServersList.value = res.items || []
  } catch (e) { /* noop */ }
}

// ─── Interfaces for egress selection ───
const interfaceOptions = ref([])
const interfacesLoading = ref(false)

// ─── Quick Add (kubeadm join style) ───
const quickAddVisible = ref(false)
const quickAddSaving = ref(false)
const quickForm = reactive({ name: '', hostname: '' })

// ─── Detail Dialog ───
const detailVisible = ref(false)
const detailData = ref(null)
const realtimeStatus = ref(null)

// ─── Bootstrap / Install ───
const bootstrapVisible = ref(false)
const bootstrapCmd = ref('')
const installVisible = ref(false)
const installScript = ref('')
const installCmdVisible = ref(false)
const installCommand = ref('')

// ─── Core Control ───
const controlVisible = ref(false)
const controlLoading = ref(false)
const controlForm = reactive({ core_type: 'xray', action: 'restart' })
const controlServerId = ref(null)

// ─── Tags ───
const tagsVisible = ref(false)
const tags = ref([])
const newTagKey = ref('')
const newTagValue = ref('')
const tagServerId = ref(null)

// ─── Auto-refresh timer ───
let refreshTimer = null

// ════════════ Data Loading ════════════

const onlineSummary = ref({})

// ─── Server ID→Name map for partner/relationship display ───
const serverMap = ref({})

async function loadServerMap() {
  try {
    const res = await listNodeServers({ limit: 500, offset: 0 })
    const map = {}
    ;(res.items || []).forEach(s => { map[s.id] = s.name })
    serverMap.value = map
  } catch (e) { /* noop */ }
}

function goToPartner(partnerId) {
  if (!partnerId) return
  const row = servers.value.find(s => s.id === partnerId)
  if (row) {
    goToDetail(row)
  } else {
    // Partner not on current page, navigate by ID
    router.push(`/node-servers/${partnerId}/detail`)
  }
}

async function loadOnlineSummary() {
  try {
    const res = await client.get('/node-servers/online-summary')
    onlineSummary.value = (res && res.nodes) || {}
  } catch (e) { /* noop */ }
}

async function loadServers() {
  loading.value = true
  try {
    const params = { limit: serverPageSize.value, offset: (serverPage.value - 1) * serverPageSize.value }
    if (filterRegion.value) params.region = filterRegion.value
    if (filterNodeType.value) params.node_type = filterNodeType.value
    const res = await listNodeServers(params)
    servers.value = res.items || []
    hasRemoteAgents.value = !!res.has_remote_agent
    serverTotal.value = res.total || 0
    // Update region options from data
    const regionsSet = new Set(regionOptions.value)
    servers.value.forEach(s => { if (s.region) regionsSet.add(s.region) })
    regionOptions.value = [...regionsSet]
    // Refresh server map if new servers appeared
    if (serverTotal.value > Object.keys(serverMap.value).length) {
      loadServerMap()
    }
  } catch (e) {
    // error handled by interceptor
  } finally {
    loading.value = false
  }
  loadOnlineSummary()
}

async function loadDashboard() {
  dashboardLoading.value = true
  try {
    const [dash, regs, traffic] = await Promise.allSettled([
      getDashboardStatus(),
      getRegionsOverview(),
      getTrafficStats(),
    ])
    if (dash.status === 'fulfilled') dashboard.value = dash.value || {}
    if (regs.status === 'fulfilled') regions.value = regs.value || []
    if (traffic.status === 'fulfilled') trafficStats.value = traffic.value
  } finally {
    dashboardLoading.value = false
  }
}

async function loadSpeedTestResults() {
  try {
    const res = await getSpeedTestResults()
    speedTestResults.value = Array.isArray(res) ? res : (res?.results || [])
  } catch (e) { /* noop */ }
}

async function runSpeedTestAll() {
  speedTestLoading.value = true
  try {
    await testAllServersSpeed()
    ElMessage.success(t('nodeServer.speedTestStarted'))
    setTimeout(() => loadSpeedTestResults(), 3000)
  } finally {
    speedTestLoading.value = false
  }
}

async function loadCA() {
  caLoading.value = true
  try {
    caStatus.value = await getCAStatus()
  } finally {
    caLoading.value = false
  }
}

async function loadAgentCerts() {
  agentCertsLoading.value = true
  try {
    const res = await listAgentCerts()
    agentCerts.value = res.certs || []
  } finally {
    agentCertsLoading.value = false
  }
}

async function loadRevocations() {
  revocationsLoading.value = true
  try {
    const res = await listRevocations()
    revocations.value = res.revocations || []
  } finally {
    revocationsLoading.value = false
  }
}

async function loadDistQueue() {
  distQueueLoading.value = true
  try {
    const params = { limit: 50, offset: 0 }
    if (distQueueFilter.value) params.status = distQueueFilter.value
    const res = await getDistributionQueue(params)
    distQueue.value = res.items || []
  } finally {
    distQueueLoading.value = false
  }
}

async function loadRetryQueue() {
  try {
    retryQueue.value = await getRetryQueueStatus()
  } catch (e) { /* noop */ }
}

async function processRetryQueueManual() {
  retryProcessing.value = true
  try {
    await processRetryQueue()
    ElMessage.success(t('nodeServer.queueProcessed'))
    loadRetryQueue()
  } finally {
    retryProcessing.value = false
  }
}

function resetAuditPage() {
  auditPage.value = 1
  loadAuditLogs()
}

async function loadAuditLogs() {
  auditLoading.value = true
  try {
    const params = { limit: auditLimit.value, offset: auditOffset.value }
    if (auditFilter.server_id) params.server_id = auditFilter.server_id
    if (auditFilter.action) params.action = auditFilter.action
    const res = await listNodeAuditLogs(params)
    auditLogs.value = res.items || []
    auditTotal.value = res.total || 0
  } finally {
    auditLoading.value = false
  }
}

async function exportAuditLogs() {
  try {
    const data = await exportNodeAuditLogs()
    const blob = new Blob([data], { type: 'text/csv' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'node-audit-logs.csv'
    a.click()
    URL.revokeObjectURL(url)
  } catch (e) { /* noop */ }
}

// ════════════ Tab Change Handler ════════════

function onTabChange(tab) {
  try { window.__tabLog = (window.__tabLog || '') + 'onTabChange:' + JSON.stringify(tab) + ';' } catch (e) {}
  switch (tab) {
    case 'servers': loadServers(); break
    case 'dashboard':
      loadDashboard()
      loadSpeedTestResults()
      break
    case 'ca':
      loadCA(); loadAgentCerts(); loadRevocations(); loadDistQueue(); loadRetryQueue()
      break
    case 'audit': loadAuditLogs(); loadAuditServerOptions(); break
  }
}

// ════════════ Create/Edit ════════════

function showCreateDialog() {
  // kubeadm join style: just name + IP, then immediately get deploy command
  quickForm.name = ''
  quickForm.hostname = ''
  quickAddVisible.value = true
}

async function quickAddSave() {
  if (!quickForm.name || !quickForm.hostname) {
    ElMessage.error(t('nodeServer.nameAndIPRequired'))
    return
  }
  quickAddSaving.value = true
  try {
    const payload = {
      name: quickForm.name,
      hostname: quickForm.hostname,
      node_type: 'standalone',
      agent_port: DEFAULT_AGENT_GRPC_PORT,
      supported_cores: JSON.stringify(['xray', 'singbox']),
      max_inbounds: 0,
      public_ip: quickForm.hostname,
      internal_ip: quickForm.hostname,
      bandwidth_mbps: 0,
      billing_enabled: true,
      enabled: true,
      sort_order: 0,
      metadata: '',
    }
    const res = await createNodeServer(payload)
    quickAddVisible.value = false
    ElMessage.success(t('common.success'))
    // Immediately show the one-line install command
    // Note: axios interceptor already unwrapped response.data.data,
    // so res IS the server object directly.
    if (res?.bootstrap_token) {
      const proto = window.location.protocol.replace(':', '')
      const host = window.location.hostname
      const port = window.location.port || (proto === 'https' ? '443' : '80')
      const tlsParam = proto === 'https' ? '&tls=true' : ''
      // Only send grpc_port for a non-zero node port; nodes without one use
      // the panel-wide gRPC port setting.
      const grpcPortParam = res.agent_port ? `&grpc_port=${res.agent_port}` : ''
      installCommand.value =
        `bash <(curl -fsSL "${proto}://${host}:${port}/api/v1/agent/install?id=${res.id}&token=${res.bootstrap_token}${grpcPortParam}&port=${port}${tlsParam}")`
      installCmdVisible.value = true
    }
    loadServers()
  } finally {
    quickAddSaving.value = false
  }
}

function showEditDialog(row) {
  formMode.value = 'edit'
  editingId.value = row.id
  Object.assign(form, {
    name: row.name, hostname: row.hostname,
    region: row.region, region_label: row.region_label,
    node_type: row.node_type, partner_server_id: row.partner_server_id,
    failover_role: row.failover_role, failover_priority: row.failover_priority,
    recover_mode: row.recover_mode || 'auto_switch_back',
    chain_config: row.chain_config || '', agent_port: row.agent_port,
    supported_cores_arr: parseCores(row.supported_cores),
    max_inbounds: row.max_inbounds, public_ip: row.public_ip,
    sub_max_connections: row.sub_max_connections || 0, sub_max_bandwidth_mbps: row.sub_max_bandwidth_mbps || 0,
    internal_ip: row.internal_ip, bandwidth_mbps: row.bandwidth_mbps,
    billing_enabled: row.billing_enabled, enabled: row.enabled,
    sort_order: row.sort_order, metadata: row.metadata || '',
    // 流量配额
    egress_interface: row.egress_interface || '',
    traffic_quota_gb: row.traffic_quota_bytes ? Math.round(row.traffic_quota_bytes / (1024 * 1024 * 1024)) : 0,
    traffic_quota_reset_day: row.traffic_quota_reset_day || 1,
    // 关系配置
    auto_set_reverse: true,
  })
  formVisible.value = true
  loadAllServersList()
  loadInterfaces()
}

function onFormClose() {
  formVisible.value = false
}

async function saveForm() {
  if (!form.name || !form.hostname) {
    ElMessage.error(t('nodeServer.nameHostRequired'))
    return
  }
  formSaving.value = true
  try {
    const payload = {
      name: form.name,
      hostname: form.hostname,
      region: form.region,
      region_label: form.region_label,
      node_type: form.node_type,
      partner_server_id: form.partner_server_id,
      failover_role: form.failover_role,
      failover_priority: form.failover_priority,
      recover_mode: form.recover_mode,
      chain_config: form.chain_config,
      agent_port: form.agent_port,
      supported_cores: JSON.stringify(form.supported_cores_arr),
      max_inbounds: form.max_inbounds,
      public_ip: form.public_ip,
      internal_ip: form.internal_ip,
      bandwidth_mbps: form.bandwidth_mbps,
      billing_enabled: form.billing_enabled,
      enabled: form.enabled,
      sort_order: form.sort_order,
      metadata: form.metadata,
      // 流量配额
      egress_interface: form.egress_interface,
      traffic_quota_bytes: form.traffic_quota_gb * 1024 * 1024 * 1024,
      traffic_quota_reset_day: form.traffic_quota_reset_day,
    }
    if (formMode.value === 'create') {
      // This path is no longer used — quickAddSave handles creation.
      // Kept for safety in case formVisible is opened in 'create' mode.
      const res = await createNodeServer(payload)
      ElMessage.success(t('common.success'))
      if (res?.bootstrap_token) {
        const proto = window.location.protocol.replace(':', '')
        const host = window.location.hostname
        const port = window.location.port || (proto === 'https' ? '443' : '80')
        const tlsParam = proto === 'https' ? '&tls=true' : ''
        const grpcPortParam = res.agent_port ? `&grpc_port=${res.agent_port}` : ''
        installCommand.value =
          `bash <(curl -fsSL "${proto}://${host}:${port}/api/v1/agent/install?id=${res.id}&token=${res.bootstrap_token}${grpcPortParam}&port=${port}${tlsParam}")`
        installCmdVisible.value = true
      }
    } else {
      await updateNodeServer(editingId.value, payload)
      ElMessage.success(t('common.success'))

      // Auto-set reverse relationship on partner node
      if (form.auto_set_reverse && form.partner_server_id && form.partner_server_id !== editingId.value) {
        try {
          // For primary_backup: set partner's partner to this node, with opposite failover role
          if (form.node_type === 'primary_backup') {
            const reverseRole = form.failover_role === 'primary' ? 'backup' : (form.failover_role === 'backup' ? 'primary' : '')
            await updateNodeServer(form.partner_server_id, {
              partner_server_id: editingId.value,
              failover_role: reverseRole,
              node_type: 'primary_backup',
            })
          }
          // For mirror: unidirectional, no reverse needed
          ElMessage.success(t('nodeServer.autoSetReverse'))
        } catch (e) {
          // Non-fatal: partner update failed
          ElMessage.warning(t('nodeServer.autoSetReverse') + ' (partial)')
        }
      }
    }
    formVisible.value = false
    loadServers()
  } finally {
    formSaving.value = false
  }
}

// ════════════ Detail ════════════

function goToDetail(row) {
  router.push(`/node-servers/${row.id}/detail`)
}

async function showDetailDialog(row) {
  detailData.value = row
  realtimeStatus.value = null
  detailVisible.value = true
  try {
    realtimeStatus.value = await getServerStatus(row.id)
  } catch (e) { /* noop */ }
}

// ════════════ Server Actions ════════════

async function handleServerAction(cmd, row) {
  switch (cmd) {
case 'edit': showEditDialog(row); break
case 'delete': handleDelete(row); break
case 'deploy': showDeployCmd(row); break
case 'status': showDetailDialog(row); break
    case 'control': showControlDialog(row); break
    case 'sync': handleSyncConfig(row); break
    case 'tags': showTagsDialog(row); break
    case 'revoke': handleRevoke(row); break
    case 'reconnect': handleReconnect(row); break
    case 'resumeTraffic': handleResumeTraffic(row); break
    case 'updateAgent': goToDetail(row); break
  }
}

async function handleResumeTraffic(row) {
  try {
    await ElMessageBox.confirm(
      t('nodeServer.resumeTrafficConfirm', { name: row.name }),
      t('nodeServer.resumeTraffic'),
      { type: 'warning' }
    )
    await resumeTrafficSuspended(row.id)
    ElMessage.success(t('nodeServer.nodeResumed'))
    loadServers()
  } catch (e) {
    if (e !== 'cancel' && e?.response?.data?.message) {
      ElMessage.error(e.response.data.message)
    }
  }
}

async function loadInterfaces() {
  if (!editingId.value) return
  interfacesLoading.value = true
  try {
    const res = await listInterfaces(editingId.value)
    interfaceOptions.value = res || []
  } catch (e) {
    // ignore - might be offline
    interfaceOptions.value = []
  } finally {
    interfacesLoading.value = false
  }
}

async function handleDelete(row) {
  try {
    await ElMessageBox.confirm(t('nodeServer.deleteConfirm', { name: row.name }), t('common.delete'), { type: 'warning' })
    await deleteNodeServer(row.id)
    ElMessage.success(t('common.success'))
    loadServers()
  } catch (e) { /* cancelled */ }
}

async function showDeployCmd(row) {
  try {
    const proto = window.location.protocol.replace(':', '')
    const host = window.location.hostname
    const port = window.location.port || (proto === 'https' ? '443' : '80')
    const tlsParam = proto === 'https' ? '&tls=true' : ''
    const grpcPortParam = row.agent_port ? `&grpc_port=${row.agent_port}` : ''
    // Generate a fresh bootstrap token (interceptor already unwrapped, res = {server_id, token, expires_at, ...})
    const res = await generateBootstrapToken(row.id)
    const token = res?.token || ''
    if (!token) {
      ElMessage.error(t('nodeServer.cannotGenerateDeployToken'))
      return
    }
    installCommand.value =
      `bash <(curl -fsSL "${proto}://${host}:${port}/api/v1/agent/install?id=${row.id}&token=${token}${grpcPortParam}&port=${port}${tlsParam}")`
    installCmdVisible.value = true
  } catch (e) {
    ElMessage.error(t('nodeServer.generateDeployFailed'))
  }
}

function showControlDialog(row) {
  controlServerId.value = row.id
  controlForm.core_type = 'xray'
  controlForm.action = 'restart'
  controlVisible.value = true
}

async function doControl() {
  controlLoading.value = true
  try {
    await controlCore(controlServerId.value, controlForm.core_type, controlForm.action)
    ElMessage.success(t('common.success'))
    controlVisible.value = false
  } finally {
    controlLoading.value = false
  }
}

async function handleSyncConfig(row) {
  try {
    await ElMessageBox.confirm(t('nodeServer.syncConfirm', { name: row.name }), t('nodeServer.syncConfig'), { type: 'info' })
    await syncServerConfig(row.id, { core_type: 'xray' })
    ElMessage.success(t('common.success'))
  } catch (e) { /* cancelled */ }
}

async function handleRevoke(row) {
  try {
    const { value } = await ElMessageBox.prompt(t('nodeServer.revokeReason'), t('nodeServer.revokeCert'), {
      inputPlaceholder: 'manual_revocation',
      type: 'warning',
    })
    await revokeAgentCertificate(row.id, value || 'manual_revocation')
    ElMessage.success(t('common.success'))
    loadServers()
  } catch (e) { /* cancelled */ }
}

async function handleReconnect(row) {
  try {
    await ElMessageBox.confirm(t('nodeServer.reconnectConfirm', { name: row.name }), t('nodeServer.forceReconnect'), { type: 'warning' })
    await forceReconnect(row.id)
    ElMessage.success(t('common.success'))
  } catch (e) { /* cancelled */ }
}

// ════════════ Tags ════════════

async function showTagsDialog(row) {
  tagServerId.value = row.id
  tagsVisible.value = true
  try {
    const res = await listServerTags(row.id)
    tags.value = res.tags || []
  } catch (e) { /* noop */ }
}

async function addTag() {
  if (!newTagKey.value) return
  try {
    await addServerTag(tagServerId.value, newTagKey.value, newTagValue.value)
    newTagKey.value = ''
    newTagValue.value = ''
    const res = await listServerTags(tagServerId.value)
    tags.value = res.tags || []
  } catch (e) { /* noop */ }
}

async function removeTag(key) {
  try {
    await removeServerTag(tagServerId.value, key)
    const res = await listServerTags(tagServerId.value)
    tags.value = res.tags || []
  } catch (e) { /* noop */ }
}

// ════════════ CA Actions ════════════

async function renewCert(serverId) {
  try {
    await renewAgentCertificate(serverId)
    ElMessage.success(t('common.success'))
    loadAgentCerts()
  } catch (e) { /* noop */ }
}

async function retryDist(id) {
  try {
    await retryDistribution(id)
    ElMessage.success(t('common.success'))
    loadDistQueue()
  } catch (e) { /* noop */ }
}

async function deleteDistItem(id) {
  try {
    await deleteDistributionQueueItem(id)
    ElMessage.success(t('common.success'))
    loadDistQueue()
  } catch (e) { /* noop */ }
}

// ════════════ Helpers ════════════

function parseCores(s) {
  if (!s) return ['xray', 'singbox']
  try { return JSON.parse(s) } catch { return [] }
}

function nodeTypeLabel(type) {
  const map = { standalone: 'Standalone', mirror: 'Mirror', primary_backup: 'Primary/Backup', chain_entry: 'Chain Entry', chain_exit: 'Chain Exit' }
  return map[type] || type
}

function nodeTypeTag(type) {
  const map = { standalone: 'success', mirror: 'info', primary_backup: 'warning', chain_entry: 'primary', chain_exit: 'danger' }
  return map[type] || 'info'
}

function statusLabel(status) {
  const map = { online: t('nodeServer.online'), offline: t('nodeServer.offline'), degraded: t('nodeServer.degraded') }
  return map[status] || status
}

const { isMobile } = useIsMobile()
// 手机端卡片字段（与桌面服务器列表列对应）
function relationText(row) {
  const name = (id) => serverMap.value?.[id] || ('#' + id)
  if (row.node_type === 'mirror' && row.partner_server_id) return `${t('nodeServer.mirrorOf')} → ${name(row.partner_server_id)}`
  if (row.node_type === 'primary_backup' && row.partner_server_id) return `${row.failover_role === 'primary' ? t('nodeServer.failoverPrimary') : t('nodeServer.failoverBackup')} ⇄ ${name(row.partner_server_id)}`
  if (row.chain_entry_for?.length) return `${t('nodeServer.chainEntry')} → ${row.chain_entry_for.map(name).join(', ')}`
  if (row.chain_exit_for?.length) return `${t('nodeServer.chainExit')} ← ${row.chain_exit_for.map(name).join(', ')}`
  if (row.node_type === 'chain_entry') return t('nodeServer.chainEntry')
  if (row.node_type === 'chain_exit') return t('nodeServer.chainExit')
  return '-'
}
const serverCardFields = computed(() => [
  { label: t('nodeServer.hostname'), text: (row) => row.hostname || '-' },
  { label: t('nodeServer.region'), text: (row) => row.region || '-' },
  { label: t('nodeServer.nodeType'), text: (row) => nodeTypeLabel(row.node_type) },
  { label: t('nodeServer.relationship'), text: relationText },
  { label: t('nodeServer.agentStatus'), text: (row) => statusLabel(row.agent_status) + (row.agent_version ? ' · ' + row.agent_version : '') },
  { label: 'CPU / ' + t('dashboard.memory') + ' / ' + t('dashboard.disk'), text: (row) => row.resource_usage ? `${row.resource_usage.cpu_usage?.toFixed(1) || 0}% / ${row.resource_usage.mem_usage?.toFixed(1) || 0}% / ${row.resource_usage.disk_usage?.toFixed(1) || 0}%` : '-' },
  { label: t('nodeServer.monthlyTraffic'), text: (row) => row.traffic_quota_bytes > 0 ? `${formatBytes(row.traffic_used_bytes)} / ${formatBytes(row.traffic_quota_bytes)}` : (row.traffic_suspended ? t('nodeServer.trafficSuspended') : '-') },
  { label: t('nodeServer.lastHeartbeat'), text: (row) => row.last_heartbeat_at ? formatAgo(row.last_heartbeat_at) : '-' },
  { label: t('nodeServer.onlineSubscriptions'), text: (row) => String(onlineSummary.value?.[row.id]?.online_subscriptions ?? 0) },
])

function statusTag(status) {
  const map = { online: 'success', offline: 'danger', degraded: 'warning' }
  return map[status] || 'info'
}

function distStatusTag(status) {
  const map = { pending: 'warning', delivered: 'success', failed: 'danger', processing: 'primary' }
  return map[status] || 'info'
}

function latencyTag(ms) {
  if (ms < 0) return 'info'
  if (ms < 100) return 'success'
  if (ms < 300) return 'warning'
  return 'danger'
}

function certExpiringSoon(time) {
  if (!time) return false
  const d = new Date(time)
  const days = (d - Date.now()) / 86400000
  return days < 3
}

function formatTime(time, fmt) {
  if (!time) return ''
  const d = new Date(time)
  if (isNaN(d)) return String(time)
  if (fmt === 'MM-DD') return `${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
  return d.toLocaleString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

// formatAgo 相对时间（心跳列窄列显示，hover title 显示完整时间）
function formatAgo(time) {
  if (!time) return ''
  const d = new Date(time)
  if (isNaN(d)) return String(time)
  const diff = Date.now() - d.getTime()
  if (diff < 0) return t('nodeServer.justNow')
  if (diff < 60e3) return Math.max(1, Math.floor(diff / 1e3)) + t('nodeServer.secondsAgo')
  if (diff < 3600e3) return Math.floor(diff / 60e3) + t('nodeServer.minutesAgo')
  if (diff < 86400e3) return Math.floor(diff / 3600e3) + t('nodeServer.hoursAgo')
  return Math.floor(diff / 86400e3) + t('nodeServer.daysAgo')
}

// ─── 流量统计条形图：解析 "16.28 GB" → bytes，条长相对最大值 ───
function parseTrafficBytes(s) {
  if (typeof s !== 'string') return 0
  const m = s.match(/^([\d.]+)\s*(B|KB|MB|GB|TB|PB)$/i)
  if (!m) return 0
  const units = { B: 1, KB: 1024, MB: 1024 ** 2, GB: 1024 ** 3, TB: 1024 ** 4, PB: 1024 ** 5 }
  return parseFloat(m[1]) * (units[m[2].toUpperCase()] || 1)
}

function trafficPercent(val) {
  if (!trafficStats.value?.stats) return 0
  const max = Math.max(...Object.values(trafficStats.value.stats).map(parseTrafficBytes), 1)
  const b = parseTrafficBytes(val)
  if (b <= 0) return 0
  return Math.max(2, Math.min(Math.round((b / max) * 100), 100))
}

function formatUptime(seconds) {
  if (!seconds || seconds < 0) return '-'
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const mins = Math.floor((seconds % 3600) / 60)
  let s = ''
  if (days > 0) s += days + 'd '
  if (hours > 0) s += hours + 'h '
  s += mins + 'm'
  return s
}

function formatBytes(bytes) {
  if (!bytes || bytes <= 0) return '0B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  let i = 0
  let val = bytes
  while (val >= 1024 && i < units.length - 1) {
    val /= 1024
    i++
  }
  if (i >= 3) return val.toFixed(1) + units[i] // GB, TB 用1位小数
  return Math.round(val) + units[i] // B, KB, MB 整数
}

function copyToClipboard(text) {
  if (navigator.clipboard && window.isSecureContext) {
    navigator.clipboard.writeText(text).then(() => {
      ElMessage.success(t('nodeServer.copied'))
    }).catch(() => {
      fallbackCopy(text)
    })
  } else {
    fallbackCopy(text)
  }
}

function fallbackCopy(text) {
  const ta = document.createElement('textarea')
  ta.value = text
  ta.style.position = 'fixed'
  ta.style.left = '-9999px'
  document.body.appendChild(ta)
  ta.select()
  try {
    document.execCommand('copy')
    ElMessage.success(t('nodeServer.copied'))
  } catch (e) {
    ElMessage.error(t('nodeServer.copyFailed'))
  }
  document.body.removeChild(ta)
}

// ════════════ Lifecycle ════════════

// Load data for the active tab whenever it changes. This covers:
//  - the initial value restored from the ?tab= URL query by useQueryState
//  - the programmatic update from useQueryState, which does NOT trigger el-tabs'
//    @tab-change event (so a refreshed non-default tab like "dashboard" would
//    otherwise stay empty until the user manually clicks a tab)
//  - normal user tab clicks
watch(activeTab, (tab) => {
  try { window.__tabLog = (window.__tabLog || '') + 'watch:' + JSON.stringify(tab) + ';' } catch (e) {}
  onTabChange(tab)
}, { immediate: true })

onMounted(() => {
  // Load server ID→name map for relationship display
  loadServerMap()
  // Auto-refresh the active tab's data every 30s.
  refreshTimer = setInterval(() => {
    onTabChange(activeTab.value)
  }, 30000)
})

onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer)
})
</script>

<style scoped>
.status-dot {
  display: inline-block;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  margin-right: 4px;
  vertical-align: middle;
}
.status-dot.online { background: #67c23a; }
.status-dot.offline { background: #f56c6c; }
.status-dot.degraded { background: #e6a23c; }
</style>
