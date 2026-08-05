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
              <el-input v-model="notifyForm.telegram_bot_token" type="password" show-password placeholder="123456:ABC-DEF..." />
            </el-form-item>
            <el-form-item v-if="notifyForm.telegram_enabled" :label="t('settings.telegram.chatId')">
              <el-input v-model="notifyForm.telegram_chat_id" placeholder="-1001234567890" />
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

        <el-card>
          <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:16px">
            <span style="font-weight:500">{{ t('settings.templates.title') }}</span>
            <el-button type="primary" size="small" @click="showAddTemplate">{{ t('settings.templates.add') }}</el-button>
          </div>
          <el-table :data="templates" stripe>
            <el-table-column prop="name" :label="t('common.name')" width="150" />
            <el-table-column prop="type" label="Type" width="80">
              <template #default="{row}"><el-tag size="small">{{ row.type }}</el-tag></template>
            </el-table-column>
            <el-table-column prop="subject" label="Subject" width="150" />
            <el-table-column prop="cron" label="Cron" width="120" />
            <el-table-column label="Enabled" width="80">
              <template #default="{row}"><el-tag :type="row.enabled?'success':'info'" size="small">{{ row.enabled ? 'ON' : 'OFF' }}</el-tag></template>
            </el-table-column>
            <el-table-column :label="t('common.actions')" width="120">
              <template #default="{row}">
                <el-button size="small" @click="showEditTemplate(row)">{{ t('common.edit') }}</el-button>
                <el-button size="small" type="danger" @click="handleDeleteTemplate(row)">{{ t('common.delete') }}</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>

        <div style="margin-top:20px;text-align:right">
          <el-button type="primary" :loading="notifySaving" @click="saveNotifyConfig">{{ t('common.save') }}</el-button>
        </div>
      </el-tab-pane>

      <!-- Backup & Restore -->
      <el-tab-pane :label="t('settings.backup.title')" name="backup">
        <!-- Create & Upload -->
        <el-card style="margin-bottom:20px">
          <div style="display:flex;gap:12px;align-items:center">
            <el-button type="primary" :loading="backupCreating" @click="handleCreateBackup">{{ t('settings.backup.create') }}</el-button>
            <el-upload :auto-upload="false" :show-file-list="false" :on-change="handleUploadRestore" accept=".tar.gz,.gz">
              <el-button type="warning">{{ t('settings.backup.uploadRestore') }}</el-button>
            </el-upload>
          </div>
        </el-card>

        <!-- Backup List -->
        <el-card style="margin-bottom:20px">
          <template #header><span>{{ t('settings.backup.list') }}</span></template>
          <el-table :data="backups" v-loading="backupsLoading" stripe>
            <el-table-column prop="name" :label="t('common.name')" width="200" />
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
                <el-button size="small" @click="handleDownload(row)">{{ t('settings.backup.download') }}</el-button>
                <el-button size="small" type="warning" @click="handleRestore(row)">{{ t('settings.backup.restore') }}</el-button>
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
          <el-form :model="oauthForm" label-width="160px" style="max-width:600px">
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
      </el-form>
      <template #footer>
        <el-button @click="webdavDialogVisible=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="webdavUploading" @click="handleUploadWebDAV">{{ t('common.upload') }}</el-button>
      </template>
    </el-dialog>

    <!-- Template Dialog -->
    <el-dialog v-model="tmplDialogVisible" :title="editingTmpl ? t('common.edit') : t('settings.templates.add')" width="600">
      <el-form :model="tmplForm" label-width="100px">
        <el-form-item :label="t('common.name')"><el-input v-model="tmplForm.name" /></el-form-item>
        <el-form-item label="Type">
          <el-select v-model="tmplForm.type" style="width:100%">
            <el-option label="Telegram" value="telegram" />
            <el-option label="Email" value="email" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="tmplForm.type==='email'" :label="t('settings.templates.recipients')">
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
        <el-form-item label="Cron">
          <el-select v-model="tmplForm.cron" filterable allow-create style="width:100%">
            <el-option :label="t('settings.cron.manual')" value="" />
            <el-option :label="t('settings.cron.hourly')" value="0 * * * *" />
            <el-option :label="t('settings.cron.daily9')" value="0 9 * * *" />
            <el-option :label="t('settings.cron.daily21')" value="0 21 * * *" />
            <el-option :label="t('settings.cron.monday9')" value="0 9 * * 1" />
          </el-select>
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
const panelForm = ref({ listen_addr: '0.0.0.0', listen_port: '8080', subscription_port: '0', tls_enabled: false, tls_cert_id: '', tls_cert_path: '', tls_key_path: '', mtls_enabled: false, mtls_ca_path: '', agent_grpc_port: '18443', web_path: '/', timezone: 'Asia/Shanghai', data_dir: 'data', trusted_proxies: '', profile_web_page_url: '', mode: 'full' })
const envForm = ref({ safe_paths: 'data' })
const envSaving = ref(false)
const panelPort = computed({ get: () => parseInt(panelForm.value.listen_port) || 8080, set: (v) => { panelForm.value.listen_port = String(v) } })
const subPort = computed({ get: () => parseInt(panelForm.value.subscription_port) || 0, set: (v) => { panelForm.value.subscription_port = String(v) } })
const agentGRPCPort = computed({ get: () => parseInt(panelForm.value.agent_grpc_port) || 18443, set: (v) => { panelForm.value.agent_grpc_port = String(v) } })
const panelSaving = ref(false)
const migrating = ref(false)
const certificates = ref([])

// Notification
const notifyForm = ref({ telegram_enabled: false, telegram_bot_token: '', telegram_chat_id: '', smtp_enabled: false, smtp_host: '', smtp_port: '465', smtp_user: '', smtp_pass: '', smtp_from: '', smtp_tls: true })
const notifySaving = ref(false)
const smtpTesting = ref(false)
const tgTesting = ref(false)
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
const schedForm = ref({ name: '', cron: '0 3 * * *', retain_days: 30, enabled: true })
const storageDialogVisible = ref(false)
const editingStorage = ref(null)
const storageForm = ref({ name: '', type: 's3', config: {}, enabled: false })
const tmplDialogVisible = ref(false)
const editingTmpl = ref(null)
const tmplForm = ref({ name: '', type: 'telegram', recipients: '', subject: '', content: '', cron: '', enabled: false })
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
const oauthForm = ref({ github_enabled: false, github_client_id: '', github_client_secret: '', google_enabled: false, google_client_id: '', google_client_secret: '' })
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

// OAuth callback URLs (computed from current location)
const githubCallbackURL = computed(() => {
  const base = window.location.origin
  return base + '/api/v1/client-auth/oauth/github/callback'
})
const googleCallbackURL = computed(() => {
  const base = window.location.origin
  return base + '/api/v1/client-auth/oauth/google/callback'
})

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

async function savePanelConfig() { panelSaving.value = true; try { await client.put('/system/panel-config', panelForm.value); ElMessage.success(t('common.success')) } catch (e) { ElMessage.error(t('common.failed')) } finally { panelSaving.value = false } }

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
async function saveNotifyConfig() { notifySaving.value = true; try { await client.put('/system/notification-config', notifyForm.value); ElMessage.success(t('common.success')) } catch (e) { ElMessage.error(t('common.failed')) } finally { notifySaving.value = false } }
async function testSMTP() { smtpTesting.value = true; try { await client.post('/system/test-smtp', notifyForm.value); ElMessage.success(t('settings.smtp.testSuccess')) } catch (e) { ElMessage.error(t('common.failed')) } finally { smtpTesting.value = false } }
async function testTelegram() { tgTesting.value = true; try { await client.post('/system/test-telegram', notifyForm.value); ElMessage.success(t('settings.telegram.testSuccess')) } catch (e) { ElMessage.error(t('common.failed')) } finally { tgTesting.value = false } }

function showAddTemplate() {
  editingTmpl.value = null
  tmplForm.value = { name: '', type: 'telegram', recipients: '', subject: '', content: '', cron: '', enabled: false }
  tmplRecipients.value = []
  tmplDialogVisible.value = true
}
function showEditTemplate(row) {
  editingTmpl.value = row
  tmplForm.value = { ...row }
  tmplRecipients.value = row.recipients ? row.recipients.split(',').map(s => s.trim()).filter(Boolean) : []
  tmplDialogVisible.value = true
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

function handleDownload(row) {
  const token = localStorage.getItem('access_token')
  window.open(`/api/v1/backups/${row.id}/download?token=${token}`, '_blank')
}

async function handleRestore(row) {
  await ElMessageBox.confirm(t('settings.backup.restoreConfirm'), t('common.confirm'), { type: 'warning' })
  try { await client.post(`/backups/${row.id}/restore`); ElMessage.success(t('settings.backup.restoreSuccess')) } catch (e) { ElMessage.error(t('common.failed')) }
}

async function handleDeleteBackup(row) {
  await ElMessageBox.confirm(t('common.confirm'))
  await client.delete(`/backups/${row.id}`); ElMessage.success(t('common.success')); fetchBackups()
}

async function handleUploadRestore(file) {
  await ElMessageBox.confirm(t('settings.backup.uploadRestoreConfirm'), t('common.confirm'), { type: 'warning' })
  const formData = new FormData()
  formData.append('file', file.raw)
  try { await client.post('/backups/upload-restore', formData, { headers: { 'Content-Type': 'multipart/form-data' } }); ElMessage.success(t('settings.backup.restoreSuccess')) } catch (e) { ElMessage.error(t('common.failed')) }
}

function getStorageName(storageId) {
  const s = storages.value.find(s => s.id === storageId)
  return s ? `${s.name} (${s.type})` : '-'
}

function showAddSchedule() { editingSched.value = null; schedForm.value = { name: '', cron: '0 3 * * *', storage_id: null, retain_days: 30, enabled: true }; schedDialogVisible.value = true }
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
  ftpForm.value = { host: '', port: 21, username: '', password: '', path: '/backups' }
  ftpDialogVisible.value = true
}

async function handleUploadFTP() {
  if (!ftpForm.value.host) { ElMessage.error('Host is required'); return }
  ftpUploading.value = true
  try {
    await client.post(`/backups/${ftpBackupId.value}/upload-ftp`, ftpForm.value)
    ElMessage.success('Uploaded to FTP successfully')
    ftpDialogVisible.value = false
  } catch (e) { ElMessage.error(t('common.failed')) } finally { ftpUploading.value = false }
}

// S3 Upload
const s3DialogVisible = ref(false)
const s3Uploading = ref(false)
const s3BackupId = ref(null)
const s3Form = ref({ endpoint: '', bucket: '', access_key: '', secret_key: '' })

function showUploadS3(row) {
  s3BackupId.value = row.id
  s3Form.value = { endpoint: '', bucket: '', access_key: '', secret_key: '' }
  s3DialogVisible.value = true
}

async function handleUploadS3() {
  if (!s3Form.value.endpoint || !s3Form.value.bucket) { ElMessage.error('Endpoint and Bucket are required'); return }
  s3Uploading.value = true
  try {
    await client.post(`/backups/${s3BackupId.value}/upload-s3`, s3Form.value)
    ElMessage.success('Uploaded to S3 successfully')
    s3DialogVisible.value = false
  } catch (e) { ElMessage.error(t('common.failed')) } finally { s3Uploading.value = false }
}

// WebDAV Upload
const webdavDialogVisible = ref(false)
const webdavUploading = ref(false)
const webdavBackupId = ref(null)
const webdavForm = ref({ url: '', username: '', password: '' })

function showUploadWebDAV(row) {
  webdavBackupId.value = row.id
  webdavForm.value = { url: '', username: '', password: '' }
  webdavDialogVisible.value = true
}

async function handleUploadWebDAV() {
  if (!webdavForm.value.url) { ElMessage.error('URL is required'); return }
  webdavUploading.value = true
  try {
    await client.post(`/backups/${webdavBackupId.value}/upload-webdav`, webdavForm.value)
    ElMessage.success('Uploaded to WebDAV successfully')
    webdavDialogVisible.value = false
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
        { key: 'oauth.github_enabled', value: String(oauthForm.value.github_enabled) },
        { key: 'oauth.github_client_id', value: oauthForm.value.github_client_id },
        { key: 'oauth.github_client_secret', value: oauthForm.value.github_client_secret },
        { key: 'oauth.google_enabled', value: String(oauthForm.value.google_enabled) },
        { key: 'oauth.google_client_id', value: oauthForm.value.google_client_id },
        { key: 'oauth.google_client_secret', value: oauthForm.value.google_client_secret },
      ]
    })
    ElMessage.success(t('common.success'))
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
    const d = await client.post('/system/db-config/migrate', { type: dbForm.value.type, dsn: dbForm.value.dsn, max_open_conns: dbForm.value.max_open_conns, max_idle_conns: dbForm.value.max_idle_conns })
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

onMounted(() => { fetchPanelConfig(); fetchEnvConfig(); fetchNotifyConfig(); fetchCertificates(); fetchTemplates(); fetchEmails(); fetchBackups(); fetchSchedules(); fetchStorages(); fetchGitHubConfig(); fetchSelfUnbanConfig(); fetchEncryptionKeyStatus(); fetchJWTSecretStatus(); fetchOAuthConfig(); fetchCurrencyConfig(); fetchDBConfig() })
</script>
