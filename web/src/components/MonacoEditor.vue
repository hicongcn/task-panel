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
  })
  editor.onDidChangeModelContent(() => {
    if (applyingExternal || !editor) return
    emit('update:modelValue', editor.getValue())
  })
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
