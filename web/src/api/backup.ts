import request from './request'

export interface BackupInfo {
  name: string
  size: number
  created_at: string
}

export const backupApi = {
  create: () => request.post('/backups'),
  list: () => request.get('/backups'),
  download: (name: string) =>
    request.get(`/backups/${encodeURIComponent(name)}/download`, { responseType: 'blob' }),
  remove: (name: string) => request.delete(`/backups/${encodeURIComponent(name)}`),
  restoreByName: (name: string) => request.post(`/backups/${encodeURIComponent(name)}/restore`),
  restoreUpload: (file: File) => {
    const fd = new FormData()
    fd.append('file', file)
    return request.post('/backups/restore', fd, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 120000,
    })
  },
  getSettings: () => request.get('/backups/settings'),
  updateSettings: (data: any) => request.put('/backups/settings', data),
}

export function formatSize(bytes: number): string {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / 1024 / 1024).toFixed(2) + ' MB'
}
