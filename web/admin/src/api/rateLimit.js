import client from './client'

// ──────────── Rate Limit Config ────────────

export function getRateLimitConfig(serverId) {
  return client.get(`/node-servers/${serverId}/rate-limit`)
}

export function updateRateLimitConfig(serverId, data) {
  return client.put(`/node-servers/${serverId}/rate-limit`, data)
}

export function getRateLimitStatus(serverId) {
  return client.get(`/node-servers/${serverId}/rate-limit/status`)
}
