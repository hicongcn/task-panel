<template>
  <el-container class="layout">
    <el-aside width="210px" class="aside">
      <div class="logo">Task Panel</div>
      <el-menu :default-active="route.path" router class="menu">
        <el-menu-item index="/dashboard"><el-icon><Monitor /></el-icon><span>仪表板</span></el-menu-item>
        <el-menu-item index="/tasks"><el-icon><Timer /></el-icon><span>定时任务</span></el-menu-item>
        <el-menu-item index="/scripts"><el-icon><Document /></el-icon><span>脚本管理</span></el-menu-item>
        <el-menu-item index="/envs"><el-icon><Key /></el-icon><span>环境变量</span></el-menu-item>
        <el-menu-item index="/logs"><el-icon><List /></el-icon><span>执行日志</span></el-menu-item>
        <el-menu-item index="/notify"><el-icon><Bell /></el-icon><span>通知渠道</span></el-menu-item>
        <el-menu-item index="/backups"><el-icon><FolderOpened /></el-icon><span>备份恢复</span></el-menu-item>
        <el-menu-item index="/deps"><el-icon><Box /></el-icon><span>依赖管理</span></el-menu-item>
        <el-menu-item index="/openapi"><el-icon><Connection /></el-icon><span>开放接口</span></el-menu-item>
        <el-menu-item index="/audit"><el-icon><Tickets /></el-icon><span>审计日志</span></el-menu-item>
        <el-menu-item index="/settings"><el-icon><Lock /></el-icon><span>安全设置</span></el-menu-item>
        <el-menu-item index="/migrate"><el-icon><Switch /></el-icon><span>数据迁移</span></el-menu-item>
      </el-menu>
    </el-aside>
    <el-container>
      <el-header class="header">
        <span class="title">{{ route.meta.title }}</span>
        <el-dropdown @command="onCommand">
          <span class="user">{{ auth.username }} <el-icon><ArrowDown /></el-icon></span>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="logout">退出登录</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </el-header>
      <el-main><router-view /></el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { Monitor, Timer, Document, Key, List, Tickets, Bell, FolderOpened, Box, Connection, Lock, Switch, ArrowDown } from '@element-plus/icons-vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

async function onCommand(cmd: string) {
  if (cmd === 'logout') {
    await auth.logout()
    router.push('/login')
  }
}
</script>

<style scoped>
.layout { height: 100vh; }
.aside { background: #1f2d3d; }
.logo { height: 56px; line-height: 56px; text-align: center; color: #fff; font-size: 18px; font-weight: 600; }
.menu { border-right: none; background: #1f2d3d; }
:deep(.el-menu) { background: #1f2d3d; }
:deep(.el-menu-item) { color: #bfcbd9; }
:deep(.el-menu-item.is-active) { color: #fff; background: #001528; }
.header { display: flex; align-items: center; justify-content: space-between; background: #fff; border-bottom: 1px solid #ebeef5; }
.title { font-size: 16px; font-weight: 600; }
.user { cursor: pointer; display: inline-flex; align-items: center; gap: 4px; }
</style>
