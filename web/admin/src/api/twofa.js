import client from './client'

// 2FA（TOTP 双步验证）API。
//
// 登录两步走：
//   1. POST /auth/login 密码验证通过且用户已启用 2FA → 返回 { requires_2fa, login_token }；
//   2. POST /auth/login/2fa 提交动态码/恢复码换取正式 token。
// 管理接口（需已登录）：setup → enable → 展示恢复码；disable / 重新生成需再次验证动态码。

// 查询指定用户名是否已启用 2FA（公开）。
// 登录页据此把输入框切换为"密码"或"动态码/恢复码"模式。
// 用户不存在/异常统一返回 { enabled: false }，不泄露账号存在性。
export async function twofaStatusByUsername(username) {
  return await client.get('/auth/login/2fa-status', {
    params: { username },
    suppressErrorMessage: true,
  })
}

// 提交 2FA 动态码/恢复码完成第二步登录（公开，兼容旧两步流程）。
// 401 是预期业务结果（动态码错 / 票据过期），由 Login.vue 自行提示。
export async function twofaLoginFinish(loginToken, code) {
  return await client.post(
    '/auth/login/2fa',
    { login_token: loginToken, code },
    { suppressErrorMessage: true }
  )
}

// 查询自己的 2FA 状态（已登录）
export async function twofaStatus() {
  return await client.get('/auth/2fa/status')
}

// 开始绑定：输入当前密码确认身份，返回 { secret, otpauth_url }（已登录）
export async function twofaSetup(password) {
  return await client.post('/auth/2fa/setup', { password })
}

// 验证动态码并启用 2FA，返回 { recovery_codes }（一次性展示）（已登录）
export async function twofaEnable(code) {
  return await client.post('/auth/2fa/enable', { code })
}

// 关闭 2FA（需再次验证动态码）（已登录）
export async function twofaDisable(code) {
  return await client.post('/auth/2fa/disable', { code })
}

// 重新生成恢复码（旧码作废；需再次验证动态码）（已登录）
export async function twofaRegenerateRecovery(code) {
  return await client.post('/auth/2fa/recovery-codes', { code })
}