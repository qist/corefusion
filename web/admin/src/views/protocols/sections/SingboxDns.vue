<template>
  <div>
    <el-form label-width="160px" label-position="left">
      <el-form-item label="Enable DNS">
        <el-switch v-model="enabled" @change="onToggle" />
      </el-form-item>
    </el-form>

    <template v-if="enabled">
      <!-- Global Settings -->
      <el-divider content-position="left">Global Settings</el-divider>
      <el-form label-width="160px" label-position="left">
        <el-form-item label="Strategy">
          <el-select v-model="form.strategy" style="width:100%" clearable @change="emit">
            <el-option label="" value="" />
            <el-option label="prefer_ipv4" value="prefer_ipv4" />
            <el-option label="prefer_ipv6" value="prefer_ipv6" />
            <el-option label="ipv4_only" value="ipv4_only" />
            <el-option label="ipv6_only" value="ipv6_only" />
          </el-select>
          <div class="hint">DNS query strategy for IP version preference</div>
        </el-form-item>
        <el-form-item label="Disable Cache">
          <el-switch v-model="form.disable_cache" @change="emit" />
        </el-form-item>
        <el-form-item label="Disable Expire">
          <el-switch v-model="form.disable_expire" @change="emit" />
          <div class="hint">Disable cache expiration</div>
        </el-form-item>
        <el-form-item label="Independent Cache">
          <el-switch v-model="form.independent_cache" @change="emit" />
          <div class="hint">Each DNS server uses an independent cache</div>
        </el-form-item>
        <el-form-item label="Reverse Mapping">
          <el-switch v-model="form.reverse_mapping" @change="emit" />
          <div class="hint">Store reverse DNS mapping for IPs</div>
        </el-form-item>
        <el-form-item label="Client Subnet">
          <el-input v-model="form.client_subnet" placeholder="0.0.0.0/24" @change="emit" />
          <div class="hint">EDNS0 client subnet</div>
        </el-form-item>
        <el-form-item label="Final Server">
          <el-input v-model="form.final" placeholder="dns-remote" @change="emit" />
          <div class="hint">Tag of the final DNS server used as fallback</div>
        </el-form-item>
        <el-form-item label="Domain Matcher">
          <el-select v-model="form.domain_matcher" style="width:100%" clearable @change="emit">
            <el-option label="" value="" />
            <el-option label="adguard" value="adguard" />
            <el-option label="compression" value="compression" />
            <el-option label="full" value="full" />
            <el-option label="hybrid" value="hybrid" />
          </el-select>
        </el-form-item>
      </el-form>

      <!-- FakeIP -->
      <el-divider content-position="left">FakeIP</el-divider>
      <el-form label-width="160px" label-position="left">
        <el-form-item label="Enabled">
          <el-switch v-model="form._fakeipEnabled" @change="emit" />
        </el-form-item>
        <template v-if="form._fakeipEnabled">
          <el-form-item label="IPv4 Range">
            <el-input v-model="form._fakeipInet4Range" placeholder="198.18.0.0/15" @change="emit" />
          </el-form-item>
          <el-form-item label="IPv6 Range">
            <el-input v-model="form._fakeipInet6Range" placeholder="fc00::/18" @change="emit" />
          </el-form-item>
          <el-form-item label="Exclude">
            <el-input v-model="form._fakeipExcludeStr" type="textarea" :rows="2" placeholder="One domain per line" @change="emit" />
            <div class="hint">Domains excluded from FakeIP</div>
          </el-form-item>
          <el-form-item label="Inet4 Bind">
            <el-input v-model="form._fakeipInet4Bind" placeholder="198.18.0.0/15" @change="emit" />
          </el-form-item>
          <el-form-item label="Inet6 Bind">
            <el-input v-model="form._fakeipInet6Bind" placeholder="fc00::/18" @change="emit" />
          </el-form-item>
        </template>
      </el-form>

      <!-- DNS Servers -->
      <el-divider content-position="left">
        DNS Servers
        <el-button size="small" type="primary" @click="showAddServer" style="margin-left:12px">Add Server</el-button>
      </el-divider>
      <div class="hint" style="margin-bottom:8px">DNS servers used for resolution</div>
      <div v-for="(s, i) in form.servers" :key="'srv'+i" class="rule-item">
        <div style="display:flex;justify-content:space-between;align-items:center">
          <div style="display:flex;align-items:center;gap:8px;flex-wrap:wrap">
            <el-tag size="small">{{ s.type || 'udp' }}</el-tag>
            <span style="font-weight:600;font-size:13px">{{ s.tag || 'unnamed' }}</span>
            <span v-if="s.address" style="font-size:12px;color:#909399">{{ s.address }}</span>
            <el-tag v-if="s.detour" size="small" type="info">{{ s.detour }}</el-tag>
          </div>
          <div>
            <el-button size="small" @click="editServer(i)">Edit</el-button>
            <el-button size="small" type="danger" text @click="rmServer(i)">X</el-button>
          </div>
        </div>
      </div>

      <!-- DNS Rules -->
      <el-divider content-position="left">
        DNS Rules
        <el-button size="small" type="primary" @click="showAddRule" style="margin-left:12px">Add Rule</el-button>
      </el-divider>
      <div class="hint" style="margin-bottom:8px">Route DNS queries to specific servers based on conditions</div>
      <div v-for="(r, i) in form.rules" :key="'rul'+i" class="rule-item">
        <div style="display:flex;justify-content:space-between;align-items:center">
          <div style="display:flex;align-items:center;gap:8px;flex-wrap:wrap">
            <el-tag v-if="r.type" size="small" type="warning">{{ r.type }}</el-tag>
            <el-tag v-if="r.server" size="small">{{ r.server }}</el-tag>
            <el-tag v-if="r.outbound" size="small" type="info">{{ r.outbound }}</el-tag>
            <span v-if="r._domain" style="font-size:12px;color:#909399">domains: {{ r._domain }}</span>
            <span v-if="r._domainSuffix" style="font-size:12px;color:#909399">suffix: {{ r._domainSuffix }}</span>
            <span v-if="r._ipCIDR" style="font-size:12px;color:#909399">ip: {{ r._ipCIDR }}</span>
            <span v-if="r._protocol?.length" style="font-size:12px;color:#909399">{{ r._protocol.join(',') }}</span>
          </div>
          <div>
            <el-button size="small" @click="editRule(i)">Edit</el-button>
            <el-button size="small" type="danger" text @click="rmRule(i)">X</el-button>
          </div>
        </div>
      </div>

      <!-- Static Hosts -->
      <el-divider content-position="left">
        Static Hosts
        <el-button size="small" type="primary" @click="addHost" style="margin-left:12px">Add Host</el-button>
      </el-divider>
      <div class="hint" style="margin-bottom:8px">Static DNS records (domain → IP)</div>
      <div v-for="(h, i) in form.hostsList" :key="'h'+i" class="sub-item">
        <el-input v-model="h.domain" placeholder="domain:example.com" style="width:40%" @change="emit" />
        <el-input v-model="h.ip" placeholder="1.2.3.4" style="width:40%" @change="emit" />
        <el-button size="small" type="danger" text @click="rmHost(i)">X</el-button>
      </div>
    </template>

    <!-- Server Edit Dialog -->
    <el-dialog v-model="serverDlg" :title="editingServerIdx >= 0 ? 'Edit DNS Server' : 'Add DNS Server'" width="550">
      <el-form label-width="140px" label-position="left">
        <el-form-item label="Type">
          <el-select v-model="srvForm.type" style="width:100%">
            <el-option label="udp" value="udp" />
            <el-option label="tcp" value="tcp" />
            <el-option label="tls" value="tls" />
            <el-option label="https" value="https" />
            <el-option label="quic" value="quic" />
            <el-option label="h3" value="h3" />
            <el-option label="local" value="local" />
            <el-option label="dhcp" value="dhcp" />
            <el-option label="predefined" value="predefined" />
          </el-select>
        </el-form-item>
        <el-form-item label="Tag">
          <el-input v-model="srvForm.tag" placeholder="dns-google" />
        </el-form-item>
        <el-form-item label="Address">
          <el-input v-model="srvForm.address" placeholder="8.8.8.8 / https://dns.google/dns-query / eth0" />
        </el-form-item>
        <el-form-item label="Detour">
          <el-input v-model="srvForm.detour" placeholder="Outbound tag (optional)" />
        </el-form-item>
        <el-form-item label="Server Port">
          <el-input-number v-model="srvForm.server_port" :min="0" :max="65535" placeholder="53" />
        </el-form-item>
        <el-form-item label="Strategy">
          <el-select v-model="srvForm.strategy" style="width:100%" clearable>
            <el-option label="" value="" />
            <el-option label="prefer_ipv4" value="prefer_ipv4" />
            <el-option label="prefer_ipv6" value="prefer_ipv6" />
            <el-option label="ipv4_only" value="ipv4_only" />
            <el-option label="ipv6_only" value="ipv6_only" />
          </el-select>
        </el-form-item>
        <el-form-item label="Disable Cache">
          <el-switch v-model="srvForm.disable_cache" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="serverDlg=false">Cancel</el-button>
        <el-button type="primary" @click="saveServer">Save</el-button>
      </template>
    </el-dialog>

    <!-- Rule Edit Dialog -->
    <el-dialog v-model="ruleDlg" :title="editingRuleIdx >= 0 ? 'Edit DNS Rule' : 'Add DNS Rule'" width="600">
      <el-form label-width="130px" label-position="left">
        <el-form-item label="Type">
          <el-select v-model="ruleForm.type" style="width:100%" clearable>
            <el-option label="default" value="" />
            <el-option label="logical" value="logical" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="ruleForm.type === 'logical'" label="Mode">
          <el-select v-model="ruleForm.mode" style="width:100%">
            <el-option label="and" value="and" />
            <el-option label="or" value="or" />
          </el-select>
        </el-form-item>
        <el-form-item label="Domain">
          <el-input v-model="ruleForm._domain" type="textarea" :rows="2" placeholder="One domain per line" />
        </el-form-item>
        <el-form-item label="Domain Suffix">
          <el-input v-model="ruleForm._domainSuffix" type="textarea" :rows="2" placeholder="One suffix per line" />
        </el-form-item>
        <el-form-item label="Domain Keyword">
          <el-input v-model="ruleForm._domainKeyword" type="textarea" :rows="2" placeholder="One keyword per line" />
        </el-form-item>
        <el-form-item label="IP CIDR">
          <el-input v-model="ruleForm._ipCIDR" type="textarea" :rows="2" placeholder="One CIDR per line" />
        </el-form-item>
        <el-form-item label="Protocol">
          <el-select v-model="ruleForm._protocol" multiple style="width:100%">
            <el-option label="http" value="http" />
            <el-option label="tls" value="tls" />
            <el-option label="quic" value="quic" />
            <el-option label="stun" value="stun" />
            <el-option label="dns" value="dns" />
          </el-select>
        </el-form-item>
        <el-form-item label="Server">
          <el-input v-model="ruleForm.server" placeholder="dns-server tag" />
        </el-form-item>
        <el-form-item label="Outbound">
          <el-input v-model="ruleForm.outbound" placeholder="Outbound tag (optional)" />
        </el-form-item>
        <el-form-item label="Action">
          <el-select v-model="ruleForm.action" style="width:100%" clearable>
            <el-option label="" value="" />
            <el-option label="route" value="route" />
            <el-option label="route-options" value="route-options" />
            <el-option label="reject" value="reject" />
            <el-option label="hijack-dns" value="hijack-dns" />
            <el-option label="sniff" value="sniff" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="ruleDlg=false">Cancel</el-button>
        <el-button type="primary" @click="saveRule">Save</el-button>
      </template>
    </el-dialog>
  </div>
</template>
<script setup>
import { reactive, ref, onMounted } from 'vue'
const props = defineProps({ data: Object })
const emitFn = defineEmits(['update'])
const enabled = ref(true)
const form = reactive({
  strategy: '', disable_cache: false, disable_expire: false,
  independent_cache: false, reverse_mapping: false,
  client_subnet: '', final: '', domain_matcher: '',
  _fakeipEnabled: false, _fakeipInet4Range: '', _fakeipInet6Range: '',
  _fakeipExcludeStr: '', _fakeipInet4Bind: '', _fakeipInet6Bind: '',
  servers: [], rules: [], hostsList: [],
})

const serverDlg = ref(false)
const editingServerIdx = ref(-1)
const srvForm = reactive({ type: 'udp', tag: '', address: '', detour: '', server_port: 0, strategy: '', disable_cache: false })

const ruleDlg = ref(false)
const editingRuleIdx = ref(-1)
const ruleForm = reactive({
  type: '', mode: '', _domain: '', _domainSuffix: '', _domainKeyword: '',
  _ipCIDR: '', _protocol: [], server: '', outbound: '', action: '',
})

function showAddServer() {
  editingServerIdx.value = -1
  Object.assign(srvForm, { type: 'udp', tag: '', address: '', detour: '', server_port: 0, strategy: '', disable_cache: false })
  serverDlg.value = true
}
function editServer(i) {
  editingServerIdx.value = i
  const s = form.servers[i]
  Object.assign(srvForm, { ...s })
  serverDlg.value = true
}
function saveServer() {
  const srv = { type: srvForm.type, tag: srvForm.tag }
  if (srvForm.address) srv.address = srvForm.address
  if (srvForm.detour) srv.detour = srvForm.detour
  if (srvForm.server_port) srv.server_port = srvForm.server_port
  if (srvForm.strategy) srv.strategy = srvForm.strategy
  if (srvForm.disable_cache) srv.disable_cache = true
  if (editingServerIdx.value >= 0) form.servers[editingServerIdx.value] = srv
  else form.servers.push(srv)
  serverDlg.value = false
  emit()
}
function rmServer(i) { form.servers.splice(i, 1); emit() }

function showAddRule() {
  editingRuleIdx.value = -1
  Object.assign(ruleForm, {
    type: '', mode: '', _domain: '', _domainSuffix: '', _domainKeyword: '',
    _ipCIDR: '', _protocol: [], server: '', outbound: '', action: '',
  })
  ruleDlg.value = true
}
function editRule(i) {
  editingRuleIdx.value = i
  const r = form.rules[i]
  Object.assign(ruleForm, {
    ...r,
    _domain: r._domain || '',
    _domainSuffix: r._domainSuffix || '',
    _domainKeyword: r._domainKeyword || '',
    _ipCIDR: r._ipCIDR || '',
    _protocol: r._protocol || [],
  })
  ruleDlg.value = true
}
function saveRule() {
  const rule = {}
  if (ruleForm.type) rule.type = ruleForm.type
  if (ruleForm.type === 'logical' && ruleForm.mode) rule.mode = ruleForm.mode
  const domains = ruleForm._domain ? ruleForm._domain.split('\n').map(d => d.trim()).filter(Boolean) : []
  if (domains.length) rule.domain = domains
  const suffixes = ruleForm._domainSuffix ? ruleForm._domainSuffix.split('\n').map(d => d.trim()).filter(Boolean) : []
  if (suffixes.length) rule.domain_suffix = suffixes
  const keywords = ruleForm._domainKeyword ? ruleForm._domainKeyword.split('\n').map(d => d.trim()).filter(Boolean) : []
  if (keywords.length) rule.domain_keyword = keywords
  const cidrs = ruleForm._ipCIDR ? ruleForm._ipCIDR.split('\n').map(d => d.trim()).filter(Boolean) : []
  if (cidrs.length) rule.ip_cidr = cidrs
  if (ruleForm._protocol?.length) rule.protocol = [...ruleForm._protocol]
  if (ruleForm.server) rule.server = ruleForm.server
  if (ruleForm.outbound) rule.outbound = ruleForm.outbound
  if (ruleForm.action) rule.action = ruleForm.action
  if (editingRuleIdx.value >= 0) form.rules[editingRuleIdx.value] = { ...rule, _domain: ruleForm._domain, _domainSuffix: ruleForm._domainSuffix, _domainKeyword: ruleForm._domainKeyword, _ipCIDR: ruleForm._ipCIDR, _protocol: ruleForm._protocol }
  else form.rules.push({ ...rule, _domain: ruleForm._domain, _domainSuffix: ruleForm._domainSuffix, _domainKeyword: ruleForm._domainKeyword, _ipCIDR: ruleForm._ipCIDR, _protocol: ruleForm._protocol })
  ruleDlg.value = false
  emit()
}
function rmRule(i) { form.rules.splice(i, 1); emit() }

function addHost() { form.hostsList.push({ domain: '', ip: '' }); emit() }
function rmHost(i) { form.hostsList.splice(i, 1); emit() }

function sync() {
  if (!props.data || Object.keys(props.data).length === 0) { enabled.value = false; return }
  if (props.data._disabled) { enabled.value = false; return }
  enabled.value = true
  form.strategy = props.data.strategy || ''
  form.disable_cache = props.data.disable_cache || false
  form.disable_expire = props.data.disable_expire || false
  form.independent_cache = props.data.independent_cache || false
  form.reverse_mapping = props.data.reverse_mapping || false
  form.client_subnet = props.data.client_subnet || ''
  form.final = props.data.final || ''
  form.domain_matcher = props.data.domain_matcher || ''
  form._fakeipEnabled = props.data.fakeip?.enabled || false
  form._fakeipInet4Range = props.data.fakeip?.inet4_range || '198.18.0.0/15'
  form._fakeipInet6Range = props.data.fakeip?.inet6_range || 'fc00::/18'
  form._fakeipExcludeStr = (props.data.fakeip?.exclude || []).join('\n')
  form._fakeipInet4Bind = props.data.fakeip?.inet4_bind_address || ''
  form._fakeipInet6Bind = props.data.fakeip?.inet6_bind_address || ''
  form.servers = (props.data.servers || []).map(s => ({ ...s }))
  form.rules = (props.data.rules || []).map(r => ({
    ...r,
    _domain: (r.domain || []).join('\n'),
    _domainSuffix: (r.domain_suffix || []).join('\n'),
    _domainKeyword: (r.domain_keyword || []).join('\n'),
    _ipCIDR: (r.ip_cidr || []).join('\n'),
    _protocol: r.protocol || [],
  }))
  const hosts = props.data.hosts || {}
  form.hostsList = Object.entries(hosts).map(([domain, ip]) => ({ domain, ip: Array.isArray(ip) ? ip.join(', ') : ip }))
}

function build() {
  const out = {
    strategy: form.strategy || undefined,
    disable_cache: form.disable_cache || undefined,
    disable_expire: form.disable_expire || undefined,
    independent_cache: form.independent_cache || undefined,
    reverse_mapping: form.reverse_mapping || undefined,
    client_subnet: form.client_subnet || undefined,
    final: form.final || undefined,
    domain_matcher: form.domain_matcher || undefined,
  }
  if (form._fakeipEnabled) {
    const fakeip = { enabled: true }
    if (form._fakeipInet4Range) fakeip.inet4_range = form._fakeipInet4Range
    if (form._fakeipInet6Range) fakeip.inet6_range = form._fakeipInet6Range
    const exclude = form._fakeipExcludeStr ? form._fakeipExcludeStr.split('\n').map(s => s.trim()).filter(Boolean) : []
    if (exclude.length) fakeip.exclude = exclude
    if (form._fakeipInet4Bind) fakeip.inet4_bind_address = form._fakeipInet4Bind
    if (form._fakeipInet6Bind) fakeip.inet6_bind_address = form._fakeipInet6Bind
    out.fakeip = fakeip
  }
  out.servers = form.servers.filter(s => s.tag).map(s => {
    const srv = { tag: s.tag }
    if (s.type) srv.type = s.type
    if (s.address) srv.address = s.address
    if (s.detour) srv.detour = s.detour
    if (s.server_port) srv.server_port = s.server_port
    if (s.strategy) srv.strategy = s.strategy
    if (s.disable_cache) srv.disable_cache = true
    return srv
  })
  out.rules = form.rules.filter(r => r.server || r.outbound || r.action).map(r => {
    const rule = {}
    if (r.type) rule.type = r.type
    if (r.type === 'logical' && r.mode) rule.mode = r.mode
    const domains = r._domain ? r._domain.split('\n').map(d => d.trim()).filter(Boolean) : []
    if (domains.length) rule.domain = domains
    const suffixes = r._domainSuffix ? r._domainSuffix.split('\n').map(d => d.trim()).filter(Boolean) : []
    if (suffixes.length) rule.domain_suffix = suffixes
    const keywords = r._domainKeyword ? r._domainKeyword.split('\n').map(d => d.trim()).filter(Boolean) : []
    if (keywords.length) rule.domain_keyword = keywords
    const cidrs = r._ipCIDR ? r._ipCIDR.split('\n').map(d => d.trim()).filter(Boolean) : []
    if (cidrs.length) rule.ip_cidr = cidrs
    if (r._protocol?.length) rule.protocol = [...r._protocol]
    if (r.server) rule.server = r.server
    if (r.outbound) rule.outbound = r.outbound
    if (r.action) rule.action = r.action
    return rule
  })
  const hosts = {}
  form.hostsList.forEach(h => {
    if (!h.domain) return
    const ips = h.ip.split(',').map(s => s.trim()).filter(Boolean)
    hosts[h.domain] = ips.length === 1 ? ips[0] : ips
  })
  if (Object.keys(hosts).length) out.hosts = hosts
  return out
}

function onToggle() {
  if (!enabled.value) {
    const data = build()
    data._disabled = true
    emitFn('update', data)
    return
  }
  if (form.servers.length === 0) {
    form.servers = [
      { type: 'https', tag: 'dns-remote', address: 'https://1.1.1.1/dns-query' },
      { type: 'https', tag: 'dns-google', address: 'https://8.8.8.8/dns-query' },
    ]
  }
  emit()
}

function emit() { if (enabled.value) emitFn('update', build()) }
onMounted(sync)
</script>
<style scoped>
.rule-item { background: var(--el-fill-color-light); border-radius: 6px; padding: 12px; margin-bottom: 8px; }
.sub-item { display: flex; gap: 6px; margin-bottom: 8px; align-items: center; }
.hint { font-size: 12px; color: var(--el-text-color-secondary); margin-top: 2px; }
</style>
