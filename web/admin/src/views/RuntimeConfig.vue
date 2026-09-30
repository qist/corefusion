<template>
  <div class="app-container">
    <!-- 概览 -->
    <el-row :gutter="16" class="overview">
      <el-col :span="6"><el-card shadow="never"><div class="stat"><div class="num">{{ overview.snapshot_count || 0 }}</div><div class="lbl">{{ t('runtime.snapshots') }}</div></div></el-card></el-col>
      <el-col :span="6"><el-card shadow="never"><div class="stat"><div class="num">{{ overview.job_count || 0 }}</div><div class="lbl">{{ t('runtime.jobs') }}</div></div></el-card></el-col>
      <el-col :span="6"><el-card shadow="never"><div class="stat"><div class="num small">{{ overview.last_sync ? showTime(overview.last_sync) : '-' }}</div><div class="lbl">{{ t('runtime.lastSync') }}</div></div></el-card></el-col>
      <el-col :span="6"><el-card shadow="never"><div class="stat">
        <template v-if="overview.last_job">
          <el-tag :type="statusTagType(overview.last_job.status)" size="small">#{{ overview.last_job.id }}</el-tag>
          <el-tag :type="verifyTagType(overview.last_job.verify_status)" size="small" class="ml-1">{{ verifyLabel(overview.last_job.verify_status) }}</el-tag>
        </template>
        <span v-else class="num small">-</span>
        <div class="lbl">{{ t('runtime.recentJob') }}</div>
      </div></el-card></el-col>
    </el-row>

    <el-tabs v-model="activeTab">
      <!-- ============ 操作 ============ -->
      <el-tab-pane :label="t('runtime.actions')" name="actions">
        <el-card shadow="never">
          <div class="op-target">
            <span class="op-target-label">{{ t('runtime.opSelectNodes') }}</span>
            <el-select
              v-model="opNodes"
              multiple
              collapse-tags
              :placeholder="t('runtime.verifySelectNodes')"
              size="small"
              style="width: 420px; max-width: 100%;"
            >
              <el-option
                v-for="n in nodes"
                :key="n.server_id"
                :label="(n.name || ('#' + n.server_id)) + (n.is_local ? '（' + t('runtime.nodeLocal') + '）' : '')"
                :value="n.server_id"
              />
            </el-select>
            <span v-if="opNodes.length" class="muted ml-1">{{ opNodes.length }} {{ t('runtime.nodeCount') }}</span>
            <span v-else class="muted ml-1">{{ t('runtime.allNodes') }}</span>
          </div>
          <div class="action-grid">
            <div class="action-item">
              <div class="action-title">{{ t('runtime.createSnapshot') }}</div>
              <div class="op-scope mb8">{{ t('runtime.targetNodes') }}：<template v-if="opNodes.length">{{ selectedNodeNames.join('、') }}</template><span v-else>{{ t('runtime.allNodes') }}</span></div>
              <el-input v-model="snapReason" :placeholder="t('runtime.reasonPlaceholder')" size="small" class="mb8" />
              <el-button type="primary" :loading="snapLoading" @click="doSnapshot">{{ t('runtime.createSnapshot') }}</el-button>
            </div>

            <div class="action-item">
              <div class="action-title">{{ t('runtime.dryRun') }}</div>
              <div class="op-scope mb8">{{ t('runtime.targetNodes') }}：<template v-if="opNodes.length">{{ selectedNodeNames.join('、') }}</template><span v-else>{{ t('runtime.allNodes') }}</span></div>
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
              <div class="op-scope mb8">{{ t('runtime.targetNodes') }}：<template v-if="opNodes.length">{{ selectedNodeNames.join('、') }}</template><span v-else>{{ t('runtime.allNodes') }}</span></div>
              <el-input v-model="applyReason" :placeholder="t('runtime.reasonPlaceholder')" size="small" class="mb8" />
              <el-button type="success" :loading="applyLoading" @click="doApply">{{ t('runtime.runApply') }}</el-button>
            </div>

            <div class="action-item danger-zone">
              <div class="action-title">{{ t('runtime.rollback') }}</div>
              <div class="op-scope mb8">{{ t('runtime.targetNodes') }}：<template v-if="opNodes.length">{{ selectedNodeNames.join('、') }}</template><span v-else>{{ t('runtime.allNodes') }}</span></div>
              <el-select v-model="rollbackSnap" :placeholder="t('runtime.selectSnapshot')" size="small" class="mb8" style="width:100%">
                <el-option v-for="s in snapshotList" :key="s.id" :label="`#${s.id} ${s.trigger_source} ${s.created_at}`" :value="s.id" />
              </el-select>
              <el-input v-model="rollbackReason" :placeholder="t('runtime.reasonPlaceholder')" size="small" class="mb8" />
              <el-button type="warning" :disabled="!rollbackSnap" :loading="rollbackLoading" @click="doRollback">{{ t('runtime.runRollback') }}</el-button>
            </div>

            <div class="action-item danger-zone">
              <div class="action-title">{{ t('runtime.disableProxy') }}</div>
              <div class="op-scope mb8">{{ t('runtime.targetNodes') }}：<template v-if="opNodes.length">{{ selectedNodeNames.join('、') }}</template><span v-else>{{ t('runtime.allNodes') }}</span></div>
              <div class="action-desc">{{ t('runtime.disableDesc') }}</div>
              <el-input v-model="disableReason" :placeholder="t('runtime.reasonPlaceholder')" size="small" class="mb8" />
              <el-button type="danger" :loading="disableLoading" @click="doDisable">{{ t('runtime.runDisable') }}</el-button>
            </div>

            <div class="action-item">
              <div class="action-title">{{ t('runtime.verifyNode') }}</div>
              <div class="action-desc">{{ t('runtime.verifyNodeDesc') }}</div>
              <el-select
                v-model="verifySelected"
                :placeholder="t('runtime.verifySelectNodes')"
                size="small"
                multiple
                collapse-tags
                class="mb8"
                style="width:100%"
              >
                <el-option
                  v-for="n in nodes"
                  :key="n.server_id"
                  :label="(n.name || ('#' + n.server_id)) + (n.is_local ? '（' + t('runtime.nodeLocal') + '）' : '')"
                  :value="n.server_id"
                />
              </el-select>
              <div class="verify-btns">
                <el-button type="primary" :loading="verifyLoading" @click="doVerify">{{ t('runtime.verifyRun') }}</el-button>
                <el-button :loading="verifyLoading" @click="doVerifyAll">{{ t('runtime.verifyAllNodes') }}</el-button>
              </div>
              <el-alert
                v-if="verifySummary"
                :closable="false"
                class="mt8"
                :type="verifySummary.fail > 0 ? 'warning' : 'success'"
                :title="t('runtime.verifyResultSummary', verifySummary)"
              />
              <div v-if="verifyResult && verifyResult.length" class="mt8 verify-list">
                <div v-for="r in verifyResult" :key="r.server_id" class="verify-row">
                  <el-tag :type="r.online ? 'success' : 'info'" size="small">{{ r.online ? t('runtime.online') : t('runtime.offline') }}</el-tag>
                  <span class="ml-1">{{ r.name || ('#' + r.server_id) }}</span>
                  <el-tag :type="r.verify_ok ? 'success' : 'danger'" size="small" class="ml-1">{{ r.verify_ok ? t('runtime.nodeVerifyOk') : t('runtime.nodeVerifyFail') }}</el-tag>
                  <span v-if="r.reason" class="muted ml-1">{{ r.reason }}</span>
                  <span v-for="c in (r.cores || [])" :key="c.core_type" class="ml-1">
                    <el-tag :type="c.crashed ? 'danger' : 'success'" size="small">{{ c.core_type }}: {{ c.state }}</el-tag>
                  </span>
                </div>
              </div>
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
          <el-table v-if="!isMobile" :data="snapshotList" v-loading="snapListLoading" border stripe>
            <el-table-column prop="id" label="ID" width="70" />
            <el-table-column prop="trigger_source" :label="t('runtime.triggerSource')" width="130" />
            <el-table-column prop="reason" :label="t('runtime.reason')" min-width="180" show-overflow-tooltip />
          <el-table-column :label="t('runtime.targetNodes')" min-width="220">
            <template #default="{ row }">
              <template v-if="targetNodeNames(row.target_server_ids_json).length">
                <el-tag v-for="(nm, i) in targetNodeNames(row.target_server_ids_json)" :key="i" size="small" class="ml-1">{{ nm }}</el-tag>
              </template>
              <span v-else class="muted">{{ t('runtime.allNodes') }}</span>
            </template>
          </el-table-column>
            <el-table-column prop="config_hash" :label="t('runtime.hash')" width="160" show-overflow-tooltip />
            <el-table-column prop="created_at" :label="t('common.time')" width="180" />
            <el-table-column :label="t('common.action')" width="160" fixed="right">
              <template #default="{ row }">
                <el-button type="primary" link size="small" @click="viewSnapshot(row)">{{ t('runtime.viewConfig') }}</el-button>
                <el-button type="danger" link size="small" @click="delSnapshot(row)">{{ t('common.delete') }}</el-button>
              </template>
            </el-table-column>
          </el-table>
          <MobileCards v-else :rows="snapshotList" :title-fn="snapshotCardTitle" :fields="snapshotCardFields">
            <template #actions="{ row }">
              <el-button type="primary" link size="small" @click="viewSnapshot(row)">{{ t('runtime.viewConfig') }}</el-button>
              <el-button type="danger" link size="small" @click="delSnapshot(row)">{{ t('common.delete') }}</el-button>
            </template>
          </MobileCards>
        </el-card>
      </el-tab-pane>

      <!-- ============ 任务 ============ -->
      <el-tab-pane :label="t('runtime.jobs')" name="jobs">
        <el-card shadow="never">
          <el-table v-if="!isMobile" :data="jobList" v-loading="jobLoading" border stripe>
            <el-table-column prop="id" label="ID" width="70" />
            <el-table-column :label="t('runtime.jobType')" width="130">
              <template #default="{ row }">{{ opLabel(row.operation_kind) }}</template>
            </el-table-column>
            <el-table-column prop="trigger_source" :label="t('runtime.triggerSource')" width="130" />
            <el-table-column prop="reason" :label="t('runtime.reason')" min-width="160" show-overflow-tooltip />
            <el-table-column prop="snapshot_id" :label="t('runtime.snapshot')" width="90" />
            <el-table-column :label="t('runtime.status')" width="110">
              <template #default="{ row }">
                <el-tag :type="statusTagType(row.status)" size="small">{{ row.status }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column :label="t('runtime.verifyStatus')" width="120">
              <template #default="{ row }">
                <el-tag :type="verifyTagType(row.verify_status)" size="small">{{ verifyLabel(row.verify_status) }}</el-tag>
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
          <MobileCards v-else :rows="jobList" :title-fn="jobCardTitle" :fields="runtimeJobCardFields">
            <template #actions="{ row }">
              <el-button type="primary" link size="small" @click="viewJob(row)">{{ t('runtime.detail') }}</el-button>
              <el-button v-if="row.status === 'failed'" type="warning" link size="small" @click="retryJobHandler(row)">{{ t('runtime.retry') }}</el-button>
              <el-button v-if="row.status === 'running' || row.status === 'pending'" type="danger" link size="small" @click="cancelJobHandler(row)">{{ t('runtime.cancel') }}</el-button>
            </template>
          </MobileCards>
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

      <!-- ============ 节点验证 ============ -->
      <el-tab-pane :label="t('runtime.tabNodes')" name="nodes">
        <el-card shadow="never">
          <div class="mb8" style="display:flex;justify-content:space-between;align-items:center;">
            <span class="muted">{{ t('runtime.tabNodes') }} · {{ nodes.length }}</span>
            <el-button type="primary" size="small" :loading="nodesLoading" @click="loadNodes">{{ t('runtime.refresh') }}</el-button>
          </div>
          <el-alert v-if="!nodes.length && !nodesLoading" :closable="false" type="info" class="mb8">
            {{ t('runtime.noNodes') }}
          </el-alert>
          <el-table v-if="!isMobile" :data="nodes" v-loading="nodesLoading" border stripe>
            <el-table-column prop="server_id" label="ID" width="70" />
            <el-table-column :label="t('runtime.nodeName')" min-width="160">
              <template #default="{ row }">
                <span>{{ row.name || ('#' + row.server_id) }}</span>
                <el-tag v-if="row.is_local" size="small" type="info" class="ml-1">{{ t('runtime.nodeLocal') }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="hostname" :label="t('runtime.nodeHost')" min-width="140" show-overflow-tooltip />
            <el-table-column prop="region" :label="t('runtime.nodeRegion')" width="120" />
            <el-table-column :label="t('runtime.agentStatus')" width="120">
              <template #default="{ row }">
                <el-tag :type="row.online ? 'success' : 'info'" size="small">{{ row.online ? t('runtime.online') : t('runtime.offline') }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column :label="t('runtime.targetStatus')" min-width="140">
              <template #default="{ row }">
                <el-tag :type="row.verify_ok ? 'success' : 'danger'" size="small">
                  {{ row.verify_ok ? t('runtime.nodeVerifyOk') : t('runtime.nodeVerifyFail') }}
                </el-tag>
                <div class="muted" v-if="row.reason">{{ row.reason }}</div>
              </template>
            </el-table-column>
            <el-table-column :label="t('runtime.coreStates')" min-width="220">
              <template #default="{ row }">
                <template v-if="row.cores && row.cores.length">
                  <div v-for="c in row.cores" :key="c.core_type" class="core-state">
                    <el-tag :type="c.crashed ? 'danger' : 'success'" size="small">{{ c.core_type }}</el-tag>
                    <span class="ml-1">{{ c.state }}</span>
                    <span v-if="c.crashed" class="danger ml-1">· {{ t('runtime.verifyCrash') }}</span>
                  </div>
                </template>
                <span v-else class="muted">—</span>
              </template>
            </el-table-column>
          </el-table>
          <MobileCards v-else :rows="nodes" :title-fn="nodeCardTitle" :fields="runtimeNodeCardFields" />
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
          <el-descriptions-item :label="t('runtime.jobType')">{{ opLabel(jobDetail.job.operation_kind) }}</el-descriptions-item>
          <el-descriptions-item :label="t('runtime.status')">
            <el-tag :type="statusTagType(jobDetail.job.status)" size="small">{{ jobDetail.job.status }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item :label="t('runtime.triggerSource')">{{ jobDetail.job.trigger_source }}</el-descriptions-item>
          <el-descriptions-item :label="t('runtime.snapshot')">{{ jobDetail.job.snapshot_id }}</el-descriptions-item>
            <el-descriptions-item :label="t('runtime.targetNodes')">
              <template v-if="targetNodeNames(jobDetail.job.target_server_ids_json).length">
                <el-tag v-for="(nm, i) in targetNodeNames(jobDetail.job.target_server_ids_json)" :key="i" size="small" class="ml-1">{{ nm }}</el-tag>
              </template>
              <span v-else class="muted">{{ t('runtime.allNodes') }}</span>
            </el-descriptions-item>
          <el-descriptions-item :label="t('runtime.verifyStatus')">
            <el-tag :type="verifyTagType(jobDetail.job.verify_status)" size="small">{{ verifyLabel(jobDetail.job.verify_status) }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item :label="t('runtime.verifyTargetCount')">{{ jobDetail.job.target_count || 0 }}</el-descriptions-item>
          <el-descriptions-item :label="t('runtime.verifySuccessCount')">{{ jobDetail.job.success_count || 0 }}</el-descriptions-item>
          <el-descriptions-item :label="t('runtime.verifyFailedCount')">{{ jobDetail.job.failed_count || 0 }}</el-descriptions-item>
          <el-descriptions-item :label="t('runtime.reason')" :span="2">{{ jobDetail.job.reason }}</el-descriptions-item>
          <el-descriptions-item :label="t('runtime.error')" :span="2">{{ jobDetail.job.error_message || '-' }}</el-descriptions-item>
        </el-descriptions>
        <el-divider>{{ t('runtime.targets') }}</el-divider>
        <el-table v-if="!isMobile" :data="jobDetail.targets || []" border size="small">
          <el-table-column type="expand">
            <template #default="{ row }">
              <div v-if="row.detail_json">
                <div v-if="parseCores(row.detail_json).length" class="core-states">
                  <div v-for="c in parseCores(row.detail_json)" :key="c.core_type" class="core-state">
                    <el-tag :type="c.crashed ? 'danger' : 'success'" size="small">{{ c.core_type || '-' }}</el-tag>
                    <span class="ml-1">{{ c.state || '-' }}</span>
                    <span v-if="c.crashed" class="danger ml-1">· {{ t('runtime.verifyCrash') }}</span>
                  </div>
                </div>
                <pre v-else class="code-block-sm">{{ row.detail_json }}</pre>
              </div>
              <span v-else class="muted">{{ t('runtime.noCoreStates') }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="target_type" :label="t('runtime.targetType')" width="90" />
          <el-table-column prop="target_id" :label="t('runtime.targetId')" width="90" />
          <el-table-column :label="t('runtime.targetStatus')" width="130">
            <template #default="{ row }">
              <el-tag :type="row.status === 'verified_ok' ? 'success' : row.status === 'verified_failed' ? 'danger' : 'info'" size="small">
                {{ row.status === 'verified_ok' ? t('runtime.verifiedOk') : row.status === 'verified_failed' ? t('runtime.verifiedFailed') : row.status }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="error_message" :label="t('runtime.error')" min-width="200" show-overflow-tooltip />
        </el-table>
        <MobileCards v-else :rows="jobDetail.targets || []" :title-fn="targetCardTitle" :fields="runtimeTargetCardFields" />
        <el-divider>{{ t('runtime.events') }}</el-divider>
        <el-timeline>
          <el-timeline-item
            v-for="(e, i) in (jobDetail.events || [])"
            :key="i"
            :timestamp="e.created_at"
            :type="eventTagType(e.event_level)"
          >
            <strong>{{ e.event_type }}</strong>
            <span class="muted ml-1" v-if="e.phase">· {{ e.phase }}</span>
            <div>{{ e.message }}</div>
          </el-timeline-item>
        </el-timeline>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  runtimeOverview, listSnapshots, createSnapshot, deleteSnapshot, getSnapshot,
  dryRun, applyRuntime, rollbackRuntime, disableProxy,
  listJobs, getJob, retryJob, cancelJob, listRuntimeNodes, verifyRuntimeNodes,
} from '../api/runtime'
import { useIsMobile } from '../composables/useIsMobile'
import MobileCards from '../components/MobileCards.vue'

const { t } = useI18n()
const { isMobile } = useIsMobile()
const activeTab = ref('actions')

// ---------------- 标签/状态辅助 ----------------
function statusTagType(s) {
  if (s === 'success') return 'success'
  if (s === 'failed') return 'danger'
  if (s === 'cancelled') return 'info'
  return 'warning'
}
function verifyTagType(v) {
  if (v === 'ok') return 'success'
  if (v === 'partial') return 'warning'
  if (v === 'failed') return 'danger'
  if (v === 'verifying') return 'warning'
  return 'info'
}
function verifyLabel(v) {
  return ({
    pending: t('runtime.verifyPending'),
    verifying: t('runtime.verifyVerifying'),
    ok: t('runtime.verifyOk'),
    partial: t('runtime.verifyPartial'),
    failed: t('runtime.verifyFailed'),
    skipped: t('runtime.verifySkipped'),
  })[v] || v || '-'
}
function opLabel(k) {
  return ({ full: t('runtime.opFull'), rollback: t('runtime.rollback'), disable_proxy: t('runtime.disableProxy') })[k] || k || '-'
}
function eventTagType(l) {
  if (l === 'success') return 'success'
  if (l === 'error') return 'danger'
  if (l === 'warning') return 'warning'
  return ''
}
function parseCores(detailJson) {
  if (!detailJson) return []
  try {
    const d = JSON.parse(detailJson)
    if (Array.isArray(d)) return d
    if (d && Array.isArray(d.cores)) return d.cores
    return []
  } catch (e) { return [] }
}
function parseTargetNodes(json) {
  if (!json) return []
  try {
    const d = JSON.parse(json)
    return Array.isArray(d) ? d : []
  } catch (e) { return [] }
}
// Resolve node IDs to human-readable names for display.
const nodeMap = computed(() => {
  const m = {}
  for (const n of nodes.value) m[n.server_id] = n.name || ('#' + n.server_id)
  return m
})
// Names of the currently selected target nodes (shared op selector).
const selectedNodeNames = computed(() =>
  opNodes.value.map((id) => nodeMap.value[id] || ('#' + id))
)
// Resolve a stored target_server_ids_json into display names.
function targetNodeNames(json) {
  return parseTargetNodes(json).map((id) => {
    const name = nodeMap.value[id]
    return name ? `${name} (#${id})` : ('#' + id)
  })
}
function showTime(ts) {
  if (!ts) return '-'
  const d = new Date(ts)
  return isNaN(d.getTime()) ? String(ts) : d.toLocaleString()
}

// ---------------- 移动端卡片字段 ----------------
const snapshotCardTitle = (row) => row.id != null ? '#' + row.id + (row.trigger_source ? ' · ' + row.trigger_source : '') : ''
const snapshotCardFields = computed(() => [
  { label: t('runtime.reason'), text: (row) => row.reason || '-' },
  { label: t('runtime.targetNodes'), text: (row) => { const n = targetNodeNames(row.target_server_ids_json); return n.length ? n.join(', ') : t('runtime.allNodes') } },
  { label: t('runtime.hash'), text: (row) => row.config_hash || '-' },
  { label: t('common.time'), text: (row) => showTime(row.created_at) },
])
const jobCardTitle = (row) => row.id != null ? '#' + row.id + ' ' + (opLabel(row.operation_kind) || '') : ''
const runtimeJobCardFields = computed(() => [
  { label: t('runtime.triggerSource'), text: (row) => row.trigger_source || '-' },
  { label: t('runtime.reason'), text: (row) => row.reason || '-' },
  { label: t('runtime.snapshot'), text: (row) => row.snapshot_id ?? '-' },
  { label: t('runtime.status'), text: (row) => row.status || '-' },
  { label: t('runtime.verifyStatus'), text: (row) => verifyLabel(row.verify_status) },
  { label: t('runtime.error'), text: (row) => row.error_message || '-' },
  { label: t('common.time'), text: (row) => showTime(row.created_at) },
])
const nodeCardTitle = (row) => (row.name || ('#' + row.server_id)) + (row.is_local ? '（' + t('runtime.nodeLocal') + '）' : '')
const runtimeNodeCardFields = computed(() => [
  { label: t('runtime.nodeHost'), text: (row) => row.hostname || '-' },
  { label: t('runtime.nodeRegion'), text: (row) => row.region || '-' },
  { label: t('runtime.agentStatus'), text: (row) => row.online ? t('runtime.online') : t('runtime.offline') },
  { label: t('runtime.targetStatus'), text: (row) => (row.verify_ok ? t('runtime.nodeVerifyOk') : t('runtime.nodeVerifyFail')) + (row.reason ? ' · ' + row.reason : '') },
  { label: t('runtime.coreStates'), text: (row) => (row.cores && row.cores.length) ? row.cores.map(c => `${c.core_type}: ${c.state}${c.crashed ? ' · ' + t('runtime.verifyCrash') : ''}`).join(', ') : '-' },
])
const targetCardTitle = (row) => `${row.target_type || '-'} #${row.target_id ?? '-'}`
const runtimeTargetCardFields = computed(() => [
  { label: t('runtime.targetStatus'), text: (row) => row.status === 'verified_ok' ? t('runtime.verifiedOk') : row.status === 'verified_failed' ? t('runtime.verifiedFailed') : (row.status || '-') },
  { label: t('runtime.coreStates'), text: (row) => { const cs = parseCores(row.detail_json); if (cs.length) return cs.map(c => `${c.core_type || '-'}: ${c.state || '-'}${c.crashed ? ' · ' + t('runtime.verifyCrash') : ''}`).join(', '); return row.detail_json || t('runtime.noCoreStates') } },
  { label: t('runtime.error'), text: (row) => row.error_message || '-' },
])

const overview = ref({})
function loadOverview() {
  runtimeOverview().then((r) => { overview.value = r || {} }).catch(() => {})
}

// ---------------- 节点验证 ----------------
const nodes = ref([])
const nodesLoading = ref(false)
function loadNodes() {
  nodesLoading.value = true
  listRuntimeNodes().then((r) => { nodes.value = (r && r.nodes) || [] }).catch(() => {}).finally(() => { nodesLoading.value = false })
}

// ---------------- 快照列表 ----------------
const snapshotList = ref([])
const snapListLoading = ref(false)
function loadSnapshots() {
  snapListLoading.value = true
  listSnapshots({ limit: 200, offset: 0 }).then((r) => { snapshotList.value = r.items || [] }).catch(() => {}).finally(() => { snapListLoading.value = false })
}

// ---------------- 操作 ----------------
const opNodes = ref([]) // 共享的目标节点选择（留空=全部）
const snapReason = ref('')
const snapLoading = ref(false)
function doSnapshot() {
  snapLoading.value = true
  createSnapshot({ trigger_source: 'manual', reason: snapReason.value || 'manual', server_ids: opNodes.value })
    .then(() => { ElMessage.success(t('common.success')); snapReason.value = ''; loadSnapshots(); loadOverview() })
    .catch(() => {}).finally(() => { snapLoading.value = false })
}

const dryResult = ref(null)
function doDryRun() {
  dryRun({ server_ids: opNodes.value }).then((r) => { dryResult.value = r }).catch(() => {})
}

const applyReason = ref('')
const applyLoading = ref(false)
const lastJobId = ref(null)
function doApply() {
  applyLoading.value = true
  applyRuntime({ trigger_source: 'manual', reason: applyReason.value || 'manual', operation_kind: 'full', server_ids: opNodes.value })
    .then((r) => { lastJobId.value = r.id; ElMessage.success(t('common.success')); applyReason.value = ''; loadJobs(); loadOverview() })
    .catch(() => {}).finally(() => { applyLoading.value = false })
}

const rollbackSnap = ref(null)
const rollbackReason = ref('')
const rollbackLoading = ref(false)
function doRollback() {
  if (!rollbackSnap.value) return
  rollbackLoading.value = true
  rollbackRuntime({ snapshot_id: rollbackSnap.value, trigger_source: 'manual', reason: rollbackReason.value || 'rollback', server_ids: opNodes.value })
    .then((r) => { lastJobId.value = r.id; ElMessage.success(t('common.success')); rollbackSnap.value = null; rollbackReason.value = ''; loadJobs(); loadOverview() })
    .catch(() => {}).finally(() => { rollbackLoading.value = false })
}

const disableReason = ref('')
const disableLoading = ref(false)
function doDisable() {
  ElMessageBox.confirm(t('runtime.disableConfirm'), t('common.confirm'), { type: 'warning' })
    .then(() => {
      disableLoading.value = true
      disableProxy({ trigger_source: 'manual', reason: disableReason.value || 'emergency disable', server_ids: opNodes.value })
        .then((r) => { lastJobId.value = r.id; ElMessage.success(t('common.success')); disableReason.value = ''; loadJobs(); loadOverview() })
        .catch(() => {}).finally(() => { disableLoading.value = false })
    }).catch(() => {})
}

// ---------------- 按节点验证 ----------------
const verifySelected = ref([])
const verifyLoading = ref(false)
const verifyResult = ref([])
const verifySummary = ref(null)
function runVerify(ids) {
  verifyLoading.value = true
  verifyResult.value = []
  verifySummary.value = null
  verifyRuntimeNodes(ids)
    .then((r) => {
      const ns = (r && r.nodes) || []
      verifyResult.value = ns
      const ok = ns.filter((n) => n.verify_ok).length
      verifySummary.value = { ok, fail: ns.length - ok, total: ns.length }
    })
    .catch(() => {})
    .finally(() => { verifyLoading.value = false })
}
function doVerify() {
  if (!verifySelected.value.length) {
    ElMessage.warning(t('runtime.verifySelectNodes'))
    return
  }
  runVerify(verifySelected.value)
}
function doVerifyAll() {
  verifySelected.value = []
  runVerify([])
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

onMounted(() => { loadOverview(); loadSnapshots(); loadJobs(1); loadNodes() })
</script>

<style scoped>
.overview { margin-bottom: 16px; }
.stat { text-align: center; }
.stat .num { font-size: 26px; font-weight: 600; }
.stat .num.small { font-size: 20px; }
.stat .num.success { color: #67c23a; }
.stat .num.danger { color: #f56c6c; }
.stat .lbl { color: #909399; font-size: 13px; margin-top: 4px; }
.action-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 16px; margin-top: 12px; }
.op-target { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; padding-bottom: 12px; border-bottom: 1px solid #ebeef5; }
.op-target-label { font-weight: 600; color: #606266; font-size: 13px; }
.op-scope { font-size: 12px; color: #409eff; background: #ecf5ff; border: 1px solid #d9ecff; border-radius: 4px; padding: 4px 8px; line-height: 1.5; }
.action-item { border: 1px solid #ebeef5; border-radius: 6px; padding: 14px; }
.action-item.danger-zone { border-color: #fbc4c4; background: #fef0f0; }
.action-title { font-weight: 600; margin-bottom: 8px; }
.action-desc { color: #909399; font-size: 12px; margin-bottom: 8px; }
.verify-btns { display: flex; gap: 8px; }
.verify-list { max-height: 320px; overflow: auto; }
.verify-row { margin: 6px 0; display: flex; align-items: center; flex-wrap: wrap; }
.mb8 { margin-bottom: 8px; }
.mt8 { margin-top: 8px; }
.code-block { background: #1e1e1e; color: #d4d4d4; padding: 12px; border-radius: 6px; max-height: 60vh; overflow: auto; font-size: 12px; }
.pager { margin-top: 14px; display: flex; justify-content: flex-end; }
.ml-1 { margin-left: 4px; }
.muted { color: #909399; }
.danger { color: #f56c6c; }
.core-states { padding: 4px 0; }
.core-state { margin: 4px 0; }
.code-block-sm { background: #1e1e1e; color: #d4d4d4; padding: 8px; border-radius: 6px; max-height: 240px; overflow: auto; font-size: 12px; white-space: pre-wrap; word-break: break-all; }
</style>
