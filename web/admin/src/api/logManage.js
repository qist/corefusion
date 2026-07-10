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

export function getCoreStdoutLog(type, lines = 500, serverId = 1) {
  const token = localStorage.getItem('access_token')
  const base = client.defaults.baseURL || '/api/v1'
  return fetch(`${base}/log-manage/core/${type}/stdout-log?lines=${lines}&server_id=${serverId}&token=${encodeURIComponent(token)}`)
    .then(r => r.json())
    .then(d => d?.data || d)
}

export function getProxyStdoutLog(type, lines = 500, serverId = 1) {
  const token = localStorage.getItem('access_token')
  const base = client.defaults.baseURL || '/api/v1'
  return fetch(`${base}/log-manage/proxy/${type}/stdout-log?lines=${lines}&server_id=${serverId}&token=${encodeURIComponent(token)}`)
    .then(r => r.json())
    .then(d => d?.data || d)
}

export function buildCoreStdoutStreamUrl(type, serverId = 1) {
  const token = localStorage.getItem('access_token')
  const base = client.defaults.baseURL || '/api/v1'
  return `${base}/log-manage/core/${type}/stdout-stream?server_id=${serverId}&token=${encodeURIComponent(token)}`
}

export function buildProxyStdoutStreamUrl(type, serverId = 1) {
  const token = localStorage.getItem('access_token')
  const base = client.defaults.baseURL || '/api/v1'
  return `${base}/log-manage/proxy/${type}/stdout-stream?server_id=${serverId}&token=${encodeURIComponent(token)}`
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

// ──────────── SSE Stream Helpers ────────────

export function buildSSEUrl(path, params = {}) {
  const token = localStorage.getItem('access_token')
  const base = client.defaults.baseURL || '/api/v1'
  const qs = new URLSearchParams({ ...params, token })
  return `${base}${path}?${qs.toString()}`
}

export function createSSE(url) {
  return new EventSource(url)
}
