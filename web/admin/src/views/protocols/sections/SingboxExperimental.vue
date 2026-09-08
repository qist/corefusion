<template>
  <div>
    <!-- Clash API -->
    <el-divider content-position="left">Clash API</el-divider>
    <el-form label-width="160px" label-position="left">
      <el-form-item label="External Controller">
        <el-input v-model="clashApi.external_controller" placeholder="127.0.0.1:9090" @change="emit" />
        <div class="hint">Clash API listening address</div>
      </el-form-item>
      <el-form-item label="Secret">
        <el-input v-model="clashApi.secret" placeholder="Optional auth secret" @change="emit" />
      </el-form-item>
      <el-form-item label="Default Mode">
        <el-select v-model="clashApi.default_mode" style="width:100%" @change="emit">
          <el-option label="rule" value="rule" />
          <el-option label="global" value="global" />
          <el-option label="direct" value="direct" />
        </el-select>
      </el-form-item>
      <el-form-item label="External UI">
        <el-input v-model="clashApi.external_ui" placeholder="/path/to/ui" @change="emit" />
        <div class="hint">Path to Clash dashboard static files</div>
      </el-form-item>
      <el-form-item label="External UI URL">
        <el-input v-model="clashApi.external_ui_url" placeholder="https://github.com/..." @change="emit" />
        <div class="hint">Download URL for Clash dashboard</div>
      </el-form-item>
      <el-form-item label="Cache ID">
        <el-input v-model="clashApi.cache_id" placeholder="Optional cache identifier" @change="emit" />
      </el-form-item>
    </el-form>

    <!-- Cache File -->
    <el-divider content-position="left">Cache File</el-divider>
    <el-form label-width="160px" label-position="left">
      <el-form-item label="Enabled">
        <el-switch v-model="cacheFile.enabled" @change="emit" />
      </el-form-item>
      <el-form-item v-if="cacheFile.enabled" label="Path">
        <el-input v-model="cacheFile.path" placeholder="cache.db" @change="emit" />
      </el-form-item>
      <el-form-item v-if="cacheFile.enabled" label="Store FakeIP">
        <el-switch v-model="cacheFile.store_fakeip" @change="emit" />
      </el-form-item>
    </el-form>

    <!-- v2ray API (optional, for sing-box stats) -->
    <el-divider content-position="left">v2ray API</el-divider>
    <el-form label-width="160px" label-position="left">
      <el-form-item label="Enabled">
        <el-switch v-model="v2rayApi.enabled" @change="emit" />
      </el-form-item>
      <template v-if="v2rayApi.enabled">
        <el-form-item label="Listen">
          <el-input v-model="v2rayApi.listen" placeholder="127.0.0.1:10085" @change="emit" />
          <div class="hint">gRPC API listening address</div>
        </el-form-item>
        <el-form-item label="Stats Service">
          <el-switch v-model="v2rayApi.stats" @change="emit" />
          <div class="hint">Enable stats service for traffic monitoring</div>
        </el-form-item>
      </template>
    </el-form>
  </div>
</template>
<script setup>
import { reactive, onMounted } from 'vue'
const props = defineProps({ data: Object })
const emitFn = defineEmits(['update'])
const clashApi = reactive({ external_controller: '', secret: '', default_mode: 'rule', external_ui: '', external_ui_url: '', cache_id: '' })
const cacheFile = reactive({ enabled: false, path: '', store_fakeip: false })
const v2rayApi = reactive({ enabled: false, listen: '', stats: true })

function sync() {
  if (!props.data) return
  const ca = props.data.clash_api || {}
  clashApi.external_controller = ca.external_controller || ''
  clashApi.secret = ca.secret || ''
  clashApi.default_mode = ca.default_mode || 'rule'
  clashApi.external_ui = ca.external_ui || ''
  clashApi.external_ui_url = ca.external_ui_url || ''
  clashApi.cache_id = ca.cache_id || ''
  const cf = props.data.cache_file || {}
  cacheFile.enabled = cf.enabled !== undefined ? cf.enabled : false
  cacheFile.path = cf.path || ''
  cacheFile.store_fakeip = cf.store_fakeip || false
  const va = props.data.v2ray_api || {}
  v2rayApi.enabled = va.listen ? true : false
  v2rayApi.listen = va.listen || ''
  v2rayApi.stats = va.stats?.enabled !== false
}

function build() {
  const out = {}
  if (clashApi.external_controller || clashApi.secret || clashApi.default_mode) {
    out.clash_api = { default_mode: clashApi.default_mode }
    if (clashApi.external_controller) out.clash_api.external_controller = clashApi.external_controller
    if (clashApi.secret) out.clash_api.secret = clashApi.secret
    if (clashApi.external_ui) out.clash_api.external_ui = clashApi.external_ui
    if (clashApi.external_ui_url) out.clash_api.external_ui_url = clashApi.external_ui_url
    if (clashApi.cache_id) out.clash_api.cache_id = clashApi.cache_id
  }
  if (cacheFile.enabled) {
    out.cache_file = { enabled: true }
    if (cacheFile.path) out.cache_file.path = cacheFile.path
    if (cacheFile.store_fakeip) out.cache_file.store_fakeip = true
  }
  if (v2rayApi.enabled) {
    out.v2ray_api = { listen: v2rayApi.listen || '127.0.0.1:10085' }
    if (v2rayApi.stats) out.v2ray_api.stats = { enabled: true }
  }
  return out
}

function emit() { emitFn('update', build()) }
onMounted(sync)
</script>
<style scoped>.hint { font-size: 12px; color: var(--el-text-color-secondary); margin-top: 2px; }</style>
