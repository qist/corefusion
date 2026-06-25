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
        <el-table-column prop="path" :label="t('subend.path')" width="150">
          <template #default="{row}">{{ row.path || '/' }}</template>
        </el-table-column>
        <el-table-column :label="t('subend.subscribeUrl')" min-width="280">
          <template #default="{row}">
            <el-text type="info" size="small" style="font-family:monospace">
              https://{{ row.domain }}{{ row.path || '/' }}/{token}
            </el-text>
          </template>
        </el-table-column>
        <el-table-column prop="description" :label="t('subend.description')" show-overflow-tooltip />
        <el-table-column :label="t('common.status')" width="80">
          <template #default="{row}"><el-tag :type="row.enabled?'success':'danger'" size="small">{{ row.enabled?t('common.enabled'):t('common.disabled') }}</el-tag></template>
        </el-table-column>
        <el-table-column :label="t('common.actions')" width="160">
          <template #default="{row}">
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
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import client from '../api/client'

const { t } = useI18n()
const endpoints = ref([])
const loading = ref(false)
const dialogVisible = ref(false)
const editing = ref(null)
const saving = ref(false)

const form = ref({ domain: '', path: '/', description: '', enabled: true })

async function fetchData() {
  loading.value = true
  try {
    const r = await client.get('/subscription-endpoints')
    endpoints.value = Array.isArray(r) ? r : []
  } finally {
    loading.value = false
  }
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

onMounted(fetchData)
</script>
