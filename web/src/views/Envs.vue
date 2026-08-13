<template>
  <div>
    <div class="toolbar">
      <el-input v-model="keyword" placeholder="搜索名称/备注" clearable style="width:220px" @keyup.enter="load" @clear="load" />
      <el-button @click="load">刷新</el-button>
      <el-button type="primary" @click="openCreate">新建变量</el-button>
    </div>
    <el-table :data="envs" v-loading="loading" border stripe>
      <el-table-column prop="name" label="名称" min-width="140" />
      <el-table-column prop="value_masked" label="值(脱敏)" min-width="160" />
      <el-table-column prop="group" label="分组" width="120" />
      <el-table-column prop="remark" label="备注" min-width="120" />
      <el-table-column label="启用" width="80">
        <template #default="{ row }"><el-tag :type="row.enabled ? 'success' : 'info'" size="small">{{ row.enabled ? '是' : '否' }}</el-tag></template>
      </el-table-column>
      <el-table-column label="操作" width="160" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="openEdit(row)">编辑</el-button>
          <el-button size="small" type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="formDialog.visible" :title="formDialog.id ? '编辑变量' : '新建变量'" width="500px">
      <el-form label-width="90px">
        <el-form-item label="名称"><el-input v-model="formDialog.form.name" :disabled="!!formDialog.id" placeholder="MY_VAR" /></el-form-item>
        <el-form-item label="值"><el-input v-model="formDialog.form.value" type="password" show-password /></el-form-item>
        <el-form-item label="分组"><el-input v-model="formDialog.form.group" /></el-form-item>
        <el-form-item label="备注"><el-input v-model="formDialog.form.remark" /></el-form-item>
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
import { envApi, type EnvVar } from '@/api/env'

const envs = ref<EnvVar[]>([])
const loading = ref(false)
const keyword = ref('')

const formDialog = reactive({
  visible: false, loading: false, id: 0,
  form: { name: '', value: '', group: '', remark: '', enabled: true } as any,
})

async function load() {
  loading.value = true
  try {
    const res: any = await envApi.list(keyword.value)
    envs.value = res.data.data || []
  } catch {} finally { loading.value = false }
}
onMounted(load)

function openCreate() {
  formDialog.id = 0
  formDialog.form = { name: '', value: '', group: '', remark: '', enabled: true }
  formDialog.visible = true
}
function openEdit(row: EnvVar) {
  formDialog.id = row.id
  formDialog.form = { name: row.name, value: '', group: row.group, remark: row.remark, enabled: row.enabled }
  formDialog.visible = true
}

async function save() {
  if (!formDialog.form.name) { ElMessage.warning('请填写名称'); return }
  formDialog.loading = true
  try {
    if (formDialog.id) await envApi.update(formDialog.id, formDialog.form)
    else await envApi.create(formDialog.form)
    ElMessage.success('保存成功')
    formDialog.visible = false
    load()
  } catch {} finally { formDialog.loading = false }
}

async function remove(row: EnvVar) {
  try {
    await ElMessageBox.confirm(`确认删除变量「${row.name}」?`, '提示', { type: 'warning' })
    await envApi.remove(row.id); ElMessage.success('已删除'); load()
  } catch {}
}
</script>

<style scoped>
.toolbar { display: flex; gap: 8px; margin-bottom: 12px; }
</style>
