import client from './client'

// ──────────── Chain Links ────────────

export function listLinks(params = {}) {
  return client.get('/chain/links', { params })
}

export function getLink(id) {
  return client.get(`/chain/links/${id}`)
}

export function createLink(data) {
  return client.post('/chain/links', data)
}

export function updateLink(id, data) {
  return client.put(`/chain/links/${id}`, data)
}

export function deleteLink(id) {
  return client.delete(`/chain/links/${id}`)
}

// ──────────── Chain Exit Configs ────────────

export function listExitConfigs() {
  return client.get('/chain/exit-configs')
}

export function getExitConfig(serverId) {
  return client.get(`/chain/exit-configs/${serverId}`)
}

export function createExitConfig(data) {
  return client.post('/chain/exit-configs', data)
}

export function updateExitConfig(serverId, data) {
  return client.put(`/chain/exit-configs/${serverId}`, data)
}

export function deleteExitConfig(serverId) {
  return client.delete(`/chain/exit-configs/${serverId}`)
}

// ──────────── External Node Servers ────────────

export function listExternalServers() {
  return client.get('/chain/external-servers')
}

export function createExternalServer(data) {
  return client.post('/chain/external-servers', data)
}

export function deleteExternalServer(id) {
  return client.delete(`/chain/external-servers/${id}`)
}

// ──────────── Topology ────────────

export function getTopology() {
  return client.get('/chain/topology')
}
