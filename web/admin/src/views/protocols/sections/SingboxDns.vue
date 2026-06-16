<template>
  <div>
    <el-form label-width="120px" label-position="left">
      <el-form-item label="Strategy">
        <el-select v-model="form.strategy" style="width:100%" clearable>
          <el-option label="prefer_ipv4" value="prefer_ipv4" /><el-option label="prefer_ipv6" value="prefer_ipv6" />
          <el-option label="ipv4_only" value="ipv4_only" /><el-option label="ipv6_only" value="ipv6_only" />
        </el-select>
      </el-form-item>
      <el-form-item label="Disable Cache">
        <el-switch v-model="form.disable_cache" />
      </el-form-item>
      <el-form-item label="Independent Cache">
        <el-switch v-model="form.independent_cache" />
      </el-form-item>
      <el-form-item label="Final Server">
        <el-input v-model="form.final" placeholder="Tag of default DNS server" />
      </el-form-item>
    </el-form>

    <div class="sub-section">
      <div class="sub-header">
        <span>DNS Servers</span>
        <el-button size="small" type="primary" @click="addServer">Add Server</el-button>
      </div>
      <div v-for="(srv, i) in form.servers" :key="i" class="sub-item">
        <el-input v-model="srv.tag" placeholder="Tag" style="width:15%" />
        <el-input v-model="srv.address" placeholder="tls://8.8.8.8" style="width:35%" />
        <el-input v-model="srv.detour" placeholder="Detour outbound" style="width:25%" />
        <el-button size="small" type="danger" text @click="form.servers.splice(i, 1)">Remove</el-button>
      </div>
    </div>

    <div class="sub-section">
      <div class="sub-header">
        <span>DNS Rules</span>
        <el-button size="small" type="primary" @click="addRule">Add Rule</el-button>
      </div>
      <div v-for="(rule, i) in form.rules" :key="i" class="sub-item">
        <el-input v-model="rule._domain" placeholder="Domains (comma-sep)" style="width:35%" />
        <el-input v-model="rule.server" placeholder="Server tag" style="width:20%" />
        <el-button size="small" type="danger" text @click="form.rules.splice(i, 1)">Remove</el-button>
      </div>
    </div>
  </div>
</template>
<script setup>
import { reactive, watch, onMounted } from 'vue'
const props = defineProps({ data: Object })
const emit = defineEmits(['update'])
const form = reactive({ strategy: 'prefer_ipv4', disable_cache: false, independent_cache: false, final: '', servers: [], rules: [] })

function syncFromData() {
  if (!props.data) return
  form.strategy = props.data.strategy || 'prefer_ipv4'
  form.disable_cache = props.data.disable_cache || false
  form.independent_cache = props.data.independent_cache || false
  form.final = props.data.final || ''
  form.servers = (props.data.servers || []).map(s => ({ ...s }))
  form.rules = (props.data.rules || []).map(r => ({ ...r, _domain: (r.domain || r.domain_suffix || []).join(', ') }))
}

function buildOutput() {
  return {
    strategy: form.strategy, disable_cache: form.disable_cache,
    independent_cache: form.independent_cache, final: form.final,
    servers: form.servers.filter(s => s.address).map(s => ({ tag: s.tag, address: s.address, detour: s.detour || undefined })),
    rules: form.rules.filter(r => r.server).map(r => {
      const domains = r._domain ? r._domain.split(',').map(d => d.trim()).filter(Boolean) : []
      return { domain_suffix: domains.length ? domains : undefined, server: r.server }
    }),
  }
}

function addServer() { form.servers.push({ tag: '', address: '', detour: '' }) }
function addRule() { form.rules.push({ _domain: '', server: '' }) }

watch(() => props.data, syncFromData, { deep: true })
watch(form, () => emit('update', buildOutput()), { deep: true })
onMounted(syncFromData)
</script>
<style scoped>
.sub-section { margin-top: 20px; }
.sub-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; font-weight: 600; }
.sub-item { display: flex; gap: 8px; margin-bottom: 8px; align-items: center; }
</style>
