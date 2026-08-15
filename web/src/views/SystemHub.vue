<template>
  <div class="hub">
    <el-tabs v-model="active" class="hub-tabs">
      <el-tab-pane label="通知渠道" name="notify">
        <NotifyChannels />
      </el-tab-pane>
      <el-tab-pane label="备份恢复" name="backups">
        <Backups />
      </el-tab-pane>
      <el-tab-pane label="审计日志" name="audit">
        <AuditLogs />
      </el-tab-pane>
      <el-tab-pane label="安全设置" name="settings">
        <Settings />
      </el-tab-pane>
      <el-tab-pane label="数据迁移" name="migrate">
        <Migrate />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import NotifyChannels from '@/views/NotifyChannels.vue'
import Backups from '@/views/Backups.vue'
import AuditLogs from '@/views/AuditLogs.vue'
import Settings from '@/views/Settings.vue'
import Migrate from '@/views/Migrate.vue'

const active = ref('notify')
</script>

<style scoped>
.hub-tabs :deep(.el-tabs__content) { overflow: visible; }
</style>
