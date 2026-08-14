// Monaco 编辑器初始化:只导入核心 API + 常用语言,避免根入口全量语言导致的超大体积。
//
// monaco-editor 0.56 的 package.json exports 为 "monaco-editor/*" -> "esm/vs/*.js",
// 因此按需导入要写 "monaco-editor/editor/editor.api"(核心 API,不含语言)
// 与 "monaco-editor/languages/definitions/<lang>/register"(单个语言)。
// 根入口 "monaco-editor"(esm/vs/index.js)会拉入全部语言,体积巨大,不采用。
// 语法高亮在编辑线程内完成;这里配置核心 editor worker,保证查找/后台任务可用。
// 语言服务 worker(如 ts/json)不引入,控制体积。
import * as monaco from 'monaco-editor/editor/editor.api'
// 用根入口做类型来源(运行时被擦除,不进 bundle);editor.api 子路径无独立类型声明。
import type * as monacoTypes from 'monaco-editor'
import editorWorker from 'monaco-editor/editor/editor.worker?worker'

;(self as any).MonacoEnvironment = {
  getWorker: () => new editorWorker(),
}

// 常用语言(语法高亮)
import 'monaco-editor/languages/definitions/javascript/register'
import 'monaco-editor/languages/definitions/python/register'
import 'monaco-editor/languages/definitions/shell/register'
import 'monaco-editor/languages/definitions/go/register'
import 'monaco-editor/languages/definitions/yaml/register'
import 'monaco-editor/languages/definitions/markdown/register'
import 'monaco-editor/languages/definitions/xml/register'
import 'monaco-editor/languages/definitions/dockerfile/register'
import 'monaco-editor/languages/definitions/html/register'
import 'monaco-editor/languages/definitions/css/register'
// typescript 会引入 tsMode/lspLanguageFeatures(数百 KB),脚本管理场景暂不引入。
// json 为独立语言服务(需 worker),MVP 不引入;monaco 核心提供基础高亮。

// languageForPath 根据脚本文件名推断 Monaco 语言 id。
export function languageForPath(path: string): string {
  const name = path.toLowerCase()
  const ext = name.includes('.') ? (name.split('.').pop() || '') : ''
  const map: Record<string, string> = {
    js: 'javascript', mjs: 'javascript', cjs: 'javascript',
    py: 'python',
    sh: 'shell', bash: 'shell', zsh: 'shell',
    go: 'go',
    yaml: 'yaml', yml: 'yaml',
    md: 'markdown', markdown: 'markdown',
    xml: 'xml',
    dockerfile: 'dockerfile',
    html: 'html', htm: 'html',
    css: 'css',
  }
  if (name === 'dockerfile') return 'dockerfile'
  return map[ext] || 'plaintext'
}

export type { monacoTypes }
export default monaco
