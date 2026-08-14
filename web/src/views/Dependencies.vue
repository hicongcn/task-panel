<template>
  <el-card shadow="never">
    <template #header>
      <div class="head">
        <span>依赖管理</span>
        <span class="tip">⚠️ 安装/卸载会真实修改本机环境,请谨慎操作</span>
      </div>
    </template>
    <el-tabs>
      <el-tab-pane label="Python (pip 系统环境)" name="py">
        <DepPanel kind="python" />
      </el-tab-pane>
      <el-tab-pane label="Node (npm 全局)" name="node">
        <DepPanel kind="node" />
      </el-tab-pane>
    </el-tabs>
  </el-card>
</template>

<script setup lang="ts">
import DepPanel from '@/components/DepPanel.vue'
</script>

<style scoped>
.head { display: flex; justify-content: space-between; align-items: center; }
.tip { color: #e6a23c; font-size: 12px; }
</style>
