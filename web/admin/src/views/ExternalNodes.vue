<template>
  <div>
    <div style="display:flex;justify-content:space-between;margin-bottom:16px">
      <h3 style="margin:0">{{ t('externalNodes.title') }}</h3>
      <el-button type="primary" size="small" @click="showImport">{{ t('externalNodes.import') }}</el-button>
    </div>

    <!-- Sources -->
    <el-card style="margin-bottom:16px">
      <div class="hint" style="margin-bottom:8px">{{ t('externalNodes.importSources') }}</div>
      <el-table :data="sourcesPagedList" stripe size="small">
        <el-table-column prop="id" label="ID" width="60" />
        <el-table-column prop="name" :label="t('common.name')" min-width="180" />
        <el-table-column prop="source_type" :label="t('common.type')" width="80">
          <template #default="{row}"><el-tag size="small">{{ row.source_type }}</el-tag></template>
        </el-table-column>
        <el-table-column :label="t('common.status')" width="80">
          <template #default="{row}"><el-switch v-model="row.enabled" @change="updateSource(row)" /></template>
        </el-table-column>
        <el-table-column :label="t('common.actions')" width="120">
          <template #default="{row}">
            <el-button size="small" type="danger" text @click="deleteSource(row)">{{ t('common.delete') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
      <PaginationBar v-model:page="sourcesPage" v-model:page-size="sourcesSize" :total="sourcesTotal" />
    </el-card>

    <!-- Nodes -->
    <el-card>
      <div class="hint" style="margin-bottom:8px">{{ t('externalNodes.allNodesHint') }}</div>
      <el-table :data="nodesPagedList" stripe size="small">
        <el-table-column prop="id" label="ID" width="50" />
        <el-table-column prop="name" :label="t('common.name')" min-width="160" />
        <el-table-column prop="protocol" :label="t('externalNodes.protocol')" width="80">
          <template #default="{row}"><el-tag size="small">{{ row.protocol }}</el-tag></template>
        </el-table-column>
        <el-table-column prop="host" :label="t('externalNodes.address')" width="180" />
        <el-table-column prop="port" :label="t('externalNodes.port')" width="70" />
        <el-table-column prop="transport" :label="t('externalNodes.transport')" width="80" />
        <el-table-column label="TLS" width="60">
          <template #default="{row}"><el-tag v-if="row.tls" size="small" type="success">TLS</el-tag></template>
        </el-table-column>
        <el-table-column :label="t('common.status')" width="120">
          <template #default="{row}">
            <div style="display:flex;gap:4px;align-items:center;white-space:nowrap">
              <el-switch v-model="row.enabled" size="small" @change="updateNode(row)" />
              <el-tag v-if="registeredNodeIds.has(row.id)" size="small" type="success">{{ t('externalNodes.registered') }}</el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column :label="t('common.actions')" width="220">
          <template #default="{row}">
            <div style="display:flex;gap:2px;align-items:center;white-space:nowrap">
              <el-button size="small" text @click="editNode(row)">{{ t('common.edit') }}</el-button>
              <el-button v-if="!registeredNodeIds.has(row.id)" size="small" type="success" text @click="registerAsEntry(row)">{{ t('externalNodes.registerAsEntry') }}</el-button>
              <el-button v-else size="small" type="warning" text @click="unregisterEntry(row)">{{ t('externalNodes.unregisterEntry') }}</el-button>
              <el-button size="small" type="danger" text @click="deleteNode(row)">{{ t('common.delete') }}</el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>
      <PaginationBar v-model:page="nodesPage" v-model:page-size="nodesSize" :total="nodesTotal" />
    </el-card>

    <!-- Import Dialog -->
    <el-dialog v-model="importDlg" :title="t('externalNodes.importDialogTitle')" width="700">
      <el-form label-width="100px" size="small">
        <el-form-item :label="t('externalNodes.name')">
          <el-input v-model="form.name" placeholder="My Nodes" />
        </el-form-item>
        <el-form-item :label="t('externalNodes.sourceType')">
          <el-select v-model="form.source_type" style="width:200px">
            <el-option :label="t('externalNodes.v2rayUri')" value="uri" />
            <el-option :label="t('externalNodes.base64Uri')" value="base64_uri" />
            <el-option label="YAML" value="yaml" />
            <el-option label="JSON" value="json" />
            <el-option :label="t('externalNodes.subUrl')" value="url" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="form.source_type==='url'" :label="t('externalNodes.sourceUrl')">
          <el-input v-model="form.source_url" placeholder="https://..." />
        </el-form-item>
        <el-form-item v-else :label="t('externalNodes.sourceData')">
          <el-input v-model="form.source_data" type="textarea" :rows="8" :placeholder="t('externalNodes.pasteContent')" />
          <div style="margin-top:8px;display:flex;gap:8px;align-items:center">
            <el-upload :auto-upload="false" :show-file-list="false" accept=".yaml,.yml,.json,.txt,.list" @change="onFileUpload">
              <el-button size="small">{{ t('externalNodes.selectFile') }}</el-button>
            </el-upload>
            <span style="font-size:12px;color:#909399">{{ t('externalNodes.supportedFiles') }}</span>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="importDlg=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="doImport">{{ t('common.import') }}</el-button>
      </template>
    </el-dialog>

    <!-- Edit Node Dialog -->
    <el-dialog v-model="editDlg" :title="t('externalNodes.editNode')" width="600">
      <el-form label-width="100px" size="small">
        <el-form-item :label="t('common.name')">
          <el-input v-model="editForm.name" />
        </el-form-item>
        <el-form-item :label="t('externalNodes.proxyConfig')">
          <el-input v-model="editForm.proxy_json" type="textarea" :rows="12" style="font-family:monospace;font-size:12px" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editDlg=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveNode">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- Register as Entry Dialog (§12.3) -->
    <el-dialog v-model="regDlg" :title="t('externalNodes.registerAsEntry')" width="520">
      <el-form label-width="100px" size="small">
        <el-form-item :label="t('externalNodes.externalNode')">
          <el-input :model-value="regForm.node_display" disabled />
        </el-form-item>
        <el-form-item :label="t('common.name')">
          <el-input v-model="regForm.name" :placeholder="t('externalNodes.nameHint')" />
        </el-form-item>
        <el-form-item :label="t('chainProxy.region')">
          <el-input v-model="regForm.region" placeholder="e.g. HK" />
        </el-form-item>
        <el-form-item :label="t('chainProxy.regionLabel')">
          <el-input v-model="regForm.region_label" placeholder="e.g. Hong Kong" />
        </el-form-item>
        <el-form-item :label="t('common.status')">
          <el-switch v-model="regForm.enabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="regDlg=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="regSaving" @click="doRegister">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import client from '../api/client'
import * as chainApi from '../api/chainlink'
import { useClientPagination } from '../composables/useClientPagination'
import PaginationBar from '../components/PaginationBar.vue'

const { t } = useI18n()

const sources = ref([])
const nodes = ref([])

const { paged: sourcesPagedList, page: sourcesPage, pageSize: sourcesSize, total: sourcesTotal } = useClientPagination(sources, { pageKey: 'extSrcPage', pageSizeKey: 'extSrcSize' })
const { paged: nodesPagedList, page: nodesPage, pageSize: nodesSize, total: nodesTotal } = useClientPagination(nodes, { pageKey: 'extNodePage', pageSizeKey: 'extNodeSize' })
const importDlg = ref(false)
const form = ref({ name: '', source_type: 'uri', source_data: '', source_url: '' })
const editDlg = ref(false)
const editForm = ref({ id: 0, name: '', proxy_json: '' })

// ── Register as Entry (§12.3) ──
const regDlg = ref(false)
const regSaving = ref(false)
const regForm = ref({ external_node_id: 0, node_display: '', name: '', region: '', region_label: '', enabled: true })
const registeredServers = ref([]) // external_node_servers from chain API
const registeredNodeIds = computed(() => new Set(registeredServers.value.map(s => s.external_node_id)))

async function fetchSources() {
  try { sources.value = (await client.get('/external-nodes/sources')) || [] } catch {}
}
async function fetchNodes() {
  try { nodes.value = (await client.get('/external-nodes')) || [] } catch {}
}
async function fetchRegisteredServers() {
  try { registeredServers.value = (await chainApi.listExternalServers()) || [] } catch {}
}
function showImport() {
  form.value = { name: '', source_type: 'uri', source_data: '', source_url: '' }
  importDlg.value = true
}
function onFileUpload(file) {
  const reader = new FileReader()
  reader.onload = (e) => {
    form.value.source_data = e.target.result
    // Auto-detect type from file extension
    const name = file.name || ''
    if (name.endsWith('.yaml') || name.endsWith('.yml')) {
      form.value.source_type = 'yaml'
    } else if (name.endsWith('.json')) {
      form.value.source_type = 'json'
    }
    if (!form.value.name) {
      form.value.name = name.replace(/\.[^.]+$/, '')
    }
  }
  reader.readAsText(file.raw)
}
async function doImport() {
  try {
    const res = await client.post('/external-nodes/sources', form.value)
    ElMessage.success(t('externalNodes.importSuccess', { count: res?.node_count || 0 }))
    importDlg.value = false
    fetchSources()
    fetchNodes()
  } catch (e) { ElMessage.error(e?.message || t('externalNodes.importError')) }
}
async function updateSource(row) {
  try { await client.put(`/external-nodes/sources/${row.id}`, { enabled: row.enabled, name: row.name }) } catch {}
}
async function deleteSource(row) {
  await ElMessageBox.confirm(t('externalNodes.confirmDeleteSource'))
  try { await client.delete(`/external-nodes/sources/${row.id}`); fetchSources(); fetchNodes() } catch {}
}
function editNode(row) {
  editForm.value = { id: row.id, name: row.name, proxy_json: row.proxy_json }
  editDlg.value = true
}
async function saveNode() {
  try {
    await client.put(`/external-nodes/${editForm.value.id}`, {
      name: editForm.value.name,
      proxy_json: editForm.value.proxy_json
    })
    editDlg.value = false
    fetchNodes()
    ElMessage.success(t('common.success'))
  } catch (e) { ElMessage.error(e?.message || t('common.failed')) }
}
async function updateNode(row) {
  try { await client.put(`/external-nodes/${row.id}`, { enabled: row.enabled }) } catch {}
}
async function deleteNode(row) {
  await ElMessageBox.confirm(t('externalNodes.confirmDeleteNode'))
  try { await client.delete(`/external-nodes/${row.id}`); fetchNodes() } catch {}
}

// ── Register as Entry (§12.3) ──
function registerAsEntry(row) {
  regForm.value = {
    external_node_id: row.id,
    node_display: `${row.name} (${row.host || row.protocol || ''})`,
    name: row.name,
    region: '',
    region_label: '',
    enabled: true,
  }
  regDlg.value = true
}

async function doRegister() {
  if (!regForm.value.external_node_id) {
    ElMessage.warning(t('chainProxy.selectExternalNode'))
    return
  }
  regSaving.value = true
  try {
    await chainApi.createExternalServer({
      external_node_id: regForm.value.external_node_id,
      name: regForm.value.name,
      region: regForm.value.region,
      region_label: regForm.value.region_label,
      enabled: regForm.value.enabled,
    })
    ElMessage.success(t('common.success'))
    regDlg.value = false
    await fetchRegisteredServers()
  } catch (e) { ElMessage.error(e?.message || t('common.failed')) } finally { regSaving.value = false }
}

async function unregisterEntry(row) {
  const server = registeredServers.value.find(s => s.external_node_id === row.id)
  if (!server) return
  try {
    await ElMessageBox.confirm(t('chainProxy.confirmDeleteExtServer'), t('common.confirm'), { type: 'warning' })
    await chainApi.deleteExternalServer(server.id)
    ElMessage.success(t('common.success'))
    await fetchRegisteredServers()
  } catch { /* cancelled */ }
}

onMounted(() => { fetchSources(); fetchNodes(); fetchRegisteredServers() })

defineExpose({ fetchSources, fetchNodes, fetchRegisteredServers })
</script>
