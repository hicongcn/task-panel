import request from './request'

export interface PkgInfo {
  name: string
  version: string
}

// 安装/卸载可能耗时(网络操作),超时放宽到 200s(后端 180s 超时)。
const opTimeout = 200000

export const depApi = {
  listPython: () => request.get('/deps/python'),
  installPython: (pkg: string) => request.post('/deps/python/install', { package: pkg }, { timeout: opTimeout }),
  uninstallPython: (pkg: string) => request.post('/deps/python/uninstall', { package: pkg }, { timeout: opTimeout }),
  listNode: () => request.get('/deps/node'),
  installNode: (pkg: string) => request.post('/deps/node/install', { package: pkg }, { timeout: opTimeout }),
  uninstallNode: (pkg: string) => request.post('/deps/node/uninstall', { package: pkg }, { timeout: opTimeout }),
}
