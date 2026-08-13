<template>
  <div>
    <div class="toolbar">
      <el-input-number v-model="taskID" :min="0" placeholder="任务ID(0=全部)" style="width:160px" />
      <el-button @click="load">查询</el-button>
    </div>
    <el-table :data="logs" v-loading="loading" border stripe>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="task_name" label="任务" min-width="140" />
      <el-table-column label="状态" width="90">
        <template #default="{ row }"><el-tag :type="statusType(row.status)" size="small">{{ statusText(row.status) }}</el-tag></template>
      </el-table-column>
      <el-table-column prop="started_at" label="开始时间" width="180" />
      <el-table-column label="耗时" width="100">
        <template #default="{ row }">{{ row.duration ? row.duration.toFixed(2) + 's' : '—' }}</template>
      </el-table-column>
      <el-table-column label="操作" width="180" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="openDetail(row)">详情</el-button>
          <el-button size="small" @click="downloadRaw(row)">下载原文</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-drawer v-model="detailDrawer.visible" title="日志详情" size="60%">
      <pre class="log-surface" v-html="detailHtml"></pre>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { logApi, type TaskLog } from '@/api/log'
import { ansiToHtml } from '@/utils/ansi'

const logs = ref<TaskLog[]>([])
const loading = ref(false)
const taskID = ref(0)

const detailDrawer = ref({ visible: false, content: '' })
const detailHtml = computed(() => ansiToHtml(detailDrawer.value.content))

async function load() {
  loading.value = true
  try {
    const res: any = await logApi.list(taskID.value)
    logs.value = res.data.data || []
  } catch {} finally { loading.value = false }
}
load()

async function openDetail(row: TaskLog) {
  const res: any = await logApi.detail(row.id)
  detailDrawer.value.content = res.data.content
  detailDrawer.value.visible = true
}

async function downloadRaw(row: TaskLog) {
  try {
    const res: any = await logApi.rawTicket(row.id)
    const a = document.createElement('a')
    a.href = res.data.url
    a.download = `log-${row.id}.log`
    a.click()
  } catch {}
}

function statusType(s: string) {
  return { success: 'success', failed: 'danger', aborted: 'warning', running: 'primary' }[s] || 'info'
}
function statusText(s: string) {
  return { success: '成功', failed: '失败', aborted: '终止', running: '运行中' }[s] || s
}
</script>

<style scoped>
.toolbar { display: flex; gap: 8px; margin-bottom: 12px; }
</style>
