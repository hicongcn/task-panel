import request from './request'

export interface OpenApp {
  id: number
  name: string
  client_id: string
  enabled: boolean
  created_at: string
}

export const scopeLabels: Record<string, string> = {
  'tasks:read': '查看任务',
  'tasks:run': '触发任务运行',
  'logs:read': '查看执行日志',
  'envs:read': '查看环境变量',
}

export const openApi = {
  list: () => request.get('/open/apps'),
  create: (name: string, scopes: string[]) => request.post('/open/apps', { name, scopes }),
  update: (id: number, data: any) => request.put(`/open/apps/${id}`, data),
  remove: (id: number) => request.delete(`/open/apps/${id}`),
  resetSecret: (id: number) => request.put(`/open/apps/${id}/reset-secret`),
  authToken: (clientId: string, clientSecret: string) =>
    request.post('/open/auth/token', { client_id: clientId, client_secret: clientSecret }),
}
