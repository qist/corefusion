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

          <el-table :data="auditList" v-loading="auditLoading" border stripe size="default">
            <el-table-column prop="created_at" :label="t('common.time')" width="180" />
            <el-table-column prop="user" :label="t('security.user')" width="140" show-overflow-tooltip />
            <el-table-column prop="remote_ip" :label="t('security.remoteIp')" width="140" />
            <el-table-column prop="category" :label="t('security.category')" width="130" />
            <el-table-column prop="event_type" :label="t('security.eventType')" width="180" show-overflow-tooltip />
            <el-table-column :label="t('security.result')" width="100">
              <template #default="{ row }">
                <el-tag :type="row.result === 'success' ? 'success' : row.result === 'failure' ? 'danger' : 'warning'" size="small">
                  {{ row.result }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="detail" :label="t('security.detail')" min-width="220" show-overflow-tooltip />
          </el-table>

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

      <!-- ===================== 安全告警 ===================== -->
      <el-tab-pane :label="t('security.alertsTitle')" name="alerts">
        <el-card shadow="never">
          <el-form :inline="true" class="filter-form" @submit.prevent>
            <el-form-item :label="t('security.alertType')">
              <el-input v-model="alertQuery.alert_type" clearable :placeholder="t('security.alertType')" style="width:160px" />
            </el-form-item>
            <el-form-item :label="t('security.severity')">
              <el-select v-model="alertQuery.severity" clearable :placeholder="t('common.all')" style="width:130px">
                <el-option label="info" value="info" />
                <el-option label="warning" value="warning" />
                <el-option label="critical" value="critical" />
              </el-select>
            </el-form-item>
            <el-form-item :label="t('security.status')">
              <el-select v-model="alertQuery.status" clearable :placeholder="t('common.all')" style="width:130px">
                <el-option :label="t('security.statusOpen')" value="open" />
                <el-option :label="t('security.statusResolved')" value="resolved" />
              </el-select>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="loadAlerts(1)">{{ t('common.query') }}</el-button>
              <el-button @click="resetAlerts">{{ t('common.reset') }}</el-button>
              <el-button type="success" :disabled="!openAlertCount" @click="resolveAll">
                {{ t('security.resolveAll') }} ({{ openAlertCount }})
              </el-button>
            </el-form-item>
          </el-form>

          <el-table :data="alertList" v-loading="alertLoading" border stripe>
            <el-table-column prop="created_at" :label="t('common.time')" width="180" />
            <el-table-column prop="alert_type" :label="t('security.alertType')" width="160" />
            <el-table-column :label="t('security.severity')" width="110">
              <template #default="{ row }">
                <el-tag :type="row.severity === 'critical' ? 'danger' : row.severity === 'warning' ? 'warning' : 'info'" size="small">
                  {{ row.severity }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column :label="t('security.status')" width="110">
              <template #default="{ row }">
                <el-tag :type="row.status === 'resolved' ? 'success' : 'danger'" size="small">
                  {{ row.status === 'resolved' ? t('security.statusResolved') : t('security.statusOpen') }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="message" :label="t('security.message')" min-width="240" show-overflow-tooltip />
            <el-table-column prop="acknowledged_by" :label="t('security.ackedBy')" width="140" show-overflow-tooltip />
            <el-table-column :label="t('common.action')" width="120" fixed="right">
              <template #default="{ row }">
                <el-button v-if="row.status !== 'resolved'" type="success" link size="small" @click="resolveOne(row)">
                  {{ t('security.resolve') }}
                </el-button>
                <span v-else class="muted">{{ t('security.statusResolved') }}</span>
              </template>
            </el-table-column>
          </el-table>

          <div class="pager">
            <el-pagination
              v-model:current-page="alertQuery.page"
              v-model:page-size="alertQuery.limit"
              :total="alertTotal"
              :page-sizes="[10, 20, 50, 100]"
              layout="total, sizes, prev, pager, next, jumper"
              @current-change="loadAlerts"
              @size-change="loadAlerts(1)"
            />
          </div>
        </el-card>
      </el-tab-pane>

      <!-- ===================== 告警渠道 ===================== -->
      <el-tab-pane :label="t('security.channelsTitle')" name="channels">
        <el-card shadow="never">
          <div class="toolbar">
            <el-button type="primary" @click="openChannelForm()">{{ t('security.addChannel') }}</el-button>
          </div>
          <el-table :data="channelList" v-loading="channelLoading" border stripe>
            <el-table-column prop="id" label="ID" width="70" />
            <el-table-column prop="name" :label="t('security.channelName')" width="160" />
            <el-table-column prop="type" :label="t('security.channelType')" width="120" />
            <el-table-column prop="url" :label="t('security.channelUrl')" min-width="220" show-overflow-tooltip />
            <el-table-column :label="t('common.enabled')" width="90">
              <template #default="{ row }">
                <el-switch :model-value="!!row.enabled" disabled />
              </template>
            </el-table-column>
            <el-table-column prop="last_error" :label="t('security.channelLastError')" min-width="200" show-overflow-tooltip />
            <el-table-column prop="last_sent_at" :label="t('security.channelLastSent')" width="180" />
            <el-table-column :label="t('common.action')" width="150" fixed="right">
              <template #default="{ row }">
                <el-button type="primary" link size="small" @click="openChannelForm(row)">{{ t('common.edit') }}</el-button>
                <el-button type="danger" link size="small" @click="removeChannel(row)">{{ t('common.delete') }}</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>
    </el-tabs>

    <!-- 告警渠道表单 -->
    <el-dialog v-model="channelDialog" :title="channelForm.id ? t('security.editChannel') : t('security.addChannel')" width="480px">
      <el-form :model="channelForm" label-width="110px">
        <el-form-item :label="t('security.channelName')" required>
          <el-input v-model="channelForm.name" :placeholder="t('security.channelName')" />
        </el-form-item>
        <el-form-item :label="t('security.channelType')" required>
          <el-select v-model="channelForm.type" style="width:100%">
            <el-option label="webhook" value="webhook" />
            <el-option label="telegram" value="telegram" />
            <el-option label="email" value="email" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('security.channelUrl')">
          <el-input v-model="channelForm.url" :placeholder="t('security.channelUrlPlaceholder')" />
        </el-form-item>
        <el-form-item :label="t('common.enabled')">
          <el-switch v-model="channelForm.enabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="channelDialog = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="channelSaving" @click="saveChannel">{{ t('common.confirm') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  listAuditEvents, exportAuditEvents,
  listAlerts, resolveAlert, resolveAllAlerts,
  listAlertChannels, createAlertChannel, updateAlertChannel, deleteAlertChannel,
} from '../api/security'

const { t } = useI18n()
const activeTab = ref('audit')

const auditCategories = ['auth', 'subscription', 'access', 'security', 'order', 'client', 'system']

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

// ---------------- 告警 ----------------
const alertList = ref([])
const alertTotal = ref(0)
const alertLoading = ref(false)
const openAlertCount = ref(0)
const alertQuery = reactive({ page: 1, limit: 20, alert_type: '', severity: '', status: '' })

function loadAlerts(p) {
  if (p) alertQuery.page = p
  alertLoading.value = true
  const params = {
    limit: alertQuery.limit,
    offset: (alertQuery.page - 1) * alertQuery.limit,
    alert_type: alertQuery.alert_type || undefined,
    severity: alertQuery.severity || undefined,
    status: alertQuery.status || undefined,
  }
  listAlerts(params).then((r) => {
    alertList.value = r.items || []
    alertTotal.value = r.total || 0
    openAlertCount.value = r.open_count != null ? r.open_count : alertList.value.filter(a => a.status !== 'resolved').length
  }).catch(() => {}).finally(() => { alertLoading.value = false })
}
function resetAlerts() {
  alertQuery.alert_type = ''; alertQuery.severity = ''; alertQuery.status = ''
  loadAlerts(1)
}
function resolveOne(row) {
  resolveAlert(row.id).then(() => {
    ElMessage.success(t('common.success'))
    loadAlerts()
  }).catch(() => {})
}
function resolveAll() {
  ElMessageBox.confirm(t('security.resolveAllConfirm'), t('common.confirm'), { type: 'warning' })
    .then(() => resolveAllAlerts().then(() => { ElMessage.success(t('common.success')); loadAlerts() }))
    .catch(() => {})
}

// ---------------- 告警渠道 ----------------
const channelList = ref([])
const channelLoading = ref(false)
const channelDialog = ref(false)
const channelSaving = ref(false)
const channelForm = reactive({ id: null, name: '', type: 'webhook', url: '', enabled: true })

function loadChannels() {
  channelLoading.value = true
  listAlertChannels().then((r) => { channelList.value = r || [] }).catch(() => {}).finally(() => { channelLoading.value = false })
}
function openChannelForm(row) {
  if (row) {
    channelForm.id = row.id; channelForm.name = row.name; channelForm.type = row.type; channelForm.url = row.url || ''; channelForm.enabled = !!row.enabled
  } else {
    channelForm.id = null; channelForm.name = ''; channelForm.type = 'webhook'; channelForm.url = ''; channelForm.enabled = true
  }
  channelDialog.value = true
}
function saveChannel() {
  if (!channelForm.name || !channelForm.type) { ElMessage.warning(t('common.required')); return }
  channelSaving.value = true
  const data = { name: channelForm.name, type: channelForm.type, url: channelForm.url, enabled: channelForm.enabled }
  const p = channelForm.id ? updateAlertChannel(channelForm.id, data) : createAlertChannel(data)
  p.then(() => { ElMessage.success(t('common.success')); channelDialog.value = false; loadChannels() })
    .catch(() => {}).finally(() => { channelSaving.value = false })
}
function removeChannel(row) {
  ElMessageBox.confirm(t('security.deleteChannelConfirm', { name: row.name }), t('common.confirm'), { type: 'warning' })
    .then(() => deleteAlertChannel(row.id).then(() => { ElMessage.success(t('common.success')); loadChannels() }))
    .catch(() => {})
}

onMounted(() => { loadAudit(1); loadAlerts(1); loadChannels() })
</script>

<style scoped>
.filter-form { margin-bottom: 12px; }
.toolbar { margin-bottom: 12px; }
.pager { margin-top: 14px; display: flex; justify-content: flex-end; }
.muted { color: #909399; }
</style>
