<template>
  <div>
    <div style="display:flex;justify-content:space-between;margin-bottom:16px">
      <h3 style="margin:0">{{ t('externalNodes.title') }}</h3>
      <el-button type="primary" size="small" @click="showImport">{{ t('externalNodes.import') }}</el-button>
    </div>

    <!-- Sources -->
    <el-card style="margin-bottom:16px">
      <div class="hint" style="margin-bottom:8px">{{ t('externalNodes.importSources') }}</div>
      <el-table v-if="!isMobile" :data="sourcesPagedList" stripe size="small">
        <el-table-column prop="id" label="ID" width="60" />
        <el-table-column prop="name" :label="t('common.name')" min-width="180" />
        <el-table-column prop="source_type" :label="t('common.type')" width="80">
          <template #default="{row}"><el-tag size="small">{{ row.source_type }}</el-tag></template>
        </el-table-column>
        <el-table-column :label="t('common.status')" width="80">
          <template #default="{row}"><el-switch v-model="row.enabled" @change="updateSource(row)" /></template>
        </el-table-column>
        <el-table-column :label="t('common.actions')" width="120">
          <template #default="{row}">
            <el-button size="small" type="danger" text @click="deleteSource(row)">{{ t('common.delete') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
      <MobileCards v-else :rows="sourcesPagedList" title-field="name" :fields="extSourceCardFields">
        <template #actions="{ row }">
          <el-switch v-model="row.enabled" @change="updateSource(row)" />
          <el-button size="small" type="danger" text @click="deleteSource(row)">{{ t('common.delete') }}</el-button>
        </template>
      </MobileCards>
      <PaginationBar v-model:page="sourcesPage" v-model:page-size="sourcesSize" :total="sourcesTotal" />
    </el-card>

    <!-- Nodes -->
    <el-card>
      <div class="hint" style="margin-bottom:8px">{{ t('externalNodes.allNodesHint') }}</div>
      <el-table v-if="!isMobile" :data="nodesPagedList" stripe size="small">
        <el-table-column prop="id" label="ID" width="50" />
        <el-table-column prop="name" :label="t('common.name')" min-width="160" />
        <el-table-column prop="protocol" :label="t('externalNodes.protocol')" width="80">
          <template #default="{row}"><el-tag size="small">{{ row.protocol }}</el-tag></template>
        </el-table-column>
        <el-table-column prop="host" :label="t('externalNodes.address')" width="180" />
        <el-table-column prop="port" :label="t('externalNodes.port')" width="70" />
        <el-table-column :label="t('protoCenter.inbounds.accessDomain')" width="180">
          <template #default="{row}">
            <el-tag v-if="accessAddress(row) !== '-'" size="small" type="success">{{ accessAddress(row) }}</el-tag>
            <span v-else style="color:#909399">-</span>
          </template>
        </el-table-column>
        <el-table-column prop="transport" :label="t('externalNodes.transport')" width="80" />
        <el-table-column label="TLS" width="60">
          <template #default="{row}"><el-tag v-if="row.tls" size="small" type="success">TLS</el-tag></template>
        </el-table-column>
        <el-table-column :label="t('common.status')" width="120">
          <template #default="{row}">
            <div style="display:flex;gap:4px;align-items:center;white-space:nowrap">
              <el-switch v-model="row.enabled" size="small" @change="updateNode(row)" />
              <el-tag v-if="registeredNodeIds.has(row.id)" size="small" type="success">{{ t('externalNodes.registered') }}</el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column :label="t('common.actions')" width="220">
          <template #default="{row}">
            <div style="display:flex;gap:2px;align-items:center;white-space:nowrap">
              <el-button size="small" text @click="editNode(row)">{{ t('common.edit') }}</el-button>
              <el-button v-if="!registeredNodeIds.has(row.id)" size="small" type="success" text @click="registerAsEntry(row)">{{ t('externalNodes.registerAsEntry') }}</el-button>
              <el-button v-else size="small" type="warning" text @click="unregisterEntry(row)">{{ t('externalNodes.unregisterEntry') }}</el-button>
              <el-button size="small" type="danger" text @click="deleteNode(row)">{{ t('common.delete') }}</el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>
      <MobileCards v-else :rows="nodesPagedList" title-field="name" :fields="extNodeCardFields">
        <template #actions="{ row }">
          <el-switch v-model="row.enabled" size="small" @change="updateNode(row)" />
          <el-tag v-if="registeredNodeIds.has(row.id)" size="small" type="success">{{ t('externalNodes.registered') }}</el-tag>
          <el-button size="small" text @click="editNode(row)">{{ t('common.edit') }}</el-button>
          <el-button v-if="!registeredNodeIds.has(row.id)" size="small" type="success" text @click="registerAsEntry(row)">{{ t('externalNodes.registerAsEntry') }}</el-button>
          <el-button v-else size="small" type="warning" text @click="unregisterEntry(row)">{{ t('externalNodes.unregisterEntry') }}</el-button>
          <el-button size="small" type="danger" text @click="deleteNode(row)">{{ t('common.delete') }}</el-button>
        </template>
      </MobileCards>
      <PaginationBar v-model:page="nodesPage" v-model:page-size="nodesSize" :total="nodesTotal" />
    </el-card>

    <!-- Import Dialog -->
    <el-dialog v-model="importDlg" :title="t('externalNodes.importDialogTitle')" width="700">
      <el-form label-width="100px" size="small">
        <el-form-item :label="t('externalNodes.name')">
          <el-input v-model="form.name" :placeholder="t('proto2.myNodes')" />
        </el-form-item>
        <el-form-item :label="t('externalNodes.sourceType')">
          <el-select v-model="form.source_type" style="width:200px">
            <el-option :label="t('externalNodes.v2rayUri')" value="uri" />
            <el-option :label="t('externalNodes.base64Uri')" value="base64_uri" />
            <el-option label="YAML" value="yaml" />
            <el-option label="JSON" value="json" />
            <el-option :label="t('externalNodes.subUrl')" value="url" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="form.source_type==='url'" :label="t('externalNodes.sourceUrl')">
          <el-input v-model="form.source_url" placeholder="https://..." />
        </el-form-item>
        <el-form-item v-else :label="t('externalNodes.sourceData')">
          <el-input v-model="form.source_data" type="textarea" :rows="8" :placeholder="t('externalNodes.pasteContent')" />
          <div style="margin-top:8px;display:flex;gap:8px;align-items:center">
            <el-upload :auto-upload="false" :show-file-list="false" accept=".yaml,.yml,.json,.txt,.list" @change="onFileUpload">
              <el-button size="small">{{ t('externalNodes.selectFile') }}</el-button>
            </el-upload>
            <span style="font-size:12px;color:#909399">{{ t('externalNodes.supportedFiles') }}</span>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="importDlg=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="doImport">{{ t('common.import') }}</el-button>
      </template>
    </el-dialog>

    <!-- Edit Node Dialog -->
    <el-dialog v-model="editDlg" :title="t('externalNodes.editNode')" width="600">
      <el-form label-width="100px" size="small">
        <el-form-item :label="t('common.name')">
          <el-input v-model="editForm.name" />
        </el-form-item>
        <!-- Access Domain（与 xray/sing-box 入站同一套逻辑） -->
        <el-divider content-position="left">{{ t('protoCenter.inbounds.accessDomain') }}</el-divider>
        <el-form-item :label="t('protoCenter.inbounds.enableAccessDomain')">
          <el-switch v-model="editForm.access_domain.enabled" />
          <span style="font-size:12px;color:#909399;margin-left:8px">{{ t('protoCenter.inbounds.accessDomainHint') }}</span>
        </el-form-item>
        <template v-if="editForm.access_domain.enabled">
          <el-form-item :label="t('protoCenter.inbounds.accessDomainType')">
            <el-select v-model="editForm.access_domain.type" style="width:100%" @change="onAccessDomainTypeChange">
              <el-option :label="t('protoCenter.inbounds.accessTypeDirect')" value="direct" />
              <el-option :label="t('protoCenter.inbounds.accessTypeCFDomain')" value="cf_domain" />
              <el-option :label="t('protoCenter.inbounds.accessTypeCFIP')" value="cf_ip" />
              <el-option :label="t('protoCenter.inbounds.accessTypeCustom')" value="custom" />
            </el-select>
          </el-form-item>
          <div v-if="!editAccessDomainSelected" class="hint warn-hint" style="margin-bottom:12px">
            {{ t('protoCenter.inbounds.accessDomainNotSelected') }}
          </div>
          <template v-if="editForm.access_domain.type === 'direct'">
            <el-form-item :label="t('protoCenter.inbounds.directDomain')">
              <el-select v-model="editForm.access_domain.directDomain" style="width:100%" filterable clearable :placeholder="t('protoCenter.inbounds.directDomainPlaceholder')">
                <el-option v-for="d in directDomains" :key="d.id" :label="d.value" :value="d.value" />
              </el-select>
              <div class="hint">{{ t('protoCenter.inbounds.directDomainHint') }}</div>
            </el-form-item>
            <el-form-item :label="t('protoCenter.inbounds.directPortMode')">
              <el-radio-group v-model="editForm.access_domain.directPortMode">
                <el-radio value="config">{{ t('protoCenter.inbounds.directPortConfig') }}</el-radio>
                <el-radio value="inbound">{{ t('protoCenter.inbounds.directPortInbound') }}</el-radio>
              </el-radio-group>
            </el-form-item>
          </template>
          <el-form-item v-if="editForm.access_domain.type === 'cf_domain'" :label="t('protoCenter.inbounds.cfDomainSelect')">
            <el-select v-model="editForm.access_domain.cfDomain" style="width:100%" filterable clearable :placeholder="t('protoCenter.inbounds.cfDomainPlaceholder')">
              <el-option v-for="d in cfDomains" :key="d.id" :label="d.value" :value="d.value" />
            </el-select>
            <div class="hint">{{ t('protoCenter.inbounds.cfDomainHint') }}</div>
          </el-form-item>
          <el-form-item v-if="editForm.access_domain.type === 'cf_ip'" :label="t('protoCenter.inbounds.cfIPSelect')">
            <el-select v-model="editForm.access_domain.cfIP" style="width:100%" filterable clearable :placeholder="t('protoCenter.inbounds.cfIPPlaceholder')">
              <el-option v-for="e in cfIPEntries" :key="e.id" :label="e.ip + (e.country ? ' (' + e.country + ')' : '')" :value="e.ip" />
            </el-select>
            <div class="hint">{{ t('protoCenter.inbounds.cfIPHint') }}</div>
          </el-form-item>
          <template v-if="editForm.access_domain.type === 'custom'">
            <el-form-item :label="t('protoCenter.inbounds.customDomain')">
              <el-input v-model="editForm.access_domain.customDomain" :placeholder="t('protoCenter.inbounds.customDomainPlaceholder')" />
            </el-form-item>
            <el-form-item :label="t('protoCenter.inbounds.directPortMode')">
              <el-radio-group v-model="editForm.access_domain.customPortMode">
                <el-radio value="custom">{{ t('protoCenter.inbounds.customPortCustom') }}</el-radio>
                <el-radio value="inbound">{{ t('protoCenter.inbounds.directPortInbound') }}</el-radio>
              </el-radio-group>
            </el-form-item>
            <el-form-item v-if="editForm.access_domain.customPortMode === 'custom'" :label="t('protoCenter.inbounds.customPort')">
              <el-input-number v-model="editForm.access_domain.customPort" :min="1" :max="65535" />
            </el-form-item>
          </template>
          <!-- 前置 TLS：核心明文、TLS 在 CDN（CF 或别家）/ nginx / caddy 上终结 -->
          <el-divider content-position="left">{{ t('protoCenter.inbounds.accessFrontTls') }}</el-divider>
          <el-form-item :label="t('protoCenter.inbounds.accessFrontTlsEnable')">
            <el-switch v-model="editForm.access_domain.tls.enabled" />
            <span style="font-size:12px;color:#909399;margin-left:8px">{{ t('protoCenter.inbounds.accessFrontTlsHint') }}</span>
          </el-form-item>
          <template v-if="editForm.access_domain.tls.enabled">
            <el-form-item :label="t('protoCenter.inbounds.accessFrontTlsSni')">
              <el-input v-model="editForm.access_domain.tls.sni" :placeholder="t('protoCenter.inbounds.accessFrontTlsSniHint')" />
            </el-form-item>
            <el-form-item :label="t('protoCenter.inbounds.accessFrontTlsAlpn')">
              <el-select
                :model-value="alpnToArray(editForm.access_domain.tls.alpn)"
                style="width:100%"
                multiple
                clearable
                placeholder="h2,http/1.1"
                @update:model-value="v => editForm.access_domain.tls.alpn = alpnToString(v)"
              >
                <el-option v-for="a in ALPN_OPTIONS" :key="a" :label="a" :value="a" />
              </el-select>
            </el-form-item>
            <el-form-item :label="t('protoCenter.inbounds.accessFrontTlsFp')">
              <el-select v-model="editForm.access_domain.tls.fingerprint" style="width:100%" clearable>
                <el-option label="chrome" value="chrome" /><el-option label="firefox" value="firefox" />
                <el-option label="safari" value="safari" /><el-option label="ios" value="ios" />
                <el-option label="android" value="android" /><el-option label="random" value="random" />
              </el-select>
            </el-form-item>
          </template>
        </template>
        <el-form-item :label="t('externalNodes.proxyConfig')">
          <el-input v-model="editForm.proxy_json" type="textarea" :rows="12" style="font-family:monospace;font-size:12px" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editDlg=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveNode">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- Register as Entry Dialog (§12.3) -->
    <el-dialog v-model="regDlg" :title="t('externalNodes.registerAsEntry')" width="520">
      <el-form label-width="100px" size="small">
        <el-form-item :label="t('externalNodes.externalNode')">
          <el-input :model-value="regForm.node_display" disabled />
        </el-form-item>
        <el-form-item :label="t('common.name')">
          <el-input v-model="regForm.name" :placeholder="t('externalNodes.nameHint')" />
        </el-form-item>
        <el-form-item :label="t('chainProxy.region')">
          <el-input v-model="regForm.region" placeholder="e.g. HK" />
        </el-form-item>
        <el-form-item :label="t('chainProxy.regionLabel')">
          <el-input v-model="regForm.region_label" placeholder="e.g. Hong Kong" />
        </el-form-item>
        <el-form-item :label="t('common.status')">
          <el-switch v-model="regForm.enabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="regDlg=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="regSaving" @click="doRegister">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import client from '../api/client'
import * as chainApi from '../api/chainlink'
import { useClientPagination } from '../composables/useClientPagination'
import { useIsMobile } from '../composables/useIsMobile'
import { ALPN_OPTIONS, alpnToArray, alpnToString } from './protocols/sections/utils/alpn'
import PaginationBar from '../components/PaginationBar.vue'
import MobileCards from '../components/MobileCards.vue'

const { t } = useI18n()

const sources = ref([])
const nodes = ref([])

const { paged: sourcesPagedList, page: sourcesPage, pageSize: sourcesSize, total: sourcesTotal } = useClientPagination(sources, { pageKey: 'extSrcPage', pageSizeKey: 'extSrcSize' })
const { paged: nodesPagedList, page: nodesPage, pageSize: nodesSize, total: nodesTotal } = useClientPagination(nodes, { pageKey: 'extNodePage', pageSizeKey: 'extNodeSize' })
const importDlg = ref(false)
const form = ref({ name: '', source_type: 'uri', source_data: '', source_url: '' })
const editDlg = ref(false)
const editForm = ref({ id: 0, name: '', proxy_json: '', port: 0, access_domain: defaultAccessDomain() })

// ── Access Domain（与 xray/sing-box 入站同一套语义）──
const directDomains = ref([])
const cfDomains = ref([])
const cfIPEntries = ref([])

function defaultAccessDomain() {
  return {
    enabled: false,
    type: 'direct',
    directDomain: '',
    directPortMode: 'config',
    cfDomain: '',
    cfIP: '',
    customDomain: '',
    customPortMode: 'custom',
    customPort: 443,
    // 前置 TLS：核心明文、TLS 在 CDN（CF 或别家）/ nginx / caddy 上终结时开启。
    tls: { enabled: false, sni: '', alpn: '', fingerprint: '' },
  }
}

function normalizeAccessDomain(ad) {
  const base = defaultAccessDomain()
  const merged = { ...base, ...(ad || {}) }
  if (!ad || ad.tls === undefined) {
    // 旧数据没有 tls：按类型给默认（CF 域名/IP 默认终结 TLS），
    // 与订阅侧的旧口径保持一致。
    merged.tls = {
      enabled: String(merged.type || '').startsWith('cf_'),
      sni: '', alpn: '', fingerprint: '',
    }
  } else {
    merged.tls = { ...base.tls, ...(ad.tls || {}) }
  }
  return merged
}

// 是否已选到可用值：
// - direct / custom 必须明确选择，否则回退导入的地址；
// - cf_domain / cf_ip 池是全局的，未指定时按设计从池里随机取，所以始终算已选择。
const editAccessDomainSelected = computed(() => {
  const ad = editForm.value.access_domain || {}
  switch (ad.type) {
    case 'direct': return !!ad.directDomain
    case 'cf_domain':
    case 'cf_ip': return true
    case 'custom': return !!ad.customDomain
    default: return false
  }
})

// 切换访问域名类型：CF 域名/IP 默认由 CDN 终结 TLS；直连/自定义默认跟随核心。
function onAccessDomainTypeChange(t) {
  const ad = editForm.value.access_domain
  if (!ad) return
  ad.tls = ad.tls || { enabled: false, sni: '', alpn: '', fingerprint: '' }
  ad.tls.enabled = String(t || '').startsWith('cf_')
  if (!ad.tls.enabled) {
    ad.tls.sni = ''
    ad.tls.alpn = ''
    ad.tls.fingerprint = ''
  }
}

function accessDomainHost(ad) {
  switch (ad?.type) {
    case 'direct': return ad.directDomain || ''
    case 'cf_domain': return ad.cfDomain || ''
    case 'cf_ip': return ad.cfIP || ''
    case 'custom': return ad.customDomain || ''
    default: return ''
  }
}

// 表格展示：启用且会改写地址时才显示，否则显示 '-'（实际使用导入的地址）
function accessAddress(row) {
  const ad = row?.access_domain
  if (!ad || !ad.enabled) return '-'
  const host = accessDomainHost(ad)
  if (!host) {
    // CF 池是全局的：未指定具体值时会从池里随机取
    if (ad.type === 'cf_domain' || ad.type === 'cf_ip') {
      const pool = ad.type === 'cf_domain' ? cfDomains.value : cfIPEntries.value
      return `${t('protoCenter.inbounds.accessDomainRandom')} (${pool.length})`
    }
    return '-'
  }
  // CF 域名/IP 的端口来自条目自身的端口池（渲染时随机），这里只展示地址
  if (ad.type === 'cf_domain' || ad.type === 'cf_ip') return host
  let port = 0
  if (ad.type === 'custom') {
    port = ad.customPortMode === 'inbound' ? (row.port || 0) : (ad.customPort || 0)
  } else if (ad.type === 'direct') {
    port = ad.directPortMode === 'inbound'
      ? (row.port || 0)
      : (directDomains.value.find(d => d.value === host)?.port || 0)
  }
  return port > 0 ? `${host}:${port}` : host
}

async function loadAccessDomainOptions() {
  try {
    const globalData = await client.get('/share-domains')
    const global = globalData?.items || (Array.isArray(globalData) ? globalData : [])
    cfDomains.value = global.filter(d => d.type === 'cf_domain' && d.enabled !== false)
    directDomains.value = global.filter(d => d.type === 'direct' && d.enabled !== false)
  } catch {}
  try {
    const data = await client.get('/cfip/entries')
    cfIPEntries.value = data?.items || (Array.isArray(data) ? data : [])
  } catch {}
}

// ── Register as Entry (§12.3) ──
const regDlg = ref(false)
const regSaving = ref(false)
const regForm = ref({ external_node_id: 0, node_display: '', name: '', region: '', region_label: '', enabled: true })
const registeredServers = ref([]) // external_node_servers from chain API
const registeredNodeIds = computed(() => new Set(registeredServers.value.map(s => s.external_node_id)))

const { isMobile } = useIsMobile()
// 手机端卡片字段（与桌面表格列对应）
const extSourceCardFields = computed(() => [
  { label: t('common.type'), text: (row) => row.source_type || '-' },
])
const extNodeCardFields = computed(() => [
  { label: t('externalNodes.protocol'), text: (row) => row.protocol || '-' },
  { label: t('externalNodes.address'), text: (row) => row.host || '-' },
  { label: t('externalNodes.port'), text: (row) => row.port ?? '-' },
  { label: t('protoCenter.inbounds.accessDomain'), text: (row) => accessAddress(row) },
  { label: t('externalNodes.transport'), text: (row) => row.transport || '-' },
  { label: 'TLS', text: () => 'TLS', hide: (row) => !row.tls },
])

async function fetchSources() {
  try { sources.value = (await client.get('/external-nodes/sources')) || [] } catch {}
}
async function fetchNodes() {
  try { nodes.value = (await client.get('/external-nodes')) || [] } catch {}
}
async function fetchRegisteredServers() {
  try { registeredServers.value = (await chainApi.listExternalServers()) || [] } catch {}
}
function showImport() {
  form.value = { name: '', source_type: 'uri', source_data: '', source_url: '' }
  importDlg.value = true
}
function onFileUpload(file) {
  const reader = new FileReader()
  reader.onload = (e) => {
    form.value.source_data = e.target.result
    // Auto-detect type from file extension
    const name = file.name || ''
    if (name.endsWith('.yaml') || name.endsWith('.yml')) {
      form.value.source_type = 'yaml'
    } else if (name.endsWith('.json')) {
      form.value.source_type = 'json'
    }
    if (!form.value.name) {
      form.value.name = name.replace(/\.[^.]+$/, '')
    }
  }
  reader.readAsText(file.raw)
}
async function doImport() {
  try {
    const res = await client.post('/external-nodes/sources', form.value)
    ElMessage.success(t('externalNodes.importSuccess', { count: res?.node_count || 0 }))
    importDlg.value = false
    fetchSources()
    fetchNodes()
  } catch (e) { ElMessage.error(e?.message || t('externalNodes.importError')) }
}
async function updateSource(row) {
  try { await client.put(`/external-nodes/sources/${row.id}`, { enabled: row.enabled, name: row.name }) } catch {}
}
async function deleteSource(row) {
  await ElMessageBox.confirm(t('externalNodes.confirmDeleteSource'))
  try { await client.delete(`/external-nodes/sources/${row.id}`); fetchSources(); fetchNodes() } catch {}
}
function editNode(row) {
  editForm.value = {
    id: row.id,
    name: row.name,
    proxy_json: row.proxy_json,
    port: row.port || 0,
    access_domain: normalizeAccessDomain(row.access_domain),
  }
  editDlg.value = true
}
async function saveNode() {
  try {
    await client.put(`/external-nodes/${editForm.value.id}`, {
      name: editForm.value.name,
      proxy_json: editForm.value.proxy_json,
      access_domain: editForm.value.access_domain,
    })
    editDlg.value = false
    fetchNodes()
    ElMessage.success(t('common.success'))
  } catch (e) { ElMessage.error(e?.message || t('common.failed')) }
}
async function updateNode(row) {
  try { await client.put(`/external-nodes/${row.id}`, { enabled: row.enabled }) } catch {}
}
async function deleteNode(row) {
  await ElMessageBox.confirm(t('externalNodes.confirmDeleteNode'))
  try { await client.delete(`/external-nodes/${row.id}`); fetchNodes() } catch {}
}

// ── Register as Entry (§12.3) ──
function registerAsEntry(row) {
  regForm.value = {
    external_node_id: row.id,
    node_display: `${row.name} (${row.host || row.protocol || ''})`,
    name: row.name,
    region: '',
    region_label: '',
    enabled: true,
  }
  regDlg.value = true
}

async function doRegister() {
  if (!regForm.value.external_node_id) {
    ElMessage.warning(t('chainProxy.selectExternalNode'))
    return
  }
  regSaving.value = true
  try {
    await chainApi.createExternalServer({
      external_node_id: regForm.value.external_node_id,
      name: regForm.value.name,
      region: regForm.value.region,
      region_label: regForm.value.region_label,
      enabled: regForm.value.enabled,
    })
    ElMessage.success(t('common.success'))
    regDlg.value = false
    await fetchRegisteredServers()
  } catch (e) { ElMessage.error(e?.message || t('common.failed')) } finally { regSaving.value = false }
}

async function unregisterEntry(row) {
  const server = registeredServers.value.find(s => s.external_node_id === row.id)
  if (!server) return
  try {
    await ElMessageBox.confirm(t('chainProxy.confirmDeleteExtServer'), t('common.confirm'), { type: 'warning' })
    await chainApi.deleteExternalServer(server.id)
    ElMessage.success(t('common.success'))
    await fetchRegisteredServers()
  } catch { /* cancelled */ }
}

onMounted(() => { fetchSources(); fetchNodes(); fetchRegisteredServers(); loadAccessDomainOptions() })

defineExpose({ fetchSources, fetchNodes, fetchRegisteredServers })
</script>

<style scoped>
.warn-hint { color: #e6a23c; }
</style>
