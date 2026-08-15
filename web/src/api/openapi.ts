import request from './request'

export interface OpenApp {
  id: number
  name: string
  client_id: string
  enabled: boolean
  created_at: string
}

export const scopeLabels: Record<string, string> = {
  'crontab:read': '定时任务读取',
  'crontab:write': '定时任务写入(增删改/运行/停止)',
  'env:read': '环境变量读取',
  'env:write': '环境变量写入',
  'log:read': '日志读取',
  'system:read': '系统信息读取',
  'config:read': '配置读取',
  'config:write': '配置写入',
  'script:read': '脚本读取',
  'script:write': '脚本写入',
  'dependence:read': '依赖读取',
  'dependence:write': '依赖写入',
  'subscription:read': '订阅读取',
  'subscription:write': '订阅写入',
}

export const openApi = {
  // 青龙规范 /open/app:列表/创建
  list: () => request.get('/open/app'),
  create: (name: string, scopes: string[]) => request.post('/open/app', { name, scopes }),
  // 更新:青龙 PUT /open/app,id 在 body
  update: (id: number, data: any) => request.put('/open/app', { id, ...data }),
  // 删除:青龙 DELETE /open/app,body 为 ID 数组
  remove: (id: number) => request.delete('/open/app', { data: [id] }),
  resetSecret: (id: number) => request.put(`/open/app/${id}/reset-secret`),
  // 换 token:青龙规范 GET /open/auth/token?client_id&client_secret
  authToken: (clientId: string, clientSecret: string) =>
    request.get('/open/auth/token', { params: { client_id: clientId, client_secret: clientSecret } }),
}
