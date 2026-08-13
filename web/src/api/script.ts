import request from './request'

export interface ScriptNode {
  key: string
  title: string
  type: 'directory' | 'file'
  extension?: string
  children?: ScriptNode[]
}

export const scriptApi = {
  tree: () => request.get('/scripts/tree'),
  content: (path: string) =>
    request.get('/scripts/content', { params: { path } }),
  save: (path: string, content: string) =>
    request.put('/scripts/content', { path, content }),
  createDir: (path: string) => request.post('/scripts/directory', { path }),
  remove: (path: string) => request.delete('/scripts', { params: { path } }),
  rename: (oldPath: string, newName: string) =>
    request.put('/scripts/rename', { old_path: oldPath, new_name: newName }),
  // 后端脚本调试最长 60 秒,单独放大超时避免 30s 默认超时误报。
  run: (path: string) => request.post('/scripts/run', { path }, { timeout: 90000 }),
  runCode: (code: string, language: string) =>
    request.post('/scripts/run-code', { code, language }, { timeout: 90000 }),
}
