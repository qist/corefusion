import client from './client'

// ──────────── Global WAF Rules ────────────

export function listWafRules(params = {}) {
  return client.get('/waf/rules', { params })
}

export function getWafRule(name) {
  return client.get(`/waf/rules/${name}`)
}

export function saveWafRule(name, content, mergeMode = 'replace') {
  return client.put(`/waf/rules/${name}`, { content, merge_mode: mergeMode })
}

export function deleteWafRule(name) {
  return client.delete(`/waf/rules/${name}`)
}

export function syncWafFromGitHub() {
  return client.post('/waf/sync-github')
}

export function syncWafAll() {
  return client.post('/waf/sync-all')
}

// ──────────── Per-Node WAF Rules ────────────

export function listNodeWafRules(serverId) {
  return client.get(`/node-servers/${serverId}/waf/rules`)
}

export function getNodeWafRule(serverId, name) {
  return client.get(`/node-servers/${serverId}/waf/rules/${name}`)
}

export function saveNodeWafRule(serverId, name, content, mergeMode = 'replace') {
  return client.put(`/node-servers/${serverId}/waf/rules/${name}`, { content, merge_mode: mergeMode })
}

export function deleteNodeWafRule(serverId, name) {
  return client.delete(`/node-servers/${serverId}/waf/rules/${name}`)
}

// ──────────── Per-Node WAF Config ────────────

export function getWafConfig(serverId, proxyType = 'caddy') {
  return client.get(`/node-servers/${serverId}/waf/config`, { params: { proxy_type: proxyType } })
}

export function saveWafConfig(serverId, proxyType, content) {
  return client.put(`/node-servers/${serverId}/waf/config`, { proxy_type: proxyType, content })
}

// ──────────── Per-Node Domain Config ────────────

export function listWafDomainConfigs(serverId) {
  return client.get(`/node-servers/${serverId}/waf/domain-config`)
}

export function getWafDomainConfig(serverId, domain) {
  return client.get(`/node-servers/${serverId}/waf/domain-config/${domain}`)
}

export function saveWafDomainConfig(serverId, domain, config) {
  return client.put(`/node-servers/${serverId}/waf/domain-config/${domain}`, { config })
}

export function getWafDomainStatus(serverId, domain) {
  return client.get(`/node-servers/${serverId}/waf/domain-status`, { params: { domain } })
}

export function enableWafDomain(serverId, domain) {
  return client.post(`/node-servers/${serverId}/waf/enable-domain`, { domain })
}

export function disableWafDomain(serverId, domain) {
  return client.post(`/node-servers/${serverId}/waf/disable-domain`, { domain })
}

// ──────────── Sync to Node ────────────

export function syncWafToNode(serverId) {
  return client.post(`/node-servers/${serverId}/waf/sync`)
}
