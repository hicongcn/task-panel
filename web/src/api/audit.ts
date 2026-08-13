import request from './request'

export interface AuditLog {
  id: number
  username: string
  action: string
  resource: string
  detail: string
  ip: string
  created_at: string
}

export const auditApi = {
  list: (username = '', action = '', page = 1, pageSize = 20) =>
    request.get('/audit-logs', { params: { username, action, page, page_size: pageSize } }),
}

// actionLabel 把审计动作枚举转成中文,便于表格展示。
export function actionLabel(action: string): string {
  const map: Record<string, string> = {
    init_admin: '初始化管理员',
    login_success: '登录成功',
    login_failed: '登录失败',
    logout: '登出',
    task_create: '创建任务',
    task_update: '更新任务',
    task_delete: '删除任务',
    task_run: '运行任务',
    task_stop: '停止任务',
    task_enable: '启用任务',
    task_disable: '禁用任务',
    script_save: '保存脚本',
    script_create_dir: '创建目录',
    script_delete: '删除脚本',
    script_rename: '重命名脚本',
    script_upload: '上传脚本',
    script_run: '运行脚本',
    script_run_code: '运行内联代码',
    env_create: '创建变量',
    env_update: '更新变量',
    env_delete: '删除变量',
    env_batch_delete: '批量删除变量',
  }
  return map[action] || action
}
