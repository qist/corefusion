<template>
  <div>
    <div style="display:flex;justify-content:space-between;margin-bottom:16px">
      <h3 style="margin:0">工单管理</h3>
      <div style="display:flex;gap:8px;align-items:center">
        <el-select v-model="statusFilter" size="small" placeholder="状态筛选" clearable style="width:120px">
          <el-option label="待处理" value="open" />
          <el-option label="已回复" value="replied" />
          <el-option label="已关闭" value="closed" />
        </el-select>
        <el-select v-model="categoryFilter" size="small" placeholder="分类筛选" clearable style="width:120px">
          <el-option label="技术" value="technical" />
          <el-option label="账务" value="billing" />
          <el-option label="账号" value="account" />
          <el-option label="其他" value="other" />
        </el-select>
        <el-button size="small" @click="fetchTickets">查询</el-button>
      </div>
    </div>
    <el-table :data="tickets" v-loading="loading" stripe size="small">
      <el-table-column prop="id" label="ID" width="60" />
      <el-table-column prop="client_id" label="客户端ID" width="90" />
      <el-table-column prop="subject" label="主题" show-overflow-tooltip />
      <el-table-column label="分类" width="80">
        <template #default="{row}">
          <el-tag size="small">{{ categoryLabel(row.category) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="优先级" width="80">
        <template #default="{row}">
          <el-tag :type="priorityType(row.priority)" size="small">{{ priorityLabel(row.priority) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="80">
        <template #default="{row}">
          <el-tag :type="statusType(row.status)" size="small">{{ statusLabel(row.status) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="updated_at" label="更新时间" width="170" />
      <el-table-column label="操作" width="200">
        <template #default="{row}">
          <el-button size="small" @click="viewTicket(row)">查看/回复</el-button>
          <el-button size="small" type="danger" @click="closeTicket(row)" v-if="row.status!=='closed'">关闭</el-button>
        </template>
      </el-table-column>
    </el-table>
    <el-pagination v-if="total>20" :total="total" :page-size="20" v-model:current-page="page" layout="prev,pager,next" style="margin-top:12px" @current-change="fetchTickets" />

    <el-dialog v-model="dialogVisible" title="工单详情" width="700px">
      <div v-if="currentTicket">
        <el-descriptions :column="2" border size="small" style="margin-bottom:16px">
          <el-descriptions-item label="ID">{{ currentTicket.id }}</el-descriptions-item>
          <el-descriptions-item label="客户端ID">{{ currentTicket.client_id }}</el-descriptions-item>
          <el-descriptions-item label="主题">{{ currentTicket.subject }}</el-descriptions-item>
          <el-descriptions-item label="分类">{{ categoryLabel(currentTicket.category) }}</el-descriptions-item>
          <el-descriptions-item label="优先级">{{ priorityLabel(currentTicket.priority) }}</el-descriptions-item>
          <el-descriptions-item label="状态">{{ statusLabel(currentTicket.status) }}</el-descriptions-item>
        </el-descriptions>
        <div style="max-height:300px;overflow-y:auto;border:1px solid #eee;padding:12px;border-radius:4px">
          <div v-for="msg in messages" :key="msg.id" style="margin-bottom:12px">
            <div :style="{textAlign:msg.sender_type==='admin'?'right':'left'}">
              <el-tag :type="msg.sender_type==='admin'?'warning':'success'" size="small" style="margin-bottom:4px">{{ msg.sender_type==='admin'?'管理员':'用户' }}</el-tag>
              <div style="background:#f5f7fa;padding:8px;border-radius:4px;display:inline-block;max-width:80%;text-align:left;white-space:pre-wrap">{{ msg.content }}</div>
              <div style="font-size:12px;color:#999;margin-top:2px">{{ msg.created_at }}</div>
            </div>
          </div>
        </div>
        <div style="margin-top:12px" v-if="currentTicket.status!=='closed'">
          <el-input v-model="replyContent" type="textarea" :rows="3" placeholder="回复内容" />
          <el-button type="primary" size="small" style="margin-top:8px" @click="submitReply" :loading="replyLoading">发送回复</el-button>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listTickets, getTicket, replyTicket, closeTicket as closeTicketApi } from '@/api/clientext'

const tickets = ref([])
const loading = ref(false)
const total = ref(0)
const page = ref(1)
const statusFilter = ref('')
const categoryFilter = ref('')
const dialogVisible = ref(false)
const currentTicket = ref(null)
const messages = ref([])
const replyContent = ref('')
const replyLoading = ref(false)

async function fetchTickets() {
  loading.value = true
  try {
    const params = { limit: 20, offset: (page.value - 1) * 20 }
    if (statusFilter.value) params.status = statusFilter.value
    if (categoryFilter.value) params.category = categoryFilter.value
    const { data } = await listTickets(params)
    tickets.value = data.items || []
    total.value = data.total || 0
  } finally { loading.value = false }
}

async function viewTicket(row) {
  const { data } = await getTicket(row.id)
  currentTicket.value = data.ticket
  messages.value = data.messages || []
  replyContent.value = ''
  dialogVisible.value = true
}

async function submitReply() {
  if (!replyContent.value.trim()) return ElMessage.warning('请输入回复内容')
  replyLoading.value = true
  try {
    await replyTicket(currentTicket.value.id, { content: replyContent.value })
    ElMessage.success('回复成功')
    replyContent.value = ''
    const { data } = await getTicket(currentTicket.value.id)
    currentTicket.value = data.ticket
    messages.value = data.messages || []
    fetchTickets()
  } finally { replyLoading.value = false }
}

async function closeTicket(row) {
  await ElMessageBox.confirm('确定关闭此工单？', '提示')
  await closeTicketApi(row.id)
  ElMessage.success('已关闭')
  fetchTickets()
}

function categoryLabel(c) { return { technical: '技术', billing: '账务', account: '账号', other: '其他' }[c] || c }
function priorityLabel(p) { return { low: '低', normal: '普通', high: '高', urgent: '紧急' }[p] || p }
function priorityType(p) { return { low: 'info', normal: '', high: 'warning', urgent: 'danger' }[p] || '' }
function statusLabel(s) { return { open: '待处理', replied: '已回复', closed: '已关闭' }[s] || s }
function statusType(s) { return { open: 'danger', replied: 'warning', closed: 'info' }[s] || '' }

onMounted(fetchTickets)
</script>
