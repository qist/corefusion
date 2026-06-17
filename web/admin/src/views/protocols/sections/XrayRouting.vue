<template>
  <div>
    <!-- Geo files warning -->
    <el-alert v-if="!geoStatus['geoip.dat'] || !geoStatus['geosite.dat']" type="warning" :closable="false" style="margin-bottom:16px">
      <template #title>
        <span>Missing geo files: </span>
        <span v-if="!geoStatus['geoip.dat']" style="color:#e6a23c;font-weight:600">geoip.dat </span>
        <span v-if="!geoStatus['geosite.dat']" style="color:#e6a23c;font-weight:600">geosite.dat </span>
        <span>— Download from </span>
        <el-link type="primary" href="/settings" target="_blank">Rule Management</el-link>
        <span> before using geoip:/geosite: rules</span>
      </template>
    </el-alert>

    <!-- Global Settings -->
    <el-divider content-position="left">Global Settings</el-divider>
    <el-form label-width="140px" label-position="left">
      <el-form-item label="Domain Strategy">
        <el-select v-model="form.domainStrategy" style="width:100%" @change="emit">
          <el-option label="AsIs" value="AsIs" />
          <el-option label="IPIfNonMatch" value="IPIfNonMatch" />
          <el-option label="IPOnDemand" value="IPOnDemand" />
        </el-select>
      </el-form-item>
    </el-form>

    <!-- Routing Rules -->
    <el-divider content-position="left">
      Routing Rules
      <el-button size="small" type="primary" @click="showAddRule" style="margin-left:12px">+ Add Rule</el-button>
    </el-divider>

    <div v-for="(r, i) in form.rules" :key="i" class="rule-item">
      <div style="display:flex;justify-content:space-between;align-items:center">
        <div style="display:flex;align-items:center;gap:8px;flex-wrap:wrap">
          <el-switch v-model="r.enabled" @change="emit" style="margin-right:4px" />
          <el-tag v-if="r.ruleTag" size="small">{{ r.ruleTag }}</el-tag>
          <el-tag v-if="r.outboundTag" size="small" :type="r.outboundTag==='blocked'?'danger':r.outboundTag==='direct'?'success':'info'">{{ r.outboundTag }}</el-tag>
          <el-tag v-if="r.balancerTag" size="small" type="warning">{{ r.balancerTag }}</el-tag>
          <span v-if="r.domain && r.domain.length" style="font-size:12px;color:#909399">domain:{{ r.domain.length }}</span>
          <span v-if="r.ip && r.ip.length" style="font-size:12px;color:#909399">ip:{{ r.ip.length }}</span>
          <span v-if="r.protocol && r.protocol.length" style="font-size:12px;color:#909399">{{ r.protocol.join(',') }}</span>
          <span v-if="r.port" style="font-size:12px;color:#909399">port:{{ r.port }}</span>
          <span v-if="r.network" style="font-size:12px;color:#909399">{{ r.network }}</span>
          <span v-if="r.inboundTag && r.inboundTag.length" style="font-size:12px;color:#909399">in:{{ r.inboundTag.join(',') }}</span>
          <span v-if="r.webhook" style="font-size:12px;color:#909399">webhook</span>
        </div>
        <div>
          <el-button size="small" @click="editRule(i)">Edit</el-button>
          <el-button size="small" type="danger" text @click="rmRule(i)">X</el-button>
        </div>
      </div>
    </div>

    <!-- Balancers -->
    <el-divider content-position="left">
      Balancers
      <el-button size="small" @click="addBalancer" style="margin-left:12px">+ Add</el-button>
    </el-divider>
    <div v-for="(b, i) in form.balancers" :key="'b'+i" class="rule-item">
      <div style="display:flex;gap:8px;align-items:center">
        <el-input v-model="b.tag" placeholder="Tag" style="width:20%" @change="emit" />
        <el-input v-model="b.selectorStr" placeholder="Selectors (comma-sep)" style="width:35%" @change="emit" />
        <el-select v-model="b.strategyType" style="width:20%" @change="emit">
          <el-option label="random" value="random" /><el-option label="leastPing" value="leastPing" /><el-option label="leastLoad" value="leastLoad" />
        </el-select>
        <el-button size="small" type="danger" text @click="rmBalancer(i)">X</el-button>
      </div>
    </div>

    <!-- Rule Dialog -->
    <el-dialog v-model="dlg" :title="editIdx >= 0 ? 'Edit Rule' : 'Add Rule'" width="700">
      <el-form label-width="120px" label-position="left">
        <!-- Rule Name -->
        <el-form-item label="Rule Name">
          <el-input v-model="rf.ruleTag" />
        </el-form-item>

        <!-- Rule Type -->
        <el-form-item label="Rule Type">
          <el-select v-model="rf._type" style="width:100%" @change="onTypeChange">
            <el-option label="Domain" value="domain" />
            <el-option label="IP" value="ip" />
            <el-option label="Port" value="port" />
            <el-option label="Source Port" value="sourcePort" />
            <el-option label="Local Port" value="localPort" />
            <el-option label="Network" value="network" />
            <el-option label="Protocol" value="protocol" />
            <el-option label="Source IP" value="sourceIP" />
            <el-option label="Local IP" value="localIP" />
            <el-option label="Process" value="process" />
            <el-option label="User" value="user" />
            <el-option label="Attrs" value="attrs" />
            <el-option label="VLESS Route" value="vlessRoute" />
            <el-option label="Webhook" value="webhook" />
          </el-select>
        </el-form-item>

        <!-- Domain -->
        <template v-if="rf._type === 'domain'">
          <el-form-item label="Domains">
            <el-select v-model="rf._domain" multiple filterable allow-create default-first-option placeholder="Select or type" style="width:100%">
              <el-option-group label="Geosite">
                <el-option label="geosite:cn" value="geosite:cn" />
                <el-option label="geosite:geolocation-cn" value="geosite:geolocation-cn" />
                <el-option label="geosite:google" value="geosite:google" />
                <el-option label="geosite:github" value="geosite:github" />
                <el-option label="geosite:telegram" value="geosite:telegram" />
                <el-option label="geosite:openai" value="geosite:openai" />
                <el-option label="geosite:netflix" value="geosite:netflix" />
                <el-option label="geosite:category-ads-all" value="geosite:category-ads-all" />
              </el-option-group>
            </el-select>
          </el-form-item>
        </template>

        <!-- IP -->
        <template v-if="rf._type === 'ip'">
          <el-form-item label="IP / CIDR">
            <el-select v-model="rf._ip" multiple filterable allow-create default-first-option placeholder="Select or type" style="width:100%">
              <el-option-group label="GeoIP">
                <el-option label="geoip:cn" value="geoip:cn" />
                <el-option label="geoip:us" value="geoip:us" />
                <el-option label="geoip:telegram" value="geoip:telegram" />
                <el-option label="geoip:private" value="geoip:private" />
                <el-option label="geoip:!cn" value="geoip:!cn" />
              </el-option-group>
            </el-select>
          </el-form-item>
        </template>

        <!-- Port -->
        <template v-if="rf._type === 'port'">
          <el-form-item label="Port Range"><el-input v-model="rf._port" placeholder="53,443,1000-2000" /></el-form-item>
        </template>

        <!-- Source Port -->
        <template v-if="rf._type === 'sourcePort'">
          <el-form-item label="Source Port"><el-input v-model="rf._sourcePort" placeholder="53,443,1000-2000" /></el-form-item>
        </template>

        <!-- Local Port -->
        <template v-if="rf._type === 'localPort'">
          <el-form-item label="Local Port"><el-input v-model="rf._localPort" placeholder="53,443,1000-2000" /></el-form-item>
        </template>

        <!-- Network -->
        <template v-if="rf._type === 'network'">
          <el-form-item label="Network">
            <el-select v-model="rf._network" style="width:100%">
              <el-option label="tcp" value="tcp" /><el-option label="udp" value="udp" /><el-option label="tcp,udp" value="tcp,udp" />
            </el-select>
          </el-form-item>
        </template>

        <!-- Protocol -->
        <template v-if="rf._type === 'protocol'">
          <el-form-item label="Protocol">
            <el-select v-model="rf._protocol" multiple style="width:100%">
              <el-option label="http" value="http" /><el-option label="tls" value="tls" />
              <el-option label="quic" value="quic" /><el-option label="bittorrent" value="bittorrent" />
              <el-option label="dns" value="dns" />
            </el-select>
          </el-form-item>
        </template>

        <!-- Source IP -->
        <template v-if="rf._type === 'sourceIP'">
          <el-form-item label="Source IP">
            <el-select v-model="rf._sourceIP" multiple filterable allow-create default-first-option placeholder="Select or type" style="width:100%">
              <el-option label="geoip:cn" value="geoip:cn" /><el-option label="geoip:private" value="geoip:private" />
            </el-select>
          </el-form-item>
        </template>

        <!-- Local IP -->
        <template v-if="rf._type === 'localIP'">
          <el-form-item label="Local IP">
            <el-select v-model="rf._localIP" multiple filterable allow-create default-first-option placeholder="Select or type" style="width:100%">
              <el-option label="192.168.0.0/16" value="192.168.0.0/16" /><el-option label="10.0.0.0/8" value="10.0.0.0/8" />
            </el-select>
          </el-form-item>
        </template>

        <!-- Process -->
        <template v-if="rf._type === 'process'">
          <el-form-item label="Process Name">
            <el-select v-model="rf._process" multiple filterable allow-create default-first-option placeholder="Select or type" style="width:100%">
              <el-option label="curl" value="curl" /><el-option label="wget" value="wget" />
              <el-option label="chrome" value="chrome" /><el-option label="firefox" value="firefox" />
            </el-select>
          </el-form-item>
        </template>

        <!-- User -->
        <template v-if="rf._type === 'user'">
          <el-form-item label="User Email">
            <el-select v-model="rf._user" multiple filterable allow-create default-first-option placeholder="Select or type" style="width:100%">
              <el-option v-for="t in userTags" :key="t" :label="t" :value="t" />
            </el-select>
          </el-form-item>
        </template>

        <!-- Attrs (multiple key-value) -->
        <template v-if="rf._type === 'attrs'">
          <el-form-item label="Attributes">
            <div v-for="(a, i) in rf._attrs" :key="i" style="display:flex;gap:8px;margin-bottom:6px">
              <el-input v-model="a.key" placeholder=":method" style="width:40%" />
              <el-input v-model="a.value" placeholder="GET" style="width:40%" />
              <el-button size="small" type="danger" text @click="rf._attrs.splice(i,1)">X</el-button>
            </div>
            <el-button size="small" @click="rf._attrs.push({key:'',value:''})">+ Add</el-button>
          </el-form-item>
        </template>

        <!-- VLESS Route -->
        <template v-if="rf._type === 'vlessRoute'">
          <el-form-item label="VLESS Route"><el-input v-model="rf._vlessRoute" placeholder="53,443,1000-2000" /></el-form-item>
        </template>

        <!-- Webhook -->
        <template v-if="rf._type === 'webhook'">
          <el-form-item label="URL">
            <el-input v-model="rf._webhookUrl" placeholder="https://api.example.com/alert" />
          </el-form-item>
          <el-form-item label="Deduplication (s)">
            <el-input-number v-model="rf._webhookDedup" :min="0" />
          </el-form-item>
          <el-form-item label="Headers">
            <div v-for="(h, i) in rf._webhookHeaders" :key="i" style="display:flex;gap:8px;margin-bottom:6px">
              <el-input v-model="h.key" placeholder="Header name" style="width:40%" />
              <el-input v-model="h.value" placeholder="Header value" style="width:40%" />
              <el-button size="small" type="danger" text @click="rf._webhookHeaders.splice(i,1)">X</el-button>
            </div>
            <el-button size="small" @click="rf._webhookHeaders.push({key:'',value:''})">+ Add</el-button>
          </el-form-item>
        </template>

        <el-divider content-position="left">Action</el-divider>

        <!-- Outbound (required) -->
        <el-form-item label="Outbound">
          <el-select v-model="rf.outboundTag" style="width:100%">
            <el-option v-for="t in outboundTags" :key="t" :label="t" :value="t" />
          </el-select>
        </el-form-item>

        <!-- Inbound Tag (optional, multi-select from existing) -->
        <el-form-item label="Inbound Tag">
          <el-select v-model="rf._actionInboundTag" multiple filterable allow-create default-first-option placeholder="Optional" style="width:100%" clearable>
            <el-option v-for="t in inboundTags" :key="t" :label="t" :value="t" />
          </el-select>
        </el-form-item>

        <!-- Balancer (optional) -->
        <el-form-item label="Balancer">
          <el-select v-model="rf.balancerTag" style="width:100%" clearable>
            <el-option label="None" value="" />
            <el-option v-for="b in form.balancers" :key="b.tag" :label="b.tag" :value="b.tag" />
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
import { reactive, ref, onMounted } from 'vue'
import client from '../../../api/client'
const props = defineProps({ data: Object, coreType: String })
const emitFn = defineEmits(['update'])

const form = reactive({ domainStrategy: 'IPIfNonMatch', rules: [], balancers: [] })
const outboundTags = ref(['direct', 'blocked'])
const inboundTags = ref([])
const userTags = ref([])
const geoStatus = ref({ 'geoip.dat': false, 'geosite.dat': false })

const dlg = ref(false)
const editIdx = ref(-1)
const rf = reactive({
  ruleTag: '', _type: 'domain',
  _domain: [], _ip: [], _port: '', _sourcePort: '', _localPort: '',
  _network: '', _protocol: [], _sourceIP: [], _localIP: [],
  _process: [], _user: [],
  _attrs: [], _vlessRoute: '',
  _webhookUrl: '', _webhookDedup: 0, _webhookHeaders: [],
  outboundTag: 'direct', _actionInboundTag: [], balancerTag: '',
})

function genTag() { return 'rule-' + Math.random().toString(36).substring(2, 8) }

function onTypeChange() {
  rf._domain = []; rf._ip = []; rf._port = ''; rf._sourcePort = ''; rf._localPort = ''
  rf._network = ''; rf._protocol = []; rf._sourceIP = []; rf._localIP = []
  rf._process = []; rf._user = []
  rf._attrs = []; rf._vlessRoute = ''
  rf._webhookUrl = ''; rf._webhookDedup = 0; rf._webhookHeaders = []
}

function resetForm() {
  Object.assign(rf, {
    ruleTag: genTag(), _type: 'domain',
    _domain: [], _ip: [], _port: '', _sourcePort: '', _localPort: '',
    _network: '', _protocol: [], _sourceIP: [], _localIP: [],
    _process: [], _user: [],
    _attrs: [], _vlessRoute: '',
    _webhookUrl: '', _webhookDedup: 0, _webhookHeaders: [],
    outboundTag: 'direct', _actionInboundTag: [], balancerTag: '',
  })
}

function showAddRule() { editIdx.value = -1; resetForm(); dlg.value = true }

function editRule(i) {
  editIdx.value = i
  const r = form.rules[i]
  let t = 'domain'
  if (r.ip && r.ip.length) t = 'ip'
  else if (r.port) t = 'port'
  else if (r.sourcePort) t = 'sourcePort'
  else if (r.localPort) t = 'localPort'
  else if (r.network) t = 'network'
  else if (r.protocol && r.protocol.length) t = 'protocol'
  else if (r.source && r.source.length) t = 'sourceIP'
  else if (r.localIP && r.localIP.length) t = 'localIP'
  else if (r.process && r.process.length) t = 'process'
  else if (r.user && r.user.length) t = 'user'
  else if (r.attrs) t = 'attrs'
  else if (r.vlessRoute) t = 'vlessRoute'
  else if (r.webhook) t = 'webhook'

  const attrs = r.attrs ? Object.entries(r.attrs).map(([k,v]) => ({key:k,value:v})) : []
  const whHeaders = r.webhook && r.webhook.headers ? Object.entries(r.webhook.headers).map(([k,v]) => ({key:k,value:v})) : []

  Object.assign(rf, {
    ruleTag: r.ruleTag || genTag(), _type: t,
    _domain: r.domain || [], _ip: r.ip || [],
    _port: r.port || '', _sourcePort: r.sourcePort || '', _localPort: r.localPort || '',
    _network: r.network || '', _protocol: r.protocol || [],
    _sourceIP: r.source || [], _localIP: r.localIP || [],
    _process: r.process || [], _user: r.user || [],
    _attrs: attrs, _vlessRoute: r.vlessRoute || '',
    _webhookUrl: r.webhook ? r.webhook.url || '' : '',
    _webhookDedup: r.webhook ? r.webhook.deduplication || 0 : 0,
    _webhookHeaders: whHeaders,
    outboundTag: r.outboundTag || 'direct',
    _actionInboundTag: r.inboundTag || [],
    balancerTag: r.balancerTag || '',
  })
  dlg.value = true
}

function saveRule() {
  const rule = { type: 'field', enabled: true, ruleTag: rf.ruleTag, outboundTag: rf.outboundTag }
  if (rf.balancerTag) rule.balancerTag = rf.balancerTag
  if (rf._actionInboundTag && rf._actionInboundTag.length) rule.inboundTag = [...rf._actionInboundTag]

  if (rf._type === 'domain' && rf._domain.length) rule.domain = [...rf._domain]
  if (rf._type === 'ip' && rf._ip.length) rule.ip = [...rf._ip]
  if (rf._type === 'port' && rf._port) rule.port = rf._port
  if (rf._type === 'sourcePort' && rf._sourcePort) rule.sourcePort = rf._sourcePort
  if (rf._type === 'localPort' && rf._localPort) rule.localPort = rf._localPort
  if (rf._type === 'network' && rf._network) rule.network = rf._network
  if (rf._type === 'protocol' && rf._protocol.length) rule.protocol = [...rf._protocol]
  if (rf._type === 'sourceIP' && rf._sourceIP.length) rule.source = [...rf._sourceIP]
  if (rf._type === 'localIP' && rf._localIP.length) rule.localIP = [...rf._localIP]
  if (rf._type === 'process' && rf._process.length) rule.process = [...rf._process]
  if (rf._type === 'user' && rf._user.length) rule.user = [...rf._user]
  if (rf._type === 'attrs' && rf._attrs.length) {
    const attrs = {}
    rf._attrs.forEach(a => { if (a.key) attrs[a.key] = a.value })
    if (Object.keys(attrs).length) rule.attrs = attrs
  }
  if (rf._type === 'vlessRoute' && rf._vlessRoute) rule.vlessRoute = rf._vlessRoute
  if (rf._type === 'webhook' && rf._webhookUrl) {
    const wh = { url: rf._webhookUrl }
    if (rf._webhookDedup) wh.deduplication = rf._webhookDedup
    if (rf._webhookHeaders.length) {
      const headers = {}
      rf._webhookHeaders.forEach(h => { if (h.key) headers[h.key] = h.value })
      if (Object.keys(headers).length) wh.headers = headers
    }
    rule.webhook = wh
  }

  if (editIdx.value >= 0) form.rules[editIdx.value] = rule
  else form.rules.push(rule)
  dlg.value = false
  emit()
}

async function loadTags() {
  try {
    const obs = await client.get(`/protocols/${props.coreType}/section/outbounds`)
    if (Array.isArray(obs)) outboundTags.value = [...new Set(['direct', 'blocked', ...obs.map(o => o.tag).filter(Boolean)])]
  } catch {}
  try {
    const ibs = await client.get(`/protocols/${props.coreType}/section/inbounds`)
    if (Array.isArray(ibs)) inboundTags.value = ibs.map(i => i.tag).filter(Boolean)
  } catch {}
}

function sync() {
  if (!props.data) return
  form.domainStrategy = props.data.domainStrategy || 'IPIfNonMatch'
  form.rules = (props.data.rules || []).map(r => ({ ...r, enabled: r.enabled !== false }))
  form.balancers = (props.data.balancers || []).map(b => ({
    tag: b.tag || '', selectorStr: (b.selector || []).join(', '), strategyType: b.strategy?.type || 'random',
  }))
}

function build() {
  return {
    domainStrategy: form.domainStrategy,
    rules: form.rules.filter(r => r.enabled !== false).map(r => {
      const rule = { ...r }
      delete rule.enabled
      return rule
    }),
    balancers: form.balancers.filter(b => b.tag).map(b => ({
      tag: b.tag,
      selector: b.selectorStr ? b.selectorStr.split(',').map(s => s.trim()).filter(Boolean) : [],
      strategy: { type: b.strategyType },
    })),
  }
}

function emit() { emitFn('update', build()) }
function rmRule(i) { form.rules.splice(i, 1); emit() }
function addBalancer() { form.balancers.push({ tag: '', selectorStr: '', strategyType: 'random' }); emit() }
function rmBalancer(i) { form.balancers.splice(i, 1); emit() }

async function loadGeoStatus() {
  try {
    const data = await client.get('/geo-rules/status')
    if (data) geoStatus.value = data
  } catch {}
}

onMounted(function() { sync(); loadTags(); loadGeoStatus() })
</script>

<style scoped>
.rule-item { background: var(--el-fill-color-light); border-radius: 6px; padding: 12px; margin-bottom: 8px; }
</style>
