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
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="name" label="名称" min-width="130" />
      <el-table-column prop="client_id" label="Client ID" min-width="200" show-overflow-tooltip />
      <el-table-column label="权限范围" min-width="200">
        <template #default="{ row }">
          <el-tag v-for="s in scopesOf(row)" :key="s" size="small" style="margin-right:4px">{{ scopeLabels[s] || s }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="created_at" label="创建时间" width="175" />
      <el-table-column label="操作" width="180" fixed="right">
        <template #default="{ row }">
          <el-button size="small" type="warning" @click="onReset(row)">重置密钥</el-button>
          <el-button size="small" type="danger" @click="onDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="createVisible" title="新建应用" width="480px">
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

    <el-dialog v-model="secretVisible" title="凭据(仅显示一次,请立即保存)" width="560px" :close-on-click-modal="false">
      <el-alert type="warning" :closable="false" title="关闭后无法再次查看,请先复制保存" style="margin-bottom:12px" />
      <el-form label-width="90px">
        <el-form-item label="Client ID">
          <el-input :model-value="secretData.client_id" readonly><template #append><el-button @click="copy(secretData.client_id)">复制</el-button></template></el-input>
        </el-form-item>
        <el-form-item label="Client Secret">
          <el-input :model-value="secretData.client_secret" readonly type="password" show-password>
            <template #append><el-button @click="copy(secretData.client_secret)">复制</el-button></template>
          </el-input>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button type="primary" @click="secretVisible = false">我已保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { openApi, scopeLabels, type OpenApp } from '@/api/openapi'

const apps = ref<OpenApp[]>([])
const loading = ref(false)
const createVisible = ref(false)
const secretVisible = ref(false)
const secretData = reactive({ client_id: '', client_secret: '' })
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
  Object.assign(form, { name: '', scopes: [] })
  createVisible.value = true
}

async function onCreate() {
  if (!form.name.trim()) return ElMessage.warning('请输入名称')
  try {
    const res: any = await openApi.create(form.name.trim(), form.scopes)
    const d = res.data.data
    Object.assign(secretData, { client_id: d.client_id, client_secret: d.client_secret })
    createVisible.value = false
    secretVisible.value = true
    load()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '创建失败')
  }
}

async function onReset(row: OpenApp) {
  try {
    await ElMessageBox.confirm(`重置「${row.name}」的密钥?旧密钥将立即失效。`, '确认重置', { type: 'warning' })
  } catch { return }
  try {
    const res: any = await openApi.resetSecret(row.id)
    Object.assign(secretData, { client_id: row.client_id, client_secret: res.data.data.client_secret })
    secretVisible.value = true
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

async function copy(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success('已复制')
  } catch {
    ElMessage.warning('复制失败,请手动选择复制')
  }
}

onMounted(load)
</script>

<style scoped>
.intro { margin-bottom: 14px; }
.intro code { background: #f0f2f5; padding: 1px 5px; border-radius: 3px; font-size: 12px; }
.toolbar { margin-bottom: 12px; }
</style>
