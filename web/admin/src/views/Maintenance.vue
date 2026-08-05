<template>
  <div class="maintenance-page">
    <div class="maintenance-card">
      <div class="maintenance-icon">
        <el-icon :size="64" color="#e6a23c"><Tools /></el-icon>
      </div>
      <h1>{{ message || t('maintenance.title') }}</h1>
      <p class="maintenance-desc">{{ t('maintenance.description') }}</p>
      <div v-if="restoreStatus" class="restore-info">
        <el-tag :type="statusTagType" size="large">
          {{ statusLabel }}
        </el-tag>
        <span class="restore-message" v-if="restoreStatus.message">{{ restoreStatus.message }}</span>
      </div>
      <el-button type="primary" :loading="checking" @click="checkStatus" style="margin-top: 20px">
        {{ t('maintenance.retry') }}
      </el-button>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { Tools } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import axios from 'axios'
import i18n from '../locales'

const t = i18n.global.t
const message = ref('')
const restoreStatus = ref(null)
const checking = ref(false)
let pollTimer = null

const statusTagType = computed(() => {
  const s = restoreStatus.value?.status
  if (s === 'done') return 'success'
  if (s === 'error') return 'danger'
  return 'warning'
})

const statusLabel = computed(() => {
  const s = restoreStatus.value?.status
  if (!s) return ''
  const labels = {
    pending: t('maintenance.statusPending'),
    downloading: t('maintenance.statusDownloading'),
    restoring: t('maintenance.statusRestoring'),
    done: t('maintenance.statusDone'),
    error: t('maintenance.statusError'),
  }
  return labels[s] || s
})

async function checkStatus() {
  checking.value = true
  try {
    // Check restore status (this endpoint is allowed during maintenance)
    const resp = await axios.get('/api/v1/backups/restore-status', {
      headers: { Authorization: `Bearer ${localStorage.getItem('access_token')}` },
    })
    const data = resp.data?.data || resp.data
    if (data.maintenance !== undefined) {
      message.value = data.maintenance_message || ''
    }
    if (data.status && data.status !== 'idle') {
      restoreStatus.value = data
    } else {
      restoreStatus.value = null
    }

    // If maintenance is off, redirect to dashboard
    if (data.maintenance === false && !restoreStatus.value) {
      window.location.href = '/'
    }
  } catch (e) {
    // If we get 503, we're still in maintenance
    if (e.response?.status === 503) {
      // Expected — keep polling
    } else {
      // Try checking maintenance endpoint directly
      try {
        const resp = await axios.get('/api/v1/system/maintenance')
        const data = resp.data?.data || resp.data
        message.value = data.message || ''
        if (!data.enabled) {
          window.location.href = '/'
        }
      } catch {
        // Ignore
      }
    }
  } finally {
    checking.value = false
  }
}

onMounted(() => {
  checkStatus()
  // Poll every 3 seconds
  pollTimer = setInterval(checkStatus, 3000)
})

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer)
})
</script>

<style scoped>
.maintenance-page {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}

.maintenance-card {
  background: white;
  border-radius: 16px;
  padding: 48px 64px;
  text-align: center;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
  max-width: 500px;
}

.maintenance-icon {
  margin-bottom: 24px;
}

.maintenance-card h1 {
  font-size: 24px;
  color: #303030;
  margin-bottom: 12px;
}

.maintenance-desc {
  color: #909399;
  font-size: 14px;
  margin-bottom: 20px;
}

.restore-info {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  margin-top: 16px;
}

.restore-message {
  font-size: 13px;
  color: #606266;
}
</style>
