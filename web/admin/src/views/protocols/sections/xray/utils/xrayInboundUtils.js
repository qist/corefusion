// Pure utility helpers shared across Xray inbound components.
// Single source of truth so protocol sub-components don't duplicate logic.

export function genId(len = 10) {
  const chars = 'abcdefghijklmnopqrstuvwxyz0123456789'
  let r = ''
  for (let i = 0; i < len; i++) r += chars.charAt(Math.floor(Math.random() * chars.length))
  return r
}

// 「节点名称」生成器与 sing-box 共用同一实现（见 sections/utils/nodeNaming.js），
// 这里只做转发，避免两套口径。
export { genRemark } from '../../utils/nodeNaming'

export function genPort() { return Math.floor(Math.random() * 50000 + 10000) }

export function genPath() { return '/' + genId(10) }

export function genUUID() {
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function (c) {
    const r = Math.random() * 16 | 0
    const v = c === 'x' ? r : (r & 0x3 | 0x8)
    return v.toString(16)
  })
}

export function genPassword() {
  const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789'
  let r = ''
  for (let i = 0; i < 12; i++) r += chars.charAt(Math.floor(Math.random() * chars.length))
  return r
}

export function genBase64Password() {
  const bytes = new Uint8Array(32)
  crypto.getRandomValues(bytes)
  return btoa(String.fromCharCode(...bytes))
}

export function genAES128Key() {
  const bytes = new Uint8Array(16)
  crypto.getRandomValues(bytes)
  return btoa(String.fromCharCode(...bytes))
}

export function genEmail() { return Math.random().toString(36).substring(2, 8) + '@xray.com' }

export function isSS2022(method) { return method && method.startsWith('2022') }

export function isSSMultiUser(method) {
  // Only 2022-blake3-aes-*-gcm support multi-user
  return method === '2022-blake3-aes-256-gcm' || method === '2022-blake3-aes-128-gcm'
}

export function genSSPassword(method) {
  if (isSS2022(method)) {
    // 2022-blake3 methods use base64 encoded key
    const length = method === '2022-blake3-aes-128-gcm' ? 16 : 32
    const bytes = new Uint8Array(length)
    crypto.getRandomValues(bytes)
    return btoa(String.fromCharCode(...bytes))
  }
  // Legacy methods use plain text password
  const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789'
  let r = ''
  for (let i = 0; i < 16; i++) r += chars.charAt(Math.floor(Math.random() * chars.length))
  return r
}

export function formatTraffic(bytes) {
  if (!bytes) return '0 B'
  const u = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let b = bytes
  while (b >= 1024 && i < u.length - 1) { b /= 1024; i++ }
  return b.toFixed(1) + ' ' + u[i]
}

export function resetTraffic(u) { u._usedUp = 0; u._usedDown = 0 }

export function getTestseed(obj) {
  if (!obj.testseed) obj.testseed = [900, 500, 900, 256]
  return obj.testseed
}

export function randomSeed(obj) {
  if (!obj.testseed) obj.testseed = [900, 500, 900, 256]
  obj.testseed = obj.testseed.map(() => Math.floor(Math.random() * 1000) + 1)
}

export function resetSeed(obj) {
  obj.testseed = [900, 500, 900, 256]
}