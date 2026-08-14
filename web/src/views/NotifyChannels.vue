<template>
  <div>
    <div class="toolbar">
      <el-button type="primary" @click="openCreate">新建渠道</el-button>
    </div>

    <el-table :data="channels" v-loading="loading" border stripe>
      <el-table-column prop="name" label="名称" show-overflow-tooltip />
      <el-table-column label="类型">
        <template #default="{ row }">{{ notifyTypeLabel(row.type) }}</template>
      </el-table-column>
      <el-table-column label="启用">
        <template #default="{ row }">
          <el-switch :model-value="row.enabled" @change="(v: boolean) => onToggle(row, v)" />
        </template>
      </el-table-column>
      <el-table-column prop="created_at" label="创建时间" show-overflow-tooltip />
      <el-table-column label="操作" width="200" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="openEdit(row)">编辑</el-button>
          <el-button size="small" type="warning" @click="onTest(row)">测试</el-button>
          <el-button size="small" type="danger" @click="onDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑渠道' : '新建渠道'" width="520px">
      <el-form :model="form" label-width="110px">
        <el-form-item label="名称" required>
          <el-input v-model="form.name" placeholder="例如: 运维通知群" />
        </el-form-item>
        <el-form-item label="类型" required>
          <el-select v-model="form.type" style="width:100%" @change="onTypeChange">
            <el-option label="Webhook" value="webhook" />
            <el-option label="Telegram" value="telegram" />
            <el-option label="Bark" value="bark" />
            <el-option label="邮件" value="email" />
          </el-select>
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="form.enabled" />
        </el-form-item>
        <el-form-item label="推送设置">
          <el-checkbox-group v-model="form.pushOn">
            <el-checkbox value="success" label="success">成功推送</el-checkbox>
            <el-checkbox value="failed" label="failed">失败推送</el-checkbox>
            <el-checkbox value="aborted" label="aborted">终止推送</el-checkbox>
          </el-checkbox-group>
          <div class="tip">留空 = 运行完毕全部推送;取消勾选某状态则结果为该状态时不推送到此渠道</div>
        </el-form-item>

        <template v-if="form.type === 'webhook'">
          <el-form-item label="URL" required>
            <el-input v-model="form.config.url" placeholder="https://example.com/hook" />
          </el-form-item>
          <el-form-item label="方法">
            <el-select v-model="form.config.method" style="width:100%">
              <el-option label="POST" value="POST" />
              <el-option label="GET" value="GET" />
              <el-option label="PUT" value="PUT" />
            </el-select>
          </el-form-item>
        </template>

        <template v-if="form.type === 'telegram'">
          <el-form-item label="Bot Token" required>
            <el-input v-model="form.config.bot_token" placeholder="123456:ABC..." />
          </el-form-item>
          <el-form-item label="Chat ID" required>
            <el-input v-model="form.config.chat_id" placeholder="-100123456789" />
          </el-form-item>
        </template>

        <template v-if="form.type === 'bark'">
          <el-form-item label="服务器">
            <el-input v-model="form.config.server" placeholder="https://api.day.app(可留空)" />
          </el-form-item>
          <el-form-item label="Device Key" required>
            <el-input v-model="form.config.device_key" placeholder="从 Bark App 获取" />
          </el-form-item>
        </template>

        <template v-if="form.type === 'email'">
          <el-form-item label="SMTP 主机" required>
            <el-input v-model="form.config.host" placeholder="smtp.example.com" />
          </el-form-item>
          <el-form-item label="端口">
            <el-input-number v-model="form.config.port" :min="1" :max="65535" style="width:100%" />
          </el-form-item>
          <el-form-item label="用户名">
            <el-input v-model="form.config.username" />
          </el-form-item>
          <el-form-item label="密码">
            <el-input v-model="form.config.password" type="password" placeholder="******(留空/掩码表示不修改)" show-password />
          </el-form-item>
          <el-form-item label="发件人" required>
            <el-input v-model="form.config.from" placeholder="noreply@example.com" />
          </el-form-item>
          <el-form-item label="收件人" required>
            <el-input v-model="form.config.to" placeholder="admin@example.com" />
          </el-form-item>
        </template>
      </el-form>

      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="success" @click="onTestForm">测试发送</el-button>
        <el-button type="primary" @click="onSave">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { notifyApi, notifyTypeLabel, type NotifyChannel } from '@/api/notify'

const channels = ref<NotifyChannel[]>([])
const loading = ref(false)
const dialogVisible = ref(false)
const editingId = ref<number | null>(null)

const form = reactive<any>({ name: '', type: 'webhook', enabled: true, pushOn: ['success', 'failed', 'aborted'], config: { method: 'POST', port: 587 } })

async function load() {
  loading.value = true
  try {
    const res: any = await notifyApi.list()
    channels.value = res.data.data || []
  } catch {} finally { loading.value = false }
}

function openCreate() {
  editingId.value = null
  Object.assign(form, { name: '', type: 'webhook', enabled: true, pushOn: ['success', 'failed', 'aborted'], config: { method: 'POST', port: 587 } })
  dialogVisible.value = true
}

function openEdit(row: NotifyChannel) {
  editingId.value = row.id
  Object.assign(form, {
    name: row.name,
    type: row.type,
    enabled: row.enabled,
    config: { method: 'POST', port: 587, ...row.config },
    pushOn: Array.isArray(row.config?.push_on) ? row.config.push_on : ['success', 'failed', 'aborted'],
  })
  dialogVisible.value = true
}

function onTypeChange() {
  form.config = { method: 'POST', port: 587 }
}

async function onToggle(row: NotifyChannel, v: boolean) {
  try {
    await notifyApi.toggle(row.id, v)
    row.enabled = v
    ElMessage.success('已更新')
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败')
    load()
  }
}

async function onSave() {
  if (!form.name.trim()) return ElMessage.warning('请填写名称')
  const payload: any = { name: form.name, type: form.type, enabled: form.enabled, config: { ...form.config, push_on: form.pushOn } }
  try {
    if (editingId.value) {
      await notifyApi.update(editingId.value, payload)
    } else {
      await notifyApi.create(payload)
    }
    ElMessage.success('已保存')
    dialogVisible.value = false
    load()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败')
  }
}

async function onTest(row: NotifyChannel) {
  await doTest(row.type, row.config)
}

async function onTestForm() {
  await doTest(form.type, form.config)
}

async function doTest(type: string, config: any) {
  try {
    await notifyApi.test(type, config)
    ElMessage.success('测试消息发送成功')
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '发送失败')
  }
}

async function onDelete(row: NotifyChannel) {
  try {
    await ElMessageBox.confirm(`确定删除渠道「${row.name}」?`, '提示', { type: 'warning' })
  } catch { return }
  try {
    await notifyApi.remove(row.id)
    ElMessage.success('已删除')
    load()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '删除失败')
  }
}

onMounted(load)
</script>

<style scoped>
.toolbar { display: flex; gap: 8px; margin-bottom: 12px; }
.tip { color: #909399; font-size: 12px; margin-top: 4px; }
</style>
