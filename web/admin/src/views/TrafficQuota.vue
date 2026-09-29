<template>
  <div>
    <div style="display:flex;justify-content:space-between;margin-bottom:20px">
      <div>
        <h2>{{ t('trafficQuota.title') }}</h2>
        <el-alert
          style="margin-top:8px;max-width:760px"
          :title="t('trafficQuota.readOnlyHint')"
          type="info"
          :closable="false"
          show-icon
        />
      </div>
      <el-button @click="fetchAll">{{ t('common.refresh') }}</el-button>
    </div>

    <el-tabs v-model="activeTab">
      <!-- Per-node effective quotas (from node_servers) -->
      <el-tab-pane :label="t('trafficQuota.quotas')" name="quotas">
        <el-card>
          <el-table :data="quotas" v-loading="loadingQuotas" stripe>
            <el-table-column prop="server_id" label="ID" width="80" />
            <el-table-column prop="name" :label="t('trafficQuota.nodeName')" min-width="160" />
            <el-table-column prop="region" :label="t('trafficQuota.region')" width="120" />
            <el-table-column prop="egress_iface" :label="t('trafficQuota.egressIface')" width="120" />
            <el-table-column :label="t('trafficQuota.resetDay')" width="110">
              <template #default="{row}">{{ row.has_quota ? ('D' + row.reset_day) : '-' }}</template>
            </el-table-column>
            <el-table-column :label="t('trafficQuota.quotaBytes')" width="140">
              <template #default="{row}">{{ row.has_quota ? formatBytes(row.quota_bytes) : t('trafficQuota.unlimited') }}</template>
            </el-table-column>
            <el-table-column :label="t('trafficQuota.usedBytes')" width="140">
              <template #default="{row}">{{ formatBytes(row.used_bytes) }}</template>
            </el-table-column>
            <el-table-column :label="t('trafficQuota.usagePct')" min-width="180">
              <template #default="{row}">
                <el-progress
                  v-if="row.has_quota"
                  :percentage="Math.min(100, row.quota_bytes > 0 ? Math.round(row.used_bytes / row.quota_bytes * 100) : 0)"
                  :status="row.suspended ? 'exception' : ''"
                />
                <span v-else>-</span>
              </template>
            </el-table-column>
            <el-table-column :label="t('trafficQuota.status')" width="110">
              <template #default="{row}">
                <el-tag v-if="row.suspended" type="danger" size="small">{{ t('trafficQuota.suspended') }}</el-tag>
                <el-tag v-else-if="row.has_quota" type="success" size="small">{{ t('trafficQuota.active') }}</el-tag>
                <el-tag v-else type="info" size="small">{{ t('trafficQuota.none') }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column :label="t('common.actions')" width="120" fixed="right">
              <template #default="{row}">
                <el-button size="small" type="primary" plain @click="showUsage(row)">{{ t('trafficQuota.usage') }}</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>

      <!-- Current period usage (from node_traffic_usage) -->
      <el-tab-pane :label="t('trafficQuota.usage')" name="usage">
        <el-card>
          <el-form inline @submit.prevent style="margin-bottom:12px">
            <el-form-item :label="t('trafficQuota.nodeName')">
              <el-select v-model="usageServerID" filterable style="width:220px" @change="fetchUsage">
                <el-option :label="t('common.all')" :value="0" />
                <el-option v-for="q in quotas" :key="q.server_id" :label="q.name" :value="q.server_id" />
              </el-select>
            </el-form-item>
            <el-button type="primary" plain @click="fetchUsage">{{ t('common.refresh') }}</el-button>
          </el-form>
          <el-table :data="usageList" v-loading="loadingUsage" stripe>
            <el-table-column prop="server_id" label="ID" width="80" />
            <el-table-column :label="t('trafficQuota.periodStart')" min-width="180">
              <template #default="{row}">{{ formatDate(row.period_start) }}</template>
            </el-table-column>
            <el-table-column :label="t('trafficQuota.periodEnd')" width="180">
              <template #default="{row}">{{ formatDate(row.period_end) }}</template>
            </el-table-column>
            <el-table-column :label="t('trafficQuota.inputBytes')" width="130">
              <template #default="{row}">{{ formatBytes(row.rx_bytes) }}</template>
            </el-table-column>
            <el-table-column :label="t('trafficQuota.outputBytes')" width="130">
              <template #default="{row}">{{ formatBytes(row.tx_bytes) }}</template>
            </el-table-column>
            <el-table-column :label="t('trafficQuota.usedBytes')" width="140">
              <template #default="{row}">{{ formatBytes(row.total_bytes) }}</template>
            </el-table-column>
            <el-table-column :label="t('trafficQuota.quotaBytes')" width="140">
              <template #default="{row}">{{ row.quota_bytes > 0 ? formatBytes(row.quota_bytes) : t('trafficQuota.unlimited') }}</template>
            </el-table-column>
            <el-table-column :label="t('trafficQuota.status')" width="110">
              <template #default="{row}">
                <el-tag :type="row.suspended ? 'danger' : 'success'" size="small">
                  {{ row.suspended ? t('trafficQuota.suspended') : t('trafficQuota.active') }}
                </el-tag>
              </template>
            </el-table-column>
          </el-table>
          <el-empty v-if="!loadingUsage && usageList.length === 0" :description="t('common.noData')" />
        </el-card>
      </el-tab-pane>

      <!-- History (from node_traffic_usage, aggregate) -->
      <el-tab-pane :label="t('trafficQuota.history')" name="history">
        <el-card>
          <el-form inline @submit.prevent style="margin-bottom:12px">
            <el-form-item :label="t('trafficQuota.months')">
              <el-select v-model="historyMonths" style="width:120px" @change="fetchHistory">
                <el-option :label="'3'" :value="3" />
                <el-option :label="'6'" :value="6" />
                <el-option :label="'12'" :value="12" />
              </el-select>
            </el-form-item>
            <el-button type="primary" plain @click="fetchHistory">{{ t('common.refresh') }}</el-button>
          </el-form>
          <el-table :data="historyList" v-loading="loadingHistory" stripe max-height="520">
            <el-table-column prop="server_id" label="ID" width="80" />
            <el-table-column :label="t('trafficQuota.periodStart')" min-width="180">
              <template #default="{row}">{{ formatDate(row.period_start) }}</template>
            </el-table-column>
            <el-table-column :label="t('trafficQuota.periodEnd')" width="180">
              <template #default="{row}">{{ formatDate(row.period_end) }}</template>
            </el-table-column>
            <el-table-column :label="t('trafficQuota.inputBytes')" width="130">
              <template #default="{row}">{{ formatBytes(row.rx_bytes) }}</template>
            </el-table-column>
            <el-table-column :label="t('trafficQuota.outputBytes')" width="130">
              <template #default="{row}">{{ formatBytes(row.tx_bytes) }}</template>
            </el-table-column>
            <el-table-column :label="t('trafficQuota.usedBytes')" width="140">
              <template #default="{row}">{{ formatBytes(row.total_bytes) }}</template>
            </el-table-column>
            <el-table-column :label="t('trafficQuota.status')" width="110">
              <template #default="{row}">
                <el-tag :type="row.suspended ? 'danger' : 'success'" size="small">
                  {{ row.suspended ? t('trafficQuota.suspended') : t('trafficQuota.active') }}
                </el-tag>
              </template>
            </el-table-column>
          </el-table>
          <el-empty v-if="!loadingHistory && historyList.length === 0" :description="t('common.noData')" />
        </el-card>
      </el-tab-pane>

      <!-- Reset logs (from traffic_quota_reset_logs) -->
      <el-tab-pane :label="t('trafficQuota.resetLogs')" name="logs">
        <el-card>
          <el-table :data="resetLogs" v-loading="loadingLogs" stripe>
            <el-table-column prop="id" label="ID" width="70" />
            <el-table-column prop="subject_type" :label="t('trafficQuota.subjectType')" width="120" />
            <el-table-column prop="subject_id" :label="t('trafficQuota.subjectId')" width="100" />
            <el-table-column prop="reset_cycle" :label="t('trafficQuota.resetCycle')" width="110" />
            <el-table-column :label="t('trafficQuota.periodStart')" width="180">
              <template #default="{row}">{{ formatDate(row.period_start) }}</template>
            </el-table-column>
            <el-table-column prop="trigger_source" :label="t('trafficQuota.triggerSource')" min-width="120" />
            <el-table-column prop="operator_id" :label="t('trafficQuota.operatorId')" width="100" />
            <el-table-column :label="t('common.createdAt')" width="180">
              <template #default="{row}">{{ formatDate(row.created_at) }}</template>
            </el-table-column>
          </el-table>
          <el-empty v-if="!loadingLogs && resetLogs.length === 0" :description="t('common.noData')" />
        </el-card>
      </el-tab-pane>
    </el-tabs>

    <!-- Current period usage dialog (read-only) -->
    <el-dialog v-model="usageVisible" :title="t('trafficQuota.usageTitle')" width="560">
      <template v-if="usage && !usage.empty">
        <el-descriptions :column="1" border>
          <el-descriptions-item :label="t('trafficQuota.nodeId')">{{ usage.server_id }}</el-descriptions-item>
          <el-descriptions-item :label="t('trafficQuota.periodStart')">{{ formatDate(usage.period_start) }}</el-descriptions-item>
          <el-descriptions-item :label="t('trafficQuota.periodEnd')">{{ formatDate(usage.period_end) }}</el-descriptions-item>
          <el-descriptions-item :label="t('trafficQuota.inputBytes')">{{ formatBytes(usage.rx_bytes) }}</el-descriptions-item>
          <el-descriptions-item :label="t('trafficQuota.outputBytes')">{{ formatBytes(usage.tx_bytes) }}</el-descriptions-item>
          <el-descriptions-item :label="t('trafficQuota.usedBytes')">{{ formatBytes(usage.total_bytes) }}</el-descriptions-item>
          <el-descriptions-item :label="t('trafficQuota.quotaBytes')">
            {{ usage.quota_bytes > 0 ? formatBytes(usage.quota_bytes) : t('trafficQuota.unlimited') }}
          </el-descriptions-item>
          <el-descriptions-item :label="t('trafficQuota.remaining')">
            {{ usage.quota_bytes > 0 ? formatBytes(usage.remaining) : '-' }}
          </el-descriptions-item>
          <el-descriptions-item :label="t('trafficQuota.exceeded')">
            <el-tag :type="usage.exceeded ? 'danger' : 'success'" size="small">
              {{ usage.exceeded ? t('common.yes') : t('common.no') }}
            </el-tag>
          </el-descriptions-item>
        </el-descriptions>
        <el-progress
          style="margin-top:16px"
          :percentage="usage.quota_bytes > 0 ? Math.min(100, Math.round(usage.total_bytes / usage.quota_bytes * 100)) : 0"
          :status="usage.exceeded ? 'exception' : ''"
        />
      </template>
      <el-empty v-else :description="t('common.noData')" />
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import {
  listTrafficQuotas, getTrafficQuotaUsage, getTrafficUsageHistory,
  listTrafficResetLogs,
} from '../api/trafficQuota'

const { t } = useI18n()
const activeTab = ref('quotas')

// ── quotas ──
const quotas = ref([])
const loadingQuotas = ref(false)

// ── current period usage ──
const usageList = ref([])
const loadingUsage = ref(false)
const usageServerID = ref(0)

// ── history ──
const historyList = ref([])
const loadingHistory = ref(false)
const historyMonths = ref(6)

// ── reset logs ──
const resetLogs = ref([])
const loadingLogs = ref(false)

// ── usage dialog ──
const usageVisible = ref(false)
const usage = ref(null)

function formatBytes(n) {
  if (n == null) return '-'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  let v = Number(n)
  let i = 0
  while (v >= 1024 && i < units.length - 1) { v /= 1024; i++ }
  return `${v.toFixed(2)} ${units[i]}`
}

function formatDate(s) {
  if (!s) return '-'
  const d = new Date(s)
  if (isNaN(d.getTime())) return String(s)
  return d.toLocaleString()
}

async function fetchQuotas() {
  loadingQuotas.value = true
  try {
    const res = await listTrafficQuotas({})
    const data = res.data ?? res
    quotas.value = Array.isArray(data) ? data : (data.items ?? [])
  } catch (e) {
    ElMessage.error(t('common.loadFailed'))
  } finally {
    loadingQuotas.value = false
  }
}

async function fetchUsage() {
  loadingUsage.value = true
  try {
    // Pull current-period usage for all nodes, then filter by selection.
    const res = await getTrafficUsageHistory({ months: 1 })
    const data = res.data ?? res
    const all = Array.isArray(data) ? data : (data.items ?? [])
    usageList.value = usageServerID.value ? all.filter(u => u.server_id === usageServerID.value) : all
  } catch (e) {
    ElMessage.error(t('common.loadFailed'))
  } finally {
    loadingUsage.value = false
  }
}

async function fetchHistory() {
  loadingHistory.value = true
  try {
    const res = await getTrafficUsageHistory({ months: historyMonths.value })
    const data = res.data ?? res
    historyList.value = Array.isArray(data) ? data : (data.items ?? [])
  } catch (e) {
    ElMessage.error(t('common.loadFailed'))
  } finally {
    loadingHistory.value = false
  }
}

async function fetchResetLogs() {
  loadingLogs.value = true
  try {
    const res = await listTrafficResetLogs({})
    const data = res.data ?? res
    resetLogs.value = Array.isArray(data) ? data : (data.items ?? [])
  } catch (e) {
    ElMessage.error(t('common.loadFailed'))
  } finally {
    loadingLogs.value = false
  }
}

function fetchAll() {
  fetchQuotas(); fetchUsage(); fetchHistory(); fetchResetLogs()
}

async function showUsage(row) {
  usage.value = null
  usageVisible.value = true
  try {
    const res = await getTrafficQuotaUsage(row.server_id)
    usage.value = res.data ?? res
  } catch (e) {
    ElMessage.error(t('common.loadFailed'))
  }
}

onMounted(fetchAll)
</script>
