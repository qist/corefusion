<template>
  <div>
    <!-- Client List -->
    <div style="display:flex;justify-content:space-between;margin-bottom:16px">
      <div style="display:flex;gap:8px">
        <h3>{{ t('client.clients') }}</h3>
        <el-select v-model="filterPlan" size="small" clearable :placeholder="t('client.filterPlan')" style="width:180px" @change="fetchClients">
          <el-option v-for="tpl in planTemplates" :key="tpl.id" :label="tpl.name" :value="tpl.id" />
        </el-select>
      </div>
      <el-button type="primary" size="small" @click="showCreateClient">{{ t('client.createClient') }}</el-button>
    </div>
    <el-table :data="clients" v-loading="clientLoading" stripe size="small">
      <el-table-column prop="id" label="ID" width="50" />
      <el-table-column prop="email" :label="t('client.email')" width="180" />
      <el-table-column :label="t('client.plan')" width="180">
        <template #default="{row}">
          {{ templateName(row.plan_id) }}
          <span v-if="row.plan_variant_id" style="color:#909399;font-size:12px">
            / {{ variantName(row.plan_variant_id) }}
          </span>
        </template>
      </el-table-column>
      <el-table-column label="下载限速" width="80">
        <template #default="{row}">{{ row.speed_limit_mbps ? row.speed_limit_mbps + 'Mbps' : '-' }}</template>
      </el-table-column>
      <el-table-column label="上传限速" width="80">
        <template #default="{row}">{{ row.speed_limit_up_mbps ? row.speed_limit_up_mbps + 'Mbps' : '-' }}</template>
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
      <el-table-column :label="t('common.actions')" width="520">
        <template #default="{row}">
          <el-button size="small" type="primary" @click="showRenewDialog(row)">续费</el-button>
          <el-button size="small" type="success" @click="showUpgradeDialog(row)">升级</el-button>
          <el-button size="small" @click="showDowngradeDialog(row)">降级</el-button>
          <el-button size="small" @click="showOrdersDialog(row)">订单</el-button>
          <el-button v-if="row.status==='active'" size="small" type="warning" @click="suspendClient(row)">{{ t('client.suspend') }}</el-button>
          <el-button v-if="row.status==='suspended'" size="small" type="success" @click="restoreClient(row)">{{ t('client.restore') }}</el-button>
          <el-button size="small" @click="resetUsage(row)">{{ t('client.resetUsage') }}</el-button>
          <el-button size="small" @click="showResetPwd(row)">{{ t('client.resetPwd') }}</el-button>
          <el-button size="small" @click="showEditClient(row)">{{ t('common.edit') }}</el-button>
          <el-button size="small" type="danger" @click="deleteClient(row)">{{ t('common.delete') }}</el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- Client Dialog -->
    <el-dialog v-model="clientDialog" :title="editingClient?t('client.editClient'):t('client.createClient')" width="500">
      <el-form :model="clientForm" label-width="100px">
        <el-form-item :label="t('client.email')"><el-input v-model="clientForm.email" :disabled="!!editingClient" /></el-form-item>
        <el-form-item label="套餐模板">
          <el-select v-model="clientForm.plan_id" style="width:100%" @change="onTemplateChange">
            <el-option v-for="tpl in planTemplates" :key="tpl.id" :label="tpl.name + ' (' + tpl.category + ')'" :value="tpl.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="套餐规格">
          <el-select v-model="clientForm.plan_variant_id" style="width:100%" :disabled="!clientForm.plan_id">
            <el-option v-for="v in availableVariants" :key="v.id" :label="v.name + ' - ¥' + (v.price/100).toFixed(2) + ' / ' + v.duration_days + '天'" :value="v.id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('client.quota')"><el-input v-model.number="clientForm.quota_gb" type="number" :min="0" :step="0.1" /><div class="hint">GB (0=用套餐默认)</div></el-form-item>
        <el-form-item label="下载限速(Mbps)"><el-input-number v-model="clientForm.speed_limit_mbps" :min="0" size="small" /><div class="hint">0=继承模板</div></el-form-item>
        <el-form-item label="上传限速(Mbps)"><el-input-number v-model="clientForm.speed_limit_up_mbps" :min="0" size="small" /><div class="hint">0=继承模板</div></el-form-item>
        <el-form-item label="并发限制"><el-input-number v-model="clientForm.concurrent_limit" :min="0" size="small" /><div class="hint">0=继承模板</div></el-form-item>
        <el-form-item :label="t('client.expires')"><el-date-picker v-model="clientForm.expires_at" type="date" value-format="YYYY-MM-DD" style="width:100%" /></el-form-item>
        <el-form-item :label="t('client.remark')"><el-input v-model="clientForm.remark" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="clientDialog=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveClient">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- 续费弹窗 -->
    <el-dialog v-model="renewDialog" title="续费" width="450px">
      <el-form :model="renewForm" label-width="100px" size="small">
        <el-form-item label="客户端">{{ renewForm.email }}</el-form-item>
        <el-form-item label="选择规格">
          <el-select v-model="renewForm.variant_id" style="width:100%" placeholder="选择续费规格">
            <el-option v-for="v in allVariants" :key="v.id" :label="v.name + ' - ¥' + (v.price/100).toFixed(2) + ' / ' + v.duration_days + '天'" :value="v.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="优惠码">
          <div style="display:flex;gap:8px">
            <el-input v-model="renewForm.promotion_code" placeholder="输入优惠码" />
            <el-button @click="validateRenewPromo">验证</el-button>
          </div>
        </el-form-item>
        <el-form-item v-if="renewPromoResult" label="优惠结果">
          <div style="font-size:12px">
            <el-tag :type="renewPromoResult.valid?'success':'danger'" size="small">{{ renewPromoResult.valid?'有效':'无效' }}</el-tag>
            <span v-if="renewPromoResult.valid" style="margin-left:8px">
              ¥{{ (renewPromoResult.original_amount/100).toFixed(2) }} → ¥{{ (renewPromoResult.paid_amount/100).toFixed(2) }}
              <span style="color:#f56c6c">(-¥{{ (renewPromoResult.discount_amount/100).toFixed(2) }})</span>
            </span>
            <span v-else style="margin-left:8px;color:#f56c6c">{{ renewPromoResult.error_message }}</span>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="renewDialog=false">取消</el-button>
        <el-button type="primary" :loading="renewLoading" @click="doRenew">确认续费</el-button>
      </template>
    </el-dialog>

    <!-- 升级弹窗 -->
    <el-dialog v-model="upgradeDialog" title="升级套餐" width="450px">
      <el-form :model="upgradeForm" label-width="100px" size="small">
        <el-form-item label="客户端">{{ upgradeForm.email }}</el-form-item>
        <el-form-item label="升级到规格">
          <el-select v-model="upgradeForm.variant_id" style="width:100%" placeholder="选择升级目标规格">
            <el-option v-for="v in allVariants" :key="v.id" :label="v.name + ' - ¥' + (v.price/100).toFixed(2) + ' / ' + v.duration_days + '天'" :value="v.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="优惠码">
          <el-input v-model="upgradeForm.promotion_code" placeholder="输入优惠码" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="upgradeDialog=false">取消</el-button>
        <el-button type="primary" :loading="upgradeLoading" @click="doUpgrade">确认升级</el-button>
      </template>
    </el-dialog>

    <!-- 降级弹窗 -->
    <el-dialog v-model="downgradeDialog" title="降级套餐" width="450px">
      <el-form :model="downgradeForm" label-width="100px" size="small">
        <el-form-item label="客户端">{{ downgradeForm.email }}</el-form-item>
        <el-form-item label="到期后降级到">
          <el-select v-model="downgradeForm.variant_id" style="width:100%" placeholder="选择降级目标规格">
            <el-option v-for="v in allVariants" :key="v.id" :label="v.name + ' - ¥' + (v.price/100).toFixed(2) + ' / ' + v.duration_days + '天'" :value="v.id" />
          </el-select>
        </el-form-item>
        <div style="color:#e6a23c;font-size:12px;margin-left:100px">降级将在当前套餐到期后生效</div>
      </el-form>
      <template #footer>
        <el-button @click="downgradeDialog=false">取消</el-button>
        <el-button type="primary" :loading="downgradeLoading" @click="doDowngrade">确认降级</el-button>
      </template>
    </el-dialog>

    <!-- 订单历史弹窗 -->
    <el-dialog v-model="ordersDialog" title="订单历史" width="750px">
      <el-table :data="clientOrders" stripe size="small">
        <el-table-column prop="id" label="ID" width="50" />
        <el-table-column label="类型" width="80">
          <template #default="{row}">
            <el-tag :type="orderTypeColor(row.order_type)" size="small">{{ orderTypeLabel(row.order_type) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="to_variant_id" label="规格ID" width="70" />
        <el-table-column label="金额" width="120">
          <template #default="{row}">¥{{ (row.paid_amount/100).toFixed(2) }}</template>
        </el-table-column>
        <el-table-column prop="promotion_code" label="优惠码" width="100" />
        <el-table-column label="生效时间" width="150">
          <template #default="{row}">{{ row.effective_from?.substring(0,10) }}</template>
        </el-table-column>
        <el-table-column label="到期时间" width="150">
          <template #default="{row}">{{ row.effective_to?.substring(0,10) || '-' }}</template>
        </el-table-column>
        <el-table-column label="状态" width="80">
          <template #default="{row}">
            <el-tag :type="orderStatusColor(row.status)" size="small">{{ orderStatusLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
      </el-table>
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
import { ref, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { generateQrDataUrl, isQrDataTooLargeError } from '../utils/qrcode'
import client from '../api/client'
import * as planApi from '../api/plan'

const { t } = useI18n()
const panelSettings = ref({ listen_port: '8080', subscription_port: '0' })

// Plan v2: Templates & Variants
const planTemplates = ref([])
const allVariants = ref([])
const availableVariants = computed(() => {
  if (!clientForm.value.plan_id) return []
  return allVariants.value.filter(v => v.template_id === clientForm.value.plan_id)
})
const templateName = (id) => planTemplates.value.find(tpl => tpl.id === id)?.name || '-'
const variantName = (id) => allVariants.value.find(v => v.id === id)?.name || '-'

// Clients
const clients = ref([])
const subscriptions = ref([])
const endpoints = ref([])
const clientLoading = ref(false)
const clientDialog = ref(false)
const editingClient = ref(null)
const clientForm = ref({ email: '', plan_id: null, plan_variant_id: null, quota_gb: 0, speed_limit_mbps: 0, speed_limit_up_mbps: 0, concurrent_limit: 0, expires_at: '', remark: '' })
const filterPlan = ref(null)

// Renew / Upgrade / Downgrade
const renewDialog = ref(false)
const renewForm = ref({ email: '', client_id: 0, variant_id: null, promotion_code: '' })
const renewPromoResult = ref(null)
const renewLoading = ref(false)
const upgradeDialog = ref(false)
const upgradeForm = ref({ email: '', client_id: 0, variant_id: null, promotion_code: '' })
const upgradeLoading = ref(false)
const downgradeDialog = ref(false)
const downgradeForm = ref({ email: '', client_id: 0, variant_id: null })
const downgradeLoading = ref(false)

// Orders
const ordersDialog = ref(false)
const clientOrders = ref([])

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

// Helpers
const formatBytes = (b) => {
  if (!b || b <= 0) return '不限'
  if (b >= 1073741824) return (b / 1073741824).toFixed(1) + ' GB'
  if (b >= 1048576) return (b / 1048576).toFixed(0) + ' MB'
  return (b / 1024).toFixed(0) + ' KB'
}
const usagePercent = (row) => row.quota_bytes > 0 ? Math.min(100, Math.round(row.used_bytes / row.quota_bytes * 100)) : 0
const usageColor = (row) => usagePercent(row) > 80 ? '#f56c6c' : '#409eff'
const statusType = (s) => ({ active: 'success', suspended: 'danger', expired: 'warning' }[s] || 'info')

// Fetch
async function fetchPlanTemplates() {
  try {
    planTemplates.value = (await planApi.listPlanTemplates()) || []
    const vPromises = planTemplates.value.map(tpl => planApi.listPlanVariants(tpl.id).catch(() => []))
    const results = await Promise.all(vPromises)
    allVariants.value = results.flat()
  } catch {}
}
function onTemplateChange() {
  clientForm.value.plan_variant_id = null
}
async function validateRenewPromo() {
  if (!renewForm.value.promotion_code || !renewForm.value.variant_id) { ElMessage.warning('请先选择规格和输入优惠码'); return }
  try {
    renewPromoResult.value = await planApi.validatePromotion(renewForm.value.promotion_code, renewForm.value.variant_id)
  } catch {}
}
async function doRenew() {
  if (!renewForm.value.variant_id) { ElMessage.warning('请选择续费规格'); return }
  renewLoading.value = true
  try {
    await planApi.renewClient(renewForm.value.client_id, { variant_id: renewForm.value.variant_id, promotion_code: renewForm.value.promotion_code || undefined })
    ElMessage.success('续费成功')
    renewDialog.value = false
    fetchClients()
  } catch {} finally { renewLoading.value = false }
}
async function doUpgrade() {
  if (!upgradeForm.value.variant_id) { ElMessage.warning('请选择升级规格'); return }
  upgradeLoading.value = true
  try {
    await planApi.upgradeClient(upgradeForm.value.client_id, { variant_id: upgradeForm.value.variant_id, promotion_code: upgradeForm.value.promotion_code || undefined })
    ElMessage.success('升级成功')
    upgradeDialog.value = false
    fetchClients()
  } catch {} finally { upgradeLoading.value = false }
}
async function doDowngrade() {
  if (!downgradeForm.value.variant_id) { ElMessage.warning('请选择降级规格'); return }
  downgradeLoading.value = true
  try {
    await planApi.downgradeClient(downgradeForm.value.client_id, { variant_id: downgradeForm.value.variant_id })
    ElMessage.success('降级已安排，将在到期后生效')
    downgradeDialog.value = false
    fetchClients()
  } catch {} finally { downgradeLoading.value = false }
}
function showRenewDialog(row) {
  renewForm.value = { email: row.email, client_id: row.id, variant_id: row.plan_variant_id || null, promotion_code: '' }
  renewPromoResult.value = null
  renewDialog.value = true
}
function showUpgradeDialog(row) {
  upgradeForm.value = { email: row.email, client_id: row.id, variant_id: null, promotion_code: '' }
  upgradeDialog.value = true
}
function showDowngradeDialog(row) {
  downgradeForm.value = { email: row.email, client_id: row.id, variant_id: null }
  downgradeDialog.value = true
}
async function showOrdersDialog(row) {
  ordersDialog.value = true
  try {
    const r = await planApi.listClientOrders(row.id, { limit: 50 })
    clientOrders.value = r.items || []
  } catch { clientOrders.value = [] }
}
const orderTypeLabel = (t) => ({ new:'新购', renew:'续费', upgrade:'升级', downgrade:'降级', trial_convert:'试用转正' })[t] || t
const orderTypeColor = (t) => ({ new:'success', renew:'', upgrade:'warning', downgrade:'info', trial_convert:'danger' })[t] || ''
const orderStatusLabel = (s) => ({ pending:'待支付', paid:'已支付', completed:'已完成', cancelled:'已取消', refunded:'已退款' })[s] || s
const orderStatusColor = (s) => ({ pending:'warning', paid:'info', completed:'success', cancelled:'danger', refunded:'danger' })[s] || ''
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

// Client CRUD
function showCreateClient() { editingClient.value = null; clientForm.value = { email: '', plan_id: planTemplates.value[0]?.id || null, plan_variant_id: null, quota_gb: 0, speed_limit_mbps: 0, speed_limit_up_mbps: 0, concurrent_limit: 0, expires_at: '', remark: '' }; clientDialog.value = true }
function showEditClient(row) { editingClient.value = row; clientForm.value = { email: row.email, plan_id: row.plan_id, plan_variant_id: row.plan_variant_id || null, quota_gb: row.quota_bytes / 1073741824, speed_limit_mbps: row.speed_limit_mbps || 0, speed_limit_up_mbps: row.speed_limit_up_mbps || 0, concurrent_limit: row.concurrent_limit || 0, expires_at: row.expires_at?.substring(0,10) || '', remark: row.remark || '' }; clientDialog.value = true }
async function saveClient() {
  try {
    const payload = { ...clientForm.value, quota_bytes: Math.round((clientForm.value.quota_gb || 0) * 1073741824) }
    delete payload.quota_gb
    if (editingClient.value) { await client.put(`/clients/${editingClient.value.id}`, { plan_id: payload.plan_id, plan_variant_id: payload.plan_variant_id, quota_bytes: payload.quota_bytes, speed_limit_mbps: payload.speed_limit_mbps, speed_limit_up_mbps: payload.speed_limit_up_mbps, concurrent_limit: payload.concurrent_limit, expires_at: payload.expires_at, remark: payload.remark }) } else { await client.post('/clients', payload) }
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

onMounted(() => { fetchPlanTemplates(); fetchClients(); fetchPanelSettings() })

async function fetchPanelSettings() {
  try {
    const d = await client.get('/system/panel-config')
    if (d) panelSettings.value = { ...panelSettings.value, ...d }
  } catch {}
}
</script>
<style scoped>.hint { font-size:12px; color:#909399; margin-top:4px }</style>
