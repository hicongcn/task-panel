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
      <el-input v-model="keyword" placeholder="搜索脚本" clearable size="small" class="tree-search" @input="onSearch" @clear="onSearch" />
      <el-tree ref="treeRef" :data="tree" node-key="key" :filter-node-method="filterNode" :props="{ label: 'title' }" highlight-current draggable @node-click="onSelect" @node-drop="onNodeDrop">
        <template #default="{ data }">
          <span class="tree-node">
            <el-icon v-if="data.type === 'directory'" class="dir-icon"><Folder /></el-icon>
            <el-icon v-else class="file-icon"><Document /></el-icon>
            <span class="node-title">{{ data.title }}</span>
          </span>
        </template>
      </el-tree>
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
        <MonacoEditor v-model="current.content" :language="editorLanguage" class="editor-wrap" />
      </template>
    </div>

    <el-dialog v-model="fileDialog.visible" title="新建文件" width="420px">
      <el-form label-width="70px">
        <el-form-item label="目录">
          <el-select v-model="fileDialog.dir" placeholder="根目录" clearable style="width:100%">
            <el-option v-for="d in dirList" :key="d" :label="d || '(根目录)'" :value="d" />
          </el-select>
        </el-form-item>
        <el-form-item label="文件名" required>
          <el-input v-model="fileDialog.name" placeholder="如 hello.py" @keyup.enter="doCreateFile" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="fileDialog.visible = false">取消</el-button>
        <el-button type="primary" @click="doCreateFile">创建</el-button>
      </template>
    </el-dialog>

    <el-drawer v-model="resultDrawer" title="运行结果" size="50%">
      <pre class="log-surface">{{ result.output }}</pre>
      <div class="result-meta">退出码 {{ result.exit_code }} · 耗时 {{ result.duration.toFixed(2) }}s <el-tag v-if="result.timed_out" type="warning" size="small">超时</el-tag></div>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowDown, Folder, Document } from '@element-plus/icons-vue'
import MonacoEditor from '@/components/MonacoEditor.vue'
import { languageForPath } from '@/monaco'
import { scriptApi, type ScriptNode } from '@/api/script'

const tree = ref<ScriptNode[]>([])
const treeRef = ref()
const keyword = ref('')
const current = reactive({ path: '', content: '' })
const resultDrawer = ref(false)
const result = reactive({ output: '', exit_code: 0, duration: 0, timed_out: false })

const editorLanguage = computed(() => (current.path ? languageForPath(current.path) : 'plaintext'))

const dirList = ref<string[]>([''])
const fileDialog = reactive({ visible: false, dir: '', name: '' })

// 展平树里的所有目录路径
function collectDirs(nodes: ScriptNode[]): string[] {
  const out: string[] = []
  for (const n of nodes || []) {
    if (n.type === 'directory') {
      out.push(n.key)
      if (n.children?.length) out.push(...collectDirs(n.children))
    }
  }
  return out
}

// filterNode 树节点名称过滤(搜索)
function filterNode(value: string, data: any) {
  if (!value) return true
  return (data.title || '').toLowerCase().includes(value.toLowerCase())
}

function onSearch() {
  treeRef.value?.filter(keyword.value)
}

async function refreshTree() {
  try {
    const res: any = await scriptApi.tree()
    tree.value = res.data.data || []
    dirList.value = ['', ...collectDirs(tree.value)]
  } catch {}
}
refreshTree()

// onNodeDrop 拖拽移动:拖到文件夹(任意落点)视为移入;拖到文件则移入其所在目录。
async function onNodeDrop(dragging: any, drop: any, _dropType: string) {
  if (dragging.data?.type !== 'file') {
    ElMessage.warning('暂不支持移动目录')
    refreshTree()
    return
  }
  // 目标目录:拖到文件夹→该文件夹;拖到文件→该文件所在目录
  let targetDir = ''
  if (drop.data?.type === 'directory') {
    targetDir = drop.data.key
  } else if (drop.data?.type === 'file') {
    const p: string = drop.data.key
    targetDir = p.includes('/') ? p.slice(0, p.lastIndexOf('/')) : ''
  }
  const oldPath: string = dragging.data.key
  const oldDir = oldPath.includes('/') ? oldPath.slice(0, oldPath.lastIndexOf('/')) : ''
  if (!targetDir || oldDir === targetDir) {
    refreshTree() // 已在目标目录或无法表达,恢复真实状态
    return
  }
  try {
    await scriptApi.move(oldPath, targetDir)
    ElMessage.success('已移动')
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '移动失败')
  } finally {
    refreshTree()
  }
}

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

function newFile() {
  Object.assign(fileDialog, { visible: true, dir: '', name: '' })
}

async function doCreateFile() {
  const name = fileDialog.name.trim()
  if (!name) return ElMessage.warning('请输入文件名')
  const path = fileDialog.dir ? `${fileDialog.dir}/${name}` : name
  try {
    await scriptApi.save(path, '')
    ElMessage.success('已创建')
    fileDialog.visible = false
    refreshTree()
    // 打开新文件
    const res: any = await scriptApi.content(path)
    current.path = res.data.data.path
    current.content = res.data.data.content
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '创建失败')
  }
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
.tree-search { margin-bottom: 8px; }
.tree-node { display: inline-flex; align-items: center; gap: 5px; }
.dir-icon { color: #e6a23c; }
.file-icon { color: #909399; }
.node-title { overflow: hidden; text-overflow: ellipsis; }
.tree-actions { display: flex; gap: 4px; }
.editor-pane { flex: 1; background: #fff; border-radius: 6px; display: flex; flex-direction: column; overflow: hidden; }
.empty { flex: 1; display: flex; align-items: center; justify-content: center; color: #909399; }
.editor-head { display: flex; justify-content: space-between; align-items: center; padding: 8px 12px; border-bottom: 1px solid #ebeef5; }
.path { font-family: monospace; font-size: 13px; }
.editor-wrap { flex: 1; min-height: 0; padding: 8px; }
.result-meta { margin-top: 8px; color: #909399; font-size: 13px; }
</style>
