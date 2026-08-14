<template>
  <div>
    <!-- 定时备份设置 -->
    <el-card class="setting-card" shadow="never">
      <template #header>
        <span>定时备份</span>
      </template>
      <el-form inline>
        <el-form-item label="启用">
          <el-switch v-model="schedule.enabled" @change="saveSchedule" />
        </el-form-item>
        <el-form-item label="Cron 表达式">
          <el-input v-model="schedule.cron" placeholder="0 3 * * *" style="width:160px" @change="saveSchedule" />
        </el-form-item>
        <el-form-item label="保留份数">
          <el-input-number v-model="schedule.keep" :min="1" :max="100" @change="saveSchedule" />
        </el-form-item>
        <el-form-item>
          <el-button size="small" @click="saveSchedule">保存设置</el-button>
        </el-form-item>
        <el-form-item style="margin-left:auto">
          <el-upload :show-file-list="false" :http-request="uploadRestore" accept=".backup">
            <el-button type="warning">上传备份恢复</el-button>
          </el-upload>
        </el-form-item>
      </el-form>
    </el-card>

    <div class="toolbar">
      <el-button type="primary" :loading="creating" @click="onCreate">立即备份</el-button>
      <span class="hint">备份内容:数据库 + 脚本目录,使用服务器密钥加密(AES-256-GCM)</span>
    </div>

    <el-table :data="backups" v-loading="loading" border stripe>
      <el-table-column prop="name" label="备份文件" min-width="50" show-overflow-tooltip />
      <el-table-column label="大小" min-width="55">
        <template #default="{ row }">{{ formatSize(row.size) }}</template>
      </el-table-column>
      <el-table-column prop="created_at" label="创建时间" min-width="120" show-overflow-tooltip />
      <el-table-column label="操作" width="200" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="onDownload(row)">下载</el-button>
          <el-button size="small" type="warning" @click="onRestore(row)">恢复</el-button>
          <el-button size="small" type="danger" @click="onDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { backupApi, formatSize, type BackupInfo } from '@/api/backup'

const backups = ref<BackupInfo[]>([])
const loading = ref(false)
const creating = ref(false)
const schedule = reactive({ enabled: false, cron: '0 3 * * *', keep: 10 })

async function load() {
  loading.value = true
  try {
    const res: any = await backupApi.list()
    backups.value = res.data.data || []
  } catch {} finally { loading.value = false }
}

async function loadSettings() {
  try {
    const res: any = await backupApi.getSettings()
    const s = res.data.data || {}
    schedule.enabled = s.backup_schedule_enabled === 'true'
    schedule.cron = s.backup_schedule_cron || '0 3 * * *'
    schedule.keep = Number(s.backup_keep) || 10
  } catch {}
}

async function saveSchedule() {
  try {
    await backupApi.updateSettings({
      backup_schedule_enabled: String(schedule.enabled),
      backup_schedule_cron: schedule.cron,
      backup_keep: String(schedule.keep),
    })
    ElMessage.success('定时备份设置已保存')
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败')
  }
}

async function onCreate() {
  creating.value = true
  try {
    await backupApi.create()
    ElMessage.success('备份创建成功')
    load()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '备份失败')
  } finally { creating.value = false }
}

async function onDownload(row: BackupInfo) {
  try {
    const res: any = await backupApi.download(row.name)
    const blob = new Blob([res.data])
    const a = document.createElement('a')
    a.href = URL.createObjectURL(blob)
    a.download = row.name
    a.click()
    URL.revokeObjectURL(a.href)
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '下载失败')
  }
}

async function onRestore(row: BackupInfo) {
  try {
    await ElMessageBox.confirm(
      `确定从「${row.name}」恢复?\n\n⚠️ 恢复会覆盖当前数据库与脚本目录,恢复前将自动备份当前状态。`,
      '恢复确认', { type: 'warning', confirmButtonText: '确认恢复' }
    )
  } catch { return }
  try {
    await backupApi.restoreByName(row.name)
    ElMessage.success('恢复成功')
    load()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '恢复失败')
  }
}

async function uploadRestore(opt: any) {
  try {
    await backupApi.restoreUpload(opt.file)
    ElMessage.success('恢复成功')
    load()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '恢复失败')
  }
}

async function onDelete(row: BackupInfo) {
  try {
    await ElMessageBox.confirm(`确定删除备份「${row.name}」?`, '提示', { type: 'warning' })
  } catch { return }
  try {
    await backupApi.remove(row.name)
    ElMessage.success('已删除')
    load()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '删除失败')
  }
}

onMounted(() => {
  load()
  loadSettings()
})
</script>

<style scoped>
.setting-card { margin-bottom: 16px; }
.toolbar { display: flex; align-items: center; gap: 12px; margin-bottom: 12px; }
.hint { color: #909399; font-size: 12px; }
</style>
