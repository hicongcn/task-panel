<template>
  <div ref="el" class="monaco-editor" />
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch } from 'vue'
import monaco, { type monacoTypes } from '@/monaco'

const props = withDefaults(defineProps<{
  modelValue: string
  language?: string
  readOnly?: boolean
}>(), {
  language: 'plaintext',
  readOnly: false,
})
const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
}>()

const el = ref<HTMLDivElement>()
let editor: monacoTypes.editor.IStandaloneCodeEditor | null = null
let applyingExternal = false
let titleObserver: MutationObserver | null = null

// find widget 按钮的原生 title 悬停提示在屏幕边缘会溢出视口、遮挡点击,
// 这里在查找框出现时移除按钮的 title,仅保留 aria-label。
function stripFindTitles() {
  if (!el.value) return
  el.value.querySelectorAll('.find-widget [title]').forEach((b) => b.removeAttribute('title'))
}

onMounted(() => {
  if (!el.value) return
  editor = monaco.editor.create(el.value, {
    value: props.modelValue,
    language: props.language,
    theme: 'vs-dark',
    automaticLayout: true,
    minimap: { enabled: false },
    fontSize: 13,
    scrollBeyondLastLine: false,
    readOnly: props.readOnly,
    tabSize: 2,
    wordWrap: 'on',
    renderWhitespace: 'selection',
    // ---- 编辑功能增强 ----
    bracketPairColorization: { enabled: true }, // 括号配对着色
    autoClosingBrackets: 'always',              // 自动闭合括号
    autoClosingQuotes: 'always',                // 自动闭合引号
    autoIndent: 'full',                         // 自动缩进
    matchBrackets: 'always',                    // 高亮配对括号
    folding: true,                              // 代码折叠
    foldingHighlight: true,                     // 折叠标记高亮
    renderLineHighlight: 'all',                 // 当前行高亮
    fontLigatures: true,                        // 字体连字
    smoothScrolling: true,                      // 平滑滚动
    cursorBlinking: 'smooth',                   // 光标平滑闪烁
    cursorSmoothCaretAnimation: 'on',           // 光标平滑移动
    padding: { top: 8, bottom: 8 },             // 编辑区上下留白
    contextmenu: true,                          // 右键菜单
    quickSuggestions: false,                    // 基础语言无补全,避免弹窗噪音
    suggestOnTriggerCharacters: false,
    wordBasedSuggestions: 'off',
  })
  editor.onDidChangeModelContent(() => {
    if (applyingExternal || !editor) return
    emit('update:modelValue', editor.getValue())
  })
  // 监听查找框出现,清理按钮 title
  titleObserver = new MutationObserver(() => stripFindTitles())
  titleObserver.observe(el.value, { childList: true, subtree: true })
  stripFindTitles()
})

watch(() => props.language, (lang) => {
  const m = editor?.getModel()
  if (editor && m && lang) {
    monaco.editor.setModelLanguage(m, lang)
  }
})

watch(() => props.modelValue, (val) => {
  const m = editor?.getModel()
  if (!editor || !m) return
  if (m.getValue() !== val) {
    applyingExternal = true
    m.setValue(val)
    applyingExternal = false
  }
})

onBeforeUnmount(() => {
  titleObserver?.disconnect()
  titleObserver = null
  editor?.dispose()
  editor = null
})
</script>

<style scoped>
.monaco-editor {
  width: 100%;
  height: 100%;
  min-height: 220px;
  border: 1px solid var(--el-border-color);
  border-radius: 4px;
  overflow: hidden;
}
</style>
