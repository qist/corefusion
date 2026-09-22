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
        <el-form-item label="Cache Capacity">
          <el-input-number v-model="form.cache_capacity" :min="0" @change="emit" />
          <div class="hint">DNS 缓存条目上限（0 = 用核心默认）</div>
        </el-form-item>
        <el-form-item label="Optimistic">
          <el-switch v-model="form.optimistic_enabled" @change="emit" />
          <div class="hint">乐观缓存：过期条目先返回、同时后台刷新（明显降延迟；代价是可能短暂返回陈旧记录）</div>
        </el-form-item>
        <el-form-item v-if="form.optimistic_enabled" label="Optimistic Timeout">
          <el-input v-model="form.optimistic_timeout" placeholder="3s" @change="emit" />
          <div class="hint">后台刷新的等待上限；留空则按核心的简写形式写 <code>optimistic: true</code></div>
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
          <div class="hint">Tag of the final DNS server used as fallback（不能指向 fakeip server）</div>
        </el-form-item>
        <el-alert type="warning" :closable="false" style="margin-bottom:12px">
          <span style="font-size:12px">
            <code>domain_matcher</code>（已移除，写入会导致核心启动失败）与 <code>independent_cache</code>（1.14 起弃用）已从界面移除。
          </span>
        </el-alert>
      </el-form>

      <!-- FakeIP：sing-box 1.14 起顶层 dns.fakeip 已被移除（写入会导致核心启动失败） -->
      <el-divider content-position="left">FakeIP / Hosts</el-divider>
      <el-alert type="info" :closable="false" show-icon style="margin-bottom:12px">
        <div style="font-size:12px;line-height:1.8">
          sing-box 1.14 起，顶层 <code>dns.fakeip</code> 与 <code>dns.hosts</code> 都已被移除，写入会让核心启动失败。<br>
          <b>FakeIP</b>：在下方 DNS Servers 添加 <code>type = fakeip</code> 的 server（填 inet4_range / inet6_range），
          再用 DNS Rule 把 <code>query_type = A,AAAA</code> 指向它；注意 fakeip 不能作为 Final / 默认 server。<br>
          <b>Hosts</b>：添加 <code>type = hosts</code> 的 server，在 “Predefined” 里填域名到 IP 的映射。
        </div>
      </el-alert>

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
            <span v-if="s.server || s.address" style="font-size:12px;color:#909399">{{ s.server || s.address }}</span>
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

      <!-- Static Hosts：顶层 dns.hosts 已被移除，改由 type=hosts 的 DNS server 承载 -->
      <el-divider content-position="left">Static Hosts</el-divider>
      <div class="hint" style="margin-bottom:8px">
        顶层 <code>dns.hosts</code> 已被 sing-box 1.14 移除；请添加 <code>type = hosts</code> 的 DNS server，在其 “Predefined” 字段里写域名映射。
      </div>
    </template>

    <!-- Server Edit Dialog -->
    <el-dialog v-model="serverDlg" :title="editingServerIdx >= 0 ? 'Edit DNS Server' : 'Add DNS Server'" width="550">
      <el-form label-width="140px" label-position="left">
        <el-form-item label="Type">
          <el-select v-model="srvForm.type" style="width:100%">
            <el-option v-for="tp in DNS_SERVER_TYPES" :key="tp.value" :label="tp.label" :value="tp.value" />
          </el-select>
          <div class="hint">{{ currentTypeHint }}</div>
        </el-form-item>
        <el-form-item label="Tag">
          <el-input v-model="srvForm.tag" placeholder="dns-google" />
        </el-form-item>
        <!-- 远程类型：server / server_port -->
        <template v-if="REMOTE_TYPES.includes(srvForm.type)">
          <el-form-item label="Server">
            <el-input v-model="srvForm.server" placeholder="8.8.8.8 / dns.google / 1.1.1.1" />
            <div class="hint">sing-box 1.12+ 使用 <code>server</code>（旧字段 <code>address</code> 已移除）</div>
          </el-form-item>
          <el-form-item label="Server Port">
            <el-input-number v-model="srvForm.server_port" :min="0" :max="65535" placeholder="53" />
          </el-form-item>
          <el-form-item v-if="['https', 'h3'].includes(srvForm.type)" label="Path">
            <el-input v-model="srvForm.path" placeholder="/dns-query" />
          </el-form-item>
          <el-form-item v-if="['tls', 'https', 'h3', 'quic'].includes(srvForm.type)" label="TLS Server Name">
            <el-input v-model="srvForm.tls_server_name" placeholder="dns.google" />
          </el-form-item>
          <el-form-item v-if="['tls', 'https', 'h3', 'quic'].includes(srvForm.type)" label="TLS Insecure">
            <el-switch v-model="srvForm.tls_insecure" />
          </el-form-item>
        </template>
        <!-- fakeip -->
        <template v-if="srvForm.type === 'fakeip'">
          <el-form-item label="IPv4 Range">
            <el-input v-model="srvForm.inet4_range" placeholder="198.18.0.0/15" />
          </el-form-item>
          <el-form-item label="IPv6 Range">
            <el-input v-model="srvForm.inet6_range" placeholder="fc00::/18" />
          </el-form-item>
          <el-alert type="warning" :closable="false" style="margin-bottom:12px"
            title="fakeip 不能作为 Final / 默认 server，请在 DNS Rule 里用 query_type A,AAAA 指向它。" />
        </template>
        <!-- hosts -->
        <template v-if="srvForm.type === 'hosts'">
          <el-form-item label="Predefined">
            <el-input v-model="srvForm._predefinedStr" type="textarea" :rows="5"
              placeholder="example.com=1.2.3.4&#10;test.com=1.1.1.1, 2.2.2.2" />
            <div class="hint">每行一条：<code>域名=IP[,IP...]</code></div>
          </el-form-item>
          <el-form-item label="Path">
            <el-input v-model="srvForm._hostsPathStr" type="textarea" :rows="2" placeholder="每行一个 hosts 文件路径" />
          </el-form-item>
        </template>
        <!-- local / dhcp / mdns -->
        <template v-if="['local', 'dhcp', 'mdns'].includes(srvForm.type)">
          <el-form-item v-if="srvForm.type === 'local'" label="Prefer Go">
            <el-switch v-model="srvForm.prefer_go" />
          </el-form-item>
          <el-form-item v-if="['dhcp', 'mdns'].includes(srvForm.type)" label="Interface">
            <el-input v-model="srvForm._interfaceStr" placeholder="eth0（mdns 可逗号分隔多个）" />
          </el-form-item>
        </template>
        <!-- 依赖 endpoint 的类型 -->
        <el-form-item v-if="ENDPOINT_TYPES.includes(srvForm.type)" label="Endpoint Tag">
          <el-input v-model="srvForm.endpoint" placeholder="endpoint 段中的 tag，如 ts-ep / vpn-ep" />
          <div class="hint">该类型的 DNS server 需要引用 endpoints 段里的 tag</div>
        </el-form-item>
        <el-form-item label="Detour">
          <el-input v-model="srvForm.detour" placeholder="Outbound tag (optional)" />
        </el-form-item>
        <el-form-item label="Domain Resolver">
          <el-input v-model="srvForm.domain_resolver" placeholder="用于解析 server 域名的 DNS server tag" />
          <div class="hint">Server 填域名时必填（否则核心报 missing domain resolver）</div>
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
import { computed, reactive, ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
const props = defineProps({ data: Object })
const emitFn = defineEmits(['update'])
const enabled = ref(true)
const form = reactive({
  strategy: '', disable_cache: false, disable_expire: false,
  reverse_mapping: false,
  cache_capacity: 0, optimistic_enabled: false, optimistic_timeout: '',
  client_subnet: '', final: '',
  servers: [], rules: [],
})

const serverDlg = ref(false)
const editingServerIdx = ref(-1)

// sing-box 支持的 DNS server 类型（constant/dns.go）
// 注意：legacy / 无 type 已被移除；predefined 已改名为 hosts；h3 = http3
const DNS_SERVER_TYPES = [
  { value: 'udp', label: 'udp — 明文 UDP' },
  { value: 'tcp', label: 'tcp — 明文 TCP' },
  { value: 'tls', label: 'tls — DNS over TLS' },
  { value: 'https', label: 'https — DoH' },
  { value: 'h3', label: 'h3 — DoH (HTTP/3)' },
  { value: 'quic', label: 'quic — DoQ' },
  { value: 'local', label: 'local — 系统解析器' },
  { value: 'hosts', label: 'hosts — 静态记录' },
  { value: 'fakeip', label: 'fakeip — Fake IP' },
  { value: 'dhcp', label: 'dhcp' },
  { value: 'mdns', label: 'mdns' },
  { value: 'resolved', label: 'resolved' },
  { value: 'tailscale', label: 'tailscale' },
  { value: 'openconnect', label: 'openconnect' },
  { value: 'openvpn', label: 'openvpn' },
]
const REMOTE_TYPES = ['udp', 'tcp', 'tls', 'https', 'h3', 'quic']
const ENDPOINT_TYPES = ['tailscale', 'openconnect', 'openvpn']
// 不支持 server/server_port 的类型
const NO_SERVER_TYPES = ['local', 'dhcp', 'mdns', 'hosts', 'fakeip', 'resolved', 'tailscale', 'openconnect', 'openvpn']
// 支持拨号字段（detour / domain_resolver）的类型：hosts、fakeip 与 endpoint 类不支持
const DIAL_TYPES = ['udp', 'tcp', 'tls', 'https', 'h3', 'quic', 'local', 'dhcp', 'mdns']
const TYPE_HINTS = {
  udp: '明文 UDP，server 填 IP 或域名',
  tcp: '明文 TCP',
  tls: 'server 填域名时需同时填 TLS Server Name',
  https: 'DoH，路径一般为 /dns-query',
  h3: 'DoH over HTTP/3，路径一般为 /dns-query',
  quic: 'DoQ，端口通常 853',
  local: '使用系统解析器（Linux 可启用 Prefer Go）',
  hosts: '静态域名记录 + 可选 hosts 文件路径',
  fakeip: '需在 DNS Rule 中用 query_type A,AAAA 指向；不能作为默认 server',
  dhcp: '从 DHCP 获取 DNS，需指定 Interface',
  mdns: '局域网组播 DNS，需指定 Interface',
  resolved: '使用 systemd-resolved',
  tailscale: '需引用 endpoints 段的 tailscale endpoint tag',
  openconnect: '需引用 endpoints 段的 openconnect endpoint tag',
  openvpn: '需引用 endpoints 段的 openvpn endpoint tag',
}
const srvForm = reactive({
  type: 'udp', tag: '', server: '', server_port: 0, detour: '', domain_resolver: '',
  path: '', tls_server_name: '', tls_insecure: false,
  inet4_range: '', inet6_range: '',
  _predefinedStr: '', _hostsPathStr: '',
  prefer_go: false, _interfaceStr: '',
  endpoint: '',
})
const currentTypeHint = computed(() => TYPE_HINTS[srvForm.type] || '')

const ruleDlg = ref(false)
const editingRuleIdx = ref(-1)
const ruleForm = reactive({
  type: '', mode: '', _domain: '', _domainSuffix: '', _domainKeyword: '',
  _ipCIDR: '', _protocol: [], server: '', outbound: '', action: '',
})

function showAddServer() {
  editingServerIdx.value = -1
  Object.assign(srvForm, blankServer())
  serverDlg.value = true
}
function blankServer() {
  return {
    type: 'udp', tag: '', server: '', server_port: 0, detour: '', domain_resolver: '',
    path: '', tls_server_name: '', tls_insecure: false,
    inet4_range: '', inet6_range: '',
    _predefinedStr: '', _hostsPathStr: '',
    prefer_go: false, _interfaceStr: '',
    endpoint: '',
  }
}
function editServer(i) {
  editingServerIdx.value = i
  const s = form.servers[i] || {}
  const predefined = s.predefined || {}
  Object.assign(srvForm, blankServer(), {
    type: s.type || 'udp', tag: s.tag || '',
    // 兼容历史配置：旧字段 address
    server: s.server || s.address || '',
    server_port: s.server_port || 0,
    detour: s.detour || '', domain_resolver: s.domain_resolver || '',
    path: s.path || '', tls_server_name: s.tls?.server_name || '',
    tls_insecure: !!s.tls?.insecure,
    inet4_range: s.inet4_range || '', inet6_range: s.inet6_range || '',
    _predefinedStr: Object.entries(predefined).map(([d, ip]) => `${d}=${Array.isArray(ip) ? ip.join(', ') : ip}`).join('\n'),
    _hostsPathStr: (Array.isArray(s.path) ? s.path : (s.path ? [s.path] : [])).join('\n'),
    prefer_go: !!s.prefer_go,
    _interfaceStr: Array.isArray(s.interface) ? s.interface.join(', ') : (s.interface || ''),
    endpoint: s.endpoint || '',
  })
  serverDlg.value = true
}
function saveServer() {
  const srv = { type: srvForm.type, tag: srvForm.tag }
  if (REMOTE_TYPES.includes(srvForm.type)) {
    if (srvForm.server) srv.server = srvForm.server
    if (srvForm.server_port) srv.server_port = srvForm.server_port
    if (srvForm.path) srv.path = srvForm.path
    if (srvForm.tls_server_name || srvForm.tls_insecure) {
      srv.tls = {}
      if (srvForm.tls_server_name) srv.tls.server_name = srvForm.tls_server_name
      if (srvForm.tls_insecure) srv.tls.insecure = true
      if (srvForm.tls_server_name) srv.tls.enabled = true
    }
  }
  if (srvForm.type === 'fakeip') {
    if (srvForm.inet4_range) srv.inet4_range = srvForm.inet4_range
    if (srvForm.inet6_range) srv.inet6_range = srvForm.inet6_range
    if (!srvForm.inet4_range && !srvForm.inet6_range) {
      ElMessage.error('fakeip server 必须至少填写 IPv4 Range 或 IPv6 Range')
      return
    }
  }
  if (srvForm.type === 'hosts') {
    const predefined = {}
    for (const line of (srvForm._predefinedStr || '').split('\n')) {
      const s = line.trim()
      if (!s) continue
      const eq = s.indexOf('=')
      if (eq < 0) { ElMessage.error(`Predefined 行格式应为 域名=IP：${s}`); return }
      const domain = s.slice(0, eq).trim()
      const ips = s.slice(eq + 1).split(',').map(x => x.trim()).filter(Boolean)
      if (!domain || !ips.length) { ElMessage.error(`Predefined 行格式应为 域名=IP：${s}`); return }
      predefined[domain] = ips.length === 1 ? ips[0] : ips
    }
    if (Object.keys(predefined).length) srv.predefined = predefined
    const paths = (srvForm._hostsPathStr || '').split('\n').map(x => x.trim()).filter(Boolean)
    if (paths.length) srv.path = paths
  }
  if (srvForm.type === 'local' && srvForm.prefer_go) srv.prefer_go = true
  if (['dhcp', 'mdns'].includes(srvForm.type)) {
    const ifs = (srvForm._interfaceStr || '').split(',').map(x => x.trim()).filter(Boolean)
    if (srvForm.type === 'dhcp') { if (ifs.length) srv.interface = ifs[0] }
    else if (ifs.length) srv.interface = ifs
  }
  if (ENDPOINT_TYPES.includes(srvForm.type) && srvForm.endpoint) srv.endpoint = srvForm.endpoint
  if (DIAL_TYPES.includes(srvForm.type)) {
    if (srvForm.detour) srv.detour = srvForm.detour
    if (srvForm.domain_resolver) srv.domain_resolver = srvForm.domain_resolver
  }
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

// 历史 DNS server（sing-box < 1.12 形态）归一：address → server、URL 形式拆解、去掉已移除字段
function normalizeLegacyServer(s) {
  const o = { ...s }
  if (!o.server && o.address) {
    const addr = String(o.address)
    const m = addr.match(/^([a-z0-9]+):\/\/([^/]+)(\/.*)?$/i)
    if (m) {
      const scheme = m[1].toLowerCase()
      const typeMap = { https: 'https', h3: 'h3', tls: 'tls', quic: 'quic', tcp: 'tcp', udp: 'udp' }
      if (typeMap[scheme]) o.type = typeMap[scheme]
      const [host, port] = m[2].split(':')
      o.server = host
      if (port) o.server_port = Number(port)
      if (m[3]) o.path = m[3]
    } else {
      o.server = addr
    }
  }
  delete o.address
  delete o.strategy      // server 级 strategy 已被移除
  delete o.disable_cache // server 级 disable_cache 已被移除
  if (o.type === 'predefined') o.type = 'hosts' // predefined 已改名 hosts
  if (!o.type) o.type = 'udp'
  // 按类型裁剪不支持的字段（sing-box 严格解码，多写字段会 FATAL）
  if (NO_SERVER_TYPES.includes(o.type)) {
    delete o.server
    delete o.server_port
  }
  if (!DIAL_TYPES.includes(o.type)) {
    delete o.detour
    delete o.domain_resolver
  }
  return o
}

function sync() {
  if (!props.data || Object.keys(props.data).length === 0) { enabled.value = false; return }
  if (props.data._disabled) { enabled.value = false; return }
  enabled.value = true
  form.strategy = props.data.strategy || ''
  form.disable_cache = props.data.disable_cache || false
  form.disable_expire = props.data.disable_expire || false
  form.cache_capacity = props.data.cache_capacity || 0
  // optimistic 有两种合法写法：简写 true，或 { enabled, timeout }（见 option/dns.go 的 MarshalJSON）
  const opt = props.data.optimistic
  form.optimistic_enabled = opt === true || !!(opt && typeof opt === 'object' && opt.enabled)
  form.optimistic_timeout = (opt && typeof opt === 'object' && opt.timeout) || ''
  form.reverse_mapping = props.data.reverse_mapping || false
  form.client_subnet = props.data.client_subnet || ''
  form.final = props.data.final || ''
  form.servers = (props.data.servers || []).map(normalizeLegacyServer)
  // 历史配置迁移：顶层 dns.fakeip / dns.hosts → 对应类型的 server（1.14 起顶层字段已移除）
  const legacyFakeip = props.data.fakeip
  if (legacyFakeip && Object.keys(legacyFakeip).length && !form.servers.some(s => s.type === 'fakeip')) {
    form.servers.push({
      type: 'fakeip', tag: 'fakeip',
      inet4_range: legacyFakeip.inet4_range || '198.18.0.0/15',
      inet6_range: legacyFakeip.inet6_range || 'fc00::/18',
    })
  }
  const legacyHosts = props.data.hosts
  if (legacyHosts && Object.keys(legacyHosts).length && !form.servers.some(s => s.type === 'hosts')) {
    form.servers.push({ type: 'hosts', tag: 'hosts', predefined: { ...legacyHosts } })
  }
  // 历史 server 级 strategy → 提升为顶层 strategy
  if (!form.strategy) {
    const legacyStrategy = (props.data.servers || []).map(s => s.strategy).find(Boolean)
    if (legacyStrategy) form.strategy = legacyStrategy
  }
  form.rules = (props.data.rules || []).map(r => ({
    ...r,
    _domain: (r.domain || []).join('\n'),
    _domainSuffix: (r.domain_suffix || []).join('\n'),
    _domainKeyword: (r.domain_keyword || []).join('\n'),
    _ipCIDR: (r.ip_cidr || []).join('\n'),
    _protocol: r.protocol || [],
  }))
}

function build() {
  const out = {
    strategy: form.strategy || undefined,
    disable_cache: form.disable_cache || undefined,
    disable_expire: form.disable_expire || undefined,
    cache_capacity: form.cache_capacity || undefined,
    // 与核心的 MarshalJSON 保持一致：只开 enabled 时写简写 true，带 timeout 才写对象
    optimistic: form.optimistic_enabled
      ? (form.optimistic_timeout ? { enabled: true, timeout: form.optimistic_timeout } : true)
      : undefined,
    reverse_mapping: form.reverse_mapping || undefined,
    client_subnet: form.client_subnet || undefined,
    final: form.final || undefined,
  }
  // 顶层 dns.fakeip / dns.hosts 已被 sing-box 1.14 移除，这里按原样透传（由后端迁移兜底）
  out.servers = form.servers.filter(s => s.tag).map(s => {
    const srv = { tag: s.tag }
    if (s.type) srv.type = s.type
    if (s.server) srv.server = s.server
    if (s.server_port) srv.server_port = s.server_port
    if (s.path) srv.path = s.path
    if (s.tls && Object.keys(s.tls).length) srv.tls = { ...s.tls }
    if (s.detour) srv.detour = s.detour
    if (s.domain_resolver) srv.domain_resolver = s.domain_resolver
    if (s.inet4_range) srv.inet4_range = s.inet4_range
    if (s.inet6_range) srv.inet6_range = s.inet6_range
    if (s.predefined && Object.keys(s.predefined).length) srv.predefined = { ...s.predefined }
    if (s.prefer_go) srv.prefer_go = true
    if (s.interface) srv.interface = Array.isArray(s.interface) ? [...s.interface] : s.interface
    if (s.endpoint) srv.endpoint = s.endpoint
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
      { type: 'https', tag: 'dns-remote', server: '1.1.1.1', server_port: 443, path: '/dns-query' },
      { type: 'https', tag: 'dns-google', server: '8.8.8.8', server_port: 443, path: '/dns-query' },
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
