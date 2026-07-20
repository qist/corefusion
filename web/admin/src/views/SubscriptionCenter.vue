<template>
  <div>
    <h2 style="margin-bottom:20px">{{ t('menu.subscriptions') }}</h2>
    <el-tabs v-model="activeTab">
      <!-- Subscriptions -->
      <el-tab-pane :label="t('subscription.authTitle')" name="subscriptions">
        <el-card>
          <el-table :data="subscriptions" v-loading="subLoading" stripe>
            <el-table-column prop="id" :label="t('common.id')" width="60" />
            <el-table-column :label="t('subscription.clientEmail')" width="180">
              <template #default="{row}">{{ clientEmail(row.client_id) }}</template>
            </el-table-column>
            <el-table-column :label="t('subscription.token')" width="160"><template #default="{row}"><span style="font-family:monospace;font-size:12px">{{ row.token_masked }}</span></template></el-table-column>
            <el-table-column :label="t('common.status')" width="80"><template #default="{row}"><el-tag :type="{active:'success',revoked:'danger',expired:'warning'}[row.status]" size="small">{{ row.status }}</el-tag></template></el-table-column>
            <el-table-column :label="t('subscription.quotaState')" width="100"><template #default="{row}"><el-tag :type="quotaType(row.quota_state)" size="small">{{ row.quota_state }}</el-tag></template></el-table-column>
            <el-table-column :label="`${t('subscription.usedBytes')}/${t('subscription.quotaBytes')}`" width="140"><template #default="{row}">{{ fmtBytes(row.used_bytes_cache) }} / {{ row.quota_bytes ? fmtBytes(row.quota_bytes) : t('common.unlimited') }}</template></el-table-column>
            <el-table-column :label="t('common.actions')" width="240">
              <template #default="{row}">
                <el-button size="small" @click="handleRotate(row)" :disabled="row.status==='revoked'">{{ t('subscription.rotate') }}</el-button>
                <el-button size="small" type="danger" @click="handleRevoke(row)" :disabled="row.status==='revoked'">{{ t('subscription.revoke') }}</el-button>
                <el-button v-if="row.status==='revoked'" size="small" type="danger" @click="handleDelete(row)">{{ t('common.delete') }}</el-button>
              </template>
            </el-table-column>
          </el-table>
          <el-pagination v-model:current-page="subPage" :page-size="20" :total="subTotal" layout="total,prev,pager,next" style="margin-top:16px;justify-content:flex-end" @current-change="fetchSubscriptions" />
        </el-card>
      </el-tab-pane>

      <!-- Subscription Formats -->
      <el-tab-pane :label="t('subfmt.title')" name="formats">
        <!-- Format Type Selection -->
        <div v-if="!selectedFormatType" style="display:grid;grid-template-columns:repeat(3,1fr);gap:16px;max-width:800px;margin:40px auto">
          <el-card v-for="ft in formatTypes" :key="ft.type" shadow="hover" style="cursor:pointer;text-align:center" @click="selectFormatType(ft.type)">
            <div style="font-size:32px;margin-bottom:12px">{{ ft.icon }}</div>
            <div style="font-size:16px;font-weight:500;margin-bottom:8px">{{ ft.name }}</div>
            <div style="font-size:12px;color:#909399">{{ ft.desc }}</div>
            <el-tag size="small" style="margin-top:8px">{{ ft.count }} {{ t('subfmt.configs') }}</el-tag>
          </el-card>
        </div>

        <!-- Format Management -->
        <div v-else>
          <div style="display:flex;justify-content:space-between;margin-bottom:16px">
            <div style="display:flex;align-items:center;gap:8px">
              <el-button size="small" @click="selectedFormatType=''">{{ t('common.back') }}</el-button>
              <h3 style="margin:0">{{ selectedFormatTypeIcon }} {{ selectedFormatTypeName }}</h3>
            </div>
            <div style="display:flex;gap:8px">
              <el-button v-if="selectedFormatType==='mihomo_yaml'" size="small" @click="showImportYAML">{{ t('subfmt.importYAML') }}</el-button>
              <el-button type="primary" size="small" @click="showCreateFormat">{{ t('subfmt.create') }}</el-button>
            </div>
          </div>
          <el-card>
            <el-table :data="filteredFormats" v-loading="fmtLoading" stripe size="small">
              <el-table-column prop="id" label="ID" width="50" />
              <el-table-column prop="name" :label="t('subfmt.name')" width="150" />
              <el-table-column prop="description" :label="t('subfmt.description')" show-overflow-tooltip />
              <el-table-column :label="t('subfmt.platform')" width="90"><template #default="{row}"><el-tag v-if="row.platform" size="small" type="info">{{ row.platform }}</el-tag><span v-else style="color:#909399">Universal</span></template></el-table-column>
              <el-table-column :label="t('subfmt.planIds')" width="120"><template #default="{row}"><span v-if="!row.plan_ids || row.plan_ids.length===0" style="color:#909399">All</span><span v-else>{{ row.plan_ids.length }} plans</span></template></el-table-column>
              <el-table-column :label="t('common.status')" width="70"><template #default="{row}"><el-tag :type="row.enabled?'success':'danger'" size="small">{{ row.enabled?'✓':'✗' }}</el-tag></template></el-table-column>
              <el-table-column :label="t('common.actions')" width="200">
                <template #default="{row}">
                  <el-button v-if="selectedFormatType==='mihomo_yaml'" size="small" @click="showFormatRules(row)">{{ t('subfmt.rules') }}</el-button>
                  <el-button size="small" @click="showEditFormat(row)">{{ t('common.edit') }}</el-button>
                  <el-button size="small" type="danger" @click="deleteFormat(row)">{{ t('common.delete') }}</el-button>
                </template>
              </el-table-column>
            </el-table>
          </el-card>
        </div>
      </el-tab-pane>

      <!-- Subscription Endpoints -->
      <el-tab-pane :label="t('subend.title')" name="endpoints">
        <div style="display:flex;justify-content:space-between;margin-bottom:16px">
          <span></span>
          <el-button type="primary" size="small" @click="showCreateEndpoint">{{ t('subend.create') }}</el-button>
        </div>
        <el-card>
          <el-table :data="endpoints" v-loading="epLoading" stripe size="small">
            <el-table-column prop="id" label="ID" width="50" />
            <el-table-column prop="domain" :label="t('subend.domain')" width="200" />
            <el-table-column prop="path" :label="t('subend.path')" width="120"><template #default="{row}">{{ row.path || '/' }}</template></el-table-column>
            <el-table-column prop="description" :label="t('subend.description')" show-overflow-tooltip />
            <el-table-column :label="t('common.status')" width="70"><template #default="{row}"><el-tag :type="row.enabled?'success':'danger'" size="small">{{ row.enabled?'✓':'✗' }}</el-tag></template></el-table-column>
            <el-table-column :label="t('common.actions')" width="140">
              <template #default="{row}">
                <el-button size="small" @click="showEditEndpoint(row)">{{ t('common.edit') }}</el-button>
                <el-button size="small" type="danger" @click="deleteEndpoint(row)">{{ t('common.delete') }}</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>

      <!-- Billing Templates -->
      <el-tab-pane :label="t('subscription.billingTemplates')" name="billingTemplates">
        <SubscriptionBillingTemplates />
      </el-tab-pane>

      <!-- External Nodes -->
      <el-tab-pane :label="t('externalNodes.title')" name="externalNodes">
        <ExternalNodes ref="extNodesRef" />
      </el-tab-pane>
    </el-tabs>

    <!-- Format Dialog -->
    <el-dialog v-model="fmtDialog" :title="editingFormat?t('subfmt.edit'):t('subfmt.create')" width="700">
      <el-tabs v-model="fmtTab">
        <el-tab-pane :label="t('subfmt.basic')" name="basic">
          <el-form :model="fmtForm" label-width="100px">
            <el-form-item :label="t('subfmt.name')"><el-input v-model="fmtForm.name" /></el-form-item>
            <el-form-item :label="t('subfmt.clientType')"><el-select v-model="fmtForm.client_type" style="width:100%"><el-option label="Clash" value="clash" /><el-option label="Shadowrocket" value="shadowrocket" /><el-option label="Sing-Box" value="singbox" /><el-option label="V2Ray" value="v2ray" /></el-select></el-form-item>
            <el-form-item :label="t('subfmt.format')"><el-select v-model="fmtForm.format" style="width:100%"><el-option label="Mihomo YAML" value="mihomo_yaml" /><el-option label="Sing-Box JSON" value="singbox_json" /><el-option label="Xray JSON" value="xray_json" /><el-option label="Base64 URI" value="base64_uri" /><el-option label="Raw URI" value="raw_uri" /></el-select></el-form-item>
            <el-form-item :label="t('subfmt.contentType')"><el-input v-model="fmtForm.content_type" placeholder="text/yaml; charset=utf-8" /></el-form-item>
            <el-form-item :label="t('subfmt.extension')"><el-input v-model="fmtForm.extension" placeholder=".yaml" /></el-form-item>
            <el-form-item :label="t('subfmt.description')"><el-input v-model="fmtForm.description" type="textarea" :rows="2" /></el-form-item>
            <el-form-item :label="t('subfmt.platform')">
              <el-select v-model="fmtForm.platform" style="width:100%" clearable>
                <el-option label="Universal" value="" />
                <el-option label="Windows" value="win" />
                <el-option label="macOS" value="mac" />
                <el-option label="Linux" value="linux" />
                <el-option label="Android" value="android" />
                <el-option label="iOS" value="ios" />
                <el-option label="Router" value="router" />
              </el-select>
            </el-form-item>
            <el-form-item :label="t('subfmt.planIds')">
              <el-select v-model="fmtForm.plan_ids" style="width:100%" multiple clearable placeholder="Universal">
                <el-option v-for="p in allPlans" :key="p.id" :label="p.name" :value="p.id" />
              </el-select>
            </el-form-item>
            <el-form-item :label="t('common.enabled')"><el-switch v-model="fmtForm.enabled" /></el-form-item>
          </el-form>
        </el-tab-pane>
        <!-- Mihomo Config (component) -->
        <el-tab-pane v-if="fmtForm.format==='mihomo_yaml'" :label="t('subfmt.mihomoConfig')" name="mihomo">
          <MihomoConfig
            v-model="generalConfig"
            :hosts="hosts"
            :tunnels="tunnels"
            :proxy-groups="proxyGroups"
            :rule-providers="ruleProviders"
            :proxy-providers="proxyProviders"
            :dns="dnsConfig"
            @update:hosts="val => hosts = val"
            @update:tunnels="val => tunnels = val"
            @update:proxy-groups="val => proxyGroups = val"
            @update:rule-providers="val => ruleProviders = val"
            @update:proxy-providers="val => proxyProviders = val"
            @update:dns="val => dnsConfig = val"
          />
        </el-tab-pane>
      </el-tabs>
      <template #footer><el-button @click="fmtDialog=false">{{ t('common.cancel') }}</el-button><el-button type="primary" @click="saveFormat">{{ t('common.save') }}</el-button></template>
    </el-dialog>

    <!-- Import YAML Dialog -->
    <el-dialog v-model="importDialog" :title="t('subfmt.importYAML')" width="700">
      <el-alert type="info" :closable="false" style="margin-bottom:16px">
        <template #title>
          <div style="font-size:13px">
            <div style="font-weight:500;margin-bottom:8px">{{ t('subfmt.importFilter') }}:</div>
            <div>• {{ t('subfmt.importFilterProxies') }}</div>
            <div>• {{ t('subfmt.importFilterProxyGroupsUse') }}</div>
            <div>• {{ t('subfmt.importFilterProxyGroupsNodes') }}</div>
            <div style="margin-top:8px">{{ t('subfmt.importKeep') }}:</div>
            <div>• {{ t('subfmt.importKeepGeneral') }}</div>
            <div>• {{ t('subfmt.importKeepDNS') }}</div>
            <div>• {{ t('subfmt.importKeepHosts') }}</div>
            <div>• {{ t('subfmt.importKeepRuleProviders') }}</div>
            <div>• {{ t('subfmt.importKeepProxyGroups') }}</div>
            <div>• {{ t('subfmt.importKeepRules') }}</div>
            <div>• {{ t('subfmt.importKeepSpecial') }}</div>
          </div>
        </template>
      </el-alert>
      <div style="margin-bottom:12px">
        <el-upload
          :auto-upload="false"
          :show-file-list="false"
          accept=".yaml,.yml,.txt"
          @change="handleFileUpload"
        >
          <el-button size="small">{{ t('subfmt.importUpload') }}</el-button>
        </el-upload>
      </div>
      <el-input v-model="importYAMLContent" type="textarea" :rows="15" :placeholder="t('subfmt.importPlaceholder')" />
      <template #footer>
        <el-button @click="importDialog=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="importing" @click="handleImportYAML">{{ t('subfmt.importBtn') }}</el-button>
      </template>
    </el-dialog>

    <!-- Format Rules Dialog -->
    <el-dialog v-model="rulesDialog" :title="currentFormat?.name + ' - ' + t('subfmt.rules')" width="700">
      <div style="margin-bottom:12px;display:flex;gap:8px;flex-wrap:wrap;align-items:center">
        <el-select v-model="newRuleType" size="small" style="width:150px" @change="onRuleTypeChange">
          <el-option-group label="域名">
            <el-option label="DOMAIN" value="DOMAIN" /><el-option label="DOMAIN-SUFFIX" value="DOMAIN-SUFFIX" />
            <el-option label="DOMAIN-KEYWORD" value="DOMAIN-KEYWORD" /><el-option label="DOMAIN-WILDCARD" value="DOMAIN-WILDCARD" />
            <el-option label="DOMAIN-REGEX" value="DOMAIN-REGEX" />
          </el-option-group>
          <el-option-group label="IP">
            <el-option label="IP-CIDR" value="IP-CIDR" /><el-option label="IP-CIDR6" value="IP-CIDR6" />
            <el-option label="IP-ASN" value="IP-ASN" /><el-option label="GEOIP" value="GEOIP" />
            <el-option label="GEOSITE" value="GEOSITE" />
          </el-option-group>
          <el-option-group label="规则集">
            <el-option label="RULE-SET" value="RULE-SET" />
          </el-option-group>
          <el-option-group label="网络">
            <el-option label="DST-PORT" value="DST-PORT" /><el-option label="SRC-PORT" value="SRC-PORT" />
            <el-option label="NETWORK" value="NETWORK" />
          </el-option-group>
          <el-option-group label="进程">
            <el-option label="PROCESS-NAME" value="PROCESS-NAME" /><el-option label="PROCESS-PATH" value="PROCESS-PATH" />
          </el-option-group>
          <el-option-group label="逻辑">
            <el-option label="MATCH" value="MATCH" />
          </el-option-group>
        </el-select>
        <el-select v-if="newRuleType==='RULE-SET'" v-model="newRuleProvider" size="small" filterable style="width:150px" :placeholder="t('subfmt.selectRuleProvider')">
          <el-option v-for="(_, name) in ruleProviders" :key="name" :label="name" :value="name" />
        </el-select>
        <el-input v-if="newRuleType!=='RULE-SET'" v-model="newRuleValue" size="small" placeholder="cn, private" style="width:130px" />
        <el-select v-model="newRuleOutbound" size="small" allow-create filterable style="width:100px">
          <el-option label="PROXY" value="PROXY" /><el-option label="DIRECT" value="DIRECT" /><el-option label="BLOCK" value="BLOCK" />
        </el-select>
        <el-button size="small" type="primary" @click="addRule">{{ t('subfmt.addRule') }}</el-button>
      </div>
      <el-table :data="formatRules" stripe size="small">
        <el-table-column prop="priority" label="#" width="50" />
        <el-table-column :label="t('subfmt.ruleType')" width="140">
          <template #default="{row}">
            <el-tag v-if="row.rule_provider" size="small" type="warning">RULE-SET</el-tag>
            <el-tag v-else size="small" type="info">{{ row.rule_type }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('subfmt.matchValue')">
          <template #default="{row}">{{ row.rule_provider || row.match_value }}</template>
        </el-table-column>
        <el-table-column prop="outbound_tag" :label="t('subfmt.outboundTag')" width="100"><template #default="{row}"><el-tag size="small">{{ row.outbound_tag }}</el-tag></template></el-table-column>
        <el-table-column :label="t('common.actions')" width="70"><template #default="{row}"><el-button size="small" type="danger" @click="deleteRule(row)">{{ t('common.delete') }}</el-button></template></el-table-column>
      </el-table>
    </el-dialog>

    <!-- Endpoint Dialog -->
    <el-dialog v-model="epDialog" :title="editingEndpoint?t('subend.edit'):t('subend.create')" width="450">
      <el-form :model="epForm" label-width="100px">
        <el-form-item :label="t('subend.domain')"><el-input v-model="epForm.domain" placeholder="sub.example.com" /></el-form-item>
        <el-form-item :label="t('subend.path')"><el-input v-model="epForm.path" placeholder="/ 或 /sub" /><div style="font-size:12px;color:#909399;margin-top:4px">{{ t('subend.pathHint') }}</div></el-form-item>
        <el-form-item :label="t('subend.description')"><el-input v-model="epForm.description" type="textarea" :rows="2" /></el-form-item>
        <el-form-item :label="t('common.enabled')"><el-switch v-model="epForm.enabled" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="epDialog=false">{{ t('common.cancel') }}</el-button><el-button type="primary" @click="saveEndpoint">{{ t('common.save') }}</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { load as yamlLoad } from 'js-yaml'
import client from '../api/client'
import MihomoConfig from './protocols/sections/MihomoConfig.vue'
import ExternalNodes from './ExternalNodes.vue'
import SubscriptionBillingTemplates from '../components/subscription/SubscriptionBillingTemplates.vue'

const { t } = useI18n()
const activeTab = ref('subscriptions')

// Subscriptions
const subscriptions = ref([])
const clientsList = ref([])
const subLoading = ref(false)
const subTotal = ref(0)
const subPage = ref(1)

// Formats
const formats = ref([])
const fmtLoading = ref(false)
const selectedFormatType = ref('')

const formatTypes = [
  { type: 'mihomo_yaml', name: 'Mihomo', icon: '🐱', desc: 'Clash / Mihomo 内核', count: 0 },
  { type: 'singbox_json', name: 'Sing-Box', icon: '📦', desc: 'Sing-Box 内核', count: 0 },
  { type: 'xray_json', name: 'V2Ray', icon: '🚀', desc: 'V2Ray / Xray 内核', count: 0 }
]

const selectedFormatTypeName = computed(() => formatTypes.find(f => f.type === selectedFormatType.value)?.name || '')
const selectedFormatTypeIcon = computed(() => formatTypes.find(f => f.type === selectedFormatType.value)?.icon || '')

const filteredFormats = computed(() => formats.value.filter(f => f.format === selectedFormatType.value))

function selectFormatType(type) {
  selectedFormatType.value = type
}
const fmtDialog = ref(false)
const editingFormat = ref(null)
const fmtForm = ref({ name: '', client_type: 'clash', format: 'mihomo_yaml', content_type: '', extension: '', description: '', platform: '', plan_ids: [], enabled: true, listeners_json: '', proxy_groups_json: '', rule_providers_json: '', proxy_providers_json: '', dns_json: '', general_json: '' })
const allPlans = ref([])
const fmtTab = ref('basic')
const hosts = ref([])
const tunnels = ref([])
const proxyGroups = ref([])
const ruleProviders = ref({})
const proxyProviders = ref({})
const dnsConfig = ref({ enable: true, _nameserverStr: '8.8.8.8\n1.1.1.1', _fallbackStr: '' })
const extNodesRef = ref(null)
const generalConfig = ref({
  mixed_port: 10801, port: 0, socks_port: 0, redir_port: 0, tproxy_port: 0,
  allow_lan: true, bind_address: '*',
  _authStr: '', _skipAuthStr: '127.0.0.1/8\n::1/128',
  _lanAllowedStr: '0.0.0.0/0\n::/0', _lanDisallowedStr: '',
  find_process_mode: 'strict', mode: 'rule', log_level: 'info'
})
const newRPName = ref('')
const newPPName = ref('')
const rulesDialog = ref(false)
const importDialog = ref(false)
const importYAMLContent = ref('')
const importing = ref(false)
const currentFormat = ref(null)
const formatRules = ref([])
const newRuleType = ref('domain_suffix')
const newRuleValue = ref('')
const newRuleProvider = ref('')
const newRuleOutbound = ref('DIRECT')

// Endpoints
const endpoints = ref([])
const epLoading = ref(false)
const epDialog = ref(false)
const editingEndpoint = ref(null)
const epForm = ref({ domain: '', path: '/', description: '', enabled: true })

// Helpers
const quotaType = (s) => ({ normal: 'success', near_limit: 'warning', over_limit_soft: 'danger', over_limit_hard: 'danger', suspended: 'danger', expired_days: 'warning' }[s] || 'info')
function fmtBytes(b) { if (!b) return '0 B'; const u = ['B', 'KB', 'MB', 'GB', 'TB']; let i = 0; while (b >= 1024 && i < u.length - 1) { b /= 1024; i++ } return b.toFixed(1) + ' ' + u[i] }
const clientEmail = (id) => clientsList.value.find(c => c.id === id)?.email || `#${id}`

// Fetch
async function fetchSubscriptions() {
  subLoading.value = true
  try {
    const [subRes, clRes] = await Promise.all([
      client.get('/subscriptions', { params: { limit: 20, offset: (subPage.value - 1) * 20 } }),
      client.get('/clients', { params: { limit: 100 } })
    ])
    subscriptions.value = subRes.items || []
    subTotal.value = subRes.total || 0
    clientsList.value = clRes.items || []
  } finally { subLoading.value = false }
}
async function fetchPlans() { try { allPlans.value = (await client.get('/plan-templates')) || [] } catch { allPlans.value = [] } }

async function fetchFormats() { fmtLoading.value = true; try { formats.value = (await client.get('/mihomo/configs')) || [] } catch {} finally { fmtLoading.value = false } }
async function fetchEndpoints() { epLoading.value = true; try { endpoints.value = (await client.get('/subscription-endpoints')) || [] } catch {} finally { epLoading.value = false } }

// Subscription actions
async function handleRotate(row) { await ElMessageBox.confirm(t('subscription.rotateConfirm'), t('common.confirm')); const r = await client.post(`/subscriptions/${row.id}/rotate`, { reason: 'manual' }); ElMessage.success(`${t('subscription.newToken')}: ${r.token}`); fetchSubscriptions() }
async function handleRevoke(row) { await ElMessageBox.confirm(t('subscription.revokeConfirm'), t('common.confirm')); await client.post(`/subscriptions/${row.id}/revoke`); ElMessage.success(t('common.success')); fetchSubscriptions() }
async function handleDelete(row) { await ElMessageBox.confirm(t('subscription.deleteConfirm'), t('common.confirm')); await client.delete(`/subscriptions/${row.id}`); ElMessage.success(t('common.success')); fetchSubscriptions() }

// Format CRUD
function showCreateFormat() {
  editingFormat.value = null
  fmtForm.value = { name: '', client_type: 'clash', format: 'mihomo_yaml', content_type: '', extension: '', description: '', platform: '', plan_ids: [], enabled: true, listeners_json: '', proxy_groups_json: '', rule_providers_json: '', proxy_providers_json: '', dns_json: '', general_json: '' }
  hosts.value = []
  tunnels.value = []
  proxyGroups.value = [{ name: 'PROXY', type: 'select', proxies: [], _proxiesStr: 'DIRECT', _showAdvanced: false, hidden: false, lazy: true, url: 'https://www.gstatic.com/generate_204', interval: 300, timeout: 5000, 'max-failed-times': 5, 'empty-fallback': '' }]
  ruleProviders.value = {}
  proxyProviders.value = {}
  dnsConfig.value = { enable: true, _listen: '0.0.0.0:53', _enhancedMode: 'redir-host', _cacheAlgorithm: 'arc', _preferH3: false, _ipv6: true, _useHosts: true, _respectRules: false, _defaultNameserverStr: '114.114.114.114\n8.8.8.8', _nameserverStr: 'https://doh.pub/dns-query\nhttps://dns.alidns.com/dns-query', _fallbackEnabled: false, _fallbackStr: '', _proxyServerNSEnabled: false, _proxyServerNSStr: '', _directNSEnabled: false, _directNSStr: '', _fakeIPRange: '198.18.0.1/16', _fakeIPFilterMode: 'blacklist', _fakeIPFilterEnabled: true, _fakeIPFilterStr: '*.lan', _fallbackGeoIP: true, _fallbackGeoCode: 'CN', _fallbackIPCIDREnabled: false, _fallbackIPCIDRStr: '', _fallbackDomainEnabled: false, _fallbackDomainStr: '', _nsPolicyEnabled: false, _nsPolicyJSON: '{}', _nsPolicyStr: '' }
  generalConfig.value = { mixed_port: 10801, _mixedPortEnabled: true, port: 0, _portEnabled: false, socks_port: 0, _socksPortEnabled: false, redir_port: 0, _redirPortEnabled: false, tproxy_port: 0, _tproxyPortEnabled: false, allow_lan: true, bind_address: '*', _authEnabled: false, _authStr: '', _skipAuthEnabled: false, _skipAuthStr: '127.0.0.1/8\n::1/128', _lanAllowedEnabled: false, _lanAllowedStr: '0.0.0.0/0\n::/0', _lanDisallowedEnabled: false, _lanDisallowedStr: '', find_process_mode: 'strict', mode: 'rule', log_level: 'info', ipv6: true, _storeSelected: false, _storeFakeIP: true, _geoAutoUpdate: false, _geoUpdateInterval: 24, _geoIPEnabled: false, _geoIPUrl: 'https://fastly.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geoip.dat', _geositeEnabled: false, _geositeUrl: 'https://fastly.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geosite.dat', _mmdbEnabled: false, _mmdbUrl: 'https://fastly.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geoip.metadb', _extControllerEnabled: true, _extController: '0.0.0.0:9093', _extControllerTLS: '', _extControllerTLSEnabled: false, _extSecretEnabled: false, _extSecret: '', _extUIEnabled: false, _extUIName: 'metacubexd', _extUIUrl: '', _tlsEnabled: false, _tlsCert: '', _tlsKey: '', _interfaceEnabled: false, _interfaceName: '', _routingMarkEnabled: false, _routingMark: 0, _tcpConcurrent: true, _keepAliveEnabled: false, _keepAliveIdle: 15, _quicDisableGso: false, _quicDisableEcn: false, _dialerIP4PConvert: false, _tunEnabled: false, _tunStack: 'system', _tunDnsHijack: '0.0.0.0:53', _tunAutoRoute: true, _tunAutoRedirect: false, _tunMTUEnabled: false, _tunMTU: 9000, _tunStrictRoute: false, _tunDisableICMP: false, _tunDevice: '', _tunAutoDetect: true, _tunEndpointNAT: false, _tunGSO: false, _tunGSOMaxSize: 65536, _tunIPv6Enabled: false, _tunIPv6Addr: '', _tunUDPTimeoutEnabled: false, _tunUDPTimeout: 300, _tunRouteAddrV4Enabled: false, _tunRouteAddrV4: '0.0.0.0/1\n128.0.0.0/1', _tunRouteAddrV6Enabled: false, _tunRouteAddrV6: '::/1\n8000::/1', _tunRouteExcludeV4Enabled: false, _tunRouteExcludeV4: '192.168.0.0/16', _tunRouteExcludeV6Enabled: false, _tunRouteExcludeV6: 'fc00::/7', _tunRouteAddrSetEnabled: false, _tunRouteAddrSet: '', _tunRouteExcludeSetEnabled: false, _tunRouteExcludeSet: '', _tunIncludeIfEnabled: false, _tunIncludeIf: '', _tunExcludeIfEnabled: false, _tunExcludeIf: '', _tunIncludeUIDEnabled: false, _tunIncludeUID: '', _tunExcludeUIDEnabled: false, _tunExcludeUID: '', _tunIncludeUIDRangeEnabled: false, _tunIncludeUIDRange: '', _tunExcludeUIDRangeEnabled: false, _tunExcludeUIDRange: '', _tunIncludeMACEnabled: false, _tunIncludeMAC: '', _tunExcludeMACEnabled: false, _tunExcludeMAC: '', _tunIncludeAndroidUserEnabled: false, _tunIncludeAndroidUser: '', _tunIncludePkgEnabled: false, _tunIncludePkg: '', _tunExcludePkgEnabled: false, _tunExcludePkg: '', _snifferEnabled: false, _snifferForceDNS: false, _snifferParsePureIP: false, _snifferOverrideDest: false, _snifferHTTP: true, _snifferHTTPPorts: '80, 8080-8880', _snifferHTTPOverride: false, _snifferTLS: true, _snifferTLSPorts: '443, 8443', _snifferQUIC: true, _snifferQUICPorts: '443, 8443', _snifferForceDomainEnabled: false, _snifferForceDomain: '', _snifferSkipDomainEnabled: false, _snifferSkipDomain: '', _snifferSkipSrcEnabled: false, _snifferSkipSrcAddr: '', _snifferSkipDstEnabled: false, _snifferSkipDstAddr: '', _dnsOutEnabled: true, _rematchOutEnabled: false, _rematchTarget: '', _directOutEnabled: false, _directInterface: '', _directRoutingMark: 0 }
  fmtTab.value = 'basic'
  fmtDialog.value = true
}

function parseRuleString(rule, index) {
  // Handle complex rules with nested parentheses (AND/OR/NOT)
  const complexTypes = ['AND', 'OR', 'NOT', 'SUB-RULE']
  const firstComma = rule.indexOf(',')
  const ruleType = (firstComma > 0 ? rule.substring(0, firstComma) : rule).trim()

  if (complexTypes.includes(ruleType)) {
    // For AND/OR/NOT: last comma-separated part is the outbound
    const lastComma = rule.lastIndexOf(',')
    if (lastComma > firstComma) {
      const condition = rule.substring(firstComma + 1, lastComma).trim()
      const outbound = rule.substring(lastComma + 1).trim()
      return {
        rule_type: ruleType,
        match_value: condition,
        outbound_tag: outbound || 'DIRECT',
        rule_provider: '',
        rule_action: '',
        priority: index,
        enabled: true
      }
    }
  }

  // MATCH: MATCH,outbound
  if (ruleType === 'MATCH') {
    const parts = rule.split(',')
    return {
      rule_type: 'MATCH',
      match_value: '',
      outbound_tag: (parts[1] || 'DIRECT').trim(),
      rule_provider: '',
      rule_action: '',
      priority: index,
      enabled: true
    }
  }

  // RULE-SET: RULE-SET,provider,outbound[,action]
  if (ruleType === 'RULE-SET') {
    const parts = rule.split(',')
    return {
      rule_type: 'RULE-SET',
      match_value: '',
      outbound_tag: (parts[2] || 'DIRECT').trim(),
      rule_provider: (parts[1] || '').trim(),
      rule_action: parts.length > 3 ? parts.slice(3).join(',').trim() : '',
      priority: index,
      enabled: true
    }
  }

  // Standard rules: TYPE,match,outbound[,action]
  const parts = rule.split(',')
  return {
    rule_type: ruleType || 'MATCH',
    match_value: (parts[1] || '').trim(),
    outbound_tag: (parts[2] || 'DIRECT').trim(),
    rule_provider: '',
    rule_action: parts.length > 3 ? parts.slice(3).join(',').trim() : '',
    priority: index,
    enabled: true
  }
}

function buildRuleString(rule) {
  // Build rule string from components
  if (rule.rule_provider) {
    // RULE-SET format
    let str = 'RULE-SET,' + rule.rule_provider + ',' + (rule.outbound_tag || 'DIRECT')
    if (rule.rule_action) str += ',' + rule.rule_action
    return str
  }
  if (rule.rule_type === 'MATCH') {
    return 'MATCH,' + (rule.outbound_tag || 'DIRECT')
  }
  // Complex rules (AND/OR/NOT) - match_value contains the full condition
  if (['AND', 'OR', 'NOT', 'SUB-RULE'].includes(rule.rule_type)) {
    return rule.rule_type + ',' + rule.match_value + ',' + (rule.outbound_tag || 'DIRECT')
  }
  // Standard rules
  let str = rule.rule_type + ',' + (rule.match_value || '') + ',' + (rule.outbound_tag || 'DIRECT')
  if (rule.rule_action) str += ',' + rule.rule_action
  return str
}

function showImportYAML() {
  importYAMLContent.value = ''
  importDialog.value = true
}

function handleFileUpload(file) {
  const reader = new FileReader()
  reader.onload = (e) => {
    importYAMLContent.value = e.target.result
    ElMessage.success(t('subfmt.importFileLoaded'))
  }
  reader.readAsText(file.raw)
}

async function handleImportYAML() {
  if (!importYAMLContent.value.trim()) { ElMessage.error(t('subfmt.importEmpty')); return }
  importing.value = true
  try {
    // Parse YAML using js-yaml
    const parsed = yamlLoad(importYAMLContent.value)

    // Create new format with default name
    editingFormat.value = null
    showCreateFormat()
    fmtForm.value.name = 'Imported Config'
    fmtForm.value.description = 'Imported from YAML'

    // General settings (mihomo uses top-level fields)
    Object.assign(generalConfig.value, {
      mixed_port: parsed['mixed-port'] || 0,
      _mixedPortEnabled: !!parsed['mixed-port'],
      port: parsed['port'] || 0,
      _portEnabled: !!parsed['port'],
      socks_port: parsed['socks-port'] || 0,
      _socksPortEnabled: !!parsed['socks-port'],
      redir_port: parsed['redir-port'] || 0,
      _redirPortEnabled: !!parsed['redir-port'],
      tproxy_port: parsed['tproxy-port'] || 0,
      _tproxyPortEnabled: !!parsed['tproxy-port'],
      allow_lan: parsed['allow-lan'] !== false,
      bind_address: parsed['bind-address'] || '*',
      mode: parsed['mode'] || 'rule',
      log_level: parsed['log-level'] || 'info',
      ipv6: parsed['ipv6'] !== false,
      find_process_mode: parsed['find-process-mode'] || 'strict',
      tcp_concurrent: parsed['tcp-concurrent'] !== false,
      _geoAutoUpdate: parsed['geo-auto-update'] || false,
      _geoUpdateInterval: parsed['geo-update-interval'] || 24,
      // Authentication
      _authEnabled: !!(parsed.authentication && parsed.authentication.length > 0),
      _authStr: (parsed.authentication || []).join('\n'),
      // GeoX URLs
      _geoIPEnabled: !!(parsed['geox-url'] && parsed['geox-url'].geoip),
      _geoIPUrl: (parsed['geox-url'] && parsed['geox-url'].geoip) || '',
      _geositeEnabled: !!(parsed['geox-url'] && parsed['geox-url'].geosite),
      _geositeUrl: (parsed['geox-url'] && parsed['geox-url'].geosite) || '',
      _mmdbEnabled: !!(parsed['geox-url'] && parsed['geox-url'].mmdb),
      _mmdbUrl: (parsed['geox-url'] && parsed['geox-url'].mmdb) || '',
      // Experimental
      _quicDisableGSO: !!(parsed.experimental && parsed.experimental['quic-go-disable-gso']),
      // Keep Alive
      _keepAliveEnabled: !!(parsed['keep-alive-idle'] || parsed['keep-alive-interval']),
      _keepAliveIdle: parsed['keep-alive-idle'] || 15,
    })

    // DNS settings
    const dns = parsed.dns || {}
    Object.assign(dnsConfig.value, {
      enable: dns.enable !== false,
      _listen: dns.listen || '0.0.0.0:53',
      _enhancedMode: dns['enhanced-mode'] || 'redir-host',
      _cacheAlgorithm: dns['cache-algorithm'] || 'arc',
      _preferH3: dns['prefer-h3'] || false,
      _ipv6: dns.ipv6 !== false,
      _useHosts: dns['use-hosts'] !== false,
      _respectRules: dns['respect-rules'] || false,
      _nameserverStr: (dns.nameserver || []).join('\n'),
      _defaultNameserverStr: (dns['default-nameserver'] || []).join('\n'),
      _fallbackEnabled: (dns.fallback || []).length > 0,
      _fallbackStr: (dns.fallback || []).join('\n'),
      _proxyServerNSEnabled: (dns['proxy-server-nameserver'] || []).length > 0,
      _proxyServerNSStr: (dns['proxy-server-nameserver'] || []).join('\n'),
      _directNSEnabled: (dns['direct-nameserver'] || []).length > 0,
      _directNSStr: (dns['direct-nameserver'] || []).join('\n'),
      _fakeIPRange: dns['fake-ip-range'] || '198.18.0.1/16',
      _fakeIPFilterMode: dns['fake-ip-filter-mode'] || 'blacklist',
      _fakeIPFilterEnabled: (dns['fake-ip-filter'] || []).length > 0,
      _fakeIPFilterStr: (dns['fake-ip-filter'] || []).join('\n'),
      _fallbackGeoIP: dns['fallback-filter'] ? dns['fallback-filter'].geoip !== false : true,
      _fallbackGeoCode: (dns['fallback-filter'] && dns['fallback-filter']['geoip-code']) || 'CN',
      _fallbackIPCIDREnabled: !!(dns['fallback-filter'] && (dns['fallback-filter'].ipcidr || []).length > 0),
      _fallbackIPCIDRStr: (dns['fallback-filter'] && dns['fallback-filter'].ipcidr || []).join('\n'),
      _fallbackDomainEnabled: !!(dns['fallback-filter'] && (dns['fallback-filter'].domain || []).length > 0),
      _fallbackDomainStr: (dns['fallback-filter'] && dns['fallback-filter'].domain || []).join('\n'),
      _nsPolicyEnabled: Object.keys(dns['nameserver-policy'] || {}).length > 0,
      _nsPolicyJSON: JSON.stringify(dns['nameserver-policy'] || {}, null, 2),
      _nsPolicyStr: Object.entries(dns['nameserver-policy'] || {}).map(([k, v]) => k + '=' + (Array.isArray(v) ? v.join(', ') : String(v))).join('\n'),
    })

    // External controller (top-level field)
    if (parsed['external-controller']) {
      Object.assign(generalConfig.value, {
        _extControllerEnabled: true,
        _extController: parsed['external-controller'],
        _extControllerTLSEnabled: !!parsed['external-controller-tls'],
        _extControllerTLS: parsed['external-controller-tls'] || '',
        _extSecretEnabled: !!parsed.secret,
        _extSecret: parsed.secret || '',
      })
    }

    // TLS settings
    const tls = parsed.tls || {}
    if (tls && tls.certificate) {
      Object.assign(generalConfig.value, {
        _tlsEnabled: true,
        _tlsCert: tls.certificate || '',
        _tlsKey: tls['private-key'] || '',
      })
    }

    // Experimental settings
    const exp = parsed.experimental || {}
    if (exp) {
      Object.assign(generalConfig.value, {
        _quicDisableGso: exp['quic-go-disable-gso'] || false,
        _quicDisableEcn: exp['quic-go-disable-ecn'] || false,
        _dialerIP4PConvert: exp['dialer-ip4p-convert'] || false,
      })
    }

    // External UI
    if (parsed['external-ui-name']) {
      Object.assign(generalConfig.value, {
        _extUIEnabled: true,
        _extUIName: parsed['external-ui-name'],
        _extUIUrl: parsed['external-ui-url'] || '',
      })
    }

    // Profile
    if (parsed.profile) {
      Object.assign(generalConfig.value, {
        _storeSelected: parsed.profile['store-selected'] || false,
        _storeFakeIP: parsed.profile['store-fake-ip'] !== false,
      })
    }

    // TUN settings
    const tun = parsed.tun || {}
    if (tun.enable) {
      Object.assign(generalConfig.value, {
        _tunEnabled: true,
        _tunStack: tun.stack || 'system',
        _tunDevice: tun.device || '',
        _tunDnsHijack: (tun['dns-hijack'] || ['0.0.0.0:53']).join('\n'),
        _tunAutoRoute: tun['auto-route'] !== false,
        _tunAutoDetect: tun['auto-detect-interface'] !== false,
        _tunAutoRedirect: tun['auto-redirect'] || false,
        _tunMTUEnabled: !!tun.mtu,
        _tunMTU: tun.mtu || 9000,
        _tunStrictRoute: tun['strict-route'] || false,
        _tunDisableICMP: tun['disable-icmp-forwarding'] || false,
        _tunGSO: tun.gso || false,
        _tunGSOMaxSize: tun['gso-max-size'] || 65536,
        _tunIPv6Enabled: !!tun['inet6-address'],
        _tunIPv6Addr: tun['inet6-address'] || '',
        _tunUDPTimeoutEnabled: !!tun['udp-timeout'],
        _tunUDPTimeout: tun['udp-timeout'] || 300,
        _tunRouteAddrV4Enabled: !!(tun['inet4-route-address'] || tun['route-address']),
        _tunRouteAddrV4: ((tun['inet4-route-address'] || tun['route-address'] || []).filter(a => !a.includes(':'))).join('\n'),
        _tunRouteAddrV6Enabled: !!(tun['inet6-route-address'] || tun['route-address']),
        _tunRouteAddrV6: ((tun['inet6-route-address'] || tun['route-address'] || []).filter(a => a.includes(':'))).join('\n'),
      })
    }

    // Sniffer settings
    const sniffer = parsed.sniffer || {}
    if (sniffer.enable) {
      Object.assign(generalConfig.value, {
        _snifferEnabled: true,
        _snifferForceDNS: sniffer['force-dns-mapping'] || false,
        _snifferParsePureIP: sniffer['parse-pure-ip'] || false,
        _snifferOverrideDest: sniffer['override-destination'] || false,
        _snifferHTTP: !!(sniffer.sniff && sniffer.sniff.HTTP),
        _snifferHTTPPorts: (sniffer.sniff && sniffer.sniff.HTTP && sniffer.sniff.HTTP.ports) ? sniffer.sniff.HTTP.ports.join(', ') : '80, 8080-8880',
        _snifferHTTPOverride: (sniffer.sniff && sniffer.sniff.HTTP && sniffer.sniff.HTTP['override-destination']) || false,
        _snifferTLS: !!(sniffer.sniff && sniffer.sniff.TLS),
        _snifferTLSPorts: (sniffer.sniff && sniffer.sniff.TLS && sniffer.sniff.TLS.ports) ? sniffer.sniff.TLS.ports.join(', ') : '443, 8443',
        _snifferQUIC: !!(sniffer.sniff && sniffer.sniff.QUIC),
        _snifferQUICPorts: (sniffer.sniff && sniffer.sniff.QUIC && sniffer.sniff.QUIC.ports) ? sniffer.sniff.QUIC.ports.join(', ') : '443, 8443',
        _snifferForceDomainEnabled: (sniffer['force-domain'] || []).length > 0,
        _snifferForceDomain: (sniffer['force-domain'] || []).join('\n'),
        _snifferSkipDomainEnabled: (sniffer['skip-domain'] || []).length > 0,
        _snifferSkipDomain: (sniffer['skip-domain'] || []).join('\n'),
        _snifferSkipSrcEnabled: (sniffer['skip-src-address'] || []).length > 0,
        _snifferSkipSrcAddr: (sniffer['skip-src-address'] || []).join('\n'),
        _snifferSkipDstEnabled: (sniffer['skip-dst-address'] || []).length > 0,
        _snifferSkipDstAddr: (sniffer['skip-dst-address'] || []).join('\n'),
      })
    }

    if (parsed.hosts) {
      hosts.value = Object.entries(parsed.hosts).map(([domain, ip]) => ({
        domain,
        ip: Array.isArray(ip) ? ip.join(', ') : String(ip)
      }))
    }

    if (parsed['proxy-groups']) {
      // Collect all proxy group names for reference resolution
      const allGroupNames = new Set(parsed['proxy-groups'].map(g => g.name))
      // Built-in outbounds
      const builtinOutbounds = new Set(['DIRECT', 'REJECT', 'dns-out'])

      proxyGroups.value = parsed['proxy-groups'].map(g => {
        const originalProxies = g.proxies || []
        const filteredProxies = originalProxies.filter(p => {
          // Keep built-in outbounds
          if (builtinOutbounds.has(p)) return true
          // Keep references to other proxy groups (they will be imported too)
          if (allGroupNames.has(p)) return true
          // Filter out external node references
          return false
        })
        // Mark if this group had external node references OR has use field
        const hasExternalNodes = originalProxies.length !== filteredProxies.length || !!(g.use && g.use.length > 0)
        return {
          name: g.name,
          type: g.type || 'select',
          _proxiesStr: filteredProxies.join('\n'),
          _showAdvanced: false,
          _hasExternalNodes: hasExternalNodes,
          hidden: g.hidden || false,
          url: g.url || 'https://www.gstatic.com/generate_204',
          interval: g.interval || 300,
          lazy: g.lazy !== false,
          timeout: g.timeout || 5000,
          'max-failed-times': g['max-failed-times'] || 5,
          'empty-fallback': g['empty-fallback'] || ''
        }
      })
    }

    if (parsed['rule-providers']) {
      ruleProviders.value = {}
      Object.entries(parsed['rule-providers']).forEach(([name, rp]) => {
        ruleProviders.value[name] = {
          type: rp.type || 'http',
          behavior: rp.behavior || 'domain',
          format: rp.format || 'yaml',
          url: rp.url || '',
          path: rp.path || '',
          proxy: rp.proxy || 'DIRECT',
          'path-in-bundle': rp['path-in-bundle'] || '',
          interval: rp.interval || 86400
        }
      })
    }

    if (parsed.rules) {
      // Import rules - parse each rule string into components
      formatRules.value = parsed.rules.filter(r => r && typeof r === 'string').map((r, i) => {
        return parseRuleString(r, i)
      })
    }

    if (parsed.tunnels) {
      tunnels.value = parsed.tunnels.map(t => {
        if (typeof t === 'string') {
          const parts = t.split(',')
          return { _network: parts[0] ? parts[0].split('/') : ['tcp','udp'], address: parts[1] || '', target: parts[2] || '', proxy: parts[3] || '' }
        }
        return { _network: t.network || ['tcp','udp'], address: t.address || '', target: t.target || '', proxy: t.proxy || '' }
      })
    }

    importDialog.value = false
    fmtDialog.value = true
    ElMessage.success(t('subfmt.importSuccess'))
  } catch (e) {
    ElMessage.error(t('subfmt.importError') + ': ' + e.message)
  } finally {
    importing.value = false
  }
}

function showEditFormat(row) {
  editingFormat.value = row
  fmtForm.value = { name: row.name, client_type: row.client_type || 'clash', format: row.format || 'mihomo_yaml', description: row.description || '', platform: row.platform || '', plan_ids: row.plan_ids || [], enabled: row.enabled }

  // Parse config_json from mihomo table
  let config = {}
  try { config = JSON.parse(row.config_json || '{}') } catch { config = {} }

  // Parse proxy groups (support both hyphen and underscore keys)
  try {
    const groups = config['proxy-groups'] || config.proxy_groups || []
    proxyGroups.value = groups.map(g => ({
      ...g,
      _proxiesStr: (g.proxies || []).join('\n'),
      _showAdvanced: false,
      _hasExternalNodes: g._hasExternalNodes || false,
      hidden: g.hidden || false,
      lazy: g.lazy !== false,
      url: g.url || 'https://www.gstatic.com/generate_204',
      interval: g.interval || 300,
      timeout: g.timeout || 5000,
      'max-failed-times': g['max-failed-times'] || 5,
      'empty-fallback': g['empty-fallback'] || ''
    }))
  } catch { proxyGroups.value = [] }

  // Parse rule providers (support both hyphen and underscore keys)
  try {
    const rp = config['rule-providers'] || config.rule_providers || {}
    Object.values(rp).forEach(v => {
      v.format = v.format || 'yaml'
      v.path = v.path || ''
      v.proxy = v.proxy || 'DIRECT'
      v['path-in-bundle'] = v['path-in-bundle'] || ''
    })
    ruleProviders.value = rp
  } catch { ruleProviders.value = {} }

  // Parse DNS (support both hyphen and underscore keys)
  try {
    const dns = config.dns || config.DNS || {}
    dnsConfig.value = {
      enable: dns.enable !== false,
      _listen: dns.listen || '0.0.0.0:53',
      _enhancedMode: dns['enhanced-mode'] || 'redir-host',
      _cacheAlgorithm: dns['cache-algorithm'] || 'arc',
      _preferH3: dns['prefer-h3'] || false,
      _ipv6: dns.ipv6 !== false,
      _useHosts: dns['use-hosts'] !== false,
      _respectRules: dns['respect-rules'] || false,
      _defaultNameserverStr: (dns['default-nameserver'] || []).join('\n'),
      _nameserverStr: (dns.nameserver || []).join('\n'),
      _fallbackEnabled: (dns.fallback || []).length > 0,
      _fallbackStr: (dns.fallback || []).join('\n'),
      _proxyServerNSEnabled: (dns['proxy-server-nameserver'] || []).length > 0,
      _proxyServerNSStr: (dns['proxy-server-nameserver'] || []).join('\n'),
      _directNSEnabled: (dns['direct-nameserver'] || []).length > 0,
      _directNSStr: (dns['direct-nameserver'] || []).join('\n'),
      _fakeIPRange: dns['fake-ip-range'] || '198.18.0.1/16',
      _fakeIPFilterMode: dns['fake-ip-filter-mode'] || 'blacklist',
      _fakeIPFilterEnabled: (dns['fake-ip-filter'] || []).length > 0,
      _fakeIPFilterStr: (dns['fake-ip-filter'] || []).join('\n'),
      _fallbackGeoIP: dns['fallback-filter'] ? dns['fallback-filter'].geoip !== false : true,
      _fallbackGeoCode: (dns['fallback-filter'] && dns['fallback-filter']['geoip-code']) || 'CN',
      _fallbackIPCIDREnabled: !!(dns['fallback-filter'] && (dns['fallback-filter'].ipcidr || []).length > 0),
      _fallbackIPCIDRStr: (dns['fallback-filter'] && dns['fallback-filter'].ipcidr || []).join('\n'),
      _fallbackDomainEnabled: !!(dns['fallback-filter'] && (dns['fallback-filter'].domain || []).length > 0),
      _fallbackDomainStr: (dns['fallback-filter'] && dns['fallback-filter'].domain || []).join('\n'),
      _nsPolicyEnabled: Object.keys(dns['nameserver-policy'] || {}).length > 0,
      _nsPolicyJSON: JSON.stringify(dns['nameserver-policy'] || {}, null, 2),
      _nsPolicyStr: Object.entries(dns['nameserver-policy'] || {}).map(([k, v]) => k + '=' + (Array.isArray(v) ? v.join(', ') : String(v))).join('\n'),
    }
  } catch { dnsConfig.value = { enable: true, _listen: '0.0.0.0:53', _enhancedMode: 'redir-host', _cacheAlgorithm: 'arc', _preferH3: false, _ipv6: true, _useHosts: true, _respectRules: false, _defaultNameserverStr: '114.114.114.114\n8.8.8.8', _nameserverStr: 'https://doh.pub/dns-query\nhttps://dns.alidns.com/dns-query', _fallbackEnabled: false, _fallbackStr: '', _proxyServerNSEnabled: false, _proxyServerNSStr: '', _directNSEnabled: false, _directNSStr: '', _fakeIPRange: '198.18.0.1/16', _fakeIPFilterMode: 'blacklist', _fakeIPFilterEnabled: true, _fakeIPFilterStr: '*.lan', _fallbackGeoIP: true, _fallbackGeoCode: 'CN', _fallbackIPCIDREnabled: false, _fallbackIPCIDRStr: '', _fallbackDomainEnabled: false, _fallbackDomainStr: '', _nsPolicyEnabled: false, _nsPolicyJSON: '{}', _nsPolicyStr: '' } }

  // Parse general settings (top-level fields in mihomo config)
  try {
    const gen = config.general || config // fallback to top-level if no nested general
    generalConfig.value = {
      mixed_port: gen['mixed-port'] || gen.mixed_port || 10801, _mixedPortEnabled: !!(gen['mixed-port'] || gen.mixed_port),
      port: gen.port || 0, _portEnabled: !!gen.port,
      socks_port: gen['socks-port'] || gen.socks_port || 0, _socksPortEnabled: !!(gen['socks-port'] || gen.socks_port),
      redir_port: gen['redir-port'] || gen.redir_port || 0, _redirPortEnabled: !!(gen['redir-port'] || gen.redir_port),
      tproxy_port: gen['tproxy-port'] || gen.tproxy_port || 0, _tproxyPortEnabled: !!(gen['tproxy-port'] || gen.tproxy_port),
      allow_lan: gen['allow-lan'] !== false && gen.allow_lan !== false, bind_address: gen['bind-address'] || gen.bind_address || '*',
      _authEnabled: (gen.authentication || []).length > 0,
      _authStr: (gen.authentication || []).join('\n'),
      _skipAuthEnabled: (gen['skip-auth-prefixes'] || gen.skip_auth_prefixes || []).length > 0,
      _skipAuthStr: (gen['skip-auth-prefixes'] || gen.skip_auth_prefixes || ['127.0.0.1/8', '::1/128']).join('\n'),
      _lanAllowedEnabled: (gen['lan-allowed-ips'] || gen.lan_allowed_ips || []).length > 0,
      _lanAllowedStr: (gen['lan-allowed-ips'] || gen.lan_allowed_ips || ['0.0.0.0/0', '::/0']).join('\n'),
      _lanDisallowedEnabled: (gen['lan-disallowed-ips'] || gen.lan_disallowed_ips || []).length > 0,
      _lanDisallowedStr: (gen['lan-disallowed-ips'] || gen.lan_disallowed_ips || []).join('\n'),
      find_process_mode: gen['find-process-mode'] || gen.find_process_mode || 'strict', mode: gen.mode || 'rule', log_level: gen['log-level'] || gen.log_level || 'info',
      ipv6: gen.ipv6 !== false,
      _geoAutoUpdate: gen['geo-auto-update'] || gen.geo_auto_update || false, _geoUpdateInterval: gen['geo-update-interval'] || gen.geo_update_interval || 24,
      _geoIPEnabled: !!((gen.geox_url || gen['geox-url']) && (gen.geox_url || gen['geox-url']).geoip),
      _geoIPUrl: ((gen.geox_url || gen['geox-url']) && (gen.geox_url || gen['geox-url']).geoip) || '',
      _geositeEnabled: !!((gen.geox_url || gen['geox-url']) && (gen.geox_url || gen['geox-url']).geosite),
      _geositeUrl: ((gen.geox_url || gen['geox-url']) && (gen.geox_url || gen['geox-url']).geosite) || '',
      _mmdbEnabled: !!((gen.geox_url || gen['geox-url']) && (gen.geox_url || gen['geox-url']).mmdb),
      _mmdbUrl: ((gen.geox_url || gen['geox-url']) && (gen.geox_url || gen['geox-url']).mmdb) || '',
      _extControllerEnabled: !!(gen['external-controller'] || gen.external_controller), _extController: gen['external-controller'] || gen.external_controller || '0.0.0.0:9093', _extControllerTLSEnabled: !!(gen['external-controller-tls'] || gen.external_controller_tls), _extControllerTLS: gen['external-controller-tls'] || gen.external_controller_tls || '',
      _extSecretEnabled: !!(gen.secret), _extSecret: gen.secret || '',
      _extUIEnabled: !!(gen['external-ui-name'] || gen.external_ui_name), _extUIName: gen['external-ui-name'] || gen.external_ui_name || 'metacubexd', _extUIUrl: gen['external-ui-url'] || gen.external_ui_url || '',
      _tlsEnabled: !!(gen.tls && gen.tls.certificate), _tlsCert: (gen.tls && gen.tls.certificate) || '', _tlsKey: (gen.tls && gen.tls['private-key']) || '',
      _interfaceEnabled: !!(gen['interface-name']), _interfaceName: gen['interface-name'] || '',
      _routingMarkEnabled: !!(gen['routing-mark']), _routingMark: gen['routing-mark'] || 0,
      _tcpConcurrent: gen['tcp-concurrent'] !== false,
      _keepAliveEnabled: !!(gen['keep-alive-idle']), _keepAliveIdle: gen['keep-alive-idle'] || 15,
      _quicDisableGso: !!(gen.experimental && gen.experimental['quic-go-disable-gso']),
      _quicDisableEcn: !!(gen.experimental && gen.experimental['quic-go-disable-ecn']),
      _dialerIP4PConvert: !!(gen.experimental && gen.experimental['dialer-ip4p-convert']),
      _storeSelected: (gen.profile && gen.profile['store-selected']) || false,
      _storeFakeIP: gen.profile ? gen.profile['store-fake-ip'] !== false : true,
      _tunEnabled: (gen.tun && gen.tun.enable) || false,
      _tunStack: (gen.tun && gen.tun.stack) || 'system',
      _tunDnsHijack: (gen.tun && gen.tun['dns-hijack'] || ['0.0.0.0:53']).join('\n'),
      _tunAutoRoute: (gen.tun && gen.tun['auto-route'] !== false),
      _tunAutoRedirect: (gen.tun && gen.tun['auto-redirect']) || false,
      _tunMTUEnabled: !!(gen.tun && gen.tun.mtu),
      _tunMTU: (gen.tun && gen.tun.mtu) || 9000,
      _tunStrictRoute: (gen.tun && gen.tun['strict-route']) || false,
      _tunDisableICMP: (gen.tun && gen.tun['disable-icmp-forwarding']) || false,
      _tunDevice: (gen.tun && gen.tun.device) || '',
      _tunAutoDetect: gen.tun ? gen.tun['auto-detect-interface'] !== false : true,
      _tunEndpointNAT: (gen.tun && gen.tun['endpoint-independent-nat']) || false,
      _tunGSO: (gen.tun && gen.tun.gso) || false,
      _tunGSOMaxSize: (gen.tun && gen.tun['gso-max-size']) || 65536,
      _tunIPv6Enabled: !!(gen.tun && gen.tun['inet6-address']),
      _tunIPv6Addr: (gen.tun && gen.tun['inet6-address']) || '',
      _tunUDPTimeoutEnabled: !!(gen.tun && gen.tun['udp-timeout']),
      _tunUDPTimeout: (gen.tun && gen.tun['udp-timeout']) || 300,
      _tunRouteAddrV4Enabled: !!(gen.tun && (gen.tun['inet4-route-address'] || gen.tun['route-address'])),
      _tunRouteAddrV4: (gen.tun && gen.tun['inet4-route-address'] || gen.tun && gen.tun['route-address'] || []).filter(a => !a.includes(':')).join('\n'),
      _tunRouteAddrV6Enabled: !!(gen.tun && (gen.tun['inet6-route-address'] || gen.tun['route-address'])),
      _tunRouteAddrV6: (gen.tun && gen.tun['inet6-route-address'] || gen.tun && gen.tun['route-address'] || []).filter(a => a.includes(':')).join('\n'),
      _tunRouteExcludeV4Enabled: !!(gen.tun && (gen.tun['inet4-route-exclude-address'] || gen.tun['route-exclude-address'])),
      _tunRouteExcludeV4: ((gen.tun && gen.tun['inet4-route-exclude-address']) || (gen.tun && gen.tun['route-exclude-address']) || []).filter(a => !a.includes(':')).join('\n'),
      _tunRouteExcludeV6Enabled: !!(gen.tun && gen.tun['inet6-route-exclude-address']),
      _tunRouteExcludeV6: ((gen.tun && gen.tun['inet6-route-exclude-address']) || []).filter(a => a.includes(':')).join('\n'),
      _tunRouteAddrSetEnabled: !!(gen.tun && gen.tun['route-address-set']),
      _tunRouteAddrSet: (gen.tun && gen.tun['route-address-set'] || []).join('\n'),
      _tunRouteExcludeSetEnabled: !!(gen.tun && gen.tun['route-exclude-address-set']),
      _tunRouteExcludeSet: (gen.tun && gen.tun['route-exclude-address-set'] || []).join('\n'),
      _tunIncludeIfEnabled: !!(gen.tun && gen.tun['include-interface']),
      _tunIncludeIf: (gen.tun && gen.tun['include-interface'] || []).join('\n'),
      _tunExcludeIfEnabled: !!(gen.tun && gen.tun['exclude-interface']),
      _tunExcludeIf: (gen.tun && gen.tun['exclude-interface'] || []).join('\n'),
      _tunIncludeUIDEnabled: !!(gen.tun && gen.tun['include-uid']),
      _tunIncludeUID: (gen.tun && gen.tun['include-uid'] || []).join('\n'),
      _tunExcludeUIDEnabled: !!(gen.tun && gen.tun['exclude-uid']),
      _tunExcludeUID: (gen.tun && gen.tun['exclude-uid'] || []).join('\n'),
      _tunIncludeUIDRangeEnabled: !!(gen.tun && gen.tun['include-uid-range']),
      _tunIncludeUIDRange: (gen.tun && gen.tun['include-uid-range'] || []).join('\n'),
      _tunExcludeUIDRangeEnabled: !!(gen.tun && gen.tun['exclude-uid-range']),
      _tunExcludeUIDRange: (gen.tun && gen.tun['exclude-uid-range'] || []).join('\n'),
      _tunIncludeMACEnabled: !!(gen.tun && gen.tun['include-mac-address']),
      _tunIncludeMAC: (gen.tun && gen.tun['include-mac-address'] || []).join('\n'),
      _tunExcludeMACEnabled: !!(gen.tun && gen.tun['exclude-mac-address']),
      _tunExcludeMAC: (gen.tun && gen.tun['exclude-mac-address'] || []).join('\n'),
      _tunIncludeAndroidUserEnabled: !!(gen.tun && gen.tun['include-android-user']),
      _tunIncludeAndroidUser: (gen.tun && gen.tun['include-android-user'] || []).join('\n'),
      _tunIncludePkgEnabled: !!(gen.tun && gen.tun['include-package']),
      _tunIncludePkg: (gen.tun && gen.tun['include-package'] || []).join('\n'),
      _tunExcludePkgEnabled: !!(gen.tun && gen.tun['exclude-package']),
      _tunExcludePkg: (gen.tun && gen.tun['exclude-package'] || []).join('\n'),
      _snifferEnabled: (gen.sniffer && gen.sniffer.enable) || false,
      _snifferForceDNS: (gen.sniffer && gen.sniffer['force-dns-mapping']) || false,
      _snifferParsePureIP: (gen.sniffer && gen.sniffer['parse-pure-ip']) || false,
      _snifferOverrideDest: (gen.sniffer && gen.sniffer['override-destination']) || false,
      _snifferHTTP: !!(gen.sniffer && gen.sniffer.sniff && gen.sniffer.sniff.HTTP),
      _snifferHTTPPorts: (gen.sniffer && gen.sniffer.sniff && gen.sniffer.sniff.HTTP && gen.sniffer.sniff.HTTP.ports) ? gen.sniffer.sniff.HTTP.ports.join(', ') : '80, 8080-8880',
      _snifferHTTPOverride: (gen.sniffer && gen.sniffer.sniff && gen.sniffer.sniff.HTTP && gen.sniffer.sniff.HTTP['override-destination']) || false,
      _snifferTLS: !!(gen.sniffer && gen.sniffer.sniff && gen.sniffer.sniff.TLS),
      _snifferTLSPorts: (gen.sniffer && gen.sniffer.sniff && gen.sniffer.sniff.TLS && gen.sniffer.sniff.TLS.ports) ? gen.sniffer.sniff.TLS.ports.join(', ') : '443, 8443',
      _snifferQUIC: !!(gen.sniffer && gen.sniffer.sniff && gen.sniffer.sniff.QUIC),
      _snifferQUICPorts: (gen.sniffer && gen.sniffer.sniff && gen.sniffer.sniff.QUIC && gen.sniffer.sniff.QUIC.ports) ? gen.sniffer.sniff.QUIC.ports.join(', ') : '443, 8443',
      _snifferForceDomainEnabled: (gen.sniffer && gen.sniffer['force-domain'] || []).length > 0,
      _snifferForceDomain: (gen.sniffer && gen.sniffer['force-domain'] || []).join('\n'),
      _snifferSkipDomainEnabled: (gen.sniffer && gen.sniffer['skip-domain'] || []).length > 0,
      _snifferSkipDomain: (gen.sniffer && gen.sniffer['skip-domain'] || []).join('\n'),
      _snifferSkipSrcEnabled: (gen.sniffer && gen.sniffer['skip-src-address'] || []).length > 0,
      _snifferSkipSrcAddr: (gen.sniffer && gen.sniffer['skip-src-address'] || []).join('\n'),
      _snifferSkipDstEnabled: (gen.sniffer && gen.sniffer['skip-dst-address'] || []).length > 0,
      _snifferSkipDstAddr: (gen.sniffer && gen.sniffer['skip-dst-address'] || []).join('\n'),
      _dnsOutEnabled: !!(gen.proxies && gen.proxies.find(p => p.type === 'dns')),
      _rematchOutEnabled: !!(gen.proxies && gen.proxies.find(p => p.type === 'rematch')),
      _rematchTarget: (gen.proxies && gen.proxies.find(p => p.type === 'rematch') || {})['target-rematch-name'] || '',
      _directOutEnabled: !!(gen.proxies && gen.proxies.find(p => p.type === 'direct' && p['interface-name'])),
      _directInterface: (gen.proxies && gen.proxies.find(p => p.type === 'direct' && p['interface-name']) || {})['interface-name'] || '',
      _directRoutingMark: (gen.proxies && gen.proxies.find(p => p.type === 'direct' && p['interface-name']) || {})['routing-mark'] || 0
    }
    // Parse tunnels
    if (gen.tunnels && Array.isArray(gen.tunnels)) {
      tunnels.value = gen.tunnels.map(t => {
        if (typeof t === 'string') {
          // One line format: "tcp/udp,127.0.0.1:6553,114.114.114.114:53,proxy"
          const parts = t.split(',')
          return { _network: parts[0] ? parts[0].split('/') : ['tcp','udp'], address: parts[1] || '', target: parts[2] || '', proxy: parts[3] || '' }
        }
        return { _network: t.network || ['tcp','udp'], address: t.address || '', target: t.target || '', proxy: t.proxy || '' }
      })
    } else {
      tunnels.value = []
    }
    // Parse hosts
    if (gen.hosts && typeof gen.hosts === 'object' && !Array.isArray(gen.hosts)) {
      hosts.value = Object.entries(gen.hosts).map(([domain, ip]) => ({ domain, ip: Array.isArray(ip) ? ip.join(', ') : String(ip) }))
    } else {
      hosts.value = []
    }
  } catch { generalConfig.value = { mixed_port: 10801, port: 0, socks_port: 0, redir_port: 0, tproxy_port: 0, allow_lan: true, bind_address: '*', _authStr: '', _skipAuthStr: '127.0.0.1/8\n::1/128', _lanAllowedStr: '0.0.0.0/0\n::/0', _lanDisallowedStr: '', find_process_mode: 'strict', mode: 'rule', log_level: 'info' } }
  fmtTab.value = 'basic'
  fmtDialog.value = true
}

async function saveFormat() {
  // Build nsPolicy from JSON
  let nsPolicy = {}
  if (dnsConfig.value._nsPolicyEnabled && dnsConfig.value._nsPolicyJSON) {
    try { nsPolicy = JSON.parse(dnsConfig.value._nsPolicyJSON) } catch { nsPolicy = {} }
  }

  const configJSON = {
    'mixed-port': generalConfig.value._mixedPortEnabled ? (generalConfig.value.mixed_port || 0) : 0,
    'port': generalConfig.value._portEnabled ? (generalConfig.value.port || 0) : 0,
    'socks-port': generalConfig.value._socksPortEnabled ? (generalConfig.value.socks_port || 0) : 0,
    'redir-port': generalConfig.value._redirPortEnabled ? (generalConfig.value.redir_port || 0) : 0,
    'tproxy-port': generalConfig.value._tproxyPortEnabled ? (generalConfig.value.tproxy_port || 0) : 0,
    'allow-lan': generalConfig.value.allow_lan,
    'bind-address': generalConfig.value.bind_address || '*',
    'mode': generalConfig.value.mode || 'rule',
    'log-level': generalConfig.value.log_level || 'info',
    'ipv6': generalConfig.value.ipv6 !== false,
    'find-process-mode': generalConfig.value.find_process_mode || 'strict',
    'tcp-concurrent': generalConfig.value._tcpConcurrent || false,
    'keep-alive-idle': generalConfig.value._keepAliveEnabled ? generalConfig.value._keepAliveIdle : 0,
    'authentication': generalConfig.value._authEnabled ? (generalConfig.value._authStr || '').split('\n').map(s => s.trim()).filter(Boolean) : [],
    'external-controller': generalConfig.value._extControllerEnabled ? generalConfig.value._extController : '',
    'external-controller-tls': generalConfig.value._extControllerEnabled && generalConfig.value._extControllerTLSEnabled ? (generalConfig.value._extControllerTLS || '') : '',
    'secret': generalConfig.value._extSecretEnabled ? generalConfig.value._extSecret : '',
    'external-ui-name': generalConfig.value._extUIEnabled ? generalConfig.value._extUIName : '',
    'external-ui-url': generalConfig.value._extUIEnabled ? generalConfig.value._extUIUrl : '',
    'geox-url': {
      geoip: generalConfig.value._geoIPEnabled ? generalConfig.value._geoIPUrl : '',
      geosite: generalConfig.value._geositeEnabled ? generalConfig.value._geositeUrl : '',
      mmdb: generalConfig.value._mmdbEnabled ? generalConfig.value._mmdbUrl : ''
    },
    'geo-auto-update': generalConfig.value._geoAutoUpdate || false,
    'geo-update-interval': generalConfig.value._geoUpdateInterval || 24,
    'dns': {
      enable: dnsConfig.value.enable,
      listen: dnsConfig.value._listen || '0.0.0.0:53',
      'enhanced-mode': dnsConfig.value._enhancedMode || 'redir-host',
      'cache-algorithm': dnsConfig.value._cacheAlgorithm || 'arc',
      'prefer-h3': dnsConfig.value._preferH3 || false,
      ipv6: dnsConfig.value._ipv6 !== false,
      'use-hosts': dnsConfig.value._useHosts !== false,
      'respect-rules': dnsConfig.value._respectRules || false,
      'default-nameserver': (dnsConfig.value._defaultNameserverStr || '').split('\n').map(s => s.trim()).filter(Boolean),
      nameserver: (dnsConfig.value._nameserverStr || '').split('\n').map(s => s.trim()).filter(Boolean),
      fallback: dnsConfig.value._fallbackEnabled ? (dnsConfig.value._fallbackStr || '').split('\n').map(s => s.trim()).filter(Boolean) : [],
      'fake-ip-range': dnsConfig.value._fakeIPRange || '198.18.0.1/16',
      'fake-ip-filter-mode': dnsConfig.value._fakeIPFilterMode || 'blacklist',
      'fake-ip-filter': dnsConfig.value._fakeIPFilterEnabled ? (dnsConfig.value._fakeIPFilterStr || '').split('\n').map(s => s.trim()).filter(Boolean) : [],
      'nameserver-policy': Object.keys(nsPolicy).length > 0 ? nsPolicy : undefined
    },
    'proxy-groups': proxyGroups.value.map(g => {
      const group = { name: g.name, type: g.type, proxies: (g._proxiesStr || '').split('\n').map(s => s.trim()).filter(Boolean) }
      if (g._hasExternalNodes) group._hasExternalNodes = true
      if (g.url) group.url = g.url
      if (g.interval) group.interval = g.interval
      if (g.lazy) group.lazy = true
      if (g.timeout) group.timeout = g.timeout
      if (g['max-failed-times']) group['max-failed-times'] = g['max-failed-times']
      if (g['empty-fallback']) group['empty-fallback'] = g['empty-fallback']
      if (g.filter) group.filter = g.filter
      if (g['exclude-filter']) group['exclude-filter'] = g['exclude-filter']
      return group
    }),
    'rule-providers': ruleProviders.value,
    'rules': formatRules.value.map(r => buildRuleString(r)),
    'hosts': hosts.value.reduce((acc, h) => { if (h.domain) acc[h.domain] = h.ip; return acc }, {}),
    'tunnels': tunnels.value.filter(t => t.address && t.target).map(t => ({ network: t._network || ['tcp','udp'], address: t.address, target: t.target, proxy: t.proxy || undefined })),
    // TLS
    ...(generalConfig.value._tlsEnabled ? { tls: { certificate: generalConfig.value._tlsCert, 'private-key': generalConfig.value._tlsKey } } : {}),
    // Sniffer
    ...(generalConfig.value._snifferEnabled ? { sniffer: {
      enable: true,
      'force-dns-mapping': generalConfig.value._snifferForceDNS || false,
      'parse-pure-ip': generalConfig.value._snifferParsePureIP || false,
      'override-destination': generalConfig.value._snifferOverrideDest || false,
      sniff: {
        ...(generalConfig.value._snifferHTTP ? { HTTP: { ports: parsePorts(generalConfig.value._snifferHTTPPorts), 'override-destination': generalConfig.value._snifferHTTPOverride || false } } : {}),
        ...(generalConfig.value._snifferTLS ? { TLS: { ports: parsePorts(generalConfig.value._snifferTLSPorts) } } : {}),
        ...(generalConfig.value._snifferQUIC ? { QUIC: { ports: parsePorts(generalConfig.value._snifferQUICPorts) } } : {})
      },
      'force-domain': generalConfig.value._snifferForceDomainEnabled ? (generalConfig.value._snifferForceDomain || '').split('\n').map(s => s.trim()).filter(Boolean) : undefined,
      'skip-domain': generalConfig.value._snifferSkipDomainEnabled ? (generalConfig.value._snifferSkipDomain || '').split('\n').map(s => s.trim()).filter(Boolean) : undefined,
      'skip-src-address': generalConfig.value._snifferSkipSrcEnabled ? (generalConfig.value._snifferSkipSrcAddr || '').split('\n').map(s => s.trim()).filter(Boolean) : undefined,
      'skip-dst-address': generalConfig.value._snifferSkipDstEnabled ? (generalConfig.value._snifferSkipDstAddr || '').split('\n').map(s => s.trim()).filter(Boolean) : undefined
    } } : {}),
    // TUN
    ...(generalConfig.value._tunEnabled ? { tun: {
      enable: true,
      stack: generalConfig.value._tunStack || 'system',
      device: generalConfig.value._tunDevice || undefined,
      'dns-hijack': (generalConfig.value._tunDnsHijack || '0.0.0.0:53').split('\n').map(s => s.trim()).filter(Boolean),
      'endpoint-independent-nat': generalConfig.value._tunEndpointNAT || false,
      'auto-route': generalConfig.value._tunAutoRoute !== false,
      'auto-detect-interface': generalConfig.value._tunAutoDetect !== false,
      'auto-redirect': generalConfig.value._tunAutoRedirect || false,
      mtu: generalConfig.value._tunMTUEnabled ? generalConfig.value._tunMTU : undefined,
      'strict-route': generalConfig.value._tunStrictRoute || false,
      'disable-icmp-forwarding': generalConfig.value._tunDisableICMP || false,
      gso: generalConfig.value._tunGSO || false,
      'gso-max-size': generalConfig.value._tunGSO ? generalConfig.value._tunGSOMaxSize : undefined,
      'inet6-address': generalConfig.value._tunIPv6Enabled ? generalConfig.value._tunIPv6Addr : undefined,
      'udp-timeout': generalConfig.value._tunUDPTimeoutEnabled ? generalConfig.value._tunUDPTimeout : undefined,
      'route-address': generalConfig.value._tunRouteAddrV4Enabled || generalConfig.value._tunRouteAddrV6Enabled ?
        (generalConfig.value._tunRouteAddrV4Enabled ? (generalConfig.value._tunRouteAddrV4 || '').split('\n') : [])
        .concat(generalConfig.value._tunRouteAddrV6Enabled ? (generalConfig.value._tunRouteAddrV6 || '').split('\n') : [])
        .map(s => s.trim()).filter(Boolean) : undefined,
      'route-exclude-address': generalConfig.value._tunRouteExcludeV4Enabled ? (generalConfig.value._tunRouteExcludeV4 || '').split('\n').map(s => s.trim()).filter(Boolean) : undefined,
      'route-address-set': generalConfig.value._tunRouteAddrSetEnabled ? (generalConfig.value._tunRouteAddrSet || '').split('\n').map(s => s.trim()).filter(Boolean) : undefined,
      'route-exclude-address-set': generalConfig.value._tunRouteExcludeSetEnabled ? (generalConfig.value._tunRouteExcludeSet || '').split('\n').map(s => s.trim()).filter(Boolean) : undefined,
      'include-interface': generalConfig.value._tunIncludeIfEnabled ? (generalConfig.value._tunIncludeIf || '').split('\n').map(s => s.trim()).filter(Boolean) : undefined,
      'exclude-interface': generalConfig.value._tunExcludeIfEnabled ? (generalConfig.value._tunExcludeIf || '').split('\n').map(s => s.trim()).filter(Boolean) : undefined,
      'include-uid': generalConfig.value._tunIncludeUIDEnabled ? (generalConfig.value._tunIncludeUID || '').split('\n').map(s => s.trim()).filter(Boolean) : undefined,
      'exclude-uid': generalConfig.value._tunExcludeUIDEnabled ? (generalConfig.value._tunExcludeUID || '').split('\n').map(s => s.trim()).filter(Boolean) : undefined,
      'include-uid-range': generalConfig.value._tunIncludeUIDRangeEnabled ? (generalConfig.value._tunIncludeUIDRange || '').split('\n').map(s => s.trim()).filter(Boolean) : undefined,
      'exclude-uid-range': generalConfig.value._tunExcludeUIDRangeEnabled ? (generalConfig.value._tunExcludeUIDRange || '').split('\n').map(s => s.trim()).filter(Boolean) : undefined,
      'include-mac-address': generalConfig.value._tunIncludeMACEnabled ? (generalConfig.value._tunIncludeMAC || '').split('\n').map(s => s.trim()).filter(Boolean) : undefined,
      'exclude-mac-address': generalConfig.value._tunExcludeMACEnabled ? (generalConfig.value._tunExcludeMAC || '').split('\n').map(s => s.trim()).filter(Boolean) : undefined,
      'include-android-user': generalConfig.value._tunIncludeAndroidUserEnabled ? (generalConfig.value._tunIncludeAndroidUser || '').split('\n').map(s => s.trim()).filter(Boolean) : undefined,
      'include-package': generalConfig.value._tunIncludePkgEnabled ? (generalConfig.value._tunIncludePkg || '').split('\n').map(s => s.trim()).filter(Boolean) : undefined,
      'exclude-package': generalConfig.value._tunExcludePkgEnabled ? (generalConfig.value._tunExcludePkg || '').split('\n').map(s => s.trim()).filter(Boolean) : undefined
    } } : {}),
    // Experimental (always save if section exists)
    'experimental': {
      'quic-go-disable-gso': generalConfig.value._quicDisableGso || false,
      'quic-go-disable-ecn': generalConfig.value._quicDisableEcn || false,
      'dialer-ip4p-convert': generalConfig.value._dialerIP4PConvert || false
    },
    // Profile
    profile: { 'store-selected': generalConfig.value._storeSelected || false, 'store-fake-ip': generalConfig.value._storeFakeIP !== false },
    // Special outbounds
    proxies: [
      ...(generalConfig.value._dnsOutEnabled ? [{ name: 'dns-out', type: 'dns' }] : []),
      ...(generalConfig.value._rematchOutEnabled ? [{ name: 'rematch', type: 'rematch', 'target-rematch-name': generalConfig.value._rematchTarget || undefined }] : []),
      ...(generalConfig.value._directOutEnabled ? [{ name: generalConfig.value._directInterface + '-direct', type: 'direct', 'interface-name': generalConfig.value._directInterface, 'routing-mark': generalConfig.value._directRoutingMark || undefined }] : [])
    ],
  }

  try {
    let formatId
    const payload = {
      name: fmtForm.value.name,
      client_type: fmtForm.value.client_type || 'clash',
      format: fmtForm.value.format || selectedFormatType.value || 'mihomo_yaml',
      description: fmtForm.value.description,
      platform: fmtForm.value.platform || '',
      plan_ids: fmtForm.value.plan_ids || [],
      enabled: fmtForm.value.enabled,
      config_json: JSON.stringify(configJSON)
    }
    if (editingFormat.value) {
      await client.put(`/mihomo/configs/${editingFormat.value.id}`, payload)
      formatId = editingFormat.value.id
    } else {
      const r = await client.post('/mihomo/configs', payload)
      formatId = r.id
    }

    // Save rules separately
    if (formatId && formatRules.value.length > 0) {
      // Delete existing rules first
      if (editingFormat.value) {
        try {
          const existingData = await client.get(`/mihomo/configs/${formatId}`)
          for (const rule of (existingData.data?.rules || [])) {
            await client.delete(`/mihomo/rules/${rule.id}`)
          }
        } catch (e) { /* ignore */ }
      }
      // Save new rules
      for (const rule of formatRules.value) {
        try {
          await client.post(`/mihomo/configs/${formatId}/rules`, {
            rule_string: buildRuleString(rule),
            priority: rule.priority || 0,
            enabled: rule.enabled !== false
          })
        } catch (e) { /* ignore */ }
      }
    }

    ElMessage.success(t('common.success')); fmtDialog.value = false; fetchFormats()
  } catch {}
}

// Rule Provider helpers
function addRuleProvider() {
  if (!newRPName.value) return
  ruleProviders.value[newRPName.value] = { type: 'http', behavior: 'domain', format: 'yaml', url: '', path: '', proxy: 'DIRECT', 'path-in-bundle': '', interval: 86400 }
  ruleProviders.value = { ...ruleProviders.value }
  newRPName.value = ''
}
function deleteRuleProvider(name) { delete ruleProviders.value[name]; ruleProviders.value = { ...ruleProviders.value } }

// Proxy Provider helpers
function addProxyProvider() {
  if (!newPPName.value) return
  proxyProviders.value[newPPName.value] = { _name: newPPName.value, type: 'http', url: '', interval: 3600 }
  newPPName.value = ''
}
function deleteProxyProvider(name) { delete proxyProviders.value[name]; proxyProviders.value = { ...proxyProviders.value } }

function parsePorts(str) {
  if (!str) return []
  return str.split(',').map(s => s.trim()).filter(Boolean)
}
async function deleteFormat(row) { await ElMessageBox.confirm(t('subfmt.deleteConfirm', { name: row.name }), t('common.confirm')); await client.delete(`/mihomo/configs/${row.id}`); ElMessage.success(t('common.success')); fetchFormats() }

// Format Rules
async function showFormatRules(row) {
  currentFormat.value = row
  rulesDialog.value = true
  try {
    const r = await client.get(`/mihomo/configs/${row.id}`)
    const rules = r.data?.rules || []
    // Parse rule strings into components
    formatRules.value = rules.map((rule, i) => ({
      ...parseRuleString(rule.rule_string || '', i),
      id: rule.id,
      priority: rule.priority || i,
      enabled: rule.enabled !== false
    }))
  } catch { formatRules.value = [] }
}
async function addRule() {
  if (newRuleType.value === 'RULE-SET') {
    if (!newRuleProvider.value) return
    const ruleString = `RULE-SET,${newRuleProvider.value},${newRuleOutbound.value}`
    await client.post(`/mihomo/configs/${currentFormat.value.id}/rules`, { rule_string: ruleString })
    newRuleProvider.value = ''
  } else {
    if (!newRuleValue.value) return
    const ruleString = `${newRuleType.value},${newRuleValue.value},${newRuleOutbound.value}`
    await client.post(`/mihomo/configs/${currentFormat.value.id}/rules`, { rule_string: ruleString })
    newRuleValue.value = ''
  }
  showFormatRules(currentFormat.value)
}
function onRuleTypeChange() { newRuleValue.value = ''; newRuleProvider.value = '' }
async function deleteRule(row) { await client.delete(`/mihomo/rules/${row.id}`); showFormatRules(currentFormat.value) }

// Endpoint CRUD
function showCreateEndpoint() { editingEndpoint.value = null; epForm.value = { domain: '', path: '/', description: '', enabled: true }; epDialog.value = true }
function showEditEndpoint(row) { editingEndpoint.value = row; epForm.value = { domain: row.domain, path: row.path || '/', description: row.description || '', enabled: row.enabled }; epDialog.value = true }
async function saveEndpoint() { if (!epForm.value.domain) { ElMessage.error(t('subend.domainRequired')); return }; try { if (editingEndpoint.value) { await client.put(`/subscription-endpoints/${editingEndpoint.value.id}`, epForm.value) } else { await client.post('/subscription-endpoints', epForm.value) } ElMessage.success(t('common.success')); epDialog.value = false; fetchEndpoints() } catch {} }
async function deleteEndpoint(row) { await ElMessageBox.confirm(t('subend.deleteConfirm', { domain: row.domain }), t('common.confirm')); await client.delete(`/subscription-endpoints/${row.id}`); ElMessage.success(t('common.success')); fetchEndpoints() }

onMounted(() => { fetchSubscriptions(); fetchFormats(); fetchEndpoints(); fetchPlans() })
</script>
