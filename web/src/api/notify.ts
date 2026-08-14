import request from './request'

export interface NotifyChannel {
  id: number
  name: string
  type: string
  enabled: boolean
  config: Record<string, any>
  created_at: string
}

export const notifyApi = {
  list: () => request.get('/notify-channels'),
  create: (data: any) => request.post('/notify-channels', data),
  update: (id: number, data: any) => request.put(`/notify-channels/${id}`, data),
  remove: (id: number) => request.delete(`/notify-channels/${id}`),
  toggle: (id: number, enabled: boolean) => request.put(`/notify-channels/${id}/toggle`, { enabled }),
  test: (type: string, config: any) => request.post('/notify-channels/test', { type, config }),
}

export function notifyTypeLabel(type: string): string {
  const map: Record<string, string> = {
    webhook: 'Webhook',
    telegram: 'Telegram',
    bark: 'Bark',
    email: '邮件',
  }
  return map[type] || type
}
