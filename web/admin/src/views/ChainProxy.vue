<template>
  <div>
    <div style="display:flex;justify-content:space-between;margin-bottom:16px">
      <h3 style="margin:0">{{ t('chainProxy.title') }}</h3>
      <div style="display:flex;gap:8px">
        <el-button type="primary" size="small" @click="showLinkDlg">{{ t('chainProxy.addLink') }}</el-button>
        <el-button size="small" @click="showExtServerDlg">{{ t('chainProxy.registerExternalEntry') }}</el-button>
      </div>
    </div>

    <!-- Topology Graph (§12.1 directed graph) -->
    <el-card style="margin-bottom:16px">
      <template #header>
        <div style="display:flex;justify-content:space-between;align-items:center">
          <span>{{ t('chainProxy.topology') }}</span>
          <el-button text size="small" @click="fetchTopology" :icon="Refresh">{{ t('common.reset') }}</el-button>
        </div>
      </template>
      <div v-loading="topoLoading" style="min-height:200px">
        <!-- Directed graph visualization using SVG -->
        <div v-if="topoNodes.length > 0" style="position:relative;overflow-x:auto">
          <svg :width="svgWidth" :height="svgHeight" style="display:block">
            <!-- Edges (arrows from entry to exit) -->
            <defs>
              <marker id="arrowhead" markerWidth="10" markerHeight="7" refX="9" refY="3.5" orient="auto">
                <polygon points="0 0, 10 3.5, 0 7" fill="#909399" />
              </marker>
              <marker id="arrowhead-health" markerWidth="10" markerHeight="7" refX="9" refY="3.5" orient="auto">
                <polygon points="0 0, 10 3.5, 0 7" fill="#67c23a" />
              </marker>
              <marker id="arrowhead-degraded" markerWidth="10" markerHeight="7" refX="9" refY="3.5" orient="auto">
                <polygon points="0 0, 10 3.5, 0 7" fill="#e6a23c" />
              </marker>
              <marker id="arrowhead-down" markerWidth="10" markerHeight="7" refX="9" refY="3.5" orient="auto">
                <polygon points="0 0, 10 3.5, 0 7" fill="#f56c6c" />
              </marker>
            </defs>
            <!-- Draw edges -->
            <line v-for="edge in topoEdges" :key="edge.key"
              :x1="edge.x1" :y1="edge.y1" :x2="edge.x2" :y2="edge.y2"
              :stroke="edge.color" stroke-width="2"
              :marker-end="edge.marker"
            />
            <!-- Edge labels (priority / loadbalance) -->
            <text v-for="edge in topoEdges" :key="'label-'+edge.key"
              :x="(edge.x1+edge.x2)/2" :y="(edge.y1+edge.y2)/2 - 5"
              text-anchor="middle" :fill="edge.color" font-size="11"
            >{{ edge.label }}</text>
            <!-- Draw nodes -->
            <g v-for="node in topoLayout" :key="node.id" @click="nodeClick(node)" style="cursor:pointer">
              <rect :x="node.x" :y="node.y" width="140" height="44" rx="8"
                :fill="nodeFill(node)" :stroke="nodeStroke(node)" stroke-width="2"
              />
              <text :x="node.x+70" :y="node.y+18" text-anchor="middle" font-size="13" font-weight="bold" fill="#303133">
                {{ node.name.length > 14 ? node.name.slice(0,13)+'…' : node.name }}
              </text>
              <text :x="node.x+70" :y="node.y+34" text-anchor="middle" font-size="10" fill="#909399">
                {{ roleLabel(node.role) }}
              </text>
            </g>
          </svg>
        </div>
        <el-empty v-else :description="t('chainProxy.noTopology')" />
      </div>
      <!-- Topology node detail table (below graph) -->
      <el-table v-if="topoNodes.length > 0" :data="topoNodes" stripe size="small" style="margin-top:12px">
        <el-table-column prop="id" label="ID" width="60" />
        <el-table-column prop="name" :label="t('common.name')" min-width="180" />
        <el-table-column prop="type" :label="t('common.type')" width="90">
          <template #default="{row}">
            <el-tag size="small" :type="row.type==='external' ? 'warning' : 'primary'">{{ row.type }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="role" :label="t('chainProxy.role')" width="130">
          <template #default="{row}">
            <el-tag size="small" :type="roleTagType(row.role)">{{ roleLabel(row.role) }}</el-tag>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Chain Links -->
    <el-card style="margin-bottom:16px">
      <template #header>{{ t('chainProxy.linksTitle') }}</template>
      <el-table :data="links" stripe size="small" v-loading="linksLoading">
        <el-table-column prop="id" label="ID" width="50" />
        <el-table-column :label="t('chainProxy.entryNode')" width="160">
          <template #default="{row}">{{ serverName(row.entry_server_id) }}</template>
        </el-table-column>
        <el-table-column :label="t('chainProxy.exitNode')" width="160">
          <template #default="{row}">{{ exitName(row) }}</template>
        </el-table-column>
        <el-table-column :label="t('chainProxy.exitInboundCol')" min-width="200">
          <template #default="{row}">
            <span v-if="row.exit_external_id">—</span>
            <span v-else>{{ linkExitInboundLabel(row) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="exit_tag" :label="t('chainProxy.exitTag')" width="120" />
        <el-table-column prop="priority" :label="t('chainProxy.priority')" width="80" />
        <el-table-column prop="weight" :label="t('chainProxy.weight')" width="80" />
        <el-table-column prop="sort_order" :label="t('chainProxy.sortOrder')" width="80" />
        <el-table-column :label="t('common.status')" width="70">
          <template #default="{row}">
            <el-switch v-model="row.enabled" size="small" @change="toggleLink(row)" />
          </template>
        </el-table-column>
        <el-table-column :label="t('common.actions')" width="120">
          <template #default="{row}">
            <div style="display:flex;gap:2px;align-items:center;white-space:nowrap">
              <el-button size="small" text @click="editLink(row)">{{ t('common.edit') }}</el-button>
              <el-button size="small" type="danger" text @click="removeLink(row)">{{ t('common.delete') }}</el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>
    </el-card>


    <!-- External Node Servers (§12.3) -->
    <el-card style="margin-top:16px">
      <template #header>{{ t('chainProxy.externalEntryServers') }}</template>
      <el-table :data="externalServers" stripe size="small">
        <el-table-column prop="id" label="ID" width="50" />
        <el-table-column prop="name" :label="t('common.name')" min-width="160" />
        <el-table-column prop="hostname" :label="t('common.host')" width="160" />
        <el-table-column prop="region" :label="t('chainProxy.region')" width="100" />
        <el-table-column prop="region_label" :label="t('chainProxy.regionLabel')" width="100" />
        <el-table-column :label="t('common.status')" width="70">
          <template #default="{row}">
            <el-tag size="small" :type="row.enabled ? 'success' : 'danger'">{{ row.enabled ? '✓' : '✗' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('common.actions')" width="100">
          <template #default="{row}">
            <el-button size="small" type="danger" text @click="removeExtServer(row)">{{ t('common.delete') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Chain Health Status -->
    <el-card style="margin-top:16px">
      <template #header>
        <div style="display:flex;justify-content:space-between;align-items:center">
          <span>{{ t('chainProxy.healthStatus') }}</span>
          <el-button text size="small" @click="fetchHealth" :icon="Refresh">{{ t('common.refresh') }}</el-button>
        </div>
      </template>
      <el-table :data="healthData" stripe size="small" v-loading="healthLoading">
        <el-table-column prop="link_id" label="ID" width="60" />
        <el-table-column :label="t('chainProxy.entryNode')" min-width="160">
          <template #default="{row}">{{ serverName(row.entry_server_id) }}</template>
        </el-table-column>
        <el-table-column :label="t('chainProxy.exitNode')" width="160">
          <template #default="{row}">{{ row.exit_external_id ? extServerName(row.exit_external_id) : serverName(row.exit_server_id) }}</template>
        </el-table-column>
        <el-table-column :label="t('chainProxy.entryOnline')" width="100">
          <template #default="{row}">
            <el-tag size="small" :type="row.entry_online ? 'success' : 'danger'">{{ row.entry_online ? '✓' : '✗' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('chainProxy.exitOnline')" width="100">
          <template #default="{row}">
            <el-tag size="small" :type="row.exit_online ? 'success' : 'danger'">{{ row.exit_online ? '✓' : '✗' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('common.status')" width="120">
          <template #default="{row}">
            <el-tag size="small" :type="healthTagType(row.status)">{{ healthLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Create Chain Proxy Wizard Dialog (see design doc §12.2) -->
    <el-dialog v-model="linkDlg" :title="t('chainProxy.createWizard')" width="680">
      <el-alert v-if="linkEditing" :title="t('chainProxy.editModeHint')" type="info" :closable="false" style="margin-bottom:12px" />
      <el-steps :active="wizardStep" finish-status="success" align-center>
        <el-step :title="t('chainProxy.stepEntry')" />
        <el-step :title="t('chainProxy.stepExits')" />
        <el-step :title="t('chainProxy.stepConfirm')" />
      </el-steps>
      <div style="margin-top:18px">
        <!-- Step 1: choose entry node + entry inbound -->
        <div v-show="wizardStep===0">
          <el-form label-width="110px" size="small">
            <el-form-item :label="t('chainProxy.entryNode')">
              <el-select v-model="wiz.entryServerId" filterable style="width:100%" @change="onEntryServerChange">
                <el-option v-for="s in servers" :key="s.id" :label="s.name" :value="s.id" />
              </el-select>
            </el-form-item>
            <el-form-item :label="t('chainProxy.entryInbound')">
              <el-select v-model="wiz.entryInboundId" filterable style="width:100%">
                <el-option v-for="ib in inboundConfigs" :key="ib.id" :label="`${ib.tag || ('in-'+ib.id)} · ${ib.protocol||''}`" :value="ib.id" />
              </el-select>
              <span style="font-size:12px;color:#909399">{{ t('chainProxy.entryInboundHint') }}</span>
            </el-form-item>
          </el-form>
        </div>
        <!-- Step 2: choose exits (cluster + external, both multi-select, like Plan/NodeGroup) -->
        <div v-show="wizardStep===1">
          <el-alert :title="t('chainProxy.multiExitHint')" type="info" :closable="false" style="margin-bottom:8px" />
          <el-alert :title="t('chainProxy.exitInboundHint')" type="warning" :closable="false" style="margin-bottom:12px" />
          <el-row :gutter="16">
            <el-col :span="12">
              <div class="exit-group-title">{{ t('chainProxy.clusterNodes') }}</div>
              <el-select v-model="wiz.clusterExits" multiple filterable style="width:100%" :placeholder="t('chainProxy.selectExits')" @change="onClusterExitsChange">
                <el-option v-for="s in servers" :key="s.id" :label="s.name" :value="s.id" />
              </el-select>
            </el-col>
            <el-col :span="12">
              <div class="exit-group-title">{{ t('chainProxy.externalNodes') }}</div>
              <el-select v-model="wiz.externalExits" multiple filterable style="width:100%" :placeholder="t('chainProxy.selectExits')">
                <el-option v-for="s in externalServers" :key="s.id" :label="s.name" :value="s.id" />
              </el-select>
            </el-col>
          </el-row>
          <!-- 每个出口一行：集群出口选它上面**已有的入站**；再统一设置优先级/权重/序号 -->
          <div v-if="totalSelectedExits" style="margin-top:14px">
            <div class="exit-group-title">{{ t('chainProxy.exitInboundTitle') }}</div>
            <div v-for="row in exitRows" :key="row.key"
                 style="display:flex;align-items:center;gap:8px;margin-bottom:8px">
              <span style="width:130px;font-size:13px;color:#303133;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">
                {{ row.isExternal ? extServerName(row.id) : serverName(row.id) }}
                <el-tag v-if="row.isExternal" size="small" type="warning" style="margin-left:4px">ext</el-tag>
              </span>
              <el-select v-if="!row.isExternal" v-model="wiz.exitInbounds[row.id]" filterable size="small" style="width:250px"
                         :placeholder="t('chainProxy.exitInboundPlaceholder')">
                <el-option v-for="ib in exitInboundCandidates(row.id)" :key="ib.id"
                           :label="inboundLabel(ib)" :value="ib.id" />
              </el-select>
              <span v-else style="width:250px;font-size:12px;color:#909399">{{ t('chainProxy.externalUsesOwnProtocol') }}</span>
              <span style="font-size:12px;color:#909399">
                {{ t('chainProxy.priority') }}
                <el-input-number v-model="paramOf(row).priority" :min="1" :max="99" size="small" controls-position="right" style="width:80px" />
              </span>
              <span style="font-size:12px;color:#909399">
                {{ t('chainProxy.weight') }}
                <el-input-number v-model="paramOf(row).weight" :min="1" :max="1000" size="small" controls-position="right" style="width:90px" />
              </span>
              <span style="font-size:12px;color:#909399">
                {{ t('chainProxy.sortOrder') }}
                <el-input-number v-model="paramOf(row).sort_order" :min="0" :max="999" size="small" controls-position="right" style="width:80px" />
              </span>
              <el-input v-model="paramOf(row).exit_tag" size="small" style="width:150px"
                        :placeholder="t('chainProxy.exitTagPlaceholder')" clearable />
              <span style="font-size:12px;color:#909399;flex:1">{{ row.isExternal ? '' : exitInboundTip(row.id) }}</span>
            </div>
            <div style="font-size:12px;color:#909399">{{ t('chainProxy.priorityHint') }}</div>
          </div>
          <div style="margin-top:10px;font-size:12px;color:#909399">
            {{ t('chainProxy.selectedCount', { n: totalSelectedExits }) }}
            <span v-if="totalSelectedExits>1" style="color:#e6a23c"> · {{ t('chainProxy.willLoadBalance') }}</span>
            <span v-else-if="totalSelectedExits===1" style="color:#67c23a"> · {{ t('chainProxy.willDirect') }}</span>
            <div style="margin-top:4px" v-if="!exitInboundsReady">{{ t('chainProxy.exitInboundRequired') }}</div>
          </div>
        </div>
        <!-- Step 3: confirm -->
        <div v-show="wizardStep===2">
          <el-descriptions :column="1" border size="small">
            <el-descriptions-item :label="t('chainProxy.entryNode')">{{ entryServerName }}</el-descriptions-item>
            <el-descriptions-item :label="t('chainProxy.entryInbound')">{{ entryInboundLabel }}</el-descriptions-item>
            <el-descriptions-item :label="t('chainProxy.exitType')">
              {{ totalSelectedExits>1 ? t('chainProxy.loadBalance') : t('chainProxy.direct') }}
            </el-descriptions-item>
            <el-descriptions-item :label="t('chainProxy.exitCount')">{{ totalSelectedExits }}</el-descriptions-item>
            <el-descriptions-item v-for="eid in wiz.clusterExits" :key="'c'+eid" :label="serverName(eid)">
              {{ exitInboundText(eid) }}
            </el-descriptions-item>
          </el-descriptions>
        </div>
      </div>
      <template #footer>
        <el-button @click="linkDlg=false">{{ t('common.cancel') }}</el-button>
        <el-button v-if="wizardStep>0" @click="wizardStep--">{{ t('common.prev') }}</el-button>
        <el-button v-if="wizardStep<2" type="primary" @click="nextWizardStep" :disabled="!canNextStep">{{ t('common.next') }}</el-button>
        <el-button v-else type="primary" :loading="linkSaving" @click="linkEditing ? saveLinkEdit() : createChainProxy()">
          {{ linkEditing ? t('common.save') : t('common.create') }}
        </el-button>
      </template>
    </el-dialog>


    <!-- External Node Server Registration Dialog (§12.3) -->
    <el-dialog v-model="extServerDlg" :title="t('chainProxy.registerExternalEntry')" width="520">
      <el-form label-width="110px" size="small">
        <el-form-item :label="t('chainProxy.externalNode')">
          <el-select v-model="extServerForm.external_node_id" filterable style="width:100%" :placeholder="t('chainProxy.selectExternalNode')">
            <el-option v-for="n in externalNodes" :key="n.id" :label="n.name + ' (' + (n.host || n.protocol || '') + ')'" :value="n.id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('common.name')">
          <el-input v-model="extServerForm.name" :placeholder="t('chainProxy.nameHint')" />
        </el-form-item>
        <el-form-item :label="t('chainProxy.region')">
          <el-input v-model="extServerForm.region" placeholder="e.g. HK" />
        </el-form-item>
        <el-form-item :label="t('chainProxy.regionLabel')">
          <el-input v-model="extServerForm.region_label" placeholder="e.g. Hong Kong" />
        </el-form-item>
        <el-form-item :label="t('common.status')">
          <el-switch v-model="extServerForm.enabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="extServerDlg=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="extServerSaving" @click="saveExtServer">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import * as chainApi from '../api/chainlink'
import { listNodeServers, getInboundConfigs } from '../api/nodeServer'

const { t } = useI18n()

// ── Health helpers ──
function healthTagType(status) {
  if (status === 'healthy') return 'success'
  if (status === 'degraded') return 'warning'
  return 'danger'
}
function healthLabel(status) {
  if (status === 'healthy') return t('chainProxy.healthy')
  if (status === 'degraded') return t('chainProxy.degraded')
  return t('chainProxy.down')
}
async function fetchHealth() {
  healthLoading.value = true
  try {
    const res = await chainApi.getChainHealth()
    // API interceptor 已解包 response.data.data，res 就是数组
    healthData.value = Array.isArray(res) ? res : (res?.data || [])
  } catch (e) {
    healthData.value = []
  } finally {
    healthLoading.value = false
  }
}
function extServerName(id) {
  const s = externalServers.value?.find(x => x.id === id)
  return s ? s.name : `External #${id}`
}

// ── Data ──
const topoNodes = ref([])
const topoLinks = ref([])
const topoLoading = ref(false)
const links = ref([])
const linksLoading = ref(false)
const servers = ref([])
const externalServers = ref([])
const externalNodes = ref([]) // available external nodes for registration dropdown

// ── Health ──
const healthData = ref([])
const healthLoading = ref(false)

// ── Link Dialog ──
const linkDlg = ref(false)
const linkEditing = ref(false)
const linkSaving = ref(false)

// ── Create Chain Proxy Wizard (§5.5 / §12.2) ──
const wizardStep = ref(0)
const inboundConfigs = ref([])
const wiz = ref({
  entryServerId: 0,
  entryInboundId: 0,
  clusterExits: [],
  externalExits: [],
  // 集群出口 → 选中的"出口节点上已有的入站" id
  exitInbounds: {},
  // 出口（集群/外部）→ 路由参数。key 形如 c<serverId> / e<externalId>（两类 id 可能撞号，所以加前缀）
  exitParams: {},
})
const totalSelectedExits = computed(() => wiz.value.clusterExits.length + wiz.value.externalExits.length)
// 出口行：集群 + 外部统一成一行（外部节点用其自身协议，不需要选入站）
const exitRows = computed(() => [
  ...wiz.value.clusterExits.map((id, i) => ({ key: `c${id}`, id, isExternal: false, index: i })),
  ...wiz.value.externalExits.map((id, i) => ({ key: `e${id}`, id, isExternal: true, index: wiz.value.clusterExits.length + i })),
])
// 路由参数默认值：priority 按选择顺序（与后端 "priority=0 → i+1" 的语义一致 ✓）
function exitParam(key, index) {
  if (!wiz.value.exitParams[key]) {
    wiz.value.exitParams[key] = { priority: index + 1, weight: 100, sort_order: index, exit_tag: '' }
  }
  return wiz.value.exitParams[key]
}
// 模板里取参数（缺省时自动补默认值，避免首次渲染读到 undefined）
function paramOf(row) { return exitParam(row.key, row.index) }

// ⚠ 与后端 `agent.ChainSupportedProtocols(coreType)` 必须保持一致：
//   实测 xray **没有** hysteria2 出站（`xray run -test` → unknown config id: hysteria2），
//   所以 hy2 出口只能配给 sing-box 入口。后端也会校验一遍，这里只是让用户选不到非法组合。
const CHAIN_PROTOCOLS = {
  xray: ['vless', 'vmess', 'trojan', 'shadowsocks'],
  singbox: ['vless', 'vmess', 'trojan', 'shadowsocks', 'hysteria2'],
}
function normCore(core) {
  const c = String(core || '').toLowerCase().trim()
  if (c === 'singbox' || c === 'sing-box' || c === 'sing_box') return 'singbox'
  return 'xray'
}
const entryCore = computed(() => {
  const ib = inboundConfigs.value.find(x => x.id === wiz.value.entryInboundId)
  return normCore(ib && ib.core_type)
})
const chainAllowedProtocols = computed(() => CHAIN_PROTOCOLS[entryCore.value] || [])
// 出口节点 → 该节点上的入站候选（按需加载）
const exitInboundOptions = ref({})
const exitInboundsReady = computed(() =>
  wiz.value.clusterExits.every(id => Number(wiz.value.exitInbounds[id]) > 0))

const canNextStep = computed(() => {
  if (wizardStep.value === 0) return wiz.value.entryServerId && wiz.value.entryInboundId
  if (wizardStep.value === 1) {
    if (totalSelectedExits.value === 0) return false
    return exitInboundsReady.value // 每个集群出口都必须选好已有入站
  }
  return true
})
const entryServerName = computed(() => serverName(wiz.value.entryServerId))
const entryInboundLabel = computed(() => {
  const ib = inboundConfigs.value.find(x => x.id === wiz.value.entryInboundId)
  return ib ? `${ib.tag || ('in-' + ib.id)} · ${ib.protocol || ''}` : `#${wiz.value.entryInboundId}`
})
function resetWizard() {
  wizardStep.value = 0
  inboundConfigs.value = []
  exitInboundOptions.value = {}
  wiz.value = { entryServerId: 0, entryInboundId: 0, clusterExits: [], externalExits: [], exitInbounds: {}, exitParams: {} }
}

// ── External Node Server Registration Dialog (§12.3) ──
const extServerDlg = ref(false)
const extServerSaving = ref(false)
const extServerForm = ref({
  external_node_id: 0,
  name: '',
  region: '',
  region_label: '',
  enabled: true,
})

// ── Topology Graph Layout (§12.1) ──
// Compute node positions in a left-to-right directed graph
const topoLayout = computed(() => {
  if (!topoNodes.value.length) return []
  // Layout strategy: 3 columns
  // Col 0 (left):  pure entry nodes (role === 'entry')
  // Col 1 (mid):   entry+exit nodes (role === 'entry+exit' or 'exit+entry')
  // Col 2 (right): pure exit nodes (role === 'exit') + external entries
  const pureEntries = topoNodes.value.filter(n => n.role === 'entry')
  const dualRoles = topoNodes.value.filter(n => n.role === 'entry+exit' || n.role === 'exit+entry')
  const pureExits = topoNodes.value.filter(n => n.role === 'exit')
  const externals = topoNodes.value.filter(n => n.role === 'external_entry' || n.type === 'external')

  const nodeH = 54
  const colW = 200
  const padX = 20
  const padY = 20

  const layout = []
  // Place pure entries in left column
  pureEntries.forEach((n, i) => {
    layout.push({ ...n, x: padX, y: padY + i * nodeH, col: 0 })
  })
  // Place dual-role nodes in middle column
  dualRoles.forEach((n, i) => {
    layout.push({ ...n, x: padX + colW, y: padY + i * nodeH, col: 1 })
  })
  // Place pure exits in right column
  pureExits.forEach((n, i) => {
    layout.push({ ...n, x: padX + colW * 2, y: padY + i * nodeH, col: 2 })
  })
  // Place externals in far-right column
  externals.forEach((n, i) => {
    layout.push({ ...n, x: padX + colW * 3, y: padY + i * nodeH, col: 3 })
  })
  return layout
})

const svgWidth = computed(() => Math.max(480, (Math.max(...topoLayout.value.map(n => n.col), 0) + 1) * 200 + 40))
const svgHeight = computed(() => Math.max(200, topoLayout.value.length * 54 + 40))

// Compute edges for the directed graph
const topoEdges = computed(() => {
  if (!topoLinks.value.length || !topoLayout.value.length) return []
  const nodeMap = new Map()
  topoLayout.value.forEach(n => nodeMap.set(n.id, n))

  // Also map external node IDs (they use a different ID space in links)
  const edges = []
  const seen = new Set()

  topoLinks.value.forEach(link => {
    // Entry → Exit edge
    if (link.entry_server_id && link.exit_server_id) {
      const src = nodeMap.get(link.entry_server_id)
      const dst = nodeMap.get(link.exit_server_id)
      if (src && dst) {
        const key = `${src.id}-${dst.id}`
        if (!seen.has(key)) {
          seen.add(key)
          edges.push(buildEdge(src, dst, link, 'server'))
        }
      }
    }
    // Entry → External exit edge
    if (link.entry_server_id && link.exit_external_id) {
      const src = nodeMap.get(link.entry_server_id)
      const dst = nodeMap.get(link.exit_external_id)
      if (src && dst) {
        const key = `${src.id}-ext${dst.id}`
        if (!seen.has(key)) {
          seen.add(key)
          edges.push(buildEdge(src, dst, link, 'external'))
        }
      }
    }
  })
  return edges
})

function buildEdge(src, dst, link, type) {
  const x1 = src.x + 140 // right edge of source node
  const y1 = src.y + 22
  const x2 = dst.x // left edge of dest node
  const y2 = dst.y + 22

  // Determine health-based color
  const healthEntry = healthData.value?.find(h => h.link_id === link.id)
  let color = '#909399'
  let marker = 'url(#arrowhead)'
  if (healthEntry) {
    if (healthEntry.status === 'healthy') {
      color = '#67c23a'
      marker = 'url(#arrowhead-health)'
    } else if (healthEntry.status === 'degraded') {
      color = '#e6a23c'
      marker = 'url(#arrowhead-degraded)'
    } else {
      color = '#f56c6c'
      marker = 'url(#arrowhead-down)'
    }
  }

  // Label: priority or "LB" for loadbalance
  let label = ''
  if (link.priority) label = `P${link.priority}`
  if (type === 'external') label = `ext${label}`

  return { key: `${src.id}-${dst.id}`, x1, y1, x2, y2, color, marker, label }
}

function nodeFill(node) {
  if (node.role === 'entry') return '#ecf5ff'
  if (node.role === 'exit') return '#f0f9eb'
  if (node.role === 'entry+exit' || node.role === 'exit+entry') return '#fdf6ec'
  if (node.type === 'external') return '#fef0f0'
  return '#f4f4f5'
}
function nodeStroke(node) {
  if (node.role === 'entry') return '#409eff'
  if (node.role === 'exit') return '#67c23a'
  if (node.role === 'entry+exit' || node.role === 'exit+entry') return '#e6a23c'
  if (node.type === 'external') return '#f56c6c'
  return '#dcdfe6'
}
function nodeClick(node) {
  // Could open detail panel — for now just a visual feedback
}

// ── Helpers ──
function serverName(id) {
  const s = servers.value?.find(x => x.id === id)
  return s ? s.name : `#${id}`
}

function exitName(row) {
  if (row.exit_server_id) return serverName(row.exit_server_id)
  const es = externalServers.value?.find(x => x.id === row.exit_external_id)
  return es ? es.name : `ext:${row.exit_external_id}`
}

function roleLabel(role) {
  const map = {
    'entry': t('chainProxy.roleEntry'),
    'exit': t('chainProxy.roleExit'),
    'entry+exit': t('chainProxy.roleEntryExit'),
    'exit+entry': t('chainProxy.roleExitEntry'),
    'external_entry': t('chainProxy.roleExternalEntry'),
    'standalone': t('chainProxy.roleStandalone'),
  }
  return map[role] || role
}

function roleTagType(role) {
  const map = {
    'entry': 'primary',
    'exit': 'success',
    'entry+exit': 'warning',
    'exit+entry': 'warning',
    'external_entry': 'info',
    'standalone': '',
  }
  return map[role] || ''
}

// ── Fetch ──
async function fetchAll() {
  await Promise.all([fetchTopology(), fetchLinks(), fetchServers(), fetchExternalServers(), fetchExternalNodes(), fetchHealth()])
}

async function fetchTopology() {
  topoLoading.value = true
  try {
    const data = await chainApi.getTopology()
    topoNodes.value = data.nodes || []
    topoLinks.value = data.links || []
  } finally { topoLoading.value = false }
}

async function fetchLinks() {
  linksLoading.value = true
  try {
    links.value = await chainApi.listLinks()
  } finally { linksLoading.value = false }
}

async function fetchServers() {
  // /node-servers returns {items:[], total:N} pagination; extract the array
  const res = await listNodeServers()
  servers.value = Array.isArray(res) ? res : (res?.items || [])
}

async function fetchExternalServers() {
  externalServers.value = await chainApi.listExternalServers()
}

async function fetchExternalNodes() {
  try {
    externalNodes.value = await chainApi.listExternalNodes()
  } catch { /* ignore */ }
}

// ── Link CRUD ──
const linkEditingId = ref(0)

// 链接列表变化时，按需把出口节点的入站列表拉进来（链接表要显示"出口入站"列）
watch(links, rows => { loadExitInboundsForLinks(rows) })

function showLinkDlg() {
  linkEditing.value = false
  linkEditingId.value = 0
  resetWizard()
  linkDlg.value = true
}

async function onEntryServerChange(serverId) {
  wiz.value.entryInboundId = 0
  inboundConfigs.value = []
  if (!serverId) return
  try {
    const data = await getInboundConfigs(serverId)
    inboundConfigs.value = Array.isArray(data) ? data : (data.inbound_configs || [])
  } catch { /* ignore */ }
}

function inboundLabel(ib) {
  const port = ib.port ? ` :${ib.port}` : ''
  return `${ib.tag || ('in-' + ib.id)} · ${ib.protocol || ''}${port}`
}

// 出口入站候选：只列**启用**且协议在入口核心支持名单里的入站
function exitInboundCandidates(serverId) {
  const list = exitInboundOptions.value[serverId] || []
  return list.filter(ib => ib.enabled !== false
    && chainAllowedProtocols.value.includes(String(ib.protocol || '').toLowerCase()))
}

function exitInboundTip(serverId) {
  const list = exitInboundOptions.value[serverId]
  if (!list) return t('chainProxy.loading')
  const cands = exitInboundCandidates(serverId)
  if (!cands.length) {
    return t('chainProxy.noExitInbound', { list: chainAllowedProtocols.value.join('/') })
  }
  const ib = cands.find(x => x.id === Number(wiz.value.exitInbounds[serverId]))
  return ib ? '' : t('chainProxy.pickExitInbound')
}

function exitInboundText(serverId) {
  const ib = exitInboundCandidates(serverId).find(x => x.id === Number(wiz.value.exitInbounds[serverId]))
  return ib ? inboundLabel(ib) : '-'
}

async function loadExitInbounds(serverId) {
  if (exitInboundOptions.value[serverId]) return
  try {
    const data = await getInboundConfigs(serverId)
    exitInboundOptions.value[serverId] = Array.isArray(data) ? data : (data.inbound_configs || [])
  } catch {
    exitInboundOptions.value[serverId] = []
  }
}

async function onClusterExitsChange(ids) {
  // 清掉已取消节点的选择与缓存
  for (const key of Object.keys(wiz.value.exitInbounds)) {
    if (!ids.map(Number).includes(Number(key))) delete wiz.value.exitInbounds[key]
  }
  for (const id of ids) await loadExitInbounds(id)
}

function nextWizardStep() {
  if (!canNextStep.value) return
  if (wizardStep.value < 2) wizardStep.value++
}

// 编辑已有链路：v2.0 下可改的是"出口入站"（节点不支持改，改节点等于重建链路）。
// 之前这里**没有分支** ✗ —— 点"编辑"再点保存会去调用 createChainProxy，
// 而它是"先清后建"，等于把该入口的所有链路删掉重建（丢 weight/priority/sort_order 与链路 id）。
async function saveLinkEdit() {
  if (!linkEditingId.value) return
  const exitId = wiz.value.clusterExits[0]
  const payload = {
    exit_inbound_id: Number(wiz.value.exitInbounds[exitId]) || 0,
  }
  // 编辑态同样要保存优先级/权重/序号 —— 否则在对话框里改了这三项会"看起来生效、实际没保存" ✗
  const row = exitRows.value[0]
  if (row) {
    const prm = paramOf(row)
    payload.priority = Number(prm.priority) || 1
    payload.weight = Number(prm.weight) || 100
    payload.sort_order = Number(prm.sort_order) || 0
    if (prm.exit_tag) payload.exit_tag = prm.exit_tag
  }
  linkSaving.value = true
  try {
    await chainApi.updateLink(linkEditingId.value, payload)
    ElMessage.success(t('common.success'))
    linkDlg.value = false
    await fetchLinks()
    await fetchTopology()
  } catch { /* error handled by interceptor */ } finally { linkSaving.value = false }
}

// 链接列表里显示每条链路用的是哪个出口入站（需要按需加载出口节点的入站列表）
function linkExitInboundLabel(row) {
  const id = row.exit_inbound_id || 0
  if (!id) return t('chainProxy.legacyRelayInbound')
  const list = exitInboundOptions.value[row.exit_server_id] || []
  const ib = list.find(x => x.id === Number(id))
  return ib ? inboundLabel(ib) : `#${id}`
}

async function loadExitInboundsForLinks(rows) {
  for (const row of rows || []) {
    if (row.exit_server_id && !row.exit_external_id) await loadExitInbounds(row.exit_server_id)
  }
}

async function createChainProxy() {
  if (!canNextStep.value) return
  // 每个出口带上自己的优先级/权重/序号（后端 CreateChainProxy 逐条落库，priority=0 时按顺序兜底）
  const withParams = (row, i) => {
    const p = exitParam(row.key, i)
    return {
      is_external: row.isExternal,
      id: row.id,
      priority: Number(p.priority) || i + 1,
      weight: Number(p.weight) || 100,
      sort_order: Number(p.sort_order) || 0,
      ...(p.exit_tag ? { exit_tag: p.exit_tag } : {}),
      ...(row.isExternal ? {} : { exit_inbound_id: Number(wiz.value.exitInbounds[row.id]) || 0 }),
    }
  }
  const exits = exitRows.value.map((row, i) => withParams(row, i))
  if (exits.length === 0) {
    ElMessage.warning(t('chainProxy.selectExit'))
    return
  }
  linkSaving.value = true
  try {
    const payload = {
      entry_server_id: wiz.value.entryServerId,
      entry_inbound_id: wiz.value.entryInboundId,
      exits,
    }
    await chainApi.createChainProxy(payload)
    ElMessage.success(t('common.success'))
    linkDlg.value = false
    await fetchLinks()
    await fetchTopology()
  } catch { /* error handled by interceptor */ } finally { linkSaving.value = false }
}

async function editLink(row) {
  linkEditing.value = true
  linkEditingId.value = row.id || 0
  resetWizard()
  wiz.value.entryServerId = row.entry_server_id
  wiz.value.entryInboundId = row.entry_inbound_id || row.entryInboundId || 0
  if (row.exit_external_id) {
    wiz.value.externalExits = [row.exit_external_id]
    wiz.value.exitParams[`e${row.exit_external_id}`] = {
      priority: row.priority || 1,
      weight: row.weight || 100,
      sort_order: row.sort_order || 0,
      exit_tag: row.exit_tag || '',
    }
  } else if (row.exit_server_id) {
    wiz.value.clusterExits = [row.exit_server_id]
    const eib = row.exit_inbound_id || 0
    if (eib) wiz.value.exitInbounds[row.exit_server_id] = eib
    wiz.value.exitParams[`c${row.exit_server_id}`] = {
      priority: row.priority || 1,
      weight: row.weight || 100,
      sort_order: row.sort_order || 0,
      exit_tag: row.exit_tag || '',
    }
    await loadExitInbounds(row.exit_server_id)
  }
  if (wiz.value.entryServerId) {
    try {
      const data = await getInboundConfigs(wiz.value.entryServerId)
      inboundConfigs.value = Array.isArray(data) ? data : (data.inbound_configs || [])
    } catch { /* ignore */ }
  }
  linkDlg.value = true
}

async function toggleLink(row) {
  try {
    await chainApi.updateLink(row.id, { enabled: row.enabled })
    ElMessage.success(t('common.success'))
  } catch {
    row.enabled = !row.enabled
  }
}

async function removeLink(row) {
  try {
    await ElMessageBox.confirm(t('chainProxy.confirmDeleteLink'), t('common.confirm'), { type: 'warning' })
    await chainApi.deleteLink(row.id)
    ElMessage.success(t('common.success'))
    await fetchLinks()
    await fetchTopology()
  } catch { /* cancelled */ }
}

// ── External Node Server Registration CRUD (§12.3) ──
function showExtServerDlg() {
  extServerForm.value = {
    external_node_id: 0,
    name: '',
    region: '',
    region_label: '',
    enabled: true,
  }
  extServerDlg.value = true
  // Ensure we have the latest external nodes list for the dropdown
  if (!externalNodes.value.length) {
    fetchExternalNodes()
  }
}

async function saveExtServer() {
  if (!extServerForm.value.external_node_id) {
    ElMessage.warning(t('chainProxy.selectExternalNode'))
    return
  }
  extServerSaving.value = true
  try {
    await chainApi.createExternalServer(extServerForm.value)
    ElMessage.success(t('common.success'))
    extServerDlg.value = false
    await fetchExternalServers()
    await fetchTopology()
  } catch { /* handled */ } finally { extServerSaving.value = false }
}

async function removeExtServer(row) {
  try {
    await ElMessageBox.confirm(t('chainProxy.confirmDeleteExtServer'), t('common.confirm'), { type: 'warning' })
    await chainApi.deleteExternalServer(row.id)
    ElMessage.success(t('common.success'))
    await fetchExternalServers()
    await fetchTopology()
  } catch { /* cancelled */ }
}

// ── 自动刷新（拓扑 / 链路 / 健康状态每 30 秒拉一次）──
// 弹窗打开或页面在后台时跳过，避免正在编辑时列表在背后跳动。
const AUTO_REFRESH_MS = 30000
let autoRefreshTimer = 0

async function autoRefreshTick() {
  if (linkDlg.value || extServerDlg.value) return
  if (typeof document !== 'undefined' && document.hidden) return
  await Promise.all([fetchTopology(), fetchLinks(), fetchHealth()])
}

function startAutoRefresh() {
  stopAutoRefresh()
  autoRefreshTimer = window.setInterval(autoRefreshTick, AUTO_REFRESH_MS)
}

function stopAutoRefresh() {
  if (autoRefreshTimer) {
    window.clearInterval(autoRefreshTimer)
    autoRefreshTimer = 0
  }
}

onUnmounted(stopAutoRefresh)

onMounted(() => {
  fetchAll()
  startAutoRefresh()
})
</script>