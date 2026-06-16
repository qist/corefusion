<template>
  <div>
    <el-form label-width="160px" label-position="left">
      <el-form-item label="Enable DNS">
        <el-switch v-model="enabled" @change="onToggle" />
        <span class="hint">Disable to remove DNS section from config</span>
      </el-form-item>
    </el-form>

    <template v-if="enabled">
      <el-divider content-position="left">Global Settings</el-divider>
      <el-form label-width="160px" label-position="left">
        <el-form-item label="Query Strategy">
          <el-select v-model="form.queryStrategy" style="width:100%" @change="emit">
            <el-option label="UseIP" value="UseIP" /><el-option label="UseIPv4" value="UseIPv4" /><el-option label="UseIPv6" value="UseIPv6" />
          </el-select>
        </el-form-item>
        <el-form-item label="Client IP (EDNS)">
          <el-input v-model="form.clientIp" placeholder="e.g. 1.2.3.4" @change="emit" />
        </el-form-item>
        <el-form-item label="Disable Cache">
          <el-switch v-model="form.disableCache" @change="emit" />
        </el-form-item>
        <el-form-item label="Serve Stale">
          <el-switch v-model="form.serveStale" @change="emit" />
          <span class="hint">Serve stale cache entries</span>
        </el-form-item>
        <el-form-item label="Serve Expired TTL">
          <el-input-number v-model="form.serveExpiredTTL" :min="0" @change="emit" />
          <span class="hint">Seconds to serve expired entries (0 = disabled)</span>
        </el-form-item>
        <el-form-item label="Disable Fallback">
          <el-switch v-model="form.disableFallback" @change="emit" />
        </el-form-item>
        <el-form-item label="Disable Fallback If Match">
          <el-switch v-model="form.disableFallbackIfMatch" @change="emit" />
        </el-form-item>
        <el-form-item label="Enable Parallel Query">
          <el-switch v-model="form.enableParallelQuery" @change="emit" />
        </el-form-item>
        <el-form-item label="Use System Hosts">
          <el-switch v-model="form.useSystemHosts" @change="emit" />
        </el-form-item>
        <el-form-item label="Tag">
          <el-input v-model="form.tag" placeholder="dns_inbound" @change="emit" />
        </el-form-item>
      </el-form>

      <el-divider content-position="left">DNS Servers</el-divider>
      <div v-for="(s, i) in form.servers" :key="i" class="server-block">
        <div class="server-header">
          <span>Server #{{ i + 1 }}</span>
          <el-button size="small" type="danger" text @click="rmServer(i)">Remove</el-button>
        </div>
        <el-form label-width="140px" label-position="left" size="small">
          <el-form-item label="Address">
            <el-input v-model="s.address" placeholder="8.8.8.8 or tls://1.1.1.1" @change="emit" />
          </el-form-item>
          <el-form-item label="Port">
            <el-input-number v-model="s.port" :min="0" :max="65535" @change="emit" />
            <span class="hint">0 = default 53</span>
          </el-form-item>
          <el-form-item label="Domains">
            <el-input v-model="s.domainsStr" placeholder="domain:xray.com, geosite:netflix" @change="emit" />
          </el-form-item>
          <el-form-item label="Expected IPs">
            <el-input v-model="s.expectIPsStr" placeholder="geoip:cn" @change="emit" />
          </el-form-item>
          <el-form-item label="Unexpected IPs">
            <el-input v-model="s.unexpectedIPsStr" placeholder="geoip:cn" @change="emit" />
          </el-form-item>
          <el-form-item label="Client IP">
            <el-input v-model="s.clientIp" placeholder="Override EDNS client IP" @change="emit" />
          </el-form-item>
          <el-form-item label="Query Strategy">
            <el-select v-model="s.queryStrategy" style="width:100%" clearable @change="emit">
              <el-option label="Inherit" value="" /><el-option label="UseIP" value="UseIP" /><el-option label="UseIPv4" value="UseIPv4" /><el-option label="UseIPv6" value="UseIPv6" />
            </el-select>
          </el-form-item>
          <el-form-item label="Skip Fallback">
            <el-switch v-model="s.skipFallback" @change="emit" />
          </el-form-item>
          <el-form-item label="Final Query">
            <el-switch v-model="s.finalQuery" @change="emit" />
            <span class="hint">Truncate server list after this</span>
          </el-form-item>
          <el-form-item label="Disable Cache">
            <el-switch v-model="s.disableCache" @change="emit" />
          </el-form-item>
          <el-form-item label="Tag">
            <el-input v-model="s.tag" @change="emit" />
          </el-form-item>
        </el-form>
      </div>
      <el-button size="small" type="primary" @click="addServer">Add DNS Server</el-button>

      <el-divider content-position="left">Static Hosts</el-divider>
      <div v-for="(h, i) in hostsList" :key="i" class="sub-item">
        <el-input v-model="h.domain" placeholder="domain:example.com" style="width:40%" @change="emit" />
        <el-input v-model="h.ip" placeholder="1.2.3.4 or array" style="width:40%" @change="emit" />
        <el-button size="small" type="danger" text @click="rmHost(i)">X</el-button>
      </div>
      <el-button size="small" @click="addHost">Add Host</el-button>
    </template>
  </div>
</template>
<script setup>
import { reactive, ref, computed, onMounted } from 'vue'
const props = defineProps({ data: Object })
const emitFn = defineEmits(['update'])
const enabled = ref(true)
const form = reactive({
  queryStrategy: 'UseIP', clientIp: '', disableCache: false, serveStale: false, serveExpiredTTL: 0,
  disableFallback: false, disableFallbackIfMatch: false, enableParallelQuery: false, useSystemHosts: true, tag: '',
  servers: [], hosts: {},
})
const hostsList = computed(() => Object.entries(form.hosts).map(([d, i]) => ({ domain: d, ip: i })))

function sync() {
  if (!props.data || Object.keys(props.data).length === 0) { enabled.value = false; return }
  enabled.value = true
  form.queryStrategy = props.data.queryStrategy || 'UseIP'
  form.clientIp = props.data.clientIp || ''
  form.disableCache = props.data.disableCache || false
  form.serveStale = props.data.serveStale || false
  form.serveExpiredTTL = props.data.serveExpiredTTL || 0
  form.disableFallback = props.data.disableFallback || false
  form.disableFallbackIfMatch = props.data.disableFallbackIfMatch || false
  form.enableParallelQuery = props.data.enableParallelQuery || false
  form.useSystemHosts = props.data.useSystemHosts !== false
  form.tag = props.data.tag || ''
  form.servers = (props.data.servers || []).map(function(s) {
    if (typeof s === 'string') return { address: s, port: 0, domainsStr: '', expectIPsStr: '', unexpectedIPsStr: '', clientIp: '', queryStrategy: '', skipFallback: false, finalQuery: false, disableCache: false, tag: '' }
    return {
      address: s.address || '', port: s.port || 0,
      domainsStr: (s.domains || []).join(', '), expectIPsStr: (s.expectedIPs || []).join(', '),
      unexpectedIPsStr: (s.unexpectedIPs || []).join(', '), clientIp: s.clientIp || '',
      queryStrategy: s.queryStrategy || '', skipFallback: s.skipFallback || false,
      finalQuery: s.finalQuery || false, disableCache: s.disableCache || false, tag: s.tag || '',
    }
  })
  form.hosts = props.data.hosts || {}
}

function build() {
  return {
    queryStrategy: form.queryStrategy, clientIp: form.clientIp || undefined,
    disableCache: form.disableCache, serveStale: form.serveStale || undefined,
    serveExpiredTTL: form.serveExpiredTTL || undefined,
    disableFallback: form.disableFallback, disableFallbackIfMatch: form.disableFallbackIfMatch,
    enableParallelQuery: form.enableParallelQuery || undefined,
    useSystemHosts: form.useSystemHosts, tag: form.tag || undefined,
    servers: form.servers.filter(function(s) { return s.address }).map(function(s) {
      return {
        address: s.address, port: s.port || undefined,
        domains: s.domainsStr ? s.domainsStr.split(',').map(function(d) { return d.trim() }).filter(Boolean) : undefined,
        expectedIPs: s.expectIPsStr ? s.expectIPsStr.split(',').map(function(d) { return d.trim() }).filter(Boolean) : undefined,
        unexpectedIPs: s.unexpectedIPsStr ? s.unexpectedIPsStr.split(',').map(function(d) { return d.trim() }).filter(Boolean) : undefined,
        clientIp: s.clientIp || undefined, queryStrategy: s.queryStrategy || undefined,
        skipFallback: s.skipFallback || undefined, finalQuery: s.finalQuery || undefined,
        disableCache: s.disableCache || undefined, tag: s.tag || undefined,
      }
    }),
    hosts: Object.keys(form.hosts).length ? form.hosts : undefined,
  }
}

function onToggle() { if (!enabled.value) emitFn('update', null); else emit() }
function emit() { if (enabled.value) emitFn('update', build()) }
function addServer() {
  form.servers.push({ address: '', port: 0, domainsStr: '', expectIPsStr: '', unexpectedIPsStr: '', clientIp: '', queryStrategy: '', skipFallback: false, finalQuery: false, disableCache: false, tag: '' })
  emit()
}
function rmServer(i) { form.servers.splice(i, 1); emit() }
function addHost() { form.hosts[''] = ''; emit() }
function rmHost(i) { delete form.hosts[Object.keys(form.hosts)[i]]; emit() }
onMounted(sync)
</script>
<style scoped>
.hint { margin-left: 8px; font-size: 12px; color: var(--el-text-color-secondary); }
.sub-section { margin-top: 20px; }
.sub-item { display: flex; gap: 6px; margin-bottom: 8px; align-items: center; }
.server-block { background: var(--el-fill-color-light); border-radius: 6px; padding: 12px; margin-bottom: 12px; }
.server-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; font-weight: 600; }
</style>
