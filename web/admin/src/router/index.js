import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../store/auth'

// Use injected base path from server, or detect from URL
function getBasePath() {
  // Server-injected base path (highest priority)
  if (window.__CF_BASE_PATH__) {
    return window.__CF_BASE_PATH__.replace(/\/$/, '') || '/'
  }
  // Fallback: detect from URL
  const path = window.location.pathname
  const segments = path.split('/').filter(Boolean)
  if (segments.length > 0) {
    const knownRoutes = ['login', 'dashboard', 'services', 'subscriptions', 'certificates', 'users', 'settings', 'node-servers', 'online-users', 'plans', 'clients', 'log-manage', 'portal', 'legal-documents', 'detail', 'charge', 'tickets', 'client-ext', 'audit', 'runtime', 'traffic-quota', 'db-stats', 'waf-rules', 'chain-proxy']
    if (segments.length > 1 && !knownRoutes.includes(segments[0])) {
      return '/' + segments[0]
    }
  }
  return '/'
}

const basePath = getBasePath()

// 历史 bug 会把 base 拼两次（/h7zgitxltt5z/h7zgitxltt5z/login）：这里自愈一次，
// 否则用户从旧书签/旧跳转进来会落到不存在的路由上，表现是"登录页打不开"。
if (basePath !== '/' && basePath !== '') {
  const doubled = basePath + basePath
  if (window.location.pathname.startsWith(doubled)) {
    window.location.replace(
      window.location.pathname.slice(basePath.length) + window.location.search + window.location.hash
    )
  }
}

const routes = [
  {
    path: '/init',
    name: 'InitWizard',
    component: () => import('../views/InitWizard.vue'),
  },
  {
    path: '/login',
    name: 'Login',
    component: () => import('../views/Login.vue'),
  },
  {
    path: '/maintenance',
    name: 'Maintenance',
    component: () => import('../views/Maintenance.vue'),
  },
  {
    path: '/',
    component: () => import('../views/Layout.vue'),
    redirect: '/dashboard',
    children: [
      { path: 'dashboard', name: 'Dashboard', component: () => import('../views/Dashboard.vue'), meta: { title: 'menu.dashboard', icon: 'Odometer' } },
      { path: 'services', name: 'Services', component: () => import('../views/Services.vue'), meta: { title: 'menu.services', icon: 'Monitor' } },
      { path: 'subscriptions', name: 'Subscriptions', component: () => import('../views/SubscriptionCenter.vue'), meta: { title: 'menu.subscriptions', icon: 'Link' } },
      { path: 'plans', name: 'PlanManagement', component: () => import('../views/PlanManagement.vue'), meta: { title: 'menu.planManagement', icon: 'Tickets' } },
      { path: 'node-servers', name: 'NodeServers', component: () => import('../views/NodeServers.vue'), meta: { title: 'menu.nodeServers', icon: 'Cpu' } },
      { path: 'online-users', name: 'OnlineUsers', component: () => import('../views/OnlineUsers.vue'), meta: { title: 'menu.onlineUsers', icon: 'Connection' } },
      { path: 'node-servers/:id/detail', name: 'NodeServerDetail', component: () => import('../views/NodeServerDetail.vue'), meta: { title: 'menu.nodeServers', icon: 'Cpu' } },
      { path: 'chain-proxy', name: 'ChainProxy', component: () => import('../views/ChainProxy.vue'), meta: { title: 'menu.chainProxy', icon: 'Share' } },
      { path: 'charge', name: 'ChargeManagement', component: () => import('../views/ChargeManagement.vue'), meta: { title: 'menu.chargeManagement', icon: 'Wallet' } },
      { path: 'tickets', name: 'TicketManagement', component: () => import('../views/TicketManagement.vue'), meta: { title: 'menu.ticketManagement', icon: 'ChatDotRound' } },
      { path: 'client-ext', name: 'ClientExtManagement', component: () => import('../views/ClientExtManagement.vue'), meta: { title: 'menu.clientExtManagement', icon: 'Grid' } },
      { path: 'clients', name: 'Clients', component: () => import('../views/Clients.vue'), meta: { title: 'client.title', icon: 'User' } },
      { path: 'certificates', name: 'Certificates', component: () => import('../views/Certificates.vue'), meta: { title: 'menu.certificates', icon: 'Lock' } },
      { path: 'users', name: 'Users', component: () => import('../views/Users.vue'), meta: { title: 'menu.users', icon: 'User' } },
      { path: 'log-manage', name: 'LogManage', component: () => import('../views/LogManage.vue'), meta: { title: 'menu.logManage', icon: 'Document' } },
      { path: 'portal', name: 'PortalManagement', component: () => import('../views/PortalManagement.vue'), meta: { title: 'menu.portalManagement', icon: 'Monitor' } },
      { path: 'legal-documents', name: 'LegalDocuments', component: () => import('../views/LegalDocuments.vue'), meta: { title: 'menu.legalDocuments', icon: 'Document' } },
      { path: 'settings', name: 'Settings', component: () => import('../views/Settings.vue'), meta: { title: 'menu.settings', icon: 'Setting' } },
      { path: 'db-stats', name: 'DatabaseStats', component: () => import('../views/DatabaseStats.vue'), meta: { title: 'menu.dbStats', icon: 'DataAnalysis' } },
      { path: 'audit', name: 'Security', component: () => import('../views/Security.vue'), meta: { title: 'menu.audit', icon: 'Bell' } },
      { path: 'runtime', name: 'RuntimeConfig', component: () => import('../views/RuntimeConfig.vue'), meta: { title: 'menu.runtimeConfig', icon: 'Switch' } },
      { path: 'traffic-quota', name: 'TrafficQuota', component: () => import('../views/TrafficQuota.vue'), meta: { title: 'menu.trafficQuota', icon: 'DataLine' } },
          ],
  },
]

const router = createRouter({
  history: createWebHistory(basePath),
  routes,
})

// ── 懒加载 chunk 失效自愈 ─────────────────────────────────────────────
// 路由组件都是 `() => import(...)`，构建产物文件名带内容 hash。若标签页是在
// 面板升级/重启之前打开的（"页面停留久了"），旧 index 引用的 chunk 已随新
// 构建被删除：点侧边栏时动态 import 404，vue-router 直接中止导航，el-menu
// 又吞掉 rejection —— 表现为"点菜单没反应，只有刷新才好"。
// 这里捕获这类错误后整页跳到目标路由，自动加载新版本；10s 内只自愈一次，
// 避免部署异常时无限刷新。
const CHUNK_RELOAD_KEY = '__cf_chunk_reload_at'
const CHUNK_ERROR_RE = /Failed to fetch dynamically imported module|Importing a module script failed|error loading dynamically imported module|Loading chunk \S+ failed/i

function isChunkLoadError(err) {
  return CHUNK_ERROR_RE.test(String(err?.message || err || ''))
}

function recoverFromChunkError(to) {
  const now = Date.now()
  const last = Number(sessionStorage.getItem(CHUNK_RELOAD_KEY) || 0)
  if (now - last < 10000) return
  sessionStorage.setItem(CHUNK_RELOAD_KEY, String(now))
  const base = basePath === '/' ? '' : basePath
  if (to?.fullPath) window.location.assign(base + to.fullPath)
  else window.location.reload()
}

router.onError((err, to) => {
  if (isChunkLoadError(err)) recoverFromChunkError(to)
})

// Vite 的 modulepreload 失败会先派发该事件（早于路由导航）
window.addEventListener('vite:preloadError', (e) => {
  e.preventDefault()
  recoverFromChunkError(null)
})

// Navigation guard — restores user info on page refresh
router.beforeEach(async (to, from, next) => {
  const token = localStorage.getItem('access_token')
  // ⚠ 这里必须是路由路径，不能带 base：createWebHistory(basePath) 已经加过一次，
  // 再拼一次就会变成 /<base>/<base>/login（真机踩到，登录页直接打不开）。
  const loginPath = '/login'

  // Allow login and maintenance pages through
  if (to.path.endsWith('/login') || to.path.endsWith('/maintenance') || to.path.endsWith('/init')) {
    next()
    return
  }

  // No token → redirect to login
  if (!token) {
    next(loginPath)
    return
  }

  // Token exists but user info not loaded (e.g. after page refresh) → fetch it
  const authStore = useAuthStore()
  if (!authStore.user) {
    try {
      await authStore.fetchMe()
      next()
    } catch {
      // Token expired or invalid → clear and redirect to login
      authStore.logout()
      next(loginPath)
    }
  } else {
    next()
  }
})

export default router
