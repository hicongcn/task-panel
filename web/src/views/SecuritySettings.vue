<template>
  <el-card shadow="never">
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
    <template #header><span>修改密码</span></template>
    <el-form label-width="90px" style="max-width:400px">
      <el-form-item label="当前密码">
        <el-input v-model="pwdForm.oldPassword" type="password" show-password placeholder="输入当前密码" />
      </el-form-item>
      <el-form-item label="新密码">
        <el-input v-model="pwdForm.newPassword" type="password" show-password placeholder="6-128 位" />
      </el-form-item>
      <el-form-item label="确认新密码">
        <el-input v-model="pwdForm.confirmPassword" type="password" show-password placeholder="再次输入新密码" />
      </el-form-item>
      <el-form-item>
        <el-button type="primary" :loading="changing" @click="onChangePassword">修改密码</el-button>
      </el-form-item>
    </el-form>
  </el-card>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import QRCode from 'qrcode'
import { useRouter } from 'vue-router'
import { authApi } from '@/api/auth'

const enabled = ref(false)
const setup = ref<{ secret: string; uri: string } | null>(null)
const qrDataUrl = ref('')
const enableCode = ref('')
const disablePassword = ref('')
const setting = ref(false)
const enabling = ref(false)
const disabling = ref(false)
const router = useRouter()
const changing = ref(false)
const pwdForm = ref({ oldPassword: '', newPassword: '', confirmPassword: '' })

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

async function onChangePassword() {
  const f = pwdForm.value
  if (!f.oldPassword || !f.newPassword) return ElMessage.warning('请填写完整')
  if (f.newPassword.length < 6) return ElMessage.warning('新密码至少 6 位')
  if (f.newPassword !== f.confirmPassword) return ElMessage.warning('两次输入的新密码不一致')
  changing.value = true
  try {
    await authApi.changePassword(f.oldPassword, f.newPassword)
    ElMessage.success('密码已修改,请重新登录')
    await authApi.logout()
    router.push('/login')
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '修改失败')
  } finally { changing.value = false }
}

onMounted(loadStatus)
</script>

<style scoped>
.card { margin-bottom: 16px; }
.qr-wrap { margin: 8px 0 10px; }
.secret-line { color: #606266; font-size: 13px; }
.secret-line code { background: #f0f2f5; padding: 2px 6px; border-radius: 3px; }
</style>
