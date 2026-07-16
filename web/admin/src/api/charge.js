import client from './client'

// ──────────── Charge Sites (充值站点) ────────────

export function listChargeSites() {
  return client.get('/charge-sites')
}

export function getChargeSite(id) {
  return client.get(`/charge-sites/${id}`)
}

export function createChargeSite(data) {
  return client.post('/charge-sites', data)
}

export function updateChargeSite(id, data) {
  return client.put(`/charge-sites/${id}`, data)
}

export function deleteChargeSite(id) {
  return client.delete(`/charge-sites/${id}`)
}

// ──────────── Charge Orders (充值订单) ────────────

export function listChargeOrders(params = {}) {
  return client.get('/charge-orders', { params })
}

export function getChargeOrder(id) {
  return client.get(`/charge-orders/${id}`)
}

export function refundChargeOrder(id, data = {}) {
  return client.post(`/charge-orders/${id}/refund`, data)
}

// ──────────── Charge Cards (充值卡) ────────────

export function listChargeCards(params = {}) {
  return client.get('/charge-cards', { params })
}

export function createChargeCard(data) {
  return client.post('/charge-cards', data)
}

export function batchCreateChargeCards(data) {
  return client.post('/charge-cards/batch', data)
}

export function deleteChargeCard(id) {
  return client.delete(`/charge-cards/${id}`)
}

// ──────────── Client Balances (客户余额) ────────────

export function listClientBalances(params = {}) {
  return client.get('/client-balances', { params })
}

export function adjustClientBalance(data) {
  return client.post('/client-balances/adjust', data)
}

export function listBalanceTransactions(params = {}) {
  return client.get('/client-balances/transactions', { params })
}
