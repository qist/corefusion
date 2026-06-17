<template>
  <div>
    <el-form label-width="160px" label-position="left">
      <el-form-item :label="t('protoCenter.dns.enable')">
        <el-switch v-model="enabled" @change="onToggle" />
      </el-form-item>
    </el-form>

    <template v-if="enabled">
      <!-- Global Settings -->
      <el-divider content-position="left">{{ t('protoCenter.dns.globalSettings') }}</el-divider>
      <el-form label-width="160px" label-position="left">
        <el-form-item :label="t('protoCenter.dns.queryStrategy')">
          <el-select v-model="form.queryStrategy" style="width:100%" @change="emit">
            <el-option label="UseIP (A+AAAA)" value="UseIP" />
            <el-option label="UseIPv4 (A only)" value="UseIPv4" />
            <el-option label="UseIPv6 (AAAA only)" value="UseIPv6" />
            <el-option label="UseSystem" value="UseSystem" />
          </el-select>
          <div class="hint">{{ t('protoCenter.dns.queryStrategyHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('protoCenter.dns.disableCache')">
          <el-switch v-model="form.disableCache" @change="emit" />
        </el-form-item>
        <el-form-item :label="t('protoCenter.dns.serveStale')">
          <el-switch v-model="form.serveStale" @change="emit" />
          <div class="hint">{{ t('protoCenter.dns.serveStaleHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('protoCenter.dns.serveExpiredTTL')">
          <el-input-number v-model="form.serveExpiredTTL" :min="0" @change="emit" />
          <div class="hint">{{ t('protoCenter.dns.serveExpiredTTLHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('protoCenter.dns.disableFallback')">
          <el-switch v-model="form.disableFallback" @change="emit" />
          <div class="hint">{{ t('protoCenter.dns.disableFallbackHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('protoCenter.dns.disableFallbackIfMatch')">
          <el-switch v-model="form.disableFallbackIfMatch" @change="emit" />
          <div class="hint">{{ t('protoCenter.dns.disableFallbackIfMatchHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('protoCenter.dns.enableParallelQuery')">
          <el-switch v-model="form.enableParallelQuery" @change="emit" />
          <div class="hint">{{ t('protoCenter.dns.enableParallelQueryHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('protoCenter.dns.useSystemHosts')">
          <el-switch v-model="form.useSystemHosts" @change="emit" />
          <div class="hint">{{ t('protoCenter.dns.useSystemHostsHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('protoCenter.dns.clientIp')">
          <el-input v-model="form.clientIp" placeholder="1.2.3.4" @change="emit" />
          <div class="hint">{{ t('protoCenter.dns.clientIpHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('protoCenter.dns.tag')">
          <el-input v-model="form.tag" placeholder="dns_inbound" @change="emit" />
          <div class="hint">{{ t('protoCenter.dns.tagHint') }}</div>
        </el-form-item>
      </el-form>

      <!-- Simple Servers -->
      <el-divider content-position="left">
        {{ t('protoCenter.dns.dnsServers') }}
        <el-button size="small" @click="addSimple" style="margin-left:12px">{{ t('protoCenter.dns.addServer') }}</el-button>
      </el-divider>
      <div class="hint" style="margin-bottom:8px">{{ t('protoCenter.dns.simpleServerHint') }}</div>
      <div v-for="(s, i) in form.simpleServers" :key="'s'+i" style="display:flex;gap:8px;margin-bottom:8px">
        <el-input v-model="form.simpleServers[i]" placeholder="https://1.1.1.1/dns-query or 8.8.8.8" style="flex:1" @change="emit" />
        <el-button size="small" type="danger" text @click="rmSimple(i)">X</el-button>
      </div>

      <!-- Rule Servers -->
      <el-divider content-position="left">
        {{ t('protoCenter.dns.ruleServers') }}
        <el-button size="small" type="primary" @click="addRule" style="margin-left:12px">{{ t('protoCenter.dns.addRule') }}</el-button>
      </el-divider>
      <div class="hint" style="margin-bottom:8px">{{ t('protoCenter.dns.ruleServerHint') }}</div>

      <div v-for="(s, i) in form.ruleServers" :key="'r'+i" class="server-block">
        <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:8px">
          <span style="font-weight:600;font-size:13px">{{ t('protoCenter.dns.ruleServer') }} #{{ i + 1 }}</span>
          <el-button size="small" type="danger" text @click="rmRule(i)">{{ t('common.delete') }}</el-button>
        </div>
        <el-form label-width="120px" label-position="left" size="small">
          <el-form-item :label="t('protoCenter.dns.dnsServers')">
            <el-input v-model="s.address" placeholder="https://8.8.8.8/dns-query" @change="emit" />
          </el-form-item>
          <el-form-item :label="t('protoCenter.dns.domains')">
            <el-input
              v-model="s._domainsStr"
              type="textarea"
              :rows="2"
              placeholder="geosite:openai&#10;domain:example.com&#10;keyword:google"
              @change="onDomainsChange(s)"
            />
            <div class="hint">{{ t('protoCenter.dns.domainsHint') }}</div>
          </el-form-item>
          <el-form-item :label="t('protoCenter.dns.queryStrategy')">
            <el-select v-model="s.queryStrategy" style="width:100%" clearable @change="emit">
              <el-option label="Inherit" value="" />
              <el-option label="UseIP (A+AAAA)" value="UseIP" />
              <el-option label="UseIPv4 (A only)" value="UseIPv4" />
              <el-option label="UseIPv6 (AAAA only)" value="UseIPv6" />
              <el-option label="UseSystem" value="UseSystem" />
            </el-select>
          </el-form-item>
          <el-form-item :label="t('protoCenter.dns.expectedIPs')">
            <el-input
              v-model="s._expectedIPsStr"
              type="textarea"
              :rows="2"
              placeholder="geoip:cn&#10;geoip:us"
              @change="onExpectedIPsChange(s)"
            />
            <div class="hint">{{ t('protoCenter.dns.expectedIPsHint') }}</div>
          </el-form-item>
          <el-form-item :label="t('protoCenter.dns.unexpectedIPs')">
            <el-input
              v-model="s._unexpectedIPsStr"
              type="textarea"
              :rows="2"
              placeholder="geoip:cloudflare"
              @change="onUnexpectedIPsChange(s)"
            />
            <div class="hint">{{ t('protoCenter.dns.unexpectedIPsHint') }}</div>
          </el-form-item>
          <el-form-item :label="t('protoCenter.dns.clientIP')">
            <el-input v-model="s.clientIP" placeholder="1.2.3.4" @change="emit" />
            <div class="hint">{{ t('protoCenter.dns.clientIPHint') }}</div>
          </el-form-item>
          <el-form-item :label="t('protoCenter.dns.port')">
            <el-input-number v-model="s.port" :min="0" :max="65535" @change="emit" />
            <div class="hint">{{ t('protoCenter.dns.portHint') }}</div>
          </el-form-item>
          <el-form-item :label="t('protoCenter.dns.timeoutMs')">
            <el-input-number v-model="s.timeoutMs" :min="0" @change="emit" />
            <div class="hint">{{ t('protoCenter.dns.timeoutMsHint') }}</div>
          </el-form-item>
          <el-form-item :label="t('protoCenter.dns.tag')">
            <el-input v-model="s.tag" placeholder="dns-tag" @change="emit" />
            <div class="hint">{{ t('protoCenter.dns.tagHint') }}</div>
          </el-form-item>
          <div style="display:flex;gap:16px;margin-top:4px">
            <el-checkbox v-model="s.skipFallback" @change="emit">{{ t('protoCenter.dns.skipFallback') }}</el-checkbox>
            <el-checkbox v-model="s.finalQuery" @change="emit">{{ t('protoCenter.dns.finalQuery') }}</el-checkbox>
            <el-checkbox v-model="s.disableCache" @change="emit">{{ t('protoCenter.dns.disableCachePerServer') }}</el-checkbox>
          </div>
        </el-form>
      </div>

      <!-- Static Hosts -->
      <el-divider content-position="left">
        {{ t('protoCenter.dns.staticHosts') }}
        <el-button size="small" type="primary" @click="addHost" style="margin-left:12px">{{ t('protoCenter.dns.addHost') }}</el-button>
      </el-divider>
      <div class="hint" style="margin-bottom:8px">{{ t('protoCenter.dns.staticHostsHint') }}</div>
      <div v-for="(h, i) in form.hostsList" :key="i" class="sub-item">
        <el-input v-model="h.domain" placeholder="domain:example.com" style="width:40%" @change="emit" />
        <el-input v-model="h.ip" placeholder="1.2.3.4" style="width:40%" @change="emit" />
        <el-button size="small" type="danger" text @click="rmHost(i)">X</el-button>
      </div>
    </template>
  </div>
</template>

<script setup>
import { reactive, ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
const props = defineProps({ data: Object })
const emitFn = defineEmits(['update'])
const enabled = ref(true)
const form = reactive({
  queryStrategy: 'UseIP', clientIp: '', tag: '',
  disableCache: false, serveStale: false, serveExpiredTTL: 0,
  disableFallback: false, disableFallbackIfMatch: false,
  enableParallelQuery: false, useSystemHosts: true,
  simpleServers: [], ruleServers: [], hostsList: [],
})

function parseServer(s) {
  if (typeof s === 'string') {
    return { _type: 'simple', address: s }
  }
  return {
    _type: 'rule',
    address: s.address || '',
    _domainsStr: (s.domains || []).join('\n'),
    queryStrategy: s.queryStrategy || '',
    _expectedIPsStr: (s.expectedIPs || []).join('\n'),
    _unexpectedIPsStr: (s.unexpectedIPs || []).join('\n'),
    clientIP: s.clientIP || '',
    port: s.port || 0,
    timeoutMs: s.timeoutMs || 0,
    tag: s.tag || '',
    skipFallback: s.skipFallback || false,
    finalQuery: s.finalQuery || false,
    disableCache: s.disableCache || false,
  }
}

function buildServer(s) {
  if (s._type === 'simple') return s.address
  const obj = { address: s.address }
  if (s._domainsStr) obj.domains = s._domainsStr.split('\n').map(d => d.trim()).filter(Boolean)
  if (s.queryStrategy) obj.queryStrategy = s.queryStrategy
  if (s._expectedIPsStr) obj.expectedIPs = s._expectedIPsStr.split('\n').map(d => d.trim()).filter(Boolean)
  if (s._unexpectedIPsStr) obj.unexpectedIPs = s._unexpectedIPsStr.split('\n').map(d => d.trim()).filter(Boolean)
  if (s.clientIP) obj.clientIP = s.clientIP
  if (s.port) obj.port = s.port
  if (s.timeoutMs) obj.timeoutMs = s.timeoutMs
  if (s.tag) obj.tag = s.tag
  if (s.skipFallback) obj.skipFallback = true
  if (s.finalQuery) obj.finalQuery = true
  if (s.disableCache) obj.disableCache = true
  return obj
}

function sync() {
  if (!props.data || Object.keys(props.data).length === 0) { enabled.value = false; return }
  enabled.value = true
  form.queryStrategy = props.data.queryStrategy || 'UseIP'
  form.clientIp = props.data.clientIp || ''
  form.tag = props.data.tag || ''
  form.disableCache = props.data.disableCache || false
  form.serveStale = props.data.serveStale || false
  form.serveExpiredTTL = props.data.serveExpiredTTL || 0
  form.disableFallback = props.data.disableFallback || false
  form.disableFallbackIfMatch = props.data.disableFallbackIfMatch || false
  form.enableParallelQuery = props.data.enableParallelQuery || false
  form.useSystemHosts = props.data.useSystemHosts !== false
  const all = (props.data.servers || []).map(parseServer)
  form.simpleServers = all.filter(s => s._type === 'simple').map(s => s.address)
  form.ruleServers = all.filter(s => s._type === 'rule')
  // Convert hosts object to array (supports string and array values)
  const hosts = props.data.hosts || {}
  form.hostsList = Object.entries(hosts).map(([domain, ip]) => ({
    domain,
    ip: Array.isArray(ip) ? ip.join(', ') : ip,
  }))
}

function build() {
  const servers = [
    ...form.simpleServers.filter(s => s),
    ...form.ruleServers.filter(s => s.address).map(buildServer),
  ]
  // Convert hosts array to object (supports single IP string or multiple IP array)
  const hosts = {}
  form.hostsList.forEach(h => {
    if (!h.domain) return
    const ips = h.ip.split(',').map(s => s.trim()).filter(Boolean)
    hosts[h.domain] = ips.length === 1 ? ips[0] : ips
  })
  return {
    queryStrategy: form.queryStrategy,
    clientIp: form.clientIp || undefined,
    tag: form.tag || undefined,
    disableCache: form.disableCache || undefined,
    serveStale: form.serveStale || undefined,
    serveExpiredTTL: form.serveExpiredTTL || undefined,
    disableFallback: form.disableFallback || undefined,
    disableFallbackIfMatch: form.disableFallbackIfMatch || undefined,
    enableParallelQuery: form.enableParallelQuery || undefined,
    useSystemHosts: form.useSystemHosts,
    servers,
    hosts: Object.keys(hosts).length ? hosts : undefined,
  }
}

function onToggle() {
  if (!enabled.value) {
    emitFn('update', null)
  } else {
    if (form.simpleServers.length === 0 && form.ruleServers.length === 0) {
      form.simpleServers = ['https://1.1.1.1/dns-query', '8.8.8.8', 'https://8.8.8.8/dns-query']
    }
    emit()
  }
}

function emit() { if (enabled.value) emitFn('update', build()) }
function addSimple() { form.simpleServers.push(''); emit() }
function rmSimple(i) { form.simpleServers.splice(i, 1); emit() }
function addRule() {
  form.ruleServers.push({
    _type: 'rule', address: '', _domainsStr: '', queryStrategy: '',
    _expectedIPsStr: '', _unexpectedIPsStr: '',
    clientIP: '', port: 0, timeoutMs: 0, tag: '',
    skipFallback: false, finalQuery: false, disableCache: false,
  })
  emit()
}
function rmRule(i) { form.ruleServers.splice(i, 1); emit() }
function onDomainsChange(s) { s.domains = s._domainsStr.split('\n').map(d => d.trim()).filter(Boolean); emit() }
function addHost() { form.hostsList.push({ domain: '', ip: '' }); emit() }
function rmHost(i) { form.hostsList.splice(i, 1); emit() }
onMounted(sync)
</script>

<style scoped>
.server-block { background: var(--el-fill-color-light); border-radius: 6px; padding: 12px; margin-bottom: 12px; }
.sub-item { display: flex; gap: 6px; margin-bottom: 8px; align-items: center; }
.hint { font-size: 12px; color: var(--el-text-color-secondary); margin-top: 2px; }
</style>
