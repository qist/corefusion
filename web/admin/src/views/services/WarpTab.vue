<template>
  <el-card v-loading="loading">
      <template #header>
        <div style="display:flex;justify-content:space-between;align-items:center">
          <span>{{ t('service.warp.title') }}</span>
          <el-button type="primary" @click="openRegisterDialog" :loading="registerLoading">
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
        <el-table-column :label="t('service.warp.tunnelType')" width="110">
          <template #default="{ row }">
            <el-tag :type="isMasque(row) ? 'success' : 'info'" size="small">
              {{ isMasque(row) ? 'MASQUE' : 'WireGuard' }}
            </el-tag>
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
        <el-table-column :label="t('common.actions')" width="240" fixed="right">
          <template #default="{ row }">
            <div style="display:flex;gap:2px;align-items:center;white-space:nowrap">
              <el-button v-if="isMasque(row)" size="small" type="success" @click="openOutboundDialog(row)">
                {{ t('service.warp.createOutbound') }}
              </el-button>
              <el-button v-else size="small" type="primary" @click="handleGenerateConfig(row)">
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
          <el-button v-if="isMasque(row)" size="small" type="success" @click="openOutboundDialog(row)">
            {{ t('service.warp.createOutbound') }}
          </el-button>
          <el-button v-else size="small" type="primary" @click="handleGenerateConfig(row)">
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
    <el-dialog v-model="showRegisterDialog" :title="t('service.warp.register')" width="460px">
      <el-form label-width="120px">
        <el-form-item :label="t('service.warp.registerType')">
          <el-radio-group v-model="registerForm.type">
            <el-radio value="wireguard">WireGuard</el-radio>
            <el-radio value="masque">MASQUE</el-radio>
          </el-radio-group>
          <div class="hint">{{ registerForm.type === 'masque' ? t('service.warp.masqueHint') : t('service.warp.wireguardHint') }}</div>
        </el-form-item>
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

    <!-- MASQUE outbound dialog -->
    <el-dialog v-model="showOutboundDialog" :title="t('service.warp.createOutbound')" width="480px">
      <el-form label-width="130px">
        <el-form-item :label="t('service.warp.outboundTag')">
          <el-input v-model="outboundForm.tag" :placeholder="defaultOutboundTag" />
          <div class="hint">{{ t('service.warp.outboundTagHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('service.warp.outboundRemark')">
          <el-input v-model="outboundForm.remark" placeholder="WARP MASQUE" />
        </el-form-item>
        <el-form-item :label="t('service.warp.outboundAlpn')">
          <el-radio-group v-model="outboundForm.alpn">
            <el-radio value="h3">h3 (QUIC)</el-radio>
            <el-radio value="h2">h2 (TCP)</el-radio>
          </el-radio-group>
          <div class="hint">{{ t('service.warp.outboundAlpnHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('service.warp.outboundPort')">
          <el-input-number v-model="outboundForm.port" :min="0" :max="65535" />
          <div class="hint">{{ t('service.warp.outboundPortHint') }}</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showOutboundDialog = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="handleCreateOutbound" :loading="outboundLoading">{{ t('common.confirm') }}</el-button>
      </template>
    </el-dialog>

    <!-- Delete dialog -->
    <el-dialog v-model="showDeleteDialog" :title="t('service.warp.deleteTitle')" width="460px">
      <div>{{ t('service.warp.resetConfirm') }}</div>
      <el-checkbox v-model="deleteRemote" style="margin-top:14px">
        {{ t('service.warp.deleteRemoteLabel') }}
      </el-checkbox>
      <div class="hint">{{ t('service.warp.deleteRemoteHint') }}</div>
      <template #footer>
        <el-button @click="showDeleteDialog = false">{{ t('common.cancel') }}</el-button>
        <el-button type="danger" @click="confirmDelete" :loading="deleting">{{ t('common.confirm') }}</el-button>
      </template>
    </el-dialog>
  </el-card>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
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
const outboundLoading = ref(false)
const warpConfigs = ref([])
const currentConfig = ref('')
const showRegisterDialog = ref(false)
const showOutboundDialog = ref(false)
const showDeleteDialog = ref(false)
const deleteTarget = ref(null)
const deleteRemote = ref(false)
const deleting = ref(false)
const registerForm = ref({ type: 'wireguard', core_type: '', remark: '' })
const outboundForm = ref({ warpId: 0, tag: '', remark: '', alpn: 'h3', port: 0 })
const lastCreatedTag = ref('')

// 历史记录 KeyType/TunnelType 为空，按 WireGuard 处理（不迁移）。
function isMasque(row) {
  return String(row?.tunnel_type || '').toLowerCase() === 'masque'
}

const defaultOutboundTag = computed(() =>
  outboundForm.value.warpId ? `warp-masque-${outboundForm.value.warpId}` : 'warp-masque'
)

// 手机端卡片字段（与桌面表格列对应）
const warpCardFields = computed(() => [
  { label: t('service.warp.tunnelType'), text: (row) => (isMasque(row) ? 'MASQUE' : 'WireGuard') },
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

function openRegisterDialog() {
  registerForm.value = { type: 'wireguard', core_type: '', remark: '' }
  showRegisterDialog.value = true
}

async function handleRegister() {
  registerLoading.value = true
  try {
    const payload = {
      core_type: registerForm.value.core_type || '',
      remark: registerForm.value.remark || '',
    }
    // 两种注册并存：MASQUE 走两步流程，WireGuard 保持原接口。
    if (registerForm.value.type === 'masque') {
      await client.post(`${warpBase.value}/masque/register`, payload)
    } else {
      await client.post(`${warpBase.value}/register`, payload)
    }
    ElMessage.success(t('common.success'))
    showRegisterDialog.value = false
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

function openOutboundDialog(row) {
  outboundForm.value = {
    warpId: row.id,
    tag: `warp-masque-${row.id}`,
    remark: row.remark || 'WARP MASQUE',
    alpn: 'h3',
    port: 0,
  }
  showOutboundDialog.value = true
}

async function handleCreateOutbound() {
  outboundLoading.value = true
  try {
    const f = outboundForm.value
    const data = await client.post(`${warpBase.value}/${f.warpId}/masque-outbound`, {
      tag: f.tag || '',
      remark: f.remark || '',
      alpn: [f.alpn],
      port: Number(f.port) || 0,
    })
    lastCreatedTag.value = data?.tag || ''
    showOutboundDialog.value = false
    ElMessage.success(t('service.warp.outboundCreated', { tag: lastCreatedTag.value }))
  } catch (e) {
    ElMessage.error(e.message || 'Failed')
  } finally {
    outboundLoading.value = false
  }
}

function handleDelete(row) {
  deleteTarget.value = row
  deleteRemote.value = false
  showDeleteDialog.value = true
}

async function confirmDelete() {
  if (!deleteTarget.value) return
  deleting.value = true
  try {
    const data = await client.delete(`${warpBase.value}/${deleteTarget.value.id}?remote=${deleteRemote.value ? 1 : 0}`)
    // 云端注销失败不影响本地删除，但要明确告知，避免以为设备已释放。
    if (data && data.remote_attempted && !data.remote_deleted) {
      ElMessage.warning(t('service.warp.deleteRemoteFailed', { error: data.remote_error || '' }))
    } else {
      ElMessage.success(t('common.success'))
    }
    showDeleteDialog.value = false
    await fetchWarpConfigs()
  } catch (e) {
    ElMessage.error(e.message || 'Failed')
  } finally {
    deleting.value = false
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
