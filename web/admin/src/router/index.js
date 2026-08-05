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
    const knownRoutes = ['login', 'dashboard', 'services', 'subscriptions', 'certificates', 'users', 'settings', 'node-servers', 'online-users', 'plans', 'clients', 'log-manage', 'portal', 'detail', 'charge', 'tickets', 'client-ext', 'audit', 'runtime', 'traffic-quota']
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
      { path: 'charge', name: 'ChargeManagement', component: () => import('../views/ChargeManagement.vue'), meta: { title: 'menu.chargeManagement', icon: 'Wallet' } },
      { path: 'tickets', name: 'TicketManagement', component: () => import('../views/TicketManagement.vue'), meta: { title: 'menu.ticketManagement', icon: 'ChatDotRound' } },
      { path: 'client-ext', name: 'ClientExtManagement', component: () => import('../views/ClientExtManagement.vue'), meta: { title: 'menu.clientExtManagement', icon: 'Grid' } },
      { path: 'clients', name: 'Clients', component: () => import('../views/Clients.vue'), meta: { title: 'client.title', icon: 'User' } },
      { path: 'certificates', name: 'Certificates', component: () => import('../views/Certificates.vue'), meta: { title: 'menu.certificates', icon: 'Lock' } },
      { path: 'users', name: 'Users', component: () => import('../views/Users.vue'), meta: { title: 'menu.users', icon: 'User' } },
      { path: 'log-manage', name: 'LogManage', component: () => import('../views/LogManage.vue'), meta: { title: 'menu.logManage', icon: 'Document' } },
      { path: 'portal', name: 'PortalManagement', component: () => import('../views/PortalManagement.vue'), meta: { title: 'menu.portalManagement', icon: 'Monitor' } },
      { path: 'settings', name: 'Settings', component: () => import('../views/Settings.vue'), meta: { title: 'menu.settings', icon: 'Setting' } },
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

// Navigation guard — restores user info on page refresh
router.beforeEach(async (to, from, next) => {
  const token = localStorage.getItem('access_token')
  const loginPath = basePath === '/' ? '/login' : basePath + '/login'

  // Allow login and maintenance pages through
  if (to.path.endsWith('/login') || to.path.endsWith('/maintenance')) {
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
