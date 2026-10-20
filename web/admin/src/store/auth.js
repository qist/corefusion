import { defineStore } from 'pinia'
import client from '../api/client'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    user: null,
    accessToken: localStorage.getItem('access_token') || '',
    refreshToken: localStorage.getItem('refresh_token') || '',
  }),

  getters: {
    isLoggedIn: (state) => !!state.accessToken,
    username: (state) => state.user?.username || '',
    role: (state) => state.user?.role || '',
  },

  actions: {
    // setTokens 保存登录凭证（密码 / Passkey / 其它入口通用）。
    setTokens(res) {
      this.accessToken = res.access_token
      this.refreshToken = res.refresh_token
      this.user = res.user
      localStorage.setItem('access_token', res.access_token)
      localStorage.setItem('refresh_token', res.refresh_token)
      return res
    },

    // login 一步完成：后端按用户状态自动识别凭据类型——
    // 已启用 2FA → code 为 TOTP 动态码/恢复码；未启用 → code 为静态密码。
    async login(username, credential) {
      const res = await client.post('/auth/login', { username, code: credential })
      return this.setTokens(res)
    },

    async fetchMe() {
      const res = await client.get('/auth/me')
      this.user = res
      return res
    },

    // logout 先让**服务端**吊销令牌（令牌版本 +1），再清本地。
    //
    // 只清 localStorage 是安全漏洞（v5 M8）：服务端仍认那个 token，
    // 被窃取后有效期内照样可用。这里用裸 fetch 而不是 client —— 避免
    // 401 拦截器在"令牌本就过期"时触发续期/跳转的副作用；请求失败也
    // 必须继续清本地，否则用户会卡在登不出去的状态。
    async logout() {
      const tk = this.accessToken
      if (tk) {
        try {
          await fetch('/api/v1/auth/logout', {
            method: 'POST',
            headers: { Authorization: `Bearer ${tk}` },
          })
        } catch (e) {
          // 网络异常不阻塞登出：本地清理优先。
        }
      }
      this.user = null
      this.accessToken = ''
      this.refreshToken = ''
      localStorage.removeItem('access_token')
      localStorage.removeItem('refresh_token')
    },

    async changePassword(oldPassword, newPassword) {
      return await client.post('/auth/change-password', {
        old_password: oldPassword,
        new_password: newPassword,
      })
    },
  },
})
