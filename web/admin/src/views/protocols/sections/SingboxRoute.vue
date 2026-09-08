<template>
  <div>
    <!-- Global Settings -->
    <el-divider content-position="left">Global Settings</el-divider>
    <el-form label-width="160px" label-position="left">
      <el-form-item label="Final Outbound">
        <el-select v-model="form.final" style="width:100%" filterable allow-create @change="emit">
          <el-option v-for="t in outboundTags" :key="t" :label="t" :value="t" />
        </el-select>
        <div class="hint">Default outbound when no rule matches</div>
      </el-form-item>
      <el-form-item label="Auto Detect Interface">
        <el-switch v-model="form.auto_detect_interface" @change="emit" />
      </el-form-item>
      <el-form-item label="Default Domain Resolver">
        <el-input v-model="form.default_domain_resolver" placeholder="dns-remote" @change="emit" />
        <div class="hint">Default DNS server tag for domain resolution</div>
      </el-form-item>
      <el-form-item label="Default Strategy">
        <el-select v-model="form.default_domain_strategy" style="width:100%" clearable @change="emit">
          <el-option label="" value="" />
          <el-option label="prefer_ipv4" value="prefer_ipv4" />
          <el-option label="prefer_ipv6" value="prefer_ipv6" />
          <el-option label="ipv4_only" value="ipv4_only" />
          <el-option label="ipv6_only" value="ipv6_only" />
        </el-select>
      </el-form-item>
    </el-form>

    <!-- Route Rules -->
    <el-divider content-position="left">
      Route Rules
      <el-button size="small" type="primary" @click="showAddRule" style="margin-left:12px">Add Rule</el-button>
    </el-divider>

    <div v-for="(r, i) in rulesPaged" :key="'rr'+i" class="rule-item">
      <div style="display:flex;justify-content:space-between;align-items:center">
        <div style="display:flex;align-items:center;gap:8px;flex-wrap:wrap">
          <el-tag v-if="r.action" size="small" :type="actionTagType(r.action)">{{ r.action }}</el-tag>
          <el-tag v-if="r.outbound" size="small" :type="r.outbound==='block'?'danger':r.outbound==='direct'?'success':'info'">{{ r.outbound }}</el-tag>
          <span v-if="r._domain" style="font-size:12px;color:#909399">domains: {{ r._domain }}</span>
          <span v-if="r._domainSuffix" style="font-size:12px;color:#909399">suffix: {{ r._domainSuffix }}</span>
          <span v-if="r._ipCIDR" style="font-size:12px;color:#909399">ip: {{ r._ipCIDR }}</span>
          <span v-if="r._protocol?.length" style="font-size:12px;color:#909399">{{ r._protocol.join(',') }}</span>
          <span v-if="r.port" style="font-size:12px;color:#909399">port: {{ r.port }}</span>
          <span v-if="r._inboundTag" style="font-size:12px;color:#909399">inbound: {{ r._inboundTag }}</span>
        </div>
        <div>
          <el-button size="small" @click="editRule(r)">Edit</el-button>
          <el-button size="small" type="danger" text @click="rmRule(r)">X</el-button>
        </div>
      </div>
    </div>
    <PaginationBar v-model:page="rulePage" v-model:page-size="ruleSize" :total="ruleTotal" />
    <div v-if="!form.rules.length" style="text-align:center;color:#909399;padding:24px">No rules</div>

    <!-- Rule Dialog -->
    <el-dialog v-model="dlg" :title="editIdx >= 0 ? 'Edit Route Rule' : 'Add Route Rule'" width="700">
      <el-form label-width="130px" label-position="left">
        <el-divider content-position="left">Match Conditions</el-divider>
        <el-form-item label="Type">
          <el-select v-model="rf.type" style="width:100%" clearable>
            <el-option label="default" value="" />
            <el-option label="logical" value="logical" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="rf.type === 'logical'" label="Mode">
          <el-select v-model="rf.mode" style="width:100%">
            <el-option label="and" value="and" />
            <el-option label="or" value="or" />
          </el-select>
        </el-form-item>
        <el-form-item label="Domain">
          <el-input v-model="rf._domain" type="textarea" :rows="2" placeholder="One domain per line" />
        </el-form-item>
        <el-form-item label="Domain Suffix">
          <el-input v-model="rf._domainSuffix" type="textarea" :rows="2" placeholder="One suffix per line" />
        </el-form-item>
        <el-form-item label="Domain Keyword">
          <el-input v-model="rf._domainKeyword" type="textarea" :rows="2" placeholder="One keyword per line" />
        </el-form-item>
        <el-form-item label="Domain Regex">
          <el-input v-model="rf._domainRegex" type="textarea" :rows="2" placeholder="One regex per line" />
        </el-form-item>
        <el-form-item label="IP CIDR">
          <el-input v-model="rf._ipCIDR" type="textarea" :rows="2" placeholder="One CIDR per line" />
        </el-form-item>
        <el-row :gutter="12">
          <el-col :span="8"><el-form-item label="Port"><el-input v-model="rf._port" placeholder="53,443,1000-2000" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="Source Port"><el-input v-model="rf._sourcePort" placeholder="53" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="Source IP CIDR"><el-input v-model="rf._sourceIPCIDR" placeholder="192.168.0.0/16" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="8"><el-form-item label="Network"><el-select v-model="rf._network" clearable style="width:100%"><el-option label="tcp" value="tcp" /><el-option label="udp" value="udp" /><el-option label="tcp,udp" value="tcp,udp" /></el-select></el-form-item></el-col>
          <el-col :span="16"><el-form-item label="Protocol"><el-select v-model="rf._protocol" multiple style="width:100%"><el-option label="http" value="http" /><el-option label="tls" value="tls" /><el-option label="quic" value="quic" /><el-option label="stun" value="stun" /><el-option label="dns" value="dns" /></el-select></el-form-item></el-col>
        </el-row>
        <el-form-item label="Inbound Tag">
          <el-select v-model="rf._inboundTag" multiple filterable allow-create default-first-option placeholder="Select or type" style="width:100%">
            <el-option v-for="tag in inboundTags" :key="tag" :label="tag" :value="tag" />
          </el-select>
        </el-form-item>
        <el-form-item label="Process">
          <el-input v-model="rf._process" placeholder="One process name per line" type="textarea" :rows="2" />
        </el-form-item>

        <el-divider content-position="left">Action</el-divider>
        <el-form-item label="Action">
          <el-select v-model="rf.action" style="width:100%" clearable>
            <el-option label="route (default)" value="" />
            <el-option label="route-options" value="route-options" />
            <el-option label="reject" value="reject" />
            <el-option label="hijack-dns" value="hijack-dns" />
            <el-option label="sniff" value="sniff" />
            <el-option label="resolve" value="resolve" />
          </el-select>
        </el-form-item>
        <el-form-item label="Outbound">
          <el-select v-model="rf.outbound" style="width:100%" clearable filterable allow-create>
            <el-option v-for="t in outboundTags" :key="t" :label="t" :value="t" />
          </el-select>
          <div class="hint">Outbound tag for matched traffic</div>
        </el-form-item>
        <el-form-item label="Domain Resolver">
          <el-input v-model="rf.domain_resolver" placeholder="DNS server tag" />
        </el-form-item>
        <el-form-item label="Strategy">
          <el-select v-model="rf.strategy" style="width:100%" clearable>
            <el-option label="" value="" />
            <el-option label="prefer_ipv4" value="prefer_ipv4" />
            <el-option label="prefer_ipv6" value="prefer_ipv6" />
            <el-option label="ipv4_only" value="ipv4_only" />
            <el-option label="ipv6_only" value="ipv6_only" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dlg=false">Cancel</el-button>
        <el-button type="primary" @click="saveRule">Save</el-button>
      </template>
    </el-dialog>
  </div>
</template>
<script setup>
import { computed, reactive, ref, onMounted } from 'vue'
import client from '../../../api/client'
import { useClientPagination } from '../../../composables/useClientPagination'
import PaginationBar from '../../../components/PaginationBar.vue'
const props = defineProps({ data: Object, coreType: String, serverId: Number })
const emitFn = defineEmits(['update'])

const form = reactive({
  final: 'direct', auto_detect_interface: true,
  default_domain_resolver: '', default_domain_strategy: '',
  rules: [],
})

const { paged: rulesPaged, page: rulePage, pageSize: ruleSize, total: ruleTotal } = useClientPagination(computed(() => form.rules), { pageKey: 'sbRoutePage', pageSizeKey: 'sbRouteSize' })
const outboundTags = ref(['direct', 'block'])
const inboundTags = ref([])

const dlg = ref(false)
const editIdx = ref(-1)
const rf = reactive({
  type: '', mode: '',
  _domain: '', _domainSuffix: '', _domainKeyword: '', _domainRegex: '',
  _ipCIDR: '', _sourceIPCIDR: '', _port: '', _sourcePort: '',
  _network: '', _protocol: [], _inboundTag: [], _process: '',
  action: '', outbound: '', domain_resolver: '', strategy: '',
})

function actionTagType(a) {
  if (a === 'reject') return 'danger'
  if (a === 'hijack-dns') return 'warning'
  if (a === 'sniff') return 'success'
  return 'info'
}

function resetForm() {
  Object.assign(rf, {
    type: '', mode: '',
    _domain: '', _domainSuffix: '', _domainKeyword: '', _domainRegex: '',
    _ipCIDR: '', _sourceIPCIDR: '', _port: '', _sourcePort: '',
    _network: '', _protocol: [], _inboundTag: [], _process: '',
    action: '', outbound: '', domain_resolver: '', strategy: '',
  })
}

function showAddRule() { editIdx.value = -1; resetForm(); dlg.value = true }

function editRule(r) {
  editIdx.value = form.rules.indexOf(r)
  Object.assign(rf, {
    type: r.type || '', mode: r.mode || '',
    _domain: r._domain || '', _domainSuffix: r._domainSuffix || '',
    _domainKeyword: r._domainKeyword || '', _domainRegex: r._domainRegex || '',
    _ipCIDR: r._ipCIDR || '', _sourceIPCIDR: r._sourceIPCIDR || '',
    _port: r.port || '', _sourcePort: r.source_port || '',
    _network: r.network || '', _protocol: r.protocol || [],
    _inboundTag: r.inbound_tag || [], _process: (r.process || []).join('\n'),
    action: r.action || '', outbound: r.outbound || '',
    domain_resolver: r.domain_resolver || '', strategy: r.strategy || '',
  })
  dlg.value = true
}

function saveRule() {
  const rule = {}
  if (rf.type) rule.type = rf.type
  if (rf.type === 'logical' && rf.mode) rule.mode = rf.mode
  const domains = rf._domain ? rf._domain.split('\n').map(d => d.trim()).filter(Boolean) : []
  if (domains.length) rule.domain = domains
  const suffixes = rf._domainSuffix ? rf._domainSuffix.split('\n').map(d => d.trim()).filter(Boolean) : []
  if (suffixes.length) rule.domain_suffix = suffixes
  const keywords = rf._domainKeyword ? rf._domainKeyword.split('\n').map(d => d.trim()).filter(Boolean) : []
  if (keywords.length) rule.domain_keyword = keywords
  const regexes = rf._domainRegex ? rf._domainRegex.split('\n').map(d => d.trim()).filter(Boolean) : []
  if (regexes.length) rule.domain_regex = regexes
  const cidrs = rf._ipCIDR ? rf._ipCIDR.split('\n').map(d => d.trim()).filter(Boolean) : []
  if (cidrs.length) rule.ip_cidr = cidrs
  const srcCidrs = rf._sourceIPCIDR ? rf._sourceIPCIDR.split('\n').map(d => d.trim()).filter(Boolean) : []
  if (srcCidrs.length) rule.source_ip_cidr = srcCidrs
  if (rf._port) rule.port = rf._port
  if (rf._sourcePort) rule.source_port = rf._sourcePort
  if (rf._network) rule.network = rf._network
  if (rf._protocol?.length) rule.protocol = [...rf._protocol]
  if (rf._inboundTag?.length) rule.inbound_tag = [...rf._inboundTag]
  const procs = rf._process ? rf._process.split('\n').map(d => d.trim()).filter(Boolean) : []
  if (procs.length) rule.process = procs
  if (rf.action) rule.action = rf.action
  if (rf.outbound) rule.outbound = rf.outbound
  if (rf.domain_resolver) rule.domain_resolver = rf.domain_resolver
  if (rf.strategy) rule.strategy = rf.strategy
  rule._domain = rf._domain
  rule._domainSuffix = rf._domainSuffix
  rule._domainKeyword = rf._domainKeyword
  rule._domainRegex = rf._domainRegex
  rule._ipCIDR = rf._ipCIDR
  rule._sourceIPCIDR = rf._sourceIPCIDR
  if (rf._port) rule.port = rf._port
  if (rf._sourcePort) rule.source_port = rf._sourcePort
  if (rf._network) rule.network = rf._network
  if (rf._protocol?.length) rule.protocol = [...rf._protocol]
  if (rf._inboundTag?.length) rule.inbound_tag = [...rf._inboundTag]
  if (procs.length) rule.process = procs

  if (editIdx.value >= 0) form.rules[editIdx.value] = rule
  else form.rules.push(rule)
  dlg.value = false
  emit()
}

function rmRule(r) { const i = form.rules.indexOf(r); form.rules.splice(i, 1); emit() }

async function loadTags() {
  try {
    const res = await client.get(`/node-servers/${props.serverId}/outbound-configs`, { params: { core_type: props.coreType || 'singbox' } })
    const obs = (res || []).filter(o => o?.enabled !== false)
    outboundTags.value = [...new Set(['direct', 'block', ...obs.map(o => o.tag).filter(Boolean)])]
  } catch {}
  try {
    const ibs = await client.get(`/node-servers/${props.serverId}/inbound-configs`, { params: { core_type: props.coreType || 'singbox' } })
    inboundTags.value = (ibs || []).filter(i => i?.enabled !== false).map(i => i.tag).filter(Boolean)
  } catch {}
}

function sync() {
  if (!props.data) return
  form.final = props.data.final || 'direct'
  form.auto_detect_interface = props.data.auto_detect_interface !== false
  form.default_domain_resolver = props.data.default_domain_resolver || ''
  form.default_domain_strategy = props.data.default_domain_strategy || ''
  form.rules = (props.data.rules || []).map(r => ({
    ...r,
    _domain: (r.domain || []).join('\n'),
    _domainSuffix: (r.domain_suffix || []).join('\n'),
    _domainKeyword: (r.domain_keyword || []).join('\n'),
    _domainRegex: (r.domain_regex || []).join('\n'),
    _ipCIDR: (r.ip_cidr || []).join('\n'),
    _sourceIPCIDR: (r.source_ip_cidr || []).join('\n'),
    _protocol: r.protocol || [],
    _inboundTag: r.inbound_tag || [],
  }))
}

function build() {
  const out = {
    final: form.final,
    auto_detect_interface: form.auto_detect_interface,
  }
  if (form.default_domain_resolver) out.default_domain_resolver = form.default_domain_resolver
  if (form.default_domain_strategy) out.default_domain_strategy = form.default_domain_strategy
  out.rules = form.rules.map(r => {
    const rule = {}
    if (r.type) rule.type = r.type
    if (r.type === 'logical' && r.mode) rule.mode = r.mode
    const domains = r._domain ? r._domain.split('\n').map(d => d.trim()).filter(Boolean) : []
    if (domains.length) rule.domain = domains
    const suffixes = r._domainSuffix ? r._domainSuffix.split('\n').map(d => d.trim()).filter(Boolean) : []
    if (suffixes.length) rule.domain_suffix = suffixes
    const keywords = r._domainKeyword ? r._domainKeyword.split('\n').map(d => d.trim()).filter(Boolean) : []
    if (keywords.length) rule.domain_keyword = keywords
    const regexes = r._domainRegex ? r._domainRegex.split('\n').map(d => d.trim()).filter(Boolean) : []
    if (regexes.length) rule.domain_regex = regexes
    const cidrs = r._ipCIDR ? r._ipCIDR.split('\n').map(d => d.trim()).filter(Boolean) : []
    if (cidrs.length) rule.ip_cidr = cidrs
    const srcCidrs = r._sourceIPCIDR ? r._sourceIPCIDR.split('\n').map(d => d.trim()).filter(Boolean) : []
    if (srcCidrs.length) rule.source_ip_cidr = srcCidrs
    if (r.port) rule.port = r.port
    if (r.source_port) rule.source_port = r.source_port
    if (r.network) rule.network = r.network
    if (r.protocol?.length) rule.protocol = [...r.protocol]
    if (r.inbound_tag?.length) rule.inbound_tag = [...r.inbound_tag]
    if (r.process?.length) rule.process = [...r.process]
    if (r.action) rule.action = r.action
    if (r.outbound) rule.outbound = r.outbound
    if (r.domain_resolver) rule.domain_resolver = r.domain_resolver
    if (r.strategy) rule.strategy = r.strategy
    return rule
  })
  return out
}

function emit() { emitFn('update', build()) }
onMounted(function() { sync(); loadTags() })
</script>
<style scoped>
.rule-item { background: var(--el-fill-color-light); border-radius: 6px; padding: 12px; margin-bottom: 8px; }
.hint { font-size: 12px; color: var(--el-text-color-secondary); margin-top: 2px; }
</style>
