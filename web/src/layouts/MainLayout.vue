<template>
  <el-container class="layout">
    <el-aside width="212px" class="aside">
      <div class="logo">
        <div class="logo-icon">{{ panel.logo || '⏰' }}</div>
        <div class="logo-text">
          <span class="brand">{{ panel.title || 'Task Panel' }}</span>
          <span class="ver">v1.2</span>
        </div>
      </div>
      <el-menu :default-active="route.path" router class="menu">
        <el-menu-item index="/dashboard"><el-icon><Monitor /></el-icon><span>仪表板</span></el-menu-item>
        <el-menu-item index="/tasks"><el-icon><Timer /></el-icon><span>定时任务</span></el-menu-item>
        <el-menu-item index="/envs"><el-icon><Key /></el-icon><span>环境变量</span></el-menu-item>
        <el-menu-item index="/scripts"><el-icon><Document /></el-icon><span>脚本管理</span></el-menu-item>
        <el-menu-item index="/logs"><el-icon><List /></el-icon><span>执行日志</span></el-menu-item>
        <el-menu-item index="/deps"><el-icon><Box /></el-icon><span>依赖管理</span></el-menu-item>
        <el-menu-item index="/openapi"><el-icon><Connection /></el-icon><span>开放接口</span></el-menu-item>
        <el-menu-item index="/system"><el-icon><Setting /></el-icon><span>系统管理</span></el-menu-item>
      </el-menu>
      <div class="aside-foot">Task Panel · 自研任务面板</div>
    </el-aside>

    <el-container class="right">
      <el-header class="header">
        <div class="crumb">
          <span class="title">{{ route.meta.title }}</span>
        </div>
        <el-dropdown trigger="click" @command="onCommand">
          <div class="user">
            <span class="avatar">{{ (auth.username || 'U').slice(0, 1).toUpperCase() }}</span>
            <span class="uname">{{ auth.username }}</span>
            <el-icon class="arrow"><ArrowDown /></el-icon>
          </div>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="logout"><el-icon><SwitchButton /></el-icon>退出登录</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </el-header>
      <el-main class="main"><router-view /></el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { reactive, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { systemApi } from '@/api/system'
import { Monitor, Timer, Document, Key, List, Box, Connection, Setting, ArrowDown, SwitchButton } from '@element-plus/icons-vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const panel = reactive({ title: '', logo: '' })

onMounted(async () => {
  try {
    const res: any = await systemApi.panelInfo()
    panel.title = res.data.data.panel_title || ''
    panel.logo = res.data.data.panel_logo || ''
  } catch {}
})

async function onCommand(cmd: string) {
  if (cmd === 'logout') {
    await auth.logout()
    router.push('/login')
  }
}
</script>

<style scoped>
.layout { height: 100vh; }

/* ---- 侧边栏 ---- */
.aside {
  background: linear-gradient(180deg, #1d2a3a 0%, #15202e 100%);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.logo {
  display: flex; align-items: center; gap: 10px;
  height: 62px; padding: 0 18px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  flex-shrink: 0;
}
.logo-icon {
  width: 34px; height: 34px; border-radius: 9px;
  background: linear-gradient(135deg, #3b82f6, #2563eb);
  display: flex; align-items: center; justify-content: center;
  color: #fff; font-size: 18px; box-shadow: 0 2px 8px rgba(59, 130, 246, 0.35);
}
.logo-text { display: flex; flex-direction: column; line-height: 1.2; }
.brand { color: #fff; font-size: 15px; font-weight: 700; letter-spacing: 0.3px; }
.ver { color: rgba(255, 255, 255, 0.4); font-size: 11px; }

.menu {
  flex: 1; border-right: none;
  background: transparent;
  padding: 10px 8px;
  overflow-y: auto;
}
.menu :deep(.el-menu-item) {
  height: 42px; line-height: 42px;
  margin: 3px 0; border-radius: 8px;
  color: rgba(255, 255, 255, 0.72);
  font-size: 13.5px;
}
.menu :deep(.el-menu-item .el-icon) { color: rgba(255, 255, 255, 0.55); }
.menu :deep(.el-menu-item:hover) {
  background: rgba(255, 255, 255, 0.07);
  color: #fff;
}
.menu :deep(.el-menu-item.is-active) {
  background: linear-gradient(135deg, #3b82f6, #2563eb);
  color: #fff;
  box-shadow: 0 3px 10px rgba(37, 99, 235, 0.35);
}
.menu :deep(.el-menu-item.is-active .el-icon) { color: #fff; }

.aside-foot {
  padding: 14px; text-align: center;
  color: rgba(255, 255, 255, 0.28); font-size: 11px;
  border-top: 1px solid rgba(255, 255, 255, 0.05);
  flex-shrink: 0;
}

/* ---- 右侧 ---- */
.right { background: #f0f2f5; }
.header {
  height: 60px; display: flex; align-items: center; justify-content: space-between;
  background: #fff; padding: 0 22px;
  box-shadow: 0 1px 4px rgba(0, 21, 41, 0.08);
  z-index: 5;
}
.title { font-size: 16px; font-weight: 600; color: #1f2d3d; }
.user { display: flex; align-items: center; gap: 8px; cursor: pointer; padding: 4px 6px; border-radius: 8px; }
.user:hover { background: #f5f7fa; }
.avatar {
  width: 32px; height: 32px; border-radius: 50%;
  background: linear-gradient(135deg, #3b82f6, #2563eb);
  color: #fff; display: flex; align-items: center; justify-content: center;
  font-size: 14px; font-weight: 600;
}
.uname { font-size: 13.5px; color: #303133; }
.arrow { color: #909399; font-size: 12px; }

.main { padding: 18px; overflow-y: auto; }
</style>
