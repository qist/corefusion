import { createRouter, createWebHistory } from 'vue-router'

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
    const knownRoutes = ['login', 'dashboard', 'protocols', 'services', 'subscriptions', 'certificates', 'users', 'settings']
    if (segments.length > 1 && !knownRoutes.includes(segments[0])) {
      return '/' + segments[0]
    }
  }
  return '/'
}

const basePath = getBasePath()

const routes = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('../views/Login.vue'),
  },
  {
    path: '/',
    component: () => import('../views/Layout.vue'),
    redirect: '/dashboard',
    children: [
      { path: 'dashboard', name: 'Dashboard', component: () => import('../views/Dashboard.vue'), meta: { title: '仪表盘', icon: 'Odometer' } },
      { path: 'protocols', name: 'Protocols', component: () => import('../views/protocols/ProtocolCenter.vue'), meta: { title: 'menu.protoCenter', icon: 'Connection' } },
      { path: 'services', name: 'Services', component: () => import('../views/Services.vue'), meta: { title: '服务管理', icon: 'Monitor' } },
      { path: 'subscriptions', name: 'Subscriptions', component: () => import('../views/Subscriptions.vue'), meta: { title: '订阅管理', icon: 'Link' } },
      { path: 'subscription-formats', name: 'SubscriptionFormats', component: () => import('../views/SubscriptionFormats.vue'), meta: { title: 'subfmt.title', icon: 'Document' } },
      { path: 'subscription-endpoints', name: 'SubscriptionEndpoints', component: () => import('../views/SubscriptionEndpoints.vue'), meta: { title: 'subend.title', icon: 'Link' } },
      { path: 'certificates', name: 'Certificates', component: () => import('../views/Certificates.vue'), meta: { title: '证书管理', icon: 'Lock' } },
      { path: 'users', name: 'Users', component: () => import('../views/Users.vue'), meta: { title: '用户管理', icon: 'User' } },
      { path: 'proxy', name: 'Proxy', component: () => import('../views/Proxy.vue'), meta: { title: '代理管理', icon: 'Monitor' } },
      { path: 'log-manage', name: 'LogManage', component: () => import('../views/LogManage.vue'), meta: { title: '日志管理', icon: 'Document' } },
      { path: 'settings', name: 'Settings', component: () => import('../views/Settings.vue'), meta: { title: '系统设置', icon: 'Setting' } },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(basePath),
  routes,
})

// Navigation guard
router.beforeEach((to, from, next) => {
  const token = localStorage.getItem('access_token')
  const loginPath = basePath === '/' ? '/login' : basePath + '/login'
  if (!to.path.endsWith('/login') && !token) {
    next(loginPath)
  } else {
    next()
  }
})

export default router
