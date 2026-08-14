<template>
  <div>
    <div class="op-row">
      <el-input
        v-model="pkg"
        placeholder="包名(支持版本约束,如 requests==2.31.0)"
        style="width:300px"
        clearable
        @keyup.enter="onInstall"
      />
      <el-button type="primary" :loading="installing" @click="onInstall">安装</el-button>
      <el-button :loading="loading" @click="load">刷新</el-button>
    </div>

    <el-table :data="pkgs" v-loading="loading" border stripe max-height="440">
      <el-table-column prop="name" label="包名" show-overflow-tooltip />
      <el-table-column prop="version" label="版本" show-overflow-tooltip />
      <el-table-column label="操作" width="110" fixed="right">
        <template #default="{ row }">
          <el-button size="small" type="danger" :loading="busy === row.name" @click="onUninstall(row)">卸载</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="outVisible" :title="outTitle" width="70%">
      <pre class="output">{{ output || '(无输出)' }}</pre>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { depApi, type PkgInfo } from '@/api/deps'

const props = defineProps<{ kind: 'python' | 'node' }>()

const pkgs = ref<PkgInfo[]>([])
const loading = ref(false)
const installing = ref(false)
const busy = ref('')
const pkg = ref('')
const outVisible = ref(false)
const outTitle = ref('')
const output = ref('')

function isPy() {
  return props.kind === 'python'
}

async function load() {
  loading.value = true
  try {
    const res: any = await (isPy() ? depApi.listPython() : depApi.listNode())
    pkgs.value = res.data.data || []
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '获取列表失败')
  } finally { loading.value = false }
}

async function onInstall() {
  if (!pkg.value.trim()) return ElMessage.warning('请输入包名')
  installing.value = true
  try {
    const res: any = await (isPy() ? depApi.installPython(pkg.value) : depApi.installNode(pkg.value))
    outTitle.value = '✅ 安装完成'
    output.value = res.data.output || ''
    outVisible.value = true
    pkg.value = ''
    load()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '安装失败')
    output.value = e?.response?.data?.output || ''
    if (output.value) {
      outTitle.value = '❌ 安装失败(输出)'
      outVisible.value = true
    }
  } finally { installing.value = false }
}

async function onUninstall(row: PkgInfo) {
  try {
    await ElMessageBox.confirm(`确认卸载「${row.name}」?`, '提示', { type: 'warning' })
  } catch { return }
  busy.value = row.name
  try {
    const res: any = await (isPy() ? depApi.uninstallPython(row.name) : depApi.uninstallNode(row.name))
    outTitle.value = '✅ 卸载完成'
    output.value = res.data.output || ''
    outVisible.value = true
    load()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '卸载失败')
  } finally { busy.value = '' }
}

onMounted(load)
</script>

<style scoped>
.op-row { display: flex; gap: 8px; margin-bottom: 12px; }
.output {
  white-space: pre-wrap; word-break: break-all;
  max-height: 60vh; overflow: auto;
  background: #1e1e1e; color: #d4d4d4;
  padding: 12px; border-radius: 4px; font-size: 12px; line-height: 1.6;
}
</style>
