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
</template>

<script setup>
import { ref, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '../store/auth'
import { setLocale, availableLocales } from '../locales'

const { t, locale } = useI18n()
const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()

const currentLocaleName = computed(() => {
  const found = availableLocales.find(l => l.code === locale.value)
  return found ? `${found.flag} ${found.name}` : locale.value
})

const menuItems = [
  { path: '/dashboard', title: 'menu.dashboard', icon: 'Odometer' },
  { path: '/node-servers', title: 'menu.nodeServers', icon: 'Cpu' },
  { path: '/services', title: 'menu.services', icon: 'Monitor' },
  { path: '/subscriptions', title: 'menu.subscriptions', icon: 'Link' },
  { path: '/plans', title: 'menu.planManagement', icon: 'Tickets' },
  { path: '/charge', title: 'menu.chargeManagement', icon: 'Wallet' },
  { path: '/tickets', title: 'menu.ticketManagement', icon: 'ChatDotRound' },
  { path: '/client-ext', title: 'menu.clientExtManagement', icon: 'Grid' },
  { path: '/clients', title: 'menu.clients', icon: 'User' },
  { path: '/certificates', title: 'menu.certificates', icon: 'Lock' },
  { path: '/users', title: 'menu.users', icon: 'User' },
  { path: '/log-manage', title: 'menu.logManage', icon: 'Document' },
  { path: '/portal', title: 'menu.portalManagement', icon: 'Monitor' },
  { path: '/settings', title: 'menu.settings', icon: 'Setting' },
  { path: '/audit', title: 'menu.audit', icon: 'Bell' },
  { path: '/runtime', title: 'menu.runtimeConfig', icon: 'Switch' },
]

const pwdVisible = ref(false)
const pwdSaving = ref(false)
const pwdForm = ref({ old_password: '', new_password: '' })

function changeLocale(code) { setLocale(code) }
function handleCommand(cmd) {
  if (cmd === 'logout') {
    authStore.logout()
    router.push('/login')
  } else if (cmd === 'password') {
    pwdForm.value = { old_password: '', new_password: '' }
    pwdVisible.value = true
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
  } catch (e) {} finally { pwdSaving.value = false }
}
</script>

<style scoped>.el-menu { border-right:none; }</style>
