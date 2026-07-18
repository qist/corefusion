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

export function generateVerifyFile(id) {
  return client.post(`/charge-sites/${id}/generate-verify`)
}

export function verifyDomainHTTP(id) {
  return client.post(`/charge-sites/${id}/verify-domain`)
}

export function verifyDomainDNS(id) {
  return client.post(`/charge-sites/${id}/verify-dns`)
}

export function getSiteIntegrationDoc(id) {
  return client.get(`/charge-sites/${id}/integration-doc`)
}

// ──────────── Secret Rotation (密钥轮换) ────────────

// v3.3: newSecret 为空时后端自动生成
export function rotateSiteSecret(id, newSecret) {
  const body = newSecret ? { new_secret: newSecret } : {}
  return client.post(`/charge-sites/${id}/rotate-secret`, body)
}

export function clearPrevSiteSecret(id) {
  return client.post(`/charge-sites/${id}/clear-prev-secret`)
}

// ──────────── Callback Logs (回调日志) ────────────

export function listCallbackLogs(params = {}) {
  return client.get('/charge-callback-logs', { params })
}

// ──────────── Callback Retries (回调重试) ────────────

export function listCallbackRetries(params = {}) {
  return client.get('/charge-callback-retries', { params })
}

export function retryCallback(id) {
  return client.post(`/charge-callback-retries/${id}/retry`)
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

// v3.3: 充值卡使用记录
export function getCardUsages(id) {
  return client.get(`/charge-cards/${id}/usages`)
}

// v3.3: 导出充值卡 CSV
export function exportCardsCSV(params = {}) {
  return client.get('/charge-cards/export-csv', { params, responseType: 'blob' })
}

// ──────────── Client Balances (客户余额) ────────────

export function listClientBalances(params = {}) {
  return client.get('/client-balances', { params })
}

export function listBalanceTransactions(params = {}) {
  return client.get('/client-balances/transactions', { params })
}
