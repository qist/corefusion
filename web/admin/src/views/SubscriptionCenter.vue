<template>
  <div>
    <h2 style="margin-bottom:20px">{{ t('menu.subscriptions') }}</h2>
    <el-tabs v-model="activeTab">
      <!-- Subscriptions -->
      <el-tab-pane :label="t('subscription.authTitle')" name="subscriptions">
        <el-card>
          <el-table :data="subscriptions" v-loading="subLoading" stripe>
            <el-table-column prop="id" :label="t('common.id')" width="60" />
            <el-table-column :label="t('subscription.clientEmail')" width="180">
              <template #default="{row}">{{ clientEmail(row.client_id) }}</template>
            </el-table-column>
            <el-table-column :label="t('subscription.token')" width="160"><template #default="{row}"><span style="font-family:monospace;font-size:12px">{{ row.token_masked }}</span></template></el-table-column>
            <el-table-column :label="t('common.status')" width="80"><template #default="{row}"><el-tag :type="{active:'success',revoked:'danger',expired:'warning'}[row.status]" size="small">{{ row.status }}</el-tag></template></el-table-column>
            <el-table-column :label="t('subscription.quotaState')" width="100"><template #default="{row}"><el-tag :type="quotaType(row.quota_state)" size="small">{{ row.quota_state }}</el-tag></template></el-table-column>
            <el-table-column :label="`${t('subscription.usedBytes')}/${t('subscription.quotaBytes')}`" width="140"><template #default="{row}">{{ fmtBytes(row.used_bytes_cache) }} / {{ row.quota_bytes ? fmtBytes(row.quota_bytes) : t('common.unlimited') }}</template></el-table-column>
            <el-table-column :label="t('common.actions')" width="240">
              <template #default="{row}">
                <el-button size="small" @click="handleRotate(row)" :disabled="row.status==='revoked'">{{ t('subscription.rotate') }}</el-button>
                <el-button size="small" type="danger" @click="handleRevoke(row)" :disabled="row.status==='revoked'">{{ t('subscription.revoke') }}</el-button>
                <el-button v-if="row.status==='revoked'" size="small" type="danger" @click="handleDelete(row)">{{ t('common.delete') }}</el-button>
              </template>
            </el-table-column>
          </el-table>
          <el-pagination v-model:current-page="subPage" :page-size="20" :total="subTotal" layout="total,prev,pager,next" style="margin-top:16px;justify-content:flex-end" @current-change="fetchSubscriptions" />
        </el-card>
      </el-tab-pane>

      <!-- Subscription Formats -->
      <el-tab-pane :label="t('subfmt.title')" name="formats">
        <div style="display:flex;justify-content:space-between;margin-bottom:16px">
          <span></span>
          <el-button type="primary" size="small" @click="showCreateFormat">{{ t('subfmt.create') }}</el-button>
        </div>
        <el-card>
          <el-table :data="formats" v-loading="fmtLoading" stripe size="small">
            <el-table-column prop="id" label="ID" width="50" />
            <el-table-column prop="name" :label="t('subfmt.name')" width="150" />
            <el-table-column prop="client_type" :label="t('subfmt.clientType')" width="100"><template #default="{row}"><el-tag size="small">{{ row.client_type }}</el-tag></template></el-table-column>
            <el-table-column prop="format" :label="t('subfmt.format')" width="120" />
            <el-table-column prop="description" :label="t('subfmt.description')" show-overflow-tooltip />
            <el-table-column :label="t('common.status')" width="70"><template #default="{row}"><el-tag :type="row.enabled?'success':'danger'" size="small">{{ row.enabled?'✓':'✗' }}</el-tag></template></el-table-column>
            <el-table-column :label="t('common.actions')" width="200">
              <template #default="{row}">
                <el-button size="small" @click="showFormatRules(row)">{{ t('subfmt.rules') }}</el-button>
                <el-button size="small" @click="showEditFormat(row)">{{ t('common.edit') }}</el-button>
                <el-button size="small" type="danger" @click="deleteFormat(row)">{{ t('common.delete') }}</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>

      <!-- Subscription Endpoints -->
      <el-tab-pane :label="t('subend.title')" name="endpoints">
        <div style="display:flex;justify-content:space-between;margin-bottom:16px">
          <span></span>
          <el-button type="primary" size="small" @click="showCreateEndpoint">{{ t('subend.create') }}</el-button>
        </div>
        <el-card>
          <el-table :data="endpoints" v-loading="epLoading" stripe size="small">
            <el-table-column prop="id" label="ID" width="50" />
            <el-table-column prop="domain" :label="t('subend.domain')" width="200" />
            <el-table-column prop="path" :label="t('subend.path')" width="120"><template #default="{row}">{{ row.path || '/' }}</template></el-table-column>
            <el-table-column prop="description" :label="t('subend.description')" show-overflow-tooltip />
            <el-table-column :label="t('common.status')" width="70"><template #default="{row}"><el-tag :type="row.enabled?'success':'danger'" size="small">{{ row.enabled?'✓':'✗' }}</el-tag></template></el-table-column>
            <el-table-column :label="t('common.actions')" width="140">
              <template #default="{row}">
                <el-button size="small" @click="showEditEndpoint(row)">{{ t('common.edit') }}</el-button>
                <el-button size="small" type="danger" @click="deleteEndpoint(row)">{{ t('common.delete') }}</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>
    </el-tabs>

    <!-- Format Dialog -->
    <el-dialog v-model="fmtDialog" :title="editingFormat?t('subfmt.edit'):t('subfmt.create')" width="450">
      <el-form :model="fmtForm" label-width="100px">
        <el-form-item :label="t('subfmt.name')"><el-input v-model="fmtForm.name" /></el-form-item>
        <el-form-item :label="t('subfmt.clientType')"><el-select v-model="fmtForm.client_type" style="width:100%"><el-option label="Clash" value="clash" /><el-option label="Shadowrocket" value="shadowrocket" /><el-option label="Sing-Box" value="singbox" /><el-option label="V2Ray" value="v2ray" /></el-select></el-form-item>
        <el-form-item :label="t('subfmt.format')"><el-select v-model="fmtForm.format" style="width:100%"><el-option label="Mihomo YAML" value="mihomo_yaml" /><el-option label="Sing-Box JSON" value="singbox_json" /><el-option label="Xray JSON" value="xray_json" /><el-option label="Base64 URI" value="base64_uri" /><el-option label="Raw URI" value="raw_uri" /></el-select></el-form-item>
        <el-form-item :label="t('subfmt.contentType')"><el-input v-model="fmtForm.content_type" placeholder="text/yaml; charset=utf-8" /></el-form-item>
        <el-form-item :label="t('subfmt.extension')"><el-input v-model="fmtForm.extension" placeholder=".yaml" /></el-form-item>
        <el-form-item :label="t('subfmt.description')"><el-input v-model="fmtForm.description" type="textarea" :rows="2" /></el-form-item>
        <el-form-item :label="t('common.enabled')"><el-switch v-model="fmtForm.enabled" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="fmtDialog=false">{{ t('common.cancel') }}</el-button><el-button type="primary" @click="saveFormat">{{ t('common.save') }}</el-button></template>
    </el-dialog>

    <!-- Format Rules Dialog -->
    <el-dialog v-model="rulesDialog" :title="currentFormat?.name + ' - ' + t('subfmt.rules')" width="600">
      <div style="margin-bottom:12px">
        <el-select v-model="newRuleType" size="small" style="width:120px">
          <el-option label="domain" value="domain" /><el-option label="domain_suffix" value="domain_suffix" /><el-option label="ip_cidr" value="ip_cidr" /><el-option label="geoip" value="geoip" /><el-option label="geosite" value="geosite" /><el-option label="match" value="match" />
        </el-select>
        <el-input v-model="newRuleValue" size="small" placeholder="cn, private" style="width:150px;margin:0 8px" />
        <el-select v-model="newRuleOutbound" size="small" allow-create filterable style="width:120px">
          <el-option label="PROXY" value="PROXY" /><el-option label="DIRECT" value="DIRECT" /><el-option label="BLOCK" value="BLOCK" />
        </el-select>
        <el-button size="small" type="primary" style="margin-left:8px" @click="addRule">{{ t('subfmt.addRule') }}</el-button>
      </div>
      <el-table :data="formatRules" stripe size="small">
        <el-table-column prop="priority" label="#" width="50" />
        <el-table-column prop="rule_type" :label="t('subfmt.ruleType')" width="120"><template #default="{row}"><el-tag size="small" type="info">{{ row.rule_type }}</el-tag></template></el-table-column>
        <el-table-column prop="match_value" :label="t('subfmt.matchValue')" />
        <el-table-column prop="outbound_tag" :label="t('subfmt.outboundTag')" width="100"><template #default="{row}"><el-tag size="small">{{ row.outbound_tag }}</el-tag></template></el-table-column>
        <el-table-column :label="t('common.actions')" width="70"><template #default="{row}"><el-button size="small" type="danger" @click="deleteRule(row)">{{ t('common.delete') }}</el-button></template></el-table-column>
      </el-table>
    </el-dialog>

    <!-- Endpoint Dialog -->
    <el-dialog v-model="epDialog" :title="editingEndpoint?t('subend.edit'):t('subend.create')" width="450">
      <el-form :model="epForm" label-width="100px">
        <el-form-item :label="t('subend.domain')"><el-input v-model="epForm.domain" placeholder="sub.example.com" /></el-form-item>
        <el-form-item :label="t('subend.path')"><el-input v-model="epForm.path" placeholder="/ 或 /sub" /><div style="font-size:12px;color:#909399;margin-top:4px">{{ t('subend.pathHint') }}</div></el-form-item>
        <el-form-item :label="t('subend.description')"><el-input v-model="epForm.description" type="textarea" :rows="2" /></el-form-item>
        <el-form-item :label="t('common.enabled')"><el-switch v-model="epForm.enabled" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="epDialog=false">{{ t('common.cancel') }}</el-button><el-button type="primary" @click="saveEndpoint">{{ t('common.save') }}</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import client from '../api/client'

const { t } = useI18n()
const activeTab = ref('subscriptions')

// Subscriptions
const subscriptions = ref([])
const clientsList = ref([])
const subLoading = ref(false)
const subTotal = ref(0)
const subPage = ref(1)

// Formats
const formats = ref([])
const fmtLoading = ref(false)
const fmtDialog = ref(false)
const editingFormat = ref(null)
const fmtForm = ref({ name: '', client_type: 'clash', format: 'mihomo_yaml', content_type: '', extension: '', description: '', enabled: true })
const rulesDialog = ref(false)
const currentFormat = ref(null)
const formatRules = ref([])
const newRuleType = ref('domain_suffix')
const newRuleValue = ref('')
const newRuleOutbound = ref('DIRECT')

// Endpoints
const endpoints = ref([])
const epLoading = ref(false)
const epDialog = ref(false)
const editingEndpoint = ref(null)
const epForm = ref({ domain: '', path: '/', description: '', enabled: true })

// Helpers
const quotaType = (s) => ({ normal: 'success', near_limit: 'warning', over_limit_soft: 'danger', over_limit_hard: 'danger', suspended: 'danger', expired_days: 'warning' }[s] || 'info')
function fmtBytes(b) { if (!b) return '0 B'; const u = ['B', 'KB', 'MB', 'GB', 'TB']; let i = 0; while (b >= 1024 && i < u.length - 1) { b /= 1024; i++ } return b.toFixed(1) + ' ' + u[i] }
const clientEmail = (id) => clientsList.value.find(c => c.id === id)?.email || `#${id}`

// Fetch
async function fetchSubscriptions() {
  subLoading.value = true
  try {
    const [subRes, clRes] = await Promise.all([
      client.get('/subscriptions', { params: { limit: 20, offset: (subPage.value - 1) * 20 } }),
      client.get('/clients', { params: { limit: 100 } })
    ])
    subscriptions.value = subRes.items || []
    subTotal.value = subRes.total || 0
    clientsList.value = clRes.items || []
  } finally { subLoading.value = false }
}
async function fetchFormats() { fmtLoading.value = true; try { formats.value = (await client.get('/subscription-formats')) || [] } catch {} finally { fmtLoading.value = false } }
async function fetchEndpoints() { epLoading.value = true; try { endpoints.value = (await client.get('/subscription-endpoints')) || [] } catch {} finally { epLoading.value = false } }

// Subscription actions
async function handleRotate(row) { await ElMessageBox.confirm(t('subscription.rotateConfirm'), t('common.confirm')); const r = await client.post(`/subscriptions/${row.id}/rotate`, { reason: 'manual' }); ElMessage.success(`${t('subscription.newToken')}: ${r.token}`); fetchSubscriptions() }
async function handleRevoke(row) { await ElMessageBox.confirm(t('subscription.revokeConfirm'), t('common.confirm')); await client.post(`/subscriptions/${row.id}/revoke`); ElMessage.success(t('common.success')); fetchSubscriptions() }
async function handleDelete(row) { await ElMessageBox.confirm(t('subscription.deleteConfirm'), t('common.confirm')); await client.delete(`/subscriptions/${row.id}`); ElMessage.success(t('common.success')); fetchSubscriptions() }

// Format CRUD
function showCreateFormat() { editingFormat.value = null; fmtForm.value = { name: '', client_type: 'clash', format: 'mihomo_yaml', content_type: '', extension: '', description: '', enabled: true }; fmtDialog.value = true }
function showEditFormat(row) { editingFormat.value = row; fmtForm.value = { name: row.name, client_type: row.client_type, format: row.format, content_type: row.content_type || '', extension: row.extension || '', description: row.description || '', enabled: row.enabled }; fmtDialog.value = true }
async function saveFormat() { try { if (editingFormat.value) { await client.put(`/subscription-formats/${editingFormat.value.id}`, fmtForm.value) } else { await client.post('/subscription-formats', fmtForm.value) } ElMessage.success(t('common.success')); fmtDialog.value = false; fetchFormats() } catch {} }
async function deleteFormat(row) { await ElMessageBox.confirm(t('subfmt.deleteConfirm', { name: row.name }), t('common.confirm')); await client.delete(`/subscription-formats/${row.id}`); ElMessage.success(t('common.success')); fetchFormats() }

// Format Rules
async function showFormatRules(row) { currentFormat.value = row; rulesDialog.value = true; try { const r = await client.get(`/subscription-formats/${row.id}`); formatRules.value = r.rules || [] } catch { formatRules.value = [] } }
async function addRule() { if (!newRuleValue.value) return; await client.post(`/subscription-formats/${currentFormat.value.id}/rules`, { rule_type: newRuleType.value, match_value: newRuleValue.value, outbound_tag: newRuleOutbound.value }); newRuleValue.value = ''; showFormatRules(currentFormat.value) }
async function deleteRule(row) { await client.delete(`/subscription-formats/${currentFormat.value.id}/rules/${row.id}`); showFormatRules(currentFormat.value) }

// Endpoint CRUD
function showCreateEndpoint() { editingEndpoint.value = null; epForm.value = { domain: '', path: '/', description: '', enabled: true }; epDialog.value = true }
function showEditEndpoint(row) { editingEndpoint.value = row; epForm.value = { domain: row.domain, path: row.path || '/', description: row.description || '', enabled: row.enabled }; epDialog.value = true }
async function saveEndpoint() { if (!epForm.value.domain) { ElMessage.error(t('subend.domainRequired')); return }; try { if (editingEndpoint.value) { await client.put(`/subscription-endpoints/${editingEndpoint.value.id}`, epForm.value) } else { await client.post('/subscription-endpoints', epForm.value) } ElMessage.success(t('common.success')); epDialog.value = false; fetchEndpoints() } catch {} }
async function deleteEndpoint(row) { await ElMessageBox.confirm(t('subend.deleteConfirm', { domain: row.domain }), t('common.confirm')); await client.delete(`/subscription-endpoints/${row.id}`); ElMessage.success(t('common.success')); fetchEndpoints() }

onMounted(() => { fetchSubscriptions(); fetchFormats(); fetchEndpoints() })
</script>
