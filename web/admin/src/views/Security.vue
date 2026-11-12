<template>
  <div class="app-container">
    <el-tabs v-model="activeTab">
      <!-- ===================== 审计日志 ===================== -->
      <el-tab-pane :label="t('security.auditTitle')" name="audit">
        <el-card shadow="never">
          <el-form :inline="true" class="filter-form" @submit.prevent>
            <el-form-item :label="t('security.category')">
              <el-select v-model="auditQuery.category" clearable :placeholder="t('common.all')" style="width:140px">
                <el-option v-for="c in auditCategories" :key="c" :label="c" :value="c" />
              </el-select>
            </el-form-item>
            <el-form-item :label="t('security.eventType')">
              <el-input v-model="auditQuery.event_type" clearable :placeholder="t('security.eventType')" style="width:160px" />
            </el-form-item>
            <el-form-item :label="t('security.result')">
              <el-select v-model="auditQuery.result" clearable :placeholder="t('common.all')" style="width:130px">
                <el-option label="success" value="success" />
                <el-option label="failure" value="failure" />
                <el-option label="warning" value="warning" />
              </el-select>
            </el-form-item>
            <el-form-item :label="t('security.remoteIp')">
              <el-input v-model="auditQuery.remote_ip" clearable :placeholder="t('security.remoteIp')" style="width:150px" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="loadAudit(1)">{{ t('common.query') }}</el-button>
              <el-button @click="resetAudit">{{ t('common.reset') }}</el-button>
              <el-button @click="exportAudit">{{ t('common.export') }}</el-button>
            </el-form-item>
          </el-form>

          <el-table v-if="!isMobile" :data="auditList" v-loading="auditLoading" border stripe size="default">
            <el-table-column :label="t('common.time')" width="170">
              <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
            </el-table-column>
            <el-table-column :label="t('security.user')" width="130" show-overflow-tooltip>
              <template #default="{ row }">{{ auditUser(row) }}</template>
            </el-table-column>
            <el-table-column prop="remote_ip" :label="t('security.remoteIp')" width="140" />
            <el-table-column prop="category" :label="t('security.category')" width="110" />
            <el-table-column prop="event_type" :label="t('security.eventType')" min-width="180" show-overflow-tooltip />
            <el-table-column :label="t('security.result')" width="95">
              <template #default="{ row }">
                <el-tag :type="row.result === 'success' ? 'success' : row.result === 'failure' ? 'danger' : 'warning'" size="small">
                  {{ row.result }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column :label="t('security.detail')" min-width="220" show-overflow-tooltip>
              <template #default="{ row }">{{ auditDetail(row) }}</template>
            </el-table-column>
          </el-table>
          <MobileCards
            v-else
            :rows="auditList"
            title-field="event_type"
            :fields="auditCardFields"
          />

          <div class="pager">
            <el-pagination
              v-model:current-page="auditQuery.page"
              v-model:page-size="auditQuery.limit"
              :total="auditTotal"
              :page-sizes="[10, 20, 50, 100]"
              layout="total, sizes, prev, pager, next, jumper"
              @current-change="loadAudit"
              @size-change="loadAudit(1)"
            />
          </div>
        </el-card>
      </el-tab-pane>

      <!-- ===================== 节点操作日志 ===================== -->
      <el-tab-pane :label="t('security.nodeAuditTitle')" name="nodeAudit">
        <el-card shadow="never">
          <el-form :inline="true" class="filter-form" @submit.prevent>
            <el-form-item :label="t('nodeServer.serverId')">
              <el-select v-model="nodeAuditQuery.server_id" clearable filterable :placeholder="t('common.all')" style="width:180px">
                <el-option v-for="s in serverOptions" :key="s.id" :label="s.name" :value="s.id" />
              </el-select>
            </el-form-item>
            <el-form-item :label="t('nodeServer.actionType')">
              <el-select v-model="nodeAuditQuery.action" clearable filterable :placeholder="t('common.all')" style="width:200px">
                <el-option v-for="a in actionOptions" :key="a.value" :label="a.label" :value="a.value" />
              </el-select>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="loadNodeAudit(1)">{{ t('common.query') }}</el-button>
              <el-button @click="resetNodeAudit">{{ t('common.reset') }}</el-button>
              <el-button @click="exportNodeAudit">{{ t('common.export') }}</el-button>
            </el-form-item>
          </el-form>

          <el-table v-if="!isMobile" :data="nodeAuditList" v-loading="nodeAuditLoading" border stripe size="default">
            <el-table-column prop="id" label="ID" width="70" />
            <el-table-column :label="t('nodeServer.serverId')" min-width="140">
              <template #default="{ row }">{{ row.server_name || row.server_id || '-' }}</template>
            </el-table-column>
            <el-table-column :label="t('nodeServer.adminId')" min-width="100">
              <template #default="{ row }">{{ row.admin_name || row.admin_id || '-' }}</template>
            </el-table-column>
            <el-table-column prop="action" :label="t('nodeServer.actionType')" width="140" />
            <el-table-column prop="detail" :label="t('security.detail')" min-width="220" show-overflow-tooltip />
            <el-table-column prop="source_ip" :label="t('security.remoteIp')" width="130" />
            <el-table-column :label="t('common.status')" width="80">
              <template #default="{ row }">
                <el-tag :type="row.success ? 'success' : 'danger'" size="small">{{ row.success ? '✓' : '✗' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column :label="t('common.time')" width="180">
              <template #default="{ row }">{{ formatNodeAuditTime(row.created_at) }}</template>
            </el-table-column>
          </el-table>
          <MobileCards
            v-else
            :rows="nodeAuditList"
            title-field="action"
            :fields="nodeAuditCardFields"
          />

          <div class="pager">
            <el-pagination
              v-model:current-page="nodeAuditQuery.page"
              v-model:page-size="nodeAuditQuery.limit"
              :total="nodeAuditTotal"
              :page-sizes="[10, 20, 50, 100]"
              layout="total, sizes, prev, pager, next, jumper"
              @current-change="loadNodeAudit"
              @size-change="loadNodeAudit(1)"
            />
          </div>
        </el-card>
      </el-tab-pane>

      <!-- ===================== 订阅访问日志 ===================== -->
      <el-tab-pane :label="t('security.subAccessTitle')" name="subAccess">
        <el-card shadow="never">
          <el-form :inline="true" class="filter-form" @submit.prevent>
            <el-form-item :label="t('security.subscriptionId')">
              <el-input v-model="subAccessQuery.subscription_id" clearable placeholder="ID" style="width:120px" />
            </el-form-item>
            <el-form-item :label="t('security.remoteIp')">
              <el-input v-model="subAccessQuery.ip" clearable :placeholder="t('security.remoteIp')" style="width:160px" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="loadSubAccess(1)">{{ t('common.query') }}</el-button>
              <el-button @click="resetSubAccess">{{ t('common.reset') }}</el-button>
            </el-form-item>
          </el-form>

          <el-table v-if="!isMobile" :data="subAccessList" v-loading="subAccessLoading" border stripe size="default">
            <el-table-column :label="t('common.time')" width="170">
              <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
            </el-table-column>
            <el-table-column :label="t('security.client')" min-width="170" show-overflow-tooltip>
              <template #default="{ row }">{{ row.client_email || '-' }}</template>
            </el-table-column>
            <el-table-column prop="subscription_id" :label="t('security.subscriptionId')" width="110" />
            <el-table-column :label="t('security.remoteIp')" width="150" show-overflow-tooltip>
              <template #default="{ row }">{{ row.real_ip || row.ip || '-' }}</template>
            </el-table-column>
            <el-table-column :label="t('security.userAgent')" min-width="240" show-overflow-tooltip>
              <template #default="{ row }">{{ row.user_agent || '-' }}</template>
            </el-table-column>
            <el-table-column prop="status_code" :label="t('security.statusCode')" width="100" />
            <el-table-column :label="t('security.result')" width="120">
              <template #default="{ row }">
                <el-tag :type="row.status_code >= 200 && row.status_code < 300 ? 'success' : 'danger'" size="small">
                  {{ row.hit_result || row.status_code }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column :label="t('security.riskLevel')" width="95">
              <template #default="{ row }">
                <el-tag v-if="row.risk_level" :type="riskTagType(row.risk_level)" size="small" effect="plain">
                  {{ row.risk_level }}
                </el-tag>
                <span v-else>-</span>
              </template>
            </el-table-column>
          </el-table>
          <MobileCards
            v-else
            :rows="subAccessList"
            title-field="client_email"
            :fields="subAccessCardFields"
          />

          <div class="pager">
            <el-pagination
              v-model:current-page="subAccessQuery.page"
              v-model:page-size="subAccessQuery.limit"
              :total="subAccessTotal"
              :page-sizes="[10, 20, 50, 100]"
              layout="total, sizes, prev, pager, next, jumper"
              @current-change="loadSubAccess"
              @size-change="loadSubAccess(1)"
            />
          </div>
        </el-card>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  listAuditEvents, exportAuditEvents, listSubscriptionAccessLogs,
} from '../api/security'
import { listNodeAuditLogs, exportNodeAuditLogs, listNodeServers } from '../api/nodeServer'
import { useIsMobile } from '../composables/useIsMobile'
import MobileCards from '../components/MobileCards.vue'

const { t } = useI18n()
const { isMobile } = useIsMobile()
const activeTab = ref('audit')

// 后端实际写入的分类：auth（登录/2FA）+ 中间件产出的管理写操作分类
const auditCategories = ['auth', 'admin', 'client', 'plan', 'subscription', 'system']

// ---------------- 审计 ----------------
const auditList = ref([])
const auditTotal = ref(0)
const auditLoading = ref(false)
const auditQuery = reactive({ page: 1, limit: 20, category: '', event_type: '', result: '', remote_ip: '' })

function loadAudit(p) {
  if (p) auditQuery.page = p
  auditLoading.value = true
  const params = {
    limit: auditQuery.limit,
    offset: (auditQuery.page - 1) * auditQuery.limit,
    category: auditQuery.category || undefined,
    event_type: auditQuery.event_type || undefined,
    result: auditQuery.result || undefined,
    remote_ip: auditQuery.remote_ip || undefined,
  }
  listAuditEvents(params).then((r) => {
    auditList.value = r.items || []
    auditTotal.value = r.total || 0
  }).catch(() => {}).finally(() => { auditLoading.value = false })
}
function resetAudit() {
  auditQuery.category = ''; auditQuery.event_type = ''; auditQuery.result = ''; auditQuery.remote_ip = ''
  loadAudit(1)
}
function exportAudit() {
  const params = {
    limit: 100000, offset: 0,
    category: auditQuery.category || undefined,
    event_type: auditQuery.event_type || undefined,
    result: auditQuery.result || undefined,
    remote_ip: auditQuery.remote_ip || undefined,
  }
  exportAuditEvents(params).then((blob) => {
    const url = window.URL.createObjectURL(new Blob([blob]))
    const a = document.createElement('a')
    a.href = url
    a.download = 'audit-events.csv'
    a.click()
    window.URL.revokeObjectURL(url)
  }).catch(() => {})
}

// ---------------- 节点操作日志 ----------------
const nodeAuditList = ref([])
const nodeAuditTotal = ref(0)
const nodeAuditLoading = ref(false)
const nodeAuditQuery = reactive({ page: 1, limit: 20, server_id: '', action: '' })
const serverOptions = ref([])
const actionOptions = [
  { value: 'create', label: 'create' },
  { value: 'update', label: 'update' },
  { value: 'delete', label: 'delete' },
  { value: 'control_core', label: 'control_core' },
  { value: 'sync_config', label: 'sync_config' },
  { value: 'generate_token', label: 'generate_token' },
  { value: 'revoke_cert', label: 'revoke_cert' },
  { value: 'renew_cert', label: 'renew_cert' },
  { value: 'distribute_cert', label: 'distribute_cert' },
  { value: 'distribute_cert_all', label: 'distribute_cert_all' },
  { value: 'associate_cert', label: 'associate_cert' },
  { value: 'dissociate_cert', label: 'dissociate_cert' },
  { value: 'update_agent', label: 'update_agent' },
  { value: 'update_all_agents', label: 'update_all_agents' },
  { value: 'update_node_type', label: 'update_node_type' },
  { value: 'sync_mirror', label: 'sync_mirror' },
  { value: 'trigger_failover', label: 'trigger_failover' },
  { value: 'save_protocol', label: 'save_protocol' },
  { value: 'save_protocol_section', label: 'save_protocol_section' },
  { value: 'control_proxy', label: 'control_proxy' },
  { value: 'sync_proxy', label: 'sync_proxy' },
  { value: 'update_ratelimit', label: 'update_ratelimit' },
  { value: 'user_create', label: 'user_create' },
  { value: 'user_update', label: 'user_update' },
  { value: 'user_delete', label: 'user_delete' },
  { value: 'cert_create', label: 'cert_create' },
  { value: 'cert_update', label: 'cert_update' },
  { value: 'cert_delete', label: 'cert_delete' },
  { value: 'cert_issue', label: 'cert_issue' },
  { value: 'cert_renew', label: 'cert_renew' },
  { value: 'xray_add_inbound', label: 'xray_add_inbound' },
  { value: 'xray_remove_inbound', label: 'xray_remove_inbound' },
  { value: 'xray_add_outbound', label: 'xray_add_outbound' },
  { value: 'xray_remove_outbound', label: 'xray_remove_outbound' },
  { value: 'xray_add_user', label: 'xray_add_user' },
  { value: 'xray_remove_user', label: 'xray_remove_user' },
  { value: 'xray_apply_routing', label: 'xray_apply_routing' },
  { value: 'xray_add_routing_rule', label: 'xray_add_routing_rule' },
  { value: 'xray_remove_routing_rule', label: 'xray_remove_routing_rule' },
  { value: 'xray_set_balancer', label: 'xray_set_balancer' },
  { value: 'xray_reset_stats', label: 'xray_reset_stats' },
]

function loadServerOptions() {
  listNodeServers({ limit: 500 }).then((r) => {
    serverOptions.value = (r.items || []).map((s) => ({ id: s.id, name: s.name }))
  }).catch(() => {})
}

function loadNodeAudit(p) {
  if (p) nodeAuditQuery.page = p
  nodeAuditLoading.value = true
  const params = {
    limit: nodeAuditQuery.limit,
    offset: (nodeAuditQuery.page - 1) * nodeAuditQuery.limit,
  }
  if (nodeAuditQuery.server_id) params.server_id = nodeAuditQuery.server_id
  if (nodeAuditQuery.action) params.action = nodeAuditQuery.action
  listNodeAuditLogs(params).then((r) => {
    nodeAuditList.value = r.items || []
    nodeAuditTotal.value = r.total || 0
  }).catch(() => {}).finally(() => { nodeAuditLoading.value = false })
}
function resetNodeAudit() {
  nodeAuditQuery.server_id = ''; nodeAuditQuery.action = ''
  loadNodeAudit(1)
}
function exportNodeAudit() {
  const params = {}
  if (nodeAuditQuery.server_id) params.server_id = nodeAuditQuery.server_id
  if (nodeAuditQuery.action) params.action = nodeAuditQuery.action
  exportNodeAuditLogs(params).then((data) => {
    const blob = new Blob([data], { type: 'text/csv' })
    const url = window.URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'node-audit-logs.csv'
    a.click()
    window.URL.revokeObjectURL(url)
  }).catch(() => {})
}
// 统一时间格式：后端返回 RFC3339（带纳秒+时区），直接渲染会把列撑爆。
function formatTime(time) {
  if (!time) return ''
  const d = new Date(time)
  if (isNaN(d)) return String(time)
  const pad = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}
const formatNodeAuditTime = formatTime

// 审计事件只存 user_id / reason，前端要用可读字段（原先取 row.user / row.detail 永远为空）。
const auditUser = (row) => row.user_name || (row.user_id ? '#' + row.user_id : '-')
const auditDetail = (row) => row.reason || row.metadata_json || '-'

// ---------------- 订阅访问日志 ----------------
// 风险等级由后端按状态码/命中结果/UA 给出（high = 未知 token、被限流）。
const riskTagType = (level) => ({ high: 'danger', medium: 'warning', low: 'info' })[level] || 'info'

const subAccessList = ref([])
const subAccessTotal = ref(0)
const subAccessLoading = ref(false)
const subAccessQuery = reactive({ page: 1, limit: 20, subscription_id: '', ip: '' })
function loadSubAccess(p) {
  if (p) subAccessQuery.page = p
  subAccessLoading.value = true
  listSubscriptionAccessLogs({
    limit: subAccessQuery.limit,
    offset: (subAccessQuery.page - 1) * subAccessQuery.limit,
    subscription_id: String(subAccessQuery.subscription_id || '').trim() || undefined,
    ip: String(subAccessQuery.ip || '').trim() || undefined,
  }).then((r) => {
    subAccessList.value = r.items || []
    subAccessTotal.value = r.total || 0
  }).catch(() => {}).finally(() => { subAccessLoading.value = false })
}
function resetSubAccess() {
  subAccessQuery.subscription_id = ''
  subAccessQuery.ip = ''
  loadSubAccess(1)
}

// 手机端卡片字段（与桌面表格列对应）
const auditCardFields = computed(() => [
  { label: t('common.time'), text: (row) => formatTime(row.created_at) || '-' },
  { label: t('security.user'), text: (row) => auditUser(row) },
  { label: t('security.remoteIp'), text: (row) => row.remote_ip || '-' },
  { label: t('security.category'), text: (row) => row.category || '-' },
  { label: t('security.result'), text: (row) => row.result || '-' },
  { label: t('security.detail'), text: (row) => auditDetail(row) },
])
const nodeAuditCardFields = computed(() => [
  { label: 'ID', text: (row) => (row.id ?? '-') },
  { label: t('nodeServer.serverId'), text: (row) => row.server_name || row.server_id || '-' },
  { label: t('nodeServer.adminId'), text: (row) => row.admin_name || row.admin_id || '-' },
  { label: t('security.detail'), text: (row) => row.detail || '-' },
  { label: t('security.remoteIp'), text: (row) => row.source_ip || '-' },
  { label: t('common.status'), text: (row) => (row.success ? '✓' : '✗') },
  { label: t('common.time'), text: (row) => formatNodeAuditTime(row.created_at) || '-' },
])
const subAccessCardFields = computed(() => [
  { label: t('common.time'), text: (row) => formatTime(row.created_at) || '-' },
  { label: t('security.client'), text: (row) => row.client_email || '-' },
  { label: t('security.subscriptionId'), text: (row) => row.subscription_id || '-' },
  { label: t('security.remoteIp'), text: (row) => row.real_ip || row.ip || '-' },
  { label: t('security.userAgent'), text: (row) => row.user_agent || '-' },
  { label: t('security.statusCode'), text: (row) => row.status_code || '-' },
  { label: t('security.riskLevel'), text: (row) => row.risk_level || '-' },
])

onMounted(() => { loadAudit(1) })

// Lazy-load node audit logs when the tab is first activated
watch(activeTab, (val) => {
  if (val === 'nodeAudit') {
    if (nodeAuditList.value.length === 0) loadNodeAudit(1)
    if (serverOptions.value.length === 0) loadServerOptions()
  }
  if (val === 'subAccess' && subAccessList.value.length === 0) loadSubAccess(1)
})
</script>

<style scoped>
.filter-form { margin-bottom: 12px; }
.toolbar { margin-bottom: 12px; }
.pager { margin-top: 14px; display: flex; justify-content: flex-end; }
.muted { color: #909399; }
</style>
