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
