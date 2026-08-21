<template>
  <div>
    <h2 style="margin-bottom:20px">Nginx {{ t('proxy.management') }}</h2>

    <!-- Port conflict warning -->
    <el-alert v-if="caddyRunning" type="warning" :closable="false" show-icon style="margin-bottom:16px">
      Caddy {{ t('proxy.isRunning') }}，{{ t('proxy.portConflictHint') }}
    </el-alert>

    <!-- Status Card -->
    <el-card shadow="hover" style="margin-bottom:20px">
      <template #header>
        <div style="display:flex;justify-content:space-between;align-items:center">
          <div style="display:flex;align-items:center;gap:12px">
            <span style="font-size:18px;font-weight:600">Nginx</span>
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
<el-button v-if="proxyInfo.status!=='running'" type="success" @click="proxyAction('start')">{{ t('proxy.start') }}</el-button>
<el-button v-else type="danger" @click="proxyAction('stop')">{{ t('proxy.stop') }}</el-button>
              <el-button @click="proxyAction('restart')">{{ t('proxy.restart') }}</el-button>
              <el-button type="warning" plain @click="showReleases">{{ t('proxy.switchVersion') }}</el-button>
              <el-button plain @click="viewConfig">{{ t('proxy.viewConfig') }}</el-button>
            </template>
          </div>
        </el-col>
      </el-row>
    </el-card>

    <!-- Nginx Settings -->
    <el-card style="margin-bottom:20px">
      <template #header>
        <span>Nginx {{ t('proxy.settings') }}</span>
      </template>
      <el-tabs v-model="settingsTab">
        <el-tab-pane label="Worker" name="worker">
          <el-form :model="settings" label-width="180px" size="small">
            <el-form-item label="worker_processes">
              <el-input v-model="settings.worker_processes" style="width:200px" />
            </el-form-item>
            <el-form-item label="worker_rlimit_nofile">
              <el-input v-model="settings.worker_rlimit_nofile" style="width:200px" />
            </el-form-item>
            <el-form-item label="worker_connections">
              <el-input v-model="settings.worker_connections" style="width:200px" />
            </el-form-item>
            <el-form-item label="worker_priority">
              <el-input v-model="settings.worker_priority" style="width:200px" />
            </el-form-item>
            <el-form-item label="daemon">
              <el-switch v-model="settings.daemon" />
            </el-form-item>
            <el-form-item label="multi_accept">
              <el-switch v-model="settings.multi_accept" />
            </el-form-item>
            <el-divider content-position="left">{{ t('proxy.defaultPorts') }}</el-divider>
            <el-form-item :label="t('proxy.defaultHttpPort')">
              <el-input v-model="settings.default_http_port" style="width:200px" placeholder="80" />
            </el-form-item>
            <el-form-item :label="t('proxy.defaultHttpsPort')">
              <el-input v-model="settings.default_https_port" style="width:200px" placeholder="443" />
            </el-form-item>
          </el-form>
        </el-tab-pane>
        <el-tab-pane :label="t('proxy.nginxLog')" name="log">
          <el-form :model="settings" label-width="150px" size="small">
            <el-form-item :label="t('proxy.logOutputMode')">
              <el-radio-group v-model="nginxLogMode" @change="onNginxLogModeChange">
                <el-radio-button value="file">{{ t('proxy.logToFile') }}</el-radio-button>
                <el-radio-button value="stdout">{{ t('proxy.logToStdout') }}</el-radio-button>
                <el-radio-button value="off">{{ t('proxy.logOff') }}</el-radio-button>
              </el-radio-group>
              <div style="margin-top:4px;font-size:12px;color:#909399">
                {{ nginxLogModeHint }}
              </div>
            </el-form-item>
            <el-divider />
            <template v-if="nginxLogMode === 'file'">
            <el-form-item :label="t('proxy.errorLog')">
              <el-switch v-model="settings.error_log" />
            </el-form-item>
            <el-form-item v-if="settings.error_log" :label="t('proxy.errorLogPath')">
              <el-input v-model="settings.error_log_path" style="width:300px" placeholder="logs/error.log" />
            </el-form-item>
            <el-form-item v-if="settings.error_log" :label="t('proxy.errorLogLevel')">
              <el-select v-model="settings.error_log_level" style="width:200px">
                <el-option label="debug" value="debug" />
                <el-option label="info" value="info" />
                <el-option label="notice" value="notice" />
                <el-option label="warn" value="warn" />
                <el-option label="error" value="error" />
                <el-option label="crit" value="crit" />
                <el-option label="alert" value="alert" />
                <el-option label="emerg" value="emerg" />
              </el-select>
            </el-form-item>
            <el-divider />
            <el-form-item :label="t('proxy.accessLog')">
              <el-switch v-model="settings.access_log" />
            </el-form-item>
            <el-form-item v-if="settings.access_log" :label="t('proxy.accessLogPath')">
              <el-input v-model="settings.access_log_path" style="width:300px" placeholder="logs/access.log" />
            </el-form-item>
            <el-divider content-position="left">Stream {{ t('proxy.accessLog') }}</el-divider>
            <el-form-item label="stream access_log">
              <el-switch v-model="settings.stream_log" />
            </el-form-item>
            <el-form-item v-if="settings.stream_log" :label="t('proxy.logPath')">
              <el-input v-model="settings.stream_log_path" style="width:300px" placeholder="logs/stream.log" />
            </el-form-item>
            <el-form-item v-if="settings.stream_log" :label="t('proxy.logFormat')">
              <el-select v-model="settings.stream_log_format" style="width:200px">
                <el-option label="basic" value="basic" />
                <el-option label="json" value="json" />
              </el-select>
            </el-form-item>
            </template>
          </el-form>
        </el-tab-pane>
        <el-tab-pane :label="t('proxy.nginxHttp')" name="http">
          <el-form :model="settings" label-width="200px" size="small">
            <el-form-item label="client_max_body_size">
              <el-input v-model="settings.client_max_body_size" style="width:200px" />
            </el-form-item>
            <el-form-item label="keepalive_timeout">
              <el-input v-model="settings.keepalive_timeout" style="width:200px" />
            </el-form-item>
            <el-form-item label="keepalive_requests">
              <el-input v-model="settings.keepalive_requests" style="width:200px" />
            </el-form-item>
            <el-form-item label="server_tokens">
              <el-switch v-model="settings.server_tokens" />
            </el-form-item>
            <el-form-item label="sendfile">
              <el-switch v-model="settings.sendfile" />
            </el-form-item>
            <el-form-item label="tcp_nopush">
              <el-switch v-model="settings.tcp_nopush" />
            </el-form-item>
            <el-form-item label="tcp_nodelay">
              <el-switch v-model="settings.tcp_nodelay" />
            </el-form-item>
            <el-form-item label="underscores_in_headers">
              <el-switch v-model="settings.underscores_in_headers" />
            </el-form-item>
            <el-form-item label="charset">
              <el-input v-model="settings.charset" style="width:200px" />
            </el-form-item>
          </el-form>
        </el-tab-pane>
        <el-tab-pane :label="t('proxy.nginxProxy')" name="proxy">
          <el-form :model="settings" label-width="200px" size="small">
            <el-form-item label="proxy_connect_timeout">
              <el-input v-model="settings.proxy_connect_timeout" style="width:200px" />
            </el-form-item>
            <el-form-item label="proxy_read_timeout">
              <el-input v-model="settings.proxy_read_timeout" style="width:200px" />
            </el-form-item>
            <el-form-item label="proxy_send_timeout">
              <el-input v-model="settings.proxy_send_timeout" style="width:200px" />
            </el-form-item>
            <el-form-item label="proxy_buffer_size">
              <el-input v-model="settings.proxy_buffer_size" style="width:200px" />
            </el-form-item>
            <el-form-item label="proxy_buffers">
              <el-input v-model="settings.proxy_buffers" style="width:200px" />
            </el-form-item>
            <el-form-item label="proxy_busy_buffers_size">
              <el-input v-model="settings.proxy_busy_buffers_size" style="width:200px" />
            </el-form-item>
            <el-form-item label="proxy_ignore_client_abort">
              <el-switch v-model="settings.proxy_ignore_client_abort" />
            </el-form-item>
          </el-form>
        </el-tab-pane>
        <el-tab-pane :label="t('proxy.nginxGzip')" name="gzip">
          <el-form :model="settings" label-width="200px" size="small">
            <el-form-item label="gzip">
              <el-switch v-model="settings.gzip" />
            </el-form-item>
            <el-form-item label="gzip_comp_level">
              <el-input v-model="settings.gzip_comp_level" style="width:200px" />
            </el-form-item>
            <el-form-item label="gzip_min_length">
              <el-input v-model="settings.gzip_min_length" style="width:200px" />
            </el-form-item>
            <el-form-item label="gzip_types">
              <el-input v-model="settings.gzip_types" type="textarea" :rows="3" style="width:100%" />
            </el-form-item>
          </el-form>
        </el-tab-pane>
        <el-tab-pane :label="t('proxy.nginxBrotli')" name="brotli">
          <el-form :model="settings" label-width="200px" size="small">
            <el-form-item label="brotli">
              <el-switch v-model="settings.brotli" />
            </el-form-item>
            <el-form-item label="brotli_comp_level">
              <el-input v-model="settings.brotli_comp_level" style="width:200px" />
            </el-form-item>
            <el-form-item label="brotli_min_length">
              <el-input v-model="settings.brotli_min_length" style="width:200px" />
            </el-form-item>
            <el-form-item label="brotli_types">
              <el-input v-model="settings.brotli_types" type="textarea" :rows="3" style="width:100%" />
            </el-form-item>
          </el-form>
        </el-tab-pane>
        <el-tab-pane :label="t('proxy.nginxHttp23')" name="http23">
          <el-form :model="settings" label-width="250px" size="small">
            <el-form-item label="http2_max_concurrent_streams">
              <el-input v-model="settings.http2_max_concurrent_streams" style="width:200px" />
            </el-form-item>
            <el-form-item label="http3">
              <el-switch v-model="settings.http3" />
            </el-form-item>
            <el-form-item label="http3_max_concurrent_streams">
              <el-input v-model="settings.http3_max_concurrent_streams" style="width:200px" />
            </el-form-item>
            <el-form-item label="quic_retry">
              <el-switch v-model="settings.quic_retry" />
            </el-form-item>
          </el-form>
        </el-tab-pane>
        <el-tab-pane :label="t('proxy.nginxLimit')" name="limit">
          <el-form :model="settings" label-width="200px" size="small">
            <el-form-item label="limit_conn_zone">
              <el-input v-model="settings.limit_conn_zone" style="width:400px" />
            </el-form-item>
            <el-form-item label="limit_conn">
              <el-input v-model="settings.limit_conn" style="width:200px" />
            </el-form-item>
          </el-form>
        </el-tab-pane>
        <el-tab-pane :label="t('proxy.nginxCollisionWaf')" name="waf">
          <el-form :model="settings" label-width="200px" size="small">
<el-form-item :label="t('proxy.luaEnabled')">
<el-tag type="success" size="small">ON</el-tag>
<span style="margin-left:12px;font-size:12px;color:#909399">{{ t('proxy.luaEnabledHint') }}</span>
</el-form-item>
            <el-divider />
            <el-form-item :label="t('proxy.wafEnabled')">
              <el-switch v-model="wafMasterEnabled" :loading="wafStatusSaving" @change="saveWafMasterSwitch" />
              <span style="margin-left:12px;font-size:12px;color:#909399">{{ t('proxy.wafEnabledHint') }}</span>
            </el-form-item>
            <div v-if="wafMasterEnabled" style="margin-left:20px;padding:12px;background:#f5f7fa;border-radius:4px;font-size:13px;color:#606266">
              <div>{{ t('proxy.wafFeatures') }}</div>
              <ul style="margin-top:8px;line-height:1.8">
                <li>{{ t('proxy.wafFeatureUrl') }}</li>
                <li>{{ t('proxy.wafFeatureArgs') }}</li>
                <li>{{ t('proxy.wafFeatureUa') }}</li>
                <li>{{ t('proxy.wafFeatureCookie') }}</li>
                <li>{{ t('proxy.wafFeaturePost') }}</li>
                <li>{{ t('proxy.wafFeatureCc') }}</li>
                <li>{{ t('proxy.wafFeatureIp') }}</li>
              </ul>
            </div>
          </el-form>
        </el-tab-pane>
        <el-tab-pane :label="t('proxy.nginxCustom')" name="custom">
          <el-input v-model="settings.custom_config" type="textarea" :rows="10" style="font-family:monospace" :placeholder="t('proxy.nginxCustomHint')" />
        </el-tab-pane>
      </el-tabs>
      <div style="margin-top:12px;text-align:right">
        <el-button type="success" @click="saveSettings">{{ t('common.save') }}</el-button>
        <el-button @click="fetchSettings">{{ t('common.reset') }}</el-button>
      </div>
    </el-card>

    <!-- WAF Management -->
    <el-card style="margin-bottom:20px">
      <template #header>
        <div style="display:flex;justify-content:space-between;align-items:center">
          <span>WAF {{ t('proxy.management') }}</span>
<el-tag :type="wafMasterEnabled ? 'success' : 'info'" size="small">
{{ wafMasterEnabled ? 'ON' : 'OFF' }}
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

    <!-- SNI Routing -->
    <el-card style="margin-bottom:20px">
      <template #header>
        <div style="display:flex;justify-content:space-between;align-items:center">
          <div style="display:flex;align-items:center;gap:12px">
            <span>SNI {{ t('proxy.routing') }}</span>
            <el-switch v-model="settings.stream_sni_enabled" @change="onSNIToggle" />
          </div>
          <div v-if="settings.stream_sni_enabled" style="display:flex;gap:8px;align-items:center">
            <el-button size="small" type="primary" @click="showAddSNI">+ SNI {{ t('proxy.rule') }}</el-button>
          </div>
        </div>
      </template>
      <template v-if="settings.stream_sni_enabled">
        <div style="display:flex;gap:16px;align-items:flex-end;flex-wrap:wrap;margin-bottom:16px">
          <div>
            <div style="font-size:12px;color:#909399;margin-bottom:4px">SNI {{ t('proxy.listenPort') }}</div>
            <el-input v-model="sniListenPort" size="small" style="width:100px" placeholder="443" />
          </div>
          <div>
            <div style="font-size:12px;color:#909399;margin-bottom:4px">{{ t('proxy.localHttpsPort') }}</div>
            <el-input v-model="sniLocalPort" size="small" style="width:100px" placeholder="8443" />
          </div>
          <el-button size="small" type="success" @click="saveSettings">{{ t('common.save') }}</el-button>
          <el-button size="small" type="warning" @click="batchMigrate">{{ t('proxy.batchMigrate') }}</el-button>
        </div>
        <!-- Stream settings inline -->
        <div style="display:flex;gap:16px;align-items:flex-end;flex-wrap:wrap;margin-bottom:16px;padding:12px;background:#f5f7fa;border-radius:4px">
          <div>
            <div style="font-size:12px;color:#909399;margin-bottom:4px">SNI ssl_preread cache</div>
            <el-input v-model="settings.stream_sni_cache_size" size="small" style="width:100px" placeholder="64" />
          </div>
          <div>
            <div style="font-size:12px;color:#909399;margin-bottom:4px">Proxy Protocol</div>
            <el-switch v-model="settings.stream_proxy_protocol" />
          </div>
          <div>
            <div style="font-size:12px;color:#909399;margin-bottom:4px">IPv6</div>
            <el-switch v-model="settings.stream_ipv6" />
          </div>
          <div>
            <div style="font-size:12px;color:#909399;margin-bottom:4px">proxy_connect_timeout</div>
            <el-input v-model="settings.stream_connect_timeout" size="small" style="width:100px" placeholder="1s" />
          </div>
          <div>
            <div style="font-size:12px;color:#909399;margin-bottom:4px">proxy_timeout</div>
            <el-input v-model="settings.stream_timeout" size="small" style="width:100px" placeholder="3s" />
          </div>
        </div>
        <el-table :data="sniPagedList" stripe size="small">
          <el-table-column :label="t('common.status')" width="70">
            <template #default="{row}">
              <el-switch v-model="row.enabled" size="small" @change="toggleSite(row)" />
            </template>
          </el-table-column>
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
          {{ t('proxy.sniFallback') }}: 127.0.0.1:{{ sniLocalPort }}
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
          <template #default="{row}">
            <span style="color:#409eff;font-weight:600">:{{ row.listen_port || row.path }}</span>
          </template>
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
        <el-table-column label="Path" min-width="100">
          <template #default="{row}">
            <span v-if="row.listen_port" style="color:#409eff;font-weight:600">:{{ row.listen_port }}</span>
            <span v-if="row.path"> {{ row.path }}</span>
            <span v-if="!row.path && !row.listen_port" style="color:#c0c4cc">/</span>
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
        <el-table-column label="WAF" width="100" v-if="wafMasterEnabled">
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

    <!-- Config View Dialog -->
    <el-dialog v-model="configViewVisible" title="nginx.conf" width="700">
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
        <el-form-item v-if="siteForm.template==='l4'" :label="t('proxy.listenPort')">
          <el-input v-model="siteForm.listen_port" placeholder="443" style="width:120px" />
        </el-form-item>
        <el-form-item v-else :label="t('proxy.listenPort')">
          <el-input v-model="siteForm.listen_port" placeholder="80" style="width:120px" />
        </el-form-item>
        <el-form-item v-if="siteForm.template==='sni'" label="SNI Domain">
          <el-input v-model="siteForm.domain" placeholder="www.example.com" />
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
<el-form-item v-if="siteForm.template!=='sni' && siteForm.template!=='static' && siteForm.template!=='portal_forward' && siteForm.template!=='subscription_forward' && siteForm.template!=='reality'" :label="t('proxy.template')">
            <el-select v-model="siteForm.template" style="width:100%">
            <el-option label="HTTP" value="http" />
            <el-option label="WebSocket" value="websocket" />
            <el-option label="gRPC" value="grpc" />
            <el-option label="XHTTP" value="xhttp" />
            <el-option label="HTTPUpgrade" value="httpupgrade" />
            <el-option :label="t('proxy.templateL4')" value="l4" />
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
            <el-option :label="t('proxy.templateL4')" value="l4" />
            <el-option label="Static" value="static" />
            <el-option v-if="isLocal" :label="t('proxy.templatePortalForward')" value="portal_forward" />
            <el-option v-if="isLocal" :label="t('proxy.templateSubscriptionForward')" value="subscription_forward" />
            <el-option :label="t('proxy.templateReality')" value="reality" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('proxy.proxyProtocol')">
          <el-switch v-model="siteForm.proxy_protocol" :disabled="siteForm.template==='reality'" />
          <span v-if="siteForm.template==='reality'" style="font-size:12px;color:#909399;margin-left:8px">{{ t('common.required') }}</span>
        </el-form-item>
        <el-form-item label="IPv6">
          <el-switch v-model="siteForm.enable_ipv6" />
        </el-form-item>
        <el-form-item :label="t('proxy.useCertificate')">
          <el-switch v-model="siteForm.use_cert" :disabled="siteForm.template==='reality'" @change="onToggleCert" />
          <span v-if="siteForm.template==='reality'" style="font-size:12px;color:#909399;margin-left:8px">{{ t('common.required') }}</span>
        </el-form-item>
        <el-form-item v-if="siteForm.use_cert" :label="t('proxy.selectCert')">
          <el-select v-model="siteForm.cert_id" style="width:100%" clearable @change="onSiteCertSelect" :placeholder="t('proxy.selectCert')">
            <el-option v-for="c in certificates" :key="c.id" :label="`${c.domain} (${c.issuer})`" :value="c.id" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="siteForm.use_cert" :label="t('proxy.certPath')">
          <el-input v-model="siteForm.cert_path" placeholder="/path/to/cert.pem" />
        </el-form-item>
        <el-form-item v-if="siteForm.use_cert" :label="t('proxy.keyPath')">
          <el-input v-model="siteForm.key_path" placeholder="/path/to/key.pem" />
        </el-form-item>
        <template v-if="siteForm.use_cert">
          <el-divider content-position="left">TLS {{ t('proxy.settings') }}</el-divider>
          <el-form-item label="HTTP/2">
            <el-switch v-model="siteForm.enable_http2" />
          </el-form-item>
          <el-form-item label="QUIC">
            <el-switch v-model="siteForm.enable_quic" />
          </el-form-item>
          <el-form-item label="ssl_early_data">
            <el-switch v-model="siteForm.enable_early_data" />
          </el-form-item>
          <el-form-item label="ssl_stapling">
            <el-switch v-model="siteForm.enable_stapling" />
          </el-form-item>
          <el-form-item label="ssl_protocols">
            <el-select v-model="siteForm.ssl_protocols" style="width:100%" allow-filter>
<el-option :label="t('proxy.tlsDefault')" value="TLSv1.2 TLSv1.3" />
<el-option :label="t('proxy.tlsCompatible')" value="TLSv1 TLSv1.1 TLSv1.2 TLSv1.3" />
<el-option :label="t('proxy.tlsSecure')" value="TLSv1.3" />
            </el-select>
          </el-form-item>
          <el-form-item label="ssl_ciphers">
<el-select v-model="siteForm.ssl_ciphers" style="width:100%" allow-filter clearable :placeholder="t('proxy.leaveEmptyDefault')">
<el-option :label="t('proxy.cipherModern')" value="ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384:ECDHE-ECDSA-CHACHA20-POLY1305:ECDHE-RSA-CHACHA20-POLY1305" />
<el-option :label="t('proxy.cipherCompatible')" value="ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384:ECDHE-ECDSA-CHACHA20-POLY1305:ECDHE-RSA-CHACHA20-POLY1305:DHE-RSA-AES128-GCM-SHA256:DHE-RSA-AES256-GCM-SHA384:DHE-RSA-CHACHA20-POLY1305" />
<el-option :label="t('proxy.cipherLegacy')" value="@SECLEVEL=0:ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384:ECDHE-ECDSA-CHACHA20-POLY1305:ECDHE-RSA-CHACHA20-POLY1305:DHE-RSA-AES128-GCM-SHA256:DHE-RSA-AES256-GCM-SHA384:DHE-RSA-CHACHA20-POLY1305:ECDHE-ECDSA-AES128-SHA256:ECDHE-RSA-AES128-SHA256:ECDHE-ECDSA-AES128-SHA:ECDHE-RSA-AES128-SHA:ECDHE-ECDSA-AES256-SHA384:ECDHE-RSA-AES256-SHA384:ECDHE-ECDSA-AES256-SHA:ECDHE-RSA-AES256-SHA:DHE-RSA-AES128-SHA256:DHE-RSA-AES256-SHA256:AES128-GCM-SHA256:AES256-GCM-SHA384:AES128-SHA256:AES256-SHA256:AES128-SHA:AES256-SHA:DES-CBC3-SHA" />
            </el-select>
          </el-form-item>
          <el-form-item label="ssl_ecdh_curve">
<el-select v-model="siteForm.ssl_ecdh_curve" style="width:100%" allow-filter clearable :placeholder="t('proxy.leaveEmptyDefault')">
<el-option :label="t('proxy.ecdhDefault')" value="X25519:prime256v1:secp384r1" />
              <el-option label="X25519:P-384:P-256" value="X25519:P-384:P-256" />
              <el-option label="prime256v1:secp384r1" value="prime256v1:secp384r1" />
              <el-option label="X25519MLKEM768:SecP384r1MLKEM1024:SecP256r1MLKEM768:X25519:P-384:P-256 (PQC)" value="X25519MLKEM768:SecP384r1MLKEM1024:SecP256r1MLKEM768:X25519:P-384:P-256" />
            </el-select>
          </el-form-item>
        </template>
        <el-divider content-position="left">WAF</el-divider>
        <el-form-item :label="t('proxy.wafEnabled')">
          <el-switch v-model="siteForm.waf_enabled" :active-value="true" :inactive-value="false" />
          <span style="margin-left:8px;font-size:12px;color:#909399">{{ t('proxy.wafEnabledHint') }}</span>
        </el-form-item>
      </el-form>
      <div v-else>
        <div style="font-size:12px;color:#909399;margin-bottom:8px">{{ t('proxy.configPreview') }}</div>
        <el-input :model-value="generatePreview(siteForm)" type="textarea" :rows="10" readonly style="font-family:monospace;font-size:13px;background:#f5f7fa" />
        <div style="margin-top:12px">
          <div style="font-size:12px;color:#909399;margin-bottom:4px">{{ t('proxy.serverConfig') }}</div>
          <el-input v-model="siteForm.server_config" type="textarea" :rows="3" style="font-family:monospace;font-size:13px" placeholder="add_header X-Frame-Options SAMEORIGIN;" />
        </div>
        <div style="margin-top:12px">
          <div style="font-size:12px;color:#909399;margin-bottom:4px">{{ t('proxy.locationConfig') }}</div>
          <el-input v-model="siteForm.location_config" type="textarea" :rows="3" style="font-family:monospace;font-size:13px" placeholder="proxy_connect_timeout 60;" />
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
import { ref, computed, onMounted, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Loading } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'
import { useQueryState } from '../composables/useQueryState'
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
const nginxApi = computed(() => `${apiBase.value}/nginx`)
const proxyApi = computed(() => `${apiBase.value}/proxy`)

const proxyInfo = ref({})
const guardian = ref({})
const caddyRunning = ref(false)
const inbounds = ref([])
const guardianConfig = ref({ autoRestart: true, maxRestarts: 5 })
const allSites = ref([])
const certificates = ref([])
const settings = ref({})
const wafMasterEnabled = ref(false)
const wafStatusSaving = ref(false)
const settingsTab = useQueryState('tab', 'worker')
const nginxLogMode = ref('file')
const nginxLogModeHint = computed(() => {
  const m = nginxLogMode.value
  if (m === 'stdout') return t('proxy.hintStdout')
  if (m === 'off') return t('proxy.hintLogOff')
  return t('proxy.hintFile')
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
  const upstream = ib._isSocket ? `unix:${ib._socketPath}` : `127.0.0.1:${ib.port}`
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

const sniListenPort = ref('443')
const sniLocalPort = ref('8443')

// Releases
const releasesVisible = ref(false)
const releasesLoading = ref(false)
const releases = ref([])
const downloadingAsset = ref('')
const downloading = ref(false)
const downloadProgress = ref('')

// WAF Management
const wafTab = ref('config-form')
const wafConfigContent = ref('')
const wafRules = ref([])
const wafDomainContent = ref('')
const wafRuleVisible = ref(false)
const wafRuleName = ref('')
const wafRuleContent = ref('')

// Rule file name → i18n key mapping
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

async function fetchWAFConfig() {
  try {
    const data = await client.get(`${apiBase.value}/waf/config`, { params: { proxy_type: 'nginx' } })
    wafConfigContent.value = data.content || ''
  } catch { wafConfigContent.value = '' }
  parseWAFConfigForm()
}
async function saveWAFConfig() {
  try {
    await client.put(`${apiBase.value}/waf/config`, { proxy_type: 'nginx', content: wafConfigContent.value })
    ElMessage.success(t('common.success'))
    parseWAFConfigForm()
    await applyConfig()
  } catch { ElMessage.error(t('common.failed')) }
}

// WAF Config Form (parse config.lua text ↔ form object)
const wafConfigForm = ref({
  waf_enable: 'on', trust_proxy_headers: 'on',
  white_url_check: 'on', white_ip_check: 'on', white_ua_check: 'on',
  black_ip_check: 'on',
  url_check: 'on', url_args_check: 'on', user_agent_check: 'on',
  cookie_check: 'on', post_check: 'on', referer_check: 'on', file_upload_check: 'on',
  cc_check: 'on', cc_rate: '', cc_block_ttl: '',
  bodyless: 'on', multipart_streaming_check: 'off',
  upload_filename_scan_limit: '0', post_body_scan_limit: '2097152',
  waf_output: 'html', waf_redirect_url: '', log_dir: '', rule_dir: ''
})

function parseWAFConfigForm() {
  const text = wafConfigContent.value || ''
  const get = (key) => {
    const m = text.match(new RegExp(`config_${key}\\s*=\\s*"([^"]*)"`))
    if (m) return m[1]
    // try unquoted number (e.g. config_cc_block_ttl = 600)
    const m2 = text.match(new RegExp(`config_${key}\\s*=\\s*([0-9]+)`))
    return m2 ? m2[1] : ''
  }
  wafConfigForm.value = {
    waf_enable: get('waf_enable') || 'on',
    trust_proxy_headers: get('trust_proxy_headers') || 'on',
    white_url_check: get('white_url_check') || 'on',
    white_ip_check: get('white_ip_check') || 'on',
    white_ua_check: get('white_ua_check') || 'on',
    black_ip_check: get('black_ip_check') || 'on',
    url_check: get('url_check') || 'on',
    url_args_check: get('url_args_check') || 'on',
    user_agent_check: get('user_agent_check') || 'on',
    cookie_check: get('cookie_check') || 'on',
    post_check: get('post_check') || 'on',
    referer_check: get('referer_check') || 'on',
    file_upload_check: get('file_upload_check') || 'on',
    bodyless: get('bodyless') || 'on',
    multipart_streaming_check: get('multipart_streaming_check') || 'off',
    upload_filename_scan_limit: get('upload_filename_scan_limit') || '0',
    post_body_scan_limit: get('post_body_scan_limit') || '2097152',
    cc_check: get('cc_check') || 'on',
    cc_rate: get('cc_rate') || '',
    cc_block_ttl: get('cc_block_ttl') || '600',
    waf_output: get('waf_output') || 'html',
    waf_redirect_url: get('waf_redirect_url') || '',
    log_dir: get('log_dir') || '',
    rule_dir: get('rule_dir') || ''
  }
}

async function saveWAFConfigForm() {
  let text = wafConfigContent.value || ''
  const set = (key, val) => {
    if (val !== '' && val != null) {
      text = text.replace(new RegExp(`config_${key}\\s*=\\s*"[^"]*"`), `config_${key} = "${val}"`)
    }
  }
  set('waf_enable', wafConfigForm.value.waf_enable)
  set('trust_proxy_headers', wafConfigForm.value.trust_proxy_headers)
  set('white_url_check', wafConfigForm.value.white_url_check)
  set('white_ip_check', wafConfigForm.value.white_ip_check)
  set('white_ua_check', wafConfigForm.value.white_ua_check)
  set('black_ip_check', wafConfigForm.value.black_ip_check)
  set('url_check', wafConfigForm.value.url_check)
  set('url_args_check', wafConfigForm.value.url_args_check)
  set('user_agent_check', wafConfigForm.value.user_agent_check)
  set('cookie_check', wafConfigForm.value.cookie_check)
  set('post_check', wafConfigForm.value.post_check)
  set('referer_check', wafConfigForm.value.referer_check)
  set('file_upload_check', wafConfigForm.value.file_upload_check)
  set('bodyless', wafConfigForm.value.bodyless)
  set('multipart_streaming_check', wafConfigForm.value.multipart_streaming_check)
  // upload_filename_scan_limit and post_body_scan_limit are Lua numbers (no quotes)
  const ulScanLimit = wafConfigForm.value.upload_filename_scan_limit
  if (ulScanLimit !== '' && ulScanLimit != null) {
    text = text.replace(new RegExp(`config_upload_filename_scan_limit\s*=\s*[^\n]+`), `config_upload_filename_scan_limit = ${ulScanLimit}`)
  }
  const postBodyLimit = wafConfigForm.value.post_body_scan_limit
  if (postBodyLimit !== '' && postBodyLimit != null) {
    text = text.replace(new RegExp(`config_post_body_scan_limit\s*=\s*[^\n]+`), `config_post_body_scan_limit = ${postBodyLimit}`)
  }
  set('cc_check', wafConfigForm.value.cc_check)
  set('cc_rate', wafConfigForm.value.cc_rate)
  // cc_block_ttl is a Lua number (no quotes), use special handling
  const ttl = wafConfigForm.value.cc_block_ttl
  if (ttl !== '' && ttl != null) {
    text = text.replace(new RegExp(`config_cc_block_ttl\\s*=\\s*[^\n]+`), `config_cc_block_ttl = ${ttl}`)
  }
  set('waf_output', wafConfigForm.value.waf_output)
  set('waf_redirect_url', wafConfigForm.value.waf_redirect_url)
  set('log_dir', wafConfigForm.value.log_dir)
  set('rule_dir', wafConfigForm.value.rule_dir)
  wafConfigContent.value = text
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
async function fetchWAFDomainConfig() {
  try {
    const data = await client.get(`${apiBase.value}/waf/domain-config`)
    wafDomainContent.value = data.content || ''
  } catch { wafDomainContent.value = '' }
}
async function saveWAFDomainConfig() {
  try {
    await client.put(`${apiBase.value}/waf/domain-config`, { content: wafDomainContent.value })
    ElMessage.success(t('common.success'))
  } catch { ElMessage.error(t('common.failed')) }
}

// WAF Domain Rules Management (site-level, not standalone)
// parseWAFDomainList and serializeWAFDomainList are no longer needed —
// domain.json is managed via site WAF toggle + config dialog.

// Site WAF Config Dialog state
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
  // Load current domain config from domain.json
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
      cc_block_ttl: cfg.cc_block_ttl || '600',
      waf_output: cfg.waf_output || 'html',
      waf_redirect_url: cfg.waf_redirect_url || ''
    }
    siteWAFRuleMode.value = cfg.rule_dir ? 'custom' : 'global'
    await loadSiteWAFDomainRules(d)
  } catch { ElMessage.error(t('common.failed')) }
  siteWAFVisible.value = true
}

async function loadSiteWAFDomainRules(domain) {
  try {
    const dirs = await client.get(`${apiBase.value}/waf/domain-rules`)
    if (dirs && dirs.includes(domain)) {
      // Domain has a rule directory — list rule files
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
    // Enable custom rules: ensure rule_dir is set, copy global rules if needed
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
  // Serialize config back to domain.json
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
    // Remove _comment if accidentally at wrong level
    delete obj['_comment']
    const content = JSON.stringify(obj, null, 4)
    await client.put(`${apiBase.value}/waf/domain-config`, { content })
    ElMessage.success(t('common.success'))
    siteWAFVisible.value = false
  } catch { ElMessage.error(t('common.failed')) }
}

// Site dialog
const siteDialogVisible = ref(false)
const siteDialogTitle = ref('')
const siteSaving = ref(false)
const siteForm = ref({
  domain: '', upstreams: [''], path: '', backend_path: '', template: 'http',
  listen_port: '80', listen_socket: '',
  use_cert: false, cert_id: null, cert_path: '', key_path: '',
  custom_config: '', server_config: '', location_config: '',
  l4_protocol: 'tcp', sni_target: '', local_port: '8443',
  proxy_protocol: false, is_panel: false,
  enable_http2: true, enable_ipv6: false, enable_quic: false,
  enable_early_data: false, enable_stapling: false,
  ssl_protocols: '', ssl_ciphers: '', ssl_ecdh_curve: '',
  waf_enabled: true,
  editMode: 'form'
})

// Panel port
const panelPort = ref('8080')

// Config view
const configViewVisible = ref(false)
const configViewContent = ref('')

// SNI
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
const upstream = ib._isSocket ? `unix:${ib._socketPath}` : `127.0.0.1:${ib.port}`
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
    proxyInfo.value = status.nginx || {}
    guardian.value = status.nginx_guardian || {}
    guardianConfig.value.autoRestart = guardian.value.auto_restart !== false
    guardianConfig.value.maxRestarts = guardian.value.max_restarts || 5
    if (status.panel_port) panelPort.value = status.panel_port
    caddyRunning.value = (status.caddy && status.caddy.status === 'running')
  } catch (e) { ElMessage.error(t('common.failed')) }
}

async function saveGuardianConfig() {
  try {
    await client.post(`${proxyApi.value}/guardian?type=nginx`, {
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
allSites.value = await client.get(`${nginxApi.value}/sites`) || []
// Check WAF status for each site
if (wafMasterEnabled.value) {
  for (const site of allSites.value) {
    if (site.domain && site.template !== 'sni' && site.template !== 'stream') {
      try {
        const res = await client.get(`${apiBase.value}/waf/domain-status`, { params: { domain: site.domain } })
        site._waf_enabled = res.enabled
      } catch { site._waf_enabled = false }
    } else {
      site._waf_enabled = false
    }
  }
}
} catch (e) { allSites.value = [] }
}

async function toggleSiteWAF(row, val) {
  const d = row.domain.replace(/:\d+$/, '')
  if (val) {
    // Enable: add to domain.json with rule_dir
    try {
      await client.post(`${apiBase.value}/waf/enable-domain`, { domain: d })
      ElMessage.success(t('proxy.wafSiteEnabled', { domain: d }))
    } catch (e) {
      row._waf_enabled = false
      ElMessage.error(t('common.failed'))
    }
  } else {
    // Disable: ask user — disable WAF entirely or fall back to global rules
    try {
      const action = await ElMessageBox.confirm(
        t('proxy.wafDisableHint', { domain: d }),
        t('proxy.wafDisableTitle'),
        {
          distinguishCancelAndClose: true,
          confirmButtonText: t('proxy.wafDisableSite'),
          cancelButtonText: t('proxy.wafUseGlobal'),
          type: 'warning',
        }
      )
      // confirmButtonText clicked → disable WAF for this domain (waf_enable: off)
      try {
        await client.put(`${apiBase.value}/waf/domain-config`, {
          content: await updateDomainJSON(d, { waf_enable: 'off' })
        })
        ElMessage.success(t('proxy.wafSiteDisabled', { domain: d }))
      } catch (e) {
        row._waf_enabled = true
        ElMessage.error(t('common.failed'))
      }
    } catch (action) {
      if (action === 'cancel') {
        // cancelButtonText clicked → use global rules (remove from domain.json)
        try {
          await client.post(`${apiBase.value}/waf/disable-domain`, { domain: d })
          ElMessage.success(t('proxy.wafUseGlobalRules', { domain: d }))
        } catch (e) {
          row._waf_enabled = true
          ElMessage.error(t('common.failed'))
        }
      } else {
        // closed (X) → revert switch
        row._waf_enabled = true
      }
    }
  }
}

// Helper: read current domain.json, update a domain's config, return new content
async function updateDomainJSON(domain, patch) {
  const data = await client.get(`${apiBase.value}/waf/domain-config`)
  const obj = JSON.parse(data.content || '{}')
  if (!obj[domain]) obj[domain] = {}
  Object.assign(obj[domain], patch)
  return JSON.stringify(obj, null, 4)
}

async function fetchCertificates() {
  try { certificates.value = (await client.get('/certificates', { params: { limit: 100 } })).items || [] } catch (e) { ElMessage.error(t('common.failed')) }
}

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
    if (val) {
      await fetchWAFConfig()
      await fetchWAFRules()
    }
  } catch {
    wafMasterEnabled.value = !val
  } finally {
    wafStatusSaving.value = false
  }
}
async function fetchSettings() {
  try {
    const data = await client.get(`${nginxApi.value}/settings`)
    if (data) settings.value = data
  } catch (e) { ElMessage.error(t('common.failed')) }
  // Load log output mode (per-node)
  try {
    const cfg = await client.get('/log-manage/log-output', { params: { server_id: props.serverId } })
    if (cfg?.nginx?.mode) nginxLogMode.value = cfg.nginx.mode
  } catch (e) { ElMessage.error(t('common.failed')) }
}

async function saveSettings() {
  try {
    await client.post(`${nginxApi.value}/settings`, settings.value)
    // Save log output mode (preserve caddy mode) — per-node
    const current = await client.get('/log-manage/log-output', { params: { server_id: props.serverId } })
    await client.put('/log-manage/log-output', {
      nginx: { mode: nginxLogMode.value },
      caddy: current?.caddy || { mode: 'file' },
    }, { params: { server_id: props.serverId } })
    await applyConfig()
    ElMessage.success(t('common.success'))
  } catch (e) { ElMessage.error(t('common.failed')) }
}

async function onNginxLogModeChange(mode) {
  nginxLogMode.value = mode
}

async function onSNIToggle(enabled) {
  try {
    settings.value.stream_sni_enabled = enabled
    await saveSettings()
    if (enabled) {
      await client.post(`${nginxApi.value}/sni-rules/toggle`, { enabled: true })
      const localPort = sniLocalPort.value
      settings.value.default_https_port = localPort
      await saveSettings()
      const existing = await client.get(`${nginxApi.value}/sites`)
      const hasFallback = (existing || []).some(s => s.template === 'sni' && (s.domain === 'fallback' || s.domain === ''))
      if (!hasFallback) {
        await client.post(`${nginxApi.value}/sites`, {
          domain: 'fallback', upstream: '', template: 'sni',
          sni_target: `127.0.0.1:${localPort}`, local_port: localPort,
          proxy_protocol: true, is_panel: false, priority: 999,
          listen_port: sniListenPort.value
        })
      }
      const res = await client.post(`${nginxApi.value}/batch-update-ports`, { port: localPort, enable: true })
      ElMessage.success(t('proxy.sniEnabled', { count: res.updated || 0 }))
    } else {
      settings.value.default_https_port = '443'
      await saveSettings()
      await client.post(`${nginxApi.value}/sni-rules/toggle`, { enabled: false })
      await client.post(`${nginxApi.value}/batch-update-ports`, { port: '443', enable: false })
      ElMessage.success(t('proxy.sniDisabled'))
    }
    await applyConfig()
    fetchSites()
  } catch (e) { ElMessage.error(t('common.failed')) }
}

async function batchMigrate() {
  const port = sniLocalPort.value
  try {
    settings.value.default_https_port = port
    await saveSettings()
    const existing = await client.get(`${nginxApi.value}/sites`)
    const hasFallback = (existing || []).some(s => s.template === 'sni' && (s.domain === 'fallback' || s.domain === ''))
    if (!hasFallback) {
      await client.post(`${nginxApi.value}/sites`, {
        domain: 'fallback', upstream: '', template: 'sni',
        sni_target: `127.0.0.1:${port}`, local_port: port,
        proxy_protocol: true, is_panel: false, priority: 999,
        listen_port: sniListenPort.value
      })
    }
    const res = await client.post(`${nginxApi.value}/batch-update-ports`, { port, enable: true })
    await applyConfig()
    ElMessage.success(t('proxy.migrationComplete', { count: res.updated || 0 }))
    fetchSites()
  } catch (e) { ElMessage.error(t('common.failed')) }
}

async function showReleases() {
  releasesVisible.value = true
  releasesLoading.value = true
  try {
    releases.value = await client.get(`${proxyApi.value}/releases`, { params: { type: 'nginx' } }) || []
  } catch (e) { releases.value = [] } finally { releasesLoading.value = false }
}

async function downloadVersion(url, tagName) {
  downloading.value = true
    downloadProgress.value = t('proxy.downloading', { tag: tagName })
  try {
    await client.post(`${proxyApi.value}/download`, { type: 'nginx', download_url: url, tag_name: tagName })
    try { await client.post(`${proxyApi.value}/start?type=nginx`) } catch {}
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
await client.post(`${proxyApi.value}/${action}?type=nginx`)
ElMessage.success(t('common.success'))
fetchProxyInfo()
} catch (e) { 
  const msg = e?.response?.data?.message || t('common.failed')
  ElMessage.error(msg)
}
}

function showAddSite(template) {
  siteDialogTitle.value = t('proxy.addSite')
  const defaultPort = settings.value.default_http_port
  const isReality = template === 'reality'
siteForm.value = {
domain: '', upstreams: [''], path: '', backend_path: '', template,
    listen_port: isReality ? '9443' : (template === 'l4' ? '443' : defaultPort), listen_socket: '',
    use_cert: isReality, cert_id: null, cert_path: '', key_path: '',
    custom_config: '', server_config: '', location_config: '',
    l4_protocol: 'tcp', sni_target: '', local_port: '8443',
    proxy_protocol: isReality, is_panel: false,
    enable_http2: true, enable_ipv6: false, enable_quic: false,
    enable_early_data: false, enable_stapling: false,
  ssl_protocols: isReality ? 'TLSv1.2 TLSv1.3' : '', ssl_ciphers: '', ssl_ecdh_curve: '',
  waf_enabled: !isReality,
  editMode: 'form',
    inbound_id: null
  }
  siteDialogVisible.value = true
}

function showPanelProxy() {
  siteDialogTitle.value = t('proxy.proxyPanel')
  const defaultPort = settings.value.default_http_port
  siteForm.value = {
    domain: '_', upstreams: [`127.0.0.1:${panelPort.value}`], path: '', template: 'http',
    listen_port: defaultPort, listen_socket: '',
    use_cert: false, cert_id: null, cert_path: '', key_path: '',
    custom_config: '', server_config: '', location_config: '',
    l4_protocol: 'tcp', sni_target: '', local_port: '8443',
    proxy_protocol: false, is_panel: true,
    enable_http2: true, enable_ipv6: false, enable_quic: false,
    enable_early_data: false, enable_stapling: false,
  ssl_protocols: '', ssl_ciphers: '', ssl_ecdh_curve: '',
  waf_enabled: true,
  editMode: 'form',
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
  const defaultHttp = settings.value.default_http_port
  const defaultHttps = settings.value.default_https_port
  if (enabled && (siteForm.value.listen_port === defaultHttp || siteForm.value.listen_port === '')) {
    siteForm.value.listen_port = defaultHttps
  } else if (!enabled && siteForm.value.listen_port === defaultHttps) {
    siteForm.value.listen_port = defaultHttp
  }
}

function editSite(row) {
  siteDialogTitle.value = t('common.edit')
  let certId = null
  if (row.cert_path) {
    const match = certificates.value.find(c => c.cert_path === row.cert_path)
    if (match) certId = match.id
  }
  let upstreams = ['']
  if (row.upstream) {
    upstreams = row.upstream.split(/[,\n]/).map(s => s.trim()).filter(s => s)
    if (upstreams.length === 0) upstreams = ['']
  }
  const listenPort = row.template === 'l4' ? (row.path || row.listen_port) : (row.listen_port)
  siteForm.value = {
    id: row.id, domain: row.domain, upstreams, path: row.path,
    backend_path: row.backend_path,
    template: row.template, listen_port: listenPort,
    listen_socket: row.listen_socket,
    use_cert: !!row.cert_path, cert_id: certId,
    cert_path: row.cert_path, key_path: row.key_path,
    custom_config: row.custom_config,
    server_config: row.server_config,
    location_config: row.location_config,
    l4_protocol: row.l4_protocol,
    sni_target: row.sni_target, local_port: row.local_port,
    proxy_protocol: row.proxy_protocol || false, is_panel: row.is_panel,
    enable_http2: row.enable_http2 !== false,
    enable_ipv6: row.enable_ipv6 || false,
    enable_quic: row.enable_quic || false,
    enable_early_data: row.enable_early_data || false,
    enable_stapling: row.enable_stapling || false,
    ssl_protocols: row.ssl_protocols,
    ssl_ciphers: row.ssl_ciphers,
    ssl_ecdh_curve: row.ssl_ecdh_curve,
    waf_enabled: row.waf_enabled !== false,
    editMode: 'form',
    inbound_id: null
  }
  siteDialogVisible.value = true
}

function generatePreview(site) {
  let lines = []
  const domain = site.domain
  // L4 preview
  if (site.template === 'l4') {
    const port = site.listen_port
    const proto = site.l4_protocol
    const upstreams = (site.upstreams || []).filter(u => u.trim())
    if (upstreams.length === 0) upstreams.push(`127.0.0.1:${panelPort.value}`)
    const upstreamName = 'l4_' + (site.domain).replace(/[^a-zA-Z0-9_]/g, '_')
    lines.push(`upstream ${upstreamName} {`)
    for (const up of upstreams) { lines.push(`    server ${up};`) }
    lines.push(`}`, ``)
    lines.push(`server {`)
    lines.push(`    listen ${port} so_keepalive=on;`)
    if (site.enable_ipv6) lines.push(`    listen [::]:${port} ipv6only=on so_keepalive=on;`)
    lines.push(`    proxy_socket_keepalive on;`)
    lines.push(`    proxy_buffer_size 512k;`)
    if (site.proxy_protocol) lines.push(`    proxy_protocol on;`)
    lines.push(`    proxy_pass ${upstreamName};`)
    lines.push(`    proxy_timeout 5m;`)
    lines.push(`    proxy_connect_timeout 2s;`)
    lines.push(`}`)
    return lines.join('\n')
  }
  // Reality preview
  if (site.template === 'reality') {
    const port = site.listen_port || '9443'
    const certPath = site.cert_path || '/path/to/cert.pem'
    const keyPath = site.key_path || '/path/to/key.pem'
    lines.push(`server {`)
    lines.push(`    listen 127.0.0.1:${port} ssl proxy_protocol;`)
    lines.push(`    http2 on;`)
    lines.push(`    server_name _;`)
    lines.push(`    ssl_certificate ${certPath};`)
    lines.push(`    ssl_certificate_key ${keyPath};`)
    lines.push(`    ssl_protocols TLSv1.2 TLSv1.3;`)
    lines.push(`    set_real_ip_from 127.0.0.1;`)
    lines.push(`    real_ip_header proxy_protocol;`)
    lines.push(`    location = / {`)
    lines.push(`        default_type 'text/html';`)
    lines.push(`        return 200 'Success';`)
    lines.push(`    }`)
    lines.push(`}`)
    return lines.join('\n')
  }
  // HTTP preview
  const path = site.path
  const upstreams = (site.upstreams || []).filter(u => u.trim())
  if (upstreams.length === 0) upstreams.push('127.0.0.1:8080')
  const upstreamStr = upstreams.join(',')
  lines.push(`server {`)
  const port = site.listen_port
  if (site.cert_path && site.key_path) {
    lines.push(`    listen ${port} ssl;`)
    if (site.enable_ipv6) lines.push(`    listen [::]:${port} ssl;`)
  } else {
    lines.push(`    listen ${port};`)
    if (site.enable_ipv6) lines.push(`    listen [::]:${port};`)
  }
  lines.push(`    server_name ${domain};`)
  if (site.cert_path && site.key_path) {
    lines.push(`    ssl_certificate ${site.cert_path};`)
    lines.push(`    ssl_certificate_key ${site.key_path};`)
    lines.push(`    ssl_protocols TLSv1.2 TLSv1.3;`)
  }
  lines.push(``)
  if (site.template === 'static') {
    const rootDir = upstreams[0]
    lines.push(`    location ${path} {`)
    lines.push(`        root ${rootDir};`)
    lines.push(`        index index.html index.htm;`)
    lines.push(`        try_files $uri $uri/ =404;`)
    lines.push(`    }`, `}`)
    return lines.join('\n')
  }
  if (upstreams.length > 1) {
    lines.push(`    upstream backend {`)
    for (const up of upstreams) { lines.push(`        server ${up};`) }
    lines.push(`    }`, ``)
  }
  lines.push(`    location ${path} {`)
  const passTarget = upstreams.length > 1 ? 'http://backend' : `http://${upstreamStr}`
  if (site.template === 'websocket') {
    lines.push(`        proxy_pass ${passTarget};`)
    lines.push(`        proxy_http_version 1.1;`)
    lines.push(`        proxy_set_header Upgrade $http_upgrade;`)
    lines.push(`        proxy_set_header Connection "upgrade";`)
    lines.push(`        proxy_set_header Host $host;`)
    lines.push(`        proxy_set_header X-Forwarded-Proto $scheme;`)
    lines.push(`        proxy_set_header X-Real-IP $remote_addr;`)
    lines.push(`        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;`)
  } else if (site.template === 'grpc') {
    lines.push(`        grpc_pass grpc://${upstreamStr};`)
    lines.push(`        proxy_set_header X-Real-IP $remote_addr;`)
    lines.push(`        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;`)
  } else {
    lines.push(`        proxy_pass ${passTarget};`)
    lines.push(`        proxy_http_version 1.1;`)
    lines.push(`        proxy_set_header Accept-Encoding "";`)
    lines.push(`        proxy_set_header Host $host;`)
    lines.push(`        proxy_set_header X-Real-IP $remote_addr;`)
    lines.push(`        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;`)
    lines.push(`        proxy_set_header X-Forwarded-Proto $scheme;`)
  }
  lines.push(`    }`, `}`)
  return lines.join('\n')
}

async function applyConfig() {
  try {
    await client.post(`${nginxApi.value}/generate-config`)
    if (proxyInfo.value.status === 'running') {
      await client.post(`${proxyApi.value}/reload?type=nginx`)
    }
    fetchProxyInfo()
  } catch (e) { ElMessage.error(t('common.failed')) }
}

async function saveSite() {
  siteSaving.value = true
  try {
    const upstream = (siteForm.value.upstreams || []).filter(u => u.trim()).join(',')
    const pathValue = siteForm.value.template === 'l4' ? (siteForm.value.listen_port) : (siteForm.value.path)
    // Check for duplicate path on the same domain (skip SNI and L4 templates)
    if (siteForm.value.template !== 'sni' && siteForm.value.template !== 'l4') {
      const domain = siteForm.value.domain
      const dup = allSites.value.find(s =>
        s.id !== siteForm.value.id &&
        (s.domain) === domain &&
        (s.path) === (pathValue)
      )
      if (dup) {
        ElMessage.error(t('proxy.duplicatePath'))
        siteSaving.value = false
        return
      }
    }
    const payload = {
domain: siteForm.value.domain,
upstream: upstream,
path: pathValue,
backend_path: siteForm.value.backend_path,
template: siteForm.value.template,
      listen_port: siteForm.value.listen_port,
      listen_socket: siteForm.value.listen_socket,
      cert_path: siteForm.value.use_cert ? siteForm.value.cert_path : '',
      key_path: siteForm.value.use_cert ? siteForm.value.key_path : '',
      server_config: siteForm.value.server_config,
      location_config: siteForm.value.location_config,
      l4_protocol: siteForm.value.l4_protocol,
      sni_target: siteForm.value.sni_target,
      local_port: siteForm.value.local_port,
      proxy_protocol: siteForm.value.proxy_protocol || false,
      is_panel: siteForm.value.is_panel,
      priority: siteForm.value.is_panel ? 0 : 100,
      enable_http2: siteForm.value.enable_http2 !== false,
      enable_ipv6: siteForm.value.enable_ipv6 || false,
      enable_quic: siteForm.value.enable_quic || false,
      enable_early_data: siteForm.value.enable_early_data || false,
      enable_stapling: siteForm.value.enable_stapling || false,
      ssl_protocols: siteForm.value.ssl_protocols,
      ssl_ciphers: siteForm.value.ssl_ciphers,
      ssl_ecdh_curve: siteForm.value.ssl_ecdh_curve,
      waf_enabled: siteForm.value.waf_enabled !== false
    }
    if (siteForm.value.id) {
      await client.put(`${nginxApi.value}/sites/${siteForm.value.id}`, payload)
    } else {
      await client.post(`${nginxApi.value}/sites`, payload)
    }
    await applyConfig()
    ElMessage.success(t('proxy.siteSaved'))
    siteDialogVisible.value = false
    fetchSites()
  } catch (e) { ElMessage.error(t('common.failed')) } finally { siteSaving.value = false }
}

async function toggleSite(row) {
  try {
    await client.put(`${nginxApi.value}/sites/${row.id}`, { enabled: row.enabled })
    await applyConfig()
    fetchSites()
  } catch (e) { row.enabled = !row.enabled }
}

async function deleteSite(row) {
  await ElMessageBox.confirm(t('common.confirm'))
  try {
    await client.delete(`${nginxApi.value}/sites/${row.id}`)
    await applyConfig()
    ElMessage.success(t('common.success'))
    fetchSites()
  } catch (e) { ElMessage.error(t('common.failed')) }
}

async function viewConfig() {
  try {
    const data = await client.get(`${proxyApi.value}/config?type=nginx`)
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
      await client.post(`${nginxApi.value}/sites`, {
        domain: domain,
        upstream: '',
        template: 'sni',
        sni_target: sniForm.value.sni_target,
        local_port: sniLocalPort.value,
        proxy_protocol: sniForm.value.proxy_protocol,
        is_panel: false,
        priority: 50,
        listen_port: sniListenPort.value
      })
    }
    await applyConfig()
    ElMessage.success(t('common.success'))
    sniDialogVisible.value = false
    fetchSites()
  } catch (e) { ElMessage.error(t('common.failed')) }
}

onMounted(async () => {
  fetchProxyInfo()
  fetchCertificates()
  fetchInbounds()
  fetchWAFConfig()
  fetchWAFRules()
  fetchWAFDomainConfig()
  // Load WAF status and settings first, then fetch sites (fetchSites depends on wafMasterEnabled)
  await fetchWAFStatus()
  await fetchSettings()
  fetchSites()
})
</script>
<style scoped>
:deep(.el-dialog__body) { max-height: calc(100vh - 200px); overflow-y: auto; }
</style>
