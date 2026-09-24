// WebAuthn 浏览器端执行工具。
//
// 职责：
//  1. 把后端下发的 options JSON（PublicKeyCredentialCreationOptionsJSON /
//     PublicKeyCredentialRequestOptionsJSON）转换成 navigator.credentials 需要的
//     buffer 形态（优先用浏览器原生 parse*FromJSON，老浏览器手工转换兜底）；
//  2. 把认证器返回的 PublicKeyCredential 序列化成后端可解析的 JSON
//     （base64url 编码所有 ArrayBuffer 字段）。
//
// ⚠ 仅在 secure context（HTTPS 或 localhost）下 WebAuthn API 才可用，
// 调用方（Login.vue / Layout.vue）应先用 /auth/passkey/status 探测。

function bytesToBase64url(bytes) {
  let binary = ''
  const chunk = 0x8000
  for (let i = 0; i < bytes.length; i += chunk) {
    binary += String.fromCharCode(...bytes.subarray(i, i + chunk))
  }
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

function base64urlToBytes(b64) {
  const base64 = b64.replace(/-/g, '+').replace(/_/g, '/')
  const padded = base64 + '='.repeat((4 - (base64.length % 4)) % 4)
  const binary = atob(padded)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i)
  return bytes
}

// 递归把 options JSON 中的 base64url 字符串字段还原为 ArrayBuffer。
// 覆盖：challenge / user.id / excludeCredentials[].id / allowCredentials[].id。
function decodeOptions(options) {
  const clone = JSON.parse(JSON.stringify(options))
  const decodeIdFields = (arr) => {
    if (!Array.isArray(arr)) return
    for (const item of arr) {
      if (item && typeof item.id === 'string') {
        item.id = base64urlToBytes(item.id).buffer
      }
    }
  }
  if (clone.challenge) clone.challenge = base64urlToBytes(clone.challenge).buffer
  if (clone.user && clone.user.id) clone.user.id = base64urlToBytes(clone.user.id).buffer
  decodeIdFields(clone.excludeCredentials)
  decodeIdFields(clone.allowCredentials)
  return clone
}

// serializeCredential 把认证器返回的 PublicKeyCredential 转成后端契约的 JSON。
function serializeCredential(cred) {
  const res = {}
  if (cred.response.clientDataJSON) res.clientDataJSON = bytesToBase64url(new Uint8Array(cred.response.clientDataJSON))
  if (cred.response.authenticatorData) res.authenticatorData = bytesToBase64url(new Uint8Array(cred.response.authenticatorData))
  if (cred.response.signature) res.signature = bytesToBase64url(new Uint8Array(cred.response.signature))
  if (cred.response.userHandle && cred.response.userHandle.byteLength > 0) {
    res.userHandle = bytesToBase64url(new Uint8Array(cred.response.userHandle))
  }
  if (cred.response.attestationObject) res.attestationObject = bytesToBase64url(new Uint8Array(cred.response.attestationObject))
  if (cred.response.transports) res.transports = cred.response.transports
  return {
    id: cred.id,
    rawId: bytesToBase64url(new Uint8Array(cred.rawId)),
    type: cred.type,
    clientExtensionResults: cred.getClientExtensionResults ? cred.getClientExtensionResults() : {},
    response: res,
  }
}

// normalizeOptions 兼容两种契约形态：
//   1. 标准 PublicKeyCredentialCreationOptionsJSON / RequestOptionsJSON（rp/user/challenge 在顶层）—— 直接透传；
//   2. go-webauthn 序列化风格 { "publicKey": {...} } —— 解包。
function normalizeOptions(options) {
  if (options && typeof options === 'object' && options.publicKey) return options.publicKey
  return options
}

// hasWebAuthn 判断当前环境是否具备 WebAuthn 能力（secure context 之外不可用）。
export function hasWebAuthn() {
  return typeof window !== 'undefined' &&
    typeof window.PublicKeyCredential === 'function' &&
    typeof navigator.credentials?.create === 'function' &&
    typeof navigator.credentials?.get === 'function'
}

// createCredential 执行注册（begin 选项 → 认证器 → 序列化响应）。
export async function createCredential(creationOptions) {
  if (!hasWebAuthn()) throw new Error('WebAuthn not available in this context')

  creationOptions = normalizeOptions(creationOptions)
  let publicKey
  if (typeof PublicKeyCredential.parseCreationOptionsFromJSON === 'function') {
    publicKey = PublicKeyCredential.parseCreationOptionsFromJSON(creationOptions)
  } else {
    publicKey = decodeOptions(creationOptions)
  }
  const cred = await navigator.credentials.create({ publicKey })
  return serializeCredential(cred)
}

// getCredential 执行登录（begin 断言选项 → 认证器 → 序列化响应）。
export async function getCredential(requestOptions) {
  if (!hasWebAuthn()) throw new Error('WebAuthn not available in this context')

  requestOptions = normalizeOptions(requestOptions)
  let publicKey
  if (typeof PublicKeyCredential.parseRequestOptionsFromJSON === 'function') {
    publicKey = PublicKeyCredential.parseRequestOptionsFromJSON(requestOptions)
  } else {
    publicKey = decodeOptions(requestOptions)
  }
  const cred = await navigator.credentials.get({ publicKey })
  return serializeCredential(cred)
}