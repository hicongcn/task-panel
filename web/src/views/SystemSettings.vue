<template>
  <div>
    <el-card shadow="never" class="card">
      <template #header><span>面板外观</span></template>
      <el-form label-width="100px" style="max-width:480px">
        <el-form-item label="面板标题">
          <el-input v-model="panelCfg.title" placeholder="Task Panel" maxlength="64" />
        </el-form-item>
        <el-form-item label="面板图标">
          <el-input v-model="panelCfg.logo" placeholder="一个 emoji,如 ⏰ 🐉 🚀(留空用默认)" maxlength="16" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="cfgSaving" @click="savePanelCfg">保存外观</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card shadow="never" class="card">
      <template #header><span>日志自动清理</span></template>
      <el-form label-width="100px" style="max-width:480px">
        <el-form-item label="保留天数">
          <el-input-number v-model="panelCfg.cleanDays" :min="0" :max="3650" />
          <div class="tip">0 表示不自动清理;开启后每天自动删除 N 天前的执行日志(记录 + 文件)</div>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="cfgSaving" @click="saveCleanCfg">保存清理设置</el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { systemApi } from '@/api/system'

const cfgSaving = ref(false)
const panelCfg = reactive({ title: '', logo: '', cleanDays: 0 })

async function loadPanelCfg() {
  try {
    const res: any = await systemApi.getConfig()
    const d = res.data.data || {}
    panelCfg.title = d.panel_title || ''
    panelCfg.logo = d.panel_logo || ''
    panelCfg.cleanDays = Number(d.log_clean_days) || 0
  } catch {}
}

async function savePanelCfg() {
  cfgSaving.value = true
  try {
    await systemApi.updateConfig({ panel_title: panelCfg.title, panel_logo: panelCfg.logo })
    ElMessage.success('外观已保存,刷新后生效')
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败')
  } finally { cfgSaving.value = false }
}

async function saveCleanCfg() {
  cfgSaving.value = true
  try {
    await systemApi.updateConfig({ log_clean_days: panelCfg.cleanDays })
    ElMessage.success('日志清理设置已保存')
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败')
  } finally { cfgSaving.value = false }
}

onMounted(loadPanelCfg)
</script>

<style scoped>
.card { margin-bottom: 16px; }
.tip { color: #909399; font-size: 12px; margin-top: 4px; }
</style>
