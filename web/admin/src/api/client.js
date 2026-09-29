import axios from 'axios'
import { ElMessage } from 'element-plus'
import i18n from '../locales'

// Detect base path from current URL
function getBasePath() {
  const path = window.location.pathname
  const segments = path.split('/').filter(Boolean)
  if (segments.length > 0) {
    const knownRoutes = ['login', 'dashboard', 'protocols', 'services', 'subscriptions', 'certificates', 'users', 'settings', 'node-servers', 'plans', 'clients', 'proxy', 'log-manage', 'charge', 'tickets', 'client-ext', 'portal', 'db-stats', 'waf-rules', 'chain-proxy']
    if (segments.length > 1 && !knownRoutes.includes(segments[0])) {
      return '/' + segments[0]
    }
  }
  return ''
}

const basePath = getBasePath()

const client = axios.create({
  baseURL: '/api/v1',
  timeout: 300000, // 5 minutes for large downloads
})

// Request interceptor
client.interceptors.request.use((config) => {
  const token = localStorage.getItem('access_token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// Refresh 静默续期：401 时用 refresh_token 换新 token 并重放原请求。
// 并发去重：多个请求同时 401 只发一次 refresh，其余排队等待结果。
let refreshingPromise = null
function tryRefresh() {
  const rt = localStorage.getItem('refresh_token')
  if (!rt) return Promise.resolve(false)
  if (!refreshingPromise) {
    // 用裸 axios（不带本文件的拦截器），避免递归 401
    refreshingPromise = axios.post('/api/v1/auth/refresh', { refresh_token: rt })
      .then((res) => {
        const data = res.data?.data ?? res.data
        if (!data?.access_token) return false
        localStorage.setItem('access_token', data.access_token)
        localStorage.setItem('refresh_token', data.refresh_token)
        return true
      })
      .catch(() => false)
      .finally(() => { refreshingPromise = null })
  }
  return refreshingPromise
}

// Response interceptor
client.interceptors.response.use(
  (response) => response.data?.data ?? response.data,
  async (error) => {
    const t = i18n.global.t
    const suppressErrorMessage = error.config?.suppressErrorMessage === true
    if (error.response) {
      const { status, data } = error.response
      if (status === 401) {
        // 登录流程（含 Passkey / 2FA 第二步）的 401 是**业务结果**（密码错 /
        // 动态码错 / 票据过期），由 Login.vue 自行处理 —— 强制整页跳转会丢掉
        // 已输入的密码与尚未消费的登录票据。其余接口的 401 才是会话失效。
        const reqUrl = error.config?.url || ''
        const isAuthFlow = reqUrl.includes('/auth/login') || reqUrl.includes('/auth/passkey/login') || reqUrl.includes('/auth/2fa/')
        if (!isAuthFlow) {
          // 先尝试静默续期；成功则重放原请求（停留在当前页面，不再丢位置）
          if (!error.config.__retried && await tryRefresh()) {
            const cfg = error.config
            cfg.__retried = true
            cfg.headers = cfg.headers || {}
            cfg.headers.Authorization = `Bearer ${localStorage.getItem('access_token')}`
            return client.request(cfg)
          }
          localStorage.removeItem('access_token')
          localStorage.removeItem('refresh_token')
          window.location.href = basePath + '/login'
        }
      } else if (status === 503 && data?.code === 'MAINTENANCE_MODE') {
        // Maintenance mode — redirect to maintenance page if not already there
        if (!window.location.pathname.endsWith('/maintenance')) {
          window.location.href = basePath + '/maintenance'
        }
      } else if (!suppressErrorMessage && status === 403) {
        ElMessage.error(t('error.forbidden'))
      } else if (!suppressErrorMessage && status === 404) {
        ElMessage.error(t('error.notFound'))
      } else if (!suppressErrorMessage && status >= 500) {
        ElMessage.error(data?.message || t('error.serverError'))
      } else if (!suppressErrorMessage) {
        ElMessage.error(data?.message || t('error.unknownError'))
      }
    } else if (!suppressErrorMessage) {
      ElMessage.error(t('error.networkError'))
    }
    return Promise.reject(error)
  }
)

export default client
