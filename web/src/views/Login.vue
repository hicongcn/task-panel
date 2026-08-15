<template>
  <div class="login-wrap">
    <div class="login-card">
      <div class="brand">
        <div class="brand-icon">{{ panel.logo || '⏰' }}</div>
        <div>
          <h1 class="brand-name">{{ panel.title || 'Task Panel' }}</h1>
          <p class="brand-sub">定时任务管理面板</p>
        </div>
      </div>

      <template v-if="needInit">
        <p class="hint">首次使用,请初始化管理员账号</p>
        <el-form @submit.prevent="doInit">
          <el-form-item><el-input v-model="form.username" size="large" placeholder="管理员用户名" /></el-form-item>
          <el-form-item><el-input v-model="form.password" size="large" type="password" show-password placeholder="密码 (至少 6 位)" /></el-form-item>
          <el-button type="primary" size="large" :loading="loading" class="submit" @click="doInit">初始化</el-button>
        </el-form>
      </template>

      <template v-else>
        <el-form @submit.prevent="doLogin">
          <el-form-item><el-input v-model="form.username" size="large" placeholder="用户名" /></el-form-item>
          <el-form-item><el-input v-model="form.password" size="large" type="password" show-password placeholder="密码" @keyup.enter="doLogin" /></el-form-item>
          <el-form-item v-if="showTotp"><el-input v-model="form.totpCode" size="large" placeholder="6 位动态验证码" maxlength="6" @keyup.enter="doLogin" /></el-form-item>
          <el-button type="primary" size="large" :loading="loading" class="submit" @click="doLogin">登 录</el-button>
          <p v-if="showTotp" class="totp-tip">已开启双重认证,请输入验证器中的 6 位动态码</p>
        </el-form>
      </template>

      <p class="foot">Task Panel v1.1 · 轻量自研</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { systemApi } from '@/api/system'

const auth = useAuthStore()
const router = useRouter()
const form = ref({ username: '', password: '', totpCode: '' })
const loading = ref(false)
const needInit = ref(false)
const showTotp = ref(false)
const panel = ref({ title: '', logo: '' })

onMounted(async () => {
  if (auth.isLoggedIn) { router.push('/dashboard'); return }
  needInit.value = await auth.checkInit()
  try {
    const res: any = await systemApi.panelInfo()
    panel.value = res.data.data || {}
  } catch {}
})

async function doInit() {
  if (!form.value.username || !form.value.password) { ElMessage.warning('请填写用户名和密码'); return }
  loading.value = true
  try {
    await auth.init(form.value.username, form.value.password)
    ElMessage.success('初始化成功,请登录')
    needInit.value = false
  } catch {} finally { loading.value = false }
}

async function doLogin() {
  if (!form.value.username || !form.value.password) { ElMessage.warning('请填写用户名和密码'); return }
  loading.value = true
  try {
    await auth.login(form.value.username, form.value.password, form.value.totpCode)
    router.push('/dashboard')
  } catch (e: any) {
    const msg: string = e?.response?.data?.message || ''
    if (msg.includes('动态验证码')) {
      showTotp.value = true
      ElMessage.warning('该账号已开启双重认证,请输入动态验证码')
    } else {
      ElMessage.error(msg || '登录失败')
    }
  } finally { loading.value = false }
}
</script>

<style scoped>
.login-wrap {
  height: 100vh; display: flex; align-items: center; justify-content: center;
  background: linear-gradient(135deg, #1d2a3a 0%, #15202e 55%, #0f1823 100%);
  position: relative; overflow: hidden;
}
.login-wrap::before {
  content: ''; position: absolute; width: 420px; height: 420px; border-radius: 50%;
  background: radial-gradient(circle, rgba(59, 130, 246, 0.18), transparent 65%);
  top: -120px; right: -80px;
}
.login-wrap::after {
  content: ''; position: absolute; width: 380px; height: 380px; border-radius: 50%;
  background: radial-gradient(circle, rgba(37, 99, 235, 0.12), transparent 65%);
  bottom: -140px; left: -100px;
}

.login-card {
  position: relative; z-index: 2;
  width: 400px; padding: 38px 36px 24px;
  background: #fff; border-radius: 14px;
  box-shadow: 0 12px 40px rgba(0, 0, 0, 0.25);
}
.brand { display: flex; align-items: center; gap: 12px; margin-bottom: 26px; }
.brand-icon {
  width: 46px; height: 46px; border-radius: 12px;
  background: linear-gradient(135deg, #3b82f6, #2563eb);
  color: #fff; display: flex; align-items: center; justify-content: center;
  font-size: 22px; box-shadow: 0 4px 14px rgba(37, 99, 235, 0.4);
}
.brand-name { margin: 0; font-size: 22px; font-weight: 700; color: #1f2d3d; }
.brand-sub { margin: 2px 0 0; font-size: 12px; color: #909399; }

.hint { color: #909399; font-size: 13px; margin: 0 0 18px; }
.submit { width: 100%; letter-spacing: 4px; }
.totp-tip { color: #e6a23c; font-size: 12px; text-align: center; margin-top: 10px; }
.foot { text-align: center; color: #c0c4cc; font-size: 12px; margin: 22px 0 0; }
</style>
