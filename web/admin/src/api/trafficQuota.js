import client from './client'

// ──────────── Traffic Quota Monitoring (read-only) ────────────
// Quota enforcement lives in the subscription/plan engine (node_servers +
// node_traffic_usage). This surface only observes effective quotas, per-node
// usage and reset logs sourced from those engine-backed tables.

export function listTrafficQuotas(params = {}) {
  return client.get('/traffic/quotas', { params })
}

export function getTrafficQuotaUsage(id) {
  return client.get(`/traffic/quotas/${id}/usage`)
}

export function getTrafficUsageHistory(params = {}) {
  return client.get('/traffic/usage-history', { params })
}

export function listTrafficResetLogs(params = {}) {
  return client.get('/traffic/reset-logs', { params })
}
