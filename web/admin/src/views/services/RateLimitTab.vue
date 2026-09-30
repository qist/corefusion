<template>
  <el-card v-loading="loading">
    <template #header>
      <div style="display:flex;justify-content:space-between;align-items:center">
        <span>{{ t('rateLimit.title') }}</span>
        <div style="display:flex;gap:8px;align-items:center">
          <el-tag v-if="config" :type="config.enabled ? 'success' : 'info'" size="small">
            {{ config.enabled ? t('rateLimit.enabled') : t('rateLimit.disabled') }}
          </el-tag>
          <el-button type="primary" size="small" @click="handleSave" :loading="saving" :disabled="!config">
            {{ t('common.save') }}
          </el-button>
        </div>
      </div>
    </template>

    <template v-if="config">
      <!-- Enable/Disable -->
      <el-form :model="form" label-width="160px" style="max-width:700px">
        <el-form-item :label="t('rateLimit.enableLabel')">
          <el-switch v-model="form.enabled" active-text="" inactive-text="" />
          <span style="margin-left:12px;font-size:12px;color:#909399">{{ t('rateLimit.enableHint') }}</span>
        </el-form-item>

        <el-divider content-position="left">{{ t('rateLimit.networkConfig') }}</el-divider>

        <el-form-item :label="t('rateLimit.ifaceLabel')">
          <div style="display:flex;gap:8px;width:100%">
            <el-select v-model="form.iface" clearable filterable allow-create style="flex:1" :placeholder="t('rateLimit.ifacePlaceholder')" :loading="interfacesLoading">
              <el-option v-for="iface in interfaceOptions" :key="iface" :label="iface" :value="iface" />
            </el-select>
            <el-button size="small" @click="loadInterfaces" :loading="interfacesLoading" style="flex-shrink:0">{{ t('common.refresh') }}</el-button>
          </div>
          <div style="font-size:12px;color:#909399;margin-top:4px">{{ t('rateLimit.ifaceHint') }}</div>
        </el-form-item>

        <el-form-item :label="t('rateLimit.ifbDeviceLabel')">
          <el-input v-model="form.ifb_device" :placeholder="t('rateLimit.ifbDevicePlaceholder')" />
          <div style="font-size:12px;color:#909399;margin-top:4px">{{ t('rateLimit.ifbDeviceHint') }}</div>
        </el-form-item>

        <el-divider content-position="left">{{ t('rateLimit.defaultSpeed') }}</el-divider>

        <el-form-item :label="t('rateLimit.defaultDownLabel')">
          <el-input-number v-model="form.default_max_down_mbps" :min="0" :max="10000" :step="10" />
          <span style="margin-left:8px;color:#909399;font-size:13px">Mbps</span>
          <span style="margin-left:8px;font-size:12px;color:#909399">{{ t('rateLimit.zeroNoLimit') }}</span>
        </el-form-item>

        <el-form-item :label="t('rateLimit.defaultUpLabel')">
          <el-input-number v-model="form.default_max_up_mbps" :min="0" :max="10000" :step="10" />
          <span style="margin-left:8px;color:#909399;font-size:13px">Mbps</span>
          <span style="margin-left:8px;font-size:12px;color:#909399">{{ t('rateLimit.zeroNoLimit') }}</span>
        </el-form-item>

        <el-divider content-position="left">{{ t('rateLimit.cleanupConfig') }}</el-divider>

        <el-form-item :label="t('rateLimit.idleTTLLabel')">
          <el-input-number v-model="form.idle_ttl_seconds" :min="0" :max="86400" :step="300" />
          <span style="margin-left:8px;color:#909399;font-size:13px">{{ t('rateLimit.seconds') }}</span>
          <div style="font-size:12px;color:#909399;margin-top:4px">{{ t('rateLimit.idleTTLHint') }}</div>
        </el-form-item>

        <el-form-item :label="t('rateLimit.cleanupIntervalLabel')">
          <el-input-number v-model="form.cleanup_interval" :min="60" :max="3600" :step="60" />
          <span style="margin-left:8px;color:#909399;font-size:13px">{{ t('rateLimit.seconds') }}</span>
        </el-form-item>

        <el-divider content-position="left">{{ t('rateLimit.backendConfig') }}</el-divider>

        <el-form-item :label="t('rateLimit.backendTypeLabel')">
          <el-select v-model="form.backend_type" style="width:200px">
            <el-option label="nftables + tc" value="nft_tc" />
            <el-option :label="t('rateLimit.ebpfFuture')" value="ebpf" disabled />
          </el-select>
        </el-form-item>
      </el-form>

      <!-- Availability Status -->
      <el-divider content-position="left">{{ t('rateLimit.availability') }}</el-divider>
      <div style="display:flex;gap:24px;flex-wrap:wrap;margin-bottom:16px">
        <div style="display:flex;align-items:center;gap:8px">
          <el-tag :type="status.nft_available ? 'success' : 'danger'" size="small">nftables</el-tag>
          <span style="font-size:13px">{{ status.nft_available ? t('rateLimit.available') : t('rateLimit.unavailable') }}</span>
        </div>
        <div style="display:flex;align-items:center;gap:8px">
          <el-tag :type="status.tc_available ? 'success' : 'danger'" size="small">tc (htb)</el-tag>
          <span style="font-size:13px">{{ status.tc_available ? t('rateLimit.available') : t('rateLimit.unavailable') }}</span>
        </div>
        <div style="display:flex;align-items:center;gap:8px">
          <el-tag :type="status.ifb_available ? 'success' : 'danger'" size="small">IFB</el-tag>
          <span style="font-size:13px">{{ status.ifb_available ? t('rateLimit.available') : t('rateLimit.unavailable') }}</span>
        </div>
      </div>

      <!-- Active Entries -->
      <el-divider content-position="left">{{ t('rateLimit.activeEntries') }} ({{ activeEntries.length }})</el-divider>
      <el-table v-if="!isMobile && activeEntries.length > 0" :data="activeEntries" border size="small" style="width:100%">
        <el-table-column prop="subscription_id" :label="t('rateLimit.subId')" width="120" />
        <el-table-column prop="ip" :label="t('rateLimit.ip')" width="160" />
        <el-table-column prop="max_down_mbps" :label="t('rateLimit.downMbps')" width="120">
          <template #default="{ row }">{{ row.max_down_mbps || '-' }}</template>
        </el-table-column>
        <el-table-column prop="max_up_mbps" :label="t('rateLimit.upMbps')" width="120">
          <template #default="{ row }">{{ row.max_up_mbps || '-' }}</template>
        </el-table-column>
        <el-table-column prop="mark" :label="t('rateLimit.mark')" width="100" />
      </el-table>
      <MobileCards v-else-if="activeEntries.length > 0" :rows="activeEntries" title-field="ip" :fields="rateLimitCardFields" />
      <el-empty v-else :description="t('rateLimit.noActiveEntries')" :image-size="40" />
    </template>

    <el-empty v-else :description="t('rateLimit.noConfig')" :image-size="60" />
  </el-card>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { getRateLimitConfig, updateRateLimitConfig, getRateLimitStatus } from '../../api/rateLimit'
import { listInterfaces } from '../../api/nodeServer'
import { useIsMobile } from '../../composables/useIsMobile'
import MobileCards from '../../components/MobileCards.vue'

const props = defineProps({
  serverId: { type: Number, required: true }
})

const { t } = useI18n()
const { isMobile } = useIsMobile()
const loading = ref(false)
const saving = ref(false)
const config = ref(null)
const status = ref({ nft_available: false, tc_available: false, ifb_available: false })
const activeEntries = ref([])
const interfaceOptions = ref([])
const interfacesLoading = ref(false)

const form = ref({
  enabled: false,
  iface: '',
  ifb_device: 'ifb0',
  default_max_down_mbps: 0,
  default_max_up_mbps: 0,
  idle_ttl_seconds: 3600,
  cleanup_interval: 300,
  backend_type: 'nft_tc',
})

async function fetchConfig() {
  if (!props.serverId) return
  loading.value = true
  try {
    const data = await getRateLimitConfig(props.serverId)
    config.value = data
    form.value = {
      enabled: data.enabled || false,
      iface: data.iface || '',
      ifb_device: data.ifb_device || 'ifb0',
      default_max_down_mbps: data.default_max_down_mbps || 0,
      default_max_up_mbps: data.default_max_up_mbps || 0,
      idle_ttl_seconds: data.idle_ttl_seconds || 3600,
      cleanup_interval: data.cleanup_interval || 300,
      backend_type: data.backend_type || 'nft_tc',
    }
    status.value = {
      nft_available: data.nft_available || false,
      tc_available: data.tc_available || false,
      ifb_available: data.ifb_available || false,
    }
  } catch {
    config.value = null
  } finally {
    loading.value = false
  }
}

async function fetchStatus() {
  if (!props.serverId) return
  try {
    const data = await getRateLimitStatus(props.serverId)
    if (data?.availability) {
      status.value = data.availability
    }
    if (data?.active_entries) {
      activeEntries.value = data.active_entries
    }
  } catch {
    // ignore
  }
}

async function loadInterfaces() {
  if (!props.serverId) return
  interfacesLoading.value = true
  try {
    const res = await listInterfaces(props.serverId)
    interfaceOptions.value = res || []
  } catch {
    interfaceOptions.value = []
  } finally {
    interfacesLoading.value = false
  }
}

async function handleSave() {
  saving.value = true
  try {
    const data = await updateRateLimitConfig(props.serverId, form.value)
    config.value = data
    ElMessage.success(t('common.success'))
    fetchStatus()
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || t('common.failed'))
  } finally {
    saving.value = false
  }
}

// 手机端卡片字段（与桌面表格列对应）
const rateLimitCardFields = computed(() => [
  { label: t('rateLimit.subId'), text: (row) => row.subscription_id || '-' },
  { label: t('rateLimit.downMbps'), text: (row) => row.max_down_mbps || '-' },
  { label: t('rateLimit.upMbps'), text: (row) => row.max_up_mbps || '-' },
  { label: t('rateLimit.mark'), text: (row) => row.mark || '-' },
])

onMounted(() => {
  fetchConfig()
  fetchStatus()
  loadInterfaces()
})
</script>
