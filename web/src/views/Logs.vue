<template>
  <div class="logs-wrap">
    <div class="tree-pane">
      <div class="tree-head">
        <span>日志</span>
        <el-button size="small" @click="load">刷新</el-button>
      </div>
      <el-input v-model="keyword" placeholder="搜索脚本/日志" clearable size="small" class="tree-search" @input="onSearch" @clear="onSearch" />
      <el-tree ref="treeRef" :data="tree" node-key="key" :filter-node-method="filterNode" :props="{ label: 'title' }" highlight-current @node-click="onSelect">
        <template #default="{ data }">
          <span class="tree-node">
            <el-icon v-if="data.type === 'dir'" class="dir-icon"><Folder /></el-icon>
            <el-icon v-else class="file-icon"><Document /></el-icon>
            <span class="node-title">{{ data.title }}</span>
          </span>
        </template>
      </el-tree>
    </div>

    <div class="detail-pane">
      <div v-if="!current.id" class="empty">点击左侧日志查看内容</div>
      <template v-else>
        <div class="detail-head">
          <span class="path">{{ current.title }}</span>
          <div>
            <el-button size="small" @click="downloadRaw(current.row)">下载原文</el-button>
          </div>
        </div>
        <pre class="log-surface detail-body" v-html="current.html"></pre>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Folder, Document } from '@element-plus/icons-vue'
import { logApi, type TaskLog } from '@/api/log'
import { taskApi } from '@/api/task'
import { ansiToHtml } from '@/utils/ansi'

const tree = ref<any[]>([])
const treeRef = ref()
const keyword = ref('')
const loading = ref(false)
const current = ref({ id: 0, title: '', html: '', row: null as TaskLog | null })

// filterNode 树节点名称过滤(搜索)
function filterNode(value: string, data: any) {
  if (!value) return true
  return (data.title || '').toLowerCase().includes(value.toLowerCase())
}

function onSearch() {
  treeRef.value?.filter(keyword.value)
}

// load 拉取全部日志 + 任务命令,按脚本路径分组为树
async function load() {
  loading.value = true
  try {
    const logs = await fetchAllLogs()
    const commandMap: Record<number, string> = {}
    try {
      const t: any = await taskApi.list()
      for (const task of t.data.data || []) {
        commandMap[task.id] = (task.command || '').trim().split(/\s+/)[0] || task.name
      }
    } catch {}

    const groups = new Map<string, TaskLog[]>()
    for (const log of logs) {
      const script = commandMap[log.task_id] || log.task_name || '未知'
      if (!groups.has(script)) groups.set(script, [])
      groups.get(script)!.push(log)
    }
    tree.value = [...groups.entries()]
      .sort((a, b) => a[0].localeCompare(b[0]))
      .map(([name, list]) => ({
        key: 'dir:' + name,
        title: `${name} (${list.length})`,
        type: 'dir',
        children: list.map((l) => ({
          key: 'log:' + l.id,
          title: `${fmtTime(l.started_at)} ${statusText(l.status)}`,
          type: 'file',
          log: l,
        })),
      }))
  } catch {} finally { loading.value = false }
}

// fetchAllLogs 循环拉取全部日志(每页 100)
async function fetchAllLogs(): Promise<TaskLog[]> {
  const all: TaskLog[] = []
  let page = 1
  for (;;) {
    const res: any = await logApi.list(0, page, 100)
    const data = res.data.data || []
    all.push(...data)
    if (data.length < 100) break
    page++
  }
  return all
}

async function onSelect(node: any) {
  if (node.type !== 'file') return
  try {
    const res: any = await logApi.detail(node.log.id)
    current.value = {
      id: node.log.id,
      title: `${node.log.task_name} · ${fmtTime(node.log.started_at)}`,
      html: ansiToHtml(res.data.content || '(空)'),
      row: node.log,
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '获取日志失败')
  }
}

async function downloadRaw(row: TaskLog | null) {
  if (!row) return
  try {
    const res: any = await logApi.rawTicket(row.id)
    const a = document.createElement('a')
    a.href = res.data.url
    a.download = `log-${row.id}.log`
    a.click()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '下载失败')
  }
}

function fmtTime(t?: string) {
  if (!t) return '-'
  return t.replace('T', ' ').slice(0, 16)
}

function statusText(s: string) {
  return { success: '成功', failed: '失败', aborted: '终止', running: '运行中' }[s] || s
}

load()
</script>

<style scoped>
.logs-wrap { display: flex; height: calc(100vh - 90px); gap: 8px; }
.tree-pane { width: 300px; background: #fff; border-radius: 6px; padding: 8px; overflow: auto; }
.tree-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; font-weight: 600; }
.tree-search { margin-bottom: 8px; }
.detail-pane { flex: 1; background: #fff; border-radius: 6px; display: flex; flex-direction: column; overflow: hidden; }
.empty { flex: 1; display: flex; align-items: center; justify-content: center; color: #909399; }
.detail-head { display: flex; justify-content: space-between; align-items: center; padding: 8px 12px; border-bottom: 1px solid #ebeef5; }
.path { font-family: monospace; font-size: 13px; }
.detail-body { flex: 1; border: none; border-radius: 0; max-height: none; margin: 0; }
.tree-node { display: inline-flex; align-items: center; gap: 5px; }
.dir-icon { color: #e6a23c; }
.file-icon { color: #909399; }
.node-title { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>
