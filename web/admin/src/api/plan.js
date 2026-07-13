import client from './client'

// ──────────── Plan Templates ────────────

export function listPlanTemplates() {
  return client.get('/plan-templates')
}

export function getPlanTemplate(id) {
  return client.get(`/plan-templates/${id}`)
}

export function createPlanTemplate(data) {
  return client.post('/plan-templates', data)
}

export function updatePlanTemplate(id, data) {
  return client.put(`/plan-templates/${id}`, data)
}

export function deletePlanTemplate(id) {
  return client.delete(`/plan-templates/${id}`)
}

export function setTemplateNodeGroups(id, nodeGroupIds) {
  return client.put(`/plan-templates/${id}/node-groups`, { node_group_ids: nodeGroupIds })
}

export function getAccessibleNodes(id) {
  return client.get(`/plan-templates/${id}/accessible-nodes`)
}

export function getBillingNodeOptions(id) {
  return client.get(`/plan-templates/${id}/billing-node-options`)
}

export function listAllInboundsWithServer() {
  return client.get('/inbounds-with-server')
}

// ──────────── Plan Variants ────────────

export function listPlanVariants(templateId) {
  return client.get(`/plan-templates/${templateId}/variants`)
}

export function createPlanVariant(templateId, data) {
  return client.post(`/plan-templates/${templateId}/variants`, data)
}

export function updatePlanVariant(id, data) {
  return client.put(`/plan-variants/${id}`, data)
}

export function deletePlanVariant(id) {
  return client.delete(`/plan-variants/${id}`)
}

// ──────────── Node Groups ────────────

export function listNodeGroups() {
  return client.get('/node-groups')
}

export function createNodeGroup(data) {
  return client.post('/node-groups', data)
}

export function updateNodeGroup(id, data) {
  return client.put(`/node-groups/${id}`, data)
}

export function deleteNodeGroup(id) {
  return client.delete(`/node-groups/${id}`)
}

export function listGroupMembers(id) {
  return client.get(`/node-groups/${id}/members`)
}

export function addGroupMember(id, data) {
  return client.post(`/node-groups/${id}/members`, data)
}

export function removeGroupMember(groupId, memberId) {
  return client.delete(`/node-groups/${groupId}/members/${memberId}`)
}

// ──────────── Promotions ────────────

export function listPromotions(params = {}) {
  return client.get('/promotions', { params })
}

export function createPromotion(data) {
  return client.post('/promotions', data)
}

export function updatePromotion(id, data) {
  return client.put(`/promotions/${id}`, data)
}

export function deletePromotion(id) {
  return client.delete(`/promotions/${id}`)
}

export function validatePromotion(code, variantId) {
  return client.post('/promotions/validate', { code, variant_id: variantId })
}

// ──────────── Client Plan Operations ────────────

export function listClientOrders(clientId, params = {}) {
  return client.get(`/clients/${clientId}/orders`, { params })
}

export function renewClient(clientId, data) {
  return client.post(`/clients/${clientId}/renew`, data)
}

export function upgradeClient(clientId, data) {
  return client.post(`/clients/${clientId}/upgrade`, data)
}

export function downgradeClient(clientId, data) {
  return client.post(`/clients/${clientId}/downgrade`, data)
}
