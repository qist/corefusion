<template>
  <div>
    <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:16px">
      <span style="font-size:15px;font-weight:600">Outbound List</span>
      <el-button size="small" type="primary" @click="showAdd">Add Outbound</el-button>
    </div>

    <el-table :data="obPaged" stripe style="width:100%" size="small">
      <el-table-column label="#" width="50" align="right">
        <template #default="{ $index }"><span style="font-size:11px;opacity:0.6">{{ (obPage - 1) * obSize + $index + 1 }}</span></template>
      </el-table-column>
      <el-table-column label="Actions" width="70" align="center">
        <template #default="{ row }">
          <el-dropdown trigger="click" @command="(cmd) => handleAction(cmd, row)">
            <el-icon style="font-size:18px;cursor:pointer"><MoreFilled /></el-icon>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="edit"><el-icon><Edit /></el-icon> Edit</el-dropdown-item>
                <el-dropdown-item command="up"><el-icon><Top /></el-icon> Move Up</el-dropdown-item>
                <el-dropdown-item command="down"><el-icon><Bottom /></el-icon> Move Down</el-dropdown-item>
                <el-dropdown-item command="delete" divided>
                  <span style="color:#f56c6c"><el-icon><Delete /></el-icon> Delete</span>
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </template>
      </el-table-column>
      <el-table-column label="Tag" min-width="120" show-overflow-tooltip>
        <template #default="{row}"><span style="font-weight:600">{{ row.tag || '-' }}</span></template>
      </el-table-column>
      <el-table-column label="Type" width="100" align="center">
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
    <PaginationBar v-model:page="obPage" v-model:page-size="obSize" :total="obTotal" />
    <div v-if="!form.length" style="text-align:center;color:#909399;padding:24px">No outbounds configured</div>

    <!-- Edit Dialog -->
    <el-dialog v-model="dlg" :title="editIdx >= 0 ? 'Edit Outbound' : 'Add Outbound'" width="760" :close-on-click-modal="false" top="5vh">
      <el-form label-width="150px" label-position="left">
        <el-form-item label="Type">
          <el-select v-model="rf.type" style="width:100%" @change="onTypeChange">
            <el-option v-for="t in OUTBOUND_TYPES" :key="t" :label="t" :value="t" />
          </el-select>
        </el-form-item>
      </el-form>
      <component
        v-if="getProtocolComp(rf.type)"
        :is="getProtocolComp(rf.type)"
        :key="rf.type"
        :model="rf"
        @update="noop"
      />
      <template #footer>
        <el-button @click="dlg = false">Cancel</el-button>
        <el-button type="primary" @click="save">Save</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { MoreFilled, Edit, Delete, Top, Bottom } from '@element-plus/icons-vue'
import { useClientPagination } from '../../../composables/useClientPagination'
import PaginationBar from '../../../components/PaginationBar.vue'
import { buildOutbound, parseOutbound, outboundDefaultsForType } from './singbox/utils/singboxBuildOutbound'
import SingboxObVless from './singbox/outbounds/SingboxObVless.vue'
import SingboxObVmess from './singbox/outbounds/SingboxObVmess.vue'
import SingboxObTrojan from './singbox/outbounds/SingboxObTrojan.vue'
import SingboxObShadowsocks from './singbox/outbounds/SingboxObShadowsocks.vue'
import SingboxObTUIC from './singbox/outbounds/SingboxObTUIC.vue'
import SingboxObHysteria2 from './singbox/outbounds/SingboxObHysteria2.vue'
import SingboxObHttp from './singbox/outbounds/SingboxObHttp.vue'
import SingboxObWireguard from './singbox/outbounds/SingboxObWireguard.vue'
import SingboxObGroup from './singbox/outbounds/SingboxObGroup.vue'
import SingboxObSimple from './singbox/outbounds/SingboxObSimple.vue'

const props = defineProps({ data: Array })
const emitFn = defineEmits(['update'])
const form = reactive([])
const { paged: obPaged, page: obPage, pageSize: obSize, total: obTotal } = useClientPagination(computed(() => form), { pageKey: 'sbObPage', pageSizeKey: 'sbObSize' })

const OUTBOUND_TYPES = ['direct', 'block', 'dns', 'selector', 'urltest', 'vmess', 'vless', 'trojan', 'hysteria2', 'shadowsocks', 'tuic', 'http', 'socks', 'wireguard']
const PROTOCOL_COMPS = {
  vmess: SingboxObVmess, vless: SingboxObVless, trojan: SingboxObTrojan,
  shadowsocks: SingboxObShadowsocks, tuic: SingboxObTUIC, hysteria2: SingboxObHysteria2,
  http: SingboxObHttp, socks: SingboxObHttp, wireguard: SingboxObWireguard,
  selector: SingboxObGroup, urltest: SingboxObGroup,
  direct: SingboxObSimple, block: SingboxObSimple, dns: SingboxObSimple,
}
// direct/block/dns 用 Simple 组件（仅 tag）
function getProtocolComp(t) { return PROTOCOL_COMPS[t] || null }

const dlg = ref(false)
const editIdx = ref(-1)
const rf = reactive({})

function noop() {}
function showAdd() { editIdx.value = -1; Object.assign(rf, outboundDefaultsForType('direct')); dlg.value = true }
function showEdit(row) {
  editIdx.value = form.indexOf(row)
  Object.assign(rf, parseOutbound(JSON.parse(JSON.stringify(row))))
  dlg.value = true
}
function onTypeChange() {
  Object.assign(rf, outboundDefaultsForType(rf.type))
}
function save() {
  const cfg = buildOutbound(rf)
  if (!cfg.tag) { ElMessage.warning('Tag is required'); return }
  if (editIdx.value >= 0) form[editIdx.value] = cfg
  else form.push(cfg)
  emit()
  dlg.value = false
  ElMessage.success('Saved')
}
function handleAction(cmd, row) {
  if (cmd === 'edit') showEdit(row)
  else if (cmd === 'delete') { const i = form.indexOf(row); form.splice(i, 1); emit() }
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