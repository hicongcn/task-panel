import request from './request'

export const authApi = {
  checkInit: () => request.get('/auth/check-init'),
  init: (username: string, password: string) =>
    request.post('/auth/init', { username, password }),
  login: (username: string, password: string, totpCode = '') =>
    request.post('/auth/login', { username, password, totp_code: totpCode }),
  logout: () => request.post('/auth/logout'),
  user: () => request.get('/auth/user'),
  totpStatus: () => request.get('/auth/totp/status'),
  totpSetup: () => request.get('/auth/totp/setup'),
  totpEnable: (secret: string, code: string) =>
    request.post('/auth/totp/enable', { secret, code }),
  totpDisable: (password: string) =>
    request.post('/auth/totp/disable', { password }),
  changePassword: (oldPassword: string, newPassword: string) =>
    request.post('/auth/password', { old_password: oldPassword, new_password: newPassword }),
}
