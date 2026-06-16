<template>
  <div>
    <el-form label-width="140px" label-position="left">
      <el-divider content-position="left">Clash API</el-divider>
      <el-form-item label="External Controller">
        <el-input v-model="clashApi.external_controller" placeholder="127.0.0.1:9090" />
      </el-form-item>
      <el-form-item label="Secret">
        <el-input v-model="clashApi.secret" />
      </el-form-item>
      <el-form-item label="Default Mode">
        <el-input v-model="clashApi.default_mode" placeholder="rule" />
      </el-form-item>

      <el-divider content-position="left">Cache File</el-divider>
      <el-form-item label="Enabled">
        <el-switch v-model="cacheFile.enabled" />
      </el-form-item>
      <el-form-item label="Path">
        <el-input v-model="cacheFile.path" placeholder="cache.db" />
      </el-form-item>
      <el-form-item label="Store FakeIP">
        <el-switch v-model="cacheFile.store_fakeip" />
      </el-form-item>
    </el-form>
  </div>
</template>
<script setup>
import { reactive, watch, onMounted } from 'vue'
const props = defineProps({ data: Object })
const emit = defineEmits(['update'])

const clashApi = reactive({ external_controller: '', secret: '', default_mode: 'rule' })
const cacheFile = reactive({ enabled: true, path: '', store_fakeip: false })

function syncFromData() {
  if (!props.data) return
  const ca = props.data.clash_api || {}
  clashApi.external_controller = ca.external_controller || ''
  clashApi.secret = ca.secret || ''
  clashApi.default_mode = ca.default_mode || 'rule'
  const cf = props.data.cache_file || {}
  cacheFile.enabled = cf.enabled !== false
  cacheFile.path = cf.path || ''
  cacheFile.store_fakeip = cf.store_fakeip || false
}

function buildOutput() {
  return {
    clash_api: { external_controller: clashApi.external_controller, secret: clashApi.secret, default_mode: clashApi.default_mode },
    cache_file: { enabled: cacheFile.enabled, path: cacheFile.path, store_fakeip: cacheFile.store_fakeip },
  }
}

watch(() => props.data, syncFromData, { deep: true })
watch([clashApi, cacheFile], () => emit('update', buildOutput()), { deep: true })
onMounted(syncFromData)
</script>
