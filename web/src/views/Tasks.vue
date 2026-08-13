<template>
  <div>
    <div class="toolbar">
      <el-input v-model="keyword" placeholder="搜索任务名" clearable style="width:220px" @keyup.enter="load" @clear="load" />
      <el-button @click="load">刷新</el-button>
      <el-button type="primary" @click="openCreate">新建任务</el-button>
    </div>
    <el-table :data="tasks" v-loading="loading" border stripe>
      <el-table-column prop="id" label="ID" width="60" />
      <el-table-column prop="name" label="名称" min-width="120" />
      <el-table-column prop="command" label="命令" min-width="180" show-overflow-tooltip />
      <el-table-column prop="cron_expression" label="Cron" width="150" />
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
import { taskApi, type Task } from '@/api/task'
import LogViewer from '@/components/LogViewer.vue'

const tasks = ref<Task[]>([])
const loading = ref(false)
const keyword = ref('')

const liveDrawer = reactive({ visible: false, taskId: 0 })

const formDialog = reactive({
  visible: false, loading: false, id: 0,
  form: { name: '', command: '', cron_expression: '', enabled: true, timeout_seconds: 0, max_retries: 0, retry_interval: 0 },
})
const cronDesc = ref('')

async function load() {
  loading.value = true
  try {
    const res: any = await taskApi.list(keyword.value)
    tasks.value = res.data.data || []
  } catch {} finally { loading.value = false }
}
onMounted(load)

function openCreate() {
  formDialog.id = 0
  formDialog.form = { name: '', command: '', cron_expression: '*/10 * * * *', enabled: true, timeout_seconds: 0, max_retries: 0, retry_interval: 0 }
  cronDesc.value = ''
  formDialog.visible = true
}
function openEdit(row: Task) {
  formDialog.id = row.id
  formDialog.form = { ...row }
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
  } catch {} finally { formDialog.loading = false }
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
.toolbar { display: flex; gap: 8px; margin-bottom: 12px; }
.cron-desc { color: #909399; font-size: 12px; margin-top: 4px; }
</style>
