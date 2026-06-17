<template>
  <div>
    <h2 style="margin-bottom:20px">Caddy {{ t('proxy.management') || '管理' }}</h2>

    <!-- Status Card -->
    <el-card shadow="hover" style="margin-bottom:20px">
      <template #header>
        <div style="display:flex;justify-content:space-between;align-items:center">
          <div style="display:flex;align-items:center;gap:12px">
            <span style="font-size:18px;font-weight:600">Caddy</span>
            <el-tag v-if="proxyInfo.installed" :type="proxyInfo.status==='running'?'success':'info'" size="small">
              {{ proxyInfo.status || 'unknown' }}
            </el-tag>
            <el-tag v-else type="danger" size="small">{{ t('proxy.notInstalled') }}</el-tag>
          </div>
          <div style="display:flex;gap:8px">
            <el-button size="small" type="danger" plain @click="$emit('back')">{{ t('proxy.switch') || '切换' }}</el-button>
          </div>
        </div>
      </template>
      <el-row :gutter="24">
        <el-col :span="12">
          <div style="display:flex;flex-direction:column;gap:10px;font-size:13px">
            <div style="display:flex;justify-content:space-between">
              <span style="color:#909399">{{ t('proxy.version') }}</span>
              <span style="font-weight:600">{{ proxyInfo.version || '-' }}</span>
            </div>
            <div style="display:flex;justify-content:space-between">
              <span style="color:#909399">{{ t('proxy.path') }}</span>
              <span style="font-weight:600;font-size:12px">{{ proxyInfo.path || '-' }}</span>
            </div>
            <template v-if="guardian.state && guardian.state !== 'no_guardian'">
              <el-divider style="margin:4px 0" />
              <div style="display:flex;align-items:center;gap:12px;margin-bottom:8px">
                <span style="color:#909399;font-size:13px">{{ t('proxy.autoRestart') || '异常自动重启' }}</span>
                <el-switch v-model="guardianConfig.autoRestart" @change="saveGuardianConfig" />
                <el-input-number v-model="guardianConfig.maxRestarts" :min="1" :max="100" size="small" style="width:100px" @change="saveGuardianConfig" />
                <span style="font-size:12px;color:#909399">{{ t('proxy.maxRestarts') || '次' }}</span>
              </div>
              <div style="display:flex;gap:16px;font-size:13px">
                <div style="display:flex;gap:6px;align-items:center">
                  <span style="color:#909399">{{ t('proxy.restartStatus') || '状态' }}</span>
                  <el-tag :type="guardian.current_restarts > 0 ? 'warning' : 'success'" size="small">
                    {{ guardian.current_restarts > 0 ? (t('proxy.restarted') || '已重启') : (t('proxy.normal') || '正常') }}
                  </el-tag>
                </div>
                <div style="display:flex;gap:6px;align-items:center">
                  <span style="color:#909399">{{ t('proxy.restartCount') || '次数' }}</span>
                  <span>{{ guardian.current_restarts }}/{{ guardian.max_restarts }}</span>
                </div>
                <div v-if="guardian.last_crash_at && guardian.last_crash_at !== '0001-01-01T00:00:00Z'" style="display:flex;gap:6px;align-items:center">
                  <span style="color:#909399">{{ t('proxy.lastCrash') || '上次崩溃' }}</span>
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
              <el-button type="warning" plain @click="showReleases">{{ t('proxy.switchVersion') || '更新' }}</el-button>
              <el-button plain @click="viewConfig">{{ t('proxy.viewConfig') }}</el-button>
            </template>
          </div>
        </el-col>
      </el-row>
    </el-card>

    <!-- Log Output Mode -->
    <el-card style="margin-bottom:20px">
      <template #header>
        <div style="display:flex;justify-content:space-between;align-items:center">
          <span style="font-weight:600">{{ t('proxy.logOutputMode') || '日志输出方式' }}</span>
          <el-button type="primary" size="small" :loading="logModeSaving" @click="saveCaddyLogMode">{{ t('common.save') }}</el-button>
        </div>
      </template>
      <el-radio-group v-model="caddyLogMode" @change="onCaddyLogModeChange">
        <el-radio-button value="file">{{ t('proxy.logToFile') || '输出到文件' }}</el-radio-button>
        <el-radio-button value="stdout">{{ t('proxy.logToStdout') || '标准输出' }}</el-radio-button>
        <el-radio-button value="off">{{ t('proxy.logOff') || '关闭日志' }}</el-radio-button>
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
            <span>SNI {{ t('proxy.routing') || '分流' }}</span>
            <el-switch v-model="sniSettings.enabled" @change="onSNIToggle" />
          </div>
          <div v-if="sniSettings.enabled" style="display:flex;gap:8px;align-items:center">
            <el-button size="small" type="primary" @click="showAddSNI">+ SNI {{ t('proxy.rule') || '规则' }}</el-button>
          </div>
        </div>
      </template>
      <template v-if="sniSettings.enabled">
        <div style="display:flex;gap:16px;align-items:flex-end;flex-wrap:wrap;margin-bottom:16px">
          <div>
            <div style="font-size:12px;color:#909399;margin-bottom:4px">SNI {{ t('proxy.listenPort') || '监听端口' }}</div>
            <el-input v-model="sniSettings.listen_port" size="small" style="width:100px" placeholder="443" />
          </div>
          <div>
            <div style="font-size:12px;color:#909399;margin-bottom:4px">{{ t('proxy.localHttpsPort') || '本地 HTTPS 端口' }}</div>
            <el-input v-model="sniSettings.local_port" size="small" style="width:100px" placeholder="8443" />
          </div>
          <el-button size="small" type="success" @click="saveSNISettings">{{ t('common.save') }}</el-button>
          <el-button size="small" type="warning" @click="batchMigrate">{{ t('proxy.batchMigrate') || '一键迁移' }}</el-button>
        </div>
        <!-- Proxy Protocol Rules -->
        <div style="margin-bottom:16px;padding:12px;background:#f5f7fa;border-radius:4px">
          <div style="font-size:12px;color:#909399;margin-bottom:8px">Proxy Protocol {{ t('proxy.allowRules') || '允许规则' }}</div>
          <div v-for="(rule, idx) in sniSettings.proxy_protocol_rules" :key="idx" style="display:flex;gap:8px;margin-bottom:6px;align-items:center">
            <el-input v-model="sniSettings.proxy_protocol_rules[idx]" size="small" style="width:250px" placeholder="127.0.0.0/8" />
            <el-button size="small" type="danger" plain @click="sniSettings.proxy_protocol_rules.splice(idx, 1)">-</el-button>
          </div>
          <el-button size="small" @click="sniSettings.proxy_protocol_rules.push('')">+ {{ t('common.add') || '添加' }}</el-button>
          <el-button size="small" type="success" @click="saveSNISettings" style="margin-left:8px">{{ t('common.save') }}</el-button>
        </div>
        <el-table :data="sniRules" stripe size="small">
          <el-table-column prop="domain" label="SNI Domain" min-width="160" />
          <el-table-column label="Target" min-width="160">
            <template #default="{row}">{{ row.sni_target || row.upstream || '-' }}</template>
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
        <div style="margin-top:12px;font-size:12px;color:#909399">
          {{ t('proxy.sniFallback') || '兜底规则' }}: 127.0.0.1:{{ sniSettings.local_port || '8443' }}
        </div>
      </template>
    </el-card>

    <!-- L4 Routing -->
    <el-card style="margin-bottom:20px">
      <template #header>
        <div style="display:flex;justify-content:space-between;align-items:center">
          <span>L4 {{ t('proxy.routing') || '转发' }}</span>
          <el-button size="small" @click="showAddSite('l4')">+ L4</el-button>
        </div>
      </template>
      <el-table :data="l4Sites" stripe size="small">
        <el-table-column :label="t('common.status')" width="70">
          <template #default="{row}">
            <el-switch v-model="row.enabled" size="small" @change="toggleSite(row)" />
          </template>
        </el-table-column>
        <el-table-column prop="domain" label="Name" width="120" />
        <el-table-column label="Upstream" min-width="140">
          <template #default="{row}">{{ row.upstream || '-' }}</template>
        </el-table-column>
        <el-table-column label="Port" width="80">
          <template #default="{row}">:{{ row.path || '443' }}</template>
        </el-table-column>
        <el-table-column label="Protocol" width="80">
          <template #default="{row}">{{ (row.l4_protocol || 'tcp').toUpperCase() }}</template>
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
            <el-button size="small" type="primary" @click="showPanelProxy">{{ t('proxy.proxyPanel') }}</el-button>
          </div>
        </div>
      </template>
      <el-table :data="httpSites" stripe>
        <el-table-column :label="t('common.status')" width="70">
          <template #default="{row}">
            <el-switch v-model="row.enabled" size="small" @change="toggleSite(row)" />
          </template>
        </el-table-column>
        <el-table-column prop="domain" :label="t('proxy.domain')" min-width="120" />
        <el-table-column prop="upstream" label="Upstream" min-width="120" />
        <el-table-column prop="path" label="Path" min-width="100">
          <template #default="{row}">
            <span>{{ row.path || '/' }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="template" :label="t('proxy.template')" width="90">
          <template #default="{row}"><el-tag size="small">{{ row.template || 'http' }}</el-tag></template>
        </el-table-column>
        <el-table-column label="TLS" width="60">
          <template #default="{row}">
            <el-tag v-if="row.cert_path" type="success" size="small">✓</el-tag>
            <el-tag v-else type="info" size="small">-</el-tag>
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
    </el-card>

    <!-- Config View Dialog -->
    <el-dialog v-model="configViewVisible" title="Caddyfile" width="700">
      <el-input v-model="configViewContent" type="textarea" :rows="20" readonly style="font-family:monospace;font-size:13px" />
      <template #footer>
        <el-button @click="copyConfig">{{ t('proxy.copyConfig') }}</el-button>
        <el-button @click="configViewVisible=false">{{ t('common.close') }}</el-button>
      </template>
    </el-dialog>

    <!-- SNI Dialog -->
    <el-dialog v-model="sniDialogVisible" :title="t('proxy.addSNIRule')" width="500">
      <el-form :model="sniForm" label-width="120px">
        <el-form-item label="Domain">
          <el-input v-model="sniForm.domain" placeholder="www.example.com" />
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
    <el-dialog v-model="siteDialogVisible" :title="siteDialogTitle" width="600">
      <div style="margin-bottom:12px">
        <el-radio-group v-model="siteForm.editMode" size="small">
          <el-radio-button value="form">{{ t('proxy.formEdit') }}</el-radio-button>
          <el-radio-button value="text">{{ t('proxy.textEdit') }}</el-radio-button>
        </el-radio-group>
      </div>
      <el-form v-if="siteForm.editMode==='form'" :model="siteForm" label-width="100px">
        <el-form-item label="Domain">
          <el-input v-model="siteForm.domain" placeholder="example.com" />
        </el-form-item>
        <el-form-item label="Upstream">
          <div v-for="(up, idx) in siteForm.upstreams" :key="idx" style="display:flex;gap:8px;margin-bottom:6px;align-items:center">
            <el-input v-model="siteForm.upstreams[idx]" placeholder="127.0.0.1:8080" style="flex:1" />
            <el-button v-if="siteForm.upstreams.length > 1" size="small" type="danger" plain @click="siteForm.upstreams.splice(idx, 1)">-</el-button>
          </div>
          <el-button size="small" @click="siteForm.upstreams.push('')">+ {{ t('common.add') || '添加' }}</el-button>
        </el-form-item>
        <el-form-item v-if="siteForm.template==='l4'" label="Port">
          <el-input v-model="siteForm.path" placeholder="443" />
        </el-form-item>
        <el-form-item v-else label="Path">
          <el-input v-model="siteForm.path" placeholder="/api (留空转发全部)" />
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
        <el-form-item v-if="siteForm.template!=='sni' && siteForm.template!=='l4'" :label="t('proxy.template')">
          <el-select v-model="siteForm.template" style="width:100%">
            <el-option label="HTTP" value="http" />
            <el-option label="WebSocket" value="websocket" />
            <el-option label="gRPC" value="grpc" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="siteForm.template!=='sni' && siteForm.template!=='l4'" label="lb_policy">
          <el-select v-model="siteForm.lb_policy" style="width:100%" clearable placeholder="留空使用默认">
            <el-option label="round_robin (轮询)" value="round_robin" />
            <el-option label="least_conn (最少连接)" value="least_conn" />
            <el-option label="ip_hash (IP哈希)" value="ip_hash" />
            <el-option label="first (第一个)" value="first" />
            <el-option label="random (随机)" value="random" />
            <el-option label="uri (URI哈希)" value="uri" />
            <el-option label="cookie (Cookie)" value="cookie" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('proxy.proxyProtocol')">
          <el-switch v-model="siteForm.proxy_protocol" />
        </el-form-item>
        <el-form-item :label="t('proxy.useCertificate')">
          <el-switch v-model="siteForm.use_cert" @change="onToggleCert" />
        </el-form-item>
        <el-form-item v-if="siteForm.use_cert" :label="t('certificate.tlsCert')">
          <el-select v-model="siteForm.cert_id" style="width:100%" @change="onSiteCertSelect" :placeholder="t('proxy.selectCert')">
            <el-option v-for="c in certificates" :key="c.id" :label="`${c.domain} (${c.issuer})`" :value="c.id" />
          </el-select>
        </el-form-item>
        <el-divider content-position="left">{{ t('proxy.logSettings') || '日志' }}</el-divider>
        <el-form-item :label="t('proxy.enableLog') || '启用日志'">
          <el-switch v-model="siteForm.log_enabled" />
        </el-form-item>
        <el-form-item v-if="siteForm.log_enabled" :label="t('proxy.logPath') || '日志路径'">
          <el-input v-model="siteForm.log_path" placeholder="logs/example.com.log" />
        </el-form-item>
        <el-form-item v-if="siteForm.log_enabled" :label="t('proxy.logFormat') || '格式'">
          <el-select v-model="siteForm.log_format" style="width:100%">
            <el-option label="json" value="json" />
            <el-option label="console" value="console" />
          </el-select>
        </el-form-item>
      </el-form>
      <div v-else>
        <div style="font-size:12px;color:#909399;margin-bottom:8px">{{ t('proxy.configPreview') || '配置预览' }}</div>
        <el-input :model-value="generatePreview(siteForm)" type="textarea" :rows="10" readonly style="font-family:monospace;font-size:13px;background:#f5f7fa" />
        <div style="margin-top:12px">
          <div style="font-size:12px;color:#909399;margin-bottom:4px">{{ t('proxy.proxyConfig') || 'reverse_proxy 内部指令' }}</div>
          <el-input v-model="siteForm.proxy_config" type="textarea" :rows="3" style="font-family:monospace;font-size:13px" placeholder="lb_policy least_conn&#10;health_uri /health&#10;health_interval 10s" />
        </div>
        <div style="margin-top:12px">
          <div style="font-size:12px;color:#909399;margin-bottom:4px">{{ t('proxy.customConfig') || '站点级指令 (reverse_proxy 外部)' }}</div>
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

const { t } = useI18n()
const emit = defineEmits(['back'])

const proxyInfo = ref({})
const guardian = ref({})
const guardianConfig = ref({ autoRestart: true, maxRestarts: 5 })
const allSites = ref([])
const certificates = ref([])
const caddyLogMode = ref('file')
const logModeSaving = ref(false)
const caddyLogModeHint = computed(() => {
  const m = caddyLogMode.value
  if (m === 'stdout') return t('proxy.caddyHintStdout') || '所有站点日志输出到标准输出'
  if (m === 'off') return t('proxy.caddyHintLogOff') || '不生成任何日志配置（Caddy 默认行为）'
  return t('proxy.caddyHintFile') || '每个站点日志写入到各自的文件路径'
})

const httpSites = computed(() => allSites.value.filter(s => s.template !== 'sni' && s.template !== 'l4'))
const l4Sites = computed(() => allSites.value.filter(s => s.template === 'l4'))
const sniRules = computed(() => allSites.value.filter(s => s.template === 'sni'))

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
const siteForm = ref({ domain: '', upstream: '', path: '', template: 'http', use_cert: false, cert_id: null, cert_path: '', key_path: '', custom_config: '', l4_protocol: 'tcp', sni_target: '', proxy_protocol: false, is_panel: false, editMode: 'form' })

// Panel port
const panelPort = ref('8080')

// Config view
const configViewVisible = ref(false)
const configViewContent = ref('')

// SNI
const sniSettings = ref({ enabled: false, listen_port: '443', local_port: '8443', proxy_protocol_rules: ['127.0.0.0/8', '::1'] })
const sniDialogVisible = ref(false)
const sniForm = ref({ domain: '', sni_target: '', proxy_protocol: false })

function formatSize(bytes) {
  if (!bytes) return ''
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1048576) return (bytes / 1024).toFixed(0) + ' KB'
  return (bytes / 1048576).toFixed(1) + ' MB'
}

async function fetchProxyInfo() {
  try {
    const status = await client.get('/proxy/status')
    proxyInfo.value = status.caddy || {}
    guardian.value = status.caddy_guardian || {}
    guardianConfig.value.autoRestart = guardian.value.auto_restart !== false
    guardianConfig.value.maxRestarts = guardian.value.max_restarts || 5
    if (status.panel_port) panelPort.value = status.panel_port
  } catch (e) {}
}

async function saveGuardianConfig() {
  try {
    await client.post('/proxy/guardian?type=caddy', {
      auto_restart: guardianConfig.value.autoRestart,
      max_restarts: guardianConfig.value.maxRestarts
    })
    fetchProxyInfo()
  } catch (e) {}
}

function formatTime(ts) {
  if (!ts || ts === '0001-01-01T00:00:00Z') return '-'
  return new Date(ts).toLocaleString()
}

async function fetchSites() {
  try {
    allSites.value = await client.get('/caddy/sites') || []
  } catch (e) { allSites.value = [] }
  // Load log output mode
  try {
    const cfg = await client.get('/log-manage/log-output')
    if (cfg?.caddy?.mode) caddyLogMode.value = cfg.caddy.mode
  } catch (e) {}
}

async function saveCaddyLogMode() {
  logModeSaving.value = true
  try {
    const current = await client.get('/log-manage/log-output')
    await client.put('/log-manage/log-output', {
      nginx: current?.nginx || { mode: 'file' },
      caddy: { mode: caddyLogMode.value },
    })
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
  try { certificates.value = (await client.get('/certificates', { params: { limit: 100 } })).items || [] } catch (e) {}
}

async function fetchSNISettings() {
  try {
    const data = await client.get('/caddy/sni-settings')
    if (data) sniSettings.value = data
  } catch (e) {}
}

async function saveSNISettings() {
  try {
    await client.post('/caddy/sni-settings', sniSettings.value)
    ElMessage.success(t('common.success'))
  } catch (e) {}
}

async function onSNIToggle(enabled) {
  try {
    sniSettings.value.enabled = enabled
    await client.post('/caddy/sni-settings', sniSettings.value)
    if (enabled) {
      // Re-enable preserved SNI rules
      await client.post('/caddy/sni-rules/toggle', { enabled: true })
      // Create fallback rule if none exist
      const hasFallback = sniRules.value.some(s => s.domain === 'fallback' || s.domain === '')
      if (!hasFallback) {
        const localPort = sniSettings.value.local_port || '8443'
        await client.post('/caddy/sites', {
          domain: 'fallback', upstream: '', template: 'sni',
          sni_target: `127.0.0.1:${localPort}`, local_port: localPort,
          proxy_protocol: true, is_panel: false, priority: 999,
          path: sniSettings.value.listen_port || '443'
        })
      }
      // Batch migrate HTTPS sites
      const port = sniSettings.value.local_port || '8443'
      const res = await client.post('/caddy/batch-update-ports', { port, enable: true })
      ElMessage.success(`SNI 已开启，迁移 ${res.updated || 0} 个站点`)
    } else {
      // Disable SNI rules (keep in DB)
      await client.post('/caddy/sni-rules/toggle', { enabled: false })
      // Restore sites to port 443
      await client.post('/caddy/batch-update-ports', { port: '443', enable: false })
      ElMessage.success('SNI 已关闭，规则已保留（禁用）')
    }
    await applyConfig()
    fetchSites()
  } catch (e) {}
}

async function batchMigrate() {
  const port = sniSettings.value.local_port || '8443'
  try {
    await client.post('/caddy/sni-settings', sniSettings.value)
    if (sniRules.value.length === 0) {
      await client.post('/caddy/sites', {
        domain: 'fallback', upstream: '', template: 'sni',
        sni_target: `127.0.0.1:${port}`, local_port: port,
        proxy_protocol: true, is_panel: false, priority: 999,
        path: sniSettings.value.listen_port || '443'
      })
    }
    const res = await client.post('/caddy/batch-update-ports', { port, enable: true })
    await applyConfig()
    ElMessage.success(`迁移完成，更新 ${res.updated || 0} 个站点`)
    fetchSites()
  } catch (e) {}
}

async function showReleases() {
  releasesVisible.value = true
  releasesLoading.value = true
  try {
    releases.value = await client.get('/proxy/releases', { params: { type: 'caddy' } }) || []
  } catch (e) { releases.value = [] } finally { releasesLoading.value = false }
}

async function downloadVersion(url, tagName) {
  downloading.value = true
  downloadProgress.value = '正在下载 ' + tagName + '...'
  try {
    await client.post('/proxy/download', { type: 'caddy', download_url: url, tag_name: tagName })
    try { await client.post('/proxy/start?type=caddy') } catch {}
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
    await client.post(`/proxy/${action}?type=caddy`)
    ElMessage.success(t('common.success'))
    fetchProxyInfo()
  } catch (e) {}
}

function showAddSite(template) {
  siteDialogTitle.value = t('proxy.addSite')
  siteForm.value = {
    domain: '', upstreams: [''], path: '', template,
    use_cert: false, cert_id: null, cert_path: '', key_path: '',
    proxy_config: '', custom_config: '', lb_policy: '',
    log_enabled: false, log_path: '', log_format: 'json',
    l4_protocol: 'tcp', sni_target: '',
    proxy_protocol: false, is_panel: false, editMode: 'form'
  }
  siteDialogVisible.value = true
}

function showPanelProxy() {
  siteDialogTitle.value = t('proxy.proxyPanel')
  siteForm.value = {
    domain: ':80', upstreams: [`127.0.0.1:${panelPort.value}`], path: '', template: 'http',
    use_cert: false, cert_id: null, cert_path: '', key_path: '',
    proxy_config: '', custom_config: '', lb_policy: '',
    log_enabled: false, log_path: '', log_format: 'json',
    l4_protocol: 'tcp', sni_target: '',
    proxy_protocol: false, is_panel: true, editMode: 'form'
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
  const d = siteForm.value.domain || ''
  const sniPort = sniSettings.value.enabled ? (sniSettings.value.local_port || '8443') : '443'
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
    path: row.path || '', template: row.template || 'http',
    use_cert: !!row.cert_path, cert_id: certId,
    cert_path: row.cert_path || '', key_path: row.key_path || '',
    proxy_config: row.proxy_config || '',
    custom_config: row.custom_config || '',
    lb_policy: row.lb_policy || '',
    log_enabled: row.log_enabled || false,
    log_path: row.log_path || '',
    log_format: row.log_format || 'json',
    l4_protocol: row.l4_protocol || 'tcp',
    sni_target: row.sni_target || '',
    proxy_protocol: row.proxy_protocol || false,
    is_panel: row.is_panel, editMode: 'form'
  }
  siteDialogVisible.value = true
}

function generatePreview(site) {
  let lines = []
  const domain = site.domain || ':80'
  const pp = site.proxy_protocol ? 'proxy_protocol' : ''
  if (site.template === 'l4') {
    const port = site.path || '443'
    const proto = site.l4_protocol || 'tcp'
    const upstream = site.upstream || '127.0.0.1:8080'
    lines.push(`# L4 ${proto.toUpperCase()} forward on :${port}`)
    lines.push(`{`, `\tlayer4 {`, `\t\t:${port} {`, `\t\t\troute {`, `\t\t\t\tproxy ${upstream}`)
    if (pp) lines.push(`\t\t\t\t${pp}`)
    lines.push(`\t\t\t}`, `\t\t}`, `\t}`, `}`)
    return lines.join('\n')
  }
  lines.push(`${domain} {`)
  if (site.cert_path && site.key_path) lines.push(`\ttls ${site.cert_path} ${site.key_path}`)
  const upstreams = (site.upstreams || []).filter(u => u.trim())
  if (upstreams.length > 0) {
    lines.push(`\treverse_proxy {`)
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
    await client.post('/caddy/generate-config')
    if (proxyInfo.value.status === 'running') {
      await client.post('/proxy/reload?type=caddy')
    }
    fetchProxyInfo()
  } catch (e) {}
}

async function saveSite() {
  siteSaving.value = true
  try {
    let domain = siteForm.value.domain
    // Auto-append local port for new HTTPS sites when SNI is active
    if (!siteForm.value.id && sniSettings.value.enabled && siteForm.value.use_cert && siteForm.value.template !== 'l4' && siteForm.value.template !== 'sni') {
      const localPort = sniSettings.value.local_port || '8443'
      if (domain && !domain.includes(':')) {
        domain = domain + ':' + localPort
      }
    }
    const upstream = (siteForm.value.upstreams || []).filter(u => u.trim()).join(',')
    const payload = {
      domain: domain,
      upstream: upstream,
      path: siteForm.value.path || '',
      template: siteForm.value.template,
      cert_path: siteForm.value.use_cert ? siteForm.value.cert_path : '',
      key_path: siteForm.value.use_cert ? siteForm.value.key_path : '',
      proxy_config: siteForm.value.proxy_config || '',
      custom_config: siteForm.value.custom_config || '',
      lb_policy: siteForm.value.lb_policy || '',
      log_enabled: siteForm.value.log_enabled || false,
      log_path: siteForm.value.log_path || '',
      log_format: siteForm.value.log_format || 'json',
      l4_protocol: siteForm.value.l4_protocol || '',
      sni_target: siteForm.value.sni_target || '',
      local_port: sniSettings.value.local_port || '8443',
      proxy_protocol: siteForm.value.proxy_protocol || false,
      is_panel: siteForm.value.is_panel,
      priority: siteForm.value.is_panel ? 0 : 100
    }
    if (siteForm.value.id) {
      await client.put(`/caddy/sites/${siteForm.value.id}`, payload)
    } else {
      await client.post('/caddy/sites', payload)
    }
    await applyConfig()
    ElMessage.success(t('proxy.siteSaved'))
    siteDialogVisible.value = false
    fetchSites()
  } catch (e) {} finally { siteSaving.value = false }
}

async function toggleSite(row) {
  try {
    await client.put(`/caddy/sites/${row.id}`, { enabled: row.enabled })
    await applyConfig()
    fetchSites()
  } catch (e) { row.enabled = !row.enabled }
}

async function deleteSite(row) {
  await ElMessageBox.confirm(t('common.confirm'))
  try {
    await client.delete(`/caddy/sites/${row.id}`)
    await applyConfig()
    ElMessage.success(t('common.success'))
    fetchSites()
  } catch (e) {}
}

async function viewConfig() {
  try {
    const data = await client.get('/proxy/config?type=caddy')
    configViewContent.value = data.config || ''
    configViewVisible.value = true
  } catch (e) {}
}

function copyConfig() {
  navigator.clipboard.writeText(configViewContent.value)
  ElMessage.success(t('proxy.configCopied'))
}

function showAddSNI() {
  sniForm.value = { domain: '', sni_target: '', proxy_protocol: false }
  sniDialogVisible.value = true
}

async function saveSNIRule() {
  try {
    await client.post('/caddy/sites', {
      domain: sniForm.value.domain,
      upstream: '',
      template: 'sni',
      sni_target: sniForm.value.sni_target,
      local_port: sniSettings.value.local_port || '8443',
      proxy_protocol: sniForm.value.proxy_protocol,
      is_panel: false,
      priority: 50,
      path: sniSettings.value.listen_port || '443'
    })
    await applyConfig()
    ElMessage.success(t('common.success'))
    sniDialogVisible.value = false
    fetchSites()
  } catch (e) {}
}

onMounted(() => {
  fetchProxyInfo()
  fetchSites()
  fetchCertificates()
  fetchSNISettings()
})
</script>
