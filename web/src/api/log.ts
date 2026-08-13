import request from './request'

export interface TaskLog {
  id: number
  task_id: number
  task_name: string
  status: string
  started_at: string
  ended_at: string | null
  duration: number
}

export const logApi = {
  list: (taskID = 0, page = 1, pageSize = 20) =>
    request.get('/logs', { params: { task_id: taskID, page, page_size: pageSize } }),
  detail: (id: number) => request.get(`/logs/${id}`),
  latest: (taskID: number) => request.get(`/tasks/${taskID}/latest-log`),
  rawTicket: (id: number) => request.get(`/logs/${id}/raw-ticket`),
  liveTicket: (taskID: number) => request.get(`/tasks/${taskID}/live-ticket`),
}

// liveLogURL 拼接 SSE 实时日志地址(EventSource 无法带 Authorization,改用短期票据)。
export function liveLogURL(taskID: number, ticket: string): string {
  return `/api/v1/tasks/${taskID}/live-logs?ticket=${encodeURIComponent(ticket)}`
}
