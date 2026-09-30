<template>
  <div>
    <div style="display:flex;justify-content:space-between;margin-bottom:16px">
      <h3 style="margin:0">{{ t('ticket.title') }}</h3>
      <div style="display:flex;gap:8px;align-items:center">
        <el-select v-model="statusFilter" size="small" :placeholder="t('ticket.statusFilter')" clearable style="width:120px">
          <el-option :label="t('ticket.statusOpen')" value="open" />
          <el-option :label="t('ticket.statusReplied')" value="replied" />
          <el-option :label="t('ticket.statusClosed')" value="closed" />
        </el-select>
        <el-select v-model="categoryFilter" size="small" :placeholder="t('ticket.categoryFilter')" clearable style="width:120px">
          <el-option :label="t('ticket.categoryTechnical')" value="technical" />
          <el-option :label="t('ticket.categoryBilling')" value="billing" />
          <el-option :label="t('ticket.categoryAccount')" value="account" />
          <el-option :label="t('ticket.categoryOther')" value="other" />
        </el-select>
        <el-button size="small" @click="fetchTickets">{{ t('ticket.search') }}</el-button>
      </div>
    </div>
    <el-table v-if="!isMobile" :data="tickets" v-loading="loading" stripe size="small">
      <el-table-column prop="id" label="ID" width="60" />
      <el-table-column prop="client_id" :label="t('ticket.clientId')" width="90" />
      <el-table-column prop="subject" :label="t('ticket.subject')" show-overflow-tooltip />
      <el-table-column :label="t('ticket.category')" width="80">
        <template #default="{row}">
          <el-tag size="small">{{ categoryLabel(row.category) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="t('ticket.priority')" width="80">
        <template #default="{row}">
          <el-tag :type="priorityType(row.priority)" size="small">{{ priorityLabel(row.priority) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="t('common.status')" width="80">
        <template #default="{row}">
          <el-tag :type="statusType(row.status)" size="small">{{ statusLabel(row.status) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="updated_at" :label="t('common.updatedAt')" width="170" />
      <el-table-column :label="t('common.actions')" width="200">
        <template #default="{row}">
          <div style="display:flex;gap:2px;align-items:center;white-space:nowrap">
            <el-button size="small" @click="viewTicket(row)">{{ t('ticket.viewReply') }}</el-button>
            <el-button size="small" type="primary" @click="assignTicketTo(row)" v-if="row.status!=='closed'">{{ t('ticket.assign') }}</el-button>
            <el-button size="small" type="danger" @click="closeTicket(row)" v-if="row.status!=='closed'">{{ t('ticket.close') }}</el-button>
          </div>
        </template>
      </el-table-column>
    </el-table>
    <MobileCards
      v-else
      :rows="tickets"
      title-field="subject"
      :fields="ticketCardFields"
    >
      <template #actions="{ row }">
        <el-button size="small" @click="viewTicket(row)">{{ t('ticket.viewReply') }}</el-button>
        <el-button size="small" type="primary" @click="assignTicketTo(row)" v-if="row.status!=='closed'">{{ t('ticket.assign') }}</el-button>
        <el-button size="small" type="danger" @click="closeTicket(row)" v-if="row.status!=='closed'">{{ t('ticket.close') }}</el-button>
      </template>
    </MobileCards>
    <PaginationBar v-model:page="page" v-model:page-size="pageSize" :total="total" @change="fetchTickets" />

    <el-dialog v-model="dialogVisible" :title="t('ticket.detail')" width="700px">
      <div v-if="currentTicket">
        <el-descriptions :column="2" border size="small" style="margin-bottom:16px">
          <el-descriptions-item label="ID">{{ currentTicket.id }}</el-descriptions-item>
          <el-descriptions-item :label="t('ticket.clientId')">{{ currentTicket.client_id }}</el-descriptions-item>
          <el-descriptions-item :label="t('ticket.subject')">{{ currentTicket.subject }}</el-descriptions-item>
          <el-descriptions-item :label="t('ticket.category')">{{ categoryLabel(currentTicket.category) }}</el-descriptions-item>
          <el-descriptions-item :label="t('ticket.priority')">{{ priorityLabel(currentTicket.priority) }}</el-descriptions-item>
          <el-descriptions-item :label="t('common.status')">{{ statusLabel(currentTicket.status) }}</el-descriptions-item>
        </el-descriptions>
        <div style="max-height:300px;overflow-y:auto;border:1px solid #eee;padding:12px;border-radius:4px">
          <div v-for="msg in messages" :key="msg.id" style="margin-bottom:12px">
            <div :style="{textAlign:msg.sender_type==='admin'?'right':'left'}">
              <el-tag :type="msg.sender_type==='admin'?'warning':'success'" size="small" style="margin-bottom:4px">{{ msg.sender_type==='admin'?t('ticket.admin'):t('ticket.user') }}</el-tag>
              <div style="background:#f5f7fa;padding:8px;border-radius:4px;display:inline-block;max-width:80%;text-align:left;white-space:pre-wrap">{{ msg.content }}</div>
              <div style="font-size:12px;color:#999;margin-top:2px">{{ msg.created_at }}</div>
            </div>
          </div>
        </div>
        <div style="margin-top:12px" v-if="currentTicket.status!=='closed'">
          <el-input v-model="replyContent" type="textarea" :rows="3" :placeholder="t('ticket.replyPlaceholder')" />
          <el-button type="primary" size="small" style="margin-top:8px" @click="submitReply" :loading="replyLoading">{{ t('ticket.sendReply') }}</el-button>
        </div>
      </div>
    </el-dialog>
    <el-dialog v-model="assignDialogVisible" :title="t('ticket.assignTicket')" width="420px">
      <el-select v-model="selectedAdminId" :loading="adminsLoading" filterable :placeholder="t('ticket.adminId')" style="width:100%">
        <el-option v-for="a in admins" :key="a.id" :label="`${a.username} (${a.role || 'user'})`" :value="a.id" />
      </el-select>
      <template #footer>
        <el-button @click="assignDialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="assignLoading" @click="confirmAssign">{{ t('common.confirm') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import client from '../api/client'
import { useQueryState } from '../composables/useQueryState'
import PaginationBar from '../components/PaginationBar.vue'
import MobileCards from '../components/MobileCards.vue'
import { useIsMobile } from '../composables/useIsMobile'
import { listTickets, getTicket, replyTicket, closeTicket as closeTicketApi, assignTicket } from '../api/clientext'

const { t } = useI18n()

const tickets = ref([])
const loading = ref(false)
const total = ref(0)
const page = useQueryState('tktPage', 1, { type: 'number' })
const pageSize = useQueryState('tktPageSize', 20, { type: 'number' })
const statusFilter = useQueryState('status', '', { omit: (v) => v === '' })
const categoryFilter = ref('')
const dialogVisible = ref(false)
const currentTicket = ref(null)
const messages = ref([])
const replyContent = ref('')
const replyLoading = ref(false)

async function fetchTickets() {
  loading.value = true
  try {
    const params = { limit: pageSize.value, offset: (page.value - 1) * pageSize.value }
    if (statusFilter.value) params.status = statusFilter.value
    if (categoryFilter.value) params.category = categoryFilter.value
    const result = await listTickets(params)
    tickets.value = result?.items || []
    total.value = result?.total || 0
  } finally { loading.value = false }
}

async function viewTicket(row) {
  const result = await getTicket(row.id)
  currentTicket.value = result?.ticket
  messages.value = result?.messages || []
  replyContent.value = ''
  dialogVisible.value = true
}

async function submitReply() {
  if (!replyContent.value.trim()) return ElMessage.warning(t('ticket.replyRequired'))
  replyLoading.value = true
  try {
    await replyTicket(currentTicket.value.id, { content: replyContent.value })
    ElMessage.success(t('ticket.replySuccess'))
    replyContent.value = ''
    const result = await getTicket(currentTicket.value.id)
    currentTicket.value = result?.ticket
    messages.value = result?.messages || []
    fetchTickets()
  } finally { replyLoading.value = false }
}

async function closeTicket(row) {
  await ElMessageBox.confirm(t('ticket.confirmClose'), t('common.confirm'))
  await closeTicketApi(row.id)
  ElMessage.success(t('ticket.closedSuccess'))
  fetchTickets()
}

// 工单指派：从管理员列表下拉选择，避免手输 admin ID
const admins = ref([])
const adminsLoading = ref(false)
const assignDialogVisible = ref(false)
const assignLoading = ref(false)
const selectedAdminId = ref(null)
const currentAssignRow = ref(null)

async function loadAdmins() {
  adminsLoading.value = true
  try {
    const r = await client.get('/users', { params: { limit: 200 } })
    admins.value = r.items || r || []
  } catch (e) { ElMessage.error(t('common.failed')) } finally { adminsLoading.value = false }
}

function assignTicketTo(row) {
  currentAssignRow.value = row
  selectedAdminId.value = null
  assignDialogVisible.value = true
  loadAdmins()
}

async function confirmAssign() {
  if (!selectedAdminId.value) {
    ElMessage.warning(t('ticket.invalidAdminId'))
    return
  }
  assignLoading.value = true
  try {
    await assignTicket(currentAssignRow.value.id, { admin_id: parseInt(selectedAdminId.value) })
    ElMessage.success(t('ticket.assignedSuccess'))
    assignDialogVisible.value = false
    fetchTickets()
  } catch (e) { ElMessage.error(t('common.failed')) } finally { assignLoading.value = false }
}

function categoryLabel(c) { return { technical: t('ticket.categoryTechnical'), billing: t('ticket.categoryBilling'), account: t('ticket.categoryAccount'), other: t('ticket.categoryOther') }[c] || c }
function priorityLabel(p) { return { low: t('ticket.priorityLow'), normal: t('ticket.priorityNormal'), high: t('ticket.priorityHigh'), urgent: t('ticket.priorityUrgent') }[p] || p }
function priorityType(p) { return { low: 'info', normal: '', high: 'warning', urgent: 'danger' }[p] || '' }
function statusLabel(s) { return { open: t('ticket.statusOpen'), replied: t('ticket.statusReplied'), closed: t('ticket.statusClosed') }[s] || s }
function statusType(s) { return { open: 'danger', replied: 'warning', closed: 'info' }[s] || '' }

const { isMobile } = useIsMobile()
// 手机端卡片字段（与桌面表格列一一对应，状态/分类/优先级复用现有本地化函数）
const ticketCardFields = computed(() => [
  { label: 'ID', text: (row) => String(row.id ?? '-') },
  { label: t('ticket.clientId'), text: (row) => row.client_id ?? '-' },
  { label: t('ticket.category'), text: (row) => categoryLabel(row.category) },
  { label: t('ticket.priority'), text: (row) => priorityLabel(row.priority) },
  { label: t('common.status'), text: (row) => statusLabel(row.status) },
  { label: t('common.updatedAt'), text: (row) => row.updated_at || '-' },
])

onMounted(fetchTickets)
</script>
