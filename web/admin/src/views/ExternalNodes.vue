<template>
  <div>
    <div style="display:flex;justify-content:space-between;margin-bottom:16px">
      <h3 style="margin:0">{{ t('externalNodes.title') }}</h3>
      <el-button type="primary" size="small" @click="showImport">{{ t('externalNodes.import') }}</el-button>
    </div>

    <!-- Sources -->
    <el-card style="margin-bottom:16px">
      <div class="hint" style="margin-bottom:8px">导入来源</div>
      <el-table :data="sources" stripe size="small">
        <el-table-column prop="id" label="ID" width="60" />
        <el-table-column prop="name" label="名称" width="180" />
        <el-table-column prop="source_type" label="类型" width="80">
          <template #default="{row}"><el-tag size="small">{{ row.source_type }}</el-tag></template>
        </el-table-column>
        <el-table-column label="状态" width="80">
          <template #default="{row}"><el-switch v-model="row.enabled" @change="updateSource(row)" /></template>
        </el-table-column>
        <el-table-column label="操作" width="120">
          <template #default="{row}">
            <el-button size="small" type="danger" text @click="deleteSource(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Nodes -->
    <el-card>
      <div class="hint" style="margin-bottom:8px">所有节点（支持单独编辑/删除）</div>
      <el-table :data="nodes" stripe size="small">
        <el-table-column prop="id" label="ID" width="50" />
        <el-table-column prop="name" label="名称" width="160" />
        <el-table-column prop="protocol" label="协议" width="80">
          <template #default="{row}"><el-tag size="small">{{ row.protocol }}</el-tag></template>
        </el-table-column>
        <el-table-column prop="host" label="地址" width="180" />
        <el-table-column prop="port" label="端口" width="70" />
        <el-table-column prop="transport" label="传输" width="80" />
        <el-table-column label="TLS" width="60">
          <template #default="{row}"><el-tag v-if="row.tls" size="small" type="success">TLS</el-tag></template>
        </el-table-column>
        <el-table-column label="状态" width="60">
          <template #default="{row}"><el-switch v-model="row.enabled" size="small" @change="updateNode(row)" /></template>
        </el-table-column>
        <el-table-column label="操作" width="120">
          <template #default="{row}">
            <el-button size="small" text @click="editNode(row)">编辑</el-button>
            <el-button size="small" type="danger" text @click="deleteNode(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Import Dialog -->
    <el-dialog v-model="importDlg" title="导入外部节点" width="700">
      <el-form label-width="100px" size="small">
        <el-form-item label="来源名称">
          <el-input v-model="form.name" placeholder="My Nodes" />
        </el-form-item>
        <el-form-item label="导入方式">
          <el-select v-model="form.source_type" style="width:200px">
            <el-option label="v2ray URI" value="uri" />
            <el-option label="Base64 URI" value="base64_uri" />
            <el-option label="YAML" value="yaml" />
            <el-option label="JSON" value="json" />
            <el-option label="订阅 URL" value="url" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="form.source_type==='url'" label="订阅 URL">
          <el-input v-model="form.source_url" placeholder="https://..." />
        </el-form-item>
        <el-form-item v-else label="导入内容">
          <el-input v-model="form.source_data" type="textarea" :rows="8" placeholder="粘贴 YAML/JSON/URI 内容..." />
          <div style="margin-top:8px;display:flex;gap:8px;align-items:center">
            <el-upload :auto-upload="false" :show-file-list="false" accept=".yaml,.yml,.json,.txt,.list" @change="onFileUpload">
              <el-button size="small">选择文件上传</el-button>
            </el-upload>
            <span style="font-size:12px;color:#909399">支持 .yaml .json .txt 文件</span>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="importDlg=false">取消</el-button>
        <el-button type="primary" @click="doImport">导入</el-button>
      </template>
    </el-dialog>

    <!-- Edit Node Dialog -->
    <el-dialog v-model="editDlg" title="编辑节点" width="600">
      <el-form label-width="100px" size="small">
        <el-form-item label="名称">
          <el-input v-model="editForm.name" />
        </el-form-item>
        <el-form-item label="代理配置 (JSON)">
          <el-input v-model="editForm.proxy_json" type="textarea" :rows="12" style="font-family:monospace;font-size:12px" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editDlg=false">取消</el-button>
        <el-button type="primary" @click="saveNode">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import client from '../api/client'

const { t } = useI18n()

const sources = ref([])
const nodes = ref([])
const importDlg = ref(false)
const form = ref({ name: '', source_type: 'uri', source_data: '', source_url: '' })
const editDlg = ref(false)
const editForm = ref({ id: 0, name: '', proxy_json: '' })

async function fetchSources() {
  try { sources.value = (await client.get('/external-nodes/sources')) || [] } catch {}
}
async function fetchNodes() {
  try { nodes.value = (await client.get('/external-nodes')) || [] } catch {}
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
    ElMessage.success(`导入成功，共 ${res?.node_count || 0} 个节点`)
    importDlg.value = false
    fetchSources()
    fetchNodes()
  } catch (e) { ElMessage.error(e?.message || '导入失败') }
}
async function updateSource(row) {
  try { await client.put(`/external-nodes/sources/${row.id}`, { enabled: row.enabled, name: row.name }) } catch {}
}
async function deleteSource(row) {
  await ElMessageBox.confirm('确定删除？将同时删除该来源下的所有节点。')
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
    ElMessage.success('保存成功')
  } catch (e) { ElMessage.error(e?.message || '保存失败') }
}
async function updateNode(row) {
  try { await client.put(`/external-nodes/${row.id}`, { enabled: row.enabled }) } catch {}
}
async function deleteNode(row) {
  await ElMessageBox.confirm('确定删除该节点？')
  try { await client.delete(`/external-nodes/${row.id}`); fetchNodes() } catch {}
}

onMounted(() => { fetchSources(); fetchNodes() })

defineExpose({ fetchSources, fetchNodes })
</script>
