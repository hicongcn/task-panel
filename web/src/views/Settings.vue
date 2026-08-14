<template>
  <div>
    <el-card shadow="never" class="card">
      <template #header><span>双重认证 (2FA / TOTP)</span></template>

      <template v-if="enabled">
        <el-alert type="success" :closable="false" title="已启用双重认证" description="登录时需要输入验证器中的 6 位动态码。丢失验证器可用 CLI 恢复: taskpanel account-reset --user admin --disable-2fa" style="margin-bottom:14px" />
        <el-form label-width="90px" style="max-width:400px">
          <el-form-item label="当前密码">
            <el-input v-model="disablePassword" type="password" show-password placeholder="输入密码以关闭 2FA" />
          </el-form-item>
          <el-button type="danger" :loading="disabling" @click="onDisable">关闭双重认证</el-button>
        </el-form>
      </template>

      <template v-else>
        <template v-if="!setup">
          <el-alert type="info" :closable="false" title="未启用" description="启用后,登录除密码外还需输入验证器动态码,增强账号安全" style="margin-bottom:14px" />
          <el-button type="primary" :loading="setting" @click="onSetup">开始绑定</el-button>
        </template>
        <template v-else>
          <el-steps :active="1" simple style="margin-bottom:14px">
            <el-step title="1. 扫描二维码" />
            <el-step title="2. 输入动态码" />
          </el-steps>
          <div class="qr-wrap"><img :src="qrDataUrl" alt="二维码" /></div>
          <p class="secret-line">密钥:<code>{{ setup.secret }}</code><el-button size="small" link type="primary" @click="copy(setup.secret)">复制</el-button></p>
          <el-form label-width="90px" style="max-width:400px">
            <el-form-item label="动态验证码">
              <el-input v-model="enableCode" placeholder="验证器中的 6 位码" maxlength="6" />
            </el-form-item>
          </el-form>
          <el-button type="primary" :loading="enabling" @click="onEnable">确认启用</el-button>
          <el-button @click="setup = null">取消</el-button>
        </template>
      </template>
    </el-card>

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

    <el-card shadow="never" class="card">
      <template #header><span>通知推送模板</span></template>
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
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import QRCode from 'qrcode'
import { authApi } from '@/api/auth'
import { systemApi } from '@/api/system'

const enabled = ref(false)
const setup = ref<{ secret: string; uri: string } | null>(null)
const qrDataUrl = ref('')
const enableCode = ref('')
const disablePassword = ref('')
const setting = ref(false)
const enabling = ref(false)
const disabling = ref(false)
const cfgSaving = ref(false)
const panelCfg = reactive({ title: '', logo: '', cleanDays: 0, tplSuccess: '', tplFailed: '', tplAborted: '', eventAlerts: true })

async function loadPanelCfg() {
  try {
    const res: any = await systemApi.getConfig()
    const d = res.data.data || {}
    panelCfg.title = d.panel_title || ''
    panelCfg.logo = d.panel_logo || ''
    panelCfg.cleanDays = Number(d.log_clean_days) || 0
    panelCfg.tplSuccess = d.notify_tpl_success || ''
    panelCfg.tplFailed = d.notify_tpl_failed || ''
    panelCfg.tplAborted = d.notify_tpl_aborted || ''
    panelCfg.eventAlerts = d.event_alerts !== false
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

async function loadStatus() {
  try {
    const res: any = await authApi.totpStatus()
    enabled.value = res.data.data.enabled
  } catch {}
}

async function onSetup() {
  setting.value = true
  try {
    const res: any = await authApi.totpSetup()
    const s = res.data.data
    setup.value = s
    qrDataUrl.value = await QRCode.toDataURL(s.uri, { width: 200, margin: 1 })
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '获取绑定信息失败')
  } finally { setting.value = false }
}

async function onEnable() {
  const s = setup.value
  if (!s || enableCode.value.length !== 6) return ElMessage.warning('请输入 6 位动态码')
  enabling.value = true
  try {
    await authApi.totpEnable(s.secret, enableCode.value)
    ElMessage.success('已启用双重认证')
    setup.value = null
    enableCode.value = ''
    loadStatus()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '启用失败')
  } finally { enabling.value = false }
}

async function onDisable() {
  if (!disablePassword.value) return ElMessage.warning('请输入当前密码')
  disabling.value = true
  try {
    await authApi.totpDisable(disablePassword.value)
    ElMessage.success('已关闭双重认证')
    disablePassword.value = ''
    loadStatus()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '关闭失败')
  } finally { disabling.value = false }
}

async function copy(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success('已复制')
  } catch { ElMessage.warning('复制失败,请手动复制') }
}

onMounted(() => { loadStatus(); loadPanelCfg() })
</script>

<style scoped>
.card { margin-bottom: 16px; }
.qr-wrap { margin: 8px 0 10px; }
.tip { color: #909399; font-size: 12px; margin-top: 4px; }
.secret-line { color: #606266; font-size: 13px; }
.secret-line code { background: #f0f2f5; padding: 2px 6px; border-radius: 3px; }
</style>
