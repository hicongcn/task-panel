<template>
  <div>
    <el-card shadow="never">
      <template #header><span>数据迁移(导入 / 导出)</span></template>

      <el-alert type="warning" :closable="false" class="alert">
        ⚠️ 导出的 JSON 包含<b>环境变量明文值</b>与脚本内容,请妥善保管;导入请确认来自可信来源。
      </el-alert>

      <div class="section">
        <h4>导出</h4>
        <p class="hint">导出全部配置:任务(含标签)、脚本、环境变量,保存为 JSON 文件。</p>
        <el-button type="primary" :loading="exporting" @click="doExport">导出全部配置</el-button>
      </div>

      <el-divider />

      <div class="section">
        <h4>导入</h4>
        <p class="hint">选择之前导出的 JSON 文件。同名任务/环境变量将跳过;同名脚本将覆盖。</p>
        <el-upload :show-file-list="false" accept=".json" :http-request="doImport" :disabled="importing">
          <el-button type="warning" :loading="importing">选择 JSON 文件导入</el-button>
        </el-upload>
      </div>

      <el-divider v-if="lastResult" />

      <div v-if="lastResult" class="section">
        <h4>最近一次导入结果</h4>
        <el-descriptions :column="3" border size="small">
          <el-descriptions-item label="任务">{{ lastResult.tasks_ok }} 成功 / {{ lastResult.tasks_skipped }} 跳过</el-descriptions-item>
          <el-descriptions-item label="脚本">{{ lastResult.scripts_ok }} 成功 / {{ lastResult.scripts_skipped }} 跳过</el-descriptions-item>
          <el-descriptions-item label="环境变量">{{ lastResult.envs_ok }} 成功 / {{ lastResult.envs_skipped }} 跳过</el-descriptions-item>
        </el-descriptions>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { migrateApi } from '@/api/migrate'

const exporting = ref(false)
const importing = ref(false)
const lastResult = ref<any>(null)

async function doExport() {
  exporting.value = true
  try {
    const res: any = await migrateApi.export()
    const blob = new Blob([JSON.stringify(res.data.data, null, 2)], { type: 'application/json' })
    const a = document.createElement('a')
    a.href = URL.createObjectURL(blob)
    a.download = `taskpanel-backup-${new Date().toISOString().slice(0, 10)}.json`
    a.click()
    URL.revokeObjectURL(a.href)
    ElMessage.success('导出成功')
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '导出失败')
  } finally { exporting.value = false }
}

async function doImport(opt: any) {
  importing.value = true
  try {
    const text = await opt.file.text()
    const data = JSON.parse(text)
    if (!data || (!data.tasks && !data.scripts && !data.envs)) {
      return ElMessage.error('文件格式不正确,缺少 tasks/scripts/envs')
    }
    const res: any = await migrateApi.import({
      tasks: data.tasks || [],
      scripts: data.scripts || [],
      envs: data.envs || [],
    })
    lastResult.value = res.data.data || {}
    ElMessage.success('导入完成')
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || (e instanceof SyntaxError ? 'JSON 解析失败' : '导入失败'))
  } finally { importing.value = false }
}
</script>

<style scoped>
.alert { margin-bottom: 14px; }
.section { padding: 0 4px; }
.section h4 { margin: 0 0 6px; }
.hint { color: #909399; font-size: 13px; margin: 0 0 10px; }
</style>
