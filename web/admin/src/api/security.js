import client from './client'

// Audit events
export const listAuditEvents = (params) =>
  client({ url: '/security/audit-events', method: 'get', params })
export const exportAuditEvents = (params) =>
  client({ url: '/security/audit-events/export', method: 'get', params, responseType: 'blob' })

// Alerts
export const listAlerts = (params) =>
  client({ url: '/security/alerts', method: 'get', params })
export const resolveAlert = (id) =>
  client({ url: `/security/alerts/${id}/resolve`, method: 'post' })
export const resolveAllAlerts = (params) =>
  client({ url: '/security/alerts/resolve', method: 'post', params })

// Alert channels
export const listAlertChannels = () =>
  client({ url: '/security/alert-channels', method: 'get' })
export const createAlertChannel = (data) =>
  client({ url: '/security/alert-channels', method: 'post', data })
export const updateAlertChannel = (id, data) =>
  client({ url: `/security/alert-channels/${id}`, method: 'put', data })
export const deleteAlertChannel = (id) =>
  client({ url: `/security/alert-channels/${id}`, method: 'delete' })
