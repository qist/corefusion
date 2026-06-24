<template>
  <div>
    <div style="display:flex;justify-content:space-between;margin-bottom:20px">
      <h2>{{ t('subend.title') }}</h2>
      <el-button type="primary" @click="showCreate">{{ t('subend.create') }}</el-button>
    </div>

    <el-card>
      <el-table :data="endpoints" v-loading="loading" stripe>
        <el-table-column prop="id" label="ID" width="60" />
        <el-table-column prop="domain" :label="t('subend.domain')" width="220" />
        <el-table-column prop="path" :label="t('subend.path')" width="120">
          <template #default="{row}">{{ row.path || '/' }}</template>
        </el-table-column>
        <el-table-column prop="description" :label="t('subend.description')" show-overflow-tooltip />
        <el-table-column :label="t('common.status')" width="80">
          <template #default="{row}"><el-tag :type="row.enabled?'success':'danger'" size="small">{{ row.enabled?t('common.enabled'):t('common.disabled') }}</el-tag></template>
        </el-table-column>
        <el-table-column :label="t('common.actions')" width="220">
          <template #default="{row}">
            <el-button size="small" type="success" @click="showQrCode(row)">{{ t('subend.qrcode') }}</el-button>
            <el-button size="small" @click="showEdit(row)">{{ t('common.edit') }}</el-button>
            <el-button size="small" type="danger" @click="handleDelete(row)">{{ t('common.delete') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Create/Edit Dialog -->
    <el-dialog v-model="dialogVisible" :title="editing?t('subend.edit'):t('subend.create')" width="480">
      <el-form :model="form" label-width="100px">
        <el-form-item :label="t('subend.domain')">
          <el-input v-model="form.domain" placeholder="sub.example.com" />
        </el-form-item>
        <el-form-item :label="t('subend.path')">
          <el-input v-model="form.path" placeholder="/ 或 /sub" />
          <div style="font-size:12px;color:#909399;margin-top:4px">{{ t('subend.pathHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('subend.description')">
          <el-input v-model="form.description" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item :label="t('common.enabled')">
          <el-switch v-model="form.enabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="saving" @click="handleSave">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- QR Code Dialog -->
    <el-dialog v-model="qrDialogVisible" :title="t('subend.qrcode')" width="400">
      <div style="text-align:center">
        <el-form-item :label="t('subend.selectSub')" label-position="top">
          <el-select v-model="selectedSubToken" style="width:100%" @change="updateQrCode">
            <el-option v-for="sub in subscriptions" :key="sub.token" :label="(sub.client_id ? 'Client #'+sub.client_id : 'Sub') + ' (' + sub.format + ')' + ' - ' + sub.token.substring(0,12)+'...'" :value="sub.token" />
          </el-select>
        </el-form-item>
        <div v-if="qrCodeUrl" style="margin:16px 0">
          <img :src="qrCodeUrl" style="max-width:250px;border:1px solid #eee;border-radius:8px" />
        </div>
        <el-input v-model="fullSubUrl" type="textarea" :rows="2" readonly style="margin-top:8px">
          <template #append>
            <el-button @click="copyUrl">{{ t('common.copy') }}</el-button>
          </template>
        </el-input>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import client from '../api/client'

const { t } = useI18n()
const endpoints = ref([])
const subscriptions = ref([])
const loading = ref(false)
const dialogVisible = ref(false)
const editing = ref(null)
const saving = ref(false)

const form = ref({ domain: '', path: '/', description: '', enabled: true })

// QR Code state
const qrDialogVisible = ref(false)
const selectedSubToken = ref('')
const currentEndpoint = ref(null)

const fullSubUrl = computed(() => {
  if (!currentEndpoint.value || !selectedSubToken.value) return ''
  const ep = currentEndpoint.value
  return 'https://' + ep.domain + (ep.path || '/') + '/' + selectedSubToken.value
})

const qrCodeUrl = computed(() => {
  if (!fullSubUrl.value) return ''
  return 'https://api.qrserver.com/v1/create-qr-code/?size=250x250&data=' + encodeURIComponent(fullSubUrl.value)
})

async function fetchData() {
  loading.value = true
  try {
    const r = await client.get('/subscription-endpoints')
    endpoints.value = Array.isArray(r) ? r : []
  } finally {
    loading.value = false
  }
}

async function fetchSubscriptions() {
  try {
    const r = await client.get('/subscriptions', { params: { limit: 100 } })
    subscriptions.value = r.items || []
  } catch { subscriptions.value = [] }
}

function showCreate() {
  editing.value = null
  form.value = { domain: '', path: '/', description: '', enabled: true }
  dialogVisible.value = true
}

function showEdit(row) {
  editing.value = row
  form.value = { domain: row.domain, path: row.path || '/', description: row.description || '', enabled: row.enabled }
  dialogVisible.value = true
}

async function showQrCode(row) {
  currentEndpoint.value = row
  if (subscriptions.value.length === 0) {
    await fetchSubscriptions()
  }
  if (subscriptions.value.length > 0 && !selectedSubToken.value) {
    selectedSubToken.value = subscriptions.value[0].token
  }
  qrDialogVisible.value = true
}

function updateQrCode() {
  // QR code URL updates automatically via computed
}

function copyUrl() {
  if (fullSubUrl.value) {
    navigator.clipboard.writeText(fullSubUrl.value)
    ElMessage.success(t('common.copied'))
  }
}

async function handleSave() {
  if (!form.value.domain) {
    ElMessage.error(t('subend.domainRequired'))
    return
  }
  saving.value = true
  try {
    if (editing.value) {
      await client.put(`/subscription-endpoints/${editing.value.id}`, form.value)
    } else {
      await client.post('/subscription-endpoints', form.value)
    }
    ElMessage.success(t('common.success'))
    dialogVisible.value = false
    fetchData()
  } finally {
    saving.value = false
  }
}

async function handleDelete(row) {
  await ElMessageBox.confirm(t('subend.deleteConfirm', { domain: row.domain }), t('common.confirm'))
  await client.delete(`/subscription-endpoints/${row.id}`)
  ElMessage.success(t('common.success'))
  fetchData()
}

onMounted(() => { fetchData(); fetchSubscriptions() })
</script>
