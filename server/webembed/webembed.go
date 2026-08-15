// Package webembed 将前端构建产物(web/dist)嵌入二进制,实现单文件分发。
//
// 说明:go:embed 无法引用模块外路径(web/dist 在仓库根),故构建脚本
// (releases/build-all.sh)会把 web/dist 同步到本目录 dist/ 后再编译;
// 仓库内 dist/.gitkeep 为占位文件,保证开发环境可编译。
package webembed

import "embed"

//go:embed dist
var Dist embed.FS
