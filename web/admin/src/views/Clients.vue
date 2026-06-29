<template>
  <div>
    <el-tabs v-model="activeTab">
      <!-- Plans Tab -->
      <el-tab-pane :label="t('client.plans')" name="plans">
        <div style="display:flex;justify-content:space-between;margin-bottom:16px">
          <h3>{{ t('client.plans') }}</h3>
          <el-button type="primary" size="small" @click="showCreatePlan">{{ t('client.createPlan') }}</el-button>
        </div>
        <el-table :data="plans" v-loading="loading" stripe size="small">
          <el-table-column prop="id" label="ID" width="50" />
          <el-table-column prop="name" :label="t('client.planName')" width="150" />
          <el-table-column :label="t('client.quota')" width="120">
            <template #default="{row}">{{ formatBytes(row.quota_bytes) }}</template>
          </el-table-column>
          <el-table-column :label="t('client.duration')" width="80">
            <template #default="{row}">{{ row.duration }}天</template>
          </el-table-column>
          <el-table-column prop="description" :label="t('client.description')" show-overflow-tooltip />
          <el-table-column :label="t('common.status')" width="70">
            <template #default="{row}"><el-tag :type="row.enabled?'success':'danger'" size="small">{{ row.enabled?'✓':'✗' }}</el-tag></template>
          </el-table-column>
          <el-table-column :label="t('common.actions')" width="200">
            <template #default="{row}">
              <el-button size="small" @click="showPlanNodes(row)">{{ t('client.nodes') }}</el-button>
              <el-button size="small" @click="showEditPlan(row)">{{ t('common.edit') }}</el-button>
              <el-button size="small" type="danger" @click="deletePlan(row)">{{ t('common.delete') }}</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <!-- Clients Tab -->
      <el-tab-pane :label="t('client.clients')" name="clients">
        <div style="display:flex;justify-content:space-between;margin-bottom:16px">
          <div style="display:flex;gap:8px">
            <h3>{{ t('client.clients') }}</h3>
            <el-select v-model="filterPlan" size="small" clearable :placeholder="t('client.filterPlan')" style="width:150px" @change="fetchClients">
              <el-option v-for="p in plans" :key="p.id" :label="p.name" :value="p.id" />
            </el-select>
          </div>
          <el-button type="primary" size="small" @click="showCreateClient">{{ t('client.createClient') }}</el-button>
        </div>
        <el-table :data="clients" v-loading="clientLoading" stripe size="small">
          <el-table-column prop="id" label="ID" width="50" />
          <el-table-column prop="email" :label="t('client.email')" width="180" />
          <el-table-column :label="t('client.plan')" width="100">
            <template #default="{row}">{{ planName(row.plan_id) }}</template>
          </el-table-column>
          <el-table-column :label="t('client.subscribeUrl')" width="80">
            <template #default="{row}">
              <el-button v-if="getClientToken(row)" size="small" type="primary" @click="showSubDetail(row)">{{ t('client.viewDetail') }}</el-button>
              <span v-else>-</span>
            </template>
          </el-table-column>
          <el-table-column :label="t('client.usage')" width="160">
            <template #default="{row}">
              <el-progress :percentage="usagePercent(row)" :color="usageColor(row)" :stroke-width="12" :format="() => formatBytes(row.used_bytes)" />
            </template>
          </el-table-column>
          <el-table-column :label="t('client.quota')" width="100">
            <template #default="{row}">{{ formatBytes(row.quota_bytes) }}</template>
          </el-table-column>
          <el-table-column :label="t('client.expires')" width="100">
            <template #default="{row}">{{ row.expires_at ? row.expires_at.substring(0,10) : '-' }}</template>
          </el-table-column>
          <el-table-column :label="t('common.status')" width="80">
            <template #default="{row}"><el-tag :type="statusType(row.status)" size="small">{{ row.status }}</el-tag></template>
          </el-table-column>
          <el-table-column :label="t('common.actions')" width="400">
            <template #default="{row}">
              <el-button v-if="row.status==='active'" size="small" type="warning" @click="suspendClient(row)">{{ t('client.suspend') }}</el-button>
              <el-button v-if="row.status==='suspended'" size="small" type="success" @click="restoreClient(row)">{{ t('client.restore') }}</el-button>
              <el-button size="small" @click="resetUsage(row)">{{ t('client.resetUsage') }}</el-button>
              <el-button size="small" @click="showResetPwd(row)">{{ t('client.resetPwd') }}</el-button>
              <el-button size="small" @click="showEditClient(row)">{{ t('common.edit') }}</el-button>
              <el-button size="small" type="danger" @click="deleteClient(row)">{{ t('common.delete') }}</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>
    </el-tabs>

    <!-- Plan Dialog -->
    <el-dialog v-model="planDialog" :title="editingPlan?t('client.editPlan'):t('client.createPlan')" width="450">
      <el-form :model="planForm" label-width="100px">
        <el-form-item :label="t('client.planName')"><el-input v-model="planForm.name" /></el-form-item>
        <el-form-item :label="t('client.quota')"><el-input v-model.number="planForm.quota_gb" type="number" :min="0" :step="0.1" /><div class="hint">GB (0=不限)</div></el-form-item>
        <el-form-item :label="t('client.duration')"><el-input v-model.number="planForm.duration" type="number" /><div class="hint">天数</div></el-form-item>
        <el-form-item :label="t('client.description')"><el-input v-model="planForm.description" type="textarea" :rows="2" /></el-form-item>
        <el-form-item :label="t('common.enabled')"><el-switch v-model="planForm.enabled" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="planDialog=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="savePlan">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- Plan Nodes Dialog -->
    <el-dialog v-model="nodesDialog" :title="currentPlan?.name + ' - ' + t('client.nodes')" width="600">
      <div style="margin-bottom:12px;display:flex;gap:8px;flex-wrap:wrap">
        <el-select v-model="newNodeId" size="small" filterable :placeholder="t('client.selectInbound')" style="width:250px">
          <el-option v-for="ib in inbounds" :key="ib.id" :label="(ib.remark || ib.tag) + ' (' + ib.protocol + ':' + ib.port + ')'" :value="ib.id" />
        </el-select>
        <el-button size="small" type="primary" @click="addPlanNode">{{ t('client.addNode') }}</el-button>
        <el-select v-model="newExtNodeId" size="small" filterable placeholder="外部节点" style="width:250px">
          <el-option v-for="en in extNodes" :key="en.id" :label="en.name + ' (' + en.host + ':' + en.port + ')'" :value="en.id" />
        </el-select>
        <el-button size="small" type="success" @click="addExtNodeToPlan">添加外部节点</el-button>
      </div>
      <el-table :data="planNodes" stripe size="small">
        <el-table-column prop="id" label="ID" width="50" />
        <el-table-column label="节点名称">
          <template #default="{row}">
            <template v-if="row.inbound_config_id">{{ inboundName(row.inbound_config_id) }}</template>
            <template v-else-if="row.external_node_id">{{ extNodeName(row.external_node_id) }}</template>
            <template v-else>-</template>
          </template>
        </el-table-column>
        <el-table-column label="类型" width="80">
          <template #default="{row}">
            <el-tag v-if="row.inbound_config_id" size="small">入站</el-tag>
            <el-tag v-else-if="row.external_node_id" size="small" type="success">外部</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('common.actions')" width="80">
          <template #default="{row}"><el-button size="small" type="danger" @click="deletePlanNode(row)">{{ t('common.delete') }}</el-button></template>
        </el-table-column>
      </el-table>
    </el-dialog>

    <!-- Client Dialog -->
    <el-dialog v-model="clientDialog" :title="editingClient?t('client.editClient'):t('client.createClient')" width="450">
      <el-form :model="clientForm" label-width="100px">
        <el-form-item :label="t('client.email')"><el-input v-model="clientForm.email" :disabled="!!editingClient" /></el-form-item>
        <el-form-item :label="t('client.plan')">
          <el-select v-model="clientForm.plan_id" style="width:100%">
            <el-option v-for="p in plans" :key="p.id" :label="p.name" :value="p.id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('client.quota')"><el-input v-model.number="clientForm.quota_gb" type="number" :min="0" :step="0.1" /><div class="hint">GB (0=用套餐默认)</div></el-form-item>
        <el-form-item :label="t('client.expires')"><el-date-picker v-model="clientForm.expires_at" type="date" value-format="YYYY-MM-DD" style="width:100%" /></el-form-item>
        <el-form-item :label="t('client.remark')"><el-input v-model="clientForm.remark" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="clientDialog=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveClient">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- Reset Password Dialog -->
    <el-dialog v-model="resetPwdDialog" :title="t('client.resetPwd')" width="400">
      <el-form :model="resetPwdForm" label-width="80px">
        <el-form-item :label="t('client.email')">
          <el-input :model-value="resetPwdForm.email" disabled />
        </el-form-item>
        <el-form-item :label="t('client.newPwd')">
          <el-input v-model="resetPwdForm.password" type="password" show-password placeholder="请输入新密码" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="resetPwdDialog=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="resetPwd">{{ t('common.confirm') }}</el-button>
      </template>
    </el-dialog>

    <!-- Subscription Detail Dialog -->
    <el-dialog v-model="subDetailDialog" :title="t('client.subDetail')" width="420">
      <div style="text-align:center">
        <div style="margin-bottom:12px;font-weight:500">{{ subDetailEmail }}</div>
        <div class="sub-link" style="word-break:break-all;font-family:monospace;font-size:12px;background:#f5f7fa;padding:10px;border-radius:6px;text-align:left">
          {{ subDetailUrl }}
          <el-button size="small" text @click="copyText(subDetailUrl)" style="float:right">复制</el-button>
        </div>
        <div v-if="subDetailQr" style="margin:16px 0">
          <img :src="subDetailQr" style="max-width:200px;border:1px solid #eee;border-radius:8px" />
          <div style="font-size:12px;color:#909399;margin-top:8px">{{ t('client.scanToImport') }}</div>
        </div>
        <div v-else-if="subDetailQrLoading" style="margin:16px 0;height:220px;display:flex;align-items:center;justify-content:center;color:#909399">
          {{ t('client.qrGenerating') }}
        </div>
        <el-button v-else size="small" :loading="subDetailQrLoading" @click="generateSubDetailQr" style="margin-top:8px">{{ t('client.showQrCode') }}</el-button>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { generateQrDataUrl, isQrDataTooLargeError } from '../utils/qrcode'
import client from '../api/client'

const { t } = useI18n()
const activeTab = ref('plans')
const panelSettings = ref({ listen_port: '8080', subscription_port: '0' })

// Plans
const plans = ref([])
const loading = ref(false)
const planDialog = ref(false)
const editingPlan = ref(null)
const planForm = ref({ name: '', description: '', quota_bytes: 0, duration: 30, enabled: true })

// Plan Nodes
const nodesDialog = ref(false)
const currentPlan = ref(null)
const planNodes = ref([])
const inbounds = ref([])
const newNodeId = ref(null)
const extNodes = ref([])
const newExtNodeId = ref(null)

// Clients
const clients = ref([])
const subscriptions = ref([])
const endpoints = ref([])
const clientLoading = ref(false)
const clientDialog = ref(false)
const editingClient = ref(null)
const clientForm = ref({ email: '', plan_id: null, quota_gb: 0, expires_at: '', remark: '' })
const filterPlan = ref(null)

// Reset Password
const resetPwdDialog = ref(false)
const resetPwdForm = ref({ id: 0, email: '', password: '' })

// Subscription Detail
const subDetailDialog = ref(false)
const subDetailEmail = ref('')
const subDetailUrl = ref('')
const subDetailQr = ref('')
const subDetailQrLoading = ref(false)

// Get subscription token for a client
const getClientToken = (row) => {
  if (!row.subscription_id) return ''
  const sub = subscriptions.value.find(s => s.id === row.subscription_id)
  return sub?.token || ''
}
const subUrl = (row) => {
  const token = getClientToken(row)
  if (!token) return ''
  const ep = endpoints.value.find(e => e.enabled) || endpoints.value[0]

  if (ep && ep.domain) {
    // 有端点域名：用端点的路径 + token
    const host = ep.domain
    const path = ep.path || '/'
    const basePath = path.endsWith('/') ? path : path + '/'
    return 'https://' + host + basePath + token
  }

  // 无端点域名：用服务器地址
  const host = window.location.hostname
  const subPort = panelSettings.value.subscription_port
  const isDedicatedPort = subPort && subPort !== '0' && subPort !== panelSettings.value.listen_port
  if (isDedicatedPort) {
    return 'https://' + host + ':' + subPort + '/' + token
  }
  return 'https://' + host + '/sub/' + token
}
async function copyToken(row) {
  const token = getClientToken(row)
  if (token) { await navigator.clipboard.writeText(token); ElMessage.success(t('common.copied')) }
}
async function copySubUrl(row) {
  const url = subUrl(row)
  if (url) { await navigator.clipboard.writeText(url); ElMessage.success(t('common.copied')) }
}

// Helpers
const formatBytes = (b) => {
  if (!b || b <= 0) return '不限'
  if (b >= 1073741824) return (b / 1073741824).toFixed(1) + ' GB'
  if (b >= 1048576) return (b / 1048576).toFixed(0) + ' MB'
  return (b / 1024).toFixed(0) + ' KB'
}
const planName = (id) => plans.value.find(p => p.id === id)?.name || '-'
const usagePercent = (row) => row.quota_bytes > 0 ? Math.min(100, Math.round(row.used_bytes / row.quota_bytes * 100)) : 0
const usageColor = (row) => usagePercent(row) > 80 ? '#f56c6c' : '#409eff'
const statusType = (s) => ({ active: 'success', suspended: 'danger', expired: 'warning' }[s] || 'info')
const inboundTag = (id) => inbounds.value.find(ib => ib.id === id)?.tag || '#' + id
const inboundName = (id) => { const ib = inbounds.value.find(ib => ib.id === id); return ib ? (ib.remark || ib.tag) : '#' + id }
const extNodeName = (id) => { const n = extNodes.value.find(n => n.id === id); return n ? n.name : '#' + id }

// Fetch
async function fetchPlans() { loading.value = true; try { plans.value = (await client.get('/plans')) || [] } catch {} finally { loading.value = false } }
async function fetchClients() {
  clientLoading.value = true
  try {
    const p = {}; if (filterPlan.value) p.plan_id = filterPlan.value
    const [clientRes, epRes] = await Promise.all([
      client.get('/clients', { params: { limit: 100, ...p } }),
      client.get('/subscription-endpoints')
    ])
    clients.value = clientRes.items || []
    endpoints.value = Array.isArray(epRes) ? epRes : []
    // Fetch full token for each client's subscription
    const subIds = clients.value.filter(c => c.subscription_id).map(c => c.subscription_id)
    const subs = await Promise.all(subIds.map(id => client.get(`/subscriptions/${id}`).catch(() => null)))
    subscriptions.value = subs.filter(Boolean)
  } catch {} finally { clientLoading.value = false }
}
async function fetchInbounds() { try { const r = await client.get('/inbound-configs'); inbounds.value = Array.isArray(r) ? r : r.data || [] } catch {} }

// Plan CRUD
function showCreatePlan() {
  editingPlan.value = null
  planForm.value = { name: '', description: '', quota_gb: 0, duration: 30, enabled: true }
  planDialog.value = true
}
function showEditPlan(row) {
  editingPlan.value = row
  planForm.value = {
    name: row.name,
    description: row.description,
    quota_gb: row.quota_bytes / 1073741824,
    duration: row.duration,
    enabled: row.enabled
  }
  planDialog.value = true
}
async function savePlan() {
  try {
    const payload = { ...planForm.value, quota_bytes: Math.round((planForm.value.quota_gb || 0) * 1073741824) }
    delete payload.quota_gb
    if (editingPlan.value) { await client.put(`/plans/${editingPlan.value.id}`, payload) } else { await client.post('/plans', payload) }
    ElMessage.success(t('common.success')); planDialog.value = false; fetchPlans()
  } catch {}
}
async function deletePlan(row) { await ElMessageBox.confirm(t('client.deletePlanConfirm'), t('common.confirm')); await client.delete(`/plans/${row.id}`); ElMessage.success(t('common.success')); fetchPlans() }

// Plan Nodes
async function showPlanNodes(row) { currentPlan.value = row; nodesDialog.value = true; try { const r = await client.get(`/plans/${row.id}`); planNodes.value = r.nodes || [] } catch { planNodes.value = [] } }
async function addPlanNode() { if (!newNodeId.value) return; await client.post(`/plans/${currentPlan.value.id}/nodes`, { inbound_config_id: newNodeId.value }); ElMessage.success(t('common.success')); newNodeId.value = null; showPlanNodes(currentPlan.value) }
async function fetchExtNodes() { try { extNodes.value = (await client.get('/external-nodes')) || [] } catch {} }
async function addExtNodeToPlan() {
  if (!newExtNodeId.value) return
  const node = extNodes.value.find(n => n.id === newExtNodeId.value)
  if (!node) return
  await client.post(`/plans/${currentPlan.value.id}/nodes`, { external_node_id: newExtNodeId.value })
  ElMessage.success(`已添加: ${node.name}`)
  newExtNodeId.value = null
  showPlanNodes(currentPlan.value)
}
async function deletePlanNode(row) { await client.delete(`/plans/${currentPlan.value.id}/nodes/${row.id}`); ElMessage.success(t('common.success')); showPlanNodes(currentPlan.value) }

// Client CRUD
function showCreateClient() { editingClient.value = null; clientForm.value = { email: '', plan_id: plans.value[0]?.id || null, quota_gb: 0, expires_at: '', remark: '' }; clientDialog.value = true }
function showEditClient(row) { editingClient.value = row; clientForm.value = { email: row.email, plan_id: row.plan_id, quota_gb: row.quota_bytes / 1073741824, expires_at: row.expires_at?.substring(0,10) || '', remark: row.remark || '' }; clientDialog.value = true }
async function saveClient() {
  try {
    const payload = { ...clientForm.value, quota_bytes: Math.round((clientForm.value.quota_gb || 0) * 1073741824) }
    delete payload.quota_gb
    if (editingClient.value) { await client.put(`/clients/${editingClient.value.id}`, { plan_id: payload.plan_id, quota_bytes: payload.quota_bytes, expires_at: payload.expires_at, remark: payload.remark }) } else { await client.post('/clients', payload) }
    ElMessage.success(t('common.success')); clientDialog.value = false; fetchClients()
  } catch {}
}
async function deleteClient(row) { await ElMessageBox.confirm(t('client.deleteClientConfirm', { email: row.email }), t('common.confirm')); await client.delete(`/clients/${row.id}`); ElMessage.success(t('common.success')); fetchClients() }
async function suspendClient(row) { await client.post(`/clients/${row.id}/suspend`); ElMessage.success(t('common.success')); fetchClients() }
async function restoreClient(row) { await client.post(`/clients/${row.id}/restore`); ElMessage.success(t('common.success')); fetchClients() }
async function resetUsage(row) { await client.post(`/clients/${row.id}/reset-usage`); ElMessage.success(t('common.success')); fetchClients() }

function showResetPwd(row) { resetPwdForm.value = { id: row.id, email: row.email, password: '' }; resetPwdDialog.value = true }
async function resetPwd() {
  if (!resetPwdForm.value.password || resetPwdForm.value.password.length < 6) { ElMessage.error(t('client.pwdMinLen')); return }
  try {
    await client.put(`/clients/${resetPwdForm.value.id}`, { password: resetPwdForm.value.password })
    ElMessage.success(t('client.pwdResetSuccess'))
    resetPwdDialog.value = false
  } catch {}
}

function showSubDetail(row) {
  subDetailEmail.value = row.email
  subDetailUrl.value = subUrl(row)
  subDetailQr.value = ''
  subDetailQrLoading.value = false
  subDetailDialog.value = true
  void generateSubDetailQr()
}

async function generateSubDetailQr() {
  if (!subDetailUrl.value) return
  subDetailQrLoading.value = true
  subDetailQr.value = ''
  try {
    subDetailQr.value = await generateQrDataUrl(subDetailUrl.value, { width: 200 })
  } catch (error) {
    const message = isQrDataTooLargeError(error) ? t('client.qrDataTooLarge') : t('client.qrGenerateFailed')
    ElMessage.error(message)
  } finally {
    subDetailQrLoading.value = false
  }
}

function copyText(text) {
  if (navigator.clipboard && navigator.clipboard.writeText) {
    navigator.clipboard.writeText(text).then(() => ElMessage.success(t('common.copied')))
  } else {
    const ta = document.createElement('textarea')
    ta.value = text
    ta.style.position = 'fixed'
    ta.style.left = '-9999px'
    document.body.appendChild(ta)
    ta.select()
    document.execCommand('copy')
    document.body.removeChild(ta)
    ElMessage.success(t('common.copied'))
  }
}

onMounted(() => { fetchPlans(); fetchClients(); fetchInbounds(); fetchPanelSettings(); fetchExtNodes() })

async function fetchPanelSettings() {
  try {
    const d = await client.get('/system/panel-config')
    if (d) panelSettings.value = { ...panelSettings.value, ...d }
  } catch {}
}
</script>
<style scoped>.hint { font-size:12px; color:#909399; margin-top:4px }</style>
