<template>
  <div>
    <el-card v-loading="loading">
      <template #header>
        <div style="display:flex;justify-content:space-between;align-items:center">
          <span>{{ t('service.warp.title') }}</span>
          <div>
            <el-button v-if="!status.registered" type="primary" @click="handleRegister" :loading="registerLoading">{{ t('service.warp.register') }}</el-button>
            <el-button v-if="status.registered" type="success" @click="handleGenerateConfig" :loading="generateLoading">{{ t('service.warp.generateConfig') }}</el-button>
            <el-button v-if="status.registered" type="danger" @click="handleReset">{{ t('service.warp.reset') }}</el-button>
          </div>
        </div>
      </template>

      <el-descriptions :column="2" border>
        <el-descriptions-item :label="t('service.warp.status')" :span="2">
          <el-tag :type="status.registered ? 'success' : 'info'">
            {{ status.registered ? t('service.warp.registered') : t('service.warp.notRegistered') }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="Private Key">
          <code v-if="status.private_key">{{ status.private_key }}</code>
          <span v-else>-</span>
        </el-descriptions-item>
        <el-descriptions-item label="Public Key (Peer)">
          <code v-if="status.public_key">{{ status.public_key }}</code>
          <span v-else>-</span>
        </el-descriptions-item>
        <el-descriptions-item label="Reserved">
          <code v-if="status.reserved">{{ status.reserved }}</code>
          <span v-else>-</span>
        </el-descriptions-item>
        <el-descriptions-item label="Endpoint">
          <code v-if="status.endpoint">{{ status.endpoint }}</code>
          <span v-else>-</span>
        </el-descriptions-item>
        <el-descriptions-item :label="t('service.warp.addressIPv4')">{{ status.address_ipv4 || '-' }}</el-descriptions-item>
        <el-descriptions-item :label="t('service.warp.addressIPv6')">{{ status.address_ipv6 || '-' }}</el-descriptions-item>
        <el-descriptions-item label="Account ID">{{ status.account_id || '-' }}</el-descriptions-item>
        <el-descriptions-item label="Account Type">{{ status.account_type || '-' }}</el-descriptions-item>
        <el-descriptions-item label="License Key" :span="2">
          <code v-if="status.license_key">{{ status.license_key }}</code>
          <span v-else>-</span>
        </el-descriptions-item>
      </el-descriptions>

      <div v-if="config" style="margin-top:20px">
        <h4>{{ t('service.warp.config') }}</h4>
        <el-input type="textarea" :model-value="config" :rows="10" readonly />
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import client from '../../api/client'

const { t } = useI18n()
const loading = ref(false)
const registerLoading = ref(false)
const generateLoading = ref(false)
const status = ref({ status: 'not_registered', registered: false })
const config = ref('')

async function fetchStatus() {
  loading.value = true
  try {
    const data = await client.get('/warp/status')
    status.value = data
  } catch (e) {
    status.value = { status: 'not_registered', registered: false }
  } finally {
    loading.value = false
  }
}

async function fetchConfig() {
  try {
    const data = await client.get('/warp/config')
    config.value = data.config || ''
  } catch (e) {
    config.value = ''
  }
}

async function handleRegister() {
  await ElMessageBox.confirm(t('service.warp.registerConfirm'), t('common.confirm'))
  registerLoading.value = true
  try {
    const data = await client.post('/warp/register')
    status.value = data
    ElMessage.success(t('common.success'))
  } catch (e) {} finally { registerLoading.value = false }
}

async function handleGenerateConfig() {
  generateLoading.value = true
  try {
    const data = await client.post('/warp/generate-config')
    config.value = data.config || ''
    ElMessage.success(t('common.success'))
  } catch (e) {} finally { generateLoading.value = false }
}

async function handleReset() {
  await ElMessageBox.confirm(t('service.warp.resetConfirm'), t('common.confirm'))
  try {
    await client.delete('/warp')
    ElMessage.success(t('common.success'))
    status.value = { status: 'not_registered', registered: false }
    config.value = ''
  } catch (e) {}
}

onMounted(() => {
  fetchStatus()
  fetchConfig()
})
</script>
