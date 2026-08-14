<template>
  <div>
    <el-alert type="info" :closable="false" class="intro">
      <template #title>
        <b>Open API(参考青龙结构)</b> — 在「应用」创建后得到 <code>client_id</code> + <code>client_secret</code>,然后:
        <code>POST /api/v1/open/auth/token</code> 换取令牌,后续请求带
        <code>Authorization: Bearer &lt;token&gt;</code> 调用开放接口(<code>/open/tasks</code> 等),按 scopes 控制权限。
      </template>
    </el-alert>

    <div class="toolbar">
      <el-button type="primary" @click="openCreate">新建应用</el-button>
    </div>

    <el-table :data="apps" v-loading="loading" border stripe>
      <el-table-column prop="name" label="名称" show-overflow-tooltip />
      <el-table-column label="Client ID">
        <template #default="{ row }">
          <span class="copy-cell">{{ row.client_id }}</span>
          <el-button link size="small" class="copy-btn" @click="copy(row.client_id)"><el-icon><CopyDocument /></el-icon></el-button>
        </template>
      </el-table-column>
      <el-table-column label="Client Secret">
        <template #default="{ row }">
          <span class="copy-cell secret-text">••••••••</span>
          <el-button link size="small" class="copy-btn" @click="copy(row.client_secret)"><el-icon><CopyDocument /></el-icon></el-button>
        </template>
      </el-table-column>
      <el-table-column label="权限">
        <template #default="{ row }">
          <el-tag v-for="s in scopesOf(row)" :key="s" size="small" style="margin-right:4px">{{ scopeLabels[s] || s }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="200" >
        <template #default="{ row }">
          <el-button size="small" @click="openEdit(row)">编辑</el-button>
          <el-button size="small" type="warning" @click="onReset(row)">重置</el-button>
          <el-button size="small" type="danger" @click="onDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="createVisible" :title="editingId ? '编辑应用' : '新建应用'" width="480px">
      <el-form label-width="90px">
        <el-form-item label="名称" required>
          <el-input v-model="form.name" placeholder="如: 自动同步脚本" />
        </el-form-item>
        <el-form-item label="权限范围">
          <el-checkbox-group v-model="form.scopes">
            <el-checkbox v-for="(label, val) in scopeLabels" :key="val" :value="val" :label="val">{{ label }}</el-checkbox>
          </el-checkbox-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" @click="onCreate">创建</el-button>
      </template>
    </el-dialog>


  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { CopyDocument } from '@element-plus/icons-vue'
import { openApi, scopeLabels, type OpenApp } from '@/api/openapi'

const apps = ref<OpenApp[]>([])
const loading = ref(false)
const createVisible = ref(false)
const editingId = ref<number | null>(null)
const form = reactive({ name: '', scopes: [] as string[] })

function scopesOf(row: OpenApp): string[] {
  const raw = (row as any).scopes
  if (Array.isArray(raw)) return raw
  try { return JSON.parse(raw || '[]') } catch { return [] }
}

async function load() {
  loading.value = true
  try {
    const res: any = await openApi.list()
    apps.value = res.data.data || []
  } catch {} finally { loading.value = false }
}

function openCreate() {
  editingId.value = null
  Object.assign(form, { name: '', scopes: [] })
  createVisible.value = true
}

// openEdit 打开编辑对话框(名称/权限)
function openEdit(row: OpenApp) {
  editingId.value = row.id
  Object.assign(form, { name: row.name, scopes: scopesOf(row) })
  createVisible.value = true
}

// copy 复制文本(Client ID / Secret 点击复制)
async function copy(text: string) {
  if (!text) return ElMessage.warning('内容为空')
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success('已复制')
  } catch {
    ElMessage.warning('复制失败,请手动选择')
  }
}

async function onCreate() {
  if (!form.name.trim()) return ElMessage.warning('请输入名称')
  try {
    if (editingId.value) {
      await openApi.update(editingId.value, { name: form.name.trim(), scopes: form.scopes })
      ElMessage.success('已更新')
    } else {
      await openApi.create(form.name.trim(), form.scopes)
      ElMessage.success('创建成功,凭据可在列表中复制')
    }
    createVisible.value = false
    load()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败')
  }
}

async function onReset(row: OpenApp) {
  try {
    await ElMessageBox.confirm(`重置「${row.name}」的密钥?旧密钥将立即失效。`, '确认重置', { type: 'warning' })
  } catch { return }
  try {
    await openApi.resetSecret(row.id)
    ElMessage.success('密钥已重置,可在列表中复制')
    load()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '重置失败')
  }
}

async function onDelete(row: OpenApp) {
  try {
    await ElMessageBox.confirm(`确定删除应用「${row.name}」?`, '提示', { type: 'warning' })
  } catch { return }
  try {
    await openApi.remove(row.id)
    ElMessage.success('已删除')
    load()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '删除失败')
  }
}

onMounted(load)
</script>

<style scoped>
.intro { margin-bottom: 14px; }
.copy-cell { vertical-align: middle; font-family: monospace; }
.secret-text { letter-spacing: 2px; }
.copy-btn { margin-left: 2px; }
.intro code { background: #f0f2f5; padding: 1px 5px; border-radius: 3px; font-size: 12px; }
.toolbar { margin-bottom: 12px; }
</style>
