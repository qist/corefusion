<template>
  <div>
    <el-form label-width="140px" label-position="left">
      <el-form-item label="Final Outbound">
        <el-input v-model="form.final" placeholder="direct" />
      </el-form-item>
      <el-form-item label="Auto Detect Interface">
        <el-switch v-model="form.auto_detect_interface" />
      </el-form-item>
    </el-form>

    <div class="sub-section">
      <div class="sub-header">
        <span>Route Rules</span>
        <el-button size="small" type="primary" @click="addRule">Add Rule</el-button>
      </div>
      <div v-for="(rule, i) in form.rules" :key="i" class="rule-item">
        <el-card shadow="never">
          <div class="rule-row">
            <el-input v-model="rule._domain" placeholder="Domain suffixes (comma-sep)" style="width:30%" />
            <el-input v-model="rule._ip" placeholder="IP CIDRs (comma-sep)" style="width:25%" />
            <el-select v-model="rule.protocol" multiple placeholder="Protocol" style="width:20%">
              <el-option label="dns" value="dns" /><el-option label="http" value="http" />
              <el-option label="tls" value="tls" /><el-option label="quic" value="quic" />
            </el-select>
            <el-input v-model="rule.outbound" placeholder="Outbound" style="width:15%" />
            <el-button size="small" type="danger" text @click="form.rules.splice(i, 1)">X</el-button>
          </div>
        </el-card>
      </div>
    </div>
  </div>
</template>
<script setup>
import { reactive, watch, onMounted } from 'vue'
const props = defineProps({ data: Object })
const emit = defineEmits(['update'])
const form = reactive({ final: 'direct', auto_detect_interface: true, rules: [] })

function syncFromData() {
  if (!props.data) return
  form.final = props.data.final || 'direct'
  form.auto_detect_interface = props.data.auto_detect_interface !== false
  form.rules = (props.data.rules || []).map(r => ({
    ...r, _domain: (r.domain_suffix || r.domain || []).join(', '),
    _ip: (r.ip_cidr || r.geoip ? ['geoip:' + (r.geoip || [])] : []).join(', '),
  }))
}

function buildOutput() {
  return {
    final: form.final, auto_detect_interface: form.auto_detect_interface,
    rules: form.rules.filter(r => r.outbound).map(r => {
      const rule = { outbound: r.outbound }
      if (r.protocol?.length) rule.protocol = r.protocol
      const domains = r._domain ? r._domain.split(',').map(d => d.trim()).filter(Boolean) : []
      if (domains.length) rule.domain_suffix = domains
      return rule
    }),
  }
}

function addRule() { form.rules.push({ _domain: '', _ip: '', protocol: [], outbound: '' }) }

watch(() => props.data, syncFromData, { deep: true })
watch(form, () => emit('update', buildOutput()), { deep: true })
onMounted(syncFromData)
</script>
<style scoped>
.sub-section { margin-top: 20px; }
.sub-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; font-weight: 600; }
.rule-item { margin-bottom: 12px; }
.rule-row { display: flex; gap: 8px; align-items: center; }
</style>
