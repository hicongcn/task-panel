import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const routes: RouteRecordRaw[] = [
  { path: '/login', name: 'login', component: () => import('@/views/Login.vue') },
  {
    path: '/',
    component: () => import('@/layouts/MainLayout.vue'),
    redirect: '/dashboard',
    children: [
      { path: 'dashboard', name: 'dashboard', component: () => import('@/views/Dashboard.vue'), meta: { title: '仪表板' } },
      { path: 'tasks', name: 'tasks', component: () => import('@/views/Tasks.vue'), meta: { title: '定时任务' } },
      { path: 'scripts', name: 'scripts', component: () => import('@/views/Scripts.vue'), meta: { title: '脚本管理' } },
      { path: 'envs', name: 'envs', component: () => import('@/views/Envs.vue'), meta: { title: '环境变量' } },
      { path: 'logs', name: 'logs', component: () => import('@/views/Logs.vue'), meta: { title: '执行日志' } },
      { path: 'notify', name: 'notify', component: () => import('@/views/NotifyChannels.vue'), meta: { title: '通知渠道' } },
      { path: 'backups', name: 'backups', component: () => import('@/views/Backups.vue'), meta: { title: '备份恢复' } },
      { path: 'deps', name: 'deps', component: () => import('@/views/Dependencies.vue'), meta: { title: '依赖管理' } },
      { path: 'openapi', name: 'openapi', component: () => import('@/views/OpenAPI.vue'), meta: { title: '开放接口' } },
      { path: 'audit', name: 'audit', component: () => import('@/views/AuditLogs.vue'), meta: { title: '审计日志' } },
      { path: 'settings', name: 'settings', component: () => import('@/views/Settings.vue'), meta: { title: '安全设置' } },
      { path: 'migrate', name: 'migrate', component: () => import('@/views/Migrate.vue'), meta: { title: '数据迁移' } },
    ],
  },
  { path: '/:pathMatch(.*)*', redirect: '/dashboard' },
]

const router = createRouter({ history: createWebHistory(), routes })

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  if (to.name === 'login') {
    return auth.isLoggedIn ? { name: 'dashboard' } : true
  }
  if (!auth.isLoggedIn) return { name: 'login' }
  return true
})

export default router
