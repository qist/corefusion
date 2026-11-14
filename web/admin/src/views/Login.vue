<template>
  <div class="login-container">
    <el-card class="login-card">
      <h2 style="text-align:center;margin-bottom:30px">{{ t('common.appName') }}</h2>

      <!-- 一步登录：用户名 + 凭据（密码 / 动态码 / 恢复码） -->
      <el-form :model="form" :autocomplete="twofaEnabled ? 'off' : 'on'" @submit.prevent="handleLogin" label-position="top">
        <el-form-item :label="t('auth.username')">
          <el-input v-model="form.username" :placeholder="t('auth.username')" prefix-icon="User" :disabled="loading" @input="onUsernameInput" @keyup.enter="focusCredential" />
        </el-form-item>

        <!-- 用户名已启用 2FA：直接输入 6 位动态码 / 恢复码（无需静态密码） -->
        <template v-if="twofaEnabled">
          <div style="text-align:center;margin-bottom:10px">
            <div style="font-size:14px;font-weight:600;color:#e6a23c">{{ t('auth.twofaTitle') }}</div>
            <div style="font-size:12px;color:#909399;margin-top:4px">{{ t('auth.twofaDetectedHint') }}</div>
          </div>
          <el-form-item>
            <!-- 动态码/恢复码属于等效凭据：掩码显示 + 主动清理浏览器/密码管理器的自动填充。
                 仅靠 autocomplete/readonly 挡不住 Chrome 的密码自动填充（实测仍会填入保存的密码），
                 因此额外按"是否有真实按键/粘贴"判断：没有用户输入的填充一律清空。 -->
            <el-input
              v-model="credential"
              type="password"
              show-password
              :placeholder="t('auth.twofaPlaceholder')"
              prefix-icon="Key"
              style="text-align:center;font-size:18px;letter-spacing:4px"
              autocomplete="one-time-code"
              name="cf-otp"
              data-lpignore="true"
              data-1p-ignore="true"
              data-bwignore="true"
              data-form-type="other"
              :readonly="otpReadonly"
              :disabled="loading"
              ref="credentialInput"
              @focus="onOtpFocus"
              @keydown="onOtpKeydown"
              @paste="onOtpPaste"
              @input="onOtpInput"
              @keyup.enter="handleLogin"
            />
          </el-form-item>
        </template>

        <!-- 未启用 2FA：静态密码登录 -->
        <el-form-item v-else :label="t('auth.password')">
          <el-input v-model="credential" type="password" :placeholder="t('auth.password')" prefix-icon="Lock" show-password autocomplete="current-password" :disabled="loading" ref="credentialInput" @keyup.enter="handleLogin" />
        </el-form-item>

        <el-form-item>
          <el-button type="primary" style="width:100%" :loading="loading" @click="handleLogin">
            {{ twofaEnabled ? t('auth.twofaLoginButton') : t('auth.loginButton') }}
          </el-button>
        </el-form-item>
        <el-form-item v-if="passkeyAvailable">
          <el-button style="width:100%" :loading="passkeyLoading" @click="handlePasskeyLogin">
            {{ t('auth.passkeyLogin') }}
          </el-button>
          <div style="font-size:11px;color:#999;margin-top:4px">{{ t('auth.passkeyLoginHint') }}</div>
        </el-form-item>
      </el-form>

      <div style="text-align:center;margin-top:12px">
        <el-dropdown @command="setLocale">
          <span style="cursor:pointer;color:#409eff">{{ currentLocaleName }}</span>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item v-for="loc in availableLocales" :key="loc.code" :command="loc.code">
                {{ loc.flag }} {{ loc.name }}
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { reactive, ref, computed, onMounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '../store/auth'
import { setLocale, availableLocales } from '../locales'
import { getPasskeyStatus, passkeyLoginBegin, passkeyLoginFinish } from '../api/passkey'
import { twofaStatusByUsername } from '../api/twofa'
import { getCredential, hasWebAuthn } from '../utils/webauthn'

const { t, locale } = useI18n()
const router = useRouter()
const authStore = useAuthStore()
const loading = ref(false)
const passkeyLoading = ref(false)
const passkeyAvailable = ref(false)

const form = reactive({ username: '' })
const credential = ref('')
// 该用户名是否已启用 2FA —— 决定输入框是"6 位动态码/恢复码"还是"密码"。
const twofaEnabled = ref(false)
const checkingTwofa = ref(false)
const credentialInput = ref(null)
// 动态码框先只读，防止浏览器在渲染瞬间把已保存的密码自动填充进来；
// 用户点击/聚焦后解除只读。
const otpReadonly = ref(true)
// 本次输入是否来自真实键盘/粘贴（自动填充不会触发 keydown/paste）。
let otpKeyed = false
// 该框是否被用户真正输入过（自动填充不算）；用于提交前的兜底判断。
let otpTouched = false

// 形如 6 位动态码或 5 组 4 位 hex 恢复码。
function isOtpLike(v) {
  const s = String(v || '').trim()
  if (!s) return true
  if (/^\d{6,8}$/.test(s)) return true
  return /^[0-9a-fA-F]{4}(-[0-9a-fA-F]{4}){4}$/.test(s)
}

function onOtpKeydown() {
  otpKeyed = true
  otpTouched = true
}

function onOtpPaste() {
  otpKeyed = true
  otpTouched = true
}

// 清空 v-model 的同时把原生 input 的值也清掉：
// 自动填充是直接改 DOM，Vue 的 model 仍是空串时不会触发重新渲染。
function clearOtpValue() {
  credential.value = ''
  nextTick(() => {
    const el = credentialInput.value?.input || credentialInput.value?.$el?.querySelector?.('input')
    if (el && el.value) el.value = ''
  })
}

// 聚焦：解除只读；若框内已有非用户输入的旧值（浏览器自动填充），先清空。
function onOtpFocus() {
  otpReadonly.value = false
  if (!otpTouched && !isOtpLike(credential.value)) clearOtpValue()
}

function onOtpInput() {
  // 没有按键/粘贴却出现密码形态的值 → 自动填充，清掉，避免把静态密码当动态码提交。
  if (!otpKeyed && !isOtpLike(credential.value)) clearOtpValue()
  otpKeyed = false
}

const currentLocaleName = computed(() => {
  const found = availableLocales.find(l => l.code === locale.value)
  return found ? `${found.flag} ${found.name}` : locale.value
})

onMounted(async () => {
  // 纯 IP / HTTP / 未在白名单的域名会自动隐藏 Passkey 按钮（设计上如此：不报错不说教）。
  if (!hasWebAuthn()) return
  try {
    const res = await getPasskeyStatus()
    passkeyAvailable.value = !!res.available
  } catch (e) { passkeyAvailable.value = false }
})

// 用户名输入后延时查询其 2FA 状态（400ms 防抖），据此切换输入模式。
let usernameTimer = null
function onUsernameInput() {
  clearTimeout(usernameTimer)
  twofaEnabled.value = false
  credential.value = ''
  otpReadonly.value = true
  otpKeyed = false
  otpTouched = false
  usernameTimer = setTimeout(checkTwofaStatus, 400)
}

async function checkTwofaStatus() {
  const name = form.username.trim()
  if (!name) { twofaEnabled.value = false; return }
  checkingTwofa.value = true
  try {
    const res = await twofaStatusByUsername(name)
    twofaEnabled.value = !!res.enabled
  } catch (e) { twofaEnabled.value = false }
  finally { checkingTwofa.value = false }
}

function focusCredential() {
  credentialInput.value?.focus?.()
}

async function handleLogin() {
  // 兜底：2FA 模式下若框内的值不是用户亲手输入/粘贴的（浏览器自动填充的密码），
  // 直接丢掉并提示，避免把静态密码当动态码提交后只报"登录失败"。
  if (twofaEnabled.value && !otpTouched) {
    clearOtpValue()
    ElMessage.warning(t('auth.twofaRequired'))
    return
  }
  if (!form.username.trim() || !credential.value.trim()) {
    ElMessage.warning(t('auth.invalidCredentials'))
    return
  }
  loading.value = true
  try {
    // 后端按用户状态自动识别凭据：2FA 用户用动态码/恢复码，其它用户用密码。
    await authStore.login(form.username.trim(), credential.value.trim())
    ElMessage.success(t('auth.loginSuccess'))
    router.push('/dashboard')
  } catch (e) {
    ElMessage.error(t('auth.loginFailed'))
  } finally { loading.value = false }
}

async function handlePasskeyLogin() {
  passkeyLoading.value = true
  try {
    // 环境性首次失败自动重试一次（Windows Hello / 安全密钥初始化、浏览器
    // 激活窗口丢失等会让第一次 credentials.get 抛 NotAllowedError/AbortError，
    // 而第二次点击往往就成功）。重试后仍失败才提示，避免用户"点两次"。
    for (let attempt = 1; attempt <= 2; attempt++) {
      let begin
      try {
        begin = await passkeyLoginBegin()
      } catch (e) {
        ElMessage.error(t('auth.passkeyLoginFailed'))
        return
      }
      let response
      try {
        response = await getCredential(begin.options)
      } catch (e) {
        const cancelled = e?.name === 'NotAllowedError' || e?.name === 'AbortError' || e?.code === 20
        if (cancelled && attempt === 1) continue // 自动重试一次
        ElMessage.info(t('auth.passkeyCancelled'))
        return
      }
      const res = await passkeyLoginFinish(begin.session_token, response)
      authStore.setTokens(res)
      ElMessage.success(t('auth.loginSuccess'))
      router.push('/dashboard')
      return
    }
  } catch (e) {
    ElMessage.error(t('auth.passkeyLoginFailed'))
  } finally { passkeyLoading.value = false }
}
</script>

<style scoped>
.login-container { height:100vh; display:flex; align-items:center; justify-content:center; background:#f5f7fa; }
.login-card { width:400px; }
@media (max-width: 768px) {
  .login-container { height:auto; min-height:100vh; padding:24px 12px; align-items:flex-start; }
  .login-card { width:100%; max-width:400px; margin:auto; }
}
</style>