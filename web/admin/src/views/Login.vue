<template>
  <div class="login-container">
    <el-card class="login-card">
      <h2 style="text-align:center;margin-bottom:30px">{{ t('common.appName') }}</h2>
      <el-form :model="form" @submit.prevent="handleLogin" label-position="top">
        <el-form-item :label="t('auth.username')">
          <el-input v-model="form.username" :placeholder="t('auth.username')" prefix-icon="User" />
        </el-form-item>
        <el-form-item :label="t('auth.password')">
          <el-input v-model="form.password" type="password" :placeholder="t('auth.password')" prefix-icon="Lock" show-password @keyup.enter="handleLogin" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" style="width:100%" :loading="loading" @click="handleLogin">{{ t('auth.loginButton') }}</el-button>
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
import { reactive, ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '../store/auth'
import { setLocale, availableLocales } from '../locales'
import { getPasskeyStatus, passkeyLoginBegin, passkeyLoginFinish } from '../api/passkey'
import { getCredential, hasWebAuthn } from '../utils/webauthn'

const { t, locale } = useI18n()
const router = useRouter()
const authStore = useAuthStore()
const loading = ref(false)
const passkeyLoading = ref(false)
const passkeyAvailable = ref(false)

const form = reactive({ username: '', password: '' })

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

async function handleLogin() {
  if (!form.username || !form.password) {
    ElMessage.warning(t('auth.invalidCredentials'))
    return
  }
  loading.value = true
  try {
    await authStore.login(form.username, form.password)
    ElMessage.success(t('auth.loginSuccess'))
    router.push('/dashboard')
  } catch (e) { ElMessage.error(t('common.failed')) } finally { loading.value = false }
}

async function handlePasskeyLogin() {
  passkeyLoading.value = true
  try {
    const begin = await passkeyLoginBegin()
    const response = await getCredential(begin.options)
    const res = await passkeyLoginFinish(begin.session_token, response)
    authStore.setTokens(res)
    ElMessage.success(t('auth.loginSuccess'))
    router.push('/dashboard')
  } catch (e) {
    // 用户取消认证器弹窗 / 未找到可用凭证 —— 提示一次，不打断密码登录
    if (e?.name === 'NotAllowedError' || e?.code === 20) return
    ElMessage.error(t('auth.passkeyLoginFailed'))
  } finally { passkeyLoading.value = false }
}
</script>

<style scoped>
.login-container { height:100vh; display:flex; align-items:center; justify-content:center; background:#f5f7fa; }
.login-card { width:400px; }
</style>