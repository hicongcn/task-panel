import request from './request'

export interface EnvVar {
  id: number
  name: string
  value_masked?: string
  value?: string
  group: string
  remark: string
  enabled: boolean
  sort_order: number
  created_at: string
}

export const envApi = {
  list: (keyword = '', group = '') =>
    request.get('/envs', { params: { keyword, group } }),
  groups: () => request.get('/envs/groups'),
  create: (data: Partial<EnvVar> & { value: string }) =>
    request.post('/envs', data),
  update: (id: number, data: Partial<EnvVar> & { value?: string }) =>
    request.put(`/envs/${id}`, data),
  remove: (id: number) => request.delete(`/envs/${id}`),
  batch: (action: 'enable' | 'disable', ids: number[]) =>
    request.post(`/envs/batch/${action}`, { ids }),
  batchDelete: (ids: number[]) => request.delete('/envs/batch', { data: { ids } }),
  reorder: (ids: number[]) => request.put('/envs/reorder', { ids }),
}
