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
      <el-table-column type="selection" />
      <el-table-column prop="name" label="名称" width="70" show-overflow-tooltip />
      <el-table-column prop="command" label="脚本" width="130" show-overflow-tooltip />
      <el-table-column prop="cron_expression" label="Cron" width="100" show-overflow-tooltip />
      <el-table-column label="标签" width="90">
        <template #default="{ row }">
          <el-tag v-for="t in row.tags" :key="t" size="small" class="tag" @click="filterByTag(t)">{{ t }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="70">
        <template #default="{ row }">
          <el-tag :type="row.enabled ? 'success' : 'info'" size="small">{{ row.enabled ? '启用' : '禁用' }}</el-tag>
          <el-tag v-if="row.status === 'running'" type="warning" size="small" style="margin-left:4px">运行中</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="200" >
        <template #default="{ row }">
          <div class="op-cell">
          <el-button size="small" :type="row.status === 'running' ? 'danger' : 'primary'" @click="row.status === 'running' ? stopTask(row) : runTask(row)">
            {{ row.status === 'running' ? '停止' : '运行' }}
          </el-button>
          <el-button size="small" @click="viewLog(row)">日志</el-button>
          <el-dropdown trigger="click" @command="(cmd: string) => onMore(cmd, row)">
            <el-button size="small">更多<el-icon class="more-arrow"><ArrowDown /></el-icon></el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="toggle">{{ row.enabled ? '禁用' : '启用' }}</el-dropdown-item>
                <el-dropdown-item command="edit">编辑</el-dropdown-item>
                <el-dropdown-item command="delete" divided>删除</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
          </div>
        </template>
      </el-table-column>
    </el-table>

    <el-drawer v-model="liveDrawer.visible" title="实时日志" size="55%" @close="liveDrawer.taskId = 0">
      <LogViewer v-if="liveDrawer.taskId" :task-id="liveDrawer.taskId" :key="liveDrawer.taskId" />
    </el-drawer>

    <el-dialog v-model="logDialog.visible" :title="'最近日志 - ' + logDialog.taskName" width="70%">
      <div class="log-surface log-static" v-html="logDialog.contentHtml"></div>
    </el-dialog>

    <el-dialog v-model="formDialog.visible" :title="formDialog.id ? '编辑任务' : '新建任务'" width="560px">
      <el-form label-width="110px">
        <el-form-item label="名称"><el-input v-model="formDialog.form.name" placeholder="留空则使用脚本文件名" /></el-form-item>
        <el-form-item label="命令">
          <el-select v-model="formDialog.form.command" filterable allow-create default-first-option placeholder="选择或输入脚本,如 hello.sh 或 sub/hello.sh(免解释器前缀)" style="width:100%">
            <el-option v-for="f in scriptFiles" :key="f" :label="f" :value="f" />
          </el-select>
          <div class="tip">直接选脚本或输入文件名即可,无需 node / python3 前缀;支持带参数,如 hello.sh --force</div>
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
import { logApi } from '@/api/log'
import { ansiToHtml } from '@/utils/ansi'
import { scriptApi } from '@/api/script'
import LogViewer from '@/components/LogViewer.vue'

const tasks = ref<Task[]>([])
const loading = ref(false)
const keyword = ref('')
const tag = ref('')
const tagStats = ref<Record<string, number>>({})
const selected = ref<Task[]>([])

const liveDrawer = reactive({ visible: false, taskId: 0 })
const logDialog = reactive({ visible: false, taskName: '', contentHtml: '' })

const formDialog = reactive({
  visible: false, loading: false, id: 0,
  form: { name: '', command: '', cron_expression: '', enabled: true, timeout_seconds: 0, max_retries: 0, retry_interval: 0, tags: [] as string[] },
})
const cronDesc = ref('')
const scriptFiles = ref<string[]>([])

// 加载脚本文件列表(展平树,供命令下拉选择)
async function loadScriptFiles() {
  try {
    const res: any = await scriptApi.tree()
    const walk = (nodes: any[]): string[] => {
      const out: string[] = []
      for (const n of nodes || []) {
        if (n.type === 'file') out.push(n.key)
        if (n.children?.length) out.push(...walk(n.children))
      }
      return out
    }
    scriptFiles.value = walk(res.data.data || [])
  } catch {}
}

async function load() {
  loading.value = true
  try {
    const res: any = await taskApi.list(keyword.value, '', tag.value)
    tasks.value = res.data.data || []
    const tags: any = await taskApi.tags()
    tagStats.value = tags.data.data || {}
  } catch {} finally { loading.value = false }
}
onMounted(() => { load(); loadScriptFiles() })

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
  // 名称留空时默认取命令的脚本文件名
  if (!formDialog.form.name.trim() && formDialog.form.command.trim()) {
    const first = formDialog.form.command.trim().split(/\s+/)[0] || ''
    formDialog.form.name = first.split('/').pop() || ''
  }
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

// onMore 处理"更多"下拉菜单操作。
function onMore(cmd: string, row: Task) {
  if (cmd === 'toggle') toggle(row)
  else if (cmd === 'edit') openEdit(row)
  else if (cmd === 'delete') remove(row)
}

// viewLog 查看最近一条日志:优先读历史日志文件内容(运行中也能读到当前已写入部分)。
async function viewLog(row: Task) {
  try {
    const res: any = await logApi.latest(row.id)
    const log = res.data.data
    if (!log?.id) {
      ElMessage.info('该任务还没有执行记录')
      return
    }
    const d: any = await logApi.detail(log.id)
    logDialog.taskName = row.name
    logDialog.contentHtml = ansiToHtml(d.data.content || '(日志内容为空)')
    logDialog.visible = true
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '获取日志失败')
  }
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

</script>

<style scoped>
.toolbar { display: flex; gap: 8px; margin-bottom: 12px; align-items: center; }
.cron-desc { color: #909399; font-size: 12px; margin-top: 4px; }
.tag { margin-right: 4px; cursor: pointer; }
.more-arrow { margin-left: 2px; font-size: 12px; }
.op-cell { display: flex; align-items: center; gap: 6px; }
.op-cell :deep(.el-button + .el-button) { margin-left: 0; }
.log-static { max-height: 60vh; }
.tip { color: #909399; font-size: 12px; margin-top: 4px; line-height: 1.5; }
</style>
