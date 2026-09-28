<template>
  <div>
    <h2 style="margin-bottom:20px">{{ t('settings.title') }}</h2>
    <el-tabs v-model="activeTab">
      <!-- Panel Config -->
      <el-tab-pane :label="t('settings.panel.title')" name="panel">
        <el-card>
          <el-form :model="panelForm" label-width="140px" style="max-width:600px">
            <el-form-item :label="t('settings.panel.listenAddr')">
              <el-input v-model="panelForm.listen_addr" placeholder="0.0.0.0" />
            </el-form-item>
            <el-form-item :label="t('settings.panel.listenPort')">
              <el-input-number v-model="panelPort" :min="1" :max="65535" />
            </el-form-item>
            <el-form-item :label="t('settings.panel.dataDir')">
              <div style="display:flex;gap:8px;width:100%">
                <el-input v-model="envForm.safe_paths" placeholder="data" style="flex:1" />
                <el-button type="warning" @click="handleMigrateData" :loading="migrating">{{ t('settings.panel.migrate') }}</el-button>
              </div>
              <div style="font-size:11px;color:#999;margin-top:4px">
                {{ t('settings.panel.dataDirHint') }}
              </div>
            </el-form-item>
            <el-form-item :label="t('settings.panel.trustedProxies')">
              <el-input v-model="panelForm.trusted_proxies" placeholder="cloudflare" />
              <div style="font-size:11px;color:#999;margin-top:4px">
                {{ t('settings.panel.trustedProxiesHint') }}
              </div>
            </el-form-item>
            <el-form-item :label="t('settings.panel.webPath')">
              <el-input v-model="panelForm.web_path" placeholder="/" />
            </el-form-item>
            <el-form-item :label="t('settings.panel.timezone')">
              <el-select v-model="panelForm.timezone" filterable style="width:100%">
                <el-option v-for="tz in timezones" :key="tz" :label="tz" :value="tz" />
              </el-select>
            </el-form-item>
            <el-divider />
            <el-form-item :label="t('settings.panel.subPort')">
              <el-input-number v-model="subPort" :min="0" :max="65535" />
              <div class="hint">{{ t('settings.panel.subPortHint') }}</div>
            </el-form-item>
            <el-form-item :label="t('settings.panel.profileWebPageUrl')">
              <el-input v-model="panelForm.profile_web_page_url" placeholder="https://example.com/p" />
              <div class="hint">{{ t('settings.panel.profileWebPageUrlHint') }}</div>
            </el-form-item>
            <el-form-item :label="t('settings.panel.exportMaxRows')">
              <el-input-number v-model="panelForm.export_max_rows" :min="1000" :max="2000000" :step="1000" />
              <div class="hint">{{ t('settings.panel.exportMaxRowsHint') }}</div>
            </el-form-item>
            <el-form-item :label="t('settings.panel.passkeyRPIDs')">
              <el-input v-model="panelForm.passkey_rp_ids" placeholder="panel.example.com,localhost" />
              <div class="hint">{{ t('settings.panel.passkeyRPIDsHint') }}</div>
              <el-button size="small" style="margin-top:6px" :loading="rpIDAdding" @click="addCurrentDomain">
                {{ t('settings.panel.passkeyAddCurrent') }}
              </el-button>
              <div style="font-size:11px;color:#999;margin-top:4px">
                {{ t('settings.panel.passkeyAddCurrentHint') }}
              </div>
            </el-form-item>
            <el-form-item :label="t('settings.panel.mode')">
              <el-radio-group v-model="panelForm.mode">
                <el-radio value="full">{{ t('settings.panel.modeFull') }}</el-radio>
                <el-radio value="management_only">{{ t('settings.panel.modeManagementOnly') }}</el-radio>
              </el-radio-group>
              <div class="hint">{{ t('settings.panel.modeHint') }}</div>
            </el-form-item>
            <el-divider />
            <el-form-item :label="t('settings.panel.tlsEnabled')">
              <el-switch v-model="panelForm.tls_enabled" />
            </el-form-item>
            <el-form-item v-if="panelForm.tls_enabled" :label="t('settings.panel.tlsCert')">
              <el-select v-model="panelForm.tls_cert_id" style="width:100%" @change="onCertSelect" :placeholder="t('settings.panel.selectCert')">
                <el-option v-for="c in certificates" :key="c.id" :label="`${c.domain} (${c.issuer} ${c.key_type} ${c.key_bits})`" :value="c.id" />
              </el-select>
            </el-form-item>
            <el-form-item v-if="panelForm.tls_enabled" :label="t('settings.panel.tlsCertPath')">
              <el-input v-model="panelForm.tls_cert_path" />
            </el-form-item>
            <el-form-item v-if="panelForm.tls_enabled" :label="t('settings.panel.tlsKeyPath')">
              <el-input v-model="panelForm.tls_key_path" />
            </el-form-item>
            <el-divider />
            <el-form-item :label="t('settings.panel.agentGRPCPort')">
              <el-input-number v-model="agentGRPCPort" :min="1" :max="65535" />
              <div class="hint">{{ t('settings.panel.agentGRPCPortHint') }}</div>
            </el-form-item>
            <el-divider />
            <el-form-item :label="t('settings.panel.mtlsEnabled')">
              <el-switch v-model="panelForm.mtls_enabled" />
            </el-form-item>
            <el-form-item v-if="panelForm.mtls_enabled" :label="t('settings.panel.mtlsCAPath')">
              <div style="display:flex;gap:8px;width:100%">
                <el-input v-model="panelForm.mtls_ca_path" style="flex:1" />
                <el-upload :auto-upload="false" :show-file-list="false" :on-change="handleMTLSUpload" accept=".pem,.crt,.cer">
                  <el-button>{{ t('settings.panel.uploadCA') }}</el-button>
                </el-upload>
              </div>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="panelSaving" @click="savePanelConfig">{{ t('common.save') }}</el-button>
              <el-button type="warning" :loading="envSaving" @click="saveEnvConfig">{{ t('settings.panel.saveEnv') }}</el-button>
            </el-form-item>
          </el-form>
        </el-card>
      </el-tab-pane>

      <!-- Notification Config -->
      <el-tab-pane :label="t('settings.notification.title')" name="notification">
        <el-card style="margin-bottom:20px">
          <template #header><span>{{ t('settings.telegram.title') }}</span></template>
          <el-form :model="notifyForm" label-width="140px" style="max-width:600px">
            <el-form-item :label="t('settings.notification.enabled')">
              <el-switch v-model="notifyForm.telegram_enabled" />
            </el-form-item>
            <el-form-item v-if="notifyForm.telegram_enabled" :label="t('settings.telegram.botToken')">
              <el-input v-model="notifyForm.telegram_bot_token" type="password" show-password placeholder="完整格式：123456789:AAHxxx...（含冒号前的数字 ID）" />
              <div class="hint">{{ t('settings.telegram.botTokenHint') }}</div>
            </el-form-item>
            <el-form-item v-if="notifyForm.telegram_enabled" :label="t('settings.telegram.chatId')">
              <div style="display:flex;gap:8px;width:100%">
                <el-input v-model="notifyForm.telegram_chat_id" placeholder="-1001234567890" />
                <el-button :loading="chatIdFetching" @click="fetchChatId">{{ t('settings.telegram.fetchChatId') }}</el-button>
              </div>
              <div class="hint">{{ t('settings.telegram.chatIdHint') }}</div>
            </el-form-item>
            <el-form-item v-if="notifyForm.telegram_enabled">
              <el-button :loading="tgTesting" @click="testTelegram">{{ t('settings.telegram.test') }}</el-button>
            </el-form-item>
          </el-form>
        </el-card>

        <el-card style="margin-bottom:20px">
          <template #header><span>{{ t('settings.smtp.title') }}</span></template>
          <el-form :model="notifyForm" label-width="140px" style="max-width:600px">
            <el-form-item :label="t('settings.notification.enabled')">
              <el-switch v-model="notifyForm.smtp_enabled" />
            </el-form-item>
            <template v-if="notifyForm.smtp_enabled">
              <el-form-item :label="t('settings.smtp.host')">
                <el-input v-model="notifyForm.smtp_host" placeholder="smtp.example.com" />
              </el-form-item>
              <el-form-item :label="t('settings.smtp.port')">
                <el-select v-model="notifyForm.smtp_port" style="width:100%">
                  <el-option label="465 (SSL/TLS)" value="465" />
                  <el-option label="587 (STARTTLS)" value="587" />
                  <el-option :label="t('settings.smtp.port25')" value="25" />
                </el-select>
              </el-form-item>
              <el-form-item :label="t('settings.smtp.user')">
                <el-input v-model="notifyForm.smtp_user" />
              </el-form-item>
              <el-form-item :label="t('settings.smtp.pass')">
                <el-input v-model="notifyForm.smtp_pass" type="password" show-password />
              </el-form-item>
              <el-form-item :label="t('settings.smtp.from')">
                <el-select v-model="notifyForm.smtp_from" filterable allow-create style="width:100%" placeholder="noreply@example.com">
                  <el-option v-for="e in emails" :key="e.id" :label="e.email" :value="e.email" />
                </el-select>
              </el-form-item>
              <el-form-item :label="t('settings.smtp.tls')">
                <el-switch v-model="notifyForm.smtp_tls" />
              </el-form-item>
              <el-form-item>
                <el-button :loading="smtpTesting" @click="testSMTP">{{ t('settings.smtp.test') }}</el-button>
              </el-form-item>
            </template>
          </el-form>
        </el-card>

        <!-- Monitor Settings（监控类通知的频率/阈值，实时生效） -->
        <el-card style="margin-bottom:20px">
          <template #header><span>{{ t('settings.monitor.title') }}</span></template>
          <el-form label-width="160px" style="max-width:600px">
            <el-form-item :label="t('settings.monitor.nodeInterval')">
              <el-input-number v-model="monitorForm.node_interval_seconds" :min="30" :max="3600" />
              <span style="margin-left:8px;font-size:12px;color:#999">{{ t('settings.monitor.seconds') }}</span>
              <div class="hint">{{ t('settings.monitor.nodeIntervalHint') }}</div>
            </el-form-item>
            <el-form-item :label="t('settings.monitor.offlineTimeout')">
              <el-input-number v-model="monitorForm.node_offline_timeout_seconds" :min="30" :max="1800" />
              <span style="margin-left:8px;font-size:12px;color:#999">{{ t('settings.monitor.seconds') }}</span>
              <div class="hint">{{ t('settings.monitor.offlineTimeoutHint') }}</div>
            </el-form-item>
            <el-form-item :label="t('settings.monitor.quotaThreshold')">
              <el-input-number v-model="monitorForm.traffic_quota_threshold_percent" :min="0" :max="100" />
              <span style="margin-left:8px;font-size:12px;color:#999">%</span>
              <div class="hint">{{ t('settings.monitor.quotaThresholdHint') }}</div>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="monitorSaving" @click="saveMonitorConfig">{{ t('common.save') }}</el-button>
            </el-form-item>
          </el-form>
        </el-card>

        <el-card>
          <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:16px">
            <span style="font-weight:500">{{ t('settings.templates.title') }}</span>
            <div style="display:flex;gap:8px">
              <el-button size="small" link type="primary" @click="showVarsDoc">{{ t('settings.templates.varsDoc') }}</el-button>
              <el-button type="primary" size="small" @click="showAddTemplate">{{ t('settings.templates.add') }}</el-button>
            </div>
          </div>
          <el-table :data="templates" stripe>
            <el-table-column prop="name" :label="t('common.name')" width="150" />
            <el-table-column prop="type" label="Type" width="80">
              <template #default="{row}"><el-tag size="small">{{ row.type }}</el-tag></template>
            </el-table-column>
            <el-table-column :label="t('settings.templates.audience')" width="90">
              <template #default="{row}">
                <el-tag size="small" :type="row.audience === 'subscriber' ? 'warning' : 'info'">
                  {{ row.audience === 'subscriber' ? t('settings.templates.audienceSubscriber') : t('settings.templates.audienceAdmin') }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="subject" label="Subject" width="150" />
            <el-table-column prop="cron" label="Cron" width="120" />
            <el-table-column label="Enabled" width="80">
              <template #default="{row}"><el-tag :type="row.enabled?'success':'info'" size="small">{{ row.enabled ? 'ON' : 'OFF' }}</el-tag></template>
            </el-table-column>
            <el-table-column :label="t('common.actions')" width="220">
              <template #default="{row}">
                <el-button size="small" @click="showEditTemplate(row)">{{ t('common.edit') }}</el-button>
                <el-button size="small" type="success" plain @click="handleSendTemplate(row)" :title="t('settings.templates.sendHint')">{{ t('settings.templates.send') }}</el-button>
                <el-button v-if="row.audience === 'subscriber'" size="small" type="warning" plain @click="showBroadcastDialog(row)" :title="t('settings.templates.broadcastHint')">{{ t('settings.templates.broadcast') }}</el-button>
                <el-button size="small" type="danger" @click="handleDeleteTemplate(row)">{{ t('common.delete') }}</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>

        <!-- Template Variables Doc Dialog（纯 HTML + i18n，避免 markdown 渲染依赖） -->
        <el-dialog v-model="varsDocVisible" :title="t('settings.templates.varsDoc')" width="760">
          <div class="template-vars-doc">
            <h3>{{ t('settings.templates.doc.genericTitle') }}</h3>
            <table>
              <thead><tr><th>{{ t('settings.templates.doc.colVar') }}</th><th>{{ t('settings.templates.doc.colDesc') }}</th><th>{{ t('settings.templates.doc.colExample') }}</th></tr></thead>
              <tbody>
                <tr><td><code>{time}</code></td><td>{{ t('settings.templates.doc.genericTime') }}</td><td>2026-09-28 14:30:00</td></tr>
                <tr><td><code>{user}</code></td><td>{{ t('settings.templates.doc.genericUser') }}</td><td>user@example.com</td></tr>
                <tr><td><code>{message}</code></td><td>{{ t('settings.templates.doc.genericMessage') }}</td><td>—</td></tr>
              </tbody>
            </table>

            <h3>{{ t('settings.templates.doc.expireTitle') }}</h3>
            <p>{{ t('settings.templates.doc.expireDesc') }}</p>
            <table>
              <thead><tr><th>{{ t('settings.templates.doc.colVar') }}</th><th>{{ t('settings.templates.doc.colDesc') }}</th><th>{{ t('settings.templates.doc.colExample') }}</th></tr></thead>
              <tbody>
                <tr><td><code>{user}</code></td><td>{{ t('settings.templates.doc.expireUser') }}</td><td>user@example.com</td></tr>
                <tr><td><code>{expire_date}</code></td><td>{{ t('settings.templates.doc.expireDate') }}</td><td>2026-10-05 09:00</td></tr>
                <tr><td><code>{days}</code></td><td>{{ t('settings.templates.doc.expireDays') }}</td><td>7</td></tr>
              </tbody>
            </table>

            <h3>{{ t('settings.templates.doc.balanceTitle') }}</h3>
            <p>{{ t('settings.templates.doc.balanceDesc') }}</p>
            <table>
              <thead><tr><th>{{ t('settings.templates.doc.colVar') }}</th><th>{{ t('settings.templates.doc.colDesc') }}</th><th>{{ t('settings.templates.doc.colExample') }}</th></tr></thead>
              <tbody>
                <tr><td><code>{user}</code></td><td>{{ t('settings.templates.doc.balanceUser') }}</td><td>user@example.com</td></tr>
                <tr><td><code>{balance}</code></td><td>{{ t('settings.templates.doc.balanceValue') }}</td><td>0.85</td></tr>
              </tbody>
            </table>

            <h3>{{ t('settings.templates.doc.opsTitle') }}</h3>
            <p>{{ t('settings.templates.doc.opsDesc') }}</p>
            <table>
              <thead><tr><th>{{ t('settings.templates.doc.colVar') }}</th><th>{{ t('settings.templates.doc.colDesc') }}</th><th>{{ t('settings.templates.doc.colExample') }}</th></tr></thead>
              <tbody>
                <tr><td><code>{node}</code></td><td>{{ t('settings.templates.doc.opsNode') }}</td><td>node-01</td></tr>
                <tr><td><code>{backup_file}</code></td><td>{{ t('settings.templates.doc.opsBackupFile') }}</td><td>corefusion-20260101-0900.tar.gz</td></tr>
                <tr><td><code>{size}</code></td><td>{{ t('settings.templates.doc.opsSize') }}</td><td>128MB</td></tr>
                <tr><td><code>{domain}</code></td><td>{{ t('settings.templates.doc.opsDomain') }}</td><td>example.com</td></tr>
                <tr><td><code>{expire_date}</code> / <code>{days}</code></td><td>{{ t('settings.templates.doc.opsExpire') }}</td><td>2026-10-12 / 14</td></tr>
                <tr><td><code>{count}</code></td><td>{{ t('settings.templates.doc.opsCount') }}</td><td>3</td></tr>
              </tbody>
            </table>

            <h3>{{ t('settings.templates.doc.quotaTitle') }}</h3>
            <p>{{ t('settings.templates.doc.quotaDesc') }}</p>
            <table>
              <thead><tr><th>{{ t('settings.templates.doc.colVar') }}</th><th>{{ t('settings.templates.doc.colDesc') }}</th><th>{{ t('settings.templates.doc.colExample') }}</th></tr></thead>
              <tbody>
                <tr><td><code>{node}</code></td><td>{{ t('settings.templates.doc.quotaNode') }}</td><td>node-01</td></tr>
                <tr><td><code>{used}</code> / <code>{quota}</code></td><td>{{ t('settings.templates.doc.quotaUsed') }}</td><td>80.00 / 100.00</td></tr>
                <tr><td><code>{percent}</code></td><td>{{ t('settings.templates.doc.quotaPercent') }}</td><td>80</td></tr>
              </tbody>
            </table>

            <h3>{{ t('settings.templates.doc.rulesTitle') }}</h3>
            <ul>
              <li>{{ t('settings.templates.doc.ruleExpire') }}</li>
              <li>{{ t('settings.templates.doc.ruleBalance') }}</li>
              <li>{{ t('settings.templates.doc.ruleNodeMonitor') }}</li>
              <li>{{ t('settings.templates.doc.ruleQuota') }}</li>
              <li>{{ t('settings.templates.doc.ruleDedup') }}</li>
              <li>{{ t('settings.templates.doc.ruleSilent') }}</li>
            </ul>
          </div>
        </el-dialog>

        <!-- Broadcast Dialog（订阅者模板群发：营销/召回，可选范围含禁用账号） -->
        <el-dialog v-model="broadcastDialogVisible" :title="t('settings.templates.broadcast')" width="480">
          <p style="margin:0 0 12px;font-size:13px">{{ t('settings.templates.broadcastHint') }}</p>
          <el-radio-group v-model="broadcastScope">
            <el-radio value="active">{{ t('settings.templates.broadcastScopeActive') }}</el-radio>
            <el-radio value="all">{{ t('settings.templates.broadcastScopeAll') }}</el-radio>
            <el-radio value="suspended">{{ t('settings.templates.broadcastScopeSuspended') }}</el-radio>
          </el-radio-group>
          <div style="font-size:11px;color:#999;margin-top:8px">{{ t('settings.templates.broadcastScopeHint') }}</div>
          <template #footer>
            <el-button @click="broadcastDialogVisible=false">{{ t('common.cancel') }}</el-button>
            <el-button type="primary" :loading="broadcasting" @click="handleBroadcastConfirm">{{ t('settings.templates.broadcast') }}</el-button>
          </template>
        </el-dialog>

        <div style="margin-top:20px;text-align:right">
          <el-button type="primary" :loading="notifySaving" @click="saveNotifyConfig">{{ t('common.save') }}</el-button>
        </div>
      </el-tab-pane>

      <!-- Backup & Restore -->
      <el-tab-pane :label="t('settings.backup.title')" name="backup">
        <!-- Restore Progress (shown during async restore) -->
        <el-card v-if="restorePolling || restoreProgress" style="margin-bottom:20px">
          <template #header><span>{{ t('settings.backup.restoreProgress') }}</span></template>
          <div style="display:flex;flex-direction:column;gap:12px;align-items:center;padding:20px">
            <el-tag :type="restoreProgress?.status === 'error' ? 'danger' : restoreProgress?.status === 'done' ? 'success' : 'warning'" size="large">
              {{ restoreProgress?.status || 'pending' }}
            </el-tag>
            <span v-if="restoreProgress?.message" style="font-size:13px;color:#606266">{{ restoreProgress.message }}</span>
            <el-progress v-if="restorePolling" :percentage="restoreProgress?.status === 'downloading' ? 30 : restoreProgress?.status === 'restoring' ? 70 : 100" :indeterminate="restoreProgress?.status === 'pending'" />
          </div>
        </el-card>

        <!-- Maintenance Mode Toggle -->
        <el-card style="margin-bottom:20px">
          <div style="display:flex;justify-content:space-between;align-items:center">
            <div>
              <span style="font-weight:bold">{{ t('maintenance.title') }}</span>
              <div style="font-size:12px;color:#909399;margin-top:4px">{{ t('maintenance.description') }}</div>
              <div style="margin-top:8px;display:flex;align-items:center;gap:8px">
                <el-tag :type="maintenanceInfo.enabled ? 'danger' : 'success'" size="small">
                  {{ maintenanceInfo.enabled ? t('maintenance.statusOn') : t('maintenance.statusOff') }}
                </el-tag>
                <span v-if="maintenanceInfo.message" style="font-size:12px;color:#606266">"{{ maintenanceInfo.message }}"</span>
              </div>
            </div>
            <div style="display:flex;flex-direction:column;gap:8px;align-items:flex-end">
              <el-input
                v-model="maintenanceMessage"
                :placeholder="t('maintenance.messagePlaceholder')"
                style="width:280px"
                size="small"
              />
              <el-button
                :loading="maintenanceLoading"
                @click="toggleMaintenance(!maintenanceInfo.enabled)"
                :type="maintenanceInfo.enabled ? 'success' : 'warning'"
                :plain="!maintenanceInfo.enabled"
              >
                {{ maintenanceInfo.enabled ? t('maintenance.disableBtn') : t('maintenance.enableBtn') }}
              </el-button>
            </div>
          </div>
        </el-card>

        <!-- Create & Upload -->
        <el-card style="margin-bottom:20px">
          <div style="display:flex;gap:12px;align-items:center;flex-wrap:wrap">
            <el-button type="primary" :loading="backupCreating" @click="handleCreateBackup">{{ t('settings.backup.create') }}</el-button>
            <el-upload :auto-upload="false" :show-file-list="false" :on-change="handleUploadRestore" accept=".tar.gz,.gz">
              <el-button type="warning">{{ t('settings.backup.uploadRestore') }}</el-button>
            </el-upload>
            <el-dropdown trigger="click" v-if="storages.length > 0">
              <el-button type="success">{{ t('settings.backup.remoteRestore') }}<el-icon class="el-icon--right"><arrow-down /></el-icon></el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item v-for="s in storages" :key="s.id" @click="showRemoteRestore(s.id)">{{ s.name }} ({{ s.type }})</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </div>
        </el-card>

        <!-- Backup List -->
        <el-card style="margin-bottom:20px">
          <template #header><span>{{ t('settings.backup.list') }}</span></template>
          <el-table :data="backups" v-loading="backupsLoading" stripe>
<el-table-column prop="name" :label="t('common.name')" width="200">
<template #default="{row}">
{{ row.name }}
<el-tag v-if="row.remote_only" size="small" type="warning" style="margin-left:4px">Remote Only</el-tag>
</template>
</el-table-column>
<el-table-column prop="type" label="Type" width="80">
<template #default="{row}"><el-tag size="small">{{ row.type }}</el-tag></template>
</el-table-column>
<el-table-column label="Size" width="100">
<template #default="{row}">{{ formatSize(row.file_size) }}</template>
</el-table-column>
<el-table-column :label="t('common.createdAt')" width="180">
<template #default="{row}">{{ new Date(row.created_at).toLocaleString() }}</template>
</el-table-column>
<el-table-column :label="t('common.actions')" width="400">
<template #default="{row}">
<el-button v-if="!row.remote_only" size="small" @click="handleDownload(row)">{{ t('settings.backup.download') }}</el-button>
<el-button v-if="!row.remote_only" size="small" type="warning" @click="handleRestore(row)">{{ t('settings.backup.restore') }}</el-button>
<el-button v-if="row.remote_only" size="small" type="warning" disabled title="Use Remote Restore below">Local Restore</el-button>
<el-dropdown trigger="click" style="margin:0 8px">
<el-button size="small" type="success">{{ t('settings.backup.uploadTo') }}<el-icon class="el-icon--right"><arrow-down /></el-icon></el-button>
<template #dropdown>
<el-dropdown-menu>
<el-dropdown-item @click="showUploadS3(row)">S3 / MinIO</el-dropdown-item>
<el-dropdown-item @click="showUploadWebDAV(row)">WebDAV</el-dropdown-item>
<el-dropdown-item @click="showUploadFTP(row)">FTP</el-dropdown-item>
</el-dropdown-menu>
</template>
</el-dropdown>
<el-button size="small" type="danger" @click="handleDeleteBackup(row)">{{ t('common.delete') }}</el-button>
</template>
</el-table-column>
          </el-table>
        </el-card>

        <!-- Schedules -->
        <el-card style="margin-bottom:20px">
          <template #header>
            <div style="display:flex;justify-content:space-between;align-items:center">
              <span>{{ t('settings.backup.schedules') }}</span>
              <el-button size="small" type="primary" @click="showAddSchedule">{{ t('settings.backup.addSchedule') }}</el-button>
            </div>
          </template>
          <el-table :data="schedules" stripe>
            <el-table-column prop="name" :label="t('common.name')" width="150" />
            <el-table-column prop="cron" label="Cron" width="140" />
            <el-table-column :label="t('settings.backup.remoteStorage')" width="150">
              <template #default="{row}">
                <span v-if="row.storage_id">{{ getStorageName(row.storage_id) }}</span>
                <span v-else style="color:#999">{{ t('settings.backup.localOnly') }}</span>
              </template>
            </el-table-column>
            <el-table-column prop="retain_days" :label="t('settings.backup.retainDays')" width="100" />
            <el-table-column label="Enabled" width="80">
              <template #default="{row}"><el-tag :type="row.enabled?'success':'info'" size="small">{{ row.enabled ? 'ON' : 'OFF' }}</el-tag></template>
            </el-table-column>
            <el-table-column :label="t('common.actions')" width="140">
              <template #default="{row}">
                <el-button size="small" @click="showEditSchedule(row)">{{ t('common.edit') }}</el-button>
                <el-button size="small" type="danger" @click="handleDeleteSchedule(row)">{{ t('common.delete') }}</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>

        <!-- Remote Storage -->
        <el-card>
          <template #header>
            <div style="display:flex;justify-content:space-between;align-items:center">
              <span>{{ t('settings.backup.remoteStorage') }}</span>
              <el-button size="small" type="primary" @click="showAddStorage">{{ t('settings.backup.addStorage') }}</el-button>
            </div>
          </template>
          <el-table :data="storages" stripe>
            <el-table-column prop="name" :label="t('common.name')" width="150" />
            <el-table-column prop="type" label="Type" width="80">
              <template #default="{row}"><el-tag size="small">{{ row.type }}</el-tag></template>
            </el-table-column>
            <el-table-column label="Enabled" width="80">
              <template #default="{row}"><el-tag :type="row.enabled?'success':'info'" size="small">{{ row.enabled ? 'ON' : 'OFF' }}</el-tag></template>
            </el-table-column>
            <el-table-column :label="t('common.actions')" width="140">
              <template #default="{row}">
                <el-button size="small" @click="showEditStorage(row)">{{ t('common.edit') }}</el-button>
                <el-button size="small" type="danger" @click="handleDeleteStorage(row)">{{ t('common.delete') }}</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>

      <!-- Self-Unban Config -->
      <el-tab-pane :label="t('settings.selfUnban.tabLabel')" name="self-unban">
        <el-card>
          <el-form :model="selfUnbanForm" label-width="160px" style="max-width:600px">
            <el-form-item :label="t('settings.selfUnban.enable')">
              <el-switch v-model="selfUnbanForm.enabled" />
              <div style="font-size:11px;color:#999;margin-top:4px">{{ t('settings.selfUnban.enableHint') }}</div>
            </el-form-item>
            <el-form-item :label="t('settings.selfUnban.cooldown')">
              <el-input-number v-model="selfUnbanForm.cooldown_hours" :min="1" :max="720" />
              <div style="font-size:11px;color:#999;margin-top:4px">{{ t('settings.selfUnban.cooldownHint') }}</div>
            </el-form-item>
            <el-form-item :label="t('settings.selfUnban.applicableStatus')">
              <el-select v-model="selfUnbanForm.applicable_status" multiple style="width:100%" :placeholder="t('settings.selfUnban.selectStatus')">
                <el-option :label="t('settings.selfUnban.suspended')" value="suspended" />
                <el-option :label="t('settings.selfUnban.expired')" value="expired" />
                <el-option :label="t('settings.selfUnban.overuse')" value="overuse" />
              </el-select>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="selfUnbanSaving" @click="saveSelfUnbanConfig">{{ t('common.save') }}</el-button>
            </el-form-item>
          </el-form>
        </el-card>
      </el-tab-pane>

      <!-- Security Config -->
      <el-tab-pane :label="t('settings.security.tab')" name="security">
        <el-card style="margin-bottom:20px">
          <template #header><span>{{ t('settings.security.encryptionKeyTitle') }}</span></template>
          <el-form label-width="180px" style="max-width:600px">
            <el-form-item :label="t('settings.security.keyStatus')">
              <div style="display:flex;align-items:center;gap:12px">
                <el-tag :type="encryptionKeyStatus.type">{{ t(encryptionKeyStatus.text) }}</el-tag>
                <span v-if="encryptionKeyStatus.configured" style="font-size:12px;color:#666">
                  {{ t('settings.security.configuredNoEdit') }}
                </span>
              </div>
              <div style="font-size:11px;color:#999;margin-top:4px">
                {{ t('settings.security.keyDesc') }}
              </div>
            </el-form-item>
            <el-form-item v-if="!encryptionKeyStatus.configured" :label="t('settings.security.genKey')">
              <el-button type="primary" v-if="$can('super_admin')" :loading="generatingKey" @click="generateEncryptionKey">
                {{ t('settings.security.randomGenSave') }}
              </el-button>
              <div style="font-size:11px;color:#999;margin-top:4px">
                {{ t('settings.security.genKeyHint') }}
              </div>
            </el-form-item>
            <el-form-item v-if="encryptionKeyStatus.configured" :label="t('settings.security.importantWarning')">
              <el-alert type="warning" :closable="false" show-icon>
                <template #title>
                  {{ t('settings.security.keyConfiguredWarn') }}
                </template>
                <div style="margin-top:8px">
                  {{ t('settings.security.keyEditHint') }}
                </div>
              </el-alert>
            </el-form-item>
          </el-form>
        </el-card>

        <el-card>
          <template #header><span>{{ t('settings.security.jwtSecretTitle') }}</span></template>
          <el-form label-width="180px" style="max-width:600px">
            <el-form-item :label="t('settings.security.jwtStatus')">
              <div style="display:flex;align-items:center;gap:12px">
                <el-tag :type="jwtSecretStatus.configured ? 'success' : 'danger'">
                  {{ jwtSecretStatus.configured ? t('settings.security.statusConfigured') : t('settings.security.statusNotConfigured') }}
                </el-tag>
                <span v-if="jwtSecretStatus.preview" style="font-size:12px;color:#666;font-family:monospace">
                  {{ jwtSecretStatus.preview }}
                </span>
              </div>
              <div style="font-size:11px;color:#999;margin-top:4px">
                {{ t('settings.security.jwtDesc') }}
              </div>
            </el-form-item>
            <el-form-item :label="t('settings.security.regenerateJwt')">
              <el-button type="danger" v-if="$can('super_admin')" :loading="regeneratingJwt" @click="regenerateJWTSecret">
                {{ t('settings.security.regenerateJwtBtn') }}
              </el-button>
              <div style="font-size:11px;color:#e6a23c;margin-top:4px">
                {{ t('settings.security.regenerateJwtWarn') }}
              </div>
            </el-form-item>
          </el-form>
        </el-card>
      </el-tab-pane>

      <!-- Currency Config -->
      <el-tab-pane :label="t('currency.defaultCurrency')" name="currency">
        <el-card>
          <el-form :model="currencyForm" label-width="160px" style="max-width:600px">
            <el-form-item :label="t('currency.defaultCurrency')">
              <el-select v-model="currencyForm.default_currency" filterable style="width:100%">
                <el-option v-for="c in currencyList" :key="c.code" :label="`${c.code} ${c.symbol} (${t(c.nameKey)})`" :value="c.code" />
              </el-select>
              <div style="font-size:11px;color:#999;margin-top:4px">{{ t('currency.defaultCurrencyHint') }}</div>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="currencySaving" @click="saveCurrencyConfig">{{ t('common.save') }}</el-button>
            </el-form-item>
          </el-form>
        </el-card>
      </el-tab-pane>

      <!-- OAuth Config -->
      <el-tab-pane :label="t('settings.oauth.tab')" name="oauth">
        <el-card>
          <el-alert type="info" :closable="false" show-icon style="margin-bottom:16px">
            <span style="font-size:12px">
              Google 正式授权需要：公共可访问的隐私权政策 + 服务条款链接（门户编辑器"设置 → 合规文档"配置内容，登录页自动展示）。
              详见 <a :href="googleVerifyDocURL" target="_blank" rel="noopener" style="color:#409eff">docs/oauth-google-verification.md</a>
            </span>
          </el-alert>
          <el-form :model="oauthForm" label-width="160px" style="max-width:600px">
            <el-form-item :label="t('settings.oauth.callbackDomain')">
              <el-input v-model="oauthForm.callback_domain" :placeholder="t('settings.oauth.callbackDomainPlaceholder')" clearable />
              <div style="font-size:11px;color:#999;margin-top:4px">{{ t('settings.oauth.callbackDomainHint') }}</div>
            </el-form-item>
            <el-divider content-position="left">GitHub OAuth</el-divider>
            <el-form-item :label="t('settings.oauth.enableGithub')">
              <el-switch v-model="oauthForm.github_enabled" />
              <div v-if="oauthForm.github_enabled && (!oauthForm.github_client_id || !oauthForm.github_client_secret)" style="font-size:11px;color:#e6a23c;margin-top:4px">
                {{ t('settings.oauth.githubNotConfigured') }}
              </div>
            </el-form-item>
            <template v-if="oauthForm.github_enabled">
              <el-form-item label="Client ID">
                <el-input v-model="oauthForm.github_client_id" placeholder="GitHub OAuth App Client ID" />
              </el-form-item>
              <el-form-item label="Client Secret">
                <el-input v-model="oauthForm.github_client_secret" type="password" show-password />
              </el-form-item>
              <el-form-item :label="t('settings.oauth.callbackUrl')">
                <el-input :model-value="githubCallbackURL" readonly />
                <div style="font-size:11px;color:#999;margin-top:4px">{{ t('settings.oauth.githubCallbackHint') }}</div>
              </el-form-item>
            </template>
            <el-divider content-position="left">Google OAuth</el-divider>
            <el-form-item :label="t('settings.oauth.enableGoogle')">
              <el-switch v-model="oauthForm.google_enabled" />
              <div v-if="oauthForm.google_enabled && (!oauthForm.google_client_id || !oauthForm.google_client_secret)" style="font-size:11px;color:#e6a23c;margin-top:4px">
                {{ t('settings.oauth.googleNotConfigured') }}
              </div>
            </el-form-item>
            <template v-if="oauthForm.google_enabled">
              <el-form-item label="Client ID">
                <el-input v-model="oauthForm.google_client_id" placeholder="Google OAuth Client ID" />
              </el-form-item>
              <el-form-item label="Client Secret">
                <el-input v-model="oauthForm.google_client_secret" type="password" show-password />
              </el-form-item>
              <el-form-item :label="t('settings.oauth.callbackUrl')">
                <el-input :model-value="googleCallbackURL" readonly />
                <div style="font-size:11px;color:#999;margin-top:4px">{{ t('settings.oauth.googleCallbackHint') }}</div>
              </el-form-item>
            </template>
            <el-form-item>
              <el-button type="primary" :loading="oauthSaving" @click="saveOAuthConfig">{{ t('common.save') }}</el-button>
            </el-form-item>
          </el-form>
        </el-card>
      </el-tab-pane>

      <!-- GitHub Acceleration -->
      <el-tab-pane :label="t('settings.github.title')" name="github">
        <el-card>
          <el-form label-width="140px" style="max-width:600px">
            <el-form-item :label="t('settings.github.enabled')">
              <el-switch v-model="githubForm.enabled" />
              <div style="font-size:11px;color:#999;margin-top:4px">{{ t('settings.github.enabledHint') }}</div>
            </el-form-item>
            <el-form-item :label="t('settings.github.proxyUrl')">
              <el-input v-model="githubForm.proxy_url" :placeholder="t('settings.github.proxyUrlPlaceholder')" clearable />
              <div style="font-size:11px;color:#999;margin-top:4px">{{ t('settings.github.proxyUrlHint') }}</div>
            </el-form-item>
          </el-form>

          <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:16px">
            <span style="font-weight:500">{{ t('settings.github.domains') }}</span>
            <el-button type="primary" size="small" @click="showAddDomain">{{ t('settings.github.addDomain') }}</el-button>
          </div>
          <el-table :data="githubForm.domains" stripe>
            <el-table-column prop="name" :label="t('settings.github.domainName')" width="150" />
            <el-table-column prop="url" :label="t('settings.github.domainUrl')" />
            <el-table-column :label="t('settings.github.domainEnabled')" width="100">
              <template #default="{row}">
                <el-switch v-model="row.enabled" size="small" />
              </template>
            </el-table-column>
            <el-table-column :label="t('common.actions')" width="140">
              <template #default="{row, $index}">
                <el-button size="small" @click="showEditDomain(row, $index)">{{ t('common.edit') }}</el-button>
                <el-button size="small" type="danger" @click="handleDeleteDomain($index)">{{ t('common.delete') }}</el-button>
              </template>
            </el-table-column>
          </el-table>

          <div style="margin-top:20px;text-align:right">
            <el-button type="primary" :loading="githubSaving" @click="saveGitHubConfig">{{ t('common.save') }}</el-button>
          </div>
        </el-card>
      </el-tab-pane>

      <!-- Database Config -->
      <el-tab-pane :label="t('settings.db.title')" name="database">
        <!-- Current Status -->
        <el-card style="margin-bottom:20px">
          <template #header><span>{{ t('settings.db.currentStatus') }}</span></template>
          <el-descriptions :column="2" border>
            <el-descriptions-item :label="t('settings.db.currentType')">
              <el-tag :type="dbForm.current_type === 'sqlite' ? 'info' : 'success'">{{ dbForm.current_type || 'sqlite' }}</el-tag>
            </el-descriptions-item>
            <el-descriptions-item :label="t('settings.db.effectiveDSN')">
              <span style="font-family:monospace;font-size:12px">{{ dbForm.effective_dsn || '-' }}</span>
            </el-descriptions-item>
            <el-descriptions-item :label="t('settings.db.restartNeeded')" :span="2">
              <el-tag v-if="dbForm.restart_needed" type="warning">{{ t('settings.db.restartNeededYes') }}</el-tag>
              <el-tag v-else type="success">{{ t('settings.db.restartNeededNo') }}</el-tag>
            </el-descriptions-item>
          </el-descriptions>
        </el-card>

        <!-- Configuration -->
        <el-card style="margin-bottom:20px">
          <template #header><span>{{ t('settings.db.config') }}</span></template>
          <el-form :model="dbForm" label-width="160px" style="max-width:700px">
            <el-form-item :label="t('settings.db.type')">
              <el-select v-model="dbForm.type" style="width:100%">
                <el-option label="SQLite" value="sqlite" />
                <el-option label="MySQL" value="mysql" />
                <el-option label="PostgreSQL" value="postgres" />
              </el-select>
              <div style="font-size:11px;color:#999;margin-top:4px">{{ t('settings.db.typeHint') }}</div>
            </el-form-item>

            <el-form-item :label="t('settings.db.dsn')">
              <el-input v-model="dbForm.dsn" :placeholder="dbDsnPlaceholder" type="textarea" :rows="2" />
              <div style="font-size:11px;color:#999;margin-top:4px">{{ t('settings.db.dsnHint') }}</div>
            </el-form-item>

            <template v-if="dbForm.type !== 'sqlite'">
              <el-form-item :label="t('settings.db.maxOpenConns')">
                <el-input-number v-model="dbForm.max_open_conns" :min="1" :max="1000" />
              </el-form-item>
              <el-form-item :label="t('settings.db.maxIdleConns')">
                <el-input-number v-model="dbForm.max_idle_conns" :min="1" :max="500" />
              </el-form-item>
            </template>

            <el-form-item>
              <el-button type="primary" :loading="dbSaving" @click="saveDBConfig">{{ t('common.save') }}</el-button>
              <el-button :loading="dbTesting" @click="testDBConnection">{{ t('settings.db.testConnection') }}</el-button>
              <el-button type="warning" :loading="dbMigrating" @click="handleMigrateDB">{{ t('settings.db.migrateData') }}</el-button>
            </el-form-item>
          </el-form>
        </el-card>

        <!-- Data Migration Help -->
        <el-card>
          <template #header><span>{{ t('settings.db.migration') }}</span></template>
          <el-alert :title="t('settings.db.migrationWarning')" type="info" :closable="false" show-icon />
        </el-card>
      </el-tab-pane>
    </el-tabs>

    <!-- Schedule Dialog -->
    <el-dialog v-model="schedDialogVisible" :title="editingSched ? t('common.edit') : t('settings.backup.addSchedule')" width="500">
      <el-form :model="schedForm" label-width="100px">
        <el-form-item :label="t('common.name')"><el-input v-model="schedForm.name" /></el-form-item>
        <el-form-item label="Cron">
          <el-select v-model="schedForm.cron" filterable allow-create style="width:100%">
            <el-option :label="t('settings.cron.daily3')" value="0 3 * * *" />
            <el-option :label="t('settings.cron.daily0')" value="0 0 * * *" />
            <el-option :label="t('settings.cron.weeklySun3')" value="0 3 * * 0" />
            <el-option :label="t('settings.cron.every12h')" value="0 */12 * * *" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('settings.backup.remoteStorage')">
          <el-select v-model="schedForm.storage_id" clearable style="width:100%" :placeholder="t('settings.backup.localOnly')">
            <el-option v-for="s in storages" :key="s.id" :label="`${s.name} (${s.type})`" :value="s.id" />
          </el-select>
          <div style="font-size:11px;color:#999;margin-top:4px">{{ t('settings.backup.storageHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('settings.backup.retainDays')"><el-input-number v-model="schedForm.retain_days" :min="1" :max="365" /></el-form-item>
<el-form-item label="Delete Local After Upload"><el-switch v-model="schedForm.delete_local_after_upload" /> <span style="color:#909399;font-size:12px">Remove local backup after successful remote upload</span></el-form-item>
        <el-form-item label="Enabled"><el-switch v-model="schedForm.enabled" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="schedDialogVisible=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="handleSaveSchedule">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- Storage Dialog -->
    <el-dialog v-model="storageDialogVisible" :title="editingStorage ? t('common.edit') : t('settings.backup.addStorage')" width="500">
      <el-form :model="storageForm" label-width="100px">
        <el-form-item :label="t('common.name')"><el-input v-model="storageForm.name" /></el-form-item>
        <el-form-item label="Type">
          <el-select v-model="storageForm.type" style="width:100%">
            <el-option label="S3 / MinIO" value="s3" />
            <el-option label="WebDAV" value="webdav" />
            <el-option label="FTP" value="ftp" />
          </el-select>
        </el-form-item>
        <template v-if="storageForm.type==='s3'">
          <el-form-item label="Endpoint"><el-input v-model="storageForm.config.endpoint" placeholder="https://s3.amazonaws.com" /></el-form-item>
          <el-form-item label="Bucket"><el-input v-model="storageForm.config.bucket" /></el-form-item>
          <el-form-item label="Access Key"><el-input v-model="storageForm.config.access_key" /></el-form-item>
          <el-form-item label="Secret Key"><el-input v-model="storageForm.config.secret_key" type="password" show-password /></el-form-item>
        </template>
        <template v-if="storageForm.type==='webdav'">
          <el-form-item label="URL"><el-input v-model="storageForm.config.url" placeholder="https://dav.example.com/backups/" /></el-form-item>
          <el-form-item label="Username"><el-input v-model="storageForm.config.username" /></el-form-item>
          <el-form-item label="Password"><el-input v-model="storageForm.config.password" type="password" show-password /></el-form-item>
        </template>
        <template v-if="storageForm.type==='ftp'">
          <el-form-item label="Host"><el-input v-model="storageForm.config.host" placeholder="ftp.example.com" /></el-form-item>
          <el-form-item label="Port"><el-input-number v-model="storageForm.config.port" :min="1" :max="65535" /></el-form-item>
          <el-form-item label="Username"><el-input v-model="storageForm.config.username" /></el-form-item>
          <el-form-item label="Password"><el-input v-model="storageForm.config.password" type="password" show-password /></el-form-item>
          <el-form-item label="Path"><el-input v-model="storageForm.config.path" placeholder="/backups" /></el-form-item>
        </template>
        <el-form-item label="Enabled"><el-switch v-model="storageForm.enabled" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="storageDialogVisible=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="handleSaveStorage">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- FTP Upload Dialog -->
    <el-dialog v-model="ftpDialogVisible" :title="'Upload to FTP'" width="500">
      <el-form :model="ftpForm" label-width="100px">
        <el-form-item label="Host"><el-input v-model="ftpForm.host" placeholder="ftp.example.com" /></el-form-item>
        <el-form-item label="Port"><el-input-number v-model="ftpForm.port" :min="1" :max="65535" /></el-form-item>
        <el-form-item label="Username"><el-input v-model="ftpForm.username" /></el-form-item>
        <el-form-item label="Password"><el-input v-model="ftpForm.password" type="password" show-password /></el-form-item>
        <el-form-item label="Path"><el-input v-model="ftpForm.path" placeholder="/backups" /></el-form-item>
<el-form-item label="Delete Local"><el-switch v-model="ftpForm.delete_local" /> <span style="color:#909399;font-size:12px">Remove local backup after upload</span></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="ftpDialogVisible=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="ftpUploading" @click="handleUploadFTP">{{ t('common.upload') }}</el-button>
      </template>
    </el-dialog>

    <!-- S3 Upload Dialog -->
    <el-dialog v-model="s3DialogVisible" :title="'Upload to S3 / MinIO'" width="500">
      <el-form :model="s3Form" label-width="100px">
        <el-form-item label="Endpoint"><el-input v-model="s3Form.endpoint" placeholder="https://s3.amazonaws.com" /></el-form-item>
        <el-form-item label="Bucket"><el-input v-model="s3Form.bucket" /></el-form-item>
        <el-form-item label="Access Key"><el-input v-model="s3Form.access_key" /></el-form-item>
<el-form-item label="Secret Key"><el-input v-model="s3Form.secret_key" type="password" show-password /></el-form-item>
<el-form-item label="Delete Local"><el-switch v-model="s3Form.delete_local" /> <span style="color:#909399;font-size:12px">Remove local backup after upload</span></el-form-item>
</el-form>
<template #footer>
<el-button @click="s3DialogVisible=false">{{ t('common.cancel') }}</el-button>
<el-button type="primary" :loading="s3Uploading" @click="handleUploadS3">{{ t('common.upload') }}</el-button>
</template>
</el-dialog>

<!-- WebDAV Upload Dialog -->
<el-dialog v-model="webdavDialogVisible" :title="'Upload to WebDAV'" width="500">
<el-form :model="webdavForm" label-width="100px">
<el-form-item label="URL"><el-input v-model="webdavForm.url" placeholder="https://dav.example.com/backups/" /></el-form-item>
<el-form-item label="Username"><el-input v-model="webdavForm.username" /></el-form-item>
<el-form-item label="Password"><el-input v-model="webdavForm.password" type="password" show-password /></el-form-item>
<el-form-item label="Delete Local"><el-switch v-model="webdavForm.delete_local" /> <span style="color:#909399;font-size:12px">Remove local backup after upload</span></el-form-item>
</el-form>
<template #footer>
<el-button @click="webdavDialogVisible=false">{{ t('common.cancel') }}</el-button>
<el-button type="primary" :loading="webdavUploading" @click="handleUploadWebDAV">{{ t('common.upload') }}</el-button>
</template>
</el-dialog>

    <!-- Remote Restore Dialog -->
    <el-dialog v-model="remoteRestoreDialogVisible" :title="t('settings.backup.remoteRestore')" width="600">
      <el-table :data="remoteRestoreFiles" v-loading="remoteRestoreLoading" stripe max-height="400">
        <el-table-column prop="name" :label="t('common.name')" />
        <el-table-column prop="size" label="Size" width="120">
          <template #default="{row}">{{ formatSize(row.size) }}</template>
        </el-table-column>
        <el-table-column :label="t('common.actions')" width="120">
          <template #default="{row}">
            <el-button size="small" type="warning" @click="handleRemoteRestore(row.name)">{{ t('settings.backup.restore') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div v-if="!remoteRestoreLoading && remoteRestoreFiles.length === 0" style="text-align:center;padding:20px;color:#909399">
        {{ t('settings.backup.noRemoteFiles') }}
      </div>
    </el-dialog>

    <!-- Template Dialog -->
    <el-dialog v-model="tmplDialogVisible" :title="editingTmpl ? t('common.edit') : t('settings.templates.add')" width="600">
      <el-form :model="tmplForm" label-width="100px">
        <el-form-item v-if="!editingTmpl" :label="t('settings.templates.preset')">
          <el-select v-model="selectedPreset" :placeholder="t('settings.templates.presetPlaceholder')" style="width:100%" @change="applyPreset">
            <el-option v-for="p in templatePresets" :key="p.key" :label="`${t('settings.templates.presets.' + p.key + '.label')}（${p.type === 'telegram' ? 'Telegram' : 'Email'}）`" :value="p.key" />
          </el-select>
          <div style="font-size:11px;color:#999;margin-top:4px">{{ t('settings.templates.presetHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('common.name')"><el-input v-model="tmplForm.name" /></el-form-item>
        <el-form-item :label="t('settings.templates.audience')">
          <el-radio-group v-model="tmplForm.audience" @change="onAudienceChange">
            <el-radio value="admin">{{ t('settings.templates.audienceAdmin') }}</el-radio>
            <el-radio value="subscriber">{{ t('settings.templates.audienceSubscriber') }}</el-radio>
          </el-radio-group>
          <div style="font-size:11px;color:#999;margin-top:4px;width:100%">{{ t('settings.templates.audienceHint') }}</div>
        </el-form-item>
        <el-form-item v-if="tmplForm.audience !== 'subscriber'" label="Type">
          <el-select v-model="tmplForm.type" style="width:100%">
            <el-option label="Telegram" value="telegram" />
            <el-option label="Email" value="email" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="tmplForm.audience === 'subscriber'" :label="t('settings.templates.eventKey')">
          <el-select v-model="tmplForm.event_key" filterable allow-create clearable style="width:100%" :placeholder="t('settings.templates.eventKeyPlaceholder')">
            <el-option label="订阅到期提醒（subscription_expire）" value="subscription_expire" />
            <el-option label="余额不足提醒（balance_low）" value="balance_low" />
          </el-select>
          <div style="font-size:11px;color:#999;margin-top:4px">{{ t('settings.templates.eventKeyHint') }}</div>
        </el-form-item>
        <el-form-item v-if="tmplForm.type==='email' && tmplForm.audience !== 'subscriber'" :label="t('settings.templates.recipients')">
          <el-select v-model="tmplRecipients" multiple filterable allow-create style="width:100%" placeholder="user@example.com">
            <el-option-group :label="t('settings.templates.userEmails')">
              <el-option v-for="u in userEmails" :key="'u'+u.id" :label="u.email" :value="u.email" />
            </el-option-group>
            <el-option-group :label="t('settings.templates.acmeEmails')">
              <el-option v-for="e in emails" :key="'e'+e.id" :label="e.email" :value="e.email" />
            </el-option-group>
          </el-select>
          <div style="font-size:11px;color:#999;margin-top:4px">{{ t('settings.templates.recipientsHint') }}</div>
        </el-form-item>
        <el-form-item v-if="tmplForm.type==='email'" label="Subject"><el-input v-model="tmplForm.subject" /></el-form-item>
        <el-form-item label="Content">
          <el-input v-model="tmplForm.content" type="textarea" :rows="6" :placeholder="t('settings.supportVars')" />
        </el-form-item>
        <el-form-item v-if="!isMonitorTemplate" label="Cron">
          <el-select v-model="tmplForm.cron" filterable allow-create style="width:100%">
            <el-option :label="t('settings.cron.manual')" value="" />
            <el-option :label="t('settings.cron.hourly')" value="0 * * * *" />
            <el-option :label="t('settings.cron.daily9')" value="0 9 * * *" />
            <el-option :label="t('settings.cron.daily21')" value="0 21 * * *" />
            <el-option :label="t('settings.cron.monday9')" value="0 9 * * 1" />
          </el-select>
        </el-form-item>
        <el-form-item v-else :label="t('settings.cron.label')">
          <div class="hint" style="margin-top:6px">{{ t('settings.templates.monitorCronHint') }}</div>
        </el-form-item>
        <el-form-item label="Enabled"><el-switch v-model="tmplForm.enabled" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="tmplDialogVisible=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="handleSaveTemplate">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- GitHub Domain Dialog -->
    <el-dialog v-model="domainDialogVisible" :title="editingDomainIndex !== null ? t('settings.github.editDomain') : t('settings.github.addDomain')" width="500">
      <el-form :model="domainForm" label-width="100px">
        <el-form-item :label="t('settings.github.domainName')"><el-input v-model="domainForm.name" placeholder="ghproxy.com" /></el-form-item>
        <el-form-item :label="t('settings.github.domainUrl')"><el-input v-model="domainForm.url" placeholder="https://ghproxy.com" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="domainDialogVisible=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="handleSaveDomain">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- Restart -->
    <el-card style="margin-top:20px">
      <div style="display:flex;justify-content:space-between;align-items:center">
        <div>
          <div style="font-weight:500">{{ t('settings.restart.title') }}</div>
          <div style="font-size:12px;color:#999;margin-top:4px">{{ t('settings.restart.hint') }}</div>
        </div>
        <el-button type="danger" v-if="$can('super_admin')" :loading="restarting" @click="handleRestart">{{ t('settings.restart.button') }}</el-button>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import client from '../api/client'
import { CURRENCIES as currencyList } from '../utils/currency'
import { useQueryState } from '../composables/useQueryState'

const { t } = useI18n()
const activeTab = useQueryState('tab', 'panel')

// Currency config
const currencyForm = ref({ default_currency: 'CNY' })
const currencySaving = ref(false)

// Panel config
const panelForm = ref({ listen_addr: '0.0.0.0', listen_port: '8080', subscription_port: '0', tls_enabled: false, tls_cert_id: '', tls_cert_path: '', tls_key_path: '', mtls_enabled: false, mtls_ca_path: '', agent_grpc_port: '18443', web_path: '/', timezone: 'Asia/Shanghai', data_dir: 'data', trusted_proxies: '', profile_web_page_url: '', mode: 'full', export_max_rows: 50000, passkey_rp_ids: '' })
const envForm = ref({ safe_paths: 'data' })
const envSaving = ref(false)
const panelPort = computed({ get: () => parseInt(panelForm.value.listen_port) || 8080, set: (v) => { panelForm.value.listen_port = String(v) } })
const subPort = computed({ get: () => parseInt(panelForm.value.subscription_port) || 0, set: (v) => { panelForm.value.subscription_port = String(v) } })
const agentGRPCPort = computed({ get: () => parseInt(panelForm.value.agent_grpc_port) || 18443, set: (v) => { panelForm.value.agent_grpc_port = String(v) } })
const panelSaving = ref(false)
const migrating = ref(false)
const rpIDAdding = ref(false)
const certificates = ref([])

// Notification
const notifyForm = ref({ telegram_enabled: false, telegram_bot_token: '', telegram_chat_id: '', smtp_enabled: false, smtp_host: '', smtp_port: '465', smtp_user: '', smtp_pass: '', smtp_from: '', smtp_tls: true })
const notifySaving = ref(false)
const smtpTesting = ref(false)
const tgTesting = ref(false)
const chatIdFetching = ref(false)
const restarting = ref(false)

// Templates
const templates = ref([])
const emails = ref([]) // ACME accounts
const userEmails = ref([]) // Users with email

// Backup
const backups = ref([])
const backupsLoading = ref(false)
const backupCreating = ref(false)
const schedules = ref([])
const storages = ref([])
const schedDialogVisible = ref(false)
const editingSched = ref(null)
const schedForm = ref({ name: '', cron: '0 3 * * *', retain_days: 30, delete_local_after_upload: false, enabled: true })
const storageDialogVisible = ref(false)
const editingStorage = ref(null)
const storageForm = ref({ name: '', type: 's3', config: {}, enabled: false })
const tmplDialogVisible = ref(false)
const editingTmpl = ref(null)
const tmplForm = ref({ name: '', type: 'telegram', audience: 'admin', event_key: '', recipients: '', subject: '', content: '', cron: '', enabled: false })
const tmplRecipients = ref([])

// GitHub acceleration
const githubForm = ref({ enabled: false, domains: [], proxy_url: '' })
const githubSaving = ref(false)
const domainDialogVisible = ref(false)
const editingDomainIndex = ref(null)
const domainForm = ref({ name: '', url: '' })

// Self-unban config
const selfUnbanForm = ref({ enabled: false, cooldown_hours: 24, applicable_status: ['suspended'] })
const selfUnbanSaving = ref(false)

// Security config (encryption key)
const encryptionKeyStatus = ref({ configured: false, text: '未配置', type: 'danger' })
const generatingKey = ref(false)
// JWT secret
const jwtSecretStatus = ref({ configured: false, preview: '' })
const regeneratingJwt = ref(false)

// OAuth config
const oauthForm = ref({ callback_domain: '', github_enabled: false, github_client_id: '', github_client_secret: '', google_enabled: false, google_client_id: '', google_client_secret: '' })
const oauthSaving = ref(false)

// Database config
const dbForm = ref({ type: 'sqlite', dsn: '', max_open_conns: 100, max_idle_conns: 10, current_type: 'sqlite', effective_dsn: '', restart_needed: false })
const dbSaving = ref(false)
const dbTesting = ref(false)
const dbMigrating = ref(false)
const dbDsnPlaceholder = computed(() => {
  switch (dbForm.value.type) {
    case 'mysql': return 'mysql://user:password@tcp(127.0.0.1:3306)/corefusion?charset=utf8mb4&parseTime=True&loc=Local'
    case 'postgres': return 'postgres://user:password@127.0.0.1:5432/corefusion?sslmode=disable'
    default: return 'data/sqlite/app.db'
  }
})

// When DB type changes, reset DSN and auto-fill SQLite default path
// Skip during initial load from API
const dbTypeInitializing = ref(false)
watch(() => dbForm.value.type, (newType) => {
  if (dbTypeInitializing.value) return
  if (newType === 'sqlite') {
    dbForm.value.dsn = 'data/sqlite/app.db'
  } else {
    dbForm.value.dsn = ''
  }
})

// OAuth callback URLs（提示用 ✓：后端 redirect_uri 按"发起登录的域名"动态拼 ✓）
// ⚠ OAuth 登录发生在**门户域名**时，要注册的是门户域名的回调 ✗ →
//   用 callback_domain 覆盖显示 ✓（空 = 当前面板地址 ✓）。
const oauthCallbackBase = computed(() => {
  let d = (oauthForm.value.callback_domain || '').trim()
  if (d && !/^https?:\/\//i.test(d)) d = 'https://' + d
  return d || window.location.origin
})
const githubCallbackURL = computed(() => oauthCallbackBase.value + '/api/v1/client-auth/oauth/github/callback')
const googleCallbackURL = computed(() => oauthCallbackBase.value + '/api/v1/client-auth/oauth/google/callback')
const googleVerifyDocURL = 'https://github.com/qist/corefusion/blob/main/docs/oauth-google-verification.md'

const timezones = ['Asia/Shanghai','Asia/Hong_Kong','Asia/Taipei','Asia/Tokyo','Asia/Seoul','Asia/Singapore','Asia/Kolkata','Asia/Dubai','Europe/London','Europe/Paris','Europe/Berlin','Europe/Moscow','America/New_York','America/Chicago','America/Denver','America/Los_Angeles','America/Sao_Paulo','Australia/Sydney','Pacific/Auckland','UTC']

async function fetchPanelConfig() { try { const d = await client.get('/system/panel-config'); if (d) panelForm.value = { ...panelForm.value, ...d } } catch (e) { ElMessage.error(t('common.failed')) } }
async function fetchEnvConfig() { try { const d = await client.get('/system/env-config'); if (d) envForm.value = { ...envForm.value, ...d } } catch (e) { ElMessage.error(t('common.failed')) } }
async function saveEnvConfig() { envSaving.value = true; try { await client.put('/system/env-config', envForm.value); ElMessage.success(t('common.success')) } catch (e) { ElMessage.error(t('common.failed')) } finally { envSaving.value = false } }
async function fetchNotifyConfig() { try { const d = await client.get('/system/notification-config'); if (d) notifyForm.value = { ...notifyForm.value, ...d } } catch (e) { ElMessage.error(t('common.failed')) } }
async function fetchCertificates() { try { const d = await client.get('/certificates', { params: { limit: 100, offset: 0 } }); certificates.value = d.items || [] } catch (e) { ElMessage.error(t('common.failed')) } }
async function fetchTemplates() { try { templates.value = await client.get('/system/notification-templates') || [] } catch (e) { ElMessage.error(t('common.failed')) } }
async function fetchEmails() {
  try { emails.value = await client.get('/acme-accounts') || [] } catch (e) { ElMessage.error(t('common.failed')) }
  try {
    const data = await client.get('/users', { params: { limit: 100 } })
    userEmails.value = (data.items || []).filter(u => u.email).map(u => ({ id: u.id, email: u.email }))
  } catch (e) { ElMessage.error(t('common.failed')) }
}

function onCertSelect(id) {
  const cert = certificates.value.find(c => c.id === id)
  if (cert) { panelForm.value.tls_cert_path = cert.cert_path; panelForm.value.tls_key_path = cert.key_path }
}

async function handleMTLSUpload(file) {
  const formData = new FormData()
  formData.append('file', file.raw)
  try {
    const d = await client.post('/system/upload-mtls-ca', formData, { headers: { 'Content-Type': 'multipart/form-data' } })
    panelForm.value.mtls_ca_path = d.path
    ElMessage.success(t('common.success'))
  } catch (e) { ElMessage.error(t('common.failed')) }
}

async function savePanelConfig() {
  const prevMode = panelForm.value.mode
  panelSaving.value = true
  try {
    await client.put('/system/panel-config', panelForm.value)
    ElMessage.success(t('common.success'))
    // 面板模式只在启动时读取，变更需重启面板生效（设计 §4.4）
    if (panelForm.value.mode !== prevMode) ElMessage.warning(t('settings.panel.modeRestartHint'), { duration: 6000 })
  } catch (e) { ElMessage.error(t('common.failed')) } finally { panelSaving.value = false }
}

// 「把当前访问域名加入 Passkey 白名单」：管理员确认一次，而非每次请求自动信任。
// 走专用 API（幂等），随后刷新配置回显。
async function addCurrentDomain() {
  const host = window.location.hostname
  if (!host) { ElMessage.error(t('common.failed')); return }
  rpIDAdding.value = true
  try {
    const res = await import('../api/passkey').then(m => m.updateRPID('add', host))
    panelForm.value.passkey_rp_ids = (res?.rpids || panelForm.value.passkey_rp_ids.split(',').filter(Boolean)).join(',')
    ElMessage.success(t('settings.panel.passkeyAddCurrentDone'))
  } catch (e) { ElMessage.error(t('common.failed')) } finally { rpIDAdding.value = false }
}

async function handleMigrateData() {
  await ElMessageBox.confirm(t('settings.panel.migrateConfirm', { dir: envForm.value.safe_paths }), t('common.confirm'), { type: 'warning' })
  migrating.value = true
  try {
    await client.post('/system/migrate-data', { new_dir: envForm.value.safe_paths })
    ElMessage.success(t('settings.panel.migrateSuccess'))
    // Auto restart
    await client.post('/system/restart')
    setTimeout(() => { window.location.reload() }, 3000)
  } catch (e) { ElMessage.error(t('common.failed')) } finally { migrating.value = false }
}
async function saveNotifyConfig() {
  notifySaving.value = true
  try {
    await client.put('/system/notification-config', notifyForm.value)
    ElMessage.success(t('common.success'))
    // 保存后立即回读：密钥字段以掩码 ******** 回显，前端不再持有明文（掩码回传时后端自动还原旧值）
    await fetchNotifyConfig()
  } catch (e) { ElMessage.error(t('common.failed')) } finally { notifySaving.value = false }
}
async function testSMTP() { smtpTesting.value = true; try { await client.post('/system/test-smtp', notifyForm.value); ElMessage.success(t('settings.smtp.testSuccess')) } catch (e) { ElMessage({ type: 'error', message: e?.response?.data?.message || t('common.failed'), duration: 8000 }) } finally { smtpTesting.value = false } }
async function testTelegram() { tgTesting.value = true; try { await client.post('/system/test-telegram', notifyForm.value); ElMessage.success(t('settings.telegram.testSuccess')) } catch (e) { ElMessage({ type: 'error', message: e?.response?.data?.message || t('common.failed'), duration: 8000 }) } finally { tgTesting.value = false } }
// 自动获取 Chat ID：用户给 bot 发一条消息后点此按钮
async function fetchChatId() {
  chatIdFetching.value = true
  try {
    const d = await client.post('/system/telegram-chat-id', { telegram_bot_token: notifyForm.value.telegram_bot_token })
    if (d?.chat_id) { notifyForm.value.telegram_chat_id = d.chat_id; ElMessage.success(t('settings.telegram.fetchChatIdSuccess', { id: d.chat_id })) }
  } catch (e) { ElMessage({ type: 'error', message: e?.response?.data?.message || t('common.failed'), duration: 8000 }) } finally { chatIdFetching.value = false }
}

function showAddTemplate() {
  editingTmpl.value = null
  tmplForm.value = { name: '', type: 'telegram', audience: 'admin', event_key: '', recipients: '', subject: '', content: '', cron: '', enabled: false }
  tmplRecipients.value = []
  selectedPreset.value = ''
  tmplDialogVisible.value = true
}
function showEditTemplate(row) {
  editingTmpl.value = row
  tmplForm.value = { audience: 'admin', event_key: '', ...row }
  tmplRecipients.value = row.recipients ? row.recipients.split(',').map(s => s.trim()).filter(Boolean) : []
  selectedPreset.value = ''
  tmplDialogVisible.value = true
}
// 受众切换：订阅者通知只走 email（发给客户本人邮箱，无 Telegram 会话）
function onAudienceChange(a) {
  if (a === 'subscriber') tmplForm.value.type = 'email'
}

// ---- 预设通知模板库 ----
// {key} 占位符与后端 notifier 渲染逻辑一致；业务事件通过 NotifyTemplate(name, vars) /
// NotifyClientEvent(eventKey, ...) 发送。⚠ 模板 name 是后端查找键（node_monitor.go /
// traffic_quota.go 固定引用中文名），applyPreset 时始终写入业务键，不做 i18n 改写；
// i18n 只负责下拉展示名（presets.<key>.label）与预填的 subject/content 文案。
const templatePresets = [
  { key: 'node_down', type: 'telegram', audience: 'admin', cron: '' },
  { key: 'node_recovered', type: 'telegram', audience: 'admin', cron: '' },
  { key: 'backup_daily', type: 'telegram', audience: 'admin', cron: '0 9 * * *' },
  { key: 'backup_failed', type: 'telegram', audience: 'admin', cron: '' },
  { key: 'cert_expire', type: 'email', audience: 'admin', cron: '0 9 * * *' },
  { key: 'sys_announce', type: 'telegram', audience: 'admin', cron: '' },
  { key: 'user_register', type: 'telegram', audience: 'admin', cron: '' },
  { key: 'node_quota', type: 'telegram', audience: 'admin', cron: '' },
  { key: 'sub_expire', type: 'email', audience: 'subscriber', eventKey: 'subscription_expire', cron: '' },
  { key: 'balance_low_sub', type: 'email', audience: 'subscriber', eventKey: 'balance_low', cron: '' },
]
const selectedPreset = ref('')

// 监控类模板：由节点健康监控/流量配额检查自动触发，Cron 不适用（频率在「监控设置」里配置）
const monitorTemplateNames = ['节点离线告警', '节点恢复通知', '节点流量配额告警']
const isMonitorTemplate = computed(() =>
  (selectedPreset.value && ['node_down', 'node_recovered', 'node_quota'].includes(selectedPreset.value)) ||
  monitorTemplateNames.includes(tmplForm.value.name)
)

function applyPreset(key) {
  const p = templatePresets.find(x => x.key === key)
  if (!p) return
  const base = `settings.templates.presets.${p.key}`
  tmplForm.value.name = t(`${base}.name`)   // 业务查找键：各语言下保持一致（见 locales 内固定中文名）
  tmplForm.value.type = p.type
  tmplForm.value.audience = p.audience
  tmplForm.value.event_key = p.eventKey || ''
  tmplForm.value.subject = t(`${base}.subject`)
  tmplForm.value.content = t(`${base}.content`)
  tmplForm.value.cron = p.cron || ''
  tmplForm.value.enabled = false
  if (p.type === 'telegram') tmplRecipients.value = []
}

// 实际发送一次模板通知（示例变量填充），验证内容与渠道连通性
async function handleSendTemplate(row) {
  try {
    await client.post(`/system/notification-templates/${row.id}/send`)
    ElMessage.success(t('settings.templates.sendSuccess'))
  } catch (e) { ElMessage({ type: 'error', message: e?.response?.data?.message || t('common.failed'), duration: 8000 }) }
}

// 订阅者模板群发（营销/召回类）：选择收件人范围后后台异步发送
const broadcastDialogVisible = ref(false)
const broadcastTemplate = ref(null)
const broadcastScope = ref('active')
const broadcasting = ref(false)

function showBroadcastDialog(row) {
  broadcastTemplate.value = row
  broadcastScope.value = 'active'
  broadcastDialogVisible.value = true
}

async function handleBroadcastConfirm() {
  if (!broadcastTemplate.value) return
  broadcasting.value = true
  try {
    const d = await client.post(`/system/notification-templates/${broadcastTemplate.value.id}/broadcast`, { scope: broadcastScope.value })
    broadcastDialogVisible.value = false
    ElMessage.success(t('settings.templates.broadcastStarted', { count: d?.recipients ?? 0 }))
  } catch (e) { ElMessage({ type: 'error', message: e?.response?.data?.message || t('common.failed'), duration: 8000 }) } finally { broadcasting.value = false }
}

// ---- 模板变量说明：纯 HTML + i18n 弹窗（见 template 段 doc.* 文案；仓库同文档 docs/notification-templates.md）----
const varsDocVisible = ref(false)
function showVarsDoc() { varsDocVisible.value = true }

// ---- 监控设置（节点监控频率/心跳超时/流量配额阈值；保存即时生效，无需重启）----
const monitorForm = ref({ node_interval_seconds: 60, node_offline_timeout_seconds: 120, traffic_quota_threshold_percent: 80 })
const monitorSaving = ref(false)

async function fetchMonitorConfig() {
  try {
    const d = await client.get('/system/settings', { params: { prefix: 'monitor' }, suppressErrorMessage: true })
    if (d?.items) {
      for (const item of d.items) {
        const n = parseInt(item.value)
        if (item.key === 'monitor.node_interval_seconds' && n >= 30) monitorForm.value.node_interval_seconds = n
        if (item.key === 'monitor.node_offline_timeout_seconds' && n >= 30) monitorForm.value.node_offline_timeout_seconds = n
        if (item.key === 'monitor.traffic_quota_threshold_percent' && n >= 0) monitorForm.value.traffic_quota_threshold_percent = n
      }
    }
  } catch (e) { /* 首次无配置用默认值 */ }
}

async function saveMonitorConfig() {
  monitorSaving.value = true
  try {
    await client.put('/system/settings/batch', {
      items: [
        { key: 'monitor.node_interval_seconds', value: String(monitorForm.value.node_interval_seconds) },
        { key: 'monitor.node_offline_timeout_seconds', value: String(monitorForm.value.node_offline_timeout_seconds) },
        { key: 'monitor.traffic_quota_threshold_percent', value: String(monitorForm.value.traffic_quota_threshold_percent) },
      ]
    })
    ElMessage.success(t('settings.monitor.saved'))
  } catch (e) { ElMessage.error(t('common.failed')) } finally { monitorSaving.value = false }
}

async function handleSaveTemplate() {
  try {
    const payload = { ...tmplForm.value, recipients: tmplRecipients.value.join(',') }
    if (editingTmpl.value) { await client.put(`/system/notification-templates/${editingTmpl.value.id}`, payload) }
    else { await client.post('/system/notification-templates', payload) }
    ElMessage.success(t('common.success')); tmplDialogVisible.value = false; fetchTemplates()
  } catch (e) { ElMessage.error(t('common.failed')) }
}
async function handleDeleteTemplate(row) { await ElMessageBox.confirm(t('common.confirm')); await client.delete(`/system/notification-templates/${row.id}`); ElMessage.success(t('common.success')); fetchTemplates() }

async function handleRestart() {
  await ElMessageBox.confirm(t('settings.restart.confirm'), t('common.confirm'), { type: 'warning' })
  restarting.value = true
  try { await client.post('/system/restart'); ElMessage.success(t('settings.restart.restarting')); setTimeout(() => { window.location.reload() }, 3000) }
  catch (e) { setTimeout(() => { window.location.reload() }, 2000) }
  finally { restarting.value = false }
}

// Backup functions
function formatSize(bytes) { if (!bytes) return '-'; if (bytes < 1024) return bytes + ' B'; if (bytes < 1048576) return (bytes / 1024).toFixed(1) + ' KB'; return (bytes / 1048576).toFixed(1) + ' MB' }

async function fetchBackups() { backupsLoading.value = true; try { backups.value = await client.get('/backups/list') || [] } catch (e) { ElMessage.error(t('common.failed')) } finally { backupsLoading.value = false } }
async function fetchSchedules() { try { schedules.value = await client.get('/backups/schedules') || [] } catch (e) { ElMessage.error(t('common.failed')) } }
async function fetchStorages() { try { storages.value = await client.get('/backups/storages') || [] } catch (e) { ElMessage.error(t('common.failed')) } }

async function handleCreateBackup() {
  backupCreating.value = true
  try { await client.post('/backups/create', {}); ElMessage.success(t('common.success')); fetchBackups() } catch (e) { ElMessage.error(t('common.failed')) } finally { backupCreating.value = false }
}

async function handleDownload(row) {
  // 一次性短时票据（M10）：新窗口下载带不了 Authorization 头，但也不该把长期 token 放进 URL ✗。
  // 票据 60 秒过期、只能用一次、且只能用在下载路径上。
  try {
    const res = await client.post('/auth/ticket', null, { params: { scope: 'backup_download' } })
    const data = res?.data?.data || res?.data || {}
    if (!data.ticket) throw new Error('no ticket')
    window.open(`/api/v1/backups/${row.id}/download?ticket=${encodeURIComponent(data.ticket)}`, '_blank')
  } catch (e) {
    ElMessage.error(t('common.failed'))
  }
}

async function handleRestore(row) {
  await ElMessageBox.confirm(t('settings.backup.restoreConfirm'), t('common.confirm'), { type: 'warning' })
  try {
    await client.post(`/backups/${row.id}/restore`)
    ElMessage.success(t('settings.backup.restoreStarted'))
    startRestorePolling()
  } catch (e) { ElMessage.error(t('common.failed')) }
}

async function handleDeleteBackup(row) {
  await ElMessageBox.confirm(t('common.confirm'))
  await client.delete(`/backups/${row.id}`); ElMessage.success(t('common.success')); fetchBackups()
}

async function handleUploadRestore(file) {
  await ElMessageBox.confirm(t('settings.backup.uploadRestoreConfirm'), t('common.confirm'), { type: 'warning' })
  const formData = new FormData()
  formData.append('file', file.raw)
  try {
    await client.post('/backups/upload-restore', formData, { headers: { 'Content-Type': 'multipart/form-data' } })
    ElMessage.success(t('settings.backup.restoreStarted'))
    startRestorePolling()
  } catch (e) { ElMessage.error(t('common.failed')) }
}

// Async restore polling
const restorePolling = ref(false)
const restoreProgress = ref(null)
let restorePollTimer = null

async function startRestorePolling() {
  restorePolling.value = true
  await pollRestoreStatus()
  restorePollTimer = setInterval(pollRestoreStatus, 2000)
}

async function pollRestoreStatus() {
  try {
    const data = await client.get('/backups/restore-status', { suppressErrorMessage: true })
    if (data && data.status && data.status !== 'idle') {
      restoreProgress.value = data
      if (data.status === 'done' || data.status === 'error') {
        clearInterval(restorePollTimer)
        restorePollTimer = null
        restorePolling.value = false
        if (data.status === 'done') {
          ElMessage.success(t('settings.backup.restoreSuccess'))
          setTimeout(() => window.location.reload(), 2000)
        } else {
          ElMessage.error(data.message || t('common.failed'))
        }
      }
    } else if (data && data.maintenance === false && restoreProgress.value) {
      // Restore finished, maintenance off
      clearInterval(restorePollTimer)
      restorePollTimer = null
      restorePolling.value = false
      restoreProgress.value = null
      ElMessage.success(t('settings.backup.restoreSuccess'))
      setTimeout(() => window.location.reload(), 1000)
    }
  } catch (e) {
    // 503 expected during maintenance — keep polling
  }
}

// Remote restore from storage
const remoteRestoreDialogVisible = ref(false)
const remoteRestoreStorageId = ref(null)
const remoteRestoreFiles = ref([])
const remoteRestoreLoading = ref(false)

async function showRemoteRestore(storageId) {
  remoteRestoreStorageId.value = storageId
  remoteRestoreDialogVisible.value = true
  remoteRestoreLoading.value = true
  try {
    const data = await client.get(`/backups/storages/${storageId}/files`)
    remoteRestoreFiles.value = data || []
  } catch (e) { ElMessage.error(t('common.failed')) } finally { remoteRestoreLoading.value = false }
}

async function handleRemoteRestore(filename) {
  await ElMessageBox.confirm(t('settings.backup.remoteRestoreConfirm'), t('common.confirm'), { type: 'warning' })
  try {
    await client.post('/backups/remote-restore', { storage_id: remoteRestoreStorageId.value, filename })
    ElMessage.success(t('settings.backup.restoreStarted'))
    remoteRestoreDialogVisible.value = false
    startRestorePolling()
  } catch (e) { ElMessage.error(t('common.failed')) }
}

// Maintenance mode
const maintenanceLoading = ref(false)
const maintenanceInfo = ref({ enabled: false, message: '', source: '' })
const maintenanceMessage = ref('')

async function fetchMaintenanceStatus() {
  try {
    const data = await client.get('/system/maintenance')
    maintenanceInfo.value = { enabled: data?.enabled || false, message: data?.message || '', source: data?.source || '' }
    maintenanceMessage.value = data?.message || ''
  } catch { /* ignore */ }
}

async function toggleMaintenance(enabled) {
  maintenanceLoading.value = true
  try {
    const msg = enabled ? maintenanceMessage.value.trim() : ''
    await client.put('/system/maintenance', { enabled, message: msg })
    ElMessage.success(enabled ? t('maintenance.enabled') : t('maintenance.disabled'))
    await fetchMaintenanceStatus()
  } catch (e) { ElMessage.error(t('common.failed')) } finally { maintenanceLoading.value = false }
}

function getStorageName(storageId) {
  const s = storages.value.find(s => s.id === storageId)
  return s ? `${s.name} (${s.type})` : '-'
}

function showAddSchedule() { editingSched.value = null; schedForm.value = { name: '', cron: '0 3 * * *', storage_id: null, retain_days: 30, delete_local_after_upload: false, enabled: true }; schedDialogVisible.value = true }
function showEditSchedule(row) { editingSched.value = row; schedForm.value = { ...row }; schedDialogVisible.value = true }
async function handleSaveSchedule() {
  try {
    if (editingSched.value) { await client.put(`/backups/schedules/${editingSched.value.id}`, schedForm.value) }
    else { await client.post('/backups/schedules', schedForm.value) }
    ElMessage.success(t('common.success')); schedDialogVisible.value = false; fetchSchedules()
  } catch (e) { ElMessage.error(t('common.failed')) }
}
async function handleDeleteSchedule(row) { await ElMessageBox.confirm(t('common.confirm')); await client.delete(`/backups/schedules/${row.id}`); ElMessage.success(t('common.success')); fetchSchedules() }

function showAddStorage() { editingStorage.value = null; storageForm.value = { name: '', type: 's3', config: {}, enabled: false }; storageDialogVisible.value = true }
function showEditStorage(row) { editingStorage.value = row; storageForm.value = { ...row, config: row.config ? JSON.parse(row.config) : {} }; storageDialogVisible.value = true }
async function handleSaveStorage() {
  try {
    const payload = { ...storageForm.value, config: JSON.stringify(storageForm.value.config) }
    if (editingStorage.value) { await client.put(`/backups/storages/${editingStorage.value.id}`, payload) }
    else { await client.post('/backups/storages', payload) }
    ElMessage.success(t('common.success')); storageDialogVisible.value = false; fetchStorages()
  } catch (e) { ElMessage.error(t('common.failed')) }
}
async function handleDeleteStorage(row) { await ElMessageBox.confirm(t('common.confirm')); await client.delete(`/backups/storages/${row.id}`); ElMessage.success(t('common.success')); fetchStorages() }

// FTP Upload
const ftpDialogVisible = ref(false)
const ftpUploading = ref(false)
const ftpBackupId = ref(null)
const ftpForm = ref({ host: '', port: 21, username: '', password: '', path: '/backups' })

function showUploadFTP(row) {
  ftpBackupId.value = row.id
  ftpForm.value = { host: '', port: 21, username: '', password: '', path: '/backups', delete_local: false }
  ftpDialogVisible.value = true
}

async function handleUploadFTP() {
  if (!ftpForm.value.host) { ElMessage.error('Host is required'); return }
  ftpUploading.value = true
  try {
    await client.post(`/backups/${ftpBackupId.value}/upload-ftp`, ftpForm.value)
    ElMessage.success('Uploaded to FTP successfully')
    ftpDialogVisible.value = false
    fetchBackups()
  } catch (e) { ElMessage.error(t('common.failed')) } finally { ftpUploading.value = false }
}

// S3 Upload
const s3DialogVisible = ref(false)
const s3Uploading = ref(false)
const s3BackupId = ref(null)
const s3Form = ref({ endpoint: '', bucket: '', access_key: '', secret_key: '' })

function showUploadS3(row) {
  s3BackupId.value = row.id
  s3Form.value = { endpoint: '', bucket: '', access_key: '', secret_key: '', delete_local: false }
  s3DialogVisible.value = true
}

async function handleUploadS3() {
  if (!s3Form.value.endpoint || !s3Form.value.bucket) { ElMessage.error('Endpoint and Bucket are required'); return }
  s3Uploading.value = true
  try {
    await client.post(`/backups/${s3BackupId.value}/upload-s3`, s3Form.value)
    ElMessage.success('Uploaded to S3 successfully')
    s3DialogVisible.value = false
    fetchBackups()
  } catch (e) { ElMessage.error(t('common.failed')) } finally { s3Uploading.value = false }
}

// WebDAV Upload
const webdavDialogVisible = ref(false)
const webdavUploading = ref(false)
const webdavBackupId = ref(null)
const webdavForm = ref({ url: '', username: '', password: '' })

function showUploadWebDAV(row) {
  webdavBackupId.value = row.id
  webdavForm.value = { url: '', username: '', password: '', delete_local: false }
  webdavDialogVisible.value = true
}

async function handleUploadWebDAV() {
  if (!webdavForm.value.url) { ElMessage.error('URL is required'); return }
  webdavUploading.value = true
  try {
    await client.post(`/backups/${webdavBackupId.value}/upload-webdav`, webdavForm.value)
    ElMessage.success('Uploaded to WebDAV successfully')
    webdavDialogVisible.value = false
    fetchBackups()
  } catch (e) { ElMessage.error(t('common.failed')) } finally { webdavUploading.value = false }
}

// GitHub acceleration
async function fetchGitHubConfig() { try { const d = await client.get('/system/github-config'); if (d) githubForm.value = { ...githubForm.value, ...d } } catch (e) { ElMessage.error(t('common.failed')) } }
async function saveGitHubConfig() { githubSaving.value = true; try { await client.put('/system/github-config', githubForm.value); ElMessage.success(t('common.success')) } catch (e) { ElMessage.error(t('common.failed')) } finally { githubSaving.value = false } }

// Self-unban config
async function fetchSelfUnbanConfig() {
  try {
    const d = await client.get('/system/settings', { params: { prefix: 'self_unban' } })
    if (d && d.items) {
      for (const item of d.items) {
        if (item.key === 'self_unban.enabled') selfUnbanForm.value.enabled = item.value === 'true'
        if (item.key === 'self_unban.cooldown_hours') selfUnbanForm.value.cooldown_hours = parseInt(item.value) || 24
        if (item.key === 'self_unban.applicable_status') {
          try { selfUnbanForm.value.applicable_status = JSON.parse(item.value) } catch { selfUnbanForm.value.applicable_status = ['suspended'] }
        }
      }
    }
  } catch (e) { ElMessage.error(t('common.failed')) }
}
async function saveSelfUnbanConfig() {
  selfUnbanSaving.value = true
  try {
    await client.put('/system/settings/batch', {
      items: [
        { key: 'self_unban.enabled', value: String(selfUnbanForm.value.enabled) },
        { key: 'self_unban.cooldown_hours', value: String(selfUnbanForm.value.cooldown_hours) },
        { key: 'self_unban.applicable_status', value: JSON.stringify(selfUnbanForm.value.applicable_status) },
      ]
    })
    ElMessage.success(t('common.success'))
  } catch (e) { ElMessage.error(t('common.failed')) } finally { selfUnbanSaving.value = false }
}

// Security config (encryption key)
async function fetchEncryptionKeyStatus() {
  try {
    const d = await client.get('/system/encryption-key-status')
    if (d) {
      encryptionKeyStatus.value = {
        configured: d.configured,
        text: d.configured ? 'settings.security.statusConfigured' : 'settings.security.statusNotConfigured',
        type: d.configured ? 'success' : 'danger'
      }
    }
  } catch (e) {
    encryptionKeyStatus.value = { configured: false, text: 'settings.security.statusUnknown', type: 'warning' }
  }
}
async function generateEncryptionKey() {
  generatingKey.value = true
  try {
    const d = await client.post('/system/generate-encryption-key')
    if (d && d.success) {
      ElMessage.success(t('settings.security.genSuccess'))
      await fetchEncryptionKeyStatus()
    } else {
      ElMessage.error(d.message || t('settings.security.genFailed'))
    }
  } catch (e) {
    ElMessage.error(t('settings.security.genError'))
  } finally { generatingKey.value = false }
}

// JWT secret
async function fetchJWTSecretStatus() {
  try {
    const d = await client.get('/system/jwt-secret-status')
    if (d) jwtSecretStatus.value = { configured: d.configured || false, preview: d.preview || '' }
  } catch (e) { /* ignore */ }
}
async function regenerateJWTSecret() {
  try {
    await ElMessageBox.confirm(t('settings.security.regenerateJwtConfirm'), t('common.confirm'), { type: 'warning' })
  } catch { return }
  regeneratingJwt.value = true
  try {
    const d = await client.post('/system/regenerate-jwt-secret')
    if (d && d.success) {
      ElMessage.success(t('settings.security.regenerateJwtSuccess'))
      await fetchJWTSecretStatus()
    } else {
      ElMessage.error(d.message || t('settings.security.regenerateJwtFailed'))
    }
  } catch (e) { ElMessage.error(t('settings.security.regenerateJwtFailed')) } finally { regeneratingJwt.value = false }
}

// OAuth config
async function fetchOAuthConfig() {
  try {
    const d = await client.get('/system/settings', { params: { prefix: 'oauth' } })
    if (d && d.items) {
      for (const item of d.items) {
        if (item.key === 'oauth.callback_domain') oauthForm.value.callback_domain = item.value || ''
        if (item.key === 'oauth.github_enabled') oauthForm.value.github_enabled = item.value === 'true'
        if (item.key === 'oauth.github_client_id') oauthForm.value.github_client_id = item.value
        if (item.key === 'oauth.github_client_secret') oauthForm.value.github_client_secret = item.value
        if (item.key === 'oauth.google_enabled') oauthForm.value.google_enabled = item.value === 'true'
        if (item.key === 'oauth.google_client_id') oauthForm.value.google_client_id = item.value
        if (item.key === 'oauth.google_client_secret') oauthForm.value.google_client_secret = item.value
      }
    }
  } catch (e) { ElMessage.error(t('common.failed')) }
}
async function saveOAuthConfig() {
  oauthSaving.value = true
  try {
    await client.put('/system/settings/batch', {
      items: [
        { key: 'oauth.callback_domain', value: oauthForm.value.callback_domain || '' },
        { key: 'oauth.github_enabled', value: String(oauthForm.value.github_enabled) },
        { key: 'oauth.github_client_id', value: oauthForm.value.github_client_id },
        { key: 'oauth.github_client_secret', value: oauthForm.value.github_client_secret },
        { key: 'oauth.google_enabled', value: String(oauthForm.value.google_enabled) },
        { key: 'oauth.google_client_id', value: oauthForm.value.google_client_id },
        { key: 'oauth.google_client_secret', value: oauthForm.value.google_client_secret },
      ]
    })
    ElMessage.success(t('common.success'))
    // 保存后回读：secret 以掩码 ******** 回显，前端不残留明文
    await fetchOAuthConfig()
  } catch (e) { ElMessage.error(t('common.failed')) } finally { oauthSaving.value = false }
}

// Currency config
async function fetchCurrencyConfig() {
  try {
    const d = await client.get('/system/settings', { params: { prefix: 'currency' } })
    if (d && d.items) {
      for (const item of d.items) {
        if (item.key === 'currency.default') currencyForm.value.default_currency = item.value || 'CNY'
      }
    }
  } catch (e) { ElMessage.error(t('common.failed')) }
}
async function saveCurrencyConfig() {
  currencySaving.value = true
  try {
    await client.put('/system/settings/batch', {
      items: [
        { key: 'currency.default', value: currencyForm.value.default_currency },
      ]
    })
    ElMessage.success(t('common.success'))
  } catch (e) { ElMessage.error(t('common.failed')) } finally { currencySaving.value = false }
}

// Database config
async function fetchDBConfig() {
  try {
    dbTypeInitializing.value = true
    const d = await client.get('/system/db-config')
    if (d) dbForm.value = { ...dbForm.value, ...d }
  } catch (e) { ElMessage.error(t('common.failed')) } finally { dbTypeInitializing.value = false }
}
async function saveDBConfig() {
  dbSaving.value = true
  try {
    await client.put('/system/db-config', dbForm.value)
    ElMessage.success(t('settings.db.saveSuccess'))
    fetchDBConfig() // refresh to show updated restart_needed
  } catch (e) { ElMessage.error(t('common.failed')) } finally { dbSaving.value = false }
}
async function testDBConnection() {
  dbTesting.value = true
  try {
    await client.post('/system/db-config/test', dbForm.value)
    ElMessage.success(t('settings.db.testSuccess'))
  } catch (e) { ElMessage.error(t('common.failed')) } finally { dbTesting.value = false }
}
async function handleMigrateDB() {
  if (!dbForm.value.dsn) {
    return
  }
  if (dbForm.value.dsn.includes('********')) {
    ElMessage.warning(t('settings.db.migrateHint'))
    return
  }
  await ElMessageBox.confirm(t('settings.db.migrateConfirm'), t('common.confirm'), { type: 'warning' })
  dbMigrating.value = true
  try {
        // 一步切换：先迁移数据（写不进去会直接失败），成功后才写 .env 并读回校验 ——
    // 这样不会出现"只保存了配置却没迁移"→ 重启后指向空库的情况。
    const d = await client.post('/system/db-config/switch', { type: dbForm.value.type, dsn: dbForm.value.dsn, max_open_conns: dbForm.value.max_open_conns, max_idle_conns: dbForm.value.max_idle_conns })
    ElMessage.success(t('settings.db.migrateSuccess') + ' (' + (d.tables_copied || 0) + ' tables, ' + (d.rows_copied || 0) + ' rows)')
  } catch (e) { ElMessage.error(t('common.failed')) } finally { dbMigrating.value = false }
}
function showAddDomain() { editingDomainIndex.value = null; domainForm.value = { name: '', url: '' }; domainDialogVisible.value = true }
function showEditDomain(row, index) { editingDomainIndex.value = index; domainForm.value = { ...row }; domainDialogVisible.value = true }
function handleSaveDomain() {
  if (!domainForm.value.name || !domainForm.value.url) return
  const domains = [...githubForm.value.domains]
  if (editingDomainIndex.value !== null) { domains[editingDomainIndex.value] = { ...domainForm.value } }
  else { domains.push({ ...domainForm.value, enabled: true }) }
  githubForm.value.domains = domains
  domainDialogVisible.value = false
}
function handleDeleteDomain(index) { githubForm.value.domains.splice(index, 1) }

onMounted(() => { fetchPanelConfig(); fetchEnvConfig(); fetchNotifyConfig(); fetchMonitorConfig(); fetchCertificates(); fetchTemplates(); fetchEmails(); fetchBackups(); fetchSchedules(); fetchStorages(); fetchGitHubConfig(); fetchSelfUnbanConfig(); fetchEncryptionKeyStatus(); fetchJWTSecretStatus(); fetchOAuthConfig(); fetchCurrencyConfig(); fetchDBConfig(); fetchMaintenanceStatus() })
</script>

<style scoped>
.template-vars-doc { font-size: 13px; line-height: 1.7; color: #303133; }
.template-vars-doc h2 { font-size: 16px; margin: 14px 0 8px; }
.template-vars-doc h3 { font-size: 14px; margin: 12px 0 6px; }
.template-vars-doc p { margin: 6px 0; }
.template-vars-doc ul { padding-left: 18px; margin: 6px 0; }
.template-vars-doc table { border-collapse: collapse; width: 100%; margin: 8px 0; }
.template-vars-doc th, .template-vars-doc td { border: 1px solid #dcdfe6; padding: 5px 10px; text-align: left; font-size: 12px; }
.template-vars-doc th { background: #f5f7fa; font-weight: 600; }
.template-vars-doc code { background: #f5f7fa; border-radius: 3px; padding: 1px 5px; font-size: 12px; color: #c7254e; }
</style>
