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
import tsWorker from 'monaco-editor/language/typescript/ts.worker?worker'
import jsonWorker from 'monaco-editor/language/json/json.worker?worker'

;(self as any).MonacoEnvironment = {
  getWorker: (_id: string, label: string) => {
    if (label === 'typescript' || label === 'javascript') return new tsWorker()
    if (label === 'json') return new jsonWorker()
    return new editorWorker()
  },
}

// 编辑器功能(features):查找/替换、注释、多光标、代码折叠、右键菜单等
// 从 editor.main 提取的非语言导入,保证 Cmd/Ctrl+F 等快捷键被编辑器接管。
import 'monaco-editor/editor/contrib/anchorSelect/browser/anchorSelect.js'
import 'monaco-editor/editor/contrib/bracketMatching/browser/bracketMatching.js'
import 'monaco-editor/editor/contrib/caretOperations/browser/transpose.js'
import 'monaco-editor/editor/contrib/clipboard/browser/clipboard.js'
import 'monaco-editor/editor/contrib/codeAction/browser/codeActionContributions.js'
import 'monaco-editor/editor/contrib/codelens/browser/codelensController.js'
import 'monaco-editor/editor/contrib/colorPicker/browser/colorPickerContribution.js'
import 'monaco-editor/editor/contrib/comment/browser/comment.js'
import 'monaco-editor/editor/contrib/contextmenu/browser/contextmenu.js'
import 'monaco-editor/editor/contrib/cursorUndo/browser/cursorUndo.js'
import 'monaco-editor/editor/contrib/dnd/browser/dnd.js'
import 'monaco-editor/editor/contrib/documentSymbols/browser/documentSymbols.js'
import 'monaco-editor/editor/contrib/dropOrPasteInto/browser/dropIntoEditorContribution.js'
import 'monaco-editor/features/find/register.js'
import 'monaco-editor/editor/contrib/floatingMenu/browser/floatingMenu.contribution.js'
import 'monaco-editor/editor/contrib/folding/browser/folding.js'
import 'monaco-editor/editor/contrib/fontZoom/browser/fontZoom.js'
import 'monaco-editor/editor/contrib/format/browser/formatActions.js'
import 'monaco-editor/editor/contrib/gotoError/browser/gotoError.js'
import 'monaco-editor/editor/standalone/browser/quickAccess/standaloneGotoLineQuickAccess.js'
import 'monaco-editor/editor/contrib/gotoSymbol/browser/link/goToDefinitionAtPosition.js'
import 'monaco-editor/editor/contrib/gpu/browser/gpuActions.js'
import 'monaco-editor/editor/contrib/hover/browser/hoverContribution.js'
import 'monaco-editor/editor/contrib/indentation/browser/indentation.js'
import 'monaco-editor/editor/contrib/inlayHints/browser/inlayHintsContribution.js'
import 'monaco-editor/editor/contrib/inlineCompletions/browser/inlineCompletions.contribution.js'
import 'monaco-editor/editor/contrib/inlineProgress/browser/inlineProgress.js'
import 'monaco-editor/editor/contrib/inPlaceReplace/browser/inPlaceReplace.js'
import 'monaco-editor/editor/contrib/insertFinalNewLine/browser/insertFinalNewLine.js'
import 'monaco-editor/editor/contrib/lineSelection/browser/lineSelection.js'
import 'monaco-editor/editor/contrib/linesOperations/browser/linesOperations.js'
import 'monaco-editor/editor/contrib/linkedEditing/browser/linkedEditing.js'
import 'monaco-editor/editor/contrib/links/browser/links.js'
import 'monaco-editor/editor/contrib/longLinesHelper/browser/longLinesHelper.js'
import 'monaco-editor/editor/contrib/middleScroll/browser/middleScroll.contribution.js'
import 'monaco-editor/editor/contrib/multicursor/browser/multicursor.js'
import 'monaco-editor/editor/contrib/parameterHints/browser/parameterHints.js'
import 'monaco-editor/editor/contrib/placeholderText/browser/placeholderText.contribution.js'
import 'monaco-editor/editor/standalone/browser/quickAccess/standaloneCommandsQuickAccess.js'
import 'monaco-editor/editor/standalone/browser/quickAccess/standaloneHelpQuickAccess.js'
import 'monaco-editor/editor/standalone/browser/quickAccess/standaloneGotoSymbolQuickAccess.js'
import 'monaco-editor/editor/contrib/readOnlyMessage/browser/contribution.js'
import 'monaco-editor/editor/contrib/rename/browser/rename.js'
import 'monaco-editor/editor/contrib/sectionHeaders/browser/sectionHeaders.js'
import 'monaco-editor/editor/contrib/smartSelect/browser/smartSelect.js'
import 'monaco-editor/editor/contrib/snippet/browser/snippetController2.js'
import 'monaco-editor/editor/contrib/stickyScroll/browser/stickyScrollContribution.js'
import 'monaco-editor/editor/contrib/suggest/browser/suggestInlineCompletions.js'
import 'monaco-editor/editor/contrib/toggleTabFocusMode/browser/toggleTabFocusMode.js'
import 'monaco-editor/editor/contrib/tokenization/browser/tokenization.js'
import 'monaco-editor/editor/contrib/unicodeHighlighter/browser/unicodeHighlighter.js'
import 'monaco-editor/editor/contrib/unusualLineTerminators/browser/unusualLineTerminators.js'
import 'monaco-editor/editor/contrib/wordHighlighter/browser/wordHighlighter.js'
import 'monaco-editor/editor/contrib/wordOperations/browser/wordOperations.js'
import 'monaco-editor/editor/contrib/wordPartOperations/browser/wordPartOperations.js'
import 'monaco-editor/editor/browser/coreCommands.js'
import 'monaco-editor/editor/contrib/caretOperations/browser/caretOperations.js'
import 'monaco-editor/editor/contrib/dropOrPasteInto/browser/copyPasteContribution.js'
import 'monaco-editor/editor/contrib/find/browser/findController.js'
import 'monaco-editor/editor/contrib/gotoSymbol/browser/goToCommands.js'
import 'monaco-editor/editor/contrib/gotoError/browser/markerSelectionStatus.js'
import 'monaco-editor/editor/contrib/suggest/browser/suggestController.js'
import 'monaco-editor/editor/common/standaloneStrings.js'

// 语言服务(代码补全):JS/TS 与 JSON
import 'monaco-editor/language/typescript/monaco.contribution'
import 'monaco-editor/language/json/monaco.contribution'

// 全部基础语言(语法高亮):javascript/python/shell/go/yaml/markdown/xml/dockerfile/
// html/css/java/c/cpp/csharp/rust/php/ruby/lua/sql/powershell/perl/swift/kotlin/ini/bat/r 等
import 'monaco-editor/basic-languages/monaco.contribution' 

// languageForPath 根据脚本文件名推断 Monaco 语言 id。
export function languageForPath(path: string): string {
  const name = path.toLowerCase()
  const ext = name.includes('.') ? (name.split('.').pop() || '') : ''
  const map: Record<string, string> = {
    js: 'javascript', mjs: 'javascript', cjs: 'javascript',
    ts: 'typescript', mts: 'typescript', cts: 'typescript',
    json: 'json',
    py: 'python',
    sh: 'shell', bash: 'shell', zsh: 'shell',
    go: 'go',
    yaml: 'yaml', yml: 'yaml',
    md: 'markdown', markdown: 'markdown',
    xml: 'xml',
    dockerfile: 'dockerfile',
    html: 'html', htm: 'html',
    css: 'css',
    java: 'java',
    c: 'cpp', h: 'cpp', hpp: 'cpp', cpp: 'cpp', cc: 'cpp', cxx: 'cpp',
    cs: 'csharp',
    rs: 'rust',
    php: 'php',
    rb: 'ruby',
    lua: 'lua',
    sql: 'sql',
    ps1: 'powershell',
    pl: 'perl', pm: 'perl',
    swift: 'swift',
    kt: 'kotlin',
    ini: 'ini', toml: 'ini', conf: 'ini',
    bat: 'bat', cmd: 'bat',
    r: 'r',
  }
  if (name === 'dockerfile') return 'dockerfile'
  return map[ext] || 'plaintext'
}

export type { monacoTypes }
export default monaco
