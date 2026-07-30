<template>
  <div class="app-container">
    <!-- 概览 -->
    <el-row :gutter="16" class="overview">
      <el-col :span="4"><el-card shadow="never"><div class="stat"><div class="num">{{ overview.snapshot_count || 0 }}</div><div class="lbl">{{ t('runtime.snapshots') }}</div></div></el-card></el-col>
      <el-col :span="4"><el-card shadow="never"><div class="stat"><div class="num">{{ overview.job_count || 0 }}</div><div class="lbl">{{ t('runtime.jobs') }}</div></div></el-card></el-col>
      <el-col :span="4"><el-card shadow="never"><div class="stat"><div class="num success">{{ overview.success_count || 0 }}</div><div class="lbl">{{ t('runtime.success') }}</div></div></el-card></el-col>
      <el-col :span="4"><el-card shadow="never"><div class="stat"><div class="num danger">{{ overview.failed_count || 0 }}</div><div class="lbl">{{ t('runtime.failed') }}</div></div></el-card></el-col>
      <el-col :span="8"><el-card shadow="never"><div class="stat"><div class="num small">{{ overview.recent_job_id || '-' }}</div><div class="lbl">{{ t('runtime.recentJob') }}</div></div></el-card></el-col>
    </el-row>

    <el-tabs v-model="activeTab">
      <!-- ============ 操作 ============ -->
      <el-tab-pane :label="t('runtime.actions')" name="actions">
        <el-card shadow="never">
          <div class="action-grid">
            <div class="action-item">
              <div class="action-title">{{ t('runtime.createSnapshot') }}</div>
              <el-input v-model="snapReason" :placeholder="t('runtime.reasonPlaceholder')" size="small" class="mb8" />
              <el-button type="primary" :loading="snapLoading" @click="doSnapshot">{{ t('runtime.createSnapshot') }}</el-button>
            </div>

            <div class="action-item">
              <div class="action-title">{{ t('runtime.dryRun') }}</div>
              <div class="action-desc">{{ t('runtime.dryRunDesc') }}</div>
              <el-button @click="doDryRun">{{ t('runtime.runDryRun') }}</el-button>
              <el-alert
                v-if="dryResult"
                :type="dryResult.ok ? 'success' : 'warning'"
                :closable="false"
                class="mt8"
                :title="dryResult.ok ? t('runtime.dryOk') : t('runtime.dryFail')"
              >
                <div v-if="dryResult.affected_cores && dryResult.affected_cores.length">
                  {{ t('runtime.affectedCores') }}: {{ dryResult.affected_cores.join(', ') }}
                </div>
                <div v-if="dryResult.port_conflicts && dryResult.port_conflicts.length">
                  {{ t('runtime.portConflicts') }}: {{ dryResult.port_conflicts.join('; ') }}
                </div>
                <div v-if="dryResult.invalid_json && dryResult.invalid_json.length">
                  {{ t('runtime.invalidJson') }}: {{ dryResult.invalid_json.join('; ') }}
                </div>
              </el-alert>
            </div>

            <div class="action-item">
              <div class="action-title">{{ t('runtime.apply') }}</div>
              <el-input v-model="applyReason" :placeholder="t('runtime.reasonPlaceholder')" size="small" class="mb8" />
              <el-button type="success" :loading="applyLoading" @click="doApply">{{ t('runtime.runApply') }}</el-button>
            </div>

            <div class="action-item danger-zone">
              <div class="action-title">{{ t('runtime.rollback') }}</div>
              <el-select v-model="rollbackSnap" :placeholder="t('runtime.selectSnapshot')" size="small" class="mb8" style="width:100%">
                <el-option v-for="s in snapshotList" :key="s.id" :label="`#${s.id} ${s.trigger_source} ${s.created_at}`" :value="s.id" />
              </el-select>
              <el-input v-model="rollbackReason" :placeholder="t('runtime.reasonPlaceholder')" size="small" class="mb8" />
              <el-button type="warning" :disabled="!rollbackSnap" :loading="rollbackLoading" @click="doRollback">{{ t('runtime.runRollback') }}</el-button>
            </div>

            <div class="action-item danger-zone">
              <div class="action-title">{{ t('runtime.disableProxy') }}</div>
              <div class="action-desc">{{ t('runtime.disableDesc') }}</div>
              <el-input v-model="disableReason" :placeholder="t('runtime.reasonPlaceholder')" size="small" class="mb8" />
              <el-button type="danger" :loading="disableLoading" @click="doDisable">{{ t('runtime.runDisable') }}</el-button>
            </div>
          </div>
          <el-alert v-if="lastJobId" :closable="false" type="info" class="mt8">
            {{ t('runtime.lastJob') }}: #{{ lastJobId }}
          </el-alert>
        </el-card>
      </el-tab-pane>

      <!-- ============ 快照 ============ -->
      <el-tab-pane :label="t('runtime.snapshots')" name="snapshots">
        <el-card shadow="never">
          <el-table :data="snapshotList" v-loading="snapListLoading" border stripe>
            <el-table-column prop="id" label="ID" width="70" />
            <el-table-column prop="trigger_source" :label="t('runtime.triggerSource')" width="130" />
            <el-table-column prop="reason" :label="t('runtime.reason')" min-width="180" show-overflow-tooltip />
            <el-table-column prop="config_hash" :label="t('runtime.hash')" width="160" show-overflow-tooltip />
            <el-table-column prop="created_at" :label="t('common.time')" width="180" />
            <el-table-column :label="t('common.action')" width="160" fixed="right">
              <template #default="{ row }">
                <el-button type="primary" link size="small" @click="viewSnapshot(row)">{{ t('runtime.viewConfig') }}</el-button>
                <el-button type="danger" link size="small" @click="delSnapshot(row)">{{ t('common.delete') }}</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>

      <!-- ============ 任务 ============ -->
      <el-tab-pane :label="t('runtime.jobs')" name="jobs">
        <el-card shadow="never">
          <el-table :data="jobList" v-loading="jobLoading" border stripe>
            <el-table-column prop="id" label="ID" width="70" />
            <el-table-column prop="job_type" :label="t('runtime.jobType')" width="130" />
            <el-table-column prop="trigger_source" :label="t('runtime.triggerSource')" width="130" />
            <el-table-column prop="reason" :label="t('runtime.reason')" min-width="160" show-overflow-tooltip />
            <el-table-column prop="snapshot_id" :label="t('runtime.snapshot')" width="90" />
            <el-table-column :label="t('runtime.status')" width="110">
              <template #default="{ row }">
                <el-tag :type="row.status === 'success' ? 'success' : row.status === 'failed' ? 'danger' : 'warning'" size="small">{{ row.status }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="error_message" :label="t('runtime.error')" min-width="160" show-overflow-tooltip />
            <el-table-column prop="created_at" :label="t('common.time')" width="180" />
            <el-table-column :label="t('common.action')" width="190" fixed="right">
              <template #default="{ row }">
                <el-button type="primary" link size="small" @click="viewJob(row)">{{ t('runtime.detail') }}</el-button>
                <el-button v-if="row.status === 'failed'" type="warning" link size="small" @click="retryJobHandler(row)">{{ t('runtime.retry') }}</el-button>
                <el-button v-if="row.status === 'running' || row.status === 'pending'" type="danger" link size="small" @click="cancelJobHandler(row)">{{ t('runtime.cancel') }}</el-button>
              </template>
            </el-table-column>
          </el-table>
          <div class="pager">
            <el-pagination
              v-model:current-page="jobQuery.page"
              v-model:page-size="jobQuery.limit"
              :total="jobTotal"
              :page-sizes="[10, 20, 50]"
              layout="total, sizes, prev, pager, next"
              @current-change="loadJobs"
              @size-change="loadJobs(1)"
            />
          </div>
        </el-card>
      </el-tab-pane>
    </el-tabs>

    <!-- 快照配置查看 -->
    <el-dialog v-model="snapDialog" :title="t('runtime.viewConfig')" width="700px">
      <pre class="code-block">{{ snapContent }}</pre>
    </el-dialog>

    <!-- 任务详情 -->
    <el-dialog v-model="jobDialog" :title="`${t('runtime.detail')} #${jobDetail.job?.id}`" width="800px">
      <template v-if="jobDetail.job">
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item :label="t('runtime.jobType')">{{ jobDetail.job.job_type }}</el-descriptions-item>
          <el-descriptions-item :label="t('runtime.status')">{{ jobDetail.job.status }}</el-descriptions-item>
          <el-descriptions-item :label="t('runtime.triggerSource')">{{ jobDetail.job.trigger_source }}</el-descriptions-item>
          <el-descriptions-item :label="t('runtime.snapshot')">{{ jobDetail.job.snapshot_id }}</el-descriptions-item>
          <el-descriptions-item :label="t('runtime.reason')" :span="2">{{ jobDetail.job.reason }}</el-descriptions-item>
          <el-descriptions-item :label="t('runtime.error')" :span="2">{{ jobDetail.job.error_message || '-' }}</el-descriptions-item>
        </el-descriptions>
        <el-divider>{{ t('runtime.targets') }}</el-divider>
        <el-table :data="jobDetail.targets || []" border size="small">
          <el-table-column prop="server_id" :label="t('runtime.serverId')" width="100" />
          <el-table-column prop="core_type" :label="t('runtime.coreType')" width="120" />
          <el-table-column prop="file_path" :label="t('runtime.file')" min-width="200" show-overflow-tooltip />
          <el-table-column prop="status" :label="t('runtime.status')" width="100" />
        </el-table>
        <el-divider>{{ t('runtime.events') }}</el-divider>
        <el-timeline>
          <el-timeline-item
            v-for="(e, i) in (jobDetail.events || [])"
            :key="i"
            :timestamp="e.created_at"
            :type="e.status === 'success' ? 'success' : e.status === 'failure' ? 'danger' : ''"
          >
            {{ e.event_type }} — {{ e.message }}
          </el-timeline-item>
        </el-timeline>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  runtimeOverview, listSnapshots, createSnapshot, deleteSnapshot, getSnapshot,
  dryRun, applyRuntime, rollbackRuntime, disableProxy,
  listJobs, getJob, retryJob, cancelJob,
} from '../api/runtime'

const { t } = useI18n()
const activeTab = ref('actions')

const overview = ref({})
function loadOverview() {
  runtimeOverview().then((r) => { overview.value = r || {} }).catch(() => {})
}

// ---------------- 快照列表 ----------------
const snapshotList = ref([])
const snapListLoading = ref(false)
function loadSnapshots() {
  snapListLoading.value = true
  listSnapshots({ limit: 200, offset: 0 }).then((r) => { snapshotList.value = r.items || [] }).catch(() => {}).finally(() => { snapListLoading.value = false })
}

// ---------------- 操作 ----------------
const snapReason = ref('')
const snapLoading = ref(false)
function doSnapshot() {
  snapLoading.value = true
  createSnapshot({ trigger_source: 'manual', reason: snapReason.value || 'manual' })
    .then(() => { ElMessage.success(t('common.success')); snapReason.value = ''; loadSnapshots(); loadOverview() })
    .catch(() => {}).finally(() => { snapLoading.value = false })
}

const dryResult = ref(null)
function doDryRun() {
  dryRun().then((r) => { dryResult.value = r }).catch(() => {})
}

const applyReason = ref('')
const applyLoading = ref(false)
const lastJobId = ref(null)
function doApply() {
  applyLoading.value = true
  applyRuntime({ trigger_source: 'manual', reason: applyReason.value || 'manual', operation_kind: 'full' })
    .then((r) => { lastJobId.value = r.id; ElMessage.success(t('common.success')); applyReason.value = ''; loadJobs(); loadOverview() })
    .catch(() => {}).finally(() => { applyLoading.value = false })
}

const rollbackSnap = ref(null)
const rollbackReason = ref('')
const rollbackLoading = ref(false)
function doRollback() {
  if (!rollbackSnap.value) return
  rollbackLoading.value = true
  rollbackRuntime({ snapshot_id: rollbackSnap.value, trigger_source: 'manual', reason: rollbackReason.value || 'rollback' })
    .then((r) => { lastJobId.value = r.id; ElMessage.success(t('common.success')); rollbackSnap.value = null; rollbackReason.value = ''; loadJobs(); loadOverview() })
    .catch(() => {}).finally(() => { rollbackLoading.value = false })
}

const disableReason = ref('')
const disableLoading = ref(false)
function doDisable() {
  ElMessageBox.confirm(t('runtime.disableConfirm'), t('common.confirm'), { type: 'warning' })
    .then(() => {
      disableLoading.value = true
      disableProxy({ trigger_source: 'manual', reason: disableReason.value || 'emergency disable' })
        .then((r) => { lastJobId.value = r.id; ElMessage.success(t('common.success')); disableReason.value = ''; loadJobs(); loadOverview() })
        .catch(() => {}).finally(() => { disableLoading.value = false })
    }).catch(() => {})
}

// ---------------- 快照查看/删除 ----------------
const snapDialog = ref(false)
const snapContent = ref('')
function viewSnapshot(row) {
  getSnapshot(row.id).then((r) => {
    try { snapContent.value = JSON.stringify(JSON.parse(r.config_json), null, 2) } catch (e) { snapContent.value = r.config_json }
    snapDialog.value = true
  }).catch(() => {})
}
function delSnapshot(row) {
  ElMessageBox.confirm(t('runtime.deleteSnapConfirm', { id: row.id }), t('common.confirm'), { type: 'warning' })
    .then(() => deleteSnapshot(row.id).then(() => { ElMessage.success(t('common.success')); loadSnapshots() }))
    .catch(() => {})
}

// ---------------- 任务 ----------------
const jobList = ref([])
const jobTotal = ref(0)
const jobLoading = ref(false)
const jobQuery = reactive({ page: 1, limit: 20 })
function loadJobs(p) {
  if (p) jobQuery.page = p
  jobLoading.value = true
  const params = { limit: jobQuery.limit, offset: (jobQuery.page - 1) * jobQuery.limit }
  listJobs(params).then((r) => { jobList.value = r.items || []; jobTotal.value = r.total || 0 }).catch(() => {}).finally(() => { jobLoading.value = false })
}
const jobDialog = ref(false)
const jobDetail = ref({})
function viewJob(row) {
  getJob(row.id).then((r) => { jobDetail.value = r; jobDialog.value = true }).catch(() => {})
}
function retryJobHandler(row) {
  retryJob(row.id).then(() => { ElMessage.success(t('common.success')); loadJobs() }).catch(() => {})
}
function cancelJobHandler(row) {
  ElMessageBox.confirm(t('runtime.cancelConfirm', { id: row.id }), t('common.confirm'), { type: 'warning' })
    .then(() => cancelJob(row.id).then(() => { ElMessage.success(t('common.success')); loadJobs() }))
    .catch(() => {})
}

onMounted(() => { loadOverview(); loadSnapshots(); loadJobs(1) })
</script>

<style scoped>
.overview { margin-bottom: 16px; }
.stat { text-align: center; }
.stat .num { font-size: 26px; font-weight: 600; }
.stat .num.small { font-size: 20px; }
.stat .num.success { color: #67c23a; }
.stat .num.danger { color: #f56c6c; }
.stat .lbl { color: #909399; font-size: 13px; margin-top: 4px; }
.action-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 16px; }
.action-item { border: 1px solid #ebeef5; border-radius: 6px; padding: 14px; }
.action-item.danger-zone { border-color: #fbc4c4; background: #fef0f0; }
.action-title { font-weight: 600; margin-bottom: 8px; }
.action-desc { color: #909399; font-size: 12px; margin-bottom: 8px; }
.mb8 { margin-bottom: 8px; }
.mt8 { margin-top: 8px; }
.code-block { background: #1e1e1e; color: #d4d4d4; padding: 12px; border-radius: 6px; max-height: 60vh; overflow: auto; font-size: 12px; }
.pager { margin-top: 14px; display: flex; justify-content: flex-end; }
</style>
