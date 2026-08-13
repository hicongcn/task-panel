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
