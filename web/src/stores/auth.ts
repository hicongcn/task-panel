import { defineStore } from 'pinia'
import { authApi } from '@/api/auth'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: localStorage.getItem('token') || '',
    username: localStorage.getItem('username') || '',
    needInit: false,
  }),
  getters: {
    isLoggedIn: (s) => !!s.token,
  },
  actions: {
    setToken(token: string, username: string) {
      this.token = token
      this.username = username
      localStorage.setItem('token', token)
      localStorage.setItem('username', username)
    },
    async checkInit() {
      const res: any = await authApi.checkInit()
      this.needInit = res.data.need_init
      return this.needInit
    },
    async init(username: string, password: string) {
      const res: any = await authApi.init(username, password)
      return res
    },
    async login(username: string, password: string, totpCode = '') {
      const res: any = await authApi.login(username, password, totpCode)
      this.setToken(res.data.access_token, res.data.username)
      return res
    },
    async logout() {
      try { await authApi.logout() } catch {}
      this.token = ''
      this.username = ''
      localStorage.removeItem('token')
      localStorage.removeItem('username')
    },
  },
})
