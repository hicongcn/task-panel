<template>
  <div class="scripts-wrap">
    <div class="tree-pane">
      <div class="tree-head">
        <span>脚本</span>
        <div class="tree-actions">
          <el-button size="small" @click="refreshTree">刷新</el-button>
          <el-dropdown trigger="click" @command="onNewCommand">
            <el-button size="small" type="primary">新建<el-icon><ArrowDown /></el-icon></el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="file">新建文件</el-dropdown-item>
                <el-dropdown-item command="dir">新建目录</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </div>
      <el-tree :data="tree" node-key="key" :props="{ label: 'title' }" highlight-current @node-click="onSelect" />
    </div>

    <div class="editor-pane">
      <div v-if="!current.path" class="empty">选择左侧文件以编辑,或在上方新建文件</div>
      <template v-else>
        <div class="editor-head">
          <span class="path">{{ current.path }}</span>
          <div>
            <el-button size="small" @click="save">保存</el-button>
            <el-button size="small" type="primary" @click="runScript">运行</el-button>
            <el-button size="small" @click="rename">重命名</el-button>
            <el-button size="small" type="danger" @click="remove">删除</el-button>
          </div>
        </div>
        <textarea v-model="current.content" class="editor" spellcheck="false"></textarea>
      </template>
    </div>

    <el-drawer v-model="resultDrawer" title="运行结果" size="50%">
      <pre class="log-surface">{{ result.output }}</pre>
      <div class="result-meta">退出码 {{ result.exit_code }} · 耗时 {{ result.duration.toFixed(2) }}s <el-tag v-if="result.timed_out" type="warning" size="small">超时</el-tag></div>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowDown } from '@element-plus/icons-vue'
import { scriptApi, type ScriptNode } from '@/api/script'

const tree = ref<ScriptNode[]>([])
const current = reactive({ path: '', content: '' })
const resultDrawer = ref(false)
const result = reactive({ output: '', exit_code: 0, duration: 0, timed_out: false })

async function refreshTree() {
  try {
    const res: any = await scriptApi.tree()
    tree.value = res.data.data || []
  } catch {}
}
refreshTree()

async function onSelect(node: ScriptNode) {
  if (node.type !== 'file') return
  try {
    const res: any = await scriptApi.content(node.key)
    current.path = res.data.data.path
    current.content = res.data.data.content
  } catch {}
}

async function save() {
  if (!current.path) return
  try { await scriptApi.save(current.path, current.content); ElMessage.success('已保存') } catch {}
}

async function runScript() {
  if (!current.path) return
  await save()
  try {
    const res: any = await scriptApi.run(current.path)
    Object.assign(result, res.data.data)
    resultDrawer.value = true
  } catch {}
}

async function onNewCommand(cmd: string) {
  if (cmd === 'file') await newFile()
  else await newDir()
}

async function newFile() {
  try {
    const { value } = await ElMessageBox.prompt('输入文件相对路径(如 demo/hello.py)', '新建文件', {
      inputValue: '',
      confirmButtonText: '创建',
      cancelButtonText: '取消',
      inputPattern: /\S/,
      inputErrorMessage: '路径不能为空',
    })
    const path = value.trim()
    await scriptApi.save(path, '')
    ElMessage.success('已创建')
    refreshTree()
    // 打开新文件
    const res: any = await scriptApi.content(path)
    current.path = res.data.data.path
    current.content = res.data.data.content
  } catch {}
}

async function newDir() {
  try {
    const { value } = await ElMessageBox.prompt('输入目录相对路径(如 demo)', '新建目录', {
      inputValue: '',
      confirmButtonText: '创建',
      cancelButtonText: '取消',
      inputPattern: /\S/,
      inputErrorMessage: '路径不能为空',
    })
    await scriptApi.createDir(value.trim())
    ElMessage.success('已创建')
    refreshTree()
  } catch {}
}

async function rename() {
  if (!current.path) return
  const oldName = current.path.split('/').pop() || current.path
  try {
    const { value } = await ElMessageBox.prompt('输入新文件名(不含目录)', '重命名', {
      inputValue: oldName,
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      inputPattern: /^[^/\\]+$/,
      inputErrorMessage: '文件名不能包含 / 或 \\',
    })
    await scriptApi.rename(current.path, value.trim())
    ElMessage.success('已重命名')
    const dir = current.path.includes('/') ? current.path.slice(0, current.path.lastIndexOf('/') + 1) : ''
    const newPath = dir + value.trim()
    current.path = newPath
    refreshTree()
  } catch {}
}

async function remove() {
  if (!current.path) return
  try {
    await ElMessageBox.confirm(`确认删除「${current.path}」?`, '提示', { type: 'warning' })
    await scriptApi.remove(current.path)
    ElMessage.success('已删除')
    current.path = ''
    current.content = ''
    refreshTree()
  } catch {}
}
</script>

<style scoped>
.scripts-wrap { display: flex; height: calc(100vh - 90px); gap: 8px; }
.tree-pane { width: 260px; background: #fff; border-radius: 6px; padding: 8px; overflow: auto; }
.tree-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; font-weight: 600; }
.tree-actions { display: flex; gap: 4px; }
.editor-pane { flex: 1; background: #fff; border-radius: 6px; display: flex; flex-direction: column; overflow: hidden; }
.empty { flex: 1; display: flex; align-items: center; justify-content: center; color: #909399; }
.editor-head { display: flex; justify-content: space-between; align-items: center; padding: 8px 12px; border-bottom: 1px solid #ebeef5; }
.path { font-family: monospace; font-size: 13px; }
.editor { flex: 1; border: none; outline: none; resize: none; padding: 12px; font-family: 'SFMono-Regular', Consolas, monospace; font-size: 13px; line-height: 1.5; }
.result-meta { margin-top: 8px; color: #909399; font-size: 13px; }
</style>
