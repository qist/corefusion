<template>
  <el-container style="height:100vh">
    <el-aside v-if="!isMobile" width="220px" style="background:#304156">
      <h2 style="color:#fff;text-align:center;padding:20px 0;margin:0;font-size:18px">{{ t('common.appName') }}</h2>
      <el-menu :default-active="route.path" background-color="#304156" text-color="#bfcbd9" active-text-color="#409EFF" router>
        <el-menu-item v-for="item in menuItems" :key="item.path" :index="item.path">
          <el-icon><component :is="item.icon" /></el-icon><span>{{ t(item.title) }}</span>
        </el-menu-item>
      </el-menu>
    </el-aside>
    <!-- 移动端：侧栏改为抽屉，点汉堡按钮展开 -->
    <el-drawer v-if="isMobile" v-model="drawerOpen" direction="ltr" size="230px" :with-header="false" class="mobile-menu-drawer">
      <div style="background:#304156;min-height:100%">
        <h2 style="color:#fff;text-align:center;padding:20px 0;margin:0;font-size:18px">{{ t('common.appName') }}</h2>
        <el-menu :default-active="route.path" background-color="#304156" text-color="#bfcbd9" active-text-color="#409EFF" router @select="drawerOpen=false">
          <el-menu-item v-for="item in menuItems" :key="item.path" :index="item.path">
            <el-icon><component :is="item.icon" /></el-icon><span>{{ t(item.title) }}</span>
          </el-menu-item>
        </el-menu>
      </div>
    </el-drawer>
    <el-container>
      <el-header style="background:#fff;border-bottom:1px solid #e6e6e6;display:flex;align-items:center;justify-content:flex-end;gap:16px">
        <el-icon v-if="isMobile" size="22" style="cursor:pointer;margin-right:auto" @click="drawerOpen=true"><Fold /></el-icon>
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
              <el-dropdown-item command="twofa">{{ twofaStatus.enabled ? t('auth.twofaDisableMenu') : t('auth.twofaEnableMenu') }}</el-dropdown-item>
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

  <!-- 2FA（双步验证）管理 Dialog -->
  <el-dialog v-model="twofaVisible" :title="t('auth.twofaManage')" width="520">
    <!-- 已启用：状态 -->
    <template v-if="twofaStatus.enabled">
      <el-alert :title="t('auth.twofaEnabledStatus')" type="success" :closable="false" show-icon style="margin-bottom:14px" />
      <div style="font-size:13px;color:#666;margin-bottom:6px">
        {{ t('auth.twofaRecoveryLeft') }}：<b>{{ twofaStatus.recovery_codes }}</b>
      </div>
      <div style="font-size:12px;color:#909399;line-height:1.6">{{ t('auth.twofaEnabledHint') }}</div>
    </template>

    <!-- 未启用：绑定向导步骤 1：说明 + 当前密码确认 -->
    <template v-else-if="twofaStep === 'intro'">
      <div style="font-size:13px;color:#666;line-height:1.7;margin-bottom:14px">{{ t('auth.twofaIntro') }}</div>
      <el-form label-position="top">
        <el-form-item :label="t('auth.currentPassword')">
          <el-input v-model="twofaPassword" type="password" show-password @keyup.enter="twofaStartSetup" />
        </el-form-item>
      </el-form>
    </template>

    <!-- 步骤 2：扫码 + 输入动态码验证启用 -->
    <template v-else-if="twofaStep === 'scan'">
      <div style="text-align:center">
        <img v-if="twofaQrUrl" :src="twofaQrUrl" alt="TOTP QR" style="width:200px;height:200px" />
      </div>
      <div style="text-align:center;font-size:12px;color:#909399;margin:8px 0">
        {{ t('auth.twofaScanHint') }}
      </div>
      <el-form label-position="top">
        <el-form-item :label="t('auth.twofaManualKey')">
          <el-input :model-value="twofaSecret" readonly>
            <template #append><el-button link @click="copyTwofaSecret">{{ t('common.copy') }}</el-button></template>
          </el-input>
        </el-form-item>
        <el-form-item :label="t('auth.twofaCodeLabel')">
          <el-input v-model="twofaCode" maxlength="6" placeholder="123456" @keyup.enter="twofaEnable" />
        </el-form-item>
      </el-form>
    </template>

    <!-- 步骤 3：恢复码展示（必须保存） -->
    <template v-else-if="twofaStep === 'recovery'">
      <el-alert :title="t('auth.twofaSaveHint')" type="warning" :closable="false" show-icon style="margin-bottom:12px" />
      <div style="background:#f7f8fa;border-radius:6px;padding:14px;font-family:monospace;font-size:14px;line-height:2;display:grid;grid-template-columns:1fr 1fr;gap:2px 20px">
        <span v-for="(code, i) in twofaRecovery" :key="i">{{ code }}</span>
      </div>
      <div style="font-size:12px;color:#909399;margin-top:10px">{{ t('auth.twofaRecoveryLostHint') }}</div>
      <div style="text-align:center;margin-top:14px">
        <el-button type="primary" plain :loading="twofaBusy" @click="downloadRecoveryCodes">{{ t('auth.twofaDownload') }}</el-button>
      </div>
    </template>

    <!-- footer：按状态渲染不同按钮 -->
    <template #footer>
      <template v-if="twofaStatus.enabled">
        <el-button @click="twofaRegenerate">{{ t('auth.twofaRegenerate') }}</el-button>
        <el-button type="danger" plain @click="twofaDisable">{{ t('auth.twofaDisable') }}</el-button>
      </template>
      <template v-else-if="twofaStep === 'intro'">
        <el-button @click="twofaVisible=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="twofaBusy" :disabled="!twofaPassword" @click="twofaStartSetup">{{ t('common.next') }}</el-button>
      </template>
      <template v-else-if="twofaStep === 'scan'">
        <el-button @click="twofaStep='intro'">{{ t('common.back') }}</el-button>
        <el-button type="primary" :loading="twofaBusy" :disabled="twofaCode.length !== 6" @click="twofaEnable">{{ t('auth.twofaVerify') }}</el-button>
      </template>
      <template v-else-if="twofaStep === 'recovery'">
        <el-button type="primary" @click="twofaVisible=false; loadTwofaStatus()">{{ t('auth.twofaSaved') }}</el-button>
      </template>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '../store/auth'
import { setLocale, availableLocales } from '../locales'
import { hasRole } from '../utils/permission'
import QRCode from 'qrcode'
import { getPasskeyStatus, listPasskeys, deletePasskey, passkeyRegisterBegin, passkeyRegisterFinish } from '../api/passkey'
import {
  twofaStatus as apiTwofaStatus,
  twofaSetup as apiTwofaSetup,
  twofaEnable as apiTwofaEnable,
  twofaDisable as apiTwofaDisable,
  twofaRegenerateRecovery as apiTwofaRegenerate,
} from '../api/twofa'
import { createCredential, hasWebAuthn } from '../utils/webauthn'

const { t, locale } = useI18n()
const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()

const currentLocaleName = computed(() => {
  const found = availableLocales.find(l => l.code === locale.value)
  return found ? `${found.flag} ${found.name}` : locale.value
})

// 挂载时预取 2FA 状态，供下拉菜单动态显示「开启/关闭 2FA」。
// 移动端适配：≤768px 视为手机，侧栏改抽屉。
const isMobile = ref(window.innerWidth <= 768)
const drawerOpen = ref(false)
const onViewportResize = () => {
  isMobile.value = window.innerWidth <= 768
  if (!isMobile.value) drawerOpen.value = false
}
onMounted(() => {
  loadTwofaStatus()
  window.addEventListener('resize', onViewportResize)
})
onBeforeUnmount(() => { window.removeEventListener('resize', onViewportResize) })

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
  { path: '/legal-documents', title: 'menu.legalDocuments', icon: 'Document', role: 'admin' },
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

// 2FA（双步验证）管理状态
const twofaVisible = ref(false)
const twofaStatus = ref({ enabled: false, recovery_codes: 0 })
const twofaStep = ref('intro') // intro → scan → recovery（绑定向导步骤）
const twofaPassword = ref('')
const twofaSecret = ref('')
const twofaOtpauth = ref('')
const twofaQrUrl = ref('')
const twofaCode = ref('')
const twofaRecovery = ref([])
const twofaBusy = ref(false)

async function openTwofaDialog() {
  twofaVisible.value = true
  twofaStep.value = 'intro'
  twofaPassword.value = ''
  twofaSecret.value = ''
  twofaOtpauth.value = ''
  twofaQrUrl.value = ''
  twofaCode.value = ''
  twofaRecovery.value = []
  await loadTwofaStatus()
}

async function loadTwofaStatus() {
  try {
    twofaStatus.value = await apiTwofaStatus()
  } catch (e) { twofaStatus.value = { enabled: false, recovery_codes: 0 } }
}

async function twofaStartSetup() {
  twofaBusy.value = true
  try {
    const res = await apiTwofaSetup(twofaPassword.value)
    twofaSecret.value = res.secret
    twofaOtpauth.value = res.otpauth_url
    twofaQrUrl.value = await QRCode.toDataURL(res.otpauth_url, { width: 200, margin: 1 })
    twofaStep.value = 'scan'
  } catch (e) { ElMessage.error(t('auth.twofaSetupFailed')) }
  finally { twofaBusy.value = false }
}

async function twofaEnable() {
  twofaBusy.value = true
  try {
    const res = await apiTwofaEnable(twofaCode.value.trim())
    twofaRecovery.value = res.recovery_codes || []
    // 保持 twofaStatus.enabled=false，让模板命中 recovery 分支展示恢复码。
    // 用户点「我已保存」关闭时会重新 loadTwofaStatus（届时显示已启用）。
    twofaStep.value = 'recovery'
  } catch (e) { ElMessage.error(t('auth.twofaVerifyFailed')) }
  finally { twofaBusy.value = false }
}

async function twofaRegenerate() {
  let value
  try {
    const r = await ElMessageBox.prompt(t('auth.twofaRegenerateHint'), t('auth.twofaManage'), {
      inputPlaceholder: t('auth.twofaPlaceholder'),
      inputValidator: (v) => (v && v.trim().length === 6) ? true : t('auth.twofaRequired'),
    })
    value = r.value
  } catch (e) { return }
  try {
    const res = await apiTwofaRegenerate(value.trim())
    twofaRecovery.value = res.recovery_codes || []
    twofaStep.value = 'recovery'
  } catch (e) { ElMessage.error(t('auth.twofaVerifyFailed')) }
}

async function twofaDisable() {
  let value
  try {
    const r = await ElMessageBox.prompt(t('auth.twofaDisableHint'), t('auth.twofaManage'), {
      inputPlaceholder: t('auth.twofaPlaceholder'),
      inputValidator: (v) => (v && v.trim().length === 6) ? true : t('auth.twofaRequired'),
    })
    value = r.value
  } catch (e) { return }
  try {
    await apiTwofaDisable(value.trim())
    ElMessage.success(t('auth.twofaDisabled'))
    twofaVisible.value = false
    await loadTwofaStatus()
  } catch (e) { ElMessage.error(t('auth.twofaVerifyFailed')) }
}

async function copyTwofaSecret() {
  try {
    await navigator.clipboard.writeText(twofaSecret.value)
    ElMessage.success(t('common.copySuccess'))
  } catch (e) { ElMessage.warning(twofaSecret.value) }
}

// 下载恢复码为本地 txt 文件（对齐 GitHub 的 github-recovery-codes.txt 体验），
// 避免手动复制遗漏。文件只在恢复码生成（启用 / 重新生成）时可用。
function downloadRecoveryCodes() {
  const codes = twofaRecovery.value
  if (!codes || codes.length === 0) return
  const lines = [
    'CoreFusion - 双步验证恢复码',
    '账号: ' + (authStore.username || ''),
    '生成时间: ' + new Date().toLocaleString(),
    '',
    '每个恢复码只能使用一次，请妥善保存（建议离线保管）。',
    '使用方式: 登录页输入用户名后，直接粘贴恢复码即可登录。',
    '',
    ...codes,
    '',
  ]
  const blob = new Blob([lines.join('\n')], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = 'corefusion-recovery-codes.txt'
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
  ElMessage.success(t('auth.twofaDownloaded'))
}

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
  } else if (cmd === 'twofa') {
    openTwofaDialog()
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
