<template>
  <div class="dashboard">
    <el-row :gutter="16">
      <el-col :span="6"><el-card><div class="metric"><div class="num">{{ stats.tasks }}</div><div class="label">任务总数</div></div></el-card></el-col>
      <el-col :span="6"><el-card><div class="metric"><div class="num">{{ stats.enabled }}</div><div class="label">已启用</div></div></el-card></el-col>
      <el-col :span="6"><el-card><div class="metric"><div class="num">{{ stats.running }}</div><div class="label">运行中</div></div></el-card></el-col>
      <el-col :span="6"><el-card><div class="metric"><div class="num">{{ stats.envs }}</div><div class="label">环境变量</div></div></el-card></el-col>
    </el-row>

    <el-card class="panel">
      <template #header><span>系统监控</span></template>
      <el-row :gutter="16">
        <el-col :span="8">
          <div class="gauge">
            <el-progress type="dashboard" :percentage="Math.round(sys.cpu_percent || 0)" :color="gaugeColor" :width="120">
              <template #default><span class="gauge-num">{{ (sys.cpu_percent || 0).toFixed(1) }}%</span></template>
            </el-progress>
            <div class="gauge-label">CPU 使用率</div>
          </div>
        </el-col>
        <el-col :span="8">
          <div class="gauge">
            <el-progress type="dashboard" :percentage="Math.round(sys.mem_percent || 0)" :color="gaugeColor" :width="120">
              <template #default><span class="gauge-num">{{ (sys.mem_percent || 0).toFixed(1) }}%</span></template>
            </el-progress>
            <div class="gauge-label">内存 {{ formatBytes(sys.mem_used) }} / {{ formatBytes(sys.mem_total) }}</div>
          </div>
        </el-col>
        <el-col :span="8">
          <div class="gauge">
            <el-progress type="dashboard" :percentage="Math.round(sys.disk_percent || 0)" :color="gaugeColor" :width="120">
              <template #default><span class="gauge-num">{{ (sys.disk_percent || 0).toFixed(1) }}%</span></template>
            </el-progress>
            <div class="gauge-label">磁盘 {{ formatBytes(sys.disk_used) }} / {{ formatBytes(sys.disk_total) }}</div>
          </div>
        </el-col>
      </el-row>
      <div class="sys-meta">
        主机 {{ sys.hostname || '-' }}({{ sys.platform || '-' }})· 运行 {{ formatUptime(sys.uptime_seconds) }}· 负载 {{ fmtLoad(sys.load1) }} / {{ fmtLoad(sys.load5) }} / {{ fmtLoad(sys.load15) }}
      </div>
    </el-card>

    <el-card class="panel">
      <template #header><span>最近日志</span></template>
      <el-table :data="recent" border size="small">
        <el-table-column prop="id" label="ID" />
        <el-table-column prop="task_name" label="任务" show-overflow-tooltip />
        <el-table-column label="状态">
          <template #default="{ row }"><el-tag :type="statusType(row.status)" size="small">{{ statusText(row.status) }}</el-tag></template>
        </el-table-column>
        <el-table-column label="时间" show-overflow-tooltip>
        <template #default="{ row }">{{ fmtTime(row.started_at) }}</template>
      </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, onBeforeUnmount } from 'vue'
import { fmtTime } from '@/utils/time'
import { taskApi } from '@/api/task'
import { envApi } from '@/api/env'
import { logApi, type TaskLog } from '@/api/log'
import { systemApi, formatBytes, formatUptime } from '@/api/system'

const stats = reactive({ tasks: 0, enabled: 0, running: 0, envs: 0 })
const recent = ref<TaskLog[]>([])
const sys = reactive({
  cpu_percent: 0, mem_total: 0, mem_used: 0, mem_percent: 0,
  disk_total: 0, disk_used: 0, disk_percent: 0,
  load1: 0, load5: 0, load15: 0, uptime_seconds: 0, hostname: '', platform: '',
})

function gaugeColor(p: number) {
  return p > 90 ? '#f56c6c' : p > 70 ? '#e6a23c' : '#409eff'
}

function fmtLoad(v: number) {
  return v ? v.toFixed(2) : '0.00'
}

let sysTimer: number | undefined
let statTimer: number | undefined

async function loadStats() {
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

async function loadSys() {
  try {
    const res: any = await systemApi.stats()
    Object.assign(sys, res.data.data)
  } catch {}
}

onMounted(() => {
  loadStats()
  loadSys()
  sysTimer = window.setInterval(loadSys, 3000)
  statTimer = window.setInterval(loadStats, 30000)
})

onBeforeUnmount(() => {
  if (sysTimer) window.clearInterval(sysTimer)
  if (statTimer) window.clearInterval(statTimer)
})

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
.gauge { text-align: center; padding: 4px 0; }
.gauge-num { font-size: 20px; font-weight: 700; color: #303133; }
.gauge-label { margin-top: 8px; color: #606266; font-size: 13px; }
.sys-meta { margin-top: 14px; text-align: center; color: #909399; font-size: 12px; }
</style>
