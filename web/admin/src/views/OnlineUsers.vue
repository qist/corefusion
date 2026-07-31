<template>
  <div class="online-users-page">
    <div class="page-header">
      <div>
        <h2 class="page-title">{{ t('menu.onlineUsers') }}</h2>
        <p class="page-subtitle">所有节点实时在线订阅人总览</p>
      </div>
      <div class="header-actions">
        <el-select v-model="filterNode" :placeholder="t('onlineUsers.filterNode')" clearable style="width:180px" @change="applyFilter">
          <el-option v-for="n in nodeOptions" :key="n.id" :label="n.name" :value="n.id" />
        </el-select>
        <el-input v-model="keyword" :placeholder="t('onlineUsers.search')" clearable style="width:240px" @input="onKeywordInput" @clear="applyFilter">
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>
        <el-button :loading="loading" @click="loadData">
          <el-icon><Refresh /></el-icon> {{ t('common.refresh') }}
        </el-button>
      </div>
    </div>

    <!-- 汇总卡 -->
    <el-row :gutter="16" class="summary-row">
      <el-col :span="8">
        <el-card shadow="hover" class="summary-card">
          <div class="sum-label">{{ t('onlineUsers.totalSubscriptions') }}</div>
          <div class="sum-value">{{ data?.total_subscriptions || 0 }}</div>
        </el-card>
      </el-col>
      <el-col :span="8">
        <el-card shadow="hover" class="summary-card">
          <div class="sum-label">{{ t('onlineUsers.totalDevices') }}</div>
          <div class="sum-value">{{ data?.total_devices || 0 }}</div>
        </el-card>
      </el-col>
      <el-col :span="8">
        <el-card shadow="hover" class="summary-card">
          <div class="sum-label">{{ t('onlineUsers.onlineNodes') }}</div>
          <div class="sum-value">{{ onlineNodeCount }}</div>
        </el-card>
      </el-col>
    </el-row>

    <el-alert v-if="error" type="error" :closable="false" style="margin-bottom:12px">{{ error }}</el-alert>

    <!-- 主表 -->
    <el-card shadow="never">
      <el-table :data="users" v-loading="loading" border stripe size="small" row-key="rowKey">
        <el-table-column type="expand">
          <template #default="{row}">
            <el-table :data="row.sessions || []" size="mini" border>
              <el-table-column prop="ip" label="IP" min-width="150" />
              <el-table-column :label="t('onlineUsers.lastActive')" min-width="110">
                <template #default="{row:s}">{{ formatAgo(s.last_seen) }}</template>
              </el-table-column>
              <el-table-column :label="t('onlineUsers.duration')" min-width="110">
                <template #default="{row:s}">{{ formatDuration((s.last_seen || 0) - (s.connected_since || 0)) }}</template>
              </el-table-column>
            </el-table>
          </template>
        </el-table-column>
        <el-table-column :label="t('onlineUsers.node')" min-width="130">
          <template #default="{row}">
            <el-link type="primary" @click="goNode(row.server_id)">{{ row.node_name }}</el-link>
          </template>
        </el-table-column>
        <el-table-column :label="t('onlineUsers.client')" min-width="200">
          <template #default="{row}">
            <div>{{ row.client_name || row.email }}</div>
            <div style="font-size:12px;color:#909399">{{ row.email }}</div>
          </template>
        </el-table-column>
        <el-table-column :label="t('onlineUsers.plan')" min-width="120">
          <template #default="{row}">
            <span v-if="row.plan_name">{{ row.plan_name }}</span>
            <span v-else style="color:#c0c4cc">-</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('onlineUsers.devices')" width="90" align="center">
          <template #default="{row}">{{ row.session_count }}</template>
        </el-table-column>
        <el-table-column :label="t('onlineUsers.upRate')" width="110" sortable :sort-method="(a,b)=>a.uplink_rate-b.uplink_rate">
          <template #default="{row}">{{ formatRate(row.uplink_rate) }}</template>
        </el-table-column>
        <el-table-column :label="t('onlineUsers.downRate')" width="110" sortable :sort-method="(a,b)=>a.downlink_rate-b.downlink_rate">
          <template #default="{row}">{{ formatRate(row.downlink_rate) }}</template>
        </el-table-column>
        <el-table-column :label="t('onlineUsers.totalUpDown')" width="130">
          <template #default="{row}">
            <div style="font-size:12px">↑ {{ formatBytes(row.uplink_bytes) }}</div>
            <div style="font-size:12px">↓ {{ formatBytes(row.downlink_bytes) }}</div>
          </template>
        </el-table-column>
        <el-table-column :label="t('onlineUsers.speedLimit')" width="130" align="center">
          <template #default="{row}">
            <span v-if="(row.speed_limit_down_mbps || 0) > 0 || (row.speed_limit_up_mbps || 0) > 0">
              ↓{{ row.speed_limit_down_mbps || 0 }} / ↑{{ row.speed_limit_up_mbps || 0 }}
              <div style="font-size:11px;color:#909399">Mbps</div>
            </span>
            <span v-else style="color:#c0c4cc;font-size:12px">{{ t('onlineUsers.unlimited') }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('onlineUsers.lastActive')" width="100" sortable :sort-method="(a,b)=>a.last_seen-b.last_seen">
          <template #default="{row}">{{ formatAgo(row.last_seen) }}</template>
        </el-table-column>
        <el-table-column :label="t('onlineUsers.duration')" width="100">
          <template #default="{row}">{{ formatDuration((row.last_seen || 0) - (row.first_seen || 0)) }}</template>
        </el-table-column>
        <template #empty>
          <el-empty :description="t('onlineUsers.noOnline')" />
        </template>
      </el-table>
      <div class="pager-wrap">
        <el-pagination
          v-model:current-page="page"
          v-model:page-size="pageSize"
          :total="total"
          :page-sizes="[20, 50, 100]"
          layout="total, sizes, prev, pager, next, jumper"
          background
          @size-change="onSizeChange"
          @current-change="onPageChange"
        />
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Search, Refresh } from '@element-plus/icons-vue'
import client from '../api/client'

const { t } = useI18n()
const router = useRouter()

const data = ref(null)
const users = ref([])
const loading = ref(false)
const error = ref('')
const filterNode = ref(null)
const keyword = ref('')
const nodeOptions = ref([])
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)

let timer = null
let searchDebounce = null

const onlineNodeCount = computed(() => {
  const ids = new Set((users.value || []).map(u => u.server_id))
  return ids.size
})

// 筛选/搜索变化：回到第 1 页并从服务端重新拉取
function applyFilter() {
  page.value = 1
  loadData()
}

// 搜索输入防抖，避免每次击键都请求
function onKeywordInput() {
  clearTimeout(searchDebounce)
  searchDebounce = setTimeout(applyFilter, 400)
}

function onPageChange(p) { page.value = p; loadData() }
function onSizeChange(s) { pageSize.value = s; page.value = 1; loadData() }

async function loadData() {
  loading.value = true
  error.value = ''
  try {
    const params = {
      limit: pageSize.value,
      offset: (page.value - 1) * pageSize.value,
    }
    if (filterNode.value) params.server_id = filterNode.value
    const kw = (keyword.value || '').trim()
    if (kw) params.keyword = kw
    const res = await client.get('/node-servers/online-users', { params })
    const list = (res && res.items) || []
    list.forEach((u, i) => { u.rowKey = `${u.server_id}-${u.subscription_id}-${i}` })
    users.value = list
    total.value = (res && res.total) || 0
    // 汇总卡数据（不受分页/筛选影响）
    data.value = { total_subscriptions: res?.total_subscriptions || 0, total_devices: res?.total_devices || 0 }
    // 节点下拉选项：仅在未做节点筛选时根据当前结果补充（避免覆盖）
    if (!filterNode.value) {
      const map = new Map(nodeOptions.value.map(o => [o.id, o.name]))
      list.forEach(u => { if (!map.has(u.server_id)) map.set(u.server_id, u.node_name) })
      nodeOptions.value = [...map.entries()].map(([id, name]) => ({ id, name }))
    }
  } catch (e) {
    error.value = e?.response?.data?.message || t('onlineUsers.loadFailed')
  } finally {
    loading.value = false
  }
}

function goNode(id) { router.push(`/node-servers/${id}/detail`) }

function formatRate(bps) {
  if (!bps || bps <= 0) return '0 B/s'
  if (bps < 1024) return bps + ' B/s'
  if (bps < 1024 * 1024) return (bps / 1024).toFixed(1) + ' KB/s'
  return (bps / 1024 / 1024).toFixed(2) + ' MB/s'
}
function formatBytes(bytes) {
  if (!bytes || bytes <= 0) return '0 B'
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  if (bytes < 1024 * 1024 * 1024) return (bytes / 1024 / 1024).toFixed(1) + ' MB'
  return (bytes / 1024 / 1024 / 1024).toFixed(2) + ' GB'
}
function formatDuration(sec) {
  if (!sec || sec <= 0) return '0s'
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  const s = Math.floor(sec % 60)
  const pad = (n) => String(n).padStart(2, '0')
  return (h > 0 ? h + ':' : '') + pad(m) + ':' + pad(s)
}
function formatAgo(ts) {
  if (!ts) return '-'
  const diff = Math.max(0, Math.floor(Date.now() / 1000) - ts)
  if (diff < 60) return diff + t('onlineUsers.secondsAgo')
  if (diff < 3600) return Math.floor(diff / 60) + t('onlineUsers.minutesAgo')
  return Math.floor(diff / 3600) + t('onlineUsers.hoursAgo')
}

onMounted(() => {
  loadData()
  timer = setInterval(loadData, 15000)
})
onUnmounted(() => { if (timer) clearInterval(timer); clearTimeout(searchDebounce) })
</script>

<style scoped>
.online-users-page { padding: 4px; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; flex-wrap: wrap; gap: 12px; }
.page-title { margin: 0; font-size: 20px; }
.page-subtitle { margin: 4px 0 0; font-size: 13px; color: #909399; }
.header-actions { display: flex; gap: 10px; align-items: center; flex-wrap: wrap; }
.summary-row { margin-bottom: 16px; }
.summary-card { text-align: center; }
.sum-label { font-size: 13px; color: #909399; margin-bottom: 8px; }
.sum-value { font-size: 28px; font-weight: 600; color: #303133; }
.pager-wrap { display: flex; justify-content: flex-end; margin-top: 14px; }
</style>
