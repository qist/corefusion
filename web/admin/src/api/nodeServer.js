import client from './client'

// ──────────── Node Servers ────────────

export function listNodeServers(params = {}) {
  return client.get('/node-servers', { params })
}

export function getNodeServer(id) {
  return client.get(`/node-servers/${id}`)
}

export function createNodeServer(data) {
  return client.post('/node-servers', data)
}

export function updateNodeServer(id, data) {
  return client.put(`/node-servers/${id}`, data)
}

export function deleteNodeServer(id) {
  return client.delete(`/node-servers/${id}`)
}

export function getServerStatus(id) {
  return client.get(`/node-servers/${id}/status`)
}

export function controlCore(id, coreType, action) {
  return client.post(`/node-servers/${id}/control`, { core_type: coreType, action })
}

export function syncServerConfig(id, data) {
  return client.post(`/node-servers/${id}/sync`, data)
}

// ──────────── Bootstrap Token ────────────

export function generateBootstrapToken(id) {
  return client.post(`/node-servers/${id}/bootstrap-token`)
}

export function getBootstrapCommand(id, params = {}) {
  return client.get(`/node-servers/${id}/bootstrap-cmd`, { params, responseType: 'text', suppressErrorMessage: true })
}

export function getAgentInstallScript(id, params = {}) {
  return client.get(`/node-servers/${id}/install-script`, { params, responseType: 'text', suppressErrorMessage: true })
}

// ──────────── Certificates ────────────

export function listServerCertificates(id) {
  return client.get(`/node-servers/${id}/certificates`)
}

export function associateCertificate(id, certId) {
  return client.post(`/node-servers/${id}/certificates/${certId}`)
}

export function dissociateCertificate(id, certId) {
  return client.delete(`/node-servers/${id}/certificates/${certId}`)
}

export function distributeCertificate(id, certId) {
  return client.post(`/node-servers/${id}/distribute-cert`, { certificate_id: certId })
}

export function distributeCertificateToAll(certId) {
  return client.post('/node-servers/distribute-cert', { certificate_id: certId })
}

export function revokeAgentCertificate(id, reason = '') {
  return client.post(`/node-servers/${id}/revoke-cert`, { reason })
}

// ──────────── Node Type ────────────

export function updateNodeType(id, nodeType) {
  return client.put(`/node-servers/${id}/node-type`, { node_type: nodeType })
}

export function getFailoverStatus(id) {
  return client.get(`/node-servers/${id}/failover-status`)
}

export function triggerFailoverCheck() {
  return client.post('/node-servers/failover-check')
}

export function triggerServerFailover(id) {
  return client.post(`/node-servers/${id}/failover`)
}

export function syncMirror(id) {
  return client.post(`/node-servers/${id}/sync-mirror`)
}

export function getMirrorStatus(id) {
  return client.get(`/node-servers/${id}/mirror-status`)
}

// ──────────── Chain Proxy ────────────

export function getChainConfig(id) {
  return client.get(`/node-servers/${id}/chain-config`)
}

export function getChainExitConfig(id) {
  return client.get(`/node-servers/${id}/chain-exit-config`)
}

export function getChainHealth(id) {
  return client.get(`/node-servers/${id}/chain-health`)
}

export function validateChainConfig(data) {
  return client.post('/node-servers/validate-chain-config', data)
}

export function listBillableServers() {
  return client.get('/node-servers/billable')
}

// ──────────── User Sync & Traffic ────────────

export function syncUser(clientId, action) {
  return client.post('/node-servers/users/sync', { client_id: clientId, action })
}

export function syncUserRemoval(clientId) {
  return client.delete(`/node-servers/users/${clientId}`)
}

export function getRetryQueueStatus() {
  return client.get('/node-servers/users/retry-queue')
}

export function processRetryQueue() {
  return client.post('/node-servers/users/retry-queue/process')
}

export function getTrafficStats() {
  return client.get('/node-servers/traffic/stats')
}

// ──────────── Dashboard ────────────

export function getDashboardStatus() {
  return client.get('/node-servers/dashboard')
}

export function recommendRegion(clientIp, allowedRegions) {
  return client.post('/node-servers/recommend-region', { client_ip: clientIp, allowed_regions: allowedRegions })
}

// ──────────── Metrics ────────────

export function getMetricsHistory(id, params = {}) {
  return client.get(`/node-servers/${id}/metrics-history`, { params })
}

export function getAggregatedMetrics() {
  return client.get('/node-servers/metrics/aggregated')
}

// ──────────── Speed Test ────────────

export function testServerSpeed(id) {
  return client.post(`/node-servers/${id}/speed-test`)
}

export function testAllServersSpeed() {
  return client.post('/node-servers/speed-test-all')
}

export function getSpeedTestResults() {
  return client.get('/node-servers/speed-test-results')
}

// ──────────── Agent Update ────────────

export function updateAgent(id, data) {
  return client.post(`/node-servers/${id}/update-agent`, data)
}

export function updateAllAgents() {
  return client.post('/node-servers/update-all-agents')
}

export function getPendingAgentUpdates() {
  return client.get('/node-servers/pending-agent-updates')
}

// ──────────── Agent Degradation ────────────

export function getDegradationStatus(id) {
  return client.get(`/node-servers/${id}/degradation-status`)
}

export function forceReconnect(id) {
  return client.post(`/node-servers/${id}/force-reconnect`)
}

// ──────────── Tags ────────────

export function listServerTags(id) {
  return client.get(`/node-servers/${id}/tags`)
}

export function addServerTag(id, tagKey, tagValue = '') {
  return client.post(`/node-servers/${id}/tags`, { tag_key: tagKey, tag_value: tagValue })
}

export function removeServerTag(id, tagKey) {
  return client.delete(`/node-servers/${id}/tags/${tagKey}`)
}

// ──────────── CA Center ────────────

export function getCAStatus() {
  return client.get('/ca/status')
}

export function listAgentCerts() {
  return client.get('/ca/agent-certs')
}

export function renewAgentCertificate(serverId) {
  return client.post(`/ca/agent-certs/${serverId}/renew`)
}

export function listRevocations() {
  return client.get('/ca/revocations')
}

export function getDistributionQueue(params = {}) {
  return client.get('/ca/distribution-queue', { params })
}

export function retryDistribution(id) {
  return client.post(`/ca/distribution-queue/${id}/retry`)
}

export function deleteDistributionQueueItem(id) {
  return client.delete(`/ca/distribution-queue/${id}`)
}

// ──────────── Region Overview ────────────

export function getRegionsOverview() {
  return client.get('/regions/overview')
}

export function listServersByRegion(region) {
  return client.get(`/regions/${region}/servers`)
}

// ──────────── Node Audit Logs ────────────

export function listNodeAuditLogs(params = {}) {
  return client.get('/node-audit-logs', { params })
}

export function exportNodeAuditLogs(params = {}) {
  return client.get('/node-audit-logs/export', { params, responseType: 'text', suppressErrorMessage: true })
}
