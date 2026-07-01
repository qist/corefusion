<template>
  <div>
    <el-tabs v-model="activeTab" type="border-card" @tab-change="onTabChange">

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
          </div>
          <el-button type="primary" size="small" @click="showCreateDialog">+ {{ t('nodeServer.addServer') }}</el-button>
        </div>

        <el-table :data="servers" v-loading="loading" stripe size="small">
          <el-table-column prop="id" label="ID" width="50" />
          <el-table-column prop="name" :label="t('common.name')" width="140" show-overflow-tooltip />
          <el-table-column prop="hostname" :label="t('nodeServer.hostname')" width="160" show-overflow-tooltip />
          <el-table-column :label="t('nodeServer.region')" width="90">
            <template #default="{row}">
              <el-tag size="small" type="info">{{ row.region || '-' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="t('nodeServer.nodeType')" width="110">
            <template #default="{row}">
              <el-tag size="small" :type="nodeTypeTag(row.node_type)">{{ nodeTypeLabel(row.node_type) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="t('nodeServer.agentStatus')" width="100">
            <template #default="{row}">
              <el-tag size="small" :type="statusTag(row.agent_status)">
                <span :class="['status-dot', row.agent_status]"></span>
                {{ statusLabel(row.agent_status) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="agent_version" :label="t('nodeServer.agentVersion')" width="90" show-overflow-tooltip />
          <el-table-column :label="t('nodeServer.lastHeartbeat')" width="155">
            <template #default="{row}">
              <span v-if="row.last_heartbeat_at" style="font-size:12px;color:#909399">{{ formatTime(row.last_heartbeat_at) }}</span>
              <span v-else style="color:#c0c4cc">-</span>
            </template>
          </el-table-column>
          <el-table-column :label="t('nodeServer.certStatus')" width="120">
            <template #default="{row}">
              <div v-if="row.agent_cert_expires_at" style="font-size:12px">
                <el-tag size="small" :type="certExpiringSoon(row.agent_cert_expires_at) ? 'warning' : 'success'">
                  {{ formatTime(row.agent_cert_expires_at, 'MM-DD') }}
                </el-tag>
              </div>
              <span v-else style="color:#c0c4cc;font-size:12px">-</span>
            </template>
          </el-table-column>
          <el-table-column :label="t('common.actions')" width="320" fixed="right">
            <template #default="{row}">
              <el-button size="small" type="primary" @click="goToDetail(row)">{{ t('nodeServer.detail') }}</el-button>
              <el-dropdown trigger="click" @command="(cmd) => handleServerAction(cmd, row)" size="small">
                <el-button size="small">{{ t('nodeServer.actions') }} <el-icon><ArrowDown /></el-icon></el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item command="bootstrap" v-if="row.bootstrap_required">{{ t('nodeServer.bootstrapCmd') }}</el-dropdown-item>
                    <el-dropdown-item command="install">{{ t('nodeServer.installScript') }}</el-dropdown-item>
                    <el-dropdown-item command="status" divided>{{ t('nodeServer.realtimeStatus') }}</el-dropdown-item>
                    <el-dropdown-item command="control">{{ t('nodeServer.controlCore') }}</el-dropdown-item>
                    <el-dropdown-item command="sync">{{ t('nodeServer.syncConfig') }}</el-dropdown-item>
                    <el-dropdown-item command="tags" divided>{{ t('nodeServer.tags') }}</el-dropdown-item>
                    <el-dropdown-item command="revoke" v-if="row.agent_cert_expires_at">{{ t('nodeServer.revokeCert') }}</el-dropdown-item>
                    <el-dropdown-item command="reconnect">{{ t('nodeServer.forceReconnect') }}</el-dropdown-item>
                    <el-dropdown-item command="edit" divided>{{ t('common.edit') }}</el-dropdown-item>
                    <el-dropdown-item command="delete" v-if="!row.is_local">{{ t('common.delete') }}</el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </template>
          </el-table-column>
        </el-table>
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
                <div v-if="trafficStats && trafficStats.enabled" style="font-size:13px;line-height:2">
                  <div v-if="trafficStats.stats">
                    <div v-for="(val, key) in trafficStats.stats" :key="key">{{ key }}: {{ val }}</div>
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
              <template #header><span>{{ t('nodeServer.caStatus') }}</span></template>
              <div v-loading="caLoading" style="font-size:13px;line-height:2;color:#606266">
                <template v-if="caStatus && caStatus.initialized">
                  <div>{{ t('nodeServer.subject') }}: <code style="font-size:12px">{{ caStatus.subject }}</code></div>
                  <div>{{ t('nodeServer.serial') }}: <code style="font-size:12px">{{ caStatus.serial }}</code></div>
                  <div>{{ t('nodeServer.notBefore') }}: {{ formatTime(caStatus.not_before) }}</div>
                  <div>{{ t('nodeServer.notAfter') }}: <span :style="{color: certExpiringSoon(caStatus.not_after) ? '#e6a23c' : '#67c23a'}">{{ formatTime(caStatus.not_after) }}</span></div>
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
        <div style="display:flex;gap:8px;margin-bottom:16px;align-items:center">
          <el-input v-model="auditFilter.server_id" :placeholder="t('nodeServer.serverId')" clearable size="small" style="width:100px" @keyup.enter="loadAuditLogs" />
          <el-input v-model="auditFilter.action" :placeholder="t('nodeServer.actionType')" clearable size="small" style="width:140px" @keyup.enter="loadAuditLogs" />
          <el-button size="small" type="primary" @click="loadAuditLogs">{{ t('common.search') }}</el-button>
          <el-button size="small" @click="exportAuditLogs">{{ t('common.export') }}</el-button>
        </div>
        <el-table :data="auditLogs" v-loading="auditLoading" stripe size="small">
          <el-table-column prop="id" label="ID" width="50" />
          <el-table-column prop="server_id" :label="t('nodeServer.serverId')" width="80" />
          <el-table-column prop="admin_id" :label="t('nodeServer.adminId')" width="80" />
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
        <div style="margin-top:8px;text-align:right;color:#909399;font-size:12px">{{ t('common.total', {count: auditTotal}) }}</div>
      </el-tab-pane>

    </el-tabs>

    <!-- ════════════ Create/Edit Dialog ════════════ -->
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
            <el-input-number v-model="form.partner_server_id" :min="0" controls-position="right" style="width:100%" />
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
        </template>
        <el-form-item :label="t('nodeServer.chainConfig')" v-if="form.node_type === 'chain_entry' || form.node_type === 'chain_exit'">
          <el-input v-model="form.chain_config" type="textarea" :rows="4" :placeholder='t(&apos;nodeServer.chainConfigPlaceholder&apos;)' />
        </el-form-item>

        <el-divider content-position="left">{{ t('nodeServer.serverInfo') }}</el-divider>
        <el-form-item :label="t('nodeServer.agentPort')">
          <el-input-number v-model="form.agent_port" :min="1" :max="65535" controls-position="right" style="width:100%" />
        </el-form-item>
        <el-form-item :label="t('nodeServer.supportedCores')">
          <el-select v-model="form.supported_cores_arr" multiple style="width:100%" :placeholder="t('nodeServer.supportedCoresPlaceholder')">
            <el-option label="Xray" value="xray" />
            <el-option label="sing-box" value="singbox" />
            <el-option label="Hysteria2" value="hysteria2" />
            <el-option label="TUIC" value="tuic" />
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

        <el-divider content-position="left">{{ t('nodeServer.advanced') }}</el-divider>
        <el-form-item :label="t('nodeServer.billingEnabled')">
          <el-switch v-model="form.billing_enabled" />
          <span style="margin-left:8px;color:#909399;font-size:12px">{{ t('nodeServer.billingHint') }}</span>
        </el-form-item>
        <el-form-item :label="t('nodeServer.enabled')">
          <el-switch v-model="form.enabled" />
        </el-form-item>
        <el-form-item :label="t('nodeServer.sortOrder')">
          <el-input-number v-model="form.sort_order" :min="0" controls-position="right" style="width:100%" />
        </el-form-item>
        <el-form-item :label="t('nodeServer.metadata')">
          <el-input v-model="form.metadata" type="textarea" :rows="2" :placeholder='&apos;{}&apos;' />
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

    <!-- ════════════ Core Control Dialog ════════════ -->
    <el-dialog v-model="controlVisible" :title="t('nodeServer.controlCore')" width="420px">
      <el-form label-width="100px" size="small">
        <el-form-item :label="t('nodeServer.coreType')">
          <el-select v-model="controlForm.core_type" style="width:100%">
            <el-option label="Xray" value="xray" />
            <el-option label="sing-box" value="singbox" />
            <el-option label="Hysteria2" value="hysteria2" />
            <el-option label="TUIC" value="tuic" />
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

  </div>
</template>

<script setup>
import { ref, reactive, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
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
} from '../api/nodeServer'

const { t } = useI18n()
const router = useRouter()

// ─── Active Tab ───
const activeTab = ref('servers')

// ─── Server List ───
const servers = ref([])
const loading = ref(false)
const filterRegion = ref('')
const filterNodeType = ref('')
const regionOptions = ref(['HK', 'JP', 'US', 'SG', 'EU', 'CN'])

const nodeTypeOptions = [
  { value: 'standalone', label: '独立节点 (standalone)' },
  { value: 'mirror', label: '镜像节点 (mirror)' },
  { value: 'primary_backup', label: '主备节点 (primary_backup)' },
  { value: 'chain_entry', label: '链式入口 (chain_entry)' },
  { value: 'chain_exit', label: '链式出口 (chain_exit)' },
]

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
const auditFilter = reactive({ server_id: '', action: '', limit: 50, offset: 0 })

// ─── Create/Edit Form ───
const formVisible = ref(false)
const formMode = ref('create')
const formSaving = ref(false)
const editingId = ref(null)
const form = reactive({
  name: '', hostname: '', region: '', region_label: '',
  node_type: 'standalone', partner_server_id: 0, failover_role: '', failover_priority: 0,
  chain_config: '', agent_port: 18443,
  supported_cores_arr: ['xray', 'singbox', 'hysteria2', 'tuic'],
  max_inbounds: 0, public_ip: '', internal_ip: '', bandwidth_mbps: 0,
  billing_enabled: true, enabled: true, sort_order: 0, metadata: '',
})

// ─── Detail Dialog ───
const detailVisible = ref(false)
const detailData = ref(null)
const realtimeStatus = ref(null)

// ─── Bootstrap / Install ───
const bootstrapVisible = ref(false)
const bootstrapCmd = ref('')
const installVisible = ref(false)
const installScript = ref('')

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

async function loadServers() {
  loading.value = true
  try {
    const params = {}
    if (filterRegion.value) params.region = filterRegion.value
    if (filterNodeType.value) params.node_type = filterNodeType.value
    const res = await listNodeServers(params)
    servers.value = res.items || []
    // Update region options from data
    const regionsSet = new Set(regionOptions.value)
    servers.value.forEach(s => { if (s.region) regionsSet.add(s.region) })
    regionOptions.value = [...regionsSet]
  } catch (e) {
    // error handled by interceptor
  } finally {
    loading.value = false
  }
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

async function loadAuditLogs() {
  auditLoading.value = true
  try {
    const params = { limit: auditFilter.limit, offset: auditFilter.offset }
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
  switch (tab) {
    case 'servers': loadServers(); break
    case 'dashboard': loadDashboard(); loadSpeedTestResults(); break
    case 'ca':
      loadCA(); loadAgentCerts(); loadRevocations(); loadDistQueue(); loadRetryQueue()
      break
    case 'audit': loadAuditLogs(); break
  }
}

// ════════════ Create/Edit ════════════

function showCreateDialog() {
  formMode.value = 'create'
  editingId.value = null
  Object.assign(form, {
    name: '', hostname: '', region: '', region_label: '',
    node_type: 'standalone', partner_server_id: 0, failover_role: '', failover_priority: 0,
    chain_config: '', agent_port: 18443,
    supported_cores_arr: ['xray', 'singbox', 'hysteria2', 'tuic'],
    max_inbounds: 0, public_ip: '', internal_ip: '', bandwidth_mbps: 0,
    billing_enabled: true, enabled: true, sort_order: 0, metadata: '',
  })
  formVisible.value = true
}

function showEditDialog(row) {
  formMode.value = 'edit'
  editingId.value = row.id
  Object.assign(form, {
    name: row.name, hostname: row.hostname,
    region: row.region, region_label: row.region_label,
    node_type: row.node_type, partner_server_id: row.partner_server_id,
    failover_role: row.failover_role, failover_priority: row.failover_priority,
    chain_config: row.chain_config || '', agent_port: row.agent_port,
    supported_cores_arr: parseCores(row.supported_cores),
    max_inbounds: row.max_inbounds, public_ip: row.public_ip,
    internal_ip: row.internal_ip, bandwidth_mbps: row.bandwidth_mbps,
    billing_enabled: row.billing_enabled, enabled: row.enabled,
    sort_order: row.sort_order, metadata: row.metadata || '',
  })
  formVisible.value = true
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
    }
    if (formMode.value === 'create') {
      await createNodeServer(payload)
      ElMessage.success(t('common.success'))
    } else {
      await updateNodeServer(editingId.value, payload)
      ElMessage.success(t('common.success'))
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
    case 'bootstrap': showBootstrapCmd(row); break
    case 'install': showInstallScript(row); break
    case 'status': showDetailDialog(row); break
    case 'control': showControlDialog(row); break
    case 'sync': handleSyncConfig(row); break
    case 'tags': showTagsDialog(row); break
    case 'revoke': handleRevoke(row); break
    case 'reconnect': handleReconnect(row); break
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

async function showBootstrapCmd(row) {
  try {
    // Generate token first
    await generateBootstrapToken(row.id)
    // Then get the command
    const cmd = await getBootstrapCommand(row.id)
    bootstrapCmd.value = cmd
    bootstrapVisible.value = true
  } catch (e) { /* noop */ }
}

async function showInstallScript(row) {
  try {
    const script = await getAgentInstallScript(row.id)
    installScript.value = script
    installVisible.value = true
  } catch (e) { /* noop */ }
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
  if (!s) return ['xray', 'singbox', 'hysteria2', 'tuic']
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

function copyToClipboard(text) {
  navigator.clipboard.writeText(text).then(() => {
    ElMessage.success(t('nodeServer.copied'))
  }).catch(() => {
    ElMessage.error(t('nodeServer.copyFailed'))
  })
}

// ════════════ Lifecycle ════════════

onMounted(() => {
  loadServers()
  // Auto-refresh server list every 30s when on servers tab
  refreshTimer = setInterval(() => {
    if (activeTab.value === 'servers') loadServers()
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
