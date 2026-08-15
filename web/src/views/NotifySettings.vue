<template>
  <el-card shadow="never">
    <template #header><span>推送模板与事件告警</span></template>
    <el-form label-width="100px" style="max-width:640px">
      <el-form-item label="成功模板">
        <el-input v-model="panelCfg.tplSuccess" type="textarea" :rows="2" placeholder="第一行为标题,其余为内容" />
      </el-form-item>
      <el-form-item label="失败模板">
        <el-input v-model="panelCfg.tplFailed" type="textarea" :rows="2" placeholder="第一行为标题,其余为内容" />
      </el-form-item>
      <el-form-item label="终止模板">
        <el-input v-model="panelCfg.tplAborted" type="textarea" :rows="2" placeholder="第一行为标题,其余为内容" />
      </el-form-item>
      <el-form-item label="占位符">
        <div class="tip"><code>{task_name}</code> 任务名 · <code>{status}</code> 状态 · <code>{duration}</code> 耗时(秒)。留空使用默认模板。</div>
      </el-form-item>
      <el-form-item label="系统事件告警">
        <el-switch v-model="panelCfg.eventAlerts" />
        <div class="tip">登录锁定 / OpenAPI 认证失败 / 备份失败时,向所有启用渠道推送告警</div>
      </el-form-item>
      <el-form-item>
        <el-button type="primary" :loading="cfgSaving" @click="saveNotifyCfg">保存通知设置</el-button>
      </el-form-item>
    </el-form>
  </el-card>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { systemApi } from '@/api/system'

const cfgSaving = ref(false)
const panelCfg = reactive({ tplSuccess: '', tplFailed: '', tplAborted: '', eventAlerts: true })

async function loadNotifyCfg() {
  try {
    const res: any = await systemApi.getConfig()
    const d = res.data.data || {}
    panelCfg.tplSuccess = d.notify_tpl_success || ''
    panelCfg.tplFailed = d.notify_tpl_failed || ''
    panelCfg.tplAborted = d.notify_tpl_aborted || ''
    panelCfg.eventAlerts = d.event_alerts !== false
  } catch {}
}

async function saveNotifyCfg() {
  cfgSaving.value = true
  try {
    await systemApi.updateConfig({
      notify_tpl_success: panelCfg.tplSuccess,
      notify_tpl_failed: panelCfg.tplFailed,
      notify_tpl_aborted: panelCfg.tplAborted,
      event_alerts: panelCfg.eventAlerts,
    })
    ElMessage.success('通知设置已保存')
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败')
  } finally { cfgSaving.value = false }
}

onMounted(loadNotifyCfg)
</script>

<style scoped>
.tip { color: #909399; font-size: 12px; margin-top: 4px; }
.tip code { background: #f0f2f5; padding: 1px 4px; border-radius: 3px; }
</style>
