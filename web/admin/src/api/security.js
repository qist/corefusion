import client from './client'

// Audit events
export const listAuditEvents = (params) =>
  client({ url: '/security/audit-events', method: 'get', params })
export const exportAuditEvents = (params) =>
  client({ url: '/security/audit-events/export', method: 'get', params, responseType: 'blob' })

// 订阅访问日志（全部订阅，审计页排查用）
export const listSubscriptionAccessLogs = (params) =>
  client({ url: '/security/subscription-access-logs', method: 'get', params })
