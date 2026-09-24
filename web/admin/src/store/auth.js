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

    async login(username, password) {
      const res = await client.post('/auth/login', { username, password })
      return this.setTokens(res)
    },

    async fetchMe() {
      const res = await client.get('/auth/me')
      this.user = res
      return res
    },

    logout() {
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
