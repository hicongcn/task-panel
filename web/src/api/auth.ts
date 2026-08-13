import request from './request'

export const authApi = {
  checkInit: () => request.get('/auth/check-init'),
  init: (username: string, password: string) =>
    request.post('/auth/init', { username, password }),
  login: (username: string, password: string) =>
    request.post('/auth/login', { username, password }),
  logout: () => request.post('/auth/logout'),
  user: () => request.get('/auth/user'),
}
