import client from './client'

// ──────────── Referral Withdrawals (推广提现) ────────────

export function listReferralWithdrawals(params = {}) {
  return client.get('/referral-withdrawals', { params })
}

export function approveReferralWithdrawal(id, data = {}) {
  return client.post(`/referral-withdrawals/${id}/approve`, data)
}

export function rejectReferralWithdrawal(id, data = {}) {
  return client.post(`/referral-withdrawals/${id}/reject`, data)
}

// ──────────── Referral Admin ────────────

export function getReferralStats() {
  return client.get('/referral/stats')
}

export function listReferralRecords(params = {}) {
  return client.get('/referral/records', { params })
}

// ──────────── Tickets (工单) ────────────

export function listTickets(params = {}) {
  return client.get('/tickets', { params })
}

export function getTicket(id) {
  return client.get(`/tickets/${id}`)
}

export function replyTicket(id, data) {
  return client.post(`/tickets/${id}/reply`, data)
}

export function closeTicket(id) {
  return client.post(`/tickets/${id}/close`)
}

// ──────────── Client Gifts (客户福利) ────────────

export function listClientGifts(params = {}) {
  return client.get('/client-gifts', { params })
}

export function createClientGift(data) {
  return client.post('/client-gifts', data)
}

export function batchCreateClientGifts(data) {
  return client.post('/client-gifts/batch', data)
}

export function searchGiftClients(keyword) {
  return client.get('/client-gifts/search-clients', { params: { keyword } })
}

// ──────────── Articles (教程文章) ────────────

export function listArticles(params = {}) {
  return client.get('/articles', { params })
}

export function getArticle(id) {
  return client.get(`/articles/${id}`)
}

export function createArticle(data) {
  return client.post('/articles', data)
}

export function updateArticle(id, data) {
  return client.put(`/articles/${id}`, data)
}

export function deleteArticle(id) {
  return client.delete(`/articles/${id}`)
}

// ──────────── Shared Accounts (共享账号) ────────────

export function listSharedAccounts(params = {}) {
  return client.get('/shared-accounts', { params })
}

export function getSharedAccount(id) {
  return client.get(`/shared-accounts/${id}`)
}

export function createSharedAccount(data) {
  return client.post('/shared-accounts', data)
}

export function updateSharedAccount(id, data) {
  return client.put(`/shared-accounts/${id}`, data)
}

export function deleteSharedAccount(id) {
  return client.delete(`/shared-accounts/${id}`)
}

export function listSharedAccountRepairLogs(id, params = {}) {
  return client.get(`/shared-accounts/${id}/repair-logs`, { params })
}

// ──────────── Ticket Admin ────────────

export function assignTicket(id, data) {
  return client.post(`/tickets/${id}/assign`, data)
}

// ──────────── IP Whitelist Admin ────────────

export function listAllIPWhitelists(params = {}) {
  return client.get('/ip-whitelists', { params })
}

// ──────────── OAuth Bindings Admin ────────────

export function listOAuthBindings(params = {}) {
  return client.get('/client-oauth-bindings', { params })
}
