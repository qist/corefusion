<template>
  <div>
    <el-form label-width="160px" label-position="left">
      <el-form-item :label="t('protoCenter.dns.enable')">
        <el-switch v-model="enabled" @change="onToggle" />
      </el-form-item>
    </el-form>

    <template v-if="enabled">
      <!-- Global Settings -->
      <el-divider content-position="left">Global Settings</el-divider>
      <el-form label-width="160px" label-position="left">
        <el-form-item :label="t('protoCenter.routing.strategy')">
          <el-select v-model="form.strategy" style="width:100%" clearable @change="emit">
            <el-option label="" value="" />
            <el-option label="prefer_ipv4" value="prefer_ipv4" />
            <el-option label="prefer_ipv6" value="prefer_ipv6" />
            <el-option label="ipv4_only" value="ipv4_only" />
            <el-option label="ipv6_only" value="ipv6_only" />
          </el-select>
          <div class="hint">DNS query strategy for IP version preference</div>
        </el-form-item>
        <el-form-item :label="t('protoCenter.dns.disableCache')">
          <el-switch v-model="form.disable_cache" @change="emit" />
        </el-form-item>
        <el-form-item :label="t('singboxDns.disableExpire')">
          <el-switch v-model="form.disable_expire" @change="emit" />
          <div class="hint">Disable cache expiration</div>
        </el-form-item>
        <el-form-item :label="t('singboxDns.cacheCapacity')">
          <el-input-number v-model="form.cache_capacity" :min="0" @change="emit" />
          <div class="hint">{{ t('protoHint.dns.cacheCapacity') }}</div>
        </el-form-item>
        <el-form-item :label="t('singboxDns.optimistic')">
          <el-switch v-model="form.optimistic_enabled" @change="emit" />
          <div class="hint">{{ t('protoHint.dns.optimistic') }}</div>
        </el-form-item>
        <el-form-item v-if="form.optimistic_enabled" :label="t('singboxDns.optimisticTimeout')">
          <el-input v-model="form.optimistic_timeout" placeholder="3s" @change="emit" />
          <div class="hint" v-html="t('protoHint.dns.optimisticTimeout')"></div>
        </el-form-item>
        <el-form-item :label="t('singboxDns.reverseMapping')">
          <el-switch v-model="form.reverse_mapping" @change="emit" />
          <div class="hint">Store reverse DNS mapping for IPs</div>
        </el-form-item>
        <el-form-item :label="t('singboxDns.clientSubnet')">
          <el-input v-model="form.client_subnet" placeholder="0.0.0.0/24" @change="emit" />
          <div class="hint">EDNS0 client subnet</div>
        </el-form-item>
        <el-form-item :label="t('singboxDns.finalServer')">
          <el-input v-model="form.final" placeholder="dns-remote" @change="emit" />
          <div class="hint">{{ t('protoHint.dns.finalFallback') }}</div>
        </el-form-item>
        <el-alert type="warning" :closable="false" style="margin-bottom:12px">
          <span style="font-size:12px">
            <span v-html="t('protoHint.dns.removedFields')"></span>
          </span>
        </el-alert>
      </el-form>

      <!-- FakeIP：sing-box 1.14 起顶层 dns.fakeip 已被移除（写入会导致核心启动失败） -->
      <el-divider content-position="left">FakeIP / Hosts</el-divider>
      <el-alert type="info" :closable="false" show-icon style="margin-bottom:12px">
        <div style="font-size:12px;line-height:1.8">
          <span v-html="t('protoHint.dns.fakeipNotice')"></span>
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
        <span v-html="t('protoHint.dns.hostsRemoved')"></span>
      </div>
    </template>

    <!-- Server Edit Dialog -->
    <el-dialog v-model="serverDlg" :title="editingServerIdx >= 0 ? 'Edit DNS Server' : 'Add DNS Server'" width="550">
      <el-form label-width="140px" label-position="left">
        <el-form-item :label="t('protoCenter.inbounds.accessDomainType')">
          <el-select v-model="srvForm.type" style="width:100%">
            <el-option v-for="tp in DNS_SERVER_TYPES" :key="tp.value" :label="tp.label" :value="tp.value" />
          </el-select>
          <div class="hint">{{ currentTypeHint }}</div>
        </el-form-item>
        <el-form-item :label="t('protoCenter.api.tag')">
          <el-input v-model="srvForm.tag" placeholder="dns-google" />
        </el-form-item>
        <!-- 远程类型：server / server_port -->
        <template v-if="REMOTE_TYPES.includes(srvForm.type)">
          <el-form-item :label="t('protoCenter.outbounds.server')">
            <el-input v-model="srvForm.server" placeholder="8.8.8.8 / dns.google / 1.1.1.1" />
            <div class="hint" v-html="t('protoHint.dns.serverField')"></div>
          </el-form-item>
          <el-form-item :label="t('singbox.outboundBase.serverPort')">
            <el-input-number v-model="srvForm.server_port" :min="0" :max="65535" placeholder="53" />
          </el-form-item>
          <el-form-item v-if="['https', 'h3'].includes(srvForm.type)" :label="t('protoCenter.masque.path')">
            <el-input v-model="srvForm.path" placeholder="/dns-query" />
          </el-form-item>
          <el-form-item v-if="['tls', 'https', 'h3', 'quic'].includes(srvForm.type)" :label="t('singboxDns.tlsServerName')">
            <el-input v-model="srvForm.tls_server_name" placeholder="dns.google" />
          </el-form-item>
          <el-form-item v-if="['tls', 'https', 'h3', 'quic'].includes(srvForm.type)" :label="t('singboxDns.tlsInsecure')">
            <el-switch v-model="srvForm.tls_insecure" />
          </el-form-item>
        </template>
        <!-- fakeip -->
        <template v-if="srvForm.type === 'fakeip'">
          <el-form-item :label="t('singboxDns.ipv4Range')">
            <el-input v-model="srvForm.inet4_range" placeholder="198.18.0.0/15" />
          </el-form-item>
          <el-form-item :label="t('singboxDns.ipv6Range')">
            <el-input v-model="srvForm.inet6_range" placeholder="fc00::/18" />
          </el-form-item>
          <el-alert type="warning" :closable="false" style="margin-bottom:12px"
            :title="t('protoHint.dns.fakeipCantBeFinal')" />
        </template>
        <!-- hosts -->
        <template v-if="srvForm.type === 'hosts'">
          <el-form-item :label="t('singboxDns.predefined')">
            <el-input v-model="srvForm._predefinedStr" type="textarea" :rows="5"
              placeholder="example.com=1.2.3.4&#10;test.com=1.1.1.1, 2.2.2.2" />
            <div class="hint" v-html="t('protoHint.dns.hostsLines')"></div>
          </el-form-item>
          <el-form-item :label="t('protoCenter.masque.path')">
            <el-input v-model="srvForm._hostsPathStr" type="textarea" :rows="2" :placeholder="t('protoHint.dns.hostsPath')" />
          </el-form-item>
        </template>
        <!-- local / dhcp / mdns -->
        <template v-if="['local', 'dhcp', 'mdns'].includes(srvForm.type)">
          <el-form-item v-if="srvForm.type === 'local'" :label="t('singboxDns.preferGo')">
            <el-switch v-model="srvForm.prefer_go" />
          </el-form-item>
          <el-form-item v-if="['dhcp', 'mdns'].includes(srvForm.type)" :label="t('protoCenter.inbounds.interface')">
            <el-input v-model="srvForm._interfaceStr" :placeholder="t('singboxDns.mdnsInterfaceHint')" />
          </el-form-item>
        </template>
        <!-- 依赖 endpoint 的类型 -->
        <el-form-item v-if="ENDPOINT_TYPES.includes(srvForm.type)" :label="t('singboxDns.endpointTag')">
          <el-input v-model="srvForm.endpoint" :placeholder="t('singboxDns.endpointTagHint')" />
          <div class="hint">{{ t('protoHint.dns.endpointTag') }}</div>
        </el-form-item>
        <el-form-item :label="t('xrayIn.detour')">
          <el-input v-model="srvForm.detour" :placeholder="t('singboxDns.outboundTagOptional')" />
        </el-form-item>
        <el-form-item :label="t('singbox.outboundBase.domainResolver')">
          <el-input v-model="srvForm.domain_resolver" :placeholder="t('protoHint.dns.domainResolverTag')" />
          <div class="hint">{{ t('protoHint.dns.domainResolverHint') }}</div>
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
        <el-form-item :label="t('protoCenter.inbounds.accessDomainType')">
          <el-select v-model="ruleForm.type" style="width:100%" clearable>
            <el-option label="default" value="" />
            <el-option label="logical" value="logical" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="ruleForm.type === 'logical'" :label="t('xrayUi.mode')">
          <el-select v-model="ruleForm.mode" style="width:100%">
            <el-option label="and" value="and" />
            <el-option label="or" value="or" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('protoCenter.routing.domain')">
          <el-input v-model="ruleForm._domain" type="textarea" :rows="2" :placeholder="t('singboxRoute.oneDomainPerLine')" />
        </el-form-item>
        <el-form-item :label="t('singboxRoute.domainSuffix')">
          <el-input v-model="ruleForm._domainSuffix" type="textarea" :rows="2" :placeholder="t('singboxRoute.oneSuffixPerLine')" />
        </el-form-item>
        <el-form-item :label="t('singboxRoute.domainKeyword')">
          <el-input v-model="ruleForm._domainKeyword" type="textarea" :rows="2" :placeholder="t('singboxRoute.oneKeywordPerLine')" />
        </el-form-item>
        <el-form-item :label="t('subfmt.dnsFallbackIPCIDR')">
          <el-input v-model="ruleForm._ipCIDR" type="textarea" :rows="2" :placeholder="t('singboxRoute.oneCidrPerLine')" />
        </el-form-item>
        <el-form-item :label="t('protoCenter.protocol')">
          <el-select v-model="ruleForm._protocol" multiple style="width:100%">
            <el-option label="http" value="http" />
            <el-option label="tls" value="tls" />
            <el-option label="quic" value="quic" />
            <el-option label="stun" value="stun" />
            <el-option label="dns" value="dns" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('singboxDns.queryDnssec')">
          <el-switch v-model="ruleForm._queryDnssec" />
          <div class="hint">{{ t('protoHint.dns.dnssec') }}</div>
        </el-form-item>
        <el-form-item :label="t('singboxDns.queryClientSubnet')">
          <el-input v-model="ruleForm._queryClientSubnet" type="textarea" :rows="2" placeholder="1.2.3.0/24" />
          <div class="hint">{{ t('protoHint.dns.ecs') }}</div>
        </el-form-item>
        <el-form-item :label="t('protoCenter.outbounds.server')">
          <el-input v-model="ruleForm.server" placeholder="dns-server tag" />
        </el-form-item>
        <el-form-item :label="t('protoCenter.routing.outbound')">
          <el-input v-model="ruleForm.outbound" :placeholder="t('singboxDns.outboundTagOptional')" />
        </el-form-item>
        <el-form-item :label="t('protoCenter.routing.action')">
          <el-select v-model="ruleForm.action" style="width:100%" clearable>
            <el-option label="" value="" />
            <el-option label="route" value="route" />
            <el-option label="route-options" value="route-options" />
            <el-option label="reject" value="reject" />
            <el-option label="hijack-dns" value="hijack-dns" />
            <el-option label="sniff" value="sniff" />
            <el-option label="evaluate" value="evaluate" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="ruleForm.action === 'route' || ruleForm.action === 'evaluate'" :label="t('singboxDns.speculative')">
          <el-switch v-model="ruleForm.speculative" />
          <div class="hint">{{ t('protoHint.dns.speculative') }}</div>
        </el-form-item>
        <el-form-item v-if="ruleForm.action === 'evaluate'" :label="t('singboxDns.evaluateTag')">
          <el-input v-model="ruleForm.evaluateTag" placeholder="ev" />
          <div class="hint">{{ t('protoHint.dns.evaluatePairing') }}</div>
        </el-form-item>
        <el-form-item v-if="ruleForm.action === 'evaluate'" :label="t('singboxDns.evaluateTimeout')">
          <el-input v-model="ruleForm.evaluateTimeout" placeholder="5s" />
        </el-form-item>
        <el-form-item :label="t('singboxDns.matchResponse')">
          <el-switch v-model="ruleForm.matchResponse" />
          <div class="hint">{{ t('protoHint.dns.matchResponse') }}</div>
        </el-form-item>
        <template v-if="ruleForm.matchResponse">
          <el-form-item :label="t('singboxDns.responseTag')">
            <el-input v-model="ruleForm.matchResponseTag" :placeholder="t('protoHint.dns.matchResponseTag')" />
          </el-form-item>
          <el-form-item :label="t('singboxDns.responseRCode')">
            <el-select v-model="ruleForm.responseRcode" style="width:100%" clearable>
              <el-option :label="t('common.any')" value="" /><el-option label="NOERROR" value="NOERROR" />
              <el-option label="NXDOMAIN" value="NXDOMAIN" /><el-option label="SERVFAIL" value="SERVFAIL" />
              <el-option label="REFUSED" value="REFUSED" /><el-option label="FORMERR" value="FORMERR" />
              <el-option label="NOTIMP" value="NOTIMP" />
            </el-select>
            <div class="hint">{{ t('protoHint.dns.rcodeHint') }}</div>
          </el-form-item>
          <el-form-item :label="t('singboxDns.ipAcceptAny')">
            <el-switch v-model="ruleForm.ipAcceptAny" />
            <div class="hint">{{ t('protoHint.dns.ipAcceptAny') }}</div>
          </el-form-item>
          <el-form-item :label="t('singboxDns.responseAnswer')">
            <el-input v-model="ruleForm._responseAnswer" type="textarea" :rows="2" placeholder="a.example. 300 IN A 1.2.3.4" />
            <div class="hint">{{ t('protoHint.dns.responseAnswer') }}</div>
          </el-form-item>
          <el-form-item :label="t('singboxDns.responseNs')">
            <el-input v-model="ruleForm._responseNs" type="textarea" :rows="2" placeholder="a.example. 300 IN NS ns.example." />
            <div class="hint">{{ t('protoHint.dns.responseNs') }}</div>
          </el-form-item>
          <el-form-item :label="t('singboxDns.responseExtra')">
            <el-input v-model="ruleForm._responseExtra" type="textarea" :rows="2" placeholder="a.example. 300 IN A 1.2.3.4" />
            <div class="hint">{{ t('protoHint.dns.responseExtra') }}</div>
          </el-form-item>
        </template>
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
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
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
  { value: 'udp', label: t('protoHint.dns.typeUdp') },
  { value: 'tcp', label: t('protoHint.dns.typeTcp') },
  { value: 'tls', label: 'tls — DNS over TLS' },
  { value: 'https', label: 'https — DoH' },
  { value: 'h3', label: 'h3 — DoH (HTTP/3)' },
  { value: 'quic', label: 'quic — DoQ' },
  { value: 'local', label: t('protoHint.dns.typeLocal') },
  { value: 'hosts', label: t('protoHint.dns.hostsTypeLabel') },
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
  udp: t('protoHint.dns.hintUdp'),
  tcp: t('protoHint.dns.hintTcp'),
  tls: t('protoHint.dns.hintTls'),
  https: t('protoHint.dns.hintHttps'),
  h3: t('protoHint.dns.hintH3'),
  quic: t('protoHint.dns.hintQuic'),
  local: t('protoHint.dns.hintLocal'),
  hosts: t('protoHint.dns.hintHosts'),
  fakeip: t('protoHint.dns.hintFakeip'),
  dhcp: t('protoHint.dns.hintDhcp'),
  mdns: t('protoHint.dns.hintMdns'),
  resolved: t('protoHint.dns.hintResolved'),
  tailscale: t('protoHint.dns.hintTailscale'),
  openconnect: t('protoHint.dns.hintOpenconnect'),
  openvpn: t('protoHint.dns.hintOpenvpn'),
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
// duration 字段给非法值会让核心拒绝启动（如 `time: invalid duration "abc"`）→ 生成前校验
function validDuration(v) {
  const t = String(v || '').trim()
  if (!t) return ""
  return /^(\d+(\.\d+)?(ns|us|ms|s|m|h))+$/.test(t) ? t : ""
}


const ruleForm = reactive({
  type: '', mode: '', _domain: '', _domainSuffix: '', _domainKeyword: '',
  _ipCIDR: '', _protocol: [], server: '', outbound: '', action: '',
  _queryDnssec: false, _queryClientSubnet: '', speculative: false,
  evaluateTag: '', evaluateTimeout: '', matchResponse: false, matchResponseTag: '',
  responseRcode: '', ipAcceptAny: false, _responseAnswer: '', _responseNs: '', _responseExtra: '',
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
      ElMessage.error(t('protoHint.dns.fakeipRangeRequired'))
      return
    }
  }
  if (srvForm.type === 'hosts') {
    const predefined = {}
    for (const line of (srvForm._predefinedStr || '').split('\n')) {
      const s = line.trim()
      if (!s) continue
      const eq = s.indexOf('=')
      if (eq < 0) { ElMessage.error(t('protoHint.dns.predefinedFormat') + s); return }
      const domain = s.slice(0, eq).trim()
      const ips = s.slice(eq + 1).split(',').map(x => x.trim()).filter(Boolean)
      if (!domain || !ips.length) { ElMessage.error(t('protoHint.dns.predefinedFormat') + s); return }
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
    _queryDnssec: false, _queryClientSubnet: '', speculative: false,
    evaluateTag: '', evaluateTimeout: '', matchResponse: false, matchResponseTag: '',
    responseRcode: '', ipAcceptAny: false, _responseAnswer: '', _responseNs: '', _responseExtra: '',
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
    _queryDnssec: !!r.query_dnssec,
    _queryClientSubnet: (r.query_client_subnet || []).join('\n'),
    evaluateTag: r.tag || '', evaluateTimeout: r.timeout || '',
    matchResponse: !!r.match_response,
    matchResponseTag: (typeof r.match_response === 'string' ? r.match_response : ''),
    responseRcode: r.response_rcode || '', ipAcceptAny: !!r.ip_accept_any,
    _responseAnswer: (r.response_answer || []).join('\n'),
    _responseNs: (r.response_ns || []).join('\n'),
    _responseExtra: (r.response_extra || []).join('\n'),
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
  if (ruleForm._queryDnssec) rule.query_dnssec = true
  const qcs = (ruleForm._queryClientSubnet || '').split('\n').map(x => x.trim()).filter(Boolean)
  if (qcs.length) rule.query_client_subnet = qcs
  if (ruleForm.speculative) rule.speculative = true
  if (ruleForm.action === 'evaluate') {
    if (ruleForm.evaluateTag) rule.tag = ruleForm.evaluateTag
    const et = validDuration(ruleForm.evaluateTimeout)
    if (et) rule.timeout = et
  }
  // 配对规则：evaluate 带 tag → 这里写 tag 字符串；不带 tag → 只能写 true（实测配错 FATAL）
  if (ruleForm.matchResponse) rule.match_response = ruleForm.matchResponseTag ? ruleForm.matchResponseTag : true
  if (ruleForm.responseRcode) rule.response_rcode = ruleForm.responseRcode
  if (ruleForm.ipAcceptAny) rule.ip_accept_any = true
  const rans = (ruleForm._responseAnswer || '').split('\n').map(x => x.trim()).filter(Boolean)
  if (rans.length) rule.response_answer = rans
  const rns = (ruleForm._responseNs || '').split('\n').map(x => x.trim()).filter(Boolean)
  if (rns.length) rule.response_ns = rns
  const rex = (ruleForm._responseExtra || '').split('\n').map(x => x.trim()).filter(Boolean)
  if (rex.length) rule.response_extra = rex
  if (editingRuleIdx.value >= 0) form.rules[editingRuleIdx.value] = { ...rule, _domain: ruleForm._domain, _domainSuffix: ruleForm._domainSuffix, _domainKeyword: ruleForm._domainKeyword, _ipCIDR: ruleForm._ipCIDR, _protocol: ruleForm._protocol, _queryDnssec: ruleForm._queryDnssec, _queryClientSubnet: ruleForm._queryClientSubnet, speculative: ruleForm.speculative, evaluateTag: ruleForm.evaluateTag, evaluateTimeout: ruleForm.evaluateTimeout, matchResponse: ruleForm.matchResponse, matchResponseTag: ruleForm.matchResponseTag, responseRcode: ruleForm.responseRcode, ipAcceptAny: ruleForm.ipAcceptAny, _responseAnswer: ruleForm._responseAnswer, _responseNs: ruleForm._responseNs, _responseExtra: ruleForm._responseExtra }
  else form.rules.push({ ...rule, _domain: ruleForm._domain, _domainSuffix: ruleForm._domainSuffix, _domainKeyword: ruleForm._domainKeyword, _ipCIDR: ruleForm._ipCIDR, _protocol: ruleForm._protocol, _queryDnssec: ruleForm._queryDnssec, _queryClientSubnet: ruleForm._queryClientSubnet, speculative: ruleForm.speculative, evaluateTag: ruleForm.evaluateTag, evaluateTimeout: ruleForm.evaluateTimeout, matchResponse: ruleForm.matchResponse, matchResponseTag: ruleForm.matchResponseTag, responseRcode: ruleForm.responseRcode, ipAcceptAny: ruleForm.ipAcceptAny, _responseAnswer: ruleForm._responseAnswer, _responseNs: ruleForm._responseNs, _responseExtra: ruleForm._responseExtra })
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
  // 停用的段也要把值载回表单：否则用户重新打开开关时，emit() 会用默认值
  // 覆盖掉之前保存的内容（表现为“关了再开，DNS 配置全没了”）。
  enabled.value = !props.data._disabled
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
    if (r.query_dnssec) rule.query_dnssec = true
    if (Array.isArray(r.query_client_subnet) && r.query_client_subnet.length) rule.query_client_subnet = [...r.query_client_subnet]
    if (r.speculative) rule.speculative = true
    if (r.tag) rule.tag = r.tag
    const et2 = validDuration(r.timeout)
    if (et2) rule.timeout = et2
    if (r.matchResponse) rule.match_response = r.matchResponseTag ? r.matchResponseTag : true
    if (r.responseRcode) rule.response_rcode = r.responseRcode
    if (r.ipAcceptAny) rule.ip_accept_any = true
    const ra2 = (r._responseAnswer || '').split('\n').map(x => x.trim()).filter(Boolean)
    if (ra2.length) rule.response_answer = ra2
    const rn2 = (r._responseNs || '').split('\n').map(x => x.trim()).filter(Boolean)
    if (rn2.length) rule.response_ns = rn2
    const re2 = (r._responseExtra || '').split('\n').map(x => x.trim()).filter(Boolean)
    if (re2.length) rule.response_extra = re2
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
