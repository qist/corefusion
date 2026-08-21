<template>
  <div>
    <h2 style="margin-bottom:20px">Caddy {{ t('proxy.management') }}</h2>

    <!-- Port conflict warning -->
    <el-alert v-if="nginxRunning" type="warning" :closable="false" show-icon style="margin-bottom:16px">
      Nginx {{ t('proxy.isRunning') }}，{{ t('proxy.portConflictHint') }}
    </el-alert>

    <!-- Status Card -->
    <el-card shadow="hover" style="margin-bottom:20px">
      <template #header>
        <div style="display:flex;justify-content:space-between;align-items:center">
          <div style="display:flex;align-items:center;gap:12px">
            <span style="font-size:18px;font-weight:600">Caddy</span>
            <el-tag v-if="proxyInfo.installed" :type="proxyInfo.status==='running'?'success':'info'" size="small">
              {{ proxyInfo.status }}
            </el-tag>
            <el-tag v-else type="danger" size="small">{{ t('proxy.notInstalled') }}</el-tag>
          </div>
          <div style="display:flex;gap:8px">
            <el-button size="small" type="danger" plain @click="$emit('back')">{{ t('proxy.switch') }}</el-button>
          </div>
        </div>
      </template>
      <el-row :gutter="24">
        <el-col :span="12">
          <div style="display:flex;flex-direction:column;gap:10px;font-size:13px">
            <div style="display:flex;justify-content:space-between">
              <span style="color:#909399">{{ t('proxy.version') }}</span>
              <span style="font-weight:600">{{ proxyInfo.version }}</span>
            </div>
            <div style="display:flex;justify-content:space-between">
              <span style="color:#909399">{{ t('proxy.path') }}</span>
              <span style="font-weight:600;font-size:12px">{{ proxyInfo.path }}</span>
            </div>
            <template v-if="guardian.state && guardian.state !== 'no_guardian'">
              <el-divider style="margin:4px 0" />
              <div style="display:flex;align-items:center;gap:12px;margin-bottom:8px">
                <span style="color:#909399;font-size:13px">{{ t('proxy.autoRestart') }}</span>
                <el-switch v-model="guardianConfig.autoRestart" @change="saveGuardianConfig" />
                <el-input-number v-model="guardianConfig.maxRestarts" :min="1" :max="100" size="small" style="width:100px" @change="saveGuardianConfig" />
                <span style="font-size:12px;color:#909399">{{ t('proxy.maxRestarts') }}</span>
              </div>
              <div style="display:flex;gap:16px;font-size:13px">
                <div style="display:flex;gap:6px;align-items:center">
                  <span style="color:#909399">{{ t('proxy.restartStatus') }}</span>
                  <el-tag :type="guardian.current_restarts > 0 ? 'warning' : 'success'" size="small">
                    {{ guardian.current_restarts > 0 ? (t('proxy.restarted')) : (t('proxy.normal')) }}
                  </el-tag>
                </div>
                <div style="display:flex;gap:6px;align-items:center">
                  <span style="color:#909399">{{ t('proxy.restartCount') }}</span>
                  <span>{{ guardian.current_restarts }}/{{ guardian.max_restarts }}</span>
                </div>
                <div v-if="guardian.last_crash_at && guardian.last_crash_at !== '0001-01-01T00:00:00Z'" style="display:flex;gap:6px;align-items:center">
                  <span style="color:#909399">{{ t('proxy.lastCrash') }}</span>
                  <span style="color:#F56C6C">{{ formatTime(guardian.last_crash_at) }}</span>
                </div>
              </div>
            </template>
          </div>
        </el-col>
        <el-col :span="12">
          <div style="display:flex;gap:8px;flex-wrap:wrap">
            <template v-if="!proxyInfo.installed">
              <el-button type="primary" :loading="releasesLoading" @click="showReleases">{{ t('proxy.download') }}</el-button>
            </template>
            <template v-else>
<el-button v-if="proxyInfo.status!=='running'" type="success" :disabled="nginxRunning" @click="proxyAction('start')">{{ t('proxy.start') }}</el-button>
<el-button v-else type="danger" @click="proxyAction('stop')">{{ t('proxy.stop') }}</el-button>
              <el-button @click="proxyAction('restart')">{{ t('proxy.restart') }}</el-button>
              <el-button type="warning" plain @click="showReleases">{{ t('proxy.switchVersion') }}</el-button>
              <el-button plain @click="viewConfig">{{ t('proxy.viewConfig') }}</el-button>
            </template>
          </div>
        </el-col>
      </el-row>
    </el-card>

    <!-- WAF Master Switch -->
    <el-card shadow="hover" style="margin-bottom:20px">
      <template #header>
        <div style="display:flex;justify-content:space-between;align-items:center">
          <span style="font-weight:600">WAF {{ t('proxy.firewall') }}</span>
          <el-switch v-model="wafMasterEnabled" :loading="wafStatusSaving" @change="saveWafMasterSwitch" />
        </div>
      </template>
      <div v-if="wafMasterEnabled" style="font-size:13px;color:#67C23A">
        WAF 已启用 — Caddyfile 将包含 caddyguard 配置
      </div>
      <div v-else style="font-size:13px;color:#909399">
        WAF 已关闭 — 不会生成任何 WAF 配置
      </div>
    </el-card>

    <!-- Log Output Mode -->
    <el-card style="margin-bottom:20px">
      <template #header>
        <div style="display:flex;justify-content:space-between;align-items:center">
          <span style="font-weight:600">{{ t('proxy.logOutputMode') }}</span>
          <el-button type="primary" size="small" :loading="logModeSaving" @click="saveCaddyLogMode">{{ t('common.save') }}</el-button>
        </div>
      </template>
      <el-radio-group v-model="caddyLogMode" @change="onCaddyLogModeChange">
        <el-radio-button value="file">{{ t('proxy.logToFile') }}</el-radio-button>
        <el-radio-button value="stdout">{{ t('proxy.logToStdout') }}</el-radio-button>
        <el-radio-button value="off">{{ t('proxy.logOff') }}</el-radio-button>
      </el-radio-group>
      <div style="margin-top:8px;font-size:12px;color:#909399">
        {{ caddyLogModeHint }}
      </div>
    </el-card>

    <!-- SNI Routing -->
    <el-card style="margin-bottom:20px">
      <template #header>
        <div style="display:flex;justify-content:space-between;align-items:center">
          <div style="display:flex;align-items:center;gap:12px">
            <span>SNI {{ t('proxy.routing') }}</span>
            <el-switch v-model="sniSettings.enabled" @change="onSNIToggle" />
          </div>
          <div v-if="sniSettings.enabled" style="display:flex;gap:8px;align-items:center">
            <el-button size="small" type="primary" @click="showAddSNI">+ SNI {{ t('proxy.rule') }}</el-button>
          </div>
        </div>
      </template>
      <template v-if="sniSettings.enabled">
        <div style="display:flex;gap:16px;align-items:flex-end;flex-wrap:wrap;margin-bottom:16px">
          <div>
            <div style="font-size:12px;color:#909399;margin-bottom:4px">SNI {{ t('proxy.listenPort') }}</div>
            <el-input v-model="sniSettings.listen_port" size="small" style="width:100px" placeholder="443" />
          </div>
          <div>
            <div style="font-size:12px;color:#909399;margin-bottom:4px">{{ t('proxy.localHttpsPort') }}</div>
            <el-input v-model="sniSettings.local_port" size="small" style="width:100px" placeholder="8443" />
          </div>
          <el-button size="small" type="success" @click="saveSNISettings">{{ t('common.save') }}</el-button>
          <el-button size="small" type="warning" @click="batchMigrate">{{ t('proxy.batchMigrate') }}</el-button>
        </div>
        <!-- Proxy Protocol Rules -->
        <div style="margin-bottom:16px;padding:12px;background:#f5f7fa;border-radius:4px">
          <div style="font-size:12px;color:#909399;margin-bottom:8px">Proxy Protocol {{ t('proxy.allowRules') }}</div>
          <div v-for="(rule, idx) in sniSettings.proxy_protocol_rules" :key="idx" style="display:flex;gap:8px;margin-bottom:6px;align-items:center">
            <el-input v-model="sniSettings.proxy_protocol_rules[idx]" size="small" style="width:250px" placeholder="127.0.0.0/8" />
            <el-button size="small" type="danger" plain @click="sniSettings.proxy_protocol_rules.splice(idx, 1)">-</el-button>
          </div>
          <el-button size="small" @click="sniSettings.proxy_protocol_rules.push('')">+ {{ t('common.add') }}</el-button>
          <el-button size="small" type="success" @click="saveSNISettings" style="margin-left:8px">{{ t('common.save') }}</el-button>
        </div>
        <el-table :data="sniPagedList" stripe size="small">
          <el-table-column prop="domain" label="SNI Domain" min-width="160" />
          <el-table-column label="Target" min-width="160">
            <template #default="{row}">{{ row.sni_target || row.upstream }}</template>
          </el-table-column>
          <el-table-column label="PP" width="60">
            <template #default="{row}"><el-tag v-if="row.proxy_protocol" size="small">v2</el-tag><span v-else>-</span></template>
          </el-table-column>
          <el-table-column :label="t('common.actions')" width="160">
            <template #default="{row}">
              <div style="display:inline-flex;gap:6px">
                <el-button size="small" @click="editSite(row)">{{ t('common.edit') }}</el-button>
                <el-button size="small" type="danger" plain @click="deleteSite(row)">{{ t('common.delete') }}</el-button>
              </div>
            </template>
          </el-table-column>
        </el-table>
        <PaginationBar v-model:page="sniPage" v-model:page-size="sniSize" :total="sniTotal" />
        <div style="margin-top:12px;font-size:12px;color:#909399">
          {{ t('proxy.sniFallback') }}: 127.0.0.1:{{ sniSettings.local_port }}
        </div>
      </template>
    </el-card>

    <!-- L4 Routing -->
    <el-card style="margin-bottom:20px">
      <template #header>
        <div style="display:flex;justify-content:space-between;align-items:center">
          <span>L4 {{ t('proxy.routing') }}</span>
          <el-button size="small" @click="showAddSite('l4')">+ L4</el-button>
        </div>
      </template>
      <el-table :data="l4PagedList" stripe size="small">
        <el-table-column :label="t('common.status')" width="70">
          <template #default="{row}">
            <el-switch v-model="row.enabled" size="small" @change="toggleSite(row)" />
          </template>
        </el-table-column>
        <el-table-column prop="domain" label="Name" width="120" />
        <el-table-column label="Upstream" min-width="140">
          <template #default="{row}">{{ row.upstream }}</template>
        </el-table-column>
        <el-table-column label="Port" width="80">
          <template #default="{row}">:{{ row.path }}</template>
        </el-table-column>
        <el-table-column label="Protocol" width="80">
          <template #default="{row}">{{ (row.l4_protocol).toUpperCase() }}</template>
        </el-table-column>
        <el-table-column :label="t('common.actions')" width="160">
          <template #default="{row}">
            <div style="display:inline-flex;gap:6px">
              <el-button size="small" @click="editSite(row)">{{ t('common.edit') }}</el-button>
              <el-button size="small" type="danger" plain @click="deleteSite(row)">{{ t('common.delete') }}</el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>
      <PaginationBar v-model:page="l4Page" v-model:page-size="l4Size" :total="l4Total" />
    </el-card>

    <!-- HTTP Site List -->
    <el-card style="margin-bottom:20px">
      <template #header>
        <div style="display:flex;justify-content:space-between;align-items:center">
          <span>{{ t('proxy.sites') }}</span>
          <div style="display:flex;gap:8px">
            <el-button size="small" @click="showAddSite('http')">+ HTTP</el-button>
            <el-button size="small" @click="showAddSite('websocket')">+ WS</el-button>
            <el-button size="small" @click="showAddSite('grpc')">+ gRPC</el-button>
            <el-button size="small" @click="showAddSite('static')">+ Static</el-button>
            <el-button v-if="isLocal" size="small" type="primary" @click="showPanelProxy">{{ t('proxy.proxyPanel') }}</el-button>
          </div>
        </div>
      </template>
      <el-table :data="httpPagedList" stripe>
        <el-table-column :label="t('common.status')" width="70">
          <template #default="{row}">
            <el-switch v-model="row.enabled" size="small" @change="toggleSite(row)" />
          </template>
        </el-table-column>
        <el-table-column prop="domain" :label="t('proxy.domain')" min-width="120" />
        <el-table-column prop="upstream" label="Upstream" min-width="120" />
        <el-table-column prop="path" label="Path" min-width="100">
          <template #default="{row}">
            <span>{{ row.path }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="template" :label="t('proxy.template')" width="90">
          <template #default="{row}"><el-tag size="small">{{ row.template }}</el-tag></template>
        </el-table-column>
        <el-table-column label="TLS" width="60">
          <template #default="{row}">
            <el-tag v-if="row.cert_path" type="success" size="small">✓</el-tag>
            <el-tag v-else type="info" size="small">-</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="WAF" width="80">
          <template #default="{row}">
            <div style="display:flex;align-items:center;gap:4px">
              <el-switch v-model="row._waf_enabled" size="small" @change="(val) => toggleSiteWAF(row, val)" />
              <el-button v-if="row._waf_enabled" size="small" text @click="openSiteWAF(row)">⚙</el-button>
            </div>
          </template>
        </el-table-column>
        <el-table-column :label="t('common.actions')" width="160" fixed="right">
          <template #default="{row}">
            <div style="display:inline-flex;gap:6px">
              <el-button size="small" @click="editSite(row)">{{ t('common.edit') }}</el-button>
              <el-button size="small" type="danger" plain @click="deleteSite(row)">{{ t('common.delete') }}</el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>
      <PaginationBar v-model:page="httpPage" v-model:page-size="httpSize" :total="httpTotal" />
    </el-card>

    <!-- WAF Management (CaddyGuard) -->
    <el-card style="margin-bottom:20px">
      <template #header>
        <div style="display:flex;justify-content:space-between;align-items:center">
          <span>WAF {{ t('proxy.management') }}</span>
          <el-tag :type="wafConfigForm.waf_enable === 'on' ? 'success' : 'info'" size="small">
            {{ wafConfigForm.waf_enable === 'on' ? 'ON' : 'OFF' }}
          </el-tag>
        </div>
      </template>
      <el-tabs v-model="wafTab">
        <el-tab-pane label="Config" name="config-form">
          <el-form :model="wafConfigForm" label-width="170px" size="small">
            <el-divider content-position="left">{{ t('proxy.wafSwitches') }}</el-divider>
            <el-form-item :label="t('proxy.wafEnable')"><el-switch v-model="wafConfigForm.waf_enable" active-value="on" inactive-value="off" /></el-form-item>
            <el-form-item :label="t('proxy.wafTrustProxy')"><el-switch v-model="wafConfigForm.trust_proxy_headers" active-value="on" inactive-value="off" /></el-form-item>
            <el-form-item :label="t('proxy.wafWhiteUrl')"><el-switch v-model="wafConfigForm.white_url_check" active-value="on" inactive-value="off" /></el-form-item>
            <el-form-item :label="t('proxy.wafWhiteIp')"><el-switch v-model="wafConfigForm.white_ip_check" active-value="on" inactive-value="off" /></el-form-item>
            <el-form-item :label="t('proxy.wafWhiteUa')">
              <el-switch v-model="wafConfigForm.white_ua_check" active-value="on" inactive-value="off" />
              <span style="font-size:11px;color:#909399;margin-left:8px">{{ t('proxy.wafWhiteUaHint') }}</span>
            </el-form-item>
            <el-form-item :label="t('proxy.wafBlackIp')"><el-switch v-model="wafConfigForm.black_ip_check" active-value="on" inactive-value="off" /></el-form-item>
            <el-form-item :label="t('proxy.wafUrlCheck')"><el-switch v-model="wafConfigForm.url_check" active-value="on" inactive-value="off" /></el-form-item>
            <el-form-item :label="t('proxy.wafUrlArgs')"><el-switch v-model="wafConfigForm.url_args_check" active-value="on" inactive-value="off" /></el-form-item>
            <el-form-item :label="t('proxy.wafUserAgent')"><el-switch v-model="wafConfigForm.user_agent_check" active-value="on" inactive-value="off" /></el-form-item>
            <el-form-item :label="t('proxy.wafCookie')"><el-switch v-model="wafConfigForm.cookie_check" active-value="on" inactive-value="off" /></el-form-item>
            <el-form-item :label="t('proxy.wafPost')"><el-switch v-model="wafConfigForm.post_check" active-value="on" inactive-value="off" /></el-form-item>
            <el-form-item :label="t('proxy.wafReferer')"><el-switch v-model="wafConfigForm.referer_check" active-value="on" inactive-value="off" /></el-form-item>
            <el-form-item :label="t('proxy.wafFileUpload')"><el-switch v-model="wafConfigForm.file_upload_check" active-value="on" inactive-value="off" /></el-form-item>
            <el-divider content-position="left">{{ t('proxy.wafAdvanced') }}</el-divider>
            <el-form-item :label="t('proxy.wafBodyless')">
              <el-switch v-model="wafConfigForm.bodyless" active-value="on" inactive-value="off" />
              <span style="font-size:11px;color:#909399;margin-left:8px">{{ t('proxy.wafBodylessHint') }}</span>
            </el-form-item>
            <el-form-item :label="t('proxy.wafMultipartStreaming')">
              <el-switch v-model="wafConfigForm.multipart_streaming_check" active-value="on" inactive-value="off" />
              <span style="font-size:11px;color:#909399;margin-left:8px">{{ t('proxy.wafMultipartStreamingHint') }}</span>
            </el-form-item>
            <el-form-item :label="t('proxy.wafUploadFilenameScan')">
              <el-input v-model="wafConfigForm.upload_filename_scan_limit" placeholder="0" style="width:120px" />
              <span style="font-size:11px;color:#909399;margin-left:8px">{{ t('proxy.wafUploadFilenameScanHint') }}</span>
            </el-form-item>
            <el-form-item :label="t('proxy.wafPostBodyScan')">
              <el-input v-model="wafConfigForm.post_body_scan_limit" placeholder="2097152" style="width:120px" />
              <span style="font-size:11px;color:#909399;margin-left:8px">{{ t('proxy.wafPostBodyScanHint') }}</span>
            </el-form-item>
            <el-divider content-position="left">{{ t('proxy.wafCcProtection') }}</el-divider>
            <el-form-item :label="t('proxy.wafCcCheck')"><el-switch v-model="wafConfigForm.cc_check" active-value="on" inactive-value="off" /></el-form-item>
            <el-form-item :label="t('proxy.wafCcRate')"><el-input v-model="wafConfigForm.cc_rate" placeholder="60/60" style="width:120px" /></el-form-item>
            <el-form-item :label="t('proxy.wafCcBlockTtl')"><el-input v-model="wafConfigForm.cc_block_ttl" placeholder="600" style="width:120px" /></el-form-item>
            <el-divider content-position="left">{{ t('proxy.wafOutputGroup') }}</el-divider>
            <el-form-item :label="t('proxy.wafOutput')">
              <el-select v-model="wafConfigForm.waf_output" style="width:120px">
                <el-option label="html" value="html" /><el-option label="redirect" value="redirect" />
              </el-select>
            </el-form-item>
            <el-form-item :label="t('proxy.wafRedirectUrl')" v-if="wafConfigForm.waf_output==='redirect'"><el-input v-model="wafConfigForm.waf_redirect_url" placeholder="https://example.com" style="width:300px" /></el-form-item>
            <el-divider content-position="left">{{ t('proxy.wafPaths') }}</el-divider>
            <el-form-item :label="t('proxy.wafLogDir')"><el-input v-model="wafConfigForm.log_dir" style="width:300px" /></el-form-item>
            <el-form-item :label="t('proxy.wafRuleDir')"><el-input v-model="wafConfigForm.rule_dir" style="width:300px" /></el-form-item>
          </el-form>
          <div style="margin-top:8px;text-align:right">
            <el-button type="success" @click="saveWAFConfigForm">{{ t('common.save') }}</el-button>
            <el-button @click="parseWAFConfigForm">{{ t('common.reset') }}</el-button>
          </div>
        </el-tab-pane>
        <el-tab-pane label="Source" name="config-source">
          <div style="margin-bottom:8px;font-size:12px;color:#909399">{{ t('proxy.wafConfigHint') }}</div>
          <el-input v-model="wafConfigContent" type="textarea" :rows="20" style="font-family:monospace;font-size:13px" />
          <div style="margin-top:8px;text-align:right">
            <el-button type="success" @click="saveWAFConfig">{{ t('common.save') }}</el-button>
            <el-button @click="fetchWAFConfig">{{ t('common.reset') }}</el-button>
          </div>
        </el-tab-pane>
        <el-tab-pane :label="t('proxy.wafRules')" name="rules">
          <el-table :data="wafRules" stripe @row-click="openWAFRule" style="cursor:pointer">
            <el-table-column prop="name" :label="t('proxy.wafRule')" width="180" />
            <el-table-column :label="t('proxy.wafRuleDesc')" min-width="200">
              <template #default="{ row }">{{ wafRuleDesc(row.name) }}</template>
            </el-table-column>
            <el-table-column prop="size" :label="t('proxy.wafSize')" width="120" />
            <el-table-column :label="t('common.action')" width="100">
              <template #default="{ row }">
                <el-button size="small" @click.stop="openWAFRule(row)">{{ t('common.edit') }}</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>
      </el-tabs>
    </el-card>

    <!-- WAF Rule Editor Dialog -->
    <el-dialog v-model="wafRuleVisible" :title="`WAF Rule: ${wafRuleName}`" width="70%">
      <el-input v-model="wafRuleContent" type="textarea" :rows="20" style="font-family:monospace;font-size:13px" />
      <template #footer>
        <el-button @click="wafRuleVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="success" @click="saveWAFRule">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- Site WAF Config Dialog -->
    <el-dialog v-model="siteWAFVisible" :title="`WAF: ${siteWAFDomain}`" width="800px">
      <el-tabs v-model="siteWAFTab">
        <el-tab-pane label="Config" name="config">
          <el-form :model="siteWAFConfig" label-width="160px" size="small">
            <el-divider content-position="left">{{ t('proxy.wafSwitches') }}</el-divider>
            <el-form-item :label="t('proxy.wafEnable')"><el-switch v-model="siteWAFConfig.waf_enable" active-value="on" inactive-value="off" /></el-form-item>
            <el-form-item :label="t('proxy.wafTrustProxy')"><el-switch v-model="siteWAFConfig.trust_proxy_headers" active-value="on" inactive-value="off" /></el-form-item>
            <el-form-item :label="t('proxy.wafUrlCheck')"><el-switch v-model="siteWAFConfig.url_check" active-value="on" inactive-value="off" /></el-form-item>
            <el-form-item :label="t('proxy.wafUrlArgs')"><el-switch v-model="siteWAFConfig.url_args_check" active-value="on" inactive-value="off" /></el-form-item>
            <el-form-item :label="t('proxy.wafUserAgent')"><el-switch v-model="siteWAFConfig.user_agent_check" active-value="on" inactive-value="off" /></el-form-item>
            <el-form-item :label="t('proxy.wafCookie')"><el-switch v-model="siteWAFConfig.cookie_check" active-value="on" inactive-value="off" /></el-form-item>
            <el-form-item :label="t('proxy.wafPost')"><el-switch v-model="siteWAFConfig.post_check" active-value="on" inactive-value="off" /></el-form-item>
            <el-form-item :label="t('proxy.wafReferer')"><el-switch v-model="siteWAFConfig.referer_check" active-value="on" inactive-value="off" /></el-form-item>
            <el-form-item :label="t('proxy.wafFileUpload')"><el-switch v-model="siteWAFConfig.file_upload_check" active-value="on" inactive-value="off" /></el-form-item>
            <el-divider content-position="left">{{ t('proxy.wafAdvanced') }}</el-divider>
            <el-form-item :label="t('proxy.wafBodyless')">
              <el-switch v-model="siteWAFConfig.bodyless" active-value="on" inactive-value="off" />
              <span style="font-size:11px;color:#909399;margin-left:8px">{{ t('proxy.wafBodylessHint') }}</span>
            </el-form-item>
            <el-form-item :label="t('proxy.wafMultipartStreaming')">
              <el-switch v-model="siteWAFConfig.multipart_streaming_check" active-value="on" inactive-value="off" />
              <span style="font-size:11px;color:#909399;margin-left:8px">{{ t('proxy.wafMultipartStreamingHint') }}</span>
            </el-form-item>
            <el-form-item :label="t('proxy.wafUploadFilenameScan')">
              <el-input v-model="siteWAFConfig.upload_filename_scan_limit" placeholder="0" style="width:120px" />
              <span style="font-size:11px;color:#909399;margin-left:8px">{{ t('proxy.wafUploadFilenameScanHint') }}</span>
            </el-form-item>
            <el-form-item :label="t('proxy.wafPostBodyScan')">
              <el-input v-model="siteWAFConfig.post_body_scan_limit" placeholder="2097152" style="width:120px" />
              <span style="font-size:11px;color:#909399;margin-left:8px">{{ t('proxy.wafPostBodyScanHint') }}</span>
            </el-form-item>
            <el-form-item :label="t('proxy.wafWhiteUrl')"><el-switch v-model="siteWAFConfig.white_url_check" active-value="on" inactive-value="off" /></el-form-item>
            <el-form-item :label="t('proxy.wafWhiteIp')"><el-switch v-model="siteWAFConfig.white_ip_check" active-value="on" inactive-value="off" /></el-form-item>
            <el-form-item :label="t('proxy.wafWhiteUa')">
              <el-switch v-model="siteWAFConfig.white_ua_check" active-value="on" inactive-value="off" />
              <span style="font-size:11px;color:#909399;margin-left:8px">{{ t('proxy.wafWhiteUaHint') }}</span>
            </el-form-item>
            <el-form-item :label="t('proxy.wafBlackIp')"><el-switch v-model="siteWAFConfig.black_ip_check" active-value="on" inactive-value="off" /></el-form-item>
            <el-divider content-position="left">{{ t('proxy.wafCcProtection') }}</el-divider>
            <el-form-item :label="t('proxy.wafCcCheck')"><el-switch v-model="siteWAFConfig.cc_check" active-value="on" inactive-value="off" /></el-form-item>
            <el-form-item :label="t('proxy.wafCcRate')"><el-input v-model="siteWAFConfig.cc_rate" placeholder="60/60" style="width:120px" /></el-form-item>
            <el-form-item :label="t('proxy.wafCcBlockTtl')"><el-input v-model="siteWAFConfig.cc_block_ttl" placeholder="600" style="width:120px" /></el-form-item>
            <el-divider content-position="left">{{ t('proxy.wafOutputGroup') }}</el-divider>
            <el-form-item :label="t('proxy.wafOutput')">
              <el-select v-model="siteWAFConfig.waf_output" style="width:120px">
                <el-option label="html" value="html" /><el-option label="redirect" value="redirect" />
              </el-select>
            </el-form-item>
            <el-form-item :label="t('proxy.wafRedirectUrl')" v-if="siteWAFConfig.waf_output==='redirect'"><el-input v-model="siteWAFConfig.waf_redirect_url" placeholder="https://example.com" /></el-form-item>
          </el-form>
        </el-tab-pane>
        <el-tab-pane label="Rules" name="rules">
          <el-form-item :label="t('proxy.wafRuleSource')" label-width="120px">
            <el-select v-model="siteWAFRuleMode" @change="onSiteWAFRuleModeChange" style="width:180px">
              <el-option :label="t('proxy.wafRuleGlobal')" value="global" />
              <el-option :label="t('proxy.wafRuleCustom')" value="custom" />
            </el-select>
          </el-form-item>
          <el-table v-if="siteWAFRuleMode==='custom'" :data="siteWAFDomainRules" stripe size="small">
            <el-table-column prop="name" :label="t('proxy.wafRule')" width="140" />
            <el-table-column :label="t('proxy.wafRuleDesc')" min-width="180">
              <template #default="{ row }">{{ wafRuleDesc(row.name) }}</template>
            </el-table-column>
            <el-table-column prop="size" :label="t('proxy.wafSize')" width="80" />
            <el-table-column :label="t('common.action')" width="100">
              <template #default="{ row }">
                <el-button size="small" @click="openSiteWAFDomainRule(row)">{{ t('common.edit') }}</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>
      </el-tabs>
      <template #footer>
        <el-button @click="siteWAFVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="success" @click="saveSiteWAFConfig">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- Site WAF Rule Editor -->
    <el-dialog v-model="siteWAFRuleEditorVisible" :title="`${siteWAFDomain} / ${siteWAFRuleEditing}`" width="70%">
      <el-input v-model="siteWAFRuleEditingContent" type="textarea" :rows="18" style="font-family:monospace;font-size:13px" />
      <template #footer>
        <el-button @click="siteWAFRuleEditorVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="success" @click="saveSiteWAFDomainRule">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- Config View Dialog -->
    <el-dialog v-model="configViewVisible" title="Caddyfile" width="700">
      <el-input v-model="configViewContent" type="textarea" :rows="20" readonly style="font-family:monospace;font-size:13px" />
      <template #footer>
        <el-button @click="copyConfig">{{ t('proxy.copyConfig') }}</el-button>
        <el-button @click="configViewVisible=false">{{ t('common.close') }}</el-button>
      </template>
    </el-dialog>

<!-- SNI Dialog -->
<el-dialog v-model="sniDialogVisible" :title="t('proxy.addSNIRule')" width="560">
<el-form :model="sniForm" label-width="120px">
<el-form-item :label="t('proxy.associateInbound')">
<el-select v-model="sniForm.inbound_id" filterable clearable :placeholder="t('proxy.selectInbound')" style="width:100%" @change="onSNIInboundSelect">
<el-option v-for="ib in sniInbounds" :key="ib.id" :label="inboundLabel(ib)" :value="ib.id" />
</el-select>
</el-form-item>
<el-form-item label="Domain">
<el-input v-model="sniForm.domain" type="textarea" :rows="2" placeholder="www.example.com, api.example.com" />
<div style="font-size:12px;color:#909399;margin-top:4px">{{ t('proxy.sniDomainHint') }}</div>
</el-form-item>
<el-form-item :label="t('proxy.sniTarget')">
<el-input v-model="sniForm.sni_target" placeholder="127.0.0.1:8443" />
</el-form-item>
<el-form-item :label="t('proxy.proxyProtocol')">
<el-switch v-model="sniForm.proxy_protocol" />
</el-form-item>
</el-form>
      <template #footer>
        <el-button @click="sniDialogVisible=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveSNIRule">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- Site Dialog -->
    <el-dialog v-model="siteDialogVisible" :title="siteDialogTitle" width="750px" top="5vh">
      <div style="margin-bottom:12px">
        <el-radio-group v-model="siteForm.editMode" size="small">
          <el-radio-button value="form">{{ t('proxy.formEdit') }}</el-radio-button>
          <el-radio-button value="text">{{ t('proxy.textEdit') }}</el-radio-button>
        </el-radio-group>
      </div>
      <el-form v-if="siteForm.editMode==='form'" :model="siteForm" label-width="120px">
        <el-form-item label="Domain">
          <el-input v-model="siteForm.domain" placeholder="example.com" />
        </el-form-item>
        <el-form-item label="Upstream" v-if="siteForm.template!=='static' && siteForm.template!=='portal_forward' && siteForm.template!=='subscription_forward' && siteForm.template!=='reality'">
          <div v-for="(up, idx) in siteForm.upstreams" :key="idx" style="display:flex;gap:8px;margin-bottom:6px;align-items:center">
            <el-input v-model="siteForm.upstreams[idx]" placeholder="127.0.0.1:8080" style="flex:1" />
            <el-button v-if="siteForm.upstreams.length > 1" size="small" type="danger" plain @click="siteForm.upstreams.splice(idx, 1)">-</el-button>
          </div>
          <el-button size="small" @click="siteForm.upstreams.push('')">+ {{ t('common.add') }}</el-button>
        </el-form-item>
        <el-form-item v-if="siteForm.template==='portal_forward'" :label="t('proxy.portalForwardHint')">
          <span style="font-size:12px;color:#909399">{{ t('proxy.portalForwardDesc') }}</span>
        </el-form-item>
        <el-form-item v-if="siteForm.template==='subscription_forward'" :label="t('proxy.subscriptionForwardHint')">
          <span style="font-size:12px;color:#909399">{{ t('proxy.subscriptionForwardDesc') }}</span>
        </el-form-item>
        <el-form-item v-if="siteForm.template==='reality'" :label="t('proxy.templateReality')">
          <span style="font-size:12px;color:#909399">{{ t('proxy.realityDesc') }}</span>
        </el-form-item>
        <el-form-item label="Root Directory" v-if="siteForm.template==='static'">
          <div v-for="(up, idx) in siteForm.upstreams" :key="idx" style="display:flex;gap:8px;margin-bottom:6px;align-items:center">
            <el-input v-model="siteForm.upstreams[idx]" placeholder="/var/www/mysite" style="flex:1" />
            <el-button v-if="siteForm.upstreams.length > 1" size="small" type="danger" plain @click="siteForm.upstreams.splice(idx, 1)">-</el-button>
          </div>
          <el-button size="small" @click="siteForm.upstreams.push('')">+ {{ t('common.add') }}</el-button>
        </el-form-item>
        <el-form-item v-if="showInboundSelect" :label="t('proxy.associateInbound')">
          <el-select v-model="siteForm.inbound_id" filterable clearable :placeholder="t('proxy.selectInbound')" style="width:100%" @change="onInboundSelect">
            <el-option v-for="ib in filteredInbounds" :key="ib.id" :label="inboundLabel(ib)" :value="ib.id" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="siteForm.template==='l4'" label="Port">
          <el-input v-model="siteForm.path" placeholder="443" />
        </el-form-item>
        <el-form-item v-else label="Path">
          <el-input v-model="siteForm.path" :placeholder="t('proxy.pathHint')" />
        </el-form-item>
        <el-form-item v-if="siteForm.template==='sni'" :label="t('proxy.sniTarget')">
          <el-input v-model="siteForm.sni_target" placeholder="127.0.0.1:443" />
        </el-form-item>
        <el-form-item v-if="siteForm.template==='l4'" label="Protocol">
          <el-select v-model="siteForm.l4_protocol" style="width:100%">
            <el-option label="TCP" value="tcp" />
            <el-option label="UDP" value="udp" />
          </el-select>
        </el-form-item>
<el-form-item v-if="siteForm.template!=='sni' && siteForm.template!=='l4' && siteForm.template!=='static' && siteForm.template!=='portal_forward' && siteForm.template!=='subscription_forward' && siteForm.template!=='reality'" :label="t('proxy.template')">
            <el-select v-model="siteForm.template" style="width:100%">
            <el-option label="HTTP" value="http" />
            <el-option label="WebSocket" value="websocket" />
            <el-option label="gRPC" value="grpc" />
            <el-option label="XHTTP" value="xhttp" />
            <el-option label="HTTPUpgrade" value="httpupgrade" />
            <el-option v-if="isLocal" :label="t('proxy.templatePortalForward')" value="portal_forward" />
            <el-option v-if="isLocal" :label="t('proxy.templateSubscriptionForward')" value="subscription_forward" />
            <el-option :label="t('proxy.templateReality')" value="reality" />
            </el-select>
        </el-form-item>
        <el-form-item v-if="siteForm.template==='static' || siteForm.template==='portal_forward' || siteForm.template==='subscription_forward' || siteForm.template==='reality'" :label="t('proxy.template')">
            <el-select v-model="siteForm.template" style="width:100%">
            <el-option label="HTTP" value="http" />
            <el-option label="WebSocket" value="websocket" />
            <el-option label="gRPC" value="grpc" />
            <el-option label="XHTTP" value="xhttp" />
            <el-option label="HTTPUpgrade" value="httpupgrade" />
            <el-option label="Static" value="static" />
            <el-option v-if="isLocal" :label="t('proxy.templatePortalForward')" value="portal_forward" />
            <el-option v-if="isLocal" :label="t('proxy.templateSubscriptionForward')" value="subscription_forward" />
            <el-option :label="t('proxy.templateReality')" value="reality" />
            </el-select>
        </el-form-item>
        <el-form-item v-if="siteForm.template!=='sni' && siteForm.template!=='l4' && siteForm.template!=='static' && siteForm.template!=='reality'" label="lb_policy">
          <el-select v-model="siteForm.lb_policy" style="width:100%" clearable :placeholder="t('proxy.lbPolicyHint')">
            <el-option :label="t('proxy.lbRoundRobin')" value="round_robin" />
            <el-option :label="t('proxy.lbLeastConn')" value="least_conn" />
            <el-option :label="t('proxy.lbIpHash')" value="ip_hash" />
            <el-option :label="t('proxy.lbFirst')" value="first" />
            <el-option :label="t('proxy.lbRandom')" value="random" />
            <el-option :label="t('proxy.lbUri')" value="uri" />
            <el-option label="cookie (Cookie)" value="cookie" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('proxy.proxyProtocol')">
          <el-switch v-model="siteForm.proxy_protocol" :disabled="siteForm.template==='reality'" />
          <span v-if="siteForm.template==='reality'" style="font-size:12px;color:#909399;margin-left:8px">{{ t('common.required') }}</span>
        </el-form-item>
        <el-form-item :label="t('proxy.useCertificate')">
          <el-switch v-model="siteForm.use_cert" :disabled="siteForm.template==='reality'" @change="onToggleCert" />
          <span v-if="siteForm.template==='reality'" style="font-size:12px;color:#909399;margin-left:8px">{{ t('common.required') }}</span>
        </el-form-item>
        <template v-if="siteForm.use_cert">
          <el-divider content-position="left">{{ t('proxy.certSettings') }}</el-divider>
          <el-form-item :label="t('proxy.selectCert')">
            <el-select v-model="siteForm.cert_id" style="width:100%" clearable @change="onSiteCertSelect" :placeholder="t('proxy.selectCert')">
              <el-option v-for="c in certificates" :key="c.id" :label="`${c.domain} (${c.issuer})`" :value="c.id" />
            </el-select>
          </el-form-item>
          <el-form-item :label="t('proxy.certPath')">
            <el-input v-model="siteForm.cert_path" placeholder="/path/to/cert.pem" @change="emit" />
          </el-form-item>
          <el-form-item :label="t('proxy.keyPath')">
            <el-input v-model="siteForm.key_path" placeholder="/path/to/key.pem" @change="emit" />
          </el-form-item>
        </template>
        <el-divider content-position="left">{{ t('proxy.logSettings') }}</el-divider>
        <el-form-item :label="t('proxy.enableLog')">
          <el-switch v-model="siteForm.log_enabled" />
        </el-form-item>
        <el-form-item v-if="siteForm.log_enabled" :label="t('proxy.logPath')">
          <el-input v-model="siteForm.log_path" placeholder="logs/example.com.log" />
        </el-form-item>
        <el-form-item v-if="siteForm.log_enabled" :label="t('proxy.logFormat')">
          <el-select v-model="siteForm.log_format" style="width:100%">
            <el-option label="json" value="json" />
            <el-option label="console" value="console" />
          </el-select>
        </el-form-item>
        <el-divider content-position="left">WAF</el-divider>
        <el-form-item label="WAF">
          <el-switch v-model="siteForm.waf_enabled" />
          <div style="font-size:11px;color:#909399;margin-top:4px">关闭后该路径的请求将跳过 WAF 检测（同域名不同路径站点合并，按路径级开关控制）</div>
        </el-form-item>
      </el-form>
      <div v-else>
        <div style="font-size:12px;color:#909399;margin-bottom:8px">{{ t('proxy.configPreview') }}</div>
        <el-input :model-value="generatePreview(siteForm)" type="textarea" :rows="10" readonly style="font-family:monospace;font-size:13px;background:#f5f7fa" />
        <div style="margin-top:12px">
          <div style="font-size:12px;color:#909399;margin-bottom:4px">{{ t('proxy.proxyConfig') }}</div>
          <el-input v-model="siteForm.proxy_config" type="textarea" :rows="3" style="font-family:monospace;font-size:13px" placeholder="lb_policy least_conn&#10;health_uri /health&#10;health_interval 10s" />
        </div>
        <div style="margin-top:12px">
          <div style="font-size:12px;color:#909399;margin-bottom:4px">{{ t('proxy.customConfig') }}</div>
          <el-input v-model="siteForm.custom_config" type="textarea" :rows="3" style="font-family:monospace;font-size:13px" placeholder="encode gzip" />
        </div>
      </div>
      <template #footer>
        <el-button @click="siteDialogVisible=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="siteSaving" @click="saveSite">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- Version Dialog -->
    <el-dialog v-model="releasesVisible" :title="t('proxy.selectVersion')" width="600" :close-on-click-modal="!downloading">
      <div v-if="downloading" style="text-align:center;padding:40px 20px">
        <el-icon class="is-loading" :size="32" color="#409eff"><Loading /></el-icon>
        <div style="margin-top:16px;font-size:14px;color:#606266">{{ downloadProgress }}</div>
      </div>
      <div v-else v-loading="releasesLoading" style="min-height:100px">
        <div v-if="releases.length === 0 && !releasesLoading" style="text-align:center;color:#999;padding:20px">{{ t('common.noData') }}</div>
        <div v-for="r in releases" :key="r.tag_name" style="margin-bottom:16px">
          <div style="display:flex;align-items:center;gap:8px;margin-bottom:8px">
            <el-tag :type="r.prerelease?'warning':'success'" size="small">{{ r.prerelease ? 'Pre' : 'Rel' }}</el-tag>
            <span style="font-weight:600">{{ r.tag_name }}</span>
          </div>
          <div style="display:flex;flex-wrap:wrap;gap:8px">
            <el-button v-for="a in r.assets" :key="a.name" size="small" :loading="downloadingAsset===a.name" @click="downloadVersion(a.browser_download_url, r.tag_name)">
              {{ a.name }} <span style="color:#999;font-size:11px;margin-left:4px">({{ formatSize(a.size) }})</span>
            </el-button>
          </div>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Loading } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'
import client from '../api/client'
import { useClientPagination } from '../composables/useClientPagination'
import PaginationBar from '../components/PaginationBar.vue'

const { t } = useI18n()
const emit = defineEmits(['back'])

const props = defineProps({
  serverId: { type: Number, required: true },
  isLocal: { type: Boolean, default: false }
})

// API base paths (node-server-scoped)
const apiBase = computed(() => `/node-servers/${props.serverId}`)
const caddyApi = computed(() => `${apiBase.value}/caddy`)
const proxyApi = computed(() => `${apiBase.value}/proxy`)

const proxyInfo = ref({})
const guardian = ref({})
const nginxRunning = ref(false)
const inbounds = ref([])
const guardianConfig = ref({ autoRestart: true, maxRestarts: 5 })
const allSites = ref([])
const certificates = ref([])
const caddyLogMode = ref('file')
const logModeSaving = ref(false)
const wafMasterEnabled = ref(false)
const wafStatusSaving = ref(false)
const caddyLogModeHint = computed(() => {
  const m = caddyLogMode.value
  if (m === 'stdout') return t('proxy.caddyHintStdout')
  if (m === 'off') return t('proxy.caddyHintLogOff')
  return t('proxy.caddyHintFile')
})

const httpSites = computed(() => allSites.value.filter(s => s.template !== 'sni' && s.template !== 'l4'))
const l4Sites = computed(() => allSites.value.filter(s => s.template === 'l4'))
const sniRules = computed(() => allSites.value.filter(s => s.template === 'sni'))

const { paged: sniPagedList, page: sniPage, pageSize: sniSize, total: sniTotal } = useClientPagination(sniRules, { pageKey: 'sniPage', pageSizeKey: 'sniSize' })
const { paged: l4PagedList, page: l4Page, pageSize: l4Size, total: l4Total } = useClientPagination(l4Sites, { pageKey: 'l4Page', pageSizeKey: 'l4Size' })
const { paged: httpPagedList, page: httpPage, pageSize: httpSize, total: httpTotal } = useClientPagination(httpSites, { pageKey: 'httpPage', pageSizeKey: 'httpSize' })

// Inbound association: filter by template type
const showInboundSelect = computed(() => {
  const tpl = siteForm.value.template
  return ['websocket', 'grpc', 'xhttp', 'httpupgrade', 'sni', 'http'].includes(tpl)
})
const filteredInbounds = computed(() => {
  const tpl = siteForm.value.template
  return inbounds.value.filter(ib => {
    const network = ib._network
    const security = ib._security
    // Exclude UDP-based protocols for all templates
    if (network === 'hysteria' || network === 'mkcp' || network === 'kcp') return false
    // SNI: only TLS and Reality (TCP-based TLS routing)
    if (tpl === 'sni') return security === 'tls' || security === 'reality'
    // All HTTP-type templates: exclude Reality (does its own TLS, not HTTP-forwardable)
    if (security === 'reality') return false
    if (tpl === 'websocket') return network === 'ws'
    if (tpl === 'grpc') return network === 'grpc'
    if (tpl === 'xhttp') return network === 'xhttp'
    if (tpl === 'httpupgrade') return network === 'httpupgrade'
    if (tpl === 'http') {
      // Exclude raw (no HTTP path to proxy)
      if (network === 'raw') return false
      return true
    }
    return false
  })
})
function inboundLabel(ib) {
  const parts = [ib._remark || ib.tag || ib.protocol]
  if (ib._isSocket) parts.push(`socket:${ib._socketPath}`)
  else parts.push(`${ib.listen}:${ib.port}`)
  if (ib._network && ib._network !== 'raw' && ib._network !== 'tcp') parts.push(ib._network)
  if (ib._security && ib._security !== 'none') parts.push(ib._security)
  return parts.join(' | ')
}
function parseInboundConfig(item) {
  let cfg = {}
  try { cfg = typeof item.config_json === 'string' ? JSON.parse(item.config_json) : (item.config_json || {}) } catch {}
  const ss = cfg.streamSettings || {}
  const listen = cfg.listen || item.listen
  const isSocket = listen.startsWith('/')
  const socketPath = isSocket ? listen.split(',')[0] : ''
  let wsPath = '', grpcService = '', xhttpPath = '', httpupgradePath = ''
  if (ss.wsSettings) wsPath = ss.wsSettings.path
  if (ss.grpcSettings) grpcService = ss.grpcSettings.serviceName
  if (ss.xhttpSettings || ss.splithttpSettings) xhttpPath = (ss.xhttpSettings || ss.splithttpSettings).path
  if (ss.httpupgradeSettings) httpupgradePath = ss.httpupgradeSettings.path
  // Extract server names from TLS / Reality settings
  let serverNames = []
  if (ss.security === 'tls' && ss.tlsSettings) {
    const sn = ss.tlsSettings.serverName
    if (sn) serverNames.push(sn)
  }
  if (ss.security === 'reality' && ss.realitySettings) {
    const names = ss.realitySettings.serverNames || []
    serverNames = serverNames.concat(names)
  }
  return {
    id: item.id,
    tag: item.tag,
    protocol: item.protocol,
    remark: item.remark,
    listen: listen,
    port: cfg.port || item.port || 0,
    _network: ss.network,
    _security: ss.security,
    _isSocket: isSocket,
    _socketPath: socketPath,
    _wsPath: wsPath,
    _grpcService: grpcService,
    _xhttpPath: xhttpPath,
    _httpupgradePath: httpupgradePath,
    _serverNames: serverNames,
    _remark: item.remark || item.tag,
  }
}
async function fetchInbounds() {
  try {
    const data = await client.get(`${apiBase.value}/inbound-configs`, { params: { core_type: 'xray' } })
    inbounds.value = (data || []).map(parseInboundConfig)
  } catch { inbounds.value = [] }
}
function onInboundSelect(id) {
  if (!id) return
  const ib = inbounds.value.find(i => i.id === id)
  if (!ib) return
  // Auto-fill upstream
  const upstream = ib._isSocket ? `unix/${ib._socketPath}` : `127.0.0.1:${ib.port}`
  siteForm.value.upstreams = [upstream]
  // Auto-fill path (location) and backend_path (what the backend expects)
  const tpl = siteForm.value.template
  const stripQuery = (p) => (p).split('?')[0]
  let backendPath = '/'
  if (tpl === 'websocket' && ib._wsPath) {
    backendPath = stripQuery(ib._wsPath)
    siteForm.value.path = backendPath
  } else if (tpl === 'grpc' && ib._grpcService) {
    backendPath = ib._grpcService
    siteForm.value.path = backendPath
  } else if (tpl === 'xhttp' && ib._xhttpPath) {
    backendPath = stripQuery(ib._xhttpPath)
    siteForm.value.path = backendPath
  } else if (tpl === 'httpupgrade' && ib._httpupgradePath) {
    backendPath = stripQuery(ib._httpupgradePath)
    siteForm.value.path = backendPath
  } else {
    siteForm.value.path = siteForm.value.path
  }
  siteForm.value.backend_path = backendPath
  // For SNI: fill sni_target
  if (tpl === 'sni') {
    siteForm.value.sni_target = upstream
  }
}

// Releases
const releasesVisible = ref(false)
const releasesLoading = ref(false)
const releases = ref([])
const downloadingAsset = ref('')
const downloading = ref(false)
const downloadProgress = ref('')

// Site dialog
const siteDialogVisible = ref(false)
const siteDialogTitle = ref('')
const siteSaving = ref(false)
const siteForm = ref({ domain: '', upstream: '', path: '', backend_path: '', template: 'http', use_cert: false, cert_id: null, cert_path: '', key_path: '', custom_config: '', waf_enabled: true, l4_protocol: 'tcp', sni_target: '', proxy_protocol: false, is_panel: false, editMode: 'form' })

// Panel port
const panelPort = ref('8080')

// Config view
const configViewVisible = ref(false)
const configViewContent = ref('')

// SNI
const sniSettings = ref({ enabled: false, listen_port: '443', local_port: '8443', proxy_protocol_rules: ['127.0.0.0/8', '::1'] })
const sniDialogVisible = ref(false)
const sniForm = ref({ domain: '', sni_target: '', proxy_protocol: false, inbound_id: null })

// SNI inbounds: only TLS and Reality, exclude UDP-based protocols (hysteria, mkcp)
const sniInbounds = computed(() => {
  return inbounds.value.filter(ib => {
    const sec = ib._security
    const net = ib._network
    if (net === 'hysteria' || net === 'mkcp' || net === 'kcp') return false
    return sec === 'tls' || sec === 'reality'
  })
})

function onSNIInboundSelect(id) {
  if (!id) return
  const ib = inbounds.value.find(i => i.id === id)
  if (!ib) return
  // Auto-fill sni_target from inbound listen:port or socket
  const upstream = ib._isSocket ? `unix/${ib._socketPath}` : `127.0.0.1:${ib.port}`
  sniForm.value.sni_target = upstream
  // Auto-fill domain from server names (TLS serverName / Reality serverNames)
  if (ib._serverNames && ib._serverNames.length > 0) {
    sniForm.value.domain = ib._serverNames.join(', ')
  }
}

function formatSize(bytes) {
  if (!bytes) return ''
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1048576) return (bytes / 1024).toFixed(0) + ' KB'
  return (bytes / 1048576).toFixed(1) + ' MB'
}

async function fetchProxyInfo() {
  try {
    const status = await client.get(`${proxyApi.value}/status`)
    proxyInfo.value = status.caddy || {}
    guardian.value = status.caddy_guardian || {}
    guardianConfig.value.autoRestart = guardian.value.auto_restart !== false
    guardianConfig.value.maxRestarts = guardian.value.max_restarts || 5
    if (status.panel_port) panelPort.value = status.panel_port
    nginxRunning.value = (status.nginx && status.nginx.status === 'running')
  } catch (e) { ElMessage.error(t('common.failed')) }
}

async function saveGuardianConfig() {
  try {
    await client.post(`${proxyApi.value}/guardian?type=caddy`, {
      auto_restart: guardianConfig.value.autoRestart,
      max_restarts: guardianConfig.value.maxRestarts
    })
    fetchProxyInfo()
  } catch (e) { ElMessage.error(t('common.failed')) }
}

function formatTime(ts) {
  if (!ts || ts === '0001-01-01T00:00:00Z') return '-'
  return new Date(ts).toLocaleString()
}

async function fetchSites() {
try {
allSites.value = await client.get(`${caddyApi.value}/sites`) || []
// Check WAF status for each site
for (const site of allSites.value) {
if (site.domain && site.template !== 'sni' && site.template !== 'l4') {
try {
const res = await client.get(`${apiBase.value}/waf/domain-status`, { params: { domain: site.domain } })
site._waf_enabled = res.enabled
} catch { site._waf_enabled = false }
} else {
site._waf_enabled = false
}
}
} catch (e) { allSites.value = [] }
// Load log output mode (per-node)
try {
const cfg = await client.get('/log-manage/log-output', { params: { server_id: props.serverId } })
if (cfg?.caddy?.mode) caddyLogMode.value = cfg.caddy.mode
} catch (e) { ElMessage.error(t('common.failed')) }
}

async function toggleSiteWAF(row, val) {
if (val) {
try {
await client.post(`${apiBase.value}/waf/enable-domain`, { domain: row.domain })
ElMessage.success(t('proxy.wafSiteEnabled', { domain: row.domain }))
} catch (e) {
row._waf_enabled = false
ElMessage.error(t('common.failed'))
}
} else {
try {
const action = await ElMessageBox.confirm(
t('proxy.wafDisableHint', { domain: row.domain }),
t('proxy.wafDisableTitle'),
{
distinguishCancelAndClose: true,
confirmButtonText: t('proxy.wafDisableSite'),
cancelButtonText: t('proxy.wafUseGlobal'),
type: 'warning',
}
)
// confirmButtonText → disable WAF for this domain
try {
await client.post(`${apiBase.value}/waf/disable-domain`, { domain: row.domain })
ElMessage.success(t('proxy.wafSiteDisabled', { domain: row.domain }))
} catch (e) {
row._waf_enabled = true
ElMessage.error(t('common.failed'))
}
} catch (action) {
if (action === 'cancel') {
// Use global rules — remove domain from domain.json
try {
await client.post(`${apiBase.value}/waf/disable-domain`, { domain: row.domain })
ElMessage.success(t('proxy.wafUseGlobalRules', { domain: row.domain }))
} catch (e) {
row._waf_enabled = true
ElMessage.error(t('common.failed'))
}
} else {
row._waf_enabled = true
}
}
}
}

async function saveCaddyLogMode() {
  logModeSaving.value = true
  try {
    // Per-node: read and save with server_id
    const current = await client.get('/log-manage/log-output', { params: { server_id: props.serverId } })
    await client.put('/log-manage/log-output', {
      nginx: current?.nginx || { mode: 'file' },
      caddy: { mode: caddyLogMode.value },
    }, { params: { server_id: props.serverId } })

    // Regenerate config — the config generator respects the log output mode:
    // "off" → no log blocks regardless of site.log_enabled
    // "file"/"stdout" → log blocks only for sites with log_enabled=true
    await applyConfig()
    
    ElMessage.success(t('common.success'))
  } finally {
    logModeSaving.value = false
  }
}

function onCaddyLogModeChange(mode) {
  caddyLogMode.value = mode
}

async function fetchCertificates() {
  try { certificates.value = (await client.get('/certificates', { params: { limit: 100 } })).items || [] } catch (e) { ElMessage.error(t('common.failed')) }
}

async function fetchSNISettings() {
  try {
    const data = await client.get(`${caddyApi.value}/sni-settings`)
    if (data) sniSettings.value = data
  } catch (e) { ElMessage.error(t('common.failed')) }
}

async function saveSNISettings() {
  try {
    await client.post(`${caddyApi.value}/sni-settings`, sniSettings.value)
    ElMessage.success(t('common.success'))
  } catch (e) { ElMessage.error(t('common.failed')) }
}

async function onSNIToggle(enabled) {
  try {
    sniSettings.value.enabled = enabled
    await client.post(`${caddyApi.value}/sni-settings`, sniSettings.value)
    if (enabled) {
      // Re-enable preserved SNI rules
      await client.post(`${caddyApi.value}/sni-rules/toggle`, { enabled: true })
      // Create fallback rule if none exist
      const hasFallback = sniRules.value.some(s => s.domain === 'fallback' || s.domain === '')
      if (!hasFallback) {
        const localPort = sniSettings.value.local_port
        await client.post(`${caddyApi.value}/sites`, {
          domain: 'fallback', upstream: '', template: 'sni',
          sni_target: `127.0.0.1:${localPort}`, local_port: localPort,
          proxy_protocol: true, is_panel: false, priority: 999,
          path: sniSettings.value.listen_port
        })
      }
      // Batch migrate HTTPS sites
      const port = sniSettings.value.local_port
      const res = await client.post(`${caddyApi.value}/batch-update-ports`, { port, enable: true })
      ElMessage.success(t('proxy.sniEnabled', { count: res.updated || 0 }))
    } else {
      // Disable SNI rules (keep in DB)
      await client.post(`${caddyApi.value}/sni-rules/toggle`, { enabled: false })
      // Restore sites to port 443
      await client.post(`${caddyApi.value}/batch-update-ports`, { port: '443', enable: false })
      ElMessage.success(t('proxy.sniDisabled'))
    }
    await applyConfig()
    fetchSites()
  } catch (e) { ElMessage.error(t('common.failed')) }
}

async function batchMigrate() {
  const port = sniSettings.value.local_port
  try {
    await client.post(`${caddyApi.value}/sni-settings`, sniSettings.value)
    if (sniRules.value.length === 0) {
      await client.post(`${caddyApi.value}/sites`, {
        domain: 'fallback', upstream: '', template: 'sni',
        sni_target: `127.0.0.1:${port}`, local_port: port,
        proxy_protocol: true, is_panel: false, priority: 999,
        path: sniSettings.value.listen_port
      })
    }
    const res = await client.post(`${caddyApi.value}/batch-update-ports`, { port, enable: true })
    await applyConfig()
    ElMessage.success(t('proxy.migrateDone', { count: res.updated || 0 }))
    fetchSites()
  } catch (e) { ElMessage.error(t('common.failed')) }
}

async function showReleases() {
  releasesVisible.value = true
  releasesLoading.value = true
  try {
    releases.value = await client.get(`${proxyApi.value}/releases`, { params: { type: 'caddy' } }) || []
  } catch (e) { releases.value = [] } finally { releasesLoading.value = false }
}

async function downloadVersion(url, tagName) {
  downloading.value = true
  downloadProgress.value = t('proxy.downloading', { name: tagName })
  try {
    await client.post(`${proxyApi.value}/download`, { type: 'caddy', download_url: url, tag_name: tagName })
    try { await client.post(`${proxyApi.value}/start?type=caddy`) } catch {}
    ElMessage.success(t('proxy.downloadSuccess'))
    releasesVisible.value = false
    fetchProxyInfo()
  } catch (e) {
    ElMessage.error(t('error.networkError'))
  } finally {
    downloading.value = false
    downloadingAsset.value = ''
  }
}

async function proxyAction(action) {
  try {
    await client.post(`${proxyApi.value}/${action}?type=caddy`)
    ElMessage.success(t('common.success'))
    fetchProxyInfo()
  } catch (e) { ElMessage.error(t('common.failed')) }
}

function showAddSite(template) {
  siteDialogTitle.value = t('proxy.addSite')
  const isReality = template === 'reality'
siteForm.value = {
domain: isReality ? ':9443' : '', upstreams: isReality ? [''] : [''], path: '', backend_path: '', template,
    use_cert: isReality, cert_id: null, cert_path: '', key_path: '',
proxy_config: '', custom_config: '', waf_enabled: !isReality, lb_policy: '',
log_enabled: false, log_path: '', log_format: 'json',
l4_protocol: 'tcp', sni_target: '',
proxy_protocol: isReality, is_panel: false, editMode: 'form',
inbound_id: null
}
  siteDialogVisible.value = true
}

function showPanelProxy() {
  siteDialogTitle.value = t('proxy.proxyPanel')
  siteForm.value = {
    domain: ':80', upstreams: [`127.0.0.1:${panelPort.value}`], path: '', template: 'http',
    use_cert: false, cert_id: null, cert_path: '', key_path: '',
proxy_config: '', custom_config: '', waf_enabled: true, lb_policy: '',
log_enabled: false, log_path: '', log_format: 'json',
l4_protocol: 'tcp', sni_target: '',
proxy_protocol: false, is_panel: true, editMode: 'form',
inbound_id: null
}
  siteDialogVisible.value = true
}

function onSiteCertSelect(id) {
  const cert = certificates.value.find(c => c.id === id)
  if (cert) {
    siteForm.value.cert_path = cert.cert_path
    siteForm.value.key_path = cert.key_path
  }
}

function onToggleCert(enabled) {
  const d = siteForm.value.domain
  const sniPort = sniSettings.value.enabled ? (sniSettings.value.local_port) : '443'
  const httpPort = ':80'
  if (enabled) {
    if (d.startsWith(':')) {
      siteForm.value.domain = ':' + sniPort
    } else if (!d.includes(':') && d !== '') {
      siteForm.value.domain = d + ':' + sniPort
    }
  } else {
    if (d === ':' + sniPort || d === ':443') {
      siteForm.value.domain = httpPort
    } else if (d.endsWith(':' + sniPort)) {
      siteForm.value.domain = d.replace(':' + sniPort, '')
    } else if (d.endsWith(':443')) {
      siteForm.value.domain = d.replace(':443', '')
    }
  }
}

function editSite(row) {
  siteDialogTitle.value = t('common.edit')
  let certId = null
  if (row.cert_path) {
    const match = certificates.value.find(c => c.cert_path === row.cert_path)
    if (match) certId = match.id
  }
  // Parse upstream into array
  let upstreams = ['']
  if (row.upstream) {
    upstreams = row.upstream.split(/[,\n]/).map(s => s.trim()).filter(s => s)
    if (upstreams.length === 0) upstreams = ['']
  }
siteForm.value = {
id: row.id, domain: row.domain, upstreams: upstreams,
path: row.path, backend_path: row.backend_path,
template: row.template,
    use_cert: !!row.cert_path, cert_id: certId,
    cert_path: row.cert_path, key_path: row.key_path,
    proxy_config: row.proxy_config,
    custom_config: row.custom_config,
waf_enabled: row.waf_enabled !== false,
    lb_policy: row.lb_policy,
    log_enabled: row.log_enabled || false,
    log_path: row.log_path,
    log_format: row.log_format,
    l4_protocol: row.l4_protocol,
    sni_target: row.sni_target,
    proxy_protocol: row.proxy_protocol || false,
    is_panel: row.is_panel, editMode: 'form',
    inbound_id: null
  }
  siteDialogVisible.value = true
}

function generatePreview(site) {
  let lines = []
  const domain = site.domain
  const pp = site.proxy_protocol ? 'proxy_protocol' : ''
  if (site.template === 'l4') {
    const port = site.path
    const proto = site.l4_protocol
    const upstream = site.upstream || `127.0.0.1:${panelPort.value}`
    lines.push(`# L4 ${proto.toUpperCase()} forward on :${port}`)
    lines.push(`{`, `\tlayer4 {`, `\t\t:${port} {`, `\t\t\troute {`, `\t\t\t\tproxy ${upstream}`)
    if (pp) lines.push(`\t\t\t\t${pp}`)
    lines.push(`\t\t\t}`, `\t\t}`, `\t}`, `}`)
    return lines.join('\n')
  }
  if (site.template === 'reality') {
    const port = (site.domain || ':9443').replace(/^:/, '') || '9443'
    const certPath = site.cert_path || '/path/to/cert.pem'
    const keyPath = site.key_path || '/path/to/key.pem'
    lines.push(`# Reality on :${port}`)
    lines.push(`{`)
    lines.push(`\tservers 127.0.0.1:${port} {`)
    lines.push(`\t\tlistener_wrappers {`)
    lines.push(`\t\t\tproxy_protocol {`)
    lines.push(`\t\t\t\tallow 127.0.0.1/32`)
    lines.push(`\t\t\t}`)
    lines.push(`\t\t\ttls`)
    lines.push(`\t\t}`)
    lines.push(`\t\tprotocols h1 h2`)
    lines.push(`\t}`)
    lines.push(`}`)
    lines.push(``)
    lines.push(`:${port} {`)
    lines.push(`\tbind 127.0.0.1`)
    lines.push(`\ttls ${certPath} ${keyPath}`)
    lines.push(`}`)
    return lines.join('\n')
  }
  lines.push(`${domain} {`)
  if (site.cert_path && site.key_path) lines.push(`\ttls ${site.cert_path} ${site.key_path}`)
  // WAF bypass for this site
  const wafBypass = site.waf_enabled === false
  if (site.template === 'static') {
    const rootDir = (site.upstreams || []).filter(u => u.trim())[0]
    lines.push(`\troot * ${rootDir}`)
    lines.push(`\tfile_server`)
  } else {
    const upstreams = (site.upstreams || []).filter(u => u.trim())
    if (upstreams.length > 0) {
      if (wafBypass) {
        lines.push(`\troute {`)
        lines.push(`\t\tcaddyguard waf_enable off`)
      }
      lines.push(`\t${wafBypass ? '  ' : ''}reverse_proxy {`)
      for (const up of upstreams) { lines.push(`\t\tto ${up}`) }
      if (site.lb_policy) lines.push(`\t\tlb_policy ${site.lb_policy}`)
      if (site.template === 'websocket') {
        lines.push(`\t\theader_up Upgrade {>Upgrade}`, `\t\theader_up Connection {>Connection}`)
        lines.push(`\t\theader_up Host {hostport}`)
      }
      lines.push(`\t\theader_up X-Real-IP {remote_host}`, `\t\theader_up X-Forwarded-For {remote_host}`)
      lines.push(`\t\theader_up X-Forwarded-Proto {scheme}`)
      if (pp) lines.push(`\t\t${pp}`)
      if (site.proxy_config) {
        for (const line of site.proxy_config.split('\n')) {
          const trimmed = line.trim()
          if (trimmed) lines.push(`\t\t${trimmed}`)
        }
      }
      lines.push(`\t}`)
      if (wafBypass) {
        lines.push(`\t}`)
      }
    }
  }
  if (site.custom_config) {
    for (const line of site.custom_config.split('\n')) {
      const trimmed = line.trim()
      if (trimmed) lines.push(`\t${trimmed}`)
    }
  }
  lines.push(`}`)
  return lines.join('\n')
}

async function applyConfig() {
  try {
    await client.post(`${caddyApi.value}/generate-config`)
    if (proxyInfo.value.status === 'running') {
      await client.post(`${proxyApi.value}/reload?type=caddy`)
    }
    fetchProxyInfo()
  } catch (e) { ElMessage.error(t('common.failed')) }
}

async function saveSite() {
  siteSaving.value = true
  try {
    let domain = siteForm.value.domain
    // Auto-append local port for new HTTPS sites when SNI is active
    if (!siteForm.value.id && sniSettings.value.enabled && siteForm.value.use_cert && siteForm.value.template !== 'l4' && siteForm.value.template !== 'sni') {
      const localPort = sniSettings.value.local_port
      if (domain && !domain.includes(':')) {
        domain = domain + ':' + localPort
      }
    }
    // Check for duplicate path on the same domain (skip SNI and L4 templates)
    if (siteForm.value.template !== 'sni' && siteForm.value.template !== 'l4') {
      const checkDomain = siteForm.value.domain
      const checkPath = siteForm.value.path
      const dup = allSites.value.find(s =>
        s.id !== siteForm.value.id &&
        (s.domain) === checkDomain &&
        (s.path) === checkPath
      )
      if (dup) {
        ElMessage.error(t('proxy.duplicatePath'))
        siteSaving.value = false
        return
      }
    }
    const upstream = (siteForm.value.upstreams || []).filter(u => u.trim()).join(',')
const payload = {
domain: domain,
upstream: upstream,
path: siteForm.value.path,
backend_path: siteForm.value.backend_path,
template: siteForm.value.template,
      cert_path: siteForm.value.use_cert ? siteForm.value.cert_path : '',
      key_path: siteForm.value.use_cert ? siteForm.value.key_path : '',
      proxy_config: siteForm.value.proxy_config,
      custom_config: siteForm.value.custom_config,
waf_enabled: siteForm.value.waf_enabled,
      lb_policy: siteForm.value.lb_policy,
      log_enabled: siteForm.value.log_enabled || false,
      log_path: siteForm.value.log_path,
      log_format: siteForm.value.log_format,
      l4_protocol: siteForm.value.l4_protocol,
      sni_target: siteForm.value.sni_target,
      local_port: sniSettings.value.local_port,
      proxy_protocol: siteForm.value.proxy_protocol || false,
      is_panel: siteForm.value.is_panel,
      priority: siteForm.value.is_panel ? 0 : 100
    }
    if (siteForm.value.id) {
      await client.put(`${caddyApi.value}/sites/${siteForm.value.id}`, payload)
    } else {
      await client.post(`${caddyApi.value}/sites`, payload)
    }
    await applyConfig()
    ElMessage.success(t('proxy.siteSaved'))
    siteDialogVisible.value = false
    fetchSites()
  } catch (e) { ElMessage.error(t('common.failed')) } finally { siteSaving.value = false }
}

async function toggleSite(row) {
  try {
    await client.put(`${caddyApi.value}/sites/${row.id}`, { enabled: row.enabled })
    await applyConfig()
    fetchSites()
  } catch (e) { row.enabled = !row.enabled }
}

async function deleteSite(row) {
  await ElMessageBox.confirm(t('common.confirm'))
  try {
    await client.delete(`${caddyApi.value}/sites/${row.id}`)
    await applyConfig()
    ElMessage.success(t('common.success'))
    fetchSites()
  } catch (e) { ElMessage.error(t('common.failed')) }
}

async function viewConfig() {
  try {
    const data = await client.get(`${proxyApi.value}/config?type=caddy`)
    configViewContent.value = data.config
    configViewVisible.value = true
  } catch (e) { ElMessage.error(t('common.failed')) }
}

function copyConfig() {
  navigator.clipboard.writeText(configViewContent.value)
  ElMessage.success(t('proxy.configCopied'))
}

function showAddSNI() {
sniForm.value = { domain: '', sni_target: '', proxy_protocol: false, inbound_id: null }
sniDialogVisible.value = true
}

async function saveSNIRule() {
  try {
    const domains = (sniForm.value.domain).split(',').map(d => d.trim()).filter(Boolean)
    if (domains.length === 0) {
      ElMessage.warning(t('proxy.sniDomainRequired'))
      return
    }
    for (const domain of domains) {
      await client.post(`${caddyApi.value}/sites`, {
        domain: domain,
        upstream: '',
        template: 'sni',
        sni_target: sniForm.value.sni_target,
        local_port: sniSettings.value.local_port,
        proxy_protocol: sniForm.value.proxy_protocol,
        is_panel: false,
        priority: 50,
        path: sniSettings.value.listen_port
      })
    }
    await applyConfig()
    ElMessage.success(t('common.success'))
    sniDialogVisible.value = false
    fetchSites()
  } catch (e) { ElMessage.error(t('common.failed')) }
}

// --- WAF Management (CaddyGuard) ---
const wafTab = ref('config-form')
const wafConfigContent = ref('')
const wafRules = ref([])
const wafRuleVisible = ref(false)
const wafRuleName = ref('')
const wafRuleContent = ref('')

const wafRuleDescMap = {
  'args.rule': 'wafRuleArgs', 'blackip.rule': 'wafRuleBlackip', 'cookie.rule': 'wafRuleCookie',
  'post.rule': 'wafRulePost', 'url.rule': 'wafRuleUrl', 'useragent.rule': 'wafRuleUseragent',
  'whiteip.rule': 'wafRuleWhiteip', 'whiteurl.rule': 'wafRuleWhiteurl', 'whiteua.rule': 'wafRuleWhiteua',
  'referer.rule': 'wafRuleReferer', 'fileext.rule': 'wafRuleFileext',
'cdnip.rule': 'wafRuleCdnip'
}
function wafRuleDesc(name) {
  const key = wafRuleDescMap[name]
  return key ? t(`proxy.${key}`) : name
}

const wafConfigForm = ref({
  waf_enable: 'on', trust_proxy_headers: 'on',
  white_url_check: 'on', white_ip_check: 'on', white_ua_check: 'on',
  black_ip_check: 'on',
  url_check: 'on', url_args_check: 'on', user_agent_check: 'on',
  cookie_check: 'on', post_check: 'on', referer_check: 'off', file_upload_check: 'on',
  cc_check: 'on', cc_rate: '', cc_block_ttl: '',
  bodyless: 'on', multipart_streaming_check: 'off',
  upload_filename_scan_limit: '0', post_body_scan_limit: '2097152',
  waf_output: 'html', waf_redirect_url: '', log_dir: '', rule_dir: ''
})

async function fetchWAFStatus() {
  try {
    const data = await client.get(`${apiBase.value}/waf/status`)
    wafMasterEnabled.value = data.enabled || false
  } catch { wafMasterEnabled.value = false }
}
async function saveWafMasterSwitch(val) {
  wafStatusSaving.value = true
  try {
    await client.put(`${apiBase.value}/waf/status`, { enabled: val })
    await applyConfig()
  } catch {
    wafMasterEnabled.value = !val
  } finally {
    wafStatusSaving.value = false
  }
}
async function fetchWAFConfig() {
  try {
    const data = await client.get(`${apiBase.value}/waf/config`, { params: { proxy_type: 'caddy' } })
    wafConfigContent.value = data.content || ''
  } catch { wafConfigContent.value = '' }
  parseWAFConfigForm()
}
async function saveWAFConfig() {
  try {
    await client.put(`${apiBase.value}/waf/config`, { proxy_type: 'caddy', content: wafConfigContent.value })
    ElMessage.success(t('common.success'))
    parseWAFConfigForm()
  } catch { ElMessage.error(t('common.failed')) }
}

// CaddyGuard config.json is JSON, so parse with JSON.parse
function parseWAFConfigForm() {
  try {
    const obj = JSON.parse(wafConfigContent.value || '{}')
    wafConfigForm.value = {
      waf_enable: obj.waf_enable || 'on',
      trust_proxy_headers: obj.trust_proxy_headers || 'on',
      white_url_check: obj.white_url_check || 'on',
      white_ip_check: obj.white_ip_check || 'on',
      white_ua_check: obj.white_ua_check || 'on',
      black_ip_check: obj.black_ip_check || 'on',
      url_check: obj.url_check || 'on',
      url_args_check: obj.url_args_check || 'on',
      user_agent_check: obj.user_agent_check || 'on',
      cookie_check: obj.cookie_check || 'on',
      post_check: obj.post_check || 'on',
      referer_check: obj.referer_check || 'off',
      file_upload_check: obj.file_upload_check || 'on',
      bodyless: obj.bodyless || 'on',
      multipart_streaming_check: obj.multipart_streaming_check || 'off',
      upload_filename_scan_limit: String(obj.upload_filename_scan_limit ?? '0'),
      post_body_scan_limit: String(obj.post_body_scan_limit ?? '2097152'),
      cc_check: obj.cc_check || 'on',
      cc_rate: obj.cc_rate || '',
      cc_block_ttl: String(obj.cc_block_ttl || '600'),
      waf_output: obj.waf_output || 'html',
      waf_redirect_url: obj.waf_redirect_url || '',
      log_dir: obj.log_dir || '',
      rule_dir: obj.rule_dir || ''
    }
  } catch { /* keep defaults */ }
}

async function saveWAFConfigForm() {
  const obj = JSON.parse(wafConfigContent.value || '{}')
  obj.waf_enable = wafConfigForm.value.waf_enable
  obj.trust_proxy_headers = wafConfigForm.value.trust_proxy_headers
  obj.white_url_check = wafConfigForm.value.white_url_check
  obj.white_ip_check = wafConfigForm.value.white_ip_check
  obj.white_ua_check = wafConfigForm.value.white_ua_check
  obj.black_ip_check = wafConfigForm.value.black_ip_check
  obj.url_check = wafConfigForm.value.url_check
  obj.url_args_check = wafConfigForm.value.url_args_check
  obj.user_agent_check = wafConfigForm.value.user_agent_check
  obj.cookie_check = wafConfigForm.value.cookie_check
  obj.post_check = wafConfigForm.value.post_check
  obj.referer_check = wafConfigForm.value.referer_check
  obj.file_upload_check = wafConfigForm.value.file_upload_check
  obj.bodyless = wafConfigForm.value.bodyless
  obj.multipart_streaming_check = wafConfigForm.value.multipart_streaming_check
  obj.upload_filename_scan_limit = parseInt(wafConfigForm.value.upload_filename_scan_limit) || 0
  obj.post_body_scan_limit = parseInt(wafConfigForm.value.post_body_scan_limit) || 2097152
  obj.cc_check = wafConfigForm.value.cc_check
  obj.cc_rate = wafConfigForm.value.cc_rate
  obj.cc_block_ttl = parseInt(wafConfigForm.value.cc_block_ttl) || 600
  obj.waf_output = wafConfigForm.value.waf_output
  obj.waf_redirect_url = wafConfigForm.value.waf_redirect_url
  obj.log_dir = wafConfigForm.value.log_dir
  obj.rule_dir = wafConfigForm.value.rule_dir
  wafConfigContent.value = JSON.stringify(obj, null, 4)
  await saveWAFConfig()
}

async function fetchWAFRules() {
  try {
    const data = await client.get('/waf/rules')
    wafRules.value = Array.isArray(data) ? data : (data.items || [])
  } catch { wafRules.value = [] }
}
async function openWAFRule(row) {
  wafRuleName.value = row.name
  try {
    const data = await client.get(`/waf/rules/${row.name}`)
    wafRuleContent.value = data.content || ''
    wafRuleVisible.value = true
  } catch { ElMessage.error(t('common.failed')) }
}
async function saveWAFRule() {
  try {
    await client.put(`/waf/rules/${wafRuleName.value}`, { content: wafRuleContent.value })
    ElMessage.success(t('common.success'))
    wafRuleVisible.value = false
    fetchWAFRules()
  } catch { ElMessage.error(t('common.failed')) }
}

// Site WAF Config Dialog
const siteWAFVisible = ref(false)
const siteWAFDomain = ref('')
const siteWAFTab = ref('config')
const siteWAFConfig = ref({})
const siteWAFRuleMode = ref('global')
const siteWAFDomainRules = ref([])
const siteWAFRuleEditorVisible = ref(false)
const siteWAFRuleEditing = ref('')
const siteWAFRuleEditingContent = ref('')

async function openSiteWAF(row) {
const d = row.domain.replace(/:\d+$/, '')
siteWAFDomain.value = d
siteWAFTab.value = 'config'
try {
const data = await client.get(`${apiBase.value}/waf/domain-config`)
const obj = JSON.parse(data.content || '{}')
const cfg = obj[d] || {}
    siteWAFConfig.value = {
      waf_enable: cfg.waf_enable || 'on',
      trust_proxy_headers: cfg.trust_proxy_headers || 'on',
      url_check: cfg.url_check || 'on',
      url_args_check: cfg.url_args_check || 'on',
      user_agent_check: cfg.user_agent_check || 'on',
      cookie_check: cfg.cookie_check || 'on',
      post_check: cfg.post_check || 'on',
      referer_check: cfg.referer_check || 'off',
file_upload_check: cfg.file_upload_check || 'on',
bodyless: cfg.bodyless || 'on',
multipart_streaming_check: cfg.multipart_streaming_check || 'off',
upload_filename_scan_limit: String(cfg.upload_filename_scan_limit ?? '0'),
post_body_scan_limit: String(cfg.post_body_scan_limit ?? '2097152'),
white_url_check: cfg.white_url_check || 'on',
      white_ip_check: cfg.white_ip_check || 'on',
      white_ua_check: cfg.white_ua_check || 'on',
      black_ip_check: cfg.black_ip_check || 'on',
      cc_check: cfg.cc_check || 'on',
      cc_rate: cfg.cc_rate || '60/60',
      cc_block_ttl: String(cfg.cc_block_ttl || '600'),
      waf_output: cfg.waf_output || 'html',
      waf_redirect_url: cfg.waf_redirect_url || ''
    }
    siteWAFRuleMode.value = cfg.rule_dir ? 'custom' : 'global'
    await loadSiteWAFDomainRules(row.domain)
  } catch { ElMessage.error(t('common.failed')) }
  siteWAFVisible.value = true
}

async function loadSiteWAFDomainRules(domain) {
  try {
    const dirs = await client.get(`${apiBase.value}/waf/domain-rules`)
    if (dirs && dirs.includes(domain)) {
      const ruleNames = ['args.rule', 'blackip.rule', 'cookie.rule', 'post.rule', 'url.rule', 'useragent.rule', 'whiteip.rule', 'whiteurl.rule', 'whiteua.rule', 'referer.rule', 'fileext.rule']
      const rules = []
      for (const name of ruleNames) {
        try {
          const data = await client.get(`${apiBase.value}/waf/domain-rules/${domain}/${name}`)
          rules.push({ name, size: (data.content || '').length })
        } catch { /* skip missing */ }
      }
      siteWAFDomainRules.value = rules
    } else {
      siteWAFDomainRules.value = []
    }
  } catch { siteWAFDomainRules.value = [] }
}

async function onSiteWAFRuleModeChange(val) {
  if (val === 'custom') {
    try {
      await client.post(`${apiBase.value}/waf/enable-domain`, { domain: siteWAFDomain.value })
      await loadSiteWAFDomainRules(siteWAFDomain.value)
    } catch { ElMessage.error(t('common.failed')) }
  }
}

async function openSiteWAFDomainRule(row) {
  siteWAFRuleEditing.value = row.name
  try {
    const data = await client.get(`${apiBase.value}/waf/domain-rules/${siteWAFDomain.value}/${row.name}`)
    siteWAFRuleEditingContent.value = data.content || ''
    siteWAFRuleEditorVisible.value = true
  } catch { ElMessage.error(t('common.failed')) }
}

async function saveSiteWAFDomainRule() {
  try {
    await client.put(`${apiBase.value}/waf/domain-rules/${siteWAFDomain.value}/${siteWAFRuleEditing.value}`, { content: siteWAFRuleEditingContent.value })
    ElMessage.success(t('common.success'))
    siteWAFRuleEditorVisible.value = false
    await loadSiteWAFDomainRules(siteWAFDomain.value)
  } catch { ElMessage.error(t('common.failed')) }
}

async function saveSiteWAFConfig() {
  try {
    const data = await client.get(`${apiBase.value}/waf/domain-config`)
    const obj = JSON.parse(data.content || '{}')
    const cfg = {}
    for (const [k, v] of Object.entries(siteWAFConfig.value)) {
      if (v !== '' && v !== undefined && v !== null) cfg[k] = v
    }
    if (siteWAFRuleMode.value === 'custom') {
      cfg.rule_dir = `domains/${siteWAFDomain.value}`
    }
    obj[siteWAFDomain.value] = cfg
    delete obj['_comment']
    const content = JSON.stringify(obj, null, 4)
    await client.put(`${apiBase.value}/waf/domain-config`, { content })
    ElMessage.success(t('common.success'))
    siteWAFVisible.value = false
  } catch { ElMessage.error(t('common.failed')) }
}

onMounted(() => {
  fetchProxyInfo()
  fetchSites()
  fetchCertificates()
  fetchSNISettings()
  fetchInbounds()
  fetchWAFConfig()
  fetchWAFRules()
  fetchWAFStatus()
})
</script>
<style scoped>
:deep(.el-dialog__body) { max-height: calc(100vh - 200px); overflow-y: auto; }
</style>
