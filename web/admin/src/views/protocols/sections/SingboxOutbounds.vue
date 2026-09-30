<template>
  <div>
    <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:16px">
      <span style="font-size:15px;font-weight:600">{{ t('singbox.outbound.title') }}</span>
      <div>
        <!-- ↑↓/删除只改本地暂存，点这里才落库并同步到节点 -->
        <el-button type="success" :loading="savingOrder" @click="persistOrder">{{ t('common.save') }}</el-button>
        <el-button size="small" type="primary" @click="showAdd">{{ t('singbox.outbound.add') }}</el-button>
      </div>
    </div>

    <el-table v-if="!isMobile" :data="obPaged" stripe style="width:100%" size="small">
      <el-table-column label="#" width="50" align="right">
        <template #default="{ $index }"><span style="font-size:11px;opacity:0.6">{{ (obPage - 1) * obSize + $index + 1 }}</span></template>
      </el-table-column>
      <el-table-column :label="t('singbox.outbound.actions')" width="210" align="center">
        <template #default="{ row, $index }">
          <!-- 对齐 xray 出站列表：↑↓ 独立文字按钮直接可见（顺序 = 配置文件顺序，
               第一个出站是 route.final 留空时的兜底出口） -->
          <el-button size="small" text :disabled="(obPage - 1) * obSize + $index === 0"
            @click="handleAction('up', row)">↑</el-button>
          <el-button size="small" text :disabled="(obPage - 1) * obSize + $index === form.length - 1"
            @click="handleAction('down', row)">↓</el-button>
          <el-button size="small" text type="primary" @click="handleAction('edit', row)">{{ t('common.edit') }}</el-button>
          <el-button size="small" text type="danger" @click="handleAction('delete', row)">{{ t('common.delete') }}</el-button>
        </template>
      </el-table-column>
      <el-table-column :label="t('singbox.outbound.tag')" min-width="120" show-overflow-tooltip>
        <template #default="{row}"><span style="font-weight:600">{{ row.tag || '-' }}</span></template>
      </el-table-column>
      <el-table-column :label="t('singbox.outbound.type')" width="100" align="center">
        <template #default="{row}"><el-tag size="small">{{ row.type }}</el-tag></template>
      </el-table-column>
      <el-table-column label="Server" min-width="160" show-overflow-tooltip>
        <template #default="{row}">
          <span v-if="row.server" style="font-size:12px">{{ row.server }}:{{ row.server_port }}</span>
          <span v-else style="color:#c0c4cc">-</span>
        </template>
      </el-table-column>
      <el-table-column label="TLS" width="60" align="center">
        <template #default="{row}">
          <el-tag v-if="row._tlsEnabled" size="small" type="warning">TLS</el-tag>
          <span v-else style="color:#c0c4cc">-</span>
        </template>
      </el-table-column>
    </el-table>
    <MobileCards v-else :rows="obPaged" title-field="tag" :fields="sbOutboundCardFields">
      <template #actions="{ row }">
        <!-- ↑↓ 沿用桌面逻辑：handleAction 内部用 form.indexOf(row) 定位，禁用态按全局序计算 -->
        <el-button size="small" text :disabled="form.indexOf(row) === 0"
          @click="handleAction('up', row)">↑</el-button>
        <el-button size="small" text :disabled="form.indexOf(row) === form.length - 1"
          @click="handleAction('down', row)">↓</el-button>
        <el-button size="small" text type="primary" @click="handleAction('edit', row)">{{ t('common.edit') }}</el-button>
        <el-button size="small" text type="danger" @click="handleAction('delete', row)">{{ t('common.delete') }}</el-button>
      </template>
    </MobileCards>
    <PaginationBar v-model:page="obPage" v-model:page-size="obSize" :total="obTotal" />
    <div v-if="!form.length" style="text-align:center;color:#909399;padding:24px">{{ t('singbox.outbound.noOutbounds') }}</div>

    <!-- Edit Dialog: 容器层统一 el-tabs（对齐 xray 弹窗模型） -->
    <el-dialog v-model="dlg" :title="editIdx >= 0 ? t('singbox.outbound.edit') : t('singbox.outbound.add')" width="760" :close-on-click-modal="false" top="5vh">
      <el-tabs v-model="activeTab">
        <!-- Basic: 类型 + 服务器/拨号通用参数 -->
        <el-tab-pane :label="t('singbox.outbound.tabs.basic')" name="basic">
          <el-form label-width="150px" label-position="left">
            <el-form-item :label="t('singbox.outbound.type')">
              <el-select v-model="rf.type" style="width:100%" @change="onTypeChange">
                <el-option v-for="t in OUTBOUND_TYPES" :key="t.value" :label="t.label" :value="t.value" />
              </el-select>
            </el-form-item>
          </el-form>
          <SingboxOutboundBase v-if="isServerType(rf.type)" :model="rf" @update="noop" />
        </el-tab-pane>

        <!-- Protocol: 协议专属设置 -->
        <el-tab-pane :label="t('singbox.outbound.tabs.protocol')" name="protocol">
          <component
            :is="getProtocolComp(rf.type)"
            :key="rf.type"
            :model="rf"
            @update="noop"
          />
        </el-tab-pane>

        <!-- TLS -->
        <el-tab-pane v-if="TLS_PROTOCOLS.includes(rf.type)" :label="t('singbox.outbound.tabs.tls')" name="tls">
          <SingboxOutboundTls :model="rf" @update="noop" />
        </el-tab-pane>

        <!-- Transport -->
        <el-tab-pane v-if="TRANS_PROTOCOLS.includes(rf.type)" :label="t('singbox.outbound.tabs.transport')" name="transport">
          <SingboxTransport :model="rf" @update="noop" />
        </el-tab-pane>

        <!-- Multiplex -->
        <el-tab-pane v-if="MUX_PROTOCOLS.includes(rf.type)" :label="t('singbox.outbound.tabs.multiplex')" name="multiplex">
          <SingboxMultiplex :model="rf" @update="noop" />
        </el-tab-pane>
      </el-tabs>
      <template #footer>
        <el-button @click="dlg = false">{{ t('singbox.outbound.cancel') }}</el-button>
        <el-button type="primary" @click="save">{{ t('singbox.outbound.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, ref, reactive, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import client from '../../../api/client'
import { useClientPagination } from '../../../composables/useClientPagination'
import { useQueryState } from '../../../composables/useQueryState'
import PaginationBar from '../../../components/PaginationBar.vue'
import MobileCards from '../../../components/MobileCards.vue'
import { useIsMobile } from '../../../composables/useIsMobile'
import { buildOutbound, parseOutbound, outboundDefaultsForType } from './singbox/utils/singboxBuildOutbound'
import {
  OUTBOUND_TLS_PROTOCOLS, OUTBOUND_TRANSPORT_PROTOCOLS, OUTBOUND_MULTIPLEX_PROTOCOLS, OUTBOUND_SERVER_PROTOCOLS,
} from './singbox/utils/singboxDefaults'
import SingboxOutboundBase from './singbox/common/SingboxOutboundBase.vue'
import SingboxOutboundTls from './singbox/common/SingboxOutboundTls.vue'
import SingboxTransport from './singbox/common/SingboxTransport.vue'
import SingboxMultiplex from './singbox/common/SingboxMultiplex.vue'
// 协议专属设置组件（通用 Base/TLS/Transport/Multiplex 由容器层提供）
import SingboxObVless from './singbox/outbounds/SingboxObVless.vue'
import SingboxObVmess from './singbox/outbounds/SingboxObVmess.vue'
import SingboxObTrojan from './singbox/outbounds/SingboxObTrojan.vue'
import SingboxObShadowsocks from './singbox/outbounds/SingboxObShadowsocks.vue'
import SingboxObShadowsocks2022 from './singbox/outbounds/SingboxObShadowsocks2022.vue'
import SingboxObTuic from './singbox/outbounds/SingboxObTuic.vue'
import SingboxObHysteria2 from './singbox/outbounds/SingboxObHysteria2.vue'
import SingboxObHysteria from './singbox/outbounds/SingboxObHysteria.vue'
import SingboxObHttp from './singbox/outbounds/SingboxObHttp.vue'
import SingboxObAnyTls from './singbox/outbounds/SingboxObAnyTls.vue'
import SingboxObShadowtls from './singbox/outbounds/SingboxObShadowtls.vue'
import SingboxObSnell from './singbox/outbounds/SingboxObSnell.vue'
import SingboxObTor from './singbox/outbounds/SingboxObTor.vue'
import SingboxObSsh from './singbox/outbounds/SingboxObSsh.vue'
import SingboxObNaive from './singbox/outbounds/SingboxObNaive.vue'
import SingboxObBridge from './singbox/outbounds/SingboxObBridge.vue'
import SingboxObGroup from './singbox/outbounds/SingboxObGroup.vue'
import SingboxObSimple from './singbox/outbounds/SingboxObSimple.vue'

const { t } = useI18n()
const props = defineProps({ data: Array, coreType: String, serverId: Number })
const emitFn = defineEmits(['update'])
const form = reactive([])
const { paged: obPaged, page: obPage, pageSize: obSize, total: obTotal } = useClientPagination(computed(() => form), { pageKey: 'sbObPage', pageSizeKey: 'sbObSize' })
const { isMobile } = useIsMobile()

// 手机端卡片字段（与桌面表格列一一对应；↑↓/编辑/删除在 actions 插槽里）
const sbOutboundCardFields = computed(() => [
  { label: t('singbox.outbound.type'), text: (row) => row.type || '-' },
  { label: 'Server', text: (row) => row.server ? `${row.server}:${row.server_port}` : '-' },
  { label: 'TLS', text: (row) => row._tlsEnabled ? 'TLS' : '-' },
])

// 出站类型选项（value 用于存储/判断，label 用于显示）
const OUTBOUND_TYPES = [
  { value: 'direct', label: 'Direct' },
  { value: 'block', label: 'Block' },
  { value: 'dns', label: 'DNS' },
  { value: 'selector', label: 'Selector' },
  { value: 'urltest', label: 'URLTest' },
  { value: 'socks', label: 'SOCKS' },
  { value: 'http', label: 'HTTP' },
  { value: 'shadowsocks', label: 'Shadowsocks' },
  { value: 'shadowsocks2022', label: 'Shadowsocks2022' },
  { value: 'vmess', label: 'VMess' },
  { value: 'vless', label: 'VLESS' },
  { value: 'trojan', label: 'Trojan' },
  { value: 'hysteria', label: 'Hysteria' },
  { value: 'hysteria2', label: 'Hysteria2' },
  { value: 'tuic', label: 'TUIC' },
  { value: 'anytls', label: 'AnyTLS' },
  { value: 'shadowtls', label: 'ShadowTLS' },
  { value: 'snell', label: 'Snell' },
  { value: 'naive', label: 'NaiveProxy' },
  { value: 'bridge', label: 'Bridge' },
  { value: 'tor', label: 'Tor' },
  { value: 'ssh', label: 'SSH' },
]
const PROTOCOL_COMPS = {
  vmess: SingboxObVmess, vless: SingboxObVless, trojan: SingboxObTrojan,
  shadowsocks: SingboxObShadowsocks, shadowsocks2022: SingboxObShadowsocks2022,
  tuic: SingboxObTuic, hysteria2: SingboxObHysteria2, hysteria: SingboxObHysteria,
  http: SingboxObHttp, socks: SingboxObHttp, anytls: SingboxObAnyTls,
  shadowtls: SingboxObShadowtls, snell: SingboxObSnell,
  naive: SingboxObNaive, bridge: SingboxObBridge,
  selector: SingboxObGroup, urltest: SingboxObGroup,
  direct: SingboxObSimple, block: SingboxObSimple, dns: SingboxObSimple,
  tor: SingboxObTor, ssh: SingboxObSsh,
}
function getProtocolComp(t) { return PROTOCOL_COMPS[t] || null }

// 容器层通用 tab 支持矩阵
const TLS_PROTOCOLS = OUTBOUND_TLS_PROTOCOLS
const TRANS_PROTOCOLS = OUTBOUND_TRANSPORT_PROTOCOLS
const MUX_PROTOCOLS = OUTBOUND_MULTIPLEX_PROTOCOLS
function isServerType(t) { return OUTBOUND_SERVER_PROTOCOLS.includes(t) }

const dlg = ref(false)
const editIdx = ref(-1)
const rf = reactive({})
const activeTab = useQueryState('sbotab', 'basic')

function noop() {}
function showAdd() {
  editIdx.value = -1
  activeTab.value = 'basic'
  Object.assign(rf, outboundDefaultsForType('direct'))
  dlg.value = true
}
function showEdit(row) {
  editIdx.value = form.indexOf(row)
  activeTab.value = 'basic'
  Object.assign(rf, parseOutbound(JSON.parse(JSON.stringify(row))))
  dlg.value = true
}
function onTypeChange() {
  Object.assign(rf, outboundDefaultsForType(rf.type))
}
function save() {
  const cfg = buildOutbound(rf)
  if (!cfg.tag) { ElMessage.warning(t('singbox.outbound.tagRequired')); return }
  if (editIdx.value >= 0) form[editIdx.value] = cfg
  else form.push(cfg)
  emit()
  dlg.value = false
  ElMessage.success(t('singbox.outbound.saved'))
}
// 「保存」按钮：把当前出站列表（含 ↑↓ 排序、增删改的暂存结果）一次性落库。
// 之前 ↑↓ 自动保存过一版，用户反馈不要动一下就触发同步 —— 改为显式手动保存。
const savingOrder = ref(false)
async function persistOrder() {
  if (savingOrder.value) return
  savingOrder.value = true
  try {
    await client.put(`/node-servers/${props.serverId}/protocols/${props.coreType || 'singbox'}/section/outbounds`, form.map(buildOutbound))
    // sing-box 没有 xray 那样的出站 gRPC 热更，配置落库+同步文件后必须重启核心才生效；
    // 核心未运行时跳过（下次启动自然用新配置）
    let restarted = false
    try {
      const st = await client.get(`/node-servers/${props.serverId}/status`)
      const core = (st?.cores || []).find(x => x.core_type === (props.coreType || 'singbox'))
      if (core?.state === 'running') {
        await client.post(`/node-servers/${props.serverId}/control`, { core_type: props.coreType || 'singbox', action: 'restart' })
        restarted = true
      }
    } catch {}
    ElMessage.success(restarted ? t('singbox.outbound.savedRestarted') : t('singbox.outbound.saved'))
  } catch (e) {
    // 透出真实错误（网络/权限/参数），不要笼统的"操作失败"——曾经 PUT 根本没到后端却只报失败
    console.error('[outbounds] persist failed:', e)
    ElMessage.error(e?.response?.data?.message || e?.message || t('common.failed'))
    sync() // 失败回滚 UI
  } finally { savingOrder.value = false }
}

function handleAction(cmd, row) {
  if (cmd === 'edit') showEdit(row)
  else if (cmd === 'delete') {
    ElMessageBox.confirm(t('singbox.outbound.deleteConfirm', { tag: row.tag || row.type }), t('common.confirm'), { type: 'warning' })
      .then(() => { const i = form.indexOf(row); form.splice(i, 1); emit() })
      .catch(() => {})
  }
  // ↑↓ 只交换本地数组并 emit 暂存，点上方「保存」才落库同步
  else if (cmd === 'up') { const i = form.indexOf(row); if (i > 0) { [form[i-1], form[i]] = [form[i], form[i-1]]; emit() } }
  else if (cmd === 'down') { const i = form.indexOf(row); if (i < form.length - 1) { [form[i+1], form[i]] = [form[i], form[i+1]]; emit() } }
}
function sync() {
  form.splice(0, form.length)
  if (!Array.isArray(props.data)) return
  props.data.forEach(ob => form.push(parseOutbound(JSON.parse(JSON.stringify(ob)))))
}
function emit() { emitFn('update', form.map(buildOutbound)) }
onMounted(sync)
</script>