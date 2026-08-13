import request from './request'

export interface Task {
  id: number
  name: string
  command: string
  cron_expression: string
  enabled: boolean
  timeout_seconds: number
  max_retries: number
  retry_interval: number
  status: string
  last_run_at: string | null
  last_run_status: string
  last_run_duration: number
  created_at: string
  updated_at: string
}

export const taskApi = {
  list: (keyword = '', status = '') =>
    request.get('/tasks', { params: { keyword, status } }),
  get: (id: number) => request.get(`/tasks/${id}`),
  create: (data: Partial<Task>) => request.post('/tasks', data),
  update: (id: number, data: Partial<Task>) => request.put(`/tasks/${id}`, data),
  remove: (id: number) => request.delete(`/tasks/${id}`),
  enable: (id: number) => request.put(`/tasks/${id}/enable`),
  disable: (id: number) => request.put(`/tasks/${id}/disable`),
  run: (id: number) => request.put(`/tasks/${id}/run`),
  stop: (id: number) => request.put(`/tasks/${id}/stop`),
  cronDescribe: (expression: string) =>
    request.post('/tasks/cron-describe', { expression }),
}
