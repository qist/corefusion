<template>
  <el-card v-loading="loading">
      <template #header>
        <div style="display:flex;justify-content:space-between;align-items:center">
          <span>{{ t('service.warp.title') }}</span>
          <el-button type="primary" @click="showRegisterDialog = true" :loading="registerLoading">
            {{ t('service.warp.register') }}
          </el-button>
        </div>
      </template>

      <!-- WARP config list -->
      <el-table v-if="!isMobile && warpConfigs.length > 0" :data="warpConfigs" border style="width: 100%">
        <el-table-column prop="id" label="ID" width="60" />
        <el-table-column prop="core_type" :label="t('service.warp.coreType')" width="100">
          <template #default="{ row }">
            <el-tag v-if="row.core_type" size="small">{{ row.core_type }}</el-tag>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column prop="remark" :label="t('service.warp.remark')" width="120" />
        <el-table-column prop="status" :label="t('service.warp.status')" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 'active' ? 'success' : 'info'" size="small">
              {{ row.status === 'active' ? t('service.warp.active') : t('service.warp.registered') }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="address_ipv4" :label="t('service.warp.addressIPv4')" width="140" />
        <el-table-column prop="address_ipv6" :label="t('service.warp.addressIPv6')" width="180" />
        <el-table-column prop="endpoint" :label="t('service.warp.endpoint')" min-width="200" />
        <el-table-column prop="registration_at" :label="t('service.warp.registrationAt')" width="180" />
        <el-table-column :label="t('common.actions')" width="200" fixed="right">
          <template #default="{ row }">
            <div style="display:flex;gap:2px;align-items:center;white-space:nowrap">
              <el-button size="small" type="primary" @click="handleGenerateConfig(row)">
                {{ t('service.warp.generateConfig') }}
              </el-button>
              <el-button size="small" type="danger" @click="handleDelete(row)">
                {{ t('common.delete') }}
              </el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>
      <MobileCards v-else-if="warpConfigs.length > 0" :rows="warpConfigs" title-field="remark" :fields="warpCardFields">
        <template #actions="{ row }">
          <el-button size="small" type="primary" @click="handleGenerateConfig(row)">
            {{ t('service.warp.generateConfig') }}
          </el-button>
          <el-button size="small" type="danger" @click="handleDelete(row)">
            {{ t('common.delete') }}
          </el-button>
        </template>
      </MobileCards>

      <el-empty v-else :description="t('service.warp.notRegistered')" />

      <!-- Config preview -->
      <div v-if="currentConfig" style="margin-top:20px">
        <h4>{{ t('service.warp.config') }}</h4>
        <el-input type="textarea" :model-value="currentConfig" :rows="10" readonly />
      </div>

    <!-- Register dialog -->
    <el-dialog v-model="showRegisterDialog" :title="t('service.warp.register')" width="420px">
      <el-form label-width="100px">
        <el-form-item :label="t('service.warp.coreType')">
          <el-select v-model="registerForm.core_type" style="width:100%" clearable>
            <el-option label="xray" value="xray" />
            <el-option label="singbox" value="singbox" />
            <el-option :label="t('service.warp.allCores')" value="" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('service.warp.remark')">
          <el-input v-model="registerForm.remark" :placeholder="t('service.warp.remarkPlaceholder')" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showRegisterDialog = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="handleRegister" :loading="registerLoading">{{ t('common.confirm') }}</el-button>
      </template>
    </el-dialog>
  </el-card>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import client from '../../api/client'
import { useIsMobile } from '../../composables/useIsMobile'
import MobileCards from '../../components/MobileCards.vue'

const props = defineProps({
  serverId: { type: Number, required: true }
})

const { t } = useI18n()
const { isMobile } = useIsMobile()
const loading = ref(false)
const registerLoading = ref(false)
const warpConfigs = ref([])
const currentConfig = ref('')
const showRegisterDialog = ref(false)
const registerForm = ref({ core_type: '', remark: '' })

// 手机端卡片字段（与桌面表格列对应）
const warpCardFields = computed(() => [
  { label: t('service.warp.coreType'), text: (row) => row.core_type || '-' },
  { label: t('service.warp.status'), text: (row) => row.status === 'active' ? t('service.warp.active') : t('service.warp.registered') },
  { label: t('service.warp.addressIPv4'), text: (row) => row.address_ipv4 || '-' },
  { label: t('service.warp.addressIPv6'), text: (row) => row.address_ipv6 || '-' },
  { label: t('service.warp.endpoint'), text: (row) => row.endpoint || '-' },
  { label: t('service.warp.registrationAt'), text: (row) => row.registration_at || '-' },
])

const warpBase = computed(() => `/node-servers/${props.serverId}/warp`)

async function fetchWarpConfigs() {
  if (!props.serverId) return
  loading.value = true
  try {
    const data = await client.get(warpBase.value)
    warpConfigs.value = data?.items || []
  } catch (e) {
    warpConfigs.value = []
  } finally {
    loading.value = false
  }
}

async function handleRegister() {
  registerLoading.value = true
  try {
    await client.post(`${warpBase.value}/register`, {
      core_type: registerForm.value.core_type || '',
      remark: registerForm.value.remark || '',
    })
    ElMessage.success(t('common.success'))
    showRegisterDialog.value = false
    registerForm.value = { core_type: '', remark: '' }
    await fetchWarpConfigs()
  } catch (e) {
    ElMessage.error(e.message || t('service.warp.registerFailed'))
  } finally {
    registerLoading.value = false
  }
}

async function handleGenerateConfig(row) {
  try {
    const data = await client.post(`${warpBase.value}/generate-config`, {
      core_type: row.core_type || '',
    })
    currentConfig.value = data?.config || ''
    ElMessage.success(t('common.success'))
    await fetchWarpConfigs()
  } catch (e) {
    ElMessage.error(e.message || 'Failed')
  }
}

async function handleDelete(row) {
  await ElMessageBox.confirm(t('service.warp.resetConfirm'), t('common.confirm'))
  try {
    await client.delete(`${warpBase.value}/${row.id}`)
    ElMessage.success(t('common.success'))
    await fetchWarpConfigs()
  } catch (e) {
    ElMessage.error(e.message || 'Failed')
  }
}

watch(() => props.serverId, () => {
  currentConfig.value = ''
  fetchWarpConfigs()
})

onMounted(() => {
  fetchWarpConfigs()
})
</script>
