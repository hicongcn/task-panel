<template>
  <div class="log-surface" ref="box">
    <span v-for="(line, i) in htmlLines" :key="i" v-html="line"></span>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onUnmounted, nextTick } from 'vue'
import { ansiToHtml } from '@/utils/ansi'
import { logApi, liveLogURL } from '@/api/log'

const props = defineProps<{ taskId: number; autoScroll?: boolean }>()

const box = ref<HTMLDivElement>()
const htmlLines = ref<string[]>([])
let es: EventSource | null = null
let retryCount = 0
let retryTimer: ReturnType<typeof setTimeout> | null = null
const MAX_RETRIES = 12

function append(line: string) {
  htmlLines.value.push(ansiToHtml(line))
  if (htmlLines.value.length > 5000) htmlLines.value.splice(0, htmlLines.value.length - 5000)
  if (props.autoScroll !== false) {
    nextTick(() => {
      if (box.value) box.value.scrollTop = box.value.scrollHeight
    })
  }
}

async function connect() {
  disconnect()
  htmlLines.value = []
  retryCount = 0
  // 先经鉴权接口换取短期 SSE 票据,再建立连接(避免 JWT 直接暴露在 URL)。
  const res: any = await logApi.liveTicket(props.taskId).catch(() => null)
  if (!res?.data?.ticket) return
  openStream(res.data.ticket)
}

function openStream(ticket: string) {
  es = new EventSource(liveLogURL(props.taskId, ticket))
  es.addEventListener('data', (e: MessageEvent) => append(e.data))
  es.addEventListener('done', (e: MessageEvent) => {
    // 任务刚触发、后端尚未建立日志流时会返回 not_running,
    // 这里做有限次重试,避免手动运行后打开实时日志却立刻结束。
    if (e.data === 'not_running' && retryCount < MAX_RETRIES) {
      retryCount++
      retryTimer = setTimeout(() => openStream(ticket), 800)
      return
    }
    if (e.data) append(`[结束: ${e.data}]`)
    disconnect()
  })
  es.addEventListener('error', (e: any) => {
    if (e?.data) append(`[错误: ${e.data}]`)
  })
  es.onerror = () => {
    // 浏览器 EventSource 遇到非 200 会触发 onerror,这里直接关闭避免无限重连。
    disconnect()
  }
}

function disconnect() {
  if (retryTimer) { clearTimeout(retryTimer); retryTimer = null }
  if (es) { es.close(); es = null }
}

watch(() => props.taskId, () => connect(), { immediate: true })
onUnmounted(disconnect)

defineExpose({ connect, disconnect })
</script>
