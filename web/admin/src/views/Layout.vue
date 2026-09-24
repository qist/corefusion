<template>
  <el-container style="height:100vh">
    <el-aside width="220px" style="background:#304156">
      <h2 style="color:#fff;text-align:center;padding:20px 0;margin:0;font-size:18px">{{ t('common.appName') }}</h2>
      <el-menu :default-active="route.path" background-color="#304156" text-color="#bfcbd9" active-text-color="#409EFF" router>
        <el-menu-item v-for="item in menuItems" :key="item.path" :index="item.path">
          <el-icon><component :is="item.icon" /></el-icon><span>{{ t(item.title) }}</span>
        </el-menu-item>
      </el-menu>
    </el-aside>
    <el-container>
      <el-header style="background:#fff;border-bottom:1px solid #e6e6e6;display:flex;align-items:center;justify-content:flex-end;gap:16px">
        <el-dropdown @command="changeLocale">
          <span style="cursor:pointer;color:#409eff">{{ currentLocaleName }}</span>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item v-for="loc in availableLocales" :key="loc.code" :command="loc.code">
                {{ loc.flag }} {{ loc.name }}
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
        <el-dropdown @command="handleCommand">
          <span style="cursor:pointer">{{ authStore.username }} <el-icon><ArrowDown /></el-icon></span>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="password">{{ t('auth.changePassword') }}</el-dropdown-item>
              <el-dropdown-item command="passkey">{{ t('auth.passkeyManage') }}</el-dropdown-item>
              <el-dropdown-item command="logout" divided>{{ t('auth.logout') }}</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </el-header>
      <el-main style="background:#f5f7fa;padding:20px"><router-view /></el-main>
    </el-container>
  </el-container>

  <!-- Change Password Dialog -->
  <el-dialog v-model="pwdVisible" :title="t('auth.changePassword')" width="400">
    <el-form :model="pwdForm" label-width="100px">
      <el-form-item :label="t('auth.oldPassword')">
        <el-input v-model="pwdForm.old_password" type="password" show-password />
      </el-form-item>
      <el-form-item :label="t('auth.newPassword')">
        <el-input v-model="pwdForm.new_password" type="password" show-password />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="pwdVisible=false">{{ t('common.cancel') }}</el-button>
      <el-button type="primary" :loading="pwdSaving" @click="handleChangePassword">{{ t('common.save') }}</el-button>
    </template>
  </el-dialog>

  <!-- Passkey Management Dialog -->
  <el-dialog v-model="passkeyVisible" :title="t('auth.passkeyManage')" width="600">
    <div v-if="!passkeyPageAvailable" style="color:#e6a23c;font-size:13px;line-height:1.6;margin-bottom:12px">
      {{ t('auth.passkeyUnavailableHint') }}
    </div>
    <el-table :data="passkeyList" style="width:100%" :empty-text="t('auth.passkeyEmpty')">
      <el-table-column :label="t('common.name')" prop="name" />
      <el-table-column :label="t('auth.passkeyRp')" prop="rpid" width="180" />
      <el-table-column :label="t('auth.passkeyAddedAt')" width="180">
        <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
      </el-table-column>
      <el-table-column :label="t('common.action')" width="90">
        <template #default="{ row }">
          <el-button link type="danger" @click="handleDeletePasskey(row)">{{ t('common.delete') }}</el-button>
        </template>
      </el-table-column>
    </el-table>
    <template #footer>
      <el-button :loading="passkeyRegistering" :disabled="!passkeyPageAvailable" @click="handleAddPasskey">
        {{ t('auth.passkeyRegister') }}
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '../store/auth'
import { setLocale, availableLocales } from '../locales'
import { hasRole } from '../utils/permission'
import { getPasskeyStatus, listPasskeys, deletePasskey, passkeyRegisterBegin, passkeyRegisterFinish } from '../api/passkey'
import { createCredential, hasWebAuthn } from '../utils/webauthn'

const { t, locale } = useI18n()
const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()

const currentLocaleName = computed(() => {
  const found = availableLocales.find(l => l.code === locale.value)
  return found ? `${found.flag} ${found.name}` : locale.value
})

// `role` is the minimum role required to see the menu entry (fail-closed:
// entries without a role default to admin so ordinary operators/viewers don't
// see management surfaces).
const allMenuItems = [
  { path: '/dashboard', title: 'menu.dashboard', icon: 'Odometer', role: 'operator' },
  { path: '/node-servers', title: 'menu.nodeServers', icon: 'Cpu', role: 'admin' },
  { path: '/chain-proxy', title: 'menu.chainProxy', icon: 'Share', role: 'admin' },
  { path: '/online-users', title: 'menu.onlineUsers', icon: 'Connection', role: 'operator' },
  { path: '/services', title: 'menu.services', icon: 'Monitor', role: 'operator' },
  { path: '/subscriptions', title: 'menu.subscriptions', icon: 'Link', role: 'operator' },
  { path: '/plans', title: 'menu.planManagement', icon: 'Tickets', role: 'operator' },
  { path: '/charge', title: 'menu.chargeManagement', icon: 'Wallet', role: 'admin' },
  { path: '/tickets', title: 'menu.ticketManagement', icon: 'ChatDotRound', role: 'operator' },
  { path: '/client-ext', title: 'menu.clientExtManagement', icon: 'Grid', role: 'admin' },
  { path: '/clients', title: 'menu.clients', icon: 'User', role: 'operator' },
  { path: '/certificates', title: 'menu.certificates', icon: 'Lock', role: 'admin' },
  { path: '/traffic-quota', title: 'menu.trafficQuota', icon: 'DataLine', role: 'admin' },
  { path: '/users', title: 'menu.users', icon: 'User', role: 'admin' },
  { path: '/log-manage', title: 'menu.logManage', icon: 'Document', role: 'admin' },
  { path: '/portal', title: 'menu.portalManagement', icon: 'Monitor', role: 'admin' },
  { path: '/settings', title: 'menu.settings', icon: 'Setting', role: 'admin' },
  { path: '/db-stats', title: 'menu.dbStats', icon: 'DataAnalysis', role: 'admin' },
  { path: '/audit', title: 'menu.audit', icon: 'Bell', role: 'admin' },
  { path: '/runtime', title: 'menu.runtimeConfig', icon: 'Switch', role: 'admin' },
]

const menuItems = computed(() =>
  allMenuItems.filter(item => hasRole(item.role, authStore.role))
)

const pwdVisible = ref(false)
const pwdSaving = ref(false)
const pwdForm = ref({ old_password: '', new_password: '' })

// Passkey 管理状态
const passkeyVisible = ref(false)
const passkeyList = ref([])
const passkeyPageAvailable = ref(false)
const passkeyRegistering = ref(false)

function formatTime(v) {
  if (!v) return '-'
  const d = new Date(v)
  return Number.isNaN(d.getTime()) ? String(v) : d.toLocaleString()
}

async function openPasskeyDialog() {
  passkeyVisible.value = true
  passkeyPageAvailable.value = hasWebAuthn()
  try {
    const status = await getPasskeyStatus()
    passkeyPageAvailable.value = passkeyPageAvailable.value && !!status.available
  } catch (e) { passkeyPageAvailable.value = false }
  await loadPasskeys()
}

async function loadPasskeys() {
  try {
    passkeyList.value = await listPasskeys()
  } catch (e) { passkeyList.value = [] }
}

async function handleAddPasskey() {
  if (!passkeyPageAvailable.value) return
  passkeyRegistering.value = true
  try {
    const begin = await passkeyRegisterBegin('')
    const response = await createCredential(begin.options)
    const res = await passkeyRegisterFinish(begin.session_token, response, '')
    passkeyList.value = res.credentials || []
    ElMessage.success(t('auth.passkeyRegistered'))
  } catch (e) {
    // 用户取消认证器弹窗属正常交互，不弹错误
    if (e?.name === 'NotAllowedError' || e?.code === 20) return
    ElMessage.error(t('auth.passkeyRegisterFailed'))
  } finally { passkeyRegistering.value = false }
}

async function handleDeletePasskey(row) {
  try {
    await ElMessageBox.confirm(t('auth.passkeyDeleteConfirm'), t('common.tip'), { type: 'warning' })
  } catch (e) { return }
  try {
    await deletePasskey(row.id)
    await loadPasskeys()
    ElMessage.success(t('common.deleteSuccess'))
  } catch (e) { ElMessage.error(t('common.failed')) }
}

function changeLocale(code) { setLocale(code) }
function handleCommand(cmd) {
  if (cmd === 'logout') {
    authStore.logout()
    router.push('/login')
  } else if (cmd === 'password') {
    pwdForm.value = { old_password: '', new_password: '' }
    pwdVisible.value = true
  } else if (cmd === 'passkey') {
    openPasskeyDialog()
  }
}

async function handleChangePassword() {
  if (!pwdForm.value.old_password || !pwdForm.value.new_password) {
    ElMessage.error(t('auth.invalidCredentials'))
    return
  }
  pwdSaving.value = true
  try {
    await authStore.changePassword(pwdForm.value.old_password, pwdForm.value.new_password)
    ElMessage.success(t('auth.passwordChanged'))
    pwdVisible.value = false
  } catch (e) { ElMessage.error(t('common.failed')) } finally { pwdSaving.value = false }
}
</script>

<style scoped>.el-menu { border-right:none; }</style>
