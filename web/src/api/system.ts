import request from './request'

export interface SysStats {
  cpu_percent: number
  mem_total: number
  mem_used: number
  mem_percent: number
  disk_total: number
  disk_used: number
  disk_percent: number
  load1: number
  load5: number
  load15: number
  uptime_seconds: number
  hostname: string
  platform: string
}

export const systemApi = {
  stats: () => request.get('/system/stats'),
  panelInfo: () => request.get('/system/panel'),
  getConfig: () => request.get('/system/config'),
  updateConfig: (data: any) => request.put('/system/config', data),
}

export function formatBytes(bytes: number): string {
  if (!bytes) return '0 B'
  const gb = bytes / 1024 / 1024 / 1024
  if (gb >= 1) return gb.toFixed(2) + ' GB'
  const mb = bytes / 1024 / 1024
  if (mb >= 1) return mb.toFixed(1) + ' MB'
  return Math.round(bytes / 1024) + ' KB'
}

export function formatUptime(seconds: number): string {
  if (!seconds) return '-'
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (d > 0) return `${d}天 ${h}小时`
  if (h > 0) return `${h}小时 ${m}分`
  return `${m}分钟`
}
