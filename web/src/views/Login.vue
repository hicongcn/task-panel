<template>
  <div class="login-wrap">
    <el-card class="card">
      <h2 class="title">Task Panel</h2>
      <template v-if="needInit">
        <p class="hint">首次使用,请初始化管理员账号</p>
        <el-form @submit.prevent="doInit">
          <el-form-item><el-input v-model="form.username" placeholder="管理员用户名" /></el-form-item>
          <el-form-item><el-input v-model="form.password" type="password" show-password placeholder="密码 (至少 6 位)" /></el-form-item>
          <el-button type="primary" :loading="loading" style="width:100%" @click="doInit">初始化</el-button>
        </el-form>
      </template>
      <template v-else>
        <el-form @submit.prevent="doLogin">
          <el-form-item><el-input v-model="form.username" placeholder="用户名" /></el-form-item>
          <el-form-item><el-input v-model="form.password" type="password" show-password placeholder="密码" @keyup.enter="doLogin" /></el-form-item>
          <el-button type="primary" :loading="loading" style="width:100%" @click="doLogin">登录</el-button>
        </el-form>
      </template>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const router = useRouter()
const form = ref({ username: '', password: '' })
const loading = ref(false)
const needInit = ref(false)

onMounted(async () => {
  if (auth.isLoggedIn) { router.push('/dashboard'); return }
  needInit.value = await auth.checkInit()
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
    await auth.login(form.value.username, form.value.password)
    router.push('/dashboard')
  } catch {} finally { loading.value = false }
}
</script>

<style scoped>
.login-wrap { height: 100vh; display: flex; align-items: center; justify-content: center; background: linear-gradient(135deg, #1f2d3d, #2d3a4b); }
.card { width: 380px; }
.title { text-align: center; margin: 0 0 16px; }
.hint { color: #909399; font-size: 13px; text-align: center; margin-bottom: 18px; }
</style>
