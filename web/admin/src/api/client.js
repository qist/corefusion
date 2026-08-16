import axios from 'axios'
import { ElMessage } from 'element-plus'
import i18n from '../locales'

// Detect base path from current URL
function getBasePath() {
  const path = window.location.pathname
  const segments = path.split('/').filter(Boolean)
  if (segments.length > 0) {
    const knownRoutes = ['login', 'dashboard', 'protocols', 'services', 'subscriptions', 'certificates', 'users', 'settings', 'node-servers', 'plans', 'clients', 'proxy', 'log-manage', 'charge', 'tickets', 'client-ext', 'portal', 'db-stats']
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

// Response interceptor
client.interceptors.response.use(
  (response) => response.data?.data ?? response.data,
  (error) => {
    const t = i18n.global.t
    const suppressErrorMessage = error.config?.suppressErrorMessage === true
    if (error.response) {
      const { status, data } = error.response
      if (status === 401) {
        localStorage.removeItem('access_token')
        localStorage.removeItem('refresh_token')
        window.location.href = basePath + '/login'
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
