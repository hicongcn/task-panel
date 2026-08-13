<template>
  <div class="dashboard">
    <el-row :gutter="16">
      <el-col :span="6"><el-card><div class="metric"><div class="num">{{ stats.tasks }}</div><div class="label">任务总数</div></div></el-card></el-col>
      <el-col :span="6"><el-card><div class="metric"><div class="num">{{ stats.enabled }}</div><div class="label">已启用</div></div></el-card></el-col>
      <el-col :span="6"><el-card><div class="metric"><div class="num">{{ stats.running }}</div><div class="label">运行中</div></div></el-card></el-col>
      <el-col :span="6"><el-card><div class="metric"><div class="num">{{ stats.envs }}</div><div class="label">环境变量</div></div></el-card></el-col>
    </el-row>
    <el-card class="panel">
      <template #header><span>最近日志</span></template>
      <el-table :data="recent" border size="small">
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column prop="task_name" label="任务" min-width="120" />
        <el-table-column label="状态" width="90">
          <template #default="{ row }"><el-tag :type="statusType(row.status)" size="small">{{ statusText(row.status) }}</el-tag></template>
        </el-table-column>
        <el-table-column prop="started_at" label="时间" min-width="160" />
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { taskApi } from '@/api/task'
import { envApi } from '@/api/env'
import { logApi, type TaskLog } from '@/api/log'

const stats = reactive({ tasks: 0, enabled: 0, running: 0, envs: 0 })
const recent = ref<TaskLog[]>([])

async function load() {
  try {
    const [t, e, l]: any[] = await Promise.all([taskApi.list(), envApi.list(), logApi.list(0, 1, 10)])
    const tasks = t.data.data || []
    stats.tasks = tasks.length
    stats.enabled = tasks.filter((x: any) => x.enabled).length
    stats.running = tasks.filter((x: any) => x.status === 'running').length
    stats.envs = (e.data.data || []).length
    recent.value = l.data.data || []
  } catch {}
}
onMounted(load)

function statusType(s: string) {
  return { success: 'success', failed: 'danger', aborted: 'warning', running: 'primary' }[s] || 'info'
}
function statusText(s: string) {
  return { success: '成功', failed: '失败', aborted: '终止', running: '运行中' }[s] || s
}
</script>

<style scoped>
.dashboard { display: flex; flex-direction: column; gap: 16px; }
.metric { text-align: center; padding: 8px 0; }
.num { font-size: 32px; font-weight: 700; color: #409eff; }
.label { color: #909399; font-size: 13px; margin-top: 4px; }
</style>
