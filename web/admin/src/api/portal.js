import client from './client'

// Portal site management (active config — direct edit)
export function getActivePortal() {
  return client.get('/portal/active')
}

export function updateActivePortal(data) {
  return client.put('/portal/active', data)
}

export function resetPortal() {
  return client.post('/portal/reset')
}

// Render cache management
export function getPortalCacheStatus() {
  return client.get('/portal/cache')
}

export function clearPortalCache() {
  return client.delete('/portal/cache')
}

export function rebuildPortalCache() {
  return client.post('/portal/cache/rebuild')
}

export function setPortalDevMode(devMode) {
  return client.put('/portal/cache/dev-mode', { dev_mode: devMode })
}

// Legacy template CRUD (kept for compatibility)
export function listPortalTemplates() {
  return client.get('/portal/templates')
}

export function getPortalTemplate(id) {
  return client.get(`/portal/templates/${id}`)
}

export function createPortalTemplate(data) {
  return client.post('/portal/templates', data)
}

export function updatePortalTemplate(id, data) {
  return client.put(`/portal/templates/${id}`, data)
}

export function deletePortalTemplate(id) {
  return client.delete(`/portal/templates/${id}`)
}

export function activatePortalTemplate(id) {
  return client.post(`/portal/templates/${id}/activate`)
}
