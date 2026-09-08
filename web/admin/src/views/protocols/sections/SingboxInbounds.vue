<template>
  <div>
    <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:16px">
      <span style="font-size:15px;font-weight:600">{{ t('singbox.inbound.title') }}</span>
      <el-button size="small" type="primary" @click="showAdd">{{ t('singbox.inbound.add') }}</el-button>
    </div>

    <!-- Table List -->
    <el-table :data="inbPaged" stripe style="width:100%" size="small">
      <el-table-column label="#" width="50" align="right">
        <template #default="{ row, $index }">
          <span style="font-size:11px;opacity:0.6">{{ $index + 1 }}</span>
        </template>
      </el-table-column>
      <el-table-column :label="t('singbox.inbound.actions')" width="80" align="center">
        <template #default="{ row }">
          <el-dropdown trigger="click" @command="(cmd) => handleAction(cmd, row)">
            <el-icon style="font-size:18px;cursor:pointer"><MoreFilled /></el-icon>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="edit"><el-icon><Edit /></el-icon> {{ t('common.edit') }}</el-dropdown-item>
                <el-dropdown-item command="share" divided><el-icon><Share /></el-icon> {{ t('singbox.inbound.shareCopied') }}</el-dropdown-item>
                <el-dropdown-item command="qrcode"><el-icon><View /></el-icon> {{ t('singbox.inbound.qrCode') }}</el-dropdown-item>
                <el-dropdown-item command="delete" divided>
                  <span style="color:#f56c6c"><el-icon><Delete /></el-icon> {{ t('common.delete') }}</span>
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </template>
      </el-table-column>
      <el-table-column :label="t('singbox.inbound.tag')" min-width="120" show-overflow-tooltip>
        <template #default="{row}"><span>{{ row.tag || '-' }}</span></template>
      </el-table-column>
      <el-table-column :label="t('singbox.inbound.port')" width="70" align="center">
        <template #default="{row}"><span style="font-weight:500">{{ row.listen_port }}</span></template>
      </el-table-column>
      <el-table-column :label="t('singbox.inbound.protocol')" min-width="170" align="left">
        <template #default="{row}">
          <div style="display:inline-flex;flex-wrap:wrap;gap:4px">
            <el-tag size="small" type="primary">{{ (row.type || '').toUpperCase() }}</el-tag>
            <el-tag v-if="row._transportType" size="small" type="success">{{ row._transportType }}</el-tag>
            <el-tag v-if="row._tlsEnabled" size="small" type="warning">TLS</el-tag>
            <el-tag v-if="row._realityEnabled" size="small" type="danger">Reality</el-tag>
          </div>
        </template>
      </el-table-column>
      <el-table-column :label="t('singbox.inbound.users')" width="80" align="center">
        <template #default="{row}">
          <el-tag v-if="row.users?.length" size="small">{{ row.users.length }}</el-tag>
          <span v-else style="color:#c0c4cc">-</span>
        </template>
      </el-table-column>
    </el-table>
    <PaginationBar v-model:page="inbPage" v-model:page-size="inbSize" :total="inbTotal" />
    <div v-if="!form.length" style="text-align:center;color:#909399;padding:24px">{{ t('singbox.inbound.noInbounds') }}</div>

    <!-- Edit Dialog: 容器层统一 el-tabs（对齐 xray 弹窗模型） -->
    <el-dialog v-model="dlg" :title="editIdx >= 0 ? t('singbox.inbound.edit') : t('singbox.inbound.add')" width="760" :close-on-click-modal="false" top="5vh">
      <el-tabs v-model="activeTab">
        <!-- Basic: 类型 + 监听通用参数 -->
        <el-tab-pane :label="t('singbox.inbound.tabs.basic')" name="basic">
          <el-form label-width="140px" label-position="left">
            <el-form-item :label="t('singbox.inbound.type')">
              <el-select v-model="rf.type" style="width:100%" @change="onTypeChange">
                <el-option v-for="t in INBOUND_TYPES" :key="t" :label="t" :value="t" />
              </el-select>
            </el-form-item>
          </el-form>
          <SingboxListen :model="rf" @update="noop" />
        </el-tab-pane>

        <!-- Protocol: 协议专属设置（仅当协议有专属参数时渲染） -->
        <el-tab-pane v-if="hasProtoTab(rf.type)" :label="t('singbox.inbound.tabs.protocol')" name="protocol">
          <component
            :is="getProtocolComp(rf.type)"
            :key="rf.type"
            :model="rf"
            :server-id="serverId"
            :access-host="accessHost"
            @update="noop"
          />
        </el-tab-pane>

        <!-- Users -->
        <el-tab-pane v-if="USER_PROTOCOLS.includes(rf.type)" :label="t('singbox.inbound.tabs.users')" name="users">
          <SingboxUsers :model="rf" :access-host="accessHost" @update="noop" />
        </el-tab-pane>

        <!-- Transport -->
        <el-tab-pane v-if="TRANSPORT_PROTOCOLS.includes(rf.type)" :label="t('singbox.inbound.tabs.transport')" name="transport">
          <SingboxTransport :model="rf" @update="noop" />
        </el-tab-pane>

        <!-- TLS -->
        <el-tab-pane v-if="TLS_ENABLED_PROTOCOLS.includes(rf.type)" :label="t('singbox.inbound.tabs.tls')" name="tls">
          <SingboxTls :model="rf" :server-id="serverId" @update="noop" />
        </el-tab-pane>

        <!-- Multiplex -->
        <el-tab-pane v-if="MULTIPLEX_PROTOCOLS.includes(rf.type)" :label="t('singbox.inbound.tabs.multiplex')" name="multiplex">
          <SingboxMultiplex :model="rf" @update="noop" />
        </el-tab-pane>
      </el-tabs>
      <template #footer>
        <el-button @click="dlg = false">{{ t('singbox.inbound.cancel') }}</el-button>
        <el-button type="primary" @click="save">{{ t('singbox.inbound.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- QR Dialog -->
    <el-dialog v-model="qrDlg" :title="t('singbox.inbound.qrCode')" width="400" top="10vh">
      <div style="text-align:center;padding:20px">
        <img v-if="qrCodeUrl" :src="qrCodeUrl" style="max-width:250px;margin-bottom:12px" />
        <el-input v-model="qrLink" type="textarea" :rows="3" readonly style="margin-top:8px" />
        <el-button size="small" style="margin-top:8px" @click="copyToClipboard(qrLink)">{{ t('singbox.inbound.copyLink') }}</el-button>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, ref, reactive, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { MoreFilled, Edit, Delete, View, Share } from '@element-plus/icons-vue'
const { t } = useI18n()
import { useClientPagination } from '../../../composables/useClientPagination'
import { useQueryState } from '../../../composables/useQueryState'
import PaginationBar from '../../../components/PaginationBar.vue'
import { generateQrDataUrl, isQrDataTooLargeError } from '../../../utils/qrcode'
import { buildInbound, parseInbound } from './singbox/utils/singboxBuild'
import {
  inboundDefaultsForType, INBOUND_TYPES,
  TLS_ENABLED_PROTOCOLS, TRANSPORT_PROTOCOLS, MULTIPLEX_PROTOCOLS, USER_PROTOCOLS,
} from './singbox/utils/singboxDefaults'
import { generateShareLink } from './singbox/utils/singboxShareLink'
import SingboxListen from './singbox/common/SingboxListen.vue'
import SingboxUsers from './singbox/common/SingboxUsers.vue'
import SingboxTransport from './singbox/common/SingboxTransport.vue'
import SingboxTls from './singbox/common/SingboxTls.vue'
import SingboxMultiplex from './singbox/common/SingboxMultiplex.vue'
// 仅含协议专属设置的协议组件（其余协议的全部参数都由通用 tab 提供，无需专属组件）
import SingboxShadowsocks from './singbox/inbounds/SingboxShadowsocks.vue'
import SingboxHysteria2 from './singbox/inbounds/SingboxHysteria2.vue'
import SingboxTuic from './singbox/inbounds/SingboxTuic.vue'
import SingboxShadowtls from './singbox/inbounds/SingboxShadowtls.vue'
import SingboxHysteria from './singbox/inbounds/SingboxHysteria.vue'
import SingboxWireguard from './singbox/inbounds/SingboxWireguard.vue'
import SingboxSnell from './singbox/inbounds/SingboxSnell.vue'

const props = defineProps({ data: Array, coreType: String, serverId: Number, serverInfo: { type: Object, default: () => ({}) } })
const emitFn = defineEmits(['update'])

const form = reactive([])
const { paged: inbPaged, page: inbPage, pageSize: inbSize, total: inbTotal } = useClientPagination(computed(() => form), { pageKey: 'sbInbPage', pageSizeKey: 'sbInbSize' })

const PROTOCOL_COMPS = {
  shadowsocks: SingboxShadowsocks, hysteria2: SingboxHysteria2, tuic: SingboxTuic,
  shadowtls: SingboxShadowtls, hysteria: SingboxHysteria, wireguard: SingboxWireguard, snell: SingboxSnell,
}
function getProtocolComp(t) { return PROTOCOL_COMPS[t] || null }

// 有协议专属 tab 的类型（其余参数全部由通用 tab 提供）
const PROTO_SPECIFIC_TYPES = ['shadowsocks', 'hysteria2', 'tuic', 'shadowtls', 'hysteria', 'wireguard', 'snell']
function hasProtoTab(t) { return PROTO_SPECIFIC_TYPES.includes(t) }

// 访问地址：优先使用 server 域名/IP（由父级传入 data 时可附带），无则用 listen
const accessHost = computed(() => props.serverInfo?.domain || props.serverInfo?.address || '')

const dlg = ref(false)
const editIdx = ref(-1)
const rf = reactive({})
const activeTab = useQueryState('sbitab', 'basic')

const qrDlg = ref(false)
const qrLink = ref('')
const qrCodeUrl = ref('')

function noop() {}

function showAdd() {
  editIdx.value = -1
  activeTab.value = 'basic'
  Object.assign(rf, inboundDefaultsForType('vless'))
  dlg.value = true
}

function showEdit(row) {
  editIdx.value = form.indexOf(row)
  activeTab.value = 'basic'
  Object.assign(rf, parseInbound(JSON.parse(JSON.stringify(row))))
  dlg.value = true
}

function onTypeChange() {
  Object.assign(rf, inboundDefaultsForType(rf.type))
}

function save() {
  const cfg = buildInbound(rf)
  if (!cfg.tag) { ElMessage.warning(t('singbox.inbound.tagRequired')); return }
  if (editIdx.value >= 0) form[editIdx.value] = cfg
  else form.push(cfg)
  emit()
  dlg.value = false
  ElMessage.success(t('singbox.inbound.saved'))
}

function handleAction(cmd, row) {
  if (cmd === 'edit') showEdit(row)
  else if (cmd === 'delete') {
    const i = form.indexOf(row)
    form.splice(i, 1)
    emit()
  } else if (cmd === 'share') { copyShare(row) }
  else if (cmd === 'qrcode') showQr(row)
}

function buildShareLink(obj) {
  const parsed = parseInbound(obj)
  return generateShareLink(parsed, parsed.users?.[0] || {}, accessHost.value || parsed.listen || '', parsed.listen_port || 0, parsed.tag)
}

function copyShare(row) {
  const link = buildShareLink(row)
  if (!link) { ElMessage.warning(t('singbox.inbound.shareNotSupported')); return }
  copyToClipboard(link)
  ElMessage.success(t('singbox.inbound.shareCopied'))
}

async function showQr(row) {
  const link = buildShareLink(row)
  if (!link) { ElMessage.warning(t('singbox.inbound.shareNotSupported')); return }
  qrLink.value = link
  qrDlg.value = true
  qrCodeUrl.value = ''
  try { qrCodeUrl.value = await generateQrDataUrl(link) }
  catch (e) {
    if (isQrDataTooLargeError(e)) ElMessage.warning(t('singbox.inbound.linkTooLong'))
    else ElMessage.error(t('singbox.inbound.qrFailed'))
  }
}

function copyToClipboard(text) {
  const ta = document.createElement('textarea')
  ta.value = text
  document.body.appendChild(ta)
  ta.select()
  try { document.execCommand('copy') } catch { /* ignore */ }
  document.body.removeChild(ta)
}

function sync() {
  form.splice(0, form.length)
  if (!Array.isArray(props.data)) return
  props.data.forEach(ib => form.push(parseInbound(JSON.parse(JSON.stringify(ib)))))
}
function emit() { emitFn('update', form.map(buildInbound)) }
onMounted(sync)
</script>