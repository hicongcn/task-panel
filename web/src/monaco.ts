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
