<template>
  <div>
    <div class="toolbar">
      <el-input v-model="username" placeholder="按用户名筛选" clearable style="width:180px" @keyup.enter="load" @clear="load" />
      <el-select v-model="action" placeholder="全部动作" clearable style="width:180px" @change="load">
        <el-option v-for="(label, val) in actionMap" :key="val" :label="label" :value="val" />
      </el-select>
      <el-button @click="load">查询</el-button>
    </div>

    <el-table :data="logs" v-loading="loading" border stripe>
      <el-table-column prop="id" label="ID" />
      <el-table-column prop="username" label="用户" />
      <el-table-column label="动作">
        <template #default="{ row }">{{ actionLabel(row.action) }}</template>
      </el-table-column>
      <el-table-column prop="resource" label="对象" show-overflow-tooltip />
      <el-table-column prop="detail" label="详情" show-overflow-tooltip />
      <el-table-column prop="ip" label="IP" />
      <el-table-column label="时间" show-overflow-tooltip>
        <template #default="{ row }">{{ fmtTime(row.created_at) }}</template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination
        layout="total, prev, pager, next"
        :total="total"
        :page-size="pageSize"
        :current-page="page"
        @current-change="onPage"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { fmtTime } from '@/utils/time'
import { auditApi, actionLabel, type AuditLog } from '@/api/audit'

const logs = ref<AuditLog[]>([])
const loading = ref(false)
const username = ref('')
const action = ref('')
const page = ref(1)
const pageSize = 20
const total = ref(0)

const actionMap: Record<string, string> = {
  init_admin: '初始化管理员',
  login_success: '登录成功',
  login_failed: '登录失败',
  logout: '登出',
  task_create: '创建任务',
  task_update: '更新任务',
  task_delete: '删除任务',
  task_run: '运行任务',
  task_stop: '停止任务',
  task_enable: '启用任务',
  task_disable: '禁用任务',
  script_save: '保存脚本',
  script_create_dir: '创建目录',
  script_delete: '删除脚本',
  script_rename: '重命名脚本',
  script_upload: '上传脚本',
  script_run: '运行脚本',
  script_run_code: '运行内联代码',
  env_create: '创建变量',
  env_update: '更新变量',
  env_delete: '删除变量',
  env_batch_delete: '批量删除变量',
}

async function load() {
  loading.value = true
  try {
    const res: any = await auditApi.list(username.value, action.value, page.value, pageSize)
    logs.value = res.data.data || []
    total.value = res.data.total || 0
  } catch {} finally { loading.value = false }
}

function onPage(p: number) {
  page.value = p
  load()
}

onMounted(load)
</script>

<style scoped>
.toolbar { display: flex; gap: 8px; margin-bottom: 12px; }
.pager { display: flex; justify-content: flex-end; margin-top: 12px; }
</style>
