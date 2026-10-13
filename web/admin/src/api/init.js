import axios from 'axios'

// 初始化令牌：优先取 URL 上的 init_token，其次用 localStorage 里缓存的。
export function initToken() {
  const q = new URLSearchParams(window.location.search).get('init_token')
  if (q) {
    localStorage.setItem('cf_init_token', q)
    return q
  }
  return localStorage.getItem('cf_init_token') || ''
}

const client = axios.create({ baseURL: '/api/v1/init', timeout: 180000 })

client.interceptors.request.use((config) => {
  const token = initToken()
  if (token) config.headers['X-Init-Token'] = token
  return config
})

client.interceptors.response.use(
  (response) => response.data?.data ?? response.data,
  (error) => Promise.reject(error)
)

export const getInitStatus = () => client.get('/status')
export const randomPorts = (n = 1) => client.get('/random-ports', { params: { n } })
export const saveDB = (p) => client.post('/config/db', p)
export const testDB = (p) => client.post('/config/db/test', p)
export const saveDataDir = (p) => client.post('/config/datadir', p)
export const saveEnv = (p) => client.post('/config/env', p)
export const saveCore = (p) => client.post('/config/core', p)
export const saveProxy = (p) => client.post('/config/proxy', p)
export const saveSites = (p) => client.post('/config/sites', p)
export const saveAdmin = (p) => client.post('/config/admin', p)
export const saveHttps = (p) => client.post('/config/https', p)
export const getSummary = () => client.get('/summary')
export const getDNSProviders = () => client.get('/dns-providers')
export const getPublicIP = () => client.get('/public-ip')
export const complete = (p = {}) => client.post('/complete', p)

// .env 下载：浏览器直接打开链接下载，令牌走 query。
export const envDownloadUrl = () => `/api/v1/init/env?init_token=${encodeURIComponent(initToken())}`
