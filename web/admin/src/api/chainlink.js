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

// ──────────── Create Chain Proxy Wizard (§5.5) ────────────
// Body: { entry_server_id, entry_inbound_id, exits: [{ is_external, id, priority, weight, sort_order }] }
export function createChainProxy(data) {
  return client.post('/chain/proxy', data)
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

// ──────────── External Nodes (for registration dropdown) ────────────

export function listExternalNodes() {
  return client.get('/external-nodes')
}

// ──────────── Topology ────────────

export function getTopology() {
  return client.get('/chain/topology')
}

// ──────────── Health ────────────

export function getChainHealth() {
  return client.get('/chain/health')
}
