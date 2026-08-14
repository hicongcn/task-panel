<template>
  <div>
    <div class="toolbar">
      <el-input v-model="keyword" placeholder="搜索任务名" clearable style="width:200px" @keyup.enter="load" @clear="load" />
      <el-select v-model="tag" placeholder="按标签筛选" clearable style="width:160px" @change="load">
        <el-option v-for="(count, t) in tagStats" :key="t" :label="`${t} (${count})`" :value="t" />
      </el-select>
      <el-button @click="load">刷新</el-button>
      <el-button type="primary" @click="openCreate">新建任务</el-button>

      <el-dropdown v-if="selected.length" trigger="click" style="margin-left:8px">
        <el-button type="warning">批量操作({{ selected.length }})<el-icon><ArrowDown /></el-icon></el-button>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item @click="batch('enable')">批量启用</el-dropdown-item>
            <el-dropdown-item @click="batch('disable')">批量禁用</el-dropdown-item>
            <el-dropdown-item @click="batch('run')">批量运行</el-dropdown-item>
            <el-dropdown-item divided @click="batch('delete')">批量删除</el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
    </div>

    <el-table :data="tasks" v-loading="loading" border stripe @selection-change="onSelection">
      <el-table-column type="selection" width="45" />
      <el-table-column prop="id" label="ID" width="60" />
      <el-table-column prop="name" label="名称" min-width="120" />
      <el-table-column prop="command" label="命令" min-width="160" show-overflow-tooltip />
      <el-table-column prop="cron_expression" label="Cron" width="140" />
      <el-table-column label="标签" min-width="130">
        <template #default="{ row }">
          <el-tag v-for="t in row.tags" :key="t" size="small" class="tag" @click="filterByTag(t)">{{ t }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="row.enabled ? 'success' : 'info'" size="small">{{ row.enabled ? '启用' : '禁用' }}</el-tag>
          <el-tag v-if="row.status === 'running'" type="warning" size="small" style="margin-left:4px">运行中</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="上次结果" width="90">
        <template #default="{ row }">
          <el-tag :type="statusType(row.last_run_status)" size="small">{{ statusText(row.last_run_status) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="280" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="runTask(row)" :disabled="row.status === 'running'">运行</el-button>
          <el-button size="small" @click="stopTask(row)" :disabled="row.status !== 'running'">停止</el-button>
          <el-button size="small" @click="toggle(row)">{{ row.enabled ? '禁用' : '启用' }}</el-button>
          <el-button size="small" @click="openEdit(row)">编辑</el-button>
          <el-button size="small" type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-drawer v-model="liveDrawer.visible" title="实时日志" size="55%" @close="liveDrawer.taskId = 0">
      <LogViewer v-if="liveDrawer.taskId" :task-id="liveDrawer.taskId" :key="liveDrawer.taskId" />
    </el-drawer>

    <el-dialog v-model="formDialog.visible" :title="formDialog.id ? '编辑任务' : '新建任务'" width="560px">
      <el-form label-width="110px">
        <el-form-item label="名称"><el-input v-model="formDialog.form.name" /></el-form-item>
        <el-form-item label="命令">
          <el-input v-model="formDialog.form.command" placeholder="例:python3 mytask.py 或 mytask.py" />
        </el-form-item>
        <el-form-item label="Cron">
          <el-input v-model="formDialog.form.cron_expression" placeholder="*/10 * * * *">
            <template #append><el-button @click="describe">校验</el-button></template>
          </el-input>
          <div v-if="cronDesc" class="cron-desc">{{ cronDesc }}</div>
        </el-form-item>
        <el-form-item label="标签">
          <el-select v-model="formDialog.form.tags" multiple filterable allow-create default-first-option placeholder="输入后回车创建" style="width:100%">
            <el-option v-for="t in Object.keys(tagStats)" :key="t" :label="t" :value="t" />
          </el-select>
        </el-form-item>
        <el-form-item label="超时(秒)"><el-input-number v-model="formDialog.form.timeout_seconds" :min="0" /></el-form-item>
        <el-form-item label="重试次数"><el-input-number v-model="formDialog.form.max_retries" :min="0" /></el-form-item>
        <el-form-item label="重试间隔(秒)"><el-input-number v-model="formDialog.form.retry_interval" :min="0" /></el-form-item>
        <el-form-item label="启用"><el-switch v-model="formDialog.form.enabled" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="formDialog.visible = false">取消</el-button>
        <el-button type="primary" :loading="formDialog.loading" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowDown } from '@element-plus/icons-vue'
import { taskApi, type Task } from '@/api/task'
import LogViewer from '@/components/LogViewer.vue'

const tasks = ref<Task[]>([])
const loading = ref(false)
const keyword = ref('')
const tag = ref('')
const tagStats = ref<Record<string, number>>({})
const selected = ref<Task[]>([])

const liveDrawer = reactive({ visible: false, taskId: 0 })

const formDialog = reactive({
  visible: false, loading: false, id: 0,
  form: { name: '', command: '', cron_expression: '', enabled: true, timeout_seconds: 0, max_retries: 0, retry_interval: 0, tags: [] as string[] },
})
const cronDesc = ref('')

async function load() {
  loading.value = true
  try {
    const res: any = await taskApi.list(keyword.value, '', tag.value)
    tasks.value = res.data.data || []
    const tags: any = await taskApi.tags()
    tagStats.value = tags.data.data || {}
  } catch {} finally { loading.value = false }
}
onMounted(load)

function filterByTag(t: string) {
  tag.value = t
  load()
}

function onSelection(rows: Task[]) {
  selected.value = rows
}

async function batch(action: 'enable' | 'disable' | 'run' | 'delete') {
  const ids = selected.value.map((t) => t.id)
  if (!ids.length) return
  if (action === 'delete') {
    try {
      await ElMessageBox.confirm(`确认删除选中的 ${ids.length} 个任务?`, '提示', { type: 'warning' })
    } catch { return }
  }
  try {
    const res: any = await taskApi.batch(action, ids)
    const r = res.data.data || {}
    ElMessage.success(`成功 ${r.ok || 0} 个,失败 ${r.fail || 0} 个`)
    load()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '批量操作失败')
  }
}

function openCreate() {
  formDialog.id = 0
  formDialog.form = { name: '', command: '', cron_expression: '*/10 * * * *', enabled: true, timeout_seconds: 0, max_retries: 0, retry_interval: 0, tags: [] }
  cronDesc.value = ''
  formDialog.visible = true
}
function openEdit(row: Task) {
  formDialog.id = row.id
  formDialog.form = { ...row, tags: row.tags ? [...row.tags] : [] }
  cronDesc.value = ''
  formDialog.visible = true
}

async function describe() {
  try {
    const res: any = await taskApi.cronDescribe(formDialog.form.cron_expression)
    cronDesc.value = res.data
  } catch {}
}

async function save() {
  formDialog.loading = true
  try {
    if (formDialog.id) await taskApi.update(formDialog.id, formDialog.form)
    else await taskApi.create(formDialog.form)
    ElMessage.success('保存成功')
    formDialog.visible = false
    load()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败')
  } finally { formDialog.loading = false }
}

async function runTask(row: Task) {
  try {
    await taskApi.run(row.id)
    ElMessage.success('已启动')
    liveDrawer.taskId = row.id
    liveDrawer.visible = true
    setTimeout(load, 1000)
  } catch {}
}
async function stopTask(row: Task) {
  try { await taskApi.stop(row.id); ElMessage.success('已停止'); setTimeout(load, 500) } catch {}
}
async function toggle(row: Task) {
  try { await (row.enabled ? taskApi.disable(row.id) : taskApi.enable(row.id)); load() } catch {}
}
async function remove(row: Task) {
  try {
    await ElMessageBox.confirm(`确认删除任务「${row.name}」?`, '提示', { type: 'warning' })
    await taskApi.remove(row.id); ElMessage.success('已删除'); load()
  } catch {}
}

function statusType(s: string) {
  return { success: 'success', failed: 'danger', aborted: 'warning', none: 'info' }[s] || 'info'
}
function statusText(s: string) {
  return { success: '成功', failed: '失败', aborted: '终止', none: '—' }[s] || s
}
</script>

<style scoped>
.toolbar { display: flex; gap: 8px; margin-bottom: 12px; align-items: center; }
.cron-desc { color: #909399; font-size: 12px; margin-top: 4px; }
.tag { margin-right: 4px; cursor: pointer; }
</style>
