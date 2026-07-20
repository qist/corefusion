<template>
  <div>
    <div style="display:flex;justify-content:space-between;margin-bottom:20px">
      <h2>{{ t('subfmt.title') }}</h2>
      <el-button type="primary" @click="showCreate">{{ t('subfmt.create') }}</el-button>
    </div>

    <el-card>
      <el-table :data="formats" v-loading="loading" stripe>
        <el-table-column prop="id" label="ID" width="60" />
        <el-table-column prop="name" :label="t('subfmt.name')" width="180" />
        <el-table-column prop="client_type" :label="t('subfmt.clientType')" width="120">
          <template #default="{row}"><el-tag size="small">{{ row.client_type }}</el-tag></template>
        </el-table-column>
        <el-table-column prop="format" :label="t('subfmt.format')" width="140" />
        <el-table-column prop="content_type" :label="t('subfmt.contentType')" width="200" />
        <el-table-column prop="description" :label="t('subfmt.description')" show-overflow-tooltip />
        <el-table-column :label="t('common.status')" width="80">
          <template #default="{row}"><el-tag :type="row.enabled?'success':'danger'" size="small">{{ row.enabled?t('common.enabled'):t('common.disabled') }}</el-tag></template>
        </el-table-column>
        <el-table-column :label="t('common.actions')" width="240">
          <template #default="{row}">
            <el-button size="small" @click="showRules(row)">{{ t('subfmt.rules') }}</el-button>
            <el-button size="small" @click="showEdit(row)">{{ t('common.edit') }}</el-button>
            <el-button size="small" type="danger" @click="handleDelete(row)">{{ t('common.delete') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Create/Edit Format Dialog -->
    <el-dialog v-model="dialogVisible" :title="editing?t('subfmt.edit'):t('subfmt.create')" width="500">
      <el-form :model="form" label-width="100px">
        <el-form-item :label="t('subfmt.name')"><el-input v-model="form.name" /></el-form-item>
        <el-form-item :label="t('subfmt.clientType')">
          <el-select v-model="form.client_type" style="width:100%">
            <el-option label="Clash" value="clash" />
            <el-option label="Shadowrocket" value="shadowrocket" />
            <el-option label="Sing-Box" value="singbox" />
            <el-option label="V2Ray" value="v2ray" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('subfmt.format')">
          <el-select v-model="form.format" style="width:100%">
            <el-option label="Mihomo YAML" value="mihomo_yaml" />
            <el-option label="Sing-Box JSON" value="singbox_json" />
            <el-option label="Xray JSON" value="xray_json" />
            <el-option label="Base64 URI" value="base64_uri" />
            <el-option label="Raw URI" value="raw_uri" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('subfmt.contentType')"><el-input v-model="form.content_type" placeholder="text/yaml; charset=utf-8" /></el-form-item>
        <el-form-item :label="t('subfmt.extension')"><el-input v-model="form.extension" placeholder=".yaml" /></el-form-item>
        <el-form-item :label="t('subfmt.description')"><el-input v-model="form.description" type="textarea" :rows="2" /></el-form-item>
        <el-form-item :label="t('subfmt.planIds')">
          <el-select v-model="form.plan_ids" style="width:100%" multiple clearable placeholder="Universal">
            <el-option v-for="p in allPlans" :key="p.id" :label="p.name" :value="p.id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('subfmt.sortOrder')"><el-input-number v-model="form.sort_order" :min="0" /></el-form-item>
        <el-form-item :label="t('common.enabled')"><el-switch v-model="form.enabled" /></el-form-item>
        <el-form-item :label="t('subfmt.isDefault')"><el-switch v-model="form.is_default" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="saving" @click="handleSave">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- Rules Dialog -->
    <el-dialog v-model="rulesDialogVisible" :title="t('subfmt.rulesOf', {name: currentFormat?.name})" width="700">
      <div style="margin-bottom:12px">
        <el-button size="small" type="primary" @click="showAddRule">{{ t('subfmt.addRule') }}</el-button>
      </div>
      <el-table :data="rules" stripe size="small">
        <el-table-column prop="priority" :label="t('subfmt.priority')" width="70" />
        <el-table-column prop="rule_type" :label="t('subfmt.ruleType')" width="130">
          <template #default="{row}"><el-tag size="small" type="info">{{ row.rule_type }}</el-tag></template>
        </el-table-column>
        <el-table-column prop="match_value" :label="t('subfmt.matchValue')" />
        <el-table-column prop="outbound_tag" :label="t('subfmt.outboundTag')" width="120">
          <template #default="{row}"><el-tag size="small" :type="outboundType(row.outbound_tag)">{{ row.outbound_tag }}</el-tag></template>
        </el-table-column>
        <el-table-column :label="t('common.status')" width="70">
          <template #default="{row}"><el-tag :type="row.enabled?'success':'danger'" size="small">{{ row.enabled?'✓':'✗' }}</el-tag></template>
        </el-table-column>
        <el-table-column :label="t('common.actions')" width="130">
          <template #default="{row}">
            <el-button size="small" @click="showEditRule(row)">{{ t('common.edit') }}</el-button>
            <el-button size="small" type="danger" @click="handleDeleteRule(row)">{{ t('common.delete') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>

    <!-- Rule Create/Edit Dialog -->
    <el-dialog v-model="ruleDialogVisible" :title="editingRule?t('subfmt.editRule'):t('subfmt.addRule')" width="460">
      <el-form :model="ruleForm" label-width="100px">
        <el-form-item :label="t('subfmt.ruleType')">
          <el-select v-model="ruleForm.rule_type" style="width:100%">
            <el-option label="domain" value="domain" />
            <el-option label="domain_suffix" value="domain_suffix" />
            <el-option label="domain_keyword" value="domain_keyword" />
            <el-option label="ip_cidr" value="ip_cidr" />
            <el-option label="geoip" value="geoip" />
            <el-option label="geosite" value="geosite" />
            <el-option label="match" value="match" />
            <el-option label="default" value="default" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('subfmt.matchValue')"><el-input v-model="ruleForm.match_value" placeholder="cn, private, google.com" /></el-form-item>
        <el-form-item :label="t('subfmt.outboundTag')">
          <el-select v-model="ruleForm.outbound_tag" allow-create filterable style="width:100%">
            <el-option label="PROXY" value="PROXY" />
            <el-option label="DIRECT" value="DIRECT" />
            <el-option label="BLOCK" value="BLOCK" />
            <el-option label="proxy" value="proxy" />
            <el-option label="direct" value="direct" />
            <el-option label="block" value="block" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('subfmt.priority')"><el-input-number v-model="ruleForm.priority" :min="0" /></el-form-item>
        <el-form-item :label="t('common.enabled')"><el-switch v-model="ruleForm.enabled" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="ruleDialogVisible=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="saving" @click="handleSaveRule">{{ t('common.save') }}</el-button>
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
const formats = ref([])
const loading = ref(false)
const dialogVisible = ref(false)
const editing = ref(null)
const saving = ref(false)

const form = ref({
  name: '', client_type: 'clash', format: 'mihomo_yaml',
  content_type: '', extension: '', description: '',
  plan_ids: [], is_default: false, enabled: true, sort_order: 0
})
const allPlans = ref([])

// Rules state
const rulesDialogVisible = ref(false)
const currentFormat = ref(null)
const rules = ref([])
const ruleDialogVisible = ref(false)
const editingRule = ref(null)
const ruleForm = ref({
  rule_type: 'domain_suffix', match_value: '', outbound_tag: 'PROXY', priority: 0, enabled: true
})

const outboundType = (tag) => {
  const t = tag?.toUpperCase()
  if (t === 'DIRECT' || t === 'DIRECT') return 'success'
  if (t === 'BLOCK') return 'danger'
  return ''
}

async function fetchData() {
  loading.value = true
  try {
    const r = await client.get('/subscription-formats')
    formats.value = Array.isArray(r) ? r : []
  } finally {
    loading.value = false
  }
}

function showCreate() {
  editing.value = null
  form.value = { name: '', client_type: 'clash', format: 'mihomo_yaml', content_type: '', extension: '', description: '', plan_ids: [], is_default: false, enabled: true, sort_order: 0 }
  dialogVisible.value = true
}

function showEdit(row) {
  editing.value = row
  form.value = { name: row.name, client_type: row.client_type, format: row.format, content_type: row.content_type || '', extension: row.extension || '', description: row.description || '', plan_ids: row.plan_ids || [], is_default: row.is_default, enabled: row.enabled, sort_order: row.sort_order || 0 }
  dialogVisible.value = true
}

async function handleSave() {
  saving.value = true
  try {
    if (editing.value) {
      await client.put(`/subscription-formats/${editing.value.id}`, form.value)
    } else {
      await client.post('/subscription-formats', form.value)
    }
    ElMessage.success(t('common.success'))
    dialogVisible.value = false
    fetchData()
  } finally {
    saving.value = false
  }
}

async function handleDelete(row) {
  await ElMessageBox.confirm(t('subfmt.deleteConfirm', { name: row.name }), t('common.confirm'))
  await client.delete(`/subscription-formats/${row.id}`)
  ElMessage.success(t('common.success'))
  fetchData()
}

// --- Rules ---
async function showRules(row) {
  currentFormat.value = row
  rulesDialogVisible.value = true
  await fetchRules(row.id)
}

async function fetchRules(formatId) {
  try {
    const r = await client.get(`/subscription-formats/${formatId}/rules`)
    rules.value = Array.isArray(r) ? r : []
  } catch { rules.value = [] }
}

function showAddRule() {
  editingRule.value = null
  ruleForm.value = { rule_type: 'domain_suffix', match_value: '', outbound_tag: 'PROXY', priority: 0, enabled: true }
  ruleDialogVisible.value = true
}

function showEditRule(row) {
  editingRule.value = row
  ruleForm.value = { rule_type: row.rule_type, match_value: row.match_value, outbound_tag: row.outbound_tag, priority: row.priority || 0, enabled: row.enabled }
  ruleDialogVisible.value = true
}

async function handleSaveRule() {
  saving.value = true
  try {
    if (editingRule.value) {
      await client.put(`/subscription-formats/${currentFormat.value.id}/rules/${editingRule.value.id}`, ruleForm.value)
    } else {
      await client.post(`/subscription-formats/${currentFormat.value.id}/rules`, ruleForm.value)
    }
    ElMessage.success(t('common.success'))
    ruleDialogVisible.value = false
    await fetchRules(currentFormat.value.id)
  } finally {
    saving.value = false
  }
}

async function handleDeleteRule(row) {
  await ElMessageBox.confirm(t('subfmt.deleteRuleConfirm'), t('common.confirm'))
  await client.delete(`/subscription-formats/${currentFormat.value.id}/rules/${row.id}`)
  ElMessage.success(t('common.success'))
  await fetchRules(currentFormat.value.id)
}

async function fetchPlans() { try { allPlans.value = (await client.get('/plan-templates')) || [] } catch { allPlans.value = [] } }

onMounted(() => { fetchData(); fetchPlans() })
</script>
