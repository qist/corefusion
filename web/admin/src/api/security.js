import client from './client'

// Audit events
export const listAuditEvents = (params) =>
  client({ url: '/security/audit-events', method: 'get', params })
export const exportAuditEvents = (params) =>
  client({ url: '/security/audit-events/export', method: 'get', params, responseType: 'blob' })
