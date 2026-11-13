// Pure utility helpers shared across Xray inbound components.
// Single source of truth so protocol sub-components don't duplicate logic.

import {
  generateUuid,
  generatePassword,
  generateShadowsocksKey,
  randomBytes,
  bytesToBase64,
} from '../../utils/credentials'
import { formatTraffic } from '../../utils/traffic'

export { formatTraffic }

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
  return generateUuid()
}

export function genPassword() {
  return generatePassword()
}

export function genBase64Password() {
  return generatePassword()
}

export function genAES128Key() {
  return bytesToBase64(randomBytes(16))
}

export function genEmail() { return Math.random().toString(36).substring(2, 8) + '@xray.com' }

export function isSS2022(method) { return method && method.startsWith('2022') }

export function isSSMultiUser(method) {
  // Only 2022-blake3-aes-*-gcm support multi-user
  return method === '2022-blake3-aes-256-gcm' || method === '2022-blake3-aes-128-gcm'
}

export function genSSPassword(method) {
  return generateShadowsocksKey(method)
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