import client from './client'

// ──────────── Status & Discovery ────────────

export function getStatus(serverId = 1) {
  return client.get('/log-manage/status', { params: { server_id: serverId } })
}

export function discoverLogs(serverId = 1) {
  return client.post('/log-manage/discover', null, { params: { server_id: serverId } })
}

export function syncPolicies(serverId = 1) {
  return client.post('/log-manage/sync', null, { params: { server_id: serverId } })
}

// ──────────── Rotation Policies ────────────

export function listPolicies(serverId = 1) {
  return client.get('/log-manage/policies', { params: { server_id: serverId } })
}

export function getPolicy(id) {
  return client.get(`/log-manage/policies/${id}`)
}

export function createPolicy(data) {
  return client.post('/log-manage/policies', data)
}

export function updatePolicy(id, data) {
  return client.put(`/log-manage/policies/${id}`, data)
}

export function deletePolicy(id) {
  return client.delete(`/log-manage/policies/${id}`)
}

export function rotatePolicy(id) {
  return client.post(`/log-manage/policies/${id}/rotate`)
}

export function getPolicyJobs(id) {
  return client.get(`/log-manage/policies/${id}/jobs`)
}

// ──────────── Log Viewing (local) ────────────

export function getPolicyLog(id, lines = 500) {
  return client.get(`/log-manage/policies/${id}/log`, { params: { lines } })
}

export async function getCoreStdoutLog(type, lines = 500, serverId = 1) {
  const ticket = await requestTicket('logs')
  const base = client.defaults.baseURL || '/api/v1'
  return fetch(`${base}/log-manage/core/${type}/stdout-log?lines=${lines}&server_id=${serverId}&ticket=${encodeURIComponent(ticket)}`)
    .then(r => r.json())
    .then(d => d?.data || d)
}

export async function getProxyStdoutLog(type, lines = 500, serverId = 1) {
  const ticket = await requestTicket('logs')
  const base = client.defaults.baseURL || '/api/v1'
  return fetch(`${base}/log-manage/proxy/${type}/stdout-log?lines=${lines}&server_id=${serverId}&ticket=${encodeURIComponent(ticket)}`)
    .then(r => r.json())
    .then(d => d?.data || d)
}

export async function buildCoreStdoutStreamUrl(type, serverId = 1) {
  const ticket = await requestTicket('logs')
  const base = client.defaults.baseURL || '/api/v1'
  return `${base}/log-manage/core/${type}/stdout-stream?server_id=${serverId}&ticket=${encodeURIComponent(ticket)}`
}

export async function buildProxyStdoutStreamUrl(type, serverId = 1) {
  const ticket = await requestTicket('logs')
  const base = client.defaults.baseURL || '/api/v1'
  return `${base}/log-manage/proxy/${type}/stdout-stream?server_id=${serverId}&ticket=${encodeURIComponent(ticket)}`
}

// ──────────── Log Output Config (per-node) ────────────

export function getProxyLogOutput(serverId = 1) {
  return client.get('/log-manage/log-output', { params: { server_id: serverId } })
}

export function saveProxyLogOutput(data, serverId = 1) {
  return client.put('/log-manage/log-output', data, { params: { server_id: serverId } })
}

export function getCoreLogOutput(serverId = 1) {
  return client.get('/log-manage/core-log-output', { params: { server_id: serverId } })
}

export function saveCoreLogOutput(data, serverId = 1) {
  return client.put('/log-manage/core-log-output', data, { params: { server_id: serverId } })
}

export function takeoverStdout(type) {
  return client.post(`/log-manage/log-output/takeover/${type}`)
}

export function releaseStdout(type) {
  return client.post(`/log-manage/log-output/release/${type}`)
}

// ──────────── Writer Management (local + remote) ────────────

export function getWriters(serverId = 1) {
  return client.get('/log-manage/writers', { params: { server_id: serverId } })
}

export function startWriter(type, data, serverId = 1) {
  return client.post(`/log-manage/writers/${type}/start`, data, { params: { server_id: serverId } })
}

export function stopWriter(type, serverId = 1) {
  return client.post(`/log-manage/writers/${type}/stop`, null, { params: { server_id: serverId } })
}

export function updateWriterConfig(type, data, serverId = 1) {
  return client.put(`/log-manage/writers/${type}/config`, data, { params: { server_id: serverId } })
}

export function getWriterFiles(type, serverId = 1) {
  return client.get(`/log-manage/writers/${type}/files`, { params: { server_id: serverId } })
}

// ──────────── Node-based Log Management ────────────

export function listNodeLogs(serverId) {
  return client.get(`/log-manage/nodes/${serverId}/logs`)
}

export function getNodeLog(serverId, type, lines = 500, logPath = '') {
  const params = { lines }
  if (logPath) params.log_path = logPath
  return client.get(`/log-manage/nodes/${serverId}/logs/${type}`, { params })
}

export function rotateNodeLog(serverId, type, logPath = '') {
  const params = {}
  if (logPath) params.log_path = logPath
  return client.post(`/log-manage/nodes/${serverId}/logs/${type}/rotate`, null, { params })
}

// ──────────── DB Log Cleanup Rules ────────────

export function listCleanupRules() {
  return client.get('/log-manage/cleanup-rules')
}

export function updateCleanupRule(id, data) {
  return client.put(`/log-manage/cleanup-rules/${id}`, data)
}

export function cleanupAll() {
  return client.post('/log-manage/cleanup-all')
}

export function cleanupTable(table) {
  return client.post(`/log-manage/cleanup-table/${table}`)
}

// ──────────── Scheduler Interval Config ────────────

export function getSchedulerConfig() {
  return client.get('/log-manage/scheduler-config')
}

export function updateSchedulerConfig(items) {
  return client.put('/log-manage/scheduler-config', { items })
}

// ──────────── SSE Stream Helpers ────────────

// 一次性短时票据（安全审计 v5 M10）。
//
// 为什么需要：`EventSource`（日志流）与 `window.open`（下载）**无法自定义请求头**，所以
// 以前只能把长期 access token 拼进 URL —— 而 URL 会进访问日志、浏览器历史、Referer，
// 等于把长期凭证散播出去。现在先换一张 60 秒、一次性、限路径的票据，再拼 `?ticket=`。
export async function requestTicket(scope = 'logs') {
  const res = await client.post('/auth/ticket', null, { params: { scope } })
  // ⚠ 响应拦截器是 `response.data?.data ?? response.data` —— 它**已经剥掉一层** ✓，
  // 所以这里的 `res` 就是 `{ ticket, expires_in, scope }` ✓。
  // 原来按 `res.data.data.ticket` 取 ✗ → **永远是空串** ✗ → 票据被静默丢弃 ✗
  //（真机表现：票据接口 200 ✓ 但下载/日志流请求**根本没发出** ✗ → 前端只弹"操作失败" ✗）。
  // 两种层级都兼容 ✓，以后改拦截器也不会再踩 ✓。
  return res?.ticket || res?.data?.ticket || ''
}

export function buildSSEUrl(path, params = {}, ticket = '') {
  const base = client.defaults.baseURL || '/api/v1'
  const qs = new URLSearchParams({ ...params, ticket })
  return `${base}${path}?${qs.toString()}`
}

export function createSSE(url) {
  return new EventSource(url)
}
