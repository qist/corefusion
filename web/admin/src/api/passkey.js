import client from './client'

// Passkey（WebAuthn 通行密钥）API。
// 约定：所有 Begin 接口返回 `{ session_token, options }`；`options` 是标准的
// PublicKeyCredentialCreationOptionsJSON / PublicKeyCredentialRequestOptionsJSON，
// 可直接交给 executor 里的 parse*FromJSON 或手工转换。

// 当前访问地址是否可用 Passkey（命中白名单 + 安全上下文）
export async function getPasskeyStatus() {
  return await client.get('/auth/passkey/status')
}

// 登录 begin（公开，discoverable 无用户名流程）
export async function passkeyLoginBegin() {
  return await client.post('/auth/passkey/login/begin', {})
}

// 登录 finish（公开）→ 返回 { access_token, refresh_token, user }
export async function passkeyLoginFinish(sessionToken, response) {
  return await client.post('/auth/passkey/login/finish', {
    session_token: sessionToken,
    response,
  })
}

// 注册 begin（需已登录）
export async function passkeyRegisterBegin(name) {
  return await client.post('/auth/passkey/register/begin', { name: name || '' })
}

// 注册 finish（需已登录）→ 返回 { credential, credentials }
export async function passkeyRegisterFinish(sessionToken, response, name) {
  return await client.post('/auth/passkey/register/finish', {
    session_token: sessionToken,
    response,
    name: name || '',
  })
}

// 我的凭证列表（需已登录）
export async function listPasskeys() {
  return await client.get('/auth/passkey/credentials')
}

// 删除一把凭证（需已登录）
export async function deletePasskey(id) {
  return await client.delete(`/auth/passkey/credentials/${id}`)
}

// 白名单管理（管理员）
export async function listRPIDs() {
  return await client.get('/auth/passkey/rpids')
}

export async function updateRPID(action, rpid) {
  return await client.post('/auth/passkey/rpids', { action, rpid })
}