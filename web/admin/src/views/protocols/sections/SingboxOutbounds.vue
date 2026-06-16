<template>
  <div>
    <div class="sub-header">
      <span>Outbound List</span>
      <el-button size="small" type="primary" @click="addOutbound">Add Outbound</el-button>
    </div>
    <div v-for="(ob, i) in form" :key="i" class="item-card">
      <el-card shadow="never">
        <div class="card-header">
          <span>#{{ i + 1 }} {{ ob.tag }}</span>
          <el-button size="small" type="danger" text @click="form.splice(i, 1)">Remove</el-button>
        </div>
        <el-form label-width="120px" label-position="left" size="small">
          <el-form-item label="Type">
            <el-select v-model="ob.type" style="width:100%">
              <el-option label="direct" value="direct" /><el-option label="block" value="block" />
              <el-option label="dns" value="dns" /><el-option label="selector" value="selector" />
              <el-option label="urltest" value="urltest" />
              <el-option label="vmess" value="vmess" /><el-option label="vless" value="vless" />
              <el-option label="trojan" value="trojan" /><el-option label="hysteria2" value="hysteria2" />
              <el-option label="shadowsocks" value="shadowsocks" /><el-option label="tuic" value="tuic" />
              <el-option label="wireguard" value="wireguard" />
            </el-select>
          </el-form-item>
          <el-form-item label="Tag">
            <el-input v-model="ob.tag" />
          </el-form-item>
          <template v-if="['vmess','vless','trojan','shadowsocks','hysteria2','tuic'].includes(ob.type)">
            <el-form-item label="Server">
              <el-input v-model="ob.server" placeholder="example.com" />
            </el-form-item>
            <el-form-item label="Server Port">
              <el-input-number v-model="ob.server_port" :min="1" :max="65535" />
            </el-form-item>
          </template>
          <template v-if="ob.type === 'vmess' || ob.type === 'vless'">
            <el-form-item label="UUID">
              <el-input v-model="ob.uuid" />
            </el-form-item>
            <el-form-item label="Flow" v-if="ob.type === 'vless'">
              <el-select v-model="ob.flow" style="width:100%" clearable>
                <el-option label="none" value="" /><el-option label="xtls-rprx-vision" value="xtls-rprx-vision" />
              </el-select>
            </el-form-item>
          </template>
          <template v-if="ob.type === 'trojan' || ob.type === 'hysteria2'">
            <el-form-item label="Password">
              <el-input v-model="ob.password" />
            </el-form-item>
          </template>
          <template v-if="ob.type === 'selector' || ob.type === 'urltest'">
            <el-form-item label="Outbounds">
              <el-input v-model="ob._outboundsStr" placeholder="Comma-separated tags" />
            </el-form-item>
            <el-form-item label="Default" v-if="ob.type === 'selector'">
              <el-input v-model="ob.default" />
            </el-form-item>
          </template>
          <template v-if="['vmess','vless','trojan','hysteria2'].includes(ob.type)">
            <el-form-item label="TLS">
              <el-switch v-model="ob._tlsEnabled" />
            </el-form-item>
            <template v-if="ob._tlsEnabled">
              <el-form-item label="SNI">
                <el-input v-model="ob._sni" placeholder="example.com" />
              </el-form-item>
              <el-form-item label="Fingerprint">
                <el-select v-model="ob._fingerprint" style="width:100%" clearable>
                  <el-option label="chrome" value="chrome" /><el-option label="firefox" value="firefox" />
                </el-select>
              </el-form-item>
            </template>
          </template>
        </el-form>
      </el-card>
    </div>
  </div>
</template>
<script setup>
import { reactive, watch, onMounted } from 'vue'
const props = defineProps({ data: Array })
const emit = defineEmits(['update'])
const form = reactive([])

function syncFromData() {
  form.splice(0, form.length)
  if (!Array.isArray(props.data)) return
  props.data.forEach(ob => form.push({
    ...ob, _tlsEnabled: ob.tls?.enabled || false,
    _sni: ob.tls?.server_name || '', _fingerprint: ob.tls?.utls?.fingerprint || '',
    _outboundsStr: (ob.outbounds || []).join(', '),
  }))
}

function buildOutbound(ob) {
  const out = { ...ob }
  delete out._tlsEnabled; delete out._sni; delete out._fingerprint; delete out._outboundsStr
  if (ob._tlsEnabled) {
    out.tls = { enabled: true, server_name: ob._sni || undefined }
    if (ob._fingerprint) out.tls.utls = { enabled: true, fingerprint: ob._fingerprint }
  } else {
    delete out.tls
  }
  if (ob._outboundsStr) out.outbounds = ob._outboundsStr.split(',').map(s => s.trim()).filter(Boolean)
  return out
}

function addOutbound() {
  form.push({ type: 'direct', tag: 'direct', server: '', server_port: 443, _tlsEnabled: false, _sni: '', _fingerprint: '', _outboundsStr: '' })
}

watch(() => props.data, syncFromData, { deep: true })
watch(form, () => emit('update', form.map(buildOutbound)), { deep: true })
onMounted(syncFromData)
</script>
<style scoped>
.sub-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; font-weight: 600; }
.item-card { margin-bottom: 16px; }
.card-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; font-weight: 600; }
</style>
